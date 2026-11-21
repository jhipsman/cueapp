package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jhipsman/cueapp/internal/auth"
	"github.com/jhipsman/cueapp/internal/iptv"
	"github.com/jhipsman/cueapp/internal/settings"
)

func TestAdultChannelName(t *testing.T) {
	for name, want := range map[string]bool{
		"XXX":              true,
		"FOR ADULTS":       true,
		"Adults Only":      true,
		"US| ADULT 18+":    true,
		"+18 Channels":     true,
		"Playboy TV":       true,
		"Brazzers TV HD":   true,
		"Dorcel TV":        true,
		"Redlight HD":      true,
		"Erotica Channel":  true,
		"Adult Swim":       false,
		"US| ADULT SWIM":   false,
		"Young Adult Pick": false,
		"News 24":          false,
		"Sports":           false,
		"Essex FM":         false,
		"Middlesex TV":     false,
		"Kids":             false,
		"Pornography Doc":  true,
		"Disney Junior HD": false,
	} {
		if got := adultChannelName(name); got != want {
			t.Errorf("%q: got %v, want %v", name, got, want)
		}
	}
	cats, chans := withoutAdultChannels(
		[]iptv.Category{{ID: "1", Name: "News"}, {ID: "2", Name: "XXX Adults"}},
		[]iptv.Channel{{ID: "10", Name: "News 24", Category: "1"}, {ID: "11", Name: "Hot Night", Category: "2"}, {ID: "12", Name: "Playboy TV", Category: "1"}, {ID: "13", Name: "Adult Swim", Category: "1"}},
	)
	if len(cats) != 1 || cats[0].ID != "1" {
		t.Errorf("categories: %+v", cats)
	}
	var names []string
	for _, c := range chans {
		names = append(names, c.Name)
	}
	if strings.Join(names, ",") != "News 24,Adult Swim" {
		t.Errorf("channels: %v", names)
	}
}

func TestLiveTVHidesAdultChannels(t *testing.T) {
	prov := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch {
		case r.URL.Path == "/player_api.php" && q.Get("action") == "":
			w.Write([]byte(`{"user_info":{"auth":1,"status":"Active","exp_date":"1893456000"},"server_info":{"timezone":"UTC"}}`))
		case r.URL.Path == "/player_api.php" && q.Get("action") == "get_live_categories":
			w.Write([]byte(`[{"category_id":"1","category_name":"News"},{"category_id":"2","category_name":"FOR ADULTS"}]`))
		case r.URL.Path == "/player_api.php" && q.Get("action") == "get_live_streams":
			w.Write([]byte(`[{"num":1,"name":"News 24","stream_type":"live","stream_id":10,"category_id":"1"},
				{"num":2,"name":"Late Night","stream_type":"live","stream_id":11,"category_id":"2"},
				{"num":3,"name":"Playboy TV","stream_type":"live","stream_id":12,"category_id":"1"}]`))
		case r.URL.Path == "/xmltv.php":
			w.Write([]byte(`<tv></tv>`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(prov.Close)

	s := newBareServer(t)
	if _, err := s.db.Exec(`INSERT INTO users (id, username, password_hash) VALUES (1, 'a', 'x')`); err != nil {
		t.Fatal(err)
	}
	user := &auth.User{ID: 1, Username: "a"}
	call := func(method, body string, h http.HandlerFunc) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, "/", strings.NewReader(body)).WithContext(auth.WithUser(context.Background(), user))
		w := httptest.NewRecorder()
		h(w, r)
		return w
	}
	if w := call("PUT", `{"server":"`+prov.URL+`","username":"me","password":"pw"}`, s.handlePutIPTV); w.Code != http.StatusOK {
		t.Fatalf("setup: %d %s", w.Code, w.Body)
	}
	list := call("GET", "", s.handleLiveChannels).Body.String()
	if !strings.Contains(list, "News 24") || strings.Contains(list, "Late Night") || strings.Contains(list, "Playboy") || strings.Contains(list, "FOR ADULTS") {
		t.Fatalf("adult channels shown with the switch on (the default): %s", list)
	}

	// Switched off by the owner: the list is read again, with them.
	_ = s.Settings.Set(settings.KeyHideAdult, "0", false)
	s.forgetLiveChannels()
	if list := call("GET", "", s.handleLiveChannels).Body.String(); !strings.Contains(list, "Late Night") || !strings.Contains(list, "Playboy") {
		t.Fatalf("switched off, still hidden: %s", list)
	}
	if s.hideAdult() {
		t.Fatal("switch reads on")
	}
}
