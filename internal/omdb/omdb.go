// Package omdb reads IMDb and Rotten Tomatoes ratings from OMDb
// (omdbapi.com), keyed by a title's IMDb id. TMDB has no IMDb ratings, so
// Watch asks here when an OMDb key is saved. A free key allows 1,000
// lookups a day; answers are kept for a day so each title costs one.
package omdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// DefaultBase is the OMDb API.
const DefaultBase = "https://www.omdbapi.com/"

// ErrBadKey is returned when OMDb refuses the API key.
var ErrBadKey = errors.New("omdb didn't accept the API key")

// keepFor is how long a title's ratings are kept.
const keepFor = 24 * time.Hour

// Ratings are a title's scores. Empty fields mean OMDb doesn't have them.
type Ratings struct {
	IMDb           string `json:"imdb,omitempty"`           // "8.1" (out of 10)
	IMDbVotes      string `json:"imdbVotes,omitempty"`      // "1,234,567"
	RottenTomatoes string `json:"rottenTomatoes,omitempty"` // "93%"
	Metacritic     string `json:"metacritic,omitempty"`     // "74" (out of 100)
}

type cached struct {
	at time.Time
	r  Ratings
}

// Client is an OMDb client for one key.
type Client struct {
	key  string
	base string
	hc   *http.Client

	mu    sync.Mutex
	cache map[string]cached
}

// New returns a client for apiKey; base "" is the real OMDb.
func New(apiKey, base string) *Client {
	if base == "" {
		base = DefaultBase
	}
	return &Client{key: strings.TrimSpace(apiKey), base: base, hc: &http.Client{Timeout: 10 * time.Second}, cache: map[string]cached{}}
}

// Key is the API key this client uses.
func (c *Client) Key() string { return c.key }

// Ratings looks up a title by its IMDb id ("tt0111161").
func (c *Client) Ratings(ctx context.Context, imdbID string) (Ratings, error) {
	imdbID = strings.TrimSpace(imdbID)
	if !strings.HasPrefix(imdbID, "tt") {
		return Ratings{}, fmt.Errorf("%q isn't an IMDb id", imdbID)
	}
	c.mu.Lock()
	if v, ok := c.cache[imdbID]; ok && time.Since(v.at) < keepFor {
		c.mu.Unlock()
		return v.r, nil
	}
	c.mu.Unlock()

	q := url.Values{"apikey": {c.key}, "i": {imdbID}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"?"+q.Encode(), nil)
	if err != nil {
		return Ratings{}, err
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) { // the URL holds the key
			return Ratings{}, fmt.Errorf("couldn't reach OMDb: %w", ue.Err)
		}
		return Ratings{}, fmt.Errorf("couldn't reach OMDb: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return Ratings{}, ErrBadKey
	}
	var out struct {
		Response   string `json:"Response"`
		Error      string `json:"Error"`
		IMDbRating string `json:"imdbRating"`
		IMDbVotes  string `json:"imdbVotes"`
		Metascore  string `json:"Metascore"`
		Ratings    []struct {
			Source string `json:"Source"`
			Value  string `json:"Value"`
		} `json:"Ratings"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return Ratings{}, fmt.Errorf("couldn't read OMDb's answer: %w", err)
	}
	if out.Response != "True" {
		low := strings.ToLower(out.Error)
		if strings.Contains(low, "api key") {
			return Ratings{}, ErrBadKey
		}
		if strings.Contains(low, "not found") {
			c.keep(imdbID, Ratings{}) // nothing to find: don't ask again today
			return Ratings{}, nil
		}
		return Ratings{}, fmt.Errorf("omdb answered: %s", out.Error)
	}
	r := Ratings{IMDb: known(out.IMDbRating), IMDbVotes: known(out.IMDbVotes), Metacritic: known(out.Metascore)}
	for _, x := range out.Ratings {
		if x.Source == "Rotten Tomatoes" {
			r.RottenTomatoes = known(x.Value)
		}
	}
	c.keep(imdbID, r)
	return r, nil
}

func (c *Client) keep(id string, r Ratings) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.cache) > 5000 { // a day's worth at most; start over rather than grow
		c.cache = map[string]cached{}
	}
	c.cache[id] = cached{at: time.Now(), r: r}
}

// known drops OMDb's "N/A".
func known(v string) string {
	if v == "N/A" {
		return ""
	}
	return v
}
