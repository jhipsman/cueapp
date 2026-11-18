package api

import (
	"context"
	"sort"
	"sync"

	"github.com/jhipsman/cueapp/internal/metadata"
)

// Top 10 on each service: Home's row of what's most watched right now on
// Netflix, Prime Video and the rest (TMDB's popularity among titles that
// service includes with a subscription, in the US), films and shows
// together, with big ranked numbers. Everything in it plays on Cue.

type topService struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	providers string // TMDB provider ids, "|" for several
}

var topServices = []topService{
	{Key: "netflix", Name: "Netflix", providers: "8"},
	{Key: "prime", Name: "Prime Video", providers: "9"},
	{Key: "appletv", Name: "Apple TV+", providers: "350"},
	{Key: "disney", Name: "Disney+", providers: "337"},
	{Key: "hulu", Name: "Hulu", providers: "15"},
	{Key: "max", Name: "Max", providers: "1899"},
	{Key: "peacock", Name: "Peacock", providers: "386"},
	{Key: "paramount", Name: "Paramount+", providers: "2303|2616"},
}

type topList struct {
	Key   string      `json:"key"`
	Name  string      `json:"name"`
	Items []watchCard `json:"items"`
}

type scored struct {
	card watchCard
	pop  float64
}

// top10 is each service's ten most popular titles right now (services with
// nothing found are left out).
func (s *Server) top10(ctx context.Context, tm *metadata.Client) []topList {
	out := make([]topList, len(topServices))
	var wg sync.WaitGroup
	for i, svc := range topServices {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var all []scored
			if p, err := tm.OnServiceMovies(ctx, svc.providers, "US"); err == nil {
				for _, m := range p.Results {
					all = append(all, scored{movieCard(m), m.Popularity})
				}
			}
			if p, err := tm.OnServiceShows(ctx, svc.providers, "US"); err == nil {
				for _, sh := range p.Results {
					all = append(all, scored{showCard(sh), sh.Popularity})
				}
			}
			sort.SliceStable(all, func(a, b int) bool { return all[a].pop > all[b].pop })
			var items []watchCard
			for _, x := range all {
				if x.card.PosterURL == "" {
					continue
				}
				items = append(items, x.card)
				if len(items) == 10 {
					break
				}
			}
			out[i] = topList{Key: svc.Key, Name: svc.Name, Items: items}
		}()
	}
	wg.Wait()
	kept := out[:0]
	for _, l := range out {
		if len(l.Items) > 0 {
			kept = append(kept, l)
		}
	}
	return kept
}
