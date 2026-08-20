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
**Outbox pattern with an in-process relay worker.**

When a command modifies an aggregate, domain events are written to an `outbox` table in the **same database transaction** as the aggregate changes. A background relay worker polls the outbox table and dispatches events to handlers (PDF service, audit logger, notification service).

```
[Command] → [Aggregate] → [Domain Event]
                               ↓
                    [outbox table] ← same TX as aggregate save
                               ↓
                    [Relay Worker] (polls every N seconds)
                               ↓
                    [Event Handlers] (PDF, Audit, Email)
```

## Consequences
- ✅ Atomic: events are never lost (same TX as domain state)
- ✅ Decoupled: handlers don't block the command
- ✅ Retryable: failed handlers can retry from outbox
- ✅ No external message broker needed for this scope
- ⚠️ Outbox table grows; needs periodic cleanup/archival
- ⚠️ Polling adds latency (configurable, default 2s)
- ⚠- Relay worker is a single point of failure (mitigated: can run multiple instances with locking)