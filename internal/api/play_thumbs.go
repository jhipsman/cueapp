package api

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

// Frames for the time bar: moving along it shows the picture at that point.
// ffmpeg takes one small frame from the file (the same one playing, by its
// sealed token, like the conversion), at 10-second steps, and the last few
// hundred are kept in memory, so going back and forth is quick. The address
// works without signing in (the token is the key), for the TV app.

const (
	thumbStep  = 10  // seconds between frames
	thumbWidth = 320 // pixels
	thumbKeep  = 400 // frames kept
)

var thumbSlots = make(chan struct{}, 4)

type thumbCache struct {
	mu    sync.Mutex
	order []string
	data  map[string][]byte
}

var thumbs = thumbCache{data: map[string][]byte{}}

func (c *thumbCache) get(k string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	b, ok := c.data[k]
	return b, ok
}

func (c *thumbCache) put(k string, b []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.data[k]; ok {
		return
	}
	c.data[k] = b
	c.order = append(c.order, k)
	for len(c.order) > thumbKeep {
		delete(c.data, c.order[0])
		c.order = c.order[1:]
	}
}

// GET /api/thumb?u=<token>&t=<seconds>: a JPEG of the picture there.
func (s *Server) handleThumb(w http.ResponseWriter, r *http.Request) {
	link, ok := s.convertTarget(r)
	if !ok || !canConvert() {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	t, _ := strconv.ParseFloat(r.URL.Query().Get("t"), 64)
	at := int(t) / thumbStep * thumbStep
	key := fmt.Sprintf("%s@%d", link, at)
	if b, ok := thumbs.get(key); ok {
		writeThumb(w, b)
		return
	}
	select {
	case thumbSlots <- struct{}{}:
		defer func() { <-thumbSlots }()
	case <-time.After(3 * time.Second):
		http.Error(w, "busy", http.StatusServiceUnavailable)
		return
	case <-r.Context().Done():
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	ffmpeg, _ := ffmpegOnce()
	args := []string{"-hide_banner", "-loglevel", "error", "-nostdin"}
	if isWebLink(link) {
		args = append(args, "-user_agent", convertUA)
	}
	args = append(args, "-ss", strconv.Itoa(at), "-i", link, "-frames:v", "1", "-an", "-sn",
		"-vf", fmt.Sprintf("scale=%d:-2", thumbWidth), "-q:v", "6", "-f", "mjpeg", "pipe:1")
	cmd := exec.CommandContext(ctx, ffmpeg, args...)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil || out.Len() == 0 {
		if r.Context().Err() == nil {
			slog.Info("play: time bar frame", "err", err, "ffmpeg", redactURLText(firstLine(errOut.String()), link))
		}
		http.Error(w, "no frame", http.StatusNotFound)
		return
	}
	thumbs.put(key, out.Bytes())
	writeThumb(w, out.Bytes())
}

func writeThumb(w http.ResponseWriter, b []byte) {
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	_, _ = w.Write(b)
}

func isWebLink(s string) bool {
	return len(s) > 7 && (s[:7] == "http://" || len(s) > 8 && s[:8] == "https://")
}
