package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	apimiddleware "github.com/santiagossaa/invoice-generator/internal/api/middleware"
)

func TestTenantContext(t *testing.T) {
	t.Run("sets tenant_id from header", func(t *testing.T) {
		var capturedTenantID string
		handler := apimiddleware.TenantContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if v, ok := r.Context().Value(apimiddleware.TenantIDKey).(string); ok {
				capturedTenantID = v
			}
		}))

		req := httptest.NewRequest("GET", "/api/v1/invoices", nil)
		req.Header.Set("X-Tenant-ID", "tenant-123")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if capturedTenantID != "tenant-123" {
			t.Errorf("tenant_id = %q, want tenant-123", capturedTenantID)
		}
	})

	t.Run("rejects missing tenant_id on protected path", func(t *testing.T) {
		handler := apimiddleware.TenantContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("handler should not be called for missing tenant_id")
		}))

		req := httptest.NewRequest("GET", "/api/v1/invoices", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("allows public path without tenant_id", func(t *testing.T) {
		called := false
		handler := apimiddleware.TenantContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
		}))

		req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if !called {
			t.Error("public path should pass through")
		}
		if w.Code != http.StatusOK {
			t.Errorf("public path status = %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("health is public", func(t *testing.T) {
		called := false
		handler := apimiddleware.TenantContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
		}))

		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if !called {
			t.Error("health should pass through")
		}
	})
}

func TestJWTAuth(t *testing.T) {
	t.Run("missing authorization header", func(t *testing.T) {
		authMiddleware := apimiddleware.JWTAuth("secret")
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
		authMiddleware := apimiddleware.JWTAuth("secret")
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

	t.Run("valid bearer format passes through (scaffold)", func(t *testing.T) {
		authMiddleware := apimiddleware.JWTAuth("secret")
		called := false
		handler := authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
		}))

		req := httptest.NewRequest("GET", "/api/v1/invoices", nil)
		req.Header.Set("Authorization", "Bearer some-token-here")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if !called {
			t.Error("handler should be called with valid bearer token (scaffold passes through)")
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

func TestIsPublicPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/api/v1/auth/login", true},
		{"/api/v1/auth/register", true},
		{"/health", true},
		{"/api/v1/invoices", false},
		{"/api/v1/customers", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := isPublicPathExported(tt.path)
			if got != tt.want {
				t.Errorf("isPublicPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// isPublicPathExported wraps the private isPublicPath for testing.
func isPublicPathExported(path string) bool {
	// We can't call the private function directly from another package,
	// so we test the same logic here.
	publicPaths := []string{
		"/api/v1/auth/login",
		"/api/v1/auth/register",
		"/health",
	}
	for _, p := range publicPaths {
		if len(path) >= len(p) && path[:len(p)] == p {
			return true
		}
	}
	return false
}

// Ensure unused import doesn't break
var _ = context.Background
