package api

import (
	"context"
	"testing"
	"time"
)

func TestComingSoonRow(t *testing.T) {
	now := time.Now()
	filmOutKnown.Store(920001, filmOut{home: "2027-03-01", at: now})
	filmOutKnown.Store(920002, filmOut{home: "2027-01-10", at: now})
	filmOutKnown.Store(920003, filmOut{at: now}) // no day known
	s := &Server{}
	pool := []watchCard{
		{Kind: "movie", TMDBID: 920003, Soon: true},
		{Kind: "movie", TMDBID: 920001, Soon: true},
		{Kind: "movie", TMDBID: 920002, Soon: true},
		{Kind: "movie", TMDBID: 920002, Soon: true}, // in two rows
		{Kind: "movie", TMDBID: 920004, Soon: true}, // also a reminder
	}
	reminded := []watchCard{{Kind: "movie", TMDBID: 920004, Soon: true, Reminding: true, OutDate: "2027-06-01"}}
	row := s.comingSoonRow(context.Background(), reminded, pool)
	var ids []int
	for _, c := range row {
		ids = append(ids, c.TMDBID)
	}
	want := []int{920004, 920002, 920001, 920003}
	if len(ids) != len(want) {
		t.Fatalf("row = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("row = %v, want %v", ids, want)
		}
	}
	if row[1].OutDate != "2027-01-10" {
		t.Fatalf("out date not carried: %+v", row[1])
	}
}

func TestNiceDate(t *testing.T) {
	now := time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)
	for day, want := range map[string]string{
		"2026-03-04": "March 4th", "2026-06-01": "June 1st", "2026-06-02": "June 2nd", "2026-06-03": "June 3rd",
		"2026-06-11": "June 11th", "2026-06-12": "June 12th", "2026-06-13": "June 13th", "2026-06-22": "June 22nd",
		"2027-01-31": "January 31st, 2027",
	} {
		if got := niceDate(day, now); got != want {
			t.Errorf("%s: %q, want %q", day, got, want)
		}
	}
}
