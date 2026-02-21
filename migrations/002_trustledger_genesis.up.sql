-- 002_trustledger_genesis.sql
-- Fix the trust_ledger table so idx values are managed explicitly.
-- Migration 001 used BIGSERIAL which assigned idx=1 to the genesis entry.
-- The Go implementation uses idx=0 for genesis. This migration corrects that
-- and makes the genesis hash the canonical all-zeros constant.

BEGIN;

-- Remove the auto-increment default so we control idx values ourselves.
ALTER TABLE trust_ledger ALTER COLUMN idx DROP DEFAULT;
DROP SEQUENCE IF EXISTS trust_ledger_idx_seq;

-- Clear the incorrect genesis inserted by migration 001 (had idx=1).
DELETE FROM trust_ledger;

-- Insert the canonical genesis at idx=0 with the well-known all-zeros hash.
-- The genesis hash is a trust anchor; it is NOT computed from its fields.
INSERT INTO trust_ledger (idx, timestamp, agent_uri, action, actor, data_hash, prev_hash, hash)
VALUES (
    0,
    now(),
    '',
    'genesis',
    'nexus-system',
    '0000000000000000000000000000000000000000000000000000000000000000',
    '0000000000000000000000000000000000000000000000000000000000000000',
    '0000000000000000000000000000000000000000000000000000000000000000'
);

COMMIT;
