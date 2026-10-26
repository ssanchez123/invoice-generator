# Closed tenant onboarding

Public registration is intentionally disabled. A trusted operator provisions each workspace and its first administrator using the one-shot `tenant-ops` command. The command is not an HTTP endpoint and does not accept a password in its arguments or environment.

## Prerequisites

- Apply database migrations, including `008_closed_onboarding_audit` and `009_subscription_billing_day`.
- Create a dedicated, non-superuser login that is a member of `invoicegen_operator`. Do not use the application role or a superuser.
- Set `DB_OPERATOR_URL` in the operator's approved secret manager or protected environment. In production it must use PostgreSQL TLS (`sslmode=verify-full` preferred).
- Set `ENVIRONMENT=production` for production provisioning.
- Build/run the command from `backend/` with Go 1.27.3 or the module's required Go version.

## Provision a workspace

```sh
cd backend
go run ./cmd/tenant-ops provision \
  --tenant-name "Example SAS" \
  --tax-id "900123456-7" \
  --address-line1 "Calle 1 # 2-3" \
  --city "Bogotá" \
  --country CO \
  --owner-email "owner@example.com" \
  --owner-first-name "Ana" \
  --owner-last-name "Pérez"
```

The operator is prompted twice for the first owner's password; input is not echoed. The command uses bcrypt and calls the restricted `provision_tenant_with_owner` database function, which atomically creates the tenant, its `admin` owner, and a tenant-scoped audit entry. It returns the tenant and owner UUIDs; pass the tenant UUID to the owner for sign-in.

The command refuses to run in production without certificate-verified TLS and uses the database's authenticated operator identity for audit. It does not provision additional users or expose an HTTP bootstrap route. Additional account provisioning remains an operator-managed operation; do not enable `/api/v1/auth/register` without implementing an explicit authorized invitation workflow.

## Product boundaries

- Workspace creation and first-admin provisioning are operator-only; there is no public signup.
- Only draft invoices can be edited. Issued, paid, and cancelled invoices are not editable.
- Subscription due dates create internal draft invoices only. The customer-facing team must review and issue each draft; no payment is collected and no DIAN transmission or validation occurs.
- The relay worker processes at most one overdue cycle for each subscription per minute. It keeps the subscription eligible until all missed cycles are recovered, and each generated draft gets a due date 30 days after creation.
- Subscription billing dates are interpreted in Colombia time (COT, UTC-05:00).
- Audit records are append-only and readable only by workspace administrators.

## Rollback

If provisioning fails, the database function rolls the tenant, owner, and audit entry back together. If it succeeds for the wrong business, do not delete records directly; follow the organization's data-retention and customer-offboarding process.
