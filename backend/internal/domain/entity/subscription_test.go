package entity

import (
	"testing"
	"time"

	"github.com/santiagossaa/invoice-generator/internal/domain/valueobject"
)

func TestSubscriptionAdvanceBillingDate(t *testing.T) {
	tests := []struct {
		frequency BillingFrequency
		startDate time.Time
		wantDay   int
		wantMonth int
		wantYear  int
	}{
		{FrequencyDaily, time.Date(2024, 6, 15, 0, 0, 0, 0, colombiaBillingLocation), 16, 6, 2024},
		{FrequencyWeekly, time.Date(2024, 6, 15, 0, 0, 0, 0, colombiaBillingLocation), 22, 6, 2024},
		{FrequencyMonthly, time.Date(2024, 6, 15, 0, 0, 0, 0, colombiaBillingLocation), 15, 7, 2024},
		{FrequencyYearly, time.Date(2024, 6, 15, 0, 0, 0, 0, colombiaBillingLocation), 15, 6, 2025},
	}

	for _, tt := range tests {
		t.Run(string(tt.frequency), func(t *testing.T) {
			s := &Subscription{NextBillingDate: tt.startDate, Frequency: tt.frequency}
			s.AdvanceBillingDate()
			billingDate := s.NextBillingDate.In(colombiaBillingLocation)
			if billingDate.Day() != tt.wantDay || int(billingDate.Month()) != tt.wantMonth || billingDate.Year() != tt.wantYear {
				t.Errorf("AdvanceBillingDate(%s) = %d-%02d-%d, want %d-%02d-%d",
					tt.frequency,
					billingDate.Year(), int(billingDate.Month()), billingDate.Day(),
					tt.wantYear, tt.wantMonth, tt.wantDay)
			}
		})
	}
}

func TestSubscriptionAdvanceBillingDateClampsAndRestoresBillingDay(t *testing.T) {
	subscription := &Subscription{
		Frequency:       FrequencyMonthly,
		NextBillingDate: time.Date(2024, time.May, 31, 9, 0, 0, 0, time.UTC),
		BillingDay:      31,
	}
	subscription.AdvanceBillingDate()
	if got := subscription.NextBillingDate; got.Day() != 30 || got.Month() != time.June {
		t.Fatalf("first advance = %s, want June 30", got)
	}
	subscription.AdvanceBillingDate()
	if got := subscription.NextBillingDate; got.Day() != 31 || got.Month() != time.July {
		t.Fatalf("second advance = %s, want July 31", got)
	}
}

func TestNewSubscriptionUsesColombiaBillingDayAndKeepsLocalDate(t *testing.T) {
	nextBillingDate := time.Date(2024, time.June, 16, 4, 59, 59, 0, time.UTC)
	subscription, err := NewSubscription(
		"subscription-1", "tenant-1", "customer-1",
		[]InvoiceItem{{
			ID:          "item-1",
			Description: "Servicio",
			Quantity:    1,
			UnitPrice:   valueobject.Money{Amount: 100, Currency: "COP"},
		}},
		FrequencyMonthly, nextBillingDate,
	)
	if err != nil {
		t.Fatalf("NewSubscription() error = %v", err)
	}
	if subscription.BillingDay != 15 {
		t.Fatalf("BillingDay = %d, want 15 in Colombia time", subscription.BillingDay)
	}
	subscription.AdvanceBillingDate()
	localDate := subscription.NextBillingDate.In(time.FixedZone("COT", -5*60*60))
	if localDate.Day() != 15 || localDate.Month() != time.July {
		t.Fatalf("next billing date in COT = %s, want July 15", localDate)
	}
}

func TestSubscriptionPause(t *testing.T) {
	t.Run("from active", func(t *testing.T) {
		s := &Subscription{Status: SubscriptionStatusActive}
		err := s.Pause()
		if err != nil {
			t.Fatalf("Pause failed: %v", err)
		}
		if s.Status != SubscriptionStatusPaused {
			t.Errorf("Status = %s, want paused", s.Status)
		}
	})

	t.Run("from cancelled", func(t *testing.T) {
		s := &Subscription{Status: SubscriptionStatusCancelled}
		err := s.Pause()
		if err == nil {
			t.Error("Pause from cancelled should error")
		}
	})
}

func TestSubscriptionResume(t *testing.T) {
	t.Run("from paused", func(t *testing.T) {
		s := &Subscription{Status: SubscriptionStatusPaused}
		err := s.Resume()
		if err != nil {
			t.Fatalf("Resume failed: %v", err)
		}
		if s.Status != SubscriptionStatusActive {
			t.Errorf("Status = %s, want active", s.Status)
		}
	})

	t.Run("from active", func(t *testing.T) {
		s := &Subscription{Status: SubscriptionStatusActive}
		err := s.Resume()
		if err == nil {
			t.Error("Resume from active should error")
		}
	})
}

func TestSubscriptionCancel(t *testing.T) {
	t.Run("from active", func(t *testing.T) {
		s := &Subscription{Status: SubscriptionStatusActive}
		err := s.Cancel()
		if err != nil {
			t.Fatalf("Cancel failed: %v", err)
		}
		if s.Status != SubscriptionStatusCancelled {
			t.Errorf("Status = %s, want cancelled", s.Status)
		}
	})

	t.Run("from cancelled", func(t *testing.T) {
		s := &Subscription{Status: SubscriptionStatusCancelled}
		err := s.Cancel()
		if err == nil {
			t.Error("Cancel from cancelled should error")
		}
	})
}
