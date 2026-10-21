package pdf

import (
	"context"
	"errors"
	"fmt"

	"github.com/santiagossaa/invoice-generator/internal/ctxkey"
	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/domain/port"
)

var ErrInvoiceSnapshotUnavailable = errors.New("immutable issued invoice snapshot is unavailable")

type InvoiceReader interface {
	FindByID(ctx context.Context, id string) (*entity.Invoice, error)
}

type Renderer interface {
	Render(Document) ([]byte, error)
}

type Document struct {
	Snapshot entity.InvoiceSnapshot
}

type Generator struct {
	invoices InvoiceReader
	renderer Renderer
	store    port.PDFArtifactStore
}

func NewGenerator(invoices InvoiceReader, renderer Renderer, store port.PDFArtifactStore) *Generator {
	return &Generator{
		invoices: invoices,
		renderer: renderer,
		store:    store,
	}
}

func (g *Generator) Generate(ctx context.Context, tenantID, invoiceID string) error {
	tenantCtx := context.WithValue(ctx, ctxkey.TenantIDKey, tenantID)

	invoice, err := g.invoices.FindByID(tenantCtx, invoiceID)
	if err != nil {
		return fmt.Errorf("load invoice: %w", err)
	}
	if invoice.TenantID != tenantID {
		return fmt.Errorf("invoice tenant does not match delivery tenant")
	}
	if invoice.IssuedSnapshot == nil {
		return ErrInvoiceSnapshotUnavailable
	}
	snapshot := *invoice.IssuedSnapshot
	if snapshot.Version != entity.InvoiceSnapshotVersion ||
		snapshot.InvoiceID != invoiceID ||
		snapshot.TenantID != tenantID ||
		snapshot.CustomerID != invoice.CustomerID {
		return fmt.Errorf("issued invoice snapshot identity or version is invalid")
	}

	contents, err := g.renderer.Render(Document{Snapshot: snapshot})
	if err != nil {
		return fmt.Errorf("render invoice PDF: %w", err)
	}
	if err := g.store.PutInvoicePDF(ctx, tenantID, invoiceID, contents); err != nil {
		return fmt.Errorf("store invoice PDF: %w", err)
	}
	return nil
}
