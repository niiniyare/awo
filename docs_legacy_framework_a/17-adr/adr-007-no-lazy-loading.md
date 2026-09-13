> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "ADR-007: No Lazy Loading for Edges"
id: adr-007
status: accepted
category: ADR
stability: FROZEN
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: informative
related:
  - "[Edges](../04-domain/edges.md)"
  - "[EntityRepository](../05-persistence/entity-repository.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
---

# ADR-007: No Lazy Loading for Edges

**Status:** Accepted
**Date:** 2024-02-05
**Authors:** Framework Team

---

## Context

Awo's entity model includes edges (relationships between entities): one-to-many, one-to-one, many-to-many. When a record is fetched, related records may or may not be needed. The framework needed to decide how related records are loaded.

---

## Options Considered

### Option A: Lazy Loading (ORM-Style)

Accessing `record.Edges["interactions"]` triggers an automatic database query on first access.

Pros: Convenient for simple cases — related data available transparently.

Cons:
- N+1 query problem: a list of 20 contacts, each accessing their interactions = 21 queries
- Queries happen invisibly inside business logic — difficult to audit query count
- Hard to test (mock must simulate lazy-load behavior)
- Incompatible with transaction boundaries (lazy load after transaction ends is inconsistent)
- PgBouncer transaction mode: lazy load after `COMMIT` cannot access the same transaction context

### Option B: Eager Loading (Always Fetch All Edges)

All edges fetched automatically on every entity read.

Cons:
- Catastrophic for list views: fetching 20 contacts always loads all their interactions
- Over-fetches data the request does not need (API response includes all edge data always)

### Option C: Explicit Loading via QueryOptions

Related data loaded only when explicitly requested via `entity.WithEdge("interactions")`:

```go
contact, _, err := repo.Query(ctx, filter.Eq("id", contactID),
    entity.WithEdge("interactions"),
)
```

Pros:
- No N+1 queries by default — developer must consciously decide to load edges
- All database queries visible in the code — auditable and testable
- No magic — what you see is what queries run
- Performance: list views never over-fetch related data

Cons:
- More verbose: developers must explicitly opt-in
- Can be forgotten — accessing `record.Edges["interactions"]` without WithEdge returns nil

---

## Decision

**No lazy loading. Edges are loaded only via explicit `WithEdge` QueryOptions.**

The N+1 query problem has caused production performance incidents in many ORMs. Making edge loading explicit ensures developers are aware of the query cost. The verbosity is a feature — it forces consideration of performance at the point of use.

---

## Consequences

**Positive:**
- No accidental N+1 queries in list views
- All queries visible in business logic code — auditable
- Test mocks are simple (no lazy-load simulation required)

**Negative:**
- Edge access returns nil if not requested — potential for nil pointer panics in code that forgets to request an edge
- More verbose query calls than ORM-style code

**Architecture Laws generated:**
- Documented in CLAUDE.md: "Never lazy-load edges — explicitly request related data via QueryOptions"

---

## Revisit Trigger

No planned revisit. The explicit loading model aligns with the framework's principle of making costs visible at the call site. If ORM-style lazy loading is added in the future, it must be opt-in per entity type, not the default.
