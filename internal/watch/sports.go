package watch

// FavoriteTeam is a sports team a profile follows.
type FavoriteTeam struct {
	League string `json:"league"`
	TeamID string `json:"id"`
	Name   string `json:"name"`
	Logo   string `json:"logo,omitempty"`
}

// FavoriteTeams is a profile's teams.
func (r *Repo) FavoriteTeams(profileID int64) ([]FavoriteTeam, error) {
	rows, err := r.db.Query(`SELECT league, team_id, name, logo FROM sports_teams WHERE profile_id = ? ORDER BY league, name`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FavoriteTeam{}
	for rows.Next() {
		var t FavoriteTeam
		if err := rows.Scan(&t.League, &t.TeamID, &t.Name, &t.Logo); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// SetFavoriteTeam follows (on) or stops following a team.
func (r *Repo) SetFavoriteTeam(profileID int64, t FavoriteTeam, on bool) error {
	if !on {
		_, err := r.db.Exec(`DELETE FROM sports_teams WHERE profile_id = ? AND league = ? AND team_id = ?`, profileID, t.League, t.TeamID)
		return err
	}
	_, err := r.db.Exec(`INSERT INTO sports_teams (profile_id, league, team_id, name, logo) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(profile_id, league, team_id) DO UPDATE SET name = excluded.name, logo = excluded.logo`,
		profileID, t.League, t.TeamID, t.Name, t.Logo)
	return err
}
