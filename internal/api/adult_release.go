package api

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

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

// titleOf is a play target's title: kind, TMDB id, season and episode.
func titleOf(t playTarget) (string, int, int, int) {
	if t.movie != nil {
		return "movie", t.movie.TMDBID, 0, 0
	}
	if t.series != nil {
		return "tv", t.series.TMDBID, t.season, t.episode
	}
	return "", 0, 0, 0
}

// withoutWrong is the candidates without those marked the wrong video for
// this title (Wrong video in the player).
func (s *Server) withoutWrong(cands []playCandidate, t playTarget) []playCandidate {
	kind, id, season, episode := titleOf(t)
	if kind == "" || s.WatchRepo == nil {
		return cands
	}
	wrong, err := s.WatchRepo.WrongReleases(kind, id, season, episode)
	if err != nil || len(wrong) == 0 {
		return cands
	}
	out := cands[:0:0]
	for _, c := range cands {
		if !wrong[c.Release] {
			out = append(out, c)
		}
	}
	return out
}

// POST /api/play/wrong {kind, tmdbId, season, episode, release}: this
// version is the wrong video for the title (another film with the same name,
// a mislabelled file). It's never offered for it again, for anyone, and the
// title's preview clip (likely cut from it) is thrown away.
func (s *Server) handleWrongVersion(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Kind    string `json:"kind"`
		TMDBID  int    `json:"tmdbId"`
		Season  int    `json:"season"`
		Episode int    `json:"episode"`
		Release string `json:"release"`
	}
	if err := decodeJSON(r, &req); err != nil || (req.Kind != "movie" && req.Kind != "tv") || req.TMDBID <= 0 || strings.TrimSpace(req.Release) == "" || len(req.Release) > 500 {
		writeError(w, http.StatusBadRequest, "That isn't a version of a title.")
		return
	}
	if req.Kind == "movie" {
		req.Season, req.Episode = 0, 0
	}
	if err := s.WatchRepo.MarkWrong(req.Kind, req.TMDBID, req.Season, req.Episode, req.Release); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't save that.")
		return
	}
	want := previewWant{kind: req.Kind, tmdbID: req.TMDBID, season: req.Season, episode: req.Episode}
	if req.Kind == "tv" && want.episode < 1 {
		want.season, want.episode = 1, 1
	}
	_ = os.Remove(filepath.Join(s.previewDir(), want.key()+".mp4"))
	// The film's own clip too, when it was an episode's show page (cut from episode 1).
	if req.Kind == "tv" {
		_ = os.Remove(filepath.Join(s.previewDir(), previewWant{kind: "tv", tmdbID: req.TMDBID, season: 1, episode: 1}.key()+".mp4"))
	}
	w.WriteHeader(http.StatusNoContent)
}

// previewWantOf is a play target as a title (for its expected length).
func previewWantOf(t playTarget) previewWant {
	kind, id, season, episode := titleOf(t)
	return previewWant{kind: kind, tmdbID: id, season: season, episode: episode}
}
