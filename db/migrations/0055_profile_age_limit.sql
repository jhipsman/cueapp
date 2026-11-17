-- A profile's age limit (kids' profiles): the highest rating it may see and
-- play, on the US film scale ("G", "PG", "PG-13", "R"); '' = no limit.
ALTER TABLE profiles ADD COLUMN max_rating TEXT NOT NULL DEFAULT '';
