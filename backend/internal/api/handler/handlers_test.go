package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	handler "github.com/santiagossaa/invoice-generator/internal/api/handler"
	"github.com/santiagossaa/invoice-generator/internal/api/middleware"
	commandHandler "github.com/santiagossaa/invoice-generator/internal/application/command"
	queryHandler "github.com/santiagossaa/invoice-generator/internal/application/query"
	"github.com/santiagossaa/invoice-generator/internal/infrastructure/repository"
)

// testDeps holds the initialized deps for seeding test data.
var testDeps *handler.HandlerDeps

// setupDeps initializes the handler dependencies with in-memory repositories.
func setupDeps() {
	invoiceRepo := repository.NewInMemoryInvoiceRepository()
	customerRepo := repository.NewInMemoryCustomerRepository()
	paymentRepo := repository.NewInMemoryPaymentRepository()

	testDeps = &handler.HandlerDeps{
		CreateInvoice: commandHandler.NewCreateInvoiceHandler(invoiceRepo, customerRepo),
		IssueInvoice:  commandHandler.NewIssueInvoiceHandler(invoiceRepo),
		CancelInvoice: commandHandler.NewCancelInvoiceHandler(invoiceRepo),
		RecordPayment: commandHandler.NewRecordPaymentHandler(invoiceRepo, paymentRepo),
		ListInvoices:  queryHandler.NewListInvoicesHandler(invoiceRepo),
		GetInvoice:    queryHandler.NewGetInvoiceHandler(invoiceRepo),
		ListCustomers: queryHandler.NewListCustomersHandler(customerRepo),
		ListPayments:  queryHandler.NewListPaymentsHandler(paymentRepo, invoiceRepo),
		CustomerRepo:  customerRepo,
	}
	handler.SetDeps(testDeps)
}

// tenantCtx returns a request with the tenant ID set in context, simulating middleware.
func tenantCtx(r *http.Request, tenantID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.TenantIDKey, tenantID)
	return r.WithContext(ctx)
}

// TestHealthEndpoint tests the /health endpoint returns 200 OK.
func TestHealthEndpoint(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	h := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}
	h(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("health status = %d, want %d", w.Code, http.StatusOK)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %q, want ok", body["status"])
	}
}

// TestCreateInvoiceEndpoint tests the POST /api/v1/invoices endpoint.
func TestCreateInvoiceEndpoint(t *testing.T) {
	setupDeps()

	// Seed a customer
	customer := repository.NewTestCustomer("tenant-001")
	_ = testDeps.CustomerRepo.Save(context.Background(), customer)

	reqBody := map[string]any{
		"customer_id": customer.ID,
		"currency":    "USD",
		"due_date":    "2024-12-31T23:59:59Z",
		"items": []map[string]any{
			{
				"description":  "Consulting",
				"quantity":     10,
				"unit_price":   5000,
				"tax_rate_bps": 1900,
			},
		},
		"notes": "Test invoice",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/invoices", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = tenantCtx(req, "tenant-001")

	w := httptest.NewRecorder()
	handler.CreateInvoice(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("create invoice status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
}

// TestListInvoicesEndpoint tests GET /api/v1/invoices.
func TestListInvoicesEndpoint(t *testing.T) {
	setupDeps()

	// Seed a customer and invoice
	customer := repository.NewTestCustomer("tenant-001")
	_ = testDeps.CustomerRepo.Save(context.Background(), customer)

	reqBody := map[string]any{
		"customer_id": customer.ID,
		"currency":    "USD",
		"due_date":    "2024-12-31T23:59:59Z",
		"items": []map[string]any{
			{
				"description":  "Test item",
				"quantity":     1,
				"unit_price":   1000,
				"tax_rate_bps": 0,
			},
		},
	}
	body, _ := json.Marshal(reqBody)

	createReq := httptest.NewRequest("POST", "/api/v1/invoices", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createReq = tenantCtx(createReq, "tenant-001")
	createW := httptest.NewRecorder()
	handler.CreateInvoice(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("seed invoice failed: %d - %s", createW.Code, createW.Body.String())
	}

	// Test listing
	req := httptest.NewRequest("GET", "/api/v1/invoices?limit=10&offset=0", nil)
	req = tenantCtx(req, "tenant-001")

	w := httptest.NewRecorder()
	handler.ListInvoices(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("list invoices status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["data"] == nil {
		t.Error("response should contain data field")
	}
}

// TestCreateCustomerEndpoint tests POST /api/v1/customers.
func TestCreateCustomerEndpoint(t *testing.T) {
	setupDeps()

	reqBody := map[string]any{
		"name":  "ACME Corp",
		"email": "billing@acme.com",
		"phone": "+57 300 123 4567",
		"address": map[string]string{
			"line1":    "Calle 100 #50-25",
			"city":     "Bogota",
			"country":  "CO",
			"postcode": "110111",
		},
		"tax_id": "900123456-7",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/customers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = tenantCtx(req, "tenant-001")

	w := httptest.NewRecorder()
	handler.CreateCustomer(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("create customer status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
}

// TestCreateCustomerValidation tests that empty name is rejected.
func TestCreateCustomerValidation(t *testing.T) {
	setupDeps()

	reqBody := map[string]any{
		"name":  "",
		"email": "test@test.com",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/customers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = tenantCtx(req, "tenant-001")

	w := httptest.NewRecorder()
	handler.CreateCustomer(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("empty name status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// TestCreateSubscriptionEndpoint tests POST /api/v1/subscriptions (scaffold).
func TestCreateSubscriptionEndpoint(t *testing.T) {
	setupDeps()

	reqBody := map[string]any{
		"customer_id":       "cust-001",
		"frequency":         "monthly",
		"next_billing_date": "2024-09-01",
		"items": []map[string]any{
			{
				"description":  "SaaS Subscription",
				"quantity":     1,
				"unit_price":   9900,
				"tax_rate_bps": 1900,
			},
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/subscriptions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = tenantCtx(req, "tenant-001")

	w := httptest.NewRecorder()
	handler.CreateSubscription(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("create subscription status = %d, want %d", w.Code, http.StatusCreated)
	}
}

// TestInvalidJSON tests that malformed JSON is rejected.
func TestInvalidJSON(t *testing.T) {
	setupDeps()

	req := httptest.NewRequest("POST", "/api/v1/invoices", bytes.NewReader([]byte("{invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req = tenantCtx(req, "tenant-001")

	w := httptest.NewRecorder()
	handler.CreateInvoice(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("invalid JSON status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
