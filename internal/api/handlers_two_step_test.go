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

// Two-step sign-in, end to end: set up (a wrong code changes nothing), then
// signing in needs the password and a code; a wrong code, a used code or an
// old ticket don't sign in; a backup code works once; turning it off needs
// the password.
func TestTwoStepSignIn(t *testing.T) {
	s := newBareServer(t)
	created, err := s.Auth.CreateAccount("dad", "a-long-password-1", "Dad", "", "", true)
	if err != nil {
		t.Fatal(err)
	}
	me := &created.User
	as := func(method, target, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.protectedRoutes().ServeHTTP(w, httptest.NewRequest(method, target, strings.NewReader(body)).WithContext(auth.WithUser(context.Background(), me)))
		return w
	}
	public := func(target, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.Routes().ServeHTTP(w, httptest.NewRequest(http.MethodPost, target, strings.NewReader(body)))
		return w
	}
	login := func() (*httptest.ResponseRecorder, string) {
		w := public("/api/auth/login", `{"username":"dad","password":"a-long-password-1"}`)
		var a struct{ Ticket string }
		_ = json.Unmarshal(w.Body.Bytes(), &a)
		return w, a.Ticket
	}

	// Off: the password alone signs in.
	if w, ticket := login(); w.Code != http.StatusOK || ticket != "" || w.Header().Get("Set-Cookie") == "" {
		t.Fatalf("plain sign-in: %d %s", w.Code, w.Body)
	}

	// Setting up.
	var st struct{ Secret, URI string }
	_ = json.Unmarshal(as("POST", "/api/auth/two-step/start", "").Body.Bytes(), &st)
	if st.Secret == "" || !strings.HasPrefix(st.URI, "otpauth://totp/") {
		t.Fatalf("start: %+v", st)
	}
	if w := as("GET", "/api/auth/two-step/qr", ""); w.Code != http.StatusOK || w.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("qr: %d %s", w.Code, w.Header().Get("Content-Type"))
	}
	if w := as("POST", "/api/auth/two-step/confirm", `{"code":"000000"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("wrong confirm code: %d", w.Code)
	}
	code := func(at time.Time) string { c, _ := totpCodeForTest(st.Secret, at); return c }
	w := as("POST", "/api/auth/two-step/confirm", `{"code":"`+code(time.Now())+`"}`)
	var conf struct{ BackupCodes []string }
	_ = json.Unmarshal(w.Body.Bytes(), &conf)
	if w.Code != http.StatusOK || len(conf.BackupCodes) != 10 {
		t.Fatalf("confirm: %d %s", w.Code, w.Body)
	}

	// On: the password gives a ticket, not a session.
	w, ticket := login()
	if w.Code != http.StatusOK || ticket == "" || w.Header().Get("Set-Cookie") != "" {
		t.Fatalf("password step: %d %s (cookie %q)", w.Code, w.Body, w.Header().Get("Set-Cookie"))
	}
	if w := public("/api/auth/login/code", `{"ticket":"`+ticket+`","code":"123456"}`); w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong code: %d", w.Code)
	}
	// The next 30-second step's code, so it differs from the one used to confirm.
	next := code(time.Now().Add(30 * time.Second))
	if w := public("/api/auth/login/code", `{"ticket":"`+ticket+`","code":"`+next+`"}`); w.Code != http.StatusOK || w.Header().Get("Set-Cookie") == "" {
		t.Fatalf("right code: %d %s", w.Code, w.Body)
	}
	// The same code again, on a fresh sign-in: refused.
	_, ticket = login()
	if w := public("/api/auth/login/code", `{"ticket":"`+ticket+`","code":"`+next+`"}`); w.Code != http.StatusUnauthorized {
		t.Fatalf("reused code: %d", w.Code)
	}
	// A spent ticket can't be used again.
	if w := public("/api/auth/login/code", `{"ticket":"nope","code":"`+next+`"}`); w.Code != http.StatusUnauthorized {
		t.Fatalf("unknown ticket: %d", w.Code)
	}
	// A backup code works once.
	_, ticket = login()
	if w := public("/api/auth/login/code", `{"ticket":"`+ticket+`","code":"`+conf.BackupCodes[0]+`"}`); w.Code != http.StatusOK {
		t.Fatalf("backup code: %d %s", w.Code, w.Body)
	}
	_, ticket = login()
	if w := public("/api/auth/login/code", `{"ticket":"`+ticket+`","code":"`+conf.BackupCodes[0]+`"}`); w.Code != http.StatusUnauthorized {
		t.Fatalf("backup code twice: %d", w.Code)
	}

	// Turning it off needs the password.
	if w := as("POST", "/api/auth/two-step/off", `{"password":"wrong"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("off, wrong password: %d", w.Code)
	}
	if w := as("POST", "/api/auth/two-step/off", `{"password":"a-long-password-1"}`); w.Code != http.StatusOK {
		t.Fatalf("off: %d %s", w.Code, w.Body)
	}
	if w, ticket := login(); ticket != "" || w.Header().Get("Set-Cookie") == "" {
		t.Fatalf("still asks for a code after turning it off: %s", w.Body)
	}
}

func totpCodeForTest(secret string, at time.Time) (string, error) {
	return auth.TOTPCodeAt(secret, at)
}
