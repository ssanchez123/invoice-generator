# ADR-006: Domain Events + Outbox Pattern

## Status
Accepted

## Context
When an invoice is issued, several side-effects must happen: PDF generation, audit logging, notification (email), subscription status update. These should be asynchronous and reliable.

Options considered:
- **Direct calls in use case** — synchronous, tight coupling
- **In-process pub/sub** — no persistence, lost on crash
- **Message queue (RabbitMQ/Kafka)** — reliable, but adds infrastructure
- **Outbox pattern** — events in same TX as aggregate, relay worker publishes

## Decision
**Transactional outbox with independent per-consumer deliveries.**

When a command modifies an aggregate, domain events are written to an `outbox` table in the **same database transaction** as the aggregate changes. A trigger creates a separate `outbox_deliveries` row for each subscribed consumer; the relay worker and PDF service claim only their own deliveries.

```
[Command] → [Aggregate] → [Domain Event]
                               ↓
                    [outbox table] ← same TX as aggregate save
                               ↓
                  [outbox_deliveries]
                    ↙             ↘
          [Relay Worker]       [PDF Service]
          (audit/email)        (invoice PDFs)
```

Each subscribed consumer gets its own delivery row. For example, `invoice.issued` is delivered independently to both `relay-worker` and `pdf-service`; acknowledging one cannot hide the event from the other. Consumer subscriptions are managed by migrations in `outbox_subscriptions`.

Workers enumerate tenant IDs, then claim deliveries in transactions scoped with `app.tenant_id`. `FOR UPDATE SKIP LOCKED` atomically assigns a lease; workers process outside the transaction and renew the lease every third of its duration. A worker whose lease expires cannot renew, acknowledge, or retry that delivery. Failed deliveries retry with exponential backoff and are marked `failed` after ten attempts.

Terminal failures can be replayed only through the internal `outbox-ops retry-failed` CLI. It requires a unique, non-superuser database login that is a member of `invoicegen_operator`; the application role has no permission to call the replay function or read its audit table. The database function enforces the tenant context, resets that delivery's retry counter, and atomically records the authenticated database login, reason, and prior attempt count. Do not set `DB_OPERATOR_URL` in API or worker environments.

Provision the non-login `invoicegen_operator` role before applying migrations on an existing production database. Create a separate login for each operator, grant it membership in `invoicegen_operator`, and provide its connection URL only to the one-off CLI process via `DB_OPERATOR_URL`. Example invocation (the URL must come from the deployment secret store, not source control or shell history):

```sh
docker compose -f deploy/docker/docker-compose.yml run --rm --no-deps \
  -e DB_OPERATOR_URL \
  backend outbox-ops retry-failed \
  --tenant-id <tenant-uuid> \
  --event-id <outbox-event-uuid> \
  --consumer pdf-service \
  --reason "renderer issue fixed"
```

Delivery is **at least once**, not exactly once. Consumers must make side effects idempotent because a worker can finish a side effect and crash before acknowledging it. Row-level security remains enabled for delivery rows; workers do not use a superuser or a cross-tenant outbox query.

## Consequences
- ✅ Atomic: events are never lost (same TX as domain state)
- ✅ Decoupled: handlers don't block the command
- ✅ Retryable: each consumer has independent retries and failure state
- ✅ Fan-out: one consumer cannot acknowledge another consumer's work
- ✅ No external message broker needed for this scope
- ⚠️ Delivery is at least once; consumers must make side effects idempotent
- ⚠️ Failed deliveries need monitoring and operator replay/remediation
- ⚠️ There is no operator replay interface yet; terminal failures require a controlled operational procedure
- ⚠️ Outbox and delivery tables grow; they need periodic cleanup/archival
- ⚠️ Polling adds latency (configurable, default 2s)
- ⚠️ PDF delivery is asynchronous; downloads return 404 until the generated artifact is available