package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rdborg/mediarium/internal/auth"
	"github.com/rdborg/mediarium/internal/watch"
)

func TestLiveRecordingAndFollows(t *testing.T) {
	if !canConvert() {
		t.Skip("no ffmpeg here")
	}
	s := newBareServer(t)
	s.cfg.ConfigDir = t.TempDir()
	clip := filepath.Join(t.TempDir(), "clip.ts")
	if out, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=size=320x240:rate=25:duration=4",
		"-f", "lavfi", "-i", "sine=duration=4", "-c:v", "libx264", "-b:v", "1M", "-pix_fmt", "yuv420p", "-c:a", "mp2", "-f", "mpegts", clip).CombinedOutput(); err != nil {
		t.Skipf("can't make a clip here: %v %s", err, out)
	}
	prov := fakeIPTV(t)
	// The channel's stream: the clip (the fake provider sends it for 10.ts).
	stream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/live/me/pw/10.ts" {
			http.ServeFile(w, r, clip)
			return
		}
		http.Redirect(w, r, prov.URL+r.URL.RequestURI(), http.StatusFound)
	}))
	defer stream.Close()
	recordLate, recordRetry = 0, 50*time.Millisecond
	defer func() { recordLate, recordRetry = 2*time.Minute, 10*time.Second }()

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

	// Follow "weather": the Weather show in half an hour gets a reminder.
	if w := call("PUT", "/", `{"phrase":"Weather"}`, s.handleSetLiveFollow); w.Code != http.StatusNoContent {
		t.Fatalf("follow: %d %s", w.Code, w.Body)
	}
	deadline := time.Now().Add(5 * time.Second)
	var rems []watch.LiveReminder
	for time.Now().Before(deadline) {
		_ = json.Unmarshal(call("GET", "/", "", s.handleLiveReminders).Body.Bytes(), &rems)
		if len(rems) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if len(rems) != 1 || rems[0].Title != "Weather" || rems[0].StreamID != "10" {
		t.Fatalf("follow reminders: %+v", rems)
	}
	// Taken off, it stays off.
	_ = call("DELETE", "/?channel=10&start="+rems[0].Start.Format(time.RFC3339), "", s.handleRemoveLiveReminder)
	_ = json.Unmarshal(call("GET", "/", "", s.handleLiveReminders).Body.Bytes(), &rems)
	if len(rems) != 0 {
		t.Fatalf("dismissed reminder came back: %+v", rems)
	}

	// Record a show that's on now and ends in a few seconds.
	start, stop := time.Now().Add(-time.Minute), time.Now().Add(8*time.Second)
	body := fmt.Sprintf(`{"channelId":"10","title":"Headlines","start":%q,"stop":%q}`, start.Format(time.RFC3339), stop.Format(time.RFC3339))
	w := call("PUT", "/", body, s.handleAddLiveRecording)
	var rec watch.Recording
	if err := json.Unmarshal(w.Body.Bytes(), &rec); err != nil || w.Code != http.StatusOK || rec.ID == 0 {
		t.Fatalf("record: %d %s", w.Code, w.Body)
	}
	deadline = time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if rec, _ = s.WatchRepo.Recording(rec.ID); rec.Status == watch.RecDone || rec.Status == watch.RecFailed {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if rec.Status != watch.RecDone || rec.Size < 10000 || rec.Channel != "News 24" {
		t.Fatalf("recording: %+v", rec)
	}
	// Plays from a signed address, with seeking.
	w = call("GET", "/", "", s.handlePlayLiveRecording, "id", fmt.Sprint(rec.ID))
	var play struct{ URL string }
	_ = json.Unmarshal(w.Body.Bytes(), &play)
	if !strings.HasPrefix(play.URL, "/api/recording?t=") {
		t.Fatalf("play: %d %s", w.Code, w.Body)
	}
	fw := httptest.NewRecorder()
	fr := httptest.NewRequest("GET", play.URL, nil)
	fr.Header.Set("Range", "bytes=0-99")
	s.handleRecordingFile(fw, fr)
	if fw.Code != http.StatusPartialContent || fw.Body.Len() != 100 {
		t.Fatalf("file: %d %d", fw.Code, fw.Body.Len())
	}
	// Deleted with its file.
	_ = call("DELETE", "/", "", s.handleDeleteLiveRecording, "id", fmt.Sprint(rec.ID))
	if _, err := os.Stat(filepath.Join(s.recordingsDir(), rec.File)); !os.IsNotExist(err) {
		t.Errorf("file still there: %v", err)
	}
}
