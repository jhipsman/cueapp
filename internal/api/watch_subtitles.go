package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rdborg/mediarium/internal/subtitles"
	"github.com/rdborg/mediarium/internal/watch"
)

// Subtitles in Watch: for whatever is playing, Cue finds an English subtitle
// on OpenSubtitles (the one made for the same release first), turns it into
// WebVTT (what browsers and the TV app read) and keeps it, so each is
// downloaded once. The player gets a signed address for it, which works
// without signing in, so the TV app's player can fetch it too.

const subsSealPrefix = "cue-subs:"

// subsMaxChoices is how many subtitles can be tried for one video (another
// one when the first is out of time).
const subsMaxChoices = 4

func (s *Server) subsDir() string { return filepath.Join(s.cfg.ConfigDir, "cache", "watch-subtitles") }

// GET /api/watch/subtitles/{kind}/{tmdbId}?season=&episode=&release=&choice=
// is where the player gets the subtitle: {"url", "label", "choices"}.
func (s *Server) handleWatchSubtitles(w http.ResponseWriter, r *http.Request) {
	kind, tmdbID, ok := watchKind(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	season, _ := strconv.Atoi(q.Get("season"))
	episode, _ := strconv.Atoi(q.Get("episode"))
	choice, _ := strconv.Atoi(q.Get("choice"))
	choice = max(0, min(choice, subsMaxChoices-1))
	if kind == watch.KindMovie {
		season, episode = 0, 0
	}
	client := s.Subtitles()
	if client == nil || !client.HasAPIKey() {
		writeError(w, http.StatusConflict, "Subtitles need an OpenSubtitles key: add one in Settings > Movie info.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()

	name := fmt.Sprintf("%s-%d-%d-%d-en-%d.vtt", kind, tmdbID, season, episode, choice)
	path := filepath.Join(s.subsDir(), name)
	choices := 0
	if _, err := os.Stat(path); err != nil {
		results, err := s.findWatchSubtitles(ctx, client, kind, tmdbID, season, episode, q.Get("release"))
		switch {
		case errors.Is(err, subtitles.ErrQuota):
			writeError(w, http.StatusTooManyRequests, "Today's subtitle downloads are used up. More tomorrow.")
			return
		case err != nil:
			slog.Info("watch: subtitles", "err", err)
			writeError(w, http.StatusBadGateway, "Couldn't reach OpenSubtitles. Try again in a moment.")
			return
		case len(results) == 0:
			writeError(w, http.StatusNotFound, "No English subtitles were found for this.")
			return
		}
		choices = min(len(results), subsMaxChoices)
		if choice >= len(results) {
			choice = len(results) - 1
			name = fmt.Sprintf("%s-%d-%d-%d-en-%d.vtt", kind, tmdbID, season, episode, choice)
			path = filepath.Join(s.subsDir(), name)
		}
		if _, err := os.Stat(path); err != nil {
			link, err := client.RequestDownload(ctx, results[choice].FileID)
			if errors.Is(err, subtitles.ErrQuota) {
				writeError(w, http.StatusTooManyRequests, "Today's subtitle downloads are used up. More tomorrow.")
				return
			}
			if err != nil {
				slog.Info("watch: subtitles: download", "err", err)
				writeError(w, http.StatusBadGateway, "Couldn't download the subtitle. Try again in a moment.")
				return
			}
			data, err := client.DownloadFile(ctx, link)
			if err != nil {
				writeError(w, http.StatusBadGateway, "Couldn't download the subtitle. Try again in a moment.")
				return
			}
			vtt, err := toWebVTT(data)
			if err != nil {
				writeError(w, http.StatusBadGateway, "That subtitle couldn't be read.")
				return
			}
			if err := os.MkdirAll(s.subsDir(), 0o755); err == nil {
				_ = os.WriteFile(path, vtt, 0o644)
			}
		}
		_ = os.WriteFile(filepath.Join(s.subsDir(), fmt.Sprintf("%s-%d-%d-%d-en.count", kind, tmdbID, season, episode)), []byte(strconv.Itoa(choices)), 0o644)
	} else if b, err := os.ReadFile(filepath.Join(s.subsDir(), fmt.Sprintf("%s-%d-%d-%d-en.count", kind, tmdbID, season, episode))); err == nil {
		choices, _ = strconv.Atoi(strings.TrimSpace(string(b)))
	}
	sealed, err := s.profileBox.Encrypt(fmt.Sprintf("%s%s|%d", subsSealPrefix, name, time.Now().Add(24*time.Hour).Unix()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't get the subtitle ready.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"url":     "/api/subs?t=" + url.QueryEscape(sealed),
		"label":   "English",
		"choice":  choice,
		"choices": max(choices, choice+1),
	})
}

// findWatchSubtitles is the English subtitles for a title, best first: one
// made for the release playing, then the most downloaded.
func (s *Server) findWatchSubtitles(ctx context.Context, client *subtitles.Client, kind string, tmdbID, season, episode int, release string) ([]subtitles.Result, error) {
	q := subtitles.Query{Language: "en"}
	if kind == watch.KindMovie {
		q.TMDBID, q.Type = tmdbID, "movie"
	} else {
		q.ParentTMDBID, q.Season, q.Episode, q.Type = tmdbID, season, episode, "episode"
	}
	results, err := client.Find(ctx, q)
	if err != nil {
		return nil, err
	}
	var en []subtitles.Result
	for _, r := range results {
		if strings.HasPrefix(strings.ToLower(r.Language), "en") && r.FileID != 0 {
			en = append(en, r)
		}
	}
	sort.SliceStable(en, func(i, j int) bool { return subtitles.Score(en[i], release) > subtitles.Score(en[j], release) })
	return en, nil
}

// GET /api/subs?t=<signed>[&shift=<seconds>]: a subtitle the player was
// given, as WebVTT. shift moves it earlier (for a video converted from a
// point further in, which starts its clock there).
func (s *Server) handleSubsFile(w http.ResponseWriter, r *http.Request) {
	plain, err := s.profileBox.Decrypt(r.URL.Query().Get("t"))
	if err != nil || !strings.HasPrefix(plain, subsSealPrefix) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	name, exp, _ := strings.Cut(strings.TrimPrefix(plain, subsSealPrefix), "|")
	until, _ := strconv.ParseInt(exp, 10, 64)
	if time.Now().Unix() > until || name != filepath.Base(name) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	data, err := os.ReadFile(filepath.Join(s.subsDir(), name))
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if shift, _ := strconv.ParseFloat(r.URL.Query().Get("shift"), 64); shift > 0 {
		data = shiftVTT(data, int64(shift*1000))
	}
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	// The TV app's player and the browser fetch it from the page's own
	// address; nothing else needs it.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_, _ = w.Write(data)
}

var (
	srtComma   = regexp.MustCompile(`(\d{2}:\d{2}:\d{2}),(\d{3})`)
	srtNumbers = regexp.MustCompile(`(?m)^\d+\s*\n(\d{2}:\d{2}:\d{2}[.,]\d{3}\s*-->)`)
	assTags    = regexp.MustCompile(`\{\\[^}]*\}`)
)

// toWebVTT turns an SRT (or WebVTT) subtitle into WebVTT: the timestamps'
// commas become dots, and old files in Latin-1 become UTF-8.
func toWebVTT(data []byte) ([]byte, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if !utf8.Valid(data) {
		runes := make([]rune, len(data))
		for i, b := range data {
			runes[i] = rune(b) // Latin-1
		}
		data = []byte(string(runes))
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if len(subtitles.Starts([]byte(text))) == 0 {
		return nil, subtitles.ErrNoCues
	}
	if !strings.HasPrefix(strings.TrimSpace(text), "WEBVTT") {
		text = srtComma.ReplaceAllString(text, "$1.$2")
		text = srtNumbers.ReplaceAllString(text, "$1")
		text = assTags.ReplaceAllString(text, "")
		text = "WEBVTT\n\n" + strings.TrimLeft(text, "\n")
	}
	return []byte(text), nil
}

// shiftVTT moves every line ms earlier, dropping the ones over by then.
func shiftVTT(data []byte, ms int64) []byte {
	blocks := strings.Split(string(data), "\n\n")
	var keep []string
	for _, b := range blocks {
		starts := subtitles.Starts([]byte(b))
		if len(starts) == 0 {
			keep = append(keep, b) // the header
			continue
		}
		m := vttTiming.FindStringSubmatch(b)
		if m == nil {
			continue
		}
		if end, ok := vttStamp(m[2]); ok && end <= ms {
			continue
		}
		keep = append(keep, b)
	}
	out, err := subtitles.Retime([]byte(strings.Join(keep, "\n\n")), 1, -ms)
	if err != nil {
		return data
	}
	return out
}

var vttTiming = regexp.MustCompile(`((?:\d+:)?\d{1,2}:\d{2}\.\d{3})\s*-->\s*((?:\d+:)?\d{1,2}:\d{2}\.\d{3})`)

func vttStamp(s string) (int64, bool) {
	main, frac, _ := strings.Cut(s, ".")
	parts := strings.Split(main, ":")
	var total int64
	for _, p := range parts {
		n, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			return 0, false
		}
		total = total*60 + n
	}
	f, _ := strconv.ParseInt(frac, 10, 64)
	return total*1000 + f, true
}
