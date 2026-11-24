// Package crashes keeps the crash reports of Cue's own apps (the TV app and
// the web app), one row per kind of crash. Settings > System > Problems shows
// them, and a crash Cue hasn't seen before (or one marked fixed that comes
// back) alerts the owner's phone.
package crashes

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// MaxTrace caps how much of one trace is kept: the top says where it broke.
	MaxTrace = 16 << 10
	// MaxRows is how many kinds of crash are kept; the oldest go first.
	MaxRows = 200
	// keepSeen is how many devices and versions are remembered per crash.
	keepSeen = 6

	timeFormat = "2006-01-02T15:04:05Z"
)

// ErrNotFound is returned for a crash id that isn't kept.
var ErrNotFound = errors.New("no such crash")

// Report is one crash as an app sends it.
type Report struct {
	App     string // "tv" or "web"
	Version string
	Device  string
	Trace   string
}

// Crash is one kind of crash, with every report of it folded in.
type Crash struct {
	ID       int64     `json:"id"`
	App      string    `json:"app"`
	Headline string    `json:"headline"`
	Place    string    `json:"place"`
	Version  string    `json:"version"`
	Device   string    `json:"device"`
	Devices  []string  `json:"devices"`
	Versions []string  `json:"versions"`
	Trace    string    `json:"trace"`
	Count    int       `json:"count"`
	FirstAt  time.Time `json:"firstAt"`
	LastAt   time.Time `json:"lastAt"`
	FixedAt  string    `json:"fixedAt,omitempty"` // when it was marked fixed; "" while open
}

// Store is the kept crashes.
type Store struct {
	db  *sql.DB
	now func() time.Time
}

// Open returns the store in db, which must have the app_crashes table.
func Open(db *sql.DB) *Store { return &Store{db: db, now: time.Now} }

// SetClock replaces the clock (tests).
func (s *Store) SetClock(now func() time.Time) { s.now = now }

// Record keeps a report. It returns the crash it was folded into, and news:
// true when this kind of crash wasn't kept before, or had been marked fixed
// and is back. Those are the ones worth an alert.
func (s *Store) Record(r Report) (Crash, bool, error) {
	r.App = clip(strings.TrimSpace(r.App), 20)
	if r.App == "" {
		r.App = "tv"
	}
	r.Version = clip(strings.TrimSpace(r.Version), 40)
	r.Device = clip(strings.TrimSpace(r.Device), 120)
	r.Trace = clip(r.Trace, MaxTrace)
	headline, place := Describe(r.Trace)
	sig := Signature(r.App, r.Trace)
	now := s.now().UTC().Format(timeFormat)

	tx, err := s.db.Begin()
	if err != nil {
		return Crash{}, false, err
	}
	defer tx.Rollback()

	var id int64
	var devices, versions, fixed string
	err = tx.QueryRow(`SELECT id, devices, versions, fixed_at FROM app_crashes WHERE signature = ?`, sig).Scan(&id, &devices, &versions, &fixed)
	news := false
	switch {
	case errors.Is(err, sql.ErrNoRows):
		news = true
		res, err := tx.Exec(`INSERT INTO app_crashes (signature, app, headline, place, version, device, devices, versions, trace, count, first_at, last_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`,
			sig, r.App, headline, place, r.Version, r.Device, r.Device, r.Version, r.Trace, now, now)
		if err != nil {
			return Crash{}, false, fmt.Errorf("keep crash: %w", err)
		}
		id, _ = res.LastInsertId()
		if _, err := tx.Exec(`DELETE FROM app_crashes WHERE id NOT IN (SELECT id FROM app_crashes ORDER BY last_at DESC, id DESC LIMIT ?)`, MaxRows); err != nil {
			return Crash{}, false, err
		}
	case err != nil:
		return Crash{}, false, fmt.Errorf("read crash: %w", err)
	default:
		news = fixed != ""
		if _, err := tx.Exec(`UPDATE app_crashes SET headline = ?, place = ?, version = ?, device = ?, devices = ?, versions = ?, trace = ?,
			count = count + 1, last_at = ?, fixed_at = '' WHERE id = ?`,
			headline, place, r.Version, r.Device, addSeen(devices, r.Device), addSeen(versions, r.Version), r.Trace, now, id); err != nil {
			return Crash{}, false, fmt.Errorf("keep crash: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return Crash{}, false, err
	}
	c, err := s.Get(id)
	return c, news, err
}

const columns = `id, app, headline, place, version, device, devices, versions, trace, count, first_at, last_at, fixed_at`

func scan(row interface{ Scan(...any) error }) (Crash, error) {
	var c Crash
	var devices, versions, first, last string
	if err := row.Scan(&c.ID, &c.App, &c.Headline, &c.Place, &c.Version, &c.Device, &devices, &versions, &c.Trace, &c.Count, &first, &last, &c.FixedAt); err != nil {
		return Crash{}, err
	}
	c.Devices, c.Versions = lines(devices), lines(versions)
	c.FirstAt, _ = time.Parse(timeFormat, first)
	c.LastAt, _ = time.Parse(timeFormat, last)
	return c, nil
}

// Get returns one crash.
func (s *Store) Get(id int64) (Crash, error) {
	c, err := scan(s.db.QueryRow(`SELECT `+columns+` FROM app_crashes WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Crash{}, ErrNotFound
	}
	return c, err
}

// List returns every kept crash: open ones first, then the latest first.
func (s *Store) List() ([]Crash, error) {
	rows, err := s.db.Query(`SELECT ` + columns + ` FROM app_crashes ORDER BY fixed_at != '', last_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Crash{}
	for rows.Next() {
		c, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// OpenCount is how many crashes aren't marked fixed.
func (s *Store) OpenCount() int {
	var n int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM app_crashes WHERE fixed_at = ''`).Scan(&n)
	return n
}

// SetFixed marks a crash fixed (or open again). A fixed crash that is
// reported again opens again, and alerts again.
func (s *Store) SetFixed(id int64, fixed bool) error {
	at := ""
	if fixed {
		at = s.now().UTC().Format(timeFormat)
	}
	res, err := s.db.Exec(`UPDATE app_crashes SET fixed_at = ? WHERE id = ?`, at, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete forgets a crash.
func (s *Store) Delete(id int64) error {
	res, err := s.db.Exec(`DELETE FROM app_crashes WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

var (
	// "at app.cue.tv.home.Parts.keepRowFocus(Parts.kt:313)" → the frame.
	javaFrame = regexp.MustCompile(`^\s*at\s+([\w$.<>]+)\(([^)]*)\)`)
	// Line and column numbers, build hashes and object addresses, which change
	// from build to build without the crash being a different one.
	noise = regexp.MustCompile(`:\d+(:\d+)?|-[A-Za-z0-9_]{8}\.js|@[0-9a-f]{6,}|\b0x[0-9a-f]+\b|\$\d+`)
)

// Describe picks out a trace's headline (the exception and its message) and
// place (the first frame in Cue's own code, else the first frame).
func Describe(trace string) (headline, place string) {
	first := ""
	for _, line := range strings.Split(trace, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if headline == "" {
			if !strings.HasPrefix(t, "thread ") { // CrashLog's first line names the thread
				headline = t
			}
			continue
		}
		var frame string
		if m := javaFrame.FindStringSubmatch(t); m != nil {
			frame = m[1] + "(" + m[2] + ")"
		} else if strings.HasPrefix(t, "at ") || strings.Contains(t, "@") {
			frame = strings.TrimPrefix(t, "at ")
		} else {
			continue // "Caused by:", "... 12 more", a component stack's text
		}
		if first == "" {
			first = frame
		}
		if strings.HasPrefix(frame, "app.cue.") || strings.Contains(frame, "/src/") {
			place = frame
			break
		}
	}
	if place == "" {
		place = first
	}
	return clip(headline, 300), clip(place, 300)
}

// Signature is what makes two reports the same crash: the app, the kind of
// exception (without its message, which often carries ids) and where in Cue's
// code it happened, without line numbers.
func Signature(app, trace string) string {
	headline, place := Describe(trace)
	kind := headline
	if i := strings.Index(kind, ":"); i > 0 {
		kind = kind[:i]
	}
	return app + "|" + noise.ReplaceAllString(kind, "") + "|" + noise.ReplaceAllString(place, "")
}

func addSeen(list, v string) string {
	if v == "" {
		return list
	}
	seen := lines(list)
	for _, s := range seen {
		if s == v {
			return list
		}
	}
	seen = append(seen, v)
	if len(seen) > keepSeen {
		seen = seen[len(seen)-keepSeen:]
	}
	return strings.Join(seen, "\n")
}

func lines(s string) []string {
	out := []string{}
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for !utf8.ValidString(s) && len(s) > 0 {
		s = s[:len(s)-1]
	}
	return s
}
