-- A reminder can switch to its channel by itself when the show starts
-- (Live TV's "Switch to it"), instead of only saying so.
ALTER TABLE live_reminders ADD COLUMN switch_to INTEGER NOT NULL DEFAULT 0;
