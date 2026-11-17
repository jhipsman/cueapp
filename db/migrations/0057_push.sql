-- Phone alerts (web push). push_subs: each device that turned alerts on, for
-- a profile; origin is the Cue address it was turned on from (named in the
-- push's VAPID claims). push_alerts: alerts waiting for the device to fetch
-- them (a push carries no text; the device asks Cue). push_sent: what a
-- profile has been told about, so nothing is sent twice.
CREATE TABLE push_subs (
    id         INTEGER PRIMARY KEY,
    profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    endpoint   TEXT    NOT NULL UNIQUE,
    origin     TEXT    NOT NULL,
    created_at TEXT    NOT NULL
);
CREATE TABLE push_alerts (
    id         INTEGER PRIMARY KEY,
    profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    title      TEXT    NOT NULL,
    body       TEXT    NOT NULL,
    url        TEXT    NOT NULL,
    created_at TEXT    NOT NULL,
    taken      INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE push_sent (
    profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    key        TEXT    NOT NULL,
    PRIMARY KEY (profile_id, key)
);
