<div align="center">

# Invoice Generator

**A SaaS-ready invoice management system built with Go, React & PostgreSQL**

Showcasing hexagonal architecture, CQRS, multi-tenancy with Row-Level Security,
and the transactional outbox pattern.

[![Go](https://img.shields.io/badge/Go-1.22-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![React](https://img.shields.io/badge/React-18-61DAFB?logo=react&logoColor=white)](https://react.dev/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql&logoColor=white)](https://www.postgresql.org/)
[![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker&logoColor=white)](https://docs.docker.com/compose/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow?logo=opensourceinitiative&logoColor=white)](LICENSE)

[![Architecture: Hexagonal](https://img.shields.io/badge/Architecture-Hexagonal-blueviolet)](docs/adr/ADR-001-hexagonal-architecture.md)
[![Pattern: CQRS](https://img.shields.io/badge/Pattern-CQRS-orange)](docs/adr/ADR-005-cqrs.md)
[![Pattern: Outbox](https://img.shields.io/badge/Pattern-Transactional%20Outbox-success)](docs/adr/ADR-006-domain-events-outbox.md)
[![Multi-tenant: RLS](https://img.shields.io/badge/Tenant-RLS%20Isolation-purple)](docs/adr/ADR-004-multi-tenancy-rls.md)

**[Features](#features)** · **[Quick Start](#quick-start)** · **[Architecture](#architecture)** · **[Docs](#documentation)** · **[Roadmap](#roadmap)** · **[Contributing](CONTRIBUTING.md)**

🌐 **Languages:** [English](README.md) · [Español](README.es.md)

</div>

---

## Overview

Invoice Generator is a production-grade backend system designed to demonstrate **real-world system design** — not a toy CRUD app. It models a multi-tenant SaaS where each tenant manages customers, invoices, payments, subscriptions, and audit trails with strict isolation enforced at the database level.

The focus is on **backend engineering**: domain modeling, separation of concerns, event-driven architecture, testability, and operational readiness. The frontend is intentionally minimal — a clean React SPA to exercise the API.

## Features

| Feature | Description |
|---|---|
| 🏢 **Multi-tenancy** | Row-Level Security (RLS) at the PostgreSQL level — defense-in-depth tenant isolation |
| 💰 **Multi-currency** | Integer-based Money (no floating-point bugs), ISO 4217 compliant |
| 🧾 **Jurisdiction-aware tax** | Tax rules per country/region, resolved per invoice line |
| 🔁 **Recurring billing** | Subscriptions with daily/weekly/monthly/yearly frequencies |
| ⚡ **CQRS** | Separate command (write) and query (read) handlers with independent evolution paths |
| 📨 **Domain Events + Outbox** | Reliable async processing via the transactional outbox pattern |
| 📄 **PDF generation** | Separate microservice for CPU-bound PDF rendering |
| 🔑 **Idempotency keys** | Safe retries on POST endpoints (Stripe-style) |
| 📋 **Audit trail** | Immutable, insert-only event log per tenant |
| 🔢 **Invoice numbering** | Tenant-scoped sequential, gap-resistant via DB sequences |
| 🗑️ **Soft deletes** | Never hard-delete — preserve audit integrity |
| 🔐 **JWT authentication** | Bearer token auth with tenant-scoped context (scaffold) |

## Quick Start

### Docker Compose (recommended)

```bash
git clone https://github.com/santiagossaa/invoice-generator.git
cd invoice-generator
docker compose -f deploy/docker/docker-compose.yml up -d
```

That's it. Six services come up:

| Service | Port | Description |
|---------|------|-------------|
| PostgreSQL | `5432` | Database with RLS policies |
| Migrate | — | One-shot migration runner (exits when done) |
| API | `8080` | Go HTTP API server |
| Relay Worker | — | Outbox event relay worker |
| PDF Service | — | PDF generation microservice |
| Frontend | `5173` | React SPA (nginx-served) |

**Try it:**
```bash
# Health check
curl http://localhost:8080/healthz

# List invoices (dev tenant)
curl -H "X-Tenant-ID: 00000000-0000-0000-0000-000000000001" \
     -H "Authorization: Bearer dev-token" \
     http://localhost:8080/api/v1/invoices

# Open the UI
open http://localhost:5173
```

### Manual Setup

See [LOCAL_SETUP.md](LOCAL_SETUP.md) for detailed local development instructions.

## Architecture

**Hexagonal Architecture (Ports & Adapters)** — the domain layer has zero dependencies on infrastructure. It defines ports (interfaces) that infrastructure implements as adapters.

```
┌─────────────────────────────────────────────────────────────┐
│                       API Layer (HTTP)                       │
│    Handlers · Middleware · Auth · Idempotency · OpenAPI     │
├─────────────────────────────────────────────────────────────┤
│                  Application Layer (Use Cases)              │
│         Commands · Queries · DTOs · CQRS Split              │
├─────────────────────────────────────────────────────────────┤
│                    Domain Layer (Core)                       │
│       Entities · Value Objects · Domain Events · Ports      │
├─────────────────────────────────────────────────────────────┤
│                 Infrastructure Layer (Adapters)              │
│         PostgreSQL · Outbox · PDF Service · Idempotency      │
└─────────────────────────────────────────────────────────────┘
```

### Key Design Decisions

| # | Decision | Why |
|---|----------|-----|
| 1 | [Hexagonal Architecture](docs/adr/ADR-001-hexagonal-architecture.md) | Domain logic is pure Go, no framework deps |
| 2 | [Go + chi router](docs/adr/ADR-002-go-chi-router.md) | Lightweight, stdlib-compatible, fast |
| 3 | [PostgreSQL](docs/adr/ADR-003-postgresql-datastore.md) | ACID, RLS, JSONB, mature ecosystem |
| 4 | [Multi-tenancy via RLS](docs/adr/ADR-004-multi-tenancy-rls.md) | Defense-in-depth tenant isolation at DB level |
| 5 | [CQRS](docs/adr/ADR-005-cqrs.md) | Separate read/write models for scalability |
| 6 | [Outbox Pattern](docs/adr/ADR-006-domain-events-outbox.md) | Reliable domain event delivery |
| 7 | [PDF as a service](docs/adr/ADR-007-pdf-service.md) | Isolate CPU-bound work from the API |
| 8 | [Multi-currency + tax](docs/adr/ADR-008-multi-currency-tax.md) | First-class domain concepts |
| 9 | [Recurring billing](docs/adr/ADR-009-recurring-billing.md) | DB-backed scheduler with `SKIP LOCKED` |
| 10 | [Idempotency keys](docs/adr/ADR-010-idempotency-keys.md) | Safe retries on critical endpoints |

## Stack

| Component | Technology |
|-----------|-----------|
| Backend | Go 1.22+ · chi v5 · pgx/v5 |
| Frontend | React 18 · Vite · TypeScript |
| Database | PostgreSQL 16 |
| PDF Service | Go (separate microservice) |
| Auth | JWT (scaffold) |
| Migrations | golang-migrate |
| API Docs | OpenAPI 3.0 |
| Deploy | Docker Compose (6 services) |

## Project Structure

```
invoice-generator/
├── backend/
│   ├── cmd/
│   │   ├── api/              # HTTP API server
│   │   ├── relay-worker/     # Outbox relay worker
│   │   └── pdf-service/      # PDF generation service
│   ├── internal/
│   │   ├── domain/           # Core: entities, value objects, events, ports
│   │   ├── application/      # Use cases: CQRS command/query handlers
│   │   ├── infrastructure/   # Adapters: PostgreSQL repos, DB connection
│   │   ├── api/              # HTTP layer: routes, handlers, middleware
│   │   ├── config/           # Configuration loading
│   │   └── ctxkey/           # Shared context key types
│   ├── migrations/           # SQL migrations (up/down)
│   └── go.mod
├── frontend/                 # React + Vite + TypeScript SPA
│   ├── src/
│   │   ├── pages/            # InvoiceList, InvoiceDetail, CreateInvoice, Customers
│   │   ├── api.ts            # Typed API client
│   │   └── index.css         # Global styles
│   ├── package.json
│   └── vite.config.ts
├── docs/
│   ├── SYSTEM_DESIGN.md      # Full system design document
│   ├── diagrams.md           # Mermaid architecture diagrams
│   ├── openapi.yaml          # OpenAPI 3.0 specification
│   └── adr/                  # 10 Architecture Decision Records
├── deploy/
│   └── docker/
│       ├── docker-compose.yml # 6 services
│       ├── Dockerfile.backend  # Multi-stage Go build
│       ├── Dockerfile.frontend # Multi-stage React + nginx
│       ├── nginx.conf          # SPA + API proxy
│       ├── init-db.sql         # PostgreSQL RLS setup
│       └── migrate.sh          # Migration runner script
└── scripts/
    └── dev.sh                # Development startup script
```

## API Reference

### Quick Reference

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/auth/register` | Register new tenant (scaffold) |
| `POST` | `/api/v1/auth/login` | Login (scaffold) |
| `POST` | `/api/v1/invoices` | Create invoice (idempotent) |
| `GET` | `/api/v1/invoices` | List invoices (paginated, filterable) |
| `GET` | `/api/v1/invoices/:id` | Get invoice detail |
| `PATCH` | `/api/v1/invoices/:id` | Update invoice (draft only, scaffold) |
| `POST` | `/api/v1/invoices/:id/issue` | Issue invoice |
| `POST` | `/api/v1/invoices/:id/cancel` | Cancel invoice |
| `POST` | `/api/v1/invoices/:id/payments` | Record payment (idempotent) |
| `GET` | `/api/v1/invoices/:id/payments` | List payments for invoice |
| `GET` | `/api/v1/invoices/:id/pdf` | Download PDF (scaffold) |
| `POST` | `/api/v1/customers` | Create customer |
| `GET` | `/api/v1/customers` | List customers (paginated) |
| `GET` | `/api/v1/customers/:id` | Get customer detail |
| `POST` | `/api/v1/subscriptions` | Create subscription (scaffold) |
| `GET` | `/api/v1/subscriptions` | List subscriptions (scaffold) |
| `DELETE` | `/api/v1/subscriptions/:id` | Cancel subscription (scaffold) |
| `GET` | `/api/v1/audit` | Query audit trail (scaffold) |
| `GET` | `/healthz` | Health check |

Full specification: [docs/openapi.yaml](docs/openapi.yaml)

### Authentication

```bash
# All protected endpoints require:
# - Authorization: Bearer <token>
# - X-Tenant-ID: <tenant-uuid>
curl -H "Authorization: Bearer <token>" \
     -H "X-Tenant-ID: 00000000-0000-0000-0000-000000000001" \
     http://localhost:8080/api/v1/invoices
```

### Idempotency

```bash
# POST endpoints accept Idempotency-Key for safe retries
curl -X POST \
     -H "Idempotency-Key: $(uuidgen)" \
     -H "X-Tenant-ID: 00000000-0000-0000-0000-000000000001" \
     -H "Authorization: Bearer dev-token" \
     -H "Content-Type: application/json" \
     -d '{"customer_id":"...", "currency":"USD", "due_date":"...", "items":[...]}' \
     http://localhost:8080/api/v1/invoices
```

## Testing

| Level | What | Command |
|-------|------|---------|
| **Unit** | Domain layer (value objects, entities, state machine) | `go test ./internal/domain/...` |
| **Integration** | Application layer with in-memory repos | `go test ./internal/application/...` |
| **E2E** | HTTP handlers with `httptest` | `go test ./internal/api/...` |
| **Config** | Configuration loading | `go test ./internal/config/...` |

```bash
# Run all tests
cd backend && go test ./...

# With coverage
cd backend && go test ./... -cover

# Verbose
cd backend && go test -v ./internal/domain/entity/...
```

## Documentation

| Document | Description |
|----------|-------------|
| [System Design](docs/SYSTEM_DESIGN.md) | Full system design document |
| [Diagrams](docs/diagrams.md) | Mermaid architecture & sequence diagrams |
| [OpenAPI Spec](docs/openapi.yaml) | API specification (import to Swagger/Postman) |
| [ADRs](docs/adr/) | 10 Architecture Decision Records |
| [Local Setup](LOCAL_SETUP.md) | Detailed local development instructions |

## Roadmap

- [ ] JWT-based authentication (full implementation)
- [ ] Email notifications via SMTP/SendGrid
- [ ] Webhook delivery for domain events
- [ ] Rate limiting middleware
- [ ] Prometheus metrics + Grafana dashboards
- [ ] Kubernetes deployment manifests
- [ ] End-to-end Playwright tests
- [ ] Internationalization (i18n) for frontend

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT © Santiago SSAA — see [LICENSE](LICENSE) for details.