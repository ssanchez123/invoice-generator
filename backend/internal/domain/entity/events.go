package entity

import (
	"time"

	"github.com/santiagossaa/invoice-generator/internal/domain/valueobject"
)

// DomainEvent is the interface for all domain events.
// Events represent something that happened in the domain that domain experts care about.
type DomainEvent interface {
	EventName() string
	AggregateID() string
	TenantID() string
	OccurredAt() time.Time
}

// InvoiceIssuedEvent fires when an invoice transitions from draft to issued.
type InvoiceIssuedEvent struct {
	InvoiceID   string
	TenantIDVal string
	IssuedAt    time.Time
}

func (e InvoiceIssuedEvent) EventName() string    { return "invoice.issued" }
func (e InvoiceIssuedEvent) AggregateID() string { return e.InvoiceID }
func (e InvoiceIssuedEvent) TenantID() string   { return e.TenantIDVal }
func (e InvoiceIssuedEvent) OccurredAt() time.Time { return e.IssuedAt }

// InvoicePaidEvent fires when an invoice is fully paid.
type InvoicePaidEvent struct {
	InvoiceID   string
	TenantIDVal string
	PaidAt      time.Time
}

func (e InvoicePaidEvent) EventName() string      { return "invoice.paid" }
func (e InvoicePaidEvent) AggregateID() string    { return e.InvoiceID }
func (e InvoicePaidEvent) TenantID() string       { return e.TenantIDVal }
func (e InvoicePaidEvent) OccurredAt() time.Time { return e.PaidAt }

// PaymentRecordedEvent fires when any payment (partial or full) is recorded.
type PaymentRecordedEvent struct {
	InvoiceID   string
	TenantIDVal string
	Amount       valueobject.Money
	PaidAt       time.Time
}

func (e PaymentRecordedEvent) EventName() string      { return "payment.recorded" }
func (e PaymentRecordedEvent) AggregateID() string   { return e.InvoiceID }
func (e PaymentRecordedEvent) TenantID() string       { return e.TenantIDVal }
func (e PaymentRecordedEvent) OccurredAt() time.Time { return e.PaidAt }

// PDFGeneratedEvent fires when the PDF service completes generating an invoice PDF.
type PDFGeneratedEvent struct {
	InvoiceID    string
	TenantIDVal  string
	ArtifactURL  string
	GeneratedAt  time.Time
}

func (e PDFGeneratedEvent) EventName() string      { return "pdf.generated" }
func (e PDFGeneratedEvent) AggregateID() string   { return e.InvoiceID }
func (e PDFGeneratedEvent) TenantID() string       { return e.TenantIDVal }
func (e PDFGeneratedEvent) OccurredAt() time.Time { return e.GeneratedAt }

// SubscriptionRenewedEvent fires when a recurring subscription generates a new invoice.
type SubscriptionRenewedEvent struct {
	SubscriptionID string
	TenantIDVal    string
	InvoiceID      string
	RenewedAt      time.Time
}

func (e SubscriptionRenewedEvent) EventName() string      { return "subscription.renewed" }
func (e SubscriptionRenewedEvent) AggregateID() string    { return e.SubscriptionID }
func (e SubscriptionRenewedEvent) TenantID() string       { return e.TenantIDVal }
func (e SubscriptionRenewedEvent) OccurredAt() time.Time { return e.RenewedAt }