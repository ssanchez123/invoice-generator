package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/santiagossaa/invoice-generator/internal/application/command"
	"github.com/santiagossaa/invoice-generator/internal/application/outbox"
	"github.com/santiagossaa/invoice-generator/internal/config"
	"github.com/santiagossaa/invoice-generator/internal/infrastructure/repository"
)

const deliveryLeaseDuration = 5 * time.Minute

// EventDispatcher routes outbox events to their handlers.
type EventHandler interface {
	Handle(ctx context.Context, eventType string, payload []byte) error
}

// RelayWorker handles its own outbox deliveries (audit logging, notifications, etc.).
//
// Outbox Pattern (ADR-006):
// 1. Domain events written to outbox in same TX as aggregate changes
// 2. Each consumer receives an independent delivery row
// 3. This worker claims tenant-scoped deliveries using leases and SKIP LOCKED
// 4. Failed handlers retry with backoff; expired leases are reclaimed
func main() {
	log.Println("relay-worker starting...")

	cfg, err := config.LoadDatabaseConfig()
	if err != nil {
		log.Fatalf("failed to load database config: %v", err)
	}
	db, err := repository.NewDB(context.Background(), cfg.DatabaseURL())
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer db.Close()

	outboxRepo := repository.NewOutboxRepository(db)
	invoiceRepo := repository.NewInvoiceRepository(db)
	customerRepo := repository.NewCustomerRepository(db)
	subscriptionRepo := repository.NewSubscriptionRepository(db)
	tenantRepo := repository.NewTenantRepository(db)
	billingHandler := command.NewProcessDueSubscriptionsHandler(
		subscriptionRepo, customerRepo, invoiceRepo, repository.NewUnitOfWork(db),
	)
	workerID := uuid.NewString()

	// Register event handlers
	handlers := map[string]EventHandler{
		"invoice.issued":       &InvoiceIssuedHandler{},
		"invoice.paid":         &InvoicePaidHandler{},
		"payment.recorded":     &PaymentRecordedHandler{},
		"pdf.generated":        &PDFGeneratedHandler{},
		"subscription.renewed": &SubscriptionRenewedHandler{},
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	billingTicker := time.NewTicker(time.Minute)
	defer billingTicker.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sig
		log.Println("relay-worker shutting down...")
		cancel()
	}()

	log.Println("relay-worker polling outbox every 2s...")

	for {
		select {
		case <-ctx.Done():
			log.Println("relay-worker stopped")
			return
		case <-ticker.C:
			if err := processBatch(ctx, outboxRepo, handlers, workerID); err != nil {
				log.Printf("outbox processing error: %v", err)
			}
		case <-billingTicker.C:
			if err := processDueSubscriptions(ctx, tenantRepo, billingHandler); err != nil {
				log.Printf("subscription billing error: %v", err)
			}
		}
	}
}

func processDueSubscriptions(ctx context.Context, tenantRepo *repository.TenantRepository, billingHandler *command.ProcessDueSubscriptionsHandler) error {
	tenantIDs, err := tenantRepo.ListIDs(ctx)
	if err != nil {
		return fmt.Errorf("list tenants for subscription billing: %w", err)
	}
	var failures []error
	for _, tenantID := range tenantIDs {
		count, err := billingHandler.Handle(ctx, tenantID, time.Now(), 50)
		if err != nil {
			failures = append(failures, fmt.Errorf("tenant %s: %w", tenantID, err))
			continue
		}
		if count > 0 {
			log.Printf("created %d subscription draft(s) for tenant %s", count, tenantID)
		}
	}
	return errors.Join(failures...)
}

func processBatch(ctx context.Context, outboxRepo *repository.OutboxRepository, handlers map[string]EventHandler, workerID string) error {
	tenantIDs, err := outboxRepo.ListTenantIDs(ctx)
	if err != nil {
		return fmt.Errorf("list tenants: %w", err)
	}

	for _, tenantID := range tenantIDs {
		for i := 0; i < 50; i++ {
			entries, err := outboxRepo.ClaimDeliveries(ctx, tenantID, "relay-worker", workerID, 1, deliveryLeaseDuration)
			if err != nil {
				return fmt.Errorf("claim relay deliveries for tenant %s: %w", tenantID, err)
			}
			if len(entries) == 0 {
				break
			}

			entry := entries[0]
			handler, ok := handlers[entry.EventType]
			if !ok {
				err = fmt.Errorf("no relay handler for event type %s", entry.EventType)
			} else {
				err = outbox.ProcessWithLease(ctx, outboxRepo, tenantID, entry.ID, "relay-worker", workerID, deliveryLeaseDuration, func(handlerCtx context.Context) error {
					return handler.Handle(handlerCtx, entry.EventType, entry.Payload)
				})
			}
			if err != nil {
				log.Printf("handler error for %s (entry %s, attempt %d): %v", entry.EventType, entry.ID, entry.Attempts, err)
				if retryErr := outboxRepo.RetryDelivery(ctx, tenantID, entry.ID, "relay-worker", workerID, err); retryErr != nil {
					log.Printf("failed to release relay delivery %s: %v", entry.ID, retryErr)
				}
				continue
			}

			if err := outboxRepo.CompleteDelivery(ctx, tenantID, entry.ID, "relay-worker", workerID); err != nil {
				log.Printf("failed to complete relay delivery %s: %v", entry.ID, err)
			}
		}
	}

	return nil
}

// ========== Event Handlers ==========

// InvoiceIssuedHandler records the issue event independently of PDF generation.
type InvoiceIssuedHandler struct{}

func (h *InvoiceIssuedHandler) Handle(ctx context.Context, eventType string, payload []byte) error {
	var event struct {
		InvoiceID string `json:"InvoiceID"`
		TenantID  string `json:"TenantID"`
		IssuedAt  string `json:"IssuedAt"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("unmarshal invoice.issued: %w", err)
	}

	log.Printf("invoice.issued: invoice=%s tenant=%s", event.InvoiceID, event.TenantID)
	return nil
}

// InvoicePaidHandler records audit + sends notification when invoice is fully paid.
type InvoicePaidHandler struct{}

func (h *InvoicePaidHandler) Handle(ctx context.Context, eventType string, payload []byte) error {
	var event struct {
		InvoiceID string `json:"InvoiceID"`
		TenantID  string `json:"TenantID"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("unmarshal invoice.paid: %w", err)
	}

	log.Printf("invoice.paid: invoice=%s tenant=%s — sending notification", event.InvoiceID, event.TenantID)
	// In real impl: send email notification, update reporting, etc.
	return nil
}

// PaymentRecordedHandler writes audit entry for any payment.
type PaymentRecordedHandler struct{}

func (h *PaymentRecordedHandler) Handle(ctx context.Context, eventType string, payload []byte) error {
	var event struct {
		InvoiceID string `json:"InvoiceID"`
		TenantID  string `json:"TenantID"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("unmarshal payment.recorded: %w", err)
	}

	log.Printf("payment.recorded: invoice=%s tenant=%s", event.InvoiceID, event.TenantID)
	// In real impl: write audit entry
	return nil
}

// PDFGeneratedHandler updates invoice metadata with PDF artifact URL.
type PDFGeneratedHandler struct{}

func (h *PDFGeneratedHandler) Handle(ctx context.Context, eventType string, payload []byte) error {
	var event struct {
		InvoiceID   string `json:"InvoiceID"`
		ArtifactURL string `json:"ArtifactURL"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("unmarshal pdf.generated: %w", err)
	}

	log.Printf("pdf.generated: invoice=%s artifact=%s", event.InvoiceID, event.ArtifactURL)
	// In real impl: update invoice.metadata with pdf_url
	return nil
}

// SubscriptionRenewedHandler writes audit for recurring billing.
type SubscriptionRenewedHandler struct{}

func (h *SubscriptionRenewedHandler) Handle(ctx context.Context, eventType string, payload []byte) error {
	var event struct {
		SubscriptionID string `json:"SubscriptionID"`
		InvoiceID      string `json:"InvoiceID"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("unmarshal subscription.renewed: %w", err)
	}

	log.Printf("subscription.renewed: sub=%s invoice=%s", event.SubscriptionID, event.InvoiceID)
	return nil
}
