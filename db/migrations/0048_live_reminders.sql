-- Live TV reminders: a show a profile asked to be told about when it starts.
CREATE TABLE live_reminders (
    profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    stream_id  TEXT    NOT NULL,
    start_at   TEXT    NOT NULL, -- RFC 3339, UTC
    stop_at    TEXT    NOT NULL,
    title      TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (profile_id, stream_id, start_at)
);
