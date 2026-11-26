package api

import (
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jhipsman/cueapp/internal/chancat"
	"github.com/jhipsman/cueapp/internal/iptv"
)

// A profile's own Live TV: its channel lists (named, in its own order), its
// names for channels, and the picks on Live TV's front: Tonight (the big
// shows this evening) and For you (what's on the channels it watches most).

type langCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// languageCounts is the languages, most channels first.
func languageCounts(m map[string]int) []langCount {
	out := make([]langCount, 0, len(m))
	for k, v := range m {
		out = append(out, langCount{k, v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func cleanItems(items []string) []string {
	out := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, id := range items {
		if id = strings.TrimSpace(id); id != "" && len(id) <= 64 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	if len(out) > 1000 {
		out = out[:1000]
	}
	return out
}

// POST /api/live/lists {name, items?}: a new list.
func (s *Server) handleCreateLiveList(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	var req struct {
		Name  string   `json:"name"`
		Items []string `json:"items"`
	}
	if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.Name) == "" || len(req.Name) > 60 {
		writeError(w, http.StatusBadRequest, "Give the list a name (60 letters at most).")
		return
	}
	l, err := s.WatchRepo.CreateLiveList(pid, strings.TrimSpace(req.Name), cleanItems(req.Items))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't make the list.")
		return
	}
	writeJSON(w, http.StatusOK, l)
}

// PUT /api/live/lists/{id} {name?, items?}: renames the list and/or sets its
// channels, in order. DELETE: removes it.
func (s *Server) handleUpdateLiveList(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Which list?")
		return
	}
	if r.Method == http.MethodDelete {
		if err := s.WatchRepo.DeleteLiveList(pid, id); err != nil {
			writeError(w, http.StatusInternalServerError, "Couldn't remove the list.")
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var req struct {
		Name  *string  `json:"name"`
		Items []string `json:"items"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "That request couldn't be read.")
		return
	}
	if req.Name != nil {
		n := strings.TrimSpace(*req.Name)
		if n == "" || len(n) > 60 {
			writeError(w, http.StatusBadRequest, "Give the list a name (60 letters at most).")
			return
		}
		req.Name = &n
	}
	var items []string
	if req.Items != nil {
		items = cleanItems(req.Items)
	}
	if err := s.WatchRepo.UpdateLiveList(pid, id, req.Name, items); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't save the list.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PUT /api/live/channels/{id}/name {name}: this profile's name for a
// channel ("" gives it back its own).
func (s *Server) handleRenameChannel(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &req); err != nil || len(req.Name) > 60 {
		writeError(w, http.StatusBadRequest, "A name of 60 letters at most.")
		return
	}
	if err := s.WatchRepo.SetLiveName(pid, r.PathValue("id"), strings.TrimSpace(req.Name)); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't rename it.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/live/picks?tz=<minutes east of UTC>: Tonight, the big shows
// starting this evening (7 to 11 pm, the viewer's time) on the main
// channels, one a channel; and For you, what's on the channels this profile
// watches most.
func (s *Server) handleLivePicks(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	cats, chans, err := s.liveChannels(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"tonight": []liveHit{}, "forYou": []liveHit{}})
		return
	}
	tz, _ := strconv.Atoi(r.URL.Query().Get("tz"))
	loc := time.FixedZone("viewer", tz*60)
	now := time.Now()
	guide, _ := s.liveGuide()
	cat := s.liveCatalogFor(cats, chans)
	names, _ := s.WatchRepo.LiveNames(pid)
	view := func(ch iptv.Channel) liveChannel {
		in := cat.info[ch.ID]
		lc := liveChannel{ID: ch.ID, Num: ch.Num, Name: ch.Name, Category: ch.Category, Catchup: ch.CatchupDays,
			Clean: in.Clean, Country: in.Country, Topic: in.Topic, Rank: chancat.Rank(in), Language: in.Language}
		if in.Clean != "" {
			lc.Name = in.Clean
		}
		if n := names[ch.ID]; n != "" {
			lc.Name, lc.Renamed = n, true
		}
		if ch.Logo != "" {
			lc.Logo = "/api/live/logo/" + url.PathEscape(ch.ID)
		}
		if ch.EPGID != "" {
			lc.Now, lc.Next = guide.At(ch.EPGID, now)
		}
		return lc
	}

	// Tonight.
	local := now.In(loc)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	from, to := day.Add(19*time.Hour), day.Add(23*time.Hour)
	if !now.Before(to) { // late: tomorrow evening's
		from, to = from.Add(24*time.Hour), to.Add(24*time.Hour)
	}
	type pick struct {
		hit   liveHit
		score int
	}
	var picks []pick
	seenShow := map[string]bool{}
	for _, ch := range chans {
		in := cat.info[ch.ID]
		if ch.EPGID == "" || in.Divider || cat.mainOf[ch.ID] != "" || chancat.Rank(in) > 1 {
			continue
		}
		if cat.home != "" && in.Country != "" && in.Country != cat.home {
			continue
		}
		for _, p := range guide.Between(ch.EPGID, from, to) {
			if p.Start.Before(from) || p.Stop.Sub(p.Start) < 45*time.Minute || seenShow[p.Title] {
				continue
			}
			score := 10
			if chancat.Rank(in) == 0 {
				score += 10
			}
			switch in.Topic {
			case "sports", "movies":
				score += 5
			case "news", "local", "247":
				score -= 8
			}
			pp := p
			seenShow[p.Title] = true
			picks = append(picks, pick{liveHit{Channel: view(ch), Show: &pp}, score})
			break // one a channel
		}
	}
	sort.SliceStable(picks, func(i, j int) bool { return picks[i].score > picks[j].score })
	if len(picks) > 12 {
		picks = picks[:12]
	}
	sort.SliceStable(picks, func(i, j int) bool { return picks[i].hit.Show.Start.Before(picks[j].hit.Show.Start) })
	tonight := make([]liveHit, 0, len(picks))
	for _, p := range picks {
		tonight = append(tonight, p.hit)
	}

	// For you: the channels watched most, what's on them now.
	forYou := []liveHit{}
	most, _ := s.WatchRepo.LiveMostWatched(pid, 12)
	s.live.mu.Lock()
	byID := s.live.byID
	s.live.mu.Unlock()
	for _, id := range most {
		ch, ok := byID[id]
		if !ok {
			continue
		}
		v := view(ch)
		forYou = append(forYou, liveHit{Channel: v, Show: v.Now, OnNow: v.Now != nil})
	}
	writeJSON(w, http.StatusOK, map[string]any{"tonight": tonight, "forYou": forYou})
}
