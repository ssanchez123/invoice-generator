//go:build integration

package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/santiagossaa/invoice-generator/internal/application/command"
	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/domain/valueobject"
	"github.com/santiagossaa/invoice-generator/internal/infrastructure/repository"
)

func TestRecordPayment_Integration_FullPayment(t *testing.T) {
	testDB := SetupTestDB(t)
	ctx := ContextWithTenant(TestTenantID)

	invoiceRepo := repository.NewInvoiceRepository(testDB.DB)
	customerRepo := repository.NewCustomerRepository(testDB.DB)
	paymentRepo := repository.NewPaymentRepository(testDB.DB)
	uow := repository.NewUnitOfWork(testDB.DB)

	// Create and issue invoice: $100.00
	createH := command.NewCreateInvoiceHandler(invoiceRepo, customerRepo)
	invoice, err := createH.Handle(ctx, command.CreateInvoiceCommand{
		TenantID:   TestTenantID,
		CustomerID: TestCustomerID,
		Currency:   "USD",
		DueDate:    time.Now().Add(30 * 24 * time.Hour),
		Items: []command.ItemInput{
			{Description: "Integration test", Quantity: 1, UnitPrice: 10000, TaxRateBPS: 0},
		},
	})
	if err != nil {
		t.Fatalf("CreateInvoice failed: %v", err)
	}

	issueH := command.NewIssueInvoiceHandler(invoiceRepo)
	if _, err := issueH.Handle(ctx, command.IssueInvoiceCommand{
		InvoiceID: invoice.ID,
		TenantID:  TestTenantID,
	}); err != nil {
		t.Fatalf("IssueInvoice failed: %v", err)
	}

	// Record full payment
	paymentH := command.NewRecordPaymentHandler(invoiceRepo, paymentRepo, uow)
	payment, err := paymentH.Handle(ctx, command.RecordPaymentCommand{
		InvoiceID: invoice.ID,
		TenantID:  TestTenantID,
		Amount:    10000,
		Currency:  "USD",
		Method:    "bank_transfer",
		Reference: "TXN-INTEG-001",
		PaidAt:    time.Now(),
	})
	if err != nil {
		t.Fatalf("RecordPayment failed: %v", err)
	}
	if payment.Amount.Amount != 10000 {
		t.Errorf("payment amount = %d, want 10000", payment.Amount.Amount)
	}

	// Invoice should be paid
	loaded, _ := invoiceRepo.FindByID(ctx, invoice.ID)
	if loaded.Status != entity.InvoiceStatusPaid {
		t.Errorf("status = %s, want paid", loaded.Status)
	}
}

func TestRecordPayment_Integration_PartialThenFull(t *testing.T) {
	testDB := SetupTestDB(t)
	ctx := ContextWithTenant(TestTenantID)

	invoiceRepo := repository.NewInvoiceRepository(testDB.DB)
	customerRepo := repository.NewCustomerRepository(testDB.DB)
	paymentRepo := repository.NewPaymentRepository(testDB.DB)
	uow := repository.NewUnitOfWork(testDB.DB)

	createH := command.NewCreateInvoiceHandler(invoiceRepo, customerRepo)
	invoice, _ := createH.Handle(ctx, command.CreateInvoiceCommand{
		TenantID:   TestTenantID,
		CustomerID: TestCustomerID,
		Currency:   "USD",
		DueDate:    time.Now().Add(30 * 24 * time.Hour),
		Items:      []command.ItemInput{{Description: "Test", Quantity: 1, UnitPrice: 10000, TaxRateBPS: 0}},
	})

	issueH := command.NewIssueInvoiceHandler(invoiceRepo)
	issueH.Handle(ctx, command.IssueInvoiceCommand{InvoiceID: invoice.ID, TenantID: TestTenantID})

	paymentH := command.NewRecordPaymentHandler(invoiceRepo, paymentRepo, uow)

	// Partial: $60
	_, err := paymentH.Handle(ctx, command.RecordPaymentCommand{
		InvoiceID: invoice.ID, TenantID: TestTenantID,
		Amount: 6000, Currency: "USD", Method: "cash", PaidAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("partial payment failed: %v", err)
	}

	loaded, _ := invoiceRepo.FindByID(ctx, invoice.ID)
	if loaded.Status != entity.InvoiceStatusIssued {
		t.Errorf("after partial, status = %s, want issued", loaded.Status)
	}

	// Remaining: $40
	_, err = paymentH.Handle(ctx, command.RecordPaymentCommand{
		InvoiceID: invoice.ID, TenantID: TestTenantID,
		Amount: 4000, Currency: "USD", Method: "cash", PaidAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("remaining payment failed: %v", err)
	}

	loaded, _ = invoiceRepo.FindByID(ctx, invoice.ID)
	if loaded.Status != entity.InvoiceStatusPaid {
		t.Errorf("after full, status = %s, want paid", loaded.Status)
	}
}

func TestRecordPayment_Integration_OverpaymentRejected(t *testing.T) {
	testDB := SetupTestDB(t)
	ctx := ContextWithTenant(TestTenantID)

	invoiceRepo := repository.NewInvoiceRepository(testDB.DB)
	customerRepo := repository.NewCustomerRepository(testDB.DB)
	paymentRepo := repository.NewPaymentRepository(testDB.DB)
	uow := repository.NewUnitOfWork(testDB.DB)

	createH := command.NewCreateInvoiceHandler(invoiceRepo, customerRepo)
	invoice, _ := createH.Handle(ctx, command.CreateInvoiceCommand{
		TenantID:   TestTenantID,
		CustomerID: TestCustomerID,
		Currency:   "USD",
		DueDate:    time.Now().Add(30 * 24 * time.Hour),
		Items:      []command.ItemInput{{Description: "Test", Quantity: 1, UnitPrice: 10000, TaxRateBPS: 0}},
	})

	issueH := command.NewIssueInvoiceHandler(invoiceRepo)
	issueH.Handle(ctx, command.IssueInvoiceCommand{InvoiceID: invoice.ID, TenantID: TestTenantID})

	paymentH := command.NewRecordPaymentHandler(invoiceRepo, paymentRepo, uow)

	// Pay $60
	paymentH.Handle(ctx, command.RecordPaymentCommand{
		InvoiceID: invoice.ID, TenantID: TestTenantID,
		Amount: 6000, Currency: "USD", Method: "cash", PaidAt: time.Now(),
	})

	// Try $50 more (60+50=110 > 100)
	_, err := paymentH.Handle(ctx, command.RecordPaymentCommand{
		InvoiceID: invoice.ID, TenantID: TestTenantID,
		Amount: 5000, Currency: "USD", Method: "cash", PaidAt: time.Now(),
	})
	if err == nil {
		t.Error("overpayment should be rejected")
	}
	if err != entity.ErrInsufficientPayment {
		t.Errorf("expected ErrInsufficientPayment, got %v", err)
	}
}

func TestRecordPayment_Integration_ConcurrentPayments(t *testing.T) {
	testDB := SetupTestDB(t)
	ctx := ContextWithTenant(TestTenantID)

	invoiceRepo := repository.NewInvoiceRepository(testDB.DB)
	customerRepo := repository.NewCustomerRepository(testDB.DB)
	paymentRepo := repository.NewPaymentRepository(testDB.DB)
	uow := repository.NewUnitOfWork(testDB.DB)

	createH := command.NewCreateInvoiceHandler(invoiceRepo, customerRepo)
	invoice, _ := createH.Handle(ctx, command.CreateInvoiceCommand{
		TenantID:   TestTenantID,
		CustomerID: TestCustomerID,
		Currency:   "USD",
		DueDate:    time.Now().Add(30 * 24 * time.Hour),
		Items:      []command.ItemInput{{Description: "Concurrent test", Quantity: 1, UnitPrice: 10000, TaxRateBPS: 0}},
	})

	issueH := command.NewIssueInvoiceHandler(invoiceRepo)
	issueH.Handle(ctx, command.IssueInvoiceCommand{InvoiceID: invoice.ID, TenantID: TestTenantID})

	paymentH := command.NewRecordPaymentHandler(invoiceRepo, paymentRepo, uow)

	// Launch 2 concurrent payments of $60 each on a $100 invoice.
	// With SELECT FOR UPDATE, one will succeed, the other will be rejected.
	var wg sync.WaitGroup
	var err1, err2 error
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, err1 = paymentH.Handle(ctx, command.RecordPaymentCommand{
			InvoiceID: invoice.ID, TenantID: TestTenantID,
			Amount: 6000, Currency: "USD", Method: "bank_transfer",
			Reference: "TXN-CONCURRENT-A", PaidAt: time.Now(),
		})
	}()

	go func() {
		defer wg.Done()
		_, err2 = paymentH.Handle(ctx, command.RecordPaymentCommand{
			InvoiceID: invoice.ID, TenantID: TestTenantID,
			Amount: 6000, Currency: "USD", Method: "cash",
			Reference: "TXN-CONCURRENT-B", PaidAt: time.Now(),
		})
	}()

	wg.Wait()

	successCount := 0
	if err1 == nil {
		successCount++
	}
	if err2 == nil {
		successCount++
	}

	if successCount != 1 {
		t.Errorf("expected exactly 1 successful payment, got %d (err1=%v, err2=%v)", successCount, err1, err2)
	}

	// Verify total paid doesn't exceed invoice total
	payments, _ := paymentRepo.FindByInvoice(ctx, invoice.ID)
	totalPaid := int64(0)
	for _, p := range payments {
		totalPaid += p.Amount.Amount
	}
	if totalPaid > 10000 {
		t.Errorf("total paid %d exceeds invoice total 10000 — race condition!", totalPaid)
	}
	if totalPaid != 6000 {
		t.Errorf("total paid = %d, want 6000", totalPaid)
	}
}

func TestRLS_CrossTenantInvoiceAccessBlocked(t *testing.T) {
	testDB := SetupTestDB(t)
	ctx := context.Background()

	// Create invoice in tenant A
	ctxA := ContextWithTenant(TestTenantID)
	invoiceRepo := repository.NewInvoiceRepository(testDB.DB)

	num, _ := invoiceRepo.NextInvoiceNumber(ctxA, TestTenantID)
	now := time.Now()
	invoice := &entity.Invoice{
		ID:         "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee",
		TenantID:   TestTenantID,
		CustomerID: TestCustomerID,
		Number:     valueobject.InvoiceNumber{Value: num},
		Status:     entity.InvoiceStatusDraft,
		DueDate:    now.Add(30 * 24 * time.Hour),
		Currency:   "USD",
		Items: []entity.InvoiceItem{
			{ID: "ffffffff-ffff-ffff-ffff-ffffffffffff", Description: "Test", Quantity: 1, UnitPrice: valueobject.Money{Amount: 1000, Currency: "USD"}},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := invoiceRepo.Save(ctxA, invoice); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Create tenant B
	tenantB := "22222222-2222-2222-2222-222222222222"
	_, err := testDB.Pool.Exec(ctx, `
		INSERT INTO tenants (id, name, address, settings, created_at, updated_at)
		VALUES ($1, 'Tenant B', '{}', '{}', NOW(), NOW())
		ON CONFLICT DO NOTHING
	`, tenantB)
	if err != nil {
		t.Fatalf("failed to create tenant B: %v", err)
	}

	// Try to read invoice from tenant A using tenant B's context
	ctxB := ContextWithTenant(tenantB)
	_, err = invoiceRepo.FindByID(ctxB, invoice.ID)
	if err == nil {
		t.Error("RLS should block cross-tenant invoice access")
	}
}
