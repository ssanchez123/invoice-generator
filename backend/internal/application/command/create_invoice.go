package command

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/domain/port"
	"github.com/santiagossaa/invoice-generator/internal/domain/valueobject"
)

// CreateInvoiceCommand creates a new draft invoice.
type CreateInvoiceCommand struct {
	TenantID   string
	CustomerID string
	Currency   string
	DueDate    time.Time
	Items      []ItemInput
	Notes      string
	Metadata   map[string]any
}

type ItemInput struct {
	Description string
	Quantity    int64
	UnitPrice   int64 // in minor units (cents)
	TaxRateBPS  int64 // basis points (1900 = 19%)
	DiscountBPS int64 // basis points
}

type CreateInvoiceHandler struct {
	invoiceRepo  port.InvoiceRepository
	customerRepo port.CustomerRepository
}

func NewCreateInvoiceHandler(ir port.InvoiceRepository, cr port.CustomerRepository) *CreateInvoiceHandler {
	return &CreateInvoiceHandler{invoiceRepo: ir, customerRepo: cr}
}

func (h *CreateInvoiceHandler) Handle(ctx context.Context, cmd CreateInvoiceCommand) (*entity.Invoice, error) {
	// Validate customer exists
	customer, err := h.customerRepo.FindByID(ctx, cmd.CustomerID)
	if err != nil {
		return nil, err
	}
	if customer.TenantID != cmd.TenantID {
		return nil, ErrCustomerNotInTenant
	}

	// Generate invoice number
	number, err := h.invoiceRepo.NextInvoiceNumber(ctx, cmd.TenantID)
	if err != nil {
		return nil, err
	}
	invoiceNumber, err := valueobject.NewInvoiceNumber(number)
	if err != nil {
		return nil, err
	}

	// Build invoice
	invoice := &entity.Invoice{
		ID:         uuid.NewString(),
		TenantID:   cmd.TenantID,
		CustomerID: cmd.CustomerID,
		Number:     invoiceNumber,
		Status:     entity.InvoiceStatusDraft,
		DueDate:    cmd.DueDate,
		Currency:   cmd.Currency,
		Notes:      cmd.Notes,
		Metadata:   cmd.Metadata,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Add items
	for _, item := range cmd.Items {
		unitPrice, err := valueobject.NewMoney(item.UnitPrice, cmd.Currency)
		if err != nil {
			return nil, err
		}
		invoiceItem := entity.InvoiceItem{
			ID:          uuid.NewString(),
			Description: item.Description,
			Quantity:    item.Quantity,
			UnitPrice:   unitPrice,
			TaxRateBPS:  item.TaxRateBPS,
			DiscountBPS: item.DiscountBPS,
		}
		if err := invoice.AddItem(invoiceItem); err != nil {
			return nil, err
		}
	}

	// Persist
	if err := h.invoiceRepo.Save(ctx, invoice); err != nil {
		return nil, err
	}

	return invoice, nil
}