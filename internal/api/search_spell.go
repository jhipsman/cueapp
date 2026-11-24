package api

import (
	"context"
	"sync"
	"time"

	"github.com/jhipsman/cueapp/internal/metadata"
)

// Spelling: TMDB finds nothing for "breakng bad". Most misspelled searches
// are for well-known titles, so the server keeps the names of the best
// known (and this week's trending) films and shows, about 900 of them, a
// day at a time, and a search that finds nothing is matched against them
// with a letter or two out ("breakng bad" is one letter from "breaking
// bad").

type titleBook struct {
	mu    sync.Mutex
	names []string // normalized
	byKey map[string]string
	at    time.Time
}

const titleBookFor = 24 * time.Hour

// knownTitles are the names of the best-known films and shows.
func (s *Server) knownTitles(ctx context.Context) []string {
	b := &s.titleBook
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.names != nil && time.Since(b.at) < titleBookFor {
		return b.names
	}
	tm := s.TMDB()
	var mu sync.Mutex
	set := map[string]string{}
	keep := func(name string) {
		if n := searchNorm(name); n != "" {
			mu.Lock()
			set[n] = name
			mu.Unlock()
		}
	}
	var wg sync.WaitGroup
	for page := 1; page <= 10; page++ {
		wg.Add(2)
		go func(p int) {
			defer wg.Done()
			if r, err := tm.DiscoverMovies(ctx, metadata.DiscoverQuery{Sort: metadata.SortVotes, Page: p}); err == nil {
				for _, m := range r.Results {
					keep(m.Title)
				}
			}
		}(page)
		go func(p int) {
			defer wg.Done()
			if r, err := tm.DiscoverTV(ctx, metadata.DiscoverQuery{Sort: metadata.SortVotes, Page: p}); err == nil {
				for _, sh := range r.Results {
					keep(sh.Name)
				}
			}
		}(page)
	}
	for page := 1; page <= 2; page++ {
		wg.Add(2)
		go func(p int) {
			defer wg.Done()
			if r, err := tm.MovieList(ctx, metadata.ListTrending, p); err == nil {
				for _, m := range r.Results {
					keep(m.Title)
				}
			}
		}(page)
		go func(p int) {
			defer wg.Done()
			if r, err := tm.TVList(ctx, metadata.ListTrending, p); err == nil {
				for _, sh := range r.Results {
					keep(sh.Name)
				}
			}
		}(page)
	}
	wg.Wait()
	if len(set) == 0 {
		return b.names // couldn't read them: the old list, if any
	}
	b.names = b.names[:0]
	for n := range set {
		b.names = append(b.names, n)
	}
	b.byKey = set
	b.at = time.Now()
	return b.names
}

// closestTitle is the well-known title the search is a letter or two from
// (one for short names, more for long ones), or "".
func closestTitle(q string, names []string) string {
	nq := searchNorm(q)
	if len(nq) < 4 {
		return ""
	}
	allowed := 1 + len(nq)/8
	best, bestD := "", allowed+1
	for _, n := range names {
		if abs(len(n)-len(nq)) > allowed {
			continue
		}
		if d := editDistance(nq, n, allowed); d < bestD {
			best, bestD = n, d
		}
	}
	return best
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// editDistance is how many letters must be added, dropped, changed or
// swapped to make a into b (stopping early past limit).
func editDistance(a, b string, limit int) int {
	ra, rb := []rune(a), []rune(b)
	prev2 := make([]int, len(rb)+1)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		rowMin := cur[0]
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
			if i > 1 && j > 1 && ra[i-1] == rb[j-2] && ra[i-2] == rb[j-1] {
				cur[j] = min(cur[j], prev2[j-2]+1) // two letters swapped
			}
			rowMin = min(rowMin, cur[j])
		}
		if rowMin > limit {
			return limit + 1
		}
		prev2, prev, cur = prev, cur, prev2
	}
	return prev[len(rb)]
}
