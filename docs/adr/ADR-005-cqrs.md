# ADR-005: CQRS — Command Query Responsibility Segregation

## Status
Accepted

## Context
Invoice operations have asymmetric workloads: reads are frequent (lists, detail views, reports) while writes are less frequent but complex (create, issue, record payment, cancel). Different optimization strategies apply to each.

Options considered:
- **Single model (CRUD)** — one repository, one set of DTOs
- **CQRS (same DB)** — separate command and query handlers, shared database
- **CQRS (separate stores)** — write to Postgres, read from denormalized projections
- **Event Sourcing** — store events as source of truth, project to read models

## Decision
**CQRS with shared database, separate command/query handlers.**

- **Commands** (write side): use domain aggregates, enforce invariants, produce domain events. E.g., `CreateInvoiceCommand`, `IssueInvoiceCommand`, `RecordPaymentCommand`.
- **Queries** (read side): bypass domain aggregates, read directly from DB with optimized projections. E.g., `ListInvoicesQuery` returns DTOs without loading aggregates.

We are NOT doing event sourcing (too complex for this scope) or separate read stores (unnecessary at this scale). But the separation of command/query handlers showcases the pattern and allows future evolution.

## Consequences
- ✅ Read side can use optimized SQL without loading domain aggregates
- ✅ Write side enforces all business invariants through the aggregate
- ✅ Clear separation: commands change state, queries never do
- ✅ Easy to evolve to separate read stores later if needed
- ⚠️ Two sets of handlers per resource (more code)
- ⚠️ Must keep read models consistent with domain model changes