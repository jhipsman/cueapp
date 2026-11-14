package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jhipsman/cueapp/internal/deezer"
	"github.com/jhipsman/cueapp/internal/indexers"
	"github.com/jhipsman/cueapp/internal/music"
	"github.com/jhipsman/cueapp/internal/premiumize"
)

// Music, streamed (Watch's Music section): Deezer's public catalog for what
// to show (charts, search, albums, artists), and the sound from the owner's
// own sources, as films are: an album is searched for on the torrent sites,
// Premiumize is asked which releases it already has, and the songs play
// straight from Premiumize. One it doesn't have yet is fetched into the
// owner's Premiumize cloud first (usually minutes). Nothing is stored on
// the server, so it works in Streaming only mode.

const (
	listenKeep        = 3 * time.Hour    // how long an album's links are reused
	listenRetry       = 10 * time.Minute // after finding nothing, how long before looking again
	listenLookTimeout = 90 * time.Second
	listenMaxChecked  = 15 // releases checked with Premiumize per album
)

var (
	listenCatalogOnce sync.Once
	listenCatalogC    *deezer.Client
	listenCatalogBase = "" // tests point it at a fake Deezer
)

func listenCatalog() *deezer.Client {
	listenCatalogOnce.Do(func() { listenCatalogC = deezer.New(listenCatalogBase) })
	return listenCatalogC
}

// albumSound is where one album's songs stand.
type albumSound struct {
	Status   string       `json:"status"` // looking, fetching, ready, none
	Message  string       `json:"message,omitempty"`
	Progress float64      `json:"progress,omitempty"` // 0 to 1 while fetching
	Release  string       `json:"release,omitempty"`
	Tracks   []soundTrack `json:"tracks,omitempty"`

	at       time.Time
	transfer string
	checked  time.Time // when the transfer was last asked about
}

// soundTrack is one song's address, by the catalog's track id.
type soundTrack struct {
	ID  int64  `json:"id"`
	URL string `json:"url"`
}

var listenState = struct {
	sync.Mutex
	albums map[int64]*albumSound
}{albums: map[int64]*albumSound{}}

// ---- Catalog ----

// GET /api/listen/home: charts and new releases.
func (s *Server) handleListenHome(w http.ResponseWriter, r *http.Request) {
	h, err := listenCatalog().Home(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "Couldn't load music right now. Try again in a moment.")
		return
	}
	writeJSON(w, http.StatusOK, h)
}

// GET /api/listen/search?q=: songs, albums and artists.
func (s *Server) handleListenSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if len(q) > 200 {
		q = q[:200]
	}
	res, err := listenCatalog().Search(r.Context(), q)
	if err != nil {
		writeError(w, http.StatusBadGateway, "Couldn't search music right now. Try again in a moment.")
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func listenID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "That isn't a valid id.")
		return 0, false
	}
	return id, true
}

// GET /api/listen/album/{id}: an album and its tracklist.
func (s *Server) handleListenAlbum(w http.ResponseWriter, r *http.Request) {
	id, ok := listenID(w, r)
	if !ok {
		return
	}
	a, err := listenCatalog().Album(r.Context(), id)
	if errors.Is(err, deezer.ErrNotFound) {
		writeError(w, http.StatusNotFound, "That album isn't in the catalog.")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "Couldn't load that album right now. Try again in a moment.")
		return
	}
	writeJSON(w, http.StatusOK, a)
}

// GET /api/listen/artist/{id}: an artist's top songs, albums and similar artists.
func (s *Server) handleListenArtist(w http.ResponseWriter, r *http.Request) {
	id, ok := listenID(w, r)
	if !ok {
		return
	}
	p, err := listenCatalog().Artist(r.Context(), id)
	if errors.Is(err, deezer.ErrNotFound) {
		writeError(w, http.StatusNotFound, "That artist isn't in the catalog.")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "Couldn't load that artist right now. Try again in a moment.")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// ---- Sound ----

// GET /api/listen/album/{id}/sound: where the album's songs stand. The first
// ask starts looking; ask again every couple of seconds until the status is
// ready (with each song's address) or none (with why).
func (s *Server) handleListenSound(w http.ResponseWriter, r *http.Request) {
	id, ok := listenID(w, r)
	if !ok {
		return
	}
	pm := s.premiumizeClient()
	if pm == nil {
		writeJSON(w, http.StatusOK, albumSound{Status: "none", Message: "Music plays through Premiumize. Add your Premiumize key in Settings first."})
		return
	}
	listenState.Lock()
	st := listenState.albums[id]
	switch {
	case st == nil,
		st.Status == "ready" && time.Since(st.at) > listenKeep,
		st.Status == "none" && time.Since(st.at) > listenRetry || r.URL.Query().Get("again") == "1" && st.Status == "none":
		st = &albumSound{Status: "looking", at: time.Now()}
		listenState.albums[id] = st
		go s.lookForAlbum(id, pm)
	case st.Status == "fetching" && time.Since(st.checked) > 3*time.Second:
		st.checked = time.Now()
		go s.checkFetch(id, pm)
	}
	out := *st
	listenState.Unlock()
	writeJSON(w, http.StatusOK, out)
}

func setSound(id int64, st albumSound) {
	listenState.Lock()
	defer listenState.Unlock()
	st.at = time.Now()
	listenState.albums[id] = &st
}

// lookForAlbum finds a release of the album Premiumize can play, or starts
// fetching the best one it doesn't have yet.
func (s *Server) lookForAlbum(id int64, pm *premiumize.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), listenLookTimeout)
	defer cancel()
	album, err := listenCatalog().Album(ctx, id)
	if err != nil {
		setSound(id, albumSound{Status: "none", Message: "Couldn't load that album right now. Try again in a moment."})
		return
	}
	instances, err := s.IndexerRepo.List()
	if err != nil {
		setSound(id, albumSound{Status: "none", Message: "Couldn't read the search sites."})
		return
	}
	var enabled []indexers.Instance
	for _, inst := range instances {
		if inst.Enabled {
			enabled = append(enabled, inst)
		}
	}
	if len(enabled) == 0 {
		setSound(id, albumSound{Status: "none", Message: "There are no search sites yet. Add a torrent site that has music in Settings > Torrent sites."})
		return
	}
	title := plainAlbumTitle(album.Title)
	query := album.Artist.Name + " " + title
	// The sites' music sections first; then everything, for sites that
	// file albums elsewhere or don't label them.
	var (
		failed  []string
		found   int
		torrent int
		ranked  []indexers.Result
	)
	for _, cats := range [][]int{musicCategories, nil} {
		outcomes := indexers.SearchAll(ctx, enabled, query, cats)
		failed = failed[:0]
		for _, o := range outcomes {
			if o.Err != nil {
				failed = append(failed, o.IndexerName)
			}
		}
		all := dropBlocked(indexers.MergeResults(outcomes), s.blockedKeys())
		results := torrentResults(all)
		found, torrent = max(found, len(all)), max(torrent, len(results))
		if ranked = rankAlbumReleases(results, album.Artist.Name, title, album.Year()); len(ranked) > 0 {
			break
		}
	}
	slog.Info("listen: search", "album", album.Title, "artist", album.Artist.Name, "sites", len(enabled), "didnt_answer", len(failed), "releases", found, "torrents", torrent, "this_album", len(ranked))
	if len(ranked) == 0 {
		setSound(id, albumSound{Status: "none", Message: nothingFound(album.Artist.Name, album.Title, len(enabled), failed, found, torrent)})
		return
	}
	if len(ranked) > listenMaxChecked {
		ranked = ranked[:listenMaxChecked]
	}
	hashes := s.infoHashes(ctx, ranked)
	var items []string
	var picks []indexers.Result
	for i, h := range hashes {
		if h != "" {
			items = append(items, h)
			picks = append(picks, ranked[i])
		}
	}
	if len(items) == 0 {
		setSound(id, albumSound{Status: "none", Message: fmt.Sprintf("Couldn't open any of the releases found for %s.", album.Title)})
		return
	}
	cached, err := pm.CacheCheck(ctx, items)
	if err != nil {
		setSound(id, albumSound{Status: "none", Message: "Premiumize didn't answer. Try again in a moment."})
		return
	}
	for i, ok := range cached {
		if !ok {
			continue
		}
		files, err := pm.DirectDL(ctx, "magnet:?xt=urn:btih:"+items[i])
		if err != nil {
			continue
		}
		if tracks := matchAlbumFiles(album.Tracks, files); len(tracks) > 0 {
			slog.Info("listen: ready", "album", album.Title, "release", picks[i].Title, "songs", len(tracks))
			setSound(id, albumSound{Status: "ready", Release: picks[i].Title, Tracks: tracks})
			return
		}
	}
	// Nothing ready: fetch the best-seeded release into the Premiumize cloud.
	best := -1
	for i, p := range picks {
		if p.Seeders > 0 && (best < 0 || p.Seeders > picks[best].Seeders) {
			best = i
		}
	}
	if best < 0 {
		best = 0
	}
	tid, err := pm.CreateTransfer(ctx, "magnet:?xt=urn:btih:"+items[best])
	if err != nil {
		setSound(id, albumSound{Status: "none", Message: "Premiumize couldn't fetch this album: " + err.Error()})
		return
	}
	slog.Info("listen: fetching", "album", album.Title, "release", picks[best].Title)
	listenState.Lock()
	listenState.albums[id] = &albumSound{Status: "fetching", Release: picks[best].Title, at: time.Now(), transfer: tid, checked: time.Now(),
		Message: "Premiumize is getting this album. It usually takes a few minutes; it'll play once it's ready."}
	listenState.Unlock()
}

// checkFetch asks Premiumize how an album it's fetching is coming along.
func (s *Server) checkFetch(id int64, pm *premiumize.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	listenState.Lock()
	st := listenState.albums[id]
	if st == nil || st.Status != "fetching" {
		listenState.Unlock()
		return
	}
	tid, release := st.transfer, st.Release
	listenState.Unlock()
	t, err := pm.Transfer(ctx, tid)
	if err != nil {
		return // asked again on the next poll
	}
	switch {
	case t.Failed():
		setSound(id, albumSound{Status: "none", Message: "Premiumize couldn't get this album (" + strings.TrimSpace(t.Status+" "+t.Message) + ")."})
	case t.Done():
		album, err := listenCatalog().Album(ctx, id)
		if err != nil {
			return
		}
		files, err := pm.Files(ctx, t)
		if err != nil {
			return
		}
		if tracks := matchAlbumFiles(album.Tracks, files); len(tracks) > 0 {
			setSound(id, albumSound{Status: "ready", Release: release, Tracks: tracks})
		} else {
			setSound(id, albumSound{Status: "none", Message: "The release Premiumize got has no songs a browser can play."})
		}
	default:
		listenState.Lock()
		if cur := listenState.albums[id]; cur != nil && cur.transfer == tid {
			cur.Progress = t.Progress
		}
		listenState.Unlock()
	}
}

// ---- Choosing and matching ----

var reEdition = regexp.MustCompile(`(?i)\s*[\(\[][^\)\]]*(edition|deluxe|remaster|version|expanded|anniversary|bonus|explicit|clean)[^\)\]]*[\)\]]`)

// plainAlbumTitle drops the edition from a catalog title ("Abbey Road
// (Remastered 2019)" is searched for, and matched, as "Abbey Road").
func plainAlbumTitle(t string) string {
	p := strings.TrimSpace(reEdition.ReplaceAllString(t, ""))
	if p == "" {
		return t
	}
	return p
}

// listenRank orders sound for streaming: 320 kbit/s MP3 or AAC first (light
// on a phone's data, and transparent), then CD-quality FLAC, then smaller
// MP3s, then hi-res FLAC (very big files), then anything unnamed.
func listenRank(t music.Tier) int {
	switch t {
	case music.TierMP3320:
		return 6
	case music.TierAAC256, music.TierFLAC:
		return 5
	case music.TierMP3256:
		return 4
	case music.TierFLAC24:
		return 3
	case music.TierMP3192:
		return 2
	}
	return 1
}

// rankAlbumReleases keeps the releases of this very album (not another
// album, artist or a whole discography) and orders them best for
// streaming first, then by seeders.
func rankAlbumReleases(results []indexers.Result, artist, album string, year int) []indexers.Result {
	type scored struct {
		r    indexers.Result
		rank int
	}
	var keep []scored
	for _, res := range results {
		rel := music.ParseRelease(res.Title)
		rank := listenRank(rel.Tier())
		if ok, _ := music.ReleaseMatch(rel, artist, album, year); !ok {
			// Public sites often leave out the dash ("Artist Album 2024
			// FLAC"): the artist's and the album's words both in the name
			// will do, after the ones named properly.
			if !namesAlbum(res.Title, rel, artist, album, year) {
				continue
			}
			rank -= 10
		}
		keep = append(keep, scored{res, rank})
	}
	sort.SliceStable(keep, func(i, j int) bool {
		if keep[i].rank != keep[j].rank {
			return keep[i].rank > keep[j].rank
		}
		return keep[i].r.Seeders > keep[j].r.Seeders
	})
	out := make([]indexers.Result, len(keep))
	for i, k := range keep {
		out[i] = k.r
	}
	return out
}

// browserAudio are the kinds of sound file every browser plays (Ogg and
// Opus aren't in Safari, so they come last).
var browserAudio = map[string]int{".mp3": 3, ".m4a": 3, ".aac": 3, ".flac": 3, ".ogg": 1, ".opus": 1}

var (
	reDiscDir   = regexp.MustCompile(`(?i)\b(?:cd|disc|disk)\s*0*(\d{1,2})\b`)
	reDiscTrack = regexp.MustCompile(`^0*(\d)[-\.](\d{1,2})\b`)    // "1-03 Song", "2.01 Song"
	reTrackNum  = regexp.MustCompile(`^(?:\D{0,3})?0*(\d{1,3})\b`) // "03 Song", "03. Song", "A1 Song"
	reWords     = regexp.MustCompile(`[^a-z0-9]+`)
)

type audioFile struct {
	f      premiumize.File
	disc   int
	number int
	name   string // the title part, lower case and plain
}

// matchAlbumFiles pairs the catalog's songs with a release's files: by disc
// and track number where the files are numbered, else by title, else in
// order when the counts agree. Songs without a file are left out.
func matchAlbumFiles(tracks []deezer.Track, files []premiumize.File) []soundTrack {
	var audio []audioFile
	for _, f := range files {
		ext := strings.ToLower(path.Ext(f.Path))
		if browserAudio[ext] == 0 || f.Link == "" {
			continue
		}
		base := strings.TrimSuffix(path.Base(f.Path), path.Ext(f.Path))
		a := audioFile{f: f, disc: 1}
		if m := reDiscDir.FindStringSubmatch(path.Dir(f.Path)); m != nil {
			a.disc, _ = strconv.Atoi(m[1])
		}
		rest := base
		if m := reDiscTrack.FindStringSubmatch(base); m != nil {
			a.disc, _ = strconv.Atoi(m[1])
			a.number, _ = strconv.Atoi(m[2])
			rest = base[len(m[0]):]
		} else if m := reTrackNum.FindStringSubmatch(base); m != nil {
			a.number, _ = strconv.Atoi(m[1])
			rest = base[len(m[0]):]
		}
		a.name = plainWords(rest)
		audio = append(audio, a)
	}
	if len(audio) == 0 {
		return nil
	}
	// Some releases come in two formats: keep the more playable one.
	best := 0
	for _, a := range audio {
		best = max(best, browserAudio[strings.ToLower(path.Ext(a.f.Path))])
	}
	kept := audio[:0]
	for _, a := range audio {
		if browserAudio[strings.ToLower(path.Ext(a.f.Path))] == best {
			kept = append(kept, a)
		}
	}
	audio = kept
	sort.SliceStable(audio, func(i, j int) bool { return audio[i].f.Path < audio[j].f.Path })

	used := make([]bool, len(audio))
	var out []soundTrack
	for _, t := range tracks {
		want := plainWords(t.Title)
		pick := -1
		// The title exactly, where the file names say it.
		for i, a := range audio {
			if !used[i] && want != "" && a.name == want {
				pick = i
				break
			}
		}
		// Else the disc and track number.
		if pick < 0 {
			for i, a := range audio {
				if !used[i] && a.number > 0 && a.number == t.Number && a.disc == max(t.Disc, 1) {
					pick = i
					break
				}
			}
		}
		// Else a name that starts with the title ("Song (Remastered)").
		if pick < 0 && len(want) >= 4 {
			for i, a := range audio {
				if !used[i] && strings.HasPrefix(a.name, want) {
					pick = i
					break
				}
			}
		}
		if pick >= 0 {
			used[pick] = true
			out = append(out, soundTrack{ID: t.ID, URL: audio[pick].f.Link})
		}
	}
	// Nothing matched by name or number but the counts agree: in order.
	if len(out) == 0 && len(audio) == len(tracks) {
		for i, t := range tracks {
			out = append(out, soundTrack{ID: t.ID, URL: audio[i].f.Link})
		}
	}
	return out
}

// namesAlbum says a release's name has every word of the artist's name and
// of the album's title in it (in any order), isn't a discography, and isn't
// dated before the album.
func namesAlbum(title string, rel music.Release, artist, album string, year int) bool {
	if rel.Discography || (rel.Year > 0 && year > 0 && rel.Year < year) {
		return false
	}
	if rel.Artist != "" && !music.SameArtist(rel.Artist, artist) {
		return false // the name says plainly whose it is, and it's someone else's
	}
	have := map[string]bool{}
	for _, w := range strings.Fields(plainWords(strings.ReplaceAll(title, "&", " and "))) {
		have[w] = true
	}
	need := strings.Fields(plainWords(strings.ReplaceAll(artist+" "+album, "&", " and ")))
	if len(need) == 0 {
		return false
	}
	for _, w := range need {
		if !have[w] && !(w == "the" || w == "a") {
			return false
		}
	}
	return true
}

// nothingFound says plainly which step came up empty.
func nothingFound(artist, album string, sites int, failed []string, found, torrents int) string {
	what := fmt.Sprintf("%s by %s", album, artist)
	switch {
	case len(failed) >= sites:
		return fmt.Sprintf("None of your torrent sites answered (%s). Press Test next to each one in Settings > Torrent sites.", strings.Join(failed, ", "))
	case found == 0:
		msg := fmt.Sprintf("Your torrent sites found nothing at all for %s.", what)
		if len(failed) > 0 {
			msg += fmt.Sprintf(" (%s didn't answer.)", strings.Join(failed, ", "))
		}
		return msg + " A site with more music may have it."
	case torrents == 0:
		return fmt.Sprintf("Your sites found %s for %s, but none are torrents, and only torrents can play through Premiumize straight away.", plural(found, "release"), what)
	default:
		return fmt.Sprintf("Your sites found %s, but none of them is %s (other albums, other artists, or whole discographies).", plural(torrents, "torrent"), what)
	}
}

func plainWords(s string) string {
	return strings.Trim(reWords.ReplaceAllString(strings.ToLower(s), " "), " ")
}
