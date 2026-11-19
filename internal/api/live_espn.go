package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jhipsman/cueapp/internal/iptv"
	"github.com/jhipsman/cueapp/internal/sports"
	"github.com/jhipsman/cueapp/internal/watch"
)

// Sports from ESPN: real schedules, live scores and the network each game is
// on, matched to a channel in the household's Live TV list so Watch goes
// straight there. The next big game is picked by league, the playoffs, a
// favorite team and how soon it starts. When ESPN can't be read, the games
// come from the TV guide instead (guideSports).

// espn is the ESPN reader (tests point it elsewhere).
var espn = sports.New("")

type espnGame struct {
	sports.Game
	Title     string       `json:"title"`
	Channel   *liveChannel `json:"channel,omitempty"` // where to watch on Live TV
	Streaming string       `json:"streaming,omitempty"`
	Favorite  bool         `json:"favorite,omitempty"`
	OnNow     bool         `json:"onNow"`
}

// GET /api/live/sports
func (s *Server) handleLiveSports(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	games, err := espn.AllGames(ctx)
	if err != nil || len(games) == 0 {
		if err != nil {
			slog.Info("sports: espn", "err", err)
		}
		s.guideSports(w, r, pid, err)
		return
	}
	favTeams, _ := s.WatchRepo.FavoriteTeams(pid)
	fav := map[string]bool{}
	for _, t := range favTeams {
		fav[t.League+":"+t.TeamID] = true
	}
	// The channels, when Live TV is set up.
	var chans []iptv.Channel
	var favCh map[string]bool
	if _, cs, err := s.liveChannels(ctx); err == nil {
		chans = cs
		favCh, _ = s.WatchRepo.LiveFavorites(pid)
	}
	now := time.Now()
	nowList, soon := []espnGame{}, []espnGame{}
	var big *espnGame
	var bigScore float64
	for _, g := range games {
		// Over (a final from yesterday) or more than a day away: not here.
		if g.State == "post" || g.Start.After(now.Add(30*time.Hour)) {
			continue
		}
		eg := espnGame{Game: g, Title: g.Title(), OnNow: g.State == "in", Favorite: fav[g.League+":"+g.Home.ID] || fav[g.League+":"+g.Away.ID]}
		for _, n := range g.Networks {
			if name, ok := sports.Streaming[strings.ToLower(n)]; ok {
				if eg.Streaming == "" {
					eg.Streaming = name
				}
				continue
			}
			if eg.Channel == nil {
				eg.Channel = matchChannel(chans, n, favCh)
			}
		}
		if eg.OnNow {
			nowList = append(nowList, eg)
		} else {
			soon = append(soon, eg)
		}
		if g.Start.Before(now.Add(8*time.Hour)) || eg.Favorite {
			if b := sports.Bigness(g, now, eg.Favorite); big == nil || b > bigScore {
				c := eg
				big, bigScore = &c, b
			}
		}
	}
	// On now: the biggest first. Coming up: by time, a favorite team's first.
	sort.SliceStable(nowList, func(i, j int) bool {
		return sports.Bigness(nowList[i].Game, now, nowList[i].Favorite) > sports.Bigness(nowList[j].Game, now, nowList[j].Favorite)
	})
	sort.SliceStable(soon, func(i, j int) bool {
		if soon[i].Favorite != soon[j].Favorite {
			return soon[i].Favorite
		}
		return soon[i].Start.Before(soon[j].Start)
	})
	if len(soon) > 60 {
		soon = soon[:60]
	}
	out := map[string]any{"now": nowList, "soon": soon, "source": "espn", "teams": favTeams}
	if big != nil {
		out["big"] = big
	}
	writeJSON(w, http.StatusOK, out)
}

var (
	netResMu sync.Mutex
	netRes   = map[string]*regexp.Regexp{}
)

// networkRes are the whole-word patterns for a network's channel words,
// compiled once.
func networkRes(words []string) []*regexp.Regexp {
	netResMu.Lock()
	defer netResMu.Unlock()
	out := make([]*regexp.Regexp, 0, len(words))
	for _, wd := range words {
		re, ok := netRes[wd]
		if !ok {
			re = regexp.MustCompile(`\b` + regexp.QuoteMeta(wd) + `\b`)
			netRes[wd] = re
		}
		out = append(out, re)
	}
	return out
}

var notThisNetwork = regexp.MustCompile(`(?i)\b(news|business|deportes|life|kids|classic|movies?|weather)\b`)

// matchChannel is the household's channel for a TV network ("CBS" → "US:
// CBS New York HD"): a favorite first, then the lowest number. A channel
// for the network's news or sister channels (CBS News, FOX Business) isn't
// it, nor is a sports channel when the network isn't one (FOX vs FOX
// Sports 1).
func matchChannel(chans []iptv.Channel, network string, favs map[string]bool) *liveChannel {
	words := sports.ChannelWords(network)
	if len(words) == 0 {
		return nil
	}
	sportsNet := strings.Contains(strings.ToLower(network), "sport")
	for _, wd := range words {
		if strings.Contains(wd, "sport") {
			sportsNet = true
		}
	}
	res := networkRes(words)
	var best *iptv.Channel
	for i := range chans {
		ch := chans[i]
		name := strings.ToLower(ch.Name)
		hit := false
		for _, re := range res {
			if re.MatchString(name) {
				hit = true
				break
			}
		}
		if !hit || notThisNetwork.MatchString(name) {
			continue
		}
		if !sportsNet && strings.Contains(name, "sport") && !strings.Contains(strings.ToLower(network), "espn") {
			continue
		}
		if best == nil || (favs[ch.ID] && !favs[best.ID]) || (favs[ch.ID] == favs[best.ID] && ch.Num > 0 && (best.Num == 0 || ch.Num < best.Num)) {
			best = &chans[i]
		}
	}
	if best == nil {
		return nil
	}
	lc := liveChannel{ID: best.ID, Num: best.Num, Name: best.Name, Category: best.Category, Favorite: favs[best.ID], Catchup: best.CatchupDays}
	if best.Logo != "" {
		lc.Logo = "/api/live/logo/" + url.PathEscape(best.ID)
	}
	return &lc
}

// GET /api/live/sports/teams?league=nfl: the league's teams to follow, and
// the profile's.
func (s *Server) handleSportsTeams(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	mine, _ := s.WatchRepo.FavoriteTeams(pid)
	out := map[string]any{"mine": mine, "leagues": sportsLeagueList()}
	if key := r.URL.Query().Get("league"); key != "" {
		l, ok := sports.LeagueByKey(key)
		if !ok {
			writeError(w, http.StatusBadRequest, "That isn't a league Cue follows.")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		teams, err := espn.Teams(ctx, l)
		if err != nil {
			writeError(w, http.StatusBadGateway, "Couldn't load the teams from ESPN right now.")
			return
		}
		out["teams"] = teams
	}
	writeJSON(w, http.StatusOK, out)
}

func sportsLeagueList() []map[string]string {
	var out []map[string]string
	for _, l := range sports.Leagues {
		out = append(out, map[string]string{"key": l.Key, "name": l.Name})
	}
	return out
}

// PUT|DELETE /api/live/sports/teams/{league}/{id} {name, logo}: follow a team
// (or stop).
func (s *Server) handleSportsTeam(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	league, id := r.PathValue("league"), r.PathValue("id")
	if _, ok := sports.LeagueByKey(league); !ok || id == "" || len(id) > 20 {
		writeError(w, http.StatusBadRequest, "That isn't a team.")
		return
	}
	t := watch.FavoriteTeam{League: league, TeamID: id}
	if r.Method == http.MethodPut {
		var req struct {
			Name string `json:"name"`
			Logo string `json:"logo"`
		}
		_ = decodeJSON(r, &req)
		t.Name, t.Logo = truncate(req.Name, 100), truncate(req.Logo, 300)
		if !strings.HasPrefix(t.Logo, "https://") {
			t.Logo = ""
		}
	}
	if err := s.WatchRepo.SetFavoriteTeam(pid, t, r.Method == http.MethodPut); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't save that.")
		return
	}
	mine, _ := s.WatchRepo.FavoriteTeams(pid)
	writeJSON(w, http.StatusOK, map[string]any{"mine": mine})
}

// kickoffAlerts are alerts for a profile's teams' games starting within the
// next hour and a quarter (the alert round runs hourly).
func (s *Server) kickoffAlerts(ctx context.Context, pid int64, tell func(key, title, body, link string)) {
	teams, err := s.WatchRepo.FavoriteTeams(pid)
	if err != nil || len(teams) == 0 {
		return
	}
	fav := map[string]bool{}
	leagues := map[string]bool{}
	for _, t := range teams {
		fav[t.League+":"+t.TeamID] = true
		leagues[t.League] = true
	}
	now := time.Now()
	for key := range leagues {
		l, _ := sports.LeagueByKey(key)
		games, err := espn.Games(ctx, l)
		if err != nil {
			continue
		}
		for _, g := range games {
			if g.State != "pre" || g.Start.Before(now) || g.Start.After(now.Add(75*time.Minute)) {
				continue
			}
			if !fav[g.League+":"+g.Home.ID] && !fav[g.League+":"+g.Away.ID] {
				continue
			}
			where := ""
			if len(g.Networks) > 0 {
				where = " on " + g.Networks[0]
			}
			tell("game:"+g.League+":"+g.ID, g.Title()+" starts soon",
				fmt.Sprintf("%s%s.", g.Start.Local().Format("3:04 PM"), where), "/watch/live")
		}
	}
}
