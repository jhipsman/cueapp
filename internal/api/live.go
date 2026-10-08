package api

// Live TV: an IPTV provider's channels (an Xtream Codes login), their logos,
// and a TV guide from the provider's XMLTV feed.
//
// The channel list is read from the provider and kept for a few hours. The
// guide, often tens of megabytes, loads in the background and keeps only what
// the channels list needs, from two hours ago to a day and a half ahead.
//
// Browsers play channels through Cue (/api/live/hls), which rewrites the
// playlists so every request comes back through it: a Cue on HTTPS can't play
// a provider's plain http stream, and providers don't allow other sites to
// read theirs. The TV app plays the provider's stream directly.
//
// The login is kept encrypted and stream addresses hold the password, so the
// address the browser sees is sealed.

import (
	"cmp"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rdborg/mediarium/internal/iptv"
	"github.com/rdborg/mediarium/internal/settings"
)

const (
	liveListFor    = 6 * time.Hour  // how long a channel list is used
	liveGuideFor   = 4 * time.Hour  // how often the guide reloads
	liveGuideBack  = 2 * time.Hour  // guide kept from this long ago...
	liveGuideAhead = 36 * time.Hour // ...to this far ahead
	liveSealPrefix = "cue-live:"
	liveGuideMax   = 300 // channels in one guide answer
)

// liveState is the Live TV caches, for the saved login.
type liveState struct {
	mu      sync.Mutex
	acct    iptv.Account // the login the caches belong to
	client  *iptv.Client
	status  iptv.Status
	cats    []iptv.Category
	chans   []iptv.Channel
	byID    map[string]iptv.Channel
	listAt  time.Time
	listing chan struct{} // closed when the running list load ends

	guide        iptv.Guide
	guideAt      time.Time
	guideLoading bool
	guideErr     string

	hc *http.Client // for the provider (tests swap it)

	logoFails       map[string]time.Time // logo address -> when fetching it last failed
	logoHostsLogged map[string]bool      // logo sites already logged as failing
}

func (s *Server) iptvAccount() (iptv.Account, bool) {
	raw, err := s.Settings.Get(settings.KeyIPTV)
	if err != nil || strings.TrimSpace(raw) == "" {
		return iptv.Account{}, false
	}
	var a iptv.Account
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		slog.Warn("live tv: read the saved login", "err", err)
		return iptv.Account{}, false
	}
	return a, a.Valid()
}

func (s *Server) liveHTTP() *http.Client {
	if s.live.hc != nil {
		return s.live.hc
	}
	return &http.Client{Timeout: 30 * time.Second}
}

// liveClient is the client for the saved login, or nil without one. A new
// login drops the caches of the old one. Callers hold s.live.mu.
func (s *Server) liveClientLocked() *iptv.Client {
	acct, ok := s.iptvAccount()
	if !ok {
		s.live.client = nil
		return nil
	}
	if s.live.client == nil || s.live.acct != acct {
		s.live.acct = acct
		s.live.client = iptv.New(acct, s.liveHTTP())
		s.live.cats, s.live.chans, s.live.byID = nil, nil, nil
		s.live.listAt = time.Time{}
		s.live.guide, s.live.guideAt, s.live.guideErr = nil, time.Time{}, ""
	}
	return s.live.client
}

// liveChannels is the provider's categories and channels, read again when
// they are old. When the provider doesn't answer, the old list is used.
func (s *Server) liveChannels(ctx context.Context) ([]iptv.Category, []iptv.Channel, error) {
	s.live.mu.Lock()
	c := s.liveClientLocked()
	if c == nil {
		s.live.mu.Unlock()
		return nil, nil, errLiveNotSet
	}
	if s.live.chans != nil && time.Since(s.live.listAt) < liveListFor {
		cats, chans := s.live.cats, s.live.chans
		s.live.mu.Unlock()
		return cats, chans, nil
	}
	if wait := s.live.listing; wait != nil { // someone is already reading it
		s.live.mu.Unlock()
		select {
		case <-wait:
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
		return s.liveChannels(ctx)
	}
	done := make(chan struct{})
	s.live.listing = done
	s.live.mu.Unlock()

	lctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 60*time.Second)
	defer cancel()
	cats, catErr := c.Categories(lctx)
	chans, err := c.Channels(lctx)
	if err == nil && catErr != nil {
		cats = nil // channels without group names still play
	}

	s.live.mu.Lock()
	defer s.live.mu.Unlock()
	s.live.listing = nil
	close(done)
	if s.live.client != c { // the login changed meanwhile
		return nil, nil, errLiveNotSet
	}
	if err != nil {
		if s.live.chans != nil {
			slog.Warn("live tv: refresh the channels; using the old list", "err", err)
			s.live.listAt = time.Now().Add(-liveListFor + 10*time.Minute) // try again in a while
			return s.live.cats, s.live.chans, nil
		}
		return nil, nil, err
	}
	s.live.cats, s.live.chans = cats, chans
	s.live.byID = make(map[string]iptv.Channel, len(chans))
	for _, ch := range chans {
		s.live.byID[ch.ID] = ch
	}
	s.live.listAt = time.Now()
	s.liveGuideLocked() // channels changed: the guide follows
	return cats, chans, nil
}

var errLiveNotSet = errors.New("live tv is not set up")

// liveGuide is the guide as loaded so far (nil before its first load), and
// whether a load is running. It starts one when the guide is old.
func (s *Server) liveGuide() (iptv.Guide, bool) {
	s.live.mu.Lock()
	defer s.live.mu.Unlock()
	return s.liveGuideLocked()
}

func (s *Server) liveGuideLocked() (iptv.Guide, bool) {
	c := s.live.client
	if c == nil || s.live.chans == nil {
		return s.live.guide, false
	}
	stale := s.live.guide == nil || time.Since(s.live.guideAt) > liveGuideFor
	if stale && !s.live.guideLoading && (s.live.guideErr == "" || time.Since(s.live.guideAt) > 30*time.Minute) {
		s.live.guideLoading = true
		wanted := map[string]bool{}
		for _, ch := range s.live.chans {
			if ch.EPGID != "" {
				wanted[ch.EPGID] = true
			}
		}
		go s.loadLiveGuide(c, wanted)
	}
	return s.live.guide, s.live.guideLoading
}

func (s *Server) loadLiveGuide(c *iptv.Client, wanted map[string]bool) {
	started := time.Now()
	now := time.Now()
	g, err := c.LoadGuide(context.Background(), wanted, now.Add(-liveGuideBack), now.Add(liveGuideAhead))
	s.live.mu.Lock()
	defer s.live.mu.Unlock()
	if s.live.client != c {
		return
	}
	s.live.guideLoading = false
	s.live.guideAt = time.Now()
	if err != nil {
		slog.Warn("live tv: load the guide", "err", err)
		s.live.guideErr = err.Error()
		return
	}
	s.live.guide, s.live.guideErr = g, ""
	slog.Info("live tv: guide loaded", "channels", len(g), "took", time.Since(started).Round(time.Millisecond))
}

// liveChannel is one channel as Watch sees it.
type liveChannel struct {
	ID       string          `json:"id"`
	Num      int             `json:"num"`
	Name     string          `json:"name"`
	Category string          `json:"category"`
	Logo     string          `json:"logo,omitempty"`
	Favorite bool            `json:"favorite"`
	Now      *iptv.Programme `json:"now,omitempty"`
	Next     *iptv.Programme `json:"next,omitempty"`
}

// GET /api/live/channels: every channel with what's on now and next.
func (s *Server) handleLiveChannels(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	cats, chans, err := s.liveChannels(r.Context())
	if errors.Is(err, errLiveNotSet) {
		writeJSON(w, http.StatusOK, map[string]any{"configured": false})
		return
	}
	if err != nil {
		slog.Warn("live tv: list channels", "err", err)
		writeError(w, http.StatusBadGateway, liveProviderMessage(err))
		return
	}
	favs, _ := s.WatchRepo.LiveFavorites(pid)
	guide, loading := s.liveGuide()
	now := time.Now()
	out := make([]liveChannel, 0, len(chans))
	for _, ch := range chans {
		lc := liveChannel{ID: ch.ID, Num: ch.Num, Name: ch.Name, Category: ch.Category, Favorite: favs[ch.ID]}
		if ch.Logo != "" {
			lc.Logo = "/api/live/logo/" + url.PathEscape(ch.ID)
		}
		if ch.EPGID != "" {
			lc.Now, lc.Next = guide.At(ch.EPGID, now)
		}
		out = append(out, lc)
	}
	if cats == nil {
		cats = []iptv.Category{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"configured":   true,
		"categories":   cats,
		"channels":     out,
		"guideReady":   guide != nil,
		"guideLoading": loading,
	})
}

// GET /api/live/guide?ids=1,2,3&from=<unix>&hours=6: the guide for those
// channels (at most liveGuideMax), from..from+hours.
func (s *Server) handleLiveGuide(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.watchProfile(w, r); !ok {
		return
	}
	if _, _, err := s.liveChannels(r.Context()); err != nil {
		if errors.Is(err, errLiveNotSet) {
			writeError(w, http.StatusNotFound, "Live TV isn't set up.")
			return
		}
		writeError(w, http.StatusBadGateway, liveProviderMessage(err))
		return
	}
	from := time.Now().Add(-30 * time.Minute)
	if n, err := strconv.ParseInt(r.URL.Query().Get("from"), 10, 64); err == nil && n > 0 {
		from = time.Unix(n, 0)
	}
	hours, _ := strconv.Atoi(r.URL.Query().Get("hours"))
	if hours <= 0 {
		hours = 6
	}
	hours = min(hours, 24)
	to := from.Add(time.Duration(hours) * time.Hour)

	guide, loading := s.liveGuide()
	s.live.mu.Lock()
	byID := s.live.byID
	s.live.mu.Unlock()
	out := map[string][]iptv.Programme{}
	for i, id := range strings.Split(r.URL.Query().Get("ids"), ",") {
		if i >= liveGuideMax {
			break
		}
		ch, ok := byID[strings.TrimSpace(id)]
		if !ok || ch.EPGID == "" {
			continue
		}
		if list := guide.Between(ch.EPGID, from, to); len(list) > 0 {
			out[ch.ID] = list
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"from": from.UTC(), "to": to.UTC(), "programmes": out,
		"guideReady": guide != nil, "guideLoading": loading,
	})
}

// PUT and DELETE /api/live/favorites/{id}: star a channel for this profile.
func (s *Server) handleLiveFavorite(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	if id == "" || len(id) > 64 {
		writeError(w, http.StatusBadRequest, "That isn't a channel.")
		return
	}
	if err := s.WatchRepo.SetLiveFavorite(pid, id, r.Method == http.MethodPut); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't save that.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"favorite": r.Method == http.MethodPut})
}

// liveChannelByID is the channel with that id, and the client to play it.
func (s *Server) liveChannelByID(ctx context.Context, id string) (iptv.Channel, *iptv.Client, error) {
	if _, _, err := s.liveChannels(ctx); err != nil {
		return iptv.Channel{}, nil, err
	}
	s.live.mu.Lock()
	defer s.live.mu.Unlock()
	ch, ok := s.live.byID[id]
	if !ok || s.live.client == nil {
		return iptv.Channel{}, nil, errLiveNoChannel
	}
	return ch, s.live.client, nil
}

var errLiveNoChannel = errors.New("no such channel")

// GET /api/live/play/{id}: where the channel plays. Browsers get Cue's
// address for it; the TV app also gets the provider's own.
func (s *Server) handleLivePlay(w http.ResponseWriter, r *http.Request) {
	ch, c, err := s.liveChannelByID(r.Context(), r.PathValue("id"))
	switch {
	case errors.Is(err, errLiveNotSet):
		writeError(w, http.StatusNotFound, "Live TV isn't set up.")
		return
	case errors.Is(err, errLiveNoChannel):
		writeError(w, http.StatusNotFound, "That channel isn't on your provider's list any more.")
		return
	case err != nil:
		writeError(w, http.StatusBadGateway, liveProviderMessage(err))
		return
	}
	sealed, err := s.profileBox.Encrypt(liveSealPrefix + c.StreamURL(ch.ID, "m3u8"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't start that channel.")
		return
	}
	out := map[string]any{
		"id": ch.ID, "num": ch.Num, "name": ch.Name,
		"url": "/api/live/hls?u=" + url.QueryEscape(sealed),
	}
	if ch.Logo != "" {
		out["logo"] = "/api/live/logo/" + url.PathEscape(ch.ID)
	}
	if strings.Contains(r.UserAgent(), "CueTV/") {
		// The TV plays MPEG-TS straight from the provider, which most
		// Xtream providers serve best; HLS when the account only has that.
		s.live.mu.Lock()
		formats := s.live.status.Formats
		s.live.mu.Unlock()
		format := "ts"
		if len(formats) > 0 && !slices.Contains(formats, "ts") {
			format = "m3u8"
		}
		out["direct"] = c.StreamURL(ch.ID, format)
	}
	writeJSON(w, http.StatusOK, out)
}

// liveStreamClient fetches playlists and video: no overall time limit (a
// segment can take a while on a slow line), but a quick first answer.
var liveStreamClient = &http.Client{
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ResponseHeaderTimeout: 20 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConnsPerHost:   8,
	},
}

// GET /api/live/hls?u=<sealed address>: a provider's playlist, with every
// address in it pointed back here, or a piece of video passed through.
func (s *Server) handleLiveHLS(w http.ResponseWriter, r *http.Request) {
	plain, err := s.profileBox.Decrypt(r.URL.Query().Get("u"))
	if err != nil || !strings.HasPrefix(plain, liveSealPrefix) {
		writeError(w, http.StatusBadRequest, "That stream address isn't one Cue made.")
		return
	}
	target := strings.TrimPrefix(plain, liveSealPrefix)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target, nil)
	if err != nil {
		writeError(w, http.StatusBadRequest, "That stream address isn't one Cue made.")
		return
	}
	req.Header.Set("User-Agent", "Cue")
	if rg := r.Header.Get("Range"); rg != "" {
		req.Header.Set("Range", rg)
	}
	client := liveStreamClient
	if s.live.hc != nil {
		client = s.live.hc
	}
	resp, err := client.Do(req)
	if err != nil {
		if r.Context().Err() == nil {
			slog.Warn("live tv: stream", "err", redactURLError(err))
		}
		writeError(w, http.StatusBadGateway, "The channel didn't answer. Try again in a moment.")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		msg := fmt.Sprintf("The channel didn't play (your provider answered %d).", resp.StatusCode)
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
			msg = "Your provider refused the channel. Too many devices may be watching at once."
		}
		writeError(w, http.StatusBadGateway, msg)
		return
	}

	ctype := resp.Header.Get("Content-Type")
	isPlaylist := strings.Contains(strings.ToLower(ctype), "mpegurl") ||
		strings.HasSuffix(strings.ToLower(resp.Request.URL.Path), ".m3u8")
	if !isPlaylist {
		for _, h := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges"} {
			if v := resp.Header.Get(h); v != "" {
				w.Header().Set(h, v)
			}
		}
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
		return
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		writeError(w, http.StatusBadGateway, "The channel didn't answer. Try again in a moment.")
		return
	}
	out, err := rewritePlaylist(string(body), resp.Request.URL, func(abs string) (string, error) {
		sealed, err := s.profileBox.Encrypt(liveSealPrefix + abs)
		if err != nil {
			return "", err
		}
		return "/api/live/hls?u=" + url.QueryEscape(sealed), nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't read the channel's playlist.")
		return
	}
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(w, out)
}

// rewritePlaylist points every address in an HLS playlist (its lines, and
// URI="..." attributes) at to(absolute address).
func rewritePlaylist(body string, base *url.URL, to func(string) (string, error)) (string, error) {
	resolve := func(ref string) (string, error) {
		u, err := base.Parse(strings.TrimSpace(ref))
		if err != nil {
			return "", err
		}
		return to(u.String())
	}
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	for i, line := range lines {
		t := strings.TrimSpace(line)
		switch {
		case t == "":
		case !strings.HasPrefix(t, "#"):
			v, err := resolve(t)
			if err != nil {
				return "", err
			}
			lines[i] = v
		case strings.Contains(t, `URI="`):
			start := strings.Index(t, `URI="`) + len(`URI="`)
			end := strings.Index(t[start:], `"`)
			if end < 0 {
				continue
			}
			v, err := resolve(t[start : start+end])
			if err != nil {
				return "", err
			}
			lines[i] = t[:start] + v + t[start+end:]
		}
	}
	return strings.Join(lines, "\n"), nil
}

// redactURLError drops the address (it holds the password) from a request error.
func redactURLError(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

// GET /api/live/logo/{id}: the channel's logo, through Cue (the page may
// only show pictures from a few places) and kept on disk for a week. A logo
// that can't be fetched is remembered for an hour, and the guide shows the
// channel's initials instead.
func (s *Server) handleLiveLogo(w http.ResponseWriter, r *http.Request) {
	ch, _, err := s.liveChannelByID(r.Context(), r.PathValue("id"))
	if err != nil || ch.Logo == "" {
		http.NotFound(w, r)
		return
	}
	link := logoURL(ch.Logo)
	sum := sha256.Sum256([]byte(link))
	name := hex.EncodeToString(sum[:16])
	dir := filepath.Join(s.cfg.ConfigDir, "cache", "live-logos")
	path := filepath.Join(dir, name)
	if st, err := os.Stat(path); err == nil && time.Since(st.ModTime()) < 7*24*time.Hour {
		if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
			serveLogo(w, b)
			return
		}
	}
	if link == "" || s.live.logoFailedRecently(link) {
		http.NotFound(w, r)
		return
	}
	b, err := fetchLogo(r.Context(), link)
	if err != nil {
		s.live.logoFailed(link, err)
		http.NotFound(w, r)
		return
	}
	if err := os.MkdirAll(dir, 0o755); err == nil {
		_ = os.WriteFile(path, b, 0o644)
	}
	serveLogo(w, b)
}

// logoURL tidies the logo addresses providers list: spaces, "//host/..."
// with no scheme, or no scheme at all. "" when it isn't a web address.
func logoURL(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.ReplaceAll(s, " ", "%20")
	switch {
	case strings.HasPrefix(s, "//"):
		s = "http:" + s
	case s != "" && !strings.Contains(s, "://"):
		s = "http://" + s
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return u.String()
}

// logoClient fetches channel logos. Logo sites are often careless with
// their certificates and turn away clients that don't look like a browser;
// a logo is only a picture (checked to be one, and served with no way to
// run anything), so it doesn't insist on a valid certificate.
var logoClient = &http.Client{
	Timeout: 15 * time.Second,
	Transport: &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // pictures only, see above
		MaxIdleConnsPerHost: 4,
		IdleConnTimeout:     60 * time.Second,
	},
}

func fetchLogo(ctx context.Context, link string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36")
	req.Header.Set("Accept", "image/avif,image/webp,image/png,image/svg+xml,image/*;q=0.8,*/*;q=0.5")
	resp, err := logoClient.Do(req)
	if err != nil {
		return nil, redactURLError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 3<<20))
	if err != nil {
		return nil, err
	}
	if logoType(b) == "" {
		return nil, fmt.Errorf("not a picture (%s)", http.DetectContentType(b))
	}
	return b, nil
}

// logoType is a logo's content type, or "" when it isn't a picture. SVG is
// recognised by its tag (Go's sniffing calls it text).
func logoType(b []byte) string {
	ct := http.DetectContentType(b)
	if strings.HasPrefix(ct, "image/") {
		return ct
	}
	head := strings.ToLower(string(b[:min(len(b), 1024)]))
	if strings.Contains(head, "<svg") {
		return "image/svg+xml"
	}
	return ""
}

func serveLogo(w http.ResponseWriter, b []byte) {
	ct := logoType(b)
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// An SVG opened on its own could carry script: nothing in it may run.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	_, _ = w.Write(b)
}

// logoFailedRecently says a logo couldn't be fetched within the last hour.
func (l *liveState) logoFailedRecently(link string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	at, ok := l.logoFails[link]
	return ok && time.Since(at) < time.Hour
}

// logoFailed remembers a logo that couldn't be fetched, and logs it (once
// per site, so a provider whose logos are all on one broken site says so
// once).
func (l *liveState) logoFailed(link string, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.logoFails == nil {
		l.logoFails = map[string]time.Time{}
	}
	l.logoFails[link] = time.Now()
	host := link
	if u, perr := url.Parse(link); perr == nil {
		host = u.Host
	}
	if l.logoHostsLogged == nil {
		l.logoHostsLogged = map[string]bool{}
	}
	if !l.logoHostsLogged[host] {
		l.logoHostsLogged[host] = true
		slog.Warn("live tv: a channel logo couldn't be fetched; the guide shows the channel's initials", "site", host, "err", err)
	}
}

// liveProviderMessage says what went wrong with the provider, for people.
func liveProviderMessage(err error) string {
	if errors.Is(err, iptv.ErrBadLogin) {
		return "Your IPTV provider didn't accept the saved login. Check it in Settings → Streaming."
	}
	return "Couldn't reach your IPTV provider. Try again in a moment."
}

// GET /api/settings/iptv: the saved login, without its password.
func (s *Server) handleGetIPTV(w http.ResponseWriter, r *http.Request) {
	acct, ok := s.iptvAccount()
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"set": false})
		return
	}
	out := map[string]any{"set": true, "server": acct.Server, "username": acct.Username}
	s.live.mu.Lock()
	if s.live.acct == acct && s.live.chans != nil {
		out["channels"] = len(s.live.chans)
		out["guideChannels"] = len(s.live.guide)
		if s.live.guideErr != "" {
			out["guideProblem"] = s.live.guideErr
		}
	}
	if !s.live.status.Expires.IsZero() && s.live.acct == acct {
		out["expires"] = s.live.status.Expires
	}
	s.live.mu.Unlock()
	writeJSON(w, http.StatusOK, out)
}

// PUT /api/settings/iptv {server, username, password}: checks the login with
// the provider and saves it, encrypted. An empty password keeps the saved
// one; an empty server removes Live TV.
func (s *Server) handlePutIPTV(w http.ResponseWriter, r *http.Request) {
	var req iptv.Account
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "That request couldn't be read.")
		return
	}
	acct := req.Clean()
	if acct.Server == "" {
		if err := s.Settings.Set(settings.KeyIPTV, "", true); err != nil {
			writeError(w, http.StatusInternalServerError, "Couldn't save that.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"set": false})
		return
	}
	if acct.Password == "" {
		if old, ok := s.iptvAccount(); ok && old.Server == acct.Server && old.Username == acct.Username {
			acct.Password = old.Password
		}
	}
	if !acct.Valid() || len(acct.Server) > 300 || len(acct.Username) > 200 || len(acct.Password) > 200 {
		writeError(w, http.StatusBadRequest, "Fill in the server address, username and password from your IPTV provider.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	st, err := iptv.New(acct, s.liveHTTP()).Status(ctx)
	if errors.Is(err, iptv.ErrBadLogin) {
		writeError(w, http.StatusBadRequest, "Your provider didn't accept that username and password.")
		return
	}
	if err != nil {
		slog.Warn("live tv: check login", "err", err)
		writeError(w, http.StatusBadGateway, "Couldn't reach that server. Check the address (with its port, like http://line.example.com:8080).")
		return
	}
	if !st.Active {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Your provider says the account is %s.", strings.ToLower(cmp.Or(st.Status, "not active"))))
		return
	}
	b, _ := json.Marshal(acct)
	if err := s.Settings.Set(settings.KeyIPTV, string(b), true); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't save that.")
		return
	}
	s.live.mu.Lock()
	s.liveClientLocked()
	s.live.status = st
	s.live.mu.Unlock()
	// Read the channels now, so Live TV opens quickly the first time.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		if _, _, err := s.liveChannels(ctx); err != nil {
			slog.Warn("live tv: first channel list", "err", err)
		}
	}()
	out := map[string]any{"set": true, "server": acct.Server, "username": acct.Username}
	if !st.Expires.IsZero() {
		out["expires"] = st.Expires
	}
	writeJSON(w, http.StatusOK, out)
}

// liveHit is one Live TV search result: a show on a channel (now or later),
// or a channel itself (Show empty).
type liveHit struct {
	Channel liveChannel     `json:"channel"`
	Show    *iptv.Programme `json:"show,omitempty"`
	OnNow   bool            `json:"onNow"`
}

// searchWords splits a search into the words that must all appear, folding
// the ways games are written ("Lakers vs Celtics", "Lakers v Celtics",
// "Lakers @ Celtics") into the team names.
func searchWords(q string) []string {
	var out []string
	for _, w := range strings.FieldsFunc(strings.ToLower(q), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r > 127 || r == '\'')
	}) {
		switch w {
		case "vs", "v", "at", "versus", "and", "the":
			continue
		}
		out = append(out, w)
	}
	return out
}

func matchesAll(text string, words []string) bool {
	text = strings.ToLower(text)
	for _, w := range words {
		if !strings.Contains(text, w) {
			return false
		}
	}
	return len(words) > 0
}

// GET /api/live/search?q=: what's on now and coming up whose title (or
// description) has every word searched for, and channels named that way.
// Games first by when they start; one result per show, on the lowest
// numbered channel carrying it.
func (s *Server) handleLiveSearch(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	words := searchWords(r.URL.Query().Get("q"))
	if len(words) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"hits": []liveHit{}})
		return
	}
	_, chans, err := s.liveChannels(r.Context())
	if errors.Is(err, errLiveNotSet) {
		writeJSON(w, http.StatusOK, map[string]any{"hits": []liveHit{}})
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, liveProviderMessage(err))
		return
	}
	favs, _ := s.WatchRepo.LiveFavorites(pid)
	guide, _ := s.liveGuide()
	now := time.Now()
	view := func(ch iptv.Channel) liveChannel {
		lc := liveChannel{ID: ch.ID, Num: ch.Num, Name: ch.Name, Category: ch.Category, Favorite: favs[ch.ID]}
		if ch.Logo != "" {
			lc.Logo = "/api/live/logo/" + url.PathEscape(ch.ID)
		}
		if ch.EPGID != "" {
			lc.Now, lc.Next = guide.At(ch.EPGID, now)
		}
		return lc
	}

	type found struct {
		hit   liveHit
		title bool // matched in the title, not only the description
	}
	var shows []found
	seenEPG := map[string]bool{}
	var channels []liveHit
	for _, ch := range chans { // by channel number
		if len(channels) < 12 && matchesAll(ch.Name, words) {
			channels = append(channels, liveHit{Channel: view(ch)})
		}
		if ch.EPGID == "" || seenEPG[ch.EPGID] {
			continue
		}
		seenEPG[ch.EPGID] = true
		for _, p := range guide[ch.EPGID] {
			if !p.Stop.After(now) {
				continue
			}
			inTitle := matchesAll(p.Title, words)
			if !inTitle && !matchesAll(p.Title+" "+p.Desc, words) {
				continue
			}
			pp := p
			shows = append(shows, found{hit: liveHit{Channel: view(ch), Show: &pp, OnNow: !p.Start.After(now)}, title: inTitle})
		}
	}
	// On now first, then by start; title matches before description ones.
	slices.SortStableFunc(shows, func(a, b found) int {
		if a.hit.OnNow != b.hit.OnNow {
			if a.hit.OnNow {
				return -1
			}
			return 1
		}
		if a.title != b.title {
			if a.title {
				return -1
			}
			return 1
		}
		return a.hit.Show.Start.Compare(b.hit.Show.Start)
	})
	hits := make([]liveHit, 0, 40)
	for i, f := range shows {
		if i >= 40 {
			break
		}
		hits = append(hits, f.hit)
	}
	hits = append(hits, channels...)
	writeJSON(w, http.StatusOK, map[string]any{"hits": hits, "guideReady": guide != nil})
}
