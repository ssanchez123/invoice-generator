# ADR-010: Idempotency Keys for Safe Retries

## Status
Accepted

## Context
Network failures and client retries can cause duplicate invoice creation or double payments. We need a mechanism to make critical POST operations safe to retry.

Options considered:
- **Client-side dedup (check before send)** — unreliable, race conditions
- **Unique constraints on business keys** — partial, doesn't cover all cases
- **Idempotency-Key header** — standard pattern (Stripe-style), explicit and robust

## Decision
**Idempotency-Key header on all POST endpoints that create or modify state.**

### Mechanism
1. Client sends `Idempotency-Key: <uuid>` header on POST
2. Server checks `idempotency_records` table for existing key within TTL window (24h)
3. If found: return cached response (same status + body)
4. If not found: process request, store `{key, tenant_id, response_hash, status, expires_at}` in same TX
5. Subsequent retries with same key return cached response

### Scope
Applied to:
- `POST /invoices` — prevent duplicate invoices
- `POST /invoices/:id/payments` — prevent double payments
- `POST /subscriptions` — prevent duplicate subscriptions

Not applied to GET (inherently idempotent) or PUT/PATCH (idempotent by nature).

## Consequences
- ✅ Clients can retry safely on network errors
- ✅ Prevents double-charging on payment endpoints
- ✅ Standard pattern, familiar to API consumers
- ⚠️ Extra DB write per request (in same TX, minimal cost)
- ⚠️ TTL cleanup needed for expired records
- ⚠️ Must document for API consumers