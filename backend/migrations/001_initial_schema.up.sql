-- Migration 001: Initial schema
-- Create all tables with multi-tenancy support

-- Required extension for uuid_generate_v4()
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ========== TENANTS ==========
CREATE TABLE tenants (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        TEXT NOT NULL,
    address     JSONB NOT NULL DEFAULT '{}',
    tax_id      TEXT,
    settings    JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ========== USERS ==========
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id     UUID NOT NULL REFERENCES tenants(id),
    email         TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'member', -- admin, member, viewer
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, email)
);

-- ========== CUSTOMERS ==========
CREATE TABLE customers (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id  UUID NOT NULL REFERENCES tenants(id),
    name       TEXT NOT NULL,
    email      TEXT,
    phone      TEXT,
    address    JSONB NOT NULL DEFAULT '{}',
    tax_id     TEXT,
    metadata   JSONB NOT NULL DEFAULT '{}',
    deleted_at  TIMESTAMPTZ, -- soft delete
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_customers_tenant ON customers(tenant_id) WHERE deleted_at IS NULL;

-- ========== INVOICES ==========
CREATE TABLE invoices (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    customer_id UUID NOT NULL REFERENCES customers(id),
    number      TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'draft', -- draft, issued, paid, overdue, cancelled
    issue_date  TIMESTAMPTZ,
    due_date    TIMESTAMPTZ NOT NULL,
    currency    TEXT NOT NULL DEFAULT 'USD',
    subtotal    BIGINT NOT NULL DEFAULT 0,  -- in minor units (cents)
    tax_total   BIGINT NOT NULL DEFAULT 0,
    total       BIGINT NOT NULL DEFAULT 0,
    notes       TEXT,
    metadata    JSONB NOT NULL DEFAULT '{}',
    deleted_at   TIMESTAMPTZ, -- soft delete
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, number)
);
CREATE INDEX idx_invoices_tenant ON invoices(tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_invoices_status ON invoices(tenant_id, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_invoices_customer ON invoices(tenant_id, customer_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_invoices_due ON invoices(due_date) WHERE status = 'issued';

-- ========== INVOICE ITEMS ==========
CREATE TABLE invoice_items (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    invoice_id   UUID NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    description  TEXT NOT NULL,
    quantity     BIGINT NOT NULL DEFAULT 1,
    unit_price   BIGINT NOT NULL,  -- in minor units
    tax_rate_bps BIGINT NOT NULL DEFAULT 0,  -- basis points (1900 = 19%)
    discount_bps BIGINT NOT NULL DEFAULT 0,
    total        BIGINT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_items_invoice ON invoice_items(invoice_id);

-- ========== PAYMENTS ==========
CREATE TABLE payments (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    invoice_id  UUID NOT NULL REFERENCES invoices(id),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    amount      BIGINT NOT NULL,  -- in minor units
    currency    TEXT NOT NULL,
    method      TEXT NOT NULL, -- bank_transfer, credit_card, cash, crypto
    reference   TEXT,
    paid_at     TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_payments_invoice ON payments(invoice_id);
CREATE INDEX idx_payments_tenant ON payments(tenant_id);

-- ========== SUBSCRIPTIONS ==========
CREATE TABLE subscriptions (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id        UUID NOT NULL REFERENCES tenants(id),
    customer_id      UUID NOT NULL REFERENCES customers(id),
    plan_items       JSONB NOT NULL, -- serialized InvoiceItem templates
    frequency        TEXT NOT NULL, -- daily, weekly, monthly, yearly
    next_billing_date TIMESTAMPTZ NOT NULL,
    status           TEXT NOT NULL DEFAULT 'active',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_subscriptions_due ON subscriptions(next_billing_date) WHERE status = 'active';

-- ========== OUTBOX (Transactional Outbox Pattern) ==========
CREATE TABLE outbox (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    aggregate_id  UUID NOT NULL,
    tenant_id     UUID NOT NULL,
    event_type    TEXT NOT NULL,
    payload       JSONB NOT NULL,
    status        TEXT NOT NULL DEFAULT 'pending', -- pending, processed
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at  TIMESTAMPTZ
);
CREATE INDEX idx_outbox_pending ON outbox(created_at) WHERE status = 'pending';

-- ========== AUDIT ENTRIES (Immutable) ==========
CREATE TABLE audit_entries (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id    UUID NOT NULL,
    entity_type  TEXT NOT NULL, -- invoice, payment, customer, subscription
    entity_id    UUID NOT NULL,
    action       TEXT NOT NULL, -- created, updated, issued, paid, cancelled
    actor_id     UUID, -- user who performed the action (NULL for system)
    payload      JSONB NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- Audit entries are INSERT-only. No UPDATE or DELETE.
CREATE INDEX idx_audit_tenant_entity ON audit_entries(tenant_id, entity_type, entity_id);

-- ========== IDEMPOTENCY RECORDS ==========
CREATE TABLE idempotency_records (
    key           TEXT NOT NULL,
    tenant_id     UUID NOT NULL,
    response_body BYTEA,
    status        INT NOT NULL,
    expires_at    TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (key, tenant_id)
);
CREATE INDEX idx_idempotency_expires ON idempotency_records(expires_at);

-- ========== TAX RULES ==========
CREATE TABLE tax_rules (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id      UUID NOT NULL REFERENCES tenants(id),
    jurisdiction   TEXT NOT NULL, -- ISO 3166-1 alpha-2 country code
    region          TEXT, -- optional region/state code
    rate_bps       BIGINT NOT NULL, -- basis points (1900 = 19%)
    description    TEXT,
    effective_from TIMESTAMPTZ NOT NULL,
    effective_to   TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_tax_rules_jurisdiction ON tax_rules(tenant_id, jurisdiction, region);

-- ========== INVOICE SEQUENCES (gap-resistant numbering) ==========
CREATE TABLE invoice_sequences (
    tenant_id  UUID NOT NULL REFERENCES tenants(id),
    year      INT NOT NULL,
    last_seq  BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (tenant_id, year)
);

-- ========== ROW-LEVEL SECURITY ==========
-- Enable RLS on all tenant-scoped tables
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE customers ENABLE ROW LEVEL SECURITY;
ALTER TABLE invoices ENABLE ROW LEVEL SECURITY;
ALTER TABLE invoice_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE payments ENABLE ROW LEVEL SECURITY;
ALTER TABLE subscriptions ENABLE ROW LEVEL SECURITY;
ALTER TABLE outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE tax_rules ENABLE ROW LEVEL SECURITY;
ALTER TABLE idempotency_records ENABLE ROW LEVEL SECURITY;

-- RLS Policies: tenant_id must match session variable app.tenant_id
CREATE POLICY tenant_isolation_users ON users
    FOR ALL USING (tenant_id = current_setting('app.tenant_id')::UUID);

CREATE POLICY tenant_isolation_customers ON customers
    FOR ALL USING (tenant_id = current_setting('app.tenant_id')::UUID);

CREATE POLICY tenant_isolation_invoices ON invoices
    FOR ALL USING (tenant_id = current_setting('app.tenant_id')::UUID);

-- invoice_items don't have tenant_id, but they're protected via invoice ownership
CREATE POLICY tenant_isolation_items ON invoice_items
    FOR ALL USING (
        EXISTS (
            SELECT 1 FROM invoices
            WHERE invoices.id = invoice_items.invoice_id
            AND invoices.tenant_id = current_setting('app.tenant_id')::UUID
        )
    );

CREATE POLICY tenant_isolation_payments ON payments
    FOR ALL USING (tenant_id = current_setting('app.tenant_id')::UUID);

CREATE POLICY tenant_isolation_subscriptions ON subscriptions
    FOR ALL USING (tenant_id = current_setting('app.tenant_id')::UUID);

CREATE POLICY tenant_isolation_outbox ON outbox
    FOR ALL USING (tenant_id = current_setting('app.tenant_id')::UUID);

CREATE POLICY tenant_isolation_audit ON audit_entries
    FOR ALL USING (tenant_id = current_setting('app.tenant_id')::UUID);

CREATE POLICY tenant_isolation_tax_rules ON tax_rules
    FOR ALL USING (tenant_id = current_setting('app.tenant_id')::UUID);

CREATE POLICY tenant_isolation_idempotency ON idempotency_records
    FOR ALL USING (tenant_id = current_setting('app.tenant_id')::UUID);