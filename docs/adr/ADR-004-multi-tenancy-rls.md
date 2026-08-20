# ADR-004: Multi-tenancy via tenant_id + Row-Level Security

## Status
Accepted

## Context
The system is SaaS-ready: multiple organizations (tenants) use the same instance. We need tenant isolation that is robust and doesn't require separate databases per tenant.

Options considered:
- **Database per tenant** — strongest isolation, operationally expensive
- **Schema per tenant** — moderate isolation, migration complexity
- **Shared DB, shared schema + tenant_id** — least isolation, simplest ops
- **Shared DB, shared schema + tenant_id + RLS** — balance of isolation and simplicity

## Decision
**Shared database, shared schema, with `tenant_id` column on every table + PostgreSQL Row-Level Security policies.**

Every table that belongs to a tenant includes a `tenant_id` column. PostgreSQL RLS policies ensure that queries scoped to a session can only access rows matching the current tenant (set via session variable `app.tenant_id`).

The application sets `SET app.tenant_id = <id>` at the start of each request transaction. RLS policies automatically filter all queries.

```sql
CREATE POLICY tenant_isolation ON invoices
  FOR ALL
  USING (tenant_id = current_setting('app.tenant_id')::uuid);
```

## Consequences
- ✅ Single database to manage and back up
- ✅ RLS provides defense-in-depth: even a bug in application code can't leak cross-tenant data
- ✅ Easy to add new tenants (no schema/DB provisioning)
- ✅ Connection pooling works normally
- ⚠️ Must remember to set tenant_id on every connection
- ⚠- Performance: RLS adds a filter to every query (negligible with proper indexes)
- ⚠️ Migrations must include tenant_id on every new table