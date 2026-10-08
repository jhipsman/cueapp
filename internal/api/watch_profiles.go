package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/rdborg/mediarium/internal/auth"
	"github.com/rdborg/mediarium/internal/watch"
)

// Watch profiles: one household account, a profile per person, picked on a
// "Who's watching?" screen. Each profile has its own progress, Continue
// Watching and My List. Picking one gives the device a token (sent back in the
// X-Cue-Profile header); a profile with a PIN gives it only for the right PIN.
// In a household with more than one profile, only the main profile (the
// account owner's) may change settings or manage profiles.

const (
	profileHeader      = "X-Cue-Profile"
	mainProfileMessage = "Only the main profile can change settings. Switch to it first."
	pickProfileMessage = "Choose who's watching first."
)

// profileToken proves the device picked p (and knew its PIN). It names the
// profile's secret, which changes with the PIN, so changing the PIN signs
// every device out of the profile.
func (s *Server) profileToken(p watch.Profile) (string, error) {
	return s.profileBox.Encrypt(fmt.Sprintf("cue-profile:%d:%d:%s", p.UserID, p.ID, p.Secret()))
}

// householdProfiles is the signed-in account's profiles, main first.
func (s *Server) householdProfiles(u *auth.User) ([]watch.Profile, error) {
	name := strings.TrimSpace(u.FirstName) // a new main profile is named after the owner
	if name == "" {
		name = u.Username
	}
	return s.WatchRepo.Profiles(u.ID, name)
}

// activeProfile is the profile r's device picked, if its token is good. A
// household with one profile and no PIN needs no token: that one is used.
func (s *Server) activeProfile(r *http.Request) (watch.Profile, bool) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		return watch.Profile{}, false
	}
	list, err := s.householdProfiles(u)
	if err != nil || len(list) == 0 {
		return watch.Profile{}, false
	}
	if tok := strings.TrimSpace(r.Header.Get(profileHeader)); tok != "" {
		if plain, err := s.profileBox.Decrypt(tok); err == nil {
			parts := strings.Split(plain, ":")
			if len(parts) == 4 && parts[0] == "cue-profile" && parts[1] == strconv.FormatInt(u.ID, 10) {
				for _, p := range list {
					if strconv.FormatInt(p.ID, 10) == parts[2] && p.Secret() == parts[3] {
						return p, true
					}
				}
			}
		}
	}
	if len(list) == 1 && !list[0].HasPIN {
		return list[0], true
	}
	return watch.Profile{}, false
}

// mainProfileActive is the settings lock: true unless the household has
// several profiles and the device isn't on the main one.
func (s *Server) mainProfileActive(r *http.Request) bool {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		return true // the role check answers this one
	}
	list, err := s.householdProfiles(u)
	if err != nil {
		slog.Warn("profiles: read for the settings lock", "err", err)
		return false
	}
	if len(list) <= 1 {
		return true
	}
	p, ok := s.activeProfile(r)
	return ok && p.Main
}

// watchProfile is the profile a Watch request is for, or a 409 asking the
// device to pick one.
func (s *Server) watchProfile(w http.ResponseWriter, r *http.Request) (int64, bool) {
	if auth.UserFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, "You need to sign in first.")
		return 0, false
	}
	p, ok := s.activeProfile(r)
	if !ok {
		writeError(w, http.StatusConflict, pickProfileMessage)
		return 0, false
	}
	return p.ID, true
}

type profileView struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	HasPIN bool   `json:"hasPin"`
	Main   bool   `json:"main"`
	// Theme is the profile's colors in Watch.
	Theme watch.Theme `json:"theme"`
}

func viewWatchProfile(p watch.Profile) profileView {
	return profileView{ID: p.ID, Name: p.Name, Avatar: p.Avatar, HasPIN: p.HasPIN, Main: p.Main, Theme: p.Theme}
}

// PUT /api/profile/theme {"accent","glow","background"}: the colors of the
// profile this device is on. Anyone can change their own.
func (s *Server) handlePutProfileTheme(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.watchProfile(w, r); !ok {
		return
	}
	p, _ := s.activeProfile(r)
	var t watch.Theme
	if err := decodeJSON(r, &t); err != nil {
		writeError(w, http.StatusBadRequest, "That request couldn't be read.")
		return
	}
	saved, err := s.WatchRepo.SetTheme(p.UserID, p.ID, t)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't save the theme.")
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

type profilesPayload struct {
	Profiles []profileView `json:"profiles"`
	Active   int64         `json:"active,omitempty"` // the one this device is on; 0 = none yet
	Avatars  []string      `json:"avatars"`
	Max      int           `json:"max"`
	LiveTV   bool          `json:"liveTV"` // an IPTV provider is set up (live.go)
}

// GET /api/profiles: who can be picked, and which one this device is on.
func (s *Server) handleListWatchProfiles(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	list, err := s.householdProfiles(u)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't load the profiles.")
		return
	}
	_, live := s.iptvAccount()
	out := profilesPayload{Profiles: []profileView{}, Avatars: watch.AvatarColors, Max: watch.MaxProfiles, LiveTV: live}
	for _, p := range list {
		out.Profiles = append(out.Profiles, viewWatchProfile(p))
	}
	if p, ok := s.activeProfile(r); ok {
		out.Active = p.ID
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /api/profiles/{id}/select {"pin": "1234"}: picks a profile on this
// device and answers with its token.
func (s *Server) handleSelectWatchProfile(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "That isn't a valid profile.")
		return
	}
	var req struct {
		PIN string `json:"pin"`
	}
	_ = decodeJSON(r, &req)
	p, err := s.WatchRepo.Profile(u.ID, id)
	if errors.Is(err, watch.ErrProfileNotFound) {
		writeError(w, http.StatusNotFound, "That profile doesn't exist any more.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't load the profile.")
		return
	}
	if p.HasPIN {
		// Four digits are quick to guess: wrong PINs count like wrong passwords.
		key := fmt.Sprintf("pin:%d:%d", u.ID, p.ID)
		if !s.LoginLimiter.Allow(key) {
			writeError(w, http.StatusTooManyRequests, "Too many wrong PINs. Wait a few minutes and try again.")
			return
		}
		if !p.CheckPIN(req.PIN) {
			s.LoginLimiter.RecordFailure(key)
			writeError(w, http.StatusForbidden, "That PIN isn't right.")
			return
		}
		s.LoginLimiter.RecordSuccess(key)
	}
	tok, err := s.profileToken(p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't open the profile.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": tok, "profile": viewWatchProfile(p)})
}

func writeWatchProfileError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, watch.ErrProfileNotFound):
		writeError(w, http.StatusNotFound, "That profile doesn't exist any more.")
	case errors.Is(err, watch.ErrBadName), errors.Is(err, watch.ErrBadPIN), errors.Is(err, watch.ErrMainProfile), errors.Is(err, watch.ErrTooManyProfiles):
		msg := err.Error()
		writeError(w, http.StatusBadRequest, strings.ToUpper(msg[:1])+msg[1:]+".")
	default:
		slog.Warn("profiles: change", "err", err)
		writeError(w, http.StatusInternalServerError, "Couldn't save the profile.")
	}
}

// POST /api/profiles {"name","avatar"} (main profile only)
func (s *Server) handleAddWatchProfile(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	var req struct {
		Name   string `json:"name"`
		Avatar string `json:"avatar"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "That request couldn't be read.")
		return
	}
	p, err := s.WatchRepo.AddProfile(u.ID, req.Name, req.Avatar)
	if err != nil {
		writeWatchProfileError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, viewWatchProfile(p))
}

// PUT /api/profiles/{id} {"name"?, "avatar"?, "pin"?} (main profile only;
// "pin": "" removes it)
func (s *Server) handleUpdateWatchProfile(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "That isn't a valid profile.")
		return
	}
	var req struct {
		Name   *string `json:"name"`
		Avatar *string `json:"avatar"`
		PIN    *string `json:"pin"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "That request couldn't be read.")
		return
	}
	p, err := s.WatchRepo.UpdateProfile(u.ID, id, watch.ProfileChange{Name: req.Name, Avatar: req.Avatar, PIN: req.PIN})
	if err != nil {
		writeWatchProfileError(w, err)
		return
	}
	out := map[string]any{"profile": viewWatchProfile(p)}
	// A new PIN signs every device out of the profile, this one included:
	// hand this device a fresh token for it.
	if req.PIN != nil {
		if tok, err := s.profileToken(p); err == nil {
			out["token"] = tok
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// DELETE /api/profiles/{id} (main profile only; not the main profile)
func (s *Server) handleRemoveWatchProfile(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "That isn't a valid profile.")
		return
	}
	if err := s.WatchRepo.RemoveProfile(u.ID, id); err != nil {
		writeWatchProfileError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
