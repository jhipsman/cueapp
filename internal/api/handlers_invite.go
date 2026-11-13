package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jhipsman/cueapp/internal/auth"
	"github.com/jhipsman/cueapp/internal/httpsec"
	"github.com/jhipsman/cueapp/internal/settings"
)

// The family invite: an administrator makes a link (/join/{token}) and
// sends it round; whoever opens it makes their own account, a member's
// that watches with this Cue's streaming (Premiumize, add-ons, live TV)
// and can't see or change its settings. One link at a time, for seven days;
// making a new one ends the old, and it can be turned off.

const inviteLife = 7 * 24 * time.Hour

// What an account made from the invite may do: watch, and subtitles; titles
// it wants become requests rather than additions.
func invitePermissions() auth.Permissions {
	return auth.Permissions{Movies: true, TV: true, Subtitles: true, Play: true}
}

type inviteState struct {
	Token   string    `json:"token,omitempty"`
	Expires time.Time `json:"expires,omitzero"`
}

func (s *Server) invite() (inviteState, bool) {
	tok, _ := s.Settings.Get(settings.KeyInviteToken)
	exp, _ := s.Settings.Get(settings.KeyInviteExpires)
	sec, _ := strconv.ParseInt(exp, 10, 64)
	if tok == "" || sec == 0 || time.Now().After(time.Unix(sec, 0)) {
		return inviteState{}, false
	}
	return inviteState{Token: tok, Expires: time.Unix(sec, 0).UTC()}, true
}

// GET /api/users/invite: the invite link's secret and end, or {} when off.
func (s *Server) handleGetInvite(w http.ResponseWriter, r *http.Request) {
	st, _ := s.invite()
	writeJSON(w, http.StatusOK, st)
}

// POST /api/users/invite: a new link (the old one stops working).
func (s *Server) handlePostInvite(w http.ResponseWriter, r *http.Request) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't make an invite. Try again.")
		return
	}
	tok := base64.RawURLEncoding.EncodeToString(b)
	exp := time.Now().Add(inviteLife)
	if err := s.Settings.Set(settings.KeyInviteToken, tok, true); err != nil {
		log.Printf("api: invite: %v", err)
		writeError(w, http.StatusInternalServerError, "Couldn't make an invite. Try again.")
		return
	}
	if err := s.Settings.Set(settings.KeyInviteExpires, strconv.FormatInt(exp.Unix(), 10), false); err != nil {
		log.Printf("api: invite: %v", err)
		writeError(w, http.StatusInternalServerError, "Couldn't make an invite. Try again.")
		return
	}
	writeJSON(w, http.StatusOK, inviteState{Token: tok, Expires: exp.UTC()})
}

// DELETE /api/users/invite: the link stops working.
func (s *Server) handleDeleteInvite(w http.ResponseWriter, r *http.Request) {
	if err := s.Settings.Set(settings.KeyInviteToken, "", false); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't turn the invite off. Try again.")
		return
	}
	writeJSON(w, http.StatusOK, inviteState{})
}

// joinAllowed checks the token in the address against the invite, counting
// wrong ones like failed sign-ins.
func (s *Server) joinAllowed(w http.ResponseWriter, r *http.Request) bool {
	ipKey := httpsec.LimitKey(s.clientIP(r))
	if !s.LoginLimiter.Allow(ipKey) {
		writeError(w, http.StatusTooManyRequests, "Too many tries. Wait a few minutes and try again.")
		return false
	}
	st, ok := s.invite()
	given := r.PathValue("token")
	if !ok || given == "" || subtle.ConstantTimeCompare([]byte(given), []byte(st.Token)) != 1 {
		s.LoginLimiter.RecordFailure(ipKey)
		writeError(w, http.StatusNotFound, "This invite link has ended or been turned off. Ask for a new one.")
		return false
	}
	return true
}

// GET /api/join/{token}: whether the invite still works.
func (s *Server) handleCheckJoin(w http.ResponseWriter, r *http.Request) {
	if !s.joinAllowed(w, r) {
		return
	}
	st, _ := s.invite()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "expires": st.Expires})
}

// POST /api/join/{token} {"name","username","password"}: a new member
// account from the invite, signed in.
func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	if !s.joinAllowed(w, r) {
		return
	}
	var req struct {
		Name     string `json:"name"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxPublicBody)
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Couldn't read that request. Reload the page and try again.")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	first, last := splitName(req.Name, "", "")
	if rejectBad(w, checkUsername(req.Username), checkPassword(req.Password, "Password"), checkMaxLen(first, "Name", nameMax)) {
		return
	}
	created, err := s.Auth.CreateAccount(req.Username, req.Password, first, last, "", false)
	if err != nil {
		writeAccountError(w, err)
		return
	}
	if err := s.Auth.SetPermissions(created.ID, invitePermissions()); err != nil {
		// Without its narrower permissions the account would have a full
		// member's: take it away rather than leave that.
		_ = s.Auth.DeleteAccount(created.ID)
		log.Printf("api: join: %v", err)
		writeError(w, http.StatusInternalServerError, "Couldn't make your account. Try again.")
		return
	}
	if err := s.Auth.RecordLogin(created.ID); err != nil && !errors.Is(err, auth.ErrUserNotFound) {
		log.Printf("api: join: %v", err)
	}
	s.startSession(w, r, created.ID)
	writeJSON(w, http.StatusCreated, userPayload(&created.User))
}
