package command

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/domain/valueobject"
)

func TestBuildSubscriptionDraftRequiresManualIssuance(t *testing.T) {
	createdAt := time.Date(2026, time.October, 25, 12, 0, 0, 0, time.UTC)
	subscription := &entity.Subscription{
		ID:         uuid.NewString(),
		TenantID:   uuid.NewString(),
		CustomerID: uuid.NewString(),
		PlanItems: []entity.InvoiceItem{{
			ID:          uuid.NewString(),
			Description: "Servicio mensual",
			Quantity:    2,
			UnitPrice:   valueobject.Money{Amount: 10000, Currency: "COP"},
			TaxRateBPS:  1900,
		}},
		NextBillingDate: createdAt.AddDate(0, 0, -3),
	}
	invoice, err := buildSubscriptionDraft(subscription, subscription.CustomerID, createdAt)
	if err != nil {
		t.Fatalf("buildSubscriptionDraft() error = %v", err)
	}
	if invoice.Status != entity.InvoiceStatusDraft || !invoice.IssueDate.IsZero() {
		t.Fatalf("generated invoice must remain draft, got status %s and issue date %s", invoice.Status, invoice.IssueDate)
	}
	if !invoice.DueDate.Equal(createdAt.AddDate(0, 0, 30)) {
		t.Fatalf("due date = %s, want 30 days from draft creation", invoice.DueDate)
	}
	if invoice.Metadata["subscription_id"] != subscription.ID {
		t.Fatalf("subscription metadata = %v, want %s", invoice.Metadata["subscription_id"], subscription.ID)
	}
	if len(invoice.Items) != 1 || invoice.Items[0].ID == subscription.PlanItems[0].ID {
		t.Fatalf("generated draft must clone plan items with new IDs: %+v", invoice.Items)
	}
}
