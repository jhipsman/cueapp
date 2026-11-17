package api

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jhipsman/cueapp/internal/metadata"
)

// Quiet titles: horror, anime, adult titles and anything rated above PG-13
// (or not rated) are kept off what plays or shows by itself (Home's banner,
// the preview clips and the screensavers),
// so nobody walking past the TV gets a fright. They can still be found,
// browsed and played like any other title; only nothing pushes them.
//
// Adult goes beyond TMDB's adult flag (only for pornography) and NC-17:
// erotic films and ones TMDB tags for nudity or sex scenes count too, by
// their keywords, which need the title's details (list results don't carry
// them), so a list result can say a title is quiet but never that it isn't.

const (
	genreHorror    = 27 // TMDB's movie genre
	genreAnimation = 16 // movies and shows alike
)

// quietBy says whether a title with these facts is kept quiet: an adult
// title, a horror film, or Japanese animation (anime).
func quietBy(genres []int, language string, countries []string, adult bool, certification string) bool {
	if adult || certification == "NC-17" || certification == "X" {
		return true
	}
	if slices.Contains(genres, genreHorror) {
		return true
	}
	return slices.Contains(genres, genreAnimation) && (language == "ja" || slices.Contains(countries, "JP"))
}

// familyRating says whether a rating lets a title show by itself: up to
// PG-13 or TV-14. R, TV-MA, NC-17 and titles with no rating never play a
// clip or show on the banner or a screensaver, whatever their genre or tags
// say (those can be missing; a rating rarely is for anything mainstream).
func familyRating(cert string) bool {
	r := ratingRank[strings.ToUpper(strings.TrimSpace(cert))]
	return r > 0 && r <= ratingRank["PG-13"]
}

// adultWords are TMDB keywords for erotic films, nudity and sex scenes.
var adultWords = regexp.MustCompile(`(?i)\b(erotic\w*|softcore|sexploitation|\w*\s?nudity|nude\w*|full frontal|sex scenes?|explicit sex|sex tape|pornograph\w*|porn\w*|adult (film|movie|content)|striptease|stripper|bdsm|orgy|threesome|sexual fantasy|sensuality|lust)\b`)

// adultKeywords says whether any of a title's keywords is an adult one.
func adultKeywords(kws []metadata.Genre) bool {
	for _, k := range kws {
		if adultWords.MatchString(k.Name) {
			return true
		}
	}
	return false
}

func genreIDs(ids []int, full []metadata.Genre) []int {
	out := slices.Clone(ids)
	for _, g := range full {
		out = append(out, g.ID)
	}
	return out
}

// quietKnown remembers each title's answer ("movie-603" → true), from list
// results as cards are made and from TMDB's details otherwise.
var quietKnown sync.Map

func quietKey(kind string, id int) string { return fmt.Sprintf("%s-%d", kind, id) }

// noteMovieQuiet and noteShowQuiet answer from a list result, remembering
// only a yes (a no still needs the keywords: quietTitle).
func noteMovieQuiet(m metadata.Movie) bool {
	q := quietBy(genreIDs(m.GenreIDs, m.Genres), m.OriginalLanguage, nil, m.Adult, "")
	if q {
		quietKnown.Store(quietKey("movie", m.TMDBID), true)
	}
	return q
}

func noteShowQuiet(sh metadata.Show) bool {
	q := quietBy(genreIDs(sh.GenreIDs, sh.Genres), sh.OriginalLanguage, sh.OriginCountry, false, "")
	if q {
		quietKnown.Store(quietKey("tv", sh.TMDBID), true)
	}
	return q
}

// quietTitle says whether a title is kept quiet, asking TMDB for its details
// when it isn't known yet. When TMDB can't be asked it isn't (and the answer
// isn't remembered, so it's asked again next time).
func (s *Server) quietTitle(ctx context.Context, kind string, id int) bool {
	if v, ok := quietKnown.Load(quietKey(kind, id)); ok {
		return v.(bool)
	}
	tm := s.TMDB()
	if tm == nil {
		return false
	}
	var q bool
	switch kind {
	case "movie":
		d, err := tm.GetMovieDetail(ctx, id)
		if err != nil {
			return false
		}
		q = quietBy(genreIDs(d.GenreIDs, d.Genres), d.OriginalLanguage, nil, d.Adult, d.Certification()) || adultKeywords(d.Keywords.List) ||
			!familyRating(d.Certification())
	case "tv":
		d, err := tm.GetShowFull(ctx, id)
		if err != nil {
			return false
		}
		q = quietBy(genreIDs(d.GenreIDs, d.Genres), d.OriginalLanguage, d.OriginCountry, false, "") || adultKeywords(d.Keywords.List) ||
			!familyRating(d.ContentRating())
	default:
		return false
	}
	quietKnown.Store(quietKey(kind, id), q)
	return q
}

// markQuiet sets Quiet on every card of these rows, looking up (a few at a
// time, for at most a few seconds) the ones not known yet: titles from your
// history and your list come without their genres.
func (s *Server) markQuiet(ctx context.Context, rows []watchRow) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	type at struct{ row, item int }
	var mu sync.Mutex
	var wg sync.WaitGroup
	slots := make(chan struct{}, 6)
	for i := range rows {
		for j := range rows[i].Items {
			c := rows[i].Items[j]
			if v, ok := quietKnown.Load(quietKey(c.Kind, c.TMDBID)); ok {
				rows[i].Items[j].Quiet = v.(bool)
				continue
			}
			wg.Add(1)
			go func(p at, kind string, id int) {
				defer wg.Done()
				select {
				case slots <- struct{}{}:
				case <-ctx.Done():
					return
				}
				q := s.quietTitle(ctx, kind, id)
				<-slots
				mu.Lock()
				rows[p.row].Items[p.item].Quiet = q
				mu.Unlock()
			}(at{i, j}, c.Kind, c.TMDBID)
		}
	}
	wg.Wait()
}
