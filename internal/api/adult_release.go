package api

import (
	"regexp"
	"slices"

	"github.com/jhipsman/cueapp/internal/indexers"
)

// Adult videos are never taken for a film or show. A release named like
// one ("Anastasia.2020.XXX.1080p", a studio's scene name) can have the same
// title and year as a children's film, and the name match alone took it:
// Play, and the preview clip cut from it, showed the wrong video. Torrent
// sites also file them under XXX (category 6000 and below 7000).

var adultRelease = regexp.MustCompile(`(?i)(^|[^a-z0-9])(xxx|porn\w*|brazzers|bangbros|realitykings|reality kings|naughtyamerica|naughty america|blacked(raw)?|tushy(raw)?|vixen|onlyfans|mofos|fakehub|fake taxi|faketaxi|digitalplayground|digital playground|evilangel|evil angel|julesjordan|jules jordan|teamskeet|nubiles|metart|x-art|hegre|sexart|playboy ?plus|penthouse|hustler|private ?gold|21sextury|ddfnetwork|kink\.com|pornhub|xvideos|hentai|jav|uncensored|18\+)([^a-z0-9]|$)`)

// adultName says whether a release's name is an adult video's.
func adultName(release string) bool { return adultRelease.MatchString(release) }

// adultResult says whether a search result is an adult video, by its name
// or the site's category for it (XXX: 6000 to 6999).
func adultResult(r indexers.Result) bool {
	return adultName(r.Title) || slices.ContainsFunc(r.Categories, func(c int) bool { return c >= 6000 && c < 7000 })
}

// withoutAdult is the play candidates that aren't adult videos.
func withoutAdult(cands []playCandidate) []playCandidate {
	out := cands[:0:0]
	for _, c := range cands {
		if !adultName(c.Release) {
			out = append(out, c)
		}
	}
	return out
}
