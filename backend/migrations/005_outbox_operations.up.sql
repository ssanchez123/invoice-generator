BEGIN;

CREATE TABLE outbox_delivery_replays (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id           UUID NOT NULL,
    event_id            UUID NOT NULL,
    consumer            TEXT NOT NULL,
    operator_login      TEXT NOT NULL,
    reason              TEXT NOT NULL,
    attempts_before     INT NOT NULL,
    replayed_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_outbox_delivery_replays_lookup
    ON outbox_delivery_replays (tenant_id, event_id, consumer, replayed_at DESC);

ALTER TABLE outbox_delivery_replays ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_outbox_delivery_replays ON outbox_delivery_replays
    FOR SELECT
    USING (tenant_id = current_setting('app.tenant_id')::UUID);

REVOKE ALL ON outbox_delivery_replays FROM PUBLIC;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'invoicegen_app') THEN
        EXECUTE 'REVOKE ALL ON outbox_delivery_replays FROM invoicegen_app';
    END IF;
END;
$$;

CREATE FUNCTION public.requeue_failed_outbox_delivery(
    p_tenant_id UUID,
    p_event_id UUID,
    p_consumer TEXT,
    p_reason TEXT
) RETURNS VOID
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE
    v_attempts INT;
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_catalog.pg_roles
        WHERE rolname = session_user
          AND rolsuper
    ) OR NOT pg_catalog.pg_has_role(session_user, 'invoicegen_operator', 'MEMBER') THEN
        RAISE EXCEPTION 'caller must use a non-superuser invoicegen_operator login'
            USING ERRCODE = '42501';
    END IF;
    IF current_setting('app.tenant_id', true) IS DISTINCT FROM p_tenant_id::TEXT THEN
        RAISE EXCEPTION 'tenant context does not match requested tenant'
            USING ERRCODE = '42501';
    END IF;
    IF p_reason IS NULL OR btrim(p_reason) = '' OR length(p_reason) > 500 THEN
        RAISE EXCEPTION 'replay reason must contain 1 to 500 characters'
            USING ERRCODE = '22023';
    END IF;

    SELECT attempts
    INTO v_attempts
    FROM public.outbox_deliveries
    WHERE tenant_id = p_tenant_id
      AND event_id = p_event_id
      AND consumer = p_consumer
      AND status = 'failed'
    FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'failed outbox delivery not found'
            USING ERRCODE = 'P0002';
    END IF;

    UPDATE public.outbox_deliveries
    SET status = 'pending',
        attempts = 0,
        available_at = NOW(),
        locked_by = NULL,
        locked_until = NULL,
        processed_at = NULL
    WHERE tenant_id = p_tenant_id
      AND event_id = p_event_id
      AND consumer = p_consumer
      AND status = 'failed';

    INSERT INTO public.outbox_delivery_replays (
        tenant_id, event_id, consumer, operator_login, reason,
        attempts_before
    ) VALUES (
        p_tenant_id, p_event_id, p_consumer, session_user, btrim(p_reason),
        v_attempts
    );
END;
$$;

REVOKE ALL ON FUNCTION public.requeue_failed_outbox_delivery(UUID, UUID, TEXT, TEXT) FROM PUBLIC;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'invoicegen_operator') THEN
        EXECUTE 'GRANT CONNECT ON DATABASE ' || quote_ident(current_database()) || ' TO invoicegen_operator';
        EXECUTE 'GRANT USAGE ON SCHEMA public TO invoicegen_operator';
        EXECUTE 'GRANT SELECT ON outbox_delivery_replays TO invoicegen_operator';
        EXECUTE 'GRANT EXECUTE ON FUNCTION public.requeue_failed_outbox_delivery(UUID, UUID, TEXT, TEXT) TO invoicegen_operator';
    END IF;
END;
$$;

COMMIT;
