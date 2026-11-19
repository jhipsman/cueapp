package sports

import (
	"encoding/json"
	"testing"
)

// One game shaped unlike the rest (a score as an object, a season that's
// text) is skipped or read as best it can be; the league still loads.
func TestScoreboardToleratesOddGames(t *testing.T) {
	raw := `{"events":[
	 {"id":"1","date":"2026-11-19T00:00Z","competitions":[{"competitors":[
	   {"homeAway":"home","score":{"value":102,"displayValue":"102"},"team":{"id":"9","displayName":"Heat"}},
	   {"homeAway":"away","score":99,"team":{"id":"15","displayName":"Bucks"}}]}]},
	 {"id":"2","season":"oops","competitions":[]},
	 {"id":"3","date":"2026-11-19T01:00Z","competitions":[{"competitors":[
	   {"homeAway":"home","score":"3","team":{"id":"1","displayName":"A"}},
	   {"homeAway":"away","score":"1","team":{"id":"2","displayName":"B"}}]}]}
	]}`
	var sb scoreboard
	if err := json.Unmarshal([]byte(raw), &sb); err != nil {
		t.Fatal(err)
	}
	if len(sb.Events) != 2 {
		t.Fatalf("want the 2 readable games, got %d", len(sb.Events))
	}
	c := sb.Events[0].Competitions[0].Competitors
	if c[0].Score != "102" || c[1].Score != "99" {
		t.Errorf("scores: %q %q", c[0].Score, c[1].Score)
	}
}
