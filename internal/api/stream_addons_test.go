package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rdborg/mediarium/internal/settings"
)

func TestResolveAddonLinkKeepsLinksThatDontRedirect(t *testing.T) {
	file := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "bytes=0-0" {
			t.Errorf("fetched more than the first byte: %q", r.Header.Get("Range"))
		}
		w.WriteHeader(http.StatusPartialContent)
	}))
	t.Cleanup(file.Close)
	old := addonLinkClient
	addonLinkClient = func() *http.Client {
		return &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	t.Cleanup(func() { addonLinkClient = old })
	if got := resolveAddonLink(t.Context(), file.URL+"/movie.mkv"); got != file.URL+"/movie.mkv" {
		t.Fatalf("a direct file link changed: %s", got)
	}
	if got := resolveAddonLink(t.Context(), "http://127.0.0.1:1/unreachable"); got != "http://127.0.0.1:1/unreachable" {
		t.Fatalf("an unreachable link changed: %s", got)
	}
}

func TestPlayUsesStreamAddonsFirst(t *testing.T) {
	s := newBareServer(t)
	s.TestSetTMDBBaseURL("key", fakeTMDBForWatch(t).URL) // movie 550 is tt0137523
	var addon *httptest.Server
	addon = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		// Like Comet: a play link sends whoever asked for the streams on to
		// the debrid file (anyone else gets "Wrong IP", which the device,
		// elsewhere, would see if it got this link).
		case "/playback/1":
			http.Redirect(w, r, "/playback/1/resolved", http.StatusFound) // a hop on the add-on first
		case "/playback/1/resolved":
			http.Redirect(w, r, "https://pm.example/dl/Fight.Club.2160p.mkv", http.StatusFound)
		case "/playback/2":
			http.Redirect(w, r, "https://pm.example/dl/Fight.Club.1080p.mkv", http.StatusFound)
		case "/secret-config/manifest.json":
			_, _ = w.Write([]byte(`{"id":"comet","name":"Comet","version":"2","resources":["stream"],"types":["movie","series"]}`))
		case "/secret-config/stream/movie/tt0137523.json":
			_, _ = w.Write([]byte(`{"streams":[
				{"name":"[PM+] 2160p","description":"Fight.Club.1999.2160p.UHD.BluRay.x265\n🇮🇹","url":"` + addon.URL + `/playback/it"},
				{"name":"[PM+] 2160p","description":"Fight.Club.1999.2160p.UHD.BluRay.x265\n💾 18 GB","url":"` + addon.URL + `/playback/1","behaviorHints":{"filename":"Fight.Club.1999.2160p.UHD.BluRay.x265.mkv","videoSize":19327352832}},
				{"name":"[PM+] 1080p","title":"Fight.Club.1999.1080p.BluRay.x264","url":"` + addon.URL + `/playback/2"}
			]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(addon.Close)
	playCache = playLists{}
	t.Cleanup(func() { playCache = playLists{} })
	oldClient := addonLinkClient
	addonLinkClient = func() *http.Client {
		return &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	t.Cleanup(func() { addonLinkClient = oldClient })

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
	if code != http.StatusOK || a.URL != "https://pm.example/dl/Fight.Club.2160p.mkv" || a.Source != "Comet" || a.Options != 2 || a.SizeBytes != 19327352832 || !strings.Contains(a.Quality, "2160") {
		t.Fatalf("play: %d %s", code, body)
	}
	if code, a, _ := play("?option=2"); code != http.StatusOK || a.URL != "https://pm.example/dl/Fight.Club.1080p.mkv" {
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

// In a browser, an add-on link whose torrent is known plays from
// Premiumize's converted copy (sound every browser plays); the TV app gets
// the add-on's file as it is.
func TestBrowserGetsPremiumizesConvertedCopy(t *testing.T) {
	s := newBareServer(t)
	s.TestSetTMDBBaseURL("key", fakeTMDBForWatch(t).URL)
	s.premiumizeBase = fakePremiumizeCache(t).URL
	if err := s.Settings.Set(settings.KeyPremiumizeAPIKey, "k", true); err != nil {
		t.Fatal(err)
	}
	var addon *httptest.Server
	addon = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/manifest.json"):
			_, _ = w.Write([]byte(`{"id":"comet","name":"Comet","version":"2","resources":["stream"],"types":["movie"]}`))
		case strings.HasSuffix(r.URL.Path, "/stream/movie/tt0137523.json"):
			_, _ = w.Write([]byte(`{"streams":[{"name":"[PM+] 1080p","description":"Fight.Club.1999.1080p.BluRay.DDP5.1.x264\n🇬🇧","url":"` + addon.URL + `/c/playback/` + hash1080 + `/0"}]}`))
		case strings.Contains(r.URL.Path, "/playback/"):
			http.Redirect(w, r, "https://pm.example/dl/Fight.Club.1080p.mkv", http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(addon.Close)
	playCache = playLists{}
	t.Cleanup(func() { playCache = playLists{} })
	old := addonLinkClient
	addonLinkClient = func() *http.Client {
		return &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	t.Cleanup(func() { addonLinkClient = old })
	add := httptest.NewRecorder()
	s.handleAddStreamAddon(add, httptest.NewRequest("POST", "/", strings.NewReader(`{"url":"`+addon.URL+`/c/manifest.json"}`)))
	if add.Code != http.StatusOK {
		t.Fatalf("add: %d %s", add.Code, add.Body)
	}
	play := func(q string) playAnswer {
		r := httptest.NewRequest("GET", "/"+q, nil)
		r.SetPathValue("tmdbId", "550")
		w := httptest.NewRecorder()
		s.handlePlayTMDBMovie(w, r)
		var a playAnswer
		if err := json.Unmarshal(w.Body.Bytes(), &a); err != nil || w.Code != http.StatusOK {
			t.Fatalf("play%s: %d %s", q, w.Code, w.Body)
		}
		return a
	}
	if a := play("?client=web"); a.StreamURL != "https://cdn.example/"+hash1080+".mp4" || a.Source != "Comet" {
		t.Fatalf("browser: %+v", a)
	}
	if a := play(""); a.URL != "https://pm.example/dl/Fight.Club.1080p.mkv" || a.StreamURL != "" {
		t.Fatalf("TV app: %+v", a)
	}
}
