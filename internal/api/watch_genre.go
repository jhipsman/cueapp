package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/jhipsman/cueapp/internal/metadata"
)

// A genre on Search: the tiles shown before anything's typed open the most
// popular films or shows of that genre, as Watch cards (with a profile's
// age limit applied, like everywhere else).

// GET /api/watch/genre?kind=movie|tv&genre=ID&page=N
func (s *Server) handleWatchGenre(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("kind")
	genre, _ := strconv.Atoi(r.URL.Query().Get("genre"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if (kind != "movie" && kind != "tv") || genre <= 0 {
		writeError(w, http.StatusBadRequest, "Pick films or shows, and a genre.")
		return
	}
	if page < 1 || page > 20 {
		page = 1
	}
	tm := s.TMDB()
	if tm == nil || !tm.HasAPIKey() {
		writeJSON(w, http.StatusOK, []watchCard{})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	q := metadata.DiscoverQuery{Genre: genre, Sort: metadata.SortPopular, Page: page}
	cards := []watchCard{}
	if kind == "movie" {
		p, err := tm.DiscoverMovies(ctx, q)
		if err != nil {
			writeUpstreamError(w, "load the titles", err)
			return
		}
		for _, m := range p.Results {
			if m.PosterPath != "" {
				cards = append(cards, movieCard(m))
			}
		}
	} else {
		p, err := tm.DiscoverTV(ctx, q)
		if err != nil {
			writeUpstreamError(w, "load the titles", err)
			return
		}
		for _, sh := range p.Results {
			if sh.PosterPath != "" {
				cards = append(cards, showCard(sh))
			}
		}
	}
	if prof, ok := s.activeProfile(r); ok && prof.MaxRating != "" {
		kept := cards[:0]
		for _, c := range cards {
			if s.titleAllowed(ctx, prof, c.Kind, c.TMDBID) {
				kept = append(kept, c)
			}
		}
		cards = kept
	}
	writeJSON(w, http.StatusOK, cards)
}
