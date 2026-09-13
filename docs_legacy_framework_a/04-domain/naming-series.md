> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Naming Series"
id: dom-006
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: normative
related:
  - "[Fields](fields.md)"
  - "[EntityDefinition](../03-kernel/entity-def.md)"
  - "[System Entities](../05-persistence/system-entities.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Naming Series

**DOM-006 | Status: Accepted | Stability: Stable**

This document specifies the NamingSeries field type: format string syntax, atomic counter behavior, tenant overrides, and reset policies.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. What NamingSeries Is

A NamingSeries field generates a human-readable sequential identifier for each record: `INV-2024-00001`, `PO-2024-00042`, `HR-2025-00001`. These identifiers are:

- **Unique per tenant** — the sequence counter is tenant-scoped
- **Atomic** — concurrent creates never produce duplicate numbers
- **Read-only after assignment** — value is set on create, rejected on update
- **Tenant-overridable** — the prefix may be changed per tenant without code changes

---

## 2. Format String Syntax

The `Series` field on `FieldDef` uses format placeholders:

| Placeholder | Expansion | Notes |
|---|---|---|
| `{YYYY}` | 4-digit year | Resets sequence when year changes (if ResetOnYear: true) |
| `{YY}` | 2-digit year | — |
| `{MM}` | 2-digit month | — |
| `{DD}` | 2-digit day | — |
| `{SEQ:N}` | Zero-padded sequence | N = minimum width (e.g., `{SEQ:5}` → `00001`) |
| `{SEQ}` | Sequence without padding | Use only when number of records is known to be small |

### Examples

| Series format | Example output |
|---|---|
| `INV-{YYYY}-{SEQ:5}` | `INV-2024-00001` |
| `PO-{YY}{MM}-{SEQ:4}` | `PO-2403-0042` |
| `HR-EMP-{SEQ:6}` | `HR-EMP-000001` |
| `{YYYY}/{SEQ:5}` | `2024/00001` |

---

## 3. Field Declaration

```go
{
    Name:            "number",
    Type:            def.FieldNamingSeries,
    Label:           "Invoice #",
    Series:          "INV-{YYYY}-{SEQ:5}",
    TenantOverridable: true,   // allows tenants to change prefix via Settings
    ResetOnYear:     true,     // sequence resets to 1 at the start of each year
}
```

---

## 4. Atomic Counter Mechanism

The sequence counter is stored per tenant per series key in a dedicated `naming_series_counters` table:

```sql
CREATE TABLE naming_series_counters (
    tenant_id   uuid    NOT NULL,
    series_key  text    NOT NULL,  -- e.g. "finance_invoice.number.INV-2024"
    current_val bigint  NOT NULL DEFAULT 0,
    PRIMARY KEY (tenant_id, series_key)
);
```

Assignment uses `UPDATE ... RETURNING` in a serialized transaction:

```sql
UPDATE naming_series_counters
SET current_val = current_val + 1
WHERE tenant_id = $1 AND series_key = $2
RETURNING current_val;
```

If no row exists, one is inserted with `current_val = 1` via `INSERT ... ON CONFLICT DO NOTHING` + retry pattern. This guarantees atomicity without application-level locking.

The `series_key` encodes the tenant, entity, field, and time-based reset group: `{entity}.{field}.{prefix-with-year}`. This allows year-based reset without affecting other series keys.

---

## 5. Tenant Override

When `TenantOverridable: true`, tenants may change the series prefix via the Settings module:

```
Setting key: {entity}.{field}.series_prefix
Default:     "INV" (extracted from the declaration's Series format)
Tenant sets: "ACME-INV"
Result:      "ACME-INV-2024-00001"
```

The override changes the prefix only — the date and sequence placeholders remain.

Changing the prefix does NOT reset the sequence counter. If a tenant changes from `INV-{YYYY}-{SEQ:5}` to `ACME-{YYYY}-{SEQ:5}`, the next value uses the counter from the previous prefix's series_key (or starts a new counter if the prefix never had records).

---

## 6. ResetOnYear

When `ResetOnYear: true`, the sequence counter resets to 1 at the start of each calendar year. The year is determined by the record's `created_at` timestamp in the tenant's configured timezone.

The `series_key` includes the year: `finance_invoice.number.INV-2024`. A new year produces a new key: `finance_invoice.number.INV-2025` — this naturally resets the counter.

When `ResetOnYear: false`, the sequence is perpetual — the counter never resets regardless of year.

---

## 7. NamingSeries in SDUI

NamingSeries fields are:
- **Absent from create forms** — the value is assigned by the framework, not the user
- **Read-only in edit forms** — declared `Immutable: true` internally
- **Displayed in list and detail views** — sortable column

SDUI generates the appropriate read-only display without module authors doing anything special.

---

## 8. Searching by NamingSeries

NamingSeries fields support exact search (by full reference number) and prefix search:

```go
// Exact match
filter.Eq("number", "INV-2024-00042")

// Prefix match (for search-as-you-type)
filter.StartsWith("number", "INV-2024-")
```

`StartsWith` on a NamingSeries field uses a B-tree index range scan (not full-text search).

---

## Related Documents

- [Fields](fields.md) — complete FieldDef reference
- [System Entities](../05-persistence/system-entities.md) — `naming_series_counters` table context
- [Settings](../10-modules/platform-modules.md#5-settings-module) — tenant override via Settings module
- [Glossary](../GLOSSARY.md) — NamingSeries, naming_series_counters
