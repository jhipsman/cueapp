package api

import "testing"

func TestTidyChannelName(t *testing.T) {
	for in, want := range map[string]string{
		"US ★ ESPN HD ◉":                 "ESPN",
		"US ★ NEXT LEVEL SPORTS HD":      "NEXT LEVEL SPORTS",
		"US ★ WILLOW SPORTS FHD":         "WILLOW SPORTS",
		"|UK| Sky Sports Main Event FHD": "Sky Sports Main Event",
		"US: Fox 8 New Orleans HD":       "Fox 8 New Orleans",
		"ESPN":                           "ESPN",
		"HD":                             "HD",
		"BBC One":                        "BBC One",
	} {
		if got := tidyChannelName(in); got != want {
			t.Errorf("tidyChannelName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSplitGameTitle(t *testing.T) {
	k, m := splitGameTitle("NBA Basketball : Milwaukee Bucks at Miami Heat")
	if k != "NBA Basketball" || m != "Milwaukee Bucks at Miami Heat" {
		t.Errorf("got %q / %q", k, m)
	}
	k, m = splitGameTitle("Socca World Cup 2026")
	if k != "" || m != "Socca World Cup 2026" {
		t.Errorf("got %q / %q", k, m)
	}
	_, a := splitGameTitle("Baloncesto NBA : Milwaukee Bucks at Miami Heat")
	if a != m && a != "Milwaukee Bucks at Miami Heat" {
		t.Errorf("languages should share a matchup, got %q", a)
	}
}
