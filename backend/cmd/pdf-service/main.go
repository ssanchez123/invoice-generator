package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/domain/port"
	"github.com/santiagossaa/invoice-generator/internal/infrastructure/repository"
)

// PDFService generates invoice PDFs as a separate service (ADR-007).
//
// It polls for "invoice.issued" events from the outbox and generates PDFs.
// The PDF is rendered from an HTML template and stored on disk (S3-compatible in prod).
//
// Why separate service:
// - PDF rendering is CPU-intensive (layout, fonts)
// - Isolation allows independent scaling
// - Failed jobs retry via outbox without blocking the API
func main() {
	log.Println("pdf-service starting...")

	storagePath := os.Getenv("PDF_STORAGE_PATH")
	if storagePath == "" {
		storagePath = "/tmp/pdfs"
	}
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		log.Fatalf("failed to create storage dir %s: %v", storagePath, err)
	}

	dbURL := repository.DatabaseURL()
	db, err := repository.NewDB(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer db.Close()

	outboxRepo := repository.NewOutboxRepository(db)

	srv := &PDFService{
		storagePath: storagePath,
		db:          db,
		outboxRepo:  outboxRepo,
		templates:   template.Must(template.New("invoice").Parse(invoiceHTMLTemplate)),
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

	log.Printf("pdf-service polling every 3s, storage: %s", storagePath)

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
	storagePath string
	db          *repository.DB
	outboxRepo  *repository.OutboxRepository
	templates   *template.Template
}

func (s *PDFService) processJobs(ctx context.Context) error {
	// Find outbox entries for "invoice.issued" events
	entries, err := s.outboxRepo.FindUnprocessed(ctx, 20)
	if err != nil {
		return fmt.Errorf("find unprocessed: %w", err)
	}

	for _, entry := range entries {
		if entry.EventType != "invoice.issued" {
			continue
		}

		if err := s.generateInvoicePDF(ctx, entry); err != nil {
			log.Printf("pdf generation failed for %s: %v", entry.AggregateID, err)
			continue // Will retry next cycle
		}

		s.outboxRepo.MarkProcessed(ctx, entry.ID)
	}

	return nil
}

func (s *PDFService) generateInvoicePDF(ctx context.Context, entry port.OutboxEntry) error {
	var event struct {
		InvoiceID string    `json:"InvoiceID"`
		TenantID  string    `json:"TenantID"`
		IssuedAt  time.Time `json:"IssuedAt"`
	}
	if err := json.Unmarshal(entry.Payload, &event); err != nil {
		return fmt.Errorf("unmarshal event: %w", err)
	}

	// In a real implementation:
	// 1. Load invoice + customer + tenant from DB
	// 2. Build the invoice data model for the template
	// 3. Render HTML template
	// 4. Convert HTML to PDF (using wkhtmltopdf, chromedp, or go-pdf)
	// 5. Store artifact: {storagePath}/{tenantID}/{invoiceID}.pdf
	// 6. Emit PDFGenerated event back to outbox

	// For scaffold: render a simple HTML template
	var buf bytes.Buffer
	data := map[string]any{
		"InvoiceID": event.InvoiceID,
		"TenantID":  event.TenantID,
		"IssuedAt":  event.IssuedAt.Format("2006-01-02"),
	}

	if err := s.templates.Execute(&buf, data); err != nil {
		return fmt.Errorf("render template: %w", err)
	}

	// Save HTML as placeholder (real impl would convert to PDF)
	tenantDir := fmt.Sprintf("%s/%s", s.storagePath, event.TenantID)
	if err := os.MkdirAll(tenantDir, 0755); err != nil {
		return fmt.Errorf("create tenant dir: %w", err)
	}

	filePath := fmt.Sprintf("%s/%s.html", tenantDir, event.InvoiceID)
	if err := os.WriteFile(filePath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	artifactURL := fmt.Sprintf("/pdfs/%s/%s.pdf", event.TenantID, event.InvoiceID)
	log.Printf("PDF generated: invoice=%s → %s", event.InvoiceID, filePath)

	// Emit PDFGenerated event
	pdfEvent := struct {
		InvoiceID   string    `json:"InvoiceID"`
		TenantID    string    `json:"TenantID"`
		ArtifactURL string    `json:"ArtifactURL"`
		GeneratedAt time.Time `json:"GeneratedAt"`
	}{
		InvoiceID:   event.InvoiceID,
		TenantID:    event.TenantID,
		ArtifactURL: artifactURL,
		GeneratedAt: time.Now(),
	}

	payload, _ := json.Marshal(pdfEvent)
	ev := domainEvent{
		name:        "pdf.generated",
		aggregateID: event.InvoiceID,
		tenantID:    event.TenantID,
		payload:     payload,
	}
	_ = s.outboxRepo.SaveEvents(ctx, event.TenantID, []entity.DomainEvent{ev})

	return nil
}

// domainEvent is a local adapter to satisfy the entity.DomainEvent interface
// without importing the full domain package (keeps the service decoupled).
type domainEvent struct {
	name        string
	aggregateID string
	tenantID    string
	payload     []byte
}

func (e domainEvent) EventName() string     { return e.name }
func (e domainEvent) AggregateID() string   { return e.aggregateID }
func (e domainEvent) TenantID() string      { return e.tenantID }
func (e domainEvent) OccurredAt() time.Time { return time.Now() }

// invoiceHTMLTemplate is the HTML template for invoice PDFs.
// In a real implementation, this would be a more sophisticated template
// with proper CSS, fonts, QR codes, etc.
const invoiceHTMLTemplate = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
  body { font-family: Arial, sans-serif; margin: 40px; }
  .header { display: flex; justify-content: space-between; border-bottom: 2px solid #333; padding-bottom: 20px; }
  .invoice-title { font-size: 24px; font-weight: bold; }
  .invoice-meta { text-align: right; color: #666; }
  table { width: 100%; border-collapse: collapse; margin-top: 20px; }
  th { background: #f5f5f5; padding: 8px; text-align: left; }
  td { padding: 8px; border-bottom: 1px solid #eee; }
  .total { font-size: 18px; font-weight: bold; text-align: right; margin-top: 20px; }
</style>
</head>
<body>
  <div class="header">
    <div class="invoice-title">INVOICE</div>
    <div class="invoice-meta">
      <div>Invoice ID: {{.InvoiceID}}</div>
      <div>Date: {{.IssuedAt}}</div>
    </div>
  </div>
  <p style="margin-top: 20px; color: #999;">
    This is a scaffold PDF. In production, this would include full invoice details,
    line items, tax breakdown, customer info, and payment terms.
  </p>
  <div class="total">Total: —</div>
</body>
</html>`
