package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jhipsman/cueapp/internal/auth"
)

// A TV gets a code, a signed-in phone confirms it, and the TV's next ask
// signs it in (a session cookie); the code then works no more.
func TestTVLinkSignsTheTVIn(t *testing.T) {
	s := newBareServer(t)
	if _, err := s.db.Exec(`INSERT INTO users (id, username, password_hash, is_admin) VALUES (5, 'amy', 'x', 0)`); err != nil {
		t.Fatal(err)
	}
	s.LoginLimiter = auth.NewLoginLimiter(20, time.Minute)

	w := httptest.NewRecorder()
	s.handleTVLinkStart(w, httptest.NewRequest("POST", "/api/auth/tv-link", nil))
	var start struct{ Code, Token string }
	if err := json.Unmarshal(w.Body.Bytes(), &start); err != nil || len(start.Code) != 6 || start.Token == "" {
		t.Fatalf("start: %d %s", w.Code, w.Body)
	}
	poll := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/api/auth/tv-link/"+start.Token, nil)
		r.SetPathValue("token", start.Token)
		w := httptest.NewRecorder()
		s.handleTVLinkPoll(w, r)
		return w
	}
	if w := poll(); !strings.Contains(w.Body.String(), "pending") || len(w.Result().Cookies()) != 0 {
		t.Fatalf("before the phone: %d %s", w.Code, w.Body)
	}

	approve := func(code string) int {
		r := httptest.NewRequest("POST", "/api/auth/tv-link/approve", strings.NewReader(`{"code":"`+code+`"}`)).
			WithContext(auth.WithUser(context.Background(), &auth.User{ID: 5, Username: "amy"}))
		w := httptest.NewRecorder()
		s.handleTVLinkApprove(w, r)
		return w.Code
	}
	if code := approve("ZZZZZZ"); code != http.StatusNotFound {
		t.Errorf("a code no TV shows: %d", code)
	}
	if code := approve(strings.ToLower(start.Code)); code != http.StatusOK {
		t.Fatalf("approve: %d", code)
	}
	w = poll()
	if !strings.Contains(w.Body.String(), `"ok":true`) || len(w.Result().Cookies()) == 0 {
		t.Fatalf("after the phone: %d %s, cookies %v", w.Code, w.Body, w.Result().Cookies())
	}
	if w := poll(); w.Code != http.StatusGone {
		t.Errorf("the code worked twice: %d", w.Code)
	}
}
