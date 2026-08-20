# ADR-003: PostgreSQL as primary datastore

## Status
Accepted

## Context
Database choice for an invoice generator with multi-tenancy, audit trail, and relational data.

Options considered:
- **PostgreSQL** — ACID, RLS, JSONB, mature
- **MySQL** — popular, weaker JSON support, no RLS
- **MongoDB** — flexible schema, but invoice data is relational
- **SQLite** — great for dev, limited concurrency for SaaS

## Decision
**PostgreSQL 16+.**

Reasoning:
- **Row-Level Security (RLS)** enables tenant isolation at the database level — defense in depth
- **JSONB** columns for flexible metadata/settings without schema migrations
- **ACID compliance** is non-negotiable for financial data
- **LISTEN/NOTIFY** could be used for real-time event delivery
- **pgx** driver in Go provides excellent performance and type-safe queries
- **Migration tooling**: `golang-migrate` for versioned, reversible migrations

## Consequences
- ✅ Relational integrity for invoices → items → payments
- ✅ RLS policies enforce tenant isolation even if application code has a bug
- ✅ JSONB for evolving schemas without migrations
- ✅ Strong ecosystem: pgAdmin, psql, connection pooling (pgx)
- ⚠️ Requires running a separate service (vs SQLite)
- ⚠️ RLS adds complexity to queries and debugging