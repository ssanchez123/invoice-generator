package entity

import (
	"time"

	"github.com/santiagossaa/invoice-generator/internal/domain/valueobject"
)

// InvoiceStatus represents the lifecycle state of an invoice.
type InvoiceStatus string

const (
	InvoiceStatusDraft     InvoiceStatus = "draft"
	InvoiceStatusIssued    InvoiceStatus = "issued"
	InvoiceStatusPaid      InvoiceStatus = "paid"
	InvoiceStatusOverdue   InvoiceStatus = "overdue"
	InvoiceStatusCancelled InvoiceStatus = "cancelled"
)

// InvoiceItem is a line item within an invoice.
type InvoiceItem struct {
	ID          string
	Description string
	Quantity    int64
	UnitPrice   valueobject.Money
	TaxRateBPS  int64 // Tax rate in basis points (0 = tax-exempt)
	DiscountBPS int64 // Discount in basis points (0 = no discount)
}

// LineTotal computes the total for this item: quantity * unit_price, minus discount, plus tax.
func (item InvoiceItem) LineTotal() (valueobject.Money, error) {
	subtotal := item.UnitPrice.Multiply(item.Quantity)

	if item.DiscountBPS > 0 {
		discount := subtotal.MultiplyBasisPoints(item.DiscountBPS)
		subtracted, err := subtotal.Subtract(discount)
		if err != nil {
			return valueobject.Money{}, err
		}
		subtotal = subtracted
	}

	if item.TaxRateBPS > 0 {
		tax := subtotal.MultiplyBasisPoints(item.TaxRateBPS)
		total, err := subtotal.Add(tax)
		if err != nil {
			return valueobject.Money{}, err
		}
		return total, nil
	}

	return subtotal, nil
}

// TaxBreakdown returns the tax component of this line item.
func (item InvoiceItem) TaxAmount() (valueobject.Money, error) {
	subtotal := item.UnitPrice.Multiply(item.Quantity)

	if item.DiscountBPS > 0 {
		discount := subtotal.MultiplyBasisPoints(item.DiscountBPS)
		subtracted, err := subtotal.Subtract(discount)
		if err != nil {
			return valueobject.Money{}, err
		}
		subtotal = subtracted
	}

	if item.TaxRateBPS > 0 {
		return subtotal.MultiplyBasisPoints(item.TaxRateBPS), nil
	}

	return valueobject.Money{Amount: 0, Currency: subtotal.Currency}, nil
}

// Invoice is the root aggregate of the billing domain.
type Invoice struct {
	ID         string
	TenantID   string
	CustomerID string
	Number     valueobject.InvoiceNumber
	Status     InvoiceStatus

	IssueDate time.Time
	DueDate   time.Time

	Items []InvoiceItem

	Currency string
	Subtotal valueobject.Money
	TaxTotal valueobject.Money
	Total    valueobject.Money

	Notes    string
	Metadata map[string]any

	CreatedAt time.Time
	UpdatedAt time.Time

	// Domain events collected during operations
	Events []DomainEvent
}

// InvoiceAggregateRoot interface ensures this entity satisfies aggregate invariants.
// Other code interacts with Invoice through the repository, which enforces consistency.

// AddItem adds a line item to the invoice (only allowed in draft status).
func (inv *Invoice) AddItem(item InvoiceItem) error {
	if inv.Status != InvoiceStatusDraft {
		return ErrInvalidStatusTransition
	}
	inv.Items = append(inv.Items, item)
	return inv.recalculate()
}

// RemoveItem removes a line item by ID (only in draft).
func (inv *Invoice) RemoveItem(itemID string) error {
	if inv.Status != InvoiceStatusDraft {
		return ErrInvalidStatusTransition
	}
	for i, item := range inv.Items {
		if item.ID == itemID {
			inv.Items = append(inv.Items[:i], inv.Items[i+1:]...)
			return inv.recalculate()
		}
	}
	return ErrItemNotFound
}

// Issue transitions the invoice from draft to issued.
func (inv *Invoice) Issue(at time.Time) error {
	if inv.Status != InvoiceStatusDraft {
		return ErrInvalidStatusTransition
	}
	if len(inv.Items) == 0 {
		return ErrNoItems
	}
	if at.After(inv.DueDate) {
		return ErrIssueAfterDue
	}

	inv.Status = InvoiceStatusIssued
	inv.IssueDate = at
	inv.UpdatedAt = at

	inv.Events = append(inv.Events, InvoiceIssuedEvent{
		InvoiceID:   inv.ID,
		TenantIDVal: inv.TenantID,
		IssuedAt:    at,
	})

	return nil
}

// Cancel transitions the invoice to cancelled (from draft or issued).
func (inv *Invoice) Cancel() error {
	if inv.Status == InvoiceStatusPaid || inv.Status == InvoiceStatusCancelled {
		return ErrInvalidStatusTransition
	}
	inv.Status = InvoiceStatusCancelled
	return nil
}

// MarkPaid transitions the invoice to paid (from issued or overdue).
func (inv *Invoice) MarkPaid(at time.Time) error {
	if inv.Status != InvoiceStatusIssued && inv.Status != InvoiceStatusOverdue {
		return ErrInvalidStatusTransition
	}
	inv.Status = InvoiceStatusPaid
	inv.UpdatedAt = at

	inv.Events = append(inv.Events, InvoicePaidEvent{
		InvoiceID:   inv.ID,
		TenantIDVal: inv.TenantID,
		PaidAt:      at,
	})

	return nil
}

// MarkOverdue transitions the invoice to overdue (from issued).
func (inv *Invoice) MarkOverdue() error {
	if inv.Status != InvoiceStatusIssued {
		return ErrInvalidStatusTransition
	}
	inv.Status = InvoiceStatusOverdue
	return nil
}

// ApplyPayment records a partial payment and updates status if fully paid.
func (inv *Invoice) ApplyPayment(amount valueobject.Money, at time.Time) error {
	if inv.Status != InvoiceStatusIssued && inv.Status != InvoiceStatusOverdue {
		return ErrInvalidStatusTransition
	}

	// Payment logic is handled by Payment entity, but invoice status updates here.
	// In a full implementation, we'd check if cumulative payments >= total.
	// For now, the application layer handles this via the RecordPaymentCommand.

	inv.Events = append(inv.Events, PaymentRecordedEvent{
		InvoiceID:   inv.ID,
		TenantIDVal: inv.TenantID,
		Amount:      amount,
		PaidAt:      at,
	})

	return nil
}

// recalculate recomputes subtotal, tax total, and total from items.
func (inv *Invoice) recalculate() error {
	var subtotal, taxTotal int64

	for _, item := range inv.Items {
		lineTotal, err := item.LineTotal()
		if err != nil {
			return err
		}
		taxAmt, err := item.TaxAmount()
		if err != nil {
			return err
		}
		subtotal += lineTotal.Amount - taxAmt.Amount // line subtotal before tax
		taxTotal += taxAmt.Amount
	}

	currency := inv.Currency
	if currency == "" && len(inv.Items) > 0 {
		currency = inv.Items[0].UnitPrice.Currency
	}

	inv.Subtotal = valueobject.Money{Amount: subtotal, Currency: currency}
	inv.TaxTotal = valueobject.Money{Amount: taxTotal, Currency: currency}
	total, _ := inv.Subtotal.Add(inv.TaxTotal)
	inv.Total = total
	inv.UpdatedAt = time.Now()

	return nil
}

// ClearEvents returns all collected domain events and clears them from the aggregate.
// Called by the repository after persisting the aggregate and its events.
func (inv *Invoice) ClearEvents() []DomainEvent {
	events := inv.Events
	inv.Events = nil
	return events
}
