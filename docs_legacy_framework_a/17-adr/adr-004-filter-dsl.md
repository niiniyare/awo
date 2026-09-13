> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "ADR-004: Declarative Filter DSL over Query Builders"
id: adr-004
status: accepted
category: ADR
stability: FROZEN
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: informative
related:
  - "[Filter DSL](../05-persistence/filter-dsl.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
---

# ADR-004: Declarative Filter DSL over Query Builders

**Status:** Accepted
**Date:** 2024-01-22
**Authors:** Framework Team

---

## Context

Queries in Awo need to be: composable by PolicyFunc (row-level security), safe against injection, serializable for API wire format, applicable to both SQL columns and JSONB fields, and auditable.

The team evaluated approaches to expressing query predicates.

---

## Options Considered

### Option A: Raw SQL Predicates in Application Code

Module code assembles SQL strings with parameterized placeholders.

Cons:
- SQL injection risk when field names come from user input
- Cannot compose PolicyFunc predicates (arbitrary SQL cannot be safely ANDed with arbitrary SQL)
- Cannot serialize to wire format (SQL is execution format, not transport format)
- Different code paths for SQL columns vs JSONB fields

### Option B: ORM Query Builder (e.g., GORM, ent)

Use an ORM's fluent API for query construction.

Pros: Type-safe, auto-escape.

Cons:
- ORM types leak into business logic layer (violates five-layer separation)
- PolicyFunc cannot safely inject predicates into ORM query objects (ORM internals are opaque)
- JSONB field queries require ORM escape hatches (back to raw SQL)
- ORM model types replace `EntityRecord` — incompatible with the dynamic entity model

### Option C: Declarative Filter DSL

A tree of composable predicate nodes: `And(Eq("status", "Draft"), Eq("assigned_to", userID))`.

Pros:
- PolicyFunc can append predicates safely (AND composition)
- Serializable to a versioned wire format (clients can send filters)
- Field names validated against CompiledSchema (injection-safe)
- Single code path for SQL columns and JSONB fields (store layer handles both)
- Immutable predicate values (safe to store and pass around)

Cons:
- Not as expressive as raw SQL for complex queries (subqueries, window functions)
- Requires version management of the wire format

---

## Decision

**Implement a declarative Filter DSL as the only predicate expression mechanism.**

The PolicyFunc composition requirement alone makes this decision clear: we cannot safely compose arbitrary query predicates from different sources without a structured representation. The Filter DSL makes composition, injection safety, and serialization first-class properties.

---

## Consequences

**Positive:**
- PolicyFunc composition is safe and guaranteed — `And(policyFilter, queryFilter)` always produces correct SQL
- Field names validated at query construction time — injection-safe
- Wire format versioned — `"v": 1` in JSON; clients sending unversioned filters are rejected
- Single abstraction for SQL and JSONB fields

**Negative:**
- Complex queries (subqueries, aggregates beyond `COUNT`/`SUM`/`AVG`) require store-layer extensions
- Filter wire format versioned — breaking changes require new version coordination

**Architecture Laws generated:**
- LAW-014: Filter wire format versioned; unversioned filters rejected

---

## Revisit Trigger

If the Filter DSL proves insufficient for complex reporting queries, add a separate `ReportRepository` interface with a richer query language. The core `EntityRepository` Filter DSL remains for operational queries; reporting queries use a separate interface.
