> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "ADR-019: Declarative Filter DSL over Query Builders"
id: adr-019
status: accepted
category: ADR
stability: STABLE
audience: [framework-authors, module-authors]
since: "1.0"
normative-level: normative
related:
  - "[Filter DSL](../05-persistence/filter-dsl.md)"
  - "[Policy Functions](../04-domain/policies.md)"
  - "[Entity Repository](../05-persistence/entity-repository.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# ADR-019: Declarative Filter DSL over Query Builders

**Status: Accepted**

---

## Context

Awo modules need to query entities with complex predicates — combining field equality, ranges, full-text search, and policy-injected row-level filters. The query mechanism must:

- Prevent SQL injection regardless of how predicates are constructed
- Allow `PolicyFunc` row-level filters to be composed with user-supplied filters at the framework layer (not by module authors)
- Support serialization for API query parameters (`GET /api/v1/entities/invoice?filter=...`)
- Be testable without a database connection (filter trees are pure data structures)
- Not require module authors to write SQL

---

## Decision

Use a **Declarative Filter DSL**: a composable predicate tree (Go structs) that the framework translates to parameterized SQL at query time. Module authors and API clients express queries in the DSL; the repository layer translates them.

```go
// Filter composition is pure data — no SQL, no string concatenation
f := filter.And(
    filter.Eq("status", "Active"),
    filter.GtEq("total_kes", decimal.NewFromFloat(10000)),
    filter.Or(
        filter.Eq("assigned_to", userID),
        filter.In("department", departmentIDs),
    ),
)
```

---

## Consequences

### Positive

**SQL injection prevention**: the filter tree never contains SQL strings. The repository layer translates each predicate node to a parameterized query with `$N` placeholders. No string interpolation of user values occurs at any layer.

**PolicyFunc composition**: the framework automatically ANDs the module's `PolicyFunc` result with the caller's filter. Module authors cannot forget to apply row-level security — it is structurally impossible to bypass.

```go
// Framework-internal — module author never writes this
effectiveFilter := filter.And(policyFilter, callerFilter)
```

**Serializable**: the filter tree serializes to a versioned JSON wire format for API query parameters. Clients can send complex filters as JSON; the framework deserializes them to the same tree structure.

**Testable**: unit tests can assert on filter trees without a database:
```go
f := BuildInvoiceFilter(actor)
assert.Equal(t, filter.Eq("status", "Submitted"), f)
```

**JSONB support**: custom field queries use JSONB path notation (`filter.Eq("custom_fields.container_number", "CONT-001")`), which the repository translates to `custom_fields->>'container_number' = $1`.

### Negative

**Not all SQL expressible**: complex SQL (window functions, CTEs, subqueries) cannot be expressed in the Filter DSL. These are handled by dedicated service methods using raw pgx queries — not by routing around the DSL.

**Learning curve**: module authors must learn the DSL rather than writing familiar SQL. Mitigated by comprehensive examples in the documentation.

**Translation layer overhead**: each filter tree must be walked and translated to SQL on every query. Benchmarks show this is sub-microsecond for typical filter trees (5–20 nodes). Negligible compared to network + DB execution time.

---

## Alternatives Considered

### Raw SQL strings

Allow module authors to write SQL predicates directly. Rejected:
- SQL injection risk if any user-supplied value is interpolated
- PolicyFunc composition requires string manipulation — fragile and error-prone
- No serialization path for API filter parameters
- Tests require a real database

### ORM query builder (GORM, ent)

Use an ORM's query builder API. Rejected:
- GORM's implicit tenant filtering via scopes is not enforced structurally — easy to forget
- ORMs abstract away decimal.Decimal handling → float64 coercion risk for money fields
- GORM does not support the `set_tenant_context()` stored procedure flow required for RLS
- Entity schema is dynamically extensible (custom fields) — ORMs assume static schema at compile time

### SQL query templates (sqlc)

Generate typed query functions from SQL templates at build time. Rejected:
- Dynamic filter predicates (PolicyFunc + API filter parameters) cannot be expressed as static SQL
- JSONB path queries for custom fields require dynamic predicate generation
- Would require maintaining two query systems (sqlc for static, custom for dynamic)

---

## Filter DSL Predicate Types

| Predicate | SQL equivalent |
|---|---|
| `filter.Eq(field, val)` | `field = $N` |
| `filter.NotEq(field, val)` | `field != $N` |
| `filter.Lt(field, val)` | `field < $N` |
| `filter.LtEq(field, val)` | `field <= $N` |
| `filter.Gt(field, val)` | `field > $N` |
| `filter.GtEq(field, val)` | `field >= $N` |
| `filter.In(field, vals)` | `field = ANY($N)` |
| `filter.NotIn(field, vals)` | `field != ALL($N)` |
| `filter.IsNull(field)` | `field IS NULL` |
| `filter.IsNotNull(field)` | `field IS NOT NULL` |
| `filter.Like(field, pat)` | `field ILIKE $N` |
| `filter.And(filters...)` | `(A AND B AND ...)` |
| `filter.Or(filters...)` | `(A OR B OR ...)` |
| `filter.Not(f)` | `NOT (A)` |
| `filter.All()` | no predicate (matches all rows) |

---

## Related Documents

- [Filter DSL](../05-persistence/filter-dsl.md) — full specification and usage guide
- [Policy Functions](../04-domain/policies.md) — how PolicyFunc composes with Filter DSL
- [Entity Repository](../05-persistence/entity-repository.md) — the interface that accepts Filter DSL predicates
