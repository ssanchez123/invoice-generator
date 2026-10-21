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

	"github.com/google/uuid"

	"github.com/santiagossaa/invoice-generator/internal/application/outbox"
	pdfapp "github.com/santiagossaa/invoice-generator/internal/application/pdf"
	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/domain/port"
	pdfrenderer "github.com/santiagossaa/invoice-generator/internal/infrastructure/pdf"
	"github.com/santiagossaa/invoice-generator/internal/infrastructure/repository"
	"github.com/santiagossaa/invoice-generator/internal/infrastructure/storage"
)

const deliveryLeaseDuration = 5 * time.Minute

// PDFService generates invoice PDFs as a separate service (ADR-007).
//
// It polls its independent "invoice.issued" deliveries and generates artifacts.
// The PDF is rendered from an HTML template and stored on disk (S3-compatible in prod).
//
// Why separate service:
// - PDF rendering is CPU-intensive (layout, fonts)
// - Isolation allows independent scaling
// - Failed jobs retry via outbox without blocking the API
func main() {
	log.Println("pdf-service starting...")

	dbURL := repository.DatabaseURL()
	db, err := repository.NewDB(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer db.Close()

	pdfStore, err := storage.NewPDFArtifactStoreFromEnv(context.Background())
	if err != nil {
		log.Fatalf("failed to configure PDF storage: %v", err)
	}
	pdfGenerator := pdfapp.NewGenerator(
		repository.NewInvoiceRepository(db),
		repository.NewCustomerRepository(db),
		repository.NewTenantRepository(db),
		pdfrenderer.NewGoFPDFRenderer(),
		pdfStore,
	)
	outboxRepo := repository.NewOutboxRepository(db)

	srv := &PDFService{
		generator:  pdfGenerator,
		outboxRepo: outboxRepo,
		workerID:   uuid.NewString(),
	}

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sig
		log.Println("pdf-service shutting down...")
		cancel()
	}()

	log.Println("pdf-service polling every 3s")

	for {
		select {
		case <-ctx.Done():
			log.Println("pdf-service stopped")
			return
		case <-ticker.C:
			if err := srv.processJobs(ctx); err != nil {
				log.Printf("pdf job error: %v", err)
			}
		}
	}
}

// PDFService handles PDF generation.
type PDFService struct {
	generator  *pdfapp.Generator
	outboxRepo *repository.OutboxRepository
	workerID   string
}

func (s *PDFService) processJobs(ctx context.Context) error {
	tenantIDs, err := s.outboxRepo.ListTenantIDs(ctx)
	if err != nil {
		return fmt.Errorf("list tenants: %w", err)
	}

	for _, tenantID := range tenantIDs {
		for i := 0; i < 20; i++ {
			entries, err := s.outboxRepo.ClaimDeliveries(ctx, tenantID, "pdf-service", s.workerID, 1, deliveryLeaseDuration)
			if err != nil {
				return fmt.Errorf("claim PDF deliveries for tenant %s: %w", tenantID, err)
			}
			if len(entries) == 0 {
				break
			}

			entry := entries[0]
			err = outbox.ProcessWithLease(ctx, s.outboxRepo, tenantID, entry.ID, "pdf-service", s.workerID, deliveryLeaseDuration, func(handlerCtx context.Context) error {
				return s.generateInvoicePDF(handlerCtx, entry)
			})
			if err != nil {
				log.Printf("pdf generation failed for %s (attempt %d): %v", entry.AggregateID, entry.Attempts, err)
				if retryErr := s.outboxRepo.RetryDelivery(ctx, tenantID, entry.ID, "pdf-service", s.workerID, err); retryErr != nil {
					log.Printf("failed to release PDF delivery %s: %v", entry.ID, retryErr)
				}
				continue
			}

			if err := s.outboxRepo.CompleteDelivery(ctx, tenantID, entry.ID, "pdf-service", s.workerID); err != nil {
				log.Printf("failed to complete PDF delivery %s: %v", entry.ID, err)
			}
		}
	}

	return nil
}

func (s *PDFService) generateInvoicePDF(ctx context.Context, entry port.OutboxEntry) error {
	if err := s.generator.Generate(ctx, entry.TenantID, entry.AggregateID); err != nil {
		return fmt.Errorf("generate invoice PDF: %w", err)
	}

	artifactURL := fmt.Sprintf("/api/v1/invoices/%s/pdf", entry.AggregateID)
	log.Printf("PDF generated: invoice=%s tenant=%s", entry.AggregateID, entry.TenantID)

	// Emit PDFGenerated event
	pdfEvent := generatedPDFEvent{
		Invoice:     entry.AggregateID,
		Tenant:      entry.TenantID,
		ArtifactURL: artifactURL,
		GeneratedAt: time.Now(),
	}
	ev := domainEvent{
		event: pdfEvent,
	}
	if err := s.outboxRepo.SaveEvents(ctx, entry.TenantID, []entity.DomainEvent{ev}); err != nil {
		return fmt.Errorf("save pdf.generated event: %w", err)
	}

	return nil
}

type generatedPDFEvent struct {
	Invoice     string    `json:"InvoiceID"`
	Tenant      string    `json:"TenantID"`
	ArtifactURL string    `json:"ArtifactURL"`
	GeneratedAt time.Time `json:"GeneratedAt"`
}

// domainEvent adapts the generated event to the domain event interface.
type domainEvent struct {
	event generatedPDFEvent
}

func (e domainEvent) EventName() string     { return "pdf.generated" }
func (e domainEvent) AggregateID() string   { return e.event.Invoice }
func (e domainEvent) TenantID() string      { return e.event.Tenant }
func (e domainEvent) OccurredAt() time.Time { return e.event.GeneratedAt }

func (e domainEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.event)
}
