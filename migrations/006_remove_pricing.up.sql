-- 006_remove_pricing.up.sql
-- Removes pricing-related columns: pricing_info from agents, tier from users.
-- The registry is open source and has no pricing model.

BEGIN;

ALTER TABLE agents DROP COLUMN IF EXISTS pricing_info;
ALTER TABLE users  DROP COLUMN IF EXISTS tier;

COMMIT;
