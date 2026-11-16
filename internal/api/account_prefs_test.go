package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jhipsman/cueapp/internal/auth"
)

// An account's settings: known keys and values are kept, others refused;
// each account sees only its own.
func TestAccountPrefs(t *testing.T) {
	s := newBareServer(t)
	a, err := s.Auth.CreateAccount("dad", "a-long-password-1", "", "", "", true)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Auth.CreateAccount("kid", "a-long-password-2", "", "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	as := func(u *auth.User, method, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.protectedRoutes().ServeHTTP(w, httptest.NewRequest(method, "/api/me/prefs", strings.NewReader(body)).WithContext(auth.WithUser(context.Background(), u)))
		return w
	}
	if w := as(&a.User, "PUT", `{"tourSeen":"1","autoplayNext":"0"}`); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"tourSeen":"1"`) {
		t.Fatalf("save: %d %s", w.Code, w.Body)
	}
	if w := as(&a.User, "PUT", `{"tourSeen":"yes"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("bad value: %d", w.Code)
	}
	if w := as(&a.User, "PUT", `{"isAdmin":"1"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("unknown key: %d", w.Code)
	}
	if w := as(&b.User, "GET", ""); w.Body.String() != "{}\n" && w.Body.String() != "{}" {
		t.Fatalf("another account sees: %s", w.Body)
	}
}

// Signing out the other devices keeps this one.
func TestSignOutOthers(t *testing.T) {
	s := newBareServer(t)
	a, err := s.Auth.CreateAccount("dad", "a-long-password-1", "", "", "", true)
	if err != nil {
		t.Fatal(err)
	}
	mine, _, err := s.Auth.CreateSession(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	other, _, _ := s.Auth.CreateSession(a.ID)
	r := httptest.NewRequest("POST", "/api/auth/sign-out-others", nil).WithContext(auth.WithUser(context.Background(), &a.User))
	r.AddCookie(&http.Cookie{Name: auth.CookieName, Value: mine})
	w := httptest.NewRecorder()
	s.protectedRoutes().ServeHTTP(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"signedOut":1`) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if _, err := s.Auth.UserForSession(mine); err != nil {
		t.Errorf("this device was signed out: %v", err)
	}
	if _, err := s.Auth.UserForSession(other); err == nil {
		t.Error("the other device is still signed in")
	}
}
