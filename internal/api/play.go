package api

import (
	"bytes"
	"context"
	"encoding/base32"
	"encoding/hex"
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

	"github.com/anacrolix/torrent/metainfo"

	"github.com/rdborg/mediarium/internal/indexers"
	"github.com/rdborg/mediarium/internal/library"
	"github.com/rdborg/mediarium/internal/parser"
	"github.com/rdborg/mediarium/internal/plainerror"
	"github.com/rdborg/mediarium/internal/premiumize"
	"github.com/rdborg/mediarium/internal/quality"
)

// One-click play.
//
// Press play on a movie or an episode and Cue picks what to stream, the
// way it picks what to download: it searches every indexer, ranks the
// releases with the title's quality profile, asks Premiumize which of them it
// already has (one call for all of them), and answers with a link to the best
// one Premiumize can serve at once. Nothing is downloaded to this machine: the
// player streams the file straight from Premiumize.
//
// The ranked list of cached releases is kept for a while, so pressing play
// again, or asking for the next option when one doesn't play, is instant.

const (
	// playCacheFor is how long a title's list of streamable releases is kept.
	playCacheFor = 15 * time.Minute
	// playMaxCandidates is how many of the best-ranked releases are checked
	// against Premiumize's cache.
	playMaxCandidates = 40
	// playMaxLookups is how many releases without an info hash in the search
	// results have their .torrent fetched to find it.
	playMaxLookups = 10
)

// playCandidate is one release Premiumize can stream.
type playCandidate struct {
	Release string
	Hash    string // info hash, lower-case hex; "" for a link from an add-on
	URL     string // a link a stream add-on gave, played as it is
	Source  string // the add-on it came from; "" for Cue's own search
	Size    int64
	Tier    quality.Tier
	// NotEnglish: its name or the add-on says it's in other languages only.
	NotEnglish bool
	// YouTube is a YouTube video's id, played in YouTube's own player
	// (play_fallback.go); URL is then "".
	YouTube string
	// Direct: URL plays as it is, without opening it first (the Internet
	// Archive's files).
	Direct bool
}

type playEntry struct {
	at         time.Time
	candidates []playCandidate
	// nothing says, in plain words, why there is nothing to play (no
	// indexers, nothing found, no torrents, nothing cached); set only when
	// candidates is empty.
	nothing string
}

// playLists keeps the streamable releases per title ("movie:12", "ep:4:1:2").
type playLists struct {
	mu      sync.Mutex
	entries map[string]playEntry
	now     func() time.Time
}

func (p *playLists) get(key string) (playEntry, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	e, ok := p.entries[key]
	if !ok || p.clock().Sub(e.at) > playCacheFor {
		return playEntry{}, false
	}
	return e, true
}

func (p *playLists) put(key string, e playEntry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.entries == nil {
		p.entries = map[string]playEntry{}
	}
	now := p.clock()
	for k, old := range p.entries { // drop stale lists so the map stays small
		if now.Sub(old.at) > playCacheFor {
			delete(p.entries, k)
		}
	}
	e.at = now
	p.entries[key] = e
}

// clear forgets every list (the stream add-ons changed).
func (p *playLists) clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.entries = nil
}

func (p *playLists) clock() time.Time {
	if p.now != nil {
		return p.now()
	}
	return time.Now()
}

var playCache playLists

// playLookups runs one lookup per title at a time: Play pressed while the
// title page's look-ahead is still searching waits for that search.
var playLookups lookupGroup

type lookupCall struct {
	done  chan struct{}
	entry playEntry
	err   error
}

type lookupGroup struct {
	mu    sync.Mutex
	calls map[string]*lookupCall
}

func (g *lookupGroup) do(key string, f func() (playEntry, error)) (playEntry, error) {
	g.mu.Lock()
	if g.calls == nil {
		g.calls = map[string]*lookupCall{}
	}
	if c, ok := g.calls[key]; ok {
		g.mu.Unlock()
		<-c.done
		return c.entry, c.err
	}
	c := &lookupCall{done: make(chan struct{})}
	g.calls[key] = c
	g.mu.Unlock()
	c.entry, c.err = f()
	close(c.done)
	g.mu.Lock()
	delete(g.calls, key)
	g.mu.Unlock()
	return c.entry, c.err
}

// playTarget is what to play: a library movie, or one episode of a show.
type playTarget struct {
	key     string
	label   string
	movie   *library.Movie
	series  *library.Series
	season  int
	episode int
}

// playAnswer is what the player gets.
type playAnswer struct {
	Title string `json:"title"`
	// URL is the original file: for players that open any format (VLC, mpv,
	// ExoPlayer on Fire TV and Google TV).
	URL string `json:"url"`
	// StreamURL is Premiumize's converted MP4 copy, when it has one: for web
	// browsers and Apple TV, which can't open MKV.
	StreamURL string `json:"streamUrl,omitempty"`
	FileName  string `json:"fileName"`
	SizeBytes int64  `json:"sizeBytes"`
	Release   string `json:"release"`
	Quality   string `json:"quality"`
	// Option is which of the streamable releases this is (1 = the best);
	// Options is how many there are. Ask for option+1 if this one won't play.
	Option  int `json:"option"`
	Options int `json:"options"`
	// Source is the stream add-on this came from (like "Comet"); empty for
	// Cue's own search.
	Source string `json:"source,omitempty"`
	// YouTube is set instead of URL for a YouTube video: its id, for
	// YouTube's embedded player.
	YouTube string `json:"youtube,omitempty"`
}

// GET /api/play/movies/{id}[?option=N][&fresh=1]
func (s *Server) handlePlayMovie(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "That isn't a valid movie ID.")
		return
	}
	m, err := s.MovieRepo.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "That movie isn't in your library.")
		return
	}
	s.servePlay(w, r, movieTarget(m))
}

// movieTarget is what to play for a movie. The key is TMDB's id, so a title
// played from Watch and from the library shares one list.
func movieTarget(m library.Movie) playTarget {
	label := m.Title
	if m.Year > 0 {
		label = fmt.Sprintf("%s (%d)", m.Title, m.Year)
	}
	return playTarget{key: fmt.Sprintf("movie:%d", m.TMDBID), label: label, movie: &m}
}

func episodeTarget(series library.Series, season, episode int) playTarget {
	return playTarget{
		key:    fmt.Sprintf("ep:%d:%d:%d", series.TMDBID, season, episode),
		label:  fmt.Sprintf("%s S%02dE%02d", series.Title, season, episode),
		series: &series, season: season, episode: episode,
	}
}

// GET /api/play/tmdb/movie/{tmdbId}: any movie, in the library or not. One
// in the library is searched with its own quality profile; any other with
// the default one.
func (s *Server) handlePlayTMDBMovie(w http.ResponseWriter, r *http.Request) {
	tmdbID, err := strconv.Atoi(r.PathValue("tmdbId"))
	if err != nil || tmdbID <= 0 {
		writeError(w, http.StatusBadRequest, "That isn't a valid TMDB id.")
		return
	}
	if m, ok, err := s.MovieRepo.GetByTMDBID(tmdbID); err == nil && ok {
		s.servePlay(w, r, movieTarget(m))
		return
	}
	d, err := s.TMDB().GetMovieDetail(r.Context(), tmdbID)
	if err != nil {
		writeUpstreamError(w, "look the movie up on TMDB", err)
		return
	}
	s.servePlay(w, r, movieTarget(library.Movie{TMDBID: tmdbID, Title: d.Title, Year: d.Year()}))
}

// GET /api/play/tmdb/tv/{tmdbId}/{season}/{episode}: any episode of any show.
func (s *Server) handlePlayTMDBEpisode(w http.ResponseWriter, r *http.Request) {
	tmdbID, err1 := strconv.Atoi(r.PathValue("tmdbId"))
	season, err2 := strconv.Atoi(r.PathValue("season"))
	episode, err3 := strconv.Atoi(r.PathValue("episode"))
	if err1 != nil || err2 != nil || err3 != nil || tmdbID <= 0 || season < 0 || episode < 1 {
		writeError(w, http.StatusBadRequest, "That isn't a valid episode.")
		return
	}
	if series, ok, err := s.MovieRepo.GetSeriesByTMDBID(tmdbID); err == nil && ok {
		s.servePlay(w, r, episodeTarget(series, season, episode))
		return
	}
	d, err := s.TMDB().GetShowFull(r.Context(), tmdbID)
	if err != nil {
		writeUpstreamError(w, "look the show up on TMDB", err)
		return
	}
	s.servePlay(w, r, episodeTarget(library.Series{TMDBID: tmdbID, Title: d.Name, Year: d.Year()}, season, episode))
}

// GET /api/play/series/{id}/{season}/{episode}[?option=N][&fresh=1]
func (s *Server) handlePlayEpisode(w http.ResponseWriter, r *http.Request) {
	id, err1 := strconv.ParseInt(r.PathValue("id"), 10, 64)
	season, err2 := strconv.Atoi(r.PathValue("season"))
	episode, err3 := strconv.Atoi(r.PathValue("episode"))
	if err1 != nil || err2 != nil || err3 != nil || season < 0 || episode < 1 {
		writeError(w, http.StatusBadRequest, "That isn't a valid episode.")
		return
	}
	series, err := s.MovieRepo.GetSeries(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "That show isn't in your library.")
		return
	}
	s.servePlay(w, r, episodeTarget(series, season, episode))
}

func (s *Server) servePlay(w http.ResponseWriter, r *http.Request, t playTarget) {
	pm := s.premiumizeClient()
	if pm == nil && len(s.savedAddons()) == 0 {
		writeError(w, http.StatusConflict, "Playing needs Premiumize. Add your API key in Settings > Downloading > Usenet and torrents, under Cloud downloader.")
		return
	}
	option := 1
	if v := r.URL.Query().Get("option"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "option must be 1 or more.")
			return
		}
		option = n
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	entry, ok := playCache.get(t.key)
	if !ok || r.URL.Query().Get("fresh") == "1" {
		var err error
		entry, err = playLookups.do(t.key, func() (playEntry, error) { return s.findStreamable(ctx, pm, t) })
		if errors.Is(err, premiumize.ErrBadKey) {
			writeError(w, http.StatusBadGateway, premiumizeKeyError().Error())
			return
		}
		if err != nil {
			slog.Warn("play: find streams", "title", t.label, "err", err)
			writeError(w, http.StatusBadGateway, "Couldn't look for something to play right now. Try again in a moment.")
			return
		}
		if len(entry.candidates) > 0 {
			// "Nothing yet" isn't kept: after adding an indexer, or once
			// Premiumize has it, pressing Play again looks again.
			playCache.put(t.key, entry)
		}
	}
	if r.URL.Query().Get("prefetch") == "1" {
		// Only looking ahead (a title page opened, the next episode is
		// near): the list is ready for when Play is pressed.
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if len(entry.candidates) == 0 {
		// Nothing from the add-ons or the torrent sites: the Internet
		// Archive and YouTube, for old and little-known shows.
		if cands := s.fallbackCandidates(ctx, t); len(cands) > 0 {
			entry.candidates = cands
			playCache.put(t.key, entry)
		} else {
			entry.nothing = strings.TrimSpace(entry.nothing + " " + s.fallbackTried())
		}
	}
	if len(entry.candidates) == 0 {
		writeError(w, http.StatusNotFound, entry.nothing)
		return
	}
	// The playback quality setting (a data saver) puts bigger versions last.
	entry.candidates = preferResolution(entry.candidates, s.playbackMaxRes())
	if s.englishOnly() {
		var other bool
		if entry.candidates, other = keepEnglish(entry.candidates); other {
			writeError(w, http.StatusNotFound, fmt.Sprintf("Only versions of %s in other languages were found. Try again later, or allow every language in Settings > Streaming.", t.label))
			return
		}
	}
	web := r.URL.Query().Get("client") == "web"
	if web {
		// A web browser: versions it can play with sound first.
		entry.candidates = browserAudioFirst(entry.candidates, pm != nil)
		entry.candidates = browserFilesFirst(entry.candidates)
	}
	if option > len(entry.candidates) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("There are only %d versions of %s to play.", len(entry.candidates), t.label))
		return
	}

	// Premiumize's cache can drop a release between the check and now, and a
	// pack may not hold the episode wanted: then the next option is used.
	for i := option - 1; i < len(entry.candidates); i++ {
		c := entry.candidates[i]
		if c.YouTube != "" || c.Direct {
			writeJSON(w, http.StatusOK, playAnswer{
				Title: t.label, URL: c.URL, YouTube: c.YouTube, FileName: c.Release, SizeBytes: c.Size,
				Release: c.Release, Quality: string(c.Tier), Source: c.Source,
				Option: i + 1, Options: len(entry.candidates),
			})
			return
		}
		if c.URL != "" && web && pm != nil && c.Hash != "" {
			// A browser can't play most add-on files' sound (Dolby, DTS).
			// Premiumize's converted copy of the same torrent plays
			// everywhere, so it goes first, with the file itself behind it.
			if files, err := pm.DirectDL(ctx, "magnet:?xt=urn:btih:"+c.Hash); err == nil {
				if f, ok := pickPlayFile(files, t, s.releaseMapperFor(t)); ok && f.StreamLink != "" {
					writeJSON(w, http.StatusOK, playAnswer{
						Title: t.label, URL: f.Link, StreamURL: f.StreamLink,
						FileName: path.Base(f.Path), SizeBytes: f.Size,
						Release: c.Release, Quality: string(c.Tier), Source: c.Source,
						Option: i + 1, Options: len(entry.candidates),
					})
					return
				}
			}
		}
		if c.URL != "" {
			// A link from an add-on, already resolved with its debrid service.
			// Opened here first, so the device gets the file's own address.
			writeJSON(w, http.StatusOK, playAnswer{
				Title: t.label, URL: resolveAddonLink(ctx, c.URL), FileName: c.Release, SizeBytes: c.Size,
				Release: c.Release, Quality: string(c.Tier), Source: c.Source,
				Option: i + 1, Options: len(entry.candidates),
			})
			return
		}
		if pm == nil {
			continue
		}
		files, err := pm.DirectDL(ctx, "magnet:?xt=urn:btih:"+c.Hash)
		if errors.Is(err, premiumize.ErrBadKey) {
			writeError(w, http.StatusBadGateway, premiumizeKeyError().Error())
			return
		}
		if err != nil {
			continue
		}
		f, ok := pickPlayFile(files, t, s.releaseMapperFor(t))
		if !ok {
			continue
		}
		writeJSON(w, http.StatusOK, playAnswer{
			Title: t.label, URL: f.Link, StreamURL: f.StreamLink,
			FileName: path.Base(f.Path), SizeBytes: f.Size,
			Release: c.Release, Quality: string(c.Tier), Source: c.Source,
			Option: i + 1, Options: len(entry.candidates),
		})
		return
	}
	writeError(w, http.StatusNotFound, fmt.Sprintf("None of the remaining versions of %s could be played. Try again with fresh=1 to search again.", t.label))
}

func (s *Server) releaseMapperFor(t playTarget) func(parser.Release) parser.Release {
	if t.series == nil {
		return nil
	}
	return s.releaseMapper(*t.series)
}

// findStreamable searches, ranks and checks Premiumize's cache for t.
func (s *Server) findStreamable(ctx context.Context, pm *premiumize.Client, t playTarget) (playEntry, error) {
	started := time.Now()
	// The stream add-ons and the indexers are asked at the same time. Links
	// from an add-on play as they are, in its order, without waiting for the
	// indexers (they finish in the background).
	type addonAnswer struct {
		links, hashes []playCandidate
		asked         int
		failed        []string
	}
	addonCh := make(chan addonAnswer, 1)
	go func() {
		var a addonAnswer
		a.links, a.hashes, a.asked, a.failed = s.addonStreams(ctx, t)
		addonCh <- a
	}()
	type searchAnswer struct {
		outcomes []indexers.Outcome
		err      error
	}
	searchCh := make(chan searchAnswer, 1)
	if pm != nil {
		go func() {
			out, err := s.playSearch(ctx, t)
			searchCh <- searchAnswer{out, err}
		}()
	}
	addon := <-addonCh
	addonsTook := time.Since(started)
	if len(addon.links) > 0 {
		slog.Info("play: streams from add-ons", "title", t.label, "links", len(addon.links), "took", addonsTook.Round(time.Millisecond))
		return playEntry{candidates: addon.links}, nil
	}
	addonHashes := addon.hashes
	addonNote := addonsNote(t.label, addon.asked, addon.failed, len(addonHashes))
	if pm == nil {
		return playEntry{nothing: strings.TrimSpace(addonNote + " Add your Premiumize key in Settings > Downloading > Usenet and torrents so Cue can search too.")}, nil
	}
	found := <-searchCh
	if found.err != nil {
		return playEntry{}, found.err
	}
	searchTook := time.Since(started)

	instances, err := s.IndexerRepo.List()
	if err != nil {
		return playEntry{}, err
	}
	profiles, err := s.loadProfiles()
	if err != nil {
		return playEntry{}, err
	}
	blocked := s.blockedKeys()

	var st playStats
	for _, inst := range instances {
		if inst.Enabled {
			st.indexers++
			if inst.Protocol == indexers.ProtocolTorrent {
				st.torrentIndexers++
			}
		}
	}
	outcomes := found.outcomes
	for _, o := range outcomes {
		if o.Err != nil {
			st.failed = append(st.failed, o.IndexerName)
			st.reasons = append(st.reasons, indexerFailure(o.IndexerName, o.Err))
		}
	}
	all := dropBlocked(indexers.MergeResults(outcomes), blocked)
	torrents := torrentResults(all)
	st.results, st.torrents = len(all), len(torrents)

	var matching, ranked []indexers.Result
	if t.movie != nil {
		matching = matchingResults(torrents, *t.movie)
		ranked = rankMovieResults(matching, profiles.resolve(t.movie.ProfileID), t.movie.Year)
	} else {
		mapper := s.releaseMapper(*t.series)
		for _, r := range torrents {
			if matchesShow(r.Title, *t.series) && coversTarget(parseFor(mapper, r.Title), t.season, t.episode) {
				matching = append(matching, r)
			}
		}
		ranked = rankTVResults(matching, *t.series, t.season, t.episode, profiles.resolve(t.series.ProfileID), mapper)
	}
	st.matching = len(matching)
	if len(ranked) == 0 {
		// The quality profile turned every release down (too big, a codec it
		// avoids, a language rule). For streaming, any watchable version
		// beats nothing: everything but cinema recordings, best first.
		ranked = rankAnyWatchable(matching, t)
	}
	if len(ranked) > playMaxCandidates {
		ranked = ranked[:playMaxCandidates]
	}

	hashes := s.infoHashes(ctx, ranked)
	var (
		items []string
		cands []playCandidate
		seen  = map[string]bool{}
	)
	// Torrents an add-on found come first, in its order.
	for _, c := range addonHashes {
		if !seen[c.Hash] {
			seen[c.Hash] = true
			items = append(items, c.Hash)
			cands = append(cands, c)
		}
	}
	for i, res := range ranked {
		h := hashes[i]
		if h == "" || seen[h] {
			continue
		}
		seen[h] = true
		items = append(items, h)
		cands = append(cands, playCandidate{Release: res.Title, Hash: h, Size: res.SizeBytes, Tier: quality.Classify(parser.Parse(res.Title)), NotEnglish: notEnglish(res.Title)})
	}
	entry := playEntry{}
	if len(items) == 0 {
		entry.nothing = strings.TrimSpace(addonNote + " " + st.message(t.label, 0))
		return entry, nil
	}
	cached, err := pm.CacheCheck(ctx, items)
	if err != nil {
		return playEntry{}, err
	}
	defer func() {
		slog.Info("play: streams from the indexers", "title", t.label, "releases", len(items),
			"addons", addonsTook.Round(time.Millisecond), "search", searchTook.Round(time.Millisecond), "total", time.Since(started).Round(time.Millisecond))
	}()
	for i, ok := range cached {
		if ok {
			entry.candidates = append(entry.candidates, cands[i])
		}
	}
	if len(entry.candidates) == 0 {
		entry.nothing = strings.TrimSpace(addonNote + " " + st.message(t.label, len(items)))
	}
	return entry, nil
}

// addonsNote says what the stream add-ons found, ahead of the rest of a
// "nothing to play" message; "" without add-ons.
func addonsNote(label string, asked int, failed []string, hashes int) string {
	var parts []string
	if asked > 0 {
		if hashes > 0 {
			parts = append(parts, fmt.Sprintf("Your stream add-ons found %s of %s but no ready-to-play links.", plural(hashes, "torrent"), label))
		} else {
			parts = append(parts, fmt.Sprintf("Your stream add-ons found nothing for %s.", label))
		}
	}
	if len(failed) > 0 {
		parts = append(parts, fmt.Sprintf("%s didn't answer.", strings.Join(failed, ", ")))
	}
	return strings.Join(parts, " ")
}

// playStats counts what one search for something to play found, to say
// plainly why there is nothing to play.
type playStats struct {
	indexers, torrentIndexers int      // enabled ones
	failed                    []string // indexers that didn't answer
	reasons                   []string // and why, one "Name: reason" each
	results                   int      // releases found
	torrents                  int      // of which torrents
	matching                  int      // torrents of this very title
}

const addTorrentSite = "Add a torrent site in Settings > Indexers & Search (the Torrent tab), for example The Pirate Bay, YTS or EZTV."

// message is why there is nothing to play for label; checked is how many
// releases were checked with Premiumize.
func (st playStats) message(label string, checked int) string {
	failed := ""
	if len(st.failed) > 0 {
		failed = fmt.Sprintf(" (%s didn't answer)", strings.Join(st.failed, ", "))
	}
	switch {
	case st.indexers == 0:
		return "No indexers are set up yet, so there's nowhere to look. " + addTorrentSite
	case st.results == 0 && len(st.failed) >= st.indexers:
		return fmt.Sprintf("None of your indexers answered. %s. Press Test next to each one in Settings > Indexers & Search for details.", strings.Join(st.reasons, ". "))
	case st.results == 0 && st.torrentIndexers == 0:
		return fmt.Sprintf("Your indexers found nothing for %s, and they're all Usenet indexers. Playing straight away needs torrents. %s", label, addTorrentSite)
	case st.results == 0:
		return fmt.Sprintf("Your indexers found nothing for %s%s. Try a site with more releases. %s", label, failed, addTorrentSite)
	case st.torrents == 0:
		return fmt.Sprintf("Your indexers found %s for %s, but none are torrents: Usenet releases can't be played straight away. %s", plural(st.results, "release"), label, addTorrentSite)
	case st.matching == 0:
		return fmt.Sprintf("Your indexers found %s, but none were for %s (they were other titles).", plural(st.torrents, "torrent"), label)
	case checked == 0:
		return fmt.Sprintf("Found %s of %s, but couldn't read which torrent each one is.", plural(st.matching, "torrent"), label)
	}
	return fmt.Sprintf("Found %s of %s, but Premiumize doesn't have any of them ready to stream yet.", plural(st.matching, "torrent"), label)
}

// indexerFailure is "Name: what went wrong" in plain words, short.
func indexerFailure(name string, err error) string {
	msg := strings.TrimSpace(plainerror.Message(err))
	msg = strings.TrimPrefix(msg, name+": ")
	msg = strings.TrimRight(msg, ". ")
	if len(msg) > 160 {
		msg = msg[:157] + "..."
	}
	if msg == "" {
		msg = "no answer"
	}
	return name + ": " + msg
}

// rankAnyWatchable orders releases with no quality profile: best quality
// first, then most seeders, leaving out cinema recordings and, for a movie,
// releases of another year.
func rankAnyWatchable(results []indexers.Result, t playTarget) []indexers.Result {
	type scored struct {
		r    indexers.Result
		rank int
	}
	var list []scored
	for _, r := range results {
		rel := parser.Parse(r.Title)
		tier := quality.Classify(rel)
		if tier == quality.TierPreRelease {
			continue
		}
		if t.movie != nil && t.movie.Year != 0 && rel.Year != 0 && rel.Year != t.movie.Year {
			continue
		}
		list = append(list, scored{r, quality.Rank(tier)})
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].rank != list[j].rank {
			return list[i].rank > list[j].rank
		}
		return list[i].r.Seeders > list[j].r.Seeders
	})
	out := make([]indexers.Result, len(list))
	for i, x := range list {
		out[i] = x.r
	}
	return out
}

// torrentResults keeps the torrent releases: only those can be streamed
// before they are downloaded.
func torrentResults(results []indexers.Result) []indexers.Result {
	out := results[:0:0]
	for _, r := range results {
		if r.Protocol == indexers.ProtocolTorrent || r.Torrent {
			out = append(out, r)
		}
	}
	return out
}

// rankMovieResults orders the releases a movie's profile accepts best first,
// then those its fallback profiles accept (the order pickBestResult would
// choose them in).
func rankMovieResults(results []indexers.Result, profile quality.Profile, wantYear int) []indexers.Result {
	return rankChain(profile, func(p quality.Profile) []scoredResult {
		var out []scoredResult
		for _, res := range results {
			rel := parser.Parse(res.Title)
			if !p.Accepts(rel) {
				continue
			}
			if ok, _ := p.TitleAllowed(res.Title); !ok {
				continue
			}
			if wantYear != 0 && rel.Year != 0 && rel.Year != wantYear {
				continue
			}
			if !quality.SizePlausible(quality.Classify(rel), res.SizeBytes, 0) || !p.SizeAllowed(res.SizeBytes) {
				continue
			}
			out = append(out, scoredResult{res: res, key: pickKey{p.LanguageRank(rel), quality.Rank(quality.Classify(rel)), p.Score(res.Title)}})
		}
		return out
	})
}

// rankTVResults is rankMovieResults for one episode: releases of that
// episode and season packs holding it. Single-episode releases come before
// packs of the same quality, as they start faster and are smaller.
func rankTVResults(results []indexers.Result, series library.Series, season, episode int, profile quality.Profile, mapper func(parser.Release) parser.Release) []indexers.Result {
	return rankChain(profile, func(p quality.Profile) []scoredResult {
		var out []scoredResult
		for _, res := range results {
			rel := parseFor(mapper, res.Title)
			if ok, _ := p.TitleAllowed(res.Title); !ok {
				continue
			}
			if !matchesShow(res.Title, series) || !coversTarget(rel, season, episode) || !p.Accepts(rel) {
				continue
			}
			eps := len(rel.Episodes)
			if eps == 0 {
				eps = quality.SeasonPack
			}
			if !quality.SizePlausible(quality.Classify(rel), res.SizeBytes, eps) || !p.SizeAllowed(res.SizeBytes) {
				continue
			}
			key := pickKey{p.LanguageRank(rel), quality.Rank(quality.Classify(rel)), p.Score(res.Title)}
			out = append(out, scoredResult{res: res, key: key, pack: len(rel.Episodes) == 0})
		}
		return out
	})
}

type scoredResult struct {
	res  indexers.Result
	key  pickKey
	pack bool
}

// rankChain ranks what each profile of the chain accepts, best first, the
// profile's own releases before its fallbacks', each release once.
func rankChain(profile quality.Profile, accepted func(quality.Profile) []scoredResult) []indexers.Result {
	var out []indexers.Result
	seen := map[string]bool{}
	for _, p := range profile.Chain() {
		list := accepted(p)
		sort.SliceStable(list, func(i, j int) bool {
			a, b := list[i], list[j]
			if a.key != b.key {
				return a.key.above(b.key)
			}
			if a.pack != b.pack {
				return !a.pack
			}
			return a.res.Seeders > b.res.Seeders
		})
		for _, sr := range list {
			id := sr.res.DownloadURL
			if seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, sr.res)
		}
	}
	return out
}

var btihRe = regexp.MustCompile(`(?i)xt=urn:btih:([a-z0-9]+)`)

// normalizeHash turns a hex or base32 info hash into lower-case hex, or "".
func normalizeHash(h string) string {
	h = strings.TrimSpace(h)
	switch len(h) {
	case 40:
		if _, err := hex.DecodeString(h); err == nil {
			return strings.ToLower(h)
		}
	case 32:
		if b, err := base32.StdEncoding.DecodeString(strings.ToUpper(h)); err == nil {
			return hex.EncodeToString(b)
		}
	}
	return ""
}

// infoHashes finds each release's info hash: from the search result, from
// its magnet link or, for the best few with neither, from its .torrent file.
func (s *Server) infoHashes(ctx context.Context, results []indexers.Result) []string {
	out := make([]string, len(results))
	var (
		wg      sync.WaitGroup
		lookups int
	)
	for i, res := range results {
		if h := normalizeHash(res.InfoHash); h != "" {
			out[i] = h
			continue
		}
		if m := btihRe.FindStringSubmatch(res.DownloadURL); m != nil {
			out[i] = normalizeHash(m[1])
			continue
		}
		if lookups >= playMaxLookups {
			continue
		}
		lookups++
		wg.Add(1)
		go func(i int, link string) {
			defer wg.Done()
			out[i] = s.hashFromLink(ctx, link)
		}(i, res.DownloadURL)
	}
	wg.Wait()
	return out
}

// hashFromLink fetches a release's .torrent (or magnet, from a
// definition-based indexer) and returns its info hash, or "".
func (s *Server) hashFromLink(ctx context.Context, link string) string {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	var data []byte
	if indexers.IsLinkRef(link) {
		dl, err := s.resolveIndexerLink(ctx, link)
		if err != nil {
			return ""
		}
		if dl.Magnet != "" {
			if m := btihRe.FindStringSubmatch(dl.Magnet); m != nil {
				return normalizeHash(m[1])
			}
			return ""
		}
		data = dl.Data
	} else {
		b, err := fetchURL(ctx, link)
		if err != nil {
			return ""
		}
		data = b
	}
	mi, err := metainfo.Load(bytes.NewReader(data))
	if err != nil {
		return ""
	}
	return mi.HashInfoBytes().HexString()
}

var videoExts = map[string]bool{".mkv": true, ".mp4": true, ".m4v": true, ".avi": true, ".mov": true, ".ts": true, ".m2ts": true, ".webm": true, ".wmv": true, ".mpg": true, ".mpeg": true}

// pickPlayFile chooses the file to play from a release: the movie's (the
// largest video), or the wanted episode's from a pack.
func pickPlayFile(files []premiumize.File, t playTarget, mapper func(parser.Release) parser.Release) (premiumize.File, bool) {
	var videos []premiumize.File
	for _, f := range files {
		name := strings.ToLower(path.Base(f.Path))
		if !videoExts[path.Ext(name)] || strings.Contains(name, "sample") || strings.Contains(name, "trailer") {
			continue
		}
		videos = append(videos, f)
	}
	sort.SliceStable(videos, func(i, j int) bool { return videos[i].Size > videos[j].Size })
	if len(videos) == 0 {
		return premiumize.File{}, false
	}
	if t.movie != nil {
		return videos[0], true
	}
	for _, f := range videos {
		rel := parseFor(mapper, path.Base(f.Path))
		if rel.Season != t.season && rel.Season != 0 {
			continue
		}
		for _, e := range rel.Episodes {
			if e == t.episode {
				return f, true
			}
		}
	}
	// A single-episode release whose file name doesn't say which episode.
	if len(videos) == 1 {
		return videos[0], true
	}
	return premiumize.File{}, false
}
