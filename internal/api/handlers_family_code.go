package api

import (
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"
	"unicode"

	"github.com/jhipsman/cueapp/internal/httpsec"
	"github.com/jhipsman/cueapp/internal/settings"
)

// The family code: one code, set once by an administrator, that family
// members type on the sign-in page ("New here? Create an account") to make
// their own account. It doesn't end by itself, can be passed on by word of
// mouth, and stops working the moment it's changed or turned off (accounts
// already made keep working). Wrong codes count like failed sign-ins, so it
// can't be guessed. Accounts made with it are the same as the invite
// link's: they watch, and can't see or change settings.

// familyWords are easy to say and spell; a code is two of them and a
// number, about four million in all.
var familyWords = strings.Fields(`
	maple river cedar willow harbor meadow canyon summit lantern orchard
	pebble comet falcon otter badger heron juniper aspen birch clover
	coral ember frost glacier hazel island jasper kettle lemon mango
	marble nectar olive panda pepper pine plum quartz raven saffron
	sierra spruce sunny tango tiger topaz tulip velvet violet walnut
	amber apple bamboo beacon berry blossom breeze button cactus candle
	cherry cocoa copper cotton daisy delta dune eagle echo fern
	fig garnet ginger harvest honey indigo ivory jade jelly kiwi
	koala lagoon lily lotus lucky magnet mint moss nova ocean
	onyx orbit papaya peach pearl pickle pilot pixel prairie puffin
	radish rocket ruby sage salsa sapphire sequoia shadow silver sparrow
	spice sprout star storm sugar sunset taco thistle thunder timber
	toffee trail tundra turtle valley vanilla waffle whale zebra zinnia`)

// normalCode is a code as compared: letters and digits only, upper case
// ("maple river 42" and "MAPLE-RIVER-42" are the same code).
func normalCode(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToUpper(r))
		}
	}
	return b.String()
}

func newFamilyCode() (string, error) {
	pick := func(n int) (int, error) {
		v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
		if err != nil {
			return 0, err
		}
		return int(v.Int64()), nil
	}
	a, err := pick(len(familyWords))
	if err != nil {
		return "", err
	}
	b, err := pick(len(familyWords) - 1)
	if err != nil {
		return "", err
	}
	if b >= a {
		b++ // two different words
	}
	n, err := pick(90)
	if err != nil {
		return "", err
	}
	return strings.ToUpper(fmt.Sprintf("%s-%s-%d", familyWords[a], familyWords[b], n+10)), nil
}

func (s *Server) familyCode() string {
	c, _ := s.Settings.Get(settings.KeyFamilyCode)
	return c
}

// GET /api/users/family-code: {"code"}, or {} when it's off.
func (s *Server) handleGetFamilyCode(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"code": s.familyCode()})
}

// POST /api/users/family-code: a new random code (the old one stops working).
func (s *Server) handleNewFamilyCode(w http.ResponseWriter, r *http.Request) {
	code, err := newFamilyCode()
	if err == nil {
		err = s.Settings.Set(settings.KeyFamilyCode, code, true)
	}
	if err != nil {
		log.Printf("api: family code: %v", err)
		writeError(w, http.StatusInternalServerError, "Couldn't make a family code. Try again.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"code": code})
}

// PUT /api/users/family-code {"code"}: a code of your own choosing.
func (s *Server) handleSetFamilyCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Couldn't read that request.")
		return
	}
	code := strings.ToUpper(strings.Join(strings.Fields(req.Code), "-"))
	switch n := len(normalCode(code)); {
	case n < 8:
		writeError(w, http.StatusBadRequest, "Make the code at least 8 letters or numbers long, so it can't be guessed.")
		return
	case len(code) > 40:
		writeError(w, http.StatusBadRequest, "Keep the code under 40 characters.")
		return
	}
	if err := s.Settings.Set(settings.KeyFamilyCode, code, true); err != nil {
		log.Printf("api: family code: %v", err)
		writeError(w, http.StatusInternalServerError, "Couldn't save the code. Try again.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"code": code})
}

// DELETE /api/users/family-code: no more accounts from a code.
func (s *Server) handleEndFamilyCode(w http.ResponseWriter, r *http.Request) {
	if err := s.Settings.Set(settings.KeyFamilyCode, "", false); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't turn the code off. Try again.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{})
}

// GET /api/join/family-code: whether the sign-in page offers "Create an
// account" ({"open":true}). Says nothing about the code itself.
func (s *Server) handleFamilyCodeOpen(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"open": s.familyCode() != ""})
}

// POST /api/join/family-code {"code","name","username","password"}: a new
// member account, signed in.
func (s *Server) handleJoinWithCode(w http.ResponseWriter, r *http.Request) {
	ipKey := httpsec.LimitKey(s.clientIP(r))
	if !s.LoginLimiter.Allow(ipKey) {
		writeError(w, http.StatusTooManyRequests, "Too many tries. Wait a few minutes and try again.")
		return
	}
	var req struct {
		Code     string `json:"code"`
		Name     string `json:"name"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxPublicBody)
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Couldn't read that request. Reload the page and try again.")
		return
	}
	want := normalCode(s.familyCode())
	given := normalCode(req.Code)
	if want == "" {
		writeError(w, http.StatusNotFound, "Making accounts with a family code is turned off. Ask whoever runs Cue.")
		return
	}
	if subtle.ConstantTimeCompare([]byte(given), []byte(want)) != 1 {
		s.LoginLimiter.RecordFailure(ipKey)
		writeError(w, http.StatusForbidden, "That family code isn't right. Check it with whoever runs Cue.")
		return
	}
	s.makeMember(w, r, req.Name, req.Username, req.Password)
}
