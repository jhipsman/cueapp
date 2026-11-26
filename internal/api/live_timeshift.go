package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/jhipsman/cueapp/internal/fsinfo"
	"github.com/jhipsman/cueapp/internal/settings"
)

// Pausing and going back in live TV. While a channel plays, Cue's server
// keeps its last half hour (up to two hours; Settings > Live TV): ffmpeg
// copies the provider's stream, unchanged, into four-second pieces and a
// playlist that keeps the newest of them, which players take as a live
// stream they can pause and move back in. The player plays that, not the
// provider's stream, so it's still one connection to the provider.
//
// One copy per channel, shared by everyone watching it. A player says who
// it is (v, a random id it makes); when it changes channel, the copy it was
// watching stops at once if no one else is (a provider allows only so many
// connections), and a copy no one has asked for in a while stops too. Its
// pieces are deleted when it stops.

const (
	timeshiftDefault  = 30  // minutes, when the owner hasn't said
	timeshiftMaxMins  = 120 // the most the owner can choose
	timeshiftMax      = 2   // copies running at once
	timeshiftPiece    = 4   // seconds a piece
	timeshiftIdle     = 2 * time.Minute
	timeshiftFirst    = 20 * time.Second // how long the first piece may take
	timeshiftQuickRun = 20 * time.Second // a copy that stops sooner than this failed
)

var timeshiftMinFree uint64 = 2 << 30 // bytes the disk keeps free

type timeshiftSession struct {
	id       string
	channel  string
	dir      string
	cancel   context.CancelFunc
	done     chan struct{}
	lastUsed time.Time
	viewers  map[string]time.Time // player ids
}

type timeshiftState struct {
	mu        sync.Mutex
	byID      map[string]*timeshiftSession
	byChannel map[string]*timeshiftSession
	janitor   bool
}

var timeshiftFile = regexp.MustCompile(`^(index\.m3u8|seg_\d{6}\.ts)$`)

// timeshiftMinutes is how much of a channel the server keeps (0: it doesn't).
func (s *Server) timeshiftMinutes() int {
	raw, err := s.Settings.Get(settings.KeyIPTVTimeshift)
	if err != nil || raw == "" {
		return timeshiftDefault
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return timeshiftDefault
	}
	return min(n, timeshiftMaxMins)
}

// timeshiftOn says live TV plays through the server's copy.
func (s *Server) timeshiftOn() bool { return s.timeshiftMinutes() > 0 && canConvert() }

func (s *Server) timeshiftDir() string { return filepath.Join(s.cfg.ConfigDir, "timeshift") }

// PUT /api/settings/iptv/timeshift {minutes}: how much of a channel the
// server keeps (0 turns it off).
func (s *Server) handlePutTimeshift(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Minutes int `json:"minutes"`
	}
	if err := decodeJSON(r, &req); err != nil || req.Minutes < 0 || req.Minutes > timeshiftMaxMins {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Between 0 (off) and %d minutes.", timeshiftMaxMins))
		return
	}
	if err := s.Settings.Set(settings.KeyIPTVTimeshift, strconv.Itoa(req.Minutes), false); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't save that.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"minutes": req.Minutes, "available": canConvert()})
}

// GET /api/live/timeshift/{id}?v=<player>: starts keeping the channel (or
// joins its copy) and answers the playlist's address once its first piece
// is made: {url, minutes}.
func (s *Server) handleLiveTimeshift(w http.ResponseWriter, r *http.Request) {
	mins := s.timeshiftMinutes()
	if mins == 0 || !canConvert() {
		writeError(w, http.StatusNotFound, "Pausing live TV is off.")
		return
	}
	viewer := r.URL.Query().Get("v")
	if len(viewer) > 64 {
		viewer = viewer[:64]
	}
	id := r.PathValue("id")
	ch, c, err := s.liveChannelByID(r.Context(), id)
	switch {
	case errors.Is(err, errLiveNotSet), errors.Is(err, errLiveNoChannel):
		writeError(w, http.StatusNotFound, "That channel isn't on your provider's list any more.")
		return
	case err != nil:
		writeError(w, http.StatusBadGateway, liveProviderMessage(err))
		return
	}
	s.live.mu.Lock()
	formats := s.live.status.Formats
	s.live.mu.Unlock()
	format := "ts"
	if len(formats) > 0 && !slices.Contains(formats, "ts") {
		format = "m3u8"
	}
	sess, err := s.timeshiftJoin(ch.ID, c.StreamURL(ch.ID, format), viewer, mins)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	// Wait for the first piece, so the player gets a playlist it can play.
	deadline := time.Now().Add(timeshiftFirst)
	ready := func() bool {
		b, err := os.ReadFile(filepath.Join(sess.dir, "index.m3u8"))
		return err == nil && bytes.Contains(b, []byte("#EXTINF"))
	}
	// When it doesn't come, the player plays the provider's own address:
	// this copy stops (if no one else watches), so it isn't a second
	// connection to the provider.
	giveUp := func() {
		if viewer != "" {
			s.shift.mu.Lock()
			s.timeshiftLeaveLocked(viewer, "")
			s.shift.mu.Unlock()
		}
	}
	for !ready() {
		select {
		case <-sess.done:
			if ready() {
				continue
			}
			giveUp()
			writeError(w, http.StatusBadGateway, "The channel didn't play.")
			return
		case <-r.Context().Done():
			giveUp()
			return
		case <-time.After(200 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			giveUp()
			writeError(w, http.StatusGatewayTimeout, "The channel is taking too long to start.")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"url": "/api/live/ts/" + sess.id + "/index.m3u8", "minutes": mins})
}

// DELETE /api/live/timeshift?v=<player>: the player stopped watching; a copy
// only it was watching stops now.
func (s *Server) handleLeaveTimeshift(w http.ResponseWriter, r *http.Request) {
	if v := r.URL.Query().Get("v"); v != "" {
		s.shift.mu.Lock()
		s.timeshiftLeaveLocked(v, "")
		s.shift.mu.Unlock()
	}
	w.WriteHeader(http.StatusNoContent)
}

// timeshiftLeaveLocked takes the player off every copy but the channel
// it's moving to, stopping the copies no one else is watching.
func (s *Server) timeshiftLeaveLocked(viewer, except string) {
	for _, ss := range s.shift.byID {
		if ss.channel == except {
			continue
		}
		if _, ok := ss.viewers[viewer]; !ok {
			continue
		}
		delete(ss.viewers, viewer)
		others := false
		for _, at := range ss.viewers {
			if time.Since(at) < timeshiftIdle {
				others = true
			}
		}
		if !others {
			s.timeshiftStopLocked(ss)
		}
	}
}

func (s *Server) timeshiftJoin(channel, link, viewer string, mins int) (*timeshiftSession, error) {
	st := &s.shift
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.byID == nil {
		st.byID = map[string]*timeshiftSession{}
		st.byChannel = map[string]*timeshiftSession{}
		_ = os.RemoveAll(s.timeshiftDir()) // pieces from before a restart
	}
	s.timeshiftJanitorLocked()
	if viewer != "" {
		s.timeshiftLeaveLocked(viewer, channel)
	}
	if ss := st.byChannel[channel]; ss != nil {
		ss.lastUsed = time.Now()
		if viewer != "" {
			ss.viewers[viewer] = time.Now()
		}
		return ss, nil
	}
	// Room for one more: the longest-unused copy makes way.
	if len(st.byID) >= timeshiftMax {
		var oldest *timeshiftSession
		for _, ss := range st.byID {
			if oldest == nil || ss.lastUsed.Before(oldest.lastUsed) {
				oldest = ss
			}
		}
		if oldest == nil || time.Since(oldest.lastUsed) < 15*time.Second {
			return nil, errors.New("Other channels are being watched on Cue right now. Try again in a moment.")
		}
		s.timeshiftStopLocked(oldest)
	}
	if err := os.MkdirAll(s.timeshiftDir(), 0o755); err != nil {
		return nil, errors.New("Couldn't make room to keep the channel.")
	}
	if u, err := fsinfo.DiskUsage(s.timeshiftDir()); err == nil && u.FreeBytes < timeshiftMinFree {
		return nil, errors.New("The server's disk is too full to keep the channel.")
	}
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	dir := filepath.Join(s.timeshiftDir(), id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, errors.New("Couldn't make room to keep the channel.")
	}
	ctx, cancel := context.WithCancel(context.Background())
	ss := &timeshiftSession{id: id, channel: channel, dir: dir, cancel: cancel, done: make(chan struct{}), lastUsed: time.Now(), viewers: map[string]time.Time{}}
	if viewer != "" {
		ss.viewers[viewer] = time.Now()
	}
	st.byID[id] = ss
	st.byChannel[channel] = ss
	go s.timeshiftRun(ctx, ss, link, mins)
	return ss, nil
}

// timeshiftRun copies the channel until it's stopped, starting again when
// the provider's stream breaks (the playlist carries on from where it was).
func (s *Server) timeshiftRun(ctx context.Context, ss *timeshiftSession, link string, mins int) {
	defer close(ss.done)
	ffmpeg, _ := ffmpegOnce()
	pieces := mins * 60 / timeshiftPiece
	quickFails := 0
	for ctx.Err() == nil {
		flags := "delete_segments+omit_endlist"
		if _, err := os.Stat(filepath.Join(ss.dir, "index.m3u8")); err == nil {
			flags += "+append_list+discont_start"
		}
		cmd := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-loglevel", "error", "-nostdin",
			"-reconnect", "1", "-reconnect_streamed", "1", "-reconnect_delay_max", "10",
			"-user_agent", "Cue", "-i", link,
			"-map", "0:v:0?", "-map", "0:a?", "-c", "copy", "-dn", "-sn",
			"-f", "hls", "-hls_time", strconv.Itoa(timeshiftPiece), "-hls_list_size", strconv.Itoa(pieces),
			"-hls_delete_threshold", "2", "-hls_flags", flags,
			"-hls_segment_filename", filepath.Join(ss.dir, "seg_%06d.ts"),
			filepath.Join(ss.dir, "index.m3u8"))
		var errOut bytes.Buffer
		cmd.Stderr = &errOut
		began := time.Now()
		err := cmd.Run()
		if ctx.Err() != nil {
			return
		}
		slog.Info("live tv: keeping a channel stopped; starting again", "err", err, "ffmpeg", redactURLText(firstLine(errOut.String()), link))
		if time.Since(began) < timeshiftQuickRun {
			quickFails++
			if quickFails > 3 {
				s.shift.mu.Lock()
				s.timeshiftStopLocked(ss)
				s.shift.mu.Unlock()
				return
			}
		} else {
			quickFails = 0
		}
		sleepCtx(ctx, 2*time.Second)
	}
}

// timeshiftStopLocked stops a copy and deletes its pieces.
func (s *Server) timeshiftStopLocked(ss *timeshiftSession) {
	ss.cancel()
	delete(s.shift.byID, ss.id)
	if s.shift.byChannel[ss.channel] == ss {
		delete(s.shift.byChannel, ss.channel)
	}
	go func() {
		select {
		case <-ss.done:
		case <-time.After(10 * time.Second):
		}
		_ = os.RemoveAll(ss.dir)
	}()
}

func (s *Server) timeshiftJanitorLocked() {
	if s.shift.janitor {
		return
	}
	s.shift.janitor = true
	go func() {
		for range time.Tick(20 * time.Second) {
			s.shift.mu.Lock()
			for _, ss := range s.shift.byID {
				if time.Since(ss.lastUsed) > timeshiftIdle {
					s.timeshiftStopLocked(ss)
				}
			}
			s.shift.mu.Unlock()
		}
	}()
}

// GET /api/live/ts/{id}/{file}: a copy's playlist or a piece. The id is long
// and random: it's the key, like a signed address (TVs fetch these without
// Cue's cookie).
func (s *Server) handleTimeshiftFile(w http.ResponseWriter, r *http.Request) {
	id, name := r.PathValue("id"), r.PathValue("file")
	if !timeshiftFile.MatchString(name) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	s.shift.mu.Lock()
	ss := s.shift.byID[id]
	if ss != nil {
		ss.lastUsed = time.Now()
	}
	s.shift.mu.Unlock()
	if ss == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	path := filepath.Join(ss.dir, name)
	if name == "index.m3u8" {
		b, err := os.ReadFile(path)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(b)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound) // gone behind the window
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "video/mp2t")
	w.Header().Set("Cache-Control", "max-age=3600")
	http.ServeContent(w, r, name, time.Time{}, f)
}

// timeshiftRunning is the channels being kept, for the owner's Settings.
func (s *Server) timeshiftRunning() []string {
	s.shift.mu.Lock()
	defer s.shift.mu.Unlock()
	var out []string
	for ch := range s.shift.byChannel {
		out = append(out, ch)
	}
	slices.Sort(out)
	return out
}
