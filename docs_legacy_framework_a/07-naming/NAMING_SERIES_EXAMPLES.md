> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Naming Series Examples

**Classification:** Reference — Tier 2
**Owner:** `07-naming/NAMING_SERIES_EXAMPLES.md`
**Status:** Frozen at v1.0

---

## Prerequisites

- [`07-naming/NAMING_SERIES_SPEC.md`](NAMING_SERIES_SPEC.md) — Pattern syntax and allocation protocol

---

## Pattern Examples

| Use Case | Pattern | Sample Output |
|----------|---------|---------------|
| Invoice | `INV-{YYYY}-{SEQ:5}` | `INV-2026-00042` |
| Purchase Order | `PO-{YYYY}-{SEQ:5}` | `PO-2026-00001` |
| Sales Order | `SO-{YYYY}-{MM}-{SEQ:4}` | `SO-2026-07-0001` |
| Customer Code | `CUST-{SEQ:6}` | `CUST-000123` |
| Stock Move | `SM-{YYYY}{MM}{DD}-{SEQ:4}` | `SM-20260720-0001` |
| Journal Entry | `JE-{YYYY}-{SEQ:6}` | `JE-2026-000001` |
| Payment | `PAY-{YYYY}-{SEQ:5}` | `PAY-2026-00042` |
| Expense | `EXP-{YYYY}-{MM}-{SEQ:4}` | `EXP-2026-07-0001` |
| Receipt | `REC/{YYYY}/{MM}/{SEQ:6}` | `REC/2026/07/000001` |
| Project | `PRJ-{SEQ:4}` | `PRJ-0023` (never resets) |

---

## Reset Behavior

### Yearly Reset
Pattern contains `{YYYY}` but not `{MM}`:
```
INV-{YYYY}-{SEQ:5}
→ Jan: INV-2026-00001, INV-2026-00002, ...
→ Dec: INV-2026-01247
→ Jan next year: INV-2027-00001 (counter reset)
```

### Monthly Reset
Pattern contains both `{YYYY}` and `{MM}`:
```
SO-{YYYY}-{MM}-{SEQ:4}
→ Jul: SO-2026-07-0001, SO-2026-07-0002
→ Aug: SO-2026-08-0001 (counter reset)
```

### Never Resets
Pattern contains neither `{YYYY}` nor `{MM}`:
```
CUST-{SEQ:6}
→ CUST-000001, CUST-000002, ... (never resets, ever)
```

---

## Gaps in Sequences

Gaps in sequence numbers are expected and acceptable:

- Transaction rollback after `INCR` → gap (counter incremented, entity not created)
- Record deletion does not return the number → permanent gap

Gaps do NOT indicate data loss. They indicate that an allocation was made but the corresponding entity was not persisted (due to rollback or other error).

Auditors should be informed that gaps are normal behavior.

---

## Multi-Prefix Variants

For entities with multiple document types that share a sequence, use separate field definitions:

```go
// Option 1: Two fields, two series (separate counters)
{Name: "invoice_number", Type: def.FieldTypeNamingSeries, Series: "INV-{YYYY}-{SEQ:5}"},
{Name: "credit_note_number", Type: def.FieldTypeNamingSeries, Series: "CN-{YYYY}-{SEQ:5}"},

// Option 2: One field, prefix determined at runtime by status (custom hook logic)
// Not recommended — adds complexity. Prefer separate fields.
```

---

## References

- [`07-naming/NAMING_SERIES_SPEC.md`](NAMING_SERIES_SPEC.md)
