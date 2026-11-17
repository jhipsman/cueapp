package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jhipsman/cueapp/internal/auth"
)

// Phone alerts, through the real server: a device turns alerts on, an alert
// is kept for its profile, and the device's worker fetches it once.
func TestPushAlerts(t *testing.T) {
	s := newBareServer(t)
	srv := httptest.NewServer(s.Routes())
	t.Cleanup(srv.Close)
	a, err := s.Auth.CreateAccount("dad", "a-long-password-1", "Dad", "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Auth.SetPermissions(a.ID, auth.Permissions{Movies: true, TV: true, Play: true})
	tok, _, _ := s.Auth.CreateSession(a.ID)
	cookie := auth.CookieName + "=" + tok
	call := func(path, body string) (int, string) {
		req, _ := http.NewRequest("POST", srv.URL+path, strings.NewReader(body))
		if body == "" {
			req.Method = "GET"
		}
		req.Header.Set("Cookie", cookie)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", srv.URL)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var b strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := res.Body.Read(buf)
			b.Write(buf[:n])
			if err != nil {
				break
			}
		}
		return res.StatusCode, b.String()
	}

	if code, body := call("/api/push/key", ""); code != 200 || !strings.Contains(body, `"key"`) {
		t.Fatalf("key: %d %s", code, body)
	}
	// Only push services' addresses are taken.
	if code, _ := call("/api/push/subscribe", `{"endpoint":"https://evil.example/x"}`); code != http.StatusBadRequest {
		t.Fatalf("odd endpoint: %d", code)
	}
	ep := "https://fcm.googleapis.com/fcm/send/device-1"
	if code, body := call("/api/push/subscribe", `{"endpoint":"`+ep+`"}`); code != http.StatusNoContent {
		t.Fatalf("subscribe: %d %s", code, body)
	}
	pid, ok := s.WatchRepo.PushSubProfile(ep)
	if !ok {
		t.Fatal("subscription not kept")
	}
	if ok, _ := s.WatchRepo.QueueAlert(pid, "ep:1:1:2", "New episode of Show", "S1:E2", "/watch/tv/1"); !ok {
		t.Fatal("alert not queued")
	}
	if ok, _ := s.WatchRepo.QueueAlert(pid, "ep:1:1:2", "again", "", ""); ok {
		t.Fatal("the same news queued twice")
	}
	code, body := call("/api/push/pending", `{"endpoint":"`+ep+`"}`)
	var alerts []map[string]any
	_ = json.Unmarshal([]byte(body), &alerts)
	if code != 200 || len(alerts) != 1 || alerts[0]["title"] != "New episode of Show" {
		t.Fatalf("pending: %d %s", code, body)
	}
	if _, body := call("/api/push/pending", `{"endpoint":"`+ep+`"}`); strings.TrimSpace(body) != "[]" {
		t.Fatalf("fetched twice: %s", body)
	}
	call("/api/push/unsubscribe", `{"endpoint":"`+ep+`"}`)
	if _, ok := s.WatchRepo.PushSubProfile(ep); ok {
		t.Fatal("still subscribed")
	}
}
