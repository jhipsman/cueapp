// Package introdb reads TheIntroDB (theintrodb.org): when a TV episode's or
// film's intro, recap and credits start and end, submitted and checked by
// its users. Reading needs no key.
package introdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DefaultBase is the API's address (tests point it elsewhere).
var DefaultBase = "https://api.theintrodb.org/v3"

// Segment is a part of the video, in seconds. Start -1 means from the very
// beginning; End -1 means to the very end.
type Segment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// Segments is what TheIntroDB has for one video; nil fields when nothing.
type Segments struct {
	Intro   *Segment
	Recap   *Segment
	Credits *Segment
}

// Client reads TheIntroDB, keeping answers for a day.
type Client struct {
	base string
	hc   *http.Client

	mu    sync.Mutex
	cache map[string]cached
}

type cached struct {
	at  time.Time
	seg Segments
}

// New returns a client; base is "" for DefaultBase.
func New(base string) *Client {
	if base == "" {
		base = DefaultBase
	}
	return &Client{base: strings.TrimRight(base, "/"), hc: &http.Client{Timeout: 8 * time.Second}, cache: map[string]cached{}}
}

// Get is what TheIntroDB has for a movie (season 0) or an episode. duration
// (seconds; 0 if not known) picks times for that cut of the episode: times
// for another cut (one with a "Previously on", or from another service)
// can be minutes off.
func (c *Client) Get(ctx context.Context, tmdbID, season, episode int, duration float64) (Segments, error) {
	return c.get(ctx, tmdbID, season, episode, duration)
}

func (c *Client) get(ctx context.Context, tmdbID, season, episode int, duration float64) (Segments, error) {
	q := url.Values{"tmdb_id": {strconv.Itoa(tmdbID)}}
	if season > 0 && episode > 0 {
		q.Set("season", strconv.Itoa(season))
		q.Set("episode", strconv.Itoa(episode))
	}
	if duration >= 300 { // TheIntroDB takes 5 minutes and longer
		q.Set("duration_ms", strconv.FormatInt(int64(duration*1000), 10))
	}
	key := q.Encode()
	c.mu.Lock()
	if e, ok := c.cache[key]; ok && time.Since(e.at) < 24*time.Hour {
		c.mu.Unlock()
		return e.seg, nil
	}
	c.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/media?"+key, nil)
	if err != nil {
		return Segments{}, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return Segments{}, fmt.Errorf("couldn't reach TheIntroDB: %w", err)
	}
	defer resp.Body.Close()
	var seg Segments
	switch {
	case resp.StatusCode == http.StatusNotFound:
		// Nobody has submitted times for it.
	case resp.StatusCode != http.StatusOK:
		return Segments{}, fmt.Errorf("TheIntroDB answered with status %d", resp.StatusCode)
	default:
		var raw struct {
			Intro   []rawSegment `json:"intro"`
			Recap   []rawSegment `json:"recap"`
			Credits []rawSegment `json:"credits"`
		}
		if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&raw); err != nil {
			return Segments{}, fmt.Errorf("TheIntroDB's answer: %w", err)
		}
		seg = Segments{Intro: first(raw.Intro), Recap: first(raw.Recap), Credits: first(raw.Credits)}
	}
	c.mu.Lock()
	for k, e := range c.cache {
		if time.Since(e.at) > 24*time.Hour {
			delete(c.cache, k)
		}
	}
	c.cache[key] = cached{at: time.Now(), seg: seg}
	c.mu.Unlock()
	return seg, nil
}

type rawSegment struct {
	StartMs     *int64   `json:"start_ms"`
	EndMs       *int64   `json:"end_ms"`
	Confidence  *float64 `json:"confidence"`
	Submissions *int     `json:"submission_count"`
}

// first is the segment most agreed on: the highest confidence, then the
// most submissions, then the first given.
func first(list []rawSegment) *Segment {
	score := func(r rawSegment) (float64, int) {
		c, n := 0.0, 0
		if r.Confidence != nil {
			c = *r.Confidence
		}
		if r.Submissions != nil {
			n = *r.Submissions
		}
		return c, n
	}
	best := -1
	for i, r := range list {
		if r.StartMs == nil && r.EndMs == nil {
			continue
		}
		if best >= 0 {
			c, n := score(r)
			bc, bn := score(list[best])
			if c < bc || c == bc && n <= bn {
				continue
			}
		}
		best = i
	}
	if best < 0 {
		return nil
	}
	r := list[best]
	s := Segment{Start: -1, End: -1}
	if r.StartMs != nil {
		s.Start = float64(*r.StartMs) / 1000
	}
	if r.EndMs != nil {
		s.End = float64(*r.EndMs) / 1000
	}
	return &s
}
