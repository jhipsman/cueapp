package api

import (
	"context"
	"sync"

	"github.com/jhipsman/cueapp/internal/metadata"
)

// A kids' Home: a profile whose age limit is PG or lower (TV-PG, G...)
// gets rows made for it at the top (family films, animated films, kids'
// shows), and no Top 10s from the grown-up services.

// kidsProfile says whether an age limit makes a profile a young child's.
func kidsProfile(limit string) bool {
	r := ratingRank[limit]
	return r > 0 && r <= ratingRank["PG"]
}

var kidsShelves = []struct {
	key, title string
	kind       string
	genre      int
}{
	{"kids-family", "Family films", "movie", 10751},
	{"kids-animated", "Animated films", "movie", 16},
	{"kids-shows", "Kids' shows", "tv", 10762},
}

// kidsRows loads the kids' rows (most popular first), each kept to what the
// profile may watch by the caller.
func (s *Server) kidsRows(ctx context.Context, tm *metadata.Client) []watchRow {
	out := make([]watchRow, len(kidsShelves))
	var wg sync.WaitGroup
	for i, sh := range kidsShelves {
		wg.Add(1)
		go func() {
			defer wg.Done()
			q := metadata.DiscoverQuery{Genre: sh.genre, Sort: metadata.SortPopular, Page: 1}
			var items []watchCard
			if sh.kind == "movie" {
				if p, err := tm.DiscoverMovies(ctx, q); err == nil {
					for _, m := range p.Results {
						if m.PosterPath != "" {
							items = append(items, movieCard(m))
						}
					}
				}
			} else if p, err := tm.DiscoverTV(ctx, q); err == nil {
				for _, sh := range p.Results {
					if sh.PosterPath != "" {
						items = append(items, showCard(sh))
					}
				}
			}
			out[i] = watchRow{Key: sh.key, Title: sh.title, Items: items}
		}()
	}
	wg.Wait()
	return out
}
