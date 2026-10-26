# Production operations and hardening

This is a runbook and configuration contract, not a production deployment. The Docker Compose stack is for local development only. Do not use its credentials or `DB_SSLMODE=disable` in production. No DIAN provider is integrated, and an invoice marked `issued` is not a DIAN-validated electronic invoice.

## Required production configuration

Set `ENVIRONMENT=production`. The API rejects the development JWT secret, JWT secrets shorter than 32 bytes, the development database password, missing database coordinates, database TLS disabled, missing CORS origins, and non-HTTPS or loopback CORS origins.

Use:

- `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, and `DB_PASSWORD` for the private database endpoint.
- `DB_SSLMODE=verify-full` where the database certificate chain is trusted by the runtime. Use `DB_SSLROOTCERT` when a private CA is required. `require` and `verify-ca` enable TLS but provide weaker server identity verification.
- `JWT_SECRET` from a secret manager and `JWT_EXPIRY` appropriate to the deployment. The production default expiry is one hour.
- `CORS_ALLOWED_ORIGINS` as a comma-separated exact HTTPS origin allowlist. Do not use `*`.
- `PDF_STORAGE_BACKEND=s3` with a private bucket and HTTPS endpoint. Prefer workload identity over static access keys where supported.

The frontend and backend container images run as non-root. The API's `/health/live` endpoint only indicates that the process responds; `/health/ready` also pings PostgreSQL and returns `503` if that dependency is unavailable. Configure deployment probes against these endpoints and terminate external TLS at a trusted ingress or load balancer.

## Authorization and login protection

The API role policy is:

| Role | Read invoices/customers/payments/PDF | Create or change invoices/customers/payments | Audit |
|---|---:|---:|---:|
| `admin` | Yes | Yes | Endpoint currently disabled |
| `member` | Yes | Yes | No |
| `viewer` | Yes | No | No |

Invoice updates, subscriptions, and audit queries return `501 Not Implemented`; they do not claim to persist data. Public registration remains disabled. Login requests are rate-limited per direct network peer (10 requests per minute by default), with `LOGIN_RATE_LIMIT` and `LOGIN_RATE_WINDOW` configuration. This in-process limit is defense in depth; a multi-instance deployment should also enforce rate limits at its trusted ingress.

## Idempotency behavior

For enabled state-changing `POST` operations, clients should send a unique `Idempotency-Key` and reuse it unchanged for retries. Completed responses are stored per tenant for 24 hours and replayed with the original status/body. Reusing a key with a different method, path, query, content type, or body returns `409`. A duplicate arriving while the first request is still processing also receives `409`; it must be retried after the first response completes. Keys are optional, so clients that omit them do not receive deduplication.

If the process stops after reserving a key but before completing the response record, the reservation remains in `processing` and prevents duplicate execution during its retention window. Investigate such records before manually removing them; do not delete active records as routine cleanup.

## PostgreSQL backup and restore verification

Backups contain customer and financial data. Store them encrypted, restrict access, define retention with the responsible legal/accounting team, and keep a copy outside the database failure domain. Supply connection credentials using a protected `PGPASSFILE` (mode `0600`) or an approved identity mechanism; do not put passwords in command arguments, shell history, or this repository.

Create a custom-format backup:

```sh
pg_dump --format=custom --no-owner \
  --host="$DB_HOST" --port="${DB_PORT:-5432}" --username="$DB_USER" \
  --dbname="$DB_NAME" --file="$BACKUP_FILE"
pg_restore --list "$BACKUP_FILE" >/dev/null
```

Verify recovery in a new, isolated database, never over the live database:

```sh
createdb --host="$DB_HOST" --port="${DB_PORT:-5432}" \
  --username="$DB_USER" "$RESTORE_DB"
pg_restore --exit-on-error --no-owner \
  --host="$DB_HOST" --port="${DB_PORT:-5432}" --username="$DB_USER" \
  --dbname="$RESTORE_DB" "$BACKUP_FILE"
psql --host="$DB_HOST" --port="${DB_PORT:-5432}" --username="$DB_USER" \
  --dbname="$RESTORE_DB" --set ON_ERROR_STOP=1 \
  --command="SELECT COUNT(*) FROM tenants; SELECT COUNT(*) FROM invoices;"
```

Record the backup timestamp, restore duration, command exit codes, and validation results without recording credentials or customer data. A backup is not verified until a restore succeeds and the application role's tenant isolation is checked against the restored database.

## Release gates

CI runs backend tests and integration tests, Go vet/format checks, frontend typecheck/build, Playwright tests, and dependency audit. E2E tests use mocked API responses; they do not replace a separate, authorized end-to-end test against a production-like API/database. No production deployment, cloud provisioning, external monitoring, or DIAN-provider operation is performed by this repository workflow.
