package api

import (
	"regexp"
	"strings"

	"github.com/rdborg/mediarium/internal/parser"
	"github.com/rdborg/mediarium/internal/settings"
)

// Watch plays English versions only (Settings > Streaming, on unless
// switched off). A version is taken as English unless something says it
// isn't: its name tags only other languages (ITA, FRENCH, LATINO...), or the
// add-on marks it with only other countries' flags (Comet and Torrentio show
// a stream's languages as flags). Untagged releases, English ones and MULTI
// or DUAL ones (which hold English as well) all stay.

// englishOnly says whether Watch keeps to English versions.
func (s *Server) englishOnly() bool {
	v, _ := s.Settings.Get(settings.KeyPlaybackEnglishOnly)
	return v != "0"
}

// englishFlags are the flags add-ons use for English audio.
var englishFlags = map[string]bool{"GB": true, "US": true, "AU": true, "CA": true, "IE": true, "NZ": true}

// flagCountries reads the countries of the flag emoji in s (each flag is two
// regional indicator letters).
func flagCountries(s string) []string {
	var out []string
	runes := []rune(s)
	for i := 0; i+1 < len(runes); i++ {
		a, b := runes[i], runes[i+1]
		if a >= 0x1F1E6 && a <= 0x1F1FF && b >= 0x1F1E6 && b <= 0x1F1FF {
			out = append(out, string([]rune{'A' + (a - 0x1F1E6), 'A' + (b - 0x1F1E6)}))
			i++
		}
	}
	return out
}

var multiWord = regexp.MustCompile(`(?i)(^|[^a-z])(multi|dual)([^a-z]|$)`)

// notEnglish says a version is in other languages only, from its release
// name and any other text the add-on gave about it.
func notEnglish(release string, extra ...string) bool {
	r := parser.Parse(release)
	if r.Multi {
		return false
	}
	for _, l := range r.Languages {
		if l == parser.English {
			return false
		}
	}
	if len(r.Languages) > 0 {
		return true
	}
	text := strings.Join(extra, "\n")
	if multiWord.MatchString(text) {
		return false
	}
	flags := flagCountries(text)
	if len(flags) == 0 {
		return false
	}
	for _, f := range flags {
		if englishFlags[f] {
			return false
		}
	}
	return true
}

// keepEnglish drops the versions in other languages only. When that would
// leave nothing, it says so (other is true) and returns nothing.
func keepEnglish(cands []playCandidate) (out []playCandidate, other bool) {
	for _, c := range cands {
		if !c.NotEnglish {
			out = append(out, c)
		}
	}
	return out, len(out) == 0 && len(cands) > 0
}
