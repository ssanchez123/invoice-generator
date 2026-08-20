package valueobject

import (
	"errors"
	"fmt"
	"strings"
)

// Money represents a monetary amount in integer minor units (cents).
// This avoids floating-point precision issues in financial calculations.
type Money struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"` // ISO 4217 code, e.g., "USD", "EUR", "COP"
}

const (
	// Default precision: cents (2 decimal places)
	// Some currencies use 0 (JPY) or 3 (KWD) decimal places.
	// For simplicity in this showcase, we use 2 decimal places for all currencies.
)

func NewMoney(amount int64, currency string) (Money, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if len(currency) != 3 {
		return Money{}, errors.New("currency must be a 3-letter ISO 4217 code")
	}
	if amount < 0 {
		// Negative money is valid in some contexts (refunds, adjustments)
		// but we validate at the entity level for specific fields
	}
	return Money{Amount: amount, Currency: currency}, nil
}

func (m Money) Add(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, fmt.Errorf("cannot add %s to %s", other.Currency, m.Currency)
	}
	return Money{Amount: m.Amount + other.Amount, Currency: m.Currency}, nil
}

func (m Money) Subtract(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, fmt.Errorf("cannot subtract %s from %s", other.Currency, m.Currency)
	}
	return Money{Amount: m.Amount - other.Amount, Currency: m.Currency}, nil
}

func (m Money) Multiply(factor int64) Money {
	return Money{Amount: m.Amount * factor, Currency: m.Currency}
}

// MultiplyFloat multiplies by a float64 factor (e.g., tax rate 0.19 for 19%)
// Uses integer arithmetic: amount * factor / 10000 (factor in basis points)
func (m Money) MultiplyBasisPoints(basisPoints int64) Money {
	// basisPoints: 1900 = 19.00%
	return Money{Amount: (m.Amount * basisPoints) / 10000, Currency: m.Currency}
}

// Allocate splits money into n parts without losing pennies
// Uses the largest remainder method for fair distribution
func (m Money) Allocate(parts int) []Money {
	if parts <= 0 {
		return nil
	}
	base := m.Amount / int64(parts)
	remainder := int(m.Amount % int64(parts))
	result := make([]Money, parts)
	for i := 0; i < parts; i++ {
		result[i] = Money{Amount: base, Currency: m.Currency}
	}
	for i := 0; i < remainder; i++ {
		result[i].Amount++
	}
	return result
}

func (m Money) IsNegative() bool {
	return m.Amount < 0
}

func (m Money) IsZero() bool {
	return m.Amount == 0
}

func (m Money) String() string {
	abs := m.Amount
	sign := ""
	if abs < 0 {
		sign = "-"
		abs = -abs
	}
	whole := abs / 100
	frac := abs % 100
	return fmt.Sprintf("%s%d.%02d %s", sign, whole, frac, m.Currency)
}

func (m Money) Equals(other Money) bool {
	return m.Amount == other.Amount && m.Currency == other.Currency
}