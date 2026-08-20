package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/domain/port"
	"github.com/santiagossaa/invoice-generator/internal/domain/valueobject"
)

// CustomerRepository implements port.CustomerRepository using PostgreSQL.
type CustomerRepository struct {
	db *DB
}

func NewCustomerRepository(db *DB) *CustomerRepository {
	return &CustomerRepository{db: db}
}

// Save creates or updates a customer.
func (r *CustomerRepository) Save(ctx context.Context, customer *entity.Customer) error {
	conn, err := r.db.TenantConn(ctx, customer.TenantID)
	if err != nil {
		return err
	}
	defer conn.Release()

	addrJSON, err := json.Marshal(customer.Address)
	if err != nil {
		return fmt.Errorf("marshal address: %w", err)
	}

	metaJSON, err := json.Marshal(customer.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	_, err = conn.Exec(ctx, `
		INSERT INTO customers (id, tenant_id, name, email, phone, address, tax_id, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			email = EXCLUDED.email,
			phone = EXCLUDED.phone,
			address = EXCLUDED.address,
			tax_id = EXCLUDED.tax_id,
			metadata = EXCLUDED.metadata,
			updated_at = EXCLUDED.updated_at
	`,
		customer.ID, customer.TenantID, customer.Name, customer.Email, customer.Phone,
		addrJSON, customer.TaxID, metaJSON, customer.CreatedAt, customer.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save customer: %w", err)
	}

	return nil
}

// FindByID loads a customer by ID.
func (r *CustomerRepository) FindByID(ctx context.Context, id string) (*entity.Customer, error) {
	tenantID, ok := TenantIDFromCtx(ctx)
	if !ok {
		return nil, errors.New("tenant_id not in context")
	}

	conn, err := r.db.TenantConn(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	return r.loadCustomer(ctx, conn, id)
}

// FindByTenant returns a paginated list of customers for a tenant.
func (r *CustomerRepository) FindByTenant(ctx context.Context, tenantID string, limit, offset int) ([]*entity.Customer, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	conn, err := r.db.TenantConn(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	// Collect all IDs first, then close the cursor before loading each customer.
	// This avoids "conn busy" errors from pgx when trying to run a second query
	// on a connection that still has an open result set.
	rows, err := conn.Query(ctx, `
		SELECT id FROM customers WHERE deleted_at IS NULL
		ORDER BY created_at DESC LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list customers: %w", err)
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

	var customers []*entity.Customer
	for _, id := range ids {
		c, err := r.loadCustomer(ctx, conn, id)
		if err != nil {
			return nil, err
		}
		customers = append(customers, c)
	}
	return customers, nil
}

// Delete soft-deletes a customer (marks as deleted, never hard-delete for audit integrity).
func (r *CustomerRepository) Delete(ctx context.Context, id string) error {
	tenantID, ok := TenantIDFromCtx(ctx)
	if !ok {
		return errors.New("tenant_id not in context")
	}

	conn, err := r.db.TenantConn(ctx, tenantID)
	if err != nil {
		return err
	}
	defer conn.Release()

	tag, err := conn.Exec(ctx, "UPDATE customers SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL", id)
	if err != nil {
		return fmt.Errorf("delete customer: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *CustomerRepository) loadCustomer(ctx context.Context, conn *pgxpool.Conn, id string) (*entity.Customer, error) {
	var c entity.Customer
	var addrJSON, metaJSON []byte

	err := conn.QueryRow(ctx, `
		SELECT id, tenant_id, name, email, phone, address, tax_id, metadata, created_at, updated_at
		FROM customers WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(
		&c.ID, &c.TenantID, &c.Name, &c.Email, &c.Phone,
		&addrJSON, &c.TaxID, &metaJSON, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("load customer: %w", err)
	}

	if addrJSON != nil {
		json.Unmarshal(addrJSON, &c.Address)
	}
	if metaJSON != nil {
		json.Unmarshal(metaJSON, &c.Metadata)
	}

	// Ensure address is at least zero-value valid
	if c.Address.Country == "" {
		c.Address = valueobject.Address{}
	}

	return &c, nil
}

var _ port.CustomerRepository = (*CustomerRepository)(nil)
