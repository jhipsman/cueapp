package api

import "testing"

func TestSportsScore(t *testing.T) {
	for _, c := range []struct {
		title, desc, channel string
		league               string
		game                 bool
	}{
		{"NFL Football: Bills at Chiefs", "", "CBS", "NFL", true},
		{"Lakers vs. Celtics", "NBA basketball from TD Garden.", "ESPN", "NBA", true},
		{"NFL Countdown", "", "ESPN", "", false},
		{"SportsCenter", "", "ESPN", "", false},
		{"Arsenal v Chelsea", "", "Sky Sports Premier League", "", true},
		{"Jeopardy!", "", "ABC", "", false},
		{"Duke at North Carolina", "", "ESPN2", "", true},
		{"Bills at Chiefs", "", "Food Network", "", false},
	} {
		league, score := sportsScore(c.title, c.desc, c.channel, "")
		if (score > 0) != c.game || league != c.league {
			t.Errorf("%q on %s: league %q score %d, want %q game=%v", c.title, c.channel, league, score, c.league, c.game)
		}
	}
	_, nfl := sportsScore("Bills at Chiefs", "NFL", "CBS", "")
	_, nba := sportsScore("Lakers vs Celtics", "NBA", "TNT", "")
	if nfl <= nba {
		t.Fatal("an NFL game should beat an NBA game")
	}
}
