package valueobject

import (
	"testing"
)

func TestNewMoney(t *testing.T) {
	tests := []struct {
		name     string
		amount   int64
		currency string
		wantErr  bool
	}{
		{"valid USD", 1000, "USD", false},
		{"valid EUR lowercase", 500, "eur", false},
		{"valid with spaces", 100, " COP ", false},
		{"invalid 2-letter", 100, "US", true},
		{"invalid 4-letter", 100, "USDX", true},
		{"empty currency", 100, "", true},
		{"negative amount", -500, "USD", false}, // negative is valid
		{"zero amount", 0, "USD", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewMoney(tt.amount, tt.currency)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewMoney(%d, %q) expected error, got none", tt.amount, tt.currency)
				}
				return
			}
			if err != nil {
				t.Errorf("NewMoney(%d, %q) unexpected error: %v", tt.amount, tt.currency, err)
				return
			}
			if m.Amount != tt.amount {
				t.Errorf("Amount = %d, want %d", m.Amount, tt.amount)
			}
		})
	}
}

func TestMoneyAdd(t *testing.T) {
	t.Run("same currency", func(t *testing.T) {
		a := Money{Amount: 1000, Currency: "USD"}
		b := Money{Amount: 500, Currency: "USD"}
		result, err := a.Add(b)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Amount != 1500 {
			t.Errorf("Add() = %d, want 1500", result.Amount)
		}
	})

	t.Run("different currency", func(t *testing.T) {
		a := Money{Amount: 1000, Currency: "USD"}
		b := Money{Amount: 500, Currency: "EUR"}
		_, err := a.Add(b)
		if err == nil {
			t.Error("Add() different currencies should error")
		}
	})
}

func TestMoneySubtract(t *testing.T) {
	t.Run("same currency", func(t *testing.T) {
		a := Money{Amount: 1000, Currency: "USD"}
		b := Money{Amount: 300, Currency: "USD"}
		result, err := a.Subtract(b)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Amount != 700 {
			t.Errorf("Subtract() = %d, want 700", result.Amount)
		}
	})

	t.Run("different currency", func(t *testing.T) {
		a := Money{Amount: 1000, Currency: "USD"}
		b := Money{Amount: 300, Currency: "EUR"}
		_, err := a.Subtract(b)
		if err == nil {
			t.Error("Subtract() different currencies should error")
		}
	})

	t.Run("result negative", func(t *testing.T) {
		a := Money{Amount: 300, Currency: "USD"}
		b := Money{Amount: 500, Currency: "USD"}
		result, err := a.Subtract(b)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsNegative() {
			t.Error("Subtract() should be negative")
		}
	})
}

func TestMoneyMultiply(t *testing.T) {
	m := Money{Amount: 1000, Currency: "USD"}
	result := m.Multiply(3)
	if result.Amount != 3000 {
		t.Errorf("Multiply(3) = %d, want 3000", result.Amount)
	}
}

func TestMoneyMultiplyBasisPoints(t *testing.T) {
	t.Run("19% tax", func(t *testing.T) {
		m := Money{Amount: 10000, Currency: "USD"} // $100.00
		tax := m.MultiplyBasisPoints(1900)          // 19%
		if tax.Amount != 1900 {
			t.Errorf("MultiplyBasisPoints(1900) = %d, want 1900 ($19.00)", tax.Amount)
		}
	})

	t.Run("0% tax", func(t *testing.T) {
		m := Money{Amount: 10000, Currency: "USD"}
		tax := m.MultiplyBasisPoints(0)
		if tax.Amount != 0 {
			t.Errorf("MultiplyBasisPoints(0) = %d, want 0", tax.Amount)
		}
	})

	t.Run("10% tax", func(t *testing.T) {
		m := Money{Amount: 5000, Currency: "USD"} // $50.00
		tax := m.MultiplyBasisPoints(1000)        // 10%
		if tax.Amount != 500 {
			t.Errorf("MultiplyBasisPoints(1000) = %d, want 500 ($5.00)", tax.Amount)
		}
	})
}

func TestMoneyAllocate(t *testing.T) {
	t.Run("even split", func(t *testing.T) {
		m := Money{Amount: 1000, Currency: "USD"} // $10.00
		parts := m.Allocate(3)
		if len(parts) != 3 {
			t.Fatalf("Allocate(3) returned %d parts, want 3", len(parts))
		}
		total := int64(0)
		for _, p := range parts {
			total += p.Amount
		}
		if total != 1000 {
			t.Errorf("Allocate(3) total = %d, want 1000", total)
		}
		// Fair distribution: 334, 333, 333
		if parts[0].Amount != 334 || parts[1].Amount != 333 || parts[2].Amount != 333 {
			t.Errorf("Allocate(3) = [%d, %d, %d], want [334, 333, 333]",
				parts[0].Amount, parts[1].Amount, parts[2].Amount)
		}
	})

	t.Run("zero parts", func(t *testing.T) {
		m := Money{Amount: 1000, Currency: "USD"}
		parts := m.Allocate(0)
		if parts != nil {
			t.Error("Allocate(0) should return nil")
		}
	})
}

func TestMoneyString(t *testing.T) {
	tests := []struct {
		m    Money
		want string
	}{
		{Money{Amount: 1099, Currency: "USD"}, "10.99 USD"},
		{Money{Amount: 0, Currency: "EUR"}, "0.00 EUR"},
		{Money{Amount: -500, Currency: "USD"}, "-5.00 USD"},
		{Money{Amount: 100, Currency: "COP"}, "1.00 COP"},
	}

	for _, tt := range tests {
		got := tt.m.String()
		if got != tt.want {
			t.Errorf("String() = %q, want %q", got, tt.want)
		}
	}
}

func TestMoneyEquals(t *testing.T) {
	a := Money{Amount: 1000, Currency: "USD"}
	b := Money{Amount: 1000, Currency: "USD"}
	c := Money{Amount: 1000, Currency: "EUR"}
	d := Money{Amount: 2000, Currency: "USD"}

	if !a.Equals(b) {
		t.Error("a should equal b")
	}
	if a.Equals(c) {
		t.Error("a should not equal c (different currency)")
	}
	if a.Equals(d) {
		t.Error("a should not equal d (different amount)")
	}
}

func TestMoneyIsNegative(t *testing.T) {
	neg := Money{Amount: -1, Currency: "USD"}
	if !neg.IsNegative() {
		t.Error("-1 should be negative")
	}
	zero := Money{Amount: 0, Currency: "USD"}
	if zero.IsNegative() {
		t.Error("0 should not be negative")
	}
	pos := Money{Amount: 1, Currency: "USD"}
	if pos.IsNegative() {
		t.Error("1 should not be negative")
	}
}

func TestMoneyIsZero(t *testing.T) {
	zero := Money{Amount: 0, Currency: "USD"}
	if !zero.IsZero() {
		t.Error("0 should be zero")
	}
	pos := Money{Amount: 1, Currency: "USD"}
	if pos.IsZero() {
		t.Error("1 should not be zero")
	}
}