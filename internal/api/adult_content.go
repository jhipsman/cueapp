package api

import (
	"regexp"
	"strings"
	"time"

	"github.com/jhipsman/cueapp/internal/iptv"
	"github.com/jhipsman/cueapp/internal/settings"
)

// Hide adult content (Settings > Streaming, on unless switched off): adult
// Live TV channels and groups are left out of everything Live TV shows (the
// channels, the guide, Sports, Live TV search, playing, recording, logos),
// and adult releases out of release search. Titles are kept clean elsewhere
// already: TMDB is asked without adult titles, and adult releases are never
// grabbed or streamed (adult_release.go).

// hideAdult reports the switch (on unless "0").
func (s *Server) hideAdult() bool {
	if s.Settings == nil {
		return true
	}
	v, _ := s.Settings.Get(settings.KeyHideAdult)
	return v != "0"
}

// adultChannel matches a Live TV channel's or group's name that is adult:
// "XXX", "Adults", "18+", "For Adults", porn and erotic channels, and the
// well-known adult networks. Not "Adult Swim" (cartoons) or "Young Adult".
var adultChannel = regexp.MustCompile(`(?i)(^|[^a-z0-9])(xxx|adults?( only| channels?| tv)?|for adults|18 ?\+|\+ ?18|porn\w*|erotic\w*|playboy|hustler|brazzers|penthouse|dorcel|redlight|babestation|nsfw)([^a-z0-9]|$)`)

var notAdultChannel = regexp.MustCompile(`(?i)adult ?swim|young adults?`)

func adultChannelName(name string) bool {
	return adultChannel.MatchString(notAdultChannel.ReplaceAllString(name, " "))
}

// withoutAdultChannels drops adult groups (and every channel in them) and
// adult channels in any group.
func withoutAdultChannels(cats []iptv.Category, chans []iptv.Channel) ([]iptv.Category, []iptv.Channel) {
	adultCats := map[string]bool{}
	keptCats := make([]iptv.Category, 0, len(cats))
	for _, c := range cats {
		if adultChannelName(c.Name) {
			adultCats[c.ID] = true
			continue
		}
		keptCats = append(keptCats, c)
	}
	keptChans := make([]iptv.Channel, 0, len(chans))
	for _, ch := range chans {
		if adultCats[ch.Category] || adultChannelName(strings.TrimSpace(ch.Name)) {
			continue
		}
		keptChans = append(keptChans, ch)
	}
	return keptCats, keptChans
}

// forgetLiveChannels makes Live TV read its channels again (the switch
// changed: what's hidden changes with it).
func (s *Server) forgetLiveChannels() {
	s.live.mu.Lock()
	defer s.live.mu.Unlock()
	s.live.chans, s.live.cats, s.live.byID = nil, nil, nil
	s.live.listAt = time.Time{}
}
