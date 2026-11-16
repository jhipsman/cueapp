package api

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jhipsman/cueapp/internal/auth"
	"github.com/jhipsman/cueapp/internal/httpsec"
	"rsc.io/qr"
)

// Two-step sign-in: after the password, a six-digit code from an
// authenticator app (or one of ten one-time backup codes). Each account
// turns it on for itself in its profile; the secret is kept encrypted.
// Signing a TV in from a phone needs no code: the phone is already signed in.

const (
	twoStepTicketLife = 5 * time.Minute
	twoStepTries      = 5 // wrong codes per sign-in before starting over
	twoStepSetupLife  = 15 * time.Minute
	twoStepIssuer     = "Cue"
)

// A sign-in waiting for its code: made by a right password, spent by a
// right code.
type twoStepTicket struct {
	userID  int64
	expires time.Time
	tries   int
}

var twoStep = struct {
	sync.Mutex
	tickets map[string]*twoStepTicket
	setups  map[int64]twoStepSetup // secrets shown but not yet confirmed
}{tickets: map[string]*twoStepTicket{}, setups: map[int64]twoStepSetup{}}

type twoStepSetup struct {
	secret  string
	expires time.Time
}

func randomToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// needsSecondStep says whether user must give a code before being signed
// in; if so it answers with a ticket for POST /api/auth/login/code.
func (s *Server) needsSecondStep(w http.ResponseWriter, user *auth.User) bool {
	ts, err := s.Auth.GetTwoStep(user.ID)
	if err != nil || !ts.On() {
		return false
	}
	tok, err := randomToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't sign you in. Try again.")
		return true
	}
	twoStep.Lock()
	for k, t := range twoStep.tickets {
		if time.Now().After(t.expires) {
			delete(twoStep.tickets, k)
		}
	}
	twoStep.tickets[tok] = &twoStepTicket{userID: user.ID, expires: time.Now().Add(twoStepTicketLife)}
	twoStep.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"twoStep": true, "ticket": tok})
	return true
}

// checkCode checks a code from the authenticator app, or a backup code,
// for userID. A used code doesn't work again.
func (s *Server) checkCode(userID int64, code string) (ok, backup bool) {
	ts, err := s.Auth.GetTwoStep(userID)
	if err != nil || !ts.On() {
		return false, false
	}
	secret, err := s.profileBox.Decrypt(ts.Secret)
	if err != nil {
		log.Printf("api: two-step: read secret: %v", err)
		return false, false
	}
	if step := auth.CheckTOTP(secret, code, time.Now()); step != 0 {
		fresh, err := s.Auth.UseTOTPStep(userID, step)
		return err == nil && fresh, false
	}
	if len(strings.TrimSpace(code)) > 6 {
		used, err := s.Auth.UseBackupCode(userID, code)
		return err == nil && used, true
	}
	return false, false
}

// POST /api/auth/login/code {"ticket","code"}: the second step of signing
// in.
func (s *Server) handleLoginCode(w http.ResponseWriter, r *http.Request) {
	ip := s.clientIP(r)
	ipKey := httpsec.LimitKey(ip)
	if !s.LoginLimiter.Allow(ipKey) {
		writeError(w, http.StatusTooManyRequests, "Too many sign-in attempts. Wait a few minutes and try again.")
		return
	}
	var req struct {
		Ticket string `json:"ticket"`
		Code   string `json:"code"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxPublicBody)
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Couldn't read that request. Reload the page and try again.")
		return
	}
	twoStep.Lock()
	t := twoStep.tickets[req.Ticket]
	if t == nil || time.Now().After(t.expires) {
		delete(twoStep.tickets, req.Ticket)
		twoStep.Unlock()
		writeError(w, http.StatusUnauthorized, "That took too long. Sign in again.")
		return
	}
	userID := t.userID
	twoStep.Unlock()

	ok, backup := s.checkCode(userID, req.Code)
	if !ok {
		s.LoginLimiter.RecordFailure(ipKey)
		twoStep.Lock()
		t.tries++
		over := t.tries >= twoStepTries
		if over {
			delete(twoStep.tickets, req.Ticket)
		}
		twoStep.Unlock()
		slog.Warn("auth: wrong two-step code", "user_id", userID, "ip", ip)
		if over {
			writeError(w, http.StatusUnauthorized, "Too many wrong codes. Sign in again.")
			return
		}
		writeError(w, http.StatusUnauthorized, "That code isn't right. Check your authenticator app, or use a backup code.")
		return
	}
	twoStep.Lock()
	delete(twoStep.tickets, req.Ticket)
	twoStep.Unlock()
	s.LoginLimiter.RecordSuccess(ipKey)
	if backup {
		slog.Info("auth: signed in with a backup code", "user_id", userID)
	}
	if err := s.Auth.RecordLogin(userID); err != nil {
		log.Printf("api: login: %v", err)
	}
	a, err := s.Auth.GetAccount(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't sign you in. Try again.")
		return
	}
	s.startSession(w, r, userID)
	writeJSON(w, http.StatusOK, userPayload(&a.User))
}

// GET /api/auth/two-step: {"on", "backupLeft"} for the signed-in account.
func (s *Server) handleGetTwoStep(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	ts, err := s.Auth.GetTwoStep(u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't read your sign-in settings.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"on": ts.On(), "backupLeft": len(ts.Backup)})
}

// POST /api/auth/two-step/start: a new secret to add to an authenticator
// app ({"secret","uri"}; the QR code is GET /api/auth/two-step/qr). Nothing
// changes until a code from the app is confirmed.
func (s *Server) handleStartTwoStep(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	secret, err := auth.NewTOTPSecret()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't start. Try again.")
		return
	}
	twoStep.Lock()
	twoStep.setups[u.ID] = twoStepSetup{secret: secret, expires: time.Now().Add(twoStepSetupLife)}
	twoStep.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"secret": secret, "uri": auth.TOTPURI(secret, twoStepIssuer, u.Username)})
}

func pendingSetup(userID int64) (string, bool) {
	twoStep.Lock()
	defer twoStep.Unlock()
	st, ok := twoStep.setups[userID]
	if !ok || time.Now().After(st.expires) {
		delete(twoStep.setups, userID)
		return "", false
	}
	return st.secret, true
}

// GET /api/auth/two-step/qr: the QR code of the secret being set up (PNG).
func (s *Server) handleTwoStepQR(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	secret, ok := pendingSetup(u.ID)
	if !ok {
		writeError(w, http.StatusNotFound, "Start setting up two-step sign-in first.")
		return
	}
	code, err := qr.Encode(auth.TOTPURI(secret, twoStepIssuer, u.Username), qr.M)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't draw the QR code.")
		return
	}
	code.Scale = 6
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(code.PNG())
}

// POST /api/auth/two-step/confirm {"code"}: turns two-step on once the app
// shows the right code; answers the ten backup codes ({"backupCodes"}),
// shown this once.
func (s *Server) handleConfirmTwoStep(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	var req struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Couldn't read that request.")
		return
	}
	secret, ok := pendingSetup(u.ID)
	if !ok {
		writeError(w, http.StatusConflict, "That took too long. Start again.")
		return
	}
	if auth.CheckTOTP(secret, req.Code, time.Now()) == 0 {
		writeError(w, http.StatusBadRequest, "That code isn't right. Check the app shows Cue, and type the code it shows now.")
		return
	}
	sealed, err := s.profileBox.Encrypt(secret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't save it. Try again.")
		return
	}
	codes, hashes, err := auth.NewBackupCodes()
	if err == nil {
		err = s.Auth.SetTwoStep(u.ID, sealed, hashes)
	}
	if err != nil {
		log.Printf("api: two-step: %v", err)
		writeError(w, http.StatusInternalServerError, "Couldn't save it. Try again.")
		return
	}
	twoStep.Lock()
	delete(twoStep.setups, u.ID)
	twoStep.Unlock()
	slog.Info("auth: two-step sign-in turned on", "user_id", u.ID)
	writeJSON(w, http.StatusOK, map[string]any{"backupCodes": codes})
}

// passwordOK checks the signed-in account's password (for turning two-step
// off and new backup codes), counting wrong ones like failed sign-ins.
func (s *Server) passwordOK(w http.ResponseWriter, r *http.Request, u *auth.User, password string) bool {
	ipKey := httpsec.LimitKey(s.clientIP(r))
	if !s.LoginLimiter.Allow(ipKey) {
		writeError(w, http.StatusTooManyRequests, "Too many tries. Wait a few minutes and try again.")
		return false
	}
	if _, err := s.Auth.Authenticate(u.Username, password); err != nil {
		s.LoginLimiter.RecordFailure(ipKey)
		writeError(w, http.StatusBadRequest, "That password isn't right.")
		return false
	}
	return true
}

// POST /api/auth/two-step/off {"password"}: turns two-step off.
func (s *Server) handleTwoStepOff(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	var req struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Couldn't read that request.")
		return
	}
	if !s.passwordOK(w, r, u, req.Password) {
		return
	}
	if err := s.Auth.ClearTwoStep(u.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't turn it off. Try again.")
		return
	}
	slog.Info("auth: two-step sign-in turned off", "user_id", u.ID)
	writeJSON(w, http.StatusOK, map[string]any{"on": false})
}

// POST /api/auth/two-step/backup {"password"}: ten new backup codes (the
// old ones stop working).
func (s *Server) handleNewBackupCodes(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	var req struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Couldn't read that request.")
		return
	}
	if !s.passwordOK(w, r, u, req.Password) {
		return
	}
	ts, err := s.Auth.GetTwoStep(u.ID)
	if err != nil || !ts.On() {
		writeError(w, http.StatusConflict, "Two-step sign-in is off.")
		return
	}
	codes, hashes, err := auth.NewBackupCodes()
	if err == nil {
		err = s.Auth.SetBackupCodes(u.ID, hashes)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't make new codes. Try again.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"backupCodes": codes})
}
