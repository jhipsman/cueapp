package sports

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A trimmed copy of ESPN's scoreboard: a live national game, a game later
// with only local TV, and a streaming-only game.
const nflBoard = `{"events":[
 {"id":"401","date":"2026-11-22T18:00Z","season":{"type":2},
  "status":{"type":{"state":"in","shortDetail":"Q3 4:12"}},
  "competitions":[{"competitors":[
    {"homeAway":"home","score":"17","team":{"id":"12","displayName":"Kansas City Chiefs","shortDisplayName":"Chiefs","abbreviation":"KC","logo":"https://a.espncdn.com/kc.png","color":"e31837"},"records":[{"summary":"9-2"}]},
    {"homeAway":"away","score":"21","team":{"id":"2","displayName":"Buffalo Bills","shortDisplayName":"Bills","abbreviation":"BUF","logo":"https://a.espncdn.com/buf.png"},"records":[{"summary":"8-3"}]}],
   "broadcasts":[{"market":"national","names":["CBS"]}],
   "geoBroadcasts":[{"market":{"type":"National"},"media":{"shortName":"CBS"}},{"market":{"type":"National"},"media":{"shortName":"Paramount+"}}],
   "notes":[]}]},
 {"id":"402","date":"2026-11-22T21:25Z","season":{"type":3},
  "status":{"type":{"state":"pre","shortDetail":"Sun, November 22nd at 4:25 PM EST"}},
  "competitions":[{"competitors":[
    {"homeAway":"home","score":"0","team":{"id":"6","displayName":"Dallas Cowboys","shortDisplayName":"Cowboys","abbreviation":"DAL"}},
    {"homeAway":"away","score":"0","team":{"id":"21","displayName":"Philadelphia Eagles","shortDisplayName":"Eagles","abbreviation":"PHI"}}],
   "geoBroadcasts":[{"market":{"type":"Home"},"media":{"shortName":"FOX"}}],
   "notes":[{"headline":"NFC Wild Card"}]}]},
 {"id":"403","date":"2026-11-23T01:15Z","season":{"type":2},
  "status":{"type":{"state":"pre","shortDetail":"Sun 8:15 PM"}},
  "competitions":[{"competitors":[
    {"homeAway":"home","team":{"id":"9","displayName":"Green Bay Packers","shortDisplayName":"Packers"}},
    {"homeAway":"away","team":{"id":"3","displayName":"Chicago Bears","shortDisplayName":"Bears"}}],
   "broadcasts":[{"market":"national","names":["Prime Video"]}]}]}
]}`

func TestScoreboard(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.String())
		if strings.HasPrefix(r.URL.Path, "/football/nfl/scoreboard") {
			w.Write([]byte(nflBoard))
			return
		}
		w.Write([]byte(`{"events":[]}`))
	}))
	defer srv.Close()
	c := New(srv.URL)
	c.http = srv.Client()
	c.now = func() time.Time { return time.Date(2026, 11, 22, 20, 0, 0, 0, time.UTC) }
	nfl, _ := LeagueByKey("nfl")
	games, err := c.Games(context.Background(), nfl)
	if err != nil || len(games) != 3 {
		t.Fatalf("games: %v %d", err, len(games))
	}
	// One day at a time (US Eastern: 3 PM on the 22nd), no range or limit:
	// ESPN refuses those.
	if len(asked) != 2 || !strings.HasSuffix(asked[0], "?dates=20261122") || !strings.HasSuffix(asked[1], "?dates=20261123") {
		t.Fatalf("asked %v", asked)
	}
	live := games[0]
	if live.Title() != "Bills at Chiefs" || live.State != "in" || live.Away.Score != "21" || live.Home.Record != "9-2" || live.Detail != "Q3 4:12" {
		t.Fatalf("live game: %+v", live)
	}
	if len(live.Networks) != 2 || live.Networks[0] != "CBS" || live.Networks[1] != "Paramount+" {
		t.Fatalf("networks: %v", live.Networks)
	}
	wild := games[1]
	if !wild.Postseason || wild.Note != "NFC Wild Card" || wild.Home.Score != "" || len(wild.Networks) != 1 || wild.Networks[0] != "FOX" {
		t.Fatalf("wild card: %+v", wild)
	}
	// A team the scoreboard gives no logo gets ESPN's usual address.
	if wild.Home.Logo != "https://a.espncdn.com/i/teamlogos/nfl/500/dal.png" || live.Home.Logo != "https://a.espncdn.com/kc.png" {
		t.Fatalf("logos: %q %q", wild.Home.Logo, live.Home.Logo)
	}
	if games[2].Networks[0] != "Prime Video" || ChannelWords("Prime Video") != nil {
		t.Fatal("streaming-only game")
	}
	// Read once a minute.
	_, _ = c.Games(context.Background(), nfl)
	if len(asked) != 2 { // today and tomorrow, once
		t.Fatalf("not cached: %d asks", len(asked))
	}
	// The playoff game later beats... the live regular game now? Live wins
	// over a game hours away; a favorite team beats both.
	now := c.now()
	if Bigness(live, now, false) <= Bigness(games[2], now, false) {
		t.Fatal("live game should beat the night game")
	}
	if Bigness(games[2], now, true) <= Bigness(live, now, false) {
		t.Fatal("a favorite team's game should come first")
	}
	if w := ChannelWords("FS1"); len(w) == 0 || w[0] != "fs1" {
		t.Fatalf("fs1 words %v", w)
	}
}

// ESPN refusing a dated ask (400): the plain scoreboard instead.
func TestScoreboardFallsBackToToday(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.String())
		if r.URL.RawQuery != "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Write([]byte(nflBoard))
	}))
	defer srv.Close()
	c := New(srv.URL)
	c.http = srv.Client()
	c.now = func() time.Time { return time.Date(2026, 11, 22, 20, 0, 0, 0, time.UTC) }
	nfl, _ := LeagueByKey("nfl")
	games, err := c.Games(context.Background(), nfl)
	if err != nil || len(games) != 3 {
		t.Fatalf("games: %v %d (asked %v)", err, len(games), asked)
	}
}
