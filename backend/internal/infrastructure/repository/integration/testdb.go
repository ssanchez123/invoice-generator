//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/santiagossaa/invoice-generator/internal/infrastructure/repository"
)

// TestDB holds the PostgreSQL connection pool and helpers.
type TestDB struct {
	Pool       *pgxpool.Pool
	DB         *repository.DB
	OperatorDB *repository.DB
	TenantID   string
}

// SetupTestDB connects to a PostgreSQL instance (from env vars or testcontainers),
// runs migrations and seed data.
// Call t.Cleanup(db.Cleanup) to tear down.
func SetupTestDB(t *testing.T) *TestDB {
	t.Helper()
	ctx := context.Background()

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Connect to PostgreSQL using env vars (set by docker-compose or manually)
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbName := getEnv("DB_NAME", "invoicegen_test")
	dbUser := getEnv("DB_USER", "testuser")
	dbPass := getEnv("DB_PASSWORD", "testpass")

	// Admin pool (superuser): runs migrations, seed, and role bootstrap.
	// The repositories under test must NOT run as superuser — superusers
	// bypass Row-Level Security, which would make the RLS tests meaningless
	// and hide tenant-isolation regressions.
	adminConnStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", dbUser, dbPass, dbHost, dbPort, dbName)

	pool, err := pgxpool.New(ctx, adminConnStr)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	// Wait for DB to be ready
	for i := 0; i < 30; i++ {
		if err := pool.Ping(ctx); err == nil {
			break
		}
		time.Sleep(time.Second)
		if i == 29 {
			t.Fatalf("failed to ping db after 30s: %v", err)
		}
	}

	// Bootstrap the non-superuser application role that the repositories
	// will connect as. Mirrors deploy/docker/init-db.sql (ADR-004: RLS only
	// applies to non-superusers). Idempotent: safe to run on reused databases.
	ensureAppRole(t, ctx, pool, dbPass)
	ensureOperatorRole(t, ctx, pool)

	// Run migrations
	migrationsDir := findMigrationsDir(t)
	runMigrations(t, ctx, pool, migrationsDir)

	// Run seed data
	runSeed(t, ctx, pool, migrationsDir)

	// Application pool: connect as the non-superuser role so RLS policies
	// are actually enforced for the code under test.
	appUser := getEnv("DB_APP_USER", "invoicegen_app")
	appConnStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", appUser, dbPass, dbHost, dbPort, dbName)
	db, err := repository.NewDB(ctx, appConnStr)
	if err != nil {
		t.Fatalf("failed to create DB wrapper: %v", err)
	}
	operatorConnStr := fmt.Sprintf("postgres://invoicegen_ops_test:%s@%s:%s/%s?sslmode=disable", "operator-test-password", dbHost, dbPort, dbName)
	operatorDB, err := repository.NewDB(ctx, operatorConnStr)
	if err != nil {
		t.Fatalf("failed to create operator DB wrapper: %v", err)
	}

	testDB := &TestDB{
		Pool:       pool,
		DB:         db,
		OperatorDB: operatorDB,
		TenantID:   TestTenantID,
	}

	// Start every test from a clean slate: tests use fixed primary keys and
	// assume an empty (but seeded) database. Without this, re-running the
	// suite against a reused database fails with duplicate-key errors and
	// half-deleted fixtures from previous runs.
	testDB.ResetDB(t)

	t.Cleanup(func() {
		testDB.Cleanup()
	})

	return testDB
}

// Cleanup closes the pool and DB.
func (tdb *TestDB) Cleanup() {
	if tdb.DB != nil {
		tdb.DB.Close()
	}
	if tdb.OperatorDB != nil {
		tdb.OperatorDB.Close()
	}
	if tdb.Pool != nil {
		tdb.Pool.Close()
	}
}

// ensureAppRole creates (if needed) the non-superuser role used by the
// application, and grants it access to existing and future objects.
// Superusers bypass RLS; repositories must run as a regular role for the
// tenant-isolation tests to be meaningful.
func ensureAppRole(t *testing.T, ctx context.Context, admin *pgxpool.Pool, password string) {
	t.Helper()
	if _, err := admin.Exec(ctx, `
		DO $$
		BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'invoicegen_app') THEN
				CREATE ROLE invoicegen_app LOGIN PASSWORD '`+password+`' NOSUPERUSER;
			END IF;
		END
		$$;`); err != nil {
		t.Fatalf("create app role: %v", err)
	}
	grants := []string{
		`GRANT CONNECT ON DATABASE ` + getEnv("DB_NAME", "invoicegen_test") + ` TO invoicegen_app`,
		`GRANT USAGE ON SCHEMA public TO invoicegen_app`,
		// Future objects (tables are created by migrations after this runs)
		`ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO invoicegen_app`,
		`ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO invoicegen_app`,
		// Objects that already exist (reused databases)
		`GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO invoicegen_app`,
		`GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO invoicegen_app`,
		// Consumer routing is migration-managed; the runtime role is read-only.
		`DO $$ BEGIN
			IF to_regclass('public.outbox_subscriptions') IS NOT NULL THEN
				EXECUTE 'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON outbox_subscriptions FROM invoicegen_app';
				EXECUTE 'GRANT SELECT ON outbox_subscriptions TO invoicegen_app';
			END IF;
			IF to_regclass('public.outbox_delivery_replays') IS NOT NULL THEN
				EXECUTE 'REVOKE ALL ON outbox_delivery_replays FROM invoicegen_app';
			END IF;
		END $$`,
	}
	for _, g := range grants {
		if _, err := admin.Exec(ctx, g); err != nil {
			t.Fatalf("grant %q: %v", g, err)
		}
	}
}

func ensureOperatorRole(t *testing.T, ctx context.Context, admin *pgxpool.Pool) {
	t.Helper()
	if _, err := admin.Exec(ctx, `
		DO $$
		BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'invoicegen_operator') THEN
				CREATE ROLE invoicegen_operator NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE;
			END IF;
			IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'invoicegen_ops_test') THEN
				CREATE ROLE invoicegen_ops_test LOGIN PASSWORD 'operator-test-password' NOSUPERUSER;
			ELSE
				ALTER ROLE invoicegen_ops_test WITH LOGIN NOSUPERUSER PASSWORD 'operator-test-password';
			END IF;
			EXECUTE 'GRANT invoicegen_operator TO invoicegen_ops_test';
			EXECUTE 'GRANT CONNECT ON DATABASE ' || quote_ident(current_database()) || ' TO invoicegen_operator';
			EXECUTE 'GRANT USAGE ON SCHEMA public TO invoicegen_operator';
		END
		$$;
	`); err != nil {
		t.Fatalf("create operator role: %v", err)
	}
}

// ResetDB truncates all tables for test isolation. Call between tests if needed.
func (tdb *TestDB) ResetDB(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	tables := []string{
		"outbox_delivery_replays", "idempotency_records", "payments", "invoice_items", "invoices", "invoice_sequences",
		"users", "customers", "tenants", "outbox", "audit_entries",
	}
	for _, tbl := range tables {
		if _, err := tdb.Pool.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE", tbl)); err != nil {
			// Table might not exist yet, ignore
			_ = err
		}
	}
	// Re-seed
	migrationsDir := findMigrationsDir(t)
	runSeed(t, ctx, tdb.Pool, migrationsDir)
}

func findMigrationsDir(t *testing.T) string {
	t.Helper()
	paths := []string{
		"../../../../migrations",
		"../../../../../migrations",
		"./migrations",
		"/app/migrations",
	}
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		if _, err := os.Stat(abs); err == nil {
			return abs
		}
	}
	cwd, _ := os.Getwd()
	for i := 0; i < 10; i++ {
		candidate := filepath.Join(cwd, "migrations")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		cwd = filepath.Dir(cwd)
	}
	t.Fatal("could not find migrations directory")
	return ""
}

func runMigrations(t *testing.T, ctx context.Context, pool *pgxpool.Pool, migrationsDir string) {
	t.Helper()
	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`)
	if err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		t.Fatalf("glob migrations: %v", err)
	}
	sort.Strings(files)
	for _, file := range files {
		version := strings.TrimSuffix(filepath.Base(file), ".up.sql")
		var applied bool
		pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", version).Scan(&applied)
		if applied {
			continue
		}
		sql, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read migration %s: %v", file, err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("exec migration %s: %v", file, err)
		}
		pool.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version)
		t.Logf("migration %s: applied", version)
	}
}

func runSeed(t *testing.T, ctx context.Context, pool *pgxpool.Pool, migrationsDir string) {
	t.Helper()
	seedFile := filepath.Join(migrationsDir, "003_seed_dev.up.sql")
	sql, err := os.ReadFile(seedFile)
	if err != nil {
		t.Fatalf("read seed: %v", err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin seed tx: %v", err)
	}
	if _, err := tx.Exec(ctx, string(sql)); err != nil {
		tx.Rollback(ctx)
		t.Fatalf("exec seed: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit seed: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func ContextWithTenant(tenantID string) context.Context {
	return repository.WithTenantID(context.Background(), tenantID)
}

const TestTenantID = "00000000-0000-0000-0000-000000000001"
const TestCustomerID = "00000000-0000-0000-0000-000000000010"
