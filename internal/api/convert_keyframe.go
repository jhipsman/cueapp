package api

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Starting a converted video part-way in, in step. When the picture is
// copied (not remade), it can only start at a keyframe: the one at or before
// the time asked for, often a few seconds earlier. The sound, remade, would
// start exactly at the time asked, so it ran seconds behind the picture. So
// such a conversion starts at that keyframe, picture and sound together, and
// the player asks first where that is (GET /api/play/convert/start), so it
// knows where in the title its video begins.

// copiesPicture says whether convertArgs copies the picture as it is.
func copiesPicture(info probeInfo, apple bool) bool {
	return ((apple || info.KeepVideo) && info.VideoCodec == "hevc") || info.CopyVideo || info.KeepVideo
}

var keyframes = struct {
	sync.Mutex
	m map[string]float64
}{m: map[string]float64{}}

// keyframeAt is the time of the keyframe at or before t (t itself when it
// can't be found, or the file has none listed nearby).
func keyframeAt(ctx context.Context, link string, t float64) float64 {
	if t <= 0 {
		return 0
	}
	key := fmt.Sprintf("%s@%d", link, int(t*10))
	keyframes.Lock()
	if k, ok := keyframes.m[key]; ok {
		keyframes.Unlock()
		return k
	}
	keyframes.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	_, ffprobe := ffmpegOnce()
	// Read a little from just before t: the frames that are keyframes.
	from := math.Max(0, t-15)
	args := []string{"-v", "error", "-select_streams", "v:0", "-skip_frame", "nokey",
		"-show_entries", "frame=best_effort_timestamp_time", "-read_intervals", fmt.Sprintf("%.2f%%+%.2f", from, t-from+1), "-of", "csv=p=0"}
	if isWebLink(link) {
		args = append(args, "-user_agent", convertUA)
	}
	out, err := exec.CommandContext(ctx, ffprobe, append(args, link)...).Output()
	best := t
	if err == nil {
		found := -1.0
		for _, line := range strings.Fields(string(out)) {
			v, err := strconv.ParseFloat(strings.TrimSuffix(line, ","), 64)
			if err == nil && v <= t+0.05 && v > found {
				found = v
			}
		}
		if found >= 0 {
			best = found
		}
	}
	keyframes.Lock()
	if len(keyframes.m) > 2000 {
		keyframes.m = map[string]float64{}
	}
	keyframes.m[key] = best
	keyframes.Unlock()
	return best
}

// convertStart is where a conversion asked to start at t really starts.
func convertStart(ctx context.Context, link string, info probeInfo, t float64, apple bool) float64 {
	if t <= 0 || !copiesPicture(info, apple) {
		return math.Max(0, t)
	}
	return keyframeAt(ctx, link, t)
}

// GET /api/play/convert/start?u=&t=[&keep=1][&apple=1][&hls=1]: where a
// conversion asked to start at t begins (in seconds), so the player's
// timeline matches what it shows.
func (s *Server) handleConvertStart(w http.ResponseWriter, r *http.Request) {
	link, ok := s.convertTarget(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "That video address isn't one Cue made.")
		return
	}
	t, _ := strconv.ParseFloat(r.URL.Query().Get("t"), 64)
	info, err := probeFile(r.Context(), link)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]float64{"start": math.Max(0, t)})
		return
	}
	keepPicture(r, &info)
	if r.URL.Query().Get("hls") == "1" && info.VideoCodec == "hevc" {
		info.KeepVideo = true // as handleConvertHLS does for Safari
	}
	writeJSON(w, http.StatusOK, map[string]float64{"start": convertStart(r.Context(), link, info, t, r.URL.Query().Get("apple") == "1")})
}
