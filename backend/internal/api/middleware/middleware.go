package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/santiagossaa/invoice-generator/internal/ctxkey"
	"github.com/santiagossaa/invoice-generator/internal/domain/port"
)

// contextKey is kept for backward compatibility with UserIDKey.
type contextKey string

const (
	TenantIDKey ctxkey.CtxKey = ctxkey.TenantIDKey
	UserIDKey   contextKey    = "user_id"
	UserRoleKey contextKey    = "user_role"
	EmailKey    contextKey    = "email"
)

// JWTAuth middleware validates JWT tokens and extracts claims into the request context.
// It sets tenant_id, user_id, role, and email in the context for downstream handlers.
func JWTAuth(tokenService port.TokenService) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeAuthError(w, "authorization required", http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				writeAuthError(w, "invalid authorization header format, expected 'Bearer <token>'", http.StatusUnauthorized)
				return
			}

			token := strings.TrimSpace(parts[1])
			if token == "" {
				writeAuthError(w, "empty token", http.StatusUnauthorized)
				return
			}

			claims, err := tokenService.Validate(token)
			if err != nil {
				writeAuthError(w, err.Error(), http.StatusUnauthorized)
				return
			}

			// Inject claims into context
			ctx := r.Context()
			ctx = context.WithValue(ctx, ctxkey.TenantIDKey, claims.TenantID)
			ctx = context.WithValue(ctx, UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, UserRoleKey, claims.Role)
			ctx = context.WithValue(ctx, EmailKey, claims.Email)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// TenantContext requires a tenant ID set by JWTAuth.
func TenantContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := r.Context().Value(ctxkey.TenantIDKey).(string)
		if !ok || strings.TrimSpace(tenantID) == "" {
			writeAuthError(w, "valid tenant claim required", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequireRole returns a middleware that checks if the user has the required role.
// Must be used after JWTAuth.
func RequireRole(role string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRole, ok := r.Context().Value(UserRoleKey).(string)
			if !ok {
				writeAuthError(w, "authentication required", http.StatusUnauthorized)
				return
			}

			// Admin has access to everything
			if userRole == "admin" {
				next.ServeHTTP(w, r)
				return
			}

			if userRole != role {
				writeAuthError(w, "insufficient permissions", http.StatusForbidden)
				return
			}

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

// writeAuthError writes a JSON error response for auth failures.
func writeAuthError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
