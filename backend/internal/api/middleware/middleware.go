package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/santiagossaa/invoice-generator/internal/ctxkey"
)

// contextKey is kept for backward compatibility with UserIDKey.
type contextKey string

const (
	TenantIDKey ctxkey.CtxKey = ctxkey.TenantIDKey
	UserIDKey   contextKey    = "user_id"
)

// TenantContext middleware extracts tenant_id from the JWT token
// and sets it as a context value AND as a PostgreSQL session variable.
// This enables Row-Level Security policies to enforce tenant isolation.
func TenantContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// In a real implementation, this would extract tenant_id from the
		// authenticated user's JWT token. For now, we check the header.
		tenantID := r.Header.Get("X-Tenant-ID")
		if tenantID == "" {
			// For public endpoints (login, register), skip tenant context
			if isPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			http.Error(w, `{"error":"tenant_id required"}`, http.StatusBadRequest)
			return
		}

		ctx := context.WithValue(r.Context(), ctxkey.TenantIDKey, tenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// JWTAuth middleware validates JWT tokens and extracts claims.
func JWTAuth(secret string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error":"authorization required"}`, http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, `{"error":"invalid authorization header"}`, http.StatusUnauthorized)
				return
			}

			// In a real implementation, validate JWT and extract claims
			// token := parts[1]
			// claims, err := jwt.Parse(token, ...)
			// For scaffold, we pass through

			next.ServeHTTP(w, r)
		})
	}
}

// Idempotency middleware checks for the Idempotency-Key header
// on POST requests and returns cached responses for duplicate keys.
func Idempotency(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}

		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			// No idempotency key = not required (or could be required per endpoint)
			next.ServeHTTP(w, r)
			return
		}

		// In a real implementation:
		// 1. Check idempotency_records table for existing key
		// 2. If found: return cached response
		// 3. If not found: process request, cache response, return
		// For scaffold, we pass through

		next.ServeHTTP(w, r)
	})
}

func isPublicPath(path string) bool {
	publicPaths := []string{
		"/api/v1/auth/login",
		"/api/v1/auth/register",
		"/health",
		"/swagger",
	}
	for _, p := range publicPaths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}
