package api

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/jhipsman/cueapp/internal/metadata"
)

// ---- Browse: a page per streaming service (its Top 10, what's new, its
// originals, its best and its genres), collections across every service
// (the acclaimed, family night, under 90 minutes, hidden gems, and the
// season's: holiday films, Halloween horror), and the big film series.

// serviceNetworks are the TMDB networks a service's own shows first air on:
// its originals.
var serviceNetworks = map[string]string{
	"netflix": "213", "prime": "1024", "appletv": "2552", "disney": "2739",
	"hulu": "453", "max": "49|3186", "peacock": "3353", "paramount": "4330",
}

// browseQuery is one row or collection: what to ask TMDB for films and for
// shows (nil: none of that kind).
type browseQuery struct {
	movie, tv *metadata.TopQuery
}

// browseCollection is a collection across services.
type browseCollection struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	Blurb string `json:"blurb"`
	// Season: shown only in its time of year ("holiday", "halloween").
	season string
	query  func() browseQuery
}

var browseCollections = []browseCollection{
	{Key: "holiday", Title: "Holiday movies", Blurb: "Christmas favorites, old and new.", season: "holiday", query: func() browseQuery {
		return browseQuery{movie: &metadata.TopQuery{Keywords: "207317", Sort: "votes"}}
	}},
	{Key: "halloween", Title: "Halloween horror", Blurb: "The scariest films people love most.", season: "halloween", query: func() browseQuery {
		return browseQuery{movie: &metadata.TopQuery{Genre: 27, MinRating: 6.5, Sort: "votes"}}
	}},
	{Key: "acclaimed", Title: "Critically acclaimed", Blurb: "The best-rated films and shows of all time.", query: func() browseQuery {
		return browseQuery{movie: &metadata.TopQuery{Sort: "rating", MinVotes: 3000}, tv: &metadata.TopQuery{Sort: "rating", MinVotes: 1500}}
	}},
	{Key: "family", Title: "Family movie night", Blurb: "Rated G or PG, and well liked.", query: func() browseQuery {
		return browseQuery{movie: &metadata.TopQuery{Genres: "10751", MaxCertification: "PG", MinRating: 6.8, Sort: "votes"}}
	}},
	{Key: "short", Title: "Under 90 minutes", Blurb: "Good films that won't keep you up.", query: func() browseQuery {
		return browseQuery{movie: &metadata.TopQuery{MaxRuntime: 90, MinRating: 6.8, Sort: "votes"}}
	}},
	{Key: "gems", Title: "Hidden gems", Blurb: "Rated highly, but fewer people have found them.", query: func() browseQuery {
		// Rated in the US (no unrated oddities), no animation or documentaries
		// (which would fill it), known to a few hundred people at least, and
		// out a year or more (not this month's hype).
		until := time.Now().AddDate(-1, 0, 0).Format("2006-01-02")
		return browseQuery{
			movie: &metadata.TopQuery{Sort: "rating", MinVotes: 300, MaxVotes: 2000, MinRating: 7.2, MaxCertification: "R", Without: "16|99", Until: until},
			tv:    &metadata.TopQuery{Sort: "rating", MinVotes: 200, MaxVotes: 1000, MinRating: 7.8, Without: "16|99|10764|10767", Until: until},
		}
	}},
}

// inSeason is whether a seasonal collection shows today: holiday films from
// mid-November through December, horror through October.
func inSeason(season string, now time.Time) bool {
	switch season {
	case "holiday":
		return (now.Month() == time.November && now.Day() >= 15) || now.Month() == time.December
	case "halloween":
		return now.Month() == time.October
	}
	return true
}

// franchises are the film series Browse offers (TMDB collection ids), in
// the order shown.
var franchises = []int{
	10, 1241, 119, 86311, 87359, 645, 328, 9485, 10194, 295, 84, 404609,
	131635, 2344, 264, 263, 2150, 86066, 468222, 8091,
}

type browseService struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	Logo string `json:"logo,omitempty"`
}

type browseFranchise struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Poster string `json:"poster,omitempty"`
}

type browseRow struct {
	Key   string      `json:"key"`
	Title string      `json:"title"`
	Items []watchCard `json:"items"`
}

// GET /api/watch/browse: the services, the collections in season, and the
// film series.
func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	out := struct {
		Services    []browseService    `json:"services"`
		Collections []browseCollection `json:"collections"`
		Franchises  []browseFranchise  `json:"franchises"`
	}{Services: []browseService{}, Collections: []browseCollection{}, Franchises: []browseFranchise{}}
	now := time.Now()
	for _, c := range browseCollections {
		if inSeason(c.season, now) {
			out.Collections = append(out.Collections, c)
		}
	}
	tm := s.TMDB()
	var logos map[int]string
	if tm != nil && tm.HasAPIKey() {
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()
		logos, _ = tm.ProviderLogos(ctx, "US")
		found := make([]browseFranchise, len(franchises))
		var wg sync.WaitGroup
		for i, id := range franchises {
			wg.Go(func() {
				if col, err := tm.GetCollection(ctx, id); err == nil {
					found[i] = browseFranchise{ID: col.ID, Name: franchiseName(col.Name), Poster: metadata.PosterURL(col.PosterPath)}
				}
			})
		}
		wg.Wait()
		for _, f := range found {
			if f.ID != 0 && f.Poster != "" {
				out.Franchises = append(out.Franchises, f)
			}
		}
	}
	for _, svc := range topServices {
		out.Services = append(out.Services, browseService{Key: svc.Key, Name: svc.Name, Logo: serviceLogo(logos, svc.providers)})
	}
	writeJSON(w, http.StatusOK, out)
}

// franchiseName drops TMDB's " Collection": "Star Wars Collection" is
// "Star Wars".
func franchiseName(n string) string {
	const suffix = " Collection"
	if len(n) > len(suffix) && n[len(n)-len(suffix):] == suffix {
		return n[:len(n)-len(suffix)]
	}
	return n
}

// GET /api/watch/browse/service/{key}: a service's page, as rows.
func (s *Server) handleBrowseService(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	var svc *topService
	for i := range topServices {
		if topServices[i].Key == key {
			svc = &topServices[i]
		}
	}
	if svc == nil {
		writeError(w, http.StatusNotFound, "That isn't a streaming service Cue knows.")
		return
	}
	tm := s.TMDB()
	out := struct {
		browseService
		Rows []browseRow `json:"rows"`
	}{browseService: browseService{Key: svc.Key, Name: svc.Name}, Rows: []browseRow{}}
	if tm == nil || !tm.HasAPIKey() {
		writeJSON(w, http.StatusOK, out)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	if logos, err := tm.ProviderLogos(ctx, "US"); err == nil {
		out.Logo = serviceLogo(logos, svc.providers)
	}
	on := func(q metadata.TopQuery) *metadata.TopQuery {
		q.Providers = svc.providers
		return &q
	}
	type spec struct {
		key, title string
		q          browseQuery
		max        int
	}
	since := time.Now().AddDate(0, -4, 0).Format("2006-01-02")
	specs := []spec{
		{"top", "Top 10 on " + svc.Name + " today", browseQuery{on(metadata.TopQuery{}), on(metadata.TopQuery{})}, 10},
		{"new", "New on " + svc.Name, browseQuery{on(metadata.TopQuery{Since: since}), on(metadata.TopQuery{Since: since})}, 20},
	}
	if n := serviceNetworks[svc.Key]; n != "" {
		specs = append(specs, spec{"originals", svc.Name + " originals", browseQuery{tv: on(metadata.TopQuery{Networks: n})}, 20})
	}
	specs = append(specs, spec{"best", "Best rated on " + svc.Name, browseQuery{on(metadata.TopQuery{Sort: "rating", MinVotes: 2000}), on(metadata.TopQuery{Sort: "rating", MinVotes: 1000, Without: "16"})}, 20})
	for _, g := range []string{"Action", "Comedy", "Drama", "Sci-Fi", "Crime", "Kids", "Documentary", "Romance", "Thriller"} {
		ids := topGenres[g]
		var q browseQuery
		if ids[0] > 0 {
			q.movie = on(metadata.TopQuery{Genre: ids[0]})
		}
		if ids[1] > 0 {
			q.tv = on(metadata.TopQuery{Genre: ids[1]})
		}
		specs = append(specs, spec{"genre-" + g, genreRowTitle(g), q, 20})
	}
	rows := make([]browseRow, len(specs))
	var wg sync.WaitGroup
	for i, sp := range specs {
		wg.Go(func() {
			items := s.browseItems(ctx, r, sp.q)
			if len(items) > sp.max {
				items = items[:sp.max]
			}
			rows[i] = browseRow{Key: sp.key, Title: sp.title, Items: items}
		})
	}
	wg.Wait()
	for _, row := range rows {
		if len(row.Items) >= 4 {
			out.Rows = append(out.Rows, row)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// genreRowTitle is a genre row's heading on a service's page.
func genreRowTitle(g string) string {
	switch g {
	case "Kids":
		return "Kids & family"
	case "Documentary":
		return "Documentaries"
	case "Sci-Fi":
		return "Sci-fi & fantasy"
	}
	return g
}

// GET /api/watch/browse/collection/{key}: one collection across services.
func (s *Server) handleBrowseCollection(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	for _, c := range browseCollections {
		if c.Key != key {
			continue
		}
		out := struct {
			Key   string      `json:"key"`
			Title string      `json:"title"`
			Blurb string      `json:"blurb"`
			Items []watchCard `json:"items"`
		}{c.Key, c.Title, c.Blurb, []watchCard{}}
		if tm := s.TMDB(); tm != nil && tm.HasAPIKey() {
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()
			out.Items = s.browseItems(ctx, r, c.query())
		}
		writeJSON(w, http.StatusOK, out)
		return
	}
	writeError(w, http.StatusNotFound, "There's no such collection.")
}

// GET /api/watch/browse/franchise/{id}: a film series, its films in the
// order they came out.
func (s *Server) handleBrowseFranchise(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "That isn't a film series.")
		return
	}
	tm := s.TMDB()
	if tm == nil || !tm.HasAPIKey() {
		writeError(w, http.StatusServiceUnavailable, "Movie info isn't set up.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	col, err := tm.GetCollection(ctx, id)
	if err != nil {
		writeError(w, http.StatusBadGateway, "Couldn't load that film series.")
		return
	}
	parts := append([]metadata.Movie(nil), col.Parts...)
	// In release order; films not out yet go last.
	sortByRelease(parts)
	items := movieCards(parts)
	if prof, ok := s.activeProfile(r); ok && prof.MaxRating != "" {
		items = s.ageFiltered(ctx, prof, items)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": col.ID, "name": franchiseName(col.Name), "overview": col.Overview,
		"poster": metadata.PosterURL(col.PosterPath), "backdrop": metadata.BackdropURL(col.BackdropPath),
		"items": nonNilCards(items),
	})
}

func nonNilCards(c []watchCard) []watchCard {
	if c == nil {
		return []watchCard{}
	}
	return c
}

// sortByRelease orders films by release date, undated last.
func sortByRelease(ms []metadata.Movie) {
	for i := 1; i < len(ms); i++ {
		for j := i; j > 0 && releaseBefore(ms[j], ms[j-1]); j-- {
			ms[j], ms[j-1] = ms[j-1], ms[j]
		}
	}
}

func releaseBefore(a, b metadata.Movie) bool {
	if a.ReleaseDate == "" {
		return false
	}
	if b.ReleaseDate == "" {
		return true
	}
	return a.ReleaseDate < b.ReleaseDate
}

// browseItems asks TMDB for a row's films and shows and mixes them, a film
// then a show (each in TMDB's order), keeping what the profile may watch.
func (s *Server) browseItems(ctx context.Context, r *http.Request, q browseQuery) []watchCard {
	tm := s.TMDB()
	var films, shows []watchCard
	var wg sync.WaitGroup
	if q.movie != nil {
		wg.Go(func() {
			if p, err := tm.TopMovies(ctx, *q.movie); err == nil {
				films = movieCards(p.Results)
			}
		})
	}
	if q.tv != nil {
		wg.Go(func() {
			if p, err := tm.TopShows(ctx, *q.tv); err == nil {
				shows = showCards(p.Results)
			}
		})
	}
	wg.Wait()
	items := make([]watchCard, 0, len(films)+len(shows))
	for i := 0; i < len(films) || i < len(shows); i++ {
		if i < len(films) {
			items = append(items, films[i])
		}
		if i < len(shows) {
			items = append(items, shows[i])
		}
	}
	if prof, ok := s.activeProfile(r); ok && prof.MaxRating != "" {
		items = s.ageFiltered(ctx, prof, items)
	}
	return nonNilCards(items)
}
