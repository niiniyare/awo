> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Persistence — Section Overview"
id: pers-000-readme
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: informative
related:
  - "[EntityRepository](entity-repository.md)"
  - "[Filter DSL](filter-dsl.md)"
  - "[Five-Layer Architecture](../02-architecture/five-layer.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Persistence

**Section 05 | Persistence Layer**

The persistence section specifies the interfaces and DSLs through which all entity data is stored and queried. The Store Layer (which implements these interfaces) is the only layer that communicates directly with PostgreSQL.

---

## Contents

| Document | ID | Purpose | Stability |
|---|---|---|---|
| [EntityRepository](entity-repository.md) | PERS-001 | Full interface specification: all methods, options, transaction semantics | FROZEN |
| [Filter DSL](filter-dsl.md) | PERS-002 | Declarative predicate DSL: operators, composition, wire format | FROZEN |
| [System Entities](system-entities.md) | PERS-003 | SQL-backed entities: schema conventions, mandatory types, migration requirements | FROZEN |
| [Custom Entities](custom-entities.md) | PERS-004 | JSONB-backed entities: storage model, custom fields, escalation criteria | STABLE |
| [Cursors and Pagination](cursors-and-pagination.md) | PERS-005 | Page-number vs cursor (keyset) pagination, PageInfo, API wire format | STABLE |
| [Aggregations](aggregations.md) | PERS-006 | Count, Sum, Avg, GroupBy, time-series aggregations | STABLE |
| [Transactions](transactions.md) | PERS-007 | WithTx, multi-repo transactions, hooks inside TX, savepoints | STABLE |
| [System Entity Catalog](system-entities-catalog.md) | PERS-008 | Complete list of all system entities and their classification rationale | STABLE |
| [Cursor-Based Pagination](cursor-pagination.md) | PERS-009 | Keyset pagination: cursor format, query generation, sort index requirements | STABLE |
| [RLS Deep Dive](rls-deep-dive.md) | PERS-010 | Tenant context flow, policy evaluation, coverage verification, failure modes | STABLE |
| [Query Options](query-options.md) | PERS-011 | Complete QueryOption reference: pagination, sorting, edges, fields, locks | STABLE |
| [Bulk Repository Operations](bulk-operations.md) | PERS-012 | BulkCreate, BulkUpdate: atomicity, hooks, performance, import pattern | STABLE |
| [filter Package Reference](filter-package-reference.md) | PERS-013 | Complete API reference: Kind constants, constructor functions, SQL generation | FROZEN |
| [SQLC Integration](sqlc-integration.md) | PERS-014 | SQLC layering, query file organization, Querier interface, module author rules | STABLE |

---

## Prerequisites

- [EntityDefinition](../03-kernel/entity-def.md)
- [Five-Layer Architecture §6](../02-architecture/five-layer.md#6-store-layer) — Store Layer responsibilities
- [Architecture Laws](../02-architecture/laws.md) — LAW-005 (tenant context), LAW-014 (filter versioning)
- [Tenancy Model](../06-tenancy/tenant-model.md) — RLS, set_tenant_context()
- [Glossary](../GLOSSARY.md) — EntityRepository, Filter, QueryOption, System Entity, Custom Entity
