package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/domain/port"
)

// PaymentRepository implements port.PaymentRepository using PostgreSQL.
type PaymentRepository struct {
	db *DB
}

func NewPaymentRepository(db *DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

// Save records a payment. Payments are append-only (no updates).
func (r *PaymentRepository) Save(ctx context.Context, payment *entity.Payment) error {
	conn, err := r.db.TenantConn(ctx, payment.TenantID)
	if err != nil {
		return err
	}
	defer conn.Release()

	_, err = conn.Exec(ctx, `
		INSERT INTO payments (id, invoice_id, tenant_id, amount, currency, method, reference, paid_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`,
		payment.ID, payment.InvoiceID, payment.TenantID,
		payment.Amount.Amount, payment.Amount.Currency,
		payment.Method, payment.Reference, payment.PaidAt, payment.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save payment: %w", err)
	}

	return nil
}

// FindByInvoice returns all payments for an invoice.
func (r *PaymentRepository) FindByInvoice(ctx context.Context, invoiceID string) ([]*entity.Payment, error) {
	tenantID, ok := TenantIDFromCtx(ctx)
	if !ok {
		return nil, errors.New("tenant_id not in context")
	}

	conn, err := r.db.TenantConn(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	rows, err := conn.Query(ctx, `
		SELECT id, invoice_id, tenant_id, amount, currency, method, reference, paid_at, created_at
		FROM payments WHERE invoice_id = $1 ORDER BY paid_at ASC
	`, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("find payments: %w", err)
	}
	defer rows.Close()

	var payments []*entity.Payment
	for rows.Next() {
		var p entity.Payment
		err := rows.Scan(
			&p.ID, &p.InvoiceID, &p.TenantID,
			&p.Amount.Amount, &p.Amount.Currency,
			&p.Method, &p.Reference, &p.PaidAt, &p.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan payment: %w", err)
		}
		payments = append(payments, &p)
	}
	return payments, rows.Err()
}

// FindByID loads a single payment by ID.
func (r *PaymentRepository) FindByID(ctx context.Context, id string) (*entity.Payment, error) {
	tenantID, ok := TenantIDFromCtx(ctx)
	if !ok {
		return nil, errors.New("tenant_id not in context")
	}

	conn, err := r.db.TenantConn(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	var p entity.Payment
	err = conn.QueryRow(ctx, `
		SELECT id, invoice_id, tenant_id, amount, currency, method, reference, paid_at, created_at
		FROM payments WHERE id = $1
	`, id).Scan(
		&p.ID, &p.InvoiceID, &p.TenantID,
		&p.Amount.Amount, &p.Amount.Currency,
		&p.Method, &p.Reference, &p.PaidAt, &p.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find payment: %w", err)
	}
	return &p, nil
}

var _ port.PaymentRepository = (*PaymentRepository)(nil)