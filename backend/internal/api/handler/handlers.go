package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/santiagossaa/invoice-generator/internal/api/middleware"
	commandHandler "github.com/santiagossaa/invoice-generator/internal/application/command"
	queryHandler "github.com/santiagossaa/invoice-generator/internal/application/query"
	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/domain/port"
	"github.com/santiagossaa/invoice-generator/internal/domain/valueobject"
)

// HandlerDeps holds the dependencies injected into all handlers.
// This struct is wired up in main.go and passed to each handler.
type HandlerDeps struct {
	CreateInvoice *commandHandler.CreateInvoiceHandler
	IssueInvoice  *commandHandler.IssueInvoiceHandler
	CancelInvoice *commandHandler.CancelInvoiceHandler
	RecordPayment *commandHandler.RecordPaymentHandler

	ListInvoices  *queryHandler.ListInvoicesHandler
	GetInvoice    *queryHandler.GetInvoiceHandler
	ListCustomers *queryHandler.ListCustomersHandler
	ListPayments  *queryHandler.ListPaymentsHandler

	CustomerRepo port.CustomerRepository
}

// deps is set by main.go via SetDeps(). In a real app, we'd use dependency injection.
var deps *HandlerDeps

func SetDeps(d *HandlerDeps) {
	deps = d
}

func respondJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body != nil {
		json.NewEncoder(w).Encode(body)
	}
}

func respondError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, map[string]string{"error": msg})
}

func parseBody(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// tenantIDFromCtx extracts the tenant ID from the request context.
func tenantIDFromCtx(r *http.Request) (string, bool) {
	v, ok := r.Context().Value(middleware.TenantIDKey).(string)
	return v, ok
}

// mustTenantID extracts tenant ID from context or writes a 400 and returns false.
func mustTenantID(w http.ResponseWriter, r *http.Request) (string, bool) {
	tid, ok := tenantIDFromCtx(r)
	if !ok || tid == "" {
		respondError(w, http.StatusBadRequest, "tenant_id is required")
		return "", false
	}
	return tid, true
}

// mapDomainError maps domain/application errors to appropriate HTTP status codes.
func mapDomainError(err error) (int, string) {
	// Check query package's ErrNotFound first.
	if errors.Is(err, queryHandler.ErrNotFound) {
		return http.StatusNotFound, err.Error()
	}
	// Check by string for repository.ErrNotFound since we don't import the repo package
	// to avoid an infrastructure dependency in the API layer.
	if err.Error() == "entity not found" {
		return http.StatusNotFound, err.Error()
	}
	switch {
	case errors.Is(err, entity.ErrInvalidStatusTransition):
		return http.StatusConflict, err.Error()
	case errors.Is(err, entity.ErrNoItems):
		return http.StatusBadRequest, err.Error()
	case errors.Is(err, entity.ErrIssueAfterDue):
		return http.StatusBadRequest, err.Error()
	case errors.Is(err, entity.ErrItemNotFound):
		return http.StatusNotFound, err.Error()
	case errors.Is(err, entity.ErrInsufficientPayment):
		return http.StatusBadRequest, err.Error()
	case errors.Is(err, entity.ErrDuplicatePayment):
		return http.StatusConflict, err.Error()
	case errors.Is(err, commandHandler.ErrCustomerNotInTenant):
		return http.StatusForbidden, err.Error()
	case errors.Is(err, commandHandler.ErrInvoiceNotInTenant):
		return http.StatusForbidden, err.Error()
	case errors.Is(err, commandHandler.ErrPaymentNotFound):
		return http.StatusNotFound, err.Error()
	default:
		return http.StatusInternalServerError, err.Error()
	}
}

// ========== DTOs ==========

// invoiceDTO is the JSON representation of an invoice entity.
// Since entity.Invoice has unexported fields (via valueobject types),
// we need a DTO to serialize all fields properly.
type invoiceDTO struct {
	ID         string           `json:"id"`
	TenantID   string           `json:"tenant_id"`
	CustomerID string           `json:"customer_id"`
	Number     string           `json:"number"`
	Status     string           `json:"status"`
	IssueDate  *time.Time       `json:"issue_date,omitempty"`
	DueDate    time.Time        `json:"due_date"`
	Currency   string           `json:"currency"`
	Subtotal   int64            `json:"subtotal"`
	TaxTotal   int64            `json:"tax_total"`
	Total      int64            `json:"total"`
	Items      []invoiceItemDTO `json:"items"`
	Notes      string           `json:"notes,omitempty"`
	Metadata   map[string]any   `json:"metadata,omitempty"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`
}

type invoiceItemDTO struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Quantity    int64  `json:"quantity"`
	UnitPrice   int64  `json:"unit_price"`
	TaxRateBPS  int64  `json:"tax_rate_bps"`
	DiscountBPS int64  `json:"discount_bps"`
}

func invoiceToDTO(inv *entity.Invoice) invoiceDTO {
	items := make([]invoiceItemDTO, 0, len(inv.Items))
	for _, item := range inv.Items {
		items = append(items, invoiceItemDTO{
			ID:          item.ID,
			Description: item.Description,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice.Amount,
			TaxRateBPS:  item.TaxRateBPS,
			DiscountBPS: item.DiscountBPS,
		})
	}

	dto := invoiceDTO{
		ID:         inv.ID,
		TenantID:   inv.TenantID,
		CustomerID: inv.CustomerID,
		Number:     inv.Number.Value,
		Status:     string(inv.Status),
		DueDate:    inv.DueDate,
		Currency:   inv.Currency,
		Subtotal:   inv.Subtotal.Amount,
		TaxTotal:   inv.TaxTotal.Amount,
		Total:      inv.Total.Amount,
		Items:      items,
		Notes:      inv.Notes,
		Metadata:   inv.Metadata,
		CreatedAt:  inv.CreatedAt,
		UpdatedAt:  inv.UpdatedAt,
	}

	if !inv.IssueDate.IsZero() {
		dto.IssueDate = &inv.IssueDate
	}

	return dto
}

// paymentDTO is the JSON representation of a payment entity.
type paymentDTO struct {
	ID        string    `json:"id"`
	InvoiceID string    `json:"invoice_id"`
	TenantID  string    `json:"tenant_id"`
	Amount    int64     `json:"amount"`
	Currency  string    `json:"currency"`
	Method    string    `json:"method"`
	Reference string    `json:"reference,omitempty"`
	PaidAt    time.Time `json:"paid_at"`
	CreatedAt time.Time `json:"created_at"`
}

func paymentToDTO(p *entity.Payment) paymentDTO {
	return paymentDTO{
		ID:        p.ID,
		InvoiceID: p.InvoiceID,
		TenantID:  p.TenantID,
		Amount:    p.Amount.Amount,
		Currency:  p.Amount.Currency,
		Method:    p.Method,
		Reference: p.Reference,
		PaidAt:    p.PaidAt,
		CreatedAt: p.CreatedAt,
	}
}

// customerDTO is the JSON representation of a customer entity.
type customerDTO struct {
	ID        string              `json:"id"`
	TenantID  string              `json:"tenant_id"`
	Name      string              `json:"name"`
	Email     string              `json:"email,omitempty"`
	Phone     string              `json:"phone,omitempty"`
	Address   valueobject.Address `json:"address"`
	TaxID     string              `json:"tax_id,omitempty"`
	Metadata  map[string]any      `json:"metadata,omitempty"`
	CreatedAt time.Time           `json:"created_at"`
	UpdatedAt time.Time           `json:"updated_at"`
}

func customerToDTO(c *entity.Customer) customerDTO {
	return customerDTO{
		ID:        c.ID,
		TenantID:  c.TenantID,
		Name:      c.Name,
		Email:     c.Email,
		Phone:     c.Phone,
		Address:   c.Address,
		TaxID:     c.TaxID,
		Metadata:  c.Metadata,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

// ========== Auth (scaffolds) ==========

func Login(w http.ResponseWriter, r *http.Request) {
	// POST /api/v1/auth/login
	respondJSON(w, http.StatusOK, map[string]string{
		"message": "login endpoint — scaffold",
	})
}

func Register(w http.ResponseWriter, r *http.Request) {
	// POST /api/v1/auth/register
	respondJSON(w, http.StatusCreated, map[string]string{
		"message": "register endpoint — scaffold",
	})
}

// ========== Invoices ==========

func CreateInvoice(w http.ResponseWriter, r *http.Request) {
	// POST /api/v1/invoices
	tenantID, ok := mustTenantID(w, r)
	if !ok {
		return
	}

	var req struct {
		CustomerID string `json:"customer_id"`
		Currency   string `json:"currency"`
		DueDate    string `json:"due_date"`
		Items      []struct {
			Description string `json:"description"`
			Quantity    int64  `json:"quantity"`
			UnitPrice   int64  `json:"unit_price"`
			TaxRateBPS  int64  `json:"tax_rate_bps"`
			DiscountBPS int64  `json:"discount_bps"`
		} `json:"items"`
		Notes string `json:"notes"`
	}

	if err := parseBody(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.CustomerID == "" {
		respondError(w, http.StatusBadRequest, "customer_id is required")
		return
	}
	if req.Currency == "" {
		respondError(w, http.StatusBadRequest, "currency is required")
		return
	}
	if len(req.Items) == 0 {
		respondError(w, http.StatusBadRequest, "at least one item is required")
		return
	}

	dueDate, err := time.Parse(time.RFC3339, req.DueDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid due_date format, expected RFC3339")
		return
	}

	items := make([]commandHandler.ItemInput, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, commandHandler.ItemInput{
			Description: it.Description,
			Quantity:    it.Quantity,
			UnitPrice:   it.UnitPrice,
			TaxRateBPS:  it.TaxRateBPS,
			DiscountBPS: it.DiscountBPS,
		})
	}

	cmd := commandHandler.CreateInvoiceCommand{
		TenantID:   tenantID,
		CustomerID: req.CustomerID,
		Currency:   req.Currency,
		DueDate:    dueDate,
		Items:      items,
		Notes:      req.Notes,
	}

	inv, err := deps.CreateInvoice.Handle(r.Context(), cmd)
	if err != nil {
		status, msg := mapDomainError(err)
		respondError(w, status, msg)
		return
	}

	respondJSON(w, http.StatusCreated, invoiceToDTO(inv))
}

func ListInvoices(w http.ResponseWriter, r *http.Request) {
	// GET /api/v1/invoices?status=issued&customer_id=...&limit=20&offset=0
	tenantID, ok := mustTenantID(w, r)
	if !ok {
		return
	}

	q := r.URL.Query()

	limit := 20
	offset := 0

	if l := q.Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	if o := q.Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = n
		}
	}

	var statusFilter *entity.InvoiceStatus
	if s := q.Get("status"); s != "" {
		st := entity.InvoiceStatus(s)
		statusFilter = &st
	}

	var customerFilter *string
	if c := q.Get("customer_id"); c != "" {
		customerFilter = &c
	}

	query := queryHandler.ListInvoicesQuery{
		TenantID:   tenantID,
		Status:     statusFilter,
		CustomerID: customerFilter,
		Limit:      limit,
		Offset:     offset,
	}

	invoices, total, err := deps.ListInvoices.Handle(r.Context(), query)
	if err != nil {
		status, msg := mapDomainError(err)
		respondError(w, status, msg)
		return
	}

	// ListInvoicesHandler returns []InvoiceReadModel, so we return them directly.
	respondJSON(w, http.StatusOK, map[string]any{
		"data":  invoices,
		"total": total,
	})
}

func GetInvoice(w http.ResponseWriter, r *http.Request) {
	// GET /api/v1/invoices/:id
	tenantID, ok := mustTenantID(w, r)
	if !ok {
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "invoice id is required")
		return
	}

	query := queryHandler.GetInvoiceQuery{
		InvoiceID: id,
		TenantID:  tenantID,
	}

	inv, err := deps.GetInvoice.Handle(r.Context(), query)
	if err != nil {
		status, msg := mapDomainError(err)
		respondError(w, status, msg)
		return
	}

	respondJSON(w, http.StatusOK, invoiceToDTO(inv))
}

func UpdateInvoice(w http.ResponseWriter, r *http.Request) {
	// PATCH /api/v1/invoices/:id (draft only) — scaffold
	id := chi.URLParam(r, "id")
	respondJSON(w, http.StatusOK, map[string]string{
		"message":    "update invoice — scaffold",
		"invoice_id": id,
	})
}

func IssueInvoice(w http.ResponseWriter, r *http.Request) {
	// POST /api/v1/invoices/:id/issue
	tenantID, ok := mustTenantID(w, r)
	if !ok {
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "invoice id is required")
		return
	}

	cmd := commandHandler.IssueInvoiceCommand{
		InvoiceID: id,
		TenantID:  tenantID,
	}

	inv, err := deps.IssueInvoice.Handle(r.Context(), cmd)
	if err != nil {
		status, msg := mapDomainError(err)
		respondError(w, status, msg)
		return
	}

	respondJSON(w, http.StatusOK, invoiceToDTO(inv))
}

func CancelInvoice(w http.ResponseWriter, r *http.Request) {
	// POST /api/v1/invoices/:id/cancel
	tenantID, ok := mustTenantID(w, r)
	if !ok {
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "invoice id is required")
		return
	}

	cmd := commandHandler.CancelInvoiceCommand{
		InvoiceID: id,
		TenantID:  tenantID,
	}

	if err := deps.CancelInvoice.Handle(r.Context(), cmd); err != nil {
		status, msg := mapDomainError(err)
		respondError(w, status, msg)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

func RecordPayment(w http.ResponseWriter, r *http.Request) {
	// POST /api/v1/invoices/:id/payments
	tenantID, ok := mustTenantID(w, r)
	if !ok {
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "invoice id is required")
		return
	}

	var req struct {
		Amount    int64  `json:"amount"`
		Currency  string `json:"currency"`
		Method    string `json:"method"`
		Reference string `json:"reference"`
	}

	if err := parseBody(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Amount <= 0 {
		respondError(w, http.StatusBadRequest, "amount must be positive")
		return
	}
	if req.Currency == "" {
		respondError(w, http.StatusBadRequest, "currency is required")
		return
	}
	if req.Method == "" {
		respondError(w, http.StatusBadRequest, "method is required")
		return
	}

	cmd := commandHandler.RecordPaymentCommand{
		InvoiceID: id,
		TenantID:  tenantID,
		Amount:    req.Amount,
		Currency:  req.Currency,
		Method:    req.Method,
		Reference: req.Reference,
		PaidAt:    time.Now(),
	}

	payment, err := deps.RecordPayment.Handle(r.Context(), cmd)
	if err != nil {
		status, msg := mapDomainError(err)
		respondError(w, status, msg)
		return
	}

	respondJSON(w, http.StatusCreated, paymentToDTO(payment))
}

func ListPayments(w http.ResponseWriter, r *http.Request) {
	// GET /api/v1/invoices/:id/payments
	tenantID, ok := mustTenantID(w, r)
	if !ok {
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "invoice id is required")
		return
	}

	query := queryHandler.ListPaymentsQuery{
		InvoiceID: id,
		TenantID:  tenantID,
	}

	payments, err := deps.ListPayments.Handle(r.Context(), query)
	if err != nil {
		status, msg := mapDomainError(err)
		respondError(w, status, msg)
		return
	}

	dtos := make([]paymentDTO, 0, len(payments))
	for _, p := range payments {
		dtos = append(dtos, paymentToDTO(p))
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"data": dtos,
	})
}

func DownloadPDF(w http.ResponseWriter, r *http.Request) {
	// GET /api/v1/invoices/:id/pdf — scaffold
	id := chi.URLParam(r, "id")
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=invoice-"+id+".pdf")
	respondJSON(w, http.StatusOK, map[string]string{
		"message":    "pdf download — scaffold",
		"invoice_id": id,
	})
}

// ========== Customers ==========

func CreateCustomer(w http.ResponseWriter, r *http.Request) {
	// POST /api/v1/customers
	tenantID, ok := mustTenantID(w, r)
	if !ok {
		return
	}

	var req struct {
		Name    string `json:"name"`
		Email   string `json:"email"`
		Phone   string `json:"phone"`
		Address struct {
			Line1    string `json:"line1"`
			Line2    string `json:"line2"`
			City     string `json:"city"`
			State    string `json:"state"`
			Postcode string `json:"postcode"`
			Country  string `json:"country"`
		} `json:"address"`
		TaxID string `json:"tax_id"`
	}

	if err := parseBody(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		respondError(w, http.StatusBadRequest, "customer name is required")
		return
	}

	addr := valueobject.Address{
		Line1:    req.Address.Line1,
		Line2:    req.Address.Line2,
		City:     req.Address.City,
		State:    req.Address.State,
		Postcode: req.Address.Postcode,
		Country:  req.Address.Country,
	}

	customer, err := entity.NewCustomer(uuid.NewString(), tenantID, req.Name, req.Email, addr)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	customer.Phone = req.Phone
	customer.TaxID = req.TaxID

	if err := deps.CustomerRepo.Save(r.Context(), customer); err != nil {
		status, msg := mapDomainError(err)
		respondError(w, status, msg)
		return
	}

	respondJSON(w, http.StatusCreated, customerToDTO(customer))
}

func ListCustomers(w http.ResponseWriter, r *http.Request) {
	// GET /api/v1/customers?limit=20&offset=0
	tenantID, ok := mustTenantID(w, r)
	if !ok {
		return
	}

	q := r.URL.Query()

	limit := 20
	offset := 0

	if l := q.Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	if o := q.Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = n
		}
	}

	query := queryHandler.ListCustomersQuery{
		TenantID: tenantID,
		Limit:    limit,
		Offset:   offset,
	}

	customers, err := deps.ListCustomers.Handle(r.Context(), query)
	if err != nil {
		status, msg := mapDomainError(err)
		respondError(w, status, msg)
		return
	}

	dtos := make([]customerDTO, 0, len(customers))
	for _, c := range customers {
		dtos = append(dtos, customerToDTO(c))
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"data": dtos,
	})
}

func GetCustomer(w http.ResponseWriter, r *http.Request) {
	// GET /api/v1/customers/:id
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "customer id is required")
		return
	}

	customer, err := deps.CustomerRepo.FindByID(r.Context(), id)
	if err != nil {
		status, msg := mapDomainError(err)
		respondError(w, status, msg)
		return
	}

	respondJSON(w, http.StatusOK, customerToDTO(customer))
}

// ========== Subscriptions (scaffolds) ==========

func CreateSubscription(w http.ResponseWriter, r *http.Request) {
	// POST /api/v1/subscriptions — scaffold
	var req struct {
		CustomerID string `json:"customer_id"`
		Frequency  string `json:"frequency"`
		Items      []struct {
			Description string `json:"description"`
			Quantity    int64  `json:"quantity"`
			UnitPrice   int64  `json:"unit_price"`
			TaxRateBPS  int64  `json:"tax_rate_bps"`
		} `json:"items"`
		NextBillingDate string `json:"next_billing_date"`
	}

	if err := parseBody(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]any{
		"message":  "create subscription — scaffold",
		"received": req,
	})
}

func ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	// GET /api/v1/subscriptions — scaffold
	respondJSON(w, http.StatusOK, map[string]any{
		"message": "list subscriptions — scaffold",
		"data":    []any{},
	})
}

func CancelSubscription(w http.ResponseWriter, r *http.Request) {
	// DELETE /api/v1/subscriptions/:id — scaffold
	id := chi.URLParam(r, "id")
	respondJSON(w, http.StatusOK, map[string]string{
		"message":         "cancel subscription — scaffold",
		"subscription_id": id,
	})
}

// ========== Audit (scaffold) ==========

func ListAuditEntries(w http.ResponseWriter, r *http.Request) {
	// GET /api/v1/audit?entity_type=invoice&entity_id=... — scaffold
	respondJSON(w, http.StatusOK, map[string]any{
		"message": "audit trail — scaffold",
		"data":    []any{},
	})
}
