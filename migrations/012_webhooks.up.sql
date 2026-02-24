-- 012: Webhook subscriptions and delivery tracking

CREATE TABLE IF NOT EXISTS webhook_subscriptions (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    url        TEXT        NOT NULL,
    events     TEXT[]      NOT NULL DEFAULT '{}',
    secret     TEXT        NOT NULL,
    active     BOOLEAN     NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id UUID        NOT NULL REFERENCES webhook_subscriptions(id) ON DELETE CASCADE,
    event_type      TEXT        NOT NULL,
    payload         JSONB       NOT NULL DEFAULT '{}',
    status_code     INT         NOT NULL DEFAULT 0,
    attempt         INT         NOT NULL DEFAULT 1,
    success         BOOLEAN     NOT NULL DEFAULT false,
    error_message   TEXT        NOT NULL DEFAULT '',
    delivered_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_webhook_subscriptions_user_id ON webhook_subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_sub_id     ON webhook_deliveries(subscription_id);
