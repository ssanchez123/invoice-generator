# ADR-008: Immutable invoice snapshots at issuance

## Status
Accepted

## Context
PDF generation is asynchronous. Loading the current customer and tenant when the worker runs can produce a document that differs from the invoice at issuance. The application must preserve the issue-time parties, line items, monetary totals, notes, and metadata so retries and later downloads remain reproducible.

## Decision
Persist a versioned JSONB snapshot on the invoice in the same database transaction that changes its status from `draft` to `issued` and writes the `invoice.issued` outbox event. The repository captures seller and customer data under row locks and includes the issued invoice fields and item calculations. A PostgreSQL trigger prevents the snapshot from being changed after it is set.

The PDF worker loads and validates this snapshot and renders only from it; it does not read current customer or tenant profile data. Invoices already issued before this migration remain without snapshots. The worker rejects those records rather than implying that current profile data is historical.

## Rationale
- Snapshot and event commit atomically, so a delivered issue event cannot precede its document data.
- JSONB keeps the historic payload together with the invoice and allows versioned evolution.
- A database trigger protects immutability even if a future code path bypasses repository conventions.
- The worker no longer depends on mutable customer and seller profiles.

## Trade-offs
- The snapshot duplicates customer, tenant, and line data.
- New snapshot schema versions require renderer compatibility or an explicit migration.
- Legacy issued invoices cannot be faithfully reconstructed and need an explicit operational policy.

## Consequences
- Every newly issued invoice has a complete snapshot before the outbox event is committed.
- PDF retries produce the same business content even after profile changes.
- Legacy snapshotless invoices fail PDF generation explicitly.
- Jurisdiction-specific fiscal fields and certified e-invoicing remain out of scope until a target jurisdiction is selected.
