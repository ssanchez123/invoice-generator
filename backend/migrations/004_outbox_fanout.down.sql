DROP TRIGGER IF EXISTS outbox_create_deliveries ON outbox;
DROP FUNCTION IF EXISTS create_outbox_deliveries();

DROP POLICY IF EXISTS tenant_isolation_outbox_deliveries ON outbox_deliveries;
ALTER TABLE IF EXISTS outbox_deliveries DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS outbox_deliveries;
DROP TABLE IF EXISTS outbox_subscriptions;

ALTER TABLE outbox
    DROP CONSTRAINT IF EXISTS outbox_id_tenant_key;
