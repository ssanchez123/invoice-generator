package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/domain/port"
)

// UserRepository implements port.UserRepository using PostgreSQL.
type UserRepository struct {
	db *DB
}

func NewUserRepository(db *DB) *UserRepository {
	return &UserRepository{db: db}
}

// Save creates or updates a user. Uses RLS via TenantTx.
func (r *UserRepository) Save(ctx context.Context, user *entity.User) error {
	tx, err := r.db.TenantTx(ctx, user.TenantID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) // safe: no-op if committed

	_, err = tx.Exec(ctx, `
		INSERT INTO users (id, tenant_id, email, role, password_hash, first_name, last_name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			email = EXCLUDED.email,
			role = EXCLUDED.role,
			password_hash = EXCLUDED.password_hash,
			first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name,
			updated_at = EXCLUDED.updated_at
	`,
		user.ID, user.TenantID, user.Email, string(user.Role),
		user.PasswordHash, nullableString(user.FirstName), nullableString(user.LastName),
		user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save user: %w", err)
	}

	return tx.Commit(ctx)
}

// FindByEmail finds a user by email within a tenant.
func (r *UserRepository) FindByEmail(ctx context.Context, tenantID, email string) (*entity.User, error) {
	conn, err := r.db.TenantConn(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	return r.scanUser(conn.QueryRow(ctx, `
		SELECT id, tenant_id, email, role, password_hash, first_name, last_name, created_at, updated_at
		FROM users WHERE email = $1
	`, email))
}

// FindByID finds a user by ID. Requires tenant_id in context for RLS.
func (r *UserRepository) FindByID(ctx context.Context, id string) (*entity.User, error) {
	tenantID, ok := TenantIDFromCtx(ctx)
	if !ok {
		return nil, errors.New("tenant_id not found in context")
	}

	conn, err := r.db.TenantConn(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	return r.scanUser(conn.QueryRow(ctx, `
		SELECT id, tenant_id, email, role, password_hash, first_name, last_name, created_at, updated_at
		FROM users WHERE id = $1
	`, id))
}

// EmailExists checks if an email is already registered for a tenant.
func (r *UserRepository) EmailExists(ctx context.Context, tenantID, email string) (bool, error) {
	conn, err := r.db.TenantConn(ctx, tenantID)
	if err != nil {
		return false, err
	}
	defer conn.Release()

	var exists bool
	err = conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", email).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check email exists: %w", err)
	}
	return exists, nil
}

// scanUser scans a pgx.Row into a User entity.
func (r *UserRepository) scanUser(row pgx.Row) (*entity.User, error) {
	var u entity.User
	var role string
	var firstName, lastName *string

	err := row.Scan(
		&u.ID, &u.TenantID, &u.Email, &role, &u.PasswordHash,
		&firstName, &lastName, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("load user: %w", err)
	}

	u.Role = entity.UserRole(role)
	if firstName != nil {
		u.FirstName = *firstName
	}
	if lastName != nil {
		u.LastName = *lastName
	}

	return &u, nil
}

// nullableString returns nil for empty strings (for SQL NULL insertion), or the string itself.
func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Ensure interface compliance
var _ port.UserRepository = (*UserRepository)(nil)

// Compile-time assertion that pgxpool is used.
var _ = (*pgxpool.Conn)(nil)
