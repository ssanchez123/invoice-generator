-- Reverse of 001_initial_schema.up.sql
DROP POLICY IF EXISTS tenant_isolation_idempotency ON idempotency_records;
DROP POLICY IF EXISTS tenant_isolation_tax_rules ON tax_rules;
DROP POLICY IF EXISTS tenant_isolation_audit ON audit_entries;
DROP POLICY IF EXISTS tenant_isolation_outbox ON outbox;
DROP POLICY IF EXISTS tenant_isolation_subscriptions ON subscriptions;
DROP POLICY IF EXISTS tenant_isolation_payments ON payments;
DROP POLICY IF EXISTS tenant_isolation_items ON invoice_items;
DROP POLICY IF EXISTS tenant_isolation_invoices ON invoices;
DROP POLICY IF EXISTS tenant_isolation_customers ON customers;
DROP POLICY IF EXISTS tenant_isolation_users ON users;

ALTER TABLE idempotency_records DISABLE ROW LEVEL SECURITY;
ALTER TABLE tax_rules DISABLE ROW LEVEL SECURITY;
ALTER TABLE audit_entries DISABLE ROW LEVEL SECURITY;
ALTER TABLE outbox DISABLE ROW LEVEL SECURITY;
ALTER TABLE subscriptions DISABLE ROW LEVEL SECURITY;
ALTER TABLE payments DISABLE ROW LEVEL SECURITY;
ALTER TABLE invoice_items DISABLE ROW LEVEL SECURITY;
ALTER TABLE invoices DISABLE ROW LEVEL SECURITY;
ALTER TABLE customers DISABLE ROW LEVEL SECURITY;
ALTER TABLE users DISABLE ROW LEVEL SECURITY;

DROP TABLE IF EXISTS tax_rules;
DROP TABLE IF EXISTS idempotency_records;
DROP TABLE IF EXISTS audit_entries;
DROP TABLE IF EXISTS outbox;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS invoice_items;
DROP TABLE IF EXISTS invoices;
DROP TABLE IF EXISTS customers;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS tenants;