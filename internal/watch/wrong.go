package watch

import (
	"fmt"
	"time"
)

// MarkWrong records that release is the wrong video for a title (a show's
// episode, or a film with season and episode 0), for the whole household.
func (r *Repo) MarkWrong(kind string, tmdbID, season, episode int, release string) error {
	_, err := r.db.Exec(`INSERT OR IGNORE INTO wrong_releases (kind, tmdb_id, season, episode, release, at) VALUES (?, ?, ?, ?, ?, ?)`,
		kind, tmdbID, season, episode, release, time.Now().UTC().Format(time.RFC3339))
	return err
}

// WrongReleases is the releases marked wrong for a title.
func (r *Repo) WrongReleases(kind string, tmdbID, season, episode int) (map[string]bool, error) {
	rows, err := r.db.Query(`SELECT release FROM wrong_releases WHERE kind = ? AND tmdb_id = ? AND season = ? AND episode = ?`, kind, tmdbID, season, episode)
	if err != nil {
		return nil, fmt.Errorf("read wrong releases: %w", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var rel string
		if err := rows.Scan(&rel); err != nil {
			return nil, err
		}
		out[rel] = true
	}
	return out, rows.Err()
}
