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
**Idempotency-Key header on enabled state-changing POST endpoints.** The header is optional, but clients must reuse it to safely retry a write.

### Mechanism
1. The API hashes the method, request URI, content type, and body.
2. It atomically reserves `(tenant_id, key)` in `idempotency_records` before running the handler.
3. A completed matching request replays the original status, headers, and body for 24 hours.
4. Reusing a key with a different request, or while its first request is processing, returns `409`.
5. The completed response is persisted after the handler returns. A crash between the business transaction and response persistence leaves a processing reservation, which blocks duplicate execution until expiry.

### Scope
Applied to:
- `POST /invoices` — prevent duplicate invoices
- `POST /invoices/:id/issue` and `/cancel` — prevent duplicate transitions
- `POST /invoices/:id/payments` — prevent double payments
- `POST /customers` — prevent duplicate customer creation

Not applied to GET (inherently idempotent) or PUT/PATCH (idempotent by nature).

## Consequences
- ✅ Completed requests can be replayed without repeating the operation
- ✅ Concurrent matching requests cannot both execute while the reservation is active
- ✅ Standard pattern, familiar to API consumers
- ⚠️ The business write and idempotency response are not committed in one transaction; processing reservations require operational care after crashes
- ⚠️ Expired keys can be reused after 24 hours
- ⚠️ Must document for API consumers