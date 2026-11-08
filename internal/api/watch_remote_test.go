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

// The phone's words reach the TV's Search through its code.
func TestRemoteSearchFromAPhone(t *testing.T) {
	s := newBareServer(t)
	if _, err := s.db.Exec(`INSERT INTO users (id, username, password_hash, is_admin) VALUES (1, 'dad', 'x', 1)`); err != nil {
		t.Fatal(err)
	}
	owner := &auth.User{ID: 1, Username: "dad", IsAdmin: true}
	routes := s.protectedRoutes()
	do := func(method, target, body string) (int, remoteSearch) {
		w := httptest.NewRecorder()
		routes.ServeHTTP(w, httptest.NewRequest(method, target, strings.NewReader(body)).WithContext(auth.WithUser(context.Background(), owner)))
		var out remoteSearch
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}
	if code, got := do("GET", "/api/watch/remote/K7Q2", ""); code != http.StatusOK || got.Seq != 0 || got.Q != "" {
		t.Fatalf("before any words: %d %+v", code, got)
	}
	do("PUT", "/api/watch/remote/K7Q2", `{"q":"toy sto"}`)
	do("PUT", "/api/watch/remote/k7q2", `{"q":"  toy story  "}`)
	if code, got := do("GET", "/api/watch/remote/K7Q2", ""); code != http.StatusOK || got.Q != "toy story" || got.Seq != 2 {
		t.Fatalf("after the phone typed: %d %+v", code, got)
	}
	if code, _ := do("GET", "/api/watch/remote/no!", ""); code != http.StatusBadRequest {
		t.Fatalf("a made-up code: %d", code)
	}
}
