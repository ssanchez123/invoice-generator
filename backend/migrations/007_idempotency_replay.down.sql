ALTER TABLE idempotency_records
    DROP CONSTRAINT IF EXISTS idempotency_records_state_check;

ALTER TABLE idempotency_records
    DROP COLUMN IF EXISTS state,
    DROP COLUMN IF EXISTS response_headers,
    DROP COLUMN IF EXISTS request_hash;
