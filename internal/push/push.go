// Package push sends phone alerts as Web Push (RFC 8030), signed with
// VAPID (RFC 8292). A push carries no text: the phone's background worker
// wakes and asks Cue for what to show, so what the alert says never passes
// through Google's, Apple's or Mozilla's push service, and nothing has to be
// encrypted for it (RFC 8291 is for pushes with a body).
package push

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Key is the server's VAPID key: push services check that every push for a
// device is signed by the key the device subscribed with.
type Key struct {
	priv *ecdsa.PrivateKey
}

// NewKey makes a new key.
func NewKey() (*Key, error) {
	p, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	return &Key{priv: p}, nil
}

// String is the key to keep (secret).
func (k *Key) String() string {
	der, err := x509.MarshalECPrivateKey(k.priv)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(der)
}

// ParseKey reads a key kept with String.
func ParseKey(s string) (*Key, error) {
	der, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	p, err := x509.ParseECPrivateKey(der)
	if err != nil {
		return nil, err
	}
	if p.Curve != elliptic.P256() {
		return nil, errors.New("not a P-256 key")
	}
	return &Key{priv: p}, nil
}

// Public is the public key as the browser wants it for subscribing
// (applicationServerKey): the uncompressed point, base64url.
func (k *Key) Public() string {
	pub, err := k.priv.PublicKey.ECDH()
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(pub.Bytes())
}

// pushHosts are the push services a device's endpoint may be at (Chrome and
// Android, Firefox, Safari and iPhone, Edge). Cue only ever sends to these,
// so a subscription can't make it call any other address.
var pushHosts = []string{"fcm.googleapis.com", "android.googleapis.com", "updates.push.services.mozilla.com", ".push.apple.com", ".notify.windows.com"}

// AllowedEndpoint says whether endpoint is an https address at a known push
// service.
func AllowedEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.User != nil || (u.Port() != "" && u.Port() != "443") {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, h := range pushHosts {
		if strings.HasPrefix(h, ".") && strings.HasSuffix(host, h) || host == h {
			return true
		}
	}
	return false
}

// ErrGone is a subscription the push service says is no more (the device
// turned alerts off, or the app was removed): forget it.
var ErrGone = errors.New("push subscription is gone")

// Send wakes the device at endpoint. subject is who's sending, for the push
// service: Cue's https address (or a mailto:).
func (k *Key) Send(ctx context.Context, client *http.Client, endpoint, subject string) error {
	if !AllowedEndpoint(endpoint) {
		return errors.New("not a push service address")
	}
	u, _ := url.Parse(endpoint)
	jwt, err := k.token(u.Scheme+"://"+u.Host, subject, time.Now())
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "vapid t="+jwt+", k="+k.Public())
	req.Header.Set("TTL", "86400")
	req.Header.Set("Urgency", "normal")
	req.ContentLength = 0
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	switch {
	case res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusGone:
		return ErrGone
	case res.StatusCode >= 200 && res.StatusCode < 300:
		return nil
	}
	return fmt.Errorf("push service answered %d", res.StatusCode)
}

// token is the VAPID JWT (ES256) for one push service, good for 12 hours.
func (k *Key) token(aud, sub string, now time.Time) (string, error) {
	head := base64.RawURLEncoding.EncodeToString([]byte(`{"typ":"JWT","alg":"ES256"}`))
	claims, err := json.Marshal(map[string]any{"aud": aud, "exp": now.Add(12 * time.Hour).Unix(), "sub": sub})
	if err != nil {
		return "", err
	}
	input := head + "." + base64.RawURLEncoding.EncodeToString(claims)
	sum := sha256.Sum256([]byte(input))
	r, s, err := ecdsa.Sign(rand.Reader, k.priv, sum[:])
	if err != nil {
		return "", err
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return input + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}
