package api

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

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
	Title     string        `json:"title"`
	Channel   *liveChannel  `json:"channel,omitempty"` // where to watch on Live TV
	More      []liveChannel `json:"more,omitempty"`    // other channels showing it
	Streaming string        `json:"streaming,omitempty"`
	Favorite  bool          `json:"favorite,omitempty"`
	OnNow     bool          `json:"onNow"`
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
		} else {
			err = fmt.Errorf("ESPN listed no games: %s", cmp.Or(espn.Problems(), "every league was empty"))
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
	now := time.Now()
	var match *gameMatcher
	if cats, cs, err := s.liveChannels(ctx); err == nil {
		favCh, _ := s.WatchRepo.LiveFavorites(pid)
		guide, _ := s.liveGuide()
		match = newGameMatcher(cs, s.liveCatalogFor(cats, cs), guide, favCh, now)
	}
	nowList, soon := []espnGame{}, []espnGame{}
	var big *espnGame
	var bigScore float64
	finished, later := 0, 0
	for _, g := range games {
		// Over (a final from yesterday) or more than a day away: not here.
		if g.State == "post" {
			finished++
			continue
		}
		if g.Start.After(now.Add(30 * time.Hour)) {
			later++
			continue
		}
		eg := espnGame{Game: g, Title: g.Title(), OnNow: g.State == "in", Favorite: fav[g.League+":"+g.Home.ID] || fav[g.League+":"+g.Away.ID]}
		for _, n := range g.Networks {
			if name, ok := sports.Streaming[strings.ToLower(n)]; ok && eg.Streaming == "" {
				eg.Streaming = name
			}
		}
		if match != nil {
			eg.Channel, eg.More = match.find(g)
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
	if len(nowList) == 0 && len(soon) == 0 {
		// Nothing to show: say what ESPN did list, so it can be told apart
		// from a quiet day.
		note := fmt.Sprintf("ESPN listed %d games: %d finished, %d more than a day away.", len(games), finished, later)
		if p := espn.Problems(); p != "" {
			note += " " + p
		}
		out["note"] = note
	}
	if big != nil {
		out["big"] = big
	}
	writeJSON(w, http.StatusOK, out)
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
