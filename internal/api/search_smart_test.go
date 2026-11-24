package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeTMDBForSearch knows one actor (Tom Hanks), one topic (zombie), one
// film called "Comedy Club", and answers any discover with a film or show
// tagged with what it was asked for.
func fakeTMDBForSearch(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reply := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		q := r.URL.Query()
		asked = append(asked, r.URL.Path+"?"+q.Encode())
		switch r.URL.Path {
		case "/search/multi":
			switch strings.ToLower(q.Get("query")) {
			case "comedy":
				reply(map[string]any{"results": []map[string]any{{"media_type": "movie", "id": 1, "title": "Comedy Club", "poster_path": "/c.jpg"}}})
			case "breaking bad":
				reply(map[string]any{"results": []map[string]any{{"media_type": "tv", "id": 1396, "name": "Breaking Bad", "poster_path": "/b.jpg"}}})
			default:
				reply(map[string]any{"results": []any{}})
			}
		case "/search/person":
			if strings.EqualFold(q.Get("query"), "tom hanks") {
				reply(map[string]any{"results": []map[string]any{{"id": 31, "name": "Tom Hanks", "known_for_department": "Acting", "popularity": 40}}})
			} else {
				reply(map[string]any{"results": []any{}})
			}
		case "/person/31":
			reply(map[string]any{"id": 31, "name": "Tom Hanks", "combined_credits": map[string]any{"cast": []map[string]any{
				{"media_type": "movie", "id": 13, "title": "Forrest Gump", "poster_path": "/f.jpg", "vote_count": 27000},
				{"media_type": "movie", "id": 862, "title": "Toy Story", "poster_path": "/t.jpg", "vote_count": 18000},
				{"media_type": "tv", "id": 999, "name": "Late Night Talk", "poster_path": "/l.jpg", "vote_count": 99999, "genre_ids": []int{10767}},
			}}})
		case "/search/keyword":
			if strings.Contains(strings.ToLower(q.Get("query")), "zombie") {
				reply(map[string]any{"results": []map[string]any{{"id": 12377, "name": "zombie"}}})
			} else {
				reply(map[string]any{"results": []any{}})
			}
		case "/discover/movie":
			reply(map[string]any{"results": []map[string]any{{"id": 500, "title": "Found by " + q.Get("with_genres") + "/" + q.Get("with_keywords"), "poster_path": "/d.jpg"}}})
		case "/discover/tv":
			reply(map[string]any{"results": []map[string]any{{"id": 600, "name": "Show by " + q.Get("with_genres") + "/" + q.Get("with_keywords"), "poster_path": "/s.jpg"}}})
		default:
			reply(map[string]any{"results": []any{}})
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &asked
}

func TestSmartSearch(t *testing.T) {
	s := newBareServer(t)
	srv, asked := fakeTMDBForSearch(t)
	s.TestSetTMDBBaseURL("key", srv.URL)
	search := func(q string) watchSearchAnswer {
		t.Helper()
		ans := s.smartSearch(context.Background(), q)
		return ans
	}
	titles := func(sec watchSearchRow) []string {
		var out []string
		for _, c := range sec.Items {
			out = append(out, c.Title)
		}
		return out
	}

	// A person: what they're in (not the talk shows), best known first.
	ans := search("tom hanks")
	if len(ans.Sections) == 0 || ans.Sections[0].Key != "person" || ans.Sections[0].Title != "With Tom Hanks" {
		t.Fatalf("tom hanks: %+v", ans.Sections)
	}
	if got := titles(ans.Sections[0]); strings.Join(got, ",") != "Forrest Gump,Toy Story" {
		t.Errorf("tom hanks: %v", got)
	}

	// A genre: comedies first, then the film named like it.
	ans = search("comedy")
	if len(ans.Sections) < 2 || ans.Sections[0].Key != "intent" || ans.Sections[1].Key != "titles" {
		t.Fatalf("comedy: %+v", ans.Sections)
	}
	if ans.Understood != "Comedy movies and shows" || titles(ans.Sections[0])[0] != "Found by 35/" {
		t.Errorf("comedy: %q %v", ans.Understood, titles(ans.Sections[0]))
	}

	// Genre, decade, kind: one browse of films.
	*asked = nil
	ans = search("funny 90s movies")
	if ans.Understood != "Comedy movies from the 1990s" {
		t.Errorf("funny 90s movies: %q", ans.Understood)
	}
	sawDecade := false
	for _, a := range *asked {
		if strings.HasPrefix(a, "/discover/tv") {
			t.Errorf("asked for shows: %s", a)
		}
		if strings.Contains(a, "primary_release_date.gte=1990-01-01") {
			sawDecade = true
		}
	}
	if !sawDecade {
		t.Errorf("no 1990s browse in %v", *asked)
	}

	// A topic, plural and all.
	ans = search("zombies")
	if len(ans.Sections) == 0 || ans.Sections[0].Key != "topic" || ans.Sections[0].Title != "About zombie" {
		t.Fatalf("zombies: %+v", ans.Sections)
	}
	// A topic with a kind: films tagged with it.
	ans = search("zombie movies")
	if len(ans.Sections) == 0 || titles(ans.Sections[0])[0] != "Found by /12377" {
		t.Errorf("zombie movies: %+v", ans.Sections)
	}

	// A misspelling: the spelling that finds it.
	ans = search("breaking badd")
	if ans.DidYouMean != "breaking bad" || len(ans.Sections) != 1 {
		t.Errorf("breaking badd: %+v", ans)
	}
}
