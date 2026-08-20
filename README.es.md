<div align="center">

# Invoice Generator

**Sistema de gestión de facturas SaaS construido con Go, React y PostgreSQL**

Demostrando arquitectura hexagonal, CQRS, multi-tenancy con Row-Level Security
y el patrón transactional outbox.

[![Go](https://img.shields.io/badge/Go-1.22-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![React](https://img.shields.io/badge/React-18-61DAFB?logo=react&logoColor=white)](https://react.dev/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql&logoColor=white)](https://www.postgresql.org/)
[![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker&logoColor=white)](https://docs.docker.com/compose/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow?logo=opensourceinitiative&logoColor=white)](LICENSE)

[![Arquitectura: Hexagonal](https://img.shields.io/badge/Arquitectura-Hexagonal-blueviolet)](docs/adr/ADR-001-hexagonal-architecture.md)
[![Patrón: CQRS](https://img.shields.io/badge/Patr%C3%B3n-CQRS-orange)](docs/adr/ADR-005-cqrs.md)
[![Patrón: Outbox](https://img.shields.io/badge/Patr%C3%B3n-Transactional%20Outbox-success)](docs/adr/ADR-006-domain-events-outbox.md)
[![Multi-tenant: RLS](https://img.shields.io/badge/Tenant-RLS%20Isolation-purple)](docs/adr/ADR-004-multi-tenancy-rls.md)

**[Características](#características)** · **[Inicio Rápido](#inicio-rápido)** · **[Arquitectura](#arquitectura)** · **[Documentación](#documentación)** · **[Roadmap](#roadmap)** · **[Contribuir](CONTRIBUTING.md)**

🌐 **Idiomas:** [English](README.md) · [Español](README.es.md)

</div>

---

## Descripción General

Invoice Generator es un sistema backend de grado producción diseñado para demostrar **system design del mundo real** — no una app CRUD de juguete. Modela un SaaS multi-tenant donde cada tenant gestiona clientes, facturas, pagos, suscripciones y trazas de auditoría con aislamiento estricto aplicado a nivel de base de datos.

El enfoque está en **ingeniería de backend**: modelado de dominio, separación de responsabilidades, arquitectura orientada a eventos, testeabilidad y preparación operacional. El frontend es intencionalmente minimalista — un SPA en React limpio para ejercer la API.

## Características

| Característica | Descripción |
|---|---|
| 🏢 **Multi-tenancy** | Row-Level Security (RLS) a nivel PostgreSQL — aislamiento de tenants por defensa en profundidad |
| 💰 **Multi-moneda** | Money basado en enteros (sin bugs de punto flotante), compatible con ISO 4217 |
| 🧾 **Impuestos por jurisdicción** | Reglas de impuesto por país/región, resueltas por línea de factura |
| 🔁 **Facturación recurrente** | Suscripciones con frecuencias diaria/semanal/mensual/anual |
| ⚡ **CQRS** | Handlers separados de comando (escritura) y consulta (lectura) |
| 📨 **Eventos de dominio + Outbox** | Procesamiento asíncrono confiable vía patrón transactional outbox |
| 📄 **Generación de PDF** | Microservicio separado para renderizado de PDF (CPU-bound) |
| 🔑 **Llaves de idempotencia** | Retries seguros en endpoints POST (estilo Stripe) |
| 📋 **Auditoría** | Log de eventos inmutable, solo inserción, por tenant |
| 🔢 **Numeración de facturas** | Secuencial por tenant, resistente a huecos vía secuencias de BD |
| 🗑️ **Soft deletes** | Nunca borrar físicamente — preservar integridad de auditoría |
| 🔐 **Autenticación JWT** | Bearer token con contexto por tenant (scaffold) |

## Inicio Rápido

### Docker Compose (recomendado)

```bash
git clone https://github.com/santiagossaa/invoice-generator.git
cd invoice-generator
docker compose -f deploy/docker/docker-compose.yml up -d
```

Eso es todo. Seis servicios arrancan:

| Servicio | Puerto | Descripción |
|---------|--------|-------------|
| PostgreSQL | `5432` | Base de datos con políticas RLS |
| Migrate | — | Runner de migraciones one-shot (sale al terminar) |
| API | `8080` | Servidor HTTP en Go |
| Relay Worker | — | Worker de relay de eventos del outbox |
| PDF Service | — | Microservicio de generación de PDF |
| Frontend | `5173` | SPA en React (servido por nginx) |

**Pruébalo:**
```bash
# Health check
curl http://localhost:8080/healthz

# Listar facturas (tenant de desarrollo)
curl -H "X-Tenant-ID: 00000000-0000-0000-0000-000000000001" \
     -H "Authorization: Bearer dev-token" \
     http://localhost:8080/api/v1/invoices

# Abrir la UI
open http://localhost:5173
```

### Configuración manual

Ver [LOCAL_SETUP.md](LOCAL_SETUP.md) para instrucciones detalladas de desarrollo local.

## Arquitectura

**Arquitectura Hexagonal (Ports & Adapters)** — la capa de dominio tiene cero dependencias de infraestructura. Define puertos (interfaces) que la infraestructura implementa como adaptadores.

```
┌─────────────────────────────────────────────────────────────┐
│                      Capa API (HTTP)                         │
│      Handlers · Middleware · Auth · Idempotencia · OpenAPI  │
├─────────────────────────────────────────────────────────────┤
│                  Capa de Aplicación (Casos de Uso)            │
│            Comandos · Consultas · DTOs · Split CQRS          │
├─────────────────────────────────────────────────────────────┤
│                    Capa de Dominio (Núcleo)                  │
│      Entidades · Value Objects · Eventos de Dominio · Ports │
├─────────────────────────────────────────────────────────────┤
│                 Capa de Infraestructura (Adaptadores)        │
│        PostgreSQL · Outbox · PDF Service · Idempotencia      │
└─────────────────────────────────────────────────────────────┘
```

### Decisiones de Diseño Clave

| # | Decisión | Por qué |
|---|----------|---------|
| 1 | [Arquitectura Hexagonal](docs/adr/ADR-001-hexagonal-architecture.md) | Lógica de dominio en Go puro, sin deps de framework |
| 2 | [Go + chi router](docs/adr/ADR-002-go-chi-router.md) | Ligero, compatible con stdlib, rápido |
| 3 | [PostgreSQL](docs/adr/ADR-003-postgresql-datastore.md) | ACID, RLS, JSONB, ecosistema maduro |
| 4 | [Multi-tenancy vía RLS](docs/adr/ADR-004-multi-tenancy-rls.md) | Aislamiento de tenants a nivel BD por defensa en profundidad |
| 5 | [CQRS](docs/adr/ADR-005-cqrs.md) | Modelos de lectura/escritura separados para escalabilidad |
| 6 | [Outbox Pattern](docs/adr/ADR-006-domain-events-outbox.md) | Entrega confiable de eventos de dominio |
| 7 | [PDF como servicio](docs/adr/ADR-007-pdf-service.md) | Aislar trabajo CPU-bound del API |
| 8 | [Multi-moneda + impuestos](docs/adr/ADR-008-multi-currency-tax.md) | Conceptos de dominio de primera clase |
| 9 | [Facturación recurrente](docs/adr/ADR-009-recurring-billing.md) | Scheduler basado en BD con `SKIP LOCKED` |
| 10 | [Llaves de idempotencia](docs/adr/ADR-010-idempotency-keys.md) | Retries seguros en endpoints críticos |

## Stack Tecnológico

| Componente | Tecnología |
|-----------|-----------|
| Backend | Go 1.22+ · chi v5 · pgx/v5 |
| Frontend | React 18 · Vite · TypeScript |
| Base de datos | PostgreSQL 16 |
| Servicio PDF | Go (microservicio separado) |
| Autenticación | JWT (scaffold) |
| Migraciones | golang-migrate |
| Docs API | OpenAPI 3.0 |
| Despliegue | Docker Compose (6 servicios) |

## Estructura del Proyecto

```
invoice-generator/
├── backend/
│   ├── cmd/
│   │   ├── api/              # Servidor HTTP API
│   │   ├── relay-worker/     # Worker de relay del outbox
│   │   └── pdf-service/      # Servicio de generación de PDF
│   ├── internal/
│   │   ├── domain/           # Núcleo: entidades, value objects, eventos, ports
│   │   ├── application/      # Casos de uso: handlers CQRS comando/consulta
│   │   ├── infrastructure/   # Adaptadores: repos PostgreSQL, conexión DB
│   │   ├── api/              # Capa HTTP: rutas, handlers, middleware
│   │   ├── config/           # Carga de configuración
│   │   └── ctxkey/           # Tipos de context key compartidos
│   ├── migrations/           # Migraciones SQL (up/down)
│   └── go.mod
├── frontend/                 # SPA React + Vite + TypeScript
│   ├── src/
│   │   ├── pages/            # InvoiceList, InvoiceDetail, CreateInvoice, Customers
│   │   ├── api.ts            # Cliente API tipado
│   │   └── index.css         # Estilos globales
│   ├── package.json
│   └── vite.config.ts
├── docs/
│   ├── SYSTEM_DESIGN.md      # Documento de diseño del sistema
│   ├── diagrams.md           # Diagramas de arquitectura (Mermaid)
│   ├── openapi.yaml          # Especificación OpenAPI 3.0
│   └── adr/                  # 10 Architecture Decision Records
├── deploy/
│   └── docker/
│       ├── docker-compose.yml # 6 servicios
│       ├── Dockerfile.backend  # Build Go multi-stage
│       ├── Dockerfile.frontend # Build React + nginx multi-stage
│       ├── nginx.conf          # SPA + proxy API
│       ├── init-db.sql         # Setup RLS PostgreSQL
│       └── migrate.sh          # Script de migraciones
└── scripts/
    └── dev.sh                # Script de arranque de desarrollo
```

## Referencia API

### Referencia rápida

| Método | Endpoint | Descripción |
|--------|----------|-------------|
| `POST` | `/api/v1/auth/register` | Registrar nuevo tenant (scaffold) |
| `POST` | `/api/v1/auth/login` | Login (scaffold) |
| `POST` | `/api/v1/invoices` | Crear factura (idempotente) |
| `GET` | `/api/v1/invoices` | Listar facturas (paginado, filtrable) |
| `GET` | `/api/v1/invoices/:id` | Detalle de factura |
| `PATCH` | `/api/v1/invoices/:id` | Actualizar factura (solo borrador, scaffold) |
| `POST` | `/api/v1/invoices/:id/issue` | Emitir factura |
| `POST` | `/api/v1/invoices/:id/cancel` | Cancelar factura |
| `POST` | `/api/v1/invoices/:id/payments` | Registrar pago (idempotente) |
| `GET` | `/api/v1/invoices/:id/payments` | Listar pagos de factura |
| `GET` | `/api/v1/invoices/:id/pdf` | Descargar PDF (scaffold) |
| `POST` | `/api/v1/customers` | Crear cliente |
| `GET` | `/api/v1/customers` | Listar clientes (paginado) |
| `GET` | `/api/v1/customers/:id` | Detalle de cliente |
| `POST` | `/api/v1/subscriptions` | Crear suscripción (scaffold) |
| `GET` | `/api/v1/subscriptions` | Listar suscripciones (scaffold) |
| `DELETE` | `/api/v1/subscriptions/:id` | Cancelar suscripción (scaffold) |
| `GET` | `/api/v1/audit` | Consultar auditoría (scaffold) |
| `GET` | `/healthz` | Health check |

Especificación completa: [docs/openapi.yaml](docs/openapi.yaml)

### Autenticación

```bash
# Todos los endpoints protegidos requieren:
# - Authorization: Bearer <token>
# - X-Tenant-ID: <tenant-uuid>
curl -H "Authorization: Bearer <token>" \
     -H "X-Tenant-ID: 00000000-0000-0000-0000-000000000001" \
     http://localhost:8080/api/v1/invoices
```

### Idempotencia

```bash
# Los endpoints POST aceptan Idempotency-Key para retries seguros
curl -X POST \
     -H "Idempotency-Key: $(uuidgen)" \
     -H "X-Tenant-ID: 00000000-0000-0000-0000-000000000001" \
     -H "Authorization: Bearer dev-token" \
     -H "Content-Type: application/json" \
     -d '{"customer_id":"...", "currency":"USD", "due_date":"...", "items":[...]}' \
     http://localhost:8080/api/v1/invoices
```

## Testing

| Nivel | Qué | Comando |
|-------|-----|---------|
| **Unit** | Capa de dominio (value objects, entidades, state machine) | `go test ./internal/domain/...` |
| **Integración** | Capa de aplicación con repos en memoria | `go test ./internal/application/...` |
| **E2E** | Handlers HTTP con `httptest` | `go test ./internal/api/...` |
| **Config** | Carga de configuración | `go test ./internal/config/...` |

```bash
# Ejecutar todos los tests
cd backend && go test ./...

# Con cobertura
cd backend && go test ./... -cover

# Verbose
cd backend && go test -v ./internal/domain/entity/...
```

## Documentación

| Documento | Descripción |
|----------|-------------|
| [Diseño del Sistema](docs/SYSTEM_DESIGN.md) | Documento completo de diseño |
| [Diagramas](docs/diagrams.md) | Diagramas de arquitectura y secuencia (Mermaid) |
| [Spec OpenAPI](docs/openapi.yaml) | Especificación API (importar a Swagger/Postman) |
| [ADRs](docs/adr/) | 10 Architecture Decision Records |
| [Setup Local](LOCAL_SETUP.md) | Instrucciones detalladas de desarrollo local |

## Roadmap

- [ ] Autenticación basada en JWT (implementación completa)
- [ ] Notificaciones por email vía SMTP/SendGrid
- [ ] Entrega de webhooks para eventos de dominio
- [ ] Middleware de rate limiting
- [ ] Métricas Prometheus + dashboards Grafana
- [ ] Manifiestos de despliegue Kubernetes
- [ ] Tests end-to-end con Playwright
- [ ] Internacionalización (i18n) para el frontend

## Contribuir

¡Las contribuciones son bienvenidas! Por favor revisa [CONTRIBUTING.md](CONTRIBUTING.md) para las guías.

## Licencia

MIT © Santiago SSAA — ver [LICENSE](LICENSE) para detalles.