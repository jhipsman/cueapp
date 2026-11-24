package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"
	"github.com/anthropics/anthropic-sdk-go/v2/toolrunner"

	"github.com/jhipsman/cueapp/internal/metadata"
	"github.com/jhipsman/cueapp/internal/plainerror"
	"github.com/jhipsman/cueapp/internal/settings"
	"github.com/jhipsman/cueapp/internal/watch"
)

// Ask Cue: say what you're in the mood for ("something funny for tonight,
// under two hours"), or ask about a film, a show or a person, and Cue
// answers with titles you can play. Claude reads the request and looks
// things up through Cue's own lists (search, genres, Top 10s, the title's
// details, your list), which already keep to the profile's age limit, so
// it never offers anything the profile couldn't otherwise see.
//
// It needs a Claude API key, added by the owner in Settings (or the
// ANTHROPIC_API_KEY environment variable).

// Ask Cue's models: Sonnet by default (it knows films and shows well and
// keeps to "like X, but Y"; a cent or two a question), Haiku when the owner
// wants it cheapest, Opus when they want the smartest answers.
const (
	askModelHaiku  = "claude-haiku-5-5"
	askModelSonnet = "claude-sonnet-5-5"
	askModelOpus   = "claude-opus-5-5"
)

// askModel is the model the owner chose in Settings (Sonnet unless "haiku"
// or "opus").
func (s *Server) askModel() string {
	switch v, _ := s.Settings.Get(settings.KeyAnthropicModel); v {
	case "opus":
		return askModelOpus
	case "haiku":
		return askModelHaiku
	}
	return askModelSonnet
}

// askModelName is the Settings name of the model in use.
func (s *Server) askModelName() string {
	switch s.askModel() {
	case askModelOpus:
		return "opus"
	case askModelHaiku:
		return "haiku"
	}
	return "sonnet"
}

// anthropicKey is Ask Cue's key: the one saved in Settings, else the
// environment's.
func (s *Server) anthropicKey() string {
	if v, err := s.Settings.Get(settings.KeyAnthropicAPIKey); err == nil && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
}

type askTurn struct {
	Role string `json:"role"` // user, assistant
	Text string `json:"text"`
}

type askAnswer struct {
	Reply   string      `json:"reply"`
	Heading string      `json:"heading,omitempty"` // over the titles shown ("Funny, under two hours")
	Cards   []watchCard `json:"cards"`
	// Play, when asked to play something: where the player should go.
	Play *struct {
		Kind    string `json:"kind"`
		TMDBID  int    `json:"tmdbId"`
		Season  int    `json:"season,omitempty"`
		Episode int    `json:"episode,omitempty"`
	} `json:"play,omitempty"`
	Open *struct {
		Kind   string `json:"kind"`
		TMDBID int    `json:"tmdbId"`
	} `json:"open,omitempty"`
}

// GET /api/watch/ask: whether Ask Cue is set up.
func (s *Server) handleAskStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ready": s.anthropicKey() != ""})
}

// POST /api/watch/ask {messages: [{role, text}]}: the conversation so far,
// the last turn the person's; the answer and the titles to show.
func (s *Server) handleAsk(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	key := s.anthropicKey()
	if key == "" {
		writeError(w, http.StatusServiceUnavailable, "Ask Cue needs a Claude API key: the owner can add one in Settings > Movie info and subtitles.")
		return
	}
	var req struct {
		Messages []askTurn `json:"messages"`
	}
	if err := decodeJSON(r, &req); err != nil || len(req.Messages) == 0 || req.Messages[len(req.Messages)-1].Role != "user" {
		writeError(w, http.StatusBadRequest, "Ask something.")
		return
	}
	if len(req.Messages) > 20 {
		req.Messages = req.Messages[len(req.Messages)-20:]
	}
	prof, _ := s.activeProfile(r)
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	a := &asker{s: s, r: r, pid: pid, seen: map[string]watchCard{}}
	tools, err := a.tools()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Ask Cue couldn't start.")
		return
	}
	var msgs []anthropic.BetaMessageParam
	for _, m := range req.Messages {
		text := truncate(strings.TrimSpace(m.Text), 2000)
		if text == "" {
			continue
		}
		if m.Role == "assistant" {
			msgs = append(msgs, anthropic.BetaMessageParam{OfAssistant: &anthropic.BetaAssistantMessageParam{Content: []anthropic.BetaContentBlockParamUnion{anthropic.NewBetaTextBlock(text)}}})
		} else {
			msgs = append(msgs, anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(text)))
		}
	}
	client := anthropic.NewClient(option.WithAPIKey(key))
	model := s.askModel()
	params := anthropic.BetaMessageNewParams{
		Model:     model,
		MaxTokens: 16000,
		System:    []anthropic.BetaTextBlockParam{{Text: askSystem(prof)}},
		Messages:  msgs,
		// Medium: enough thought to weigh picks against everything asked
		// (low gave the first popular titles that roughly fit).
		OutputConfig: anthropic.BetaOutputConfigParam{Effort: anthropic.BetaOutputConfigEffortMedium},
	}
	if model != askModelHaiku {
		// A declined request is answered by another model instead (Sonnet
		// and Opus: Haiku has no such fallback).
		params.Betas = []anthropic.AnthropicBeta{"server-side-fallback-2026-07-01"}
		params.Fallbacks = anthropic.BetaFallbacksParamOfDefault()
	}
	// Room to think of candidates, look them up, check them and widen the
	// search once, before showing them.
	runner := client.Beta.Messages.NewToolRunner(tools, anthropic.BetaToolRunnerParams{BetaMessageNewParams: params, MaxIterations: 10})
	msg, err := runner.RunToCompletion(ctx)
	if err != nil {
		slog.Warn("ask cue", "err", err)
		writeError(w, http.StatusBadGateway, "Ask Cue couldn't answer right now. Try again in a moment.")
		return
	}
	var reply []string
	for _, b := range msg.Content {
		if t, ok := b.AsAny().(anthropic.BetaTextBlock); ok && strings.TrimSpace(t.Text) != "" {
			reply = append(reply, strings.TrimSpace(t.Text))
		}
	}
	if msg.StopReason == anthropic.BetaStopReasonRefusal && len(reply) == 0 {
		reply = append(reply, "I can't help with that one. Ask me for something to watch.")
	}
	a.mu.Lock()
	ans := a.ans
	a.mu.Unlock()
	ans.Reply = strings.Join(reply, "\n\n")
	if ans.Reply == "" {
		// Out of turns (or a reply of tool calls only): never a blank answer.
		switch {
		case ans.Play != nil:
			ans.Reply = "Playing it now."
		case ans.Open != nil:
			ans.Reply = "Here it is."
		case len(ans.Cards) > 0:
			ans.Reply = "Here's what I found."
		default:
			ans.Reply = "I couldn't find a good match for that. Try saying it another way, or tell me a film or show you liked."
		}
	}
	if ans.Cards == nil {
		ans.Cards = []watchCard{}
	}
	writeJSON(w, http.StatusOK, ans)
}

func askSystem(p watch.Profile) string {
	var b strings.Builder
	b.WriteString(`You are Ask Cue, the assistant in Cue, a family's home streaming app. People ask you for something to watch ("something funny for tonight", "a film like Dune but shorter", "a cozy mystery show"), to play a title, or about a film, a show or a person.

Picking titles is the main job, and the picks should feel chosen by a friend who knows film and TV well, not pulled from a popularity chart:
- Read the request closely and treat every constraint as a hard rule: films or shows, length, decade, mood, tone, genre, service, who's watching (kids, a date, the whole family), and anything they said they didn't want. A pick that breaks one is a bad pick, however popular.
- Start from your own knowledge: think of the titles that fit best, including well-loved and lesser-known ones, then look them all up at once with find_titles. It returns each title's running time, genres, rating and seasons, so check the constraints against them and drop what doesn't fit.
- For "like X", look X up and use similar_titles too, then keep only what shares what the person liked about X (its tone, premise, style), not just its genre.
- browse is for broad asks ("a good documentary", "best rated sci-fi of the 90s"); sort by best_rated rather than popular unless they want what's big right now.
- my_titles shows what this person is watching and saved: use it for taste, and don't suggest what they're already watching unless asked.
- Only offer titles a tool returned: they're already limited to what this profile may watch. If something you thought of wasn't found, leave it out.
- Call show_titles once with 4 to 8 picks, best fit first, and a short heading that says what they are. Then reply in one or two warm sentences: why these fit, naming the first pick or two. The posters appear under your reply, so don't list every title.
- If a request is vague, still make good picks, and say in a few words what you went for.

Other requests:
- To play a specific title ("play Dune", "put on the next Bluey"), find it and call play_title. For a show, play the episode they asked for, or leave season and episode at 0 to carry on where they were.
- To see a title's page ("show me Dune"), call open_title.
- For questions (who directed it, how long is it, who's in it), look it up with title_info and answer briefly.

Keep replies short, plain text, no markdown, no lists.
`)
	b.WriteString(fmt.Sprintf("\nToday is %s. The person asking is %q.", time.Now().Format("Monday, January 2, 2006"), p.Name))
	if p.MaxRating != "" {
		b.WriteString(fmt.Sprintf(" This is a child's profile limited to %s: suggest titles for children and families.", p.MaxRating))
	}
	return b.String()
}

// asker holds one request's tools and what they decided to show.
type asker struct {
	s    *Server
	r    *http.Request
	pid  int64
	mu   sync.Mutex
	seen map[string]watchCard // every title a tool returned, by kind-id
	ans  askAnswer
}

func cardKey(kind string, id int) string { return fmt.Sprintf("%s-%d", kind, id) }

// note remembers titles a tool returned and describes them for Claude.
func (a *asker) note(cards []watchCard) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	type brief struct {
		Kind     string  `json:"kind"`
		ID       int     `json:"id"`
		Title    string  `json:"title"`
		Year     int     `json:"year,omitempty"`
		Rating   float64 `json:"rating,omitempty"`
		Overview string  `json:"overview,omitempty"`
	}
	out := []brief{}
	for _, c := range cards {
		a.seen[cardKey(c.Kind, c.TMDBID)] = c
		out = append(out, brief{c.Kind, c.TMDBID, c.Title, c.Year, c.Rating, truncate(c.Overview, 160)})
	}
	js, _ := json.Marshal(out)
	if len(out) == 0 {
		return "No titles found."
	}
	return string(js)
}

func textResult(s string) anthropic.BetaToolResultBlockParamContentUnion {
	return anthropic.BetaToolResultBlockParamContentUnion{OfText: &anthropic.BetaTextBlockParam{Text: s}}
}

type askSearchIn struct {
	Query string `json:"query" jsonschema:"required,description=A title or a person's name"`
}

type askBrowseIn struct {
	Kind       string  `json:"kind" jsonschema:"required,enum=movie,enum=tv,enum=any,description=Films, shows or either"`
	Genre      string  `json:"genre,omitempty" jsonschema:"description=One of: Action, Comedy, Drama, Sci-Fi, Family, Animation, Crime, Mystery, Thriller, Romance, Documentary, Reality, Kids, Fantasy, History & War, Western"`
	Decade     int     `json:"decade,omitempty" jsonschema:"description=A decade such as 1990 for the 1990s; 0 for any"`
	MaxMinutes int     `json:"max_minutes,omitempty" jsonschema:"description=Longest running time in minutes; 0 for any"`
	MinRating  float64 `json:"min_rating,omitempty" jsonschema:"description=Lowest audience score out of 10; 0 for any"`
	Service    string  `json:"service,omitempty" jsonschema:"description=Only what's on one service: netflix, prime, appletv, disney, hulu, max, peacock or paramount"`
	Sort       string  `json:"sort,omitempty" jsonschema:"enum=best_rated,enum=well_known,enum=popular,description=best_rated (highly rated with enough votes; the default), well_known (the most voted for) or popular (what's big right now)"`
}

type askFindIn struct {
	Titles []askFindTitle `json:"titles" jsonschema:"required,description=The titles you have in mind, up to 15"`
}

type askFindTitle struct {
	Title string `json:"title" jsonschema:"required"`
	Year  int    `json:"year,omitempty" jsonschema:"description=Its year, if you know it: tells same-named titles apart"`
	Kind  string `json:"kind,omitempty" jsonschema:"enum=movie,enum=tv,description=movie or tv, if you know it"`
}

type askTitleIn struct {
	Kind string `json:"kind" jsonschema:"required,enum=movie,enum=tv"`
	ID   int    `json:"id" jsonschema:"required,description=The title's id from a tool result"`
}

type askShowIn struct {
	Heading string       `json:"heading" jsonschema:"required,description=A few words over the posters, such as Funny and under two hours"`
	Titles  []askTitleIn `json:"titles" jsonschema:"required,description=The picks, best first, from earlier tool results"`
}

type askPlayIn struct {
	Kind    string `json:"kind" jsonschema:"required,enum=movie,enum=tv"`
	ID      int    `json:"id" jsonschema:"required"`
	Season  int    `json:"season,omitempty" jsonschema:"description=For a show: the season, or 0 to carry on where they were"`
	Episode int    `json:"episode,omitempty" jsonschema:"description=For a show: the episode, or 0"`
}

type askEmpty struct{}

func (a *asker) tools() ([]anthropic.BetaTool, error) {
	ctx := a.r.Context()
	filter := func(cards []watchCard) []watchCard {
		if prof, ok := a.s.activeProfile(a.r); ok {
			cards = a.s.ageFiltered(ctx, prof, cards)
		}
		return cards
	}
	search, err := toolrunner.NewBetaToolFromJSONSchema("search_titles", "Search films and shows by title, or everything a person has been in by their name.",
		func(ctx context.Context, in askSearchIn) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			tm := a.s.TMDB()
			if tm == nil {
				return textResult("Search isn't available."), nil
			}
			res, err := tm.SearchMulti(ctx, truncate(in.Query, 200))
			if err != nil {
				return textResult("Search failed: try again."), nil
			}
			var cards []watchCard
			for _, m := range res {
				if m.PosterPath == "" {
					continue
				}
				cards = append(cards, watchCard{Kind: m.MediaType, TMDBID: m.ID, Title: m.DisplayTitle(), Year: m.Year(), Overview: m.Overview,
					PosterURL: metadata.PosterURL(m.PosterPath), BackdropURL: metadata.BackdropURL(m.BackdropPath), Rating: metadata.RoundRating(m.VoteAverage)})
			}
			if len(cards) > 12 {
				cards = cards[:12]
			}
			return textResult(a.note(filter(cards))), nil
		})
	if err != nil {
		return nil, err
	}
	find, err := toolrunner.NewBetaToolFromJSONSchema("find_titles", "Look up the films and shows you have in mind, all at once: each comes back with its id, year, running time, genres, rating, seasons and story, or is missing if it wasn't found.",
		func(ctx context.Context, in askFindIn) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			tm := a.s.TMDB()
			if tm == nil {
				return textResult("Search isn't available."), nil
			}
			wanted := in.Titles
			if len(wanted) > 15 {
				wanted = wanted[:15]
			}
			found := make([]*askFound, len(wanted))
			var wg sync.WaitGroup
			sem := make(chan struct{}, 6)
			for i, t := range wanted {
				wg.Add(1)
				go func(i int, t askFindTitle) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()
					found[i] = a.findOne(ctx, tm, t)
				}(i, t)
			}
			wg.Wait()
			var cards []watchCard
			facts := map[string]*askFound{}
			var missing []string
			for i, f := range found {
				if f == nil {
					missing = append(missing, wanted[i].Title)
					continue
				}
				cards = append(cards, f.card)
				facts[cardKey(f.card.Kind, f.card.TMDBID)] = f
			}
			allowed := filter(cards)
			kept := map[string]bool{}
			for _, c := range allowed {
				kept[cardKey(c.Kind, c.TMDBID)] = true
			}
			a.note(allowed)
			type out struct {
				Kind    string   `json:"kind"`
				ID      int      `json:"id"`
				Title   string   `json:"title"`
				Year    int      `json:"year,omitempty"`
				Minutes int      `json:"minutes,omitempty"`
				Seasons int      `json:"seasons,omitempty"`
				Genres  []string `json:"genres,omitempty"`
				Rated   string   `json:"rated,omitempty"`
				Score   float64  `json:"score,omitempty"`
				Story   string   `json:"story,omitempty"`
			}
			res := struct {
				Found    []out    `json:"found"`
				NotFound []string `json:"not_found,omitempty"`
				Hidden   int      `json:"hidden_by_age_limit,omitempty"`
			}{Found: []out{}, NotFound: missing}
			for _, c := range cards {
				k := cardKey(c.Kind, c.TMDBID)
				if !kept[k] {
					res.Hidden++
					continue
				}
				f := facts[k]
				res.Found = append(res.Found, out{c.Kind, c.TMDBID, c.Title, c.Year, f.minutes, f.seasons, f.genres, f.rated, c.Rating, truncate(c.Overview, 200)})
			}
			js, _ := json.Marshal(res)
			return textResult(string(js)), nil
		})
	if err != nil {
		return nil, err
	}
	similar, err := toolrunner.NewBetaToolFromJSONSchema("similar_titles", "What people who liked a film or show also liked (by its id from a tool result).",
		func(ctx context.Context, in askTitleIn) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			tm := a.s.TMDB()
			if tm == nil {
				return textResult("Not available."), nil
			}
			var cards []watchCard
			if in.Kind == "movie" {
				ms, err := tm.RelatedMovies(ctx, in.ID, metadata.RelatedRecommended)
				if err == nil && len(ms) == 0 {
					ms, err = tm.RelatedMovies(ctx, in.ID, metadata.RelatedSimilar)
				}
				if err != nil {
					return textResult("Couldn't find titles like that one."), nil
				}
				for _, m := range ms {
					if m.PosterPath != "" {
						cards = append(cards, movieCard(m))
					}
				}
			} else {
				ss, err := tm.RelatedShows(ctx, in.ID, metadata.RelatedRecommended)
				if err == nil && len(ss) == 0 {
					ss, err = tm.RelatedShows(ctx, in.ID, metadata.RelatedSimilar)
				}
				if err != nil {
					return textResult("Couldn't find titles like that one."), nil
				}
				for _, sh := range ss {
					if sh.PosterPath != "" {
						cards = append(cards, showCard(sh))
					}
				}
			}
			if len(cards) > 20 {
				cards = cards[:20]
			}
			return textResult(a.note(filter(cards))), nil
		})
	if err != nil {
		return nil, err
	}
	browse, err := toolrunner.NewBetaToolFromJSONSchema("browse", "Find well-rated (or popular) films or shows by genre, decade, length, score or streaming service.",
		func(ctx context.Context, in askBrowseIn) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			tm := a.s.TMDB()
			if tm == nil {
				return textResult("Browsing isn't available."), nil
			}
			q := metadata.TopQuery{Region: "US", MaxRuntime: in.MaxMinutes, MinRating: in.MinRating, Sort: "rating"}
			switch in.Sort {
			case "well_known":
				q.Sort = "votes"
			case "popular":
				q.Sort = ""
			}
			if in.Decade >= 1920 && in.Decade%10 == 0 {
				q.Decade = in.Decade
			}
			for _, svc := range topServices {
				if svc.Key == in.Service {
					q.Providers = svc.providers
				}
			}
			ids, hasGenre := topGenres[in.Genre]
			var cards []watchCard
			if in.Kind != "tv" && (!hasGenre || ids[0] > 0) {
				mq := q
				if hasGenre {
					mq.Genre = ids[0]
				}
				if p, err := tm.TopMovies(ctx, mq); err == nil {
					for _, m := range p.Results {
						if m.PosterPath != "" {
							cards = append(cards, movieCard(m))
						}
					}
				}
			}
			if in.Kind != "movie" && (!hasGenre || ids[1] > 0) {
				sq := q
				if hasGenre {
					sq.Genre = ids[1]
				}
				if p, err := tm.TopShows(ctx, sq); err == nil {
					for _, sh := range p.Results {
						if sh.PosterPath != "" {
							cards = append(cards, showCard(sh))
						}
					}
				}
			}
			if len(cards) > 20 {
				cards = cards[:20]
			}
			return textResult(a.note(filter(cards))), nil
		})
	if err != nil {
		return nil, err
	}
	info, err := toolrunner.NewBetaToolFromJSONSchema("title_info", "A film's or show's details: running time, genres, rating, cast, director, story.",
		func(ctx context.Context, in askTitleIn) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			tm := a.s.TMDB()
			if tm == nil {
				return textResult("Not available."), nil
			}
			var facts map[string]any
			if in.Kind == "movie" {
				d, err := tm.GetMovieDetail(ctx, in.ID)
				if err != nil {
					return textResult("Couldn't find that title."), nil
				}
				a.note([]watchCard{movieCard(d.Movie)})
				facts = map[string]any{"title": d.Title, "year": d.Year(), "minutes": d.Runtime, "genres": genreNames(d.Genres), "rated": d.Certification(),
					"score": metadata.RoundRating(d.VoteAverage), "cast": castNames(d.Cast(8)), "directed_by": d.Directors(), "story": d.Overview}
			} else {
				d, err := tm.GetShowFull(ctx, in.ID)
				if err != nil {
					return textResult("Couldn't find that title."), nil
				}
				a.note([]watchCard{showCard(d.Show)})
				facts = map[string]any{"title": d.Name, "first_aired": d.FirstAirDate, "seasons": d.NumberOfSeasons, "genres": genreNames(d.Genres),
					"rated": d.ContentRating(), "score": metadata.RoundRating(d.VoteAverage), "cast": castNames(d.Cast(8)), "story": d.Overview}
			}
			js, _ := json.Marshal(facts)
			return textResult(string(js)), nil
		})
	if err != nil {
		return nil, err
	}
	mine, err := toolrunner.NewBetaToolFromJSONSchema("my_titles", "What this person is watching now and has saved to My List.",
		func(ctx context.Context, in askEmpty) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			cards := append(a.s.continueWatching(ctx, a.pid), a.s.myListCards(a.pid)...)
			return textResult(a.note(cards)), nil
		})
	if err != nil {
		return nil, err
	}
	show, err := toolrunner.NewBetaToolFromJSONSchema("show_titles", "Show your picks as posters under your reply.",
		func(ctx context.Context, in askShowIn) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			a.mu.Lock()
			defer a.mu.Unlock()
			var cards []watchCard
			for _, t := range in.Titles {
				if c, ok := a.seen[cardKey(t.Kind, t.ID)]; ok {
					cards = append(cards, c)
				}
			}
			if len(cards) == 0 {
				return textResult("None of those came from a tool: look them up first."), nil
			}
			a.ans.Heading, a.ans.Cards = truncate(in.Heading, 80), cards
			return textResult(fmt.Sprintf("Showing %d titles.", len(cards))), nil
		})
	if err != nil {
		return nil, err
	}
	play, err := toolrunner.NewBetaToolFromJSONSchema("play_title", "Start playing a title (one a tool returned).",
		func(ctx context.Context, in askPlayIn) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			a.mu.Lock()
			defer a.mu.Unlock()
			if _, ok := a.seen[cardKey(in.Kind, in.ID)]; !ok {
				return textResult("Look the title up first."), nil
			}
			a.ans.Play = &struct {
				Kind    string `json:"kind"`
				TMDBID  int    `json:"tmdbId"`
				Season  int    `json:"season,omitempty"`
				Episode int    `json:"episode,omitempty"`
			}{in.Kind, in.ID, in.Season, in.Episode}
			return textResult("Playing it now."), nil
		})
	if err != nil {
		return nil, err
	}
	open, err := toolrunner.NewBetaToolFromJSONSchema("open_title", "Open a title's page (one a tool returned).",
		func(ctx context.Context, in askTitleIn) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			a.mu.Lock()
			defer a.mu.Unlock()
			if _, ok := a.seen[cardKey(in.Kind, in.ID)]; !ok {
				return textResult("Look the title up first."), nil
			}
			a.ans.Open = &struct {
				Kind   string `json:"kind"`
				TMDBID int    `json:"tmdbId"`
			}{in.Kind, in.ID}
			return textResult("Opening it."), nil
		})
	if err != nil {
		return nil, err
	}
	return []anthropic.BetaTool{find, similar, search, browse, info, mine, show, play, open}, nil
}

// askFound is one title find_titles found, with the facts to check it by.
type askFound struct {
	card    watchCard
	minutes int
	seasons int
	genres  []string
	rated   string
}

// findOne looks up one title Claude has in mind: the search result that
// best matches its name, kind and year, with its running time, genres and
// rating (or seasons, for a show).
func (a *asker) findOne(ctx context.Context, tm *metadata.Client, t askFindTitle) *askFound {
	name := strings.TrimSpace(t.Title)
	if name == "" {
		return nil
	}
	res, err := tm.SearchMulti(ctx, truncate(name, 200))
	if err != nil {
		return nil
	}
	best, bestScore := -1, 0
	for i, m := range res {
		if (m.MediaType != "movie" && m.MediaType != "tv") || m.PosterPath == "" {
			continue
		}
		score := 0
		switch {
		case sameTitle(m.DisplayTitle(), name):
			score += 5
		case titleContains(m.DisplayTitle(), name):
			score += 2
		}
		if t.Kind != "" && m.MediaType == t.Kind {
			score += 2
		}
		if t.Year > 0 {
			switch d := m.Year() - t.Year; {
			case d == 0:
				score += 3
			case d >= -1 && d <= 1:
				score += 2
			}
		}
		if score > bestScore {
			best, bestScore = i, score
		}
	}
	// Nothing named like it (or of its kind and year): not found, rather
	// than a different title that merely came up in the search.
	if best < 0 || bestScore < 4 {
		return nil
	}
	m := res[best]
	f := &askFound{card: watchCard{Kind: m.MediaType, TMDBID: m.ID, Title: m.DisplayTitle(), Year: m.Year(), Overview: m.Overview,
		PosterURL: metadata.PosterURL(m.PosterPath), BackdropURL: metadata.BackdropURL(m.BackdropPath), Rating: metadata.RoundRating(m.VoteAverage)}}
	if m.MediaType == "movie" {
		if d, err := tm.GetMovieDetail(ctx, m.ID); err == nil {
			f.minutes, f.genres, f.rated = d.Runtime, genreNames(d.Genres), d.Certification()
		}
	} else if d, err := tm.GetShowFull(ctx, m.ID); err == nil {
		f.seasons, f.genres, f.rated = d.NumberOfSeasons, genreNames(d.Genres), d.ContentRating()
	}
	return f
}

// sameTitle compares two titles ignoring case, punctuation and a leading "the".
func sameTitle(a, b string) bool { return normTitle(a) == normTitle(b) }

// titleContains says whether one title holds the other ("Dune" and "Dune:
// Part One").
func titleContains(a, b string) bool {
	na, nb := normTitle(a), normTitle(b)
	return na != "" && nb != "" && (strings.Contains(na, nb) || strings.Contains(nb, na))
}

func normTitle(s string) string {
	var out strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r > 127 {
			out.WriteRune(r)
		}
	}
	return strings.TrimPrefix(out.String(), "the")
}

// testAnthropicKey asks Anthropic about Ask Cue's model with the key (no
// tokens spent), for Settings' Test button.
func testAnthropicKey(ctx context.Context, key string) connTestResult {
	client := anthropic.NewClient(option.WithAPIKey(key), option.WithMaxRetries(0))
	if _, err := client.Models.Get(ctx, askModelHaiku, anthropic.ModelGetParams{}); err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusUnauthorized || apiErr.StatusCode == http.StatusForbidden) {
			return connTestResult{Message: "Anthropic didn't accept that key."}
		}
		return connTestResult{Message: "Couldn't reach Anthropic: " + plainerror.Message(err)}
	}
	return connTestResult{OK: true, Message: "Anthropic accepted the key. Ask Cue is ready."}
}
