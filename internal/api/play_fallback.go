package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rdborg/mediarium/internal/archiveorg"
	"github.com/rdborg/mediarium/internal/settings"
	"github.com/rdborg/mediarium/internal/youtube"
)

// When the stream add-ons and the torrent sites have nothing (old children's
// TV, say), Play looks in two more places: the Internet Archive, whose files
// play in Cue's own player, and YouTube (with the owner's API key), whose
// videos play in YouTube's own embedded player. Nothing is downloaded.

const (
	// fallbackCacheFor is how long what was found is kept: a YouTube search
	// uses 100 of the key's 10,000 daily units.
	fallbackCacheFor = 12 * time.Hour
	// fallbackNothingFor is how long "nothing there either" is kept.
	fallbackNothingFor = 2 * time.Hour
	fallbackMax        = 4 // per source

	sourceArchive = "Internet Archive"
	sourceYouTube = "YouTube"
)

type fallbackEntry struct {
	at    time.Time
	cands []playCandidate
}

type fallbackState struct {
	mu      sync.Mutex
	entries map[string]fallbackEntry
	// Tests point these elsewhere; "" = the real ones.
	archiveBase string
	youtubeBase string
}

func (f *fallbackState) get(key string) ([]playCandidate, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.entries[key]
	if !ok {
		return nil, false
	}
	keep := fallbackCacheFor
	if len(e.cands) == 0 {
		keep = fallbackNothingFor
	}
	if time.Since(e.at) > keep {
		delete(f.entries, key)
		return nil, false
	}
	return e.cands, true
}

func (f *fallbackState) put(key string, cands []playCandidate) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.entries == nil {
		f.entries = map[string]fallbackEntry{}
	}
	for k, old := range f.entries {
		if time.Since(old.at) > fallbackCacheFor {
			delete(f.entries, k)
		}
	}
	f.entries[key] = fallbackEntry{at: time.Now(), cands: cands}
}

// youtubeKey is the saved YouTube Data API key, or "".
func (s *Server) youtubeKey() string {
	key, err := s.Settings.Get(settings.KeyYouTubeAPIKey)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(key)
}

// fallbackLookups runs one look per title at a time.
var fallbackLookups lookupGroup

// fallbackCandidates is what the Internet Archive and YouTube have for t:
// the Internet Archive's first (it plays in Cue's own player), then YouTube's.
func (s *Server) fallbackCandidates(ctx context.Context, t playTarget) []playCandidate {
	if cands, ok := s.fallback.get(t.key); ok {
		return cands
	}
	entry, _ := fallbackLookups.do("fallback:"+t.key, func() (playEntry, error) {
		want := s.fallbackWant(ctx, t)
		var arch, yt []playCandidate
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			c, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			arch = archiveCandidates(c, archiveorg.New(s.fallback.archiveBase), want)
		}()
		go func() {
			defer wg.Done()
			key := s.youtubeKey()
			if key == "" {
				return
			}
			c, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()
			yt = youtubeCandidates(c, youtube.New(key, s.fallback.youtubeBase), want)
		}()
		wg.Wait()
		cands := append(arch, yt...)
		if ctx.Err() == nil { // a look cut short isn't kept
			s.fallback.put(t.key, cands)
		}
		slog.Info("play: other places", "title", t.label, "archive", len(arch), "youtube", len(yt))
		return playEntry{candidates: cands}, nil
	})
	return entry.candidates
}

// fallbackWant is what is being looked for, in words.
type fallbackWant struct {
	title   string // the film or the show
	year    int
	episode bool
	season  int
	number  int
	name    string // the episode's name, when TMDB has it
}

func (s *Server) fallbackWant(ctx context.Context, t playTarget) fallbackWant {
	if t.movie != nil {
		return fallbackWant{title: t.movie.Title, year: t.movie.Year}
	}
	w := fallbackWant{episode: true, season: t.season, number: t.episode}
	if t.series != nil {
		w.title, w.year = t.series.Title, t.series.Year
		if tm := s.TMDB(); tm != nil && tm.HasAPIKey() && t.series.TMDBID > 0 {
			c, cancel := context.WithTimeout(ctx, 8*time.Second)
			defer cancel()
			if eps, err := tm.GetSeasonCached(c, t.series.TMDBID, t.season); err == nil {
				for _, e := range eps {
					if e.Episode == t.episode {
						w.name = e.Name
					}
				}
			}
		}
	}
	// TMDB's stand-in names ("Episode 4") say nothing.
	if regexp.MustCompile(`(?i)^(episode|chapter|part)\s*\d+$`).MatchString(strings.TrimSpace(w.name)) {
		w.name = ""
	}
	return w
}

// words is s in lower case, letters and digits only, one space apart.
func words(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "&", " and ")
	s = strings.NewReplacer("'", "", "’", "", "`", "").Replace(s)
	var b strings.Builder
	space := true
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			space = false
		} else if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}

var smallWords = map[string]bool{"the": true, "a": true, "an": true, "and": true, "of": true}

// hasTitle says whether text names title (every word that matters, in any
// order).
func hasTitle(text, title string) bool {
	have := map[string]bool{}
	for _, w := range strings.Fields(words(text)) {
		have[w] = true
	}
	n := 0
	for _, w := range strings.Fields(words(title)) {
		if smallWords[w] {
			continue
		}
		if !have[w] {
			return false
		}
		n++
	}
	return n > 0 || strings.Contains(" "+words(text)+" ", " "+words(title)+" ")
}

var (
	epSxE      = regexp.MustCompile(`(?i)\bs(\d{1,2})[ ._-]*e(\d{1,3})\b`)
	epNxN      = regexp.MustCompile(`\b(\d{1,2})x(\d{1,3})\b`)
	epWords    = regexp.MustCompile(`(?i)\bseason\W*(\d{1,2})\W+(?:ep|episode)\W*(\d{1,3})\b`)
	seasonWord = regexp.MustCompile(`(?i)\b(?:season|series|s)\W*(\d{1,2})\b`)
	epOnly     = regexp.MustCompile(`(?i)(?:^|\b(?:ep|episode|e)\W*)(\d{1,3})\b`)
	notWhole   = regexp.MustCompile(`(?i)\b(trailer|teaser|clip|clips|scene|promo|preview|reaction|review|intro|theme song|opening|credits|shorts|compilation|behind the scenes|unboxing|toy|toys|lyrics|karaoke|cover)\b`)
	yearIn     = regexp.MustCompile(`\b(19\d\d|20\d\d)\b`)
)

// isEpisode says whether text (an upload's and file's names together) is
// episode w.number of season w.season.
func isEpisode(text, file string, w fallbackWant) bool {
	if w.name != "" && len(words(w.name)) >= 5 && strings.Contains(" "+words(text+" "+file)+" ", " "+words(w.name)+" ") {
		return true
	}
	for _, re := range []*regexp.Regexp{epSxE, epNxN, epWords} {
		if m := re.FindAllStringSubmatch(text+" "+file, -1); m != nil {
			for _, g := range m {
				s, _ := strconv.Atoi(g[1])
				e, _ := strconv.Atoi(g[2])
				if s == w.season && e == w.number {
					return true
				}
			}
			return false // numbered, but another episode
		}
	}
	// "Fraggle Rock Season 1" holding "05 - Name.mp4" or "Episode 5.mp4".
	if m := seasonWord.FindStringSubmatch(text); m != nil {
		if s, _ := strconv.Atoi(m[1]); s != w.season {
			return false
		}
		base := file[strings.LastIndex(file, "/")+1:]
		if m := epOnly.FindStringSubmatch(base); m != nil {
			e, _ := strconv.Atoi(m[1])
			return e == w.number
		}
	}
	return false
}

// otherYear says whether text names a year far from the film's.
func otherYear(text string, year int) bool {
	if year == 0 {
		return false
	}
	ys := yearIn.FindAllString(text, -1)
	if len(ys) == 0 {
		return false
	}
	for _, y := range ys {
		n, _ := strconv.Atoi(y)
		if n >= year-1 && n <= year+1 {
			return false
		}
	}
	return true
}

func archiveCandidates(ctx context.Context, c *archiveorg.Client, w fallbackWant) []playCandidate {
	if w.title == "" {
		return nil
	}
	items, err := c.Search(ctx, w.title, 30)
	if err != nil {
		slog.Info("play: Internet Archive search", "title", w.title, "err", err)
		return nil
	}
	var keep []archiveorg.Item
	for _, it := range items {
		if !hasTitle(it.Title, w.title) || !w.episode && otherYear(it.Title, w.year) {
			continue
		}
		// An upload numbered as another episode isn't opened.
		if w.episode {
			if m := epSxE.FindStringSubmatch(it.Title); m != nil && !isEpisode(it.Title, "", w) {
				continue
			}
		}
		keep = append(keep, it)
		if len(keep) == 8 {
			break
		}
	}
	lists := make([][]archiveorg.Video, len(keep))
	var wg sync.WaitGroup
	for i, it := range keep {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lists[i], _ = c.Videos(ctx, it)
		}()
	}
	wg.Wait()

	type pick struct {
		v     archiveorg.Video
		score int
	}
	var picks []pick
	for _, vids := range lists {
		var best *pick
		for _, v := range vids {
			if w.episode {
				if !isEpisode(v.Title, v.Name, w) || v.Length > 0 && v.Length < 3*time.Minute {
					continue
				}
			} else if v.Length > 0 && v.Length < 40*time.Minute || v.Length == 0 && v.Size > 0 && v.Size < 150<<20 {
				continue
			}
			p := pick{v: v}
			// MP4 plays in every browser; archive.org's own MP4 copy is
			// made for that.
			if strings.HasSuffix(strings.ToLower(v.Name), ".mp4") {
				p.score += 2
			}
			if v.Original {
				p.score++
			}
			if best == nil || p.score > best.score || p.score == best.score && v.Size > best.v.Size {
				best = &p
			}
		}
		if best != nil {
			picks = append(picks, *best)
		}
	}
	slices.SortStableFunc(picks, func(a, b pick) int { return b.score - a.score })
	var out []playCandidate
	for _, p := range picks {
		if len(out) == fallbackMax {
			break
		}
		name := p.v.Name[strings.LastIndex(p.v.Name, "/")+1:]
		out = append(out, playCandidate{Release: p.v.Title + " · " + name, URL: p.v.URL, Source: sourceArchive, Size: p.v.Size, Direct: true})
	}
	return out
}

func youtubeCandidates(ctx context.Context, c *youtube.Client, w fallbackWant) []playCandidate {
	if w.title == "" {
		return nil
	}
	q := w.title + " full movie"
	if w.year > 0 {
		q = fmt.Sprintf("%s %d full movie", w.title, w.year)
	}
	if w.episode {
		q = fmt.Sprintf("%s season %d episode %d", w.title, w.season, w.number)
		if w.name != "" {
			q = w.title + " " + w.name
		}
	}
	vids, err := c.Search(ctx, q, 15)
	if errors.Is(err, youtube.ErrBadKey) {
		slog.Warn("play: YouTube didn't accept the saved API key")
		return nil
	}
	if err != nil {
		slog.Info("play: YouTube search", "title", w.title, "err", err)
		return nil
	}
	var out []playCandidate
	for _, v := range vids {
		if len(out) == fallbackMax {
			break
		}
		if !v.Embeddable || !hasTitle(v.Title+" "+v.Channel, w.title) || notWhole.MatchString(v.Title) {
			continue
		}
		if w.episode {
			if v.Length < 5*time.Minute || v.Length > 3*time.Hour || !isEpisode(v.Title, "", w) {
				continue
			}
		} else if v.Length < 45*time.Minute || otherYear(v.Title, w.year) {
			continue
		}
		out = append(out, playCandidate{Release: v.Title, YouTube: v.ID, Source: sourceYouTube})
	}
	return out
}

// GET /api/settings/youtube: whether a YouTube key is saved.
func (s *Server) handleGetYouTube(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"set": s.youtubeKey() != ""})
}

// PUT /api/settings/youtube {"apiKey": "..."}: checks the key with Google and
// saves it, encrypted. An empty key removes it.
func (s *Server) handlePutYouTube(w http.ResponseWriter, r *http.Request) {
	var req struct {
		APIKey string `json:"apiKey"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "That request couldn't be read.")
		return
	}
	key := strings.TrimSpace(req.APIKey)
	if len(key) > 100 || strings.ContainsAny(key, " \r\n\t&?=/") {
		writeError(w, http.StatusBadRequest, "That doesn't look like a YouTube API key. Copy it again from Google Cloud (it starts with AIza).")
		return
	}
	if key != "" {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		err := youtube.New(key, s.fallback.youtubeBase).Check(ctx)
		if errors.Is(err, youtube.ErrBadKey) {
			writeError(w, http.StatusBadRequest, "Google didn't accept that key. Check that YouTube Data API v3 is turned on for its project.")
			return
		}
		if err != nil {
			slog.Warn("youtube: check key", "err", err)
			writeError(w, http.StatusBadGateway, "Couldn't reach YouTube to check the key. Try again in a moment.")
			return
		}
	}
	if err := s.Settings.Set(settings.KeyYouTubeAPIKey, key, true); err != nil {
		writeError(w, http.StatusInternalServerError, "The key couldn't be saved.")
		return
	}
	s.fallback.mu.Lock()
	s.fallback.entries = nil // look again with (or without) YouTube
	s.fallback.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]bool{"set": key != ""})
}
