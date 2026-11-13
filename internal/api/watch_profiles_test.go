package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/jhipsman/cueapp/internal/auth"
)

func TestWatchProfilesAndTheSettingsLock(t *testing.T) {
	s := newBareServer(t)
	if _, err := s.db.Exec(`INSERT INTO users (id, username, password_hash, is_admin) VALUES (1, 'dad', 'x', 1)`); err != nil {
		t.Fatal(err)
	}
	owner := &auth.User{ID: 1, Username: "dad", IsAdmin: true}
	routes := s.protectedRoutes()
	req := func(method, target, body, token string, pathValues ...string) *http.Request {
		r := httptest.NewRequest(method, target, strings.NewReader(body)).WithContext(auth.WithUser(context.Background(), owner))
		if token != "" {
			r.Header.Set(profileHeader, token)
		}
		for i := 0; i+1 < len(pathValues); i += 2 {
			r.SetPathValue(pathValues[i], pathValues[i+1])
		}
		return r
	}
	serve := func(r *http.Request) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		routes.ServeHTTP(w, r)
		return w
	}
	settingsCode := func(token string) int { return serve(req("GET", "/api/settings/premiumize", "", token)).Code }
	list := func(token string) profilesPayload {
		var p profilesPayload
		_ = json.Unmarshal(serve(req("GET", "/api/profiles", "", token)).Body.Bytes(), &p)
		return p
	}
	selectProfile := func(id int64, pin string) (int, string) {
		w := serve(req("POST", "/api/profiles/"+itoa(id)+"/select", `{"pin":"`+pin+`"}`, ""))
		var out struct{ Token string }
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out.Token
	}

	// One profile, no PIN: everything works without picking.
	first := list("")
	if len(first.Profiles) != 1 || !first.Profiles[0].Main || first.Active != first.Profiles[0].ID || first.Profiles[0].Name != "dad" {
		t.Fatalf("first list = %+v", first)
	}
	mainID := first.Profiles[0].ID
	if code := settingsCode(""); code != http.StatusOK {
		t.Fatalf("settings with one profile: %d", code)
	}

	// A second profile: now a device has to pick, and only main opens settings.
	if w := serve(req("POST", "/api/profiles", `{"name":"Sam","avatar":"purple"}`, "")); w.Code != http.StatusCreated {
		t.Fatalf("add profile: %d %s", w.Code, w.Body)
	}
	two := list("")
	if len(two.Profiles) != 2 || two.Active != 0 {
		t.Fatalf("after adding = %+v", two)
	}
	samID := two.Profiles[1].ID
	if w := serve(req("GET", "/api/watch/home", "", "")); w.Code != http.StatusConflict {
		t.Fatalf("Watch without a profile: %d", w.Code)
	}
	if code := settingsCode(""); code != http.StatusForbidden {
		t.Fatalf("settings without a profile: %d", code)
	}
	_, sam := selectProfile(samID, "")
	_, dad := selectProfile(mainID, "")
	if code := settingsCode(sam); code != http.StatusForbidden {
		t.Fatalf("settings from Sam's profile: %d", code)
	}
	if w := serve(req("POST", "/api/profiles", `{"name":"Sneaky"}`, sam)); w.Code != http.StatusForbidden {
		t.Fatalf("Sam adding a profile: %d", w.Code)
	}
	if code := settingsCode(dad); code != http.StatusOK {
		t.Fatalf("settings from the main profile: %d", code)
	}
	if got := list(sam).Active; got != samID {
		t.Fatalf("active with Sam's token = %d", got)
	}

	// Progress is per profile.
	if w := serve(req("PUT", "/api/watch/progress", `{"kind":"movie","tmdbId":550,"position":100,"duration":1000}`, sam)); w.Code != http.StatusNoContent {
		t.Fatalf("Sam saving progress: %d %s", w.Code, w.Body)
	}
	if p, ok, _ := s.WatchRepo.GetProgress(samID, "movie", 550, 0, 0); !ok || p.Position != 100 {
		t.Fatal("Sam's progress wasn't saved to Sam")
	}
	if _, ok, _ := s.WatchRepo.GetProgress(mainID, "movie", 550, 0, 0); ok {
		t.Fatal("Sam's progress showed up on the main profile")
	}

	// A PIN on the main profile: old tokens stop working, the right PIN opens it.
	w := serve(req("PUT", "/api/profiles/"+itoa(mainID), `{"pin":"4321"}`, dad, "id", itoa(mainID)))
	var upd struct{ Token string }
	_ = json.Unmarshal(w.Body.Bytes(), &upd)
	if w.Code != http.StatusOK || upd.Token == "" {
		t.Fatalf("set PIN: %d %s", w.Code, w.Body)
	}
	if code := settingsCode(dad); code != http.StatusForbidden {
		t.Fatalf("the token from before the PIN still opens settings: %d", code)
	}
	if code := settingsCode(upd.Token); code != http.StatusOK {
		t.Fatalf("the fresh token after setting the PIN: %d", code)
	}
	if code, _ := selectProfile(mainID, "0000"); code != http.StatusForbidden {
		t.Fatalf("wrong PIN: %d", code)
	}
	if code, tok := selectProfile(mainID, "4321"); code != http.StatusOK || settingsCode(tok) != http.StatusOK {
		t.Fatalf("right PIN: %d", code)
	}

	// The main profile can't be removed; others can.
	if w := serve(req("DELETE", "/api/profiles/"+itoa(mainID), "", upd.Token, "id", itoa(mainID))); w.Code != http.StatusBadRequest {
		t.Fatalf("removing main: %d", w.Code)
	}
	if w := serve(req("DELETE", "/api/profiles/"+itoa(samID), "", upd.Token, "id", itoa(samID))); w.Code != http.StatusNoContent {
		t.Fatalf("removing Sam: %d %s", w.Code, w.Body)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// A family member (not an administrator) manages their own account's
// profiles, and can't touch anyone else's.
func TestMemberManagesOwnProfiles(t *testing.T) {
	s := newBareServer(t)
	if _, err := s.db.Exec(`INSERT INTO users (id, username, password_hash, is_admin) VALUES (1, 'dad', 'x', 1), (2, 'amy', 'x', 0)`); err != nil {
		t.Fatal(err)
	}
	routes := s.protectedRoutes()
	as := func(u *auth.User, method, target, body string, pathValues ...string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, target, strings.NewReader(body)).WithContext(auth.WithUser(context.Background(), u))
		for i := 0; i+1 < len(pathValues); i += 2 {
			r.SetPathValue(pathValues[i], pathValues[i+1])
		}
		w := httptest.NewRecorder()
		routes.ServeHTTP(w, r)
		return w
	}
	dad := &auth.User{ID: 1, Username: "dad", IsAdmin: true}
	amy := &auth.User{ID: 2, Username: "amy"}
	listOf := func(u *auth.User) profilesPayload {
		var p profilesPayload
		_ = json.Unmarshal(as(u, "GET", "/api/profiles", "").Body.Bytes(), &p)
		return p
	}
	dadMain := listOf(dad).Profiles[0].ID
	amyMain := listOf(amy).Profiles[0].ID

	if w := as(amy, "PUT", "/api/profiles/"+itoa(amyMain), `{"avatar":"pink/unicorn"}`, "id", itoa(amyMain)); w.Code != http.StatusOK {
		t.Fatalf("Amy picking her picture: %d %s", w.Code, w.Body)
	}
	if got := listOf(amy).Profiles[0].Avatar; got != "pink/unicorn" {
		t.Errorf("Amy's avatar = %q", got)
	}
	if w := as(amy, "POST", "/api/profiles", `{"name":"Amy's kids","avatar":"yellow/panda"}`); w.Code != http.StatusCreated {
		t.Fatalf("Amy adding a profile: %d %s", w.Code, w.Body)
	}
	if n := len(listOf(dad).Profiles); n != 1 {
		t.Errorf("Amy's profile landed in Dad's account: he has %d", n)
	}
	if w := as(amy, "PUT", "/api/profiles/"+itoa(dadMain), `{"name":"hacked"}`, "id", itoa(dadMain)); w.Code == http.StatusOK {
		t.Fatalf("Amy changed Dad's profile")
	}
	if w := as(amy, "DELETE", "/api/profiles/"+itoa(dadMain), "", "id", itoa(dadMain)); w.Code == http.StatusNoContent {
		t.Fatalf("Amy removed Dad's profile")
	}
	if listOf(dad).Profiles[0].Name != "dad" {
		t.Error("Dad's profile changed")
	}
}
