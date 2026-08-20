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
	"github.com/santiagossaa/invoice-generator/internal/domain/valueobject"
)

// TenantRepository implements port.TenantRepository using PostgreSQL.
// Tenant operations don't use RLS (tenants are the RLS root — they can't isolate themselves).
type TenantRepository struct {
	db *DB
}

func NewTenantRepository(db *DB) *TenantRepository {
	return &TenantRepository{db: db}
}

func (r *TenantRepository) FindByID(ctx context.Context, id string) (*entity.Tenant, error) {
	// Tenants are accessed directly (no RLS — they ARE the tenant boundary)
	conn, err := r.db.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Release()

	var t entity.Tenant
	var addrJSON, settingsJSON []byte

	err = conn.QueryRow(ctx, `
		SELECT id, name, address, tax_id, settings, created_at, updated_at
		FROM tenants WHERE id = $1
	`, id).Scan(
		&t.ID, &t.Name, &addrJSON, &t.TaxID, &settingsJSON,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("load tenant: %w", err)
	}

	if addrJSON != nil {
		json.Unmarshal(addrJSON, &t.Address)
	}
	if settingsJSON != nil {
		json.Unmarshal(settingsJSON, &t.Settings)
	}

	return &t, nil
}

func (r *TenantRepository) Save(ctx context.Context, tenant *entity.Tenant) error {
	conn, err := r.db.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Release()

	addrJSON, _ := json.Marshal(tenant.Address)
	settingsJSON, _ := json.Marshal(tenant.Settings)

	_, err = conn.Exec(ctx, `
		INSERT INTO tenants (id, name, address, tax_id, settings, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			address = EXCLUDED.address,
			tax_id = EXCLUDED.tax_id,
			settings = EXCLUDED.settings,
			updated_at = EXCLUDED.updated_at
	`,
		tenant.ID, tenant.Name, addrJSON, tenant.TaxID,
		settingsJSON, tenant.CreatedAt, tenant.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save tenant: %w", err)
	}
	return nil
}

// TaxRuleRepository implements port.TaxResolverPort using PostgreSQL.
type TaxRuleRepository struct {
	db *DB
}

func NewTaxRuleRepository(db *DB) *TaxRuleRepository {
	return &TaxRuleRepository{db: db}
}

// Resolve finds the applicable tax rule for a jurisdiction and date.
func (r *TaxRuleRepository) Resolve(ctx context.Context, jurisdiction string, region string, at time.Time) (*valueobject.TaxRule, error) {
	tenantID, ok := TenantIDFromCtx(ctx)
	if !ok {
		return nil, errors.New("tenant_id not in context")
	}

	conn, err := r.db.TenantConn(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	var rule valueobject.TaxRule
	var effectiveTo *time.Time

	query := `
		SELECT jurisdiction, COALESCE(region, ''), rate_bps, COALESCE(description, ''),
		       effective_from, effective_to
		FROM tax_rules
		WHERE tenant_id = $1 AND jurisdiction = $2
	`
	args := []any{tenantID, jurisdiction}

	if region != "" {
		query += ` AND (region = $3 OR region IS NULL)`
		args = append(args, region)
	}

	query += ` AND effective_from <= $` + fmt.Sprintf("%d", len(args)+1)
	args = append(args, at)

	// Handle effective_to: NULL or >= at
	query += ` AND (effective_to IS NULL OR effective_to >= $` + fmt.Sprintf("%d", len(args)+1) + `)`
	args = append(args, at)

	query += ` ORDER BY effective_from DESC LIMIT 1`

	err = conn.QueryRow(ctx, query, args...).Scan(
		&rule.Jurisdiction, &rule.Region, &rule.RateBPS,
		&rule.Description, &rule.EffectiveFrom, &effectiveTo,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // No tax rule = tax-exempt
		}
		return nil, fmt.Errorf("resolve tax rule: %w", err)
	}

	if effectiveTo != nil {
		rule.EffectiveTo = effectiveTo
	}

	return &rule, nil
}

var (
	_ port.TenantRepository       = (*TenantRepository)(nil)
	_ valueobject.TaxResolverPort = (*TaxRuleRepository)(nil)
)
