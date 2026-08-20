package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/santiagossaa/invoice-generator/internal/application/command"
	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/infrastructure/repository"
)

func TestCreateInvoice(t *testing.T) {
	tenantID := "tenant-001"

	// Set up in-memory repos
	invoiceRepo := repository.NewInMemoryInvoiceRepository()
	customerRepo := repository.NewInMemoryCustomerRepository()

	// Create a customer first
	customer := repository.NewTestCustomer(tenantID)
	ctx := repository.WithTenantID(context.Background(), tenantID)
	if err := customerRepo.Save(ctx, customer); err != nil {
		t.Fatalf("failed to save customer: %v", err)
	}

	// Create invoice
	handler := command.NewCreateInvoiceHandler(invoiceRepo, customerRepo)
	cmd := command.CreateInvoiceCommand{
		TenantID:   tenantID,
		CustomerID: customer.ID,
		Currency:   "USD",
		DueDate:    time.Now().Add(30 * 24 * time.Hour),
		Items: []command.ItemInput{
			{Description: "Consulting", Quantity: 10, UnitPrice: 5000, TaxRateBPS: 1900},
			{Description: "Support", Quantity: 5, UnitPrice: 2000, TaxRateBPS: 0},
		},
		Notes: "Test invoice",
	}

	invoice, err := handler.Handle(ctx, cmd)
	if err != nil {
		t.Fatalf("CreateInvoice failed: %v", err)
	}

	// Verify invoice was created
	if invoice.ID == "" {
		t.Error("invoice ID should not be empty")
	}
	if invoice.TenantID != tenantID {
		t.Errorf("TenantID = %s, want %s", invoice.TenantID, tenantID)
	}
	if invoice.CustomerID != customer.ID {
		t.Errorf("CustomerID = %s, want %s", invoice.CustomerID, customer.ID)
	}
	if invoice.Status != entity.InvoiceStatusDraft {
		t.Errorf("Status = %s, want draft", invoice.Status)
	}
	if len(invoice.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(invoice.Items))
	}

	// Verify calculations
	// Item 1: 10 * 5000 = 50000, tax = 50000 * 0.19 = 9500, total = 59500
	// Item 2: 5 * 2000 = 10000, tax = 0, total = 10000
	// Subtotal = 50000 + 10000 = 60000
	// TaxTotal = 9500
	// Total = 69500
	if invoice.Subtotal.Amount != 60000 {
		t.Errorf("Subtotal = %d, want 60000", invoice.Subtotal.Amount)
	}
	if invoice.TaxTotal.Amount != 9500 {
		t.Errorf("TaxTotal = %d, want 9500", invoice.TaxTotal.Amount)
	}
	if invoice.Total.Amount != 69500 {
		t.Errorf("Total = %d, want 69500", invoice.Total.Amount)
	}

	// Verify invoice number was generated
	if invoice.Number.Value == "" {
		t.Error("invoice number should not be empty")
	}

	// Verify it was persisted
	loaded, err := invoiceRepo.FindByID(ctx, invoice.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if loaded.ID != invoice.ID {
		t.Errorf("loaded ID = %s, want %s", loaded.ID, invoice.ID)
	}
}

func TestCreateInvoiceInvalidCustomer(t *testing.T) {
	tenantID := "tenant-001"

	invoiceRepo := repository.NewInMemoryInvoiceRepository()
	customerRepo := repository.NewInMemoryCustomerRepository()

	handler := command.NewCreateInvoiceHandler(invoiceRepo, customerRepo)
	cmd := command.CreateInvoiceCommand{
		TenantID:   tenantID,
		CustomerID: "nonexistent-customer",
		Currency:   "USD",
		DueDate:    time.Now().Add(30 * 24 * time.Hour),
		Items: []command.ItemInput{
			{Description: "Test", Quantity: 1, UnitPrice: 1000, TaxRateBPS: 0},
		},
	}

	ctx := repository.WithTenantID(context.Background(), tenantID)
	_, err := handler.Handle(ctx, cmd)
	if err == nil {
		t.Error("CreateInvoice with nonexistent customer should fail")
	}
}

func TestCreateInvoiceWrongTenant(t *testing.T) {
	tenantID := "tenant-001"
	otherTenantID := "tenant-002"

	invoiceRepo := repository.NewInMemoryInvoiceRepository()
	customerRepo := repository.NewInMemoryCustomerRepository()

	// Customer in tenant-001
	customer := repository.NewTestCustomer(tenantID)
	ctx := repository.WithTenantID(context.Background(), tenantID)
	customerRepo.Save(ctx, customer)

	// Try to create invoice in tenant-002 with customer from tenant-001
	handler := command.NewCreateInvoiceHandler(invoiceRepo, customerRepo)
	cmd := command.CreateInvoiceCommand{
		TenantID:   otherTenantID,
		CustomerID: customer.ID,
		Currency:   "USD",
		DueDate:    time.Now().Add(30 * 24 * time.Hour),
		Items: []command.ItemInput{
			{Description: "Test", Quantity: 1, UnitPrice: 1000, TaxRateBPS: 0},
		},
	}

	_, err := handler.Handle(ctx, cmd)
	if err == nil {
		t.Error("CreateInvoice with customer from different tenant should fail")
	}
}

func TestIssueInvoice(t *testing.T) {
	tenantID := "tenant-001"

	invoiceRepo := repository.NewInMemoryInvoiceRepository()
	customerRepo := repository.NewInMemoryCustomerRepository()

	customer := repository.NewTestCustomer(tenantID)
	ctx := repository.WithTenantID(context.Background(), tenantID)
	customerRepo.Save(ctx, customer)

	// Create invoice
	createH := command.NewCreateInvoiceHandler(invoiceRepo, customerRepo)
	invoice, err := createH.Handle(ctx, command.CreateInvoiceCommand{
		TenantID:   tenantID,
		CustomerID: customer.ID,
		Currency:   "USD",
		DueDate:    time.Now().Add(30 * 24 * time.Hour),
		Items: []command.ItemInput{
			{Description: "Test", Quantity: 1, UnitPrice: 1000, TaxRateBPS: 0},
		},
	})
	if err != nil {
		t.Fatalf("CreateInvoice failed: %v", err)
	}

	// Issue it
	issueH := command.NewIssueInvoiceHandler(invoiceRepo)
	issued, err := issueH.Handle(ctx, command.IssueInvoiceCommand{
		InvoiceID: invoice.ID,
		TenantID:  tenantID,
	})
	if err != nil {
		t.Fatalf("IssueInvoice failed: %v", err)
	}
	if issued.Status != entity.InvoiceStatusIssued {
		t.Errorf("Status = %s, want issued", issued.Status)
	}
	if issued.IssueDate.IsZero() {
		t.Error("IssueDate should be set")
	}
}

func TestCancelInvoice(t *testing.T) {
	tenantID := "tenant-001"

	invoiceRepo := repository.NewInMemoryInvoiceRepository()
	customerRepo := repository.NewInMemoryCustomerRepository()

	customer := repository.NewTestCustomer(tenantID)
	ctx := repository.WithTenantID(context.Background(), tenantID)
	customerRepo.Save(ctx, customer)

	createH := command.NewCreateInvoiceHandler(invoiceRepo, customerRepo)
	invoice, _ := createH.Handle(ctx, command.CreateInvoiceCommand{
		TenantID:   tenantID,
		CustomerID: customer.ID,
		Currency:   "USD",
		DueDate:    time.Now().Add(30 * 24 * time.Hour),
		Items: []command.ItemInput{
			{Description: "Test", Quantity: 1, UnitPrice: 1000, TaxRateBPS: 0},
		},
	})

	cancelH := command.NewCancelInvoiceHandler(invoiceRepo)
	err := cancelH.Handle(ctx, command.CancelInvoiceCommand{
		InvoiceID: invoice.ID,
		TenantID:  tenantID,
	})
	if err != nil {
		t.Fatalf("CancelInvoice failed: %v", err)
	}

	loaded, _ := invoiceRepo.FindByID(ctx, invoice.ID)
	if loaded.Status != entity.InvoiceStatusCancelled {
		t.Errorf("Status = %s, want cancelled", loaded.Status)
	}
}

func TestRecordPayment(t *testing.T) {
	tenantID := "tenant-001"

	invoiceRepo := repository.NewInMemoryInvoiceRepository()
	customerRepo := repository.NewInMemoryCustomerRepository()
	paymentRepo := repository.NewInMemoryPaymentRepository()

	customer := repository.NewTestCustomer(tenantID)
	ctx := repository.WithTenantID(context.Background(), tenantID)
	customerRepo.Save(ctx, customer)

	// Create and issue invoice
	createH := command.NewCreateInvoiceHandler(invoiceRepo, customerRepo)
	invoice, _ := createH.Handle(ctx, command.CreateInvoiceCommand{
		TenantID:   tenantID,
		CustomerID: customer.ID,
		Currency:   "USD",
		DueDate:    time.Now().Add(30 * 24 * time.Hour),
		Items: []command.ItemInput{
			{Description: "Test", Quantity: 1, UnitPrice: 10000, TaxRateBPS: 0}, // $100.00
		},
	})

	issueH := command.NewIssueInvoiceHandler(invoiceRepo)
	issueH.Handle(ctx, command.IssueInvoiceCommand{
		InvoiceID: invoice.ID,
		TenantID:  tenantID,
	})

	// Record full payment
	paymentH := command.NewRecordPaymentHandler(invoiceRepo, paymentRepo)
	payment, err := paymentH.Handle(ctx, command.RecordPaymentCommand{
		InvoiceID: invoice.ID,
		TenantID:  tenantID,
		Amount:    10000, // full payment
		Currency:  "USD",
		Method:    "bank_transfer",
		Reference: "TXN-001",
		PaidAt:    time.Now(),
	})
	if err != nil {
		t.Fatalf("RecordPayment failed: %v", err)
	}
	if payment.Amount.Amount != 10000 {
		t.Errorf("payment amount = %d, want 10000", payment.Amount.Amount)
	}

	// Invoice should be marked as paid
	loaded, _ := invoiceRepo.FindByID(ctx, invoice.ID)
	if loaded.Status != entity.InvoiceStatusPaid {
		t.Errorf("Status = %s, want paid", loaded.Status)
	}
}

func TestRecordPaymentExceedsTotal(t *testing.T) {
	tenantID := "tenant-001"

	invoiceRepo := repository.NewInMemoryInvoiceRepository()
	customerRepo := repository.NewInMemoryCustomerRepository()
	paymentRepo := repository.NewInMemoryPaymentRepository()

	customer := repository.NewTestCustomer(tenantID)
	ctx := repository.WithTenantID(context.Background(), tenantID)
	customerRepo.Save(ctx, customer)

	createH := command.NewCreateInvoiceHandler(invoiceRepo, customerRepo)
	invoice, _ := createH.Handle(ctx, command.CreateInvoiceCommand{
		TenantID:   tenantID,
		CustomerID: customer.ID,
		Currency:   "USD",
		DueDate:    time.Now().Add(30 * 24 * time.Hour),
		Items: []command.ItemInput{
			{Description: "Test", Quantity: 1, UnitPrice: 5000, TaxRateBPS: 0}, // $50.00
		},
	})

	issueH := command.NewIssueInvoiceHandler(invoiceRepo)
	issueH.Handle(ctx, command.IssueInvoiceCommand{
		InvoiceID: invoice.ID,
		TenantID:  tenantID,
	})

	paymentH := command.NewRecordPaymentHandler(invoiceRepo, paymentRepo)
	_, err := paymentH.Handle(ctx, command.RecordPaymentCommand{
		InvoiceID: invoice.ID,
		TenantID:  tenantID,
		Amount:    60000, // $600 — exceeds $50 total
		Currency:  "USD",
		Method:    "bank_transfer",
		PaidAt:    time.Now(),
	})
	if err != entity.ErrInsufficientPayment {
		t.Errorf("expected ErrInsufficientPayment, got %v", err)
	}
}