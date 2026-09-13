> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "ADR-008: Entity Names Are Immutable After First Migration"
id: adr-008
status: accepted
category: ADR
stability: FROZEN
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: informative
related:
  - "[EntityDefinition](../03-kernel/entity-def.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
  - "[Invariants](../02-architecture/invariants.md)"
---

# ADR-008: Entity Names Are Immutable After First Migration

**Status:** Accepted
**Date:** 2024-02-10
**Authors:** Framework Team

---

## Context

Entity names in Awo are used as identifiers in many systems simultaneously:
- PostgreSQL table names (for system entities)
- `entity_type` column values in `custom_entity_records`
- API URLs: `/api/v1/entities/{entity-name}`
- Redis cache keys: `page:{entity-name}:{version}:{tenant}`
- Temporal workflow IDs: `{tenant}.{entity-name}.{record}.{event}.{fn}` — stored for months/years
- Casbin policy objects
- Migration filenames
- SDUI schema cache keys

If an entity name changes, all of these references become inconsistent.

---

## Decision

**Entity names are immutable after the first migration that introduces the entity.**

Rationale: Temporal workflow IDs containing the entity name are stored in Temporal's event history for the lifetime of the workflow — which can be months or years. Renaming an entity mid-flight would make historical workflow IDs refer to a name that no longer exists in the current EntityRegistry.

---

## What "First Migration" Means

- For system entities: the `.up.sql` migration that creates the table
- For custom entities: the first record written with `entity_type = '{entity-name}'`

Before either of these events, the entity name can still be changed (it only exists in code). After, it is immutable.

---

## Consequences

**Positive:**
- Consistency: all systems that store the entity name remain valid indefinitely
- Historical Temporal workflow IDs remain parseable
- No need for name-aliasing systems

**Negative:**
- Developer mistakes in entity naming must be caught in code review, before the first deploy
- If a name must change (rebranding, module restructure): a new entity is created, data is migrated, old entity is deprecated

**Architecture Laws generated:**
- LAW-011: Entity names are globally unique and immutable
- INV-012: Entity names stable after first migration

---

## How to Handle a Necessary Rename

If an entity genuinely needs renaming after first migration:

1. Create the new entity with the new name
2. Write a data migration to copy existing records to the new entity type
3. Switch all code to reference the new entity name
4. Deprecate the old entity (mark as deprecated in EntityDefinition, stop creating new records)
5. After all running workflows with old entity name have completed, archive the old entity
6. Remove the old entity definition in a subsequent release

This is deliberately cumbersome — it should deter premature renames and encourage careful naming upfront.

---

## Revisit Trigger

Not applicable. This is a correctness requirement, not a design preference. Any system that stores entity names externally (Temporal, audit logs, Redis) requires name stability.
