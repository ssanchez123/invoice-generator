package query

import (
	"context"
	"time"

	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/domain/port"
)

// ListInvoicesQuery returns a paginated list of invoices for a tenant.
type ListInvoicesQuery struct {
	TenantID   string
	Status     *entity.InvoiceStatus
	CustomerID *string
	FromDate   *time.Time
	ToDate     *time.Time
	Limit      int
	Offset     int
}

type ListInvoicesHandler struct {
	// In a full CQRS implementation with separate read stores, this would query
	// a denormalized projection. For now, we query through the repository.
	// This is CQRS with shared DB — the handler is separate from commands,
	// but reads from the same store. The separation allows future evolution.
	invoiceRepo port.InvoiceRepository
}

func NewListInvoicesHandler(ir port.InvoiceRepository) *ListInvoicesHandler {
	return &ListInvoicesHandler{invoiceRepo: ir}
}

// InvoiceReadModel is the DTO returned by queries.
// It's NOT the domain entity — it's a flattened view for the read side.
type InvoiceReadModel struct {
	ID         string                 `json:"id"`
	Number     string                 `json:"number"`
	CustomerID string                 `json:"customer_id"`
	Status     string                 `json:"status"`
	IssueDate  time.Time              `json:"issue_date"`
	DueDate    time.Time              `json:"due_date"`
	Currency   string                 `json:"currency"`
	Subtotal   int64                  `json:"subtotal"`
	TaxTotal   int64                  `json:"tax_total"`
	Total      int64                  `json:"total"`
	Items      []InvoiceItemReadModel `json:"items"`
	CreatedAt  time.Time              `json:"created_at"`
}

type InvoiceItemReadModel struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Quantity    int64  `json:"quantity"`
	UnitPrice   int64  `json:"unit_price"`
	TaxRateBPS  int64  `json:"tax_rate_bps"`
	DiscountBPS int64  `json:"discount_bps"`
	Total       int64  `json:"total"`
}

func (h *ListInvoicesHandler) Handle(ctx context.Context, q ListInvoicesQuery) ([]InvoiceReadModel, int, error) {
	invoices, total, err := h.invoiceRepo.FindByTenant(ctx, q.TenantID, q.Limit, q.Offset)
	if err != nil {
		return nil, 0, err
	}

	result := make([]InvoiceReadModel, 0, len(invoices))
	for _, inv := range invoices {
		// Apply optional status filter
		if q.Status != nil && inv.Status != *q.Status {
			continue
		}
		// Apply optional customer filter
		if q.CustomerID != nil && inv.CustomerID != *q.CustomerID {
			continue
		}

		items := make([]InvoiceItemReadModel, 0, len(inv.Items))
		for _, item := range inv.Items {
			lineTotal, _ := item.LineTotal()
			items = append(items, InvoiceItemReadModel{
				ID:          item.ID,
				Description: item.Description,
				Quantity:    item.Quantity,
				UnitPrice:   item.UnitPrice.Amount,
				TaxRateBPS:  item.TaxRateBPS,
				DiscountBPS: item.DiscountBPS,
				Total:       lineTotal.Amount,
			})
		}

		result = append(result, InvoiceReadModel{
			ID:         inv.ID,
			Number:     inv.Number.Value,
			CustomerID: inv.CustomerID,
			Status:     string(inv.Status),
			IssueDate:  inv.IssueDate,
			DueDate:    inv.DueDate,
			Currency:   inv.Currency,
			Subtotal:   inv.Subtotal.Amount,
			TaxTotal:   inv.TaxTotal.Amount,
			Total:      inv.Total.Amount,
			Items:      items,
			CreatedAt:  inv.CreatedAt,
		})
	}

	// If filters were applied in-memory, the total count is the filtered count
	if (q.Status != nil || q.CustomerID != nil) && len(result) != total {
		total = len(result)
	}

	return result, total, nil
}

// GetInvoiceQuery returns a single invoice detail.
type GetInvoiceQuery struct {
	InvoiceID string
	TenantID  string
}

type GetInvoiceHandler struct {
	invoiceRepo port.InvoiceRepository
}

func NewGetInvoiceHandler(ir port.InvoiceRepository) *GetInvoiceHandler {
	return &GetInvoiceHandler{invoiceRepo: ir}
}

func (h *GetInvoiceHandler) Handle(ctx context.Context, q GetInvoiceQuery) (*entity.Invoice, error) {
	invoice, err := h.invoiceRepo.FindByID(ctx, q.InvoiceID)
	if err != nil {
		return nil, err
	}
	if invoice.TenantID != q.TenantID {
		return nil, ErrNotFound
	}
	return invoice, nil
}

// ListCustomersQuery returns a paginated list of customers.
type ListCustomersQuery struct {
	TenantID string
	Limit    int
	Offset   int
}

type ListCustomersHandler struct {
	customerRepo port.CustomerRepository
}

func NewListCustomersHandler(cr port.CustomerRepository) *ListCustomersHandler {
	return &ListCustomersHandler{customerRepo: cr}
}

func (h *ListCustomersHandler) Handle(ctx context.Context, q ListCustomersQuery) ([]*entity.Customer, error) {
	return h.customerRepo.FindByTenant(ctx, q.TenantID, q.Limit, q.Offset)
}

// ListPaymentsQuery returns all payments for an invoice.
type ListPaymentsQuery struct {
	InvoiceID string
	TenantID  string
}

type ListPaymentsHandler struct {
	paymentRepo port.PaymentRepository
	invoiceRepo port.InvoiceRepository
}

func NewListPaymentsHandler(pr port.PaymentRepository, ir port.InvoiceRepository) *ListPaymentsHandler {
	return &ListPaymentsHandler{paymentRepo: pr, invoiceRepo: ir}
}

func (h *ListPaymentsHandler) Handle(ctx context.Context, q ListPaymentsQuery) ([]*entity.Payment, error) {
	// Verify invoice belongs to tenant
	invoice, err := h.invoiceRepo.FindByID(ctx, q.InvoiceID)
	if err != nil {
		return nil, err
	}
	if invoice.TenantID != q.TenantID {
		return nil, ErrNotFound
	}
	return h.paymentRepo.FindByInvoice(ctx, q.InvoiceID)
}
