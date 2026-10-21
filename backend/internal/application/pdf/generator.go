package pdf

import (
	"context"
	"fmt"

	"github.com/santiagossaa/invoice-generator/internal/ctxkey"
	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/domain/port"
)

type InvoiceReader interface {
	FindByID(ctx context.Context, id string) (*entity.Invoice, error)
}

type CustomerReader interface {
	FindByID(ctx context.Context, id string) (*entity.Customer, error)
}

type TenantReader interface {
	FindByID(ctx context.Context, id string) (*entity.Tenant, error)
}

type Renderer interface {
	Render(Document) ([]byte, error)
}

type Document struct {
	Invoice  *entity.Invoice
	Customer *entity.Customer
	Tenant   *entity.Tenant
}

type Generator struct {
	invoices  InvoiceReader
	customers CustomerReader
	tenants   TenantReader
	renderer  Renderer
	store     port.PDFArtifactStore
}

func NewGenerator(invoices InvoiceReader, customers CustomerReader, tenants TenantReader, renderer Renderer, store port.PDFArtifactStore) *Generator {
	return &Generator{
		invoices:  invoices,
		customers: customers,
		tenants:   tenants,
		renderer:  renderer,
		store:     store,
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

	customer, err := g.customers.FindByID(tenantCtx, invoice.CustomerID)
	if err != nil {
		return fmt.Errorf("load invoice customer: %w", err)
	}
	if customer.TenantID != tenantID {
		return fmt.Errorf("customer tenant does not match delivery tenant")
	}

	tenant, err := g.tenants.FindByID(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("load invoice tenant: %w", err)
	}
	if tenant.ID != tenantID {
		return fmt.Errorf("loaded tenant does not match delivery tenant")
	}

	contents, err := g.renderer.Render(Document{
		Invoice:  invoice,
		Customer: customer,
		Tenant:   tenant,
	})
	if err != nil {
		return fmt.Errorf("render invoice PDF: %w", err)
	}
	if err := g.store.PutInvoicePDF(ctx, tenantID, invoiceID, contents); err != nil {
		return fmt.Errorf("store invoice PDF: %w", err)
	}
	return nil
}
