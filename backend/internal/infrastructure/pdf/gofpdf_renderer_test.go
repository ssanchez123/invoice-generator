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
		Snapshot: entity.InvoiceSnapshot{
			Version:   entity.InvoiceSnapshotVersion,
			InvoiceID: "invoice-1",
			Number:    "INV-2026-001",
			IssueDate: time.Date(2026, 5, 20, 17, 0, 0, 0, time.UTC),
			DueDate:   time.Date(2026, 6, 20, 17, 0, 0, 0, time.UTC),
			Currency:  "COP",
			Subtotal:  20000,
			TaxTotal:  3800,
			Total:     23800,
			Notes:     "Gracias por su compra.",
			Seller: entity.InvoicePartySnapshot{
				Name:    "Compañía de Prueba",
				TaxID:   "900123456-7",
				Address: valueobject.Address{Line1: "Calle 10", City: "Bogotá", Country: "CO"},
			},
			Customer: entity.InvoicePartySnapshot{
				Name:    "José Pérez",
				Email:   "jose@example.test",
				Address: valueobject.Address{Line1: "Carrera 1", City: "Medellín", Country: "CO"},
			},
			Items: []entity.InvoiceLineSnapshot{{
				Description: "Consultoría técnica",
				Quantity:    2,
				UnitPrice:   10000,
				TaxRateBPS:  1900,
				Total:       23800,
			}},
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
	if got := formatBasisPoints(1900); got != "19,00 %" {
		t.Fatalf("formatBasisPoints(1900) = %q, want 19,00 %%", got)
	}
}

func TestColombianFormatting(t *testing.T) {
	if got := formatColombianMoney(123456789, "cop"); got != "COP 1.234.567,89" {
		t.Fatalf("formatColombianMoney() = %q, want COP 1.234.567,89", got)
	}
	if got := formatColombianMoney(-12345, "COP"); got != "COP -123,45" {
		t.Fatalf("formatColombianMoney() with negative amount = %q, want COP -123,45", got)
	}
	if got := formatColombianDate(time.Date(2026, 5, 20, 17, 0, 0, 0, time.UTC)); got != "20/05/2026" {
		t.Fatalf("formatColombianDate() = %q, want 20/05/2026", got)
	}
}
