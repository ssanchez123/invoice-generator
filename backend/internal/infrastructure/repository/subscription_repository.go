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

type SubscriptionRepository struct {
	db *DB
}

func NewSubscriptionRepository(db *DB) *SubscriptionRepository {
	return &SubscriptionRepository{db: db}
}

func (r *SubscriptionRepository) Save(ctx context.Context, sub *entity.Subscription) error {
	tx, err := r.db.TenantTx(ctx, sub.TenantID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	txCtx := port.CtxWithTx(ctx, tx)
	if err := r.SaveTx(txCtx, sub); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *SubscriptionRepository) SaveTx(ctx context.Context, sub *entity.Subscription) error {
	tx := port.TxFromCtx(ctx)
	if tx == nil {
		return errors.New("SaveTx requires a transaction in context")
	}
	created, err := subscriptionCreatedTx(ctx, tx, sub.ID)
	if err != nil {
		return err
	}
	itemsJSON, err := json.Marshal(sub.PlanItems)
	if err != nil {
		return fmt.Errorf("marshal plan items: %w", err)
	}

	result, err := tx.Exec(ctx, `
		INSERT INTO subscriptions (
			id, tenant_id, customer_id, plan_items, frequency, next_billing_date,
			billing_day, status, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE SET
			plan_items = EXCLUDED.plan_items,
			frequency = EXCLUDED.frequency,
			next_billing_date = EXCLUDED.next_billing_date,
			billing_day = EXCLUDED.billing_day,
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at
		WHERE subscriptions.tenant_id = EXCLUDED.tenant_id
	`,
		sub.ID, sub.TenantID, sub.CustomerID, itemsJSON,
		string(sub.Frequency), sub.NextBillingDate, sub.BillingDay,
		string(sub.Status), sub.CreatedAt, sub.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save subscription: %w", err)
	}
	if result.RowsAffected() == 0 {
		return errors.New("subscription belongs to another tenant")
	}
	if err := r.saveAuditTx(ctx, tx, sub, created); err != nil {
		return err
	}
	return nil
}

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

	sub, err := scanSubscription(conn.QueryRow(ctx, `
		SELECT id, tenant_id, customer_id, plan_items, frequency, next_billing_date,
		       billing_day, status, created_at, updated_at
		FROM subscriptions WHERE id = $1
	`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load subscription: %w", err)
	}
	return sub, nil
}

func (r *SubscriptionRepository) FindByIDForUpdate(ctx context.Context, id string) (*entity.Subscription, error) {
	tx := port.TxFromCtx(ctx)
	if tx == nil {
		return nil, errors.New("FindByIDForUpdate requires a transaction in context")
	}
	sub, err := scanSubscription(tx.QueryRow(ctx, `
		SELECT id, tenant_id, customer_id, plan_items, frequency, next_billing_date,
		       billing_day, status, created_at, updated_at
		FROM subscriptions WHERE id = $1 FOR UPDATE
	`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock subscription: %w", err)
	}
	return sub, nil
}

func (r *SubscriptionRepository) FindByTenant(ctx context.Context, tenantID, status string, limit, offset int) ([]*entity.Subscription, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	conn, err := r.db.TenantConn(ctx, tenantID)
	if err != nil {
		return nil, 0, err
	}
	defer conn.Release()

	var total int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM subscriptions WHERE ($1 = '' OR status = $1)`, status).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count subscriptions: %w", err)
	}
	rows, err := conn.Query(ctx, `
		SELECT id, tenant_id, customer_id, plan_items, frequency, next_billing_date,
		       billing_day, status, created_at, updated_at
		FROM subscriptions
		WHERE ($1 = '' OR status = $1)
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`, status, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list subscriptions: %w", err)
	}
	defer rows.Close()

	subscriptions := make([]*entity.Subscription, 0)
	for rows.Next() {
		sub, err := scanSubscription(rows)
		if err != nil {
			return nil, 0, err
		}
		subscriptions = append(subscriptions, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate subscriptions: %w", err)
	}
	return subscriptions, total, nil
}

func (r *SubscriptionRepository) FindDueForUpdate(ctx context.Context, asOf time.Time, limit int) ([]*entity.Subscription, error) {
	if _, ok := TenantIDFromCtx(ctx); !ok {
		return nil, errors.New("tenant_id not in context")
	}
	tx := port.TxFromCtx(ctx)
	if tx == nil {
		return nil, errors.New("FindDueForUpdate requires a transaction in context")
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := tx.Query(ctx, `
		SELECT id, tenant_id, customer_id, plan_items, frequency, next_billing_date,
		       billing_day, status, created_at, updated_at
		FROM subscriptions
		WHERE status = 'active' AND next_billing_date <= $1
		ORDER BY next_billing_date ASC, id ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`, asOf, limit)
	if err != nil {
		return nil, fmt.Errorf("find due subscriptions: %w", err)
	}
	defer rows.Close()

	subscriptions := make([]*entity.Subscription, 0)
	for rows.Next() {
		sub, err := scanSubscription(rows)
		if err != nil {
			return nil, err
		}
		subscriptions = append(subscriptions, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate due subscriptions: %w", err)
	}
	return subscriptions, nil
}

func scanSubscription(row pgx.Row) (*entity.Subscription, error) {
	var sub entity.Subscription
	var itemsJSON []byte
	var frequency, status string
	if err := row.Scan(
		&sub.ID, &sub.TenantID, &sub.CustomerID, &itemsJSON, &frequency,
		&sub.NextBillingDate, &sub.BillingDay, &status, &sub.CreatedAt, &sub.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(itemsJSON, &sub.PlanItems); err != nil {
		return nil, fmt.Errorf("decode subscription plan items: %w", err)
	}
	sub.Frequency = entity.BillingFrequency(frequency)
	sub.Status = entity.SubscriptionStatus(status)
	return &sub, nil
}

func subscriptionCreatedTx(ctx context.Context, tx pgx.Tx, subscriptionID string) (bool, error) {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM subscriptions WHERE id = $1)`, subscriptionID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check subscription existence: %w", err)
	}
	return !exists, nil
}

func (r *SubscriptionRepository) saveAuditTx(ctx context.Context, tx pgx.Tx, sub *entity.Subscription, created bool) error {
	actorID, err := auditActorID(ctx)
	if err != nil {
		return err
	}
	action := "subscription.updated"
	if created {
		action = "subscription.created"
	} else if sub.Status == entity.SubscriptionStatusCancelled {
		action = "subscription.cancelled"
	}
	payload, err := json.Marshal(map[string]any{
		"status":            string(sub.Status),
		"frequency":         string(sub.Frequency),
		"next_billing_date": sub.NextBillingDate,
		"billing_day":       sub.BillingDay,
	})
	if err != nil {
		return fmt.Errorf("marshal subscription audit payload: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_entries (tenant_id, entity_type, entity_id, action, actor_id, payload)
		VALUES ($1, 'subscription', $2, $3, $4, $5)
	`, sub.TenantID, sub.ID, action, actorID, payload)
	if err != nil {
		return fmt.Errorf("insert subscription audit entry: %w", err)
	}
	return nil
}

var _ port.SubscriptionRepository = (*SubscriptionRepository)(nil)
