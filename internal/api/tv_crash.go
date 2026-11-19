package api

import (
	"log/slog"
	"net/http"
	"strings"
)

// maxCrashTrace caps how much of a crash report is kept: the top of a stack
// trace says where it broke, and the cap stops one report filling the log.
const maxCrashTrace = 16 << 10

// POST /api/play/crash {app, version, device, trace}: the TV app crashed the
// last time it ran. It's written to the server log ("tv app crashed") so the
// owner can find where it broke: docker compose logs cue | grep crashed.
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
	slog.Error("tv app crashed",
		"app", clip(req.App, 40),
		"version", clip(req.Version, 40),
		"device", clip(req.Device, 120),
		"trace", clip(req.Trace, maxCrashTrace))
	w.WriteHeader(http.StatusNoContent)
}
