package pdf

import (
	"bytes"
	"testing"
	"time"

	pdfapp "github.com/santiagossaa/invoice-generator/internal/application/pdf"
	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/domain/valueobject"
)

func TestGoFPDFRendererProducesPDF(t *testing.T) {
	renderer := NewGoFPDFRenderer()
	doc := pdfapp.Document{
		Tenant: &entity.Tenant{
			Name:    "Compañía de Prueba",
			TaxID:   "NIT 123",
			Address: valueobject.Address{Line1: "Calle 10", City: "Bogotá", Country: "CO"},
		},
		Customer: &entity.Customer{
			Name:    "José Pérez",
			Email:   "jose@example.test",
			Address: valueobject.Address{Line1: "Carrera 1", City: "Medellín", Country: "CO"},
		},
		Invoice: &entity.Invoice{
			Number:    valueobject.InvoiceNumber{Value: "INV-2026-001"},
			IssueDate: time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC),
			DueDate:   time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC),
			Items: []entity.InvoiceItem{{
				Description: "Consultoría técnica",
				Quantity:    2,
				UnitPrice:   valueobject.Money{Amount: 10000, Currency: "COP"},
				TaxRateBPS:  1900,
			}},
			Subtotal: valueobject.Money{Amount: 20000, Currency: "COP"},
			TaxTotal: valueobject.Money{Amount: 3800, Currency: "COP"},
			Total:    valueobject.Money{Amount: 23800, Currency: "COP"},
			Notes:    "Gracias por su compra.",
		},
	}

	contents, err := renderer.Render(doc)
	if err != nil {
		t.Fatalf("render PDF: %v", err)
	}
	if !bytes.HasPrefix(contents, []byte("%PDF-")) {
		t.Fatalf("rendered artifact does not have a PDF header: %q", contents[:min(len(contents), 20)])
	}
	if !bytes.Contains(contents, []byte("%%EOF")) {
		t.Fatal("rendered PDF has no EOF marker")
	}
}

func TestGoFPDFRendererRequiresInvoiceData(t *testing.T) {
	if _, err := NewGoFPDFRenderer().Render(pdfapp.Document{}); err == nil {
		t.Fatal("expected missing invoice data to fail")
	}
}

func TestFormatBasisPoints(t *testing.T) {
	if got := formatBasisPoints(1900); got != "19.00%" {
		t.Fatalf("formatBasisPoints(1900) = %q, want 19.00%%", got)
	}
}
