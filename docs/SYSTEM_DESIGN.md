# Invoice Generator — System Design

## Overview

SaaS invoice generator with multi-tenancy, multi-currency, recurring billing, jurisdiction-aware tax rules, and PDF generation as a separate service. Built to showcase system design and software architecture.

## Architecture: Hexagonal (Ports & Adapters)

```
┌─────────────────────────────────────────────────────────┐
│                    API Layer (HTTP)                      │
│  Controllers · Middleware · Auth · Rate Limit · OpenAPI  │
├─────────────────────────────────────────────────────────┤
│              Application Layer (Use Cases)               │
│    Commands · Queries · DTOs · CQRS Split · Handlers     │
├─────────────────────────────────────────────────────────┤
│                 Domain Layer (Core)                      │
│  Entities · Value Objects · Domain Events · Policies     │
├─────────────────────────────────────────────────────────┤
│            Infrastructure Layer (Adapters)              │
│  PostgreSQL · Message Bus · PDF Service · Email · Auth   │
└─────────────────────────────────────────────────────────┘
```

The domain layer has **zero dependencies** on infrastructure. It defines ports (interfaces) that the infrastructure layer implements as adapters.

## Key Architectural Decisions

See `docs/adr/` for detailed Architecture Decision Records.

1. **Hexagonal Architecture** — isolate domain from infrastructure (ADR-001)
2. **Go + chi router** — performance, simplicity, strong typing (ADR-002)
3. **PostgreSQL** — relational integrity, JSONB for flexible fields (ADR-003)
4. **Multi-tenancy via tenant_id + row-level security** — isolation without separate DBs (ADR-004)
5. **CQRS** — separate read/write models for scalability (ADR-005)
6. **Domain Events + outbox pattern** — reliable async processing (ADR-006)
7. **PDF generation as separate service** — isolate CPU-bound work (ADR-007)
8. **Multi-currency + jurisdiction-aware tax** — domain complexity (ADR-008)
9. **Recurring billing engine** — scheduled job expansion (ADR-009)
10. **Idempotency keys** — safe retries on critical endpoints (ADR-010)

## System Components

```
┌──────────┐     ┌──────────────┐     ┌─────────────┐
│  React   │────▶│  Go API      │────▶│ PostgreSQL  │
│ Frontend │     │  (hexagonal) │     │             │
└──────────┘     └──────┬───────┘     └─────────────┘
                        │
                        ▼
                 ┌──────────────┐
                 │  Outbox      │
                 │  Table       │
                 └──────┬───────┘
                        │ (poll/relay)
                        ▼
                 ┌──────────────┐     ┌─────────────┐
                 │  Event Relay │────▶│ PDF Service │
                 │  (Go worker) │     │  (Go + PDF) │
                 └──────────────┘     └─────────────┘
```

## Domain Model

### Entities

- **Tenant** — organization that owns invoices
- **User** — belongs to tenant, has roles
- **Customer** — recipient of invoices, belongs to tenant
- **Invoice** — root aggregate: contains items, has lifecycle
- **InvoiceItem** — line item within invoice
- **Payment** — recorded payment against an invoice
- **Subscription** — recurring billing definition
- **AuditEntry** — immutable event log

### Value Objects

- **Money** — amount + currency, arithmetic with validation
- **TaxRule** — jurisdiction + rate + applicable items
- **Address** — structured postal address
- **InvoiceNumber** — tenant-scoped sequential, format validation
- **IdempotencyKey** — deduplication key for safe retries
- **DateRange** — period for recurring billing

### Domain Events

- `InvoiceIssued` — invoice transitioned to ISSUED state
- `InvoicePaid` — full payment received
- `InvoiceOverdue` — past due date without full payment
- `PaymentRecorded` — partial or full payment
- `PDFGenerated` — PDF artifact ready
- `SubscriptionRenewed` — recurring invoice created

## API Design (v1)

### Endpoints

```
POST   /api/v1/invoices           — Create invoice (idempotent)
GET    /api/v1/invoices           — List (paginated, filtered)
GET    /api/v1/invoices/:id      — Get detail
PATCH  /api/v1/invoices/:id      — Update (draft only)
POST   /api/v1/invoices/:id/issue — Issue (draft → issued)
POST   /api/v1/invoices/:id/cancel — Cancel
POST   /api/v1/invoices/:id/payments — Record payment
GET    /api/v1/invoices/:id/pdf   — Download PDF

POST   /api/v1/customers          — Create customer
GET    /api/v1/customers           — List
GET    /api/v1/customers/:id       — Get detail

POST   /api/v1/subscriptions       — Create recurring billing
GET    /api/v1/subscriptions       — List
DELETE /api/v1/subscriptions/:id  — Cancel subscription

GET    /api/v1/audit               — Audit trail (filtered)

POST   /api/v1/auth/login          — Login (JWT)
POST   /api/v1/auth/register        — Register tenant
```

## Data Model (simplified)

```sql
tenants (id, name, address, tax_id, settings jsonb, created_at)
users (id, tenant_id, email, role, password_hash, created_at)
customers (id, tenant_id, name, email, address jsonb, metadata jsonb)
invoices (id, tenant_id, customer_id, number, status, 
          issue_date, due_date, currency, subtotal, tax_total, 
          total, metadata jsonb, created_at)
invoice_items (id, invoice_id, description, quantity, unit_price, 
               tax_rate, discount, total)
payments (id, invoice_id, amount, currency, method, reference, paid_at)
subscriptions (id, tenant_id, customer_id, plan, frequency, 
               next_billing_date, status)
audit_entries (id, tenant_id, entity_type, entity_id, action, 
               actor_id, payload jsonb, created_at)
outbox (id, aggregate_id, event_type, payload jsonb, status, 
        created_at, processed_at)
idempotency_records (key, tenant_id, response_hash, expires_at)
```

## Complexity Features

1. **Multi-currency** — all amounts stored as (value, currency), conversion at display time
2. **Jurisdiction-aware tax** — tax rules by country/state, applicable per line item
3. **Recurring billing** — subscription → scheduled invoice generation via cron-like worker
4. **Idempotency** — `Idempotency-Key` header on POST endpoints, stored with response hash
5. **Outbox pattern** — domain events written to outbox table in same TX as aggregate, relay worker publishes them
6. **Row-level security** — PostgreSQL RLS policies enforce tenant isolation at DB level
7. **Audit trail** — every state change produces an immutable audit entry
8. **Invoice numbering** — tenant-scoped sequential, gap-resistant, format configurable per tenant
9. **PDF generation** — separate Go service, receives events, generates PDF, stores artifact
10. **Soft deletes** — never hard-delete, mark as deleted for audit integrity