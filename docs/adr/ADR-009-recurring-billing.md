# ADR-009: Recurring Billing Engine

## Status
Accepted

## Context
Tenants need to prepare invoices for recurring customer charges. Recurring generation must not bypass a human review or issuance step.

Options considered:
- **Cron job per subscription** — doesn't scale, operational headache
- **External scheduler (Temporal, Quartz)** — powerful, adds infrastructure
- **Scheduler worker with DB-backed schedule** — simple, reliable enough

## Decision
**DB-backed subscription schedule with a worker that polls and creates draft invoices.**

### Model
A `Subscription` entity stores: customer, plan (items template), frequency (daily/weekly/monthly/yearly), next_billing_date, billing_day, and status (active/paused/cancelled).

### Worker
A background worker (Go goroutine or separate process) runs every minute:
1. Per tenant, lock up to 50 active subscriptions whose `next_billing_date <= NOW()` using `SELECT FOR UPDATE SKIP LOCKED`.
2. For each subscription, create one draft invoice from its plan template, with a due date 30 days after draft creation.
3. Advance `next_billing_date` by one cycle in Colombia time (COT, UTC-05:00), preserving the configured day and clamping to month-end when necessary.
4. Leave the invoice in draft; a workspace user must review and issue it manually. No DIAN transmission or validation occurs.
5. Repeat polling once per minute. If the worker was down, recover one overdue cycle per subscription per pass until caught up.

### Idempotency
Invoice creation and schedule advancement commit atomically under the subscription row lock. A crash before commit rolls back both; after commit the schedule has advanced, so the same cycle is not generated twice.

## Consequences
- ✅ No external scheduler needed
- ✅ `SKIP LOCKED` allows horizontal scaling of workers
- ✅ Each recovered cycle remains a separate reviewable draft
- ⚠️ Billing dates currently use Colombia time (COT, UTC-05:00); supporting tenant-selected time zones requires an explicit setting and migration.
- ⚠️ Need retry logic for transient failures
- ⚠️ Monitoring: must alert if worker falls behind
- ⚠️ Catch-up is intentionally gradual: no more than one missed cycle per subscription per worker pass