package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rdborg/mediarium/internal/crypto"
)

// TestConvertPlaysInBrowsers makes files browsers can't open (an Xvid AVI
// with MP3 sound, an HEVC MKV with Dolby sound) and checks the conversion
// sends H.264 and AAC in an MP4, from the start and from further in.
func TestConvertPlaysInBrowsers(t *testing.T) {
	if !canConvert() {
		t.Skip("no ffmpeg here")
	}
	dir := t.TempDir()
	make := func(name string, args ...string) {
		base := []string{"-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=size=320x240:rate=25:duration=20",
			"-f", "lavfi", "-i", "sine=frequency=440:duration=20"}
		out, err := exec.Command("ffmpeg", append(append(base, args...), filepath.Join(dir, name))...).CombinedOutput()
		if err != nil {
			t.Skipf("can't make %s here: %v %s", name, err, out)
		}
	}
	make("xvid.avi", "-c:v", "mpeg4", "-c:a", "libmp3lame")
	make("hevc.mkv", "-c:v", "libx265", "-c:a", "ac3", "-x265-params", "log-level=none")
	make("h264.mkv", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "ac3")
	files := httptest.NewServer(http.FileServer(http.Dir(dir)))
	defer files.Close()

	box, err := crypto.LoadOrCreateKey(filepath.Join(dir, "key"))
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{profileBox: box}
	for _, c := range []struct {
		name string
		copy bool
	}{{"xvid.avi", false}, {"hevc.mkv", false}, {"h264.mkv", true}} {
		t.Run(c.name, func(t *testing.T) {
			tok := s.convertToken(files.URL + "/" + c.name)
			if tok == "" {
				t.Fatal("no token")
			}
			rec := httptest.NewRecorder()
			s.handleConvertProbe(rec, httptest.NewRequest("GET", "/api/play/convert?u="+url.QueryEscape(tok), nil))
			var probe struct {
				Duration  float64 `json:"duration"`
				CopyVideo bool    `json:"copyVideo"`
				Stream    string  `json:"stream"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &probe); err != nil || rec.Code != 200 {
				t.Fatalf("probe: %d %s", rec.Code, rec.Body)
			}
			if probe.Duration < 19 || probe.Duration > 21 || probe.CopyVideo != c.copy {
				t.Errorf("probe = %+v", probe)
			}
			for _, from := range []string{"", "&t=10"} {
				rec := httptest.NewRecorder()
				s.handleConvertStream(rec, httptest.NewRequest("GET", probe.Stream+from, nil).WithContext(context.Background()))
				if rec.Code != 200 || rec.Header().Get("Content-Type") != "video/mp4" || rec.Body.Len() < 1000 {
					t.Fatalf("stream%s: %d %d bytes", from, rec.Code, rec.Body.Len())
				}
				out := filepath.Join(dir, "out.mp4")
				if err := os.WriteFile(out, rec.Body.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
				var info bytes.Buffer
				cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "stream=codec_name", "-of", "csv=p=0", out)
				cmd.Stdout = &info
				if err := cmd.Run(); err != nil {
					t.Fatal(err)
				}
				if got := strings.Fields(info.String()); strings.Join(got, ",") != "h264,aac" {
					t.Errorf("stream%s codecs = %v", from, got)
				}
			}
		})
	}
}
