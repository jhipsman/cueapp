package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Browsers open only some kinds of video: MP4 or WebM, H.264 (or VP9/AV1)
// pictures, AAC or Opus sound. Many files aren't that: MKV and AVI, Xvid and
// HEVC pictures, Dolby and DTS sound. When a file won't play in a browser,
// the player asks Cue to convert it as it plays: ffmpeg reads the file and
// sends an MP4 the browser opens, copying the picture when it can (cheap)
// and remaking it only when it must. Moving through the video starts the
// conversion again from that point.

const convertSealPrefix = "cue-convert:"

// convertMaxRunning is how many conversions run at once, so a small server
// isn't swamped.
const convertMaxRunning = 3

var convertSlots = make(chan struct{}, convertMaxRunning)

var ffmpegOnce = sync.OnceValues(func() (string, string) {
	f, _ := exec.LookPath("ffmpeg")
	p, _ := exec.LookPath("ffprobe")
	return f, p
})

// canConvert says whether this server has ffmpeg.
func canConvert() bool {
	f, p := ffmpegOnce()
	return f != "" && p != ""
}

// convertToken seals a file's address for the conversion endpoints, so only
// addresses Cue handed out are fetched.
func (s *Server) convertToken(link string) string {
	if link == "" || !canConvert() || s.profileBox == nil {
		return ""
	}
	sealed, err := s.profileBox.Encrypt(convertSealPrefix + link)
	if err != nil {
		return ""
	}
	return sealed
}

func (s *Server) convertTarget(r *http.Request) (string, bool) {
	plain, err := s.profileBox.Decrypt(r.URL.Query().Get("u"))
	if err != nil || !strings.HasPrefix(plain, convertSealPrefix) {
		return "", false
	}
	return strings.TrimPrefix(plain, convertSealPrefix), true
}

// probeInfo is what ffprobe says about a file, and what to do with it.
type probeInfo struct {
	Duration   float64 `json:"duration"` // seconds; 0 = unknown
	CopyVideo  bool    `json:"copyVideo"`
	Height     int     `json:"height"`
	VideoIndex int     `json:"-"`
	AudioIndex int     `json:"-"` // -1 = no sound
	AudioAAC   bool    `json:"-"`
}

var probeCache = struct {
	sync.Mutex
	m map[string]probeEntry
}{m: map[string]probeEntry{}}

type probeEntry struct {
	at   time.Time
	info probeInfo
}

func probeFile(ctx context.Context, link string) (probeInfo, error) {
	probeCache.Lock()
	if e, ok := probeCache.m[link]; ok && time.Since(e.at) < time.Hour {
		probeCache.Unlock()
		return e.info, nil
	}
	probeCache.Unlock()

	_, ffprobe := ffmpegOnce()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, ffprobe, "-v", "error", "-user_agent", convertUA,
		"-print_format", "json", "-show_format", "-show_streams", link)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		return probeInfo{}, fmt.Errorf("ffprobe: %v: %s", err, strings.TrimSpace(firstLine(errOut.String())))
	}
	var p struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			Index    int    `json:"index"`
			Type     string `json:"codec_type"`
			Codec    string `json:"codec_name"`
			PixFmt   string `json:"pix_fmt"`
			Height   int    `json:"height"`
			Channels int    `json:"channels"`
			Tags     struct {
				Language string `json:"language"`
			} `json:"tags"`
			Disposition struct {
				AttachedPic int `json:"attached_pic"`
				Default     int `json:"default"`
			} `json:"disposition"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out.Bytes(), &p); err != nil {
		return probeInfo{}, fmt.Errorf("ffprobe output: %w", err)
	}
	info := probeInfo{VideoIndex: -1, AudioIndex: -1}
	info.Duration, _ = strconv.ParseFloat(p.Format.Duration, 64)
	audioScore := -1
	for _, st := range p.Streams {
		switch st.Type {
		case "video":
			if st.Disposition.AttachedPic == 1 || info.VideoIndex >= 0 {
				continue
			}
			info.VideoIndex = st.Index
			info.Height = st.Height
			// H.264 in 8-bit 4:2:0 plays in every browser as it is.
			info.CopyVideo = st.Codec == "h264" && (st.PixFmt == "" || st.PixFmt == "yuv420p" || st.PixFmt == "yuvj420p")
		case "audio":
			// English first, then the file's default, then the first.
			score := 0
			lang := strings.ToLower(st.Tags.Language)
			if lang == "eng" || lang == "en" || lang == "english" {
				score += 4
			} else if lang == "" || lang == "und" {
				score += 1
			}
			if st.Disposition.Default == 1 {
				score += 2
			}
			if score > audioScore {
				audioScore = score
				info.AudioIndex = st.Index
				info.AudioAAC = st.Codec == "aac" && st.Channels <= 2
			}
		}
	}
	if info.VideoIndex < 0 {
		return probeInfo{}, fmt.Errorf("no picture in the file")
	}
	probeCache.Lock()
	for k, e := range probeCache.m {
		if time.Since(e.at) > time.Hour {
			delete(probeCache.m, k)
		}
	}
	probeCache.m[link] = probeEntry{at: time.Now(), info: info}
	probeCache.Unlock()
	return info, nil
}

const convertUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36"

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// GET /api/play/convert?u=<token>: whether the file can be converted, how
// long it is, and the address that plays it (add &t=<seconds> to start
// further in).
func (s *Server) handleConvertProbe(w http.ResponseWriter, r *http.Request) {
	link, ok := s.convertTarget(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "That video address isn't one Cue made.")
		return
	}
	if !canConvert() {
		writeError(w, http.StatusNotImplemented, "This server can't convert videos (ffmpeg isn't installed).")
		return
	}
	info, err := probeFile(r.Context(), link)
	if err != nil {
		slog.Info("play: convert: read the file", "err", redactURLText(err.Error(), link))
		writeError(w, http.StatusBadGateway, "Couldn't open that version to convert it.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"duration":  info.Duration,
		"copyVideo": info.CopyVideo,
		"stream":    "/api/play/convert/stream?u=" + url.QueryEscape(r.URL.Query().Get("u")),
	})
}

// GET /api/play/convert/stream?u=<token>[&t=<seconds>]: the file as an MP4
// a browser plays, made as it's sent.
func (s *Server) handleConvertStream(w http.ResponseWriter, r *http.Request) {
	link, ok := s.convertTarget(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "That video address isn't one Cue made.")
		return
	}
	if !canConvert() {
		writeError(w, http.StatusNotImplemented, "This server can't convert videos (ffmpeg isn't installed).")
		return
	}
	info, err := probeFile(r.Context(), link)
	if err != nil {
		writeError(w, http.StatusBadGateway, "Couldn't open that version to convert it.")
		return
	}
	select {
	case convertSlots <- struct{}{}:
		defer func() { <-convertSlots }()
	default:
		writeError(w, http.StatusServiceUnavailable, "Cue is converting as many videos as it can right now. Try again in a moment.")
		return
	}
	start, _ := strconv.ParseFloat(r.URL.Query().Get("t"), 64)

	args := []string{"-hide_banner", "-loglevel", "error", "-nostdin",
		"-reconnect", "1", "-reconnect_streamed", "1", "-reconnect_delay_max", "5",
		"-user_agent", convertUA}
	if start > 0 {
		args = append(args, "-ss", strconv.FormatFloat(start, 'f', 1, 64))
	}
	args = append(args, "-i", link, "-map", fmt.Sprintf("0:%d", info.VideoIndex))
	if info.AudioIndex >= 0 {
		args = append(args, "-map", fmt.Sprintf("0:%d", info.AudioIndex))
	}
	if info.CopyVideo {
		args = append(args, "-c:v", "copy")
	} else {
		// Remade small enough to keep up on a modest server.
		preset, height := "veryfast", 720
		if info.Height > 720 {
			preset = "ultrafast"
		}
		args = append(args, "-c:v", "libx264", "-preset", preset, "-crf", "23", "-pix_fmt", "yuv420p",
			"-vf", fmt.Sprintf("scale=-2:'min(%d,ih)'", height))
	}
	if info.AudioIndex >= 0 {
		if info.AudioAAC {
			args = append(args, "-c:a", "copy")
		} else {
			args = append(args, "-c:a", "aac", "-ac", "2", "-b:a", "160k")
		}
	}
	args = append(args, "-sn", "-dn", "-f", "mp4",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof", "-frag_duration", "2000000", "pipe:1")

	ffmpeg, _ := ffmpegOnce()
	cmd := exec.CommandContext(r.Context(), ffmpeg, args...)
	var errOut bytes.Buffer
	cmd.Stderr = &errOut
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "no-store")
	cmd.Stdout = flushWriter{w}
	if err := cmd.Run(); err != nil && r.Context().Err() == nil {
		slog.Info("play: convert", "err", err, "ffmpeg", redactURLText(firstLine(errOut.String()), link))
	}
}

// flushWriter sends each piece to the player as soon as ffmpeg makes it.
type flushWriter struct{ w http.ResponseWriter }

func (f flushWriter) Write(p []byte) (int, error) {
	n, err := f.w.Write(p)
	if fl, ok := f.w.(http.Flusher); ok {
		fl.Flush()
	}
	return n, err
}

// redactURLText keeps a file's address (which can hold a debrid key) out of
// the log.
func redactURLText(s, link string) string {
	if link == "" {
		return s
	}
	return strings.ReplaceAll(s, link, "<file>")
}
