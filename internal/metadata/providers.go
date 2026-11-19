package metadata

import (
	"context"
	"fmt"
	"net/url"
)

// Provider is a streaming service a title is on.
type Provider struct {
	ID       int    `json:"provider_id"`
	Name     string `json:"provider_name"`
	LogoPath string `json:"logo_path"`
	Priority int    `json:"display_priority"`
}

// WatchProviders is where a title streams in one country (TMDB's data, from
// JustWatch).
type WatchProviders struct {
	Link         string     `json:"link"` // TMDB's page of where to watch it
	Subscription []Provider `json:"flatrate"`
	Free         []Provider `json:"free"`
	Ads          []Provider `json:"ads"`
}

// GetWatchProviders is where a movie ("movie") or show ("tv") streams in
// region (like "US"); none when TMDB doesn't know.
func (c *Client) GetWatchProviders(ctx context.Context, kind string, tmdbID int, region string) (WatchProviders, error) {
	if kind != "movie" && kind != "tv" {
		return WatchProviders{}, fmt.Errorf("unknown kind %q", kind)
	}
	var out struct {
		Results map[string]WatchProviders `json:"results"`
	}
	if err := c.get(ctx, fmt.Sprintf("/%s/%d/watch/providers", kind, tmdbID), nil, &out); err != nil {
		return WatchProviders{}, err
	}
	return out.Results[region], nil
}

// LogoURL builds a provider logo's address.
func LogoURL(logoPath string) string {
	if logoPath == "" {
		return ""
	}
	return "https://image.tmdb.org/t/p/w92" + logoPath
}

// ProviderLogos is every streaming service TMDB knows in a region, by its
// id: the logo's address (for the services' tabs and buttons).
func (c *Client) ProviderLogos(ctx context.Context, region string) (map[int]string, error) {
	key := "provider-logos:" + region
	if v, ok := c.pages.get(key); ok {
		return v.(map[int]string), nil
	}
	out := map[int]string{}
	for _, kind := range []string{"movie", "tv"} {
		var res struct {
			Results []Provider `json:"results"`
		}
		if err := c.get(ctx, "/watch/providers/"+kind, url.Values{"watch_region": {region}, "language": {"en-US"}}, &res); err != nil {
			return nil, err
		}
		for _, p := range res.Results {
			if p.LogoPath != "" && out[p.ID] == "" {
				out[p.ID] = LogoURL(p.LogoPath)
			}
		}
	}
	c.pages.put(key, out)
	return out, nil
}
