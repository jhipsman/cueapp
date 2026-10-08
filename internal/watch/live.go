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

// LiveReminder is a show a profile asked to be told about when it starts.
type LiveReminder struct {
	StreamID string    `json:"channelId"`
	Start    time.Time `json:"start"`
	Stop     time.Time `json:"stop"`
	Title    string    `json:"title"`
}

// AddLiveReminder saves a reminder (again: no change).
func (r *Repo) AddLiveReminder(profileID int64, rem LiveReminder) error {
	_, err := r.db.Exec(`INSERT INTO live_reminders (profile_id, stream_id, start_at, stop_at, title) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(profile_id, stream_id, start_at) DO UPDATE SET stop_at = excluded.stop_at, title = excluded.title, dismissed = 0`,
		profileID, rem.StreamID, rem.Start.UTC().Format(time.RFC3339), rem.Stop.UTC().Format(time.RFC3339), rem.Title)
	if err != nil {
		return fmt.Errorf("save reminder: %w", err)
	}
	return nil
}

// AddFollowReminder adds a reminder a follow found, unless there is one
// for the show already, or one was taken off.
func (r *Repo) AddFollowReminder(profileID int64, rem LiveReminder) error {
	_, err := r.db.Exec(`INSERT OR IGNORE INTO live_reminders (profile_id, stream_id, start_at, stop_at, title) VALUES (?, ?, ?, ?, ?)`,
		profileID, rem.StreamID, rem.Start.UTC().Format(time.RFC3339), rem.Stop.UTC().Format(time.RFC3339), rem.Title)
	return err
}

// RemoveLiveReminder takes a reminder off (kept as dismissed until the show
// ends, so a follow doesn't add it again).
func (r *Repo) RemoveLiveReminder(profileID int64, streamID string, start time.Time) error {
	_, err := r.db.Exec(`UPDATE live_reminders SET dismissed = 1 WHERE profile_id = ? AND stream_id = ? AND start_at = ?`,
		profileID, streamID, start.UTC().Format(time.RFC3339))
	return err
}

// LiveReminders is profileID's reminders for shows that haven't ended,
// soonest first. Ended ones are cleared on the way.
func (r *Repo) LiveReminders(profileID int64, now time.Time) ([]LiveReminder, error) {
	_, _ = r.db.Exec(`DELETE FROM live_reminders WHERE stop_at < ?`, now.UTC().Format(time.RFC3339))
	rows, err := r.db.Query(`SELECT stream_id, start_at, stop_at, title FROM live_reminders WHERE profile_id = ? AND dismissed = 0 ORDER BY start_at`, profileID)
	if err != nil {
		return nil, fmt.Errorf("list reminders: %w", err)
	}
	defer rows.Close()
	out := []LiveReminder{}
	for rows.Next() {
		var rem LiveReminder
		var start, stop string
		if err := rows.Scan(&rem.StreamID, &start, &stop, &rem.Title); err != nil {
			return nil, err
		}
		rem.Start, _ = time.Parse(time.RFC3339, start)
		rem.Stop, _ = time.Parse(time.RFC3339, stop)
		out = append(out, rem)
	}
	return out, rows.Err()
}

// LiveFollows is the words profileID follows, oldest first.
func (r *Repo) LiveFollows(profileID int64) ([]string, error) {
	rows, err := r.db.Query(`SELECT phrase FROM live_follows WHERE profile_id = ? ORDER BY created_at`, profileID)
	if err != nil {
		return nil, fmt.Errorf("list follows: %w", err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SetLiveFollow follows (on) or stops following words.
func (r *Repo) SetLiveFollow(profileID int64, phrase string, on bool) error {
	var err error
	if on {
		_, err = r.db.Exec(`INSERT OR IGNORE INTO live_follows (profile_id, phrase, created_at) VALUES (?, ?, ?)`,
			profileID, phrase, time.Now().UTC().Format(time.RFC3339))
	} else {
		_, err = r.db.Exec(`DELETE FROM live_follows WHERE profile_id = ? AND phrase = ?`, profileID, phrase)
	}
	return err
}
