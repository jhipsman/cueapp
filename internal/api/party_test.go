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

// A watch party, through the real server: one person starts it, another
// joins with the code, and what one does (pause, a jump, a chat line, a
// reaction) reaches the other's open event stream.
func TestWatchParty(t *testing.T) {
	s := newBareServer(t)
	srv := httptest.NewServer(s.Routes())
	t.Cleanup(srv.Close)
	cookieFor := func(name string) string {
		a, err := s.Auth.CreateAccount(name, "a-long-password-1", strings.ToUpper(name[:1])+name[1:], "", "", false)
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
	dad, mom := cookieFor("dad"), cookieFor("mom")
	call := func(cookie, method, path, body string) (int, map[string]any) {
		req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
		req.Header.Set("Cookie", cookie)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", srv.URL)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var out map[string]any
		_ = json.NewDecoder(res.Body).Decode(&out)
		return res.StatusCode, out
	}

	code, started := call(dad, "POST", "/api/party", `{"kind":"movie","tmdbId":42,"title":"A Film","pos":61.5,"playing":true,"client":"dad-tab-123"}`)
	if code != http.StatusOK || started["code"] == nil {
		t.Fatalf("start: %d %v", code, started)
	}
	partyCode := started["code"].(string)

	// Mom sees it on Home, without any link.
	req0, _ := http.NewRequest("GET", srv.URL+"/api/parties", nil)
	req0.Header.Set("Cookie", mom)
	res0, err := http.DefaultClient.Do(req0)
	if err != nil {
		t.Fatal(err)
	}
	var list []map[string]any
	_ = json.NewDecoder(res0.Body).Decode(&list)
	res0.Body.Close()
	if len(list) != 1 || list[0]["code"] != partyCode || list[0]["host"].(map[string]any)["name"] != "Dad" {
		t.Fatalf("parties now: %v", list)
	}

	// Mom joins and listens.
	if code, got := call(mom, "GET", "/api/party/"+partyCode, ""); code != http.StatusOK || got["title"].(map[string]any)["title"] != "A Film" {
		t.Fatalf("look: %d %v", code, got)
	}
	if code, _ := call(mom, "GET", "/api/party/"+partyCode+"/events?client=mom-tab-456", ""); code != http.StatusForbidden {
		t.Fatalf("events before joining: %d", code)
	}
	if code, got := call(mom, "POST", "/api/party/"+partyCode+"/join", `{"client":"mom-tab-456"}`); code != http.StatusOK || len(got["party"].(map[string]any)["members"].([]any)) != 2 {
		t.Fatalf("join: %d %v", code, got)
	}
	req, _ := http.NewRequest("GET", srv.URL+"/api/party/"+partyCode+"/events?client=mom-tab-456", nil)
	req.Header.Set("Cookie", mom)
	res, err := http.DefaultClient.Do(req)
	if err != nil || res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("events: %v %v", err, res)
	}
	defer res.Body.Close()
	events := make(chan string, 64)
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
	wait := func(want string) string {
		deadline := time.After(3 * time.Second)
		for {
			select {
			case e := <-events:
				if strings.Contains(e, want) {
					return e
				}
			case <-deadline:
				t.Fatalf("no event with %q", want)
			}
		}
	}
	wait(`"code":"` + partyCode + `"`) // the first state on opening

	// Dad pauses: Mom hears who did it, and the new state.
	if code, _ := call(dad, "POST", "/api/party/"+partyCode+"/state", `{"client":"dad-tab-123","action":"pause","pos":75}`); code != http.StatusOK {
		t.Fatalf("pause: %d", code)
	}
	wait("Dad paused")
	st := wait(`"playing":false`)
	if !strings.Contains(st, `"pos":75`) {
		t.Errorf("paused state: %s", st)
	}
	// A jump, a chat line and a reaction.
	call(dad, "POST", "/api/party/"+partyCode+"/state", `{"client":"dad-tab-123","action":"seek","pos":600}`)
	wait("Dad jumped to 10:00")
	call(dad, "POST", "/api/party/"+partyCode+"/chat", `{"client":"dad-tab-123","text":"best part"}`)
	wait("best part")
	call(dad, "POST", "/api/party/"+partyCode+"/react", `{"client":"dad-tab-123","emoji":"😂"}`)
	wait("reaction")
	if code, _ := call(dad, "POST", "/api/party/"+partyCode+"/react", `{"client":"dad-tab-123","emoji":"<b>"}`); code != http.StatusBadRequest {
		t.Errorf("unknown reaction: %d", code)
	}
	// Someone can't act as another member's tab.
	if code, _ := call(mom, "POST", "/api/party/"+partyCode+"/state", `{"client":"dad-tab-123","action":"play","pos":1}`); code != http.StatusForbidden {
		t.Errorf("acting as another's tab: %d", code)
	}
	// Leaving says so.
	call(dad, "POST", "/api/party/"+partyCode+"/leave", `{"client":"dad-tab-123"}`)
	wait("Dad left")

	if code, _ := call(mom, "GET", "/api/party/NOPE2345", ""); code != http.StatusNotFound {
		t.Errorf("unknown party: %d", code)
	}
}
