package watch

import (
	"fmt"
	"time"
)

// Marks a profile puts on titles.
const (
	MarkHidden = "hidden" // off Continue Watching until watched again
	MarkUp     = "up"     // liked
	MarkDown   = "down"   // not for me: kept off Home
	MarkRemind = "remind" // tell me when it's out to stream
)

// Marks is a profile's marks: "movie:603" → mark → when.
type Marks map[string]map[string]time.Time

// Key is a title's key in Marks.
func Key(kind string, tmdbID int) string { return fmt.Sprintf("%s:%d", kind, tmdbID) }

// Has says whether the title has the mark, and when it was made.
func (m Marks) Has(kind string, tmdbID int, mark string) (time.Time, bool) {
	at, ok := m[Key(kind, tmdbID)][mark]
	return at, ok
}

// SetMark puts a mark on a title (on) or takes it off. Thumbs up and down
// replace each other.
func (r *Repo) SetMark(profileID int64, kind string, tmdbID int, mark string, on bool) error {
	if mark != MarkHidden && mark != MarkUp && mark != MarkDown && mark != MarkRemind {
		return fmt.Errorf("unknown mark %q", mark)
	}
	if mark == MarkUp || mark == MarkDown {
		if _, err := r.db.Exec(`DELETE FROM title_marks WHERE profile_id = ? AND kind = ? AND tmdb_id = ? AND mark IN ('up', 'down')`, profileID, kind, tmdbID); err != nil {
			return err
		}
	}
	if !on {
		_, err := r.db.Exec(`DELETE FROM title_marks WHERE profile_id = ? AND kind = ? AND tmdb_id = ? AND mark = ?`, profileID, kind, tmdbID, mark)
		return err
	}
	_, err := r.db.Exec(`INSERT INTO title_marks (profile_id, kind, tmdb_id, mark, at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (profile_id, kind, tmdb_id, mark) DO UPDATE SET at = excluded.at`,
		profileID, kind, tmdbID, mark, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

// Marks is everything profileID has marked.
func (r *Repo) Marks(profileID int64) (Marks, error) {
	rows, err := r.db.Query(`SELECT kind, tmdb_id, mark, at FROM title_marks WHERE profile_id = ?`, profileID)
	if err != nil {
		return nil, fmt.Errorf("read marks: %w", err)
	}
	defer rows.Close()
	out := Marks{}
	for rows.Next() {
		var kind, mark, at string
		var id int
		if err := rows.Scan(&kind, &id, &mark, &at); err != nil {
			return nil, err
		}
		t, _ := time.Parse(time.RFC3339Nano, at)
		k := Key(kind, id)
		if out[k] == nil {
			out[k] = map[string]time.Time{}
		}
		out[k][mark] = t
	}
	return out, rows.Err()
}

// Thumb is the profile's thumb on a title: "up", "down" or "".
func (r *Repo) Thumb(profileID int64, kind string, tmdbID int) string {
	var mark string
	_ = r.db.QueryRow(`SELECT mark FROM title_marks WHERE profile_id = ? AND kind = ? AND tmdb_id = ? AND mark IN ('up', 'down')`, profileID, kind, tmdbID).Scan(&mark)
	return mark
}

// Marked is the titles profileID marked with mark, newest first.
func (r *Repo) Marked(profileID int64, mark string) ([]MarkedTitle, error) {
	rows, err := r.db.Query(`SELECT kind, tmdb_id, at FROM title_marks WHERE profile_id = ? AND mark = ? ORDER BY at DESC`, profileID, mark)
	if err != nil {
		return nil, fmt.Errorf("read marks: %w", err)
	}
	defer rows.Close()
	var out []MarkedTitle
	for rows.Next() {
		var m MarkedTitle
		var at string
		if err := rows.Scan(&m.Kind, &m.TMDBID, &at); err != nil {
			return nil, err
		}
		m.At, _ = time.Parse(time.RFC3339Nano, at)
		out = append(out, m)
	}
	return out, rows.Err()
}

// MarkedTitle is one title with a mark.
type MarkedTitle struct {
	Kind   string
	TMDBID int
	At     time.Time
}
