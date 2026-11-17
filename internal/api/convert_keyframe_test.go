package api

import (
	"context"
	"math"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestKeyframeAt(t *testing.T) {
	if !canConvert() {
		t.Skip("no ffmpeg here")
	}
	clip := filepath.Join(t.TempDir(), "gop.mp4")
	// A keyframe every 5 seconds (120 frames at 24 fps).
	if out, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=size=320x180:rate=24:duration=30",
		"-c:v", "libx264", "-g", "120", "-keyint_min", "120", "-sc_threshold", "0", "-pix_fmt", "yuv420p", clip).CombinedOutput(); err != nil {
		t.Skipf("can't make a clip: %v %s", err, out)
	}
	for asked, want := range map[float64]float64{12: 10, 10: 10, 24.9: 20, 3: 0} {
		if got := keyframeAt(context.Background(), clip, asked); math.Abs(got-want) > 0.1 {
			t.Errorf("keyframe at or before %.1f: %.2f, want %.0f", asked, got, want)
		}
	}
	// Remade pictures start exactly where asked.
	if got := convertStart(context.Background(), clip, probeInfo{VideoCodec: "mpeg4"}, 12, false); got != 12 {
		t.Errorf("remade picture starts at %.2f", got)
	}
	if got := convertStart(context.Background(), clip, probeInfo{VideoCodec: "h264", CopyVideo: true}, 12, false); math.Abs(got-10) > 0.1 {
		t.Errorf("copied picture starts at %.2f", got)
	}
}
