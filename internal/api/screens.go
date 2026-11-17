package api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jhipsman/cueapp/internal/auth"
)

// Send to TV. A TV app that's open keeps a line to Cue (GET
// /api/screens/listen, Server-Sent Events) and says what it's playing
// (POST .../state). A phone or computer signed in to the same Cue lists the
// screens (GET /api/screens), sends one a title to play (POST .../play) and
// then works as its remote (POST .../control: play, pause, back, forward,
// seek, stop). Like watch parties, a household's screens are everyone's on
// this Cue: they're the household's TVs. Screens live in memory and go as
// soon as their line closes.

const (
	screenGone     = 20 * time.Second // a screen whose line dropped this long ago is off
	screenMaxCount = 30
)

// screenNow is what a screen is showing.
type screenNow struct {
	Kind    string  `json:"kind,omitempty"` // "movie" or "tv"; "" on Home
	TMDBID  int     `json:"tmdbId,omitempty"`
	Season  int     `json:"season,omitempty"`
	Episode int     `json:"episode,omitempty"`
	Title   string  `json:"title,omitempty"`
	Pos     float64 `json:"pos,omitempty"`      // seconds, when it said
	Length  float64 `json:"duration,omitempty"` // seconds
	Playing bool    `json:"playing"`
	At      int64   `json:"at,omitempty"` // Unix milliseconds of Pos
}

type screen struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Profile string    `json:"profile,omitempty"` // whose profile it's on
	Now     screenNow `json:"now"`
	Online  bool      `json:"online"`
	lines   map[chan []byte]bool
	lastOn  time.Time
}

var screens = struct {
	sync.Mutex
	byID map[string]*screen
}{byID: map[string]*screen{}}

func screenSend(sc *screen, event string, data any) {
	body, err := json.Marshal(data)
	if err != nil {
		return
	}
	frame := []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", event, body))
	for ch := range sc.lines {
		select {
		case ch <- frame:
		default:
		}
	}
}

func pruneScreens() {
	for id, sc := range screens.byID {
		if len(sc.lines) == 0 && time.Since(sc.lastOn) > screenGone {
			delete(screens.byID, id)
		}
	}
}

// GET /api/screens/listen?id=&name=: a TV's line (SSE): "play" {kind,
// tmdbId, season, episode, from} and "control" {action, pos}.
func (s *Server) handleScreenListen(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	name := truncate(r.URL.Query().Get("name"), 40)
	if !validClient(id) {
		writeError(w, http.StatusBadRequest, "That screen id isn't valid.")
		return
	}
	if name == "" {
		name = "TV"
	}
	prof, _ := s.activeProfile(r)
	ch := make(chan []byte, 16)
	screens.Lock()
	pruneScreens()
	sc := screens.byID[id]
	if sc == nil {
		if len(screens.byID) >= screenMaxCount {
			screens.Unlock()
			writeError(w, http.StatusServiceUnavailable, "Too many screens are connected.")
			return
		}
		sc = &screen{ID: id, lines: map[chan []byte]bool{}}
		screens.byID[id] = sc
	}
	sc.Name, sc.Profile = name, prof.Name
	sc.lines[ch] = true
	sc.lastOn = time.Now()
	screens.Unlock()

	fl, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "retry: 3000\n: hello\n\n")
	if fl != nil {
		fl.Flush()
	}
	beat := time.NewTicker(partyBeat)
	defer beat.Stop()
	defer func() {
		screens.Lock()
		delete(sc.lines, ch)
		sc.lastOn = time.Now()
		screens.Unlock()
	}()
	for {
		select {
		case <-r.Context().Done():
			return
		case frame := <-ch:
			if _, err := w.Write(frame); err != nil {
				return
			}
		case <-beat.C:
			if _, err := w.Write([]byte(": still here\n\n")); err != nil {
				return
			}
		}
		if fl != nil {
			fl.Flush()
		}
	}
}

// POST /api/screens/{id}/state {kind, tmdbId, season, episode, title,
// pos, duration, playing}: what the TV is showing (it says so on every
// change, and every few seconds while playing). An empty kind: Home.
func (s *Server) handleScreenState(w http.ResponseWriter, r *http.Request) {
	var now screenNow
	if err := decodeJSON(r, &now); err != nil {
		writeError(w, http.StatusBadRequest, "Couldn't read that.")
		return
	}
	now.Title = truncate(now.Title, 200)
	now.At = time.Now().UnixMilli()
	screens.Lock()
	defer screens.Unlock()
	sc := screens.byID[r.PathValue("id")]
	if sc == nil {
		writeError(w, http.StatusNotFound, "That screen isn't connected.")
		return
	}
	sc.Now = now
	if p, ok := s.activeProfile(r); ok {
		sc.Profile = p.Name
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/screens: the household's TVs that are on, with what each shows.
func (s *Server) handleListScreens(w http.ResponseWriter, r *http.Request) {
	screens.Lock()
	pruneScreens()
	out := []screen{}
	for _, sc := range screens.byID {
		c := *sc
		c.Online = len(sc.lines) > 0
		c.lines = nil
		if c.Online {
			out = append(out, c)
		}
	}
	screens.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	writeJSON(w, http.StatusOK, out)
}

// POST /api/screens/{id}/play {kind, tmdbId, season, episode}: the TV plays it.
func (s *Server) handleScreenPlay(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Kind    string `json:"kind"`
		TMDBID  int    `json:"tmdbId"`
		Season  int    `json:"season"`
		Episode int    `json:"episode"`
	}
	if err := decodeJSON(r, &req); err != nil || (req.Kind != "movie" && req.Kind != "tv") || req.TMDBID <= 0 {
		writeError(w, http.StatusBadRequest, "That isn't something a TV can play.")
		return
	}
	if req.Kind == "tv" && req.Episode < 1 {
		req.Season, req.Episode = 1, 1
	}
	from := ""
	if p, ok := s.activeProfile(r); ok {
		from = p.Name
	} else if u := auth.UserFromContext(r.Context()); u != nil {
		from = u.Username
	}
	if !s.toScreen(w, r.PathValue("id"), "play", map[string]any{
		"kind": req.Kind, "tmdbId": req.TMDBID, "season": req.Season, "episode": req.Episode, "from": from,
	}) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/screens/{id}/control {action: play|pause|back|forward|seek|stop, pos}.
func (s *Server) handleScreenControl(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action string  `json:"action"`
		Pos    float64 `json:"pos"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Couldn't read that.")
		return
	}
	switch req.Action {
	case "play", "pause", "back", "forward", "seek", "stop":
	default:
		writeError(w, http.StatusBadRequest, "That isn't something the remote does.")
		return
	}
	if !s.toScreen(w, r.PathValue("id"), "control", map[string]any{"action": req.Action, "pos": math.Max(0, req.Pos)}) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) toScreen(w http.ResponseWriter, id, event string, data any) bool {
	screens.Lock()
	defer screens.Unlock()
	sc := screens.byID[strings.TrimSpace(id)]
	if sc == nil || len(sc.lines) == 0 {
		writeError(w, http.StatusNotFound, "That TV isn't on any more. Open Cue on it and try again.")
		return false
	}
	screenSend(sc, event, data)
	return true
}
