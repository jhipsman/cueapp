-- Each profile's streaming quality preferences: JSON {"quality":"4k",
-- "size":"smart","remux":"avoid",...}; '' = Cue's defaults.
ALTER TABLE profiles ADD COLUMN stream_prefs TEXT NOT NULL DEFAULT '';
