> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "ADR-018: UUID v7 for Primary Keys"
id: adr-018
status: accepted
category: ADR
stability: STABLE
audience: [framework-authors, module-authors]
since: "1.0"
normative-level: normative
related:
  - "[System Entities Catalog](../05-persistence/system-entities-catalog.md)"
  - "[Fields](../04-domain/fields.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# ADR-018: UUID v7 for Primary Keys

**Status: Accepted**

---

## Context

Awo needs a primary key strategy for all entity tables. Requirements:

- **Globally unique** without coordination — multiple application instances must be able to generate IDs concurrently without collisions
- **Non-guessable** — IDs appear in URLs and API responses; sequential integer IDs expose entity counts and enable enumeration attacks
- **Index-friendly** — random UUIDs (v4) cause B-tree index fragmentation, leading to poor insert performance at scale
- **Sortable by creation time** — helpful for pagination, debugging, and time-ordered queries without a separate `created_at` index on the hot path

---

## Decision

Use **UUID v7** as the primary key type for all entity tables.

UUID v7 encodes a millisecond-precision Unix timestamp in the high bits (bits 0–47), followed by random bits. This makes v7 UUIDs:
- Time-ordered (roughly) — inserts into B-tree indexes are sequential, not random
- Globally unique — the random component prevents collisions even within the same millisecond
- Non-guessable — the random suffix prevents enumeration

---

## Consequences

### Positive

**B-tree friendliness**: because new UUIDs sort after all existing ones (within the same millisecond), page splits in the primary key B-tree index are minimized. INSERT performance degrades much more slowly than with v4.

**Approximate ordering**: `ORDER BY id` is a reasonable proxy for `ORDER BY created_at` for recent data. Not exact (millisecond granularity, not nanosecond), but useful for debugging.

**No sequence table**: no need for `CREATE SEQUENCE` — IDs are generated in application code. Reduces DB round-trips on batch inserts.

**Standard UUID wire format**: `xxxxxxxx-xxxx-7xxx-xxxx-xxxxxxxxxxxx` — compatible with all UUID-aware clients and frameworks.

### Negative

**Not human-readable**: UUIDs are harder to type and communicate verbally than sequential integers. Mitigated by NamingSeries (human-readable reference numbers like `INV-2024-00001`) on entities that need user-facing IDs.

**Millisecond clustering, not exact ordering**: two records inserted within the same millisecond may have non-deterministic ordering by ID. Always use `created_at` column for strict time ordering.

**16 bytes vs 8 bytes**: UUID PKs are twice the size of `bigint` PKs. At scale this increases index size. Acceptable trade-off for the global uniqueness and non-guessability properties.

---

## Alternatives Considered

### Sequential `bigint` (SERIAL / IDENTITY)

Simple and storage-efficient. Rejected:
- Exposes entity counts and enables enumeration attacks (`/api/v1/invoices/1`, `/api/v1/invoices/2`)
- Requires coordination across instances (single sequence) — coupling

### UUID v4 (random)

Globally unique and non-guessable. Rejected:
- Completely random 128 bits causes maximum B-tree fragmentation — every insert goes to a random page, causing frequent page splits
- At >10M rows per table, v4 UUID insert performance degrades significantly
- No time-ordering property

### ULID

Similar time-ordered properties to UUID v7. Rejected:
- Not a standard UUID format — requires custom encoding/decoding
- Less library support than UUID v7 in Go ecosystem (google/uuid added v7 in v1.6.0)
- UUID v7 achieves the same properties in a standard format

### NanoID / custom short IDs

Short, URL-friendly. Rejected:
- More collision probability at scale
- Non-standard — no built-in support in pgx, amis, or Temporal

---

## Implementation

```go
import "github.com/google/uuid"

// In framework entity creation
func newEntityID() uuid.UUID {
    id, err := uuid.NewV7()
    if err != nil {
        // Extremely unlikely — falls back to v4 on clock error
        return uuid.New()
    }
    return id
}
```

PostgreSQL column definition:

```sql
id uuid PRIMARY KEY DEFAULT gen_random_uuid()
-- Note: gen_random_uuid() generates v4; application always provides the v7 ID.
-- DEFAULT is a safety net only — should never be triggered in normal operation.
```

---

## Related Documents

- [System Entities Catalog](../05-persistence/system-entities-catalog.md) — all entities using UUID v7 PKs
- [NamingSeries](../04-domain/naming-series.md) — human-readable IDs alongside UUID PKs
- [Cursor-Based Pagination](../05-persistence/pagination.md) — cursor uses UUID v7's time-ordering property
