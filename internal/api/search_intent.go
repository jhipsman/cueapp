package api

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// What a search means beyond a title's name, the way streaming apps read
// it: "funny 90s movies", "horror on netflix", "best sci-fi shows",
// "new comedies", "short animated films". The words it knows (genres,
// decades and years, services, films or shows, best or new, short) become a
// browse of TMDB's catalogue; the words left over ("zombie", "heist",
// "time travel") are looked up as a topic. The title search runs as well,
// so a film called "Comedy" is still found.

type searchGenre struct {
	label     string // "Comedy"
	movie, tv int    // TMDB's genre id for films and for shows (0: none)
}

// genreLanguage: a genre that's also where it's made (anime is Japanese animation).
var genreLanguage = map[string]string{"Anime": "ja"}

// genreWords are the words people use for each genre. Longer phrases are
// matched first ("science fiction" before "fiction").
var genreWords = map[string]searchGenre{
	"action": {"Action", 28, 10759}, "action adventure": {"Action", 28, 10759},
	"adventure": {"Adventure", 12, 10759}, "adventures": {"Adventure", 12, 10759},
	"animated": {"Animated", 16, 16}, "animation": {"Animated", 16, 16}, "cartoon": {"Animated", 16, 16}, "cartoons": {"Animated", 16, 16},
	"anime":  {"Anime", 16, 16},
	"comedy": {"Comedy", 35, 35}, "comedies": {"Comedy", 35, 35}, "funny": {"Comedy", 35, 35}, "sitcom": {"Comedy", 0, 35}, "sitcoms": {"Comedy", 0, 35},
	"crime":       {"Crime", 80, 80},
	"documentary": {"Documentary", 99, 99}, "documentaries": {"Documentary", 99, 99}, "doc": {"Documentary", 99, 99}, "docs": {"Documentary", 99, 99}, "docuseries": {"Documentary", 0, 99},
	"drama": {"Drama", 18, 18}, "dramas": {"Drama", 18, 18},
	"family": {"Family", 10751, 10751}, "kids": {"Kids", 10751, 10762}, "children": {"Kids", 10751, 10762}, "childrens": {"Kids", 10751, 10762},
	"fantasy": {"Fantasy", 14, 10765},
	"history": {"History", 36, 0}, "historical": {"History", 36, 0},
	"horror": {"Horror", 27, 0}, "scary": {"Horror", 27, 0},
	"musical": {"Musical", 10402, 0}, "musicals": {"Musical", 10402, 0},
	"mystery": {"Mystery", 9648, 9648}, "mysteries": {"Mystery", 9648, 9648}, "whodunit": {"Mystery", 9648, 9648},
	"romance": {"Romance", 10749, 0}, "romantic": {"Romance", 10749, 0},
	"sci-fi": {"Sci-fi", 878, 10765}, "scifi": {"Sci-fi", 878, 10765}, "sci fi": {"Sci-fi", 878, 10765}, "science fiction": {"Sci-fi", 878, 10765},
	"thriller": {"Thriller", 53, 0}, "thrillers": {"Thriller", 53, 0}, "suspense": {"Thriller", 53, 0},
	"war":     {"War", 10752, 10768},
	"western": {"Western", 37, 37}, "westerns": {"Western", 37, 37},
	"reality":    {"Reality", 0, 10764},
	"true crime": {"True crime", 99, 99}, // a documentary, about crime (see read)
	"romcom":     {"Romantic comedy", 10749, 35}, "rom com": {"Romantic comedy", 10749, 35}, "romantic comedy": {"Romantic comedy", 10749, 35}, "romantic comedies": {"Romantic comedy", 10749, 35}, "romcoms": {"Romantic comedy", 10749, 35},
}

// The genres a phrase stands for beyond its own (a romantic comedy is a
// comedy too; true crime is crime).
var genreAlso = map[string]searchGenre{
	"Romantic comedy": {"", 35, 35},
	"True crime":      {"", 80, 80},
}

var searchServiceWords = map[string]string{
	"netflix": "netflix", "prime": "prime", "prime video": "prime", "amazon": "prime", "amazon prime": "prime",
	"disney": "disney", "disney+": "disney", "disney plus": "disney", "hulu": "hulu", "max": "max", "hbo": "max", "hbo max": "max",
	"apple": "appletv", "apple tv": "appletv", "apple tv+": "appletv", "peacock": "peacock", "paramount": "paramount", "paramount+": "paramount",
	"paramount plus": "paramount",
}

var (
	searchMovieWords = map[string]bool{"movie": true, "movies": true, "film": true, "films": true}
	searchShowWords  = map[string]bool{"show": true, "shows": true, "series": true, "tv": true, "tv shows": true, "tv show": true, "tv series": true}
	searchBestWords  = map[string]bool{"best": true, "top": true, "top rated": true, "highest rated": true, "greatest": true, "acclaimed": true, "classic": true, "classics": true, "award winning": true}
	searchNewWords   = map[string]bool{"new": true, "newest": true, "latest": true, "recent": true, "new releases": true, "this year": true}
	searchShortWords = map[string]bool{"short": true, "quick": true, "under 90 minutes": true, "under 90 mins": true}
	// Words that say nothing about what's wanted.
	searchFiller = map[string]bool{
		"a": true, "an": true, "the": true, "some": true, "good": true, "great": true, "with": true, "about": true, "from": true, "of": true,
		"to": true, "for": true, "in": true, "on": true, "me": true, "something": true, "i": true, "want": true, "watch": true, "like": true,
		"stuff": true, "and": true, "or": true, "that": true, "are": true, "is": true, "really": true, "very": true, "find": true, "show me": true,
		"popular": true, "trending": true, "films": false,
	}
	searchDecade = regexp.MustCompile(`^(?:(19|20)?(\d)0)'?s$`)
	searchYear   = regexp.MustCompile(`^(19[2-9]\d|20[0-3]\d)$`)
)

// searchIntent is what a search asked for, beyond a title's name.
type searchIntent struct {
	Kind     string        // "movie", "tv" or "" (both)
	Genres   []searchGenre // all must apply
	Decade   int           // 1990 for "90s"
	Year     int
	Service  string // a topServices key
	Best     bool
	New      bool
	Short    bool
	Leftover string // the words it didn't know: a topic to look up
	Language string // made in this language
}

// settle picks films or shows when a genre has only one of them (TMDB
// has no horror shows, nor reality films).
func (in *searchIntent) settle() {
	if in.Kind != "" {
		return
	}
	for _, g := range in.Genres {
		switch {
		case g.tv == 0:
			in.Kind = "movie"
		case g.movie == 0:
			in.Kind = "tv"
		}
	}
}

// Understood is whether the search said anything beyond a name.
func (in searchIntent) Understood() bool {
	return in.describes() || (in.Kind != "" && in.Leftover != "") // "zombie movies": a topic, as films
}

// describes is whether it named a genre, a time, a service or an order.
func (in searchIntent) describes() bool {
	return len(in.Genres) > 0 || in.Decade > 0 || in.Year > 0 || in.Service != "" || in.Best || in.New || in.Short
}

// readSearch reads a search for what it asks for.
func readSearch(q string) searchIntent {
	var in searchIntent
	words := queryWords(q)
	var left []string
	for i := 0; i < len(words); {
		// The longest known phrase (up to three words) at this word.
		took := 0
		for n := min(3, len(words)-i); n >= 1 && took == 0; n-- {
			p := strings.Join(words[i:i+n], " ")
			switch {
			case genreWords[p].label != "":
				g := genreWords[p]
				in.Genres = appendGenre(in.Genres, g)
				if lang := genreLanguage[g.label]; lang != "" {
					in.Language = lang
				}
				if also, ok := genreAlso[g.label]; ok {
					in.Genres = appendGenre(in.Genres, also)
				}
				if p == "sitcom" || p == "sitcoms" || p == "docuseries" {
					in.Kind = "tv"
				}
			case searchServiceWords[p] != "":
				in.Service = searchServiceWords[p]
			case searchMovieWords[p]:
				in.Kind = "movie"
			case searchShowWords[p]:
				in.Kind = "tv"
			case searchBestWords[p]:
				in.Best = true
			case searchNewWords[p]:
				in.New = true
			case searchShortWords[p]:
				in.Short = true
			case n == 1 && searchDecade.MatchString(p):
				m := searchDecade.FindStringSubmatch(p)
				d, _ := strconv.Atoi(m[2])
				century := 1900
				if m[1] == "20" || (m[1] == "" && d <= 2) {
					century = 2000
				}
				in.Decade = century + d*10
			case n == 1 && searchYear.MatchString(p):
				in.Year, _ = strconv.Atoi(p)
			case n == 1 && searchFiller[p]:
			default:
				continue
			}
			took = n
		}
		if took == 0 {
			left = append(left, words[i])
			took = 1
		}
		i += took
	}
	in.Leftover = strings.Join(left, " ")
	in.settle()
	return in
}

func appendGenre(list []searchGenre, g searchGenre) []searchGenre {
	for _, have := range list {
		if have.movie == g.movie && have.tv == g.tv {
			return list
		}
	}
	return append(list, g)
}

// queryWords are a search's words, lowercased, without punctuation (but
// keeping "sci-fi", "disney+" and "90's").
func queryWords(q string) []string {
	q = strings.ToLower(q)
	q = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '+' || r == '\'' {
			return r
		}
		return ' '
	}, q)
	return strings.Fields(q)
}

// Title is how the search was understood, as a row's heading: "Comedy
// movies from the 1990s", "Best sci-fi shows on Netflix".
func (in searchIntent) Title() string {
	var b strings.Builder
	if in.Best {
		b.WriteString("Best ")
	} else if in.New {
		b.WriteString("New ")
	} else if in.Short {
		b.WriteString("Short ")
	}
	var labels []string
	for _, g := range in.Genres {
		if g.label != "" {
			labels = append(labels, g.label)
		}
	}
	genre := strings.Join(labels, " ")
	what := map[string]string{"movie": "movies", "tv": "shows", "": "movies and shows"}[in.Kind]
	if genre != "" {
		if b.Len() > 0 {
			genre = strings.ToLower(genre)
		}
		b.WriteString(genre + " " + what)
	} else if b.Len() > 0 {
		b.WriteString(what)
	} else {
		b.WriteString(strings.ToUpper(what[:1]) + what[1:])
	}
	if in.Leftover != "" {
		b.WriteString(" about " + in.Leftover)
	}
	switch {
	case in.Year > 0:
		fmt.Fprintf(&b, " from %d", in.Year)
	case in.Decade > 0:
		fmt.Fprintf(&b, " from the %ds", in.Decade)
	}
	if in.Service != "" {
		for _, s := range topServices {
			if s.Key == in.Service {
				b.WriteString(" on " + s.Name)
			}
		}
	}
	return b.String()
}

// newSince is the first day "new" covers: the last year.
func newSince(now time.Time) string { return now.AddDate(-1, 0, 0).Format("2006-01-02") }
