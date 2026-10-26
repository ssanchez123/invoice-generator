BEGIN;

ALTER TABLE subscriptions
    ADD COLUMN billing_day SMALLINT;

UPDATE subscriptions
SET billing_day = EXTRACT(DAY FROM next_billing_date AT TIME ZONE 'America/Bogota')::SMALLINT;

ALTER TABLE subscriptions
    ALTER COLUMN billing_day SET NOT NULL,
    ADD CONSTRAINT subscriptions_billing_day_check
        CHECK (billing_day BETWEEN 1 AND 31);

CREATE INDEX idx_subscriptions_tenant_created
    ON subscriptions (tenant_id, created_at DESC, id DESC);

COMMIT;
