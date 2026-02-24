-- 011: Abuse reporting table

CREATE TABLE IF NOT EXISTS abuse_reports (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id         UUID        NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    reporter_user_id UUID        NOT NULL REFERENCES users(id),
    reason           TEXT        NOT NULL,
    details          TEXT        NOT NULL DEFAULT '',
    status           TEXT        NOT NULL DEFAULT 'open'
        CHECK (status IN ('open', 'investigating', 'resolved', 'dismissed')),
    resolution_note  TEXT        NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at      TIMESTAMPTZ,
    resolved_by      UUID        REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_abuse_reports_agent_id   ON abuse_reports(agent_id);
CREATE INDEX IF NOT EXISTS idx_abuse_reports_status     ON abuse_reports(status);
CREATE INDEX IF NOT EXISTS idx_abuse_reports_created_at ON abuse_reports(created_at DESC);
