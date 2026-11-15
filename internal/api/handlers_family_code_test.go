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

// The family code: off until an administrator makes one; with it, typed in
// any case and spacing, someone makes their own watching-only account; a
// wrong one makes nothing; changed or turned off, the old one stops working.
func TestFamilyCode(t *testing.T) {
	s := newBareServer(t)
	if _, err := s.db.Exec(`INSERT INTO users (id, username, password_hash, is_admin) VALUES (1, 'dad', 'x', 1)`); err != nil {
		t.Fatal(err)
	}
	owner := &auth.User{ID: 1, Username: "dad", IsAdmin: true}
	admin := func(method, target, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.protectedRoutes().ServeHTTP(w, httptest.NewRequest(method, target, strings.NewReader(body)).WithContext(auth.WithUser(context.Background(), owner)))
		return w
	}
	public := func(method, target, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.Routes().ServeHTTP(w, httptest.NewRequest(method, target, strings.NewReader(body)))
		return w
	}
	open := func() bool {
		var o struct{ Open bool }
		_ = json.Unmarshal(public("GET", "/api/join/family-code", "").Body.Bytes(), &o)
		return o.Open
	}
	if open() {
		t.Fatal("open before any code was made")
	}
	if w := public("POST", "/api/join/family-code", `{"code":"anything","username":"x","password":"a-long-password-1"}`); w.Code != http.StatusNotFound {
		t.Fatalf("join with no code set: %d", w.Code)
	}

	var got struct{ Code string }
	w := admin("POST", "/api/users/family-code", "")
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if w.Code != http.StatusOK || strings.Count(got.Code, "-") != 2 || len(normalCode(got.Code)) < 8 {
		t.Fatalf("new code: %d %s", w.Code, w.Body)
	}
	if !open() {
		t.Fatal("not open once a code is made")
	}
	if w := public("POST", "/api/join/family-code", `{"code":"WRONG-CODE-11","username":"mom","password":"a-long-password-1"}`); w.Code != http.StatusForbidden {
		t.Fatalf("wrong code: %d", w.Code)
	}
	typed := strings.ToLower(strings.ReplaceAll(got.Code, "-", " "))
	w = public("POST", "/api/join/family-code", `{"code":"`+typed+`","name":"Mom","username":"mom","password":"a-long-password-1"}`)
	if w.Code != http.StatusCreated || !strings.Contains(w.Header().Get("Set-Cookie"), "=") {
		t.Fatalf("join: %d %s", w.Code, w.Body)
	}
	var id int64
	if err := s.db.QueryRow(`SELECT id FROM users WHERE username = 'mom'`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	a, err := s.Auth.GetAccount(id)
	if err != nil {
		t.Fatal(err)
	}
	if a.IsAdmin || !a.Permissions.Play || a.Permissions.Manage || a.Permissions.Releases {
		t.Fatalf("a family-code account should watch, not manage: admin=%v %+v", a.IsAdmin, a.Permissions)
	}

	// A code of your own; too short is refused.
	if w := admin("PUT", "/api/users/family-code", `{"code":"abc"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("short code: %d", w.Code)
	}
	if w := admin("PUT", "/api/users/family-code", `{"code":"the maple street house 7"}`); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "THE-MAPLE-STREET-HOUSE-7") {
		t.Fatalf("own code: %d %s", w.Code, w.Body)
	}
	if w := public("POST", "/api/join/family-code", `{"code":"`+got.Code+`","username":"kid","password":"a-long-password-2"}`); w.Code != http.StatusForbidden {
		t.Fatalf("old code after a change: %d", w.Code)
	}

	admin("DELETE", "/api/users/family-code", "")
	if open() {
		t.Fatal("still open after turning it off")
	}
}
