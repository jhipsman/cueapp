-- Each profile's color theme for Watch: JSON {"accent":"#34d1bf",
-- "glow":"soft","background":"midnight"}; '' = Cue's own.
ALTER TABLE profiles ADD COLUMN theme TEXT NOT NULL DEFAULT '';
