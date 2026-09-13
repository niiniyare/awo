> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "ADR-009: UUID v7 for Primary Keys"
id: adr-009
status: accepted
category: ADR
stability: FROZEN
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: informative
related:
  - "[System Entities](../05-persistence/system-entities.md)"
  - "[EntityDefinition](../03-kernel/entity-def.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
---

# ADR-009: UUID v7 for Primary Keys

**Status:** Accepted
**Date:** 2024-02-15
**Authors:** Framework Team

---

## Context

Awo generates primary keys for all entity records. The choice of primary key type has performance, ordering, and operational implications for a multi-tenant system with potentially millions of records per tenant.

---

## Options Considered

### Option A: Auto-Increment Integer (SERIAL / BIGSERIAL)

Pros: Simple, compact (8 bytes), clustered index performance.

Cons:
- **Leaks record count**: `/api/v1/entities/finance_invoice/1` vs `/api/v1/entities/finance_invoice/1000` reveals business volume
- **Multi-tenant collision risk**: integer sequences in a shared table require per-tenant sequences to prevent ID conflicts
- **No distributed generation**: cannot generate IDs outside the database without coordination

### Option B: UUID v4 (Random)

Pros: Globally unique, no business information leaked, can be generated client-side.

Cons:
- **Index fragmentation**: random insertion order causes page splits in B-tree indexes
- **No natural ordering**: cannot sort by ID to approximate creation order
- **Cache inefficiency**: random access pattern defeats buffer pool locality

### Option C: ULID (Universally Unique Lexicographically Sortable Identifier)

Pros: Time-ordered, globally unique, lexicographically sortable.

Cons:
- Not a standard PostgreSQL type — requires custom extension or text column
- Ecosystem support weaker than UUID

### Option D: UUID v7 (Time-Ordered)

UUID v7 encodes a Unix timestamp in the high 48 bits, followed by random bits. This produces IDs that are:
- Globally unique (random component)
- Time-ordered (timestamp component)
- Standard UUID format (compatible with PostgreSQL `uuid` type)
- Monotonically increasing within a millisecond window

Pros:
- Compatible with PostgreSQL `uuid` type natively
- Clustered index performance (sequential insertion = no page splits)
- Natural creation-time ordering without a separate `created_at` index
- No business information leaked (not sequential integers)

Cons:
- Not yet supported natively in PostgreSQL `gen_random_uuid()` (generates v4) — requires application-level generation
- Go ecosystem support: `github.com/google/uuid` v1.6+ supports v7

---

## Decision

**Use UUID v7 for all entity primary keys.**

UUID v7 resolves the core tension between UUID v4 (index fragmentation) and sequential integers (information leakage). The time-ordered property provides B-tree index locality without revealing sequential record counts.

The framework generates UUID v7 values in the application layer (Go) before inserting records, not via `DEFAULT gen_random_uuid()` in PostgreSQL (which generates v4).

---

## Consequences

**Positive:**
- Index fragmentation eliminated — insertions are approximately sequential
- Natural creation-time ordering via `ORDER BY id` (approximately)
- No business information leaked via ID patterns
- Compatible with PostgreSQL `uuid` type — no schema changes needed

**Negative:**
- Framework must generate IDs in Go (not `DEFAULT gen_random_uuid()`)
- IDs encode approximate creation time — if this is a privacy concern, use UUID v4
- Two IDs generated within the same millisecond may not be strictly ordered

**Implementation note:**
```go
import "github.com/google/uuid"

id, err := uuid.NewV7()  // generates UUID v7
```

---

## Revisit Trigger

If PostgreSQL adds native UUID v7 generation (via `gen_random_uuid_v7()` or similar), migrate default generation to the database level. The application-level generation can remain as a fallback.
