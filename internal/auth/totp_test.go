package auth

import (
	"testing"
	"time"
)

// RFC 6238's SHA-1 test key and times (6-digit form).
func TestTOTPMatchesRFC6238(t *testing.T) {
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ" // base32 of "12345678901234567890"
	for unix, want := range map[int64]string{59: "287082", 1111111109: "081804", 1234567890: "005924", 2000000000: "279037"} {
		got, err := totpAt(secret, unix/totpStep)
		if err != nil || got != want {
			t.Errorf("at %d: %s, %v; want %s", unix, got, err, want)
		}
		if CheckTOTP(secret, want, time.Unix(unix, 0)) == 0 {
			t.Errorf("at %d: the right code was refused", unix)
		}
	}
	if CheckTOTP(secret, "287082", time.Unix(59+300, 0)) != 0 {
		t.Error("a code five minutes old was accepted")
	}
	if CheckTOTP(secret, "12345", time.Now()) != 0 {
		t.Error("a short code was accepted")
	}
}

func TestBackupCodes(t *testing.T) {
	codes, hashes, err := NewBackupCodes()
	if err != nil || len(codes) != 10 || len(hashes) != 10 {
		t.Fatalf("%v %v %v", codes, hashes, err)
	}
	if backupHash(" "+codes[0]+" ") != hashes[0] || backupHash(stripDashes(codes[0])) != hashes[0] {
		t.Error("a backup code typed with spaces or without dashes should still match")
	}
}

func stripDashes(s string) string {
	out := ""
	for _, c := range s {
		if c != '-' {
			out += string(c)
		}
	}
	return out
}
