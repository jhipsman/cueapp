package watch

import (
	"testing"
	"time"
)

func TestSummariseYear(t *testing.T) {
	at := func(s string) time.Time { v, _ := time.Parse(time.RFC3339, s); return v }
	ep := func(id, e int, pos, dur float64, when string) Progress {
		return Progress{Kind: KindTV, TMDBID: id, Season: 1, Episode: e, Position: pos, Duration: dur, Title: map[int]string{1: "Harbor", 2: "Copper"}[id], UpdatedAt: at(when)}
	}
	ps := []Progress{
		ep(1, 1, 2700, 2800, "2026-03-14T20:00:00Z"), // finished: counts the whole 2800 s
		ep(1, 2, 2790, 2800, "2026-03-14T21:00:00Z"),
		ep(1, 3, 1800, 2800, "2026-03-15T20:00:00Z"),
		ep(2, 1, 1200, 3000, "2026-07-01T20:00:00Z"),
		{Kind: KindMovie, TMDBID: 9, Title: "Night Film", Position: 7000, Duration: 7200, UpdatedAt: at("2026-07-02T20:00:00Z")},
		{Kind: KindMovie, TMDBID: 10, Title: "Opened", Position: 20, Duration: 7200, UpdatedAt: at("2026-07-03T20:00:00Z")}, // under a minute: not counted
	}
	st := summariseYear(2026, ps)
	if st.Titles != 3 || st.Shows != 2 || st.Movies != 1 || st.Episodes != 4 {
		t.Fatalf("counts: %+v", st)
	}
	// 2800 + 2800 + 1800 + 1200 + 7200 = 15800 s = 4.4 h; July (8400 s) beats March (7400 s)
	if st.Hours != 4.4 {
		t.Errorf("hours = %v, want 4.4", st.Hours)
	}
	if len(st.Top) != 2 || st.Top[0].Title != "Harbor" || st.Top[0].Episodes != 3 {
		t.Errorf("top shows: %+v", st.Top)
	}
	if st.TopMovie == nil || st.TopMovie.Title != "Night Film" || st.TopMovie.Hours != 2 {
		t.Errorf("top movie: %+v", st.TopMovie)
	}
	if st.BusiestDay != "2026-07-02" || st.BusiestMonth != "July" || st.Days != 4 || st.First != "Harbor" {
		t.Errorf("busiest day %s (%v h), month %s, days %d, first %q", st.BusiestDay, st.BusiestDayHours, st.BusiestMonth, st.Days, st.First)
	}
}

func TestSummariseEmptyYear(t *testing.T) {
	st := summariseYear(2026, nil)
	if st.Hours != 0 || st.Titles != 0 || st.Top == nil {
		t.Fatalf("%+v", st)
	}
}
