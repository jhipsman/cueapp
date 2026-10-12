package auth

import (
	"context"
	"net/http"
	"time"
)

const CookieName = "cue_session"

// legacyCookieName is the session cookie's name before the rename to Cue. It
// is still read, so nobody is signed out by the update; a new sign-in replaces
// it, and it runs out with its session (7 days) otherwise.
const legacyCookieName = "mediarium_session"

type contextKey string

const userContextKey contextKey = "auth_user"

// SetSessionCookie writes the session cookie: an opaque random token that
// is looked up server-side, HttpOnly (scripts cannot read it) and
// SameSite=Strict (browsers never send it on a request started by another
// site). secure marks it Secure (HTTPS only); the caller sets it whenever the
// visitor reached Cue over HTTPS, directly or through a trusted proxy.
func SetSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time, secure bool) {
	clearCookie(w, legacyCookieName, secure)
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		Expires:  expiresAt,
	})
}

// ClearSessionCookie expires the session cookie; secure must match the way
// it was set.
func ClearSessionCookie(w http.ResponseWriter, secure bool) {
	clearCookie(w, CookieName, secure)
	clearCookie(w, legacyCookieName, secure)
}

func clearCookie(w http.ResponseWriter, name string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

// Middleware resolves the caller's session cookie or X-API-Key header into
// a *User in the request context, or rejects with 401.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := s.userFromRequest(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// UserFromRequest is the account behind the request's X-API-Key header or
// session cookie.
func (s *Service) UserFromRequest(r *http.Request) (*User, error) { return s.userFromRequest(r) }

// WithUser returns ctx carrying user, as Middleware does, for callers that
// sign a request in another way (a trusted reverse proxy).
func WithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func (s *Service) userFromRequest(r *http.Request) (*User, error) {
	if key := r.Header.Get("X-API-Key"); key != "" {
		return s.UserForAPIKey(key)
	}
	token, ok := SessionToken(r)
	if !ok {
		return nil, ErrSessionNotFound
	}
	return s.UserForSession(token)
}

// SessionToken is the session token in the request's session cookie, under
// its current name or the one from before the rename.
func SessionToken(r *http.Request) (string, bool) {
	for _, name := range []string{CookieName, legacyCookieName} {
		if c, err := r.Cookie(name); err == nil && c.Value != "" {
			return c.Value, true
		}
	}
	return "", false
}

// UserFromContext retrieves the authenticated user set by Middleware.
func UserFromContext(ctx context.Context) *User {
	user, _ := ctx.Value(userContextKey).(*User)
	return user
}
