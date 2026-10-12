package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jhipsman/cueapp/internal/auth"
)

func TestSessionCookieFromBeforeTheRenameStillSignsIn(t *testing.T) {
	svc := newTestService(t)
	u, err := svc.CreateUser("ryan", "hunter2hunter2")
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := svc.CreateSession(u.ID)
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{auth.CookieName, "mediarium_session"} {
		r := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
		r.AddCookie(&http.Cookie{Name: name, Value: token})
		got, err := svc.UserFromRequest(r)
		if err != nil || got.ID != u.ID {
			t.Errorf("cookie %s: user %+v, err %v", name, got, err)
		}
	}
}

func TestSigningInReplacesTheOldCookie(t *testing.T) {
	w := httptest.NewRecorder()
	auth.SetSessionCookie(w, "tok", time.Now().Add(time.Hour), false)
	set := map[string]*http.Cookie{}
	for _, c := range w.Result().Cookies() {
		set[c.Name] = c
	}
	if c := set[auth.CookieName]; c == nil || c.Value != "tok" {
		t.Errorf("new cookie: %+v", c)
	}
	if c := set["mediarium_session"]; c == nil || c.MaxAge >= 0 {
		t.Errorf("the old cookie should be expired: %+v", c)
	}

	w = httptest.NewRecorder()
	auth.ClearSessionCookie(w, false)
	if n := len(w.Result().Cookies()); n != 2 {
		t.Errorf("signing out should expire both names, got %d cookies", n)
	}
}
