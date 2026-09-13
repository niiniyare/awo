> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "filter Package Reference"
id: pers-013
status: accepted
category: SPEC
stability: FROZEN
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Filter DSL](filter-dsl.md)"
  - "[Entity Repository](entity-repository.md)"
  - "[Policy Functions](../04-domain/policies.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# `filter` Package Reference

**PERS-013 | Status: Accepted | Stability: Frozen**

Complete API reference for `awo.so/awo/filter` — the package that provides the declarative predicate DSL used in all repository queries and policy functions.

---

## 1. Import Path

```go
import "awo.so/awo/filter"
```

The `filter` package is the **only** way to express query predicates in Awo. Raw SQL predicates in application code are forbidden (see LAW-003).

---

## 2. Core Type

```go
// Filter is a node in a predicate tree.
// A nil *Filter means "match all" (no restriction).
type Filter struct {
    Kind  Kind
    Field string    // field name (for leaf nodes)
    Value any       // comparison value (for leaf nodes)
    Values []any    // multiple values (In, NotIn, Between)
    Children []*Filter // sub-predicates (And, Or, Not)

    // JSONB-specific
    JSONPath string  // dot-notation path within a JSONB field
}
```

`Filter` is value-safe: constructed via constructor functions, never mutated after creation.

---

## 3. `Kind` Constants

```go
type Kind int

const (
    KindNone      Kind = iota  // nil filter — match all
    KindEq                     // field = value
    KindNeq                    // field != value
    KindGt                     // field > value
    KindGte                    // field >= value
    KindLt                     // field < value
    KindLte                    // field <= value
    KindBetween                // value[0] <= field <= value[1]
    KindIn                     // field IN (values...)
    KindNotIn                  // field NOT IN (values...)
    KindIsNull                 // field IS NULL
    KindIsNotNull              // field IS NOT NULL
    KindContains               // field ILIKE '%value%'
    KindStartsWith             // field ILIKE 'value%'
    KindEndsWith               // field ILIKE '%value'
    KindAnd                    // all children must match
    KindOr                     // any child must match
    KindNot                    // child must NOT match
    KindJSONPath               // JSONB: field->>'path' = value
    KindJSONContains           // JSONB: field @> value
    KindTrue                   // always true (match all rows)
    KindFalse                  // always false (match no rows)
)
```

---

## 4. Constructor Functions

### Comparison

```go
// Eq builds: field = value
func Eq(field string, value any) *Filter

// Neq builds: field != value
func Neq(field string, value any) *Filter

// Gt builds: field > value
func Gt(field string, value any) *Filter

// Gte builds: field >= value
func Gte(field string, value any) *Filter

// Lt builds: field < value
func Lt(field string, value any) *Filter

// Lte builds: field <= value
func Lte(field string, value any) *Filter

// Between builds: low <= field <= high
func Between(field string, low, high any) *Filter
```

### Set Membership

```go
// In builds: field IN (values...)
// values must be a non-empty slice.
func In(field string, values []any) *Filter

// NotIn builds: field NOT IN (values...)
func NotIn(field string, values []any) *Filter
```

### Null Checks

```go
// IsNull builds: field IS NULL
func IsNull(field string) *Filter

// IsNotNull builds: field IS NOT NULL
func IsNotNull(field string) *Filter
```

### String Matching

```go
// Contains builds: field ILIKE '%value%'
// Case-insensitive. Requires Searchable field or sequential scan.
func Contains(field, value string) *Filter

// StartsWith builds: field ILIKE 'value%'
func StartsWith(field, value string) *Filter

// EndsWith builds: field ILIKE '%value'
func EndsWith(field, value string) *Filter
```

### Logical Composition

```go
// And builds a conjunction — ALL children must match.
// Nil children are ignored.
// And() with zero non-nil children returns nil (match all).
func And(children ...*Filter) *Filter

// Or builds a disjunction — ANY child must match.
// Or() with zero non-nil children returns nil (match all).
func Or(children ...*Filter) *Filter

// Not negates a filter.
// Not(nil) returns nil.
func Not(f *Filter) *Filter
```

### Sentinel Values

```go
// True returns a filter that matches all rows explicitly.
// Different from nil (match all implicitly) — use True() when
// you need to return a valid non-nil filter from PolicyFunc that means "no restriction".
func True() *Filter

// False returns a filter that matches no rows.
// Use in PolicyFunc when the actor has no access.
func False() *Filter
```

### JSONB

```go
// JSONPath builds: (field->>'path') = value
// Use for querying inside custom_fields or JSON columns.
// path uses dot notation: "address.city"
func JSONPath(field, path string, value any) *Filter

// JSONContains builds: field @> value::jsonb
// Use for checking if a JSONB column contains a sub-document.
func JSONContains(field string, value map[string]any) *Filter
```

---

## 5. Usage Examples

### Basic query

```go
// All submitted invoices for a customer
f := filter.And(
    filter.Eq("status", "Submitted"),
    filter.Eq("customer", customerID),
)
invoices, pageInfo, err := repo.Query(ctx, f)
```

### Range filter

```go
// Invoices due in the next 30 days
f := filter.And(
    filter.Gte("due_date", time.Now()),
    filter.Lte("due_date", time.Now().AddDate(0, 0, 30)),
    filter.Eq("status", "Submitted"),
)
```

### Set membership

```go
// Draft or Cancelled invoices
f := filter.In("status", []any{"Draft", "Cancelled"})
```

### Policy function

```go
var InvoicePolicy = def.PolicyFunc(func(ctx context.Context) *filter.Filter {
    actor := session.ActorFromContext(ctx)
    if actor == nil {
        return filter.False()
    }
    if actor.HasRole("role:tenant.admin") {
        return filter.True()
    }
    return filter.Eq("created_by", actor.UserID)
})
```

### JSONB custom field query

```go
// custom_fields->>'industry' = 'Retail'
f := filter.JSONPath("custom_fields", "industry", "Retail")
```

### Nested AND/OR

```go
// (status = Draft AND created_by = me) OR (status = Submitted AND approver = me)
f := filter.Or(
    filter.And(
        filter.Eq("status", "Draft"),
        filter.Eq("created_by", actor.UserID),
    ),
    filter.And(
        filter.Eq("status", "Submitted"),
        filter.Eq("approver", actor.UserID),
    ),
)
```

---

## 6. Wire Format (API Filter Parameter)

Filters are serialized to JSON for the `?filter=` query parameter:

```json
{
  "and": [
    {"field": "status", "op": "eq", "value": "Submitted"},
    {"field": "customer", "op": "eq", "value": "018e..."}
  ]
}
```

| Go constructor | JSON `"op"` value |
|---|---|
| `Eq` | `"eq"` |
| `Neq` | `"neq"` |
| `Gt` | `"gt"` |
| `Gte` | `"gte"` |
| `Lt` | `"lt"` |
| `Lte` | `"lte"` |
| `Between` | `"between"` |
| `In` | `"in"` |
| `NotIn` | `"not_in"` |
| `IsNull` | `"is_null"` |
| `IsNotNull` | `"is_not_null"` |
| `Contains` | `"contains"` |
| `And` | top-level `"and": [...]` |
| `Or` | top-level `"or": [...]` |
| `Not` | `"not": {...}` |
| `JSONPath` | `"op": "json_path", "path": "..."` |

The repository deserializes the wire format back into a `*Filter` tree before generating SQL. SQL injection is impossible: values are never interpolated into the query string.

---

## 7. SQL Generation Rules

The persistence layer translates `*Filter` to parameterized SQL:

| Kind | SQL generated |
|---|---|
| `KindEq` | `field = $N` |
| `KindIn` | `field = ANY($N)` |
| `KindContains` | `field ILIKE '%' \|\| $N \|\| '%'` |
| `KindIsNull` | `field IS NULL` |
| `KindFalse` | `FALSE` (entire query returns empty) |
| `KindTrue` | *(no condition added)* |
| `KindAnd` | `(child1 AND child2 AND ...)` |
| `KindOr` | `(child1 OR child2 OR ...)` |
| `KindJSONPath` | `(field->>'path') = $N` |
| `KindJSONContains` | `field @> $N::jsonb` |

RLS `tenant_id = current_tenant_id()` is always prepended by the persistence layer — never expressed in a `*Filter`.

---

## Related Documents

- [Filter DSL](filter-dsl.md) — conceptual overview, versioning, policy composition
- [Entity Repository](entity-repository.md) — Query/Count/Exists/Aggregate accept `*Filter`
- [Policy Functions](../04-domain/policies.md) — PolicyFunc returns `*Filter`
- [def Package Reference](../03-kernel/def-package-reference.md) — `def.PolicyFunc` type
