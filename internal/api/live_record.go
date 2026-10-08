package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rdborg/mediarium/internal/fsinfo"
	"github.com/rdborg/mediarium/internal/watch"
)

// Live TV recording: Cue records a show on the server, from just before it
// starts to a couple of minutes after it ends, and keeps it to watch later.
// ffmpeg copies the provider's stream as it comes (a dropped stream is
// picked up again in a new piece); at the end the pieces are joined into an
// MP4 with AAC sound, which browsers and the TV app play and seek.
// Recording uses one of the provider's connections while it runs.

var (
	recordLate  = 2 * time.Minute  // kept this long after the show (tests shorten it)
	recordRetry = 10 * time.Second // before asking for a dropped stream again
)

const (
	recordEarly      = 60 * time.Second // started this long before the show
	recordKeep       = 30 * 24 * time.Hour
	recordMinFree    = 3 << 30 // no recording starts with less free space than this
	recordSealPrefix = "cue-rec:"
)

type recorderState struct {
	mu      sync.Mutex
	running map[int64]context.CancelFunc
	started bool // interrupted recordings were looked at
}

func (s *Server) recordingsDir() string { return filepath.Join(s.cfg.ConfigDir, "recordings") }

// recordingsJob starts the recordings that are due, settles ones Cue missed
// or was stopped during, and clears out old ones. Runs every 20 seconds.
func (s *Server) recordingsJob(ctx context.Context) {
	now := time.Now()
	s.recorder.mu.Lock()
	first := !s.recorder.started
	s.recorder.started = true
	if s.recorder.running == nil {
		s.recorder.running = map[int64]context.CancelFunc{}
	}
	s.recorder.mu.Unlock()

	if first {
		// Cue stopped during these: carry on if the show is still on (a new
		// piece), else finish with what was recorded.
		if list, err := s.WatchRepo.InterruptedRecordings(); err == nil {
			for _, rec := range list {
				if rec.Stop.Add(recordLate).After(now) {
					_ = s.WatchRepo.SetRecordingState(rec.ID, watch.RecScheduled, "", 0, "")
				} else {
					s.finishRecording(context.WithoutCancel(ctx), rec)
				}
			}
		}
	}
	if list, err := s.WatchRepo.MissedRecordings(now); err == nil {
		for _, rec := range list {
			_ = s.WatchRepo.SetRecordingState(rec.ID, watch.RecFailed, "", 0, "Cue wasn't running while it was on.")
		}
	}
	due, err := s.WatchRepo.RecordingsDue(now.Add(recordEarly), now)
	if err != nil {
		slog.Warn("live tv: recordings due", "err", err)
		return
	}
	for _, rec := range due {
		s.recorder.mu.Lock()
		_, busy := s.recorder.running[rec.ID]
		var rctx context.Context
		if !busy {
			var cancel context.CancelFunc
			rctx, cancel = context.WithCancel(context.Background())
			s.recorder.running[rec.ID] = cancel
		}
		s.recorder.mu.Unlock()
		if !busy {
			go s.record(rctx, rec)
		}
	}
	// Old ones go, to keep the server's disk from filling.
	if list, err := s.WatchRepo.Recordings(); err == nil {
		for _, rec := range list {
			if (rec.Status == watch.RecDone || rec.Status == watch.RecFailed) && now.Sub(rec.Stop) > recordKeep {
				s.removeRecording(rec)
			}
		}
	}
}

// record records one show, in pieces when the stream drops, then finishes it.
func (s *Server) record(ctx context.Context, rec watch.Recording) {
	defer func() {
		s.recorder.mu.Lock()
		delete(s.recorder.running, rec.ID)
		s.recorder.mu.Unlock()
	}()
	if err := os.MkdirAll(s.recordingsDir(), 0o755); err != nil {
		_ = s.WatchRepo.SetRecordingState(rec.ID, watch.RecFailed, "", 0, "Couldn't make the recordings folder.")
		return
	}
	if u, err := fsinfo.DiskUsage(s.recordingsDir()); err == nil && u.FreeBytes < recordMinFree {
		_ = s.WatchRepo.SetRecordingState(rec.ID, watch.RecFailed, "", 0, "Not enough free space on the server.")
		return
	}
	if !canConvert() {
		_ = s.WatchRepo.SetRecordingState(rec.ID, watch.RecFailed, "", 0, "This server can't record (ffmpeg isn't installed).")
		return
	}
	_ = s.WatchRepo.SetRecordingState(rec.ID, watch.RecRecording, "", 0, "")
	slog.Info("live tv: recording", "title", rec.Title, "channel", rec.Channel)

	end := rec.Stop.Add(recordLate)
	quickFails := 0
	for ctx.Err() == nil && time.Until(end) > 5*time.Second {
		ch, c, err := s.liveChannelByID(ctx, rec.StreamID)
		if err != nil {
			slog.Info("live tv: recording: the channel", "title", rec.Title, "err", err)
			quickFails++
			if quickFails > 6 {
				break
			}
			sleepCtx(ctx, recordRetry)
			continue
		}
		s.live.mu.Lock()
		formats := s.live.status.Formats
		s.live.mu.Unlock()
		format := "ts"
		if len(formats) > 0 && !slices.Contains(formats, "ts") {
			format = "m3u8"
		}
		link := c.StreamURL(ch.ID, format)
		part := filepath.Join(s.recordingsDir(), fmt.Sprintf("rec-%d-%d.ts", rec.ID, time.Now().UnixNano()))
		secs := int(time.Until(end).Seconds())
		ffmpeg, _ := ffmpegOnce()
		cmd := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-loglevel", "error", "-nostdin",
			"-reconnect", "1", "-reconnect_streamed", "1", "-reconnect_delay_max", "10",
			"-user_agent", "Cue", "-i", link,
			"-map", "0:v:0?", "-map", "0:a?", "-c", "copy", "-dn", "-sn",
			"-t", strconv.Itoa(secs), "-f", "mpegts", part)
		var errOut bytes.Buffer
		cmd.Stderr = &errOut
		began := time.Now()
		err = cmd.Run()
		if err != nil && ctx.Err() == nil {
			slog.Info("live tv: recording stopped; starting again", "title", rec.Title, "ffmpeg", redactURLText(firstLine(errOut.String()), link))
		}
		if time.Since(began) < 30*time.Second {
			quickFails++
			if quickFails > 6 {
				break
			}
			sleepCtx(ctx, recordRetry)
		} else {
			quickFails = 0
		}
	}
	if ctx.Err() != nil {
		return // deleted while recording: removeRecording clears the pieces
	}
	s.finishRecording(context.Background(), rec)
}

func sleepCtx(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

// recordingParts is a recording's pieces, in order.
func (s *Server) recordingParts(id int64) []string {
	parts, _ := filepath.Glob(filepath.Join(s.recordingsDir(), fmt.Sprintf("rec-%d-*.ts", id)))
	sort.Strings(parts)
	return parts
}

// finishRecording joins the pieces into the MP4 (picture copied, sound made
// AAC) and marks it done, or failed when nothing came.
func (s *Server) finishRecording(ctx context.Context, rec watch.Recording) {
	parts := s.recordingParts(rec.ID)
	var total int64
	var keep []string
	for _, p := range parts {
		if st, err := os.Stat(p); err == nil && st.Size() > 32<<10 {
			total += st.Size()
			keep = append(keep, p)
		} else {
			_ = os.Remove(p)
		}
	}
	if len(keep) == 0 {
		_ = s.WatchRepo.SetRecordingState(rec.ID, watch.RecFailed, "", 0, "The channel wouldn't play, so nothing was recorded.")
		return
	}
	out := filepath.Join(s.recordingsDir(), fmt.Sprintf("recording-%d.mp4", rec.ID))
	ffmpeg, _ := ffmpegOnce()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Hour)
	defer cancel()
	cmd := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-loglevel", "error", "-nostdin", "-y",
		"-i", "concat:"+strings.Join(keep, "|"),
		"-map", "0:v:0?", "-map", "0:a:0?", "-c:v", "copy", "-c:a", "aac", "-b:a", "192k",
		"-movflags", "+faststart", out)
	var errOut bytes.Buffer
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		slog.Warn("live tv: finish recording", "title", rec.Title, "err", err, "ffmpeg", firstLine(errOut.String()))
		_ = os.Remove(out)
		_ = s.WatchRepo.SetRecordingState(rec.ID, watch.RecFailed, "", 0, "The recording couldn't be put together.")
		return
	}
	for _, p := range keep {
		_ = os.Remove(p)
	}
	var size int64
	if st, err := os.Stat(out); err == nil {
		size = st.Size()
	}
	_ = s.WatchRepo.SetRecordingState(rec.ID, watch.RecDone, filepath.Base(out), size, "")
	slog.Info("live tv: recorded", "title", rec.Title, "bytes", size)
}

// removeRecording stops it if it's running and deletes it with its files.
func (s *Server) removeRecording(rec watch.Recording) {
	s.recorder.mu.Lock()
	if cancel, ok := s.recorder.running[rec.ID]; ok {
		cancel()
		delete(s.recorder.running, rec.ID)
	}
	s.recorder.mu.Unlock()
	time.Sleep(200 * time.Millisecond) // let ffmpeg let go of its piece
	for _, p := range s.recordingParts(rec.ID) {
		_ = os.Remove(p)
	}
	if rec.File != "" {
		_ = os.Remove(filepath.Join(s.recordingsDir(), filepath.Base(rec.File)))
	}
	_ = s.WatchRepo.DeleteRecording(rec.ID)
}

// ---- The API

type recordingView struct {
	watch.Recording
	Logo string `json:"logo,omitempty"`
}

// GET /api/live/recordings: every recording, newest show first.
func (s *Server) handleLiveRecordings(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.watchProfile(w, r); !ok {
		return
	}
	list, err := s.WatchRepo.Recordings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't load the recordings.")
		return
	}
	s.live.mu.Lock()
	byID := s.live.byID
	s.live.mu.Unlock()
	out := make([]recordingView, 0, len(list))
	for _, rec := range list {
		v := recordingView{Recording: rec}
		if ch, ok := byID[rec.StreamID]; ok && ch.Logo != "" {
			v.Logo = "/api/live/logo/" + url.PathEscape(ch.ID)
		}
		out = append(out, v)
	}
	writeJSON(w, http.StatusOK, out)
}

// PUT /api/live/recordings {channelId, start, stop, title}: record a show
// (one on now starts at once).
func (s *Server) handleAddLiveRecording(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	var req watch.LiveReminder
	if err := decodeJSON(r, &req); err != nil || req.StreamID == "" || len(req.StreamID) > 64 || req.Start.IsZero() || !req.Stop.After(req.Start) {
		writeError(w, http.StatusBadRequest, "Which show? (channelId, start, stop and title)")
		return
	}
	if req.Stop.Before(time.Now()) {
		writeError(w, http.StatusBadRequest, "That show has ended.")
		return
	}
	if req.Stop.Sub(req.Start) > 6*time.Hour {
		writeError(w, http.StatusBadRequest, "Recordings can be up to 6 hours long.")
		return
	}
	if !canConvert() {
		writeError(w, http.StatusNotImplemented, "This server can't record (ffmpeg isn't installed).")
		return
	}
	s.live.mu.Lock()
	name := s.live.byID[req.StreamID].Name
	s.live.mu.Unlock()
	title := req.Title
	if len(title) > 300 {
		title = title[:300]
	}
	rec, err := s.WatchRepo.AddRecording(watch.Recording{ProfileID: pid, StreamID: req.StreamID, Channel: name, Title: title, Start: req.Start, Stop: req.Stop})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't schedule the recording.")
		return
	}
	go s.recordingsJob(context.Background()) // one on now starts straight away
	writeJSON(w, http.StatusOK, rec)
}

// DELETE /api/live/recordings/{id}: cancel or delete a recording.
func (s *Server) handleDeleteLiveRecording(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.watchProfile(w, r); !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Which recording?")
		return
	}
	rec, err := s.WatchRepo.Recording(id)
	if errors.Is(err, watch.ErrRecordingNotFound) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't delete the recording.")
		return
	}
	s.removeRecording(rec)
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/live/recordings/{id}/play: where a finished recording plays (a
// signed address, so the TV app's player can open it).
func (s *Server) handlePlayLiveRecording(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.watchProfile(w, r); !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Which recording?")
		return
	}
	rec, err := s.WatchRepo.Recording(id)
	if err != nil || rec.Status != watch.RecDone || rec.File == "" {
		writeError(w, http.StatusNotFound, "That recording isn't ready.")
		return
	}
	sealed, err := s.profileBox.Encrypt(fmt.Sprintf("%s%s|%d", recordSealPrefix, rec.File, time.Now().Add(12*time.Hour).Unix()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't start the recording.")
		return
	}
	link := "/api/recording?t=" + url.QueryEscape(sealed)
	writeJSON(w, http.StatusOK, map[string]any{
		"url": link, "title": rec.Title, "channel": rec.Channel, "start": rec.Start,
		// For a browser that can't play it as it is (MPEG-2 pictures):
		// ffmpeg reads the file itself.
		"convert": s.convertToken(filepath.Join(s.recordingsDir(), rec.File)),
	})
}

// GET /api/recording?t=<signed>: a finished recording's file, with seeking.
func (s *Server) handleRecordingFile(w http.ResponseWriter, r *http.Request) {
	plain, err := s.profileBox.Decrypt(r.URL.Query().Get("t"))
	if err != nil || !strings.HasPrefix(plain, recordSealPrefix) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	name, exp, _ := strings.Cut(strings.TrimPrefix(plain, recordSealPrefix), "|")
	until, _ := strconv.ParseInt(exp, 10, 64)
	if time.Now().Unix() > until || name != filepath.Base(name) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	f, err := os.Open(filepath.Join(s.recordingsDir(), name))
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeContent(w, r, name, st.ModTime(), f)
}
