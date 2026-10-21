//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/santiagossaa/invoice-generator/internal/infrastructure/repository"
)

func TestOutboxFailedDeliveryReplayRequiresOperatorAndAudits(t *testing.T) {
	testDB := SetupTestDB(t)
	ctx := ContextWithTenant(TestTenantID)
	outboxRepo := repository.NewOutboxRepository(testDB.DB)
	eventID := saveOutboxEvent(t, testDB, outboxRepo, TestTenantID, "invoice.issued")

	if _, err := testDB.Pool.Exec(context.Background(), `
		UPDATE outbox_deliveries
		SET status = 'failed', attempts = 10, last_error = 'terminal test failure'
		WHERE event_id = $1 AND tenant_id = $2 AND consumer = 'pdf-service'
	`, eventID, TestTenantID); err != nil {
		t.Fatalf("mark delivery failed: %v", err)
	}

	appTx, err := testDB.DB.TenantTx(ctx, TestTenantID)
	if err != nil {
		t.Fatalf("begin app transaction: %v", err)
	}
	_, appErr := appTx.Exec(ctx, `
		SELECT public.requeue_failed_outbox_delivery($1, $2, $3, $4)
	`, TestTenantID, eventID, "pdf-service", "app role must not replay")
	_ = appTx.Rollback(ctx)
	if !hasSQLState(appErr, "42501") {
		t.Fatalf("application role should not replay failed deliveries, got %v", appErr)
	}

	adminTx, err := testDB.Pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin admin transaction: %v", err)
	}
	if _, err := adminTx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, TestTenantID); err != nil {
		adminTx.Rollback(ctx)
		t.Fatalf("set admin tenant context: %v", err)
	}
	_, adminErr := adminTx.Exec(ctx, `
		SELECT public.requeue_failed_outbox_delivery($1, $2, $3, $4)
	`, TestTenantID, eventID, "pdf-service", "superuser must use operator login")
	_ = adminTx.Rollback(ctx)
	if !hasSQLState(adminErr, "42501") {
		t.Fatalf("superuser should not replay failed deliveries, got %v", adminErr)
	}

	operatorTx, err := testDB.OperatorDB.TenantTx(ctx, TestTenantID)
	if err != nil {
		t.Fatalf("begin operator transaction: %v", err)
	}
	_, err = operatorTx.Exec(ctx, `
		SELECT public.requeue_failed_outbox_delivery($1, $2, $3, $4)
	`, uuid.NewString(), eventID, "pdf-service", "cross-tenant replay")
	if !hasSQLState(err, "42501") {
		operatorTx.Rollback(ctx)
		t.Fatalf("operator should not replay under a mismatched tenant context, got %v", err)
	}
	operatorTx.Rollback(ctx)

	operatorTx, err = testDB.OperatorDB.TenantTx(ctx, TestTenantID)
	if err != nil {
		t.Fatalf("begin authorized operator transaction: %v", err)
	}
	_, err = operatorTx.Exec(ctx, `
		SELECT public.requeue_failed_outbox_delivery($1, $2, $3, $4)
	`, TestTenantID, eventID, "pdf-service", "replay after fixing renderer")
	if err != nil {
		operatorTx.Rollback(ctx)
		t.Fatalf("operator replay: %v", err)
	}
	if err := operatorTx.Commit(ctx); err != nil {
		t.Fatalf("commit operator replay: %v", err)
	}

	var status, lastError string
	var attempts int
	if err := testDB.Pool.QueryRow(context.Background(), `
		SELECT status, attempts, last_error
		FROM outbox_deliveries
		WHERE event_id = $1 AND tenant_id = $2 AND consumer = 'pdf-service'
	`, eventID, TestTenantID).Scan(&status, &attempts, &lastError); err != nil {
		t.Fatalf("read requeued delivery: %v", err)
	}
	if status != "pending" || attempts != 0 || lastError != "terminal test failure" {
		t.Fatalf("unexpected replayed delivery state: status=%s attempts=%d last_error=%q", status, attempts, lastError)
	}

	var login, reason string
	var attemptsBefore int
	if err := testDB.Pool.QueryRow(context.Background(), `
		SELECT operator_login, reason, attempts_before
		FROM outbox_delivery_replays
		WHERE tenant_id = $1 AND event_id = $2 AND consumer = 'pdf-service'
	`, TestTenantID, eventID).Scan(&login, &reason, &attemptsBefore); err != nil {
		t.Fatalf("read replay audit: %v", err)
	}
	if login != "invoicegen_ops_test" || reason != "replay after fixing renderer" || attemptsBefore != 10 {
		t.Fatalf("unexpected replay audit: login=%s reason=%q attempts=%d", login, reason, attemptsBefore)
	}

	readTx, err := testDB.DB.TenantTx(ctx, TestTenantID)
	if err != nil {
		t.Fatalf("begin app audit query: %v", err)
	}
	var auditRows int
	err = readTx.QueryRow(ctx, `SELECT COUNT(*) FROM outbox_delivery_replays`).Scan(&auditRows)
	_ = readTx.Rollback(ctx)
	if !hasSQLState(err, "42501") {
		t.Fatalf("application role should not read replay audit, got %v", err)
	}

	repeatTx, err := testDB.OperatorDB.TenantTx(ctx, TestTenantID)
	if err != nil {
		t.Fatalf("begin duplicate replay transaction: %v", err)
	}
	_, err = repeatTx.Exec(ctx, `
		SELECT public.requeue_failed_outbox_delivery($1, $2, $3, $4)
	`, TestTenantID, eventID, "pdf-service", "duplicate replay")
	_ = repeatTx.Rollback(ctx)
	if !hasSQLState(err, "P0002") {
		t.Fatalf("already requeued delivery should not be replayed again, got %v", err)
	}
}

func hasSQLState(err error, expected string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == expected
}
