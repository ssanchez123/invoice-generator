package command

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/domain/port"
	"github.com/santiagossaa/invoice-generator/internal/domain/valueobject"
)

type UpdateInvoiceCommand struct {
	InvoiceID  string
	TenantID   string
	CustomerID string
	DueDate    time.Time
	Currency   string
	Items      []ItemInput
	Notes      string
	Metadata   map[string]any
}

type UpdateInvoiceHandler struct {
	invoiceRepo  port.InvoiceRepository
	customerRepo port.CustomerRepository
	uow          port.UnitOfWork
}

func NewUpdateInvoiceHandler(ir port.InvoiceRepository, cr port.CustomerRepository, uow port.UnitOfWork) *UpdateInvoiceHandler {
	return &UpdateInvoiceHandler{invoiceRepo: ir, customerRepo: cr, uow: uow}
}

func (h *UpdateInvoiceHandler) Handle(ctx context.Context, cmd UpdateInvoiceCommand) (*entity.Invoice, error) {
	if h.uow != nil {
		return h.handleWithTx(ctx, cmd)
	}
	invoice, err := h.invoiceRepo.FindByID(ctx, cmd.InvoiceID)
	if err != nil {
		return nil, err
	}
	return h.updateAndSave(ctx, invoice, cmd, false)
}

func (h *UpdateInvoiceHandler) handleWithTx(ctx context.Context, cmd UpdateInvoiceCommand) (*entity.Invoice, error) {
	txCtx, tx, err := h.uow.BeginTx(ctx, cmd.TenantID)
	if err != nil {
		return nil, fmt.Errorf("begin invoice update transaction: %w", err)
	}
	defer tx.Rollback(txCtx)

	invoice, err := h.invoiceRepo.FindByIDForUpdate(txCtx, cmd.InvoiceID)
	if err != nil {
		return nil, err
	}
	invoice, err = h.updateAndSave(txCtx, invoice, cmd, true)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(txCtx); err != nil {
		return nil, fmt.Errorf("commit invoice update transaction: %w", err)
	}
	return invoice, nil
}

func (h *UpdateInvoiceHandler) updateAndSave(ctx context.Context, invoice *entity.Invoice, cmd UpdateInvoiceCommand, inTx bool) (*entity.Invoice, error) {
	if invoice.TenantID != cmd.TenantID {
		return nil, ErrInvoiceNotInTenant
	}
	customer, err := h.customerRepo.FindByID(ctx, cmd.CustomerID)
	if err != nil {
		return nil, err
	}
	if customer.TenantID != cmd.TenantID {
		return nil, ErrCustomerNotInTenant
	}

	currency := strings.ToUpper(strings.TrimSpace(cmd.Currency))
	if _, err := valueobject.NewMoney(0, currency); err != nil {
		return nil, entity.ErrInvalidInvoiceDetails
	}
	items := make([]entity.InvoiceItem, 0, len(cmd.Items))
	for _, input := range cmd.Items {
		description := strings.TrimSpace(input.Description)
		if len(description) == 0 || len(description) > 240 || input.Quantity <= 0 ||
			input.UnitPrice < 0 || input.TaxRateBPS < 0 ||
			input.DiscountBPS < 0 || input.DiscountBPS > 10000 {
			return nil, entity.ErrInvalidInvoiceDetails
		}
		price, err := valueobject.NewMoney(input.UnitPrice, currency)
		if err != nil {
			return nil, entity.ErrInvalidInvoiceDetails
		}
		items = append(items, entity.InvoiceItem{
			ID:          uuid.NewString(),
			Description: description,
			Quantity:    input.Quantity,
			UnitPrice:   price,
			TaxRateBPS:  input.TaxRateBPS,
			DiscountBPS: input.DiscountBPS,
		})
	}
	if err := invoice.UpdateDraft(cmd.CustomerID, cmd.DueDate, currency, items, cmd.Notes, cmd.Metadata); err != nil {
		return nil, err
	}
	if inTx {
		err = h.invoiceRepo.SaveTx(ctx, invoice)
	} else {
		err = h.invoiceRepo.Save(ctx, invoice)
	}
	if err != nil {
		return nil, err
	}
	return invoice, nil
}
