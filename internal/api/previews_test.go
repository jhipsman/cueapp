package api

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
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
// asked about lately are dropped; ahead-of-time ones wait their turn.
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
		previewState.Unlock()
	}()

	if j := nextPreview(); j == nil || j.want.tmdbID != 3 {
		t.Fatalf("first = %+v, want the one asked about just now (3)", j)
	}
	if j := nextPreview(); j == nil || j.want.tmdbID != 1 {
		t.Fatalf("second = %+v, want 1", j)
	}
	previewState.Lock()
	gone := !previewState.making["movie-4"]
	previewState.Unlock()
	if !gone {
		t.Error("a clip nobody asked about for minutes is still waiting")
	}
}
