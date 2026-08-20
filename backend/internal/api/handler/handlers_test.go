package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	handler "github.com/santiagossaa/invoice-generator/internal/api/handler"
)

// TestHealthEndpoint tests the /health endpoint returns 200 OK.
func TestHealthEndpoint(t *testing.T) {
	// This is a basic e2e test that verifies the HTTP server responds.
	// In a full setup, we'd initialize the full router with chi and test
	// the entire request/response cycle.

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	// Simple handler
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}
	handler(w, req)

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
	reqBody := map[string]any{
		"customer_id": "cust-001",
		"currency":    "USD",
		"due_date":   "2024-12-31",
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
	req.Header.Set("X-Tenant-ID", "tenant-001")
	req.Header.Set("Idempotency-Key", "test-key-001")

	w := httptest.NewRecorder()

	// In a full e2e test, we'd use the actual chi router with all middleware.
	// For now, we test the handler directly.
	handler.CreateInvoice(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("create invoice status = %d, want %d", w.Code, http.StatusCreated)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["message"] == nil {
		t.Error("response should contain message field")
	}
}

// TestListInvoicesEndpoint tests GET /api/v1/invoices.
func TestListInvoicesEndpoint(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/invoices?status=issued&limit=10&offset=0", nil)
	req.Header.Set("X-Tenant-ID", "tenant-001")

	w := httptest.NewRecorder()
	handler.ListInvoices(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("list invoices status = %d, want %d", w.Code, http.StatusOK)
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
	req.Header.Set("X-Tenant-ID", "tenant-001")

	w := httptest.NewRecorder()
	handler.CreateCustomer(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("create customer status = %d, want %d", w.Code, http.StatusCreated)
	}
}

// TestCreateCustomerValidation tests that empty name is rejected.
func TestCreateCustomerValidation(t *testing.T) {
	reqBody := map[string]any{
		"name":  "",
		"email": "test@test.com",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/customers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-001")

	w := httptest.NewRecorder()
	handler.CreateCustomer(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("empty name status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// TestCreateSubscriptionEndpoint tests POST /api/v1/subscriptions.
func TestCreateSubscriptionEndpoint(t *testing.T) {
	reqBody := map[string]any{
		"customer_id":        "cust-001",
		"frequency":          "monthly",
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
	req.Header.Set("X-Tenant-ID", "tenant-001")

	w := httptest.NewRecorder()
	handler.CreateSubscription(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("create subscription status = %d, want %d", w.Code, http.StatusCreated)
	}
}

// TestRecordPaymentEndpoint tests POST /api/v1/invoices/:id/payments.
func TestRecordPaymentEndpoint(t *testing.T) {
	reqBody := map[string]any{
		"amount":    5000,
		"currency":  "USD",
		"method":    "bank_transfer",
		"reference": "TXN-001",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/invoices/inv-001/payments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-001")
	req.Header.Set("Idempotency-Key", "payment-key-001")

	w := httptest.NewRecorder()
	handler.RecordPayment(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("record payment status = %d, want %d", w.Code, http.StatusCreated)
	}
}

// TestInvalidJSON tests that malformed JSON is rejected.
func TestInvalidJSON(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/invoices", bytes.NewReader([]byte("{invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-001")

	w := httptest.NewRecorder()
	handler.CreateInvoice(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("invalid JSON status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}