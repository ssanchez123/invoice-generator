# ADR-008: Multi-currency + Jurisdiction-aware Tax Rules

## Status
Accepted

## Context
Invoices need to support multiple currencies and tax rules that vary by jurisdiction (country, state, region). This is a core domain complexity that should be modeled explicitly, not bolted on.

Options considered:
- **Hardcode tax rates** — inflexible, wrong
- **Single tax rate per invoice** — too simple, real invoices have per-item taxes
- **Tax rules engine** — model tax as a first-class domain concept with jurisdiction scoping

## Decision
**Money and TaxRule as first-class value objects in the domain.**

### Money
All monetary values are represented as `Money{Amount int64, Currency string}`. Amounts stored as integer minor units (cents) to avoid floating point errors. Arithmetic operations (add, subtract, multiply, allocate) are defined on Money.

### TaxRule
A `TaxRule` defines: jurisdiction (country code + optional region), tax rate (basis points), applicable item categories, and effective date range. Tax rules are resolved per invoice based on the tenant's jurisdiction and the customer's location.

### Invoice calculation
```
Subtotal = Σ(item.quantity × item.unit_price)
TaxTotal = Σ(item.tax_amount)  // computed per-item based on resolved TaxRule
Total = Subtotal - Discounts + TaxTotal
```

Tax resolution happens in the domain layer via a `TaxResolver` port. The infrastructure layer implements it with a tax rules table in PostgreSQL.

## Consequences
- ✅ Correctness: integer cents, no float bugs
- ✅ Flexibility: per-item tax, multiple jurisdictions
- ✅ Extensible: can add tax exemption, reverse charge, etc.
- ⚠️ Complexity: tax rules table needs maintenance
- ⚠️ Display layer must format Money → human-readable per locale