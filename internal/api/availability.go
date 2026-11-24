package api

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jhipsman/cueapp/internal/metadata"
)

// Out to stream. TMDB's release dates say whether a film is out at home (a
// digital, disc or TV release that has happened, or for older films with
// none listed, theaters long enough ago: metadata.Availability). A film in
// theaters now but not out at home still plays (the versions found are
// usually camera recordings, which Play then takes too), and is tagged "In
// theaters" everywhere. One not released anywhere yet has nothing to play:
// it's on Coming soon instead, with the day it streams.

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
	// On a streaming service already (a service's own film often lists only
	// its premiere): out, whatever the dates say.
	if !f.out {
		if wp, err := tm.GetWatchProviders(ctx, "movie", tmdbID, "US"); err == nil && len(wp.Subscription)+len(wp.Free)+len(wp.Ads) > 0 {
			f.out = true
		}
	}
	filmOutKnown.Store(tmdbID, f)
	return f
}

// inTheaters: showing in theaters now, not out at home yet.
func (f filmOut) inTheaters(now time.Time) bool {
	return !f.out && f.cinemas != "" && f.cinemas <= now.Format("2006-01-02")
}

// upcoming: not released anywhere yet (nothing to play).
func (f filmOut) upcoming(now time.Time) bool { return !f.out && !f.inTheaters(now) }

// theaterNote is a film in theaters' tag on its page: "In theaters now ·
// streaming March 4th".
func (f filmOut) theaterNote(now time.Time) string {
	note := "In theaters now"
	if f.home != "" {
		note += " · streaming " + niceDate(f.home, now)
	}
	return note
}

func filmOutOf(d *metadata.MovieDetail, now time.Time) filmOut {
	out, home, cinemas := d.Availability(now)
	return filmOut{out: out, home: home, cinemas: cinemas, at: now}
}

// notOutNote is what a film's page says instead of Play: "In theaters now ·
// streaming March 4th", "In theaters June 12th", "Not out yet".
func (f filmOut) notOutNote(now time.Time) string {
	if f.out {
		return ""
	}
	today := now.Format("2006-01-02")
	var parts []string
	switch {
	case f.cinemas != "" && f.cinemas <= today:
		parts = append(parts, "In theaters now")
	case f.cinemas != "":
		parts = append(parts, "In theaters "+niceDate(f.cinemas, now))
	}
	if f.home != "" {
		parts = append(parts, "streaming "+niceDate(f.home, now))
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

// niceDate is "March 4th", or "March 4th, 2027" when not this year.
func niceDate(day string, now time.Time) string {
	t, err := time.Parse("2006-01-02", day)
	if err != nil {
		return day
	}
	if t.Year() == now.Year() {
		return fmt.Sprintf("%s %s", t.Month(), ordinal(t.Day()))
	}
	return fmt.Sprintf("%s %s, %d", t.Month(), ordinal(t.Day()), t.Year())
}

// ordinal is "1st", "2nd", "3rd", "4th", "11th", "22nd"...
func ordinal(n int) string {
	suffix := "th"
	if n%100 < 11 || n%100 > 13 {
		switch n % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return fmt.Sprintf("%d%s", n, suffix)
}

// markSoon sets Soon on the films among these cards not released anywhere
// yet, and InTheaters on those showing in theaters only (a few at a time,
// for a few seconds at most).
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
			f := s.filmOutNow(ctx, cards[i].TMDBID)
			cards[i].Soon, cards[i].InTheaters = f.upcoming(time.Now()), f.inTheaters(time.Now())
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
