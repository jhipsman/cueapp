package api

import (
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Searching the TV from a phone. The TV's Search shows a QR code for
// /watch/remote/{code}; the phone that opens it (signed in to the same Cue)
// types or speaks there, PUT here, and the TV, asking every moment while its
// Search is open, takes the words as its search. A code lives half an hour
// after its last use. Only words pass this way, between the household's own
// signed-in screens.

type remoteSearch struct {
	Q    string `json:"q"`
	Seq  int    `json:"seq"`
	used time.Time
}

var remoteSearches = struct {
	sync.Mutex
	m map[string]*remoteSearch
}{m: map[string]*remoteSearch{}}

var remoteCode = regexp.MustCompile(`^[A-Z0-9]{4,8}$`)

const remoteSearchLife = 30 * time.Minute

// GET /api/watch/remote/{code}: the words sent so far ({"q":"","seq":0} before any).
func (s *Server) handleGetRemoteSearch(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(r.PathValue("code"))
	if !remoteCode.MatchString(code) {
		writeError(w, http.StatusBadRequest, "That code isn't one Cue gives out.")
		return
	}
	remoteSearches.Lock()
	out := remoteSearch{}
	if e, ok := remoteSearches.m[code]; ok && time.Since(e.used) < remoteSearchLife {
		out = *e
	}
	remoteSearches.Unlock()
	writeJSON(w, http.StatusOK, out)
}

// PUT /api/watch/remote/{code} {"q"}: the phone's words for the TV.
func (s *Server) handlePutRemoteSearch(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(r.PathValue("code"))
	var req struct {
		Q string `json:"q"`
	}
	if !remoteCode.MatchString(code) || decodeJSON(r, &req) != nil {
		writeError(w, http.StatusBadRequest, "That request couldn't be read.")
		return
	}
	q := strings.TrimSpace(req.Q)
	if len(q) > 120 {
		q = q[:120]
	}
	now := time.Now()
	remoteSearches.Lock()
	for k, e := range remoteSearches.m {
		if now.Sub(e.used) > remoteSearchLife {
			delete(remoteSearches.m, k)
		}
	}
	e, ok := remoteSearches.m[code]
	if !ok {
		e = &remoteSearch{}
		remoteSearches.m[code] = e
	}
	e.Q, e.used = q, now
	e.Seq++
	out := *e
	remoteSearches.Unlock()
	writeJSON(w, http.StatusOK, out)
}
