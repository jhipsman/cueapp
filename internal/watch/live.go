package watch

import (
	"encoding/json"
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
	// Switch: at the start, change to the channel by itself (not only say so).
	Switch bool `json:"switch,omitempty"`
}

// AddLiveReminder saves a reminder (again: no change).
func (r *Repo) AddLiveReminder(profileID int64, rem LiveReminder) error {
	_, err := r.db.Exec(`INSERT INTO live_reminders (profile_id, stream_id, start_at, stop_at, title, switch_to) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(profile_id, stream_id, start_at) DO UPDATE SET stop_at = excluded.stop_at, title = excluded.title, switch_to = excluded.switch_to, dismissed = 0`,
		profileID, rem.StreamID, rem.Start.UTC().Format(time.RFC3339), rem.Stop.UTC().Format(time.RFC3339), rem.Title, rem.Switch)
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
	rows, err := r.db.Query(`SELECT stream_id, start_at, stop_at, title, switch_to FROM live_reminders WHERE profile_id = ? AND dismissed = 0 ORDER BY start_at`, profileID)
	if err != nil {
		return nil, fmt.Errorf("list reminders: %w", err)
	}
	defer rows.Close()
	out := []LiveReminder{}
	for rows.Next() {
		var rem LiveReminder
		var start, stop string
		if err := rows.Scan(&rem.StreamID, &start, &stop, &rem.Title, &rem.Switch); err != nil {
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

// ---- A profile's own channel lists, names and habits ----

// LiveList is one of a profile's own channel lists, in its own order.
type LiveList struct {
	ID    int64    `json:"id"`
	Name  string   `json:"name"`
	Items []string `json:"items"`
}

// LiveLists is profileID's lists, oldest first.
func (r *Repo) LiveLists(profileID int64) ([]LiveList, error) {
	rows, err := r.db.Query(`SELECT id, name, items FROM live_lists WHERE profile_id = ? ORDER BY id`, profileID)
	if err != nil {
		return nil, fmt.Errorf("list channel lists: %w", err)
	}
	defer rows.Close()
	out := []LiveList{}
	for rows.Next() {
		var l LiveList
		var items string
		if err := rows.Scan(&l.ID, &l.Name, &items); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(items), &l.Items)
		if l.Items == nil {
			l.Items = []string{}
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// CreateLiveList makes a list (with these channels, in order).
func (r *Repo) CreateLiveList(profileID int64, name string, items []string) (LiveList, error) {
	if items == nil {
		items = []string{}
	}
	b, _ := json.Marshal(items)
	res, err := r.db.Exec(`INSERT INTO live_lists (profile_id, name, items, created_at) VALUES (?, ?, ?, ?)`,
		profileID, name, string(b), time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return LiveList{}, fmt.Errorf("make channel list: %w", err)
	}
	id, _ := res.LastInsertId()
	return LiveList{ID: id, Name: name, Items: items}, nil
}

// UpdateLiveList renames a list and/or sets its channels (nil leaves each as it is).
func (r *Repo) UpdateLiveList(profileID, id int64, name *string, items []string) error {
	if name != nil {
		if _, err := r.db.Exec(`UPDATE live_lists SET name = ? WHERE id = ? AND profile_id = ?`, *name, id, profileID); err != nil {
			return err
		}
	}
	if items != nil {
		b, _ := json.Marshal(items)
		if _, err := r.db.Exec(`UPDATE live_lists SET items = ? WHERE id = ? AND profile_id = ?`, string(b), id, profileID); err != nil {
			return err
		}
	}
	return nil
}

// DeleteLiveList removes a list.
func (r *Repo) DeleteLiveList(profileID, id int64) error {
	_, err := r.db.Exec(`DELETE FROM live_lists WHERE id = ? AND profile_id = ?`, id, profileID)
	return err
}

// LiveNames is the channels profileID renamed: channel id -> its name.
func (r *Repo) LiveNames(profileID int64) (map[string]string, error) {
	rows, err := r.db.Query(`SELECT stream_id, name FROM live_names WHERE profile_id = ?`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = name
	}
	return out, rows.Err()
}

// SetLiveName renames a channel for profileID ("" gives it back its own name).
func (r *Repo) SetLiveName(profileID int64, streamID, name string) error {
	if name == "" {
		_, err := r.db.Exec(`DELETE FROM live_names WHERE profile_id = ? AND stream_id = ?`, profileID, streamID)
		return err
	}
	_, err := r.db.Exec(`INSERT INTO live_names (profile_id, stream_id, name) VALUES (?, ?, ?)
		ON CONFLICT(profile_id, stream_id) DO UPDATE SET name = excluded.name`, profileID, streamID, name)
	return err
}

// NoteLiveWatch counts a channel played by profileID.
func (r *Repo) NoteLiveWatch(profileID int64, streamID string, at time.Time) error {
	_, err := r.db.Exec(`INSERT INTO live_watched (profile_id, stream_id, plays, last_at) VALUES (?, ?, 1, ?)
		ON CONFLICT(profile_id, stream_id) DO UPDATE SET plays = plays + 1, last_at = excluded.last_at`,
		profileID, streamID, at.UTC().Format(time.RFC3339))
	return err
}

// LiveMostWatched is the channels profileID watches most (in the last 60
// days), most first.
func (r *Repo) LiveMostWatched(profileID int64, limit int) ([]string, error) {
	since := time.Now().Add(-60 * 24 * time.Hour).UTC().Format(time.RFC3339)
	rows, err := r.db.Query(`SELECT stream_id FROM live_watched WHERE profile_id = ? AND last_at >= ? ORDER BY plays DESC, last_at DESC LIMIT ?`, profileID, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
