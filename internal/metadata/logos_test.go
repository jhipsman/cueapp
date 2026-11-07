package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestLogoPicksEnglishPNGAndCaches(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/tv/1421/images" || r.URL.Query().Get("include_image_language") != "en,null" {
			t.Errorf("unexpected request %s", r.URL)
		}
		_, _ = w.Write([]byte(`{"logos":[
			{"file_path":"/de.png","iso_639_1":"de"},
			{"file_path":"/en.svg","iso_639_1":"en"},
			{"file_path":"/none.png","iso_639_1":null},
			{"file_path":"/en.png","iso_639_1":"en"}]}`))
	}))
	defer srv.Close()
	c := &Client{apiKey: "k", baseURL: srv.URL, httpClient: srv.Client()}

	for range 2 {
		u, err := c.Logo(context.Background(), "tv", 1421)
		if err != nil || u != "https://image.tmdb.org/t/p/w500/en.png" {
			t.Fatalf("logo = %q, %v", u, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("TMDB asked %d times, want 1 (cached)", calls.Load())
	}
	if _, err := c.Logo(context.Background(), "person", 1); err == nil {
		t.Fatal("an unknown kind should be refused")
	}
}

func TestPickLogoFallbacks(t *testing.T) {
	var r imageList
	r.Logos = append(r.Logos, struct {
		FilePath string `json:"file_path"`
		Lang     string `json:"iso_639_1"`
	}{"/only.svg", "en"})
	if got := pickLogo(r); got != "" {
		t.Fatalf("an SVG alone should give no logo, got %q", got)
	}
}
