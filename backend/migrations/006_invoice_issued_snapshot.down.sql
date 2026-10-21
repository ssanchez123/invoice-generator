BEGIN;

DROP TRIGGER IF EXISTS invoices_issued_snapshot_immutable ON invoices;
DROP FUNCTION IF EXISTS public.prevent_invoice_issued_snapshot_mutation();
ALTER TABLE invoices DROP COLUMN IF EXISTS issued_snapshot;

COMMIT;
