package watch

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// SkipMark is a learned intro (Start to End seconds) or credits (starting
// End seconds before the end) for a show's season or a movie.
type SkipMark struct {
	Start, End float64
	UpdatedAt  time.Time
}

// SetSkipMark keeps a learned mark, replacing the one before.
func (r *Repo) SetSkipMark(kind string, tmdbID, season int, segment string, start, end float64) error {
	_, err := r.db.Exec(`INSERT INTO skip_marks (kind, tmdb_id, season, segment, start_sec, end_sec, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(kind, tmdb_id, season, segment) DO UPDATE SET start_sec = excluded.start_sec, end_sec = excluded.end_sec, updated_at = excluded.updated_at`,
		kind, tmdbID, season, segment, start, end, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("save skip mark: %w", err)
	}
	return nil
}

// SkipMarkFor is the learned mark for the season, or else the show's most
// recent one from another season (intros rarely change); ok false when none.
func (r *Repo) SkipMarkFor(kind string, tmdbID, season int, segment string) (SkipMark, bool, error) {
	var m SkipMark
	var at string
	err := r.db.QueryRow(`SELECT start_sec, end_sec, updated_at FROM skip_marks WHERE kind = ? AND tmdb_id = ? AND segment = ?
		ORDER BY (season = ?) DESC, updated_at DESC LIMIT 1`, kind, tmdbID, segment, season).Scan(&m.Start, &m.End, &at)
	if errors.Is(err, sql.ErrNoRows) {
		return SkipMark{}, false, nil
	}
	if err != nil {
		return SkipMark{}, false, err
	}
	m.UpdatedAt, _ = time.Parse(time.RFC3339, at)
	return m, true, nil
}
