package api

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/jhipsman/cueapp/internal/metadata"
	"github.com/jhipsman/cueapp/internal/watch"
)

// Search as the streaming apps do it: not just titles named like the
// search, but what it means. Each kind of answer is its own row:
//
//   - a person ("tom hanks", "nolan"): what they're in or made;
//   - what the search describes ("funny 90s movies", "horror on netflix",
//     see search_intent.go);
//   - a topic ("zombies", "heist", "time travel"): titles tagged with it;
//   - titles named like it, as before.
//
// When nothing at all is found, a few spellings close to it are tried
// ("breakng bad"), and the answer says which one found something.

type watchSearchRow struct {
	Key    string             `json:"key"`   // "titles", "intent", "person", "topic"
	Title  string             `json:"title"` // the row's heading
	Items  []watchCard        `json:"items"`
	Person *watchSearchPerson `json:"person,omitempty"`
}

type watchSearchPerson struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Photo string `json:"photo,omitempty"`
}

type watchSearchAnswer struct {
	Query string `json:"query"`
	// Understood: how the search was read, when it described something
	// ("Comedy movies from the 1990s").
	Understood string `json:"understood,omitempty"`
	// DidYouMean: the spelling that found something, when the search as
	// typed found nothing.
	DidYouMean string           `json:"didYouMean,omitempty"`
	Sections   []watchSearchRow `json:"sections"`
}

const searchRowMax = 20

func (s *Server) smartSearch(ctx context.Context, q string) watchSearchAnswer {
	ans := watchSearchAnswer{Query: q, Sections: []watchSearchRow{}}
	in := readSearch(q)
	norm := searchNorm(q)

	var (
		wg                         sync.WaitGroup
		titles                     []watchCard
		intent, topic, personCards []watchCard
		person                     *metadata.PersonHit
		topicName                  string
		exactTitle, exactPerson    bool
	)
	wg.Add(3)
	go func() {
		defer wg.Done()
		titles, exactTitle = s.smartTitles(ctx, q, norm)
	}()
	go func() {
		defer wg.Done()
		if !in.Understood() {
			// A topic: the whole search, when it names one.
			topicName, topic = s.searchTopic(ctx, q)
			return
		}
		intent = s.searchIntentCards(ctx, in)
	}()
	go func() {
		defer wg.Done()
		person, personCards, exactPerson = s.searchPerson(ctx, q, norm)
	}()
	wg.Wait()

	added := map[string]bool{}
	add := func(sec watchSearchRow) {
		if len(sec.Items) > 0 && !added[sec.Key] {
			added[sec.Key] = true
			ans.Sections = append(ans.Sections, sec)
		}
	}
	personSec := watchSearchRow{Key: "person", Items: personCards}
	if person != nil {
		personSec.Person = &watchSearchPerson{ID: person.ID, Name: person.Name, Photo: metadata.PosterURL(person.ProfilePath)}
		if person.Department == "Acting" {
			personSec.Title = "With " + person.Name
		} else {
			personSec.Title = "From " + person.Name
		}
	}
	intentSec := watchSearchRow{Key: "intent", Title: in.Title(), Items: intent}
	titleSec := watchSearchRow{Key: "titles", Title: "Titles", Items: titles}
	topicSec := watchSearchRow{Key: "topic", Title: "About " + topicName, Items: topic}

	// The best answer first: a title or person named exactly so, then what
	// the search describes, then the rest.
	// A search that's only a description ("anime", "horror") wants that,
	// before titles that happen to be called it.
	if in.describes() && in.Leftover == "" {
		add(intentSec)
	}
	switch {
	case exactPerson && !exactTitle:
		add(personSec)
		add(titleSec)
	case exactTitle:
		add(titleSec)
		add(personSec)
	}
	add(intentSec)
	add(topicSec)
	if !exactPerson && !exactTitle {
		add(titleSec)
		add(personSec)
	}
	if in.Understood() && len(intent) > 0 {
		ans.Understood = in.Title()
	}

	// Nothing: a spelling close to it (a well-known title a letter or two
	// away, else the search a little shorter).
	if len(ans.Sections) == 0 {
		tries := spellings(q)
		if near := closestTitle(q, s.knownTitles(ctx)); near != "" {
			tries = append([]string{near}, tries...)
		}
		for _, v := range tries {
			if cards, _ := s.smartTitles(ctx, v, searchNorm(v)); len(cards) > 0 {
				ans.DidYouMean = v
				ans.Sections = append(ans.Sections, watchSearchRow{Key: "titles", Title: "Titles", Items: cards})
				break
			}
		}
	}
	return ans
}

// smartTitles are titles named like the search, ones named exactly so
// first; and whether there was one.
func (s *Server) smartTitles(ctx context.Context, q, norm string) ([]watchCard, bool) {
	results, err := s.TMDB().SearchMulti(ctx, q)
	if err != nil {
		return nil, false
	}
	out := []watchCard{}
	exact := false
	for _, m := range results {
		if m.PosterPath == "" {
			continue
		}
		c := watchCard{Kind: m.MediaType, TMDBID: m.ID, Title: m.DisplayTitle(), Year: m.Year(), Overview: m.Overview,
			PosterURL: metadata.PosterURL(m.PosterPath), BackdropURL: metadata.BackdropURL(m.BackdropPath), Rating: metadata.RoundRating(m.VoteAverage)}
		if searchNorm(c.Title) == norm {
			exact = true
		}
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return searchNorm(out[i].Title) == norm && searchNorm(out[j].Title) != norm
	})
	return out, exact
}

// searchIntentCards browse the catalogue for what the search describes.
func (s *Server) searchIntentCards(ctx context.Context, in searchIntent) []watchCard {
	var keyword string
	if in.Leftover != "" {
		if kw, err := s.TMDB().SearchKeywords(ctx, in.Leftover); err == nil && len(kw) > 0 {
			keyword = strconv.Itoa(bestKeyword(kw, in.Leftover).ID)
		}
	}
	if keyword == "" && !in.describes() {
		return nil // "movie 43": a title, not a topic
	}
	base := metadata.TopQuery{Keywords: keyword}
	switch {
	case in.Year > 0:
		base.Since, base.Until = strconv.Itoa(in.Year)+"-01-01", strconv.Itoa(in.Year)+"-12-31"
	case in.Decade > 0:
		base.Decade = in.Decade
	}
	if in.Service != "" {
		for _, sv := range topServices {
			if sv.Key == in.Service {
				base.Providers = sv.providers
			}
		}
	}
	if in.Best {
		base.Sort = "rating"
		base.MinVotes = 500
	}
	if in.New {
		base.Since = newSince(time.Now())
		base.MinVotes = 40 // new, but seen by someone
	}
	if in.Short {
		base.MaxRuntime = 100
	}
	base.Language = in.Language
	// Plain "comedy": the well known, not this week's unknowns.
	if !in.New && !in.Best && in.Year == 0 && in.Decade == 0 {
		base.MinVotes = 500
	}
	genres := func(tv bool) (string, bool) {
		var ids []string
		for _, g := range in.Genres {
			id := g.movie
			if tv {
				id = g.tv
			}
			if id == 0 {
				return "", false // no such genre for this kind
			}
			ids = append(ids, strconv.Itoa(id))
		}
		return strings.Join(ids, ","), true
	}
	var movies, shows []watchCard
	var wg sync.WaitGroup
	if in.Kind != watch.KindTV {
		if g, ok := genres(false); ok {
			q := base
			q.Genres = g
			wg.Add(1)
			go func() {
				defer wg.Done()
				if p, err := s.TMDB().TopMovies(ctx, q); err == nil {
					movies = movieCards(p.Results)
				}
			}()
		}
	}
	if in.Kind != watch.KindMovie {
		if g, ok := genres(true); ok {
			q := base
			q.Genres = g
			wg.Add(1)
			go func() {
				defer wg.Done()
				if p, err := s.TMDB().TopShows(ctx, q); err == nil {
					shows = showCards(p.Results)
				}
			}()
		}
	}
	wg.Wait()
	return interleave(movies, shows, searchRowMax)
}

// searchTopic is a topic named like the search and its titles ("zombie").
func (s *Server) searchTopic(ctx context.Context, q string) (string, []watchCard) {
	words := queryWords(q)
	if len(words) == 0 || len(words) > 3 {
		return "", nil
	}
	kw, err := s.TMDB().SearchKeywords(ctx, q)
	if err != nil || len(kw) == 0 {
		return "", nil
	}
	k := bestKeyword(kw, q)
	// Only a topic named like the search itself (not one that merely
	// contains a word of it).
	if !sameTopic(k.Name, q) {
		return "", nil
	}
	tq := metadata.TopQuery{Keywords: strconv.Itoa(k.ID), Sort: "votes"}
	var movies, shows []watchCard
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if p, err := s.TMDB().TopMovies(ctx, tq); err == nil {
			movies = movieCards(p.Results)
		}
	}()
	go func() {
		defer wg.Done()
		if p, err := s.TMDB().TopShows(ctx, tq); err == nil {
			shows = showCards(p.Results)
		}
	}()
	wg.Wait()
	return strings.ToLower(k.Name), interleave(movies, shows, searchRowMax)
}

// watchSearchPerson is a person named like the search and what they're known
// for, and whether the name is exactly the search.
func (s *Server) searchPerson(ctx context.Context, q, norm string) (*metadata.PersonHit, []watchCard, bool) {
	if len(queryWords(q)) > 4 {
		return nil, nil, false
	}
	hits, err := s.TMDB().SearchPeople(ctx, q)
	if err != nil || len(hits) == 0 {
		return nil, nil, false
	}
	h := hits[0]
	name := searchNorm(h.Name)
	exact := name == norm
	// A search that's a whole word of a well-known name ("nolan", "zendaya").
	partial := false
	for _, w := range strings.Fields(name) {
		if w == norm && len(norm) >= 4 {
			partial = true
		}
	}
	if !exact && !(partial && h.Popularity >= 8) {
		return nil, nil, false
	}
	p, err := s.TMDB().GetPerson(ctx, h.ID)
	if err != nil {
		return nil, nil, false
	}
	credits := p.Credits.Cast
	if h.Department != "Acting" {
		credits = nil
		for _, c := range p.Credits.Crew {
			if c.Department == h.Department {
				credits = append(credits, c)
			}
		}
	}
	sort.SliceStable(credits, func(i, j int) bool { return credits[i].VoteCount > credits[j].VoteCount })
	seen := map[string]bool{}
	var out []watchCard
	for _, c := range credits {
		key := c.MediaType + strconv.Itoa(c.ID)
		if seen[key] || c.PosterPath == "" {
			continue
		}
		// Talk shows and award nights they appeared on aren't what they're known for.
		if c.MediaType == watch.KindTV && hasGenreID(c.GenreIDs, 10767, 10763, 10764) {
			continue
		}
		seen[key] = true
		switch c.MediaType {
		case watch.KindMovie:
			out = append(out, movieCard(c.Movie()))
		case watch.KindTV:
			out = append(out, showCard(c.Show()))
		}
		if len(out) == searchRowMax {
			break
		}
	}
	return &h, out, exact
}

func hasGenreID(ids []int, any ...int) bool {
	for _, id := range ids {
		for _, a := range any {
			if id == a {
				return true
			}
		}
	}
	return false
}

// bestKeyword is the topic named most like q (TMDB lists "zombie" after
// "zombie apocalypse" at times).
func bestKeyword(kw []metadata.Keyword, q string) metadata.Keyword {
	for _, k := range kw {
		if sameTopic(k.Name, q) {
			return k
		}
	}
	return kw[0]
}

// interleave takes films and shows in turn, up to n.
func interleave(a, b []watchCard, n int) []watchCard {
	out := make([]watchCard, 0, n)
	for i := 0; len(out) < n && (i < len(a) || i < len(b)); i++ {
		if i < len(a) {
			out = append(out, a[i])
		}
		if i < len(b) && len(out) < n {
			out = append(out, b[i])
		}
	}
	return out
}

// searchNorm is a name without case, punctuation or extra spaces.
func searchNorm(s string) string {
	return strings.Join(strings.Fields(strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return ' '
	}, s)), " ")
}

// sameTopic is whether two names are one, plural or not ("zombies" and
// "zombie", "parties" and "party").
func sameTopic(a, b string) bool {
	forms := func(s string) []string {
		out := []string{s}
		if strings.HasSuffix(s, "ies") && len(s) > 4 {
			out = append(out, s[:len(s)-3]+"y")
		}
		if strings.HasSuffix(s, "es") && len(s) > 4 {
			out = append(out, s[:len(s)-2])
		}
		if strings.HasSuffix(s, "s") && !strings.HasSuffix(s, "ss") && len(s) > 3 {
			out = append(out, s[:len(s)-1])
		}
		return out
	}
	for _, x := range forms(searchNorm(a)) {
		for _, y := range forms(searchNorm(b)) {
			if x == y {
				return true
			}
		}
	}
	return false
}

// spellings are what a search might have meant when it found nothing:
// with doubled letters made single ("matrixx"), without its last letter,
// without its last word.
func spellings(q string) []string {
	var out []string
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v != "" && v != q && len(v) >= 3 {
			for _, have := range out {
				if have == v {
					return
				}
			}
			out = append(out, v)
		}
	}
	var b strings.Builder
	var last rune
	for _, r := range q {
		if r != last || !unicode.IsLetter(r) {
			b.WriteRune(r)
		}
		last = r
	}
	add(b.String())
	if len(q) > 4 {
		add(q[:len(q)-1])
	}
	if f := strings.Fields(q); len(f) > 1 {
		add(strings.Join(f[:len(f)-1], " "))
	}
	return out
}
