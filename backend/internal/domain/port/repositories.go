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

	// FindByIDForUpdate loads an invoice and acquires a row lock (SELECT FOR UPDATE).
	// Must be called within a transaction (use TxFromCtx to check).
	// Returns ErrNotFound if the invoice doesn't exist.
	FindByIDForUpdate(ctx context.Context, id string) (*entity.Invoice, error)

	// SaveTx persists an invoice within an existing transaction (from TxFromCtx).
	// Use this when you need to save the invoice as part of a larger transaction.
	SaveTx(ctx context.Context, invoice *entity.Invoice) error

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

	// SaveTx saves a payment within an existing transaction (from TxFromCtx).
	SaveTx(ctx context.Context, payment *entity.Payment) error

	// FindByInvoiceTx loads payments for an invoice within an existing transaction.
	FindByInvoiceTx(ctx context.Context, invoiceID string) ([]*entity.Payment, error)
}

// SubscriptionRepository is the port for subscription persistence.
type SubscriptionRepository interface {
	Save(ctx context.Context, sub *entity.Subscription) error
	SaveTx(ctx context.Context, sub *entity.Subscription) error
	FindByID(ctx context.Context, id string) (*entity.Subscription, error)
	FindByIDForUpdate(ctx context.Context, id string) (*entity.Subscription, error)
	FindByTenant(ctx context.Context, tenantID, status string, limit, offset int) ([]*entity.Subscription, int, error)
	// FindDue returns active subscriptions whose next_billing_date is <= asOf.
	FindDueForUpdate(ctx context.Context, asOf time.Time, limit int) ([]*entity.Subscription, error)
}

// TenantRepository is the port for tenant persistence.
type TenantRepository interface {
	FindByID(ctx context.Context, id string) (*entity.Tenant, error)
	Save(ctx context.Context, tenant *entity.Tenant) error
	ListIDs(ctx context.Context) ([]string, error)
}

type AuditFilter struct {
	TenantID   string
	EntityType string
	EntityID   string
	Action     string
	Limit      int
	Offset     int
}

type AuditRepository interface {
	List(ctx context.Context, filter AuditFilter) ([]entity.AuditEntry, int, error)
}

// OutboxRepository is the port for the transactional outbox.
type OutboxRepository interface {
	// SaveEvents writes domain events to the outbox table.
	// Must be called within the same transaction as the aggregate save.
	SaveEvents(ctx context.Context, tenantID string, events []entity.DomainEvent) error

	// ListTenantIDs returns tenant IDs for background processing.
	ListTenantIDs(ctx context.Context) ([]string, error)

	// ClaimDeliveries claims pending or expired deliveries for one tenant and consumer.
	ClaimDeliveries(ctx context.Context, tenantID, consumer, workerID string, limit int, lease time.Duration) ([]OutboxEntry, error)

	// RenewDeliveryLease extends an unexpired lease still owned by this worker.
	RenewDeliveryLease(ctx context.Context, tenantID, eventID, consumer, workerID string, lease time.Duration) error

	// CompleteDelivery acknowledges a delivery owned by this worker.
	CompleteDelivery(ctx context.Context, tenantID, eventID, consumer, workerID string) error

	// RetryDelivery releases a failed delivery with backoff or marks it failed after the retry limit.
	RetryDelivery(ctx context.Context, tenantID, eventID, consumer, workerID string, cause error) error
}

// OutboxEntry is the read model for outbox entries.
type OutboxEntry struct {
	ID          string
	AggregateID string
	TenantID    string
	EventType   string
	Payload     []byte
	CreatedAt   time.Time
	Attempts    int
}

// IdempotencyRepository is the port for idempotency key storage.
type IdempotencyRepository interface {
	// Reserve atomically claims a key or returns the existing record.
	Reserve(ctx context.Context, record *IdempotencyRecord) (*IdempotencyRecord, bool, error)
	// Complete stores the response for a previously reserved key.
	Complete(ctx context.Context, record *IdempotencyRecord) error
}

type IdempotencyRecord struct {
	Key             string
	TenantID        string
	RequestHash     string
	State           string
	ResponseBody    []byte
	ResponseHeaders []byte
	Status          int
	ExpiresAt       time.Time
}
