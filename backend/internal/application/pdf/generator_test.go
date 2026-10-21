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
	snapshot := entity.InvoiceSnapshot{
		Version:    entity.InvoiceSnapshotVersion,
		InvoiceID:  invoiceID,
		TenantID:   tenantID,
		CustomerID: "customer-1",
		Number:     "INV-2026-00001",
		Seller:     entity.InvoicePartySnapshot{Name: "Seller"},
		Customer:   entity.InvoicePartySnapshot{Name: "Customer"},
	}
	invoice := &entity.Invoice{
		ID:             invoiceID,
		TenantID:       tenantID,
		CustomerID:     "customer-1",
		IssuedSnapshot: &snapshot,
	}
	renderer := &rendererStub{}
	store := &storeStub{}
	generator := NewGenerator(
		invoiceReaderStub{invoice: invoice},
		renderer,
		store,
	)

	if err := generator.Generate(context.Background(), tenantID, invoiceID); err != nil {
		t.Fatalf("generate PDF: %v", err)
	}
	if renderer.document.Snapshot.InvoiceID != invoiceID ||
		renderer.document.Snapshot.Seller.Name != "Seller" ||
		renderer.document.Snapshot.Customer.Name != "Customer" {
		t.Fatal("renderer did not receive the immutable invoice snapshot")
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

func TestGeneratorRejectsMissingSnapshot(t *testing.T) {
	invoice := &entity.Invoice{ID: "invoice-1", TenantID: "tenant-1"}
	store := &storeStub{}
	generator := NewGenerator(invoiceReaderStub{invoice: invoice}, &rendererStub{}, store)

	if err := generator.Generate(context.Background(), invoice.TenantID, invoice.ID); !errors.Is(err, ErrInvoiceSnapshotUnavailable) {
		t.Fatalf("generate error = %v, want %v", err, ErrInvoiceSnapshotUnavailable)
	}
	if store.contents != nil {
		t.Fatal("invoice without snapshot must not produce an artifact")
	}
}
