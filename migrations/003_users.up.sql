-- 003_users.up.sql — User accounts, OAuth links, email verification, free-tier agent ownership

BEGIN;

CREATE TABLE users (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email          TEXT        UNIQUE NOT NULL,
    password_hash  TEXT,                   -- NULL for OAuth-only users
    display_name   TEXT        NOT NULL DEFAULT '',
    username       TEXT        UNIQUE NOT NULL,  -- slug used in agent URI namespace
    tier           TEXT        NOT NULL DEFAULT 'free' CHECK (tier IN ('free','pro','enterprise')),
    email_verified BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE user_oauth (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider    TEXT        NOT NULL,   -- 'github' | 'google'
    provider_id TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(provider, provider_id)
);

CREATE TABLE email_verifications (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token      TEXT        UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS users_email_idx ON users (email);
CREATE INDEX IF NOT EXISTS users_username_idx ON users (username);
CREATE INDEX IF NOT EXISTS email_verifications_token_idx ON email_verifications (token);

ALTER TABLE agents ADD COLUMN owner_user_id UUID REFERENCES users(id);
ALTER TABLE agents ADD COLUMN registration_type TEXT NOT NULL DEFAULT 'domain'
    CHECK (registration_type IN ('domain','nap_hosted'));

CREATE INDEX IF NOT EXISTS agents_owner_user_id_idx ON agents (owner_user_id);

COMMIT;
