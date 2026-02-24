-- 010: Deprecation — deprecated status, sunset columns

ALTER TABLE agents DROP CONSTRAINT IF EXISTS agents_status_check;
ALTER TABLE agents ADD CONSTRAINT agents_status_check
    CHECK (status IN ('pending', 'active', 'revoked', 'expired', 'suspended', 'deprecated'));

ALTER TABLE agents ADD COLUMN IF NOT EXISTS deprecated_at   TIMESTAMPTZ;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS sunset_date     DATE;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS replacement_uri TEXT NOT NULL DEFAULT '';
