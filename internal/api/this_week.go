package api

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jhipsman/cueapp/internal/watch"
)

// This week: the next episodes of the shows a profile watches (watched in
// the last four months, on My List, or with Remind me), airing in the next
// two weeks, soonest first, for the Coming soon page.

const thisWeekDays = 14

type weekEpisode struct {
	watchCard
	AirDate string `json:"airDate"`
}

// GET /api/watch/this-week
func (s *Server) handleThisWeek(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	out := s.comingEpisodes(ctx, pid, thisWeekDays)
	if prof, ok := s.activeProfile(r); ok && prof.MaxRating != "" {
		kept := out[:0]
		for _, e := range out {
			if s.titleAllowed(ctx, prof, e.Kind, e.TMDBID) {
				kept = append(kept, e)
			}
		}
		out = kept
	}
	writeJSON(w, http.StatusOK, out)
}

// comingEpisodes is the next episode of each show a profile watches that
// airs within days, soonest first.
func (s *Server) comingEpisodes(ctx context.Context, pid int64, days int) []weekEpisode {
	out := []weekEpisode{}
	tm := s.TMDB()
	if tm == nil || !tm.HasAPIKey() {
		return out
	}
	ids := map[int]bool{}
	var shows []int
	add := func(id int) {
		if !ids[id] && len(shows) < 30 {
			ids[id] = true
			shows = append(shows, id)
		}
	}
	if recent, err := s.WatchRepo.Recent(pid, 40); err == nil {
		for _, p := range recent {
			if p.Kind == watch.KindTV && time.Since(p.UpdatedAt) < 120*24*time.Hour {
				add(p.TMDBID)
			}
		}
	}
	if list, err := s.WatchRepo.List(pid); err == nil {
		for _, it := range list {
			if it.Kind == watch.KindTV {
				add(it.TMDBID)
			}
		}
	}
	if marked, err := s.WatchRepo.Marked(pid, watch.MarkRemind); err == nil {
		for _, m := range marked {
			if m.Kind == watch.KindTV {
				add(m.TMDBID)
			}
		}
	}
	today := time.Now().Format("2006-01-02")
	until := time.Now().Add(time.Duration(days) * 24 * time.Hour).Format("2006-01-02")
	found := make([]*weekEpisode, len(shows))
	var wg sync.WaitGroup
	slots := make(chan struct{}, 6)
	for i, id := range shows {
		wg.Add(1)
		go func() {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			d, err := tm.GetShowFull(ctx, id)
			if err != nil || d.NextEpisodeToAir == nil {
				return
			}
			n := d.NextEpisodeToAir
			if n.AirDate < today || n.AirDate > until {
				return
			}
			c := showCard(d.Show)
			c.Season, c.Episode, c.EpisodeTitle = n.Season, n.Episode, n.Name
			found[i] = &weekEpisode{watchCard: c, AirDate: n.AirDate}
		}()
	}
	wg.Wait()
	for _, e := range found {
		if e != nil {
			out = append(out, *e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].AirDate < out[j].AirDate })
	return out
}

// weekAhead is Friday's alert: what's coming this week on your shows
// ("3 new episodes this week: Paper Moons Sunday, ..."), once a week, from
// Friday morning.
func (s *Server) weekAhead(ctx context.Context, pid int64, now time.Time) (key, title, body string, ok bool) {
	if now.Weekday() != time.Friday || now.Hour() < 9 || now.Hour() >= 21 {
		return "", "", "", false
	}
	eps := s.comingEpisodes(ctx, pid, 7)
	if len(eps) == 0 {
		return "", "", "", false
	}
	y, wk := now.ISOWeek()
	key = fmt.Sprintf("week-ahead:%d-%02d", y, wk)
	title = fmt.Sprintf("%d new episodes this week", len(eps))
	if len(eps) == 1 {
		title = "A new episode this week"
	}
	var parts []string
	for i, e := range eps {
		if i == 4 {
			parts = append(parts, fmt.Sprintf("and %d more", len(eps)-4))
			break
		}
		day := e.AirDate
		if t, err := time.ParseInLocation("2006-01-02", e.AirDate, now.Location()); err == nil {
			day = t.Weekday().String()
			if t.Format("2006-01-02") == now.Format("2006-01-02") {
				day = "today"
			}
		}
		parts = append(parts, e.Title+" "+day)
	}
	return key, title, strings.Join(parts, ", "), true
}
