# ADR-009: Recurring Billing Engine

## Status
Accepted

## Context
Tenants need to bill customers on a recurring schedule (monthly, quarterly, annually). This requires a mechanism to generate invoices on schedule without manual intervention.

Options considered:
- **Cron job per subscription** — doesn't scale, operational headache
- **External scheduler (Temporal, Quartz)** — powerful, adds infrastructure
- **Scheduler worker with DB-backed schedule** — simple, reliable enough

## Decision
**DB-backed subscription schedule with a worker that polls and generates invoices.**

### Model
A `Subscription` entity stores: customer, plan (items template), frequency (daily/weekly/monthly/yearly), next_billing_date, and status (active/paused/cancelled).

### Worker
A background worker (Go goroutine or separate process) runs every minute:
1. `SELECT * FROM subscriptions WHERE status = 'active' AND next_billing_date <= NOW()`
2. For each: create a new Invoice from the subscription's plan template
3. Issue the invoice (draft → issued)
4. Update `next_billing_date` to next occurrence
5. All in a transaction with row-level locking (`SELECT FOR UPDATE SKIP LOCKED`) to handle multiple worker instances

### Idempotency
The subscription ID + billing period form a natural idempotency key. If the worker crashes mid-processing, re-running won't create duplicate invoices.

## Consequences
- ✅ No external scheduler needed
- ✅ `SKIP LOCKED` allows horizontal scaling of workers
- ✅ Idempotent by design (period-based dedup)
- ⚠️ Worker must handle timezone-aware billing dates
- ⚠️ Need retry logic for transient failures
- ⚠️ Monitoring: must alert if worker falls behind