package pdf

import (
	"context"
	"errors"
	"testing"

	"github.com/santiagossaa/invoice-generator/internal/ctxkey"
	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
)

type invoiceReaderStub struct {
	invoice *entity.Invoice
	err     error
}

func (s invoiceReaderStub) FindByID(ctx context.Context, _ string) (*entity.Invoice, error) {
	if ctx.Value(ctxkey.TenantIDKey) != s.invoice.TenantID {
		return nil, errors.New("tenant context was not set")
	}
	return s.invoice, s.err
}

type customerReaderStub struct {
	customer *entity.Customer
}

func (s customerReaderStub) FindByID(ctx context.Context, _ string) (*entity.Customer, error) {
	if ctx.Value(ctxkey.TenantIDKey) != s.customer.TenantID {
		return nil, errors.New("customer lookup was not tenant scoped")
	}
	return s.customer, nil
}

type tenantReaderStub struct {
	tenant *entity.Tenant
}

func (s tenantReaderStub) FindByID(context.Context, string) (*entity.Tenant, error) {
	return s.tenant, nil
}

type rendererStub struct {
	document Document
}

func (s *rendererStub) Render(document Document) ([]byte, error) {
	s.document = document
	return []byte("%PDF-test"), nil
}

type storeStub struct {
	tenantID  string
	invoiceID string
	contents  []byte
	err       error
}

func (s *storeStub) PutInvoicePDF(_ context.Context, tenantID, invoiceID string, contents []byte) error {
	s.tenantID = tenantID
	s.invoiceID = invoiceID
	s.contents = contents
	return s.err
}

func (s *storeStub) GetInvoicePDF(context.Context, string, string) ([]byte, error) {
	return nil, nil
}

func TestGeneratorLoadsTenantScopedInvoiceAndStoresPDF(t *testing.T) {
	const tenantID = "tenant-1"
	const invoiceID = "invoice-1"
	invoice := &entity.Invoice{ID: invoiceID, TenantID: tenantID, CustomerID: "customer-1"}
	customer := &entity.Customer{ID: "customer-1", TenantID: tenantID}
	tenant := &entity.Tenant{ID: tenantID}
	renderer := &rendererStub{}
	store := &storeStub{}
	generator := NewGenerator(
		invoiceReaderStub{invoice: invoice},
		customerReaderStub{customer: customer},
		tenantReaderStub{tenant: tenant},
		renderer,
		store,
	)

	if err := generator.Generate(context.Background(), tenantID, invoiceID); err != nil {
		t.Fatalf("generate PDF: %v", err)
	}
	if renderer.document.Invoice != invoice || renderer.document.Customer != customer || renderer.document.Tenant != tenant {
		t.Fatal("renderer did not receive the loaded invoice, customer, and tenant")
	}
	if store.tenantID != tenantID || store.invoiceID != invoiceID {
		t.Fatalf("stored artifact under tenant %q invoice %q", store.tenantID, store.invoiceID)
	}
	if string(store.contents) != "%PDF-test" {
		t.Fatalf("stored contents = %q", store.contents)
	}
}

func TestGeneratorRejectsCrossTenantInvoice(t *testing.T) {
	invoice := &entity.Invoice{ID: "invoice-1", TenantID: "other-tenant"}
	store := &storeStub{}
	generator := NewGenerator(
		invoiceReaderStub{invoice: invoice},
		customerReaderStub{customer: &entity.Customer{TenantID: "other-tenant"}},
		tenantReaderStub{tenant: &entity.Tenant{ID: "requested-tenant"}},
		&rendererStub{},
		store,
	)

	if err := generator.Generate(context.Background(), "requested-tenant", invoice.ID); err == nil {
		t.Fatal("expected cross-tenant invoice to be rejected")
	}
	if store.contents != nil {
		t.Fatal("cross-tenant invoice must not be stored")
	}
}
