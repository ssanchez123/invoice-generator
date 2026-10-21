package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	apimiddleware "github.com/santiagossaa/invoice-generator/internal/api/middleware"
	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
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

func TestIdempotency(t *testing.T) {
	t.Run("non-POST passes through", func(t *testing.T) {
		called := false
		handler := apimiddleware.Idempotency(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		handler := apimiddleware.Idempotency(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
		}))

		req := httptest.NewRequest("POST", "/api/v1/invoices", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if !called {
			t.Error("POST without idempotency key should pass through")
		}
	})

	t.Run("POST with key passes through (scaffold)", func(t *testing.T) {
		called := false
		handler := apimiddleware.Idempotency(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
		}))

		req := httptest.NewRequest("POST", "/api/v1/invoices", nil)
		req.Header.Set("Idempotency-Key", "key-123")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if !called {
			t.Error("POST with idempotency key should pass through (scaffold)")
		}
	})
}

// Ensure unused import doesn't break
var _ = context.Background
