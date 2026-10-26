package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/santiagossaa/invoice-generator/internal/domain/port"
)

// IdempotencyRepository implements port.IdempotencyRepository using PostgreSQL.
type IdempotencyRepository struct {
	db *DB
}

func NewIdempotencyRepository(db *DB) *IdempotencyRepository {
	return &IdempotencyRepository{db: db}
}

func (r *IdempotencyRepository) Reserve(ctx context.Context, record *port.IdempotencyRecord) (*port.IdempotencyRecord, bool, error) {
	tx, err := r.db.TenantTx(ctx, record.TenantID)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback(ctx)

	reserved, err := scanIdempotencyRecord(tx.QueryRow(ctx, `
		INSERT INTO idempotency_records
			(key, tenant_id, request_hash, state, response_body, response_headers, status, expires_at)
		VALUES ($1, $2, $3, 'processing', NULL, '{}'::jsonb, 0, $4)
		ON CONFLICT (key, tenant_id) DO UPDATE
		SET request_hash = EXCLUDED.request_hash,
			state = 'processing',
			response_body = NULL,
			response_headers = '{}'::jsonb,
			status = 0,
			expires_at = EXCLUDED.expires_at,
			created_at = NOW()
		WHERE idempotency_records.expires_at <= NOW()
		RETURNING key, tenant_id, request_hash, state, response_body, response_headers, status, expires_at
	`, record.Key, record.TenantID, record.RequestHash, record.ExpiresAt))
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return nil, false, fmt.Errorf("commit idempotency reservation: %w", err)
		}
		return reserved, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, fmt.Errorf("reserve idempotency key: %w", err)
	}

	existing, err := scanIdempotencyRecord(tx.QueryRow(ctx, `
		SELECT key, tenant_id, request_hash, state, response_body, response_headers, status, expires_at
		FROM idempotency_records
		WHERE key = $1 AND tenant_id = $2
	`, record.Key, record.TenantID))
	if err != nil {
		return nil, false, fmt.Errorf("read idempotency key: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, fmt.Errorf("commit idempotency lookup: %w", err)
	}
	return existing, false, nil
}

func (r *IdempotencyRepository) Complete(ctx context.Context, record *port.IdempotencyRecord) error {
	tx, err := r.db.TenantTx(ctx, record.TenantID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	result, err := tx.Exec(ctx, `
		UPDATE idempotency_records
		SET state = 'completed',
			response_body = $4,
			response_headers = $5::jsonb,
			status = $6,
			expires_at = $7
		WHERE key = $1 AND tenant_id = $2 AND request_hash = $3 AND state = 'processing'
	`, record.Key, record.TenantID, record.RequestHash, record.ResponseBody,
		string(record.ResponseHeaders), record.Status, record.ExpiresAt)
	if err != nil {
		return fmt.Errorf("complete idempotency response: %w", err)
	}
	if result.RowsAffected() != 1 {
		return errors.New("idempotency reservation was not found")
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit idempotency response: %w", err)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanIdempotencyRecord(row rowScanner) (*port.IdempotencyRecord, error) {
	var record port.IdempotencyRecord
	err := row.Scan(&record.Key, &record.TenantID, &record.RequestHash, &record.State,
		&record.ResponseBody, &record.ResponseHeaders, &record.Status, &record.ExpiresAt)
	if err != nil {
		return nil, err
	}
	return &record, nil
}

var _ port.IdempotencyRepository = (*IdempotencyRepository)(nil)
