//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/infrastructure/repository"
)

func TestOutboxFanOutAndIndependentRetry(t *testing.T) {
	testDB := SetupTestDB(t)
	ctx := ContextWithTenant(TestTenantID)
	outboxRepo := repository.NewOutboxRepository(testDB.DB)

	eventID := saveOutboxEvent(t, testDB, outboxRepo, TestTenantID, "invoice.issued")

	var deliveries int
	if err := testDB.Pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM outbox_deliveries WHERE event_id = $1
	`, eventID).Scan(&deliveries); err != nil {
		t.Fatalf("count fan-out deliveries: %v", err)
	}
	if deliveries != 2 {
		t.Fatalf("expected two independent deliveries, got %d", deliveries)
	}

	relay, err := outboxRepo.ClaimDeliveries(ctx, TestTenantID, "relay-worker", "relay-1", 1, time.Minute)
	if err != nil {
		t.Fatalf("claim relay delivery: %v", err)
	}
	pdf, err := outboxRepo.ClaimDeliveries(ctx, TestTenantID, "pdf-service", "pdf-1", 1, time.Minute)
	if err != nil {
		t.Fatalf("claim PDF delivery: %v", err)
	}
	if len(relay) != 1 || len(pdf) != 1 {
		t.Fatalf("expected one delivery for each consumer, got relay=%d pdf=%d", len(relay), len(pdf))
	}

	if err := outboxRepo.CompleteDelivery(ctx, TestTenantID, eventID, "relay-worker", "relay-1"); err != nil {
		t.Fatalf("complete relay delivery: %v", err)
	}
	assertOutboxStatus(t, testDB, eventID, "pending")

	if err := outboxRepo.RetryDelivery(ctx, TestTenantID, eventID, "pdf-service", "pdf-1", errors.New("temporary failure")); err != nil {
		t.Fatalf("retry PDF delivery: %v", err)
	}
	pdf, err = outboxRepo.ClaimDeliveries(ctx, TestTenantID, "pdf-service", "pdf-2", 1, time.Minute)
	if err != nil {
		t.Fatalf("claim before retry backoff: %v", err)
	}
	if len(pdf) != 0 {
		t.Fatal("delivery was claimed before its retry delay elapsed")
	}

	makeDeliveryAvailable(t, testDB, eventID, "pdf-service")
	pdf, err = outboxRepo.ClaimDeliveries(ctx, TestTenantID, "pdf-service", "pdf-2", 1, time.Minute)
	if err != nil {
		t.Fatalf("reclaim PDF delivery: %v", err)
	}
	if len(pdf) != 1 || pdf[0].Attempts != 2 {
		t.Fatalf("expected retry attempt 2, got %+v", pdf)
	}
	if err := outboxRepo.CompleteDelivery(ctx, TestTenantID, eventID, "pdf-service", "pdf-2"); err != nil {
		t.Fatalf("complete PDF delivery: %v", err)
	}
	assertOutboxStatus(t, testDB, eventID, "processed")
}

func TestOutboxLeaseRecoveryRetryLimitAndTenantRLS(t *testing.T) {
	testDB := SetupTestDB(t)
	ctx := ContextWithTenant(TestTenantID)
	outboxRepo := repository.NewOutboxRepository(testDB.DB)

	tenantB := uuid.NewString()
	if _, err := testDB.Pool.Exec(context.Background(),
		`INSERT INTO tenants (id, name) VALUES ($1, 'Tenant B')`, tenantB); err != nil {
		t.Fatalf("create tenant B: %v", err)
	}
	eventID := saveOutboxEvent(t, testDB, outboxRepo, tenantB, "invoice.issued")

	first, err := outboxRepo.ClaimDeliveries(ctx, tenantB, "pdf-service", "pdf-stale", 1, time.Minute)
	if err != nil {
		t.Fatalf("claim PDF delivery: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("expected tenant B delivery, got %d", len(first))
	}
	if err := outboxRepo.RenewDeliveryLease(ctx, tenantB, eventID, "pdf-service", "pdf-stale", time.Minute); err != nil {
		t.Fatalf("renew active delivery lease: %v", err)
	}

	conn, err := testDB.DB.TenantConn(ctx, TestTenantID)
	if err != nil {
		t.Fatalf("acquire tenant A connection: %v", err)
	}
	var visible int
	err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM outbox_deliveries WHERE event_id = $1`, eventID).Scan(&visible)
	conn.Release()
	if err != nil {
		t.Fatalf("query deliveries under tenant A: %v", err)
	}
	if visible != 0 {
		t.Fatalf("tenant A can see %d tenant B deliveries", visible)
	}

	expireDeliveryLease(t, testDB, eventID, "pdf-service")
	if err := outboxRepo.RenewDeliveryLease(ctx, tenantB, eventID, "pdf-service", "pdf-stale", time.Minute); !errors.Is(err, repository.ErrOutboxLeaseLost) {
		t.Fatalf("expired worker should not renew its lease, got %v", err)
	}
	if err := outboxRepo.CompleteDelivery(ctx, tenantB, eventID, "pdf-service", "pdf-stale"); !errors.Is(err, repository.ErrOutboxLeaseLost) {
		t.Fatalf("expired worker should not acknowledge its delivery, got %v", err)
	}
	if err := outboxRepo.RetryDelivery(ctx, tenantB, eventID, "pdf-service", "pdf-stale", errors.New("late failure")); !errors.Is(err, repository.ErrOutboxLeaseLost) {
		t.Fatalf("expired worker should not retry its delivery, got %v", err)
	}
	current, err := outboxRepo.ClaimDeliveries(ctx, tenantB, "pdf-service", "pdf-current", 1, time.Minute)
	if err != nil {
		t.Fatalf("reclaim expired delivery: %v", err)
	}
	if len(current) != 1 || current[0].Attempts != 2 {
		t.Fatalf("expected reclaimed attempt 2, got %+v", current)
	}
	if err := outboxRepo.CompleteDelivery(ctx, tenantB, eventID, "pdf-service", "pdf-stale"); !errors.Is(err, repository.ErrOutboxLeaseLost) {
		t.Fatalf("stale worker should not acknowledge a reclaimed lease, got %v", err)
	}

	for attempt := 2; attempt <= 10; attempt++ {
		if err := outboxRepo.RetryDelivery(ctx, tenantB, eventID, "pdf-service", "pdf-current", fmt.Errorf("failure %d", attempt)); err != nil {
			t.Fatalf("release attempt %d: %v", attempt, err)
		}
		if attempt == 10 {
			break
		}
		makeDeliveryAvailable(t, testDB, eventID, "pdf-service")
		current, err = outboxRepo.ClaimDeliveries(ctx, tenantB, "pdf-service", "pdf-current", 1, time.Minute)
		if err != nil {
			t.Fatalf("claim attempt %d: %v", attempt+1, err)
		}
		if len(current) != 1 || current[0].Attempts != attempt+1 {
			t.Fatalf("expected attempt %d, got %+v", attempt+1, current)
		}
	}

	current, err = outboxRepo.ClaimDeliveries(ctx, tenantB, "pdf-service", "pdf-current", 1, time.Minute)
	if err != nil {
		t.Fatalf("claim terminal delivery: %v", err)
	}
	if len(current) != 0 {
		t.Fatal("terminally failed delivery was claimed again")
	}

	var status string
	var attempts int
	if err := testDB.Pool.QueryRow(context.Background(), `
		SELECT status, attempts FROM outbox_deliveries
		WHERE event_id = $1 AND consumer = 'pdf-service'
	`, eventID).Scan(&status, &attempts); err != nil {
		t.Fatalf("read terminal delivery: %v", err)
	}
	if status != "failed" || attempts != 10 {
		t.Fatalf("expected failed delivery after 10 attempts, got status=%s attempts=%d", status, attempts)
	}
}

func saveOutboxEvent(t *testing.T, testDB *TestDB, outboxRepo *repository.OutboxRepository, tenantID, eventType string) string {
	t.Helper()

	aggregateID := uuid.NewString()
	event := testOutboxEvent{
		Invoice: aggregateID,
		Tenant:  tenantID,
		Type:    eventType,
	}
	if err := outboxRepo.SaveEvents(ContextWithTenant(tenantID), tenantID, []entity.DomainEvent{event}); err != nil {
		t.Fatalf("save outbox event: %v", err)
	}

	var eventID string
	if err := testDB.Pool.QueryRow(context.Background(), `
		SELECT id FROM outbox WHERE aggregate_id = $1 AND tenant_id = $2
	`, aggregateID, tenantID).Scan(&eventID); err != nil {
		t.Fatalf("read outbox event ID: %v", err)
	}
	return eventID
}

func makeDeliveryAvailable(t *testing.T, testDB *TestDB, eventID, consumer string) {
	t.Helper()
	if _, err := testDB.Pool.Exec(context.Background(), `
		UPDATE outbox_deliveries SET available_at = NOW() - INTERVAL '1 second'
		WHERE event_id = $1 AND consumer = $2
	`, eventID, consumer); err != nil {
		t.Fatalf("make delivery available: %v", err)
	}
}

func expireDeliveryLease(t *testing.T, testDB *TestDB, eventID, consumer string) {
	t.Helper()
	if _, err := testDB.Pool.Exec(context.Background(), `
		UPDATE outbox_deliveries SET locked_until = NOW() - INTERVAL '1 second'
		WHERE event_id = $1 AND consumer = $2
	`, eventID, consumer); err != nil {
		t.Fatalf("expire delivery lease: %v", err)
	}
}

func assertOutboxStatus(t *testing.T, testDB *TestDB, eventID, expected string) {
	t.Helper()
	var status string
	if err := testDB.Pool.QueryRow(context.Background(), `
		SELECT status FROM outbox WHERE id = $1
	`, eventID).Scan(&status); err != nil {
		t.Fatalf("read outbox status: %v", err)
	}
	if status != expected {
		t.Fatalf("expected outbox status %s, got %s", expected, status)
	}
}

type testOutboxEvent struct {
	Invoice string `json:"InvoiceID"`
	Tenant  string `json:"TenantID"`
	Type    string `json:"EventType"`
}

func (e testOutboxEvent) EventName() string     { return e.Type }
func (e testOutboxEvent) AggregateID() string   { return e.Invoice }
func (e testOutboxEvent) TenantID() string      { return e.Tenant }
func (e testOutboxEvent) OccurredAt() time.Time { return time.Now() }
