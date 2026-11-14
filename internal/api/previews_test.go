package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jhipsman/cueapp/internal/auth"
)

// A preview is a short, silent, browser-friendly clip cut from early in the
// title: here from a generated two-minute film.
func TestCutPreview(t *testing.T) {
	if !canConvert() {
		t.Skip("no ffmpeg here")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "film.mkv")
	ffmpeg, ffprobe := ffmpegOnce()
	if out, err := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc=size=1920x1080:rate=24:duration=120",
		"-f", "lavfi", "-i", "sine=duration=120", "-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac", "-shortest", src).CombinedOutput(); err != nil {
		t.Fatalf("make a film: %v %s", err, out)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	want := previewWant{kind: "movie", tmdbID: 7}
	if err := cutPreview(ctx, src, want, dir); err != nil {
		t.Fatal(err)
	}
	clip := filepath.Join(dir, "movie-7.mp4")
	out, err := exec.Command(ffprobe, "-v", "error", "-show_entries", "stream=codec_type,codec_name,height:format=duration", "-of", "default=nw=1", clip).Output()
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, w := range []string{"codec_name=h264", "height=720"} {
		if !strings.Contains(got, w) {
			t.Errorf("clip: want %s in:\n%s", w, got)
		}
	}
	if !strings.Contains(got, "codec_name=aac") {
		t.Errorf("clip has no AAC sound:\n%s", got)
	}
	if _, err := os.Stat(clip + ".part"); err == nil {
		t.Error("the part file was left behind")
	}
}

// The cache keeps within its count, least recently used first.
func TestPrunePreviews(t *testing.T) {
	dir := t.TempDir()
	old := time.Now().Add(-time.Hour)
	for i := 0; i < previewMaxFiles+3; i++ {
		p := filepath.Join(dir, "movie-"+strconv.Itoa(i+1)+".mp4")
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		at := old.Add(time.Duration(i) * time.Second)
		_ = os.Chtimes(p, at, at)
	}
	prunePreviews(dir)
	if _, err := os.Stat(filepath.Join(dir, "movie-1.mp4")); err == nil {
		t.Error("the oldest clip was kept")
	}
	if _, err := os.Stat(filepath.Join(dir, "movie-"+strconv.Itoa(previewMaxFiles+3)+".mp4")); err != nil {
		t.Error("the newest clip was removed")
	}
}

// The clip someone asked about most recently goes first; ones nobody has
// asked about lately wait with the ahead-of-time ones.
func TestNextPreviewOrder(t *testing.T) {
	previewState.Lock()
	saved := previewState.queue
	previewState.queue = nil
	previewState.making = map[string]bool{}
	previewState.asked = map[string]time.Time{}
	now := time.Now()
	add := func(id int, ahead bool, asked time.Duration) {
		w := previewWant{kind: "movie", tmdbID: id}
		previewState.making[w.key()] = true
		if !ahead {
			previewState.asked[w.key()] = now.Add(-asked)
		}
		previewState.queue = append(previewState.queue, &previewJob{want: w, ahead: ahead})
	}
	add(1, false, 30*time.Second) // asked a while ago
	add(2, true, 0)               // ahead of time
	add(3, false, 2*time.Second)  // asked just now
	add(4, false, 5*time.Minute)  // long gone
	previewState.Unlock()
	defer func() {
		previewState.Lock()
		previewState.queue = saved
		previewState.making = map[string]bool{}
		previewState.asked = map[string]time.Time{}
		previewState.Unlock()
	}()

	if j := nextPreview(); j == nil || j.want.tmdbID != 3 {
		t.Fatalf("first = %+v, want the one asked about just now (3)", j)
	}
	if j := nextPreview(); j == nil || j.want.tmdbID != 1 {
		t.Fatalf("second = %+v, want 1", j)
	}
	// The one they left minutes ago is kept, waiting with the ahead-of-time
	// ones, so it's ready if they come back.
	previewState.Lock()
	kept := previewState.making["movie-4"]
	var demoted bool
	for _, j := range previewState.queue {
		if j.want.tmdbID == 4 {
			demoted = j.ahead
		}
	}
	previewState.Unlock()
	if !kept || !demoted {
		t.Errorf("a clip they left: kept %v, waiting with the ahead-of-time ones %v", kept, demoted)
	}
}

// Overnight, titles Home showed are made one by one, skipping ones that
// already have a clip.
func TestNextTonightSkipsMadeClips(t *testing.T) {
	s := newBareServer(t)
	dir := s.previewDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "movie-1.mp4"), []byte("clip"), 0o644); err != nil {
		t.Fatal(err)
	}
	previewState.Lock()
	previewState.making = map[string]bool{}
	previewState.failed = map[string]time.Time{}
	previewState.Unlock()
	tonight.Lock()
	tonight.byUser = map[int64]tonightList{7: {ctx: context.Background(), header: http.Header{}, wants: []previewWant{
		{kind: "movie", tmdbID: 1}, {kind: "movie", tmdbID: 2}, {kind: "tv", tmdbID: 3, season: 1, episode: 1},
	}}}
	tonight.Unlock()
	defer func() {
		previewState.Lock()
		delete(previewState.making, "movie-2")
		delete(previewState.making, "tv-3-1-1")
		previewState.Unlock()
	}()

	if j := s.nextTonight(); j == nil || j.want.key() != "movie-2" || !j.ahead {
		t.Fatalf("first = %+v, want movie-2 (movie-1 is made)", j)
	}
	if j := s.nextTonight(); j == nil || j.want.key() != "tv-3-1-1" {
		t.Fatalf("second = %+v, want tv-3-1-1", j)
	}
	if j := s.nextTonight(); j != nil {
		t.Fatalf("third = %+v, want nothing left", j)
	}
}

// A film whose first sound is Spanish still gets an English clip.
func TestCutPreviewPicksEnglish(t *testing.T) {
	ffmpeg, ffprobe := ffmpegOnce()
	if ffmpeg == "" || ffprobe == "" {
		t.Skip("no ffmpeg here")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "two.mkv")
	if out, err := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc=size=640x360:rate=24:duration=80",
		"-f", "lavfi", "-i", "sine=frequency=300:duration=80", "-f", "lavfi", "-i", "sine=frequency=900:duration=80",
		"-map", "0", "-map", "1", "-map", "2", "-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac",
		"-metadata:s:a:0", "language=spa", "-metadata:s:a:1", "language=eng", "-disposition:a:0", "default", "-disposition:a:1", "0",
		"-shortest", src).CombinedOutput(); err != nil {
		t.Fatalf("make the film: %v %s", err, out)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	want := previewWant{kind: "movie", tmdbID: 8}
	if err := cutPreview(ctx, src, want, dir); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(ffprobe, "-v", "error", "-select_streams", "a", "-show_entries", "stream_tags=language", "-of", "default=nw=1:nk=1",
		filepath.Join(dir, want.key()+".mp4")).Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got != "eng" {
		t.Errorf("clip's sound is %q, want eng", got)
	}
}

// Cutting a clip prefers a 1080p (or smaller) H.264 version over 4K or
// HEVC, keeping the usual order otherwise.
func TestClipFriendlyFirst(t *testing.T) {
	in := []playCandidate{
		{Release: "Film.2024.2160p.WEB-DL.x265"},
		{Release: "Film.2024.1080p.WEB-DL.x265"},
		{Release: "Film.2024.2160p.BluRay.x264"},
		{Release: "Film.2024.1080p.BluRay.x264"},
		{Release: "Film.2024.720p.WEB.x264"},
	}
	got := clipFriendlyFirst(in)
	want := []string{"Film.2024.1080p.BluRay.x264", "Film.2024.720p.WEB.x264", "Film.2024.1080p.WEB-DL.x265", "Film.2024.2160p.BluRay.x264", "Film.2024.2160p.WEB-DL.x265"}
	for i := range want {
		if got[i].Release != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
	if in[0].Release != "Film.2024.2160p.WEB-DL.x265" {
		t.Error("the list Play uses was changed")
	}
}

// A title's More Like This joins the night's list after Home's, and Home
// loading again keeps it (without repeats).
func TestTonightKeepsMoreLikeThis(t *testing.T) {
	if !canConvert() {
		t.Skip("no ffmpeg")
	}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = r.WithContext(auth.WithUser(r.Context(), &auth.User{ID: 4242}))
	defer func() {
		tonight.Lock()
		delete(tonight.byUser, 4242)
		tonight.Unlock()
	}()
	home := []previewWant{{kind: "movie", tmdbID: 1}, {kind: "movie", tmdbID: 2}}
	rememberForTonight(r, home)
	alsoTonight(r, []watchCard{{Kind: "movie", TMDBID: 2}, {Kind: "movie", TMDBID: 3}}, 8)
	rememberForTonight(r, home)
	tonight.Lock()
	got := tonight.byUser[4242].wants
	tonight.Unlock()
	if len(got) != 3 || got[2].tmdbID != 3 {
		t.Fatalf("night's list = %+v, want Home's two then 3", got)
	}
}
