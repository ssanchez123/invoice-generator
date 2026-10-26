package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/santiagossaa/invoice-generator/internal/ctxkey"
	"github.com/santiagossaa/invoice-generator/internal/domain/port"
)

const (
	maxIdempotencyBodySize = 1 << 20
	idempotencyTTL         = 24 * time.Hour
	maxLoginRateLimitKeys  = 4096
)

// contextKey is kept for backward compatibility with UserIDKey.
type contextKey string

const (
	TenantIDKey ctxkey.CtxKey = ctxkey.TenantIDKey
	UserIDKey   contextKey    = "user_id"
	UserRoleKey contextKey    = "user_role"
	EmailKey    contextKey    = "email"
	clientIPKey contextKey    = "client_ip"
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
			ctx = context.WithValue(ctx, ctxkey.ActorIDKey, claims.UserID)
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

// RequireAnyRole rejects authenticated users whose role is not in the allowlist.
func RequireAnyRole(roles ...string) func(next http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRole, ok := r.Context().Value(UserRoleKey).(string)
			if !ok {
				writeAuthError(w, "authentication required", http.StatusUnauthorized)
				return
			}
			if _, ok := allowed[userRole]; !ok {
				writeAuthError(w, "insufficient permissions", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CaptureClientAddress stores the network peer before any trusted-proxy middleware rewrites RemoteAddr.
func CaptureClientAddress(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), clientIPKey, r.RemoteAddr)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type loginAttempt struct {
	at time.Time
}

type loginRateBucket struct {
	attempts []loginAttempt
	lastSeen time.Time
}

// LoginRateLimit applies an in-process rolling-window limit keyed by the direct network peer.
func LoginRateLimit(limit int, window time.Duration) func(next http.Handler) http.Handler {
	var mu sync.Mutex
	buckets := make(map[string]loginRateBucket)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawAddress, _ := r.Context().Value(clientIPKey).(string)
			if rawAddress == "" {
				rawAddress = r.RemoteAddr
			}
			clientIP, _, err := net.SplitHostPort(rawAddress)
			if err != nil {
				clientIP = rawAddress
			}
			if clientIP == "" {
				clientIP = "unknown"
			}

			now := time.Now()
			mu.Lock()
			for key, bucket := range buckets {
				if now.Sub(bucket.lastSeen) >= window {
					delete(buckets, key)
				}
			}
			bucket := buckets[clientIP]
			active := bucket.attempts[:0]
			for _, attempt := range bucket.attempts {
				if now.Sub(attempt.at) < window {
					active = append(active, attempt)
				}
			}
			if len(active) >= limit {
				retryAfter := int(math.Ceil(window.Seconds() - now.Sub(active[0].at).Seconds()))
				bucket.attempts = active
				bucket.lastSeen = now
				buckets[clientIP] = bucket
				mu.Unlock()
				w.Header().Set("Retry-After", strconv.Itoa(max(retryAfter, 1)))
				writeAuthError(w, "too many login attempts", http.StatusTooManyRequests)
				return
			}

			if _, exists := buckets[clientIP]; !exists && len(buckets) >= maxLoginRateLimitKeys {
				evictOldestLoginBucket(buckets)
			}
			bucket.attempts = append(active, loginAttempt{at: now})
			bucket.lastSeen = now
			buckets[clientIP] = bucket
			mu.Unlock()
			next.ServeHTTP(w, r)
		})
	}
}

func evictOldestLoginBucket(buckets map[string]loginRateBucket) {
	var oldestKey string
	var oldest time.Time
	for key, bucket := range buckets {
		if oldestKey == "" || bucket.lastSeen.Before(oldest) {
			oldestKey, oldest = key, bucket.lastSeen
		}
	}
	delete(buckets, oldestKey)
}

// Idempotency stores and replays keyed POST and DELETE responses within the authenticated tenant.
func Idempotency(repository port.IdempotencyRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost && r.Method != http.MethodDelete {
				next.ServeHTTP(w, r)
				return
			}

			key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}
			if len(key) > 255 {
				writeAuthError(w, "idempotency key must not exceed 255 characters", http.StatusBadRequest)
				return
			}
			tenantID, ok := r.Context().Value(ctxkey.TenantIDKey).(string)
			if !ok || strings.TrimSpace(tenantID) == "" {
				writeAuthError(w, "valid tenant claim required", http.StatusUnauthorized)
				return
			}

			r.Body = http.MaxBytesReader(w, r.Body, maxIdempotencyBodySize)
			body, err := io.ReadAll(r.Body)
			if err != nil {
				var maxBytesErr *http.MaxBytesError
				if errors.As(err, &maxBytesErr) {
					writeAuthError(w, "request body is too large", http.StatusRequestEntityTooLarge)
					return
				}
				writeAuthError(w, "unable to read request body", http.StatusBadRequest)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			requestHash := hashIdempotentRequest(r, body)

			record := &port.IdempotencyRecord{
				Key:         key,
				TenantID:    tenantID,
				RequestHash: requestHash,
				ExpiresAt:   time.Now().Add(idempotencyTTL),
			}
			existing, claimed, err := repository.Reserve(r.Context(), record)
			if err != nil {
				log.Printf("idempotency reservation failed: %v", err)
				writeAuthError(w, "idempotency service unavailable", http.StatusServiceUnavailable)
				return
			}
			if !claimed {
				if existing.RequestHash != requestHash {
					writeAuthError(w, "idempotency key was already used with a different request", http.StatusConflict)
					return
				}
				if existing.State != "completed" {
					writeAuthError(w, "request with this idempotency key is still processing", http.StatusConflict)
					return
				}
				replayIdempotentResponse(w, existing)
				return
			}

			capture := &responseCapture{header: w.Header().Clone()}
			next.ServeHTTP(capture, r)
			if capture.status == 0 {
				capture.status = http.StatusOK
			}
			responseHeaders, err := json.Marshal(persistableHeaders(capture.header))
			if err != nil {
				log.Printf("idempotency response header encoding failed: %v", err)
				writeAuthError(w, "unable to store idempotent response", http.StatusServiceUnavailable)
				return
			}
			record.State = "completed"
			record.Status = capture.status
			record.ResponseBody = capture.body.Bytes()
			record.ResponseHeaders = responseHeaders
			if err := repository.Complete(r.Context(), record); err != nil {
				log.Printf("idempotency response persistence failed: %v", err)
				writeAuthError(w, "unable to store idempotent response", http.StatusServiceUnavailable)
				return
			}
			copyHeaders(w.Header(), capture.header, false)
			w.WriteHeader(capture.status)
			_, _ = w.Write(capture.body.Bytes())
		})
	}
}

func hashIdempotentRequest(r *http.Request, body []byte) string {
	hash := sha256.New()
	_, _ = io.WriteString(hash, r.Method+"\n"+r.URL.RequestURI()+"\n"+r.Header.Get("Content-Type")+"\n")
	_, _ = hash.Write(body)
	return fmt.Sprintf("%x", hash.Sum(nil))
}

type responseCapture struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (w *responseCapture) Header() http.Header {
	return w.header
}

func (w *responseCapture) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *responseCapture) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(body)
}

func persistableHeaders(headers http.Header) http.Header {
	persisted := make(http.Header)
	copyHeaders(persisted, headers, true)
	return persisted
}

func copyHeaders(destination, source http.Header, omitRequestID bool) {
	for key, values := range source {
		if strings.EqualFold(key, "Content-Length") || strings.EqualFold(key, "Date") ||
			strings.EqualFold(key, "Transfer-Encoding") || omitRequestID && strings.EqualFold(key, "X-Request-ID") {
			continue
		}
		destination[key] = append([]string(nil), values...)
	}
}

func replayIdempotentResponse(w http.ResponseWriter, record *port.IdempotencyRecord) {
	var headers http.Header
	if err := json.Unmarshal(record.ResponseHeaders, &headers); err != nil {
		writeAuthError(w, "stored idempotent response is invalid", http.StatusInternalServerError)
		return
	}
	copyHeaders(w.Header(), headers, false)
	w.WriteHeader(record.Status)
	_, _ = w.Write(record.ResponseBody)
}

// writeAuthError writes a JSON error response for auth failures.
func writeAuthError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
