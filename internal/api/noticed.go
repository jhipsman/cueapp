package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/jhipsman/cueapp/internal/watch"
)

// Problems nobody reported: when people keep moving on from a title's
// first version (Other version, a version that wouldn't play or had no
// sound), Cue files a report itself, so the owner hears about it even when
// nobody says anything. Counted in memory over a week; once a title has a
// report filed this way it isn't filed again until that one's answered.

const (
	noticedWindow   = 7 * 24 * time.Hour
	noticedPeople   = 2 // different profiles moving on, or
	noticedSwitches = 3 // this many moves in all
)

type noticedKey struct {
	kind            string
	tmdbID          int
	season, episode int
}

type noticedTally struct {
	mu    sync.Mutex
	seen  map[noticedKey][]noticedHit
	filed map[noticedKey]int64 // the report filed, while it's open
}

type noticedHit struct {
	pid int64
	at  time.Time
}

var noticed = &noticedTally{seen: map[noticedKey][]noticedHit{}, filed: map[noticedKey]int64{}}

// add counts one move and says whether it tips the title over (and so a
// report should be filed now).
func (t *noticedTally) add(k noticedKey, pid int64, now time.Time, stillOpen func(id int64) bool) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if id, ok := t.filed[k]; ok {
		if stillOpen(id) {
			return false
		}
		delete(t.filed, k)
	}
	hits := t.seen[k][:0:0]
	for _, h := range t.seen[k] {
		if now.Sub(h.at) < noticedWindow {
			hits = append(hits, h)
		}
	}
	hits = append(hits, noticedHit{pid, now})
	t.seen[k] = hits
	people := map[int64]bool{}
	for _, h := range hits {
		people[h.pid] = true
	}
	if len(people) < noticedPeople && len(hits) < noticedSwitches {
		return false
	}
	t.filed[k] = 0 // being filed
	delete(t.seen, k)
	return true
}

func (t *noticedTally) filedAs(k noticedKey, id int64) {
	t.mu.Lock()
	t.filed[k] = id
	t.mu.Unlock()
}

// noteVersionMove is called on each play request; a request for a version
// past the first (not a look-ahead) counts as moving on.
func (s *Server) noteVersionMove(r *http.Request, kind string, tmdbID, season, episode int) {
	opt, _ := strconv.Atoi(r.URL.Query().Get("option"))
	if opt < 2 || r.URL.Query().Get("prefetch") != "" || r.URL.Query().Get("party") != "" || s.WatchRepo == nil {
		return
	}
	prof, ok := s.activeProfile(r)
	if !ok {
		return
	}
	k := noticedKey{kind, tmdbID, season, episode}
	open := func(id int64) bool {
		if id == 0 {
			return true // still being filed
		}
		rp, err := s.WatchRepo.Report(id)
		return err == nil && rp.Status == "open"
	}
	if !noticed.add(k, prof.ID, time.Now(), open) {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		title := fmt.Sprintf("%s %d", kind, tmdbID)
		if tm := s.TMDB(); tm != nil {
			if kind == watch.KindMovie {
				if d, err := tm.GetMovieDetail(ctx, tmdbID); err == nil {
					title = d.Title
				}
			} else if d, err := tm.GetShowFull(ctx, tmdbID); err == nil {
				title = d.Name
			}
		}
		rp, err := s.WatchRepo.AddReport(watch.Report{ProfileID: prof.ID, Kind: kind, TMDBID: tmdbID, Season: season, Episode: episode,
			Title: title, Problem: "other", Note: "Noticed by Cue: people keep switching away from the first version this week."})
		if err != nil {
			return
		}
		noticed.filedAs(k, rp.ID)
		owners, _ := s.WatchRepo.OwnerProfiles()
		for _, o := range owners {
			if ok, _ := s.WatchRepo.QueueAlert(o, fmt.Sprintf("report:%d", rp.ID), "Cue noticed a problem with "+title, "People keep switching versions. Have a look in Reports.", "/watch/reports"); ok {
				s.wakeProfile(ctx, o)
			}
		}
	}()
}
