-- Problems someone reported with a title as they watched (Report a problem):
-- open until the owner answers (fixed or can't fix), with a reply the
-- reporter sees once (seen = 1 after).
CREATE TABLE reports (
    id          INTEGER PRIMARY KEY,
    profile_id  INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    kind        TEXT    NOT NULL,
    tmdb_id     INTEGER NOT NULL,
    season      INTEGER NOT NULL DEFAULT 0,
    episode     INTEGER NOT NULL DEFAULT 0,
    title       TEXT    NOT NULL,
    release     TEXT    NOT NULL DEFAULT '',
    problem     TEXT    NOT NULL,
    note        TEXT    NOT NULL DEFAULT '',
    status      TEXT    NOT NULL DEFAULT 'open',
    reply       TEXT    NOT NULL DEFAULT '',
    created_at  TEXT    NOT NULL,
    updated_at  TEXT    NOT NULL,
    seen        INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX reports_open ON reports (status, created_at);
