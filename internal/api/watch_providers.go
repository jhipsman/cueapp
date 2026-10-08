package api

import (
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/rdborg/mediarium/internal/metadata"
)

// Where else a title streams: TMDB's list of services (from JustWatch), so a
// title page can say "Free on Tubi" with a link that opens it there.

type watchProvider struct {
	Name string `json:"name"`
	Logo string `json:"logo,omitempty"`
	URL  string `json:"url"`
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
	seen := map[int]bool{}
	add := func(list []metadata.Provider) []watchProvider {
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
			res = append(res, watchProvider{Name: p.Name, Logo: metadata.LogoURL(p.LogoPath), URL: link})
		}
		return res
	}
	out.Free = append(out.Free, add(append(append([]metadata.Provider(nil), wp.Free...), wp.Ads...))...)
	out.Subscription = append(out.Subscription, add(wp.Subscription)...)
	writeJSON(w, http.StatusOK, out)
}
