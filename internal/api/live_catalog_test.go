package api

import (
	"testing"

	"github.com/jhipsman/cueapp/internal/iptv"
)

// The catalog arranges a list; it never loses a channel from it.
func TestCatalogKeepsEveryChannel(t *testing.T) {
	// Most of this list doesn't say a country: it starts on every country,
	// and the channels without one count under each country.
	chans := []iptv.Channel{
		{ID: "1", Name: "ESPN"}, {ID: "2", Name: "CNN"}, {ID: "3", Name: "Nickelodeon"},
		{ID: "4", Name: "US: Fox 5 New York"}, {ID: "5", Name: "UK: BBC One"},
		{ID: "6", Name: "ESPN HD"}, // a copy of ESPN
		{ID: "7", Name: "##### SPORTS #####"},
	}
	c := buildCatalog(nil, chans)
	if c.home != "" {
		t.Errorf("home = %q, want every country for a list that mostly doesn't say", c.home)
	}
	if c.real != 5 || c.noCountry != 3 || len(c.mainOf) != 1 {
		t.Errorf("real %d, no country %d, copies %d", c.real, c.noCountry, len(c.mainOf))
	}
	for _, co := range c.countries {
		if co.Count != 4 { // its own one, and the three that don't say
			t.Errorf("%s counts %d", co.Code, co.Count)
		}
	}
	// Same name, different places: not copies.
	c = buildCatalog(nil, []iptv.Channel{{ID: "a", Name: "US: FOX 5 New York"}, {ID: "b", Name: "US: FOX 11 Los Angeles"}, {ID: "c", Name: "UK: ESPN"}, {ID: "d", Name: "US: ESPN"}})
	if len(c.mainOf) != 0 {
		t.Errorf("folded different channels: %v", c.mainOf)
	}
	if c.home != "US" {
		t.Errorf("home = %q", c.home)
	}
}
