package valueobject

import (
	"context"
	"errors"
	"strings"
	"time"
)

// TaxRule defines how tax applies based on jurisdiction.
// Tax rates are stored in basis points (100 = 1.00%, 1900 = 19.00%).
type TaxRule struct {
	Jurisdiction  string    // ISO 3166-1 alpha-2 country code, e.g., "CO", "US"
	Region        string    // Optional region/state code, e.g., "CA", "NY"
	RateBPS       int64     // Rate in basis points (1900 = 19%)
	Description   string    // Human-readable description
	EffectiveFrom time.Time // When this rule became active
	EffectiveTo   *time.Time // Optional: when this rule expired
}

func (tr TaxRule) IsActiveAt(t time.Time) bool {
	if t.Before(tr.EffectiveFrom) {
		return false
	}
	if tr.EffectiveTo != nil && t.After(*tr.EffectiveTo) {
		return false
	}
	return true
}

func (tr TaxRule) Validate() error {
	if len(tr.Jurisdiction) != 2 {
		return errors.New("jurisdiction must be a 2-letter ISO country code")
	}
	if tr.RateBPS < 0 || tr.RateBPS > 10000 {
		return errors.New("rate must be between 0 and 10000 basis points (0-100%)")
	}
	if tr.EffectiveFrom.IsZero() {
		return errors.New("effective_from is required")
	}
	return nil
}

// TaxResolverPort defines the interface for resolving applicable tax rules.
// Infrastructure layer implements this with a PostgreSQL-backed tax rules table.
type TaxResolverPort interface {
	// Resolve returns the applicable tax rule for a given jurisdiction and date.
	// Returns nil if no tax applies (e.g., tax-exempt region).
	Resolve(ctx context.Context, jurisdiction string, region string, at time.Time) (*TaxRule, error)
}

// Address is a structured postal address.
type Address struct {
	Line1    string `json:"line1"`
	Line2    string `json:"line2,omitempty"`
	City     string `json:"city"`
	State    string `json:"state,omitempty"`
	Postcode string `json:"postcode"`
	Country  string `json:"country"` // ISO 3166-1 alpha-2
}

func (a Address) Validate() error {
	if strings.TrimSpace(a.Line1) == "" {
		return errors.New("address line1 is required")
	}
	if strings.TrimSpace(a.City) == "" {
		return errors.New("address city is required")
	}
	if len(a.Country) != 2 {
		return errors.New("address country must be a 2-letter ISO code")
	}
	return nil
}

// InvoiceNumber is a tenant-scoped invoice identifier.
// Format is configurable per tenant (e.g., "INV-2024-00001").
type InvoiceNumber struct {
	Value string
}

func NewInvoiceNumber(value string) (InvoiceNumber, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return InvoiceNumber{}, errors.New("invoice number cannot be empty")
	}
	if len(value) > 50 {
		return InvoiceNumber{}, errors.New("invoice number must be 50 chars or less")
	}
	return InvoiceNumber{Value: value}, nil
}

func (n InvoiceNumber) String() string {
	return n.Value
}

// DateRange represents a period for recurring billing.
type DateRange struct {
	From time.Time
	To   time.Time
}

func (dr DateRange) Validate() error {
	if dr.From.IsZero() {
		return errors.New("date range 'from' is required")
	}
	if dr.To.IsZero() {
		return errors.New("date range 'to' is required")
	}
	if dr.To.Before(dr.From) {
		return errors.New("date range 'to' must be after 'from'")
	}
	return nil
}

func (dr DateRange) Contains(t time.Time) bool {
	return (t.Equal(dr.From) || t.After(dr.From)) && (t.Equal(dr.To) || t.Before(dr.To))
}