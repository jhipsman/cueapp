package metadata

import (
	"context"
	"fmt"
	"net/url"
	"sort"
)

// CollectionRef is the series a film belongs to (TMDB's
// belongs_to_collection on a film's details).
type CollectionRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Collection is a series of films (GET /collection/{id}), its parts in
// release order, the ones without a date (announced) last.
type Collection struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	Overview     string  `json:"overview"`
	PosterPath   string  `json:"poster_path"`
	BackdropPath string  `json:"backdrop_path"`
	Parts        []Movie `json:"parts"`
}

// GetCollection fetches a film series (kept a few minutes, like details).
func (c *Client) GetCollection(ctx context.Context, id int) (*Collection, error) {
	key := fmt.Sprintf("collection:%d", id)
	if v, ok := c.pages.get(key); ok {
		return v.(*Collection), nil
	}
	var col Collection
	if err := c.get(ctx, fmt.Sprintf("/collection/%d", id), url.Values{"language": {"en-US"}}, &col); err != nil {
		return nil, err
	}
	sort.SliceStable(col.Parts, func(i, j int) bool {
		a, b := col.Parts[i].ReleaseDate, col.Parts[j].ReleaseDate
		if a == "" || b == "" {
			return a != "" && b == ""
		}
		return a < b
	})
	c.pages.put(key, &col)
	return &col, nil
}
