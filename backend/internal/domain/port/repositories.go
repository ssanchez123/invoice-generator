package port

import (
	"context"
	"time"

	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
)

// InvoiceRepository is the port (interface) for invoice persistence.
// Infrastructure layer provides the adapter (PostgreSQL implementation).
type InvoiceRepository interface {
	// Save persists an invoice and its domain events (in a transaction).
	// Events are written to the outbox table atomically.
	Save(ctx context.Context, invoice *entity.Invoice) error

	// FindByID loads an invoice aggregate by ID.
	// Returns ErrNotFound if the invoice doesn't exist.
	FindByID(ctx context.Context, id string) (*entity.Invoice, error)

	// FindByNumber loads an invoice by its tenant-scoped number.
	FindByNumber(ctx context.Context, tenantID, number string) (*entity.Invoice, error)

	// ExistsByNumber checks if an invoice number is already used within a tenant.
	ExistsByNumber(ctx context.Context, tenantID, number string) (bool, error)

	// NextInvoiceNumber generates the next sequential invoice number for a tenant.
	// Format depends on tenant settings (e.g., "INV-2024-00001").
	NextInvoiceNumber(ctx context.Context, tenantID string) (string, error)

	// FindByTenant returns invoices for a tenant with pagination.
	FindByTenant(ctx context.Context, tenantID string, limit, offset int) ([]*entity.Invoice, int, error)

	// FindOverdue returns invoices past their due date that are still in "issued" status.
	FindOverdue(ctx context.Context, tenantID string, asOf time.Time) ([]*entity.Invoice, error)
}

// CustomerRepository is the port for customer persistence.
type CustomerRepository interface {
	Save(ctx context.Context, customer *entity.Customer) error
	FindByID(ctx context.Context, id string) (*entity.Customer, error)
	FindByTenant(ctx context.Context, tenantID string, limit, offset int) ([]*entity.Customer, error)
	Delete(ctx context.Context, id string) error
}

// PaymentRepository is the port for payment persistence.
type PaymentRepository interface {
	Save(ctx context.Context, payment *entity.Payment) error
	FindByInvoice(ctx context.Context, invoiceID string) ([]*entity.Payment, error)
	FindByID(ctx context.Context, id string) (*entity.Payment, error)
}

// SubscriptionRepository is the port for subscription persistence.
type SubscriptionRepository interface {
	Save(ctx context.Context, sub *entity.Subscription) error
	FindByID(ctx context.Context, id string) (*entity.Subscription, error)
	// FindDue returns active subscriptions whose next_billing_date is <= asOf.
	FindDue(ctx context.Context, asOf time.Time, limit int) ([]*entity.Subscription, error)
	UpdateNextBillingDate(ctx context.Context, id string, nextDate time.Time) error
}

// TenantRepository is the port for tenant persistence.
type TenantRepository interface {
	FindByID(ctx context.Context, id string) (*entity.Tenant, error)
	Save(ctx context.Context, tenant *entity.Tenant) error
}

// OutboxRepository is the port for the transactional outbox.
type OutboxRepository interface {
	// SaveEvents writes domain events to the outbox table.
	// Must be called within the same transaction as the aggregate save.
	SaveEvents(ctx context.Context, tenantID string, events []entity.DomainEvent) error

	// FindUnprocessed returns outbox entries not yet relayed.
	FindUnprocessed(ctx context.Context, limit int) ([]OutboxEntry, error)

	// MarkProcessed marks an outbox entry as successfully relayed.
	MarkProcessed(ctx context.Context, id string) error
}

// OutboxEntry is the read model for outbox entries.
type OutboxEntry struct {
	ID          string
	AggregateID string
	TenantID    string
	EventType   string
	Payload     []byte
	CreatedAt   time.Time
}

// IdempotencyRepository is the port for idempotency key storage.
type IdempotencyRepository interface {
	// Get retrieves a cached response by key.
	Get(ctx context.Context, key string) (*IdempotencyRecord, error)

	// Save stores a response with a TTL.
	Save(ctx context.Context, record *IdempotencyRecord) error
}

type IdempotencyRecord struct {
	Key          string
	TenantID     string
	ResponseBody []byte
	Status       int
	ExpiresAt    time.Time
}