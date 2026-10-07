-- Watch: where each person stopped in each movie or episode they streamed,
-- and the titles they saved to My List. Both are per account and only hold
-- what Watch shows (title and artwork), so no TMDB lookup is needed to draw
-- the Continue Watching and My List rows.
CREATE TABLE watch_progress (
    user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind          TEXT    NOT NULL,           -- "movie" or "tv"
    tmdb_id       INTEGER NOT NULL,
    season        INTEGER NOT NULL DEFAULT 0, -- 0 for a movie
    episode       INTEGER NOT NULL DEFAULT 0,
    position_sec  REAL    NOT NULL DEFAULT 0,
    duration_sec  REAL    NOT NULL DEFAULT 0,
    title         TEXT    NOT NULL DEFAULT '',
    episode_title TEXT    NOT NULL DEFAULT '',
    poster_path   TEXT    NOT NULL DEFAULT '',
    backdrop_path TEXT    NOT NULL DEFAULT '',
    updated_at    TEXT    NOT NULL,           -- RFC 3339
    PRIMARY KEY (user_id, kind, tmdb_id, season, episode)
);
CREATE INDEX watch_progress_recent ON watch_progress (user_id, updated_at);

CREATE TABLE watch_list (
    user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind          TEXT    NOT NULL,           -- "movie" or "tv"
    tmdb_id       INTEGER NOT NULL,
    title         TEXT    NOT NULL DEFAULT '',
    year          INTEGER NOT NULL DEFAULT 0,
    poster_path   TEXT    NOT NULL DEFAULT '',
    backdrop_path TEXT    NOT NULL DEFAULT '',
    added_at      TEXT    NOT NULL,           -- RFC 3339
    PRIMARY KEY (user_id, kind, tmdb_id)
);
