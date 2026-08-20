// Package ctxkey defines shared context keys used across layers.
//
// The main purpose is to allow the API middleware (which sets tenant_id
// from the request) and the infrastructure repositories (which read
// tenant_id for RLS) to use the SAME context key without creating
// a circular dependency between the api and infrastructure packages.
package ctxkey

// TenantIDKey is the context key for the tenant ID.
// Both the API middleware and the repository layer use this key
// to propagate the tenant ID through the request context.
type CtxKey string

const TenantIDKey CtxKey = "tenant_id"