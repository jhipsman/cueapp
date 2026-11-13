package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jhipsman/cueapp/internal/auth"
)

// The family invite: an administrator makes a link; with it someone makes
// their own account (a member's that can watch but not change settings),
// and is signed in. A wrong or ended link makes nothing.
func TestFamilyInvite(t *testing.T) {
	s := newBareServer(t)
	if _, err := s.db.Exec(`INSERT INTO users (id, username, password_hash, is_admin) VALUES (1, 'dad', 'x', 1)`); err != nil {
		t.Fatal(err)
	}
	owner := &auth.User{ID: 1, Username: "dad", IsAdmin: true}
	admin := func(method, target string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.protectedRoutes().ServeHTTP(w, httptest.NewRequest(method, target, nil).WithContext(auth.WithUser(context.Background(), owner)))
		return w
	}
	public := func(method, target, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.Routes().ServeHTTP(w, httptest.NewRequest(method, target, strings.NewReader(body)))
		return w
	}
	if w := public("GET", "/api/join/nothing", ""); w.Code != http.StatusNotFound {
		t.Fatalf("no invite yet: %d", w.Code)
	}
	w := admin("POST", "/api/users/invite")
	var inv inviteState
	_ = json.Unmarshal(w.Body.Bytes(), &inv)
	if w.Code != http.StatusOK || len(inv.Token) < 20 || inv.Expires.IsZero() {
		t.Fatalf("make invite: %d %s", w.Code, w.Body)
	}
	if w := public("GET", "/api/join/"+inv.Token, ""); w.Code != http.StatusOK {
		t.Fatalf("check invite: %d %s", w.Code, w.Body)
	}
	if w := public("POST", "/api/join/wrong"+inv.Token, `{"username":"mom","password":"a-long-password-1"}`); w.Code != http.StatusNotFound {
		t.Fatalf("wrong link made an account: %d", w.Code)
	}
	w = public("POST", "/api/join/"+inv.Token, `{"name":"Mom","username":"mom","password":"a-long-password-1"}`)
	if w.Code != http.StatusCreated || !strings.Contains(w.Header().Get("Set-Cookie"), "=") {
		t.Fatalf("join: %d %s (cookie %q)", w.Code, w.Body, w.Header().Get("Set-Cookie"))
	}
	var id int64
	if err := s.db.QueryRow(`SELECT id FROM users WHERE username = 'mom'`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	a, err := s.Auth.GetAccount(id)
	if err != nil {
		t.Fatal(err)
	}
	if a.IsAdmin || !a.Permissions.Play || a.Permissions.Releases || a.Permissions.Manage || a.Permissions.AddDirect {
		t.Fatalf("an invited account should watch, not manage: admin=%v %+v", a.IsAdmin, a.Permissions)
	}
	// Turned off, the link no longer works.
	admin("DELETE", "/api/users/invite")
	if w := public("POST", "/api/join/"+inv.Token, `{"username":"kid","password":"a-long-password-2"}`); w.Code != http.StatusNotFound {
		t.Fatalf("turned-off invite still works: %d", w.Code)
	}
}
