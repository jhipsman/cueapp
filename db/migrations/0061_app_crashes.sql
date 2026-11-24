-- Crashes of Cue's own apps (the TV app, the web app), one row per kind of
-- crash: the same exception at the same place folds into one row with a
-- count, whatever the device or build. Settings > System > Problems lists
-- them; fixed_at is set when the owner marks one fixed, and cleared if it
-- comes back.
CREATE TABLE IF NOT EXISTS app_crashes (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    signature   TEXT    NOT NULL UNIQUE,
    app         TEXT    NOT NULL,
    headline    TEXT    NOT NULL DEFAULT '',  -- the exception and its message
    place       TEXT    NOT NULL DEFAULT '',  -- the first line of Cue's own code in the trace
    version     TEXT    NOT NULL DEFAULT '',  -- of the latest report
    device      TEXT    NOT NULL DEFAULT '',  -- of the latest report
    devices     TEXT    NOT NULL DEFAULT '',  -- the devices seen, one per line (at most a few)
    versions    TEXT    NOT NULL DEFAULT '',  -- the versions seen, one per line (at most a few)
    trace       TEXT    NOT NULL DEFAULT '',  -- the latest report's trace
    count       INTEGER NOT NULL DEFAULT 1,
    first_at    TEXT    NOT NULL,
    last_at     TEXT    NOT NULL,
    fixed_at    TEXT    NOT NULL DEFAULT ''
);
