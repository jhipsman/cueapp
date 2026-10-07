// Package watch keeps what Watch remembers per profile: where each person
// stopped in each movie and episode (Continue Watching, resume) and the
// titles they saved to My List.
package watch

import (
	"database/sql"
	"fmt"
	"time"
)

// Kinds of title.
const (
	KindMovie = "movie"
	KindTV    = "tv"
)

// Progress is where a person stopped in a movie or an episode.
type Progress struct {
	Kind         string
	TMDBID       int
	Season       int // 0 for a movie
	Episode      int
	Position     float64 // seconds
	Duration     float64 // seconds; 0 when the player didn't know
	Title        string
	EpisodeTitle string
	PosterPath   string
	BackdropPath string
	UpdatedAt    time.Time
}

// Fraction is how much has been watched, 0 to 1.
func (p Progress) Fraction() float64 {
	if p.Duration <= 0 {
		return 0
	}
	return min(max(p.Position/p.Duration, 0), 1)
}

// Finished says it was watched to the end (the credits don't count).
func (p Progress) Finished() bool { return p.Duration > 0 && p.Fraction() >= FinishedAt }

// FinishedAt is the share watched from which a title counts as finished.
const FinishedAt = 0.92

// ListItem is one title on My List.
type ListItem struct {
	Kind         string
	TMDBID       int
	Title        string
	Year         int
	PosterPath   string
	BackdropPath string
	AddedAt      time.Time
}

// Repo stores progress and My List.
type Repo struct{ db *sql.DB }

// NewRepo returns a Repo on db.
func NewRepo(db *sql.DB) *Repo { return &Repo{db: db} }

// SaveProgress records where profileID is in a title.
func (r *Repo) SaveProgress(profileID int64, p Progress) error {
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = time.Now()
	}
	_, err := r.db.Exec(`INSERT INTO watch_progress
		(profile_id, kind, tmdb_id, season, episode, position_sec, duration_sec, title, episode_title, poster_path, backdrop_path, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(profile_id, kind, tmdb_id, season, episode) DO UPDATE SET
		  position_sec = excluded.position_sec, duration_sec = excluded.duration_sec,
		  title = excluded.title, episode_title = excluded.episode_title,
		  poster_path = excluded.poster_path, backdrop_path = excluded.backdrop_path,
		  updated_at = excluded.updated_at`,
		profileID, p.Kind, p.TMDBID, p.Season, p.Episode, p.Position, p.Duration,
		p.Title, p.EpisodeTitle, p.PosterPath, p.BackdropPath, p.UpdatedAt.UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("save watch progress: %w", err)
	}
	return nil
}

const progressCols = `kind, tmdb_id, season, episode, position_sec, duration_sec, title, episode_title, poster_path, backdrop_path, updated_at`

func scanProgress(sc interface{ Scan(...any) error }) (Progress, error) {
	var p Progress
	var at string
	if err := sc.Scan(&p.Kind, &p.TMDBID, &p.Season, &p.Episode, &p.Position, &p.Duration, &p.Title, &p.EpisodeTitle, &p.PosterPath, &p.BackdropPath, &at); err != nil {
		return p, err
	}
	p.UpdatedAt, _ = time.Parse(time.RFC3339, at)
	return p, nil
}

// GetProgress is where profileID stopped in one movie or episode; ok is false
// if they never started it.
func (r *Repo) GetProgress(profileID int64, kind string, tmdbID, season, episode int) (Progress, bool, error) {
	p, err := scanProgress(r.db.QueryRow(`SELECT `+progressCols+` FROM watch_progress
		WHERE profile_id = ? AND kind = ? AND tmdb_id = ? AND season = ? AND episode = ?`, profileID, kind, tmdbID, season, episode))
	if err == sql.ErrNoRows {
		return Progress{}, false, nil
	}
	if err != nil {
		return Progress{}, false, fmt.Errorf("get watch progress: %w", err)
	}
	return p, true, nil
}

// ShowProgress is every episode of a show profileID has started.
func (r *Repo) ShowProgress(profileID int64, tmdbID int) ([]Progress, error) {
	return r.query(`SELECT `+progressCols+` FROM watch_progress WHERE profile_id = ? AND kind = ? AND tmdb_id = ?
		ORDER BY updated_at DESC`, profileID, KindTV, tmdbID)
}

// Recent is the latest progress per title for profileID, newest first: for a
// show, its most recently watched episode.
func (r *Repo) Recent(profileID int64, limit int) ([]Progress, error) {
	all, err := r.query(`SELECT `+progressCols+` FROM watch_progress WHERE profile_id = ? ORDER BY updated_at DESC LIMIT 500`, profileID)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []Progress
	for _, p := range all {
		k := fmt.Sprintf("%s:%d", p.Kind, p.TMDBID)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, p)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// ForgetTitle removes profileID's progress in a title (every episode of a show),
// to take it off Continue Watching.
func (r *Repo) ForgetTitle(profileID int64, kind string, tmdbID int) error {
	_, err := r.db.Exec(`DELETE FROM watch_progress WHERE profile_id = ? AND kind = ? AND tmdb_id = ?`, profileID, kind, tmdbID)
	return err
}

func (r *Repo) query(q string, args ...any) ([]Progress, error) {
	rows, err := r.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("list watch progress: %w", err)
	}
	defer rows.Close()
	var out []Progress
	for rows.Next() {
		p, err := scanProgress(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// AddToList saves a title to profileID's My List (again, it moves to the front).
func (r *Repo) AddToList(profileID int64, it ListItem) error {
	_, err := r.db.Exec(`INSERT INTO watch_list (profile_id, kind, tmdb_id, title, year, poster_path, backdrop_path, added_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(profile_id, kind, tmdb_id) DO UPDATE SET title = excluded.title, year = excluded.year,
		  poster_path = excluded.poster_path, backdrop_path = excluded.backdrop_path, added_at = excluded.added_at`,
		profileID, it.Kind, it.TMDBID, it.Title, it.Year, it.PosterPath, it.BackdropPath, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("add to my list: %w", err)
	}
	return nil
}

// RemoveFromList takes a title off profileID's My List.
func (r *Repo) RemoveFromList(profileID int64, kind string, tmdbID int) error {
	_, err := r.db.Exec(`DELETE FROM watch_list WHERE profile_id = ? AND kind = ? AND tmdb_id = ?`, profileID, kind, tmdbID)
	return err
}

// InList says whether a title is on profileID's My List.
func (r *Repo) InList(profileID int64, kind string, tmdbID int) bool {
	var n int
	_ = r.db.QueryRow(`SELECT COUNT(*) FROM watch_list WHERE profile_id = ? AND kind = ? AND tmdb_id = ?`, profileID, kind, tmdbID).Scan(&n)
	return n > 0
}

// List is profileID's My List, most recently added first.
func (r *Repo) List(profileID int64) ([]ListItem, error) {
	rows, err := r.db.Query(`SELECT kind, tmdb_id, title, year, poster_path, backdrop_path, added_at FROM watch_list
		WHERE profile_id = ? ORDER BY added_at DESC`, profileID)
	if err != nil {
		return nil, fmt.Errorf("list my list: %w", err)
	}
	defer rows.Close()
	var out []ListItem
	for rows.Next() {
		var it ListItem
		var at string
		if err := rows.Scan(&it.Kind, &it.TMDBID, &it.Title, &it.Year, &it.PosterPath, &it.BackdropPath, &at); err != nil {
			return nil, err
		}
		it.AddedAt, _ = time.Parse(time.RFC3339, at)
		out = append(out, it)
	}
	return out, rows.Err()
}
