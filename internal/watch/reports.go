package watch

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Report is a problem someone reported with a title.
type Report struct {
	ID        int64     `json:"id"`
	ProfileID int64     `json:"-"`
	Who       string    `json:"who"`    // the reporter's profile name
	Avatar    string    `json:"avatar"` // and picture
	Kind      string    `json:"kind"`
	TMDBID    int       `json:"tmdbId"`
	Season    int       `json:"season,omitempty"`
	Episode   int       `json:"episode,omitempty"`
	Title     string    `json:"title"`
	Release   string    `json:"release,omitempty"`
	Problem   string    `json:"problem"` // sound, subtitles, picture, wrong, buffering, other
	Note      string    `json:"note,omitempty"`
	Status    string    `json:"status"` // open, fixed, wontfix
	Reply     string    `json:"reply,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Problems a report can be about.
var Problems = map[string]bool{"sound": true, "subtitles": true, "picture": true, "wrong": true, "buffering": true, "other": true}

// ErrReportNotFound is a report that doesn't exist.
var ErrReportNotFound = errors.New("that report doesn't exist")

const reportCols = `r.id, r.profile_id, p.name, p.avatar, r.kind, r.tmdb_id, r.season, r.episode, r.title, r.release, r.problem, r.note, r.status, r.reply, r.created_at, r.updated_at`

func scanReport(sc interface{ Scan(...any) error }) (Report, error) {
	var rp Report
	var created, updated string
	if err := sc.Scan(&rp.ID, &rp.ProfileID, &rp.Who, &rp.Avatar, &rp.Kind, &rp.TMDBID, &rp.Season, &rp.Episode, &rp.Title, &rp.Release, &rp.Problem, &rp.Note, &rp.Status, &rp.Reply, &created, &updated); err != nil {
		return rp, err
	}
	rp.CreatedAt, _ = time.Parse(time.RFC3339, created)
	rp.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
	return rp, nil
}

// AddReport keeps a new report.
func (r *Repo) AddReport(rp Report) (Report, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.db.Exec(`INSERT INTO reports (profile_id, kind, tmdb_id, season, episode, title, release, problem, note, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'open', ?, ?)`,
		rp.ProfileID, rp.Kind, rp.TMDBID, rp.Season, rp.Episode, rp.Title, rp.Release, rp.Problem, rp.Note, now, now)
	if err != nil {
		return Report{}, fmt.Errorf("add report: %w", err)
	}
	id, _ := res.LastInsertId()
	return r.Report(id)
}

// Report is one report.
func (r *Repo) Report(id int64) (Report, error) {
	rp, err := scanReport(r.db.QueryRow(`SELECT `+reportCols+` FROM reports r JOIN profiles p ON p.id = r.profile_id WHERE r.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Report{}, ErrReportNotFound
	}
	return rp, err
}

// Reports is every report (open first, newest first), or only profileID's
// when it isn't 0.
func (r *Repo) Reports(profileID int64) ([]Report, error) {
	q := `SELECT ` + reportCols + ` FROM reports r JOIN profiles p ON p.id = r.profile_id`
	args := []any{}
	if profileID != 0 {
		q += ` WHERE r.profile_id = ?`
		args = append(args, profileID)
	}
	q += ` ORDER BY (r.status = 'open') DESC, r.created_at DESC LIMIT 200`
	rows, err := r.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("list reports: %w", err)
	}
	defer rows.Close()
	out := []Report{}
	for rows.Next() {
		rp, err := scanReport(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rp)
	}
	return out, rows.Err()
}

// ResolveReport answers a report: fixed or wontfix, with a reply the
// reporter sees once.
func (r *Repo) ResolveReport(id int64, status, reply string) (Report, error) {
	res, err := r.db.Exec(`UPDATE reports SET status = ?, reply = ?, updated_at = ?, seen = 0 WHERE id = ?`,
		status, reply, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return Report{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Report{}, ErrReportNotFound
	}
	return r.Report(id)
}

// ReportNews is a profile's answered reports it hasn't seen yet; they count
// as seen from now on.
func (r *Repo) ReportNews(profileID int64) ([]Report, error) {
	rows, err := r.db.Query(`SELECT `+reportCols+` FROM reports r JOIN profiles p ON p.id = r.profile_id
		WHERE r.profile_id = ? AND r.status != 'open' AND r.seen = 0 ORDER BY r.updated_at`, profileID)
	if err != nil {
		return nil, err
	}
	out := []Report{}
	for rows.Next() {
		rp, err := scanReport(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, rp)
	}
	rows.Close()
	_, err = r.db.Exec(`UPDATE reports SET seen = 1 WHERE profile_id = ? AND status != 'open'`, profileID)
	return out, err
}

// OpenReports is how many reports wait for an answer.
func (r *Repo) OpenReports() int {
	var n int
	_ = r.db.QueryRow(`SELECT COUNT(*) FROM reports WHERE status = 'open'`).Scan(&n)
	return n
}

// OwnerProfiles is the main profile of every administrator's account: who
// hears about new reports.
func (r *Repo) OwnerProfiles() ([]int64, error) {
	rows, err := r.db.Query(`SELECT p.id FROM profiles p JOIN users u ON u.id = p.user_id WHERE p.is_main = 1 AND u.is_admin = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
