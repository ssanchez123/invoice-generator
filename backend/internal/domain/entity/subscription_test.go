package entity

import (
	"testing"
	"time"
)

func TestSubscriptionAdvanceBillingDate(t *testing.T) {
	tests := []struct {
		frequency BillingFrequency
		startDate time.Time
		wantDay   int
		wantMonth int
		wantYear  int
	}{
		{FrequencyDaily, time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC), 16, 6, 2024},
		{FrequencyWeekly, time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC), 22, 6, 2024},
		{FrequencyMonthly, time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC), 15, 7, 2024},
		{FrequencyYearly, time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC), 15, 6, 2025},
	}

	for _, tt := range tests {
		t.Run(string(tt.frequency), func(t *testing.T) {
			s := &Subscription{NextBillingDate: tt.startDate, Frequency: tt.frequency}
			s.AdvanceBillingDate()
			if s.NextBillingDate.Day() != tt.wantDay || int(s.NextBillingDate.Month()) != tt.wantMonth || s.NextBillingDate.Year() != tt.wantYear {
				t.Errorf("AdvanceBillingDate(%s) = %d-%02d-%d, want %d-%02d-%d",
					tt.frequency,
					s.NextBillingDate.Year(), int(s.NextBillingDate.Month()), s.NextBillingDate.Day(),
					tt.wantYear, tt.wantMonth, tt.wantDay)
			}
		})
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