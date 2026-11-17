package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jhipsman/cueapp/internal/metadata"
)

func TestQuietTitles(t *testing.T) {
	for _, c := range []struct {
		name      string
		genres    []int
		lang      string
		countries []string
		adult     bool
		cert      string
		want      bool
	}{
		{"horror film", []int{27, 53}, "en", nil, false, "R", true},
		{"anime film", []int{16, 14}, "ja", nil, false, "PG", true},
		{"anime show by country", []int{16}, "en", []string{"JP"}, false, "", true},
		{"adult", []int{18}, "en", nil, true, "", true},
		{"NC-17", []int{18}, "en", nil, false, "NC-17", true},
		{"cartoon from elsewhere", []int{16, 10751}, "en", []string{"US"}, false, "PG", false},
		{"thriller", []int{53, 80}, "en", nil, false, "R", false},
		{"Japanese drama", []int{18}, "ja", []string{"JP"}, false, "", false},
	} {
		if got := quietBy(c.genres, c.lang, c.countries, c.adult, c.cert); got != c.want {
			t.Errorf("%s: quiet = %v, want %v", c.name, got, c.want)
		}
	}

	// Erotic films and ones tagged for nudity or sex scenes, by their keywords.
	for kw, want := range map[string]bool{
		"erotic movie": true, "softcore": true, "female nudity": true, "sex scene": true, "erotic thriller": true,
		"time travel": false, "sexual identity": false, "superhero": false,
	} {
		if got := adultKeywords([]metadata.Genre{{Name: kw}}); got != want {
			t.Errorf("keyword %q: %v, want %v", kw, got, want)
		}
	}

	// Cards made from TMDB's lists know at once, and remember it.
	horror := movieCard(metadata.Movie{TMDBID: 900001, Title: "Scary", GenreIDs: []int{27}})
	comedy := movieCard(metadata.Movie{TMDBID: 900002, Title: "Funny", GenreIDs: []int{35}})
	anime := showCard(metadata.Show{TMDBID: 900003, Name: "Drawn", GenreIDs: []int{16}, OriginalLanguage: "ja"})
	if !horror.Quiet || comedy.Quiet || !anime.Quiet {
		t.Fatalf("cards: horror %v, comedy %v, anime %v", horror.Quiet, comedy.Quiet, anime.Quiet)
	}
	s := &Server{}
	if !s.quietTitle(context.Background(), "movie", 900001) || s.quietTitle(context.Background(), "movie", 900002) {
		t.Fatal("remembered answers not used")
	}

	// A card from your history (no genres) is looked up; with no TMDB to ask
	// it stays as it is.
	rows := []watchRow{{Key: "continue", Items: []watchCard{{Kind: "movie", TMDBID: 900001}, {Kind: "movie", TMDBID: 900099}}}}
	s.markQuiet(context.Background(), rows)
	if !rows[0].Items[0].Quiet || rows[0].Items[1].Quiet {
		t.Fatalf("marked: %+v", rows[0].Items)
	}

	// A quiet title gets no preview clip.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/watch/preview/{kind}/{tmdbId}", s.handleWatchPreview)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/api/watch/preview/movie/900001", nil))
	var body struct {
		None bool `json:"none"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || rec.Code != 200 || !body.None {
		t.Fatalf("preview of a horror film: %d %s", rec.Code, rec.Body)
	}
}
