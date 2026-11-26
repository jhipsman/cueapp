package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/jhipsman/cueapp/internal/chancat"
	"github.com/jhipsman/cueapp/internal/iptv"
	"github.com/jhipsman/cueapp/internal/settings"
)

// More TV guides than the provider's. A provider's guide often leaves
// channels out (or has the wrong one), so the owner can add XMLTV guides
// (Settings > Live TV > Guides). Their channels are known in Cue's guide as
// "s1:cnn.us" (the first extra source's "cnn.us"), and each channel takes its
// guide from, in order: a guide channel matched to it by hand; the
// provider's own; an extra guide's channel with the same name.

// guideSourceInfo is how loading an extra guide went.
type guideSourceInfo struct {
	URL      string `json:"url"`
	Channels int    `json:"channels"`
	Matched  int    `json:"matched"`
	Problem  string `json:"problem,omitempty"`
}

type extraGuides struct {
	names   map[string]string
	sources []guideSourceInfo
}

var sourceKeyPattern = regexp.MustCompile(`^s[0-9]+:`)

// isSourceKey says a guide id is an extra source's ("s1:cnn.us").
func isSourceKey(id string) bool { return sourceKeyPattern.MatchString(id) }

func firstName(names []string) string {
	for _, n := range names {
		if n = strings.TrimSpace(n); n != "" {
			return n
		}
	}
	return ""
}

// guideNameKey is a channel name as two guides can agree on it: its plain
// name (chancat: no country prefix, quality or symbols), lower case, only
// letters and digits ("US| CNN HD" and "CNN" are both "cnn").
func guideNameKey(name string) string {
	clean := chancat.Read(name, "").Clean
	var b strings.Builder
	for _, r := range strings.ToLower(clean) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// guideConfig is the extra guides' addresses and the matches made by hand.
func (s *Server) guideConfig() ([]string, map[string]string) {
	var sources []string
	if raw, err := s.Settings.Get(settings.KeyIPTVGuideSources); err == nil && raw != "" {
		_ = json.Unmarshal([]byte(raw), &sources)
	}
	match := map[string]string{}
	if raw, err := s.Settings.Get(settings.KeyIPTVGuideMatch); err == nil && raw != "" {
		_ = json.Unmarshal([]byte(raw), &match)
	}
	return sources, match
}

// applyGuideIDsLocked sets each channel's guide id: matched by hand, else
// the provider's, else an extra guide's by name (from the last load).
// Callers hold s.live.mu.
func (s *Server) applyGuideIDsLocked(chans []iptv.Channel) {
	_, match := s.guideConfig()
	if s.live.providerEPG == nil {
		s.live.providerEPG = map[string]string{}
	}
	for i := range chans {
		ch := &chans[i]
		if own, ok := s.live.providerEPG[ch.ID]; ok {
			ch.EPGID = own // the provider's, as first read (this list may be one already set)
		} else {
			s.live.providerEPG[ch.ID] = ch.EPGID
		}
		switch {
		case match[ch.ID] != "":
			ch.EPGID = match[ch.ID]
		case ch.EPGID == "" && s.live.autoGuide[ch.ID] != "":
			ch.EPGID = s.live.autoGuide[ch.ID]
		}
	}
}

// setAutoGuideLocked keeps the extra guides' matches by name and gives them
// to the channels that have no guide. Callers hold s.live.mu.
func (s *Server) setAutoGuideLocked(auto map[string]string) {
	s.live.autoGuide = auto
	if s.live.chans == nil {
		return
	}
	s.applyGuideIDsLocked(s.live.chans)
	for _, ch := range s.live.chans {
		s.live.byID[ch.ID] = ch
	}
	s.live.catalog = nil // a channel with a guide now sorts as one
}

// loadExtraGuides reads the extra guides: for each, the programmes of the
// channels matched to it by hand, and of channels with no guide whose name
// one of its channels has. Answers the guide (keys "sN:id"), the matches
// by name (channel id -> key) and how each source went.
func (s *Server) loadExtraGuides(now time.Time) (iptv.Guide, map[string]string, extraGuides) {
	sources, match := s.guideConfig()
	out := iptv.Guide{}
	auto := map[string]string{}
	info := extraGuides{names: map[string]string{}}
	if len(sources) == 0 {
		return out, auto, info
	}
	// The channels with no guide of their own, by name.
	s.live.mu.Lock()
	byName := map[string][]string{}
	for _, ch := range s.live.chans {
		if s.live.providerEPG[ch.ID] != "" || match[ch.ID] != "" || chancat.Read(ch.Name, "").Divider {
			continue
		}
		if k := guideNameKey(ch.Name); k != "" {
			byName[k] = append(byName[k], ch.ID)
		}
	}
	s.live.mu.Unlock()
	hc := s.liveHTTP()
	for i, url := range sources {
		prefix := fmt.Sprintf("s%d:", i+1)
		keep := map[string]bool{}
		for _, key := range match {
			if strings.HasPrefix(key, prefix) {
				keep[strings.TrimPrefix(key, prefix)] = true
			}
		}
		si := guideSourceInfo{URL: url}
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
		g, err := iptv.LoadXMLTV(ctx, hc, url, iptv.GuideRead{
			Keep: func(id string) bool { return keep[id] },
			OnChannel: func(id string, names []string) {
				si.Channels++
				info.names[prefix+id] = firstName(names)
				for _, n := range names {
					for _, chID := range byName[guideNameKey(n)] {
						if _, done := auto[chID]; !done {
							auto[chID] = prefix + id
							keep[id] = true
							si.Matched++
						}
					}
				}
			},
			From: now.Add(-liveGuideBack), To: now.Add(liveGuideAhead), DescUntil: now.Add(liveDescAhead),
		})
		cancel()
		if err != nil {
			si.Problem = err.Error()
			slog.Warn("live tv: an extra guide", "source", i+1, "err", err)
		}
		for id, list := range g {
			out[prefix+id] = list
		}
		info.sources = append(info.sources, si)
	}
	return out, auto, info
}

// GET /api/live/guide-setup: the extra guides (how each went) and the
// channels matched by hand. PUT {sources}: the extra guides' addresses; the
// guide loads again.
func (s *Server) handleGetGuideSetup(w http.ResponseWriter, r *http.Request) {
	sources, match := s.guideConfig()
	s.live.mu.Lock()
	info := s.live.sourceInfo
	loading := s.live.guideLoading
	s.live.mu.Unlock()
	if sources == nil {
		sources = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"sources": sources, "status": info, "matches": match, "loading": loading})
}

func (s *Server) handlePutGuideSetup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Sources []string `json:"sources"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "That request couldn't be read.")
		return
	}
	var clean []string
	for _, u := range req.Sources {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("%q isn't a web address (http:// or https://).", u))
			return
		}
		clean = append(clean, u)
	}
	if len(clean) > 5 {
		writeError(w, http.StatusBadRequest, "Five extra guides at most.")
		return
	}
	b, _ := json.Marshal(clean)
	if err := s.Settings.Set(settings.KeyIPTVGuideSources, string(b), true); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't save the guides.")
		return
	}
	s.reloadGuide()
	s.handleGetGuideSetup(w, r)
}

// reloadGuide has the guide load again soon (its sources or matches changed).
func (s *Server) reloadGuide() {
	s.live.mu.Lock()
	s.live.guideAt = time.Time{}
	s.live.guideErr = ""
	if s.live.chans != nil {
		s.applyGuideIDsLocked(s.live.chans)
		for _, ch := range s.live.chans {
			s.live.byID[ch.ID] = ch
		}
		s.live.catalog = nil
	}
	s.liveGuideLocked()
	s.live.mu.Unlock()
}

// GET /api/live/guide-channels?q=: guide channels to match a channel to by
// hand, by name (up to 40), with each one's guide.
func (s *Server) handleGuideChannels(w http.ResponseWriter, r *http.Request) {
	q := guideNameKey(r.URL.Query().Get("q"))
	sources, _ := s.guideConfig()
	s.live.mu.Lock()
	names := s.live.guideNames
	s.live.mu.Unlock()
	type option struct {
		Key    string `json:"key"`
		Name   string `json:"name"`
		Source string `json:"source"`
	}
	out := []option{}
	for key, name := range names {
		if q != "" && !strings.Contains(guideNameKey(name), q) && !strings.Contains(strings.ToLower(key), q) {
			continue
		}
		src := "Your provider's guide"
		if isSourceKey(key) {
			var n int
			_, _ = fmt.Sscanf(key, "s%d:", &n)
			src = fmt.Sprintf("Extra guide %d", n)
			if n >= 1 && n <= len(sources) {
				src += " (" + hostOf(sources[n-1]) + ")"
			}
		}
		out = append(out, option{Key: key, Name: name, Source: src})
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i].Name) != len(out[j].Name) {
			return len(out[i].Name) < len(out[j].Name) // the closest names first
		}
		return out[i].Name < out[j].Name
	})
	if len(out) > 40 {
		out = out[:40]
	}
	writeJSON(w, http.StatusOK, out)
}

// PUT /api/live/channels/{id}/guide {key}: matches the channel to a guide
// channel by hand ("" goes back to the automatic one).
func (s *Server) handleMatchGuide(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Key string `json:"key"`
	}
	if err := decodeJSON(r, &req); err != nil || id == "" || len(req.Key) > 200 {
		writeError(w, http.StatusBadRequest, "Which guide channel?")
		return
	}
	_, match := s.guideConfig()
	if req.Key == "" {
		delete(match, id)
	} else {
		match[id] = req.Key
	}
	b, _ := json.Marshal(match)
	if err := s.Settings.Set(settings.KeyIPTVGuideMatch, string(b), false); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't save the match.")
		return
	}
	s.reloadGuide()
	w.WriteHeader(http.StatusNoContent)
}

func hostOf(u string) string {
	u = strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://")
	if i := strings.IndexAny(u, "/?"); i >= 0 {
		u = u[:i]
	}
	return u
}
