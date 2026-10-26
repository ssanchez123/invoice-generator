BEGIN;

DROP FUNCTION IF EXISTS public.provision_tenant_with_owner(
    TEXT, TEXT, JSONB, JSONB, TEXT, TEXT, TEXT, TEXT
);
DROP TRIGGER IF EXISTS audit_entries_immutable ON audit_entries;
DROP FUNCTION IF EXISTS public.reject_audit_mutation();
DROP INDEX IF EXISTS idx_audit_tenant_created;

COMMIT;
