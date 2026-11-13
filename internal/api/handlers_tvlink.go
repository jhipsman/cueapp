package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jhipsman/cueapp/internal/auth"
	"github.com/jhipsman/cueapp/internal/httpsec"
)

// Signing a TV in from a phone. The TV's sign-in shows a short code and a QR
// code for /watch/link/{code}; the phone that opens it (already signed in)
// confirms, and the TV, asking every couple of seconds with its own secret
// token, is signed in to that account: no typing a password on a remote.
// A code lasts ten minutes and works once.

const (
	tvLinkLife     = 10 * time.Minute
	tvLinkMax      = 200 // codes waiting at once
	tvLinkAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
)

type tvLink struct {
	code    string
	token   string
	created time.Time
	userID  int64 // set once a phone confirms
}

var tvLinks = struct {
	sync.Mutex
	byCode map[string]*tvLink
}{byCode: map[string]*tvLink{}}

func tvLinkCode() (string, error) {
	var b strings.Builder
	for range 6 {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(tvLinkAlphabet))))
		if err != nil {
			return "", err
		}
		b.WriteByte(tvLinkAlphabet[n.Int64()])
	}
	return b.String(), nil
}

// pruneTVLinks drops expired codes. Call with tvLinks locked.
func pruneTVLinks() {
	for c, l := range tvLinks.byCode {
		if time.Since(l.created) > tvLinkLife {
			delete(tvLinks.byCode, c)
		}
	}
}

// POST /api/auth/tv-link: a new code for a TV to show, and the secret
// token it asks with ({"code","token","expiresIn"}).
func (s *Server) handleTVLinkStart(w http.ResponseWriter, r *http.Request) {
	if !s.LoginLimiter.Allow(httpsec.LimitKey(s.clientIP(r))) {
		writeError(w, http.StatusTooManyRequests, "Too many tries. Wait a few minutes and try again.")
		return
	}
	code, err := tvLinkCode()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't make a code. Try again.")
		return
	}
	tok := make([]byte, 24)
	if _, err := rand.Read(tok); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't make a code. Try again.")
		return
	}
	l := &tvLink{code: code, token: hex.EncodeToString(tok), created: time.Now()}
	tvLinks.Lock()
	pruneTVLinks()
	if len(tvLinks.byCode) >= tvLinkMax {
		tvLinks.Unlock()
		writeError(w, http.StatusTooManyRequests, "Too many TVs are signing in right now. Try again in a few minutes.")
		return
	}
	tvLinks.byCode[code] = l
	tvLinks.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"code": l.code, "token": l.token, "expiresIn": int(tvLinkLife.Seconds())})
}

// GET /api/auth/tv-link/{token}: the TV asking whether a phone has
// confirmed. Once one has, this signs the TV in (its session cookie) and
// the code is used up: {"ok":true}. Otherwise {"pending":true}, or 410
// when the code has run out.
func (s *Server) handleTVLinkPoll(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	tvLinks.Lock()
	var found *tvLink
	for _, l := range tvLinks.byCode {
		if subtle.ConstantTimeCompare([]byte(l.token), []byte(token)) == 1 {
			found = l
			break
		}
	}
	if found == nil || time.Since(found.created) > tvLinkLife {
		if found != nil {
			delete(tvLinks.byCode, found.code)
		}
		tvLinks.Unlock()
		writeError(w, http.StatusGone, "That code has run out. A new one is on the TV.")
		return
	}
	userID := found.userID
	if userID != 0 {
		delete(tvLinks.byCode, found.code)
	}
	tvLinks.Unlock()
	if userID == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"pending": true})
		return
	}
	s.startSession(w, r, userID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// POST /api/auth/tv-link/approve {"code"}: a signed-in phone signs the TV
// showing this code in to its own account.
func (s *Server) handleTVLinkApprove(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	var req struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "That request couldn't be read.")
		return
	}
	code := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(req.Code), " ", ""))
	tvLinks.Lock()
	defer tvLinks.Unlock()
	l, ok := tvLinks.byCode[code]
	if !ok || time.Since(l.created) > tvLinkLife {
		writeError(w, http.StatusNotFound, "That code isn't on any TV right now. Check it, or open sign-in on the TV again for a new one.")
		return
	}
	if l.userID != 0 && l.userID != u.ID {
		writeError(w, http.StatusConflict, "Someone else already signed that TV in.")
		return
	}
	l.userID = u.ID
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
