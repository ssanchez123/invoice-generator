package repository

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB is the PostgreSQL connection manager.
// It wraps a pgxpool and provides tenant-scoped transactions.
//
// Key responsibility: set the `app.tenant_id` session variable before
// any query, enabling PostgreSQL Row-Level Security policies (ADR-004)
// to enforce tenant isolation at the database level.
type DB struct {
	pool *pgxpool.Pool
}

// NewDB creates a new connection pool from a database URL.
func NewDB(ctx context.Context, databaseURL string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}

	// Connection pool tuning
	cfg.MaxConns = 25
	cfg.MinConns = 5
	cfg.MaxConnLifetime = 0 // let pool manage
	cfg.MaxConnIdleTime = 0

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	// Verify connectivity
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return &DB{pool: pool}, nil
}

// Close closes the connection pool.
func (db *DB) Close() {
	db.pool.Close()
}

// Pool returns the underlying pgxpool for direct access (e.g., migrations).
func (db *DB) Pool() *pgxpool.Pool {
	return db.pool
}

// TenantTx begins a transaction and sets the tenant_id session variable
// for Row-Level Security. Every repository operation MUST use this.
//
// Usage:
//
//	tx, err := db.TenantTx(ctx, tenantID)
//	defer tx.Rollback(ctx) // safe: no-op if committed
//	// ... queries using tx ...
//	tx.Commit(ctx)
func (db *DB) TenantTx(ctx context.Context, tenantID string) (pgx.Tx, error) {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}

	// Set tenant_id for RLS policies. Use set_config() instead of SET LOCAL
	// because pgx uses the extended query protocol, which sends parameters
	// as $1 placeholders — and SET LOCAL does not support parameterized values.
	// set_config('name', 'value', is_local) is a regular function that
	// accepts parameters normally. The third argument `true` makes it
	// equivalent to SET LOCAL (scoped to the current transaction).
	_, err = tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID)
	if err != nil {
		tx.Rollback(ctx)
		return nil, fmt.Errorf("set tenant_id: %w", err)
	}

	return tx, nil
}

// TenantConn acquires a connection and sets the tenant_id session variable.
// Use for read-only queries that don't need a full transaction.
//
// The caller must return the connection to the pool via conn.Release().
func (db *DB) TenantConn(ctx context.Context, tenantID string) (*pgxpool.Conn, error) {
	conn, err := db.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire conn: %w", err)
	}

	// Use set_config() for the same reason as TenantTx — pgx extended protocol
	// doesn't support parameterized SET statements. The third argument `false`
	// makes it session-scoped (equivalent to SET, not SET LOCAL), which is
	// correct for a connection that may be used across multiple queries.
	_, err = conn.Exec(ctx, "SELECT set_config('app.tenant_id', $1, false)", tenantID)
	if err != nil {
		conn.Release()
		return nil, fmt.Errorf("set tenant_id: %w", err)
	}

	return conn, nil
}

// Ping checks database connectivity.
func (db *DB) Ping(ctx context.Context) error {
	return db.pool.Ping(ctx)
}

// DatabaseURL builds a PostgreSQL connection URL from environment variables.
func DatabaseURL() string {
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	name := getEnv("DB_NAME", "invoicegen")
	user := getEnv("DB_USER", "invoicegen")
	password := getEnv("DB_PASSWORD", "invoicegen_dev")
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, password, host, port, name)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}