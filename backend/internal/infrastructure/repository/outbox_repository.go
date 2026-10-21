package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/domain/port"
)

const maxOutboxDeliveryAttempts = 10

var ErrOutboxLeaseLost = errors.New("outbox delivery lease lost")

// OutboxRepository implements port.OutboxRepository using PostgreSQL.
type OutboxRepository struct {
	db *DB
}

func NewOutboxRepository(db *DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

// SaveEvents writes domain events to the outbox table.
// This is called within the same transaction as the aggregate save (by InvoiceRepository.Save).
// When called directly (not in a tx), it uses a tenant-scoped connection.
func (r *OutboxRepository) SaveEvents(ctx context.Context, tenantID string, events []entity.DomainEvent) error {
	conn, err := r.db.TenantConn(ctx, tenantID)
	if err != nil {
		return err
	}
	defer conn.Release()

	for _, event := range events {
		payload, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("marshal event: %w", err)
		}

		_, err = conn.Exec(ctx, `
			INSERT INTO outbox (aggregate_id, tenant_id, event_type, payload, status, created_at)
			VALUES ($1, $2, $3, $4, 'pending', NOW())
		`, event.AggregateID(), tenantID, event.EventName(), payload)
		if err != nil {
			return fmt.Errorf("insert outbox: %w", err)
		}
	}
	return nil
}

// SaveEventsTx writes domain events to the outbox within an existing transaction.
// Used by InvoiceRepository.Save to write events in the same TX as the aggregate.
func SaveEventsTx(ctx context.Context, tx pgx.Tx, tenantID string, events []entity.DomainEvent) error {
	for _, event := range events {
		payload, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("marshal event: %w", err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO outbox (aggregate_id, tenant_id, event_type, payload, status, created_at)
			VALUES ($1, $2, $3, $4, 'pending', NOW())
		`, event.AggregateID(), tenantID, event.EventName(), payload)
		if err != nil {
			return fmt.Errorf("insert outbox tx: %w", err)
		}
	}
	return nil
}

// ListTenantIDs lists tenant IDs so background consumers can process each tenant
// inside its own RLS-scoped transaction. The tenants table itself is not tenant-scoped.
func (r *OutboxRepository) ListTenantIDs(ctx context.Context) ([]string, error) {
	rows, err := r.db.pool.Query(ctx, `SELECT id FROM tenants ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list tenant IDs: %w", err)
	}
	defer rows.Close()

	var tenantIDs []string
	for rows.Next() {
		var tenantID string
		if err := rows.Scan(&tenantID); err != nil {
			return nil, fmt.Errorf("scan tenant ID: %w", err)
		}
		tenantIDs = append(tenantIDs, tenantID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tenant IDs: %w", err)
	}
	return tenantIDs, nil
}

// ClaimDeliveries atomically leases available deliveries for a single tenant
// and consumer. The lease survives transaction completion and can be reclaimed
// after expiry if the worker exits before acknowledging it.
func (r *OutboxRepository) ClaimDeliveries(
	ctx context.Context,
	tenantID, consumer, workerID string,
	limit int,
	lease time.Duration,
) ([]port.OutboxEntry, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	lease = normalizeOutboxLease(lease)

	tx, err := r.db.TenantTx(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		WITH candidates AS (
			SELECT d.event_id, d.consumer
			FROM outbox_deliveries d
			WHERE d.tenant_id = $1
			  AND d.consumer = $2
			  AND (
				(d.status = 'pending' AND d.available_at <= NOW())
				OR (d.status = 'processing' AND d.locked_until <= NOW())
			  )
			ORDER BY d.created_at
			LIMIT $3
			FOR UPDATE OF d SKIP LOCKED
		)
		UPDATE outbox_deliveries d
		SET status = 'processing',
		    attempts = d.attempts + 1,
		    locked_by = $4,
		    locked_until = NOW() + make_interval(secs => $5::double precision),
		    last_error = NULL
		FROM candidates c
		JOIN outbox o ON o.id = c.event_id
		WHERE d.event_id = c.event_id
		  AND d.consumer = c.consumer
		  AND d.tenant_id = $1
		RETURNING o.id, o.aggregate_id, o.tenant_id, o.event_type, o.payload, o.created_at, d.attempts
	`, tenantID, consumer, limit, workerID, lease.Seconds())
	if err != nil {
		return nil, fmt.Errorf("claim outbox deliveries: %w", err)
	}

	var entries []port.OutboxEntry
	for rows.Next() {
		var entry port.OutboxEntry
		if err := rows.Scan(
			&entry.ID, &entry.AggregateID, &entry.TenantID,
			&entry.EventType, &entry.Payload, &entry.CreatedAt, &entry.Attempts,
		); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan outbox delivery: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate outbox deliveries: %w", err)
	}
	rows.Close()

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit outbox claim: %w", err)
	}
	return entries, nil
}

// RenewDeliveryLease extends a lease only while it is still active and owned
// by this worker. An expired lease cannot be resurrected after another worker
// has become eligible to claim it.
func (r *OutboxRepository) RenewDeliveryLease(
	ctx context.Context,
	tenantID, eventID, consumer, workerID string,
	lease time.Duration,
) error {
	tx, err := r.db.TenantTx(ctx, tenantID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE outbox_deliveries
		SET locked_until = NOW() + make_interval(secs => $5::double precision)
		WHERE event_id = $1
		  AND tenant_id = $2
		  AND consumer = $3
		  AND status = 'processing'
		  AND locked_by = $4
		  AND locked_until > NOW()
	`, eventID, tenantID, consumer, workerID, normalizeOutboxLease(lease).Seconds())
	if err != nil {
		return fmt.Errorf("renew outbox delivery lease: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrOutboxLeaseLost
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit outbox lease renewal: %w", err)
	}
	return nil
}

// CompleteDelivery acknowledges a delivery only if this worker still owns its lease.
func (r *OutboxRepository) CompleteDelivery(
	ctx context.Context,
	tenantID, eventID, consumer, workerID string,
) error {
	tx, err := r.db.TenantTx(ctx, tenantID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var id string
	if err := tx.QueryRow(ctx, `
		SELECT id FROM outbox
		WHERE id = $1 AND tenant_id = $2
		FOR UPDATE
	`, eventID, tenantID).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrOutboxLeaseLost
		}
		return fmt.Errorf("lock outbox event: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		UPDATE outbox_deliveries
		SET status = 'processed',
		    processed_at = NOW(),
		    locked_by = NULL,
		    locked_until = NULL,
		    last_error = NULL
		WHERE event_id = $1
		  AND tenant_id = $2
		  AND consumer = $3
		  AND status = 'processing'
		  AND locked_by = $4
		  AND locked_until > NOW()
	`, eventID, tenantID, consumer, workerID)
	if err != nil {
		return fmt.Errorf("complete outbox delivery: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrOutboxLeaseLost
	}

	_, err = tx.Exec(ctx, `
		UPDATE outbox
		SET status = 'processed', processed_at = NOW()
		WHERE id = $1
		  AND tenant_id = $2
		  AND NOT EXISTS (
			SELECT 1 FROM outbox_deliveries
			WHERE event_id = $1
			  AND tenant_id = $2
			  AND status <> 'processed'
		  )
	`, eventID, tenantID)
	if err != nil {
		return fmt.Errorf("update outbox status: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit outbox completion: %w", err)
	}
	return nil
}

// RetryDelivery releases a failed lease with exponential backoff. After the
// retry limit, the delivery is marked failed for operator review.
func (r *OutboxRepository) RetryDelivery(
	ctx context.Context,
	tenantID, eventID, consumer, workerID string,
	cause error,
) error {
	if cause == nil {
		cause = errors.New("unspecified delivery failure")
	}

	tx, err := r.db.TenantTx(ctx, tenantID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE outbox_deliveries
		SET status = CASE WHEN attempts >= $5 THEN 'failed' ELSE 'pending' END,
		    available_at = CASE
				WHEN attempts >= $5 THEN available_at
				ELSE NOW() + LEAST(900, POWER(2, LEAST(attempts - 1, 9))) * INTERVAL '1 second'
			END,
		    locked_by = NULL,
		    locked_until = NULL,
		    last_error = LEFT($6, 2000)
		WHERE event_id = $1
		  AND tenant_id = $2
		  AND consumer = $3
		  AND status = 'processing'
		  AND locked_by = $4
		  AND locked_until > NOW()
	`, eventID, tenantID, consumer, workerID, maxOutboxDeliveryAttempts, cause.Error())
	if err != nil {
		return fmt.Errorf("retry outbox delivery: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrOutboxLeaseLost
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit outbox retry: %w", err)
	}
	return nil
}

func normalizeOutboxLease(lease time.Duration) time.Duration {
	if lease <= 0 || lease > time.Hour {
		return 5 * time.Minute
	}
	return lease
}

var _ port.OutboxRepository = (*OutboxRepository)(nil)
