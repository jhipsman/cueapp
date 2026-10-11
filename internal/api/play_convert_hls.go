package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Conversion as HLS, for Safari (iPhone, iPad, Mac). Safari plays a video
// only when it can fetch it in pieces (a byte range at a time), which a
// stream being made can't offer; HLS it plays natively. ffmpeg writes
// four-second pieces and a playlist that grows into a folder for each
// session; Safari reads them as they come. A session no one has asked for
// in a while is stopped and its folder removed. Moving further than what's
// made starts a session from there (the page's timeline does that).
//
// The Apple TV app asks with &apple=1: its player takes HEVC (4K, HDR) and
// Dolby sound, so the picture and sound are copied rather than remade
// (convertArgs), into fragmented-MP4 pieces, which is what Apple's player
// wants for HEVC. Copying runs far faster than playing, so it's held about
// ten minutes ahead of the viewer, and pieces well behind them are
// deleted, which keeps a 4K film from filling the server's disk.

const (
	hlsIdle      = 90 * time.Second // a session unused this long is stopped
	hlsIdleApple = 20 * time.Minute // the Apple TV's (it may be paused a while)
	hlsFirstWait = 30 * time.Second // how long the first piece may take

	hlsAheadMax   = 150 // Apple TV: pieces (4 s each) made past the viewer before pausing
	hlsAheadMin   = 75  // and when to carry on
	hlsBehindKeep = 450 // pieces kept behind the viewer (30 minutes)
)

type hlsSession struct {
	id       string
	dir      string
	cancel   context.CancelFunc
	lastUsed time.Time
	done     chan struct{}
	apple    bool
	cmd      *exec.Cmd
	lastSeg  int  // the furthest piece the player has asked for
	paused   bool // ffmpeg stopped while the viewer catches up
}

type hlsState struct {
	mu       sync.Mutex
	sessions map[string]*hlsSession // by id
	byKey    map[string]string      // token@start -> id
	janitor  bool
}

var hls = hlsState{sessions: map[string]*hlsSession{}, byKey: map[string]string{}}

var hlsName = regexp.MustCompile(`^(index\.m3u8|init\.mp4|seg_(\d{5})\.(ts|m4s))$`)

// GET /api/play/convert/hls?u=<token>[&t=<seconds>]: starts (or finds) a
// session and sends the player on to its playlist once the first piece is
// ready.
func (s *Server) handleConvertHLS(w http.ResponseWriter, r *http.Request) {
	link, ok := s.convertTarget(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "That video address isn't one Cue made.")
		return
	}
	if !canConvert() {
		writeError(w, http.StatusNotImplemented, "This server can't convert videos (ffmpeg isn't installed).")
		return
	}
	start, _ := strconv.ParseFloat(r.URL.Query().Get("t"), 64)
	apple := r.URL.Query().Get("apple") == "1"
	key := fmt.Sprintf("%s@%d@%t", link, int(start), apple)

	hls.mu.Lock()
	s.startHLSJanitorLocked()
	sess := hls.sessions[hls.byKey[key]]
	if sess != nil {
		sess.lastUsed = time.Now()
	}
	hls.mu.Unlock()

	if sess == nil {
		info, err := probeFile(r.Context(), link)
		if err != nil {
			slog.Info("play: convert (HLS): read the file", "err", redactURLText(err.Error(), link))
			writeError(w, http.StatusBadGateway, "Couldn't open that version to convert it.")
			return
		}
		sess, err = startHLS(key, link, info, start, apple)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
	}

	// Wait for the first piece, so the player gets a playlist it can play.
	deadline := time.Now().Add(hlsFirstWait)
	ready := func() bool {
		b, err := os.ReadFile(filepath.Join(sess.dir, "index.m3u8"))
		return err == nil && bytes.Contains(b, []byte("#EXTINF"))
	}
	for !ready() {
		select {
		case <-sess.done:
			if ready() {
				continue
			}
			writeError(w, http.StatusBadGateway, "That version couldn't be converted.")
			return
		case <-r.Context().Done():
			return
		case <-time.After(250 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			writeError(w, http.StatusGatewayTimeout, "Converting that version is taking too long.")
			return
		}
	}
	http.Redirect(w, r, "/api/play/convert/hls/"+sess.id+"/index.m3u8", http.StatusFound)
}

func startHLS(key, link string, info probeInfo, start float64, apple bool) (*hlsSession, error) {
	hls.mu.Lock()
	defer hls.mu.Unlock()
	if id, ok := hls.byKey[key]; ok && hls.sessions[id] != nil {
		return hls.sessions[id], nil
	}
	// Room for one more: the longest-idle session makes way.
	if len(hls.sessions) >= convertMaxRunning {
		var oldest *hlsSession
		for _, ss := range hls.sessions {
			if oldest == nil || ss.lastUsed.Before(oldest.lastUsed) {
				oldest = ss
			}
		}
		if oldest == nil || time.Since(oldest.lastUsed) < 15*time.Second {
			return nil, fmt.Errorf("Cue is converting as many videos as it can right now. Try again in a moment.")
		}
		stopHLSLocked(oldest)
	}
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	dir := filepath.Join(os.TempDir(), "cue-hls", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("Couldn't get the conversion ready.")
	}
	ctx, cancel := context.WithCancel(context.Background())
	sess := &hlsSession{id: id, dir: dir, cancel: cancel, lastUsed: time.Now(), done: make(chan struct{}), apple: apple}
	args := append(convertArgs(link, info, start, apple),
		"-f", "hls", "-hls_time", "4", "-hls_list_size", "0", "-hls_playlist_type", "event",
		"-hls_flags", "independent_segments+temp_file")
	if apple {
		args = append(args, "-hls_segment_type", "fmp4", "-hls_fmp4_init_filename", "init.mp4",
			"-hls_segment_filename", filepath.Join(dir, "seg_%05d.m4s"), filepath.Join(dir, "index.m3u8"))
	} else {
		args = append(args, "-hls_segment_filename", filepath.Join(dir, "seg_%05d.ts"), filepath.Join(dir, "index.m3u8"))
	}
	ffmpeg, _ := ffmpegOnce()
	cmd := exec.CommandContext(ctx, ffmpeg, args...)
	var errOut bytes.Buffer
	cmd.Stderr = &errOut
	if err := cmd.Start(); err != nil {
		cancel()
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("Couldn't start converting.")
	}
	sess.cmd = cmd
	if apple {
		go paceHLS(sess)
	}
	go func() {
		err := cmd.Wait()
		if err != nil && ctx.Err() == nil {
			slog.Info("play: convert (HLS)", "err", err, "ffmpeg", redactURLText(firstLine(errOut.String()), link))
		}
		close(sess.done)
	}()
	hls.sessions[id] = sess
	hls.byKey[key] = id
	return sess, nil
}

func stopHLSLocked(sess *hlsSession) {
	if sess.paused && sess.cmd != nil {
		pauseProcess(sess.cmd.Process, false) // so it can end
	}
	sess.cancel()
	delete(hls.sessions, sess.id)
	for k, id := range hls.byKey {
		if id == sess.id {
			delete(hls.byKey, k)
		}
	}
	go func() {
		<-sess.done
		_ = os.RemoveAll(sess.dir)
	}()
}

// startHLSJanitorLocked stops sessions nobody has asked for in a while.
func (s *Server) startHLSJanitorLocked() {
	if hls.janitor {
		return
	}
	hls.janitor = true
	go func() {
		for range time.Tick(20 * time.Second) {
			hls.mu.Lock()
			for _, ss := range hls.sessions {
				idle := hlsIdle
				if ss.apple {
					idle = hlsIdleApple
				}
				if time.Since(ss.lastUsed) > idle {
					stopHLSLocked(ss)
				}
			}
			hls.mu.Unlock()
		}
	}()
}

// GET /api/play/convert/hls/{id}/{file}: a session's playlist or a piece.
// The id is long and random: it's the key, like a signed address.
func (s *Server) handleConvertHLSFile(w http.ResponseWriter, r *http.Request) {
	id, name := r.PathValue("id"), r.PathValue("file")
	m := hlsName.FindStringSubmatch(name)
	if m == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	hls.mu.Lock()
	sess := hls.sessions[id]
	if sess != nil {
		sess.lastUsed = time.Now()
		if n, err := strconv.Atoi(m[2]); err == nil && n > sess.lastSeg {
			sess.lastSeg = n
		}
	}
	hls.mu.Unlock()
	if sess == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	path := filepath.Join(sess.dir, name)
	if name == "index.m3u8" {
		b, err := os.ReadFile(path)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		// Played from the start, not from the newest piece.
		text := strings.Replace(string(b), "#EXTM3U\n", "#EXTM3U\n#EXT-X-START:TIME-OFFSET=0,PRECISE=YES\n", 1)
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(text))
		return
	}
	switch {
	case name == "init.mp4":
		w.Header().Set("Content-Type", "video/mp4")
	case strings.HasSuffix(name, ".m4s"):
		w.Header().Set("Content-Type", "video/iso.segment")
	default:
		w.Header().Set("Content-Type", "video/mp2t")
	}
	w.Header().Set("Cache-Control", "private, max-age=600")
	http.ServeFile(w, r, path)
}

// paceHLS keeps an Apple TV conversion about ten minutes ahead of the
// viewer (pausing ffmpeg while they catch up) and deletes pieces well
// behind them, until the conversion ends.
func paceHLS(sess *hlsSession) {
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-sess.done:
			return
		case <-tick.C:
		}
		made := -1
		entries, _ := os.ReadDir(sess.dir)
		var pieces []int
		for _, e := range entries {
			m := hlsName.FindStringSubmatch(e.Name())
			if m == nil || m[2] == "" {
				continue
			}
			n, _ := strconv.Atoi(m[2])
			pieces = append(pieces, n)
			made = max(made, n)
		}
		hls.mu.Lock()
		viewer := sess.lastSeg
		ahead := made - viewer
		switch {
		case !sess.paused && ahead > hlsAheadMax && sess.cmd != nil:
			pauseProcess(sess.cmd.Process, true)
			sess.paused = true
		case sess.paused && ahead < hlsAheadMin && sess.cmd != nil:
			pauseProcess(sess.cmd.Process, false)
			sess.paused = false
		}
		hls.mu.Unlock()
		for _, n := range pieces {
			if n < viewer-hlsBehindKeep {
				_ = os.Remove(filepath.Join(sess.dir, fmt.Sprintf("seg_%05d.m4s", n)))
			}
		}
	}
}
