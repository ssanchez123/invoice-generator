package entity

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/santiagossaa/invoice-generator/internal/domain/valueobject"
)

const InvoiceSnapshotVersion = 1

type InvoiceSnapshot struct {
	Version    int                   `json:"version"`
	InvoiceID  string                `json:"invoice_id"`
	TenantID   string                `json:"tenant_id"`
	CustomerID string                `json:"customer_id"`
	Number     string                `json:"number"`
	IssueDate  time.Time             `json:"issue_date"`
	DueDate    time.Time             `json:"due_date"`
	Currency   string                `json:"currency"`
	Subtotal   int64                 `json:"subtotal"`
	TaxTotal   int64                 `json:"tax_total"`
	Total      int64                 `json:"total"`
	Notes      string                `json:"notes"`
	Metadata   map[string]any        `json:"metadata"`
	Seller     InvoicePartySnapshot  `json:"seller"`
	Customer   InvoicePartySnapshot  `json:"customer"`
	Items      []InvoiceLineSnapshot `json:"items"`
}

type InvoicePartySnapshot struct {
	Name    string              `json:"name"`
	Email   string              `json:"email,omitempty"`
	Phone   string              `json:"phone,omitempty"`
	TaxID   string              `json:"tax_id,omitempty"`
	Address valueobject.Address `json:"address"`
}

type InvoiceLineSnapshot struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Quantity    int64  `json:"quantity"`
	UnitPrice   int64  `json:"unit_price"`
	TaxRateBPS  int64  `json:"tax_rate_bps"`
	DiscountBPS int64  `json:"discount_bps"`
	Total       int64  `json:"total"`
}

func NewInvoiceSnapshot(invoice *Invoice, seller, customer InvoicePartySnapshot) (*InvoiceSnapshot, error) {
	if invoice == nil {
		return nil, errors.New("invoice is required")
	}
	if invoice.Status != InvoiceStatusIssued {
		return nil, errors.New("only issued invoices can be snapshotted")
	}
	if invoice.IssueDate.IsZero() {
		return nil, errors.New("invoice issue date is required")
	}
	if strings.TrimSpace(seller.Name) == "" || strings.TrimSpace(customer.Name) == "" {
		return nil, errors.New("seller and customer names are required")
	}
	if invoice.ID == "" || invoice.TenantID == "" || invoice.CustomerID == "" ||
		invoice.Number.String() == "" || invoice.Currency == "" || len(invoice.Items) == 0 {
		return nil, errors.New("invoice identity, number, currency, and items are required")
	}

	metadataJSON, err := json.Marshal(invoice.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal invoice metadata: %w", err)
	}
	metadata := make(map[string]any)
	if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
		return nil, fmt.Errorf("copy invoice metadata: %w", err)
	}
	if metadata == nil {
		metadata = make(map[string]any)
	}

	items := make([]InvoiceLineSnapshot, 0, len(invoice.Items))
	var subtotal, taxTotal, total int64
	for _, item := range invoice.Items {
		lineTotal, err := item.LineTotal()
		if err != nil {
			return nil, fmt.Errorf("calculate item %s total: %w", item.ID, err)
		}
		tax, err := item.TaxAmount()
		if err != nil {
			return nil, fmt.Errorf("calculate item %s tax: %w", item.ID, err)
		}
		if item.UnitPrice.Currency != invoice.Currency {
			return nil, fmt.Errorf("item %s currency does not match invoice currency", item.ID)
		}
		subtotal += lineTotal.Amount - tax.Amount
		taxTotal += tax.Amount
		total += lineTotal.Amount
		items = append(items, InvoiceLineSnapshot{
			ID:          item.ID,
			Description: item.Description,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice.Amount,
			TaxRateBPS:  item.TaxRateBPS,
			DiscountBPS: item.DiscountBPS,
			Total:       lineTotal.Amount,
		})
	}
	if subtotal != invoice.Subtotal.Amount || taxTotal != invoice.TaxTotal.Amount || total != invoice.Total.Amount {
		return nil, errors.New("invoice totals do not match its line items")
	}

	return &InvoiceSnapshot{
		Version:    InvoiceSnapshotVersion,
		InvoiceID:  invoice.ID,
		TenantID:   invoice.TenantID,
		CustomerID: invoice.CustomerID,
		Number:     invoice.Number.String(),
		IssueDate:  invoice.IssueDate,
		DueDate:    invoice.DueDate,
		Currency:   invoice.Currency,
		Subtotal:   invoice.Subtotal.Amount,
		TaxTotal:   invoice.TaxTotal.Amount,
		Total:      invoice.Total.Amount,
		Notes:      invoice.Notes,
		Metadata:   metadata,
		Seller:     seller,
		Customer:   customer,
		Items:      items,
	}, nil
}
