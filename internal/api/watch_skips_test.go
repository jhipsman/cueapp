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

func TestSkipsLearnedFromTheHousehold(t *testing.T) {
	s := newBareServer(t)
	if _, err := s.db.Exec(`INSERT INTO users (id, username, password_hash, is_admin) VALUES (1, 'dad', 'x', 1)`); err != nil {
		t.Fatal(err)
	}
	owner := &auth.User{ID: 1, Username: "dad", IsAdmin: true}
	routes := s.protectedRoutes()
	do := func(method, target, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		routes.ServeHTTP(w, httptest.NewRequest(method, target, strings.NewReader(body)).WithContext(auth.WithUser(context.Background(), owner)))
		return w
	}
	get := func(target string) skipsAnswer {
		w := do("GET", target, "")
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", target, w.Code, w.Body)
		}
		var a skipsAnswer
		_ = json.Unmarshal(w.Body.Bytes(), &a)
		return a
	}
	if a := get("/api/watch/skips/tv/42?season=1&episode=3&duration=1500"); a.Intro != nil || a.CreditsStart != 0 {
		t.Fatalf("nothing known yet: %+v", a)
	}
	// Jumped from 0:31 to 1:52 in episode 2; pressed Next with 1:30 to go.
	for _, body := range []string{
		`{"kind":"tv","tmdbId":42,"season":1,"segment":"intro","start":31,"end":112}`,
		`{"kind":"tv","tmdbId":42,"season":1,"segment":"credits","start":1410,"duration":1500}`,
		`{"kind":"tv","tmdbId":42,"season":1,"segment":"intro","start":900,"end":1000}`, // not an intro: ignored
	} {
		if w := do("POST", "/api/watch/skips/learn", body); w.Code != http.StatusNoContent {
			t.Fatalf("learn %s: %d %s", body, w.Code, w.Body)
		}
	}
	a := get("/api/watch/skips/tv/42?season=1&episode=3&duration=1440")
	if a.Intro == nil || a.Intro.Start != 31 || a.Intro.End != 112 || a.CreditsStart != 1350 || a.Source != "learned" {
		t.Fatalf("learned: %+v %+v", a, a.Intro)
	}
	// Another season uses the show's intro; without a length, credits say
	// how long before the end.
	a = get("/api/watch/skips/tv/42?season=2&episode=1")
	if a.Intro == nil || a.Intro.End != 112 || a.CreditsFromEnd != 90 {
		t.Fatalf("other season: %+v", a)
	}
	// Next pressed with 9 minutes to go isn't the credits: not learned, and
	// a mark that far back saved before is ignored.
	if w := do("POST", "/api/watch/skips/learn", `{"kind":"tv","tmdbId":43,"season":1,"segment":"credits","start":960,"duration":1500}`); w.Code != http.StatusNoContent {
		t.Fatalf("learn far: %d", w.Code)
	}
	if a = get("/api/watch/skips/tv/43?season=1&episode=1&duration=1500"); a.CreditsStart != 0 || a.CreditsFromEnd != 0 {
		t.Fatalf("9 minutes before the end was learned as the credits: %+v", a)
	}
	if err := s.WatchRepo.SetSkipMark("tv", 44, 1, "credits", 0, 540); err != nil {
		t.Fatal(err)
	}
	if a = get("/api/watch/skips/tv/44?season=1&episode=1"); a.CreditsFromEnd != 0 {
		t.Fatalf("an old 9-minute mark is still used: %+v", a)
	}
}
