package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

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

// Get retrieves a cached idempotency response by key.
func (r *IdempotencyRepository) Get(ctx context.Context, key string) (*port.IdempotencyRecord, error) {
	tenantID, ok := TenantIDFromCtx(ctx)
	if !ok {
		return nil, errors.New("tenant_id not in context")
	}

	conn, err := r.db.TenantConn(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	var record port.IdempotencyRecord
	var expiresAt time.Time

	err = conn.QueryRow(ctx, `
		SELECT key, tenant_id, response_body, status, expires_at
		FROM idempotency_records
		WHERE key = $1 AND tenant_id = $2 AND expires_at > NOW()
	`, key, tenantID).Scan(
		&record.Key, &record.TenantID, &record.ResponseBody,
		&record.Status, &expiresAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // No cached response = proceed with request
		}
		return nil, fmt.Errorf("get idempotency: %w", err)
	}

	record.ExpiresAt = expiresAt
	return &record, nil
}

// Save stores an idempotency response with a TTL (default 24 hours).
func (r *IdempotencyRepository) Save(ctx context.Context, record *port.IdempotencyRecord) error {
	conn, err := r.db.TenantConn(ctx, record.TenantID)
	if err != nil {
		return err
	}
	defer conn.Release()

	_, err = conn.Exec(ctx, `
		INSERT INTO idempotency_records (key, tenant_id, response_body, status, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (key, tenant_id) DO NOTHING
	`,
		record.Key, record.TenantID, record.ResponseBody,
		record.Status, record.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("save idempotency: %w", err)
	}

	return nil
}

var _ port.IdempotencyRepository = (*IdempotencyRepository)(nil)