package watch

import (
	"fmt"
	"time"
)

// LiveFavorites is the set of Live TV channels (the provider's stream ids)
// profileID starred.
func (r *Repo) LiveFavorites(profileID int64) (map[string]bool, error) {
	rows, err := r.db.Query(`SELECT stream_id FROM live_favorites WHERE profile_id = ?`, profileID)
	if err != nil {
		return nil, fmt.Errorf("list live favorites: %w", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// SetLiveFavorite stars or unstars a channel for profileID.
func (r *Repo) SetLiveFavorite(profileID int64, streamID string, on bool) error {
	var err error
	if on {
		_, err = r.db.Exec(`INSERT INTO live_favorites (profile_id, stream_id, added_at) VALUES (?, ?, ?)
			ON CONFLICT(profile_id, stream_id) DO NOTHING`, profileID, streamID, time.Now().UTC().Format(time.RFC3339))
	} else {
		_, err = r.db.Exec(`DELETE FROM live_favorites WHERE profile_id = ? AND stream_id = ?`, profileID, streamID)
	}
	if err != nil {
		return fmt.Errorf("save live favorite: %w", err)
	}
	return nil
}
