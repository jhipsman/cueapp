package api

import (
	"log"
	"log/slog"
	"net/http"

	"github.com/jhipsman/cueapp/internal/auth"
)

// An account's own settings, kept on the server so they follow it to every
// device and browser (the web app's Settings page). Only known keys, each
// with its allowed values, are kept.
var accountPrefKeys = map[string][]string{
	"tourSeen":     {"0", "1"}, // the welcome tour was shown (once per account, not per browser)
	"autoplayNext": {"0", "1"}, // the next episode starts by itself after a countdown
	"previews":     {"0", "1"}, // moving previews on Home and title pages
	"previewSound": {"0", "1"}, // a title page's preview plays with sound
}

// GET /api/me/prefs: {"key":"value"} for the account's settings that are set.
func (s *Server) handleGetAccountPrefs(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	rows, err := s.db.Query(`SELECT key, value FROM account_prefs WHERE user_id = ?`, u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't read your settings.")
		return
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if rows.Scan(&k, &v) == nil {
			out[k] = v
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// PUT /api/me/prefs {"key":"value",...}: saves some of the account's
// settings; answers them all.
func (s *Server) handlePutAccountPrefs(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	var req map[string]string
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Couldn't read that request.")
		return
	}
	for k, v := range req {
		allowed, ok := accountPrefKeys[k]
		if !ok || !contains(allowed, v) {
			writeError(w, http.StatusBadRequest, "That isn't a setting Cue knows.")
			return
		}
	}
	for k, v := range req {
		if _, err := s.db.Exec(`INSERT INTO account_prefs (user_id, key, value) VALUES (?, ?, ?)
			ON CONFLICT (user_id, key) DO UPDATE SET value = excluded.value`, u.ID, k, v); err != nil {
			log.Printf("api: account prefs: %v", err)
			writeError(w, http.StatusInternalServerError, "Couldn't save that. Try again.")
			return
		}
	}
	s.handleGetAccountPrefs(w, r)
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// DELETE /api/watch/history: the active profile's Continue Watching and
// what it has watched, forgotten.
func (s *Server) handleClearWatchHistory(w http.ResponseWriter, r *http.Request) {
	p, ok := s.activeProfile(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "Pick a profile first.")
		return
	}
	if err := s.WatchRepo.ClearProgress(p.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't clear it. Try again.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// POST /api/auth/sign-out-others: signs the account out on every other
// device and browser (this one stays signed in).
func (s *Server) handleSignOutOthers(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	token, ok := auth.SessionToken(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "This device isn't signed in with a session.")
		return
	}
	n, err := s.Auth.DeleteOtherSessions(u.ID, token)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't sign the other devices out. Try again.")
		return
	}
	slog.Info("auth: signed out other devices", "user_id", u.ID, "sessions", n)
	writeJSON(w, http.StatusOK, map[string]int64{"signedOut": n})
}
