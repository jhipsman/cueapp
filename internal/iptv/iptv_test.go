package iptv

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func fakeProvider(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("username") != "me" || q.Get("password") != "pw" {
			w.Write([]byte(`{"user_info":{"auth":0}}`))
			return
		}
		switch {
		case r.URL.Path == "/xmltv.php":
			w.Write([]byte(guideXML))
		case q.Get("action") == "":
			w.Write([]byte(`{"user_info":{"auth":1,"status":"Active","exp_date":"1893456000","max_connections":"2","allowed_output_formats":["m3u8","ts"]}}`))
		case q.Get("action") == "get_live_categories":
			w.Write([]byte(`[{"category_id":"1","category_name":" News "},{"category_id":2,"category_name":"Sports"}]`))
		case q.Get("action") == "get_live_streams":
			w.Write([]byte(`[
				{"num":2,"name":"Sports One","stream_type":"live","stream_id":20,"stream_icon":"http://x/s1.png","epg_channel_id":"sports1","category_id":"2"},
				{"num":"1","name":"News 24","stream_type":"live","stream_id":"10","stream_icon":null,"epg_channel_id":"news24","category_id":"1"},
				{"num":3,"name":"A movie","stream_type":"movie","stream_id":30}
			]`))
		}
	}))
}

const guideXML = `<?xml version="1.0" encoding="UTF-8"?>
<tv>
<channel id="news24"><display-name>News 24</display-name></channel>
<programme start="20260101100000 +0000" stop="20260101110000 +0000" channel="news24"><title>Morning</title><desc>The news.</desc></programme>
<programme start="20260101110000 +0000" stop="20260101120000 +0000" channel="news24"><title>Midday</title></programme>
<programme start="20260101120000 +0100" stop="20260101130000 +0100" channel="news24"><title>Lunch</title></programme>
<programme start="20260101100000 +0000" stop="20260101110000 +0000" channel="other"><title>Not wanted</title></programme>
<programme start="20250101100000 +0000" stop="20250101110000 +0000" channel="news24"><title>Too old</title></programme>
</tv>`

func TestAccountClean(t *testing.T) {
	a := Account{Server: " line.example.com:8080/player_api.php?username=x ", Username: " me ", Password: "pw\n"}.Clean()
	if a.Server != "http://line.example.com:8080" || a.Username != "me" || a.Password != "pw" {
		t.Fatalf("got %+v", a)
	}
	if !a.Valid() {
		t.Fatal("should be valid")
	}
	if (Account{Server: "http://x"}).Valid() {
		t.Fatal("no login should not be valid")
	}
}

func TestClient(t *testing.T) {
	srv := fakeProvider(t)
	defer srv.Close()
	ctx := context.Background()

	if _, err := New(Account{Server: srv.URL, Username: "me", Password: "nope"}, nil).Status(ctx); !errors.Is(err, ErrBadLogin) {
		t.Fatalf("bad login: got %v", err)
	}
	c := New(Account{Server: srv.URL, Username: "me", Password: "pw"}, nil)
	st, err := c.Status(ctx)
	if err != nil || !st.Active || st.MaxConnections != 2 || st.Expires.Year() != 2030 {
		t.Fatalf("status: %+v %v", st, err)
	}
	cats, err := c.Categories(ctx)
	if err != nil || len(cats) != 2 || cats[0].Name != "News" || cats[1].ID != "2" {
		t.Fatalf("categories: %+v %v", cats, err)
	}
	chans, err := c.Channels(ctx)
	if err != nil || len(chans) != 2 || chans[0].Name != "News 24" || chans[1].Logo != "http://x/s1.png" || chans[0].EPGID != "news24" {
		t.Fatalf("channels: %+v %v", chans, err)
	}
	if u := c.StreamURL("10", ""); u != srv.URL+"/live/me/pw/10.m3u8" {
		t.Fatalf("stream url %s", u)
	}

	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	g, err := c.LoadGuide(ctx, map[string]bool{"news24": true}, from, from.Add(48*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(g) != 1 || len(g["news24"]) != 3 {
		t.Fatalf("guide: %+v", g)
	}
	now, next := g.At("news24", time.Date(2026, 1, 1, 10, 30, 0, 0, time.UTC))
	if now == nil || now.Title != "Morning" || now.Desc != "The news." || next == nil || next.Title != "Midday" {
		t.Fatalf("at: %+v %+v", now, next)
	}
	// "Lunch" is 12:00 +0100, so 11:00 UTC: it sorts with Midday.
	if g["news24"][1].Start != time.Date(2026, 1, 1, 11, 0, 0, 0, time.UTC) {
		t.Fatalf("times: %+v", g["news24"])
	}
	now, next = g.At("news24", time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC))
	if now != nil || next == nil || next.Title != "Morning" {
		t.Fatalf("before: %+v %+v", now, next)
	}
	if got := g.Between("news24", time.Date(2026, 1, 1, 10, 30, 0, 0, time.UTC), time.Date(2026, 1, 1, 10, 45, 0, 0, time.UTC)); len(got) != 1 {
		t.Fatalf("between: %+v", got)
	}
}

func TestParseGuideCutShort(t *testing.T) {
	cut := guideXML[:strings.Index(guideXML, `<programme start="20260101120000`)] + `<programme start="2026`
	g, err := ParseGuide(strings.NewReader(cut), nil, time.Time{}, time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil || len(g["news24"]) != 2 {
		t.Fatalf("cut guide: %+v %v", g, err)
	}
}
