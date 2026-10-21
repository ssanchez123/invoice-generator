BEGIN;

ALTER TABLE invoices
    ADD COLUMN issued_snapshot JSONB;

CREATE FUNCTION public.prevent_invoice_issued_snapshot_mutation() RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog, public
AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF EXISTS (SELECT 1 FROM public.invoices WHERE id = NEW.id) THEN
            RETURN NEW;
        END IF;
        IF NEW.status <> 'draft' OR NEW.issued_snapshot IS NOT NULL THEN
            RAISE EXCEPTION 'new invoices must start as drafts';
        END IF;
        RETURN NEW;
    END IF;

    IF OLD.issued_snapshot IS NOT NULL THEN
        IF NEW.issued_snapshot IS DISTINCT FROM OLD.issued_snapshot THEN
            RAISE EXCEPTION 'issued invoice snapshot is immutable';
        END IF;
        RETURN NEW;
    END IF;

    IF OLD.status = 'draft' AND NEW.status = 'issued' AND NEW.issued_snapshot IS NULL THEN
        RAISE EXCEPTION 'issued invoices require an immutable snapshot';
    END IF;

    IF NEW.issued_snapshot IS NOT NULL
       AND (
           OLD.status <> 'draft'
           OR NEW.status <> 'issued'
           OR NEW.issued_snapshot->>'invoice_id' IS DISTINCT FROM NEW.id::TEXT
           OR NEW.issued_snapshot->>'tenant_id' IS DISTINCT FROM NEW.tenant_id::TEXT
           OR NEW.issued_snapshot->>'customer_id' IS DISTINCT FROM NEW.customer_id::TEXT
       ) THEN
        RAISE EXCEPTION 'invoice snapshot can only be recorded when issuing a draft';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER invoices_issued_snapshot_immutable
    BEFORE INSERT OR UPDATE OF status, issued_snapshot ON invoices
    FOR EACH ROW
    EXECUTE FUNCTION public.prevent_invoice_issued_snapshot_mutation();

COMMIT;
