package api

import (
	"cmp"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jhipsman/cueapp/internal/iptv"
	"github.com/jhipsman/cueapp/internal/sports"
)

// Which channel a game is on. A big list carries the same network many
// times over (the US one, the Canadian one, a Spanish one, its news
// channel) and plenty of channels for one game only, so a game is matched
// in this order:
//
//  1. a channel whose guide, around the game's start, names both teams
//     ("NFL Football: Bills at Chiefs"): the event channels lists carry
//     for games on ESPN+, Peacock, League Pass, NHL Network…;
//  2. a channel named for the game ("NHL 03: Sabres vs Bruins");
//  3. the network's own channel, by its plain name (internal/chancat): the
//     American one (or one with no country), never its news or sister
//     channels; a streaming service's channel ("ESPN+ 01") for a game only
//     on one.
//
// Copies of a channel (HD, FHD, backup) count as one; the best picture is
// picked. The other matches come back as more ways to watch.

type gameMatcher struct {
	chans  []iptv.Channel
	cat    *liveCatalog
	favs   map[string]bool
	index  map[string]int // channel id -> index into chans
	shows  []matchShow    // guide entries that look like games
	events []int          // channels whose name looks like a game
}

type matchShow struct {
	ch          int // index into chans
	start, stop time.Time
	text        string // title and description, lowercased
}

var sportsTitleWords = regexp.MustCompile(`(?i)\b(football|basketball|hockey|baseball|soccer|nfl|nba|nhl|mlb|wnba|mls|ncaa|college|premier league|champions league|la liga|serie a|bundesliga)\b`)

func newGameMatcher(chans []iptv.Channel, cat *liveCatalog, guide iptv.Guide, favs map[string]bool, now time.Time) *gameMatcher {
	m := &gameMatcher{chans: chans, cat: cat, favs: favs, index: make(map[string]int, len(chans))}
	for i, ch := range chans {
		m.index[ch.ID] = i
		in := cat.info[ch.ID]
		if in.Divider {
			continue
		}
		low := strings.ToLower(in.Clean)
		if isMatchup(low) || in.Topic == "events" {
			m.events = append(m.events, i)
		}
		if ch.EPGID == "" || guide == nil {
			continue
		}
		for _, p := range guide.Between(ch.EPGID, now.Add(-4*time.Hour), now.Add(30*time.Hour)) {
			desc := p.Desc
			if len(desc) > 200 {
				desc = desc[:200]
			}
			if !isMatchup(p.Title) && !isMatchup(desc) && !sportsTitleWords.MatchString(p.Title) {
				continue
			}
			m.shows = append(m.shows, matchShow{i, p.Start, p.Stop, strings.ToLower(p.Title + " " + desc)})
		}
	}
	return m
}

// teamNames are what a team goes by in a guide or a channel's name,
// lowercased: "buffalo bills", "bills".
func teamNames(t sports.Team) []string {
	var out []string
	for _, n := range []string{t.Name, t.Short} {
		n = strings.ToLower(strings.TrimSpace(n))
		if len(n) >= 3 {
			out = append(out, n)
		}
	}
	return out
}

func namesBoth(text string, a, b []string) bool {
	return hasAny(text, a) && hasAny(text, b)
}

func hasAny(text string, names []string) bool {
	for _, n := range names {
		if i := strings.Index(text, n); i >= 0 {
			// A whole word: "Kings" isn't in "Vikings".
			before := i == 0 || !isWordByte(text[i-1])
			after := i+len(n) == len(text) || !isWordByte(text[i+len(n)])
			if before && after {
				return true
			}
		}
	}
	return false
}

func isWordByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= '0' && c <= '9'
}

// streamingWords are what a streaming service's channels are called in a
// list ("ESPN+ 05", "Peacock Event 3").
var streamingWords = map[string][]string{
	"ESPN+": {"espn+", "espn plus"}, "Peacock": {"peacock"}, "Prime Video": {"prime video", "amazon prime"},
	"Paramount+": {"paramount+", "paramount plus"}, "Apple TV+": {"apple tv", "mls season pass"}, "NFL+": {"nfl+", "nfl plus", "nfl sunday ticket"},
	"NBA League Pass": {"nba league pass", "league pass"}, "MLB.TV": {"mlb.tv", "extra innings"}, "Max": {"max"},
	"Netflix": {"netflix"}, "YouTube": {"youtube"}, "Fox One": {"fox one"}, "ESPN": {"espn"},
}

// otherSide are words that make a channel another of the network's: its
// news, business, Spanish or kids' channel.
var otherSide = regexp.MustCompile(`(?i)\b(news|business|deportes|español|espanol|latino|life|kids|classic|movies?|weather|reality|drama|comedy|crime|justice|gold|xtra|extra)\b`)

type matchHit struct {
	i     int
	score int
}

// find is the channel a game is on and other channels that show it.
func (m *gameMatcher) find(g sports.Game) (*liveChannel, []liveChannel) {
	score := map[int]int{}
	add := func(i, s int) {
		// A copy counts as its main channel.
		if main, ok := m.index[m.cat.mainOf[m.chans[i].ID]]; ok {
			i = main
		}
		if s > score[i] {
			score[i] = s
		}
	}
	away, home := teamNames(g.Away), teamNames(g.Home)
	// 1. The guide names both teams around the start (or now, when it's on).
	from, to := g.Start.Add(-20*time.Minute), g.Start.Add(3*time.Hour)
	for _, sh := range m.shows {
		if sh.stop.Before(from) || sh.start.After(to) {
			continue
		}
		if namesBoth(sh.text, away, home) {
			add(sh.ch, 100)
		}
	}
	// 2. A channel named for the game.
	for _, i := range m.events {
		if namesBoth(strings.ToLower(m.cat.info[m.chans[i].ID].Clean), away, home) {
			add(i, 95)
		}
	}
	// 3. The network's channel (or the streaming service's).
	for _, n := range g.Networks {
		if svc, ok := sports.Streaming[strings.ToLower(n)]; ok {
			m.byName(streamingWords[svc], true, 50, add)
			continue
		}
		m.byName(sports.ChannelWords(n), strings.Contains(strings.ToLower(n), "sport") || strings.Contains(strings.ToLower(n), "espn"), 80, add)
	}
	if len(score) == 0 {
		return nil, nil
	}
	hits := make([]matchHit, 0, len(score))
	for i, s := range score {
		in := m.cat.info[m.chans[i].ID]
		s += in.Quality
		if m.favs[m.chans[i].ID] {
			s += 6
		}
		hits = append(hits, matchHit{i, s})
	}
	sort.Slice(hits, func(a, b int) bool {
		if hits[a].score != hits[b].score {
			return hits[a].score > hits[b].score
		}
		x, y := m.chans[hits[a].i], m.chans[hits[b].i]
		if (x.Num > 0) != (y.Num > 0) {
			return x.Num > 0
		}
		return x.Num < y.Num
	})
	best := m.live(hits[0].i)
	var more []liveChannel
	for _, h := range hits[1:] {
		if len(more) == 4 {
			break
		}
		more = append(more, *m.live(h.i))
	}
	return best, more
}

// byName scores the channels whose plain name is one of words ("ESPN"),
// or starts with one ("CBS New York", "ESPN+ 03"): American ones, or ones
// with no country.
func (m *gameMatcher) byName(words []string, sporty bool, top int, add func(int, int)) {
	if len(words) == 0 {
		return
	}
	for i, ch := range m.chans {
		in := m.cat.info[ch.ID]
		if in.Divider || (in.Country != "" && in.Country != "US") {
			continue
		}
		name := strings.ToLower(in.Clean)
		for _, w := range words {
			switch {
			case name == w:
				add(i, top)
			case strings.HasPrefix(name, w+" ") || (strings.HasSuffix(w, "+") && strings.HasPrefix(name, w)):
				rest := strings.TrimSpace(name[len(w):])
				if otherSide.MatchString(rest) {
					continue
				}
				// FOX vs FOX Sports 1: a sports channel isn't the network's
				// own unless the network is a sports one.
				if !sporty && strings.Contains(rest, "sport") {
					continue
				}
				add(i, top-10)
			default:
				continue
			}
			break
		}
	}
}

func (m *gameMatcher) live(i int) *liveChannel {
	ch := m.chans[i]
	in := m.cat.info[ch.ID]
	lc := liveChannel{ID: ch.ID, Num: ch.Num, Name: cmp.Or(in.Clean, ch.Name), Category: ch.Category, Favorite: m.favs[ch.ID], Catchup: ch.CatchupDays,
		Clean: in.Clean, Country: in.Country, Topic: in.Topic}
	if ch.Logo != "" {
		lc.Logo = "/api/live/logo/" + url.PathEscape(ch.ID)
	}
	return &lc
}
