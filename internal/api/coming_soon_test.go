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
