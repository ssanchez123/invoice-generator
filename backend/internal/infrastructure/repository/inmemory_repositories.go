package repository

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/domain/valueobject"
)

// InMemoryInvoiceRepository is a test double for InvoiceRepository.
// It implements the same port but stores data in memory — no PostgreSQL needed.
type InMemoryInvoiceRepository struct {
	mu        sync.RWMutex
	invoices  map[string]*entity.Invoice
	sequences map[string]int // tenant_id -> last seq
}

func NewInMemoryInvoiceRepository() *InMemoryInvoiceRepository {
	return &InMemoryInvoiceRepository{
		invoices:  make(map[string]*entity.Invoice),
		sequences: make(map[string]int),
	}
}

func (r *InMemoryInvoiceRepository) Save(ctx context.Context, invoice *entity.Invoice) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Clone to avoid external mutation
	inv := *invoice
	r.invoices[invoice.ID] = &inv
	// Clear events (simulating outbox write)
	invoice.ClearEvents()
	return nil
}

func (r *InMemoryInvoiceRepository) FindByID(ctx context.Context, id string) (*entity.Invoice, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	inv, ok := r.invoices[id]
	if !ok {
		return nil, ErrNotFound
	}
	// Return clone
	c := *inv
	return &c, nil
}

func (r *InMemoryInvoiceRepository) FindByNumber(ctx context.Context, tenantID, number string) (*entity.Invoice, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, inv := range r.invoices {
		if inv.TenantID == tenantID && inv.Number.Value == number {
			c := *inv
			return &c, nil
		}
	}
	return nil, ErrNotFound
}

func (r *InMemoryInvoiceRepository) ExistsByNumber(ctx context.Context, tenantID, number string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, inv := range r.invoices {
		if inv.TenantID == tenantID && inv.Number.Value == number {
			return true, nil
		}
	}
	return false, nil
}

func (r *InMemoryInvoiceRepository) NextInvoiceNumber(ctx context.Context, tenantID string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sequences[tenantID]++
	seq := r.sequences[tenantID]
	year := time.Now().Year()
	return formatInvoiceNumber(year, seq), nil
}

func (r *InMemoryInvoiceRepository) FindByTenant(ctx context.Context, tenantID string, limit, offset int) ([]*entity.Invoice, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var all []*entity.Invoice
	for _, inv := range r.invoices {
		if inv.TenantID == tenantID {
			c := *inv
			all = append(all, &c)
		}
	}

	total := len(all)
	if offset >= total {
		return nil, total, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return all[offset:end], total, nil
}

func (r *InMemoryInvoiceRepository) FindOverdue(ctx context.Context, tenantID string, asOf time.Time) ([]*entity.Invoice, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*entity.Invoice
	for _, inv := range r.invoices {
		if inv.TenantID == tenantID && inv.Status == entity.InvoiceStatusIssued && inv.DueDate.Before(asOf) {
			c := *inv
			result = append(result, &c)
		}
	}
	return result, nil
}

// InMemoryCustomerRepository is a test double for CustomerRepository.
type InMemoryCustomerRepository struct {
	mu        sync.RWMutex
	customers map[string]*entity.Customer
}

func NewInMemoryCustomerRepository() *InMemoryCustomerRepository {
	return &InMemoryCustomerRepository{customers: make(map[string]*entity.Customer)}
}

func (r *InMemoryCustomerRepository) Save(ctx context.Context, customer *entity.Customer) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := *customer
	r.customers[customer.ID] = &c
	return nil
}

func (r *InMemoryCustomerRepository) FindByID(ctx context.Context, id string) (*entity.Customer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.customers[id]
	if !ok {
		return nil, ErrNotFound
	}
	clone := *c
	return &clone, nil
}

func (r *InMemoryCustomerRepository) FindByTenant(ctx context.Context, tenantID string, limit, offset int) ([]*entity.Customer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*entity.Customer
	for _, c := range r.customers {
		if c.TenantID == tenantID {
			clone := *c
			result = append(result, &clone)
		}
	}
	return result, nil
}

func (r *InMemoryCustomerRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.customers[id]; !ok {
		return ErrNotFound
	}
	delete(r.customers, id)
	return nil
}

// InMemoryPaymentRepository is a test double for PaymentRepository.
type InMemoryPaymentRepository struct {
	mu       sync.RWMutex
	payments map[string]*entity.Payment
}

func NewInMemoryPaymentRepository() *InMemoryPaymentRepository {
	return &InMemoryPaymentRepository{payments: make(map[string]*entity.Payment)}
}

func (r *InMemoryPaymentRepository) Save(ctx context.Context, payment *entity.Payment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := *payment
	r.payments[payment.ID] = &c
	return nil
}

func (r *InMemoryPaymentRepository) FindByInvoice(ctx context.Context, invoiceID string) ([]*entity.Payment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*entity.Payment
	for _, p := range r.payments {
		if p.InvoiceID == invoiceID {
			clone := *p
			result = append(result, &clone)
		}
	}
	return result, nil
}

func (r *InMemoryPaymentRepository) FindByID(ctx context.Context, id string) (*entity.Payment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.payments[id]
	if !ok {
		return nil, ErrNotFound
	}
	clone := *p
	return &clone, nil
}

// formatInvoiceNumber generates the invoice number string.
func formatInvoiceNumber(year, seq int) string {
	return fmt.Sprintf("INV-%d-%05d", year, seq)
}

func fmtInvoiceNum(year, seq int) string {
	return formatInvoiceNumber(year, seq)
}

// Helper to create a test customer
func NewTestCustomer(tenantID string) *entity.Customer {
	return &entity.Customer{
		ID:        uuid.NewString(),
		TenantID:  tenantID,
		Name:      "Test Customer",
		Email:     "test@example.com",
		Address:   valueobject.Address{Line1: "123 Test St", City: "Test City", Country: "CO"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// Helper to create a test invoice
func NewTestInvoice(tenantID, customerID string) *entity.Invoice {
	return &entity.Invoice{
		ID:         uuid.NewString(),
		TenantID:   tenantID,
		CustomerID: customerID,
		Number:     valueobject.InvoiceNumber{Value: "INV-2024-00001"},
		Status:     entity.InvoiceStatusDraft,
		DueDate:    time.Now().Add(30 * 24 * time.Hour),
		Currency:   "USD",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}
