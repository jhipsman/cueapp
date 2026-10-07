package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPlayUsesStreamAddonsFirst(t *testing.T) {
	s := newBareServer(t)
	s.TestSetTMDBBaseURL("key", fakeTMDBForWatch(t).URL) // movie 550 is tt0137523
	addon := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/secret-config/manifest.json":
			_, _ = w.Write([]byte(`{"id":"comet","name":"Comet","version":"2","resources":["stream"],"types":["movie","series"]}`))
		case "/secret-config/stream/movie/tt0137523.json":
			_, _ = w.Write([]byte(`{"streams":[
				{"name":"[PM+] 2160p","description":"Fight.Club.1999.2160p.UHD.BluRay.x265\n💾 18 GB","url":"https://comet.example/playback/1","behaviorHints":{"filename":"Fight.Club.1999.2160p.UHD.BluRay.x265.mkv","videoSize":19327352832}},
				{"name":"[PM+] 1080p","title":"Fight.Club.1999.1080p.BluRay.x264","url":"https://comet.example/playback/2"}
			]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(addon.Close)
	playCache = playLists{}
	t.Cleanup(func() { playCache = playLists{} })

	// Adding it checks the manifest; the list never shows the full address.
	add := httptest.NewRecorder()
	s.handleAddStreamAddon(add, httptest.NewRequest("POST", "/", strings.NewReader(`{"url":"`+addon.URL+`/secret-config/manifest.json"}`)))
	if add.Code != http.StatusOK || strings.Contains(add.Body.String(), "secret-config") || !strings.Contains(add.Body.String(), `"name":"Comet"`) {
		t.Fatalf("add: %d %s", add.Code, add.Body)
	}
	dup := httptest.NewRecorder()
	s.handleAddStreamAddon(dup, httptest.NewRequest("POST", "/", strings.NewReader(`{"url":"`+addon.URL+`/secret-config"}`)))
	if dup.Code != http.StatusConflict {
		t.Fatalf("adding it twice: %d", dup.Code)
	}

	// No Premiumize key in Cue: Comet's own links still play, in its order.
	play := func(q string) (int, playAnswer, string) {
		r := httptest.NewRequest("GET", "/"+q, nil)
		r.SetPathValue("tmdbId", "550")
		w := httptest.NewRecorder()
		s.handlePlayTMDBMovie(w, r)
		var a playAnswer
		_ = json.Unmarshal(w.Body.Bytes(), &a)
		return w.Code, a, w.Body.String()
	}
	code, a, body := play("")
	if code != http.StatusOK || a.URL != "https://comet.example/playback/1" || a.Source != "Comet" || a.Options != 2 || a.SizeBytes != 19327352832 || !strings.Contains(a.Quality, "2160") {
		t.Fatalf("play: %d %s", code, body)
	}
	if code, a, _ := play("?option=2"); code != http.StatusOK || a.URL != "https://comet.example/playback/2" {
		t.Fatalf("option 2: %d %+v", code, a)
	}

	rm := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/", nil)
	r.SetPathValue("index", "0")
	s.handleRemoveStreamAddon(rm, r)
	if rm.Code != http.StatusOK || strings.TrimSpace(rm.Body.String()) != "[]" {
		t.Fatalf("remove: %d %s", rm.Code, rm.Body)
	}
	if code, _, body := play(""); code != http.StatusConflict || !strings.Contains(body, "Premiumize") {
		t.Fatalf("without add-ons or a key: %d %s", code, body)
	}
}
