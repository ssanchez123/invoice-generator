package command_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/santiagossaa/invoice-generator/internal/application/command"
	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/infrastructure/repository"
)

func TestUpdateInvoiceDraftPersistsChangedCustomerCurrencyAndLines(t *testing.T) {
	const tenantID = "tenant-update"
	invoiceRepo := repository.NewInMemoryInvoiceRepository()
	customerRepo := repository.NewInMemoryCustomerRepository()
	firstCustomer := repository.NewTestCustomer(tenantID)
	secondCustomer := repository.NewTestCustomer(tenantID)
	ctx := repository.WithTenantID(context.Background(), tenantID)
	if err := customerRepo.Save(ctx, firstCustomer); err != nil {
		t.Fatalf("save first customer: %v", err)
	}
	if err := customerRepo.Save(ctx, secondCustomer); err != nil {
		t.Fatalf("save second customer: %v", err)
	}
	created, err := command.NewCreateInvoiceHandler(invoiceRepo, customerRepo).Handle(ctx, command.CreateInvoiceCommand{
		TenantID: tenantID, CustomerID: firstCustomer.ID, Currency: "COP",
		DueDate: time.Now().AddDate(0, 0, 30),
		Items:   []command.ItemInput{{Description: "Servicio", Quantity: 1, UnitPrice: 10000, TaxRateBPS: 1900}},
	})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}

	updated, err := command.NewUpdateInvoiceHandler(
		invoiceRepo, customerRepo, repository.NewInMemoryUnitOfWork(),
	).Handle(ctx, command.UpdateInvoiceCommand{
		InvoiceID: created.ID, TenantID: tenantID, CustomerID: secondCustomer.ID,
		Currency: "USD", DueDate: time.Now().AddDate(0, 0, 45),
		Items: []command.ItemInput{{Description: "Servicio actualizado", Quantity: 2, UnitPrice: 5000}},
		Notes: "Revisado",
	})
	if err != nil {
		t.Fatalf("update draft: %v", err)
	}
	if updated.CustomerID != secondCustomer.ID || updated.Currency != "USD" {
		t.Fatalf("updated customer/currency = %s/%s", updated.CustomerID, updated.Currency)
	}
	if len(updated.Items) != 1 || updated.Items[0].Description != "Servicio actualizado" || updated.Total.Amount != 10000 {
		t.Fatalf("updated invoice lines/totals = %+v / %d", updated.Items, updated.Total.Amount)
	}
	if updated.Notes != "Revisado" {
		t.Fatalf("updated notes = %q, want %q", updated.Notes, "Revisado")
	}
}

func TestUpdateInvoiceRejectsIssuedInvoice(t *testing.T) {
	const tenantID = "tenant-issued"
	invoiceRepo := repository.NewInMemoryInvoiceRepository()
	customerRepo := repository.NewInMemoryCustomerRepository()
	customer := repository.NewTestCustomer(tenantID)
	ctx := repository.WithTenantID(context.Background(), tenantID)
	if err := customerRepo.Save(ctx, customer); err != nil {
		t.Fatalf("save customer: %v", err)
	}
	invoice, err := command.NewCreateInvoiceHandler(invoiceRepo, customerRepo).Handle(ctx, command.CreateInvoiceCommand{
		TenantID: tenantID, CustomerID: customer.ID, Currency: "COP",
		DueDate: time.Now().AddDate(0, 0, 30),
		Items:   []command.ItemInput{{Description: "Servicio", Quantity: 1, UnitPrice: 10000}},
	})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	if _, err := command.NewIssueInvoiceHandler(invoiceRepo).Handle(ctx, command.IssueInvoiceCommand{
		InvoiceID: invoice.ID, TenantID: tenantID,
	}); err != nil {
		t.Fatalf("issue invoice: %v", err)
	}

	_, err = command.NewUpdateInvoiceHandler(invoiceRepo, customerRepo, repository.NewInMemoryUnitOfWork()).Handle(ctx, command.UpdateInvoiceCommand{
		InvoiceID: invoice.ID, TenantID: tenantID, CustomerID: customer.ID,
		Currency: "COP", DueDate: time.Now().AddDate(0, 0, 30),
		Items: []command.ItemInput{{Description: "Cambio", Quantity: 1, UnitPrice: 1}},
	})
	if !errors.Is(err, entity.ErrInvalidStatusTransition) {
		t.Fatalf("update issued invoice error = %v, want %v", err, entity.ErrInvalidStatusTransition)
	}
}
