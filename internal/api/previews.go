package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Previews, made the way the streaming services make theirs: a short,
// silent clip of the title itself (about 22 seconds from early on, 720p
// H.264, a few megabytes), cut once on this server from the same version
// Play would use and kept in a cache. Pages then play it in an ordinary
// video element: it starts at once, and nothing but the picture shows (no
// YouTube player, no buttons). Home asks for its banner's and the first of
// Continue Watching's ahead of time, so they're ready before anyone rests
// on them.

const (
	previewSeconds  = 22
	previewMaxFiles = 400
	previewMaxBytes = 1500 << 20
	previewRetry    = 30 * time.Minute // after a failed try, how long before another
)

var (
	previewName  = regexp.MustCompile(`^(movie-\d+|tv-\d+-\d+-\d+)\.mp4$`)
	previewSlots = make(chan struct{}, 1) // one clip made at a time: it's ffmpeg work
	previewState = struct {
		sync.Mutex
		making map[string]bool
		failed map[string]time.Time
	}{making: map[string]bool{}, failed: map[string]time.Time{}}
)

type previewWant struct {
	kind                    string
	tmdbID, season, episode int
}

func (p previewWant) key() string {
	if p.kind == "movie" {
		return fmt.Sprintf("movie-%d", p.tmdbID)
	}
	return fmt.Sprintf("tv-%d-%d-%d", p.tmdbID, p.season, p.episode)
}

func (s *Server) previewDir() string { return filepath.Join(s.cfg.ConfigDir, "cache", "previews") }

// GET /api/watch/preview/{kind}/{tmdbId}?season=&episode=[&ready=1]: the
// clip's address when it's made ({"url"}); otherwise it's started and the
// answer is {"pending":true} (ask again shortly), or {"none":true} when it
// couldn't be made lately. With ready=1 nothing is started (a card the
// mouse passes over only plays a clip that's already there).
func (s *Server) handleWatchPreview(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	id, err := strconv.Atoi(r.PathValue("tmdbId"))
	if (kind != "movie" && kind != "tv") || err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "That isn't a title Cue can preview.")
		return
	}
	want := previewWant{kind: kind, tmdbID: id}
	if kind == "tv" {
		want.season, _ = strconv.Atoi(r.URL.Query().Get("season"))
		want.episode, _ = strconv.Atoi(r.URL.Query().Get("episode"))
		if want.season < 0 {
			want.season = 0
		}
		if want.episode < 1 {
			want.season, want.episode = 1, 1
		}
	}
	file := filepath.Join(s.previewDir(), want.key()+".mp4")
	if fi, err := os.Stat(file); err == nil && fi.Size() > 0 {
		now := time.Now()
		_ = os.Chtimes(file, now, now) // recently used: kept longest
		writeJSON(w, http.StatusOK, map[string]any{"url": "/api/watch/preview-file/" + want.key() + ".mp4"})
		return
	}
	if !canConvert() {
		writeJSON(w, http.StatusOK, map[string]any{"none": true})
		return
	}
	previewState.Lock()
	failedAt, failed := previewState.failed[want.key()]
	making := previewState.making[want.key()]
	previewState.Unlock()
	if failed && time.Since(failedAt) < previewRetry {
		writeJSON(w, http.StatusOK, map[string]any{"none": true})
		return
	}
	if r.URL.Query().Get("ready") == "1" {
		writeJSON(w, http.StatusOK, map[string]any{"none": !making, "pending": making})
		return
	}
	s.startPreview(r, want)
	writeJSON(w, http.StatusAccepted, map[string]any{"pending": true})
}

// GET /api/watch/preview-file/{name}: a made clip.
func (s *Server) handleWatchPreviewFile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !previewName.MatchString(name) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("Content-Type", "video/mp4")
	http.ServeFile(w, r, filepath.Join(s.previewDir(), name))
}

// startPreview makes a clip in the background (once at a time per title),
// as the profile asking would play it.
func (s *Server) startPreview(r *http.Request, want previewWant) {
	previewState.Lock()
	if previewState.making[want.key()] {
		previewState.Unlock()
		return
	}
	previewState.making[want.key()] = true
	previewState.Unlock()
	// Who's asking (account and profile) stays with the work; the request
	// itself may be long finished by then.
	ctx := context.WithoutCancel(r.Context())
	header := r.Header.Clone()
	go func() {
		defer func() {
			previewState.Lock()
			delete(previewState.making, want.key())
			previewState.Unlock()
		}()
		previewSlots <- struct{}{}
		defer func() { <-previewSlots }()
		ctx, cancel := context.WithTimeout(ctx, 4*time.Minute)
		defer cancel()
		if err := s.makePreview(ctx, header, want); err != nil {
			slog.Info("preview: couldn't make one", "title", want.key(), "err", err)
			previewState.Lock()
			previewState.failed[want.key()] = time.Now()
			previewState.Unlock()
		}
	}()
}

// previewSource is the address Play would use for the title: the same
// lookup, asked as a web browser would (Premiumize's converted copy first,
// which is lighter to cut from).
func (s *Server) previewSource(ctx context.Context, header http.Header, want previewWant) (string, error) {
	req := httptest.NewRequest(http.MethodGet, "/api/play/tmdb?client=web", nil).WithContext(ctx)
	req.Header = header.Clone()
	rec := httptest.NewRecorder()
	req.SetPathValue("tmdbId", strconv.Itoa(want.tmdbID))
	if want.kind == "movie" {
		s.handlePlayTMDBMovie(rec, req)
	} else {
		req.SetPathValue("season", strconv.Itoa(want.season))
		req.SetPathValue("episode", strconv.Itoa(want.episode))
		s.handlePlayTMDBEpisode(rec, req)
	}
	if rec.Code != http.StatusOK {
		return "", fmt.Errorf("no version to play (%d)", rec.Code)
	}
	var a struct {
		URL       string `json:"url"`
		StreamURL string `json:"streamUrl"`
		YouTube   string `json:"youtube"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		return "", err
	}
	if a.YouTube != "" {
		return "", fmt.Errorf("only on YouTube")
	}
	if a.StreamURL != "" {
		return a.StreamURL, nil
	}
	if a.URL == "" {
		return "", fmt.Errorf("no address")
	}
	return a.URL, nil
}

func (s *Server) makePreview(ctx context.Context, header http.Header, want previewWant) error {
	link, err := s.previewSource(ctx, header, want)
	if err != nil {
		return err
	}
	return cutPreview(ctx, link, want, s.previewDir())
}

// cutPreview cuts the clip for want from link into dir.
func cutPreview(ctx context.Context, link string, want previewWant, dir string) error {
	info, err := probeFile(ctx, link)
	if err != nil {
		return err
	}
	// Early on, past any opening logos, too early to spoil much: an eighth
	// of the way in for a film, a sixth for an episode.
	at := 90.0
	if info.Duration > 0 {
		frac := 0.125
		if want.kind == "tv" {
			frac = 1.0 / 6
		}
		at = max(30, min(info.Duration*frac, info.Duration-previewSeconds-10))
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	final := filepath.Join(dir, want.key()+".mp4")
	tmp := final + ".part"
	ffmpeg, _ := ffmpegOnce()
	args := []string{"-v", "error", "-y"}
	if isWebLink(link) {
		args = append(args, "-user_agent", convertUA)
	}
	args = append(args,
		"-ss", strconv.FormatFloat(at, 'f', 1, 64), "-i", link,
		"-t", strconv.Itoa(previewSeconds), "-map", "0:v:0", "-an", "-sn",
		"-vf", "scale=-2:'min(720,ih)',fps=30,format=yuv420p",
		"-c:v", "libx264", "-preset", "superfast", "-crf", "27", "-profile:v", "high",
		"-movflags", "+faststart", "-f", "mp4", tmp)
	cmd := exec.CommandContext(ctx, ffmpeg, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("ffmpeg: %v: %s", err, redactURLText(firstLine(string(out)), link))
	}
	if err := os.Rename(tmp, final); err != nil {
		return err
	}
	prunePreviews(dir)
	return nil
}

// prunePreviews keeps the cache within its size, the least recently used
// clips going first.
func prunePreviews(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type f struct {
		path string
		size int64
		at   time.Time
	}
	var files []f
	var total int64
	for _, e := range entries {
		if e.IsDir() || !previewName.MatchString(e.Name()) {
			continue
		}
		if fi, err := e.Info(); err == nil {
			files = append(files, f{filepath.Join(dir, e.Name()), fi.Size(), fi.ModTime()})
			total += fi.Size()
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].at.Before(files[j].at) })
	for len(files) > 0 && (len(files) > previewMaxFiles || total > previewMaxBytes) {
		_ = os.Remove(files[0].path)
		total -= files[0].size
		files = files[1:]
	}
}

// prewarmPreviews starts clips for what Home shows first (its banner and
// the start of Continue Watching), so they're ready when someone rests
// there. Ones already made, being made or recently failed are skipped.
func (s *Server) prewarmPreviews(r *http.Request, wants []previewWant) {
	if !canConvert() {
		return
	}
	for _, want := range wants {
		if fi, err := os.Stat(filepath.Join(s.previewDir(), want.key()+".mp4")); err == nil && fi.Size() > 0 {
			continue
		}
		previewState.Lock()
		failedAt, failed := previewState.failed[want.key()]
		previewState.Unlock()
		if failed && time.Since(failedAt) < previewRetry {
			continue
		}
		s.startPreview(r, want)
	}
}
