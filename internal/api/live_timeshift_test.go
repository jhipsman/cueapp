package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jhipsman/cueapp/internal/auth"
)

// Pausing live TV: the server keeps a copy of the channel as a live
// playlist, shared by the players watching it; a player changing channel
// stops the copy no one else watches; the owner can turn it off.
func TestLiveTimeshift(t *testing.T) {
	if !canConvert() {
		t.Skip("no ffmpeg here")
	}
	s := newBareServer(t)
	s.cfg.ConfigDir = t.TempDir()
	timeshiftMinFree = 0
	defer func() { timeshiftMinFree = 2 << 30 }()
	clip := filepath.Join(t.TempDir(), "clip.ts")
	if out, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=size=320x240:rate=25:duration=10",
		"-f", "lavfi", "-i", "sine=duration=10", "-c:v", "libx264", "-g", "25", "-b:v", "500k", "-pix_fmt", "yuv420p", "-c:a", "mp2", "-f", "mpegts", clip).CombinedOutput(); err != nil {
		t.Skipf("can't make a clip here: %v %s", err, out)
	}
	prov := fakeIPTV(t)
	stream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/live/me/pw/10.ts" || r.URL.Path == "/live/me/pw/11.ts" {
			http.ServeFile(w, r, clip)
			return
		}
		http.Redirect(w, r, prov.URL+r.URL.RequestURI(), http.StatusFound)
	}))
	defer stream.Close()

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
	if w := call("PUT", "/", `{"server":"`+stream.URL+`","username":"me","password":"pw"}`, s.handlePutIPTV); w.Code != http.StatusOK {
		t.Fatalf("save login: %d %s", w.Code, w.Body)
	}
	_ = call("GET", "/", "", s.handleLiveChannels)

	// Playing a channel offers the copy.
	var play struct{ Timeshift string }
	_ = json.Unmarshal(call("GET", "/", "", s.handleLivePlay, "id", "10").Body.Bytes(), &play)
	if play.Timeshift != "/api/live/timeshift/10" {
		t.Fatalf("play: timeshift %q", play.Timeshift)
	}
	start := func(ch, viewer string) string {
		t.Helper()
		w := call("GET", "/?v="+viewer, "", s.handleLiveTimeshift, "id", ch)
		var got struct {
			URL     string
			Minutes int
		}
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || w.Code != http.StatusOK || got.Minutes != 30 {
			t.Fatalf("start %s: %d %s", ch, w.Code, w.Body)
		}
		return got.URL
	}
	file := func(u string) *httptest.ResponseRecorder {
		parts := strings.Split(strings.TrimPrefix(u, "/api/live/ts/"), "/")
		r := httptest.NewRequest("GET", u, nil)
		r.SetPathValue("id", parts[0])
		r.SetPathValue("file", parts[1])
		w := httptest.NewRecorder()
		s.handleTimeshiftFile(w, r)
		return w
	}
	u := start("10", "tv")
	w := file(u)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "#EXTINF") {
		t.Fatalf("playlist: %d %s", w.Code, w.Body)
	}
	var seg string
	for _, line := range strings.Split(w.Body.String(), "\n") {
		if strings.HasPrefix(line, "seg_") {
			seg = line
			break
		}
	}
	if w := file(strings.Replace(u, "index.m3u8", seg, 1)); w.Code != http.StatusOK || w.Header().Get("Content-Type") != "video/mp2t" || w.Body.Len() < 1000 {
		t.Fatalf("piece %q: %d %d", seg, w.Code, w.Body.Len())
	}
	if w := file(strings.Replace(u, "index.m3u8", "../../secret", 1)); w.Code != http.StatusNotFound {
		t.Fatalf("another file: %d", w.Code)
	}

	// A second player on the same channel shares the copy.
	if u2 := start("10", "phone"); u2 != u {
		t.Fatalf("not shared: %s %s", u, u2)
	}
	// The TV changes channel: channel 10 carries on for the phone.
	u11 := start("11", "tv")
	if file(u).Code != http.StatusOK {
		t.Fatal("the phone's channel stopped")
	}
	// The phone stops watching: channel 10's copy stops.
	_ = call("DELETE", "/?v=phone", "", s.handleLeaveTimeshift)
	if file(u).Code != http.StatusNotFound {
		t.Fatal("channel 10 kept going with no one watching")
	}
	if file(u11).Code != http.StatusOK {
		t.Fatal("channel 11 stopped")
	}
	// The stream ends (the clip is short) and the copy carries on from
	// where it was: the playlist keeps its earlier pieces.
	time.Sleep(3 * time.Second)
	if body := file(u11).Body.String(); !strings.Contains(body, "seg_000000.ts") {
		t.Fatalf("the copy lost its start:\n%s", body)
	}

	// Turned off: no copy offered.
	if w := call("PUT", "/", `{"minutes":0}`, s.handlePutTimeshift); w.Code != http.StatusOK {
		t.Fatalf("turn off: %d %s", w.Code, w.Body)
	}
	play.Timeshift = ""
	_ = json.Unmarshal(call("GET", "/", "", s.handleLivePlay, "id", "10").Body.Bytes(), &play)
	if play.Timeshift != "" {
		t.Fatalf("still offered: %q", play.Timeshift)
	}
	if w := call("GET", "/", "", s.handleLiveTimeshift, "id", "10"); w.Code != http.StatusNotFound {
		t.Fatalf("start when off: %d", w.Code)
	}
	if w := call("PUT", "/", `{"minutes":500}`, s.handlePutTimeshift); w.Code != http.StatusBadRequest {
		t.Fatalf("too long: %d", w.Code)
	}
	_ = call("DELETE", "/?v=tv", "", s.handleLeaveTimeshift)
}
