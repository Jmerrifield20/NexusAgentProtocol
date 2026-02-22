-- 004_account_recovery.up.sql
-- Adds token_type to email_verifications so the same table serves both
-- email verification and password-reset flows, and adds password_reset_tokens
-- index for fast lookup.

BEGIN;

ALTER TABLE email_verifications
    ADD COLUMN IF NOT EXISTS token_type TEXT NOT NULL DEFAULT 'email_verification'
        CHECK (token_type IN ('email_verification', 'password_reset'));

-- Backfill existing rows (all are email verifications)
UPDATE email_verifications SET token_type = 'email_verification' WHERE token_type IS NULL;

CREATE INDEX IF NOT EXISTS email_verifications_type_idx ON email_verifications (token_type);

COMMIT;
