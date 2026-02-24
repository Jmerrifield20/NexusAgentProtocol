-- 009: Revocation improvements — suspended status, revocation reason
-- Drops and re-adds the status check constraint to include 'suspended'.

ALTER TABLE agents DROP CONSTRAINT IF EXISTS agents_status_check;
ALTER TABLE agents ADD CONSTRAINT agents_status_check
    CHECK (status IN ('pending', 'active', 'revoked', 'expired', 'suspended'));

ALTER TABLE agents ADD COLUMN IF NOT EXISTS revocation_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE agents ADD COLUMN IF NOT EXISTS suspended_at     TIMESTAMPTZ;
