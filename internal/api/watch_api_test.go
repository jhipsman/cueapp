package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rdborg/mediarium/internal/auth"
)

// fakeTMDBForWatch serves one movie (550) and one show (1399) with two
// aired episodes in season 1 and one in season 2.
func fakeTMDBForWatch(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reply := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		switch p := r.URL.Path; {
		case p == "/movie/550":
			reply(map[string]any{"id": 550, "title": "Fight Club", "release_date": "1999-10-15", "overview": "An insomniac...",
				"poster_path": "/p.jpg", "backdrop_path": "/b.jpg", "runtime": 139, "imdb_id": "tt0137523", "vote_average": 8.4,
				"genres": []any{map[string]any{"id": 18, "name": "Drama"}}})
		case p == "/tv/1399":
			reply(map[string]any{"id": 1399, "name": "Game of Thrones", "first_air_date": "2011-04-17", "poster_path": "/gp.jpg",
				"backdrop_path": "/gb.jpg", "seasons": []any{
					map[string]any{"season_number": 0, "episode_count": 3, "name": "Specials"},
					map[string]any{"season_number": 1, "episode_count": 2, "name": "Season 1"},
					map[string]any{"season_number": 2, "episode_count": 1, "name": "Season 2"},
				}})
		case p == "/tv/1399/season/1":
			reply(map[string]any{"episodes": []any{
				map[string]any{"season_number": 1, "episode_number": 1, "name": "Winter Is Coming", "air_date": "2011-04-17", "still_path": "/s1.jpg", "runtime": 62},
				map[string]any{"season_number": 1, "episode_number": 2, "name": "The Kingsroad", "air_date": "2011-04-24"},
			}})
		case p == "/tv/1399/season/2":
			reply(map[string]any{"episodes": []any{
				map[string]any{"season_number": 2, "episode_number": 1, "name": "The North Remembers", "air_date": "2012-04-01"},
			}})
		case p == "/search/multi":
			reply(map[string]any{"results": []any{
				map[string]any{"media_type": "movie", "id": 550, "title": "Fight Club", "poster_path": "/p.jpg"},
				map[string]any{"media_type": "person", "id": 1, "name": "Someone"},
			}})
		default: // every list and recommendation is empty
			reply(map[string]any{"results": []any{}, "page": 1, "total_pages": 1})
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestWatchFlow(t *testing.T) {
	s := newBareServer(t)
	s.TestSetTMDBBaseURL("key", fakeTMDBForWatch(t).URL)
	user := &auth.User{ID: 1, Username: "a"}
	if _, err := s.db.Exec(`INSERT INTO users (id, username, password_hash) VALUES (1, 'a', 'x')`); err != nil {
		t.Fatal(err)
	}

	call := func(method, target, body string, h http.HandlerFunc, pathValues ...string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, target, strings.NewReader(body)).WithContext(auth.WithUser(context.Background(), user))
		for i := 0; i+1 < len(pathValues); i += 2 {
			r.SetPathValue(pathValues[i], pathValues[i+1])
		}
		w := httptest.NewRecorder()
		h(w, r)
		return w
	}
	home := func() watchHome {
		t.Helper()
		var h watchHome
		_ = json.Unmarshal(call("GET", "/api/watch/home", "", s.handleWatchHome).Body.Bytes(), &h)
		return h
	}

	// Half-way through the movie, and finished episode 2 of season 1.
	if w := call("PUT", "/", `{"kind":"movie","tmdbId":550,"position":3000,"duration":8340}`, s.handlePutWatchProgress); w.Code != http.StatusNoContent {
		t.Fatalf("save movie progress: %d %s", w.Code, w.Body)
	}
	if w := call("PUT", "/", `{"kind":"tv","tmdbId":1399,"season":1,"episode":2,"position":3600,"duration":3700}`, s.handlePutWatchProgress); w.Code != http.StatusNoContent {
		t.Fatalf("save episode progress: %d %s", w.Code, w.Body)
	}
	if w := call("PUT", "/", `{"kind":"tv","tmdbId":1399,"season":1,"episode":0,"position":1}`, s.handlePutWatchProgress); w.Code != http.StatusBadRequest {
		t.Fatalf("episode 0 accepted: %d", w.Code)
	}

	h := home()
	if len(h.Rows) == 0 || h.Rows[0].Key != "continue" || len(h.Rows[0].Items) != 2 {
		t.Fatalf("home rows = %+v", h.Rows)
	}
	var show, movie watchCard
	for _, c := range h.Rows[0].Items {
		if c.Kind == "tv" {
			show = c
		} else {
			movie = c
		}
	}
	if show.Season != 2 || show.Episode != 1 || show.EpisodeTitle != "The North Remembers" {
		t.Fatalf("after finishing S1E2 the show card = %+v; want S2E1 up next", show)
	}
	if movie.Title != "Fight Club" || movie.Progress < 0.3 || movie.Progress > 0.4 || movie.BackdropURL == "" {
		t.Fatalf("movie card = %+v", movie)
	}

	// The movie page resumes where it stopped and offers My List.
	var title watchTitle
	_ = json.Unmarshal(call("GET", "/", "", s.handleWatchMovie, "tmdbId", "550").Body.Bytes(), &title)
	if title.ResumePosition != 3000 || title.Runtime != 139 || title.IMDbID != "tt0137523" || title.InList || len(title.Genres) != 1 {
		t.Fatalf("movie page = %+v", title)
	}
	if w := call("PUT", "/", "", s.handleAddToWatchList, "kind", "movie", "tmdbId", "550"); w.Code != http.StatusNoContent {
		t.Fatalf("add to list: %d %s", w.Code, w.Body)
	}
	if h := home(); len(h.Rows) < 2 || h.Rows[1].Key != "my-list" || h.Rows[1].Items[0].TMDBID != 550 {
		t.Fatalf("My List row missing: %+v", h.Rows)
	}

	// The show page skips specials and resumes at the next episode.
	_ = json.Unmarshal(call("GET", "/", "", s.handleWatchShow, "tmdbId", "1399").Body.Bytes(), &title)
	if len(title.Seasons) != 2 || title.Seasons[0].Number != 1 || title.ResumeSeason != 2 || title.ResumeEpisode != 1 {
		t.Fatalf("show page = %+v", title)
	}
	var eps []watchEpisode
	_ = json.Unmarshal(call("GET", "/", "", s.handleWatchSeason, "tmdbId", "1399", "season", "1").Body.Bytes(), &eps)
	if len(eps) != 2 || eps[0].StillURL == "" || eps[0].Runtime != 62 || !eps[1].Finished || !eps[0].Aired {
		t.Fatalf("season 1 = %+v", eps)
	}
	if w := call("GET", "/?season=2&episode=1", "", s.handleWatchNext, "tmdbId", "1399"); w.Code != http.StatusNotFound {
		t.Fatalf("next after the last episode: %d", w.Code)
	}

	var found []watchCard
	_ = json.Unmarshal(call("GET", "/?q=fight", "", s.handleWatchSearch).Body.Bytes(), &found)
	if len(found) != 1 || found[0].Kind != "movie" {
		t.Fatalf("search = %+v; want the movie and no people", found)
	}

	// Removing it from Continue Watching.
	if w := call("DELETE", "/", "", s.handleForgetWatchProgress, "kind", "movie", "tmdbId", "550"); w.Code != http.StatusNoContent {
		t.Fatalf("forget: %d", w.Code)
	}
	if h := home(); len(h.Rows[0].Items) != 1 {
		t.Fatalf("after forgetting the movie: %+v", h.Rows[0])
	}
}
