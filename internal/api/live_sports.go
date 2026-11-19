package api

import (
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Sports on Live TV: the games on now and coming up in the next day, found
// in the TV guide, with the next big game on top (a league game between
// two teams, the bigger the league the better, sooner first).

// sportsLeagues are leagues and events by how big a game in them is (the
// first that matches a programme counts).
var sportsLeagues = []struct {
	name  string
	re    *regexp.Regexp
	score int
}{
	{"Super Bowl", regexp.MustCompile(`(?i)\bsuper bowl\b`), 100},
	{"World Cup", regexp.MustCompile(`(?i)\bworld cup\b`), 95},
	{"NFL", regexp.MustCompile(`(?i)\b(nfl|sunday night football|monday night football|thursday night football)\b`), 90},
	{"NBA", regexp.MustCompile(`(?i)\bnba\b`), 80},
	{"College football", regexp.MustCompile(`(?i)\b(college football|ncaa football|cfp)\b`), 78},
	{"MLB", regexp.MustCompile(`(?i)\b(mlb|world series)\b`), 75},
	{"NHL", regexp.MustCompile(`(?i)\b(nhl|stanley cup)\b`), 72},
	{"Champions League", regexp.MustCompile(`(?i)\bchampions league\b`), 72},
	{"Premier League", regexp.MustCompile(`(?i)\bpremier league\b`), 70},
	{"College basketball", regexp.MustCompile(`(?i)\b(college basketball|ncaa basketball|march madness)\b`), 68},
	{"UFC", regexp.MustCompile(`(?i)\bufc\b`), 66},
	{"Formula 1", regexp.MustCompile(`(?i)\b(formula 1|f1 grand prix|grand prix)\b`), 64},
	{"MLS", regexp.MustCompile(`(?i)\bmls\b`), 55},
	{"WNBA", regexp.MustCompile(`(?i)\bwnba\b`), 55},
	{"Golf", regexp.MustCompile(`(?i)\b(pga|masters tournament|ryder cup)\b`), 50},
	{"Tennis", regexp.MustCompile(`(?i)\b(wimbledon|us open tennis|atp|wta)\b`), 50},
	{"Boxing", regexp.MustCompile(`(?i)\bboxing\b`), 45},
	{"NASCAR", regexp.MustCompile(`(?i)\bnascar\b`), 45},
}

// gameRe is a programme that's a game between two sides ("Bills at Chiefs",
// "Lakers vs. Celtics", "Arsenal v Chelsea").
var gameRe = regexp.MustCompile(`(?i)\S\s+(at|vs\.?|v\.?|@)\s+\S`)

// talkRe is talk about sport rather than a game.
var talkRe = regexp.MustCompile(`(?i)\b(countdown|pregame|pre-game|postgame|post-game|highlights|report|center|centre|tonight|live!|show|talk|podcast|classic|replay|recap|preview|insider|analysis|draft)\b`)

var sportsChannelRe = regexp.MustCompile(`(?i)(espn|fox sports|fs1|fs2|nfl|nba|mlb|nhl|tnt|tbs|sports|sec network|big ten|acc network|golf|tennis|dazn|bein|sky sports|tsn|sportsnet)`)

type sportsGame struct {
	liveHit
	League string `json:"league,omitempty"`
	// The guide's title split for showing: "NBA Basketball : Bucks v Heat"
	// is kicker "NBA Basketball", matchup "Bucks v Heat".
	Kicker  string `json:"kicker,omitempty"`
	Matchup string `json:"matchup,omitempty"`
	Big     int    `json:"-"`
}

// sportsScore says how big a game a programme on a channel is (0: not a game).
func sportsScore(title, desc, channel, category string) (league string, score int) {
	text := title + " " + desc
	if talkRe.MatchString(title) {
		return "", 0
	}
	for _, l := range sportsLeagues {
		if l.re.MatchString(text) {
			league, score = l.name, l.score
			break
		}
	}
	isGame := gameRe.MatchString(title)
	onSports := sportsChannelRe.MatchString(channel) || strings.Contains(strings.ToLower(category), "sport")
	switch {
	case score > 0 && isGame:
		return league, score + 10
	case score > 0 && onSports:
		return league, score
	case isGame && onSports:
		return "", 30
	}
	return "", 0
}

// guideSports is the fallback when ESPN can't be read: games found in the
// TV guide by their titles. It always answers (with a note saying what went
// wrong), so the Sports page never waits on an error.
func (s *Server) guideSports(w http.ResponseWriter, r *http.Request, pid int64, espnErr error) {
	empty := map[string]any{"now": []sportsGame{}, "soon": []sportsGame{}, "source": "guide"}
	if espnErr != nil {
		empty["note"] = "Couldn't get the games from ESPN just now (" + espnReason(espnErr) + "). Trying again in a minute."
	}
	_, chans, err := s.liveChannels(r.Context())
	if errors.Is(err, errLiveNotSet) {
		writeJSON(w, http.StatusOK, empty)
		return
	}
	if err != nil {
		if _, ok := empty["note"]; !ok {
			empty["note"] = liveProviderMessage(err)
		}
		writeJSON(w, http.StatusOK, empty)
		return
	}
	favs, _ := s.WatchRepo.LiveFavorites(pid)
	guide, _ := s.liveGuide()
	now := time.Now()
	until := now.Add(24 * time.Hour)
	var games []sportsGame
	seen := map[string]bool{} // one listing per game (the same game on several channels)
	seenEPG := map[string]bool{}
	for _, ch := range chans {
		if ch.EPGID == "" || seenEPG[ch.EPGID] {
			continue
		}
		seenEPG[ch.EPGID] = true
		for _, p := range guide[ch.EPGID] {
			if !p.Stop.After(now) || p.Start.After(until) {
				continue
			}
			league, score := sportsScore(p.Title, p.Desc, ch.Name, ch.Category)
			if score == 0 {
				continue
			}
			kicker, matchup := splitGameTitle(p.Title)
			// The same game on several channels (or in another language,
			// "Baloncesto NBA : Bucks v Heat"): listed once.
			key := strings.ToLower(matchup) + p.Start.Format(time.RFC3339)
			if seen[key] {
				continue
			}
			seen[key] = true
			lc := liveChannel{ID: ch.ID, Num: ch.Num, Name: tidyChannelName(ch.Name), Category: ch.Category, Favorite: favs[ch.ID], Catchup: ch.CatchupDays}
			if ch.Logo != "" {
				lc.Logo = "/api/live/logo/" + url.PathEscape(ch.ID)
			}
			pp := p
			games = append(games, sportsGame{liveHit: liveHit{Channel: lc, Show: &pp, OnNow: !p.Start.After(now)}, League: league, Kicker: kicker, Matchup: matchup, Big: score})
		}
	}
	// By time; at the same time, real matchups ("Bulls at Nets") before
	// shows about sport ("Good Morning Football").
	slices.SortStableFunc(games, func(a, b sportsGame) int {
		if c := a.Show.Start.Compare(b.Show.Start); c != 0 && !(a.OnNow && b.OnNow) {
			return c
		}
		if ma, mb := isMatchup(a.Matchup), isMatchup(b.Matchup); ma != mb {
			if ma {
				return -1
			}
			return 1
		}
		return a.Show.Start.Compare(b.Show.Start)
	})
	nowList, soon := []sportsGame{}, []sportsGame{}
	var big *sportsGame
	for i := range games {
		g := games[i]
		if g.OnNow {
			nowList = append(nowList, g)
		} else {
			soon = append(soon, g)
		}
		// The next big game: the biggest on now or within six hours (a bit of
		// bigness traded for sooner).
		if g.Show.Start.Before(now.Add(6 * time.Hour)) {
			if big == nil || bigness(g, now) > bigness(*big, now) {
				big = &games[i]
			}
		}
	}
	slices.SortStableFunc(nowList, func(a, b sportsGame) int { return b.Big - a.Big })
	if len(soon) > 40 {
		soon = soon[:40]
	}
	out := map[string]any{"now": nowList, "soon": soon, "guideReady": guide != nil, "source": "guide"}
	if n, ok := empty["note"]; ok {
		out["note"] = n // why these come from the guide, not ESPN
	}
	if big != nil {
		out["big"] = big
	}
	writeJSON(w, http.StatusOK, out)
}

func bigness(g sportsGame, now time.Time) float64 {
	wait := g.Show.Start.Sub(now).Hours()
	if wait < 0 {
		wait = 0
	}
	return float64(g.Big) - wait*4
}

// espnReason is the short reason ESPN couldn't be read, for the page: the
// last part of the error ("status 403", "timeout"), not the whole chain.
func espnReason(err error) string {
	msg := err.Error()
	if i := strings.LastIndex(msg, ": "); i >= 0 {
		msg = msg[i+2:]
	}
	if len(msg) > 80 {
		msg = msg[:80]
	}
	return msg
}

// splitGameTitle splits a guide title at its colon: "NBA Basketball :
// Milwaukee Bucks at Miami Heat" is ("NBA Basketball", "Milwaukee Bucks at
// Miami Heat"). A title without one is all matchup.
func splitGameTitle(title string) (kicker, matchup string) {
	// Providers' small-caps tags ("ᴸᶦᵛᵉ", "ᴺᵉʷ") go.
	title = strings.Map(func(r rune) rune {
		if (r >= 0x1D2C && r <= 0x1DBF) || r == 0x2071 || r == 0x207F {
			return -1
		}
		return r
	}, title)
	title = strings.Join(strings.Fields(title), " ")
	if i := strings.Index(title, ":"); i > 0 && i < len(title)-1 {
		k, m := strings.TrimSpace(title[:i]), strings.TrimSpace(title[i+1:])
		if k != "" && m != "" && len(k) <= 40 {
			return k, m
		}
	}
	return "", title
}

// tidyChannelName drops a provider's decoration from a channel's name:
// "US ★ ESPN HD ◉" is "ESPN", "|UK| Sky Sports Main Event FHD" is "Sky
// Sports Main Event".
func tidyChannelName(name string) string {
	n := strings.TrimSpace(name)
	// A country prefix: "US ★ ", "US: ", "US | ", "|US| ", "[US] ".
	for _, sep := range []string{"★", "|", ":", "-"} {
		if i := strings.Index(n, sep); i >= 0 && i <= 6 {
			head := strings.Trim(n[:i], " |[]")
			if head == "" && sep == "|" {
				// "|US| ESPN": the prefix sits between two bars.
				if j := strings.Index(n[i+1:], "|"); j >= 0 && j <= 5 {
					n = strings.TrimSpace(n[i+1+j+1:])
				}
				continue
			}
			if len(head) >= 2 && len(head) <= 4 && strings.ToUpper(head) == head {
				n = strings.TrimSpace(n[i+len(sep):])
			}
		}
	}
	// Symbols and a quality suffix.
	n = strings.Map(func(r rune) rune {
		if r == '◉' || r == '●' || r == '★' || r == 'ᴴ' || r == 'ᴰ' || (r >= 0x1D2C && r <= 0x1DBF) {
			return -1
		}
		return r
	}, n)
	f := strings.Fields(n)
	for len(f) > 1 {
		switch strings.ToUpper(f[len(f)-1]) {
		case "HD", "FHD", "UHD", "SD", "4K", "HEVC", "H265", "RAW", "50FPS", "60FPS":
			f = f[:len(f)-1]
			continue
		}
		break
	}
	if len(f) == 0 {
		return strings.TrimSpace(name)
	}
	return strings.Join(f, " ")
}

// isMatchup is whether a title names two sides: "Bulls at Nets", "France
// U17 vs. Haiti U17", "New Zealand v England".
func isMatchup(t string) bool {
	l := " " + strings.ToLower(t) + " "
	for _, sep := range []string{" vs ", " vs. ", " v ", " v. ", " at ", " @ "} {
		if strings.Contains(l, sep) {
			return true
		}
	}
	return false
}
