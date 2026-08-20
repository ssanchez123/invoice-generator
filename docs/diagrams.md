# Architecture Diagrams

## High-Level Architecture

```mermaid
graph TB
    subgraph "Client"
        FE["React Frontend<br/>(Vite + TS)"]
    end

    subgraph "API Service (Go + chi)"
        API["HTTP API<br/>Routes · Middleware<br/>Auth · TenantContext · Idempotency"]
        APP["Application Layer<br/>Commands · Queries<br/>(CQRS)"]
        DOM["Domain Layer<br/>Entities · Value Objects<br/>Events · Ports"]
        INFRA["Infrastructure<br/>Repository Adapters<br/>(pgx)"]
        API --> APP
        APP --> DOM
        APP --> INFRA
    end

    subgraph "Workers"
        RELAY["Relay Worker<br/>(Outbox Poller)"]
        SUB["Billing Worker<br/>(Subscription Scheduler)"]
    end

    subgraph "PDF Service (Go)"
        PDF["PDF Generator<br/>(Template Render)"]
    end

    subgraph "Data"
        PG[("PostgreSQL 16<br/>+ RLS")]
        FS["File Storage<br/>(PDFs)"]
    end

    FE -->|HTTP /api/v1| API
    INFRA -->|pgx| PG
    RELAY -->|poll outbox| PG
    RELAY -->|trigger| PDF
    PDF -->|store artifact| FS
    PDF -->|emit event| PG
    SUB -->|poll subscriptions| PG
    SUB -->|create invoice| INFRA
```

## Hexagonal Architecture (Ports & Adapters)

```mermaid
graph TB
    subgraph "Domain Layer (Zero Dependencies)"
        ENT["Entities<br/>Invoice · Customer · Payment<br/>Subscription · Tenant"]
        VO["Value Objects<br/>Money · TaxRule · Address<br/>InvoiceNumber · DateRange"]
        EVT["Domain Events<br/>InvoiceIssued · InvoicePaid<br/>PaymentRecorded · PDFGenerated"]
        PORT["Ports (Interfaces)<br/>InvoiceRepository<br/>CustomerRepository<br/>TaxResolverPort<br/>OutboxRepository"]
    end

    subgraph "Application Layer"
        CMD["Commands<br/>CreateInvoice<br/>IssueInvoice<br/>CancelInvoice<br/>RecordPayment"]
        QRY["Queries<br/>ListInvoices<br/>GetInvoice<br/>ListCustomers<br/>ListPayments"]
    end

    subgraph "Infrastructure Layer (Adapters)"
        REPO["Repository Adapters<br/>PostgreSQL implementations<br/>(pgx + RLS)"]
        BUS["Outbox Relay<br/>Event dispatching"]
        PDFI["PDF Adapter<br/>HTML → PDF rendering"]
    end

    subgraph "API Layer"
        HTTP["HTTP Controllers<br/>chi routes · handlers"]
        MW["Middleware<br/>Auth · Tenant · Idempotency<br/>CORS · Logging · Recovery"]
    end

    HTTP --> CMD
    HTTP --> QRY
    CMD --> ENT
    CMD --> PORT
    QRY --> PORT
    REPO -.->|implements| PORT
    ENT --> VO
    ENT --> EVT
    CMD --> EVT
```

## Invoice State Machine

```mermaid
stateDiagram-v2
    [*] --> draft: CreateInvoice
    draft --> issued: Issue()
    draft --> cancelled: Cancel()
    issued --> paid: MarkPaid()\n(full payment)
    issued --> overdue: MarkOverdue()\n(past due date)
    issued --> cancelled: Cancel()
    overdue --> paid: MarkPaid()
    overdue --> cancelled: Cancel()
    cancelled --> [*]
    paid --> [*]

    note right of draft
        Can add/remove items
        Can modify fields
    end note

    note right of issued
        Immutable
        PDF generated (async)
        Payments can be recorded
    end note
```

## Outbox Pattern Flow

```mermaid
sequenceDiagram
    participant Client
    participant API
    participant Domain
    participant DB as PostgreSQL
    participant Relay as Relay Worker
    participant PDF as PDF Service

    Client->>API: POST /invoices/:id/issue
    API->>Domain: IssueInvoice command
    Domain->>Domain: Validate + change state
    Domain-->>API: Domain events

    rect rgb(240, 240, 255)
        Note over API, DB: Same transaction
        API->>DB: UPDATE invoices SET status='issued'
        API->>DB: INSERT INTO outbox (event_type='invoice.issued')
        API->>DB: INSERT INTO audit_entries
        API->>DB: COMMIT
    end

    API-->>Client: 200 OK (invoice issued)

    rect rgb(255, 255, 240)
        Note over Relay, DB: Async processing (every 2s)
        Relay->>DB: SELECT FROM outbox WHERE status='pending'<br/>FOR UPDATE SKIP LOCKED
        DB-->>Relay: invoice.issued event
        Relay->>PDF: Generate PDF for invoice
        PDF->>PDF: Render HTML template
        PDF->>DB: INSERT INTO outbox (event_type='pdf.generated')
        Relay->>DB: UPDATE outbox SET status='processed'
    end
```

## Multi-Tenancy with RLS

```mermaid
graph LR
    subgraph "Application"
        REQ["HTTP Request<br/>JWT → tenant_id"]
    end

    subgraph "Connection Pool"
        POOL["pgxpool<br/>25 connections"]
    end

    subgraph "PostgreSQL"
        SET["SET LOCAL app.tenant_id = 'xxx'"]
        RLS["Row-Level Security Policies<br/>WHERE tenant_id = current_setting('app.tenant_id')"]
        T1["tenants"]
        T2["invoices<br/>(tenant_id)"]
        T3["customers<br/>(tenant_id)"]
        T4["payments<br/>(tenant_id)"]
        T5["audit_entries<br/>(tenant_id)"]
    end

    REQ -->|"TenantTx(tenant_id)"| POOL
    POOL --> SET
    SET --> RLS
    RLS --> T2
    RLS --> T3
    RLS --> T4
    RLS --> T5
```

## CQRS Flow

```mermaid
graph LR
    subgraph "Write Path (Commands)"
        C1["CreateInvoice"] --> AGG["Invoice Aggregate"]
        C2["IssueInvoice"] --> AGG
        C3["CancelInvoice"] --> AGG
        C4["RecordPayment"] --> AGG
        AGG -->|"enforce invariants"| REPO_W["Repository.Write<br/>(full aggregate)"]
        AGG --> EVT1["Domain Events → Outbox"]
    end

    subgraph "Read Path (Queries)"
        Q1["ListInvoices"] --> REPO_R["Repository.Read<br/>(optimized SQL)"]
        Q2["GetInvoice"] --> REPO_R
        Q3["ListCustomers"] --> REPO_R
        Q4["ListPayments"] --> REPO_R
        REPO_R --> DTO["DTOs<br/>(read models)"]
    end

    subgraph "Shared Database"
        DB[("PostgreSQL")]
    end

    REPO_W --> DB
    REPO_R --> DB
```

## Subscription Billing Worker

```mermaid
sequenceDiagram
    participant Worker as Billing Worker
    participant DB as PostgreSQL
    participant Domain
    participant Outbox

    loop Every minute
        Worker->>DB: SELECT FROM subscriptions<br/>WHERE status='active'<br/>AND next_billing_date <= NOW()<br/>FOR UPDATE SKIP LOCKED
        DB-->>Worker: Due subscriptions

        for each subscription
            Worker->>Domain: CreateInvoice(subscription.plan_items)
            Domain->>Domain: Build invoice from template
            Domain->>Domain: Issue invoice
            Worker->>DB: INSERT invoices + items
            Worker->>Outbox: INSERT outbox (invoice.issued)
            Worker->>DB: UPDATE subscription.next_billing_date
            Worker->>Outbox: INSERT outbox (subscription.renewed)
        end
    end
```

## System Context

```mermaid
graph TB
    subgraph "Invoice Generator System"
        API["API Service<br/>(Go)"]
        RELAY["Relay Worker<br/>(Go)"]
        PDF["PDF Service<br/>(Go)"]
        FE["Frontend<br/>(React)"]
        DB[("PostgreSQL")]
    end

    subgraph "External"
        USER["Tenant User"]
        EMAIL["Email Provider<br/>(future)"]
        STORAGE["Object Storage<br/>(S3-compatible, future)"]
    end

    USER --> FE
    USER --> API
    FE --> API
    API --> DB
    RELAY --> DB
    RELAY --> PDF
    PDF --> DB
    PDF --> STORAGE
    RELAY -.->|future| EMAIL
```

## Data Model ERD

```mermaid
erDiagram
    tenants ||--o{ users : has
    tenants ||--o{ customers : has
    tenants ||--o{ invoices : has
    tenants ||--o{ subscriptions : has
    tenants ||--o{ tax_rules : has
    tenants ||--|| invoice_sequences : has

    customers ||--o{ invoices : receives
    customers ||--o{ subscriptions : has
    invoices ||--o{ invoice_items : contains
    invoices ||--o{ payments : has
    invoices ||--o{ audit_entries : tracked_by

    tenants {
        uuid id PK
        text name
        jsonb address
        text tax_id
        jsonb settings
        timestamptz created_at
    }

    users {
        uuid id PK
        uuid tenant_id FK
        text email
        text role
        text password_hash
    }

    customers {
        uuid id PK
        uuid tenant_id FK
        text name
        text email
        jsonb address
        text tax_id
        timestamptz deleted_at
    }

    invoices {
        uuid id PK
        uuid tenant_id FK
        uuid customer_id FK
        text number
        text status
        timestamptz issue_date
        timestamptz due_date
        text currency
        bigint subtotal
        bigint tax_total
        bigint total
        timestamptz deleted_at
    }

    invoice_items {
        uuid id PK
        uuid invoice_id FK
        text description
        bigint quantity
        bigint unit_price
        bigint tax_rate_bps
        bigint discount_bps
        bigint total
    }

    payments {
        uuid id PK
        uuid invoice_id FK
        uuid tenant_id FK
        bigint amount
        text currency
        text method
        text reference
        timestamptz paid_at
    }

    subscriptions {
        uuid id PK
        uuid tenant_id FK
        uuid customer_id FK
        jsonb plan_items
        text frequency
        timestamptz next_billing_date
        text status
    }

    outbox {
        uuid id PK
        uuid aggregate_id
        uuid tenant_id
        text event_type
        jsonb payload
        text status
        timestamptz created_at
        timestamptz processed_at
    }

    audit_entries {
        uuid id PK
        uuid tenant_id
        text entity_type
        uuid entity_id
        text action
        uuid actor_id
        jsonb payload
        timestamptz created_at
    }

    tax_rules {
        uuid id PK
        uuid tenant_id FK
        text jurisdiction
        text region
        bigint rate_bps
        timestamptz effective_from
        timestamptz effective_to
    }

    invoice_sequences {
        uuid tenant_id PK
        int year PK
        bigint last_seq
    }

    idempotency_records {
        text key PK
        uuid tenant_id PK
        bytea response_body
        int status
        timestamptz expires_at
    }
```