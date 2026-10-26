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
	ErrInvalidInvoiceDetails   = errors.New("invalid invoice details")
	ErrInvalidSubscription     = errors.New("invalid subscription details")
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
	BillingDay      int
	Status          SubscriptionStatus
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

var colombiaBillingLocation = time.FixedZone("COT", -5*60*60)

func NewSubscription(id, tenantID, customerID string, items []InvoiceItem, frequency BillingFrequency, nextBillingDate time.Time) (*Subscription, error) {
	if id == "" || tenantID == "" || customerID == "" || nextBillingDate.IsZero() || len(items) == 0 {
		return nil, ErrInvalidSubscription
	}
	switch frequency {
	case FrequencyDaily, FrequencyWeekly, FrequencyMonthly, FrequencyYearly:
	default:
		return nil, ErrInvalidSubscription
	}
	currency := items[0].UnitPrice.Currency
	for _, item := range items {
		if item.ID == "" || item.Description == "" || item.Quantity <= 0 ||
			item.UnitPrice.Currency != currency || item.UnitPrice.Amount < 0 ||
			item.TaxRateBPS < 0 || item.DiscountBPS < 0 || item.DiscountBPS > 10000 {
			return nil, ErrInvalidSubscription
		}
	}
	now := time.Now()
	return &Subscription{
		ID:              id,
		TenantID:        tenantID,
		CustomerID:      customerID,
		PlanItems:       append([]InvoiceItem(nil), items...),
		Frequency:       frequency,
		NextBillingDate: nextBillingDate,
		BillingDay:      nextBillingDate.In(colombiaBillingLocation).Day(),
		Status:          SubscriptionStatusActive,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

// AdvanceBillingDate moves next_billing_date to the next occurrence.
func (s *Subscription) AdvanceBillingDate() {
	billingDate := s.NextBillingDate.In(colombiaBillingLocation)
	billingDay := s.BillingDay
	if billingDay <= 0 {
		billingDay = billingDate.Day()
	}
	switch s.Frequency {
	case FrequencyDaily:
		s.NextBillingDate = billingDate.AddDate(0, 0, 1)
	case FrequencyWeekly:
		s.NextBillingDate = billingDate.AddDate(0, 0, 7)
	case FrequencyMonthly:
		s.NextBillingDate = addMonthsClamped(billingDate, 1, billingDay)
	case FrequencyYearly:
		s.NextBillingDate = addMonthsClamped(billingDate, 12, billingDay)
	}
	s.UpdatedAt = time.Now()
}

func addMonthsClamped(value time.Time, months, desiredDay int) time.Time {
	year, month, _ := value.Date()
	targetMonth := month + time.Month(months)
	firstOfMonth := time.Date(year, targetMonth, 1, value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), value.Location())
	lastDay := time.Date(firstOfMonth.Year(), firstOfMonth.Month()+1, 0, value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), value.Location()).Day()
	if desiredDay > lastDay {
		desiredDay = lastDay
	}
	return time.Date(firstOfMonth.Year(), firstOfMonth.Month(), desiredDay, value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), value.Location())
}

// Pause transitions subscription to paused.
func (s *Subscription) Pause() error {
	if s.Status != SubscriptionStatusActive {
		return errors.New("only active subscriptions can be paused")
	}
	s.Status = SubscriptionStatusPaused
	s.UpdatedAt = time.Now()
	return nil
}

// Resume transitions subscription back to active.
func (s *Subscription) Resume() error {
	if s.Status != SubscriptionStatusPaused {
		return errors.New("only paused subscriptions can be resumed")
	}
	s.Status = SubscriptionStatusActive
	s.UpdatedAt = time.Now()
	return nil
}

// Cancel transitions subscription to cancelled.
func (s *Subscription) Cancel() error {
	if s.Status == SubscriptionStatusCancelled {
		return errors.New("subscription is already cancelled")
	}
	s.Status = SubscriptionStatusCancelled
	s.UpdatedAt = time.Now()
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
