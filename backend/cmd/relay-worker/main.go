package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/santiagossaa/invoice-generator/internal/infrastructure/repository"
)

// EventDispatcher routes outbox events to their handlers.
type EventHandler interface {
	Handle(ctx context.Context, eventType string, payload []byte) error
}

// RelayWorker polls the outbox table for unprocessed domain events
// and dispatches them to event handlers (PDF generation, audit logging, etc.)
//
// Outbox Pattern (ADR-006):
// 1. Domain events written to outbox in same TX as aggregate changes
// 2. This worker polls for unprocessed entries using SELECT FOR UPDATE SKIP LOCKED
// 3. For each entry, routes to the appropriate handler
// 4. On success, marks entry as processed
// 5. Multiple worker instances run safely (SKIP LOCKED prevents duplicate processing)
func main() {
	log.Println("relay-worker starting...")

	dbURL := repository.DatabaseURL()
	db, err := repository.NewDB(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer db.Close()

	outboxRepo := repository.NewOutboxRepository(db)

	// Register event handlers
	handlers := map[string]EventHandler{
		"invoice.issued":      &InvoiceIssuedHandler{db: db},
		"invoice.paid":         &InvoicePaidHandler{},
		"payment.recorded":     &PaymentRecordedHandler{},
		"pdf.generated":        &PDFGeneratedHandler{},
		"subscription.renewed": &SubscriptionRenewedHandler{},
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

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
			if err := processBatch(ctx, outboxRepo, handlers); err != nil {
				log.Printf("outbox processing error: %v", err)
			}
		}
	}
}

func processBatch(ctx context.Context, outboxRepo *repository.OutboxRepository, handlers map[string]EventHandler) error {
	entries, err := outboxRepo.FindUnprocessed(ctx, 50)
	if err != nil {
		return fmt.Errorf("find unprocessed: %w", err)
	}

	if len(entries) == 0 {
		return nil
	}

	log.Printf("processing %d outbox entries...", len(entries))

	for _, entry := range entries {
		handler, ok := handlers[entry.EventType]
		if !ok {
			log.Printf("no handler for event type %s, marking as processed", entry.EventType)
			outboxRepo.MarkProcessed(ctx, entry.ID)
			continue
		}

		if err := handler.Handle(ctx, entry.EventType, entry.Payload); err != nil {
			log.Printf("handler error for %s (entry %s): %v — will retry", entry.EventType, entry.ID, err)
			continue // Don't mark as processed — will retry next cycle
		}

		if err := outboxRepo.MarkProcessed(ctx, entry.ID); err != nil {
			log.Printf("failed to mark outbox entry %s as processed: %v", entry.ID, err)
		}
	}

	return nil
}

// ========== Event Handlers ==========

// InvoiceIssuedHandler triggers PDF generation when an invoice is issued.
type InvoiceIssuedHandler struct {
	db *repository.DB
}

func (h *InvoiceIssuedHandler) Handle(ctx context.Context, eventType string, payload []byte) error {
	var event struct {
		InvoiceID string `json:"InvoiceID"`
		TenantID  string `json:"TenantID"`
		IssuedAt  string `json:"IssuedAt"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("unmarshal invoice.issued: %w", err)
	}

	log.Printf("invoice.issued: invoice=%s tenant=%s — triggering PDF generation", event.InvoiceID, event.TenantID)

	// In a real implementation:
	// 1. Load invoice from DB
	// 2. Call PDF service (HTTP or direct function call)
	// 3. Store artifact and emit PDFGenerated event
	// For now, we log it

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