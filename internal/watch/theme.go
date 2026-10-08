package watch

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Theme is a profile's colors in Watch: the accent (buttons, focus, the
// logo), how much it glows, and the background.
type Theme struct {
	Accent     string `json:"accent"`     // "#rrggbb"
	Glow       string `json:"glow"`       // "off", "soft" or "strong"
	Background string `json:"background"` // "midnight", "black", "slate" or "tinted" (the accent, faintly)
}

// DefaultTheme is Cue's own: cyan, a soft glow, midnight.
var DefaultTheme = Theme{Accent: "#34d1bf", Glow: "soft", Background: "midnight"}

var (
	hexColor    = regexp.MustCompile(`^#[0-9a-f]{6}$`)
	glows       = map[string]bool{"off": true, "soft": true, "strong": true}
	backgrounds = map[string]bool{"midnight": true, "black": true, "slate": true, "tinted": true}
)

// Clean fills anything missing or unknown from DefaultTheme.
func (t Theme) Clean() Theme {
	t.Accent = strings.ToLower(strings.TrimSpace(t.Accent))
	if !hexColor.MatchString(t.Accent) {
		t.Accent = DefaultTheme.Accent
	}
	if !glows[t.Glow] {
		t.Glow = DefaultTheme.Glow
	}
	if !backgrounds[t.Background] {
		t.Background = DefaultTheme.Background
	}
	return t
}

func parseTheme(s string) Theme {
	var t Theme
	if s != "" {
		_ = json.Unmarshal([]byte(s), &t)
	}
	return t.Clean()
}

// SetTheme saves one of userID's profiles' theme.
func (r *Repo) SetTheme(userID, id int64, t Theme) (Theme, error) {
	t = t.Clean()
	b, _ := json.Marshal(t)
	res, err := r.db.Exec(`UPDATE profiles SET theme = ? WHERE id = ? AND user_id = ?`, string(b), id, userID)
	if err != nil {
		return Theme{}, fmt.Errorf("save theme: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Theme{}, ErrProfileNotFound
	}
	return t, nil
}
