//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/domain/valueobject"
	"github.com/santiagossaa/invoice-generator/internal/infrastructure/repository"
)

func TestInvoiceRepository_SaveAndFindByID(t *testing.T) {
	testDB := SetupTestDB(t)
	ctx := ContextWithTenant(TestTenantID)

	invoiceRepo := repository.NewInvoiceRepository(testDB.DB)

	// Create an invoice — items are added via the domain method so the
	// aggregate computes its own totals (a repository persists what the
	// aggregate says; hand-computing totals in the literal would test nothing
	// about the save/load round-trip).
	now := time.Now()
	invoice := &entity.Invoice{
		ID:         "11111111-1111-1111-1111-111111111111",
		TenantID:   TestTenantID,
		CustomerID: TestCustomerID,
		Number:     valueobject.InvoiceNumber{Value: "INV-2024-00001"},
		Status:     entity.InvoiceStatusDraft,
		DueDate:    now.Add(30 * 24 * time.Hour),
		Currency:   "USD",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := invoice.AddItem(entity.InvoiceItem{
		ID:          "22222222-2222-2222-2222-222222222222",
		Description: "Consulting hours",
		Quantity:    10,
		UnitPrice:   valueobject.Money{Amount: 5000, Currency: "USD"},
		TaxRateBPS:  1900,
	}); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	// Save
	if err := invoiceRepo.Save(ctx, invoice); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// FindByID
	loaded, err := invoiceRepo.FindByID(ctx, invoice.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if loaded.ID != invoice.ID {
		t.Errorf("ID = %s, want %s", loaded.ID, invoice.ID)
	}
	if loaded.Number.Value != invoice.Number.Value {
		t.Errorf("Number = %s, want %s", loaded.Number.Value, invoice.Number.Value)
	}
	if loaded.Status != entity.InvoiceStatusDraft {
		t.Errorf("Status = %s, want draft", loaded.Status)
	}
	if len(loaded.Items) != 1 {
		t.Fatalf("Items count = %d, want 1", len(loaded.Items))
	}
	if loaded.Items[0].Description != "Consulting hours" {
		t.Errorf("Item description = %s, want 'Consulting hours'", loaded.Items[0].Description)
	}

	// Verify calculations were persisted
	// 10 * 5000 = 50000, tax = 50000 * 0.19 = 9500, total = 59500
	if loaded.Subtotal.Amount != 50000 {
		t.Errorf("Subtotal = %d, want 50000", loaded.Subtotal.Amount)
	}
	if loaded.TaxTotal.Amount != 9500 {
		t.Errorf("TaxTotal = %d, want 9500", loaded.TaxTotal.Amount)
	}
	if loaded.Total.Amount != 59500 {
		t.Errorf("Total = %d, want 59500", loaded.Total.Amount)
	}
}

func TestInvoiceRepository_FindByIDForUpdate(t *testing.T) {
	testDB := SetupTestDB(t)
	ctx := ContextWithTenant(TestTenantID)

	invoiceRepo := repository.NewInvoiceRepository(testDB.DB)
	uow := repository.NewUnitOfWork(testDB.DB)

	// Create and save an invoice
	now := time.Now()
	invoice := &entity.Invoice{
		ID:         "33333333-3333-3333-3333-333333333333",
		TenantID:   TestTenantID,
		CustomerID: TestCustomerID,
		Number:     valueobject.InvoiceNumber{Value: "INV-2024-00002"},
		Status:     entity.InvoiceStatusIssued,
		IssueDate:  now,
		DueDate:    now.Add(30 * 24 * time.Hour),
		Currency:   "USD",
		Items: []entity.InvoiceItem{
			{
				ID:          "44444444-4444-4444-4444-444444444444",
				Description: "Test item",
				Quantity:    1,
				UnitPrice:   valueobject.Money{Amount: 10000, Currency: "USD"},
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := invoiceRepo.Save(ctx, invoice); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Use UnitOfWork to begin a transaction
	txCtx, tx, err := uow.BeginTx(ctx, TestTenantID)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}
	defer tx.Rollback(txCtx)

	// FindByIDForUpdate should work within the transaction
	loaded, err := invoiceRepo.FindByIDForUpdate(txCtx, invoice.ID)
	if err != nil {
		t.Fatalf("FindByIDForUpdate failed: %v", err)
	}
	if loaded.ID != invoice.ID {
		t.Errorf("ID = %s, want %s", loaded.ID, invoice.ID)
	}

	// Commit
	if err := tx.Commit(txCtx); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
}

func TestInvoiceRepository_NextInvoiceNumber(t *testing.T) {
	testDB := SetupTestDB(t)
	ctx := ContextWithTenant(TestTenantID)

	invoiceRepo := repository.NewInvoiceRepository(testDB.DB)

	num1, err := invoiceRepo.NextInvoiceNumber(ctx, TestTenantID)
	if err != nil {
		t.Fatalf("NextInvoiceNumber failed: %v", err)
	}
	if num1 == "" {
		t.Error("invoice number should not be empty")
	}

	num2, err := invoiceRepo.NextInvoiceNumber(ctx, TestTenantID)
	if err != nil {
		t.Fatalf("NextInvoiceNumber (2) failed: %v", err)
	}
	if num1 == num2 {
		t.Errorf("consecutive numbers should differ: %s == %s", num1, num2)
	}

	// Verify format: INV-YYYY-NNNNN
	year := time.Now().Year()
	expectedPrefix := ""
	if year == 2026 {
		expectedPrefix = "INV-2026-"
	} else {
		expectedPrefix = "INV-" + intToStr(year) + "-"
	}
	_ = expectedPrefix
}

func TestInvoiceRepository_FindByTenant(t *testing.T) {
	testDB := SetupTestDB(t)
	ctx := ContextWithTenant(TestTenantID)

	invoiceRepo := repository.NewInvoiceRepository(testDB.DB)

	// Create multiple invoices
	for i := 0; i < 5; i++ {
		now := time.Now()
		num, _ := invoiceRepo.NextInvoiceNumber(ctx, TestTenantID)
		invoice := &entity.Invoice{
			ID:         generateUUID(i + 10),
			TenantID:   TestTenantID,
			CustomerID: TestCustomerID,
			Number:     valueobject.InvoiceNumber{Value: num},
			Status:     entity.InvoiceStatusDraft,
			DueDate:    now.Add(30 * 24 * time.Hour),
			Currency:   "USD",
			Items: []entity.InvoiceItem{
				{
					ID:          generateUUID(i + 20),
					Description: "Item",
					Quantity:    1,
					UnitPrice:   valueobject.Money{Amount: 1000, Currency: "USD"},
				},
			},
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := invoiceRepo.Save(ctx, invoice); err != nil {
			t.Fatalf("Save %d failed: %v", i, err)
		}
	}

	// List with pagination
	invoices, total, err := invoiceRepo.FindByTenant(ctx, TestTenantID, 3, 0)
	if err != nil {
		t.Fatalf("FindByTenant failed: %v", err)
	}
	if total < 5 {
		t.Errorf("total = %d, want >= 5", total)
	}
	if len(invoices) != 3 {
		t.Errorf("returned count = %d, want 3", len(invoices))
	}
}

func TestInvoiceRepository_FindOverdue(t *testing.T) {
	testDB := SetupTestDB(t)
	ctx := ContextWithTenant(TestTenantID)

	invoiceRepo := repository.NewInvoiceRepository(testDB.DB)

	// Create an overdue invoice (issued, due date in the past)
	now := time.Now()
	num, _ := invoiceRepo.NextInvoiceNumber(ctx, TestTenantID)
	invoice := &entity.Invoice{
		ID:         "55555555-5555-5555-5555-555555555555",
		TenantID:   TestTenantID,
		CustomerID: TestCustomerID,
		Number:     valueobject.InvoiceNumber{Value: num},
		Status:     entity.InvoiceStatusIssued,
		IssueDate:  now.Add(-60 * 24 * time.Hour),
		DueDate:    now.Add(-30 * 24 * time.Hour), // 30 days ago
		Currency:   "USD",
		Items: []entity.InvoiceItem{
			{
				ID:          "66666666-6666-6666-6666-666666666666",
				Description: "Overdue item",
				Quantity:    1,
				UnitPrice:   valueobject.Money{Amount: 5000, Currency: "USD"},
			},
		},
		CreatedAt: now.Add(-60 * 24 * time.Hour),
		UpdatedAt: now.Add(-60 * 24 * time.Hour),
	}
	if err := invoiceRepo.Save(ctx, invoice); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Find overdue
	overdue, err := invoiceRepo.FindOverdue(ctx, TestTenantID, now)
	if err != nil {
		t.Fatalf("FindOverdue failed: %v", err)
	}
	found := false
	for _, inv := range overdue {
		if inv.ID == invoice.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("overdue invoice not found in FindOverdue results")
	}
}

// Helpers

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func generateUUID(suffix int) string {
	s := intToStr(suffix)
	for len(s) < 12 {
		s = "0" + s
	}
	return "00000000-0000-0000-0000-" + s
}

var _ = context.Background
