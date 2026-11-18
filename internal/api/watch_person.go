package api

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/jhipsman/cueapp/internal/metadata"
)

// People: a title's cast with their photos, each opening a page of who they
// are and what else they've been in (all playable, most popular first).

type watchPerson struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Character string `json:"character,omitempty"`
	Photo     string `json:"photo,omitempty"`
}

func castPeople(cs []metadata.CastMember) []watchPerson {
	out := []watchPerson{}
	for _, c := range cs {
		if c.ID > 0 {
			out = append(out, watchPerson{ID: c.ID, Name: c.Name, Character: c.Character, Photo: metadata.ProfileURL(c.ProfilePath)})
		}
	}
	return out
}

type personPage struct {
	watchPerson
	Biography  string      `json:"biography,omitempty"`
	Born       string      `json:"born,omitempty"`
	Died       string      `json:"died,omitempty"`
	Birthplace string      `json:"birthplace,omitempty"`
	KnownFor   string      `json:"knownFor,omitempty"` // Acting, Directing...
	Credits    []watchCard `json:"credits"`
}

// GET /api/watch/person/{id}
func (s *Server) handleWatchPerson(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "That isn't a person.")
		return
	}
	tm := s.TMDB()
	if tm == nil || !tm.HasAPIKey() {
		writeError(w, http.StatusServiceUnavailable, "Cue needs its movie info key for this.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	p, err := tm.GetPerson(ctx, id)
	if err != nil {
		writeUpstreamError(w, "look that person up", err)
		return
	}
	page := personPage{watchPerson: watchPerson{ID: p.ID, Name: p.Name, Photo: metadata.ProfileURL(p.ProfilePath)},
		Biography: p.Biography, Born: p.Birthday, Died: p.Deathday, Birthplace: p.Birthplace, KnownFor: p.KnownFor}
	page.Credits = personCredits(p)
	if prof, ok := s.activeProfile(r); ok && prof.MaxRating != "" {
		page.Credits = s.ageFiltered(ctx, prof, page.Credits)
	}
	writeJSON(w, http.StatusOK, page)
}

// personCredits is what a person was in: as cast, then (for a director or
// writer) as crew; once each, with a poster, no adult titles, no talk-show
// or awards appearances (shows they were in for an episode or two as
// themselves), most popular first, at most 60.
func personCredits(p *metadata.Person) []watchCard {
	type hit struct {
		card watchCard
		pop  float64
	}
	seen := map[string]bool{}
	var hits []hit
	add := func(c metadata.PersonCredit, role string) {
		if c.PosterPath == "" || c.Adult || (c.MediaType != "movie" && c.MediaType != "tv") {
			return
		}
		if c.MediaType == "tv" && (c.Character == "Self" || c.Character == "Himself" || c.Character == "Herself" || (c.EpisodeCount > 0 && c.EpisodeCount < 3 && role == "")) {
			return
		}
		key := c.MediaType + strconv.Itoa(c.ID)
		if seen[key] {
			return
		}
		seen[key] = true
		var card watchCard
		if c.MediaType == "movie" {
			card = movieCard(c.Movie())
		} else {
			card = showCard(c.Show())
		}
		card.EpisodeTitle = role
		if role == "" {
			card.EpisodeTitle = c.Character
		}
		hits = append(hits, hit{card, c.Popularity + float64(c.VoteCount)/1000})
	}
	for _, c := range p.Credits.Cast {
		add(c, "")
	}
	for _, c := range p.Credits.Crew {
		if c.Job == "Director" || c.Job == "Creator" || c.Job == "Screenplay" || c.Job == "Writer" {
			add(c, c.Job)
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].pop > hits[j].pop })
	out := []watchCard{}
	for _, h := range hits {
		out = append(out, h.card)
		if len(out) == 60 {
			break
		}
	}
	return out
}
