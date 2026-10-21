package outbox

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestProcessWithLeaseRenewsWhileHandlerRuns(t *testing.T) {
	renewer := &testLeaseRenewer{renewed: make(chan struct{}, 1)}
	release := make(chan struct{})

	done := make(chan error, 1)
	go func() {
		done <- ProcessWithLease(
			context.Background(), renewer, "tenant", "event", "consumer", "worker",
			60*time.Millisecond,
			func(ctx context.Context) error {
				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			},
		)
	}()

	select {
	case <-renewer.renewed:
	case <-time.After(time.Second):
		t.Fatal("handler lease was not renewed")
	}

	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("process with lease: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("handler did not finish")
	}
}

func TestProcessWithLeaseCancelsHandlerWhenRenewalFails(t *testing.T) {
	wantErr := errors.New("lease lost")
	renewer := &testLeaseRenewer{err: wantErr}
	handlerCancelled := make(chan struct{})

	err := ProcessWithLease(
		context.Background(), renewer, "tenant", "event", "consumer", "worker",
		60*time.Millisecond,
		func(ctx context.Context) error {
			<-ctx.Done()
			close(handlerCancelled)
			return ctx.Err()
		},
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected renewal error %v, got %v", wantErr, err)
	}
	select {
	case <-handlerCancelled:
	case <-time.After(time.Second):
		t.Fatal("handler was not cancelled after lease renewal failed")
	}
}

type testLeaseRenewer struct {
	renewed chan struct{}
	err     error
}

func (r *testLeaseRenewer) RenewDeliveryLease(context.Context, string, string, string, string, time.Duration) error {
	if r.renewed != nil {
		select {
		case r.renewed <- struct{}{}:
		default:
		}
	}
	return r.err
}
