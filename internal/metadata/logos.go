package metadata

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// imageList is the part of TMDB's /images answer the logo lookup reads.
type imageList struct {
	Logos []struct {
		FilePath string `json:"file_path"`
		Lang     string `json:"iso_639_1"`
	} `json:"logos"`
}

// Logo is a movie's or show's logo artwork (its name as drawn on the
// poster), as a PNG address, or "" when TMDB has none. kind is "movie" or
// "tv". An English logo comes first, then one without words; PNGs are
// preferred over SVGs, which some TV image loaders can't draw. TMDB lists
// them best-voted first.
func (c *Client) Logo(ctx context.Context, kind string, tmdbID int) (string, error) {
	if kind != "movie" && kind != "tv" {
		return "", fmt.Errorf("logo: unknown kind %q", kind)
	}
	key := fmt.Sprintf("logo:%s:%d", kind, tmdbID)
	if v, ok := c.logos.get(key); ok {
		return v.(string), nil
	}
	var r imageList
	q := url.Values{"include_image_language": {"en,null"}}
	if err := c.get(ctx, fmt.Sprintf("/%s/%d/images", kind, tmdbID), q, &r); err != nil {
		return "", err
	}
	u := ""
	if p := pickLogo(r); p != "" {
		u = "https://image.tmdb.org/t/p/w500" + p
	}
	c.logos.put(key, u)
	return u, nil
}

func pickLogo(r imageList) string {
	png := func(p string) bool { return strings.HasSuffix(strings.ToLower(p), ".png") }
	for _, want := range []func(lang, path string) bool{
		func(l, p string) bool { return l == "en" && png(p) },
		func(l, p string) bool { return l == "" && png(p) },
		func(l, p string) bool { return png(p) },
	} {
		for _, l := range r.Logos {
			if l.FilePath != "" && want(l.Lang, l.FilePath) {
				return l.FilePath
			}
		}
	}
	return ""
}
