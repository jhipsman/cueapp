-- Where a show's intro and credits are, learned from what the household
-- skips (a jump over the intro, Next episode during the credits), for
-- skip buttons when TheIntroDB has nothing. Per show and season; a movie's
-- credits under season 0.
CREATE TABLE skip_marks (
    kind       TEXT    NOT NULL,
    tmdb_id    INTEGER NOT NULL,
    season     INTEGER NOT NULL,
    segment    TEXT    NOT NULL,  -- 'intro' or 'credits'
    start_sec  REAL    NOT NULL,  -- intro: where it starts
    end_sec    REAL    NOT NULL,  -- intro: where it ends; credits: seconds before the end they start
    updated_at TEXT    NOT NULL,
    PRIMARY KEY (kind, tmdb_id, season, segment)
);
