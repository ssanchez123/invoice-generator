# Agent Safety Boundaries

## Data and environment
- Treat production databases, deployments, customer records, and credentials as sensitive.
- Do not deploy, run migrations against production, or change production configuration without explicit user authorization.
- Do not run `DROP`, `TRUNCATE`, database resets, or destructive cleanup unless the target is confirmed to be the test database and the task requires it.
- Do not print or commit secrets. Use existing environment-variable configuration and sanitized test values.

## Authentication and tenant isolation
- Trust tenant identity only after JWT validation. A client-supplied tenant ID is not an authorization check.
- Public registration is disabled at the HTTP layer. The underlying registration use case still accepts roles; do not expose it without a trusted tenant bootstrap/invitation flow and authorization tests.
- Keep the application database role non-superuser. Preserve RLS policies and tenant-scoped access for every tenant-owned query and write.
- Protected routes must validate JWT before deriving tenant context. Do not reintroduce request-header-based tenant authorization.

## Billing and event processing
- Preserve integer minor-unit money, currency validation, basis-point tax rates, invoice state invariants, and transaction boundaries for payments.
- Do not claim idempotency is enforced until the middleware/repository path actually persists and replays responses.
- Outbox deliveries are tracked per consumer in `outbox_deliveries`; keep tenant-scoped RLS, leases, retries, and at-least-once/idempotency semantics when changing workers. `invoice.issued` is independently delivered to the relay and PDF service. The PDF service writes a real PDF before emitting `pdf.generated`; artifact keys must be derived from tenant and invoice UUIDs, never client-provided paths.
- PDF downloads must verify invoice ownership through the authenticated tenant before reading the artifact. S3 protocol tests use an in-process HTTP server and must not require AWS credentials. Production must configure S3-compatible storage and must not use the local filesystem backend.
- Requeue terminal outbox deliveries only with the internal `outbox-ops` CLI and a unique login granted `invoicegen_operator`; never expose replay through the public API or provide `DB_OPERATOR_URL` to API/worker services.
- PDF, subscription, invoice-update, and audit routes contain scaffold behavior. Verify the concrete implementation before depending on their API responses.

## Verification expectations
- Test security-sensitive changes for cross-tenant access, missing/invalid JWTs, role escalation, and failure/retry behavior as applicable.
- Run integration tests only against the dedicated test database configured by `deploy/docker/docker-compose.test.yml`.
- If deployment target, data sensitivity, or destructive impact is unclear, stop and ask before acting.
