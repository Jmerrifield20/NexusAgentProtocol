-- 005_agent_extended_fields.up.sql
-- Adds richer metadata columns to the agents table:
--   version       — agent version string (e.g. "1.0.0")
--   tags          — free-form keyword array for search/filter
--   support_url   — contact / support URL
--   pricing_info  — free-form pricing or rate-limit description
--   last_seen_at  — timestamp of last successful health check (system-managed)
--   health_status — current health state (system-managed)

BEGIN;

ALTER TABLE agents
    ADD COLUMN IF NOT EXISTS version      TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS tags         TEXT[]      NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS support_url  TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS pricing_info TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS health_status TEXT       NOT NULL DEFAULT 'unknown'
        CHECK (health_status IN ('healthy', 'degraded', 'unknown'));

CREATE INDEX IF NOT EXISTS agents_tags_idx ON agents USING GIN (tags);
CREATE INDEX IF NOT EXISTS agents_health_status_idx ON agents (health_status);

COMMIT;
