package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The LG TV app signs in with headers: its session comes back in a header
// (no cookie), it's let in by that header alone (cookies ignored), and only
// its origin ("null": pages that are files on the TV) gets CORS answers.
func TestAppSignIn(t *testing.T) {
	s := newBareServer(t)
	if _, err := s.Auth.CreateAccount("dad", "a-long-password-1", "Dad", "", "", true); err != nil {
		t.Fatal(err)
	}
	h := s.Routes()
	do := func(method, target, body string, headers map[string]string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, target, strings.NewReader(body))
		for k, v := range headers {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	app := map[string]string{"X-Cue-App": "webos", "Origin": "null", "Content-Type": "application/json"}

	// Signing in from the app: the session in a header, no cookie.
	w := do("POST", "/api/auth/login", `{"username":"dad","password":"a-long-password-1"}`, app)
	token := w.Header().Get("X-Cue-Session")
	if w.Code != http.StatusOK || token == "" {
		t.Fatalf("app sign-in: %d %q %s", w.Code, token, w.Body)
	}
	if c := w.Header().Get("Set-Cookie"); c != "" {
		t.Fatalf("app sign-in set a cookie: %s", c)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "null" {
		t.Fatalf("CORS origin %q", got)
	}
	if exp := w.Header().Get("Access-Control-Expose-Headers"); !strings.Contains(exp, "X-Cue-Session") {
		t.Fatalf("session header not exposed: %q", exp)
	}
	if w.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Fatal("CORS must never allow credentials")
	}

	// The header signs it in.
	withSession := map[string]string{"X-Cue-App": "webos", "X-Cue-Session": token, "Origin": "null"}
	if w := do("GET", "/api/auth/me", "", withSession); w.Code != http.StatusOK {
		t.Fatalf("me with the header: %d %s", w.Code, w.Body)
	}
	// An app request without the header isn't signed in, even with a cookie.
	r := httptest.NewRequest("GET", "/api/auth/me", nil)
	r.Header.Set("X-Cue-App", "webos")
	r.AddCookie(&http.Cookie{Name: "cue_session", Value: token})
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("app request signed in by a cookie: %d", w.Code)
	}
	// A wrong token: not signed in.
	if w := do("GET", "/api/auth/me", "", map[string]string{"X-Cue-App": "webos", "X-Cue-Session": "nope"}); w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token: %d", w.Code)
	}

	// The preflight, from the app's origin.
	w = do("OPTIONS", "/api/watch/home", "", map[string]string{"Origin": "null", "Access-Control-Request-Method": "GET", "Access-Control-Request-Headers": "x-cue-app, x-cue-session"})
	if w.Code != http.StatusNoContent || !strings.Contains(w.Header().Get("Access-Control-Allow-Headers"), "X-Cue-Session") {
		t.Fatalf("preflight: %d %v", w.Code, w.Header())
	}
	// Any other site: no CORS answer.
	w = do("GET", "/api/auth/me", "", map[string]string{"Origin": "https://evil.example", "X-Cue-App": "webos", "X-Cue-Session": token})
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("CORS answer for another site: %v", w.Header())
	}
	// Signing out ends that session.
	if w := do("POST", "/api/auth/logout", "", withSession); w.Code != http.StatusOK {
		t.Fatalf("sign out: %d %s", w.Code, w.Body)
	}
	if w := do("GET", "/api/auth/me", "", withSession); w.Code != http.StatusUnauthorized {
		t.Fatalf("still signed in after signing out: %d", w.Code)
	}
}

func TestTVLinkQR(t *testing.T) {
	s := newBareServer(t)
	h := s.Routes()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/auth/tv-link/qr/ABC234", nil))
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("qr: %d %s", w.Code, w.Header().Get("Content-Type"))
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/auth/tv-link/qr/nope!!", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad code: %d", w.Code)
	}
}
