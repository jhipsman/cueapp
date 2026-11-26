package chancat

import "testing"

func TestRead(t *testing.T) {
	cases := []struct {
		name, group    string
		clean, country string
		topic          string
		divider        bool
	}{
		{"US ★ ESPN FHD ◉", "USA SPORTS", "ESPN", "US", "sports", false},
		{"UK: BBC One HD", "UK | ENTERTAINMENT", "BBC One", "UK", "general", false},
		{"|UK| Sky News", "", "Sky News", "UK", "news", false},
		{"[US] Cartoon Network", "", "Cartoon Network", "US", "kids", false},
		{"BBC Two ᴴᴰ", "UK | ENTERTAINMENT FHD", "BBC Two", "UK", "general", false},
		{"##### UK BACKUP #####", "UK | ENTERTAINMENT", "UK BACKUP", "UK", "general", true},
		{"-----------", "", "-----------", "", "general", true},
		{"USA Network", "USA ENTERTAINMENT", "USA Network", "US", "general", false},
		{"US: CBS New York (WCBS)", "US LOCALS", "CBS New York (WCBS)", "US", "local", false},
		{"Friends 24/7", "UK | 24/7", "Friends 24/7", "UK", "247", false},
		{"HBO", "", "HBO", "", "movies", false},
		{"DE: Das Erste HD", "DE | VOLLPROGRAMM HD", "Das Erste", "DE", "general", false},
	}
	for _, c := range cases {
		got := Read(c.name, c.group)
		if got.Clean != c.clean || got.Country != c.country || got.Topic != c.topic || got.Divider != c.divider {
			t.Errorf("Read(%q, %q) = %+v; want clean %q country %q topic %q divider %v", c.name, c.group, got, c.clean, c.country, c.topic, c.divider)
		}
	}
}

func TestFold(t *testing.T) {
	list := []Copy{
		{ID: "1", Num: 10, Info: Read("US: ESPN SD", "")},
		{ID: "2", Num: 11, Info: Read("US: ESPN FHD", "")},
		{ID: "3", Num: 12, Info: Read("US: ESPN HD", "")},
		{ID: "4", Num: 13, Info: Read("UK: ESPN", "")},
		{ID: "5", Num: 14, Info: Read("US: ESPN Backup", "")}, // a different name: its own channel
	}
	mainOf, copies := Fold(list)
	if mainOf["1"] != "2" || mainOf["3"] != "2" || mainOf["4"] != "" || mainOf["2"] != "" {
		t.Errorf("mainOf = %v", mainOf)
	}
	if len(copies["2"]) != 2 || copies["2"][0] != "3" {
		t.Errorf("copies = %v (want the HD copy before the SD one)", copies)
	}
}

func TestCleanSymbolsAndRank(t *testing.T) {
	for name, want := range map[string]string{
		"⬤LOCAL NEWS ⬤":              "LOCAL NEWS",
		"24/7 ,24/7 Trust Me (2017)": "24/7 Trust Me (2017)",
		"US ❖ ABC HD":                "ABC",
		"🔴 ESPN":                     "ESPN",
	} {
		if got := Read(name, "").Clean; got != want {
			t.Errorf("%q: %q, want %q", name, got, want)
		}
	}
	if Rank(Read("US: ABC", "")) != 0 || Rank(Read("US: Hallmark", "")) != 1 || Rank(Read("24/7 Friends", "24/7")) != 3 {
		t.Errorf("ranks: %d %d %d", Rank(Read("US: ABC", "")), Rank(Read("US: Hallmark", "")), Rank(Read("24/7 Friends", "24/7")))
	}
}

func TestEventFeedsAreEvents(t *testing.T) {
	for _, name := range []string{
		"UCL 01 Sabah vs Barcelona 7:45 CET", "UCL 03 PSG vs Roma 20:00 CET", "NBA 12: Lakers @ Celtics",
		"PPV 3", "EPL 04 Arsenal v Chelsea", "Bills at Chiefs 8:20pm ET",
	} {
		if got := Read(name, "").Topic; got != "events" {
			t.Errorf("%q: topic %q, want events", name, got)
		}
		if r := Rank(Read(name, "")); r != 2 {
			t.Errorf("%q: rank %d, want 2", name, r)
		}
	}
	for _, name := range []string{"ESPN", "Channel V", "FOX 5 New York", "Sky Sports 1", "BBC One", "TSN 2", "NBC Sports Boston"} {
		if got := Read(name, "").Topic; got == "events" {
			t.Errorf("%q counted as an event", name)
		}
	}
}
