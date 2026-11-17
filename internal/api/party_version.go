package api

import (
	"sort"
	"strings"
)

// partyVersionFirst puts the party's version first, for a player asking
// with party=CODE, so everyone in a watch party plays the same release (the
// same cut, so the same moment is at the same second everywhere). The first
// to ask chooses it, as the one every device plays most easily: Premiumize's
// converted copy (an MP4 every browser opens and can jump around in), then
// a version whose picture and sound a browser plays as they are, then the
// rest. Unknown codes change nothing.
func partyVersionFirst(code string, cands []playCandidate, converted bool) []playCandidate {
	parties.Lock()
	p := parties.byCode[strings.ToUpper(strings.TrimSpace(code))]
	parties.Unlock()
	if p == nil || len(cands) == 0 {
		return cands
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	out := append([]playCandidate(nil), cands...)
	if p.release != "" {
		sort.SliceStable(out, func(i, j int) bool { return out[i].Release == p.release && out[j].Release != p.release })
		if out[0].Release == p.release {
			return out
		}
		// That version isn't here any more: choose again.
	}
	ease := func(c playCandidate) int {
		switch {
		case converted && c.Hash != "":
			return 0
		case c.URL != "" && !cinemaAudio.MatchString(c.Release) && !hevcPicture.MatchString(c.Release) && !av1Picture.MatchString(c.Release):
			return 1
		case !hevcPicture.MatchString(c.Release) && !av1Picture.MatchString(c.Release):
			return 2
		}
		return 3
	}
	sort.SliceStable(out, func(i, j int) bool { return ease(out[i]) < ease(out[j]) })
	p.release = out[0].Release
	return out
}
