package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/rdborg/mediarium/internal/watch"
)

// Live TV follows: a profile follows words (a team, a show), and every show
// in the guide over the next day and a half with them in its title gets a
// reminder by itself, once per show (on the lowest numbered channel carrying
// it). A reminder taken off stays off.

const followAhead = 36 * time.Hour

// GET /api/live/follows: the words this profile follows.
func (s *Server) handleLiveFollows(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	list, err := s.WatchRepo.LiveFollows(pid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't load what you follow.")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// PUT or DELETE /api/live/follows {"phrase"}: follow, or stop following.
func (s *Server) handleSetLiveFollow(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	var req struct {
		Phrase string `json:"phrase"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "What to follow?")
		return
	}
	phrase := strings.Join(strings.Fields(req.Phrase), " ")
	if len(searchWords(phrase)) == 0 || len(phrase) > 60 {
		writeError(w, http.StatusBadRequest, "Follow a team or a show by name, like Packers.")
		return
	}
	if err := s.WatchRepo.SetLiveFollow(pid, phrase, r.Method == http.MethodPut); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't save that.")
		return
	}
	if r.Method == http.MethodPut {
		s.applyFollows(pid)
	}
	w.WriteHeader(http.StatusNoContent)
}

// applyFollows adds reminders for the coming shows the profile's follows
// match. Called whenever its reminders are read (every minute while Watch is
// open), so new guide days are picked up.
func (s *Server) applyFollows(pid int64) {
	follows, err := s.WatchRepo.LiveFollows(pid)
	if err != nil || len(follows) == 0 {
		return
	}
	guide, _ := s.liveGuide() // starts loading it when it's old
	if guide == nil {
		return
	}
	s.live.mu.Lock()
	chans := s.live.chans
	s.live.mu.Unlock()
	now := time.Now()
	type key struct {
		title string
		start int64
	}
	done := map[key]bool{}
	seenEPG := map[string]bool{}
	for _, ch := range chans { // by channel number
		if ch.EPGID == "" || seenEPG[ch.EPGID] {
			continue
		}
		seenEPG[ch.EPGID] = true
		for _, p := range guide.Between(ch.EPGID, now, now.Add(followAhead)) {
			if !p.Start.After(now) {
				continue // on already: the search shows it
			}
			k := key{strings.ToLower(p.Title), p.Start.Unix()}
			if done[k] {
				continue
			}
			for _, f := range follows {
				if matchesAll(p.Title, searchWords(f)) {
					done[k] = true
					_ = s.WatchRepo.AddFollowReminder(pid, watch.LiveReminder{StreamID: ch.ID, Start: p.Start, Stop: p.Stop, Title: p.Title})
					break
				}
			}
		}
	}
}
