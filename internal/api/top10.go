package api

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

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
	Logo  string      `json:"logo,omitempty"` // the service's logo (TMDB's)
	Items []watchCard `json:"items"`
	// The ten films and the ten shows on their own (Home's top10 only).
	Movies []watchCard `json:"movies,omitempty"`
	Shows  []watchCard `json:"shows,omitempty"`
}

// serviceLogo is a service's logo (its first TMDB provider id's), or "".
func serviceLogo(logos map[int]string, providers string) string {
	id, _ := strconv.Atoi(strings.SplitN(providers, "|", 2)[0])
	return logos[id]
}

type scored struct {
	card watchCard
	pop  float64
}

// top10 is each service's ten most popular titles right now (services with
// nothing found are left out).
func (s *Server) top10(ctx context.Context, tm *metadata.Client) []topList {
	out := make([]topList, len(topServices))
	logos, _ := tm.ProviderLogos(ctx, "US")
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
			// The ten across films and shows, and the ten of each (for the
			// Films and Series homes, so filtering doesn't leave two).
			var items, movies, shows []watchCard
			for _, x := range all {
				if x.card.PosterURL == "" {
					continue
				}
				if len(items) < 10 {
					items = append(items, x.card)
				}
				if x.card.Kind == "movie" && len(movies) < 10 {
					movies = append(movies, x.card)
				}
				if x.card.Kind == "tv" && len(shows) < 10 {
					shows = append(shows, x.card)
				}
			}
			out[i] = topList{Key: svc.Key, Name: svc.Name, Logo: serviceLogo(logos, svc.providers), Items: items, Movies: movies, Shows: shows}
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

// ---- The Top 10 page: any service, country, genre or decade.

// topGenres maps the genre names Watch shows to TMDB's ids for films and
// shows (0: none of that kind).
var topGenres = map[string][2]int{
	"Action": {28, 10759}, "Comedy": {35, 35}, "Drama": {18, 18}, "Sci-Fi": {878, 10765}, "Family": {10751, 10751},
	"Animation": {16, 16}, "Crime": {80, 80}, "Mystery": {9648, 9648}, "Thriller": {53, 0}, "Romance": {10749, 0},
	"Documentary": {99, 99}, "Reality": {0, 10764}, "Kids": {10751, 10762}, "Fantasy": {14, 10765},
	"History & War": {10752, 10768}, "Western": {37, 37},
}

// topCountries are the countries the page offers (where TMDB knows what's
// streaming), by code.
var topCountries = map[string]string{
	"US": "United States", "GB": "United Kingdom", "CA": "Canada", "AU": "Australia", "IE": "Ireland",
	"DE": "Germany", "FR": "France", "ES": "Spain", "IT": "Italy", "NL": "Netherlands", "BR": "Brazil",
	"MX": "Mexico", "IN": "India", "JP": "Japan", "KR": "South Korea",
}

type topPage struct {
	Title     string            `json:"title"`
	Items     []watchCard       `json:"items"`
	Services  []topList         `json:"services"` // the choices: key and name (no items)
	Genres    []string          `json:"genres"`
	Countries map[string]string `json:"countries"`
}

// GET /api/watch/top?kind=all|movie|tv&service=netflix&country=US&genre=Comedy&decade=1990
func (s *Server) handleWatchTop(w http.ResponseWriter, r *http.Request) {
	qv := r.URL.Query()
	kind := qv.Get("kind")
	if kind != "movie" && kind != "tv" {
		kind = "all"
	}
	country := strings.ToUpper(qv.Get("country"))
	if _, ok := topCountries[country]; !ok {
		country = "US"
	}
	q := metadata.TopQuery{Region: country}
	title := "Top 10"
	var svcName string
	if key := qv.Get("service"); key != "" {
		for _, svc := range topServices {
			if svc.Key == key {
				q.Providers, svcName = svc.providers, svc.Name
			}
		}
	}
	genre := qv.Get("genre")
	ids, hasGenre := topGenres[genre]
	decade, _ := strconv.Atoi(qv.Get("decade"))
	if decade < 1920 || decade > time.Now().Year() || decade%10 != 0 {
		decade = 0
	}
	q.Decade = decade
	// "Top 10 Comedy films of the 1990s on Netflix in Canada"
	what := map[string]string{"all": "titles", "movie": "films", "tv": "shows"}[kind]
	if hasGenre {
		what = genre + " " + what
	}
	title += " " + what
	if decade > 0 {
		title += fmt.Sprintf(" of the %ds", decade)
	}
	if svcName != "" {
		title += " on " + svcName
	}
	if country != "US" {
		title += " in " + topCountries[country]
	}
	var logos map[int]string
	if tm := s.TMDB(); tm != nil && tm.HasAPIKey() {
		lctx, lcancel := context.WithTimeout(r.Context(), 5*time.Second)
		logos, _ = tm.ProviderLogos(lctx, "US")
		lcancel()
	}
	page := topPage{Title: title, Items: []watchCard{}, Countries: topCountries}
	for _, svc := range topServices {
		page.Services = append(page.Services, topList{Key: svc.Key, Name: svc.Name, Logo: serviceLogo(logos, svc.providers)})
	}
	for g := range topGenres {
		page.Genres = append(page.Genres, g)
	}
	sort.Strings(page.Genres)
	tm := s.TMDB()
	if tm == nil || !tm.HasAPIKey() {
		writeJSON(w, http.StatusOK, page)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	var all []scored
	if kind != "tv" && (!hasGenre || ids[0] > 0) {
		mq := q
		if hasGenre {
			mq.Genre = ids[0]
		}
		if p, err := tm.TopMovies(ctx, mq); err == nil {
			for i, m := range p.Results {
				all = append(all, scored{movieCard(m), rankScore(i, m.Popularity, decade)})
			}
		}
	}
	if kind != "movie" && (!hasGenre || ids[1] > 0) {
		sq := q
		if hasGenre {
			sq.Genre = ids[1]
		}
		if p, err := tm.TopShows(ctx, sq); err == nil {
			for i, sh := range p.Results {
				all = append(all, scored{showCard(sh), rankScore(i, sh.Popularity, decade)})
			}
		}
	}
	sort.SliceStable(all, func(a, b int) bool { return all[a].pop > all[b].pop })
	var items []watchCard
	for _, x := range all {
		if x.card.PosterURL != "" {
			items = append(items, x.card)
		}
	}
	if prof, ok := s.activeProfile(r); ok && prof.MaxRating != "" {
		items = s.ageFiltered(ctx, prof, items)
	}
	if len(items) > 10 {
		items = items[:10]
	}
	if items != nil {
		page.Items = items
	}
	writeJSON(w, http.StatusOK, page)
}

// rankScore orders films and shows together: by popularity now, or for a
// past decade (sorted by votes, whose numbers don't compare across films
// and shows) by place in each list.
func rankScore(place int, pop float64, decade int) float64 {
	if decade > 0 {
		return float64(1000 - place)
	}
	return pop
}
