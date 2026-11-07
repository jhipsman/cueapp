package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWatchExtrasGivesLogoAndTrailer(t *testing.T) {
	server, base, client := loginNewServer(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/movie/603", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"id": 603, "title": "The Matrix", "release_date": "1999-03-31",
			"videos": map[string]any{"results": []map[string]any{
				{"site": "YouTube", "key": "featurette", "type": "Featurette"},
				{"site": "YouTube", "key": "trailer1", "type": "Trailer", "official": true},
			}}})
	})
	mux.HandleFunc("/movie/603/images", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"logos": []map[string]any{{"file_path": "/logo.png", "iso_639_1": "en"}}})
	})
	tmdb := httptest.NewServer(mux)
	defer tmdb.Close()
	server.TestSetTMDBBaseURL("k", tmdb.URL)

	got := getJSON[map[string]string](t, client, base+"/api/watch/extras/movie/603")
	if got["trailer"] != "trailer1" {
		t.Errorf("trailer = %q, want the official trailer", got["trailer"])
	}
	if got["logo"] != "https://image.tmdb.org/t/p/w500/logo.png" {
		t.Errorf("logo = %q", got["logo"])
	}
	if got["colour"] != "" {
		t.Errorf("a title with no picture has no colour, got %q", got["colour"])
	}

	resp, err := client.Get(base + "/api/watch/extras/person/1")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("an unknown kind should be refused, got %d", resp.StatusCode)
	}
}
