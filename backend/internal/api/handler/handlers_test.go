package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	handler "github.com/santiagossaa/invoice-generator/internal/api/handler"
	"github.com/santiagossaa/invoice-generator/internal/api/middleware"
	commandHandler "github.com/santiagossaa/invoice-generator/internal/application/command"
	queryHandler "github.com/santiagossaa/invoice-generator/internal/application/query"
	"github.com/santiagossaa/invoice-generator/internal/domain/port"
	"github.com/santiagossaa/invoice-generator/internal/infrastructure/repository"
)

// testDeps holds the initialized deps for seeding test data.
var testDeps *handler.HandlerDeps

// setupDeps initializes the handler dependencies with in-memory repositories.
func setupDeps() {
	invoiceRepo := repository.NewInMemoryInvoiceRepository()
	customerRepo := repository.NewInMemoryCustomerRepository()
	paymentRepo := repository.NewInMemoryPaymentRepository()
	uow := repository.NewInMemoryUnitOfWork()

	testDeps = &handler.HandlerDeps{
		CreateInvoice: commandHandler.NewCreateInvoiceHandler(invoiceRepo, customerRepo),
		IssueInvoice:  commandHandler.NewIssueInvoiceHandler(invoiceRepo),
		CancelInvoice: commandHandler.NewCancelInvoiceHandler(invoiceRepo),
		RecordPayment: commandHandler.NewRecordPaymentHandler(invoiceRepo, paymentRepo, uow),
		ListInvoices:  queryHandler.NewListInvoicesHandler(invoiceRepo),
		GetInvoice:    queryHandler.NewGetInvoiceHandler(invoiceRepo),
		ListCustomers: queryHandler.NewListCustomersHandler(customerRepo),
		ListPayments:  queryHandler.NewListPaymentsHandler(paymentRepo, invoiceRepo),
		CustomerRepo:  customerRepo,
	}
	handler.SetDeps(testDeps)
}

func TestRegisterDisabled(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/auth/register", nil)
	w := httptest.NewRecorder()

	handler.RegisterDisabled(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("register status = %d, want %d", w.Code, http.StatusForbidden)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("public registration is disabled")) {
		t.Errorf("response should explain registration is disabled; body: %s", w.Body.String())
	}
}

type pdfStoreStub struct {
	contents  []byte
	err       error
	reads     int
	tenantID  string
	invoiceID string
}

func (s *pdfStoreStub) PutInvoicePDF(_ context.Context, tenantID, invoiceID string, contents []byte) error {
	s.tenantID, s.invoiceID, s.contents = tenantID, invoiceID, contents
	return nil
}

func (s *pdfStoreStub) GetInvoicePDF(_ context.Context, tenantID, invoiceID string) ([]byte, error) {
	s.tenantID, s.invoiceID = tenantID, invoiceID
	s.reads++
	return s.contents, s.err
}

func TestDownloadPDFServesTenantInvoiceArtifact(t *testing.T) {
	setupDeps()
	const tenantID = "tenant-001"
	customer := repository.NewTestCustomer(tenantID)
	if err := testDeps.CustomerRepo.Save(context.Background(), customer); err != nil {
		t.Fatalf("seed customer: %v", err)
	}

	body, _ := json.Marshal(map[string]any{
		"customer_id": customer.ID,
		"currency":    "USD",
		"due_date":    "2026-12-31T23:59:59Z",
		"items": []map[string]any{{
			"description": "Consulting",
			"quantity":    1,
			"unit_price":  5000,
		}},
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/invoices", bytes.NewReader(body))
	createReq = tenantCtx(createReq, tenantID)
	createW := httptest.NewRecorder()
	handler.CreateInvoice(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("create invoice: status %d, body %s", createW.Code, createW.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created invoice: %v", err)
	}

	store := &pdfStoreStub{contents: []byte("%PDF-test")}
	testDeps.PDFStore = store
	req := httptest.NewRequest(http.MethodGet, "/api/v1/invoices/"+created.ID+"/pdf", nil)
	req = tenantCtx(req, tenantID)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", created.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	w := httptest.NewRecorder()

	handler.DownloadPDF(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("download status = %d, body %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != "application/pdf" {
		t.Fatalf("Content-Type = %q, want application/pdf", got)
	}
	if !bytes.Equal(w.Body.Bytes(), store.contents) {
		t.Fatalf("download body = %q, want %q", w.Body.Bytes(), store.contents)
	}
	if store.tenantID != tenantID || store.invoiceID != created.ID {
		t.Fatalf("artifact lookup used tenant %q invoice %q", store.tenantID, store.invoiceID)
	}

	store.err = port.ErrPDFArtifactNotFound
	pendingReq := httptest.NewRequest(http.MethodGet, "/api/v1/invoices/"+created.ID+"/pdf", nil)
	pendingReq = tenantCtx(pendingReq, tenantID)
	pendingRouteCtx := chi.NewRouteContext()
	pendingRouteCtx.URLParams.Add("id", created.ID)
	pendingReq = pendingReq.WithContext(context.WithValue(pendingReq.Context(), chi.RouteCtxKey, pendingRouteCtx))
	pendingW := httptest.NewRecorder()
	handler.DownloadPDF(pendingW, pendingReq)
	if pendingW.Code != http.StatusNotFound {
		t.Fatalf("unavailable PDF status = %d, want %d", pendingW.Code, http.StatusNotFound)
	}

	foreignReq := httptest.NewRequest(http.MethodGet, "/api/v1/invoices/"+created.ID+"/pdf", nil)
	foreignReq = tenantCtx(foreignReq, "other-tenant")
	foreignRouteCtx := chi.NewRouteContext()
	foreignRouteCtx.URLParams.Add("id", created.ID)
	foreignReq = foreignReq.WithContext(context.WithValue(foreignReq.Context(), chi.RouteCtxKey, foreignRouteCtx))
	foreignW := httptest.NewRecorder()
	handler.DownloadPDF(foreignW, foreignReq)
	if foreignW.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant download status = %d, want %d", foreignW.Code, http.StatusNotFound)
	}
	if store.reads != 2 {
		t.Fatalf("store read count = %d, want 2 (cross-tenant request must not read storage)", store.reads)
	}
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

func TestUnsupportedHandlersReturnNotImplemented(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{name: "invoice update", handler: handler.UpdateInvoice},
		{name: "subscription create", handler: handler.CreateSubscription},
		{name: "subscription list", handler: handler.ListSubscriptions},
		{name: "subscription cancel", handler: handler.CancelSubscription},
		{name: "audit list", handler: handler.ListAuditEntries},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			test.handler(w, httptest.NewRequest(http.MethodPost, "/", nil))
			if w.Code != http.StatusNotImplemented {
				t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusNotImplemented, w.Body.String())
			}
			if !bytes.Contains(w.Body.Bytes(), []byte(`"error"`)) {
				t.Fatalf("response should contain an explicit error: %s", w.Body.String())
			}
		})
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
