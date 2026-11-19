package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jhipsman/cueapp/internal/metadata"
)

func TestTop10(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if strings.HasPrefix(r.URL.Path, "/watch/providers/") {
			fmt.Fprint(w, `{"results":[{"provider_id":8,"provider_name":"Netflix","logo_path":"/netflix.jpg"}]}`)
			return
		}
		if q.Get("watch_region") != "US" || q.Get("with_watch_monetization_types") != "flatrate" {
			t.Errorf("query %v", q)
		}
		if q.Get("with_watch_providers") != "8" { // only Netflix has anything
			fmt.Fprint(w, `{"page":1,"total_pages":1,"results":[]}`)
			return
		}
		if r.URL.Path == "/discover/movie" {
			fmt.Fprint(w, `{"page":1,"results":[{"id":1,"title":"Film A","poster_path":"/a.jpg","popularity":50},{"id":2,"title":"Film B","poster_path":"/b.jpg","popularity":10}]}`)
			return
		}
		fmt.Fprint(w, `{"page":1,"results":[{"id":3,"name":"Show C","poster_path":"/c.jpg","popularity":30},{"id":4,"name":"No poster","popularity":99}]}`)
	}))
	defer srv.Close()
	s := &Server{}
	lists := s.top10(context.Background(), metadata.NewWithBaseURL("k", srv.URL))
	if len(lists) != 1 || lists[0].Name != "Netflix" || lists[0].Logo != "https://image.tmdb.org/t/p/w92/netflix.jpg" {
		t.Fatalf("lists = %+v", lists)
	}
	var got []string
	for _, c := range lists[0].Items {
		got = append(got, c.Title)
	}
	if fmt.Sprint(got) != "[Film A Show C Film B]" {
		t.Fatalf("order = %v", got)
	}
}

func TestWeekAheadOnlyOnFriday(t *testing.T) {
	s := &Server{}
	thu := time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)
	if _, _, _, ok := s.weekAhead(context.Background(), 1, thu); ok {
		t.Fatal("week ahead on a Thursday")
	}
	fri := time.Date(2026, 11, 20, 7, 0, 0, 0, time.UTC)
	if _, _, _, ok := s.weekAhead(context.Background(), 1, fri); ok {
		t.Fatal("week ahead before 9am")
	}
	if !kidsProfile("PG") || !kidsProfile("TV-Y7") || kidsProfile("PG-13") || kidsProfile("") {
		t.Fatal("kidsProfile")
	}
}

func TestTopPageTitle(t *testing.T) {
	s := &Server{}
	for q, want := range map[string]string{
		"": "Top 10 titles",
		"?kind=movie&genre=Comedy&decade=1990&service=netflix&country=CA": "Top 10 Comedy films of the 1990s on Netflix in Canada",
		"?kind=tv&country=XX&decade=1995":                                 "Top 10 shows",
	} {
		rec := httptest.NewRecorder()
		s.handleWatchTop(rec, httptest.NewRequest("GET", "/api/watch/top"+q, nil))
		var p struct {
			Title    string `json:"title"`
			Services []any  `json:"services"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || p.Title != want || len(p.Services) != len(topServices) {
			t.Errorf("%q: %q (%v), want %q", q, p.Title, err, want)
		}
	}
}
