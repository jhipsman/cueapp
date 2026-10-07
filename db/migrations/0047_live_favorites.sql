-- Live TV: the channels each profile starred, by the provider's stream id.
CREATE TABLE live_favorites (
    profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    stream_id  TEXT    NOT NULL,
    added_at   TEXT    NOT NULL,
    PRIMARY KEY (profile_id, stream_id)
);
