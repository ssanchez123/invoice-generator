package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/domain/port"
)

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

// FindUnprocessed returns pending outbox entries, ordered by creation time.
// Uses SELECT FOR UPDATE SKIP LOCKED so multiple relay workers don't conflict.
func (r *OutboxRepository) FindUnprocessed(ctx context.Context, limit int) ([]port.OutboxEntry, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	conn, err := r.db.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Release()

	rows, err := conn.Query(ctx, `
		SELECT id, aggregate_id, tenant_id, event_type, payload, created_at
		FROM outbox
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("find unprocessed outbox: %w", err)
	}
	defer rows.Close()

	var entries []port.OutboxEntry
	for rows.Next() {
		var entry port.OutboxEntry
		err := rows.Scan(
			&entry.ID, &entry.AggregateID, &entry.TenantID,
			&entry.EventType, &entry.Payload, &entry.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan outbox: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// MarkProcessed marks an outbox entry as successfully relayed.
func (r *OutboxRepository) MarkProcessed(ctx context.Context, id string) error {
	conn, err := r.db.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Release()

	_, err = conn.Exec(ctx, `
		UPDATE outbox SET status = 'processed', processed_at = $1 WHERE id = $2
	`, time.Now(), id)
	if err != nil {
		return fmt.Errorf("mark processed: %w", err)
	}
	return nil
}

var _ port.OutboxRepository = (*OutboxRepository)(nil)
