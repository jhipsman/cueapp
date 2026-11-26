package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jhipsman/cueapp/internal/auth"
)

// A profile's own Live TV: renaming a channel, its own lists, the
// languages to filter by, For you (the channels it watches) and Tonight.
func TestLiveListsNamesAndPicks(t *testing.T) {
	s := newBareServer(t)
	prov := fakeIPTV(t)
	user := &auth.User{ID: 1, Username: "a"}
	if _, err := s.db.Exec(`INSERT INTO users (id, username, password_hash) VALUES (1, 'a', 'x')`); err != nil {
		t.Fatal(err)
	}
	req := func(method, target, body string, pathValues ...string) *http.Request {
		r := httptest.NewRequest(method, target, strings.NewReader(body)).WithContext(auth.WithUser(context.Background(), user))
		for i := 0; i+1 < len(pathValues); i += 2 {
			r.SetPathValue(pathValues[i], pathValues[i+1])
		}
		return r
	}
	call := func(method, target, body string, h http.HandlerFunc, pathValues ...string) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		h(w, req(method, target, body, pathValues...))
		return w
	}
	if w := call("PUT", "/", `{"server":"`+prov.URL+`","username":"me","password":"pw"}`, s.handlePutIPTV); w.Code != http.StatusOK {
		t.Fatalf("save login: %d %s", w.Code, w.Body)
	}
	type answer struct {
		Channels []liveChannel
		Lists    []struct {
			ID    int64
			Name  string
			Items []string
		}
		Languages []langCount
		Ready     bool `json:"guideReady"`
	}
	get := func() answer {
		var a answer
		deadline := time.Now().Add(5 * time.Second)
		for {
			_ = json.Unmarshal(call("GET", "/", "", s.handleLiveChannels).Body.Bytes(), &a)
			if a.Ready || time.Now().After(deadline) {
				return a
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	_ = get()

	// Renaming.
	if w := call("PUT", "/", `{"name":"My News"}`, s.handleRenameChannel, "id", "10"); w.Code != http.StatusNoContent {
		t.Fatalf("rename: %d %s", w.Code, w.Body)
	}
	a := get()
	if a.Channels[0].Name != "My News" || !a.Channels[0].Renamed || a.Channels[0].Raw != "News 24" {
		t.Fatalf("renamed: %+v", a.Channels[0])
	}
	if w := call("PUT", "/", `{"name":""}`, s.handleRenameChannel, "id", "10"); w.Code != http.StatusNoContent {
		t.Fatal("rename back")
	}
	if a = get(); a.Channels[0].Name != "News 24" || a.Channels[0].Renamed {
		t.Fatalf("name back: %+v", a.Channels[0])
	}

	// Lists: made, put in order, renamed, removed.
	w := call("POST", "/", `{"name":"Evenings","items":["11","10","11"]}`, s.handleCreateLiveList)
	var made struct{ ID int64 }
	_ = json.Unmarshal(w.Body.Bytes(), &made)
	if w.Code != http.StatusOK || made.ID == 0 {
		t.Fatalf("make list: %d %s", w.Code, w.Body)
	}
	id := fmt.Sprint(made.ID)
	if w := call("PUT", "/", `{"name":"Nights","items":["10","11"]}`, s.handleUpdateLiveList, "id", id); w.Code != http.StatusNoContent {
		t.Fatalf("update list: %d %s", w.Code, w.Body)
	}
	if a = get(); len(a.Lists) != 1 || a.Lists[0].Name != "Nights" || strings.Join(a.Lists[0].Items, ",") != "10,11" {
		t.Fatalf("lists: %+v", a.Lists)
	}
	if w := call("DELETE", "/", "", s.handleUpdateLiveList, "id", id); w.Code != http.StatusNoContent {
		t.Fatal("remove list")
	}
	if a = get(); len(a.Lists) != 0 {
		t.Fatalf("after removing: %+v", a.Lists)
	}

	// For you: the channel this profile watched.
	pid, ok := s.watchProfile(httptest.NewRecorder(), req("GET", "/", ""))
	if !ok {
		t.Fatal("no profile")
	}
	_ = s.WatchRepo.NoteLiveWatch(pid, "10", time.Now())
	// Tonight: the viewer's clock set so the next show starts in its evening.
	utc := time.Now().UTC()
	tz := 19*60 + 10 - (utc.Hour()*60 + utc.Minute())
	for tz < -720 {
		tz += 1440
	}
	for tz > 840 {
		tz -= 1440
	}
	var picks struct{ Tonight, ForYou []liveHit }
	_ = json.Unmarshal(call("GET", fmt.Sprintf("/?tz=%d", tz), "", s.handleLivePicks).Body.Bytes(), &picks)
	if len(picks.ForYou) != 1 || picks.ForYou[0].Channel.ID != "10" || picks.ForYou[0].Show == nil || picks.ForYou[0].Show.Title != "Headlines" {
		t.Fatalf("for you: %+v", picks.ForYou)
	}
	if len(picks.Tonight) != 1 || picks.Tonight[0].Show.Title != "Weather" {
		t.Fatalf("tonight: %+v", picks.Tonight)
	}
}
