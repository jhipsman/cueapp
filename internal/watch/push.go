package watch

import (
	"fmt"
	"time"
)

// PushSub is a device that turned phone alerts on, for a profile.
type PushSub struct {
	ID        int64
	ProfileID int64
	Endpoint  string
	Origin    string
}

// Alert is one phone alert waiting for its device to fetch it.
type Alert struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
}

// AddPushSub turns alerts on for a device (a device moves to the profile it
// was turned on from last).
func (r *Repo) AddPushSub(profileID int64, endpoint, origin string) error {
	_, err := r.db.Exec(`INSERT INTO push_subs (profile_id, endpoint, origin, created_at) VALUES (?, ?, ?, ?)
		ON CONFLICT (endpoint) DO UPDATE SET profile_id = excluded.profile_id, origin = excluded.origin`,
		profileID, endpoint, origin, time.Now().UTC().Format(time.RFC3339))
	return err
}

// RemovePushSub turns alerts off for a device.
func (r *Repo) RemovePushSub(endpoint string) error {
	_, err := r.db.Exec(`DELETE FROM push_subs WHERE endpoint = ?`, endpoint)
	return err
}

// PushSubs is every device with alerts on.
func (r *Repo) PushSubs() ([]PushSub, error) {
	rows, err := r.db.Query(`SELECT id, profile_id, endpoint, origin FROM push_subs`)
	if err != nil {
		return nil, fmt.Errorf("read push subscriptions: %w", err)
	}
	defer rows.Close()
	var out []PushSub
	for rows.Next() {
		var s PushSub
		if err := rows.Scan(&s.ID, &s.ProfileID, &s.Endpoint, &s.Origin); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// PushSubProfile is the profile a device's alerts are for.
func (r *Repo) PushSubProfile(endpoint string) (int64, bool) {
	var pid int64
	err := r.db.QueryRow(`SELECT profile_id FROM push_subs WHERE endpoint = ?`, endpoint).Scan(&pid)
	return pid, err == nil
}

// QueueAlert keeps an alert for a profile's devices, once per key (an
// episode, a film): it reports false when the profile was told already.
func (r *Repo) QueueAlert(profileID int64, key, title, body, url string) (bool, error) {
	res, err := r.db.Exec(`INSERT OR IGNORE INTO push_sent (profile_id, key) VALUES (?, ?)`, profileID, key)
	if err != nil {
		return false, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return false, nil
	}
	_, err = r.db.Exec(`INSERT INTO push_alerts (profile_id, title, body, url, created_at) VALUES (?, ?, ?, ?, ?)`,
		profileID, title, body, url, time.Now().UTC().Format(time.RFC3339))
	return err == nil, err
}

// AlreadyTold says whether a profile has been told about key.
func (r *Repo) AlreadyTold(profileID int64, key string) bool {
	var one int
	return r.db.QueryRow(`SELECT 1 FROM push_sent WHERE profile_id = ? AND key = ?`, profileID, key).Scan(&one) == nil
}

// TakeAlerts is a profile's alerts from the last two days not fetched yet;
// they count as fetched from now on.
func (r *Repo) TakeAlerts(profileID int64) ([]Alert, error) {
	since := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)
	rows, err := r.db.Query(`SELECT id, title, body, url FROM push_alerts WHERE profile_id = ? AND taken = 0 AND created_at >= ? ORDER BY id`, profileID, since)
	if err != nil {
		return nil, err
	}
	var out []Alert
	for rows.Next() {
		var a Alert
		if err := rows.Scan(&a.ID, &a.Title, &a.Body, &a.URL); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, a)
	}
	rows.Close()
	if _, err := r.db.Exec(`UPDATE push_alerts SET taken = 1 WHERE profile_id = ? AND taken = 0`, profileID); err != nil {
		return nil, err
	}
	_, _ = r.db.Exec(`DELETE FROM push_alerts WHERE created_at < ?`, since)
	return out, nil
}
