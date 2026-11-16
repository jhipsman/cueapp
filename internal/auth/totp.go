package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Two-step sign-in with an authenticator app (RFC 6238: six digits, every
// 30 seconds, SHA-1), plus one-time backup codes for a lost phone.

const totpStep = 30

// NewTOTPSecret is a new random secret, as authenticator apps take it
// (base32, no padding).
func NewTOTPSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), nil
}

// TOTPURI is the otpauth:// address an authenticator app reads (from a QR
// code, or opened on the phone itself).
func TOTPURI(secret, issuer, account string) string {
	label := url.PathEscape(issuer + ":" + account)
	q := url.Values{"secret": {secret}, "issuer": {issuer}, "algorithm": {"SHA1"}, "digits": {"6"}, "period": {"30"}}
	return "otpauth://totp/" + label + "?" + q.Encode()
}

func totpAt(secret string, step int64) (string, error) {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.TrimRight(secret, "=")))
	if err != nil {
		return "", err
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step))
	h := hmac.New(sha1.New, key)
	h.Write(msg[:])
	sum := h.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	n := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", n%1_000_000), nil
}

// CheckTOTP says which 30-second step code belongs to (allowing one step
// either side, for a clock that's a little off), or 0 when it doesn't.
func CheckTOTP(secret, code string, now time.Time) int64 {
	code = strings.ReplaceAll(strings.TrimSpace(code), " ", "")
	if len(code) != 6 {
		return 0
	}
	cur := now.Unix() / totpStep
	for _, step := range []int64{cur, cur - 1, cur + 1} {
		want, err := totpAt(secret, step)
		if err == nil && subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 {
			return step
		}
	}
	return 0
}

// NewBackupCodes are ten one-time codes ("abcd-efgh-jk"), and their hashes
// to store.
func NewBackupCodes() (codes, hashes []string, err error) {
	const letters = "abcdefghjkmnpqrstuvwxyz23456789"
	for range 10 {
		b := make([]byte, 10)
		if _, err := rand.Read(b); err != nil {
			return nil, nil, err
		}
		var s strings.Builder
		for i, c := range b {
			if i == 4 || i == 8 {
				s.WriteByte('-')
			}
			s.WriteByte(letters[int(c)%len(letters)])
		}
		codes = append(codes, s.String())
		hashes = append(hashes, backupHash(s.String()))
	}
	return codes, hashes, nil
}

func backupHash(code string) string {
	plain := strings.ToLower(strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(code)))
	sum := sha256.Sum256([]byte("cue-backup:" + plain))
	return hex.EncodeToString(sum[:])
}

// TwoStep is an account's two-step state as stored. Secret is as the
// caller stored it (Cue encrypts it).
type TwoStep struct {
	Secret   string
	Backup   []string // hashes of the unused backup codes
	LastStep int64
}

// On says two-step sign-in is turned on.
func (t TwoStep) On() bool { return t.Secret != "" }

// GetTwoStep reads an account's two-step state.
func (s *Service) GetTwoStep(userID int64) (TwoStep, error) {
	var secret, backup string
	var last int64
	err := s.db.QueryRow(`SELECT totp_secret, totp_backup, totp_last_step FROM users WHERE id = ?`, userID).Scan(&secret, &backup, &last)
	if errors.Is(err, sql.ErrNoRows) {
		return TwoStep{}, ErrUserNotFound
	}
	if err != nil {
		return TwoStep{}, err
	}
	t := TwoStep{Secret: secret, LastStep: last}
	if backup != "" {
		t.Backup = strings.Split(backup, ",")
	}
	return t, nil
}

// SetTwoStep turns two-step on with secret (as stored) and backup hashes.
func (s *Service) SetTwoStep(userID int64, secret string, backupHashes []string) error {
	_, err := s.db.Exec(`UPDATE users SET totp_secret = ?, totp_backup = ?, totp_last_step = 0 WHERE id = ?`, secret, strings.Join(backupHashes, ","), userID)
	return err
}

// SetBackupCodes replaces the backup codes.
func (s *Service) SetBackupCodes(userID int64, backupHashes []string) error {
	_, err := s.db.Exec(`UPDATE users SET totp_backup = ? WHERE id = ?`, strings.Join(backupHashes, ","), userID)
	return err
}

// ClearTwoStep turns two-step off.
func (s *Service) ClearTwoStep(userID int64) error {
	_, err := s.db.Exec(`UPDATE users SET totp_secret = '', totp_backup = '', totp_last_step = 0 WHERE id = ?`, userID)
	return err
}

// UseTOTPStep records that a code for step was used, refusing one already
// used (or older): each code works once.
func (s *Service) UseTOTPStep(userID, step int64) (bool, error) {
	res, err := s.db.Exec(`UPDATE users SET totp_last_step = ? WHERE id = ? AND totp_last_step < ?`, step, userID, step)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// UseBackupCode spends a backup code, if it's one of the account's unused
// ones.
func (s *Service) UseBackupCode(userID int64, code string) (bool, error) {
	t, err := s.GetTwoStep(userID)
	if err != nil {
		return false, err
	}
	h := backupHash(code)
	for i, b := range t.Backup {
		if subtle.ConstantTimeCompare([]byte(b), []byte(h)) == 1 {
			left := append(append([]string{}, t.Backup[:i]...), t.Backup[i+1:]...)
			return true, s.SetBackupCodes(userID, left)
		}
	}
	return false, nil
}

// TOTPCodeAt is the code an authenticator app shows at t (for tests).
func TOTPCodeAt(secret string, t time.Time) (string, error) {
	return totpAt(secret, t.Unix()/totpStep)
}
