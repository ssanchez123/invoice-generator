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

type CreateSubscriptionCommand struct {
	TenantID        string
	CustomerID      string
	Currency        string
	Frequency       entity.BillingFrequency
	NextBillingDate time.Time
	Items           []ItemInput
}

type CreateSubscriptionHandler struct {
	subscriptionRepo port.SubscriptionRepository
	customerRepo     port.CustomerRepository
}

func NewCreateSubscriptionHandler(sr port.SubscriptionRepository, cr port.CustomerRepository) *CreateSubscriptionHandler {
	return &CreateSubscriptionHandler{subscriptionRepo: sr, customerRepo: cr}
}

func (h *CreateSubscriptionHandler) Handle(ctx context.Context, cmd CreateSubscriptionCommand) (*entity.Subscription, error) {
	customer, err := h.customerRepo.FindByID(ctx, cmd.CustomerID)
	if err != nil {
		return nil, err
	}
	if customer.TenantID != cmd.TenantID {
		return nil, ErrCustomerNotInTenant
	}
	if len(cmd.Items) == 0 {
		return nil, entity.ErrInvalidSubscription
	}

	items := make([]entity.InvoiceItem, 0, len(cmd.Items))
	for _, input := range cmd.Items {
		price, err := valueobject.NewMoney(input.UnitPrice, cmd.Currency)
		if err != nil {
			return nil, entity.ErrInvalidSubscription
		}
		items = append(items, entity.InvoiceItem{
			ID:          uuid.NewString(),
			Description: input.Description,
			Quantity:    input.Quantity,
			UnitPrice:   price,
			TaxRateBPS:  input.TaxRateBPS,
			DiscountBPS: input.DiscountBPS,
		})
	}
	subscription, err := entity.NewSubscription(
		uuid.NewString(), cmd.TenantID, cmd.CustomerID, items,
		cmd.Frequency, cmd.NextBillingDate,
	)
	if err != nil {
		return nil, err
	}
	if err := h.subscriptionRepo.Save(ctx, subscription); err != nil {
		return nil, err
	}
	return subscription, nil
}

type CancelSubscriptionCommand struct {
	SubscriptionID string
	TenantID       string
}

type CancelSubscriptionHandler struct {
	subscriptionRepo port.SubscriptionRepository
	uow              port.UnitOfWork
}

func NewCancelSubscriptionHandler(sr port.SubscriptionRepository, uow port.UnitOfWork) *CancelSubscriptionHandler {
	return &CancelSubscriptionHandler{subscriptionRepo: sr, uow: uow}
}

func (h *CancelSubscriptionHandler) Handle(ctx context.Context, cmd CancelSubscriptionCommand) error {
	txCtx, tx, err := h.uow.BeginTx(ctx, cmd.TenantID)
	if err != nil {
		return fmt.Errorf("begin subscription cancellation transaction: %w", err)
	}
	defer tx.Rollback(txCtx)

	subscription, err := h.subscriptionRepo.FindByIDForUpdate(txCtx, cmd.SubscriptionID)
	if err != nil {
		return err
	}
	if subscription.TenantID != cmd.TenantID {
		return ErrSubscriptionNotInTenant
	}
	if err := subscription.Cancel(); err != nil {
		return err
	}
	if err := h.subscriptionRepo.SaveTx(txCtx, subscription); err != nil {
		return err
	}
	if err := tx.Commit(txCtx); err != nil {
		return fmt.Errorf("commit subscription cancellation: %w", err)
	}
	return nil
}

type ProcessDueSubscriptionsHandler struct {
	subscriptionRepo port.SubscriptionRepository
	customerRepo     port.CustomerRepository
	invoiceRepo      port.InvoiceRepository
	uow              port.UnitOfWork
}

func NewProcessDueSubscriptionsHandler(
	sr port.SubscriptionRepository,
	cr port.CustomerRepository,
	ir port.InvoiceRepository,
	uow port.UnitOfWork,
) *ProcessDueSubscriptionsHandler {
	return &ProcessDueSubscriptionsHandler{subscriptionRepo: sr, customerRepo: cr, invoiceRepo: ir, uow: uow}
}

// Handle creates at most one draft invoice per due subscription. Missed cycles
// remain due and are processed in subsequent batches; no invoice is issued here.
func (h *ProcessDueSubscriptionsHandler) Handle(ctx context.Context, tenantID string, asOf time.Time, limit int) (int, error) {
	txCtx, tx, err := h.uow.BeginTx(ctx, tenantID)
	if err != nil {
		return 0, fmt.Errorf("begin subscription billing transaction: %w", err)
	}
	defer tx.Rollback(txCtx)

	subscriptions, err := h.subscriptionRepo.FindDueForUpdate(txCtx, asOf, limit)
	if err != nil {
		return 0, err
	}
	for _, subscription := range subscriptions {
		customer, err := h.customerRepo.FindByID(txCtx, subscription.CustomerID)
		if err != nil {
			return 0, fmt.Errorf("load subscription customer %s: %w", subscription.CustomerID, err)
		}
		if customer.TenantID != tenantID {
			return 0, ErrCustomerNotInTenant
		}
		invoice, err := buildSubscriptionDraft(subscription, customer.ID, asOf)
		if err != nil {
			return 0, err
		}
		number, err := h.invoiceRepo.NextInvoiceNumber(txCtx, tenantID)
		if err != nil {
			return 0, fmt.Errorf("generate recurring invoice number: %w", err)
		}
		invoice.Number, err = valueobject.NewInvoiceNumber(number)
		if err != nil {
			return 0, fmt.Errorf("validate recurring invoice number: %w", err)
		}
		if err := h.invoiceRepo.SaveTx(txCtx, invoice); err != nil {
			return 0, fmt.Errorf("save recurring invoice draft: %w", err)
		}

		subscription.AdvanceBillingDate()
		if err := h.subscriptionRepo.SaveTx(txCtx, subscription); err != nil {
			return 0, fmt.Errorf("advance recurring billing date: %w", err)
		}
	}
	if err := tx.Commit(txCtx); err != nil {
		return 0, fmt.Errorf("commit subscription billing transaction: %w", err)
	}
	return len(subscriptions), nil
}

func buildSubscriptionDraft(subscription *entity.Subscription, customerID string, createdAt time.Time) (*entity.Invoice, error) {
	if len(subscription.PlanItems) == 0 {
		return nil, entity.ErrInvalidSubscription
	}
	currency := subscription.PlanItems[0].UnitPrice.Currency
	invoice := &entity.Invoice{
		ID:         uuid.NewString(),
		TenantID:   subscription.TenantID,
		CustomerID: customerID,
		Status:     entity.InvoiceStatusDraft,
		DueDate:    createdAt.AddDate(0, 0, 30),
		Currency:   currency,
		Notes:      "Borrador generado automáticamente por suscripción. Requiere revisión y emisión manual.",
		Metadata: map[string]any{
			"subscription_id": subscription.ID,
			"billing_period":  subscription.NextBillingDate.Format(time.RFC3339),
		},
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
	for _, planItem := range subscription.PlanItems {
		item := planItem
		item.ID = uuid.NewString()
		if err := invoice.AddItem(item); err != nil {
			return nil, fmt.Errorf("add recurring invoice item: %w", err)
		}
	}
	return invoice, nil
}
