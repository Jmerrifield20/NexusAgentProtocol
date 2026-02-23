BEGIN;

CREATE TABLE IF NOT EXISTS registered_registries (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    trust_root      TEXT        UNIQUE NOT NULL,
    endpoint_url    TEXT        NOT NULL,
    intermediate_ca TEXT        NOT NULL DEFAULT '',
    status          TEXT        NOT NULL DEFAULT 'pending'
                                CHECK (status IN ('pending','active','suspended')),
    registered_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS registered_registries_status_idx
    ON registered_registries (status);

COMMIT;
