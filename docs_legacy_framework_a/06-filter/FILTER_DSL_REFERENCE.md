> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Filter DSL Reference

**Classification:** Reference — Tier 0
**Owner:** `06-filter/FILTER_DSL_REFERENCE.md`
**Status:** Frozen at v1.0
**Package:** `awo.so/awo/filter`

---

## Purpose

This document is the exhaustive reference for the Awo Filter DSL — all constructors, the `Filter` struct, composition rules, and the custom-field predicate variants.

---

## 1. Filter Struct

```go
// Package: awo.so/awo/filter

type Filter struct {
    Kind  Kind
    Field string   // set for leaf predicates
    Value any      // set for single-value predicates
    Lo    any      // set for KindBetween (lower bound)
    Hi    any      // set for KindBetween (upper bound)
    In    []any    // set for KindIn, KindNotIn
    Sub   []*Filter // set for KindAnd, KindOr, KindNot
}
```

The `Filter` struct is immutable after construction. All constructors return `*Filter`. The zero value is invalid — always use a constructor.

---

## 2. Scalar Predicates

### Eq(field, value) — field = value

```go
filter.Eq("status", "Active")
filter.Eq("customer_id", customerUUID)
filter.Eq("amount", decimal.NewFromFloat(1000.50))
filter.Eq("is_active", true)
filter.Eq("created_at", time.Now())

// Special case: nil is equivalent to IsNull
filter.Eq("deleted_at", nil)  // same as filter.IsNull("deleted_at")
```

Accepted value types: `string`, `int64`, `bool`, `uuid.UUID`, `decimal.Decimal`, `time.Time`, `nil`.

---

### Neq(field, value) — field != value

```go
filter.Neq("status", "Cancelled")
```

---

### Gt(field, value) — field > value

```go
filter.Gt("amount", decimal.NewFromFloat(0))
filter.Gt("created_at", thirtyDaysAgo)
```

---

### Gte(field, value) — field >= value

```go
filter.Gte("age", int64(18))
```

---

### Lt(field, value) — field < value

```go
filter.Lt("stock_level", int64(10))
```

---

### Lte(field, value) — field <= value

```go
filter.Lte("due_date", time.Now())
```

---

### Between(field, lo, hi) — lo <= field <= hi

```go
filter.Between("amount", decimal.NewFromFloat(100), decimal.NewFromFloat(10000))
filter.Between("created_at", startDate, endDate)
```

Both bounds are inclusive.

---

## 3. Set Predicates

### In(field, values...) — field IN (values...)

```go
filter.In("status", "Draft", "Submitted")
filter.In("country_code", "KE", "UG", "TZ")
filter.In("priority", int64(1), int64(2), int64(3))
```

Empty `values` produces a filter that never matches (equivalent to `FALSE`).

---

### NotIn(field, values...) — field NOT IN (values...)

```go
filter.NotIn("status", "Cancelled", "Archived")
```

---

### InUUIDs(field, ids) — convenience for []uuid.UUID

```go
filter.InUUIDs("customer_id", []uuid.UUID{id1, id2, id3})
```

---

### InStrings(field, strs) — convenience for []string

```go
filter.InStrings("status", []string{"Draft", "Submitted"})
```

---

## 4. Null Predicates

### IsNull(field) — field IS NULL

```go
filter.IsNull("deleted_at")
```

---

### IsNotNull(field) — field IS NOT NULL

```go
filter.IsNotNull("submitted_at")
```

---

## 5. String Predicates

### Contains(field, value) — ILIKE '%value%'

```go
filter.Contains("name", "acme")
filter.Contains("email", "@gmail.com")
```

Uses GIN trigram index when the field has `Searchable: true`. Full table scan otherwise. Avoid `Contains` on non-searchable fields in large tables.

Case-insensitive.

---

### StartsWith(field, value) — ILIKE 'value%'

```go
filter.StartsWith("invoice_number", "INV-2026")
```

Can use B-tree index. Case-insensitive.

---

### EndsWith(field, value) — ILIKE '%value'

```go
filter.EndsWith("email", ".co.ke")
```

Cannot use B-tree index (leading wildcard). Avoid on large tables unless trigram index exists.

---

## 6. Logical Combinators

### And(filters...) — all must match

```go
filter.And(
    filter.Eq("status", "Active"),
    filter.Gte("amount", decimal.NewFromFloat(100)),
    filter.IsNotNull("customer_id"),
)
```

**Nil filtering:** `And` ignores nil filters in the input slice. This enables conditional filter construction:

```go
var filters []*filter.Filter
filters = append(filters, filter.Eq("status", "Active"))
if req.CustomerID != uuid.Nil {
    filters = append(filters, filter.Eq("customer_id", req.CustomerID))
}
f := filter.And(filters...)  // works even if only one filter
```

**Simplification:** `And` with one non-nil filter returns that filter unchanged (not wrapped in AND). `And` with zero non-nil filters returns nil.

---

### Or(filters...) — at least one must match

```go
filter.Or(
    filter.Eq("status", "Draft"),
    filter.Eq("status", "Submitted"),
)
```

**Simplification:** Same as `And` — single filter returned unchanged; zero filters returns nil.

---

### Not(f) — logical negation

```go
filter.Not(filter.Eq("status", "Cancelled"))
filter.Not(filter.In("country", "KE", "UG"))
```

`Not(nil)` returns nil.

---

## 7. Custom Field Predicates (JSONB)

For custom fields stored in `custom_fields jsonb`, use the `Custom*` constructors. These generate JSONB path expressions.

### CustomEq(field, value) — custom_fields->>'field' = value

```go
filter.CustomEq("industry", "Manufacturing")
filter.CustomEq("tier", "Gold")
```

---

### CustomGt(field, value decimal.Decimal)

```go
filter.CustomGt("credit_limit", decimal.NewFromFloat(50000))
```

Generates: `(custom_fields->>'field')::numeric > value`

---

### CustomLt(field, value decimal.Decimal)

```go
filter.CustomLt("risk_score", decimal.NewFromFloat(0.5))
```

---

### CustomIn(field, values...string)

```go
filter.CustomIn("category", "A", "B", "C")
```

---

### CustomIsNull(field)

```go
filter.CustomIsNull("referral_code")
```

---

## 8. Composition Examples

### Paginated search with multiple conditions

```go
f := filter.And(
    filter.Eq("tenant_id", tenantID),      // usually injected by RLS; shown for clarity
    filter.Eq("status", "Active"),
    filter.Contains("name", searchQuery),
    filter.Gte("created_at", startDate),
)
```

### Policy filter (owner-only)

```go
// PolicyFunc for owner-only row visibility:
func ownerOnlyPolicy(ctx context.Context) def.Filter {
    viewer := auth.ViewerFromContext(ctx)
    return filter.Eq("created_by", viewer.UserID())
}
```

### Combining request filter with policy filter

```go
// The runtime AND's the policy filter with the request filter automatically.
// Module authors do not need to do this manually.
combined := filter.And(requestFilter, policyFilter)
```

---

## 9. Filter as ActionFilter

`*filter.Filter` implements `def.ActionFilter` (an empty interface). Action handlers pass filters to `ActionEntityRepo` query methods:

```go
func MyAction(ctx context.Context, action def.ActionContext) (*def.ActionResult, error) {
    repo := action.Runtime.Repo("finance_invoice")
    records, err := repo.Query(ctx,
        filter.And(
            filter.Eq("status", "Draft"),
            filter.Eq("customer_id", action.RecordID),
        ),
        def.WithActionLimit(50),
    )
    // ...
}
```

---

## References

- `awo/filter/filter.go` — Full implementation
- [`06-filter/FILTER_POLICY_PATTERNS.md`](FILTER_POLICY_PATTERNS.md) — PolicyFunc patterns
- [`13-actions/ACTION_RUNTIME_REFERENCE.md`](../13-actions/ACTION_RUNTIME_REFERENCE.md) — ActionEntityRepo.Query()
