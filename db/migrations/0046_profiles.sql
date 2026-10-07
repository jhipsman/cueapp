-- Watch profiles: one household account, a profile per person (Netflix
-- style). Progress and My List move from the account to the profile; what
-- each account had goes to its main profile, made here for every account.
CREATE TABLE profiles (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name       TEXT    NOT NULL,
    avatar     TEXT    NOT NULL DEFAULT '',  -- a color name from the picker
    pin_hash   TEXT    NOT NULL DEFAULT '',  -- bcrypt of the PIN; '' = none
    secret     TEXT    NOT NULL DEFAULT '',  -- changes with the PIN, so old sign-ins to the profile stop working
    is_main    INTEGER NOT NULL DEFAULT 0,   -- the account owner's: the only one that may change settings
    created_at TEXT    NOT NULL
);
CREATE INDEX profiles_user ON profiles (user_id);

INSERT INTO profiles (user_id, name, avatar, is_main, secret, created_at)
SELECT id, username, 'teal', 1, lower(hex(randomblob(16))), strftime('%Y-%m-%dT%H:%M:%SZ', 'now') FROM users;

CREATE TABLE watch_progress_p (
    profile_id    INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    kind          TEXT    NOT NULL,
    tmdb_id       INTEGER NOT NULL,
    season        INTEGER NOT NULL DEFAULT 0,
    episode       INTEGER NOT NULL DEFAULT 0,
    position_sec  REAL    NOT NULL DEFAULT 0,
    duration_sec  REAL    NOT NULL DEFAULT 0,
    title         TEXT    NOT NULL DEFAULT '',
    episode_title TEXT    NOT NULL DEFAULT '',
    poster_path   TEXT    NOT NULL DEFAULT '',
    backdrop_path TEXT    NOT NULL DEFAULT '',
    updated_at    TEXT    NOT NULL,
    PRIMARY KEY (profile_id, kind, tmdb_id, season, episode)
);
INSERT INTO watch_progress_p
SELECT p.id, w.kind, w.tmdb_id, w.season, w.episode, w.position_sec, w.duration_sec, w.title, w.episode_title, w.poster_path, w.backdrop_path, w.updated_at
FROM watch_progress w JOIN profiles p ON p.user_id = w.user_id AND p.is_main = 1;
DROP TABLE watch_progress;
ALTER TABLE watch_progress_p RENAME TO watch_progress;
CREATE INDEX watch_progress_recent ON watch_progress (profile_id, updated_at);

CREATE TABLE watch_list_p (
    profile_id    INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    kind          TEXT    NOT NULL,
    tmdb_id       INTEGER NOT NULL,
    title         TEXT    NOT NULL DEFAULT '',
    year          INTEGER NOT NULL DEFAULT 0,
    poster_path   TEXT    NOT NULL DEFAULT '',
    backdrop_path TEXT    NOT NULL DEFAULT '',
    added_at      TEXT    NOT NULL,
    PRIMARY KEY (profile_id, kind, tmdb_id)
);
INSERT INTO watch_list_p
SELECT p.id, w.kind, w.tmdb_id, w.title, w.year, w.poster_path, w.backdrop_path, w.added_at
FROM watch_list w JOIN profiles p ON p.user_id = w.user_id AND p.is_main = 1;
DROP TABLE watch_list;
ALTER TABLE watch_list_p RENAME TO watch_list;
