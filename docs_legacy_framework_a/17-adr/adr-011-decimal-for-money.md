> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "ADR-011: Use decimal.Decimal for All Monetary Amounts"
id: adr-011
status: accepted
category: ADR
stability: FROZEN
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Architecture Laws](../02-architecture/laws.md)"
  - "[Fields](../04-domain/fields.md)"
  - "[Finance Patterns](../10-modules/finance-patterns.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# ADR-011: Use decimal.Decimal for All Monetary Amounts

**Status**: Accepted | **Stability**: Frozen

---

## Context

Early prototypes of the Finance module used `float64` for monetary amounts. During testing, a payroll run for 1,000 employees produced a total that was off by KES 0.40 due to floating-point accumulation error. This is a known limitation of IEEE 754 binary floating-point: `0.1 + 0.2 ≠ 0.3` in binary representation.

The Finance module handles:
- Invoice totals summed from hundreds of line items
- Payroll calculations applying percentage deductions to many employees
- VAT computation (16% of gross — a non-terminating binary fraction)
- Ledger entry balancing (sum of debits must exactly equal sum of credits)

Floating-point rounding errors in any of these contexts produce incorrect financial records, compliance failures (KRA eTIMS rejects records that don't match computed VAT), and audit discrepancies.

---

## Decision

All monetary amounts in Awo use `github.com/shopspring/decimal.Decimal` for Go types and `numeric(20,4)` for PostgreSQL column types.

This is enforced by:
1. `FieldCurrency` field type maps to `numeric(20,4)` SQL and `decimal.Decimal` Go
2. `FieldFloat` is prohibited for fields with semantic monetary meaning (enforced in code review via MDG-012 checklist)
3. LAW-003 permanently prohibits storing money as float
4. The EntityRegistry validator rejects `FieldFloat` on fields named `*_kes`, `*_amount`, `*_price`, `*_cost`, `*_total`, `*_balance`

### Why `numeric(20,4)`

- 20 digits of precision: handles amounts up to KES 9,999,999,999,999,999 (covers all realistic ERP amounts)
- 4 decimal places: KES standard; also sufficient for USD, EUR, and other currencies in common use
- Exact: no binary rounding; `0.1 + 0.2 = 0.3` exactly in `numeric`

### Why `shopspring/decimal`

- Arbitrary precision decimal arithmetic in Go
- Parses directly from `numeric` PostgreSQL values via pgx driver
- Serializes to string in JSON (`"45000.0000"`) — avoids JSON number precision loss
- Widely used in Go financial applications; actively maintained

---

## Alternatives Considered

### `float64` (Rejected)

Fast and built-in, but accumulates rounding errors. KES 0.40 error on a 1,000-employee payroll is unacceptable. Rejected.

### `int64` in smallest unit (e.g., fils/cents) (Considered)

Common in payment processor APIs. Avoids decimal issues by using integers. Rejected because:
- KES has no cents (or uses 1/100 shilling as the minor unit, not universally)
- Many ERP amounts are computed as percentages (16% VAT, 5.125% pension) — truncating to cents introduces its own error
- Legibility: `decimal.NewFromFloat(45000)` is clearer than `int64(4500000)` with an implicit 100x multiplier

### `big.Rat` (Considered)

Go standard library rational numbers. Exact arithmetic, but no standard PostgreSQL type; cumbersome API. Rejected.

---

## Consequences

### Positive

- Zero rounding errors in financial calculations
- KRA eTIMS computed VAT matches stored VAT exactly
- Ledger balancing assertions pass deterministically

### Negative

- Slightly slower arithmetic than `float64` (negligible for ERP workloads)
- JSON serialization as strings (`"45000.0000"`) rather than numbers — API clients must handle string amounts. This is documented in API Conventions (API-001 §4)
- Developers unfamiliar with `shopspring/decimal` need to learn the API

### Mitigation for JSON Consumers

The API response includes both machine-readable (`"45000.0000"`) and human-readable (`"KES 45,000.00"`) representations for Currency fields:

```json
{
  "total_kes":         "45000.0000",
  "total_kes_display": "KES 45,000.00"
}
```

---

## Enforcement

LAW-003 is permanently binding. PRs that introduce `float64` or `FieldFloat` for monetary fields are rejected in code review. The EntityRegistry validator provides a runtime check as a backstop.

---

## Related

- [LAW-003](../02-architecture/laws.md#law-003) — Never store money as float
- [FieldCurrency](../04-domain/fields.md#fieldcurrency) — field type specification
- [Finance Patterns §1](../10-modules/finance-patterns.md#1-currency-field-usage) — usage examples
