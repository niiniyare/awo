# Naming Series Specification

**Classification:** Specification — Tier 1
**Owner:** `07-naming/NAMING_SERIES_SPEC.md`
**Status:** Frozen at v1.0
**Package:** `awo.so/awo/naming`

---

## Purpose

This document specifies the naming series subsystem: pattern syntax, token semantics, counter key derivation, allocation protocol, and tenant override behavior.

---

## 1. What Is a Naming Series

A naming series produces human-readable, unique, sequential identifiers for entity records. Examples:

```
INV-2026-00042      (Invoice #42 of 2026)
ORD-07-2026-000001  (Order #1 of July 2026)
PO-2026-00001       (Purchase Order #1 of 2026)
```

The value is `NULL` on record creation and assigned by an `AfterCreateHook` within the same transaction.

---

## 2. Field Declaration

```go
{
    Name:   "number",
    Type:   def.FieldTypeNamingSeries,
    Series: "INV-{YYYY}-{SEQ:5}",
    Label:  "Invoice Number",
}
```

The `Series` field contains the pattern string. The pattern is validated at compile time by `naming.Validate(pattern)`.

---

## 3. Pattern Syntax

A pattern is a string containing literal text interspersed with tokens enclosed in `{}`.

### Tokens

| Token | Description | Example output |
|-------|-------------|----------------|
| `{YYYY}` | 4-digit calendar year | `2026` |
| `{MM}` | 2-digit month (01–12) | `07` |
| `{DD}` | 2-digit day (01–31) | `20` |
| `{SEQ:N}` | Zero-padded sequence counter, N digits | `{SEQ:5}` → `00042` |

### Rules

- At least one `{SEQ:N}` token MUST appear in every pattern.
- N in `{SEQ:N}` MUST be between 1 and 9.
- Patterns are case-sensitive: `{YYYY}` is correct; `{yyyy}` is invalid.
- Literal text may contain any printable ASCII character except `{` and `}`.

### Valid Pattern Examples

```
INV-{YYYY}-{SEQ:5}          → INV-2026-00042
ORD-{MM}-{YYYY}-{SEQ:6}     → ORD-07-2026-000001
{YYYY}/{MM}/{SEQ:4}         → 2026/07/0042
PO{YYYY}{SEQ:6}             → PO202600001
```

### Invalid Pattern Examples

```
INV-2026-00042              (no {SEQ} token)
INV-{YYYY}-{SEQ:10}        ({SEQ} N > 9)
INV-{yyyy}-{SEQ:5}         (lowercase token)
INV-{YYYY}-{seq:5}         (lowercase token)
```

---

## 4. Counter Key Derivation

The counter is stored in Redis as an integer. The key uniquely identifies one counter for one tenant and one time period.

```
naming:{pattern_hash}:{tenant_id}:{period}
```

Where:
- `pattern_hash` = SHA-256 hex of the `Series` string, truncated to 16 characters
- `tenant_id` = UUID of the owning tenant
- `period` = derived from the reset token:
  - Pattern contains `{MM}` → `{YYYY}-{MM}` (monthly reset)
  - Pattern contains `{YYYY}` but not `{MM}` → `{YYYY}` (yearly reset)
  - Pattern contains neither → `global` (never resets)

**Example keys:**
```
naming:a3f1b2c4d5e6f7a8:550e8400-e29b-41d4-a716-446655440000:2026-07
naming:b4c5d6e7f8a9b0c1:550e8400-e29b-41d4-a716-446655440000:2026
```

---

## 5. Allocation Protocol

The `NamingSeriesService.Allocate()` method is called from an `AfterCreateHook` within the entity transaction:

```go
type AllocateInput struct {
    Pattern  string
    TenantID uuid.UUID
    Now      time.Time   // wall clock at allocation time; use ActionRuntime.Clock()
}

func (s *NamingSeriesService) Allocate(ctx context.Context, input AllocateInput) (string, error)
```

**Atomicity:** Uses Redis `INCR {key}` — atomic, returns the new counter value. No race condition between concurrent allocations.

**Transaction behavior:** The allocation happens in the `AfterCreateHook` stage (inside the entity's PostgreSQL transaction). If the PostgreSQL transaction rolls back, the Redis counter is NOT rolled back — counter values are consumed regardless of entity persistence. This means sequence numbers may have gaps. This is acceptable. Gaps are preferable to duplicate numbers.

**Failure semantics:** If Redis is unavailable during `Allocate`, the hook returns an error, causing the entire entity transaction to roll back. The entity record is not created. This is the correct behavior — an entity record without a series number violates the naming invariant.

---

## 6. Tenant Override

When a field has `TenantOverridable: true`:

```go
{
    Name:             "number",
    Type:             def.FieldTypeNamingSeries,
    Series:           "INV-{YYYY}-{SEQ:5}",
    TenantOverridable: true,
}
```

A tenant administrator may configure a custom prefix via the Settings module. The override replaces the literal prefix of the pattern while keeping the token structure.

Example: Default pattern `INV-{YYYY}-{SEQ:5}`, tenant override prefix `ACME-INV` → produces `ACME-INV-2026-00042`.

The override is stored in the Settings module under key `naming_series_prefix:{entity_name}:{field_name}`.

---

## 7. Preview and Validation

```go
// Preview returns what the next allocation would look like without
// actually incrementing the counter.
func (s *NamingSeriesService) Preview(pattern string, now time.Time, tenantID uuid.UUID) (string, error)

// Validate checks that a pattern is syntactically valid.
// Returns a non-nil error with a descriptive message on failure.
func (s *NamingSeriesService) Validate(pattern string) error
```

---

## 8. Implementation in an AfterCreateHook

```go
type InvoiceNumberAssigner struct {
    Naming *naming.NamingSeriesService
}

func (h *InvoiceNumberAssigner) AfterCreate(ctx context.Context, rec *def.EntityRecord) error {
    value, err := h.Naming.Allocate(ctx, naming.AllocateInput{
        Pattern:  "INV-{YYYY}-{SEQ:5}",
        TenantID: rec.TenantID,
        Now:      time.Now(),  // use ActionRuntime.Clock() in action handlers
    })
    if err != nil {
        return fmt.Errorf("InvoiceNumberAssigner.AfterCreate: %w", err)
    }
    rec.Set("number", value)
    // The runtime will UPDATE the persisted record with the assigned number
    // within the open transaction
    return nil
}
```

---

## References

- `awo/naming/service.go` — NamingSeriesService implementation
- [`07-naming/NAMING_SERIES_EXAMPLES.md`](NAMING_SERIES_EXAMPLES.md) — More examples
- [`02-pipeline/HOOK_CONTRACT.md`](../02-pipeline/HOOK_CONTRACT.md) — AfterCreateHook interface
