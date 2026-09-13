> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "ADR-020: No Lazy Loading for Edges"
id: adr-020
status: accepted
category: ADR
stability: STABLE
audience: [framework-authors, module-authors]
since: "1.0"
normative-level: normative
related:
  - "[Edges](../04-domain/edges.md)"
  - "[Entity Repository](../05-persistence/entity-repository.md)"
  - "[Performance Tuning](../14-operations/performance-tuning.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# ADR-020: No Lazy Loading for Edges

**Status: Accepted**

---

## Context

Entities have edges (relationships) to other entities — an Invoice has many Lines, a Customer has one primary Contact, a StockMove has a Product. When loading an entity, the system must decide when to load related entities.

Two common approaches:
1. **Lazy loading**: related entities are loaded on first access, transparently. ORMs like GORM and Hibernate support this via proxy objects.
2. **Eager loading**: related entities are loaded only when explicitly requested via query options.

---

## Decision

Awo **does not support lazy loading**. All edge loading is explicit. Module authors declare which edges they need via `QueryOption`s at the call site.

```go
// CORRECT — explicit edge loading
invoice, err := repo.Get(ctx, invoiceID,
    def.WithEdge("lines"),
    def.WithEdge("customer"),
)

// WRONG — invoice.Lines will be empty unless explicitly loaded
invoice, err := repo.Get(ctx, invoiceID)
// invoice.Edges["lines"] == nil
```

---

## Consequences

### Positive

**No N+1 queries**: without lazy loading, it is structurally impossible to accidentally issue N+1 queries. The developer must think about data requirements at the call site. The framework loads all requested edges in a minimal number of queries (typically one per edge type).

**Predictable performance**: a function's database behavior is visible from its code. There are no hidden queries triggered by property access in templates or hooks deep in the call stack.

**Works without active database connection**: edge fields are simply nil/empty when not loaded. No proxy object requires a live connection to trigger a query — safe to pass entities across goroutine boundaries.

**Explicit contract**: the call site documents exactly what data is needed. Code reviewers and future maintainers can understand the data access pattern without tracing through ORM internals.

### Negative

**More verbose call sites**: developers must list required edges explicitly. Mitigated by the compiler catching uninitialized edge accesses (nil pointer) rather than silently returning empty data.

**No transparent access**: code like `invoice.Customer.Name` will panic if `customer` edge was not explicitly loaded. Developers must know what they need upfront. This is the intended behavior — the panic is a bug signal, not a feature.

---

## Why Lazy Loading Is Problematic in This Context

### Multi-tenant RLS

Lazy loading in ORMs typically loads related entities using the same connection. In Awo, `set_tenant_context()` must be called on the connection before any query. Lazy-loaded queries triggered from application code (hooks, page builders) may not have the correct tenant context set — leading to either empty results (RLS silently filters) or cross-tenant data leakage if `set_tenant_context` was called with a different tenant ID.

Explicit loading happens at the `EntityRepository` layer, which always ensures `set_tenant_context` is called correctly.

### Request-scoped connections via PgBouncer

PgBouncer in transaction mode means the application does not hold a persistent connection. Each `repo.Get` or `repo.Query` acquires a connection, executes, and returns it to the pool. Lazy loading relies on holding a connection open to lazily fetch related data later — incompatible with transaction mode pooling.

### Temporal activities

Temporal workflow activities are executed potentially minutes or hours after the initial workflow start. Passing entity objects with lazy-loadable proxies across activity boundaries is unsafe — the proxy would attempt to load data on a connection that no longer exists in the activity's context.

---

## Alternatives Considered

### Lazy loading with explicit session management

Require developers to pass a "session" object that manages the connection for lazy loading. Rejected:
- Increases API surface area
- Still susceptible to N+1 in loops
- Incompatible with PgBouncer transaction mode

### Automatic eager loading based on field access analysis

Static analysis at compile time to detect which edges are accessed and pre-load them. Rejected:
- Not feasible with Go's type system (no runtime reflection at compile time)
- Custom fields are dynamically registered — static analysis cannot know what edges are needed

---

## Related Documents

- [Edges](../04-domain/edges.md) — EdgeDef types and cascade rules
- [Entity Repository](../05-persistence/entity-repository.md) — `WithEdge` QueryOption specification
- [Performance Tuning](../14-operations/performance-tuning.md) — N+1 detection and batch loading
