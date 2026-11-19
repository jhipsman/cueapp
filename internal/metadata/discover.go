package metadata

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// Browse sort orders accepted by DiscoverQuery.Sort.
const (
	SortPopular = "popular" // most popular first (the default)
	SortRating  = "rating"  // best rated first, among titles with enough votes
	SortNewest  = "newest"  // latest release or first-air date first
	SortOldest  = "oldest"
)

// minVotesForRating keeps a handful of five-star votes from putting an
// unknown title at the top of a "best rated" list.
const minVotesForRating = 200

// maxDiscoverPages is the deepest page TMDB's discover endpoints serve.
const maxDiscoverPages = 500

// DiscoverQuery filters a browse of TMDB's catalogue. Zero values mean "any".
// Year is an exact year and wins over the YearFrom/YearTo range.
type DiscoverQuery struct {
	Genre    int
	Year     int
	YearFrom int
	YearTo   int
	Sort     string
	Page     int
}

// ValidSort reports whether sort is empty or one of the Sort* constants.
func ValidSort(sort string) bool {
	switch sort {
	case "", SortPopular, SortRating, SortNewest, SortOldest:
		return true
	}
	return false
}

// MoviePage is one page of a movie browse or list. TotalPages is capped at
// the deepest page TMDB serves, and TotalResults at what those pages hold.
type MoviePage struct {
	Page         int
	TotalPages   int
	TotalResults int
	Results      []Movie
}

// ShowPage is one page of a TV browse or list.
type ShowPage struct {
	Page         int
	TotalPages   int
	TotalResults int
	Results      []Show
}

// params builds TMDB's discover query string. dateField is the date filter's
// name ("primary_release_date" for movies, "first_air_date" for TV) and
// yearParam the exact-year filter ("primary_release_year" or
// "first_air_date_year").
func (q DiscoverQuery) params(dateField, yearParam string) url.Values {
	v := url.Values{"include_adult": {"false"}, "language": {"en-US"}}
	if q.Genre > 0 {
		v.Set("with_genres", strconv.Itoa(q.Genre))
	}
	switch {
	case q.Year > 0:
		v.Set(yearParam, strconv.Itoa(q.Year))
	default:
		if q.YearFrom > 0 {
			v.Set(dateField+".gte", fmt.Sprintf("%04d-01-01", q.YearFrom))
		}
		if q.YearTo > 0 {
			v.Set(dateField+".lte", fmt.Sprintf("%04d-12-31", q.YearTo))
		}
	}
	switch q.Sort {
	case SortRating:
		v.Set("sort_by", "vote_average.desc")
		v.Set("vote_count.gte", strconv.Itoa(minVotesForRating))
	case SortNewest:
		v.Set("sort_by", dateField+".desc")
	case SortOldest:
		v.Set("sort_by", dateField+".asc")
	default:
		v.Set("sort_by", "popularity.desc")
	}
	v.Set("page", strconv.Itoa(ClampPage(q.Page)))
	return v
}

// ClampPage keeps a requested page number between 1 and the deepest page
// TMDB serves.
func ClampPage(page int) int {
	if page < 1 {
		return 1
	}
	if page > maxDiscoverPages {
		return maxDiscoverPages
	}
	return page
}

// pageMeta is the paging part of every TMDB list response.
type pageMeta struct {
	Page         int `json:"page"`
	TotalPages   int `json:"total_pages"`
	TotalResults int `json:"total_results"`
}

// clamped caps the page count at maxDiscoverPages, and the result count at
// what those pages can hold, since TMDB refuses anything deeper.
func (p pageMeta) clamped() pageMeta {
	if p.TotalPages > maxDiscoverPages {
		p.TotalPages = maxDiscoverPages
		if limit := maxDiscoverPages * tmdbPageSize; p.TotalResults > limit {
			p.TotalResults = limit
		}
	}
	return p
}

// tmdbPageSize is how many results TMDB puts on each list page.
const tmdbPageSize = 20

// moviePage fetches one page of any TMDB movie list, through the page cache.
func (c *Client) moviePage(ctx context.Context, path string, params url.Values) (*MoviePage, error) {
	key := path + "?" + params.Encode()
	if v, ok := c.pages.get(key); ok {
		return v.(*MoviePage), nil
	}
	var resp struct {
		pageMeta
		Results []Movie `json:"results"`
	}
	if err := c.get(ctx, path, params, &resp); err != nil {
		return nil, err
	}
	m := resp.pageMeta.clamped()
	out := &MoviePage{Page: m.Page, TotalPages: m.TotalPages, TotalResults: m.TotalResults, Results: resp.Results}
	if out.Results == nil {
		out.Results = []Movie{}
	}
	c.pages.put(key, out)
	return out, nil
}

// showPage is moviePage for shows.
func (c *Client) showPage(ctx context.Context, path string, params url.Values) (*ShowPage, error) {
	key := path + "?" + params.Encode()
	if v, ok := c.pages.get(key); ok {
		return v.(*ShowPage), nil
	}
	var resp struct {
		pageMeta
		Results []Show `json:"results"`
	}
	if err := c.get(ctx, path, params, &resp); err != nil {
		return nil, err
	}
	m := resp.pageMeta.clamped()
	out := &ShowPage{Page: m.Page, TotalPages: m.TotalPages, TotalResults: m.TotalResults, Results: resp.Results}
	if out.Results == nil {
		out.Results = []Show{}
	}
	c.pages.put(key, out)
	return out, nil
}

// DiscoverMovies browses TMDB's movies by genre, year and sort order.
func (c *Client) DiscoverMovies(ctx context.Context, q DiscoverQuery) (*MoviePage, error) {
	return c.moviePage(ctx, "/discover/movie", q.params("primary_release_date", "primary_release_year"))
}

// DiscoverTV is DiscoverMovies for shows.
func (c *Client) DiscoverTV(ctx context.Context, q DiscoverQuery) (*ShowPage, error) {
	return c.showPage(ctx, "/discover/tv", q.params("first_air_date", "first_air_date_year"))
}

// Discover list names accepted by MovieList and TVList.
const (
	ListTrending = "trending" // TMDB's trending this week
	ListPopular  = "popular"  // TMDB's popular list
	ListUpcoming = "upcoming" // not released (or first aired) yet, most popular first
)

// ValidList reports whether list is one of the List* names.
func ValidList(list string) bool {
	switch list {
	case ListTrending, ListPopular, ListUpcoming:
		return true
	}
	return false
}

// today is the date "upcoming" lists start from, as TMDB's YYYY-MM-DD.
func (c *Client) today() string {
	now := time.Now
	if c.now != nil {
		now = c.now
	}
	return now().UTC().Format("2006-01-02")
}

// upcomingParams asks a discover endpoint for titles dated today or later,
// most popular first. dateField is "primary_release_date" or
// "first_air_date"; primary release dates are worldwide, so the list does
// not depend on a region.
func (c *Client) upcomingParams(dateField string, page int) url.Values {
	return url.Values{
		"include_adult": {"false"}, "language": {"en-US"},
		dateField + ".gte": {c.today()},
		"sort_by":          {"popularity.desc"},
		"page":             {strconv.Itoa(ClampPage(page))},
	}
}

// MovieList fetches one page (TMDB serves 20 per page, up to page 500) of
// the trending, popular or upcoming movies.
func (c *Client) MovieList(ctx context.Context, list string, page int) (*MoviePage, error) {
	paged := url.Values{"page": {strconv.Itoa(ClampPage(page))}}
	switch list {
	case ListTrending:
		return c.moviePage(ctx, "/trending/movie/week", paged)
	case ListPopular:
		return c.moviePage(ctx, "/movie/popular", paged)
	case ListUpcoming:
		return c.moviePage(ctx, "/discover/movie", c.upcomingParams("primary_release_date", page))
	}
	return nil, fmt.Errorf("unknown discover list %q", list)
}

// TVList is MovieList for shows; "upcoming" means shows whose first episode
// has not aired yet.
func (c *Client) TVList(ctx context.Context, list string, page int) (*ShowPage, error) {
	paged := url.Values{"page": {strconv.Itoa(ClampPage(page))}}
	switch list {
	case ListTrending:
		return c.showPage(ctx, "/trending/tv/week", paged)
	case ListPopular:
		return c.showPage(ctx, "/tv/popular", paged)
	case ListUpcoming:
		return c.showPage(ctx, "/discover/tv", c.upcomingParams("first_air_date", page))
	}
	return nil, fmt.Errorf("unknown discover list %q", list)
}

// OnService is what's most popular right now on a streaming service (its
// TMDB provider id, or several joined with "|"), in a region ("US"): one
// page of films or shows included with a subscription.
func (c *Client) OnServiceMovies(ctx context.Context, providers, region string) (*MoviePage, error) {
	return c.moviePage(ctx, "/discover/movie", onServiceParams(providers, region))
}

// OnServiceShows is OnServiceMovies for shows.
func (c *Client) OnServiceShows(ctx context.Context, providers, region string) (*ShowPage, error) {
	return c.showPage(ctx, "/discover/tv", onServiceParams(providers, region))
}

func onServiceParams(providers, region string) url.Values {
	return url.Values{
		"include_adult": {"false"}, "language": {"en-US"},
		"with_watch_providers":          {providers},
		"watch_region":                  {region},
		"with_watch_monetization_types": {"flatrate"},
		"sort_by":                       {"popularity.desc"},
		"vote_count.gte":                {"20"},
		"page":                          {"1"},
	}
}

// TopQuery narrows a Top 10: on a streaming service (TMDB provider ids,
// "|" for several; "" for any), streaming in a country ("US"), of a genre
// (TMDB's id for films or shows), from a decade (1990 for the 1990s; 0 for
// any). The most popular first; for a past decade, the most voted for (the
// classics, not what happens to be trending).
type TopQuery struct {
	Providers string
	Region    string
	Genre     int
	Decade    int
	// MaxRuntime keeps films (or episodes) at most this many minutes; 0 for any.
	MaxRuntime int
	// MinRating keeps titles TMDB's users rate at least this (out of 10).
	MinRating float64
	// Sort: "" (most popular), "rating" (best rated, with enough votes)
	// or "votes" (most voted for: the well known).
	Sort string
	// Networks keeps shows first shown on these TMDB networks ("213" for
	// Netflix; "|" for several): a service's originals. Films ignore it.
	Networks string
	// Keywords keeps titles tagged with these TMDB keywords ("|" for any).
	Keywords string
	// Genres keeps titles of any of these TMDB genres ("16|10751"); with
	// Genre, both apply.
	Genres string
	// MaxCertification keeps films rated at most this in the US ("PG").
	// Shows ignore it.
	MaxCertification string
	// MinVotes / MaxVotes bound how many people have rated a title (0: the
	// usual floor / no ceiling). A ceiling finds the lesser known.
	MinVotes, MaxVotes int
	// Since keeps titles first out on or after this day ("2026-08-01").
	Since string
	// Without leaves out titles of these TMDB genres ("16|99").
	Without string
	// Until keeps titles first out on or before this day.
	Until string
}

func (q TopQuery) params(dateField string) url.Values {
	v := url.Values{
		"include_adult": {"false"}, "language": {"en-US"},
		"sort_by": {"popularity.desc"}, "vote_count.gte": {"40"}, "page": {"1"},
	}
	region := q.Region
	if region == "" {
		region = "US"
	}
	v.Set("watch_region", region)
	v.Set("with_watch_monetization_types", "flatrate|free|ads")
	if q.Providers != "" {
		v.Set("with_watch_providers", q.Providers)
	}
	if q.Genre > 0 {
		v.Set("with_genres", fmt.Sprint(q.Genre))
	}
	if q.MaxRuntime > 0 {
		v.Set("with_runtime.lte", fmt.Sprint(q.MaxRuntime))
		v.Set("with_runtime.gte", "20")
	}
	if q.MinRating > 0 {
		v.Set("vote_average.gte", fmt.Sprintf("%.1f", q.MinRating))
		v.Set("vote_count.gte", "200")
	}
	if q.Decade >= 1900 {
		v.Set(dateField+".gte", fmt.Sprintf("%d-01-01", q.Decade))
		v.Set(dateField+".lte", fmt.Sprintf("%d-12-31", q.Decade+9))
		v.Set("sort_by", "vote_count.desc")
	}
	switch q.Sort {
	case "rating":
		v.Set("sort_by", "vote_average.desc")
		if v.Get("vote_count.gte") == "40" {
			v.Set("vote_count.gte", "300")
		}
	case "votes":
		v.Set("sort_by", "vote_count.desc")
	}
	if q.Networks != "" && dateField == "first_air_date" {
		v.Set("with_networks", q.Networks)
	}
	if q.Keywords != "" {
		v.Set("with_keywords", q.Keywords)
	}
	if q.Genres != "" {
		if g := v.Get("with_genres"); g != "" {
			v.Set("with_genres", g+","+q.Genres)
		} else {
			v.Set("with_genres", q.Genres)
		}
	}
	if q.MaxCertification != "" && dateField == "primary_release_date" {
		v.Set("certification_country", "US")
		v.Set("certification.lte", q.MaxCertification)
	}
	if q.MinVotes > 0 {
		v.Set("vote_count.gte", fmt.Sprint(q.MinVotes))
	}
	if q.MaxVotes > 0 {
		v.Set("vote_count.lte", fmt.Sprint(q.MaxVotes))
	}
	if q.Since != "" {
		v.Set(dateField+".gte", q.Since)
	}
	if q.Until != "" {
		v.Set(dateField+".lte", q.Until)
	}
	if q.Without != "" {
		v.Set("without_genres", q.Without)
	}
	return v
}

// TopMovies is one page of films matching q.
func (c *Client) TopMovies(ctx context.Context, q TopQuery) (*MoviePage, error) {
	return c.moviePage(ctx, "/discover/movie", q.params("primary_release_date"))
}

// TopShows is one page of shows matching q.
func (c *Client) TopShows(ctx context.Context, q TopQuery) (*ShowPage, error) {
	return c.showPage(ctx, "/discover/tv", q.params("first_air_date"))
}
