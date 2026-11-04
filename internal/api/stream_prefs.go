package api

import (
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/jhipsman/cueapp/internal/watch"
)

// Picking a version by the profile's streaming preferences (as in apps like
// Nuvio): the picture to aim for, how big a file may be, and what to prefer
// or avoid. Add-ons like Comet list the biggest file first; at the size
// streaming services use, 4K looks as sharp, starts sooner and doesn't
// stutter on a home connection.

// releaseTraits is what a version's name says about it.
type releaseTraits struct {
	res        int // 2160, 1080, 720, 480; 0 when it doesn't say
	remux      bool
	hdr        bool
	dv         bool
	efficient  bool // HEVC or AV1
	heavySound bool // TrueHD, DTS-HD, DTS:X, lossless
	lightSound bool // Dolby Digital Plus, Atmos over DD+, AC3
}

var (
	reRemux     = regexp.MustCompile(`(?i)\bremux\b`)
	reHDR       = regexp.MustCompile(`(?i)\b(hdr|hdr10|hdr10\+|hdr10plus|dv|dovi|dolby[. ]?vision)\b`)
	reDV        = regexp.MustCompile(`(?i)\b(dv|dovi|dolby[. ]?vision)\b`)
	reEfficient = regexp.MustCompile(`(?i)\b(x265|h[. ]?265|hevc|av1)\b`)
	reHeavy     = regexp.MustCompile(`(?i)\b(truehd|dts[-. ]?hd|dts[-. ]?x|dts[-. ]?ma|flac|lpcm|pcm)\b`)
	reLight     = regexp.MustCompile(`(?i)\b(ddp|dd\+|eac3|e-ac-3|ddp?5[. ]1|ddp?7[. ]1|ac3|dd5[. ]1|atmos)\b`)
)

func readTraits(name string) releaseTraits {
	// Separators become spaces so \b works on "Movie.2160p.WEB-DL.DDP5.1".
	n := strings.NewReplacer("_", " ").Replace(name)
	return releaseTraits{
		res:        resolutionLines(name),
		remux:      reRemux.MatchString(n),
		hdr:        reHDR.MatchString(n),
		dv:         reDV.MatchString(n),
		efficient:  reEfficient.MatchString(n),
		heavySound: reHeavy.MatchString(n),
		lightSound: reLight.MatchString(n),
	}
}

func prefLines(q string) int {
	switch q {
	case "1080p":
		return 1080
	case "720p":
		return 720
	}
	return 2160
}

// mbps is a version's average bitrate in megabits a second, over secs.
func mbps(size int64, secs float64) float64 {
	if size <= 0 || secs <= 0 {
		return 0
	}
	return float64(size) * 8 / secs / 1e6
}

// sweetSpot is the bitrate range streaming services use for a picture
// (Mbps): sharp, and steady on a home connection.
func sweetSpot(lines int) (lo, hi float64) {
	switch {
	case lines >= 2160:
		return 12, 32
	case lines >= 1080:
		return 4, 14
	default:
		return 2, 7
	}
}

// prefScore says how well a version fits the preferences: higher is better.
func prefScore(c playCandidate, p watch.StreamPrefs, secs float64) int {
	if c.YouTube != "" || c.Direct {
		return -100000 // the Internet Archive and YouTube stay last, as found
	}
	t := readTraits(c.Release)
	want := prefLines(p.Quality)
	res := t.res
	if res == 0 {
		res = 1080 // most names that don't say are HD
	}
	score := 0
	switch {
	case res == want:
		score += 1000
	case res < want:
		// The nearer the better: 1080p before 720p when 4K is missing.
		score += 1000 - 300*steps(res, want)
	default:
		// Better than wanted: only when nothing at it is found.
		score += 600 - 100*steps(want, res)
	}

	rate := mbps(c.Size, secs)
	switch p.Size {
	case "any":
	case "smart":
		if rate > 0 {
			lo, hi := sweetSpot(res)
			switch {
			case rate >= lo && rate <= hi:
				score += 250
			case rate > hi && rate <= hi*1.5:
				score += 60
			case rate > hi*1.5:
				score -= 250
			case rate < lo*0.5:
				score -= 200 // suspiciously small for the picture it claims
			default:
				score += 100
			}
		}
	default:
		// A size limit in GB for a two-hour film, as a bitrate.
		gb := map[string]float64{"10": 10, "20": 20, "30": 30, "50": 50}[p.Size]
		if limit := gb * 8 * 1000 / 7200; rate > limit && limit > 0 {
			score -= 900
		}
	}

	if t.remux && p.Remux == "avoid" {
		score -= 400
	}
	if res >= 2160 && t.hdr && p.HDR == "prefer" {
		score += 100
	}
	if t.dv {
		switch p.DolbyVision {
		case "prefer":
			score += 80
		case "avoid":
			score -= 350
		}
	}
	if t.efficient && p.Efficient {
		score += 60
	}
	if p.LightSound {
		if t.heavySound {
			score -= 80
		} else if t.lightSound {
			score += 40
		}
	}
	return score
}

func steps(lower, higher int) int {
	order := []int{480, 720, 1080, 2160}
	return max(slices.Index(order, higher)-slices.Index(order, lower), 1)
}

// rankByPrefs orders the versions found by p, best first, keeping the
// add-on's (or the search's) order between equals.
func rankByPrefs(cands []playCandidate, p watch.StreamPrefs, secs float64) []playCandidate {
	type scored struct {
		c playCandidate
		s int
	}
	list := make([]scored, len(cands))
	for i, c := range cands {
		list[i] = scored{c, prefScore(c, p, secs)}
	}
	slices.SortStableFunc(list, func(a, b scored) int { return b.s - a.s })
	out := make([]playCandidate, len(list))
	for i := range list {
		out[i] = list[i].c
	}
	return out
}

// targetSeconds is about how long the title runs: a film two hours, an
// episode 45 minutes (close enough to tell a 4K web file from a Remux).
func targetSeconds(t playTarget) float64 {
	if t.series != nil || t.episode > 0 {
		return 45 * 60
	}
	return 120 * 60
}

// profileStreamPrefs is the preferences of the profile a request is for,
// or Cue's defaults.
func (s *Server) profileStreamPrefs(r *http.Request) watch.StreamPrefs {
	if p, ok := s.activeProfile(r); ok {
		return p.StreamPrefs
	}
	return watch.DefaultStreamPrefs
}

// GET /api/profile/streaming: this profile's streaming preferences.
func (s *Server) handleGetStreamPrefs(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.watchProfile(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.profileStreamPrefs(r))
}

// PUT /api/profile/streaming: change them. Anyone can change their own.
func (s *Server) handlePutStreamPrefs(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.watchProfile(w, r); !ok {
		return
	}
	p, _ := s.activeProfile(r)
	var prefs watch.StreamPrefs
	if err := decodeJSON(r, &prefs); err != nil {
		writeError(w, http.StatusBadRequest, "That request couldn't be read.")
		return
	}
	saved, err := s.WatchRepo.SetStreamPrefs(p.UserID, p.ID, prefs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, saved)
}
