-- Versions someone said are the wrong video for a title (a different film
-- with the same name, a mislabelled file): never offered for it again.
CREATE TABLE wrong_releases (
    kind     TEXT    NOT NULL,
    tmdb_id  INTEGER NOT NULL,
    season   INTEGER NOT NULL DEFAULT 0,
    episode  INTEGER NOT NULL DEFAULT 0,
    release  TEXT    NOT NULL,
    at       TEXT    NOT NULL,
    PRIMARY KEY (kind, tmdb_id, season, episode, release)
);
