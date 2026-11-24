package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jhipsman/cueapp/internal/crashes"
	"github.com/jhipsman/cueapp/internal/settings"
)

// maxCrashTrace caps how much of a crash report is kept: the top of a stack
// trace says where it broke, and the cap stops one report filling the log.
const maxCrashTrace = crashes.MaxTrace

// crashAlertsPerHour caps the phone alerts about crashes, so a burst of
// different crashes (or a device sending nonsense) can't flood a phone.
const crashAlertsPerHour = 5

var crashAlertBudget struct {
	sync.Mutex
	hour time.Time
	sent int
}

// POST /api/play/crash {app, version, device, trace}: the TV app crashed the
// last time it ran, or a page of the web app failed to draw. It's written to
// the server log ("tv app crashed"), kept under Settings > System > Problems,
// and a crash not seen before alerts the administrators' phones.
func (s *Server) handleTVCrash(w http.ResponseWriter, r *http.Request) {
	var req struct {
		App     string `json:"app"`
		Version string `json:"version"`
		Device  string `json:"device"`
		Trace   string `json:"trace"`
	}
	if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.Trace) == "" {
		writeError(w, http.StatusBadRequest, "That isn't a crash report.")
		return
	}
	clip := func(v string, n int) string {
		if len(v) > n {
			return v[:n]
		}
		return v
	}
	app := strings.ToLower(clip(strings.TrimSpace(req.App), 20))
	if app != "web" {
		app = "tv"
	}
	if app == "web" && strings.TrimSpace(req.Version) == "" {
		req.Version = s.version // the website comes with the server
	}
	slog.Error(app+" app crashed",
		"app", app,
		"version", clip(req.Version, 40),
		"device", clip(req.Device, 120),
		"trace", clip(req.Trace, maxCrashTrace))
	if s.Crashes != nil {
		c, news, err := s.Crashes.Record(crashes.Report{App: app, Version: req.Version, Device: req.Device, Trace: req.Trace})
		if err != nil {
			slog.Warn("crash report not kept", "err", err)
		} else if news {
			s.crashAlert(c)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// crashAlertsOn reports the "tell my phone when an app crashes" switch (on
// unless switched off).
func (s *Server) crashAlertsOn() bool {
	v, _ := s.Settings.Get(settings.KeyCrashAlerts)
	return v != "0"
}

// crashTitle is how an alert names the app.
func crashTitle(app string) string {
	if app == "web" {
		return "Cue's website hit a crash"
	}
	return "Cue for TV crashed"
}

// crashAlert tells the administrators' phones (those with alerts on) about a
// crash not seen before, or one marked fixed that came back, and the
// notification targets that listen for Health when "tell me about errors" is on.
func (s *Server) crashAlert(c crashes.Crash) {
	if s.Settings == nil || !s.crashAlertsOn() || !takeCrashAlert(time.Now()) {
		return
	}
	title := crashTitle(c.App)
	body := c.Headline
	if len(body) > 140 {
		body = body[:140] + "…"
	}
	if c.Device != "" {
		body += "\nOn " + c.Device + "."
	}
	if c.Count > 1 {
		body += " It's back after being marked fixed."
	}
	body += " Open Problems in Settings to copy it for Claude."
	if s.notifyOnProblems() {
		s.notifyEvent("health", title, body)
	}
	if s.WatchRepo == nil || s.db == nil {
		return
	}
	rows, err := s.db.Query(`SELECT DISTINCT p.id FROM profiles p JOIN users u ON u.id = p.user_id
		JOIN push_subs ps ON ps.profile_id = p.id WHERE u.is_admin = 1`)
	if err != nil {
		slog.Info("crash alert: find phones", "err", err)
		return
	}
	var profiles []int64
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			profiles = append(profiles, id)
		}
	}
	rows.Close()
	key := fmt.Sprintf("crash:%d:%s", c.ID, c.LastAt.UTC().Format(time.RFC3339))
	s.background(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		for _, pid := range profiles {
			if queued, err := s.WatchRepo.QueueAlert(pid, key, title, body, "/settings/problems"); err == nil && queued {
				s.wakeProfile(ctx, pid)
			}
		}
	})
}

// takeCrashAlert says whether another crash alert may go this hour.
func takeCrashAlert(now time.Time) bool {
	crashAlertBudget.Lock()
	defer crashAlertBudget.Unlock()
	hour := now.Truncate(time.Hour)
	if !hour.Equal(crashAlertBudget.hour) {
		crashAlertBudget.hour, crashAlertBudget.sent = hour, 0
	}
	if crashAlertBudget.sent >= crashAlertsPerHour {
		return false
	}
	crashAlertBudget.sent++
	return true
}

// GET /api/system/crashes: the kept crashes, open ones first.
func (s *Server) handleCrashes(w http.ResponseWriter, r *http.Request) {
	list, err := s.Crashes.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't read the crash reports.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":         list,
		"open":          s.Crashes.OpenCount(),
		"alerts":        s.crashAlertsOn(),
		"serverVersion": s.version,
	})
}

func crashID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "That isn't a crash.")
		return 0, false
	}
	return id, true
}

// POST /api/system/crashes/{id}/fixed {"fixed": true|false}: mark a crash
// fixed (it alerts again if it comes back), or open again.
func (s *Server) handleCrashFixed(w http.ResponseWriter, r *http.Request) {
	id, ok := crashID(w, r)
	if !ok {
		return
	}
	var req struct {
		Fixed bool `json:"fixed"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Couldn't read that.")
		return
	}
	if err := s.Crashes.SetFixed(id, req.Fixed); errors.Is(err, crashes.ErrNotFound) {
		writeError(w, http.StatusNotFound, "That crash isn't kept any more.")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't save that.")
		return
	}
	c, _ := s.Crashes.Get(id)
	writeJSON(w, http.StatusOK, c)
}

// DELETE /api/system/crashes/{id}: forget a crash.
func (s *Server) handleCrashDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := crashID(w, r)
	if !ok {
		return
	}
	if err := s.Crashes.Delete(id); errors.Is(err, crashes.ErrNotFound) {
		writeError(w, http.StatusNotFound, "That crash isn't kept any more.")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't remove it.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PUT /api/system/crashes/alerts {"enabled"}: phone alerts about new crashes on or off.
func (s *Server) handleCrashAlerts(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Couldn't read that.")
		return
	}
	value := "0"
	if req.Enabled {
		value = "1"
	}
	if err := s.Settings.Set(settings.KeyCrashAlerts, value, false); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't save that.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"alerts": req.Enabled})
}
