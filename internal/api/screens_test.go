package api

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jhipsman/cueapp/internal/auth"
)

// Send to TV, through the real server: a TV listens, a phone sees it in the
// list, sends it a film and presses pause, and the TV hears both.
func TestSendToTV(t *testing.T) {
	s := newBareServer(t)
	srv := httptest.NewServer(s.Routes())
	t.Cleanup(srv.Close)
	cookieFor := func(name string) string {
		a, err := s.Auth.CreateAccount(name, "a-long-password-1", name, "", "", false)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Auth.SetPermissions(a.ID, auth.Permissions{Movies: true, TV: true, Play: true}); err != nil {
			t.Fatal(err)
		}
		tok, _, err := s.Auth.CreateSession(a.ID)
		if err != nil {
			t.Fatal(err)
		}
		return auth.CookieName + "=" + tok
	}
	tv, phone := cookieFor("dad"), cookieFor("mom")
	do := func(cookie, method, path, body string) (*http.Response, error) {
		req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
		req.Header.Set("Cookie", cookie)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", srv.URL)
		return http.DefaultClient.Do(req)
	}

	res, err := do(tv, "GET", "/api/screens/listen?id=tv-livingroom-1&name=Living%20room", "")
	if err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("listen: %v %v", err, res)
	}
	defer res.Body.Close()
	events := make(chan string, 16)
	go func() {
		sc := bufio.NewScanner(res.Body)
		var ev string
		for sc.Scan() {
			line := sc.Text()
			if strings.HasPrefix(line, "event: ") {
				ev = strings.TrimPrefix(line, "event: ")
			} else if strings.HasPrefix(line, "data: ") {
				events <- ev + " " + strings.TrimPrefix(line, "data: ")
			}
		}
	}()
	wait := func(want string) {
		t.Helper()
		select {
		case got := <-events:
			if !strings.Contains(got, want) {
				t.Fatalf("got %q, want %q", got, want)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("no %q", want)
		}
	}

	// The phone sees the TV.
	var list []map[string]any
	for i := 0; i < 50 && len(list) == 0; i++ {
		r, err := do(phone, "GET", "/api/screens", "")
		if err != nil {
			t.Fatal(err)
		}
		_ = json.NewDecoder(r.Body).Decode(&list)
		r.Body.Close()
		if len(list) == 0 {
			time.Sleep(20 * time.Millisecond)
		}
	}
	if len(list) != 1 || list[0]["name"] != "Living room" || list[0]["online"] != true {
		t.Fatalf("screens: %v", list)
	}

	r, _ := do(phone, "POST", "/api/screens/tv-livingroom-1/play", `{"kind":"movie","tmdbId":603}`)
	if r.StatusCode != http.StatusNoContent {
		t.Fatalf("play: %d", r.StatusCode)
	}
	wait(`play {"episode":0,"from":"mom","kind":"movie","season":0,"tmdbId":603}`)
	do(phone, "POST", "/api/screens/tv-livingroom-1/control", `{"action":"pause"}`)
	wait(`control {"action":"pause"`)
	if r, _ := do(phone, "POST", "/api/screens/tv-livingroom-1/control", `{"action":"explode"}`); r.StatusCode != http.StatusBadRequest {
		t.Errorf("unknown action: %d", r.StatusCode)
	}

	// The TV says what it's playing; the phone sees it.
	do(tv, "POST", "/api/screens/tv-livingroom-1/state", `{"kind":"movie","tmdbId":603,"title":"The Matrix","pos":42,"duration":8160,"playing":false}`)
	r, _ = do(phone, "GET", "/api/screens", "")
	_ = json.NewDecoder(r.Body).Decode(&list)
	r.Body.Close()
	if now := list[0]["now"].(map[string]any); now["title"] != "The Matrix" || now["pos"] != 42.0 {
		t.Fatalf("now: %v", now)
	}
	if r, _ := do(phone, "POST", "/api/screens/nope-nope-nope/play", `{"kind":"movie","tmdbId":603}`); r.StatusCode != http.StatusNotFound {
		t.Errorf("unknown screen: %d", r.StatusCode)
	}
}
