package watch

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Recording states.
const (
	RecScheduled = "scheduled"
	RecRecording = "recording"
	RecDone      = "done"
	RecFailed    = "failed"
)

// Recording is a Live TV show Cue records (or recorded) on the server.
type Recording struct {
	ID        int64     `json:"id"`
	ProfileID int64     `json:"-"`
	StreamID  string    `json:"channelId"`
	Channel   string    `json:"channel"`
	Title     string    `json:"title"`
	Start     time.Time `json:"start"`
	Stop      time.Time `json:"stop"`
	Status    string    `json:"status"`
	File      string    `json:"-"`
	Size      int64     `json:"size"`
	Problem   string    `json:"problem,omitempty"`
	Created   time.Time `json:"created"`
}

// ErrRecordingNotFound is returned for a recording that doesn't exist.
var ErrRecordingNotFound = errors.New("that recording doesn't exist")

const recCols = `id, profile_id, stream_id, channel, title, start_at, stop_at, status, file, size, problem, created_at`

func scanRecording(sc interface{ Scan(...any) error }) (Recording, error) {
	var rec Recording
	var start, stop, created string
	if err := sc.Scan(&rec.ID, &rec.ProfileID, &rec.StreamID, &rec.Channel, &rec.Title, &start, &stop, &rec.Status, &rec.File, &rec.Size, &rec.Problem, &created); err != nil {
		return rec, err
	}
	rec.Start, _ = time.Parse(time.RFC3339, start)
	rec.Stop, _ = time.Parse(time.RFC3339, stop)
	rec.Created, _ = time.Parse(time.RFC3339, created)
	return rec, nil
}

// AddRecording schedules a show; the same show again changes nothing.
func (r *Repo) AddRecording(rec Recording) (Recording, error) {
	_, err := r.db.Exec(`INSERT INTO live_recordings (profile_id, stream_id, channel, title, start_at, stop_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(stream_id, start_at) DO NOTHING`,
		rec.ProfileID, rec.StreamID, rec.Channel, rec.Title, rec.Start.UTC().Format(time.RFC3339), rec.Stop.UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return Recording{}, fmt.Errorf("schedule recording: %w", err)
	}
	return scanRecording(r.db.QueryRow(`SELECT `+recCols+` FROM live_recordings WHERE stream_id = ? AND start_at = ?`, rec.StreamID, rec.Start.UTC().Format(time.RFC3339)))
}

// Recordings is every recording, newest show first.
func (r *Repo) Recordings() ([]Recording, error) {
	return r.recordings(`SELECT ` + recCols + ` FROM live_recordings ORDER BY start_at DESC`)
}

// RecordingsDue is the scheduled ones starting before by that haven't ended.
func (r *Repo) RecordingsDue(by, now time.Time) ([]Recording, error) {
	return r.recordings(`SELECT `+recCols+` FROM live_recordings WHERE status = ? AND start_at <= ? AND stop_at > ? ORDER BY start_at`,
		RecScheduled, by.UTC().Format(time.RFC3339), now.UTC().Format(time.RFC3339))
}

func (r *Repo) recordings(q string, args ...any) ([]Recording, error) {
	rows, err := r.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("list recordings: %w", err)
	}
	defer rows.Close()
	out := []Recording{}
	for rows.Next() {
		rec, err := scanRecording(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// Recording is one recording.
func (r *Repo) Recording(id int64) (Recording, error) {
	rec, err := scanRecording(r.db.QueryRow(`SELECT `+recCols+` FROM live_recordings WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Recording{}, ErrRecordingNotFound
	}
	return rec, err
}

// SetRecordingState records how a recording is going.
func (r *Repo) SetRecordingState(id int64, status, file string, size int64, problem string) error {
	_, err := r.db.Exec(`UPDATE live_recordings SET status = ?, file = ?, size = ?, problem = ? WHERE id = ?`, status, file, size, problem, id)
	return err
}

// DeleteRecording forgets a recording (its file is the caller's to remove).
func (r *Repo) DeleteRecording(id int64) error {
	_, err := r.db.Exec(`DELETE FROM live_recordings WHERE id = ?`, id)
	return err
}

// InterruptedRecordings is the ones left "recording" (Cue stopped during
// them).
func (r *Repo) InterruptedRecordings() ([]Recording, error) {
	return r.recordings(`SELECT `+recCols+` FROM live_recordings WHERE status = ?`, RecRecording)
}

// MissedRecordings is the scheduled ones whose show ended without them.
func (r *Repo) MissedRecordings(now time.Time) ([]Recording, error) {
	return r.recordings(`SELECT `+recCols+` FROM live_recordings WHERE status = ? AND stop_at <= ?`, RecScheduled, now.UTC().Format(time.RFC3339))
}
