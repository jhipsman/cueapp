-- An account's own settings (account_prefs.go): small switches that follow
-- it to every device, such as having seen the tour, or autoplay.
CREATE TABLE account_prefs (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key     TEXT    NOT NULL,
    value   TEXT    NOT NULL,
    PRIMARY KEY (user_id, key)
);
