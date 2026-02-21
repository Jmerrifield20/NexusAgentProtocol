-- 001_init.sql — Initial Nexus registry schema
-- Applied by golang-migrate

BEGIN;

-- agents: core registry table
CREATE TABLE IF NOT EXISTS agents (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    trust_root      TEXT        NOT NULL,
    capability_node TEXT        NOT NULL,
    agent_id        TEXT        NOT NULL,
    display_name    TEXT        NOT NULL,
    description     TEXT        NOT NULL DEFAULT '',
    endpoint        TEXT        NOT NULL,
    owner_domain    TEXT        NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'pending'
                                CHECK (status IN ('pending', 'active', 'revoked', 'expired')),
    cert_serial     TEXT        NOT NULL DEFAULT '',
    public_key_pem  TEXT        NOT NULL DEFAULT '',
    metadata        JSONB       NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ
);

-- Unique constraint: one agent per (trust_root, capability_node, agent_id)
CREATE UNIQUE INDEX IF NOT EXISTS agents_uri_idx
    ON agents (trust_root, capability_node, agent_id);

-- Index for fast domain-scoped lookups
CREATE INDEX IF NOT EXISTS agents_owner_domain_idx ON agents (owner_domain);
CREATE INDEX IF NOT EXISTS agents_status_idx ON agents (status);
CREATE INDEX IF NOT EXISTS agents_created_at_idx ON agents (created_at DESC);

-- certificates: X.509 certificates issued to agents
CREATE TABLE IF NOT EXISTS certificates (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id    UUID        NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    serial      TEXT        NOT NULL UNIQUE,
    pem         TEXT        NOT NULL,
    issued_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS certificates_agent_id_idx ON certificates (agent_id);

-- trust_ledger: append-only Merkle-chain audit log
CREATE TABLE IF NOT EXISTS trust_ledger (
    idx         BIGSERIAL   PRIMARY KEY,
    timestamp   TIMESTAMPTZ NOT NULL DEFAULT now(),
    agent_uri   TEXT        NOT NULL DEFAULT '',
    action      TEXT        NOT NULL,  -- register, activate, revoke, update, genesis
    actor       TEXT        NOT NULL DEFAULT '',
    data_hash   TEXT        NOT NULL,
    prev_hash   TEXT        NOT NULL,
    hash        TEXT        NOT NULL UNIQUE
);

CREATE INDEX IF NOT EXISTS trust_ledger_agent_uri_idx ON trust_ledger (agent_uri);
CREATE INDEX IF NOT EXISTS trust_ledger_timestamp_idx ON trust_ledger (timestamp DESC);

-- Insert genesis ledger entry
INSERT INTO trust_ledger (agent_uri, action, actor, data_hash, prev_hash, hash)
VALUES (
    '',
    'genesis',
    'nexus-system',
    '0000000000000000000000000000000000000000000000000000000000000000',
    '0000000000000000000000000000000000000000000000000000000000000000',
    '0000000000000000000000000000000000000000000000000000000000000000'
)
ON CONFLICT DO NOTHING;

-- dns_challenges: DNS-01 domain ownership challenges
CREATE TABLE IF NOT EXISTS dns_challenges (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    domain      TEXT        NOT NULL,
    token       TEXT        NOT NULL,
    txt_record  TEXT        NOT NULL,
    verified    BOOLEAN     NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS dns_challenges_domain_idx ON dns_challenges (domain);

COMMIT;
