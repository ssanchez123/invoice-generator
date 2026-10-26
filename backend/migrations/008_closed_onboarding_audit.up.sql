BEGIN;

CREATE INDEX idx_audit_tenant_created
    ON audit_entries (tenant_id, created_at DESC, id DESC);

CREATE FUNCTION public.reject_audit_mutation()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog, public
AS $$
BEGIN
    RAISE EXCEPTION 'audit entries are immutable'
        USING ERRCODE = '42501';
END;
$$;

CREATE TRIGGER audit_entries_immutable
    BEFORE UPDATE OR DELETE ON audit_entries
    FOR EACH ROW EXECUTE FUNCTION public.reject_audit_mutation();

REVOKE UPDATE, DELETE, TRUNCATE ON audit_entries FROM PUBLIC;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'invoicegen_app') THEN
        EXECUTE 'REVOKE UPDATE, DELETE, TRUNCATE ON audit_entries FROM invoicegen_app';
    END IF;
END;
$$;

CREATE FUNCTION public.provision_tenant_with_owner(
    p_tenant_name TEXT,
    p_tax_id TEXT,
    p_address JSONB,
    p_settings JSONB,
    p_owner_email TEXT,
    p_owner_password_hash TEXT,
    p_owner_first_name TEXT,
    p_owner_last_name TEXT
) RETURNS TABLE (created_tenant_id UUID, created_owner_user_id UUID)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE
    v_tenant_id UUID := public.uuid_generate_v4();
    v_owner_user_id UUID := public.uuid_generate_v4();
    v_email TEXT := lower(btrim(p_owner_email));
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'invoicegen_operator')
       OR EXISTS (
            SELECT 1 FROM pg_catalog.pg_roles
            WHERE rolname = session_user AND rolsuper
       )
       OR NOT pg_catalog.pg_has_role(session_user, 'invoicegen_operator', 'MEMBER') THEN
        RAISE EXCEPTION 'caller must use a non-superuser invoicegen_operator login'
            USING ERRCODE = '42501';
    END IF;

    IF p_tenant_name IS NULL OR length(btrim(p_tenant_name)) NOT BETWEEN 1 AND 200 THEN
        RAISE EXCEPTION 'tenant name must contain 1 to 200 characters'
            USING ERRCODE = '22023';
    END IF;
    IF p_tax_id IS NOT NULL AND length(btrim(p_tax_id)) > 64 THEN
        RAISE EXCEPTION 'tax ID must be at most 64 characters'
            USING ERRCODE = '22023';
    END IF;
    IF p_address IS NULL OR jsonb_typeof(p_address) <> 'object'
       OR length(btrim(coalesce(p_address->>'line1', ''))) = 0
       OR length(btrim(coalesce(p_address->>'city', ''))) = 0
       OR length(btrim(coalesce(p_address->>'country', ''))) <> 2 THEN
        RAISE EXCEPTION 'a valid business address is required'
            USING ERRCODE = '22023';
    END IF;
    IF p_settings IS NULL OR jsonb_typeof(p_settings) <> 'object' THEN
        RAISE EXCEPTION 'tenant settings must be a JSON object'
            USING ERRCODE = '22023';
    END IF;
    IF length(v_email) > 254 OR v_email !~ '^[^[:space:]@]+@[^[:space:]@]+$' THEN
        RAISE EXCEPTION 'owner email is invalid'
            USING ERRCODE = '22023';
    END IF;
    IF p_owner_first_name IS NULL OR length(btrim(p_owner_first_name)) NOT BETWEEN 1 AND 100
       OR p_owner_last_name IS NULL OR length(btrim(p_owner_last_name)) NOT BETWEEN 1 AND 100 THEN
        RAISE EXCEPTION 'owner names must contain 1 to 100 characters'
            USING ERRCODE = '22023';
    END IF;
    IF p_owner_password_hash !~ '^\$2[abxy]\$[0-9]{2}\$[./A-Za-z0-9]{53}$' THEN
        RAISE EXCEPTION 'owner password must be supplied as a bcrypt hash'
            USING ERRCODE = '22023';
    END IF;

    INSERT INTO public.tenants (id, name, address, tax_id, settings)
    VALUES (
        v_tenant_id,
        btrim(p_tenant_name),
        p_address,
        nullif(btrim(p_tax_id), ''),
        p_settings
    );

    INSERT INTO public.users (
        id, tenant_id, email, role, password_hash, first_name, last_name
    ) VALUES (
        v_owner_user_id,
        v_tenant_id,
        v_email,
        'admin',
        p_owner_password_hash,
        btrim(p_owner_first_name),
        btrim(p_owner_last_name)
    );

    INSERT INTO public.audit_entries (
        tenant_id, entity_type, entity_id, action, payload
    ) VALUES (
        v_tenant_id,
        'tenant',
        v_tenant_id,
        'tenant.provisioned',
        jsonb_build_object(
            'operator_login', session_user,
            'owner_user_id', v_owner_user_id
        )
    );

    created_tenant_id := v_tenant_id;
    created_owner_user_id := v_owner_user_id;
    RETURN NEXT;
END;
$$;

REVOKE ALL ON FUNCTION public.provision_tenant_with_owner(
    TEXT, TEXT, JSONB, JSONB, TEXT, TEXT, TEXT, TEXT
) FROM PUBLIC;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'invoicegen_operator') THEN
        EXECUTE 'GRANT CONNECT ON DATABASE ' || quote_ident(current_database()) || ' TO invoicegen_operator';
        EXECUTE 'GRANT USAGE ON SCHEMA public TO invoicegen_operator';
        EXECUTE 'GRANT EXECUTE ON FUNCTION public.provision_tenant_with_owner(TEXT, TEXT, JSONB, JSONB, TEXT, TEXT, TEXT, TEXT) TO invoicegen_operator';
    END IF;
END;
$$;

COMMIT;
