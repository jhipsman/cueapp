package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rdborg/mediarium/internal/auth"
)

// fakeIPTV is an Xtream provider with two channels, a guide, and an HLS
// channel whose address redirects to a tokened playlist elsewhere.
func fakeIPTV(t *testing.T) *httptest.Server {
	t.Helper()
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00")
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		now := time.Now().UTC()
		switch {
		case r.URL.Path == "/player_api.php" && (q.Get("username") != "me" || q.Get("password") != "pw"):
			w.Write([]byte(`{"user_info":{"auth":0}}`))
		case r.URL.Path == "/player_api.php" && q.Get("action") == "":
			w.Write([]byte(`{"user_info":{"auth":1,"status":"Active","exp_date":"1893456000","allowed_output_formats":["m3u8","ts"]}}`))
		case r.URL.Path == "/player_api.php" && q.Get("action") == "get_live_categories":
			w.Write([]byte(`[{"category_id":"1","category_name":"News"}]`))
		case r.URL.Path == "/player_api.php" && q.Get("action") == "get_live_streams":
			w.Write([]byte(`[{"num":1,"name":"News 24","stream_type":"live","stream_id":10,"stream_icon":"` + srv.URL + `/logo.png","epg_channel_id":"news24","category_id":"1"},
				{"num":2,"name":"No Guide","stream_type":"live","stream_id":11,"category_id":"1"}]`))
		case r.URL.Path == "/xmltv.php":
			f := func(t time.Time) string { return t.Format("20060102150405 -0700") }
			w.Write([]byte(`<tv><programme start="` + f(now.Add(-30*time.Minute)) + `" stop="` + f(now.Add(30*time.Minute)) + `" channel="news24"><title>Headlines</title></programme>` +
				`<programme start="` + f(now.Add(30*time.Minute)) + `" stop="` + f(now.Add(90*time.Minute)) + `" channel="news24"><title>Weather</title></programme></tv>`))
		case r.URL.Path == "/logo.png":
			w.Write(png)
		case r.URL.Path == "/live/me/pw/10.m3u8":
			http.Redirect(w, r, "/hls/abc/index.m3u8?token=t", http.StatusFound)
		case r.URL.Path == "/hls/abc/index.m3u8":
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			w.Write([]byte("#EXTM3U\n#EXT-X-KEY:METHOD=AES-128,URI=\"key.bin\"\n#EXTINF:6,\nseg1.ts?token=t\n"))
		case r.URL.Path == "/hls/abc/seg1.ts":
			w.Header().Set("Content-Type", "video/mp2t")
			w.Write([]byte("video"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestLiveTV(t *testing.T) {
	s := newBareServer(t)
	prov := fakeIPTV(t)
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

	if w := call("GET", "/", "", s.handleLiveChannels); !strings.Contains(w.Body.String(), `"configured":false`) {
		t.Fatalf("before setup: %d %s", w.Code, w.Body)
	}
	if w := call("PUT", "/", `{"server":"`+prov.URL+`","username":"me","password":"nope"}`, s.handlePutIPTV); w.Code != http.StatusBadRequest {
		t.Fatalf("bad login saved: %d %s", w.Code, w.Body)
	}
	if w := call("PUT", "/", `{"server":"`+prov.URL+`/","username":"me","password":"pw"}`, s.handlePutIPTV); w.Code != http.StatusOK {
		t.Fatalf("save login: %d %s", w.Code, w.Body)
	}
	if w := call("GET", "/", "", s.handleGetIPTV); strings.Contains(w.Body.String(), "pw") || !strings.Contains(w.Body.String(), `"set":true`) {
		t.Fatalf("settings answer: %s", w.Body)
	}
	// Saving again without the password keeps it.
	if w := call("PUT", "/", `{"server":"`+prov.URL+`","username":"me","password":""}`, s.handlePutIPTV); w.Code != http.StatusOK {
		t.Fatalf("save without password: %d %s", w.Code, w.Body)
	}

	type channels struct {
		Channels []liveChannel `json:"channels"`
		Ready    bool          `json:"guideReady"`
	}
	var got channels
	deadline := time.Now().Add(5 * time.Second)
	for {
		w := call("GET", "/", "", s.handleLiveChannels)
		_ = json.Unmarshal(w.Body.Bytes(), &got)
		if got.Ready || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(got.Channels) != 2 || got.Channels[0].Now == nil || got.Channels[0].Now.Title != "Headlines" || got.Channels[0].Next.Title != "Weather" {
		t.Fatalf("channels: %+v", got)
	}
	if got.Channels[0].Logo != "/api/live/logo/10" || got.Channels[1].Logo != "" {
		t.Fatalf("logos: %+v", got.Channels)
	}

	// Favourites, per profile.
	if w := call("PUT", "/", "", s.handleLiveFavorite, "id", "11"); w.Code != http.StatusOK {
		t.Fatalf("favorite: %d %s", w.Code, w.Body)
	}
	_ = json.Unmarshal(call("GET", "/", "", s.handleLiveChannels).Body.Bytes(), &got)
	if got.Channels[0].Favorite || !got.Channels[1].Favorite {
		t.Fatalf("favorites: %+v", got.Channels)
	}

	// The guide grid.
	var guide struct {
		Programmes map[string][]struct{ Title string } `json:"programmes"`
	}
	_ = json.Unmarshal(call("GET", "/api/live/guide?ids=10,11&hours=3", "", s.handleLiveGuide).Body.Bytes(), &guide)
	if len(guide.Programmes["10"]) != 2 || len(guide.Programmes) != 1 {
		t.Fatalf("guide: %+v", guide)
	}

	// The logo comes through Cue.
	if w := call("GET", "/", "", s.handleLiveLogo, "id", "10"); w.Code != http.StatusOK || w.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("logo: %d %s", w.Code, w.Header())
	}

	// Playing: the browser gets a sealed address with no password in it,
	// the TV app the provider's own.
	w := call("GET", "/", "", s.handleLivePlay, "id", "10")
	var play struct{ URL, Direct string }
	_ = json.Unmarshal(w.Body.Bytes(), &play)
	if !strings.HasPrefix(play.URL, "/api/live/hls?u=") || strings.Contains(play.URL, "pw") || play.Direct != "" {
		t.Fatalf("play: %s", w.Body)
	}
	r := httptest.NewRequest("GET", "/", nil).WithContext(auth.WithUser(context.Background(), user))
	r.Header.Set("User-Agent", "Mozilla/5.0 CueTV/1")
	r.SetPathValue("id", "10")
	tw := httptest.NewRecorder()
	s.handleLivePlay(tw, r)
	_ = json.Unmarshal(tw.Body.Bytes(), &play)
	if play.Direct != prov.URL+"/live/me/pw/10.ts" {
		t.Fatalf("tv play: %s", tw.Body)
	}

	// The playlist comes back with every address pointed through Cue.
	w = call("GET", play.URL, "", s.handleLiveHLS)
	body := w.Body.String()
	if w.Code != http.StatusOK || !strings.HasPrefix(body, "#EXTM3U") || strings.Contains(body, "seg1.ts") || strings.Contains(body, `URI="key.bin"`) {
		t.Fatalf("playlist: %d %s", w.Code, body)
	}
	var seg string
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "/api/live/hls?u=") {
			seg = line
		}
	}
	if w := call("GET", seg, "", s.handleLiveHLS); w.Code != http.StatusOK || w.Body.String() != "video" || w.Header().Get("Content-Type") != "video/mp2t" {
		t.Fatalf("segment: %d %q", w.Code, w.Body)
	}
	if w := call("GET", "/api/live/hls?u="+url.QueryEscape("forged"), "", s.handleLiveHLS); w.Code != http.StatusBadRequest {
		t.Fatalf("forged address: %d", w.Code)
	}

	// Removing the login turns Live TV off.
	if w := call("PUT", "/", `{"server":""}`, s.handlePutIPTV); w.Code != http.StatusOK {
		t.Fatalf("remove: %d", w.Code)
	}
	if w := call("GET", "/", "", s.handleLiveChannels); !strings.Contains(w.Body.String(), `"configured":false`) {
		t.Fatalf("after removing: %s", w.Body)
	}
}
