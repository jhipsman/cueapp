package api

import (
	"context"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/jhipsman/cueapp/internal/watch"
)

// Coming soon, and Remind me. Films Home's rows leave out because they
// aren't out to stream yet go to their own row, Coming soon, with the day
// they stream when TMDB knows it. Any title not out yet can be marked Remind
// me (per profile); it heads Coming soon, and once it's out it moves to an
// "Out now" row at the top of Home until it's played (playing it clears the
// reminder), with a push to the phone where that's on (push.go).

// soonPool gathers, while Home's rows load, the films they leave out.
type soonPool struct {
	mu    sync.Mutex
	cards []watchCard
}

func (p *soonPool) add(cards []watchCard) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range cards {
		if c.Soon {
			p.cards = append(p.cards, c)
		}
	}
}

// remindedCards is a profile's reminders as cards: those out now, and those
// still to come (each with OutDate when known).
func (s *Server) remindedCards(ctx context.Context, pid int64) (out, coming []watchCard) {
	marked, err := s.WatchRepo.Marked(pid, watch.MarkRemind)
	if err != nil || len(marked) == 0 {
		return nil, nil
	}
	tm := s.TMDB()
	if tm == nil {
		return nil, nil
	}
	for _, m := range marked {
		var c watchCard
		var isOut bool
		switch m.Kind {
		case watch.KindMovie:
			d, err := tm.GetMovieDetail(ctx, m.TMDBID)
			if err != nil {
				continue
			}
			c = movieCard(d.Movie)
			f := filmOutOf(d, time.Now())
			filmOutKnown.Store(m.TMDBID, f)
			isOut, c.OutDate = f.out, f.home
		case watch.KindTV:
			d, err := tm.GetShowFull(ctx, m.TMDBID)
			if err != nil {
				continue
			}
			c = showCard(d.Show)
			isOut, c.OutDate = aired(d.FirstAirDate), d.FirstAirDate
		default:
			continue
		}
		if isOut {
			c.OutNow = true
			out = append(out, c)
		} else {
			c.Soon, c.Reminding = true, true
			coming = append(coming, c)
		}
	}
	return out, coming
}

// comingSoonRow is the reminders still to come, then the films Home left
// out, soonest first (those without a day last), at most 20.
func (s *Server) comingSoonRow(ctx context.Context, reminded []watchCard, pool []watchCard) []watchCard {
	seen := map[int]bool{}
	var films []watchCard
	for _, c := range pool {
		if c.Kind != watch.KindMovie || seen[c.TMDBID] {
			continue
		}
		seen[c.TMDBID] = true
		c.OutDate = s.filmOutNow(ctx, c.TMDBID).home
		films = append(films, c)
	}
	for _, c := range reminded {
		if c.Kind == watch.KindMovie {
			seen[c.TMDBID] = true
		}
	}
	rest := films[:0:0]
	for _, c := range films {
		if !reminderIn(reminded, c) {
			rest = append(rest, c)
		}
	}
	sort.SliceStable(rest, func(i, j int) bool {
		a, b := rest[i].OutDate, rest[j].OutDate
		if a == "" || b == "" {
			return a != "" && b == ""
		}
		return a < b
	})
	all := append(append([]watchCard{}, reminded...), rest...)
	if len(all) > 20 {
		all = all[:20]
	}
	return all
}

func reminderIn(list []watchCard, c watchCard) bool {
	for _, r := range list {
		if r.Kind == c.Kind && r.TMDBID == c.TMDBID {
			return true
		}
	}
	return false
}

// PUT and DELETE /api/watch/remind/{kind}/{tmdbId}: tell this profile when
// the title is out to stream (or not any more).
func (s *Server) handleRemind(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	kind, tmdbID, ok := watchKind(w, r)
	if !ok {
		return
	}
	if err := s.WatchRepo.SetMark(pid, kind, tmdbID, watch.MarkRemind, r.Method == http.MethodPut); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't save the reminder.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"reminding": r.Method == http.MethodPut})
}

// marksOf is a profile's marks (none when they can't be read).
func (s *Server) marksOf(pid int64) watch.Marks {
	m, err := s.WatchRepo.Marks(pid)
	if err != nil {
		return watch.Marks{}
	}
	return m
}
