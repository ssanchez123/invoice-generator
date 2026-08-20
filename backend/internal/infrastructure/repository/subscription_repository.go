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

// SubscriptionRepository implements port.SubscriptionRepository using PostgreSQL.
type SubscriptionRepository struct {
	db *DB
}

func NewSubscriptionRepository(db *DB) *SubscriptionRepository {
	return &SubscriptionRepository{db: db}
}

// Save creates or updates a subscription.
func (r *SubscriptionRepository) Save(ctx context.Context, sub *entity.Subscription) error {
	conn, err := r.db.TenantConn(ctx, sub.TenantID)
	if err != nil {
		return err
	}
	defer conn.Release()

	itemsJSON, err := json.Marshal(sub.PlanItems)
	if err != nil {
		return fmt.Errorf("marshal plan items: %w", err)
	}

	_, err = conn.Exec(ctx, `
		INSERT INTO subscriptions (id, tenant_id, customer_id, plan_items, frequency,
		                            next_billing_date, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			plan_items = EXCLUDED.plan_items,
			frequency = EXCLUDED.frequency,
			next_billing_date = EXCLUDED.next_billing_date,
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at
	`,
		sub.ID, sub.TenantID, sub.CustomerID, itemsJSON,
		string(sub.Frequency), sub.NextBillingDate, string(sub.Status),
		sub.CreatedAt, sub.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save subscription: %w", err)
	}

	return nil
}

// FindByID loads a subscription by ID.
func (r *SubscriptionRepository) FindByID(ctx context.Context, id string) (*entity.Subscription, error) {
	tenantID, ok := TenantIDFromCtx(ctx)
	if !ok {
		return nil, errors.New("tenant_id not in context")
	}

	conn, err := r.db.TenantConn(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	var sub entity.Subscription
	var itemsJSON []byte
	var frequency, status string

	err = conn.QueryRow(ctx, `
		SELECT id, tenant_id, customer_id, plan_items, frequency, next_billing_date, status,
		       created_at, updated_at
		FROM subscriptions WHERE id = $1
	`, id).Scan(
		&sub.ID, &sub.TenantID, &sub.CustomerID, &itemsJSON, &frequency,
		&sub.NextBillingDate, &status, &sub.CreatedAt, &sub.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("load subscription: %w", err)
	}

	sub.Frequency = entity.BillingFrequency(frequency)
	sub.Status = entity.SubscriptionStatus(status)
	if itemsJSON != nil {
		json.Unmarshal(itemsJSON, &sub.PlanItems)
	}

	return &sub, nil
}

// FindDue returns active subscriptions whose next_billing_date is <= asOf.
// Uses SELECT FOR UPDATE SKIP LOCKED to support multiple worker instances.
func (r *SubscriptionRepository) FindDue(ctx context.Context, asOf time.Time, limit int) ([]*entity.Subscription, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	// This query runs without tenant context because the billing worker
	// operates across all tenants. It uses a direct pool connection.
	conn, err := r.db.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Release()

	rows, err := conn.Query(ctx, `
		SELECT id, tenant_id, customer_id, plan_items, frequency, next_billing_date, status,
		       created_at, updated_at
		FROM subscriptions
		WHERE status = 'active' AND next_billing_date <= $1
		ORDER BY next_billing_date ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`, asOf, limit)
	if err != nil {
		return nil, fmt.Errorf("find due subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []*entity.Subscription
	for rows.Next() {
		var sub entity.Subscription
		var itemsJSON []byte
		var frequency, status string

		err := rows.Scan(
			&sub.ID, &sub.TenantID, &sub.CustomerID, &itemsJSON, &frequency,
			&sub.NextBillingDate, &status, &sub.CreatedAt, &sub.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan subscription: %w", err)
		}

		sub.Frequency = entity.BillingFrequency(frequency)
		sub.Status = entity.SubscriptionStatus(status)
		if itemsJSON != nil {
			json.Unmarshal(itemsJSON, &sub.PlanItems)
		}

		subs = append(subs, &sub)
	}
	return subs, rows.Err()
}

// UpdateNextBillingDate updates the next billing date for a subscription.
func (r *SubscriptionRepository) UpdateNextBillingDate(ctx context.Context, id string, nextDate time.Time) error {
	conn, err := r.db.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Release()

	_, err = conn.Exec(ctx, `
		UPDATE subscriptions SET next_billing_date = $1, updated_at = NOW() WHERE id = $2
	`, nextDate, id)
	if err != nil {
		return fmt.Errorf("update next billing date: %w", err)
	}
	return nil
}

var _ port.SubscriptionRepository = (*SubscriptionRepository)(nil)
