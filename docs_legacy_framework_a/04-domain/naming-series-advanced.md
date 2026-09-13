> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Naming Series — Advanced Patterns"
id: dom-010
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Naming Series](naming-series.md)"
  - "[Settings Patterns](../10-modules/settings-patterns.md)"
  - "[Multi-Branch Tenancy](../06-tenancy/multi-branch.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Naming Series — Advanced Patterns

**DOM-010 | Status: Accepted | Stability: Stable**

Advanced patterns for NamingSeries: branch-specific prefixes, conditional series, multi-segment formats, and reset strategies.

---

## 1. Branch-Specific Prefixes

For multi-branch tenants where each branch has its own document numbering:

```go
{
    Name:              "number",
    Type:              def.FieldNamingSeries,
    Series:            "INV-{YYYY}-{SEQ:5}",
    TenantOverridable: true,
    BranchOverridable: true,  // Branch can also override prefix
}
```

When `BranchOverridable: true` and a branch context is present, the framework uses the branch's prefix setting:

```
INV-2024-00001   (no branch context, or branch with no override)
NBI-2024-00001   (Nairobi branch with prefix override "NBI")
MSA-2024-00001   (Mombasa branch with prefix override "MSA")
```

Each branch maintains its own sequence counter (independent numbering):

```sql
-- naming_series_counters has branch_id column
SELECT current_val FROM naming_series_counters
WHERE tenant_id = $1 AND entity_type = $2 AND series_key = $3 AND branch_id = $4
FOR UPDATE;
```

---

## 2. Yearly Reset

Reset the sequence counter at the start of each year:

```go
{
    Name:       "number",
    Type:       def.FieldNamingSeries,
    Series:     "INV-{YYYY}-{SEQ:5}",
    ResetOnYear: true,
}
```

With `ResetOnYear: true`:
- Sequence resets to 1 on January 1 of each year
- Counter key includes the year: `finance_invoice.number.2024`, `finance_invoice.number.2025`
- Year change is detected by comparing `{YYYY}` in the series format against the current year

```
INV-2024-00347  (December 2024)
INV-2025-00001  (January 2025 — resets)
```

---

## 3. Monthly Reset

For high-volume operations requiring monthly numbering:

```go
{
    Name:        "number",
    Type:        def.FieldNamingSeries,
    Series:      "SO-{YYYY}-{MM}-{SEQ:4}",
    ResetOnMonth: true,
}
```

Counter key: `sales_order.number.2024-03`, `sales_order.number.2024-04`.

```
SO-2024-03-0892  (March 2024)
SO-2024-04-0001  (April 2024 — resets)
```

---

## 4. Multi-Segment Formats

Complex series for regulatory compliance:

```go
// KRA-compliant invoice numbering: PREFIX/YEAR/SEQUENCE
{
    Name:   "etims_number",
    Type:   def.FieldNamingSeries,
    Series: "{PREFIX}/{YYYY}/{SEQ:6}",
    TenantOverridable: true,
    // Tenant can change PREFIX from "INV" to their registered prefix
}
// Output: INV/2024/000001
```

Available tokens:

| Token | Output | Example |
|---|---|---|
| `{YYYY}` | 4-digit year | `2024` |
| `{YY}` | 2-digit year | `24` |
| `{MM}` | 2-digit month | `03` |
| `{DD}` | 2-digit day | `15` |
| `{SEQ:N}` | Zero-padded sequence, N digits | `{SEQ:5}` → `00001` |
| `{PREFIX}` | Tenant/branch prefix override | `INV` (default) or tenant override |
| `{BRANCH}` | Branch code (when branch context) | `NBI`, `MSA` |

---

## 5. Multiple NamingSeries Fields

An entity can have multiple NamingSeries fields for different numbering systems:

```go
Fields: []def.FieldDef{
    {
        Name:   "internal_number",    // Internal tracking number
        Type:   def.FieldNamingSeries,
        Series: "PO-{YYYY}-{SEQ:5}",
    },
    {
        Name:   "supplier_ref",       // Supplier-facing reference (tenant override)
        Type:   def.FieldNamingSeries,
        Series: "{PREFIX}-{SEQ:6}",
        TenantOverridable: true,
    },
},
```

Each field maintains its own independent counter.

---

## 6. Conditional Series (Hook-Based)

For entities where the series depends on a field value:

```go
// Use a before_save hook to set a custom number format
type SalesOrderNumberHook struct {
    SeriesService NamingSeriesService
}

func (h *SalesOrderNumberHook) BeforeCreate(ctx context.Context, record *def.EntityRecord) error {
    orderType, _ := record.Fields["order_type"].(string)

    // Different series for different order types
    var series string
    switch orderType {
    case "Export":
        series = "EXP-{YYYY}-{SEQ:5}"
    case "Local":
        series = "LOC-{YYYY}-{SEQ:5}"
    default:
        series = "SO-{YYYY}-{SEQ:5}"
    }

    number, err := h.SeriesService.Next(ctx, "sales_order.number."+orderType, series)
    if err != nil {
        return fmt.Errorf("SalesOrderNumberHook: next number: %w", err)
    }
    record.Fields["number"] = number

    return nil
}
```

---

## 7. Counter Management

Tenant admins can view and (with caution) reset counters via the Settings UI:

```
/settings/naming-series → Lists all series with current counters
/settings/naming-series/finance_invoice.number → Counter management
```

**Reset warning**: Resetting a counter creates a risk of duplicate numbers if any existing records have numbers from the old sequence. The UI shows a warning and requires confirmation. Only recommended when:
- Tenant is starting fresh after test data cleanup
- The old series is no longer in use (different prefix after override)

---

## Related Documents

- [Naming Series](naming-series.md) — core specification
- [Settings Patterns](../10-modules/settings-patterns.md) — `TenantOverridable` prefix settings
- [Multi-Branch Tenancy](../06-tenancy/multi-branch.md) — branch-specific prefix overrides
