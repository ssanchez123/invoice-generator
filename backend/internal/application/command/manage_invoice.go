package command

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/domain/port"
	"github.com/santiagossaa/invoice-generator/internal/domain/valueobject"
)

// IssueInvoiceCommand transitions an invoice from draft to issued.
type IssueInvoiceCommand struct {
	InvoiceID string
	TenantID  string
}

type IssueInvoiceHandler struct {
	invoiceRepo port.InvoiceRepository
}

func NewIssueInvoiceHandler(ir port.InvoiceRepository) *IssueInvoiceHandler {
	return &IssueInvoiceHandler{invoiceRepo: ir}
}

func (h *IssueInvoiceHandler) Handle(ctx context.Context, cmd IssueInvoiceCommand) (*entity.Invoice, error) {
	invoice, err := h.invoiceRepo.FindByID(ctx, cmd.InvoiceID)
	if err != nil {
		return nil, err
	}
	if invoice.TenantID != cmd.TenantID {
		return nil, ErrInvoiceNotInTenant
	}

	if err := invoice.Issue(time.Now()); err != nil {
		return nil, err
	}

	if err := h.invoiceRepo.Save(ctx, invoice); err != nil {
		return nil, err
	}

	return invoice, nil
}

// CancelInvoiceCommand cancels an invoice.
type CancelInvoiceCommand struct {
	InvoiceID string
	TenantID  string
}

type CancelInvoiceHandler struct {
	invoiceRepo port.InvoiceRepository
}

func NewCancelInvoiceHandler(ir port.InvoiceRepository) *CancelInvoiceHandler {
	return &CancelInvoiceHandler{invoiceRepo: ir}
}

func (h *CancelInvoiceHandler) Handle(ctx context.Context, cmd CancelInvoiceCommand) error {
	invoice, err := h.invoiceRepo.FindByID(ctx, cmd.InvoiceID)
	if err != nil {
		return err
	}
	if invoice.TenantID != cmd.TenantID {
		return ErrInvoiceNotInTenant
	}

	if err := invoice.Cancel(); err != nil {
		return err
	}

	return h.invoiceRepo.Save(ctx, invoice)
}

// RecordPaymentCommand records a payment against an invoice.
type RecordPaymentCommand struct {
	InvoiceID string
	TenantID  string
	Amount    int64 // in minor units
	Currency  string
	Method    string // "bank_transfer", "credit_card", "cash"
	Reference string
	PaidAt    time.Time
}

type RecordPaymentHandler struct {
	invoiceRepo port.InvoiceRepository
	paymentRepo port.PaymentRepository
	uow         port.UnitOfWork
}

func NewRecordPaymentHandler(ir port.InvoiceRepository, pr port.PaymentRepository, uow port.UnitOfWork) *RecordPaymentHandler {
	return &RecordPaymentHandler{invoiceRepo: ir, paymentRepo: pr, uow: uow}
}

func (h *RecordPaymentHandler) Handle(ctx context.Context, cmd RecordPaymentCommand) (*entity.Payment, error) {
	// If we have a UnitOfWork, use a transaction with pessimistic lock.
	// This prevents concurrent payments from causing race conditions.
	if h.uow != nil {
		return h.handleWithTx(ctx, cmd)
	}
	// Fallback: no UoW (e.g., in-memory repos in tests) — use the old path.
	return h.handleWithoutTx(ctx, cmd)
}

// handleWithTx executes the payment recording within a transaction,
// using SELECT FOR UPDATE to lock the invoice row and prevent concurrent modifications.
func (h *RecordPaymentHandler) handleWithTx(ctx context.Context, cmd RecordPaymentCommand) (*entity.Payment, error) {
	txCtx, tx, err := h.uow.BeginTx(ctx, cmd.TenantID)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(txCtx) // safe: no-op if committed

	// Lock the invoice row (SELECT FOR UPDATE) — concurrent payments will block here
	invoice, err := h.invoiceRepo.FindByIDForUpdate(txCtx, cmd.InvoiceID)
	if err != nil {
		return nil, err
	}
	if invoice.TenantID != cmd.TenantID {
		return nil, ErrInvoiceNotInTenant
	}

	amount, err := valueobject.NewMoney(cmd.Amount, cmd.Currency)
	if err != nil {
		return nil, err
	}

	// Check if payment exceeds invoice total
	if amount.Amount > invoice.Total.Amount {
		return nil, entity.ErrInsufficientPayment
	}

	// Apply payment to invoice (produces domain event)
	if err := invoice.ApplyPayment(amount, cmd.PaidAt); err != nil {
		return nil, err
	}

	// Read existing payments WITHIN the transaction (consistent read under lock)
	existingPayments, err := h.paymentRepo.FindByInvoiceTx(txCtx, cmd.InvoiceID)
	if err != nil {
		return nil, err
	}

	totalPaid := int64(0)
	for _, p := range existingPayments {
		totalPaid += p.Amount.Amount
	}
	totalPaid += amount.Amount

	// Check for overpayment (concurrent payments that already covered the total)
	if totalPaid > invoice.Total.Amount {
		return nil, entity.ErrInsufficientPayment
	}

	if totalPaid >= invoice.Total.Amount {
		if err := invoice.MarkPaid(cmd.PaidAt); err != nil {
			return nil, err
		}
	}

	// Save payment within the transaction
	payment := &entity.Payment{
		ID:        uuid.NewString(),
		InvoiceID: cmd.InvoiceID,
		TenantID:  cmd.TenantID,
		Amount:    amount,
		Method:    cmd.Method,
		Reference: cmd.Reference,
		PaidAt:    cmd.PaidAt,
		CreatedAt: time.Now(),
	}

	if err := h.paymentRepo.SaveTx(txCtx, payment); err != nil {
		return nil, err
	}

	// Save invoice (with events) within the transaction
	if err := h.invoiceRepo.SaveTx(txCtx, invoice); err != nil {
		return nil, err
	}

	if err := tx.Commit(txCtx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return payment, nil
}

// handleWithoutTx is the fallback path for in-memory repositories (no real transaction).
// Used in tests where concurrency is not a concern.
func (h *RecordPaymentHandler) handleWithoutTx(ctx context.Context, cmd RecordPaymentCommand) (*entity.Payment, error) {
	invoice, err := h.invoiceRepo.FindByID(ctx, cmd.InvoiceID)
	if err != nil {
		return nil, err
	}
	if invoice.TenantID != cmd.TenantID {
		return nil, ErrInvoiceNotInTenant
	}

	amount, err := valueobject.NewMoney(cmd.Amount, cmd.Currency)
	if err != nil {
		return nil, err
	}

	// Check if payment exceeds invoice total
	if amount.Amount > invoice.Total.Amount {
		return nil, entity.ErrInsufficientPayment
	}

	// Apply payment to invoice (produces domain event)
	if err := invoice.ApplyPayment(amount, cmd.PaidAt); err != nil {
		return nil, err
	}

	// Check if invoice is now fully paid
	existingPayments, err := h.paymentRepo.FindByInvoice(ctx, cmd.InvoiceID)
	if err != nil {
		return nil, err
	}

	totalPaid := int64(0)
	for _, p := range existingPayments {
		totalPaid += p.Amount.Amount
	}
	totalPaid += amount.Amount

	// Check for overpayment
	if totalPaid > invoice.Total.Amount {
		return nil, entity.ErrInsufficientPayment
	}

	if totalPaid >= invoice.Total.Amount {
		if err := invoice.MarkPaid(cmd.PaidAt); err != nil {
			return nil, err
		}
	}

	// Save payment
	payment := &entity.Payment{
		ID:        uuid.NewString(),
		InvoiceID: cmd.InvoiceID,
		TenantID:  cmd.TenantID,
		Amount:    amount,
		Method:    cmd.Method,
		Reference: cmd.Reference,
		PaidAt:    cmd.PaidAt,
		CreatedAt: time.Now(),
	}

	if err := h.paymentRepo.Save(ctx, payment); err != nil {
		return nil, err
	}

	// Save invoice (with events)
	if err := h.invoiceRepo.Save(ctx, invoice); err != nil {
		return nil, err
	}

	return payment, nil
}
