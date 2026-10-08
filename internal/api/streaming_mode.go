package api

import (
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/rdborg/mediarium/internal/parser"
	"github.com/rdborg/mediarium/internal/settings"
)

// Streaming only: Cue as a family streaming app. Watch is the front door; the
// library manager (downloads, Usenet, the torrent client, music, books, media
// servers) is hidden and its automatic searches rest. Nothing is deleted: the
// switch brings it all back. Until it is set, CUE_STREAMING_ONLY decides
// (on unless "0").

func (s *Server) streamingOnly() bool {
	switch v, _ := s.Settings.Get(settings.KeyStreamingOnly); v {
	case "0":
		return false
	case "1":
		return true
	}
	return s.cfg.StreamingOnly
}

// playbackMaxRes is the biggest resolution preferred for playing, in lines
// (1080, 720), or 0 for the best there is.
func (s *Server) playbackMaxRes() int {
	v, _ := s.Settings.Get(settings.KeyPlaybackMaxRes)
	switch v {
	case "1080":
		return 1080
	case "720":
		return 720
	}
	return 0
}

// resolutionLines reads a release name's resolution in lines (2160, 1080,
// 720, 480), or 0 when it doesn't say.
func resolutionLines(release string) int {
	switch r := strings.ToLower(parser.Parse(release).Resolution); {
	case strings.Contains(r, "2160"), strings.Contains(r, "4k"):
		return 2160
	case strings.Contains(r, "1080"):
		return 1080
	case strings.Contains(r, "720"):
		return 720
	case strings.Contains(r, "480"), strings.Contains(r, "576"):
		return 480
	}
	return 0
}

// preferResolution moves versions bigger than maxRes to the end, keeping
// the order within each group (the add-on's or the quality ranking).
func preferResolution(cands []playCandidate, maxRes int) []playCandidate {
	if maxRes == 0 {
		return cands
	}
	out := append([]playCandidate(nil), cands...)
	sort.SliceStable(out, func(i, j int) bool {
		return resolutionLines(out[i].Release) <= maxRes && resolutionLines(out[j].Release) > maxRes
	})
	return out
}

type streamingSettings struct {
	StreamingOnly bool   `json:"streamingOnly"`
	MaxResolution string `json:"maxResolution"` // "", "1080" or "720"
	EnglishOnly   bool   `json:"englishOnly"`   // play English versions only (play_language.go)
}

func (s *Server) streamingSettingsNow() streamingSettings {
	v, _ := s.Settings.Get(settings.KeyPlaybackMaxRes)
	return streamingSettings{StreamingOnly: s.streamingOnly(), MaxResolution: v, EnglishOnly: s.englishOnly()}
}

// GET /api/settings/streaming
func (s *Server) handleGetStreamingSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.streamingSettingsNow())
}

// streamingSetup is what Watch still needs, for the owner's "Finish setting
// up" card: movie info (a TMDB key) and something to play from (Premiumize
// or a stream add-on). Live TV is optional.
type streamingSetup struct {
	MovieInfo bool `json:"movieInfo"`
	Streams   bool `json:"streams"`
	LiveTV    bool `json:"liveTV"`
}

// GET /api/settings/streaming/setup
func (s *Server) handleStreamingSetup(w http.ResponseWriter, r *http.Request) {
	_, live := s.iptvAccount()
	writeJSON(w, http.StatusOK, streamingSetup{
		MovieInfo: s.TMDB().HasAPIKey(),
		Streams:   s.premiumizeClient() != nil || len(s.savedAddons()) > 0,
		LiveTV:    live,
	})
}

// PUT /api/settings/streaming {"streamingOnly"?, "maxResolution"?}
func (s *Server) handlePutStreamingSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		StreamingOnly *bool   `json:"streamingOnly"`
		MaxResolution *string `json:"maxResolution"`
		EnglishOnly   *bool   `json:"englishOnly"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "That request couldn't be read.")
		return
	}
	if req.MaxResolution != nil {
		switch *req.MaxResolution {
		case "", "1080", "720":
		default:
			writeError(w, http.StatusBadRequest, `The playback quality must be "", "1080" or "720".`)
			return
		}
		if err := s.Settings.Set(settings.KeyPlaybackMaxRes, *req.MaxResolution, false); err != nil {
			writeError(w, http.StatusInternalServerError, "The setting couldn't be saved.")
			return
		}
	}
	if req.StreamingOnly != nil {
		v := "1"
		if !*req.StreamingOnly {
			v = "0"
		}
		if err := s.Settings.Set(settings.KeyStreamingOnly, v, false); err != nil {
			writeError(w, http.StatusInternalServerError, "The setting couldn't be saved.")
			return
		}
	}
	if req.EnglishOnly != nil {
		v := "1"
		if !*req.EnglishOnly {
			v = "0"
		}
		if err := s.Settings.Set(settings.KeyPlaybackEnglishOnly, v, false); err != nil {
			writeError(w, http.StatusInternalServerError, "The setting couldn't be saved.")
			return
		}
	}
	writeJSON(w, http.StatusOK, s.streamingSettingsNow())
}

// cinemaAudio matches release names with sound web browsers can't play:
// Dolby Digital (AC-3, E-AC-3/DD+), DTS, TrueHD and Atmos.
var cinemaAudio = regexp.MustCompile(`(?i)(^|[^a-z0-9])(dts(-?hd|-?x|-?ma)?|truehd|atmos|ddp?(\+|5\.?1|7\.?1|2\.?0)?|e-?ac-?3|ac-?3|dolby[ .]?digital)([^a-z0-9]|$)`)

// browserAudioFirst puts versions a web browser can play with sound first,
// for Watch in a browser (the TV app plays every kind of sound). Versions
// Premiumize finds itself come with its own browser-ready stream, so only
// add-on links are judged, by their names.
//
// With Premiumize set up (converted), an add-on link whose torrent is known
// gets Premiumize's converted copy, which has browser sound, so it isn't
// judged either.
func browserAudioFirst(cands []playCandidate, converted bool) []playCandidate {
	silent := func(c playCandidate) bool {
		return c.URL != "" && !(converted && c.Hash != "") && cinemaAudio.MatchString(c.Release)
	}
	out := append([]playCandidate(nil), cands...)
	sort.SliceStable(out, func(i, j int) bool { return !silent(out[i]) && silent(out[j]) })
	return out
}
