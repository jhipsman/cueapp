package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jhipsman/cueapp/internal/auth"
)

// An extra guide: a channel the provider has no guide for gets one by its
// name, a channel can be matched to another guide channel by hand, and the
// guide runs a week ahead (descriptions only for the first day and a half).
func TestExtraGuideSources(t *testing.T) {
	s := newBareServer(t)
	prov := fakeIPTV(t)
	user := &auth.User{ID: 1, Username: "a"}
	if _, err := s.db.Exec(`INSERT INTO users (id, username, password_hash) VALUES (1, 'a', 'x')`); err != nil {
		t.Fatal(err)
	}
	call := func(method, target, body string, h http.HandlerFunc, pathValues ...string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, target, strings.NewReader(body)).WithContext(auth.WithUser(context.Background(), user))
		for i := 0; i+1 < len(pathValues); i += 2 {
			r.SetPathValue(pathValues[i], pathValues[i+1])
		}
		w := httptest.NewRecorder()
		h(w, r)
		return w
	}
	// The extra guide: compressed, as most are.
	now := time.Now().UTC()
	f := func(t time.Time) string { return t.Format("20060102150405 -0700") }
	doc := `<tv><channel id="ng.x"><display-name>No Guide HD</display-name></channel>` +
		`<channel id="other.x"><display-name>Other Channel</display-name></channel>` +
		`<programme start="` + f(now.Add(-time.Hour)) + `" stop="` + f(now.Add(time.Hour)) + `" channel="ng.x"><title>Late Movie</title><desc>A film.</desc></programme>` +
		`<programme start="` + f(now.Add(-time.Hour)) + `" stop="` + f(now.Add(time.Hour)) + `" channel="other.x"><title>Other Show</title></programme>` +
		`<programme start="` + f(now.Add(5*24*time.Hour)) + `" stop="` + f(now.Add(5*24*time.Hour+time.Hour)) + `" channel="ng.x"><title>Next Week</title><desc>Gone.</desc></programme></tv>`
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	_, _ = zw.Write([]byte(doc))
	_ = zw.Close()
	extra := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(gz.Bytes()) }))
	t.Cleanup(extra.Close)

	if w := call("PUT", "/", `{"server":"`+prov.URL+`","username":"me","password":"pw"}`, s.handlePutIPTV); w.Code != http.StatusOK {
		t.Fatalf("save login: %d %s", w.Code, w.Body)
	}
	if w := call("PUT", "/", `{"sources":["ftp://nope"]}`, s.handlePutGuideSetup); w.Code != http.StatusBadRequest {
		t.Fatalf("a non-web address: %d", w.Code)
	}
	if w := call("PUT", "/", `{"sources":["`+extra.URL+`/guide.xml.gz"]}`, s.handlePutGuideSetup); w.Code != http.StatusOK {
		t.Fatalf("save sources: %d %s", w.Code, w.Body)
	}
	type channels struct {
		Channels []liveChannel `json:"channels"`
		Ready    bool          `json:"guideReady"`
	}
	nowOf := func(id string) string {
		deadline := time.Now().Add(5 * time.Second)
		for {
			var got channels
			_ = json.Unmarshal(call("GET", "/", "", s.handleLiveChannels).Body.Bytes(), &got)
			for _, ch := range got.Channels {
				if ch.ID == id && ch.Now != nil && got.Ready {
					return ch.Now.Title
				}
			}
			if time.Now().After(deadline) {
				return ""
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	// "No Guide" (no guide from the provider) takes the extra guide's "No Guide HD".
	if got := nowOf("11"); got != "Late Movie" {
		t.Fatalf("matched by name: now %q", got)
	}
	var setup struct {
		Status []guideSourceInfo
	}
	_ = json.Unmarshal(call("GET", "/", "", s.handleGetGuideSetup).Body.Bytes(), &setup)
	if len(setup.Status) != 1 || setup.Status[0].Channels != 2 || setup.Status[0].Matched != 1 || setup.Status[0].Problem != "" {
		t.Fatalf("source status: %+v", setup.Status)
	}
	// A week ahead, without the description.
	var guide struct {
		Programmes map[string][]struct{ Title, Desc string } `json:"programmes"`
	}
	from := now.Add(5*24*time.Hour - time.Hour).Unix()
	_ = json.Unmarshal(call("GET", "/api/live/guide?ids=11&hours=3&from="+strconv.FormatInt(from, 10), "", s.handleLiveGuide).Body.Bytes(), &guide)
	if len(guide.Programmes["11"]) != 1 || guide.Programmes["11"][0].Title != "Next Week" || guide.Programmes["11"][0].Desc != "" {
		t.Fatalf("next week: %+v", guide.Programmes)
	}
	// The guide channels to pick from, and matching "News 24" by hand.
	var opts []struct{ Key, Name, Source string }
	_ = json.Unmarshal(call("GET", "/?q=other", "", s.handleGuideChannels).Body.Bytes(), &opts)
	if len(opts) != 1 || opts[0].Key != "s1:other.x" || !strings.HasPrefix(opts[0].Source, "Extra guide 1") {
		t.Fatalf("guide channels: %+v", opts)
	}
	if w := call("PUT", "/", `{"key":"s1:other.x"}`, s.handleMatchGuide, "id", "10"); w.Code != http.StatusNoContent {
		t.Fatalf("match: %d %s", w.Code, w.Body)
	}
	if got := nowOf("10"); got != "Other Show" {
		t.Fatalf("matched by hand: now %q", got)
	}
	// Back to automatic: the provider's own guide again.
	if w := call("PUT", "/", `{"key":""}`, s.handleMatchGuide, "id", "10"); w.Code != http.StatusNoContent {
		t.Fatalf("unmatch: %d", w.Code)
	}
	if got := nowOf("10"); got != "Headlines" {
		t.Fatalf("back to the provider's: now %q", got)
	}
}
