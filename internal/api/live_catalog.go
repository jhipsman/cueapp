package api

import (
	"sort"

	"github.com/jhipsman/cueapp/internal/chancat"
	"github.com/jhipsman/cueapp/internal/iptv"
)

// The channel list sorted like a TV's, not by the provider's groups: each
// channel's plain name, country and topic, its copies folded into one (the
// best picture shows; the others are its spares), and the provider's
// divider entries left out. See internal/chancat.

type liveCatalog struct {
	info      map[string]chancat.Info
	mainOf    map[string]string   // a copy's id -> the channel shown instead
	spares    map[string][]string // a channel's id -> its copies, best first
	countries []catalogCountry
	home      string // the country to start on ("": every country)
	real      int    // channels, one of each (no copies, no dividers)
	noCountry int    // of those, ones whose country the list doesn't say
}

type catalogCountry struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// liveCatalogFor is the catalog of the list (built once per list).
func (s *Server) liveCatalogFor(cats []iptv.Category, chans []iptv.Channel) *liveCatalog {
	s.live.mu.Lock()
	if c := s.live.catalog; c != nil && len(c.info) == len(chans) {
		s.live.mu.Unlock()
		return c
	}
	s.live.mu.Unlock()
	c := buildCatalog(cats, chans)
	s.live.mu.Lock()
	s.live.catalog = c
	s.live.mu.Unlock()
	return c
}

func buildCatalog(cats []iptv.Category, chans []iptv.Channel) *liveCatalog {
	groupName := make(map[string]string, len(cats))
	for _, c := range cats {
		groupName[c.ID] = c.Name
	}
	c := &liveCatalog{info: make(map[string]chancat.Info, len(chans))}
	copies := make([]chancat.Copy, 0, len(chans))
	counts := map[string]int{}
	for _, ch := range chans {
		in := chancat.Read(ch.Name, groupName[ch.Category])
		c.info[ch.ID] = in
		copies = append(copies, chancat.Copy{ID: ch.ID, Num: ch.Num, Info: in, HasEPG: ch.EPGID != "", Picture: ch.Logo != ""})
	}
	c.mainOf, c.spares = chancat.Fold(copies)
	// Channels whose country the list doesn't say show under every country
	// (picking one must never hide them), so each country counts them too.
	noCountry, real := 0, 0
	for _, ch := range chans {
		in := c.info[ch.ID]
		if in.Divider || c.mainOf[ch.ID] != "" {
			continue
		}
		real++
		if in.Country == "" {
			noCountry++
			continue
		}
		counts[in.Country]++
	}
	c.real, c.noCountry = real, noCountry
	for code, n := range counts {
		n += noCountry
		name := chancat.CountryNames[code]
		if name == "" {
			name = code
		}
		c.countries = append(c.countries, catalogCountry{code, name, n})
	}
	sort.Slice(c.countries, func(i, j int) bool {
		if c.countries[i].Count != c.countries[j].Count {
			return c.countries[i].Count > c.countries[j].Count
		}
		return c.countries[i].Code < c.countries[j].Code
	})
	// Start where most people using Cue are: the US when the list has it,
	// else the country with the most channels. A list that mostly doesn't
	// say starts on every country.
	if noCountry*2 > real {
		c.home = ""
	} else if counts["US"] > 0 {
		c.home = "US"
	} else if len(c.countries) > 0 {
		c.home = c.countries[0].Code
	}
	return c
}

func catalogTopics() []map[string]string {
	out := make([]map[string]string, 0, len(chancat.Topics))
	for _, t := range chancat.Topics {
		out = append(out, map[string]string{"key": t.Key, "name": t.Name})
	}
	return out
}
