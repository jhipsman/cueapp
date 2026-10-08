-- Live TV recordings: a show Cue records on the server (from its start to
-- just after its end) to watch later. Shared by the household; profile_id
-- is who asked.
CREATE TABLE live_recordings (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    stream_id  TEXT    NOT NULL,
    channel    TEXT    NOT NULL DEFAULT '',
    title      TEXT    NOT NULL DEFAULT '',
    start_at   TEXT    NOT NULL, -- RFC 3339, UTC
    stop_at    TEXT    NOT NULL,
    status     TEXT    NOT NULL DEFAULT 'scheduled', -- scheduled, recording, done, failed
    file       TEXT    NOT NULL DEFAULT '',          -- the finished MP4, in the recordings folder
    size       INTEGER NOT NULL DEFAULT 0,
    problem    TEXT    NOT NULL DEFAULT '',
    created_at TEXT    NOT NULL
);
CREATE UNIQUE INDEX live_recordings_show ON live_recordings (stream_id, start_at);

-- Live TV follows: words a profile follows (a team, a show); upcoming shows
-- with them in the title get a reminder by themselves.
CREATE TABLE live_follows (
    profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    phrase     TEXT    NOT NULL,
    created_at TEXT    NOT NULL,
    PRIMARY KEY (profile_id, phrase)
);

-- A reminder taken off is kept as dismissed, so a follow doesn't put it back.
ALTER TABLE live_reminders ADD COLUMN dismissed INTEGER NOT NULL DEFAULT 0;
