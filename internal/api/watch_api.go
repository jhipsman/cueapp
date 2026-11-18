package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jhipsman/cueapp/internal/metadata"
	"github.com/jhipsman/cueapp/internal/omdb"
	"github.com/jhipsman/cueapp/internal/settings"
	"github.com/jhipsman/cueapp/internal/watch"
)

// Watch: the streaming side of Cue.
//
// Everything here is keyed by TMDB id, so any movie or show TMDB knows can be
// browsed and played, whether it is in the library or not. Artwork, text and
// TMDB's score come from TMDB; IMDb and Rotten Tomatoes ratings from OMDb when
// a key is saved. Each account keeps its own progress and My List.

// watchCard is one title (or, on Continue Watching, one episode) in a row.
type watchCard struct {
	Kind         string  `json:"kind"` // "movie" or "tv"
	TMDBID       int     `json:"tmdbId"`
	Title        string  `json:"title"`
	Year         int     `json:"year,omitempty"`
	Overview     string  `json:"overview,omitempty"`
	PosterURL    string  `json:"posterUrl,omitempty"`
	BackdropURL  string  `json:"backdropUrl,omitempty"`
	Rating       float64 `json:"rating,omitempty"`   // TMDB's score out of 10
	Progress     float64 `json:"progress,omitempty"` // 0 to 1, on Continue Watching
	Season       int     `json:"season,omitempty"`
	Episode      int     `json:"episode,omitempty"`
	EpisodeTitle string  `json:"episodeTitle,omitempty"`
	// New: the episode came out in the last two weeks.
	New bool `json:"new,omitempty"`
	// LogoURL is the title's logo artwork, on the Home banner and title
	// pages only (the TV apps draw it instead of the name).
	LogoURL string `json:"logoUrl,omitempty"`
	// Quiet: horror, anime or adult (quiet.go). It's kept off the banner,
	// the previews and the screensavers.
	Quiet bool `json:"quiet,omitempty"`
	// Soon: a film not released anywhere yet (it's on Coming soon);
	// InTheaters: one showing in theaters, not out at home yet (it plays,
	// usually as a camera recording, and is tagged so).
	Soon       bool `json:"soon,omitempty"`
	InTheaters bool `json:"inTheaters,omitempty"`
	// OutDate, on Coming soon, is the day it streams ("2025-03-04") when
	// known; Reminding, that this profile asked to be told; OutNow, on the
	// "Out now" row, that a title it asked about has come out.
	OutDate   string `json:"outDate,omitempty"`
	Reminding bool   `json:"reminding,omitempty"`
	OutNow    bool   `json:"outNow,omitempty"`
	// Recap, on Continue Watching for a show not watched in a few weeks:
	// what happened in the last episode seen ("Previously…").
	Recap string `json:"recap,omitempty"`
}

type watchRow struct {
	Key   string      `json:"key"`
	Title string      `json:"title"`
	Items []watchCard `json:"items"`
}

func movieCard(m metadata.Movie) watchCard {
	return watchCard{Kind: watch.KindMovie, TMDBID: m.TMDBID, Title: m.Title, Year: m.Year(), Overview: m.Overview,
		PosterURL: metadata.PosterURL(m.PosterPath), BackdropURL: metadata.BackdropURL(m.BackdropPath), Rating: metadata.RoundRating(m.VoteAverage),
		Quiet: noteMovieQuiet(m)}
}

func showCard(sh metadata.Show) watchCard {
	return watchCard{Kind: watch.KindTV, TMDBID: sh.TMDBID, Title: sh.Name, Year: sh.Year(), Overview: sh.Overview,
		PosterURL: metadata.PosterURL(sh.PosterPath), BackdropURL: metadata.BackdropURL(sh.BackdropPath), Rating: metadata.RoundRating(sh.VoteAverage),
		Quiet: noteShowQuiet(sh)}
}

func movieCards(ms []metadata.Movie) []watchCard {
	out := make([]watchCard, 0, len(ms))
	for _, m := range ms {
		if m.PosterPath != "" {
			out = append(out, movieCard(m))
		}
	}
	return out
}

func showCards(ss []metadata.Show) []watchCard {
	out := make([]watchCard, 0, len(ss))
	for _, sh := range ss {
		if sh.PosterPath != "" {
			out = append(out, showCard(sh))
		}
	}
	return out
}

func watchKind(w http.ResponseWriter, r *http.Request) (string, int, bool) {
	kind := r.PathValue("kind")
	id, err := strconv.Atoi(r.PathValue("tmdbId"))
	if (kind != watch.KindMovie && kind != watch.KindTV) || err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "That isn't a valid title.")
		return "", 0, false
	}
	return kind, id, true
}

// ---- Home

// watchShelf is one of the rows everyone sees (not per account).
type watchShelf struct {
	key, title string
	load       func(ctx context.Context, tm *metadata.Client) ([]watchCard, error)
}

func moviesFrom(f func(ctx context.Context, tm *metadata.Client) (*metadata.MoviePage, error)) func(context.Context, *metadata.Client) ([]watchCard, error) {
	return func(ctx context.Context, tm *metadata.Client) ([]watchCard, error) {
		p, err := f(ctx, tm)
		if err != nil {
			return nil, err
		}
		return movieCards(p.Results), nil
	}
}

func showsFrom(f func(ctx context.Context, tm *metadata.Client) (*metadata.ShowPage, error)) func(context.Context, *metadata.Client) ([]watchCard, error) {
	return func(ctx context.Context, tm *metadata.Client) ([]watchCard, error) {
		p, err := f(ctx, tm)
		if err != nil {
			return nil, err
		}
		return showCards(p.Results), nil
	}
}

func movieGenreShelf(key, title string, genre int) watchShelf {
	return watchShelf{key, title, moviesFrom(func(ctx context.Context, tm *metadata.Client) (*metadata.MoviePage, error) {
		return tm.DiscoverMovies(ctx, metadata.DiscoverQuery{Genre: genre, Sort: metadata.SortPopular})
	})}
}

func showGenreShelf(key, title string, genre int) watchShelf {
	return watchShelf{key, title, showsFrom(func(ctx context.Context, tm *metadata.Client) (*metadata.ShowPage, error) {
		return tm.DiscoverTV(ctx, metadata.DiscoverQuery{Genre: genre, Sort: metadata.SortPopular})
	})}
}

// watchShelves are the rows below Continue Watching and My List. TMDB genre
// ids: 28 Action, 35 Comedy, 878 Science Fiction, 27 Horror, 16 Animation,
// 80 Crime, 18 Drama, 10765 Sci-Fi & Fantasy (TV).
var watchShelves = []watchShelf{
	{"trending-movies", "Trending movies", moviesFrom(func(ctx context.Context, tm *metadata.Client) (*metadata.MoviePage, error) {
		return tm.MovieList(ctx, metadata.ListTrending, 1)
	})},
	{"trending-tv", "Trending shows", showsFrom(func(ctx context.Context, tm *metadata.Client) (*metadata.ShowPage, error) {
		return tm.TVList(ctx, metadata.ListTrending, 1)
	})},
	{"popular-movies", "Popular movies", moviesFrom(func(ctx context.Context, tm *metadata.Client) (*metadata.MoviePage, error) {
		return tm.MovieList(ctx, metadata.ListPopular, 1)
	})},
	{"popular-tv", "Popular shows", showsFrom(func(ctx context.Context, tm *metadata.Client) (*metadata.ShowPage, error) {
		return tm.TVList(ctx, metadata.ListPopular, 1)
	})},
	movieGenreShelf("action", "Action", 28),
	showGenreShelf("crime-tv", "Crime shows", 80),
	movieGenreShelf("comedy", "Comedy", 35),
	showGenreShelf("drama-tv", "Drama shows", 18),
	movieGenreShelf("scifi", "Science fiction", 878),
	showGenreShelf("scifi-tv", "Sci-fi and fantasy shows", 10765),
	movieGenreShelf("horror", "Horror", 27),
	movieGenreShelf("animation", "Animation", 16),
}

type watchHome struct {
	Hero *watchCard `json:"hero,omitempty"`
	Rows []watchRow `json:"rows"`
	// Top10 is the Top 10 on each streaming service (top10.go).
	Top10 []topList `json:"top10,omitempty"`
}

// GET /api/watch/home: Continue Watching, My List and the shelves.
func (s *Server) handleWatchHome(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	tm := s.TMDB()

	shelves := make([]watchRow, len(watchShelves))
	var wg sync.WaitGroup
	for i, sh := range watchShelves {
		wg.Add(1)
		go func(i int, sh watchShelf) {
			defer wg.Done()
			items, err := sh.load(ctx, tm)
			if err != nil {
				slog.Info("watch: home row", "row", sh.key, "err", err)
			}
			// Home's rows are what you can watch now: films not out to
			// stream yet are left off (they're in Coming soon).
			s.markSoon(ctx, items)
			items = outOnly(items)
			shelves[i] = watchRow{Key: sh.key, Title: sh.title, Items: items}
		}(i, sh)
	}
	var top []topList
	var kids []watchRow
	prof, hasProf := s.activeProfile(r)
	young := hasProf && kidsProfile(prof.MaxRating)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if young {
			kids = s.kidsRows(ctx, tm)
		} else {
			top = s.top10(ctx, tm)
		}
	}()
	cont := s.continueWatching(ctx, pid)
	var because []watchRow
	var newEps []watchCard
	var pwg sync.WaitGroup
	pwg.Add(2)
	go func() { defer pwg.Done(); because = s.becauseYouWatched(ctx, pid) }()
	go func() { defer pwg.Done(); newEps = s.newEpisodes(ctx, pid, cont) }()
	pwg.Wait()
	wg.Wait()

	home := watchHome{}
	if len(cont) > 0 {
		home.Rows = append(home.Rows, watchRow{Key: "continue", Title: "Continue watching", Items: cont})
	}
	if len(newEps) > 0 {
		home.Rows = append(home.Rows, watchRow{Key: "new-episodes", Title: "New episodes", Items: newEps})
	}
	// Titles this profile asked to be told about: out now at the top, still
	// to come heading Coming soon.
	outNow, reminded := s.remindedCards(ctx, pid)
	if len(outNow) > 0 {
		home.Rows = append([]watchRow{{Key: "out-now", Title: "Out now: you asked to be told", Items: outNow}}, home.Rows...)
	}
	if list := s.myListCards(pid); len(list) > 0 {
		s.markSoon(ctx, list)
		home.Rows = append(home.Rows, watchRow{Key: "my-list", Title: "My List", Items: list})
	}
	_ = reminded // still to come: on the Coming soon page, not Home
	// A young child's profile: its own rows first, after what it's watching.
	if young {
		for _, row := range kids {
			if len(row.Items) > 0 {
				home.Rows = append(home.Rows, row)
			}
		}
	}
	// "Because you watched" rows sit among the others, after the first.
	for i, row := range shelves {
		if i == 1 && len(because) > 0 {
			home.Rows = append(home.Rows, because[0])
		}
		if i == 4 && len(because) > 1 {
			home.Rows = append(home.Rows, because[1])
		}
		if len(row.Items) > 0 {
			home.Rows = append(home.Rows, row)
		}
	}
	// Titles marked "not for me" stay off the rows (My List aside).
	if marks, err := s.WatchRepo.Marks(pid); err == nil && len(marks) > 0 {
		for i, row := range home.Rows {
			if row.Key == "my-list" {
				continue
			}
			kept := row.Items[:0:0]
			for _, c := range row.Items {
				if _, down := marks.Has(c.Kind, c.TMDBID, watch.MarkDown); !down {
					kept = append(kept, c)
				}
			}
			home.Rows[i].Items = kept
		}
	}
	// A kids' profile sees only what its age limit allows.
	if prof, ok := s.activeProfile(r); ok && prof.MaxRating != "" {
		kept := home.Rows[:0]
		for _, row := range home.Rows {
			row.Items = s.ageFiltered(ctx, prof, row.Items)
			if len(row.Items) > 0 {
				kept = append(kept, row)
			}
		}
		home.Rows = kept
		cont = nil
		for _, row := range kept {
			if row.Key == "continue" {
				cont = row.Items
			}
		}
	}
	// The Top 10s get the same care as the rows: a kids' profile's limit.
	if prof, ok := s.activeProfile(r); ok && prof.MaxRating != "" {
		kept := top[:0]
		for _, l := range top {
			if l.Items = s.ageFiltered(ctx, prof, l.Items); len(l.Items) > 0 {
				kept = append(kept, l)
			}
		}
		top = kept
	}
	home.Top10 = top
	s.markQuiet(ctx, home.Rows)
	// The banner: something trending this week with artwork, changing
	// through the day so the page doesn't look the same every visit.
	for _, row := range home.Rows {
		if row.Key != "trending-movies" && row.Key != "trending-tv" {
			continue
		}
		var withArt []watchCard
		for _, c := range row.Items {
			if c.BackdropURL != "" && c.Overview != "" && !c.Quiet {
				withArt = append(withArt, c)
			}
		}
		if len(withArt) > 0 {
			hero := withArt[time.Now().Hour()%min(len(withArt), 5)]
			hero.LogoURL, _ = s.TMDB().Logo(r.Context(), hero.Kind, hero.TMDBID)
			home.Hero = &hero
			break
		}
	}
	if home.Rows == nil {
		home.Rows = []watchRow{}
	}
	// Previews made ahead of time for what's seen first: the banner and the
	// start of Continue Watching (the episode each stopped in).
	var warm []previewWant
	if home.Hero != nil {
		warm = append(warm, cardPreview(*home.Hero))
	}
	for i, c := range cont {
		if i == 3 {
			break
		}
		if !c.Quiet {
			warm = append(warm, cardPreview(c))
		}
	}
	s.prewarmPreviews(r, warm)
	// Everything else Home shows is made overnight, while the server is
	// quiet, so most titles have a clip before anyone opens them.
	var later []previewWant
	for _, row := range home.Rows {
		for _, c := range row.Items {
			if !c.Quiet && !c.Soon {
				later = append(later, cardPreview(c))
			}
		}
	}
	rememberForTonight(r, later)
	writeJSON(w, http.StatusOK, home)
}

// cardPreview is the preview a card shows: a film's, or a show's episode
// (the one in progress, or the first).
func cardPreview(c watchCard) previewWant {
	if c.Kind == "movie" {
		return previewWant{kind: "movie", tmdbID: c.TMDBID}
	}
	season, episode := c.Season, c.Episode
	if episode < 1 {
		season, episode = 1, 1
	}
	return previewWant{kind: "tv", tmdbID: c.TMDBID, season: season, episode: episode}
}

// continueWatching is userID's titles in progress: a movie or episode
// started and not finished, or the next episode after one finished.
func (s *Server) continueWatching(ctx context.Context, userID int64) []watchCard {
	recent, err := s.WatchRepo.Recent(userID, 20)
	if err != nil {
		slog.Warn("watch: continue watching", "err", err)
		return nil
	}
	marks, _ := s.WatchRepo.Marks(userID)
	out := []watchCard{}
	for _, p := range recent {
		// Taken off the row, and not watched since.
		if at, hidden := marks.Has(p.Kind, p.TMDBID, watch.MarkHidden); hidden && !p.UpdatedAt.After(at) {
			continue
		}
		c := watchCard{Kind: p.Kind, TMDBID: p.TMDBID, Title: p.Title, PosterURL: metadata.PosterURL(p.PosterPath),
			BackdropURL: metadata.BackdropURL(p.BackdropPath), Season: p.Season, Episode: p.Episode, EpisodeTitle: p.EpisodeTitle}
		if p.Finished() {
			if p.Kind != watch.KindTV {
				continue // watched to the end
			}
			next, ok := s.nextEpisode(ctx, p.TMDBID, p.Season, p.Episode)
			if !ok {
				continue // caught up
			}
			c.Season, c.Episode, c.EpisodeTitle = next.Season, next.Episode, next.Name
			c.New = recentlyAired(next.AirDate, 14)
		} else {
			c.Progress = p.Fraction()
		}
		// Away a while: what happened last time. The last episode seen to
		// the end (the one before, while one's half-watched).
		if p.Kind == watch.KindTV && time.Since(p.UpdatedAt) > recapAfter {
			last := p.Episode
			if !p.Finished() {
				last--
			}
			c.Recap = s.episodeRecap(ctx, p.TMDBID, p.Season, last)
		}
		out = append(out, c)
	}
	return out
}

func (s *Server) myListCards(userID int64) []watchCard {
	list, err := s.WatchRepo.List(userID)
	if err != nil {
		slog.Warn("watch: my list", "err", err)
		return nil
	}
	out := make([]watchCard, 0, len(list))
	for _, it := range list {
		out = append(out, watchCard{Kind: it.Kind, TMDBID: it.TMDBID, Title: it.Title, Year: it.Year,
			PosterURL: metadata.PosterURL(it.PosterPath), BackdropURL: metadata.BackdropURL(it.BackdropPath)})
	}
	return out
}

// becauseYouWatched is a row of what TMDB recommends for each of the two
// titles the profile watched most recently, without what it has watched.
func (s *Server) becauseYouWatched(ctx context.Context, pid int64) []watchRow {
	recent, err := s.WatchRepo.Recent(pid, 40)
	if err != nil || len(recent) == 0 {
		return nil
	}
	tm := s.TMDB()
	if tm == nil || !tm.HasAPIKey() {
		return nil
	}
	watched := map[string]bool{}
	for _, p := range recent {
		watched[fmt.Sprintf("%s:%d", p.Kind, p.TMDBID)] = true
	}
	// What you liked comes first; what wasn't for you isn't a reason.
	marks, _ := s.WatchRepo.Marks(pid)
	sort.SliceStable(recent, func(i, j int) bool {
		_, ui := marks.Has(recent[i].Kind, recent[i].TMDBID, watch.MarkUp)
		_, uj := marks.Has(recent[j].Kind, recent[j].TMDBID, watch.MarkUp)
		return ui && !uj
	})
	var picks []watch.Progress
	for _, p := range recent {
		if _, down := marks.Has(p.Kind, p.TMDBID, watch.MarkDown); down {
			continue
		}
		if len(picks) == 2 {
			break
		}
		// Only what was really watched, not a title opened for a moment.
		if p.Position >= 300 || p.Finished() || p.Kind == watch.KindTV && p.Episode > 1 {
			picks = append(picks, p)
		}
	}
	rows := make([]watchRow, len(picks))
	var wg sync.WaitGroup
	for i, p := range picks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var cards []watchCard
			if p.Kind == watch.KindMovie {
				ms, err := tm.RelatedMovies(ctx, p.TMDBID, metadata.RelatedRecommended)
				if err != nil {
					return
				}
				cards = movieCards(ms)
			} else {
				ss, err := tm.RelatedShows(ctx, p.TMDBID, metadata.RelatedRecommended)
				if err != nil {
					return
				}
				cards = showCards(ss)
			}
			var keep []watchCard
			for _, c := range cards {
				_, down := marks.Has(c.Kind, c.TMDBID, watch.MarkDown)
				if !watched[fmt.Sprintf("%s:%d", c.Kind, c.TMDBID)] && !down {
					keep = append(keep, c)
				}
			}
			if len(keep) >= 4 {
				rows[i] = watchRow{Key: fmt.Sprintf("because-%s-%d", p.Kind, p.TMDBID), Title: "Because you watched " + p.Title, Items: keep}
			}
		}()
	}
	wg.Wait()
	var out []watchRow
	for _, r := range rows {
		if len(r.Items) > 0 {
			out = append(out, r)
		}
	}
	return out
}

// newEpisodes is the shows on the profile's My List, or watched lately, with
// an episode out in the last two weeks it hasn't seen; ones already on
// Continue Watching aren't repeated.
func (s *Server) newEpisodes(ctx context.Context, pid int64, cont []watchCard) []watchCard {
	tm := s.TMDB()
	if tm == nil || !tm.HasAPIKey() {
		return nil
	}
	skip := map[int]bool{}
	for _, c := range cont {
		if c.Kind == watch.KindTV {
			skip[c.TMDBID] = true
		}
	}
	type show struct {
		id              int
		title, poster   string
		backdrop        string
		season, episode int // the latest the profile watched; 0 = none
	}
	var shows []show
	seen := map[int]bool{}
	if recent, err := s.WatchRepo.Recent(pid, 30); err == nil {
		for _, p := range recent {
			if p.Kind == watch.KindTV && !skip[p.TMDBID] && !seen[p.TMDBID] && time.Since(p.UpdatedAt) < 120*24*time.Hour {
				seen[p.TMDBID] = true
				shows = append(shows, show{p.TMDBID, p.Title, p.PosterPath, p.BackdropPath, p.Season, p.Episode})
			}
		}
	}
	if list, err := s.WatchRepo.List(pid); err == nil {
		for _, it := range list {
			if it.Kind == watch.KindTV && !skip[it.TMDBID] && !seen[it.TMDBID] {
				seen[it.TMDBID] = true
				shows = append(shows, show{it.TMDBID, it.Title, it.PosterPath, it.BackdropPath, 0, 0})
			}
		}
	}
	if len(shows) > 15 {
		shows = shows[:15]
	}
	cards := make([]*watchCard, len(shows))
	var wg sync.WaitGroup
	for i, sh := range shows {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d, err := tm.GetShowFull(ctx, sh.id)
			if err != nil || d.LastEpisodeToAir == nil {
				return
			}
			last := d.LastEpisodeToAir
			if !recentlyAired(last.AirDate, 14) || sh.season > last.Season || sh.season == last.Season && sh.episode >= last.Episode {
				return
			}
			c := watchCard{Kind: watch.KindTV, TMDBID: sh.id, Title: sh.title, PosterURL: metadata.PosterURL(sh.poster),
				BackdropURL: metadata.BackdropURL(sh.backdrop), Season: last.Season, Episode: last.Episode, EpisodeTitle: last.Name, New: true}
			if c.BackdropURL == "" {
				c.BackdropURL = metadata.BackdropURL(d.BackdropPath)
			}
			cards[i] = &c
		}()
	}
	wg.Wait()
	var out []watchCard
	for _, c := range cards {
		if c != nil {
			out = append(out, *c)
		}
	}
	return out
}

// recentlyAired says a TMDB air date is within the last days days.
func recentlyAired(date string, days int) bool {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return false
	}
	age := time.Since(t)
	return age >= 0 && age < time.Duration(days)*24*time.Hour
}

// ---- Episodes

// aired says a TMDB air date ("2024-03-01") is today or earlier.
func aired(date string) bool {
	return date != "" && date <= time.Now().Format("2006-01-02")
}

// recapAfter is how long away from a show before its card says what
// happened last time.
const recapAfter = 21 * 24 * time.Hour

// episodeRecap is an episode's story ("" when it isn't known or there's no
// such episode).
func (s *Server) episodeRecap(ctx context.Context, tmdbID, season, episode int) string {
	tm := s.TMDB()
	if tm == nil || episode < 1 {
		return ""
	}
	eps, err := tm.GetSeasonCached(ctx, tmdbID, season)
	if err != nil {
		return ""
	}
	for _, e := range eps {
		if e.Episode == episode {
			return e.Overview
		}
	}
	return ""
}

// nextEpisode is the aired episode after season/episode: the next one in
// the season, or the first of the next season.
func (s *Server) nextEpisode(ctx context.Context, tmdbID, season, episode int) (metadata.EpisodeInfo, bool) {
	tm := s.TMDB()
	if eps, err := tm.GetSeasonCached(ctx, tmdbID, season); err == nil {
		for _, e := range eps {
			if e.Episode == episode+1 {
				return e, aired(e.AirDate)
			}
		}
	}
	if eps, err := tm.GetSeasonCached(ctx, tmdbID, season+1); err == nil && len(eps) > 0 {
		first := eps[0]
		for _, e := range eps {
			if e.Episode < first.Episode {
				first = e
			}
		}
		return first, aired(first.AirDate)
	}
	return metadata.EpisodeInfo{}, false
}

type watchEpisode struct {
	Season    int     `json:"season"`
	Episode   int     `json:"episode"`
	Title     string  `json:"title"`
	Overview  string  `json:"overview,omitempty"`
	AirDate   string  `json:"airDate,omitempty"`
	Runtime   int     `json:"runtime,omitempty"` // minutes
	StillURL  string  `json:"stillUrl,omitempty"`
	Rating    float64 `json:"rating,omitempty"`
	Aired     bool    `json:"aired"`
	Progress  float64 `json:"progress,omitempty"` // 0 to 1
	Finished  bool    `json:"finished,omitempty"`
	ShowTitle string  `json:"showTitle,omitempty"`
}

// GET /api/watch/tv/{tmdbId}/season/{season}: the season's episodes, with
// thumbnails and how far this account got in each.
func (s *Server) handleWatchSeason(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	tmdbID, err1 := strconv.Atoi(r.PathValue("tmdbId"))
	season, err2 := strconv.Atoi(r.PathValue("season"))
	if err1 != nil || err2 != nil || tmdbID <= 0 || season < 0 {
		writeError(w, http.StatusBadRequest, "That isn't a valid season.")
		return
	}
	if s.ageBlocked(w, r, watch.KindTV, tmdbID) {
		return
	}
	eps, err := s.TMDB().GetSeasonCached(r.Context(), tmdbID, season)
	if err != nil {
		writeUpstreamError(w, "load the season from TMDB", err)
		return
	}
	progress := map[[2]int]watch.Progress{}
	if ps, err := s.WatchRepo.ShowProgress(pid, tmdbID); err == nil {
		for _, p := range ps {
			progress[[2]int{p.Season, p.Episode}] = p
		}
	}
	out := make([]watchEpisode, 0, len(eps))
	for _, e := range eps {
		we := watchEpisode{Season: e.Season, Episode: e.Episode, Title: e.Name, Overview: e.Overview, AirDate: e.AirDate,
			Runtime: e.Runtime, StillURL: metadata.StillURL(e.StillPath), Rating: metadata.RoundRating(e.VoteAverage), Aired: aired(e.AirDate)}
		if p, ok := progress[[2]int{e.Season, e.Episode}]; ok {
			we.Progress, we.Finished = p.Fraction(), p.Finished()
		}
		out = append(out, we)
	}
	writeJSON(w, http.StatusOK, out)
}

// GET /api/watch/tv/{tmdbId}/next?season=1&episode=2: the episode to play
// after that one, for autoplay. 404 when there is none (yet).
func (s *Server) handleWatchNext(w http.ResponseWriter, r *http.Request) {
	tmdbID, err1 := strconv.Atoi(r.PathValue("tmdbId"))
	season, err2 := strconv.Atoi(r.URL.Query().Get("season"))
	episode, err3 := strconv.Atoi(r.URL.Query().Get("episode"))
	if err1 != nil || err2 != nil || err3 != nil {
		writeError(w, http.StatusBadRequest, "season and episode are needed.")
		return
	}
	next, ok := s.nextEpisode(r.Context(), tmdbID, season, episode)
	if !ok {
		writeError(w, http.StatusNotFound, "That was the latest episode.")
		return
	}
	writeJSON(w, http.StatusOK, watchEpisode{Season: next.Season, Episode: next.Episode, Title: next.Name, Overview: next.Overview,
		AirDate: next.AirDate, Runtime: next.Runtime, StillURL: metadata.StillURL(next.StillPath), Aired: true})
}

// ---- Title pages

type watchSeason struct {
	Number   int    `json:"number"`
	Name     string `json:"name"`
	Episodes int    `json:"episodes"`
}

type watchTitle struct {
	watchCard
	Tagline       string        `json:"tagline,omitempty"`
	Runtime       int           `json:"runtime,omitempty"` // minutes, movies
	Genres        []string      `json:"genres"`
	Certification string        `json:"certification,omitempty"`
	Cast          []string      `json:"cast"`
	Directors     []string      `json:"directors,omitempty"` // or the show's creators
	Networks      []string      `json:"networks,omitempty"`
	Seasons       []watchSeason `json:"seasons,omitempty"`
	IMDbID        string        `json:"imdbId,omitempty"`
	Ratings       omdb.Ratings  `json:"ratings"`
	InList        bool          `json:"inList"`
	// Resume is where Play starts: a movie's position, or for a show the
	// episode to watch next and where in it.
	ResumeSeason   int         `json:"resumeSeason,omitempty"`
	ResumeEpisode  int         `json:"resumeEpisode,omitempty"`
	ResumePosition float64     `json:"resumePosition,omitempty"` // seconds
	More           []watchCard `json:"more"`
	// NotOut, for a title not out to stream yet, says when it's coming
	// ("In theaters now · streaming March 4th"); its page offers no Play.
	// OutDate is the day it streams, when known.
	NotOut  string `json:"notOut,omitempty"`
	OutDate string `json:"outDate,omitempty"`
	// TheaterNote, for a film in theaters now (it still plays), says so:
	// "In theaters now · streaming March 4th".
	TheaterNote string `json:"theaterNote,omitempty"`
	// Thumb is this profile's thumb on it: "up", "down" or "".
	Thumb string `json:"thumb,omitempty"`
	// Reminding: this profile asked to be told when it's out.
	Reminding bool `json:"reminding,omitempty"`
	// Collection: the film series it's part of, its films in order, and
	// the one after this (Next in the series, when it's out).
	Collection *watchCollection `json:"collection,omitempty"`
}

type watchCollection struct {
	ID    int         `json:"id"`
	Name  string      `json:"name"`
	Parts []watchCard `json:"parts"`
	Next  *watchCard  `json:"next,omitempty"`
}

// GET /api/watch/movie/{tmdbId}
func (s *Server) handleWatchMovie(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	tmdbID, err := strconv.Atoi(r.PathValue("tmdbId"))
	if err != nil || tmdbID <= 0 {
		writeError(w, http.StatusBadRequest, "That isn't a valid TMDB id.")
		return
	}
	ctx := r.Context()
	tm := s.TMDB()
	if s.ageBlocked(w, r, watch.KindMovie, tmdbID) {
		return
	}
	d, err := tm.GetMovieDetail(ctx, tmdbID)
	if err != nil {
		writeUpstreamError(w, "load the movie from TMDB", err)
		return
	}
	t := watchTitle{watchCard: movieCard(d.Movie), Tagline: d.Tagline, Runtime: d.Runtime, Genres: genreNames(d.Genres),
		Certification: d.Certification(), Cast: castNames(d.Cast(10)), Directors: d.Directors(), IMDbID: d.IMDBID,
		InList: s.WatchRepo.InList(pid, watch.KindMovie, tmdbID), Thumb: s.WatchRepo.Thumb(pid, watch.KindMovie, tmdbID)}
	if p, ok, _ := s.WatchRepo.GetProgress(pid, watch.KindMovie, tmdbID, 0, 0); ok && !p.Finished() {
		t.ResumePosition = p.Position
	}
	if _, inLib, _ := s.MovieRepo.GetByTMDBID(tmdbID); !inLib {
		f := filmOutOf(d, time.Now())
		filmOutKnown.Store(tmdbID, f)
		now := time.Now()
		switch {
		case f.inTheaters(now):
			t.InTheaters, t.TheaterNote, t.OutDate = true, f.theaterNote(now), f.home
		case !f.out:
			t.Soon, t.NotOut, t.OutDate = true, f.notOutNote(now), f.home
		}
	}
	_, t.Reminding = s.marksOf(pid).Has(watch.KindMovie, tmdbID, watch.MarkRemind)
	var wg sync.WaitGroup
	wg.Add(4)
	go func() { defer wg.Done(); t.Ratings = s.ratingsFor(ctx, d.IMDBID) }()
	go func() { defer wg.Done(); t.LogoURL, _ = tm.Logo(ctx, watch.KindMovie, tmdbID) }()
	go func() { defer wg.Done(); t.Collection = s.filmSeries(r, d.BelongsToCollection, tmdbID) }()
	go func() {
		defer wg.Done()
		more, _ := tm.RelatedMovies(ctx, tmdbID, metadata.RelatedRecommended)
		t.More = movieCards(more)
		s.markSoon(ctx, t.More)
		t.More = outOnly(t.More) // what's not out yet is on Coming soon
		if prof, ok := s.activeProfile(r); ok {
			t.More = s.ageFiltered(ctx, prof, t.More)
		}
	}()
	wg.Wait()
	alsoTonight(r, t.More, 8)
	writeJSON(w, http.StatusOK, t)
}

// GET /api/watch/tv/{tmdbId}
func (s *Server) handleWatchShow(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	tmdbID, err := strconv.Atoi(r.PathValue("tmdbId"))
	if err != nil || tmdbID <= 0 {
		writeError(w, http.StatusBadRequest, "That isn't a valid TMDB id.")
		return
	}
	ctx := r.Context()
	tm := s.TMDB()
	if s.ageBlocked(w, r, watch.KindTV, tmdbID) {
		return
	}
	d, err := tm.GetShowFull(ctx, tmdbID)
	if err != nil {
		writeUpstreamError(w, "load the show from TMDB", err)
		return
	}
	t := watchTitle{watchCard: showCard(d.Show), Tagline: d.Tagline, Genres: genreNames(d.Genres), Cast: castNames(d.Cast(10)),
		Directors: d.Creators(), IMDbID: d.ExternalIDs.IMDBID, InList: s.WatchRepo.InList(pid, watch.KindTV, tmdbID), Thumb: s.WatchRepo.Thumb(pid, watch.KindTV, tmdbID)}
	for _, n := range d.Networks {
		t.Networks = append(t.Networks, n.Name)
	}
	for _, se := range d.Seasons {
		if se.SeasonNumber == 0 || se.EpisodeCount == 0 {
			continue // specials stay out of the way, as on Netflix
		}
		name := se.Name
		if name == "" {
			name = fmt.Sprintf("Season %d", se.SeasonNumber)
		}
		t.Seasons = append(t.Seasons, watchSeason{Number: se.SeasonNumber, Name: name, Episodes: se.EpisodeCount})
	}
	sort.Slice(t.Seasons, func(i, j int) bool { return t.Seasons[i].Number < t.Seasons[j].Number })

	// Resume: the latest episode watched, or the one after it if it was
	// finished; for a show not started yet, the first episode.
	if ps, err := s.WatchRepo.ShowProgress(pid, tmdbID); err == nil && len(ps) > 0 {
		last := ps[0]
		if !last.Finished() {
			t.ResumeSeason, t.ResumeEpisode, t.ResumePosition = last.Season, last.Episode, last.Position
		} else if next, ok := s.nextEpisode(ctx, tmdbID, last.Season, last.Episode); ok {
			t.ResumeSeason, t.ResumeEpisode = next.Season, next.Episode
		}
	}
	if t.ResumeEpisode == 0 && len(t.Seasons) > 0 {
		t.ResumeSeason, t.ResumeEpisode = t.Seasons[0].Number, 1
	}
	// A show that hasn't started yet.
	if !aired(d.FirstAirDate) {
		t.Soon, t.OutDate = true, d.FirstAirDate
		t.NotOut = "Not out yet"
		if d.FirstAirDate != "" {
			t.NotOut = "Starts " + niceDate(d.FirstAirDate, time.Now())
		}
	}
	_, t.Reminding = s.marksOf(pid).Has(watch.KindTV, tmdbID, watch.MarkRemind)

	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); t.Ratings = s.ratingsFor(ctx, d.ExternalIDs.IMDBID) }()
	go func() { defer wg.Done(); t.LogoURL, _ = tm.Logo(ctx, watch.KindTV, tmdbID) }()
	go func() {
		defer wg.Done()
		more, _ := tm.RelatedShows(ctx, tmdbID, metadata.RelatedRecommended)
		t.More = showCards(more)
		if prof, ok := s.activeProfile(r); ok {
			t.More = s.ageFiltered(ctx, prof, t.More)
		}
	}()
	wg.Wait()
	alsoTonight(r, t.More, 8)
	writeJSON(w, http.StatusOK, t)
}

func genreNames(gs []metadata.Genre) []string {
	out := []string{}
	for _, g := range gs {
		out = append(out, g.Name)
	}
	return out
}

func castNames(cs []metadata.CastMember) []string {
	out := []string{}
	for _, c := range cs {
		out = append(out, c.Name)
	}
	return out
}

// ---- Search

// GET /api/watch/search?q=...: movies and shows, best match first.
func (s *Server) handleWatchSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, []watchCard{})
		return
	}
	if len(q) > 200 {
		q = q[:200]
	}
	results, err := s.TMDB().SearchMulti(r.Context(), q)
	if err != nil {
		writeUpstreamError(w, "search TMDB", err)
		return
	}
	out := []watchCard{}
	for _, m := range results {
		if m.PosterPath == "" {
			continue
		}
		out = append(out, watchCard{Kind: m.MediaType, TMDBID: m.ID, Title: m.DisplayTitle(), Year: m.Year(), Overview: m.Overview,
			PosterURL: metadata.PosterURL(m.PosterPath), BackdropURL: metadata.BackdropURL(m.BackdropPath), Rating: metadata.RoundRating(m.VoteAverage)})
	}
	s.markSoon(r.Context(), out) // films not out yet show "Coming soon"
	if prof, ok := s.activeProfile(r); ok {
		out = s.ageFiltered(r.Context(), prof, out)
	}
	writeJSON(w, http.StatusOK, out)
}

// ---- Progress

// PUT /api/watch/progress {"kind","tmdbId","season","episode","position","duration"}:
// the player saves where it is every few seconds.
func (s *Server) handlePutWatchProgress(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	var req struct {
		Kind     string  `json:"kind"`
		TMDBID   int     `json:"tmdbId"`
		Season   int     `json:"season"`
		Episode  int     `json:"episode"`
		Position float64 `json:"position"`
		Duration float64 `json:"duration"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "That request couldn't be read.")
		return
	}
	if (req.Kind != watch.KindMovie && req.Kind != watch.KindTV) || req.TMDBID <= 0 || req.Position < 0 || req.Duration < 0 ||
		(req.Kind == watch.KindTV && req.Episode < 1) {
		writeError(w, http.StatusBadRequest, "That isn't a valid title or position.")
		return
	}
	if req.Kind == watch.KindMovie {
		req.Season, req.Episode = 0, 0
	}
	p := watch.Progress{Kind: req.Kind, TMDBID: req.TMDBID, Season: req.Season, Episode: req.Episode, Position: req.Position, Duration: req.Duration}
	// The title and artwork are looked up here (cached), not taken from the
	// player, so the rows only ever show what TMDB says.
	ctx := r.Context()
	if req.Kind == watch.KindMovie {
		if d, err := s.TMDB().GetMovieDetail(ctx, req.TMDBID); err == nil {
			p.Title, p.PosterPath, p.BackdropPath = d.Title, d.PosterPath, d.BackdropPath
		}
	} else {
		if d, err := s.TMDB().GetShowFull(ctx, req.TMDBID); err == nil {
			p.Title, p.PosterPath, p.BackdropPath = d.Name, d.PosterPath, d.BackdropPath
		}
		if eps, err := s.TMDB().GetSeasonCached(ctx, req.TMDBID, req.Season); err == nil {
			for _, e := range eps {
				if e.Episode == req.Episode {
					p.EpisodeTitle = e.Name
				}
			}
		}
	}
	if err := s.WatchRepo.SaveProgress(pid, p); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't save where you are.")
		return
	}
	// Watching it answers the reminder.
	_ = s.WatchRepo.SetMark(pid, req.Kind, req.TMDBID, watch.MarkRemind, false)
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/watch/progress/{kind}/{tmdbId}?season=&episode=: where to resume.
func (s *Server) handleGetWatchProgress(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	kind, tmdbID, ok := watchKind(w, r)
	if !ok {
		return
	}
	season, _ := strconv.Atoi(r.URL.Query().Get("season"))
	episode, _ := strconv.Atoi(r.URL.Query().Get("episode"))
	p, found, err := s.WatchRepo.GetProgress(pid, kind, tmdbID, season, episode)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't read where you were.")
		return
	}
	out := struct {
		Position float64 `json:"position"`
		Duration float64 `json:"duration"`
		Finished bool    `json:"finished"`
	}{}
	if found {
		out.Position, out.Duration, out.Finished = p.Position, p.Duration, p.Finished()
	}
	writeJSON(w, http.StatusOK, out)
}

// DELETE /api/watch/progress/{kind}/{tmdbId}: take it off Continue Watching.
func (s *Server) handleForgetWatchProgress(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	kind, tmdbID, ok := watchKind(w, r)
	if !ok {
		return
	}
	if err := s.WatchRepo.ForgetTitle(pid, kind, tmdbID); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't remove it.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- My List

// PUT /api/watch/list/{kind}/{tmdbId}
func (s *Server) handleAddToWatchList(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	kind, tmdbID, ok := watchKind(w, r)
	if !ok {
		return
	}
	it := watch.ListItem{Kind: kind, TMDBID: tmdbID}
	if kind == watch.KindMovie {
		d, err := s.TMDB().GetMovieDetail(r.Context(), tmdbID)
		if err != nil {
			writeUpstreamError(w, "look the movie up on TMDB", err)
			return
		}
		it.Title, it.Year, it.PosterPath, it.BackdropPath = d.Title, d.Year(), d.PosterPath, d.BackdropPath
	} else {
		d, err := s.TMDB().GetShowFull(r.Context(), tmdbID)
		if err != nil {
			writeUpstreamError(w, "look the show up on TMDB", err)
			return
		}
		it.Title, it.Year, it.PosterPath, it.BackdropPath = d.Name, d.Year(), d.PosterPath, d.BackdropPath
	}
	if err := s.WatchRepo.AddToList(pid, it); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't add it to My List.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/watch/list/{kind}/{tmdbId}
func (s *Server) handleRemoveFromWatchList(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	kind, tmdbID, ok := watchKind(w, r)
	if !ok {
		return
	}
	if err := s.WatchRepo.RemoveFromList(pid, kind, tmdbID); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't take it off My List.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- IMDb ratings (OMDb)

// omdbState keeps one OMDb client per saved key, so its day-long cache of
// ratings survives between requests.
type omdbState struct {
	mu     sync.Mutex
	client *omdb.Client
	base   string // tests point OMDb elsewhere; "" = the real one
}

// omdbClient is the client for the saved key, or nil without one.
func (s *Server) omdbClient() *omdb.Client {
	key, err := s.Settings.Get(settings.KeyOMDbAPIKey)
	key = strings.TrimSpace(key)
	if err != nil || key == "" {
		return nil
	}
	s.omdb.mu.Lock()
	defer s.omdb.mu.Unlock()
	if s.omdb.client == nil || s.omdb.client.Key() != key {
		s.omdb.client = omdb.New(key, s.omdb.base)
	}
	return s.omdb.client
}

// ratingsFor is a title's IMDb and Rotten Tomatoes ratings, or none without
// an OMDb key (or when OMDb doesn't answer quickly).
func (s *Server) ratingsFor(ctx context.Context, imdbID string) omdb.Ratings {
	c := s.omdbClient()
	if c == nil || imdbID == "" {
		return omdb.Ratings{}
	}
	// The page doesn't wait long for OMDb: past ratingsWait it shows without
	// the ratings, and the lookup finishes in the background so they are
	// there (from the cache) next time.
	ch := make(chan omdb.Ratings, 1)
	go func() {
		bg, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		r, err := c.Ratings(bg, imdbID)
		if err != nil {
			slog.Info("watch: OMDb ratings", "imdbId", imdbID, "err", err)
		}
		ch <- r
	}()
	select {
	case r := <-ch:
		return r
	case <-time.After(ratingsWait):
		return omdb.Ratings{}
	}
}

const ratingsWait = 1500 * time.Millisecond

// GET /api/settings/omdb: whether an OMDb key is saved.
func (s *Server) handleGetOMDb(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"set": s.omdbClient() != nil})
}

// PUT /api/settings/omdb {"apiKey": "..."}: checks the key with OMDb (on a
// well-known title) and saves it, encrypted. An empty key removes it.
func (s *Server) handlePutOMDb(w http.ResponseWriter, r *http.Request) {
	var req struct {
		APIKey string `json:"apiKey"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "That request couldn't be read.")
		return
	}
	key := strings.TrimSpace(req.APIKey)
	if len(key) > 64 || strings.ContainsAny(key, " \r\n\t&?=/") {
		writeError(w, http.StatusBadRequest, "That doesn't look like an OMDb API key. Copy it again from the email OMDb sent you.")
		return
	}
	if key != "" {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		_, err := omdb.New(key, s.omdb.base).Ratings(ctx, "tt0111161")
		if errors.Is(err, omdb.ErrBadKey) {
			writeError(w, http.StatusBadRequest, "OMDb didn't accept that key. Check you clicked the activation link in OMDb's email.")
			return
		}
		if err != nil {
			slog.Warn("omdb: check key", "err", err)
			writeError(w, http.StatusBadGateway, "Couldn't reach OMDb to check the key. Try again in a moment.")
			return
		}
	}
	if err := s.Settings.Set(settings.KeyOMDbAPIKey, key, true); err != nil {
		writeError(w, http.StatusInternalServerError, "The key couldn't be saved.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"set": key != ""})
}

// GET /api/watch/logo/{kind}/{tmdbId}: a title's logo artwork, for the TV
// apps' banner as the highlight moves along a row. {"url": ""} when TMDB
// has none.
func (s *Server) handleWatchLogo(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	tmdbID, err := strconv.Atoi(r.PathValue("tmdbId"))
	if (kind != watch.KindMovie && kind != watch.KindTV) || err != nil || tmdbID <= 0 {
		writeError(w, http.StatusBadRequest, "That isn't a movie or show.")
		return
	}
	u, err := s.TMDB().Logo(r.Context(), kind, tmdbID)
	if err != nil {
		writeUpstreamError(w, "look up the logo on TMDB", err)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=86400")
	writeJSON(w, http.StatusOK, map[string]string{"url": u})
}

// watchExtras is what the apps need to bring a title's artwork alive.
type watchExtras struct {
	Logo    string `json:"logo"`              // logo artwork, "" when none
	Colour  string `json:"colour"`            // the picture's main colour, "r, g, b"
	Trailer string `json:"trailer,omitempty"` // a YouTube video id
}

// GET /api/watch/extras/{kind}/{tmdbId}: a title's logo, the main colour of
// its picture (for the glow and highlights) and its trailer (for the living
// banner), in one answer the apps ask for as the highlight rests on a title.
func (s *Server) handleWatchExtras(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	tmdbID, err := strconv.Atoi(r.PathValue("tmdbId"))
	if (kind != watch.KindMovie && kind != watch.KindTV) || err != nil || tmdbID <= 0 {
		writeError(w, http.StatusBadRequest, "That isn't a movie or show.")
		return
	}
	ctx := r.Context()
	tm := s.TMDB()
	var backdrop string
	var trailers []metadata.Trailer
	if kind == watch.KindMovie {
		d, err := tm.GetMovieDetail(ctx, tmdbID)
		if err != nil {
			writeUpstreamError(w, "load the movie from TMDB", err)
			return
		}
		backdrop, trailers = d.BackdropPath, d.Trailers()
	} else {
		d, err := tm.GetShowFull(ctx, tmdbID)
		if err != nil {
			writeUpstreamError(w, "load the show from TMDB", err)
			return
		}
		backdrop, trailers = d.BackdropPath, d.Trailers()
	}
	var ex watchExtras
	if len(trailers) > 0 {
		ex.Trailer = trailers[0].Key
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); ex.Logo, _ = tm.Logo(ctx, kind, tmdbID) }()
	go func() { defer wg.Done(); ex.Colour, _ = tm.MainColour(ctx, backdrop) }()
	wg.Wait()
	w.Header().Set("Cache-Control", "private, max-age=3600")
	writeJSON(w, http.StatusOK, ex)
}

// yearShow is a title in the year's recap, with its picture.
type yearShow struct {
	watch.YearShow
	BackdropURL string `json:"backdropUrl,omitempty"`
}

type yearAnswer struct {
	watch.YearStats
	Top      []yearShow `json:"top"`
	TopMovie *yearShow  `json:"topMovie,omitempty"`
}

// GET /api/watch/year?year=2026: the profile's year in Cue (this year
// unless asked): hours, titles, the most watched shows and movie, the
// busiest day and month.
func (s *Server) handleWatchYear(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	year := time.Now().Year()
	if v := r.URL.Query().Get("year"); v != "" {
		y, err := strconv.Atoi(v)
		if err != nil || y < 2000 || y > year {
			writeError(w, http.StatusBadRequest, "That isn't a year Cue can look back on.")
			return
		}
		year = y
	}
	st, err := s.WatchRepo.Year(pid, year)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't add up the year.")
		return
	}
	pic := func(y watch.YearShow) yearShow { return yearShow{y, metadata.BackdropURL(y.BackdropPath)} }
	a := yearAnswer{YearStats: st, Top: make([]yearShow, 0, len(st.Top))}
	for _, t := range st.Top {
		a.Top = append(a.Top, pic(t))
	}
	if st.TopMovie != nil {
		m := pic(*st.TopMovie)
		a.TopMovie = &m
	}
	writeJSON(w, http.StatusOK, a)
}

// DELETE /api/watch/continue/{kind}/{tmdbId}: takes a title off Continue
// Watching. Where you were is kept (and your year in Cue still counts it);
// watching it again brings it back.
func (s *Server) handleHideFromContinue(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	kind, tmdbID, ok := watchKind(w, r)
	if !ok {
		return
	}
	if err := s.WatchRepo.SetMark(pid, kind, tmdbID, watch.MarkHidden, true); err != nil {
		slog.Warn("watch: hide from continue", "err", err)
		writeError(w, http.StatusInternalServerError, "Couldn't take it off the row.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PUT /api/watch/thumb/{kind}/{tmdbId} {"thumb":"up"|"down"|""}: this
// profile's thumb on a title. Up makes it a reason for "Because you
// watched"; down ("Not for me") keeps it off Home's rows.
func (s *Server) handlePutThumb(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	kind, tmdbID, ok := watchKind(w, r)
	if !ok {
		return
	}
	var req struct {
		Thumb string `json:"thumb"`
	}
	if err := decodeJSON(r, &req); err != nil || (req.Thumb != "" && req.Thumb != watch.MarkUp && req.Thumb != watch.MarkDown) {
		writeError(w, http.StatusBadRequest, `A thumb is "up", "down" or "".`)
		return
	}
	var err error
	if req.Thumb == "" {
		err = s.WatchRepo.SetMark(pid, kind, tmdbID, watch.MarkUp, false) // clears either
	} else {
		err = s.WatchRepo.SetMark(pid, kind, tmdbID, req.Thumb, true)
	}
	if err != nil {
		slog.Warn("watch: thumb", "err", err)
		writeError(w, http.StatusInternalServerError, "Couldn't save that.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"thumb": req.Thumb})
}

// filmSeries is the series a film is in (the films in release order, those
// above a kids' profile's limit left out), or nil.
func (s *Server) filmSeries(r *http.Request, ref *metadata.CollectionRef, tmdbID int) *watchCollection {
	if ref == nil || ref.ID <= 0 {
		return nil
	}
	ctx := r.Context()
	col, err := s.TMDB().GetCollection(ctx, ref.ID)
	if err != nil || len(col.Parts) < 2 {
		return nil
	}
	parts := movieCards(col.Parts)
	s.markSoon(ctx, parts)
	if prof, ok := s.activeProfile(r); ok {
		parts = s.ageFiltered(ctx, prof, parts)
	}
	out := &watchCollection{ID: col.ID, Name: col.Name, Parts: parts}
	for i, c := range parts {
		if c.TMDBID == tmdbID && i+1 < len(parts) && !parts[i+1].Soon {
			next := parts[i+1]
			out.Next = &next
		}
	}
	return out
}
