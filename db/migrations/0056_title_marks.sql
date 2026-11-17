-- What a profile has said about a title: "hidden" (taken off Continue
-- Watching; it comes back when watched again), "up" (liked) or "down" (not
-- for me: kept off Home) and "remind" (tell me when it's out to stream).
-- "up" and "down" are exclusive.
CREATE TABLE title_marks (
    profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    kind       TEXT    NOT NULL,
    tmdb_id    INTEGER NOT NULL,
    mark       TEXT    NOT NULL,
    at         TEXT    NOT NULL,
    PRIMARY KEY (profile_id, kind, tmdb_id, mark)
);
