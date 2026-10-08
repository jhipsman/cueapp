package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rdborg/mediarium/internal/netguard"
	"github.com/rdborg/mediarium/internal/parser"
	"github.com/rdborg/mediarium/internal/quality"
	"github.com/rdborg/mediarium/internal/settings"
	"github.com/rdborg/mediarium/internal/stremio"
)

// Stream add-ons: Stremio add-ons such as Comet (on ElfHosted), Torrentio or
// MediaFusion, which Play asks for streams before searching the indexers.
//
// An add-on set up with a debrid service (Comet with a Premiumize key) answers
// with links that play straight away, already picked and sorted the way the
// person set it up on the add-on's own page; those are used as they come. An
// add-on without one answers with torrent info hashes, which are checked
// against Premiumize's cache like Cue's own search results.
//
// The add-on addresses usually hold the debrid key, so they are stored
// encrypted and never sent back to the browser in full.

// savedAddon is one add-on as stored.
type savedAddon struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

func (s *Server) savedAddons() []savedAddon {
	raw, err := s.Settings.Get(settings.KeyStreamAddons)
	if err != nil || strings.TrimSpace(raw) == "" {
		return nil
	}
	var out []savedAddon
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		slog.Warn("stream add-ons: read the saved list", "err", err)
		return nil
	}
	return out
}

func (s *Server) saveAddons(list []savedAddon) error {
	b, err := json.Marshal(list)
	if err != nil {
		return err
	}
	return s.Settings.Set(settings.KeyStreamAddons, string(b), true)
}

// addonStreams asks every saved add-on for t's streams, in parallel, and
// returns them add-on by add-on in the saved order: links to play, and info
// hashes to check with Premiumize. failed names the add-ons that didn't answer.
func (s *Server) addonStreams(ctx context.Context, t playTarget) (links, hashes []playCandidate, asked int, failed []string) {
	saved := s.savedAddons()
	if len(saved) == 0 {
		return nil, nil, 0, nil
	}
	kind, id, ok := s.stremioID(ctx, t)
	if !ok {
		return nil, nil, 0, nil
	}
	type answer struct {
		streams []stremio.Stream
		err     error
	}
	answers := make([]answer, len(saved))
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	for i, a := range saved {
		addon, err := stremio.New(a.URL)
		if err != nil {
			answers[i].err = err
			continue
		}
		wg.Add(1)
		go func(i int, addon *stremio.Addon) {
			defer wg.Done()
			answers[i].streams, answers[i].err = addon.Streams(ctx, kind, id)
		}(i, addon)
	}
	wg.Wait()

	seen := map[string]bool{}
	for i, ans := range answers {
		name := saved[i].Name
		if ans.err != nil {
			slog.Info("play: stream add-on", "addon", name, "err", ans.err)
			failed = append(failed, name)
			continue
		}
		asked++
		for _, st := range ans.streams {
			if notCached(st.Name, st.Title, st.Description) {
				continue // it would play the add-on's "not ready yet" video
			}
			label := st.Label()
			c := playCandidate{Release: label, Size: st.Hints.VideoSize, Tier: quality.Classify(parser.Parse(label)), Source: name,
				NotEnglish: notEnglish(label, st.Name, st.Title, st.Description)}
			switch {
			case st.URL != "" && !seen[st.URL]:
				seen[st.URL] = true
				c.URL = st.URL
				// The torrent behind the link, when the add-on says (or its
				// link holds it): a browser can then get Premiumize's own
				// converted copy, which plays with sound everywhere.
				c.Hash = normalizeHash(st.InfoHash)
				if c.Hash == "" {
					if m := linkHash.FindString(st.URL); m != "" {
						c.Hash = normalizeHash(m)
					}
				}
				links = append(links, c)
			case st.URL == "" && normalizeHash(st.InfoHash) != "":
				h := normalizeHash(st.InfoHash)
				if seen[h] {
					continue
				}
				seen[h] = true
				c.Hash = h
				hashes = append(hashes, c)
			}
		}
	}
	return links, hashes, asked, failed
}

// stremioID is the add-on id of t: "tt…" for a movie, "tt…:season:episode"
// for an episode, from TMDB's IMDb id.
func (s *Server) stremioID(ctx context.Context, t playTarget) (kind, id string, ok bool) {
	if t.movie != nil {
		if t.movie.TMDBID == 0 {
			return "", "", false
		}
		d, err := s.TMDB().GetMovieDetail(ctx, t.movie.TMDBID)
		if err != nil || d.IMDBID == "" {
			return "", "", false
		}
		return "movie", d.IMDBID, true
	}
	if t.series == nil || t.series.TMDBID == 0 {
		return "", "", false
	}
	d, err := s.TMDB().GetShowFull(ctx, t.series.TMDBID)
	if err != nil || d.ExternalIDs.IMDBID == "" {
		return "", "", false
	}
	return "series", fmt.Sprintf("%s:%d:%d", d.ExternalIDs.IMDBID, t.season, t.episode), true
}

// ---- Settings

type addonView struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
	Host  string `json:"host"`
}

func addonViews(list []savedAddon) []addonView {
	out := make([]addonView, 0, len(list))
	for i, a := range list {
		host := ""
		if ad, err := stremio.New(a.URL); err == nil {
			host = ad.Name()
		}
		out = append(out, addonView{Index: i, Name: a.Name, Host: host})
	}
	return out
}

// GET /api/settings/stream-addons: the saved add-ons, without their full
// addresses (those hold the debrid key).
func (s *Server) handleListStreamAddons(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, addonViews(s.savedAddons()))
}

// POST /api/settings/stream-addons {"url": ".../manifest.json"}: checks the
// add-on answers and serves streams, then adds it at the end.
func (s *Server) handleAddStreamAddon(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "That request couldn't be read.")
		return
	}
	if len(req.URL) > 4096 {
		writeError(w, http.StatusBadRequest, "That link is too long to be an add-on link.")
		return
	}
	addon, err := stremio.New(req.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, "That isn't an add-on link. Copy the link that ends in /manifest.json from the add-on's configure page (for Comet: the Copy link button).")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	m, err := addon.Manifest(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, "Couldn't reach that add-on: "+err.Error()+".")
		return
	}
	if !m.ServesStreams() {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("%s doesn't offer streams, so it can't help Play.", m.Name))
		return
	}
	list := s.savedAddons()
	for _, a := range list {
		if old, err := stremio.New(a.URL); err == nil && old.Base == addon.Base {
			writeError(w, http.StatusConflict, "That add-on is already added.")
			return
		}
	}
	name := strings.TrimSpace(m.Name)
	if name == "" {
		name = addon.Name()
	}
	list = append(list, savedAddon{URL: addon.Base + "/manifest.json", Name: name})
	if err := s.saveAddons(list); err != nil {
		writeError(w, http.StatusInternalServerError, "The add-on couldn't be saved.")
		return
	}
	playCache.clear()
	writeJSON(w, http.StatusOK, addonViews(list))
}

// DELETE /api/settings/stream-addons/{index}
func (s *Server) handleRemoveStreamAddon(w http.ResponseWriter, r *http.Request) {
	i, err := strconv.Atoi(r.PathValue("index"))
	list := s.savedAddons()
	if err != nil || i < 0 || i >= len(list) {
		writeError(w, http.StatusNotFound, "That add-on isn't in the list.")
		return
	}
	list = append(list[:i], list[i+1:]...)
	if err := s.saveAddons(list); err != nil {
		writeError(w, http.StatusInternalServerError, "The list couldn't be saved.")
		return
	}
	playCache.clear()
	writeJSON(w, http.StatusOK, addonViews(list))
}

// linkHash finds a torrent's info hash in an add-on's play link (Comet and
// Torrentio put it in the path).
// uncachedMark is how add-ons mark a stream the debrid service doesn't have
// yet (Comet and Torrentio: [PM⬇️] instead of [PM⚡]).
var uncachedMark = regexp.MustCompile(`(?i)⬇|⏳|\buncached\b|\bnot cached\b|\bdownload\]`)

func notCached(texts ...string) bool {
	for _, t := range texts {
		if uncachedMark.MatchString(t) {
			return true
		}
	}
	return false
}

// slateMax is the size under which a "video" from an add-on is its
// placeholder ("Not ready yet", a few seconds long), not a film or episode.
const slateMax = 15 << 20

// addonFileReal checks the file an add-on link led to: false for the
// add-on's placeholder video (ElfHosted's "Not ready yet" slate), which is
// tiny or a web page.
func addonFileReal(ctx context.Context, file string) bool {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	u, err := url.Parse(file)
	if err != nil {
		return true
	}
	if strings.Contains(strings.ToLower(u.Host), "slate") {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, file, nil)
	if err != nil {
		return true
	}
	req.Header.Set("Range", "bytes=0-0")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36")
	c := &http.Client{Transport: netguard.Transport(), Timeout: 15 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return true // can't tell: let the player try
	}
	resp.Body.Close()
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
		return false
	}
	size := resp.ContentLength
	if cr := resp.Header.Get("Content-Range"); cr != "" {
		if i := strings.LastIndexByte(cr, '/'); i >= 0 {
			if n, err := strconv.ParseInt(cr[i+1:], 10, 64); err == nil {
				size = n
			}
		}
	} else if resp.StatusCode == http.StatusPartialContent {
		size = -1
	}
	return size < 0 || size >= slateMax
}

var linkHash = regexp.MustCompile(`(?i)\b[0-9a-f]{40}\b`)

// addonLinkClient opens add-on links from the server (tests swap it). It
// never follows a redirect: resolveAddonLink reads each one itself.
var addonLinkClient = func() *http.Client {
	c := &http.Client{Transport: netguard.Transport(), Timeout: 45 * time.Second}
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return c
}

// resolveAddonLink opens an add-on's play link from the server and returns
// where it leads: the debrid service's own link to the file.
//
// Comet (and others) only play a link from the IP address that asked for it,
// and Cue asks from the server, while the TV or phone plays from somewhere
// else (a cloud server and a home are never the same address). The add-on
// answers its link with a redirect to the debrid file, and that one plays
// from anywhere, so the device gets that instead. The redirect's target
// itself is not opened. When the link doesn't redirect (it is the file, or
// the add-on streams through itself), it is returned as it is.
func resolveAddonLink(ctx context.Context, link string) string {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	start, err := url.Parse(link)
	if err != nil {
		return link
	}
	cur := start
	client := addonLinkClient()
	for range 5 {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, cur.String(), nil)
		if err != nil {
			return link
		}
		req.Header.Set("Range", "bytes=0-0") // only where it leads, not the file
		// Add-ons behind Cloudflare turn away clients that don't look
		// like a player.
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36")
		took := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			slog.Warn("play: open an add-on link; the device gets the add-on's own link", "host", start.Host, "err", netguard.CleanError(err))
			return link
		}
		ctype := resp.Header.Get("Content-Type")
		resp.Body.Close()
		loc := resp.Header.Get("Location")
		if resp.StatusCode < 300 || resp.StatusCode >= 400 || loc == "" {
			slog.Warn("play: an add-on link didn't lead to a file; the device gets the add-on's own link",
				"host", start.Host, "status", resp.StatusCode, "type", ctype, "hops", cur != start, "took", time.Since(took).Round(time.Millisecond))
			if cur == start {
				return link
			}
			return cur.String()
		}
		next, err := cur.Parse(loc)
		if err != nil || (next.Scheme != "http" && next.Scheme != "https") {
			return link
		}
		if next.Host != start.Host {
			slog.Info("play: add-on link opened on the server", "host", start.Host, "file host", next.Host, "took", time.Since(took).Round(time.Millisecond))
			return next.String() // off the add-on: the debrid file
		}
		cur = next
	}
	return link
}
