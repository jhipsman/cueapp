package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jhipsman/cueapp/internal/watch"
)

// Kids' profiles: a profile with an age limit (watch.AgeLimits: G, PG,
// PG-13, R) only sees and plays titles rated at or under it, everywhere:
// Home's rows, search, More like this, a title's page and Play. Films go by
// their US rating, shows by their TV rating matched to the film scale. A
// title with no rating is shown only under an R limit (most unrated titles
// on TMDB aren't made for children).

// ratingRank puts film and TV ratings on one scale: 1 G, 2 PG, 3 PG-13,
// 4 R, 5 NC-17; 0 for unrated or unknown.
var ratingRank = map[string]int{
	"G": 1, "TV-Y": 1, "TV-Y7": 1, "TV-Y7-FV": 1, "TV-G": 1,
	"PG": 2, "TV-PG": 2,
	"PG-13": 3, "TV-14": 3,
	"R": 4, "TV-MA": 4,
	"NC-17": 5,
}

// allowedRating says whether a title rated rank may be seen under limit.
func allowedRating(limit string, rank int) bool {
	max := ratingRank[limit]
	if limit == "" || max == 0 {
		return true
	}
	if rank == 0 {
		return max >= ratingRank["R"]
	}
	return rank <= max
}

type knownRating struct {
	rank int
	at   time.Time
}

// ratingsKnown remembers each title's rating rank ("movie-603" → 4).
var ratingsKnown sync.Map

// titleRank is a title's rating rank, from TMDB (remembered for a day).
// ok is false when TMDB couldn't be asked.
func (s *Server) titleRank(ctx context.Context, kind string, id int) (int, bool) {
	key := fmt.Sprintf("%s-%d", kind, id)
	if v, ok := ratingsKnown.Load(key); ok && time.Since(v.(knownRating).at) < 24*time.Hour {
		return v.(knownRating).rank, true
	}
	tm := s.TMDB()
	if tm == nil {
		return 0, false
	}
	var cert string
	switch kind {
	case "movie":
		d, err := tm.GetMovieDetail(ctx, id)
		if err != nil {
			return 0, false
		}
		cert = d.Certification()
	case "tv":
		d, err := tm.GetShowFull(ctx, id)
		if err != nil {
			return 0, false
		}
		cert = d.ContentRating()
	default:
		return 0, false
	}
	rank := ratingRank[strings.ToUpper(strings.TrimSpace(cert))]
	ratingsKnown.Store(key, knownRating{rank: rank, at: time.Now()})
	return rank, true
}

// titleAllowed says whether the profile may see and play a title. A title
// whose rating can't be looked up is kept from a profile with a limit.
func (s *Server) titleAllowed(ctx context.Context, p watch.Profile, kind string, id int) bool {
	if p.MaxRating == "" {
		return true
	}
	rank, ok := s.titleRank(ctx, kind, id)
	return ok && allowedRating(p.MaxRating, rank)
}

// ageFiltered is the cards the profile may see (all of them without a
// limit), looking ratings up a few at a time.
func (s *Server) ageFiltered(ctx context.Context, p watch.Profile, cards []watchCard) []watchCard {
	if p.MaxRating == "" || len(cards) == 0 {
		return cards
	}
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	keep := make([]bool, len(cards))
	var wg sync.WaitGroup
	slots := make(chan struct{}, 8)
	for i, c := range cards {
		wg.Add(1)
		go func(i int, c watchCard) {
			defer wg.Done()
			select {
			case slots <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-slots }()
			keep[i] = s.titleAllowed(ctx, p, c.Kind, c.TMDBID)
		}(i, c)
	}
	wg.Wait()
	out := cards[:0:0]
	for i, c := range cards {
		if keep[i] {
			out = append(out, c)
		}
	}
	return out
}

// ageBlocked answers a request for a title above the profile's limit (and
// reports that it did).
func (s *Server) ageBlocked(w http.ResponseWriter, r *http.Request, kind string, id int) bool {
	p, ok := s.activeProfile(r)
	if !ok || s.titleAllowed(r.Context(), p, kind, id) {
		return false
	}
	writeError(w, http.StatusForbidden, fmt.Sprintf("That isn't available on %s's profile (its age limit is %s).", p.Name, p.MaxRating))
	return true
}
