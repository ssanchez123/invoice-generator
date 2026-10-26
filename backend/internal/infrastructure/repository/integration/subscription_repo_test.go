//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/santiagossaa/invoice-generator/internal/application/command"
	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/domain/port"
	"github.com/santiagossaa/invoice-generator/internal/domain/valueobject"
	"github.com/santiagossaa/invoice-generator/internal/infrastructure/repository"
)

func TestSubscriptionBillingCreatesDraftAndAdvancesScheduleAtomically(t *testing.T) {
	testDB := SetupTestDB(t)
	ctx := ContextWithTenant(TestTenantID)
	now := time.Now().UTC().Truncate(time.Second)
	dueDate := now.AddDate(0, 0, -2)
	subscription, err := entity.NewSubscription(
		uuid.NewString(), TestTenantID, TestCustomerID,
		[]entity.InvoiceItem{{
			ID:          uuid.NewString(),
			Description: "Servicio mensual",
			Quantity:    1,
			UnitPrice:   valueobject.Money{Amount: 10000, Currency: "COP"},
			TaxRateBPS:  1900,
		}},
		entity.FrequencyMonthly, dueDate,
	)
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	subscriptionRepo := repository.NewSubscriptionRepository(testDB.DB)
	if err := subscriptionRepo.Save(ctx, subscription); err != nil {
		t.Fatalf("save subscription: %v", err)
	}

	invoiceRepo := repository.NewInvoiceRepository(testDB.DB)
	customerRepo := repository.NewCustomerRepository(testDB.DB)
	billingHandler := command.NewProcessDueSubscriptionsHandler(
		subscriptionRepo, customerRepo, invoiceRepo, repository.NewUnitOfWork(testDB.DB),
	)
	processed, err := billingHandler.Handle(ctx, TestTenantID, now, 10)
	if err != nil {
		t.Fatalf("process due subscription: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}

	invoices, total, err := invoiceRepo.FindByTenant(ctx, TestTenantID, 10, 0)
	if err != nil {
		t.Fatalf("list generated invoices: %v", err)
	}
	if total != 1 || len(invoices) != 1 {
		t.Fatalf("generated invoices = %d (total %d), want 1", len(invoices), total)
	}
	if invoices[0].Status != entity.InvoiceStatusDraft || invoices[0].DueDate.Before(now) {
		t.Fatalf("generated invoice must be a future-due draft; status=%s due=%s", invoices[0].Status, invoices[0].DueDate)
	}
	if invoices[0].Metadata["subscription_id"] != subscription.ID {
		t.Fatalf("generated invoice subscription_id = %v, want %s", invoices[0].Metadata["subscription_id"], subscription.ID)
	}

	loaded, err := subscriptionRepo.FindByID(ctx, subscription.ID)
	if err != nil {
		t.Fatalf("load subscription after billing: %v", err)
	}
	if !loaded.NextBillingDate.After(dueDate) {
		t.Fatalf("next billing date = %s, want after %s", loaded.NextBillingDate, dueDate)
	}

	auditRepo := repository.NewAuditRepository(testDB.DB)
	entries, _, err := auditRepo.List(ctx, port.AuditFilter{
		TenantID: TestTenantID, EntityType: "subscription", EntityID: subscription.ID, Limit: 10,
	})
	if err != nil {
		t.Fatalf("read subscription audit: %v", err)
	}
	if len(entries) < 2 {
		t.Fatalf("subscription audit entries = %d, want create and advance events", len(entries))
	}
}
