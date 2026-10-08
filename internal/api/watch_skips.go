package api

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/rdborg/mediarium/internal/introdb"
	"github.com/rdborg/mediarium/internal/watch"
)

// Skip intro and Next episode during the credits. The times come from
// TheIntroDB when its users have submitted them, and otherwise from what this
// household has skipped before in the same show (a jump over the intro,
// Next episode pressed during the credits), kept per season.

var introDBOnce = sync.OnceValue(func() *introdb.Client { return introdb.New("") })

// skipsAnswer is in seconds. Credits are where they start, or how long
// before the end (when only that was learned and the video's length isn't
// known yet).
type skipsAnswer struct {
	Intro          *introdb.Segment `json:"intro,omitempty"`
	Recap          *introdb.Segment `json:"recap,omitempty"`
	CreditsStart   float64          `json:"creditsStart,omitempty"`
	CreditsFromEnd float64          `json:"creditsFromEnd,omitempty"`
	Source         string           `json:"source,omitempty"` // "theintrodb", "learned" or both
}

// GET /api/watch/skips/{kind}/{tmdbId}?season=&episode=&duration=
func (s *Server) handleWatchSkips(w http.ResponseWriter, r *http.Request) {
	kind, tmdbID, ok := watchKind(w, r)
	if !ok {
		return
	}
	season, _ := strconv.Atoi(r.URL.Query().Get("season"))
	episode, _ := strconv.Atoi(r.URL.Query().Get("episode"))
	duration, _ := strconv.ParseFloat(r.URL.Query().Get("duration"), 64)
	if kind == watch.KindMovie {
		season, episode = 0, 0
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()

	var out skipsAnswer
	seg, err := introDBOnce().Get(ctx, tmdbID, season, episode, duration)
	if err != nil {
		slog.Info("watch: TheIntroDB", "err", err)
	}
	if seg.Intro != nil && seg.Intro.End > 0 {
		out.Intro = &introdb.Segment{Start: max(seg.Intro.Start, 0), End: seg.Intro.End}
		out.Source = "theintrodb"
	}
	if seg.Recap != nil && seg.Recap.End > 0 {
		out.Recap = &introdb.Segment{Start: max(seg.Recap.Start, 0), End: seg.Recap.End}
		out.Source = "theintrodb"
	}
	if seg.Credits != nil && seg.Credits.Start > 0 && (duration == 0 || seg.Credits.Start < duration) {
		out.CreditsStart = seg.Credits.Start
		out.Source = "theintrodb"
	}

	learned := false
	if out.Intro == nil && kind == watch.KindTV {
		if m, ok, err := s.WatchRepo.SkipMarkFor(kind, tmdbID, season, "intro"); err == nil && ok {
			out.Intro = &introdb.Segment{Start: m.Start, End: m.End}
			learned = true
		}
	}
	if out.CreditsStart == 0 {
		if m, ok, err := s.WatchRepo.SkipMarkFor(kind, tmdbID, season, "credits"); err == nil && ok {
			if duration > 0 {
				out.CreditsStart = duration - m.End
			} else {
				out.CreditsFromEnd = m.End
			}
			learned = true
		}
	}
	if learned {
		if out.Source != "" {
			out.Source += "+learned"
		} else {
			out.Source = "learned"
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /api/watch/skips/learn {"kind","tmdbId","season","segment","start","end","duration"}:
// the household skipped an intro (start to end) or the credits (pressed
// Next at start, of duration). Implausible ones are ignored.
func (s *Server) handleLearnSkip(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.watchProfile(w, r); !ok {
		return
	}
	var req struct {
		Kind     string  `json:"kind"`
		TMDBID   int     `json:"tmdbId"`
		Season   int     `json:"season"`
		Segment  string  `json:"segment"`
		Start    float64 `json:"start"`
		End      float64 `json:"end"`
		Duration float64 `json:"duration"`
	}
	if err := decodeJSON(r, &req); err != nil || (req.Kind != watch.KindMovie && req.Kind != watch.KindTV) || req.TMDBID <= 0 {
		writeError(w, http.StatusBadRequest, "That request couldn't be read.")
		return
	}
	if req.Kind == watch.KindMovie {
		req.Season = 0
	}
	var err error
	switch {
	case req.Segment == "intro" && req.Kind == watch.KindTV && req.Start >= 0 && req.Start < 480 && req.End-req.Start >= 15 && req.End-req.Start <= 200:
		err = s.WatchRepo.SetSkipMark(req.Kind, req.TMDBID, req.Season, "intro", req.Start, req.End)
	case req.Segment == "credits" && req.Duration > 0 && req.Duration-req.Start >= 15 && req.Duration-req.Start <= 900:
		err = s.WatchRepo.SetSkipMark(req.Kind, req.TMDBID, req.Season, "credits", 0, req.Duration-req.Start)
	default:
		w.WriteHeader(http.StatusNoContent) // not a skip worth keeping
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't save that.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
