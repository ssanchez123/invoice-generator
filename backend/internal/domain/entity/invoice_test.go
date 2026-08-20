package entity

import (
	"testing"
	"time"

	"github.com/santiagossaa/invoice-generator/internal/domain/valueobject"
)

func TestInvoiceItemLineTotal(t *testing.T) {
	t.Run("simple item no tax no discount", func(t *testing.T) {
		item := InvoiceItem{
			Quantity:  3,
			UnitPrice: valueobject.Money{Amount: 1000, Currency: "USD"}, // $10.00
		}
		total, err := item.LineTotal()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total.Amount != 3000 {
			t.Errorf("LineTotal() = %d, want 3000", total.Amount)
		}
	})

	t.Run("item with tax", func(t *testing.T) {
		item := InvoiceItem{
			Quantity:   2,
			UnitPrice:  valueobject.Money{Amount: 5000, Currency: "USD"}, // $50.00
			TaxRateBPS: 1900,                                                // 19%
		}
		total, err := item.LineTotal()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// subtotal = 10000, tax = 1900, total = 11900
		if total.Amount != 11900 {
			t.Errorf("LineTotal() = %d, want 11900", total.Amount)
		}
	})

	t.Run("item with discount and tax", func(t *testing.T) {
		item := InvoiceItem{
			Quantity:    1,
			UnitPrice:   valueobject.Money{Amount: 10000, Currency: "USD"}, // $100.00
			TaxRateBPS:  1000,                                                 // 10%
			DiscountBPS: 1000,                                                  // 10% discount
		}
		total, err := item.LineTotal()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// subtotal = 10000, discount = 1000, after discount = 9000, tax = 900, total = 9900
		if total.Amount != 9900 {
			t.Errorf("LineTotal() = %d, want 9900", total.Amount)
		}
	})
}

func TestInvoiceItemTaxAmount(t *testing.T) {
	item := InvoiceItem{
		Quantity:   1,
		UnitPrice:  valueobject.Money{Amount: 10000, Currency: "USD"},
		TaxRateBPS: 1900,
	}
	tax, err := item.TaxAmount()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tax.Amount != 1900 {
		t.Errorf("TaxAmount() = %d, want 1900", tax.Amount)
	}

	// Tax-exempt item
	exempt := InvoiceItem{
		Quantity:   1,
		UnitPrice:  valueobject.Money{Amount: 10000, Currency: "USD"},
		TaxRateBPS: 0,
	}
	tax, _ = exempt.TaxAmount()
	if tax.Amount != 0 {
		t.Errorf("TaxAmount() tax-exempt = %d, want 0", tax.Amount)
	}
}

func TestInvoiceAddItem(t *testing.T) {
	inv := newDraftInvoice()

	item := InvoiceItem{
		ID:          "item-1",
		Description: "Consulting",
		Quantity:    10,
		UnitPrice:   valueobject.Money{Amount: 5000, Currency: "USD"},
		TaxRateBPS:  1900,
	}
	err := inv.AddItem(item)
	if err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}
	if len(inv.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(inv.Items))
	}

	// Subtotal should be recalculated
	// subtotal = 10 * 5000 = 50000, tax = 50000 * 0.19 = 9500
	if inv.Subtotal.Amount != 50000 {
		t.Errorf("Subtotal = %d, want 50000", inv.Subtotal.Amount)
	}
	if inv.TaxTotal.Amount != 9500 {
		t.Errorf("TaxTotal = %d, want 9500", inv.TaxTotal.Amount)
	}
	if inv.Total.Amount != 59500 {
		t.Errorf("Total = %d, want 59500", inv.Total.Amount)
	}
}

func TestInvoiceAddItemNotDraft(t *testing.T) {
	inv := newDraftInvoice()
	inv.Status = InvoiceStatusIssued

	err := inv.AddItem(InvoiceItem{
		ID:          "item-1",
		Description: "Test",
		Quantity:    1,
		UnitPrice:   valueobject.Money{Amount: 1000, Currency: "USD"},
	})
	if err != ErrInvalidStatusTransition {
		t.Errorf("AddItem on issued invoice should return ErrInvalidStatusTransition, got %v", err)
	}
}

func TestInvoiceRemoveItem(t *testing.T) {
	inv := newDraftInvoice()
	inv.Items = []InvoiceItem{
		{ID: "item-1", Description: "A", Quantity: 1, UnitPrice: valueobject.Money{Amount: 1000, Currency: "USD"}},
		{ID: "item-2", Description: "B", Quantity: 1, UnitPrice: valueobject.Money{Amount: 2000, Currency: "USD"}},
	}
	inv.recalculate()

	err := inv.RemoveItem("item-1")
	if err != nil {
		t.Fatalf("RemoveItem failed: %v", err)
	}
	if len(inv.Items) != 1 {
		t.Fatalf("expected 1 item after remove, got %d", len(inv.Items))
	}
	if inv.Items[0].ID != "item-2" {
		t.Errorf("remaining item ID = %s, want item-2", inv.Items[0].ID)
	}
	// Subtotal should be updated
	if inv.Subtotal.Amount != 2000 {
		t.Errorf("Subtotal after remove = %d, want 2000", inv.Subtotal.Amount)
	}
}

func TestInvoiceRemoveItemNotFound(t *testing.T) {
	inv := newDraftInvoice()
	err := inv.RemoveItem("nonexistent")
	if err != ErrItemNotFound {
		t.Errorf("RemoveItem nonexistent should return ErrItemNotFound, got %v", err)
	}
}

func TestInvoiceIssue(t *testing.T) {
	t.Run("valid issue from draft", func(t *testing.T) {
		inv := newDraftInvoice()
		inv.Items = []InvoiceItem{
			{ID: "i1", Description: "Test", Quantity: 1, UnitPrice: valueobject.Money{Amount: 1000, Currency: "USD"}},
		}
		inv.recalculate()

		now := time.Now()
		err := inv.Issue(now)
		if err != nil {
			t.Fatalf("Issue failed: %v", err)
		}
		if inv.Status != InvoiceStatusIssued {
			t.Errorf("Status = %s, want issued", inv.Status)
		}
		if !inv.IssueDate.Equal(now) {
			t.Errorf("IssueDate = %v, want %v", inv.IssueDate, now)
		}
		// Should produce InvoiceIssued event
		if len(inv.Events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(inv.Events))
		}
		if inv.Events[0].EventName() != "invoice.issued" {
			t.Errorf("event = %s, want invoice.issued", inv.Events[0].EventName())
		}
	})

	t.Run("issue with no items", func(t *testing.T) {
		inv := newDraftInvoice()
		err := inv.Issue(time.Now())
		if err != ErrNoItems {
			t.Errorf("Issue with no items should return ErrNoItems, got %v", err)
		}
	})

	t.Run("issue after due date", func(t *testing.T) {
		inv := newDraftInvoice()
		inv.Items = []InvoiceItem{
			{ID: "i1", Description: "Test", Quantity: 1, UnitPrice: valueobject.Money{Amount: 1000, Currency: "USD"}},
		}
		inv.recalculate()
		inv.DueDate = time.Now().Add(-24 * time.Hour) // due yesterday

		err := inv.Issue(time.Now())
		if err != ErrIssueAfterDue {
			t.Errorf("Issue after due should return ErrIssueAfterDue, got %v", err)
		}
	})

	t.Run("issue from issued", func(t *testing.T) {
		inv := newDraftInvoice()
		inv.Status = InvoiceStatusIssued
		err := inv.Issue(time.Now())
		if err != ErrInvalidStatusTransition {
			t.Errorf("Issue from issued should error, got %v", err)
		}
	})
}

func TestInvoiceCancel(t *testing.T) {
	t.Run("cancel from draft", func(t *testing.T) {
		inv := newDraftInvoice()
		err := inv.Cancel()
		if err != nil {
			t.Fatalf("Cancel failed: %v", err)
		}
		if inv.Status != InvoiceStatusCancelled {
			t.Errorf("Status = %s, want cancelled", inv.Status)
		}
	})

	t.Run("cancel from issued", func(t *testing.T) {
		inv := newDraftInvoice()
		inv.Status = InvoiceStatusIssued
		err := inv.Cancel()
		if err != nil {
			t.Fatalf("Cancel failed: %v", err)
		}
		if inv.Status != InvoiceStatusCancelled {
			t.Errorf("Status = %s, want cancelled", inv.Status)
		}
	})

	t.Run("cancel from paid", func(t *testing.T) {
		inv := newDraftInvoice()
		inv.Status = InvoiceStatusPaid
		err := inv.Cancel()
		if err != ErrInvalidStatusTransition {
			t.Errorf("Cancel from paid should error, got %v", err)
		}
	})

	t.Run("cancel from cancelled", func(t *testing.T) {
		inv := newDraftInvoice()
		inv.Status = InvoiceStatusCancelled
		err := inv.Cancel()
		if err != ErrInvalidStatusTransition {
			t.Errorf("Cancel from cancelled should error, got %v", err)
		}
	})
}

func TestInvoiceMarkPaid(t *testing.T) {
	t.Run("valid from issued", func(t *testing.T) {
		inv := newDraftInvoice()
		inv.Status = InvoiceStatusIssued
		now := time.Now()
		err := inv.MarkPaid(now)
		if err != nil {
			t.Fatalf("MarkPaid failed: %v", err)
		}
		if inv.Status != InvoiceStatusPaid {
			t.Errorf("Status = %s, want paid", inv.Status)
		}
		// Should produce InvoicePaid event
		if len(inv.Events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(inv.Events))
		}
		if inv.Events[0].EventName() != "invoice.paid" {
			t.Errorf("event = %s, want invoice.paid", inv.Events[0].EventName())
		}
	})

	t.Run("from draft", func(t *testing.T) {
		inv := newDraftInvoice()
		err := inv.MarkPaid(time.Now())
		if err != ErrInvalidStatusTransition {
			t.Errorf("MarkPaid from draft should error, got %v", err)
		}
	})

	t.Run("from overdue", func(t *testing.T) {
		inv := newDraftInvoice()
		inv.Status = InvoiceStatusOverdue
		err := inv.MarkPaid(time.Now())
		if err != nil {
			t.Fatalf("MarkPaid from overdue should work: %v", err)
		}
		if inv.Status != InvoiceStatusPaid {
			t.Errorf("Status = %s, want paid", inv.Status)
		}
	})
}

func TestInvoiceMarkOverdue(t *testing.T) {
	t.Run("from issued", func(t *testing.T) {
		inv := newDraftInvoice()
		inv.Status = InvoiceStatusIssued
		err := inv.MarkOverdue()
		if err != nil {
			t.Fatalf("MarkOverdue failed: %v", err)
		}
		if inv.Status != InvoiceStatusOverdue {
			t.Errorf("Status = %s, want overdue", inv.Status)
		}
	})

	t.Run("from draft", func(t *testing.T) {
		inv := newDraftInvoice()
		err := inv.MarkOverdue()
		if err != ErrInvalidStatusTransition {
			t.Errorf("MarkOverdue from draft should error, got %v", err)
		}
	})
}

func TestInvoiceApplyPayment(t *testing.T) {
	t.Run("from issued", func(t *testing.T) {
		inv := newDraftInvoice()
		inv.Status = InvoiceStatusIssued
		amount := valueobject.Money{Amount: 5000, Currency: "USD"}
		err := inv.ApplyPayment(amount, time.Now())
		if err != nil {
			t.Fatalf("ApplyPayment failed: %v", err)
		}
		if len(inv.Events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(inv.Events))
		}
		if inv.Events[0].EventName() != "payment.recorded" {
			t.Errorf("event = %s, want payment.recorded", inv.Events[0].EventName())
		}
	})

	t.Run("from draft", func(t *testing.T) {
		inv := newDraftInvoice()
		amount := valueobject.Money{Amount: 5000, Currency: "USD"}
		err := inv.ApplyPayment(amount, time.Now())
		if err != ErrInvalidStatusTransition {
			t.Errorf("ApplyPayment from draft should error, got %v", err)
		}
	})
}

func TestInvoiceClearEvents(t *testing.T) {
	inv := newDraftInvoice()
	inv.Status = InvoiceStatusIssued
	inv.Events = []DomainEvent{
		InvoiceIssuedEvent{InvoiceID: inv.ID, TenantIDVal: "t1", IssuedAt: time.Now()},
	}

	events := inv.ClearEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if len(inv.Events) != 0 {
		t.Errorf("after ClearEvents, len(Events) = %d, want 0", len(inv.Events))
	}
}

func TestInvoiceRecalculate(t *testing.T) {
	inv := &Invoice{
		Currency: "USD",
		Items: []InvoiceItem{
			{ID: "i1", Quantity: 2, UnitPrice: valueobject.Money{Amount: 1000, Currency: "USD"}, TaxRateBPS: 1900},
			{ID: "i2", Quantity: 1, UnitPrice: valueobject.Money{Amount: 5000, Currency: "USD"}, TaxRateBPS: 0},
		},
	}

	err := inv.recalculate()
	if err != nil {
		t.Fatalf("recalculate failed: %v", err)
	}

	// Item 1: subtotal = 2000, tax = 380, total = 2380
	// Item 2: subtotal = 5000, tax = 0, total = 5000
	// Aggregate subtotal (before tax) = 2000 + 5000 = 7000
	// Tax total = 380
	// Total = 7000 + 380 = 7380
	if inv.Subtotal.Amount != 7000 {
		t.Errorf("Subtotal = %d, want 7000", inv.Subtotal.Amount)
	}
	if inv.TaxTotal.Amount != 380 {
		t.Errorf("TaxTotal = %d, want 380", inv.TaxTotal.Amount)
	}
	if inv.Total.Amount != 7380 {
		t.Errorf("Total = %d, want 7380", inv.Total.Amount)
	}
}

// newDraftInvoice creates a minimal draft invoice for testing.
func newDraftInvoice() *Invoice {
	return &Invoice{
		ID:         "inv-test-1",
		TenantID:   "tenant-test-1",
		CustomerID: "cust-test-1",
		Number:     valueobject.InvoiceNumber{Value: "INV-2024-00001"},
		Status:     InvoiceStatusDraft,
		DueDate:    time.Now().Add(30 * 24 * time.Hour), // 30 days from now
		Currency:   "USD",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}