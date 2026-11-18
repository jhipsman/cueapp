-- Favorite sports teams, per profile: their games go first in Sports on Live
-- TV, and with alerts on, the phone hears before kickoff.
CREATE TABLE IF NOT EXISTS sports_teams (
    profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    league     TEXT    NOT NULL,
    team_id    TEXT    NOT NULL,
    name       TEXT    NOT NULL DEFAULT '',
    logo       TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (profile_id, league, team_id)
);
