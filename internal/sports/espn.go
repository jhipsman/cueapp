// Package sports reads game schedules, live scores and TV networks from
// ESPN's public scoreboard feed (the one behind espn.com's scoreboards: no
// key needed). It isn't an official API, so callers fall back to the TV
// guide when it fails.
package sports

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jhipsman/cueapp/internal/netguard"
)

// League is one league Cue follows: its ESPN path and how big its games are.
type League struct {
	Key   string // nfl, nba...
	Name  string
	Path  string // sport/league on ESPN
	Score int
}

// Leagues are the leagues read, biggest first.
var Leagues = []League{
	{"nfl", "NFL", "football/nfl", 90},
	{"nba", "NBA", "basketball/nba", 80},
	{"cfb", "College football", "football/college-football", 76},
	{"mlb", "MLB", "baseball/mlb", 74},
	{"nhl", "NHL", "hockey/nhl", 72},
	{"ucl", "Champions League", "soccer/uefa.champions", 70},
	{"epl", "Premier League", "soccer/eng.1", 68},
	{"cbb", "College basketball", "basketball/mens-college-basketball", 66},
	{"mls", "MLS", "soccer/usa.1", 55},
	{"wnba", "WNBA", "basketball/wnba", 55},
}

// LeagueByKey finds a league.
func LeagueByKey(key string) (League, bool) {
	for _, l := range Leagues {
		if l.Key == key {
			return l, true
		}
	}
	return League{}, false
}

// Team is one side of a game.
type Team struct {
	ID     string `json:"id"`
	Name   string `json:"name"`  // Kansas City Chiefs
	Short  string `json:"short"` // Chiefs
	Abbr   string `json:"abbr"`  // KC
	Logo   string `json:"logo,omitempty"`
	Color  string `json:"color,omitempty"` // hex, no #
	Score  string `json:"score,omitempty"`
	Record string `json:"record,omitempty"` // 9-2
	Winner bool   `json:"winner,omitempty"`
}

// Game is one game.
type Game struct {
	ID         string    `json:"id"`
	League     string    `json:"league"`     // key
	LeagueName string    `json:"leagueName"` // NFL
	Start      time.Time `json:"start"`
	State      string    `json:"state"`  // pre, in, post
	Detail     string    `json:"detail"` // "Q3 4:12", "Sun 1:00 PM", "Final"
	Home       Team      `json:"home"`
	Away       Team      `json:"away"`
	Networks   []string  `json:"networks,omitempty"` // national TV and streaming: CBS, ESPN, Prime Video
	Note       string    `json:"note,omitempty"`     // "AFC Wild Card"
	Postseason bool      `json:"postseason,omitempty"`
	Size       int       `json:"-"` // the league's score
}

// Title is "Bills at Chiefs".
func (g Game) Title() string {
	a, h := g.Away.Short, g.Home.Short
	if a == "" {
		a = g.Away.Name
	}
	if h == "" {
		h = g.Home.Name
	}
	return a + " at " + h
}

// Client reads ESPN, keeping each league's scoreboard for a minute (live
// scores) and team lists for a day.
type Client struct {
	base string
	http *http.Client
	now  func() time.Time

	mu    sync.Mutex
	cache map[string]cached
	// When every league failed, and why: for a couple of minutes after,
	// AllGames answers at once rather than waiting on ESPN again.
	failedAt  time.Time
	failedErr error
}

// failMemory is how long AllGames remembers ESPN being unreachable.
const failMemory = 2 * time.Minute

type cached struct {
	at    time.Time
	games []Game
	teams []Team
}

// New is a client for ESPN's public site API (base "" for the real one).
func New(base string) *Client {
	if base == "" {
		base = "https://site.api.espn.com/apis/site/v2/sports"
	}
	return &Client{base: strings.TrimRight(base, "/"), http: netguard.Client(10 * time.Second), now: time.Now, cache: map[string]cached{}}
}

// raw shapes of ESPN's scoreboard.
type scoreboard struct {
	Events []struct {
		ID     string `json:"id"`
		Date   string `json:"date"`
		Season struct {
			Type int `json:"type"` // 3: postseason
		} `json:"season"`
		Status struct {
			Type struct {
				State       string `json:"state"`
				ShortDetail string `json:"shortDetail"`
			} `json:"type"`
		} `json:"status"`
		Competitions []struct {
			Competitors []struct {
				HomeAway string `json:"homeAway"`
				Score    string `json:"score"`
				Winner   bool   `json:"winner"`
				Team     struct {
					ID               string `json:"id"`
					DisplayName      string `json:"displayName"`
					ShortDisplayName string `json:"shortDisplayName"`
					Abbreviation     string `json:"abbreviation"`
					Logo             string `json:"logo"`
					Color            string `json:"color"`
				} `json:"team"`
				Records []struct {
					Summary string `json:"summary"`
				} `json:"records"`
			} `json:"competitors"`
			Broadcasts []struct {
				Market string   `json:"market"`
				Names  []string `json:"names"`
			} `json:"broadcasts"`
			GeoBroadcasts []struct {
				Market struct {
					Type string `json:"type"` // National, Home, Away
				} `json:"market"`
				Media struct {
					ShortName string `json:"shortName"`
				} `json:"media"`
			} `json:"geoBroadcasts"`
			Notes []struct {
				Headline string `json:"headline"`
			} `json:"notes"`
		} `json:"competitions"`
	} `json:"events"`
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("espn %s: %w", path, netguard.CleanError(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("espn %s: status %d", path, resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(out)
}

// Games is a league's games from yesterday to tomorrow (local days), read
// at most once a minute.
func (c *Client) Games(ctx context.Context, l League) ([]Game, error) {
	key := "games:" + l.Key
	c.mu.Lock()
	if v, ok := c.cache[key]; ok && c.now().Sub(v.at) < time.Minute {
		c.mu.Unlock()
		return v.games, nil
	}
	c.mu.Unlock()
	now := c.now()
	dates := now.AddDate(0, 0, -1).Format("20060102") + "-" + now.AddDate(0, 0, 1).Format("20060102")
	var sb scoreboard
	if err := c.get(ctx, "/"+l.Path+"/scoreboard?limit=200&dates="+dates, &sb); err != nil {
		return nil, err
	}
	games := parseScoreboard(sb, l)
	c.mu.Lock()
	c.cache[key] = cached{at: c.now(), games: games}
	c.mu.Unlock()
	return games, nil
}

func parseScoreboard(sb scoreboard, l League) []Game {
	var out []Game
	for _, e := range sb.Events {
		if len(e.Competitions) == 0 {
			continue
		}
		comp := e.Competitions[0]
		start, err := time.Parse("2006-01-02T15:04Z", e.Date)
		if err != nil {
			if start, err = time.Parse(time.RFC3339, e.Date); err != nil {
				continue
			}
		}
		g := Game{ID: e.ID, League: l.Key, LeagueName: l.Name, Start: start, State: e.Status.Type.State, Detail: e.Status.Type.ShortDetail,
			Postseason: e.Season.Type == 3, Size: l.Score}
		for _, t := range comp.Competitors {
			team := Team{ID: t.Team.ID, Name: t.Team.DisplayName, Short: t.Team.ShortDisplayName, Abbr: t.Team.Abbreviation,
				Logo: t.Team.Logo, Color: t.Team.Color, Score: t.Score, Winner: t.Winner}
			if len(t.Records) > 0 {
				team.Record = t.Records[0].Summary
			}
			if g.State == "pre" {
				team.Score = ""
			}
			if team.Logo == "" {
				team.Logo = LogoURL(l, team)
			}
			if t.HomeAway == "home" {
				g.Home = team
			} else {
				g.Away = team
			}
		}
		if g.Home.Name == "" || g.Away.Name == "" {
			continue
		}
		seen := map[string]bool{}
		add := func(n string) {
			n = strings.TrimSpace(n)
			if n != "" && !seen[strings.ToLower(n)] {
				seen[strings.ToLower(n)] = true
				g.Networks = append(g.Networks, n)
			}
		}
		for _, b := range comp.GeoBroadcasts {
			if strings.EqualFold(b.Market.Type, "National") {
				add(b.Media.ShortName)
			}
		}
		for _, b := range comp.Broadcasts {
			if b.Market == "" || strings.EqualFold(b.Market, "national") {
				for _, n := range b.Names {
					add(n)
				}
			}
		}
		// A local game (no national network): its local channels, which a
		// viewer far away may still have.
		if len(g.Networks) == 0 {
			for _, b := range comp.GeoBroadcasts {
				add(b.Media.ShortName)
			}
		}
		if len(comp.Notes) > 0 {
			g.Note = comp.Notes[0].Headline
		}
		out = append(out, g)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out
}

// AllGames is every league's games (leagues that fail are skipped; an
// error only when all fail).
func (c *Client) AllGames(ctx context.Context) ([]Game, error) {
	c.mu.Lock()
	if c.failedErr != nil && c.now().Sub(c.failedAt) < failMemory {
		err := c.failedErr
		c.mu.Unlock()
		return nil, err
	}
	c.mu.Unlock()
	var mu sync.Mutex
	var all []Game
	var wg sync.WaitGroup
	fails := 0
	var first error
	for _, l := range Leagues {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g, err := c.Games(ctx, l)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				fails++
				if first == nil {
					first = err
				}
				return
			}
			all = append(all, g...)
		}()
	}
	wg.Wait()
	if fails == len(Leagues) {
		err := fmt.Errorf("espn: no league could be read: %w", first)
		// A cancelled request (the person left the page) says nothing about ESPN.
		if ctx.Err() == nil || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			c.mu.Lock()
			c.failedAt, c.failedErr = c.now(), err
			c.mu.Unlock()
		}
		return nil, err
	}
	c.mu.Lock()
	c.failedErr = nil
	c.mu.Unlock()
	sort.SliceStable(all, func(i, j int) bool { return all[i].Start.Before(all[j].Start) })
	return all, nil
}

// Teams is a league's teams (for picking favorites), kept for a day.
func (c *Client) Teams(ctx context.Context, l League) ([]Team, error) {
	key := "teams:" + l.Key
	c.mu.Lock()
	if v, ok := c.cache[key]; ok && c.now().Sub(v.at) < 24*time.Hour {
		c.mu.Unlock()
		return v.teams, nil
	}
	c.mu.Unlock()
	var raw struct {
		Sports []struct {
			Leagues []struct {
				Teams []struct {
					Team struct {
						ID               string `json:"id"`
						DisplayName      string `json:"displayName"`
						ShortDisplayName string `json:"shortDisplayName"`
						Abbreviation     string `json:"abbreviation"`
						Color            string `json:"color"`
						Logos            []struct {
							Href string `json:"href"`
						} `json:"logos"`
					} `json:"team"`
				} `json:"teams"`
			} `json:"leagues"`
		} `json:"sports"`
	}
	if err := c.get(ctx, "/"+l.Path+"/teams?limit=400", &raw); err != nil {
		return nil, err
	}
	var teams []Team
	for _, s := range raw.Sports {
		for _, lg := range s.Leagues {
			for _, t := range lg.Teams {
				tm := Team{ID: t.Team.ID, Name: t.Team.DisplayName, Short: t.Team.ShortDisplayName, Abbr: t.Team.Abbreviation, Color: t.Team.Color}
				if len(t.Team.Logos) > 0 {
					tm.Logo = t.Team.Logos[0].Href
				} else {
					tm.Logo = LogoURL(l, tm)
				}
				teams = append(teams, tm)
			}
		}
	}
	sort.SliceStable(teams, func(i, j int) bool { return teams[i].Name < teams[j].Name })
	c.mu.Lock()
	c.cache[key] = cached{at: c.now(), teams: teams}
	c.mu.Unlock()
	return teams, nil
}

// Bigness says how much a game is worth watching now: the league's size,
// the playoffs and finals more, a favorite team's games most; live games a
// little more, later games less the longer the wait.
func Bigness(g Game, now time.Time, favorite bool) float64 {
	b := float64(g.Size)
	if g.Postseason {
		b += 25
	}
	n := strings.ToLower(g.Note)
	switch {
	case strings.Contains(n, "super bowl") || strings.Contains(n, "final") || strings.Contains(n, "championship") || strings.Contains(n, "world series"):
		b += 30
	case n != "":
		b += 8
	}
	if favorite {
		b += 100
	}
	if g.State == "in" {
		b += 10
	} else if wait := g.Start.Sub(now).Hours(); wait > 0 {
		b -= wait * 4
	}
	return b
}

// networkAliases are other names a network's channel goes by in a TV list.
var networkAliases = map[string][]string{
	"espn":               {"espn"},
	"espn2":              {"espn2", "espn 2"},
	"espnu":              {"espnu"},
	"abc":                {"abc"},
	"cbs":                {"cbs"},
	"nbc":                {"nbc"},
	"fox":                {"fox"},
	"fs1":                {"fs1", "fox sports 1"},
	"fs2":                {"fs2", "fox sports 2"},
	"tnt":                {"tnt"},
	"tbs":                {"tbs"},
	"trutv":              {"trutv", "tru tv"},
	"usa net":            {"usa network", "usa"},
	"nfl net":            {"nfl network"},
	"nfl network":        {"nfl network"},
	"nba tv":             {"nba tv"},
	"mlb network":        {"mlb network"},
	"mlb net":            {"mlb network"},
	"nhl network":        {"nhl network"},
	"big ten network":    {"big ten", "btn"},
	"btn":                {"big ten", "btn"},
	"sec network":        {"sec network"},
	"secn":               {"sec network"},
	"acc network":        {"acc network"},
	"accn":               {"acc network"},
	"cbs sports network": {"cbs sports network", "cbssn"},
	"cbssn":              {"cbs sports network", "cbssn"},
	"golf":               {"golf channel"},
	"univision":          {"univision"},
	"telemundo":          {"telemundo"},
}

// Streaming are networks that are streaming services, not channels.
var Streaming = map[string]string{
	"prime video": "Prime Video", "amazon prime video": "Prime Video", "peacock": "Peacock", "espn+": "ESPN+", "netflix": "Netflix",
	"apple tv+": "Apple TV+", "apple tv": "Apple TV+", "paramount+": "Paramount+", "max": "Max", "nfl+": "NFL+", "youtube": "YouTube",
	"fox one": "Fox One", "nba league pass": "NBA League Pass", "mlb.tv": "MLB.TV", "espn unlimited": "ESPN",
}

// ChannelWords are what to look for in a TV channel's name for a network
// ("" for a streaming service, which no channel carries).
func ChannelWords(network string) []string {
	n := strings.ToLower(strings.TrimSpace(network))
	if _, ok := Streaming[n]; ok {
		return nil
	}
	if a, ok := networkAliases[n]; ok {
		return a
	}
	return []string{n}
}

// LogoURL is a team's logo on ESPN's image server when the scoreboard
// leaves it out: the pro leagues file logos by team abbreviation, college
// and soccer by team id.
func LogoURL(l League, t Team) string {
	sport := strings.SplitN(l.Path, "/", 2)[1]
	switch l.Key {
	case "nfl", "nba", "mlb", "nhl", "wnba":
		if t.Abbr == "" {
			return ""
		}
		return "https://a.espncdn.com/i/teamlogos/" + sport + "/500/" + strings.ToLower(t.Abbr) + ".png"
	case "cfb", "cbb":
		if t.ID == "" {
			return ""
		}
		return "https://a.espncdn.com/i/teamlogos/ncaa/500/" + t.ID + ".png"
	default: // soccer
		if t.ID == "" {
			return ""
		}
		return "https://a.espncdn.com/i/teamlogos/soccer/500/" + t.ID + ".png"
	}
}
