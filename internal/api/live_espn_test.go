package api

import (
	"testing"
	"time"

	"github.com/jhipsman/cueapp/internal/iptv"
	"github.com/jhipsman/cueapp/internal/sports"
)

func matcherFor(chans []iptv.Channel, guide iptv.Guide, favs map[string]bool, now time.Time) *gameMatcher {
	return newGameMatcher(chans, buildCatalog(nil, chans), guide, favs, now)
}

func gameOn(networks ...string) sports.Game {
	return sports.Game{
		League: "nfl", Start: time.Date(2026, 11, 29, 18, 0, 0, 0, time.UTC), State: "pre", Networks: networks,
		Away: sports.Team{Name: "Buffalo Bills", Short: "Bills", Abbr: "BUF"},
		Home: sports.Team{Name: "Kansas City Chiefs", Short: "Chiefs", Abbr: "KC"},
	}
}

func TestGameChannelByNetwork(t *testing.T) {
	chans := []iptv.Channel{
		{ID: "a", Num: 4, Name: "US: CBS News 24/7"},
		{ID: "b", Num: 5, Name: "US: CBS New York HD"},
		{ID: "c", Num: 9, Name: "US: CBS Sports Network"},
		{ID: "d", Num: 20, Name: "US: ESPN2 HD"},
		{ID: "e", Num: 21, Name: "US: ESPN HD"},
		{ID: "e2", Num: 22, Name: "US: ESPN FHD"},
		{ID: "f", Num: 30, Name: "US: FOX 5 New York"},
		{ID: "g", Num: 31, Name: "US: FOX Sports 1"},
		{ID: "h", Num: 32, Name: "US: FOX Business"},
		{ID: "i", Num: 3, Name: "US: CBS Los Angeles"},
		{ID: "j", Num: 1, Name: "AR: ESPN"},       // another country's ESPN
		{ID: "k", Num: 2, Name: "USA | Hallmark"}, // "USA" isn't USA Network
		{ID: "l", Num: 40, Name: "US: USA Network HD"},
		{ID: "m", Num: 41, Name: "UK: CBS Reality"},
		{ID: "n", Num: 50, Name: "US: ESPN Deportes"},
	}
	m := matcherFor(chans, nil, nil, time.Now())
	for network, want := range map[string]string{
		"CBS": "b", "ESPN": "e2", "ESPN2": "d", "FOX": "f", "FS1": "g", "CBSSN": "c", "USA Net": "l", "Prime Video": "", "NBC": "",
	} {
		got, _ := m.find(gameOn(network))
		id := ""
		if got != nil {
			id = got.ID
		}
		if id != want {
			t.Errorf("%s: channel %q, want %q", network, id, want)
		}
	}
	// A favorite channel wins over a better picture.
	fav := matcherFor(chans, nil, map[string]bool{"i": true}, time.Now())
	if got, _ := fav.find(gameOn("CBS")); got == nil || got.ID != "i" {
		t.Fatalf("favorite: %+v", got)
	}
}

func TestGameChannelOnlyStreaming(t *testing.T) {
	g := gameOn("Prime Video")
	chans := []iptv.Channel{
		{ID: "a", Num: 1, Name: "US: NFL Network"},
		{ID: "b", Num: 2, Name: "US: Prime Video 01", EPGID: "pv1"},
		{ID: "c", Num: 3, Name: "US: Prime Video 02", EPGID: "pv2"},
		{ID: "d", Num: 4, Name: "NFL 07: Bills vs Chiefs"},
		{ID: "e", Num: 5, Name: "US: Bills Network"},
	}
	guide := iptv.Guide{
		"pv1": {{Title: "Thursday Night Football", Desc: "Lions at Packers.", Start: g.Start, Stop: g.Start.Add(3 * time.Hour)}},
		"pv2": {{Title: "NFL Football", Desc: "Buffalo Bills at Kansas City Chiefs. From Arrowhead.", Start: g.Start.Add(-10 * time.Minute), Stop: g.Start.Add(3 * time.Hour)}},
	}
	m := matcherFor(chans, guide, nil, g.Start.Add(-time.Hour))
	got, more := m.find(g)
	if got == nil || got.ID != "c" {
		t.Fatalf("got %+v, want the Prime channel whose guide has the game", got)
	}
	ids := map[string]bool{}
	for _, c := range more {
		ids[c.ID] = true
	}
	if !ids["d"] || !ids["b"] || ids["a"] || ids["e"] {
		t.Errorf("more = %+v; want the game's own channel and the other Prime one, not NFL Network or a team's", more)
	}
}

func TestGameChannelWholeNames(t *testing.T) {
	g := gameOn()
	g.Away = sports.Team{Name: "Los Angeles Kings", Short: "Kings"}
	g.Home = sports.Team{Name: "Minnesota Wild", Short: "Wild"}
	chans := []iptv.Channel{{ID: "a", Name: "NHL 01: Vikings vs Wild Country"}}
	if got, _ := matcherFor(chans, nil, nil, time.Now()).find(g); got != nil {
		t.Errorf("Kings found in Vikings: %+v", got)
	}
}
