-- Live TV, per profile: its own channel lists (named, in its own order),
-- channels it renamed, and how often it watches each channel (Live TV's
-- "For you").
CREATE TABLE live_lists (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    profile_id  INTEGER NOT NULL,
    name        TEXT NOT NULL,
    items       TEXT NOT NULL DEFAULT '[]', -- JSON list of channel ids, in the list's order
    created_at  TEXT NOT NULL
);
CREATE INDEX live_lists_profile ON live_lists (profile_id);

CREATE TABLE live_names (
    profile_id  INTEGER NOT NULL,
    stream_id   TEXT NOT NULL,
    name        TEXT NOT NULL,
    PRIMARY KEY (profile_id, stream_id)
);

CREATE TABLE live_watched (
    profile_id  INTEGER NOT NULL,
    stream_id   TEXT NOT NULL,
    plays       INTEGER NOT NULL DEFAULT 0,
    last_at     TEXT NOT NULL,
    PRIMARY KEY (profile_id, stream_id)
);
