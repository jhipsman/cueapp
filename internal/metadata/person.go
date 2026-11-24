package metadata

import (
	"context"
	"fmt"
	"net/url"
)

// A person (an actor, a director): who they are and what they've been in,
// for the person pages a cast photo opens.

// Person is one person with their films and shows.
type Person struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Biography   string `json:"biography"`
	Birthday    string `json:"birthday"`
	Deathday    string `json:"deathday"`
	Birthplace  string `json:"place_of_birth"`
	ProfilePath string `json:"profile_path"`
	KnownFor    string `json:"known_for_department"`
	Credits     struct {
		Cast []PersonCredit `json:"cast"`
		Crew []PersonCredit `json:"crew"`
	} `json:"combined_credits"`
}

// PersonCredit is a film or show a person was in (a movie or show's fields,
// with the role).
type PersonCredit struct {
	MediaType        string   `json:"media_type"` // movie, tv
	ID               int      `json:"id"`
	Title            string   `json:"title"` // movies
	Name             string   `json:"name"`  // shows
	Overview         string   `json:"overview"`
	ReleaseDate      string   `json:"release_date"`
	FirstAirDate     string   `json:"first_air_date"`
	PosterPath       string   `json:"poster_path"`
	BackdropPath     string   `json:"backdrop_path"`
	VoteAverage      float64  `json:"vote_average"`
	VoteCount        int      `json:"vote_count"`
	Popularity       float64  `json:"popularity"`
	GenreIDs         []int    `json:"genre_ids"`
	OriginalLanguage string   `json:"original_language"`
	OriginCountry    []string `json:"origin_country"`
	Adult            bool     `json:"adult"`
	Character        string   `json:"character"`
	Job              string   `json:"job"`
	Department       string   `json:"department"` // crew: Directing, Writing…
	EpisodeCount     int      `json:"episode_count"`
}

// Movie is the credit as a movie (MediaType "movie").
func (p PersonCredit) Movie() Movie {
	return Movie{TMDBID: p.ID, Title: p.Title, Overview: p.Overview, ReleaseDate: p.ReleaseDate, PosterPath: p.PosterPath,
		BackdropPath: p.BackdropPath, VoteAverage: p.VoteAverage, VoteCount: p.VoteCount, Popularity: p.Popularity,
		GenreIDs: p.GenreIDs, OriginalLanguage: p.OriginalLanguage, Adult: p.Adult}
}

// Show is the credit as a show (MediaType "tv").
func (p PersonCredit) Show() Show {
	return Show{TMDBID: p.ID, Name: p.Name, Overview: p.Overview, FirstAirDate: p.FirstAirDate, PosterPath: p.PosterPath,
		BackdropPath: p.BackdropPath, VoteAverage: p.VoteAverage, VoteCount: p.VoteCount, Popularity: p.Popularity,
		GenreIDs: p.GenreIDs, OriginalLanguage: p.OriginalLanguage, OriginCountry: p.OriginCountry}
}

// GetPerson is a person with their combined film and TV credits (cached
// like other pages).
func (c *Client) GetPerson(ctx context.Context, id int) (*Person, error) {
	key := fmt.Sprintf("person:%d", id)
	if v, ok := c.pages.get(key); ok {
		return v.(*Person), nil
	}
	var p Person
	if err := c.get(ctx, fmt.Sprintf("/person/%d", id), url.Values{"append_to_response": {"combined_credits"}, "language": {"en-US"}}, &p); err != nil {
		return nil, err
	}
	c.pages.put(key, &p)
	return &p, nil
}

// PersonHit is one person from a name search.
type PersonHit struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Department  string  `json:"known_for_department"` // Acting, Directing, Writing…
	Popularity  float64 `json:"popularity"`
	ProfilePath string  `json:"profile_path"`
}

// SearchPeople finds people by name, the best known first.
func (c *Client) SearchPeople(ctx context.Context, query string) ([]PersonHit, error) {
	var page struct {
		Results []PersonHit `json:"results"`
	}
	if err := c.get(ctx, "/search/person", url.Values{"query": {query}, "include_adult": {"false"}}, &page); err != nil {
		return nil, err
	}
	return page.Results, nil
}

// Keyword is one of TMDB's topic tags ("zombie", "heist", "time travel").
type Keyword struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// SearchKeywords finds topic tags by name.
func (c *Client) SearchKeywords(ctx context.Context, query string) ([]Keyword, error) {
	var page struct {
		Results []Keyword `json:"results"`
	}
	if err := c.get(ctx, "/search/keyword", url.Values{"query": {query}}, &page); err != nil {
		return nil, err
	}
	return page.Results, nil
}
