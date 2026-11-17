package api

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jhipsman/cueapp/internal/metadata"
)

// Out to stream: a film just announced, or only in cinemas, has no copy
// worth watching yet, so Cue doesn't offer to play it. It says when it's
// coming instead, and keeps it off Home's rows (they're for what you can
// watch tonight). TMDB's release dates say: a digital, disc or TV release
// that has happened, or (for older films with none listed) cinemas long
// enough ago (metadata.Availability).

type filmOut struct {
	out     bool
	home    string // the home release date, when known ("2025-03-04")
	cinemas string // the cinema date
	at      time.Time
}

// filmOutKnown remembers each film's answer for a few hours (dates move
// rarely, and Home asks about dozens of films).
var filmOutKnown sync.Map // tmdbID -> filmOut

const filmOutFor = 6 * time.Hour

// filmOutNow says whether a film is out to stream, with its dates. When TMDB
// can't be asked it's taken to be out (nothing is hidden by a hiccup).
func (s *Server) filmOutNow(ctx context.Context, tmdbID int) filmOut {
	if v, ok := filmOutKnown.Load(tmdbID); ok {
		if f := v.(filmOut); time.Since(f.at) < filmOutFor {
			return f
		}
	}
	tm := s.TMDB()
	if tm == nil {
		return filmOut{out: true}
	}
	d, err := tm.GetMovieDetail(ctx, tmdbID)
	if err != nil {
		return filmOut{out: true}
	}
	f := filmOutOf(d, time.Now())
	filmOutKnown.Store(tmdbID, f)
	return f
}

func filmOutOf(d *metadata.MovieDetail, now time.Time) filmOut {
	out, home, cinemas := d.Availability(now)
	return filmOut{out: out, home: home, cinemas: cinemas, at: now}
}

// notOutNote is what a film's page says instead of Play: "In cinemas now ·
// streaming from 4 March", "In cinemas 12 June", "Not out yet".
func (f filmOut) notOutNote(now time.Time) string {
	if f.out {
		return ""
	}
	today := now.Format("2006-01-02")
	var parts []string
	switch {
	case f.cinemas != "" && f.cinemas <= today:
		parts = append(parts, "In cinemas now")
	case f.cinemas != "":
		parts = append(parts, "In cinemas "+niceDate(f.cinemas, now))
	}
	if f.home != "" {
		parts = append(parts, "streaming from "+niceDate(f.home, now))
	}
	if len(parts) == 0 {
		return "Not out yet"
	}
	note := parts[0]
	for _, p := range parts[1:] {
		note += " · " + p
	}
	return note
}

// niceDate is "4 March", or "4 March 2027" when not this year.
func niceDate(day string, now time.Time) string {
	t, err := time.Parse("2006-01-02", day)
	if err != nil {
		return day
	}
	if t.Year() == now.Year() {
		return fmt.Sprintf("%d %s", t.Day(), t.Month())
	}
	return fmt.Sprintf("%d %s %d", t.Day(), t.Month(), t.Year())
}

// markSoon sets Soon on the films among these cards that aren't out to
// stream yet (a few at a time, for a few seconds at most).
func (s *Server) markSoon(ctx context.Context, cards []watchCard) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	slots := make(chan struct{}, 8)
	for i := range cards {
		if cards[i].Kind != "movie" {
			continue
		}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			select {
			case slots <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-slots }()
			cards[i].Soon = !s.filmOutNow(ctx, cards[i].TMDBID).out
		}(i)
	}
	wg.Wait()
}

// outOnly is the cards without the films not out yet.
func outOnly(cards []watchCard) []watchCard {
	out := cards[:0:0]
	for _, c := range cards {
		if !c.Soon {
			out = append(out, c)
		}
	}
	return out
}
