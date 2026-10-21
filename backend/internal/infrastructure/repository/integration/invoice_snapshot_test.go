//go:build integration

package integration

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/santiagossaa/invoice-generator/internal/application/command"
	"github.com/santiagossaa/invoice-generator/internal/domain/valueobject"
	"github.com/santiagossaa/invoice-generator/internal/infrastructure/repository"
)

func TestInvoiceRepositoryCapturesImmutableSnapshotOnIssue(t *testing.T) {
	testDB := SetupTestDB(t)
	ctx := ContextWithTenant(TestTenantID)

	invoiceRepo := repository.NewInvoiceRepository(testDB.DB)
	customerRepo := repository.NewCustomerRepository(testDB.DB)
	tenantRepo := repository.NewTenantRepository(testDB.DB)

	tenant, err := tenantRepo.FindByID(ctx, TestTenantID)
	if err != nil {
		t.Fatalf("load tenant: %v", err)
	}
	tenant.Name = "Seller at issue"
	tenant.Address = valueobject.Address{Line1: "Seller address at issue", City: "Bogotá", Country: "CO"}
	if err := tenantRepo.Save(ctx, tenant); err != nil {
		t.Fatalf("save tenant: %v", err)
	}

	customer, err := customerRepo.FindByID(ctx, TestCustomerID)
	if err != nil {
		t.Fatalf("load customer: %v", err)
	}
	customer.Name = "Customer at issue"
	customer.Email = "customer-at-issue@example.test"
	customer.Address = valueobject.Address{Line1: "Customer address at issue", City: "Medellín", Country: "CO"}
	if err := customerRepo.Save(ctx, customer); err != nil {
		t.Fatalf("save customer: %v", err)
	}

	create := command.NewCreateInvoiceHandler(invoiceRepo, customerRepo)
	invoice, err := create.Handle(ctx, command.CreateInvoiceCommand{
		TenantID:   TestTenantID,
		CustomerID: TestCustomerID,
		Currency:   "COP",
		DueDate:    time.Now().Add(30 * 24 * time.Hour),
		Notes:      "Notes at issue",
		Metadata:   map[string]any{"purchase_order": "PO-100"},
		Items: []command.ItemInput{{
			Description: "Service at issue",
			Quantity:    2,
			UnitPrice:   10000,
			TaxRateBPS:  1900,
			DiscountBPS: 500,
		}},
	})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}

	issue := command.NewIssueInvoiceHandler(invoiceRepo)
	issued, err := issue.Handle(ctx, command.IssueInvoiceCommand{
		InvoiceID: invoice.ID,
		TenantID:  TestTenantID,
	})
	if err != nil {
		t.Fatalf("issue invoice: %v", err)
	}
	if issued.IssuedSnapshot == nil {
		t.Fatal("issued invoice has no snapshot")
	}
	snapshot := *issued.IssuedSnapshot
	if snapshot.Seller.Name != "Seller at issue" ||
		snapshot.Seller.Address.Line1 != "Seller address at issue" ||
		snapshot.Customer.Name != "Customer at issue" ||
		snapshot.Customer.Email != "customer-at-issue@example.test" ||
		snapshot.Customer.Address.Line1 != "Customer address at issue" ||
		snapshot.Notes != "Notes at issue" ||
		snapshot.Metadata["purchase_order"] != "PO-100" ||
		len(snapshot.Items) != 1 ||
		snapshot.Items[0].Description != "Service at issue" {
		t.Fatalf("snapshot did not capture invoice parties and terms at issue: %+v", snapshot)
	}

	tenant.Name = "Seller after issue"
	tenant.Address.Line1 = "Seller address after issue"
	if err := tenantRepo.Save(ctx, tenant); err != nil {
		t.Fatalf("update tenant after issue: %v", err)
	}
	customer.Name = "Customer after issue"
	customer.Email = "customer-after-issue@example.test"
	customer.Address.Line1 = "Customer address after issue"
	if err := customerRepo.Save(ctx, customer); err != nil {
		t.Fatalf("update customer after issue: %v", err)
	}

	loaded, err := invoiceRepo.FindByID(ctx, invoice.ID)
	if err != nil {
		t.Fatalf("reload issued invoice: %v", err)
	}
	expectedSnapshot := snapshot
	expectedSnapshot.IssueDate = expectedSnapshot.IssueDate.UTC()
	expectedSnapshot.DueDate = expectedSnapshot.DueDate.UTC()
	actualSnapshot := *loaded.IssuedSnapshot
	actualSnapshot.IssueDate = actualSnapshot.IssueDate.UTC()
	actualSnapshot.DueDate = actualSnapshot.DueDate.UTC()
	if !reflect.DeepEqual(actualSnapshot, expectedSnapshot) {
		t.Fatalf("snapshot changed after seller and customer records were edited: got %+v, want %+v", actualSnapshot, expectedSnapshot)
	}

	tx, err := testDB.DB.TenantTx(ctx, TestTenantID)
	if err != nil {
		t.Fatalf("begin snapshot mutation transaction: %v", err)
	}
	_, err = tx.Exec(ctx, `
		UPDATE invoices
		SET issued_snapshot = jsonb_set(issued_snapshot, '{version}', '2'::jsonb)
		WHERE id = $1
	`, invoice.ID)
	_ = tx.Rollback(ctx)
	if err == nil {
		t.Fatal("database allowed an issued snapshot to be modified")
	}

	var eventCount int
	if err := testDB.Pool.QueryRow(context.Background(), `
		SELECT count(*) FROM outbox
		WHERE aggregate_id = $1 AND event_type = 'invoice.issued'
	`, invoice.ID).Scan(&eventCount); err != nil {
		t.Fatalf("count issue events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("invoice.issued event count = %d, want 1", eventCount)
	}
}
