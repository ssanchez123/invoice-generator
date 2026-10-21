//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/domain/valueobject"
	"github.com/santiagossaa/invoice-generator/internal/infrastructure/repository"
)

func TestPaymentRepository_SaveAndFindByInvoice(t *testing.T) {
	testDB := SetupTestDB(t)
	ctx := ContextWithTenant(TestTenantID)

	invoiceRepo := repository.NewInvoiceRepository(testDB.DB)
	paymentRepo := repository.NewPaymentRepository(testDB.DB)

	// Create and issue an invoice
	now := time.Now()
	num, _ := invoiceRepo.NextInvoiceNumber(ctx, TestTenantID)
	invoice := &entity.Invoice{
		ID:         "99999999-9999-9999-9999-999999999999",
		TenantID:   TestTenantID,
		CustomerID: TestCustomerID,
		Number:     valueobject.InvoiceNumber{Value: num},
		Status:     entity.InvoiceStatusDraft,
		DueDate:    now.Add(30 * 24 * time.Hour),
		Currency:   "USD",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := invoice.AddItem(entity.InvoiceItem{
		ID:          "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		Description: "Test",
		Quantity:    1,
		UnitPrice:   valueobject.Money{Amount: 10000, Currency: "USD"},
	}); err != nil {
		t.Fatalf("add invoice item: %v", err)
	}
	if err := invoiceRepo.Save(ctx, invoice); err != nil {
		t.Fatalf("save draft invoice: %v", err)
	}
	if err := invoice.Issue(now); err != nil {
		t.Fatalf("issue invoice: %v", err)
	}
	if err := invoiceRepo.Save(ctx, invoice); err != nil {
		t.Fatalf("save issued invoice: %v", err)
	}

	// Save a payment
	payment := &entity.Payment{
		ID:        "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		InvoiceID: invoice.ID,
		TenantID:  TestTenantID,
		Amount:    valueobject.Money{Amount: 5000, Currency: "USD"},
		Method:    "bank_transfer",
		Reference: "TXN-TEST-001",
		PaidAt:    now,
		CreatedAt: now,
	}

	if err := paymentRepo.Save(ctx, payment); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// FindByInvoice
	payments, err := paymentRepo.FindByInvoice(ctx, invoice.ID)
	if err != nil {
		t.Fatalf("FindByInvoice failed: %v", err)
	}
	if len(payments) != 1 {
		t.Fatalf("payments count = %d, want 1", len(payments))
	}
	if payments[0].Amount.Amount != 5000 {
		t.Errorf("payment amount = %d, want 5000", payments[0].Amount.Amount)
	}
	if payments[0].Method != "bank_transfer" {
		t.Errorf("payment method = %s, want bank_transfer", payments[0].Method)
	}

	// FindByID
	loaded, err := paymentRepo.FindByID(ctx, payment.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if loaded.ID != payment.ID {
		t.Errorf("ID = %s, want %s", loaded.ID, payment.ID)
	}
}
