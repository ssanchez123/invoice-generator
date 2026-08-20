package valueobject

import (
	"testing"
	"time"
)

func TestTaxRuleValidate(t *testing.T) {
	validRule := TaxRule{
		Jurisdiction:  "CO",
		RateBPS:       1900,
		EffectiveFrom: time.Now(),
	}
	if err := validRule.Validate(); err != nil {
		t.Errorf("valid rule failed: %v", err)
	}

	tests := []struct {
		name string
		rule TaxRule
	}{
		{"empty jurisdiction", TaxRule{Jurisdiction: "", RateBPS: 1900, EffectiveFrom: time.Now()}},
		{"1-letter jurisdiction", TaxRule{Jurisdiction: "C", RateBPS: 1900, EffectiveFrom: time.Now()}},
		{"negative rate", TaxRule{Jurisdiction: "CO", RateBPS: -1, EffectiveFrom: time.Now()}},
		{"rate over 100%", TaxRule{Jurisdiction: "CO", RateBPS: 10001, EffectiveFrom: time.Now()}},
		{"zero effective_from", TaxRule{Jurisdiction: "CO", RateBPS: 1900, EffectiveFrom: time.Time{}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.rule.Validate(); err == nil {
				t.Errorf("expected error for %s", tt.name)
			}
		})
	}
}

func TestTaxRuleIsActiveAt(t *testing.T) {
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)

	rule := TaxRule{
		Jurisdiction:  "CO",
		RateBPS:       1900,
		EffectiveFrom: from,
		EffectiveTo:   &to,
	}

	tests := []struct {
		name string
		at   time.Time
		want bool
	}{
		{"before effective", time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC), false},
		{"at effective start", from, true},
		{"mid-range", time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), true},
		{"at effective end", to, true},
		{"after effective", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rule.IsActiveAt(tt.at)
			if got != tt.want {
				t.Errorf("IsActiveAt() = %v, want %v", got, tt.want)
			}
		})
	}

	// Rule with no effective_to (indefinite)
	openRule := TaxRule{
		Jurisdiction:  "US",
		RateBPS:       700,
		EffectiveFrom: from,
	}
	if !openRule.IsActiveAt(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Error("open-ended rule should be active in 2030")
	}
}

func TestAddressValidate(t *testing.T) {
	valid := Address{Line1: "123 Main St", City: "Bogota", Country: "CO"}
	if err := valid.Validate(); err != nil {
		t.Errorf("valid address failed: %v", err)
	}

	tests := []struct {
		name string
		addr Address
	}{
		{"empty line1", Address{City: "Bogota", Country: "CO"}},
		{"empty city", Address{Line1: "123 Main St", Country: "CO"}},
		{"invalid country", Address{Line1: "123 Main St", City: "Bogota", Country: "COL"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.addr.Validate(); err == nil {
				t.Errorf("expected error for %s", tt.name)
			}
		})
	}
}

func TestInvoiceNumber(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		n, err := NewInvoiceNumber("INV-2024-00001")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n.String() != "INV-2024-00001" {
			t.Errorf("String() = %q, want INV-2024-00001", n.String())
		}
	})

	t.Run("empty", func(t *testing.T) {
		_, err := NewInvoiceNumber("")
		if err == nil {
			t.Error("empty invoice number should error")
		}
	})

	t.Run("too long", func(t *testing.T) {
		long := string(make([]byte, 51))
		_, err := NewInvoiceNumber(long)
		if err == nil {
			t.Error("51-char invoice number should error")
		}
	})
}

func TestDateRangeValidate(t *testing.T) {
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)

	t.Run("valid", func(t *testing.T) {
		dr := DateRange{From: from, To: to}
		if err := dr.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("to before from", func(t *testing.T) {
		dr := DateRange{From: to, To: from}
		if err := dr.Validate(); err == nil {
			t.Error("to before from should error")
		}
	})

	t.Run("zero from", func(t *testing.T) {
		dr := DateRange{To: to}
		if err := dr.Validate(); err == nil {
			t.Error("zero from should error")
		}
	})
}

func TestDateRangeContains(t *testing.T) {
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	dr := DateRange{From: from, To: to}

	mid := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	before := time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC)
	after := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	if !dr.Contains(mid) {
		t.Error("mid should be contained")
	}
	if dr.Contains(before) {
		t.Error("before should not be contained")
	}
	if dr.Contains(after) {
		t.Error("after should not be contained")
	}
	if !dr.Contains(from) {
		t.Error("from boundary should be contained")
	}
}
