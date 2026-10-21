-- Track delivery and retry state independently for every event consumer.
ALTER TABLE outbox
    ADD CONSTRAINT outbox_id_tenant_key UNIQUE (id, tenant_id);

CREATE TABLE outbox_subscriptions (
    event_type TEXT NOT NULL,
    consumer   TEXT NOT NULL,
    PRIMARY KEY (event_type, consumer)
);

INSERT INTO outbox_subscriptions (event_type, consumer) VALUES
    ('invoice.issued', 'relay-worker'),
    ('invoice.issued', 'pdf-service'),
    ('invoice.paid', 'relay-worker'),
    ('payment.recorded', 'relay-worker'),
    ('pdf.generated', 'relay-worker'),
    ('subscription.renewed', 'relay-worker');

-- Consumer routing is migration-managed, not mutable by the application role.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'invoicegen_app') THEN
        EXECUTE 'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON outbox_subscriptions FROM invoicegen_app';
        EXECUTE 'GRANT SELECT ON outbox_subscriptions TO invoicegen_app';
    END IF;
END;
$$;

CREATE TABLE outbox_deliveries (
    event_id     UUID NOT NULL,
    tenant_id    UUID NOT NULL REFERENCES tenants(id),
    consumer     TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending', 'processing', 'processed', 'failed')),
    attempts     INT NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    locked_by    TEXT,
    locked_until TIMESTAMPTZ,
    last_error   TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    PRIMARY KEY (event_id, consumer),
    FOREIGN KEY (event_id, tenant_id)
        REFERENCES outbox(id, tenant_id) ON DELETE CASCADE
);

CREATE INDEX idx_outbox_deliveries_pending
    ON outbox_deliveries (tenant_id, consumer, available_at, created_at)
    WHERE status = 'pending';
CREATE INDEX idx_outbox_deliveries_expired
    ON outbox_deliveries (tenant_id, consumer, locked_until, created_at)
    WHERE status = 'processing';

ALTER TABLE outbox_deliveries ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_outbox_deliveries ON outbox_deliveries
    FOR ALL
    USING (tenant_id = current_setting('app.tenant_id')::UUID)
    WITH CHECK (tenant_id = current_setting('app.tenant_id')::UUID);

CREATE FUNCTION create_outbox_deliveries() RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO outbox_deliveries (event_id, tenant_id, consumer)
    SELECT NEW.id, NEW.tenant_id, s.consumer
    FROM outbox_subscriptions s
    WHERE s.event_type = NEW.event_type;

    IF NOT FOUND THEN
        -- Unknown events remain visible to the relay and become failed after retries.
        INSERT INTO outbox_deliveries (event_id, tenant_id, consumer)
        VALUES (NEW.id, NEW.tenant_id, 'relay-worker');
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER outbox_create_deliveries
    AFTER INSERT ON outbox
    FOR EACH ROW
    EXECUTE FUNCTION create_outbox_deliveries();

-- Preserve pending events created before fan-out was enabled. Already-processed
-- events are intentionally not replayed because their prior consumer is unknown.
INSERT INTO outbox_deliveries (event_id, tenant_id, consumer)
SELECT o.id, o.tenant_id, s.consumer
FROM outbox o
JOIN outbox_subscriptions s ON s.event_type = o.event_type
WHERE o.status = 'pending'
ON CONFLICT (event_id, consumer) DO NOTHING;

INSERT INTO outbox_deliveries (event_id, tenant_id, consumer)
SELECT o.id, o.tenant_id, 'relay-worker'
FROM outbox o
WHERE o.status = 'pending'
  AND NOT EXISTS (
      SELECT 1 FROM outbox_subscriptions s WHERE s.event_type = o.event_type
  )
ON CONFLICT (event_id, consumer) DO NOTHING;
