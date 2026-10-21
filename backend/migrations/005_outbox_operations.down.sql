BEGIN;

DROP FUNCTION IF EXISTS public.requeue_failed_outbox_delivery(UUID, UUID, TEXT, TEXT);
DROP POLICY IF EXISTS tenant_isolation_outbox_delivery_replays ON outbox_delivery_replays;
DROP TABLE IF EXISTS outbox_delivery_replays;

COMMIT;
