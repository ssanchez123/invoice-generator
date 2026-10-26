ALTER TABLE idempotency_records
    ADD COLUMN request_hash TEXT NOT NULL DEFAULT '',
    ADD COLUMN response_headers JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN state TEXT NOT NULL DEFAULT 'completed';

ALTER TABLE idempotency_records
    ADD CONSTRAINT idempotency_records_state_check
    CHECK (state IN ('processing', 'completed'));
