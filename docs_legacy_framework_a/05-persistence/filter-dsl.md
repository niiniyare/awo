> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Filter DSL"
id: pers-002
status: accepted
category: SPEC
stability: FROZEN
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[EntityRepository](entity-repository.md)"
  - "[Policy Functions](../04-domain/policies.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Filter DSL

**PERS-002 | Status: Accepted | Stability: Frozen**

This document specifies the Filter DSL: the declarative predicate language used in all `EntityRepository` query and bulk-write operations.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## Table of Contents

1. [Design Principles](#1-design-principles)
2. [Filter Type](#2-filter-type)
3. [Comparison Operators](#3-comparison-operators)
4. [Collection Operators](#4-collection-operators)
5. [Null Operators](#5-null-operators)
6. [String Operators](#6-string-operators)
7. [Logical Composition](#7-logical-composition)
8. [Special Filters](#8-special-filters)
9. [Wire Format](#9-wire-format)
10. [Custom Field Filtering](#10-custom-field-filtering)
11. [DynamicLink Filtering](#11-dynamiclink-filtering)
12. [Filter Security Model](#12-filter-security-model)

---

## 1. Design Principles

The Filter DSL is designed for three purposes:
1. **Domain code** — expressing business queries in hooks, services, and policies
2. **API clients** — transmitting queries over the network in a safe, versioned format
3. **SDUI** — generating filters from user input in list page search forms

The DSL provides no raw SQL escape hatch. All predicates are parameterized and translated to SQL by the store layer. SQL injection through the Filter DSL is structurally impossible.

---

## 2. Filter Type

`Filter` is an opaque predicate tree. It is constructed using builder functions in the `awo.so/awo/filter` package (import alias: `filter`).

```go
import "awo.so/awo/filter"

// Simple equality filter
f := filter.Eq("status", "Draft")

// Composed filter
f := filter.And(
    filter.Eq("status", "Draft"),
    filter.Gt("total_kes", decimal.NewFromInt(10000)),
)
```

A `Filter` is immutable after construction. Builder functions return new `Filter` values.

---

## 3. Comparison Operators

```go
// Equal
filter.Eq(field string, value any) Filter

// Not equal
filter.NotEq(field string, value any) Filter

// Greater than
filter.Gt(field string, value any) Filter

// Greater than or equal
filter.Gte(field string, value any) Filter

// Less than
filter.Lt(field string, value any) Filter

// Less than or equal
filter.Lte(field string, value any) Filter
```

**Type safety:** The `value` argument must be a Go type compatible with the field's declared FieldType. The store layer validates type compatibility before generating SQL. A `string` value for a `Currency` field produces a compile-time error if using typed wrappers, or a runtime `ValidationError` if using the untyped API.

**NULL semantics:** Comparison operators (`Eq`, `NotEq`, `Gt`, etc.) follow SQL NULL semantics. `Eq("field", nil)` generates `field IS NULL`. `NotEq("field", nil)` generates `field IS NOT NULL`. See [§5](#5-null-operators) for explicit NULL operators.

---

## 4. Collection Operators

```go
// Field value is in the provided set
filter.In(field string, values []any) Filter

// Field value is NOT in the provided set
filter.NotIn(field string, values []any) Filter
```

`In` with an empty slice generates a always-false predicate (no records can satisfy membership in an empty set). `NotIn` with an empty slice generates a always-true predicate.

---

## 5. Null Operators

```go
// Field value is NULL
filter.IsNull(field string) Filter

// Field value is NOT NULL
filter.IsNotNull(field string) Filter
```

Use explicit null operators instead of `Eq("field", nil)` when the intent is specifically to check for NULL presence/absence. Explicit operators produce more readable filter representations.

---

## 6. String Operators

String operators apply only to `Data`, `SmallText`, and `LongText` fields. They produce `ILIKE` predicates (case-insensitive).

```go
// Field contains substring (case-insensitive)
// Requires: field declared Searchable: true (GIN trgm index)
filter.Contains(field string, substring string) Filter

// Field starts with prefix (case-insensitive)
filter.StartsWith(field string, prefix string) Filter

// Field ends with suffix (case-insensitive)
filter.EndsWith(field string, suffix string) Filter
```

`Contains` on a non-Searchable field performs a full-table scan using `ILIKE`. For production use at scale, always declare `Searchable: true` on fields used with `Contains`.

---

## 7. Logical Composition

```go
// All sub-filters must be satisfied (SQL AND)
filter.And(filters ...Filter) Filter

// At least one sub-filter must be satisfied (SQL OR)
filter.Or(filters ...Filter) Filter

// Negates a sub-filter (SQL NOT)
filter.Not(f Filter) Filter
```

Composition is unlimited. `And` and `Or` accept any number of arguments. `And` with zero arguments is equivalent to `filter.None()`. `Or` with zero arguments is equivalent to `filter.Impossible()`.

---

## 8. Special Filters

```go
// No predicate — returns all tenant-scoped records (subject to RLS and PolicyFunc)
filter.None() Filter

// Always-false predicate — returns zero records
filter.Impossible() Filter

// Match records modified after a given time (uses updated_at system column)
filter.ModifiedAfter(t time.Time) Filter

// Match records created by a specific actor
filter.CreatedBy(userID uuid.UUID) Filter
```

`filter.None()` is distinct from a nil Filter. Passing `nil` as a filter is a programming error; the repository will return an error. Always use `filter.None()` to express "no additional constraint".

`filter.Impossible()` is used in PolicyFunctions to produce zero-row results for actors without access (preferred over returning an error, which would distinguish the "no access" case from the "no records" case).

---

## 9. Wire Format

Filters transmitted over network boundaries (API requests from external clients) MUST be serialized using the versioned wire format. See [LAW-014](../02-architecture/laws.md#law-014-filter-wire-format-is-versioned).

### Serialization

```go
// Serialize a filter to wire format (JSON)
wireBytes, err := filter.Marshal(f)

// Deserialize from wire format
f, err := filter.Unmarshal(wireBytes)
```

### Wire Format Structure

```json
{
  "v": 1,
  "op": "and",
  "args": [
    {"op": "eq",  "field": "status", "value": "Draft"},
    {"op": "gte", "field": "total_kes", "value": "10000.0000"}
  ]
}
```

**`v` field is mandatory.** Deserializing a filter without a `v` field MUST return an error. The runtime rejects filter inputs without version fields with HTTP 400.

**`v: 1`** is the current version. Future DSL versions increment this value. The deserializer routes to the appropriate version-specific parser.

### Wire Format Operators

| DSL function | Wire `op` value |
|---|---|
| `filter.Eq` | `"eq"` |
| `filter.NotEq` | `"neq"` |
| `filter.Gt` | `"gt"` |
| `filter.Gte` | `"gte"` |
| `filter.Lt` | `"lt"` |
| `filter.Lte` | `"lte"` |
| `filter.In` | `"in"` |
| `filter.NotIn` | `"nin"` |
| `filter.IsNull` | `"null"` |
| `filter.IsNotNull` | `"notnull"` |
| `filter.Contains` | `"contains"` |
| `filter.StartsWith` | `"starts"` |
| `filter.EndsWith` | `"ends"` |
| `filter.And` | `"and"` |
| `filter.Or` | `"or"` |
| `filter.Not` | `"not"` |
| `filter.None` | `"none"` |
| `filter.Impossible` | `"impossible"` |

---

## 10. Custom Field Filtering

Custom fields (tenant-defined via the Metadata module) are stored in the `custom_fields jsonb` column. They are filtered using the same DSL operators with a field name prefixed by `custom.`:

```go
// Filter on a custom field named "territory"
filter.Eq("custom.territory", "Nairobi")

// Works with all operators
filter.Contains("custom.notes", "urgent")
filter.IsNull("custom.approval_date")
```

The store layer generates JSONB path expressions: `custom_fields->>'territory' = $1`.

GIN indexes on `custom_fields` make custom field filtering efficient for equality and containment operators. Range operators on custom fields may be slower (JSON values are compared as strings unless explicitly cast).

---

## 11. DynamicLink Filtering

DynamicLink fields store two values: type and ID. Filter using compound predicates:

```go
// Find all attachments linked to a specific invoice
filter.DynamicLinkEq("parent", "finance_invoice", invoiceID)

// Equivalent to:
filter.And(
    filter.Eq("parent_type", "finance_invoice"),
    filter.Eq("parent_name", invoiceID),
)
```

`filter.DynamicLinkEq` is a convenience wrapper for the compound predicate.

---

## 12. Filter Security Model

Filters submitted by external clients (API requests) cannot escape tenant isolation:

1. **PolicyFunc is always applied** — the entity's PolicyFunc predicate is ANDed with any external filter. External filters cannot override the policy.
2. **RLS is always applied** — PostgreSQL RLS filters to the current tenant regardless of what filter the application layer constructs.
3. **No raw SQL** — the DSL has no raw SQL operator. All predicates are parameterized.
4. **Field name validation** — the store layer validates that each field name in the filter corresponds to a declared field on the entity (or a `custom.` prefixed custom field). Unknown field names produce a `ValidationError`, not SQL injection.

An external client constructing a filter that attempts to read other tenants' data will receive an empty result set, not an error and not other tenants' data.

---

## Related Documents

- [EntityRepository](entity-repository.md) — the interface that consumes Filters
- [Policy Functions](../04-domain/policies.md) — PolicyFunc uses filter.None(), filter.Eq(), etc.
- [Architecture Laws](../02-architecture/laws.md) — LAW-014 (wire format versioning)
- [Glossary](../GLOSSARY.md) — Filter, Filter DSL
