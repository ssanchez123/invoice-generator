package entity

import (
	"errors"
	"time"

	"github.com/santiagossaa/invoice-generator/internal/domain/valueobject"
)

// Sentinel errors for domain invariants.
var (
	ErrInvalidStatusTransition = errors.New("invalid invoice status transition")
	ErrNoItems                 = errors.New("invoice must have at least one item")
	ErrIssueAfterDue           = errors.New("issue date cannot be after due date")
	ErrItemNotFound            = errors.New("invoice item not found")
	ErrInsufficientPayment     = errors.New("payment amount exceeds invoice total")
	ErrDuplicatePayment        = errors.New("payment already recorded for this invoice")
)

// Customer is the recipient of invoices.
type Customer struct {
	ID       string
	TenantID string
	Name     string
	Email    string
	Phone    string
	Address  valueobject.Address
	TaxID    string // National tax identifier (NIT, RFC, EIN, etc.)
	Metadata map[string]any

	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewCustomer(id, tenantID, name, email string, address valueobject.Address) (*Customer, error) {
	if name == "" {
		return nil, errors.New("customer name is required")
	}
	if err := address.Validate(); err != nil {
		return nil, err
	}
	return &Customer{
		ID:        id,
		TenantID:  tenantID,
		Name:      name,
		Email:     email,
		Address:   address,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

// Payment represents a recorded payment against an invoice.
type Payment struct {
	ID        string
	InvoiceID string
	TenantID  string
	Amount    valueobject.Money
	Method    string // "bank_transfer", "credit_card", "cash", "crypto", etc.
	Reference string // external reference (transaction ID, check number, etc.)
	PaidAt    time.Time
	CreatedAt time.Time
}

// Subscription defines recurring billing for a customer.
type SubscriptionStatus string

const (
	SubscriptionStatusActive    SubscriptionStatus = "active"
	SubscriptionStatusPaused    SubscriptionStatus = "paused"
	SubscriptionStatusCancelled SubscriptionStatus = "cancelled"
)

type BillingFrequency string

const (
	FrequencyDaily   BillingFrequency = "daily"
	FrequencyWeekly  BillingFrequency = "weekly"
	FrequencyMonthly BillingFrequency = "monthly"
	FrequencyYearly  BillingFrequency = "yearly"
)

type Subscription struct {
	ID              string
	TenantID        string
	CustomerID      string
	PlanItems       []InvoiceItem // template for generated invoices
	Frequency       BillingFrequency
	NextBillingDate time.Time
	Status          SubscriptionStatus
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// AdvanceBillingDate moves next_billing_date to the next occurrence.
func (s *Subscription) AdvanceBillingDate() {
	switch s.Frequency {
	case FrequencyDaily:
		s.NextBillingDate = s.NextBillingDate.AddDate(0, 0, 1)
	case FrequencyWeekly:
		s.NextBillingDate = s.NextBillingDate.AddDate(0, 0, 7)
	case FrequencyMonthly:
		s.NextBillingDate = s.NextBillingDate.AddDate(0, 1, 0)
	case FrequencyYearly:
		s.NextBillingDate = s.NextBillingDate.AddDate(1, 0, 0)
	}
}

// Pause transitions subscription to paused.
func (s *Subscription) Pause() error {
	if s.Status != SubscriptionStatusActive {
		return errors.New("only active subscriptions can be paused")
	}
	s.Status = SubscriptionStatusPaused
	return nil
}

// Resume transitions subscription back to active.
func (s *Subscription) Resume() error {
	if s.Status != SubscriptionStatusPaused {
		return errors.New("only paused subscriptions can be resumed")
	}
	s.Status = SubscriptionStatusActive
	return nil
}

// Cancel transitions subscription to cancelled.
func (s *Subscription) Cancel() error {
	if s.Status == SubscriptionStatusCancelled {
		return errors.New("subscription is already cancelled")
	}
	s.Status = SubscriptionStatusCancelled
	return nil
}

// Tenant represents an organization in the SaaS.
type Tenant struct {
	ID        string
	Name      string
	Address   valueobject.Address
	TaxID     string
	Settings  map[string]any // invoice number format, default currency, tax rules, etc.
	CreatedAt time.Time
	UpdatedAt time.Time
}
