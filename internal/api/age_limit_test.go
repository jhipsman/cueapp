package api

import (
	"context"
	"testing"
	"time"

	"github.com/jhipsman/cueapp/internal/watch"
)

func TestAgeLimit(t *testing.T) {
	for _, c := range []struct {
		limit string
		rank  int
		want  bool
	}{
		{"", 5, true},
		{"PG", ratingRank["G"], true},
		{"PG", ratingRank["TV-PG"], true},
		{"PG", ratingRank["PG-13"], false},
		{"PG-13", ratingRank["TV-14"], true},
		{"PG-13", ratingRank["TV-MA"], false},
		{"PG-13", 0, false}, // unrated: only under R
		{"R", 0, true},
		{"R", ratingRank["NC-17"], false},
	} {
		if got := allowedRating(c.limit, c.rank); got != c.want {
			t.Errorf("limit %q, rank %d: %v, want %v", c.limit, c.rank, got, c.want)
		}
	}

	s := &Server{}
	now := time.Now()
	ratingsKnown.Store("movie-910001", knownRating{rank: ratingRank["PG"], at: now})
	ratingsKnown.Store("movie-910002", knownRating{rank: ratingRank["R"], at: now})
	ratingsKnown.Store("tv-910003", knownRating{rank: ratingRank["TV-Y7"], at: now})
	kid := watch.Profile{Name: "Sam", MaxRating: "PG"}
	cards := []watchCard{{Kind: "movie", TMDBID: 910001}, {Kind: "movie", TMDBID: 910002}, {Kind: "tv", TMDBID: 910003}, {Kind: "movie", TMDBID: 910099}}
	got := s.ageFiltered(context.Background(), kid, cards)
	if len(got) != 2 || got[0].TMDBID != 910001 || got[1].TMDBID != 910003 {
		t.Fatalf("kid sees %+v", got)
	}
	// No limit: everything, nothing looked up.
	if got := s.ageFiltered(context.Background(), watch.Profile{}, cards); len(got) != 4 {
		t.Fatalf("no limit sees %d", len(got))
	}
}
