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
	if strings.Contains(got, "codec_type=audio") {
		t.Errorf("clip has sound:\n%s", got)
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
