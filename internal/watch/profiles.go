package watch

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Profile is one person's profile in a household (an account).
type Profile struct {
	ID     int64
	UserID int64
	Name   string
	Avatar string // a color name from AvatarColors
	HasPIN bool
	Main   bool // the account owner's: the only one that may change settings
	Theme  Theme
	secret string
	pin    string // bcrypt hash
}

// Secret changes whenever the PIN does, so a sign-in to the profile made
// before stops working.
func (p Profile) Secret() string { return p.secret }

// AvatarColors are the avatars to choose from.
var AvatarColors = []string{"teal", "red", "blue", "purple", "orange", "green", "pink", "yellow"}

// MaxProfiles is how many profiles one account can have (as on Netflix).
const MaxProfiles = 6

// Errors the profile store returns.
var (
	ErrProfileNotFound = errors.New("that profile doesn't exist")
	ErrTooManyProfiles = fmt.Errorf("an account can have at most %d profiles", MaxProfiles)
	ErrMainProfile     = errors.New("the main profile can't be removed")
	ErrBadPIN          = errors.New("a PIN is 4 digits")
	ErrBadName         = errors.New("give the profile a name of up to 30 characters")
)

var pinRe = regexp.MustCompile(`^[0-9]{4}$`)

func newSecret() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func cleanName(name string) (string, error) {
	name = strings.Join(strings.Fields(name), " ")
	if name == "" || len([]rune(name)) > 30 {
		return "", ErrBadName
	}
	return name, nil
}

func cleanAvatar(a string) string {
	for _, c := range AvatarColors {
		if a == c {
			return a
		}
	}
	return AvatarColors[0]
}

const profileCols = `id, user_id, name, avatar, pin_hash, secret, is_main, theme`

func scanProfile(sc interface{ Scan(...any) error }) (Profile, error) {
	var p Profile
	var theme string
	if err := sc.Scan(&p.ID, &p.UserID, &p.Name, &p.Avatar, &p.pin, &p.secret, &p.Main, &theme); err != nil {
		return p, err
	}
	p.Theme = parseTheme(theme)
	p.HasPIN = p.pin != ""
	return p, nil
}

// Profiles lists userID's profiles, the main one first, making it if the
// account has none yet (accounts made after the profiles came in).
func (r *Repo) Profiles(userID int64, ownerName string) ([]Profile, error) {
	list, err := r.listProfiles(userID)
	if err != nil || len(list) > 0 {
		return list, err
	}
	if ownerName == "" {
		ownerName = "Me"
	}
	if _, err := r.db.Exec(`INSERT INTO profiles (user_id, name, avatar, is_main, secret, created_at) VALUES (?, ?, ?, 1, ?, ?)`,
		userID, ownerName, AvatarColors[0], newSecret(), time.Now().UTC().Format(time.RFC3339)); err != nil {
		return nil, fmt.Errorf("make the main profile: %w", err)
	}
	return r.listProfiles(userID)
}

func (r *Repo) listProfiles(userID int64) ([]Profile, error) {
	rows, err := r.db.Query(`SELECT `+profileCols+` FROM profiles WHERE user_id = ? ORDER BY is_main DESC, id`, userID)
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}
	defer rows.Close()
	var out []Profile
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Profile is one of userID's profiles.
func (r *Repo) Profile(userID, id int64) (Profile, error) {
	p, err := scanProfile(r.db.QueryRow(`SELECT `+profileCols+` FROM profiles WHERE id = ? AND user_id = ?`, id, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return Profile{}, ErrProfileNotFound
	}
	return p, err
}

// AddProfile makes a new profile for userID.
func (r *Repo) AddProfile(userID int64, name, avatar string) (Profile, error) {
	name, err := cleanName(name)
	if err != nil {
		return Profile{}, err
	}
	list, err := r.Profiles(userID, "")
	if err != nil {
		return Profile{}, err
	}
	if len(list) >= MaxProfiles {
		return Profile{}, ErrTooManyProfiles
	}
	res, err := r.db.Exec(`INSERT INTO profiles (user_id, name, avatar, is_main, secret, created_at) VALUES (?, ?, ?, 0, ?, ?)`,
		userID, name, cleanAvatar(avatar), newSecret(), time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return Profile{}, fmt.Errorf("add profile: %w", err)
	}
	id, _ := res.LastInsertId()
	return r.Profile(userID, id)
}

// ProfileChange is what UpdateProfile changes; nil fields stay as they are.
// An empty PIN removes it.
type ProfileChange struct {
	Name   *string
	Avatar *string
	PIN    *string
}

// UpdateProfile changes one of userID's profiles.
func (r *Repo) UpdateProfile(userID, id int64, c ProfileChange) (Profile, error) {
	p, err := r.Profile(userID, id)
	if err != nil {
		return Profile{}, err
	}
	if c.Name != nil {
		if p.Name, err = cleanName(*c.Name); err != nil {
			return Profile{}, err
		}
	}
	if c.Avatar != nil {
		p.Avatar = cleanAvatar(*c.Avatar)
	}
	if c.PIN != nil {
		switch {
		case *c.PIN == "":
			p.pin = ""
		case pinRe.MatchString(*c.PIN):
			h, err := bcrypt.GenerateFromPassword([]byte(*c.PIN), bcrypt.DefaultCost)
			if err != nil {
				return Profile{}, err
			}
			p.pin = string(h)
		default:
			return Profile{}, ErrBadPIN
		}
		p.secret = newSecret()
	}
	if _, err := r.db.Exec(`UPDATE profiles SET name = ?, avatar = ?, pin_hash = ?, secret = ? WHERE id = ? AND user_id = ?`,
		p.Name, p.Avatar, p.pin, p.secret, id, userID); err != nil {
		return Profile{}, fmt.Errorf("update profile: %w", err)
	}
	return r.Profile(userID, id)
}

// RemoveProfile deletes one of userID's profiles with its progress and My
// List. The main profile stays.
func (r *Repo) RemoveProfile(userID, id int64) error {
	p, err := r.Profile(userID, id)
	if err != nil {
		return err
	}
	if p.Main {
		return ErrMainProfile
	}
	_, err = r.db.Exec(`DELETE FROM profiles WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// CheckPIN says whether pin opens the profile (a profile without a PIN
// opens with anything).
func (p Profile) CheckPIN(pin string) bool {
	if p.pin == "" {
		return true
	}
	return bcrypt.CompareHashAndPassword([]byte(p.pin), []byte(pin)) == nil
}
