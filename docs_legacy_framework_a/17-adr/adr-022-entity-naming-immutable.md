> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "ADR-022: Entity Names Are Immutable After First Migration"
id: adr-022
status: accepted
category: ADR
stability: STABLE
audience: [framework-authors, module-authors]
since: "1.0"
normative-level: normative
related:
  - "[System Entities Catalog](../05-persistence/system-entities-catalog.md)"
  - "[Migrations](../14-operations/migrations.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# ADR-022: Entity Names Are Immutable After First Migration

**Status: Accepted**

---

## Context

The `EntityDefinition.Name` field is the canonical identifier for an entity type. It is used in:

- PostgreSQL table name (`finance_invoice` → table `finance_invoice`)
- PostgreSQL RLS policy names (`tenant_isolation ON finance_invoice`)
- PostgreSQL index names (`idx_finance_invoice_tenant_id`)
- Casbin policy objects (`p role:finance.accounts_payable, {tenant_id}, finance_invoice, create`)
- Redis cache keys (`page:finance_invoice:v3:{tenant_id}`)
- Temporal workflow IDs (`{tenant-uuid}.finance_invoice.{record-id}.on_submit`)
- API URL paths (`/api/v1/entities/finance_invoice`)
- amis API paths in page schemas (`"GET /api/v1/entities/finance_invoice"`)
- Migration filenames (`20241201_create_finance_invoice.up.sql`)
- Audit log `entity_type` field (stored in all audit records)

---

## Decision

Once a migration file references an entity name, that entity name is **immutable**. It MUST NOT be changed after the migration is applied to any environment (development, staging, or production).

---

## Consequences

### Positive

**Stable identifiers**: Workflow IDs stored in Temporal history for running workflows are valid indefinitely. Audit log entries remain coherent. Redis cache keys do not require migration.

**No rename migration complexity**: Table renames require complex migrations (new table, data copy, FK updates, RLS policy recreation, index recreation). By prohibiting renames, this complexity never arises.

### Negative

**Naming mistakes are permanent**: If `finance_inovice` is deployed to production before the typo is caught, the correct name is `finance_inovice` forever (or until the entity is retired and a new entity `finance_invoice` is introduced).

**Domain language evolution**: If the business domain renames a concept (e.g., "Invoice" → "Sales Order"), the entity name does not change. The `Label` and `LabelPlural` fields update the UI display name; the entity `Name` remains for all internal purposes.

---

## How to Rename in Practice (If Absolutely Necessary)

Entity names cannot be renamed, but an entity can be **replaced**:

1. Create a new entity definition with the correct name and an identical (or improved) schema
2. Write a migration that creates the new table and copies data
3. Add a migration that adds a NOT NULL FK from old table to new table (for data integrity during dual-write period)
4. Update all code to write to the new entity; update reads to query from new entity
5. Verify all Temporal workflows complete naturally or are terminated (no new workflows against old entity)
6. After sufficient bake time: deprecate old entity (read-only PermissionSet), archive data, drop old table in a final migration

This process takes weeks. The cost reinforces the importance of choosing names carefully.

---

## Naming Conventions That Prevent Mistakes

Follow `{module}_{noun}` strictly:

```
finance_invoice         ✓  (not "invoice", not "financeinvoice")
inventory_stock_move    ✓  (not "stockmove", not "stock_moves")
hr_employee             ✓  (not "employees", not "hremployee")
crm_customer            ✓  (not "customer", not "customers")
```

Rules:
- Snake_case only
- Module prefix always first
- Singular noun (not plural)
- No abbreviations that aren't universally understood
- Review the name with at least one other developer before the first migration

---

## Related Documents

- [System Entities Catalog](../05-persistence/system-entities-catalog.md) — all current entity names
- [Architecture Invariants](../02-architecture/invariants.md) — INV-004 (entity names immutable)
- [Migrations](../14-operations/migrations.md) — migration file naming which embeds entity names
