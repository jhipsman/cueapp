package push

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"strings"
	"testing"
	"time"
)

func TestKeyAndToken(t *testing.T) {
	k, err := NewKey()
	if err != nil {
		t.Fatal(err)
	}
	again, err := ParseKey(k.String())
	if err != nil || again.Public() != k.Public() {
		t.Fatalf("key didn't survive keeping: %v", err)
	}
	pub, _ := base64.RawURLEncoding.DecodeString(k.Public())
	if len(pub) != 65 || pub[0] != 4 {
		t.Fatalf("public key is %d bytes", len(pub))
	}

	tok, err := k.token("https://fcm.googleapis.com", "https://cue.example", time.Unix(1_700_000_000, 0))
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("token %q", tok)
	}
	var claims map[string]any
	raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
	_ = json.Unmarshal(raw, &claims)
	if claims["aud"] != "https://fcm.googleapis.com" || claims["sub"] != "https://cue.example" || claims["exp"].(float64) != 1_700_000_000+12*3600 {
		t.Fatalf("claims %v", claims)
	}
	sig, _ := base64.RawURLEncoding.DecodeString(parts[2])
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	r, s := new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])
	if len(sig) != 64 || !ecdsa.Verify(&k.priv.PublicKey, sum[:], r, s) {
		t.Fatal("signature doesn't verify")
	}
}

func TestAllowedEndpoint(t *testing.T) {
	for ep, want := range map[string]bool{
		"https://fcm.googleapis.com/fcm/send/abc":            true,
		"https://web.push.apple.com/QGx":                     true,
		"https://updates.push.services.mozilla.com/wpush/v2": true,
		"https://wns2-par02p.notify.windows.com/w/?token=x":  true,
		"http://fcm.googleapis.com/fcm/send/abc":             false,
		"https://evil.example/fcm.googleapis.com":            false,
		"https://fcm.googleapis.com.evil.example/x":          false,
		"https://localhost:8080/":                            false,
		"https://fcm.googleapis.com:8443/x":                  false,
		"https://user@fcm.googleapis.com/x":                  false,
		"https://evilpush.apple.com.example/x":               false,
	} {
		if got := AllowedEndpoint(ep); got != want {
			t.Errorf("%s: %v, want %v", ep, got, want)
		}
	}
}
