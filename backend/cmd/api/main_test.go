package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/infrastructure/auth"
)

func protectedRouter(t *testing.T) (*chi.Mux, *auth.JWTTokenService) {
	t.Helper()
	tokenService := auth.NewJWTTokenService(auth.JWTConfig{Secret: "route-test-secret"})
	router := chi.NewRouter()
	router.Route("/api/v1", func(r chi.Router) {
		registerProtectedRoutes(r, tokenService, nil)
	})
	return router, tokenService
}

func routeRequest(t *testing.T, tokenService *auth.JWTTokenService, role, method, path string) *http.Request {
	t.Helper()
	token, err := tokenService.Generate(entity.TokenClaims{
		UserID: "user-1", TenantID: "tenant-1", Email: "user@example.com", Role: role,
	})
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	return req
}

func TestProtectedRoutesEnforceRolePolicy(t *testing.T) {
	router, tokenService := protectedRouter(t)

	t.Run("viewer cannot access invoice write route", func(t *testing.T) {
		req := routeRequest(t, tokenService, "viewer", http.MethodPatch, "/api/v1/invoices/invoice-1")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
		}
	})

	t.Run("member reaches supported write policy but endpoint remains disabled", func(t *testing.T) {
		req := routeRequest(t, tokenService, "member", http.MethodPatch, "/api/v1/invoices/invoice-1")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusNotImplemented {
			t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusNotImplemented, w.Body.String())
		}
	})

	t.Run("member cannot read administrator-only audit route", func(t *testing.T) {
		req := routeRequest(t, tokenService, "member", http.MethodGet, "/api/v1/audit")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
		}
	})

	t.Run("administrator reaches audit route and receives explicit disabled status", func(t *testing.T) {
		req := routeRequest(t, tokenService, "admin", http.MethodGet, "/api/v1/audit")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusNotImplemented {
			t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusNotImplemented, w.Body.String())
		}
	})
}
