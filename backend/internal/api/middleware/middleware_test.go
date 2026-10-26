package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	apimiddleware "github.com/santiagossaa/invoice-generator/internal/api/middleware"
	"github.com/santiagossaa/invoice-generator/internal/ctxkey"
	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/domain/port"
	"github.com/santiagossaa/invoice-generator/internal/infrastructure/auth"
)

// testTokenService creates a JWTTokenService for tests.
func testTokenService() *auth.JWTTokenService {
	return auth.NewJWTTokenService(auth.JWTConfig{
		Secret:      "test-secret-key-for-tests",
		TokenExpiry: 3600000000000, // 1 hour in nanoseconds
	})
}

// generateTestToken creates a valid JWT token for testing.
func generateTestToken(t *testing.T, ts *auth.JWTTokenService, tenantID, userID, role string) string {
	t.Helper()
	token, err := ts.Generate(entity.TokenClaims{
		UserID:   userID,
		TenantID: tenantID,
		Email:    "test@test.com",
		Role:     role,
	})
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	return token.AccessToken
}

func TestTenantContext(t *testing.T) {
	t.Run("does not trust tenant header without JWT context", func(t *testing.T) {
		handler := apimiddleware.TenantContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("handler should not be called without a tenant claim")
		}))

		req := httptest.NewRequest("GET", "/api/v1/invoices", nil)
		req.Header.Set("X-Tenant-ID", "tenant-123")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
		}
	})

	t.Run("rejects missing tenant claim", func(t *testing.T) {
		handler := apimiddleware.TenantContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("handler should not be called for missing tenant claim")
		}))

		req := httptest.NewRequest("GET", "/api/v1/invoices", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
		}
	})

	t.Run("accepts tenant_id from JWT context", func(t *testing.T) {
		var capturedTenantID string
		handler := apimiddleware.TenantContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if v, ok := r.Context().Value(apimiddleware.TenantIDKey).(string); ok {
				capturedTenantID = v
			}
		}))

		// Simulate JWTAuth having already set tenant_id in context
		req := httptest.NewRequest("GET", "/api/v1/invoices", nil)
		ctx := context.WithValue(req.Context(), apimiddleware.TenantIDKey, "tenant-from-jwt")
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if capturedTenantID != "tenant-from-jwt" {
			t.Errorf("tenant_id = %q, want tenant-from-jwt", capturedTenantID)
		}
	})
}

func TestJWTAuth(t *testing.T) {
	ts := testTokenService()

	t.Run("missing authorization header", func(t *testing.T) {
		authMiddleware := apimiddleware.JWTAuth(ts)
		handler := authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("handler should not be called without auth")
		}))

		req := httptest.NewRequest("GET", "/api/v1/invoices", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
		}
	})

	t.Run("invalid auth format", func(t *testing.T) {
		authMiddleware := apimiddleware.JWTAuth(ts)
		handler := authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("handler should not be called with invalid auth")
		}))

		req := httptest.NewRequest("GET", "/api/v1/invoices", nil)
		req.Header.Set("Authorization", "Basic abc123")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
		}
	})

	t.Run("invalid token is rejected", func(t *testing.T) {
		authMiddleware := apimiddleware.JWTAuth(ts)
		handler := authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("handler should not be called with invalid token")
		}))

		req := httptest.NewRequest("GET", "/api/v1/invoices", nil)
		req.Header.Set("Authorization", "Bearer invalid.token.here")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
		}
	})

	t.Run("valid token passes through and sets context", func(t *testing.T) {
		authMiddleware := apimiddleware.JWTAuth(ts)
		token := generateTestToken(t, ts, "tenant-001", "user-001", "admin")

		var capturedTenantID, capturedUserID, capturedRole string
		handler := authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedTenantID, _ = r.Context().Value(apimiddleware.TenantIDKey).(string)
			capturedUserID, _ = r.Context().Value(apimiddleware.UserIDKey).(string)
			capturedRole, _ = r.Context().Value(apimiddleware.UserRoleKey).(string)
		}))

		req := httptest.NewRequest("GET", "/api/v1/invoices", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
		}
		if capturedTenantID != "tenant-001" {
			t.Errorf("tenant_id = %q, want tenant-001", capturedTenantID)
		}
		if capturedUserID != "user-001" {
			t.Errorf("user_id = %q, want user-001", capturedUserID)
		}
		if capturedRole != "admin" {
			t.Errorf("role = %q, want admin", capturedRole)
		}
	})

}

func TestJWTAuthTenantContext(t *testing.T) {
	ts := testTokenService()
	token := generateTestToken(t, ts, "tenant-001", "user-001", "member")

	for _, tenantHeader := range []string{"", "tenant-from-untrusted-header"} {
		t.Run("tenant header "+tenantHeader, func(t *testing.T) {
			var capturedTenantID string
			protected := apimiddleware.JWTAuth(ts)(apimiddleware.TenantContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedTenantID, _ = r.Context().Value(apimiddleware.TenantIDKey).(string)
				w.WriteHeader(http.StatusNoContent)
			})))

			req := httptest.NewRequest("GET", "/api/v1/invoices", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			if tenantHeader != "" {
				req.Header.Set("X-Tenant-ID", tenantHeader)
			}
			w := httptest.NewRecorder()
			protected.ServeHTTP(w, req)

			if w.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusNoContent, w.Body.String())
			}
			if capturedTenantID != "tenant-001" {
				t.Errorf("tenant_id = %q, want tenant-001", capturedTenantID)
			}
		})
	}
}

func TestRequireRole(t *testing.T) {
	t.Run("admin has access to everything", func(t *testing.T) {
		roleMiddleware := apimiddleware.RequireRole("admin")
		called := false
		handler := roleMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
		}))

		ctx := context.WithValue(context.Background(), apimiddleware.UserRoleKey, "admin")
		req := httptest.NewRequest("GET", "/api/v1/invoices", nil).WithContext(ctx)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if !called {
			t.Error("admin should have access")
		}
	})

	t.Run("member cannot access admin-only endpoint", func(t *testing.T) {
		roleMiddleware := apimiddleware.RequireRole("admin")
		handler := roleMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("member should not access admin endpoint")
		}))

		ctx := context.WithValue(context.Background(), apimiddleware.UserRoleKey, "member")
		req := httptest.NewRequest("GET", "/api/v1/invoices", nil).WithContext(ctx)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
		}
	})

	t.Run("missing role is unauthorized", func(t *testing.T) {
		roleMiddleware := apimiddleware.RequireRole("admin")
		handler := roleMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("handler should not be called without role")
		}))

		req := httptest.NewRequest("GET", "/api/v1/invoices", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
		}
	})
}

func TestRequireAnyRole(t *testing.T) {
	for _, test := range []struct {
		role       string
		wantStatus int
	}{
		{role: "admin", wantStatus: http.StatusNoContent},
		{role: "member", wantStatus: http.StatusNoContent},
		{role: "viewer", wantStatus: http.StatusNoContent},
		{role: "unknown", wantStatus: http.StatusForbidden},
	} {
		t.Run(test.role, func(t *testing.T) {
			handler := apimiddleware.RequireAnyRole("admin", "member", "viewer")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}))
			ctx := context.WithValue(context.Background(), apimiddleware.UserRoleKey, test.role)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx))
			if w.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, test.wantStatus)
			}
		})
	}
}

type memoryIdempotencyRepository struct {
	mu      sync.Mutex
	records map[string]*port.IdempotencyRecord
}

func newMemoryIdempotencyRepository() *memoryIdempotencyRepository {
	return &memoryIdempotencyRepository{records: make(map[string]*port.IdempotencyRecord)}
}

func idempotencyRecordKey(record *port.IdempotencyRecord) string {
	return record.TenantID + ":" + record.Key
}

func cloneIdempotencyRecord(record *port.IdempotencyRecord) *port.IdempotencyRecord {
	clone := *record
	clone.ResponseBody = append([]byte(nil), record.ResponseBody...)
	clone.ResponseHeaders = append([]byte(nil), record.ResponseHeaders...)
	return &clone
}

func (r *memoryIdempotencyRepository) Reserve(_ context.Context, record *port.IdempotencyRecord) (*port.IdempotencyRecord, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := idempotencyRecordKey(record)
	if existing, ok := r.records[key]; ok {
		return cloneIdempotencyRecord(existing), false, nil
	}
	claimed := cloneIdempotencyRecord(record)
	claimed.State = "processing"
	r.records[key] = claimed
	return cloneIdempotencyRecord(claimed), true, nil
}

func (r *memoryIdempotencyRepository) Complete(_ context.Context, record *port.IdempotencyRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := idempotencyRecordKey(record)
	r.records[key] = cloneIdempotencyRecord(record)
	return nil
}

func idempotencyRequest(method, path, tenantID, key, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Idempotency-Key", key)
	ctx := context.WithValue(req.Context(), ctxkey.TenantIDKey, tenantID)
	return req.WithContext(ctx)
}

func TestIdempotency(t *testing.T) {
	t.Run("GET passes through", func(t *testing.T) {
		called := false
		handler := apimiddleware.Idempotency(newMemoryIdempotencyRepository())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
		}))

		req := httptest.NewRequest("GET", "/api/v1/invoices", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if !called {
			t.Error("GET should pass through idempotency middleware")
		}
	})

	t.Run("POST without key passes through", func(t *testing.T) {
		called := false
		handler := apimiddleware.Idempotency(newMemoryIdempotencyRepository())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
		}))

		req := httptest.NewRequest("POST", "/api/v1/invoices", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if !called {
			t.Error("POST without idempotency key should pass through")
		}
	})

	t.Run("same key and payload replays the original response", func(t *testing.T) {
		repo := newMemoryIdempotencyRepository()
		calls := 0
		handler := apimiddleware.Idempotency(repo)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Location", "/api/v1/invoices/invoice-1")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"invoice-1"}`))
		}))

		first := httptest.NewRecorder()
		handler.ServeHTTP(first, idempotencyRequest(http.MethodPost, "/api/v1/invoices", "tenant-1", "key-123", `{"customer_id":"customer-1"}`))
		second := httptest.NewRecorder()
		handler.ServeHTTP(second, idempotencyRequest(http.MethodPost, "/api/v1/invoices", "tenant-1", "key-123", `{"customer_id":"customer-1"}`))

		if calls != 1 {
			t.Fatalf("handler called %d times, want 1", calls)
		}
		if second.Code != http.StatusCreated || second.Body.String() != first.Body.String() ||
			second.Header().Get("Location") != "/api/v1/invoices/invoice-1" {
			t.Fatalf("replayed response differs: status=%d body=%q headers=%v", second.Code, second.Body.String(), second.Header())
		}
	})

	t.Run("same DELETE key replays the original response", func(t *testing.T) {
		repo := newMemoryIdempotencyRepository()
		calls := 0
		handler := apimiddleware.Idempotency(repo)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"cancelled"}`))
		}))

		first := httptest.NewRecorder()
		handler.ServeHTTP(first, idempotencyRequest(http.MethodDelete, "/api/v1/subscriptions/sub-1", "tenant-1", "cancel-key", ""))
		second := httptest.NewRecorder()
		handler.ServeHTTP(second, idempotencyRequest(http.MethodDelete, "/api/v1/subscriptions/sub-1", "tenant-1", "cancel-key", ""))

		if calls != 1 {
			t.Fatalf("handler called %d times, want 1", calls)
		}
		if second.Code != http.StatusOK || second.Body.String() != first.Body.String() {
			t.Fatalf("replayed response differs: status=%d body=%q", second.Code, second.Body.String())
		}
	})

	t.Run("same key with a different payload conflicts", func(t *testing.T) {
		repo := newMemoryIdempotencyRepository()
		calls := 0
		handler := apimiddleware.Idempotency(repo)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.WriteHeader(http.StatusCreated)
		}))
		first := httptest.NewRecorder()
		handler.ServeHTTP(first, idempotencyRequest(http.MethodPost, "/api/v1/invoices", "tenant-1", "key-123", `{"customer_id":"one"}`))
		second := httptest.NewRecorder()
		handler.ServeHTTP(second, idempotencyRequest(http.MethodPost, "/api/v1/invoices", "tenant-1", "key-123", `{"customer_id":"two"}`))

		if calls != 1 || second.Code != http.StatusConflict {
			t.Fatalf("calls=%d status=%d, want calls=1 and status=%d", calls, second.Code, http.StatusConflict)
		}
	})

	t.Run("same key is isolated per tenant", func(t *testing.T) {
		repo := newMemoryIdempotencyRepository()
		calls := 0
		handler := apimiddleware.Idempotency(repo)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.WriteHeader(http.StatusCreated)
		}))
		for _, tenantID := range []string{"tenant-1", "tenant-2"} {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, idempotencyRequest(http.MethodPost, "/api/v1/invoices", tenantID, "key-123", `{"customer_id":"one"}`))
		}
		if calls != 2 {
			t.Fatalf("handler called %d times, want 2", calls)
		}
	})

	t.Run("processing duplicate conflicts instead of running twice", func(t *testing.T) {
		repo := newMemoryIdempotencyRepository()
		req := idempotencyRequest(http.MethodPost, "/api/v1/invoices", "tenant-1", "key-123", `{}`)
		firstHandler := apimiddleware.Idempotency(repo)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("simulate interrupted request")
		}))
		func() {
			defer func() {
				if recover() == nil {
					t.Fatal("expected simulated panic")
				}
			}()
			firstHandler.ServeHTTP(httptest.NewRecorder(), req)
		}()

		handler := apimiddleware.Idempotency(repo)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not run while a matching request is processing")
		}))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusConflict)
		}
	})
}

func TestLoginRateLimit(t *testing.T) {
	calls := 0
	handler := apimiddleware.CaptureClientAddress(apimiddleware.LoginRateLimit(2, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	})))

	for attempt := 0; attempt < 3; attempt++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = "192.0.2.10:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if attempt < 2 && w.Code != http.StatusNoContent {
			t.Fatalf("attempt %d status = %d, want %d", attempt+1, w.Code, http.StatusNoContent)
		}
		if attempt == 2 && (w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") == "") {
			t.Fatalf("limited status = %d retry-after=%q", w.Code, w.Header().Get("Retry-After"))
		}
	}
	if calls != 2 {
		t.Fatalf("handler called %d times, want 2", calls)
	}
}
