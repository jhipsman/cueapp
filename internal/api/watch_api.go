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

	"github.com/rdborg/mediarium/internal/metadata"
	"github.com/rdborg/mediarium/internal/omdb"
	"github.com/rdborg/mediarium/internal/settings"
	"github.com/rdborg/mediarium/internal/watch"
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
}

type watchRow struct {
	Key   string      `json:"key"`
	Title string      `json:"title"`
	Items []watchCard `json:"items"`
}

func movieCard(m metadata.Movie) watchCard {
	return watchCard{Kind: watch.KindMovie, TMDBID: m.TMDBID, Title: m.Title, Year: m.Year(), Overview: m.Overview,
		PosterURL: metadata.PosterURL(m.PosterPath), BackdropURL: metadata.BackdropURL(m.BackdropPath), Rating: metadata.RoundRating(m.VoteAverage)}
}

func showCard(sh metadata.Show) watchCard {
	return watchCard{Kind: watch.KindTV, TMDBID: sh.TMDBID, Title: sh.Name, Year: sh.Year(), Overview: sh.Overview,
		PosterURL: metadata.PosterURL(sh.PosterPath), BackdropURL: metadata.BackdropURL(sh.BackdropPath), Rating: metadata.RoundRating(sh.VoteAverage)}
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
			shelves[i] = watchRow{Key: sh.key, Title: sh.title, Items: items}
		}(i, sh)
	}
	cont := s.continueWatching(ctx, pid)
	wg.Wait()

	home := watchHome{}
	if len(cont) > 0 {
		home.Rows = append(home.Rows, watchRow{Key: "continue", Title: "Continue watching", Items: cont})
	}
	if list := s.myListCards(pid); len(list) > 0 {
		home.Rows = append(home.Rows, watchRow{Key: "my-list", Title: "My List", Items: list})
	}
	for _, row := range shelves {
		if len(row.Items) > 0 {
			home.Rows = append(home.Rows, row)
		}
	}
	// The banner: something trending this week with artwork, changing
	// through the day so the page doesn't look the same every visit.
	for _, row := range shelves {
		if row.Key != "trending-movies" && row.Key != "trending-tv" {
			continue
		}
		var withArt []watchCard
		for _, c := range row.Items {
			if c.BackdropURL != "" && c.Overview != "" {
				withArt = append(withArt, c)
			}
		}
		if len(withArt) > 0 {
			hero := withArt[time.Now().Hour()%min(len(withArt), 5)]
			home.Hero = &hero
			break
		}
	}
	if home.Rows == nil {
		home.Rows = []watchRow{}
	}
	writeJSON(w, http.StatusOK, home)
}

// continueWatching is userID's titles in progress: a movie or episode
// started and not finished, or the next episode after one finished.
func (s *Server) continueWatching(ctx context.Context, userID int64) []watchCard {
	recent, err := s.WatchRepo.Recent(userID, 20)
	if err != nil {
		slog.Warn("watch: continue watching", "err", err)
		return nil
	}
	out := []watchCard{}
	for _, p := range recent {
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
		} else {
			c.Progress = p.Fraction()
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

// ---- Episodes

// aired says a TMDB air date ("2024-03-01") is today or earlier.
func aired(date string) bool {
	return date != "" && date <= time.Now().Format("2006-01-02")
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
	d, err := tm.GetMovieDetail(ctx, tmdbID)
	if err != nil {
		writeUpstreamError(w, "load the movie from TMDB", err)
		return
	}
	t := watchTitle{watchCard: movieCard(d.Movie), Tagline: d.Tagline, Runtime: d.Runtime, Genres: genreNames(d.Genres),
		Certification: d.Certification(), Cast: castNames(d.Cast(10)), Directors: d.Directors(), IMDbID: d.IMDBID,
		InList: s.WatchRepo.InList(pid, watch.KindMovie, tmdbID)}
	if p, ok, _ := s.WatchRepo.GetProgress(pid, watch.KindMovie, tmdbID, 0, 0); ok && !p.Finished() {
		t.ResumePosition = p.Position
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); t.Ratings = s.ratingsFor(ctx, d.IMDBID) }()
	go func() {
		defer wg.Done()
		more, _ := tm.RelatedMovies(ctx, tmdbID, metadata.RelatedRecommended)
		t.More = movieCards(more)
	}()
	wg.Wait()
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
	d, err := tm.GetShowFull(ctx, tmdbID)
	if err != nil {
		writeUpstreamError(w, "load the show from TMDB", err)
		return
	}
	t := watchTitle{watchCard: showCard(d.Show), Tagline: d.Tagline, Genres: genreNames(d.Genres), Cast: castNames(d.Cast(10)),
		Directors: d.Creators(), IMDbID: d.ExternalIDs.IMDBID, InList: s.WatchRepo.InList(pid, watch.KindTV, tmdbID)}
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

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); t.Ratings = s.ratingsFor(ctx, d.ExternalIDs.IMDBID) }()
	go func() {
		defer wg.Done()
		more, _ := tm.RelatedShows(ctx, tmdbID, metadata.RelatedRecommended)
		t.More = showCards(more)
	}()
	wg.Wait()
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
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	r, err := c.Ratings(ctx, imdbID)
	if err != nil {
		slog.Info("watch: OMDb ratings", "imdbId", imdbID, "err", err)
	}
	return r
}

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
