-- Two-step sign-in (handlers_two_step.go): an authenticator app's secret
-- (encrypted), the one-time backup codes (SHA-256 hashes, comma-separated),
-- and the last 30-second step a code was used for, so a code can't be used
-- twice.
ALTER TABLE users ADD COLUMN totp_secret TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN totp_backup TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN totp_last_step INTEGER NOT NULL DEFAULT 0;
