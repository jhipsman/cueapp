package api

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/jhipsman/cueapp/internal/metadata"
)

// Where else a title streams: TMDB's list of services (from JustWatch), so a
// title page can say "Free on Tubi" with a link that opens it there.

type watchProvider struct {
	Name string `json:"name"`
	Logo string `json:"logo,omitempty"`
	URL  string `json:"url"`
	// Key is Cue's name for the service ("netflix"; "" for one it doesn't
	// know), which the TV app opens in the service's own app.
	Key string `json:"key,omitempty"`
	// Mine: a service this profile has (My services), or a free one.
	Mine bool `json:"mine,omitempty"`
}

// providerKeys are TMDB's provider ids for the services Cue knows by name
// (with their ad tiers and channel versions); the free ones too.
var providerKeys = map[int]string{
	8: "netflix", 1796: "netflix",
	9: "prime", 119: "prime", 2100: "prime",
	337:  "disney",
	1899: "max", 384: "max", 1825: "max",
	15:  "hulu",
	386: "peacock", 387: "peacock",
	531: "paramount", 2303: "paramount", 2616: "paramount", 582: "paramount",
	350: "appletv",
	73:  "tubi", 300: "pluto", 207: "roku", 538: "plex", 613: "freevee",
}

// onMyService says a title streams on one of the services in services.
func (s *Server) onMyService(ctx context.Context, kind string, tmdbID int, services []string) bool {
	if len(services) == 0 || s.TMDB() == nil {
		return false
	}
	wp, err := s.TMDB().GetWatchProviders(ctx, kind, tmdbID, "US")
	if err != nil {
		return false
	}
	for _, p := range wp.Subscription {
		if slices.Contains(services, providerKeys[p.ID]) {
			return true
		}
	}
	return false
}

type watchProvidersAnswer struct {
	Free         []watchProvider `json:"free"`         // free, or free with ads
	Subscription []watchProvider `json:"subscription"` // on a service you pay for
}

var regionCode = regexp.MustCompile(`^[A-Z]{2}$`)

// providerSearch opens a search for the title on services Cue knows; others
// get TMDB's page, which links on to them.
func providerSearch(name, title, fallback string) string {
	q := url.QueryEscape(title)
	p := url.PathEscape(title)
	n := strings.ToLower(name)
	switch {
	case strings.Contains(n, "tubi"):
		return "https://tubitv.com/search/" + p
	case strings.Contains(n, "pluto"):
		return "https://pluto.tv/search/details?query=" + q
	case strings.Contains(n, "roku"):
		return "https://therokuchannel.roku.com/search/" + p
	case strings.Contains(n, "plex"):
		return "https://watch.plex.tv/search?q=" + q
	case strings.Contains(n, "youtube"):
		return "https://www.youtube.com/results?search_query=" + q
	case strings.Contains(n, "freevee"), strings.Contains(n, "amazon"):
		return "https://www.amazon.com/s?i=instant-video&k=" + q
	case strings.Contains(n, "netflix"):
		return "https://www.netflix.com/search?q=" + q
	case strings.Contains(n, "crackle"):
		return "https://www.crackle.com/search?query=" + q
	}
	return fallback
}

// GET /api/watch/providers/{kind}/{tmdbId}?region=US
func (s *Server) handleWatchProviders(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	tmdbID, err := strconv.Atoi(r.PathValue("tmdbId"))
	if err != nil || tmdbID <= 0 || kind != "movie" && kind != "tv" {
		writeError(w, http.StatusBadRequest, "That isn't a valid title.")
		return
	}
	region := strings.ToUpper(r.URL.Query().Get("region"))
	if !regionCode.MatchString(region) {
		region = "US"
	}
	out := watchProvidersAnswer{Free: []watchProvider{}, Subscription: []watchProvider{}}
	tm := s.TMDB()
	if tm == nil || !tm.HasAPIKey() {
		writeJSON(w, http.StatusOK, out)
		return
	}
	wp, err := tm.GetWatchProviders(r.Context(), kind, tmdbID, region)
	if err != nil {
		writeJSON(w, http.StatusOK, out) // only extra: the page shows without it
		return
	}
	var title string
	if kind == "movie" {
		if d, err := tm.GetMovieDetail(r.Context(), tmdbID); err == nil {
			title = d.Title
		}
	} else if d, err := tm.GetShowFull(r.Context(), tmdbID); err == nil {
		title = d.Name
	}
	services := s.profileStreamPrefs(r).Services
	seen := map[int]bool{}
	add := func(list []metadata.Provider, free bool) []watchProvider {
		var res []watchProvider
		for _, p := range list {
			if seen[p.ID] {
				continue
			}
			seen[p.ID] = true
			link := wp.Link
			if title != "" {
				link = providerSearch(p.Name, title, wp.Link)
			}
			key := providerKeys[p.ID]
			mine := free || (key != "" && slices.Contains(services, key))
			res = append(res, watchProvider{Name: p.Name, Logo: metadata.LogoURL(p.LogoPath), URL: link, Key: key, Mine: mine})
		}
		return res
	}
	out.Free = append(out.Free, add(append(append([]metadata.Provider(nil), wp.Free...), wp.Ads...), true)...)
	out.Subscription = append(out.Subscription, add(wp.Subscription, false)...)
	// Yours first: the button shows the first of them.
	slices.SortStableFunc(out.Subscription, func(a, b watchProvider) int {
		switch {
		case a.Mine == b.Mine:
			return 0
		case a.Mine:
			return -1
		}
		return 1
	})
	writeJSON(w, http.StatusOK, out)
}
