package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/santiagossaa/invoice-generator/internal/ctxkey"
	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/domain/port"
	"github.com/santiagossaa/invoice-generator/internal/domain/valueobject"
)

// Common errors
var (
	ErrNotFound = errors.New("entity not found")
)

// InvoiceRepository implements port.InvoiceRepository using PostgreSQL.
type InvoiceRepository struct {
	db *DB
}

func NewInvoiceRepository(db *DB) *InvoiceRepository {
	return &InvoiceRepository{db: db}
}

// Save persists an invoice and its domain events atomically.
// The invoice (aggregate) and its events (outbox) are written in the same transaction.
func (r *InvoiceRepository) Save(ctx context.Context, invoice *entity.Invoice) error {
	tx, err := r.db.TenantTx(ctx, invoice.TenantID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) // safe: no-op if committed

	created, err := invoiceCreatedTx(ctx, tx, invoice.ID)
	if err != nil {
		return err
	}
	// Upsert invoice
	err = r.saveInvoiceTx(ctx, tx, invoice)
	if err != nil {
		return err
	}

	// Save domain events to outbox (same transaction)
	events := invoice.ClearEvents()
	if len(events) > 0 {
		if err := SaveEventsTx(ctx, tx, invoice.TenantID, events); err != nil {
			return err
		}
	}

	// Write audit entry
	if err := r.saveAuditTx(ctx, tx, invoice, events, created); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// saveInvoiceTx upserts the invoice and its items within a transaction.
func (r *InvoiceRepository) saveInvoiceTx(ctx context.Context, tx pgx.Tx, inv *entity.Invoice) error {
	if err := r.prepareIssuedSnapshot(ctx, tx, inv); err != nil {
		return err
	}

	// Marshal metadata to JSON; nil maps become '{}' not NULL (column is NOT NULL).
	metadataJSON, err := json.Marshal(inv.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	if string(metadataJSON) == "null" {
		metadataJSON = []byte("{}")
	}
	var snapshotJSON []byte
	if inv.IssuedSnapshot != nil {
		snapshotJSON, err = json.Marshal(inv.IssuedSnapshot)
		if err != nil {
			return fmt.Errorf("marshal issued invoice snapshot: %w", err)
		}
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO invoices (id, tenant_id, customer_id, number, status, issue_date, due_date,
		                      currency, subtotal, tax_total, total, notes, metadata, issued_snapshot, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		ON CONFLICT (id) DO UPDATE SET
			customer_id = EXCLUDED.customer_id,
			status = EXCLUDED.status,
			issue_date = EXCLUDED.issue_date,
			due_date = EXCLUDED.due_date,
			currency = EXCLUDED.currency,
			subtotal = EXCLUDED.subtotal,
			tax_total = EXCLUDED.tax_total,
			total = EXCLUDED.total,
			notes = EXCLUDED.notes,
			metadata = EXCLUDED.metadata,
			issued_snapshot = COALESCE(invoices.issued_snapshot, EXCLUDED.issued_snapshot),
			updated_at = EXCLUDED.updated_at
	`,
		inv.ID, inv.TenantID, inv.CustomerID, inv.Number.Value, string(inv.Status),
		inv.IssueDate, inv.DueDate, inv.Currency,
		inv.Subtotal.Amount, inv.TaxTotal.Amount, inv.Total.Amount,
		inv.Notes, metadataJSON, snapshotJSON, inv.CreatedAt, inv.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert invoice: %w", err)
	}

	// Delete existing items and re-insert (simple approach for aggregate consistency)
	_, err = tx.Exec(ctx, "DELETE FROM invoice_items WHERE invoice_id = $1", inv.ID)
	if err != nil {
		return fmt.Errorf("delete items: %w", err)
	}

	for _, item := range inv.Items {
		lineTotal, _ := item.LineTotal()
		_, err = tx.Exec(ctx, `
			INSERT INTO invoice_items (id, invoice_id, description, quantity, unit_price,
			                           tax_rate_bps, discount_bps, total, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`,
			item.ID, inv.ID, item.Description, item.Quantity, item.UnitPrice.Amount,
			item.TaxRateBPS, item.DiscountBPS, lineTotal.Amount, time.Now(),
		)
		if err != nil {
			return fmt.Errorf("insert item: %w", err)
		}
	}

	return nil
}

func (r *InvoiceRepository) prepareIssuedSnapshot(ctx context.Context, tx pgx.Tx, inv *entity.Invoice) error {
	var persistedStatus entity.InvoiceStatus
	var persistedTenantID, persistedCustomerID string
	var persistedSnapshot []byte
	err := tx.QueryRow(ctx, `
		SELECT status, tenant_id, customer_id, issued_snapshot
		FROM invoices
		WHERE id = $1
		FOR UPDATE
	`, inv.ID).Scan(&persistedStatus, &persistedTenantID, &persistedCustomerID, &persistedSnapshot)
	if errors.Is(err, pgx.ErrNoRows) {
		inv.IssuedSnapshot = nil
		return nil
	}
	if err != nil {
		return fmt.Errorf("load invoice state before saving snapshot: %w", err)
	}
	if persistedTenantID != inv.TenantID {
		return errors.New("invoice tenant cannot be changed")
	}
	if persistedCustomerID != inv.CustomerID &&
		(persistedStatus != entity.InvoiceStatusDraft || inv.Status != entity.InvoiceStatusDraft || len(persistedSnapshot) > 0) {
		return errors.New("customer can only be changed while the invoice is a draft")
	}
	if len(persistedSnapshot) > 0 {
		var snapshot entity.InvoiceSnapshot
		if err := json.Unmarshal(persistedSnapshot, &snapshot); err != nil {
			return fmt.Errorf("decode persisted invoice snapshot: %w", err)
		}
		inv.IssuedSnapshot = &snapshot
		return nil
	}
	inv.IssuedSnapshot = nil
	if persistedStatus != entity.InvoiceStatusDraft || inv.Status != entity.InvoiceStatusIssued {
		return nil
	}

	var seller, customer entity.InvoicePartySnapshot
	var sellerAddress, customerAddress []byte
	err = tx.QueryRow(ctx, `
		SELECT t.name, t.tax_id, t.address,
		       c.name, c.email, c.phone, c.tax_id, c.address
		FROM tenants t
		JOIN customers c ON c.tenant_id = t.id
		WHERE t.id = $1 AND c.id = $2
		FOR SHARE OF t, c
	`, persistedTenantID, persistedCustomerID).Scan(
		&seller.Name, &seller.TaxID, &sellerAddress,
		&customer.Name, &customer.Email, &customer.Phone, &customer.TaxID, &customerAddress,
	)
	if err != nil {
		return fmt.Errorf("load seller and customer for invoice snapshot: %w", err)
	}
	if err := json.Unmarshal(sellerAddress, &seller.Address); err != nil {
		return fmt.Errorf("decode seller address for invoice snapshot: %w", err)
	}
	if err := json.Unmarshal(customerAddress, &customer.Address); err != nil {
		return fmt.Errorf("decode customer address for invoice snapshot: %w", err)
	}

	snapshot, err := entity.NewInvoiceSnapshot(inv, seller, customer)
	if err != nil {
		return fmt.Errorf("create issued invoice snapshot: %w", err)
	}
	inv.IssuedSnapshot = snapshot
	return nil
}

// saveEventsTx writes domain events to the outbox table.
func (r *InvoiceRepository) saveEventsTx(ctx context.Context, tx pgx.Tx, tenantID string, events []entity.DomainEvent) error {
	for _, event := range events {
		payload, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("marshal event %s: %w", event.EventName(), err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO outbox (id, aggregate_id, tenant_id, event_type, payload, status, created_at)
			VALUES ($1, $2, $3, $4, $5, 'pending', NOW())
		`,
			uuid.NewString(), event.AggregateID(), tenantID, event.EventName(), payload,
		)
		if err != nil {
			return fmt.Errorf("insert outbox: %w", err)
		}
	}
	return nil
}

// saveAuditTx writes an immutable audit entry.
func (r *InvoiceRepository) saveAuditTx(ctx context.Context, tx pgx.Tx, inv *entity.Invoice, events []entity.DomainEvent, created bool) error {
	payload, err := json.Marshal(map[string]any{
		"status":     string(inv.Status),
		"total":      inv.Total.Amount,
		"currency":   inv.Currency,
		"updated_at": inv.UpdatedAt,
	})
	if err != nil {
		return fmt.Errorf("marshal invoice audit payload: %w", err)
	}
	var actorID any
	if actor, ok := ctx.Value(ctxkey.ActorIDKey).(string); ok && actor != "" {
		if _, err := uuid.Parse(actor); err != nil {
			return fmt.Errorf("invalid audit actor ID: %w", err)
		}
		actorID = actor
	}
	action := "invoice.updated"
	if created {
		action = "invoice.created"
	}
	switch inv.Status {
	case entity.InvoiceStatusIssued:
		action = "invoice.issued"
	case entity.InvoiceStatusPaid:
		action = "invoice.paid"
	case entity.InvoiceStatusCancelled:
		action = "invoice.cancelled"
	}
	for _, event := range events {
		switch event.EventName() {
		case "payment.recorded":
			action = "payment.recorded"
		}
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO audit_entries (id, tenant_id, entity_type, entity_id, action, actor_id, payload, created_at)
		VALUES ($1, $2, 'invoice', $3, $4, $5, $6, NOW())
	`,
		uuid.NewString(), inv.TenantID, inv.ID, action, actorID, payload,
	)
	if err != nil {
		return fmt.Errorf("insert audit: %w", err)
	}
	return nil
}

// FindByID loads an invoice aggregate by ID.
func (r *InvoiceRepository) FindByID(ctx context.Context, id string) (*entity.Invoice, error) {
	// We need tenant_id from context to set RLS. In the application layer,
	// tenant_id is passed through context. We extract it here.
	tenantID, ok := TenantIDFromCtx(ctx)
	if !ok {
		return nil, errors.New("tenant_id not found in context")
	}

	conn, err := r.db.TenantConn(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	return r.loadInvoice(ctx, conn, id)
}

// FindByIDForUpdate loads an invoice and acquires a row lock (SELECT FOR UPDATE).
// Must be called within a transaction (provided via TxFromCtx).
// This prevents concurrent modifications to the invoice while the transaction is open.
func (r *InvoiceRepository) FindByIDForUpdate(ctx context.Context, id string) (*entity.Invoice, error) {
	tx := port.TxFromCtx(ctx)
	if tx == nil {
		return nil, errors.New("FindByIDForUpdate requires a transaction in context")
	}

	// Load invoice header with FOR UPDATE lock
	var inv entity.Invoice
	var number string
	var metadata, snapshotJSON []byte
	var issueDate *time.Time

	err := tx.QueryRow(ctx, `
		SELECT id, tenant_id, customer_id, number, status, issue_date, due_date,
		       currency, subtotal, tax_total, total, notes, metadata, issued_snapshot, created_at, updated_at
		FROM invoices WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`, id).Scan(
		&inv.ID, &inv.TenantID, &inv.CustomerID, &number,
		&inv.Status, &issueDate, &inv.DueDate,
		&inv.Currency, &inv.Subtotal.Amount, &inv.TaxTotal.Amount, &inv.Total.Amount,
		&inv.Notes, &metadata, &snapshotJSON, &inv.CreatedAt, &inv.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("load invoice for update: %w", err)
	}

	if issueDate != nil {
		inv.IssueDate = *issueDate
	}

	inv.Number = valueobject.InvoiceNumber{Value: number}
	if metadata != nil {
		json.Unmarshal(metadata, &inv.Metadata)
	}
	inv.IssuedSnapshot, err = decodeInvoiceSnapshot(snapshotJSON)
	if err != nil {
		return nil, err
	}

	// Load items (no lock needed — items are only modified via the aggregate)
	rows, err := tx.Query(ctx, `
		SELECT id, description, quantity, unit_price, tax_rate_bps, discount_bps, total
		FROM invoice_items WHERE invoice_id = $1
	`, id)
	if err != nil {
		return nil, fmt.Errorf("load items for update: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var item entity.InvoiceItem
		var lineTotal int64
		err := rows.Scan(
			&item.ID, &item.Description, &item.Quantity, &item.UnitPrice.Amount,
			&item.TaxRateBPS, &item.DiscountBPS, &lineTotal,
		)
		if err != nil {
			return nil, fmt.Errorf("scan item: %w", err)
		}
		item.UnitPrice.Currency = inv.Currency
		inv.Items = append(inv.Items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows err: %w", err)
	}

	return &inv, nil
}

// SaveTx persists an invoice within an existing transaction (from TxFromCtx).
// Use this when the invoice save is part of a larger transaction.
func (r *InvoiceRepository) SaveTx(ctx context.Context, invoice *entity.Invoice) error {
	tx := port.TxFromCtx(ctx)
	if tx == nil {
		return errors.New("SaveTx requires a transaction in context")
	}

	created, err := invoiceCreatedTx(ctx, tx, invoice.ID)
	if err != nil {
		return err
	}
	err = r.saveInvoiceTx(ctx, tx, invoice)
	if err != nil {
		return err
	}

	// Save domain events to outbox (same transaction)
	events := invoice.ClearEvents()
	if len(events) > 0 {
		if err := SaveEventsTx(ctx, tx, invoice.TenantID, events); err != nil {
			return err
		}
	}

	// Write audit entry
	if err := r.saveAuditTx(ctx, tx, invoice, events, created); err != nil {
		return err
	}
	return nil
}

func invoiceCreatedTx(ctx context.Context, tx pgx.Tx, invoiceID string) (bool, error) {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM invoices WHERE id = $1)`, invoiceID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check invoice existence for audit: %w", err)
	}
	return !exists, nil
}

// FindByNumber loads an invoice by tenant-scoped number.
func (r *InvoiceRepository) FindByNumber(ctx context.Context, tenantID, number string) (*entity.Invoice, error) {
	conn, err := r.db.TenantConn(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	var id string
	err = conn.QueryRow(ctx, "SELECT id FROM invoices WHERE number = $1", number).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find by number: %w", err)
	}

	return r.loadInvoice(ctx, conn, id)
}

// ExistsByNumber checks if an invoice number is already used within a tenant.
func (r *InvoiceRepository) ExistsByNumber(ctx context.Context, tenantID, number string) (bool, error) {
	conn, err := r.db.TenantConn(ctx, tenantID)
	if err != nil {
		return false, err
	}
	defer conn.Release()

	var exists bool
	err = conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM invoices WHERE number = $1)", number).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check number exists: %w", err)
	}
	return exists, nil
}

// NextInvoiceNumber generates the next sequential invoice number for a tenant.
// Format: INV-{YEAR}-{SEQUENCE} (e.g., INV-2024-00001)
// Uses a tenant-scoped sequence for gap-resistant numbering.
func (r *InvoiceRepository) NextInvoiceNumber(ctx context.Context, tenantID string) (string, error) {
	conn, err := r.db.TenantConn(ctx, tenantID)
	if err != nil {
		return "", err
	}
	defer conn.Release()

	// Atomically get and increment a counter using UPSERT on a sequence table.
	// This is gap-resistant: the number is consumed even if the invoice creation fails later.
	var seq int64
	year := time.Now().Year()
	err = conn.QueryRow(ctx, `
		INSERT INTO invoice_sequences (tenant_id, year, last_seq)
		VALUES ($1, $2, 1)
		ON CONFLICT (tenant_id, year)
		DO UPDATE SET last_seq = invoice_sequences.last_seq + 1
		RETURNING last_seq
	`, tenantID, year).Scan(&seq)
	if err != nil {
		return "", fmt.Errorf("next invoice number: %w", err)
	}

	return fmt.Sprintf("INV-%d-%05d", year, seq), nil
}

// FindByTenant returns a paginated list of invoices for a tenant.
func (r *InvoiceRepository) FindByTenant(ctx context.Context, tenantID string, limit, offset int) ([]*entity.Invoice, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	conn, err := r.db.TenantConn(ctx, tenantID)
	if err != nil {
		return nil, 0, err
	}
	defer conn.Release()

	// Count total invoices for the tenant (for pagination metadata).
	var total int
	err = conn.QueryRow(ctx,
		`SELECT count(*) FROM invoices WHERE deleted_at IS NULL`,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count invoices: %w", err)
	}

	// Collect IDs first, then close cursor before loading each invoice.
	rows, err := conn.Query(ctx, `
		SELECT id FROM invoices WHERE deleted_at IS NULL
		ORDER BY created_at DESC LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list invoices: %w", err)
	}

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, 0, err
	}
	rows.Close()

	var invoices []*entity.Invoice
	for _, id := range ids {
		inv, err := r.loadInvoice(ctx, conn, id)
		if err != nil {
			return nil, 0, err
		}
		invoices = append(invoices, inv)
	}
	return invoices, total, nil
}

// FindOverdue returns invoices past their due date that are still in "issued" status.
func (r *InvoiceRepository) FindOverdue(ctx context.Context, tenantID string, asOf time.Time) ([]*entity.Invoice, error) {
	conn, err := r.db.TenantConn(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	// Collect all IDs first, then close the cursor before loading each invoice.
	// This avoids "conn busy" errors from pgx when trying to run a second query
	// on a connection that still has an open result set.
	rows, err := conn.Query(ctx, `
		SELECT id FROM invoices
		WHERE status = 'issued' AND due_date < $1 AND deleted_at IS NULL
		ORDER BY due_date ASC
	`, asOf)
	if err != nil {
		return nil, fmt.Errorf("find overdue: %w", err)
	}

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close() // Close the cursor before using conn for more queries

	var invoices []*entity.Invoice
	for _, id := range ids {
		inv, err := r.loadInvoice(ctx, conn, id)
		if err != nil {
			return nil, err
		}
		invoices = append(invoices, inv)
	}
	return invoices, nil
}

// loadInvoice loads a full invoice aggregate (header + items) from the database.
func (r *InvoiceRepository) loadInvoice(ctx context.Context, conn *pgxpool.Conn, id string) (*entity.Invoice, error) {
	var inv entity.Invoice
	var number string
	var metadata, snapshotJSON []byte
	var issueDate *time.Time

	err := conn.QueryRow(ctx, `
		SELECT id, tenant_id, customer_id, number, status, issue_date, due_date,
		       currency, subtotal, tax_total, total, notes, metadata, issued_snapshot, created_at, updated_at
		FROM invoices WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(
		&inv.ID, &inv.TenantID, &inv.CustomerID, &number,
		&inv.Status, &issueDate, &inv.DueDate,
		&inv.Currency, &inv.Subtotal.Amount, &inv.TaxTotal.Amount, &inv.Total.Amount,
		&inv.Notes, &metadata, &snapshotJSON, &inv.CreatedAt, &inv.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("load invoice: %w", err)
	}

	if issueDate != nil {
		inv.IssueDate = *issueDate
	}

	inv.Number = valueobject.InvoiceNumber{Value: number}
	if metadata != nil {
		json.Unmarshal(metadata, &inv.Metadata)
	}
	inv.IssuedSnapshot, err = decodeInvoiceSnapshot(snapshotJSON)
	if err != nil {
		return nil, err
	}

	// Load items
	rows, err := conn.Query(ctx, `
		SELECT id, description, quantity, unit_price, tax_rate_bps, discount_bps, total
		FROM invoice_items WHERE invoice_id = $1
	`, id)
	if err != nil {
		return nil, fmt.Errorf("load items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var item entity.InvoiceItem
		var lineTotal int64
		err := rows.Scan(
			&item.ID, &item.Description, &item.Quantity, &item.UnitPrice.Amount,
			&item.TaxRateBPS, &item.DiscountBPS, &lineTotal,
		)
		if err != nil {
			return nil, fmt.Errorf("scan item: %w", err)
		}
		item.UnitPrice.Currency = inv.Currency
		inv.Items = append(inv.Items, item)
	}

	return &inv, rows.Err()
}

func decodeInvoiceSnapshot(encoded []byte) (*entity.InvoiceSnapshot, error) {
	if len(encoded) == 0 {
		return nil, nil
	}
	var snapshot entity.InvoiceSnapshot
	if err := json.Unmarshal(encoded, &snapshot); err != nil {
		return nil, fmt.Errorf("decode issued invoice snapshot: %w", err)
	}
	return &snapshot, nil
}

// tenantCtxKey is the legacy context key for tenant_id.
// Kept for backward compatibility; new code should use ctxkey.TenantIDKey.
type tenantCtxKey struct{}

// WithTenantID returns a context with the tenant_id set, for repository use.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, ctxkey.TenantIDKey, tenantID)
}

// TenantIDFromCtx extracts tenant_id from context. It checks the shared
// ctxkey.TenantIDKey (set by the API middleware) first, then falls back
// to the legacy tenantCtxKey for backward compatibility.
func TenantIDFromCtx(ctx context.Context) (string, bool) {
	if id, ok := ctx.Value(ctxkey.TenantIDKey).(string); ok && id != "" {
		return id, true
	}
	id, ok := ctx.Value(tenantCtxKey{}).(string)
	return id, ok
}

// Ensure interface compliance
var _ port.InvoiceRepository = (*InvoiceRepository)(nil)
