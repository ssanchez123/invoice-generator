package outbox

import (
	"context"
	"fmt"
	"time"
)

type LeaseRenewer interface {
	RenewDeliveryLease(ctx context.Context, tenantID, eventID, consumer, workerID string, lease time.Duration) error
}

// ProcessWithLease renews an active delivery lease while the handler runs.
// A failed renewal cancels the handler so it cannot continue as a stale owner.
func ProcessWithLease(
	ctx context.Context,
	renewer LeaseRenewer,
	tenantID, eventID, consumer, workerID string,
	lease time.Duration,
	process func(context.Context) error,
) error {
	if lease < 3*time.Nanosecond {
		return fmt.Errorf("lease duration must be at least 3ns")
	}

	processCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	result := make(chan error, 1)
	go func() {
		result <- process(processCtx)
	}()

	ticker := time.NewTicker(lease / 3)
	defer ticker.Stop()

	for {
		select {
		case err := <-result:
			return err
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := renewer.RenewDeliveryLease(ctx, tenantID, eventID, consumer, workerID, lease); err != nil {
				cancel()
				return fmt.Errorf("renew delivery lease: %w", err)
			}
		}
	}
}
