package api

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jhipsman/cueapp/internal/auth"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Previews, made the way the streaming services make theirs: a short clip
// of the title itself (12 seconds from early on, 720p H.264 with
// stereo AAC sound that fades in and out, a few megabytes), cut once on this
// server from the same version Play would use and kept in a cache. Pages
// then play it in an ordinary video element: it starts at once, and nothing
// but the picture shows (no YouTube player, no buttons). A title's page
// plays it with sound, as Netflix does; Home and the cards keep it muted. Home asks for its banner's and the first of
// Continue Watching's ahead of time, so they're ready before anyone rests
// on them.

const (
	previewSeconds   = 12
	previewMaxFiles  = 400
	previewMaxBytes  = 1500 << 20
	previewRetry     = 30 * time.Minute // after a failed try, how long before another
	previewMaxQueued = 12               // clips waiting their turn
	previewForgotten = 75 * time.Second // a clip nobody has asked about for this long isn't wanted any more
)

var (
	previewName  = regexp.MustCompile(`^(movie-\d+|tv-\d+-\d+-\d+)\.mp4$`)
	previewWake  = make(chan struct{}, 1) // a new clip is waiting
	previewOnce  sync.Once
	previewState = struct {
		sync.Mutex
		making  map[string]bool      // waiting or being made
		asked   map[string]time.Time // when someone last asked for each
		queue   []*previewJob        // waiting, in no order (the worker picks)
		failed  map[string]time.Time
		started []time.Time // when each ahead-of-time clip of the last hour was started
	}{making: map[string]bool{}, asked: map[string]time.Time{}, failed: map[string]time.Time{}}
)

// previewJob is a clip waiting its turn. Ahead-of-time ones (ahead: Home's
// banner and Continue Watching) wait for a quiet server; one someone is
// looking at goes first, newest first, and is dropped once they've moved on.
type previewJob struct {
	want   previewWant
	ctx    context.Context
	header http.Header
	ahead  bool
}

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

// previewDir holds the clips. (Clips with sound live in previews-v2; the
// first, silent ones in previews are cleared the first time one is made.)
func (s *Server) previewDir() string { return filepath.Join(s.cfg.ConfigDir, "cache", "previews-v2") }

var clearSilentPreviews sync.Once

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
	// Asked for by someone looking at it: made next (or kept at the front
	// while it waits, as long as they keep asking).
	s.startPreview(r, want, false)
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

// previewRoom says whether an ahead-of-time clip may start: the server isn't
// busy (its load, averaged over the last minute, is under 70% of its cores)
// and it hasn't made its hour's share of them (12 an hour for each core).
// They're a nicety: on a one-core server they wait for quiet moments. (A
// clip someone is looking at doesn't wait: it runs at the lowest priority,
// so it never slows anyone's film.)
func previewRoom() bool {
	cores := runtime.NumCPU()
	if b, err := os.ReadFile("/proc/loadavg"); err == nil {
		if f := strings.Fields(string(b)); len(f) > 0 {
			if load, err := strconv.ParseFloat(f[0], 64); err == nil && load > 0.7*float64(cores) {
				return false
			}
		}
	}
	previewState.Lock()
	defer previewState.Unlock()
	cut := time.Now().Add(-time.Hour)
	recent := previewState.started[:0]
	for _, t := range previewState.started {
		if t.After(cut) {
			recent = append(recent, t)
		}
	}
	previewState.started = recent
	perHour := 12
	if quietTime() {
		perHour = 40 // nobody's using Cue: one after another
	}
	return len(recent) < perHour*cores
}

// ---- While nobody's watching ----

// tonight is what each account's Home last showed: made while nobody is
// using Cue (quietTime: overnight, mostly), one after another, so most
// titles have a clip before anyone opens them, as the streaming services
// prepare theirs ahead.
var tonight = struct {
	sync.Mutex
	byUser map[int64]tonightList
}{byUser: map[int64]tonightList{}}

type tonightList struct {
	ctx    context.Context
	header http.Header
	wants  []previewWant
}

// rememberForTonight keeps a Home's titles (with who asked, so they're
// made as that profile would play them) for the night.
func rememberForTonight(r *http.Request, wants []previewWant) {
	u := auth.UserFromContext(r.Context())
	if u == nil || len(wants) == 0 || !canConvert() {
		return
	}
	tonight.Lock()
	tonight.byUser[u.ID] = tonightList{ctx: context.WithoutCancel(r.Context()), header: r.Header.Clone(), wants: wants}
	tonight.Unlock()
}

// lastUsed is when a signed-in request last came in (access.go), in Unix
// seconds.
var lastUsed atomic.Int64

// quietTime is when nobody has used Cue for half an hour: overnight, mostly,
// whatever the server's time zone.
func quietTime() bool {
	return time.Since(time.Unix(lastUsed.Load(), 0)) > 30*time.Minute
}

// nextTonight is the next title remembered for the night that has no clip
// yet (nor a recent failed try), taken off its list.
func (s *Server) nextTonight() *previewJob {
	tonight.Lock()
	defer tonight.Unlock()
	for id, l := range tonight.byUser {
		for len(l.wants) > 0 {
			want := l.wants[0]
			l.wants = l.wants[1:]
			key := want.key()
			if fi, err := os.Stat(filepath.Join(s.previewDir(), key+".mp4")); err == nil && fi.Size() > 0 {
				continue
			}
			previewState.Lock()
			failedAt, failed := previewState.failed[key]
			busy := previewState.making[key]
			if !busy && !(failed && time.Since(failedAt) < previewRetry) {
				previewState.making[key] = true
				previewState.started = append(previewState.started, time.Now())
				previewState.Unlock()
				tonight.byUser[id] = l
				return &previewJob{want: want, ctx: l.ctx, header: l.header, ahead: true}
			}
			previewState.Unlock()
		}
		delete(tonight.byUser, id)
	}
	return nil
}

// startPreview puts a clip in line (once per title), to be made as the
// profile asking would play it. ahead marks one nobody is looking at yet.
func (s *Server) startPreview(r *http.Request, want previewWant, ahead bool) {
	key := want.key()
	previewState.Lock()
	if !ahead {
		previewState.asked[key] = time.Now()
	}
	if previewState.making[key] {
		if !ahead { // someone's looking at it now: it's no longer just ahead of time
			for _, j := range previewState.queue {
				if j.want.key() == key {
					j.ahead = false
				}
			}
		}
		previewState.Unlock()
		return
	}
	if len(previewState.queue) >= previewMaxQueued {
		// Full: the oldest waiting clip gives way.
		old := previewState.queue[0]
		delete(previewState.making, old.want.key())
		previewState.queue = previewState.queue[1:]
	}
	previewState.making[key] = true
	// Who's asking (account and profile) stays with the work; the request
	// itself may be long finished by then.
	previewState.queue = append(previewState.queue, &previewJob{want: want, ctx: context.WithoutCancel(r.Context()), header: r.Header.Clone(), ahead: ahead})
	previewState.Unlock()
	previewOnce.Do(func() { go s.previewWorker() })
	select {
	case previewWake <- struct{}{}:
	default:
	}
}

// nextPreview takes the clip to make now: the one someone asked about most
// recently, or (with none of those) an ahead-of-time one if the server has
// room. Clips nobody has asked about for a while are dropped.
func nextPreview() *previewJob {
	room := previewRoom() // asked first: it takes the lock itself
	previewState.Lock()
	defer previewState.Unlock()
	best := -1
	var bestAt time.Time
	kept := previewState.queue[:0]
	for _, j := range previewState.queue {
		key := j.want.key()
		at := previewState.asked[key]
		if !j.ahead && time.Since(at) > previewForgotten {
			delete(previewState.making, key) // they've moved on
			continue
		}
		kept = append(kept, j)
		if !j.ahead && (best < 0 || at.After(bestAt)) {
			best, bestAt = len(kept)-1, at
		}
	}
	previewState.queue = kept
	if best < 0 {
		for i, j := range kept {
			if j.ahead {
				best = i
				break
			}
		}
		if best < 0 || !room {
			return nil
		}
		previewState.started = append(previewState.started, time.Now())
	}
	j := previewState.queue[best]
	previewState.queue = append(previewState.queue[:best], previewState.queue[best+1:]...)
	return j
}

// previewWorker makes clips one at a time (it's ffmpeg work), for as long
// as Cue runs, checking back every few seconds while ahead-of-time ones
// wait for a quiet server.
func (s *Server) previewWorker() {
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	for {
		j := nextPreview()
		if j == nil && quietTime() && previewRoom() {
			j = s.nextTonight()
		}
		if j == nil {
			select {
			case <-previewWake:
			case <-tick.C:
			}
			continue
		}
		key := j.want.key()
		ctx, cancel := context.WithTimeout(j.ctx, 4*time.Minute)
		err := s.makePreview(ctx, j.header, j.want)
		cancel()
		previewState.Lock()
		delete(previewState.making, key)
		delete(previewState.asked, key)
		if err != nil {
			previewState.failed[key] = time.Now()
		}
		previewState.Unlock()
		if err != nil {
			slog.Info("preview: couldn't make one", "title", key, "err", err)
		}
	}
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
	clearSilentPreviews.Do(func() { _ = os.RemoveAll(filepath.Join(s.cfg.ConfigDir, "cache", "previews")) })
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
		"-t", strconv.Itoa(previewSeconds), "-map", "0:v:0", "-map", "0:a:0?", "-sn", "-dn",
		"-vf", "scale=-2:'min(720,ih)',fps=30,format=yuv420p",
		// The cheapest encoding there is: the work is in reading the source,
		// and on a small server every second of processor counts.
		"-c:v", "libx264", "-preset", "ultrafast", "-tune", "fastdecode", "-crf", "28", "-threads", "1",
		// The sound in stereo, fading in and out so the loop doesn't jump.
		"-af", fmt.Sprintf("afade=t=in:d=1.2,afade=t=out:st=%d:d=1.8", previewSeconds-2),
		"-c:a", "aac", "-b:a", "128k", "-ac", "2",
		"-movflags", "+faststart", "-f", "mp4", tmp)
	cmd := exec.CommandContext(ctx, ffmpeg, args...)
	var out strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		return err
	}
	backgroundPriority(cmd.Process) // never in the way of someone watching
	if err := cmd.Wait(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("ffmpeg: %v: %s", err, redactURLText(firstLine(out.String()), link))
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
		s.startPreview(r, want, true)
	}
}
