> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Custom Entities"
id: pers-004
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[System Entities](system-entities.md)"
  - "[EntityRepository](entity-repository.md)"
  - "[Fields](../04-domain/fields.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Custom Entities

**PERS-004 | Status: Accepted | Stability: Stable**

Custom entities store records as JSONB documents. This document specifies the custom entity storage model, JSONB schema, custom field behavior, and escalation criteria.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Storage Model

Custom entity records are stored in a single framework-managed table: `custom_entity_records`.

```sql
CREATE TABLE custom_entity_records (
    id           uuid        NOT NULL DEFAULT gen_random_uuid(),
    tenant_id    uuid        NOT NULL REFERENCES tenants(id),
    entity_type  varchar(128) NOT NULL,  -- entity name: "site_visit", "survey_response"
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    created_by   uuid,
    data         jsonb       NOT NULL DEFAULT '{}',

    PRIMARY KEY (id)
);

ALTER TABLE custom_entity_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE custom_entity_records FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON custom_entity_records
    USING (tenant_id = current_tenant_id());

CREATE INDEX idx_custom_entity_records_tenant_type
    ON custom_entity_records(tenant_id, entity_type);
CREATE INDEX idx_custom_entity_records_data
    ON custom_entity_records USING GIN(data);
```

All declared fields from the `EntityDefinition.Fields` array are stored as keys in the `data` JSONB column. System columns (`id`, `tenant_id`, `created_at`, `updated_at`, `created_by`) are typed columns, not JSONB.

---

## 2. JSONB Storage Behavior

For a `FieldDef{Name: "territory", Type: entity.Data}` on a custom entity:

- **Stored as:** `data->>'territory'`
- **Filtered as:** `data->>'territory' = $1`
- **Indexed as:** GIN index on `data` column (covers all keys)
- **Range filtered as:** `(data->>'total_amount')::numeric >= $1` (requires explicit cast)

### Type Coercion

JSONB stores all values as JSON. The store layer coerces values on read:
- Numeric fields: parsed from JSON number to Go type
- Boolean fields: JSON `true`/`false` to Go `bool`
- DateTime fields: JSON string to `time.Time` (UTC)
- Currency fields: JSON string (decimal notation) to `decimal.Decimal`

Type coercion errors produce a `BusinessError` with code `"type_coercion_error"` and the affected field name.

---

## 3. Custom Fields on Custom Entities

Custom entities support [Custom Fields](../GLOSSARY.md#custom-field) — tenant-defined fields added at runtime via the Metadata module. These are stored as additional keys in the `data` JSONB document alongside declared fields.

There is no structural distinction between declared fields and custom fields at the storage layer. Both are JSONB keys. The distinction exists only at the schema layer: declared fields are in the `CompiledSchema`, custom fields are in the Metadata module's runtime registry.

---

## 4. Filter DSL on JSONB Fields

All Filter DSL operators work on custom entity fields. The store layer generates appropriate JSONB path expressions:

| DSL | Generated SQL |
|---|---|
| `filter.Eq("territory", "Nairobi")` | `data->>'territory' = 'Nairobi'` |
| `filter.Gt("score", 80)` | `(data->>'score')::bigint > 80` |
| `filter.Contains("notes", "urgent")` | `data->>'notes' ILIKE '%urgent%'` |
| `filter.IsNull("approval_date")` | `data->>'approval_date' IS NULL` |

Range operators on JSONB fields require an explicit type cast in the generated SQL. The store layer infers the cast from the field's declared `FieldType`. Unknown types default to text comparison.

---

## 5. Constraints on Custom Entities

Custom entities cannot have:
- Database-level `UNIQUE` constraints on individual fields (JSONB keys cannot have unique constraints)
- Database-level `NOT NULL` constraints on individual fields (only `data` as a whole is NOT NULL)
- `CHECK` constraints on individual fields
- FK constraints from custom entity fields to other tables

These constraints are enforced at the application layer by field validators and `before_create`/`before_save` hooks, not at the database layer.

**This is an important limitation:** if the process crashes between validation and persistence, invalid data can reach the database. For entities requiring database-level constraint enforcement (financial data, inventory quantities), use [System Entities](system-entities.md).

---

## 6. Escalation Criteria

Escalate from Custom Entity to System Entity when:

| Criterion | Threshold |
|---|---|
| Record volume | > 10 million records |
| Write rate | > hundreds per second |
| Field participation in financial calculations | Any field used in monetary arithmetic |
| FK constraints to system entity PKs required | Any FK constraint needed |
| Unique constraint required at DB level | Any uniqueness guarantee needed at DB level |
| JSONB corruption risk unacceptable | IAM data, regulatory records |

Escalation requires:
1. Writing a new migration that creates a typed SQL table
2. A data migration that transfers JSONB data to typed columns
3. Changing `StorageModel: entity.SystemEntity` in the EntityDefinition
4. Updating the store layer routing
5. A down-migration that returns data to JSONB

Escalation migrations must be planned carefully and tested against production data volumes before deployment.

---

## 7. Good Candidates for Custom Entities

Custom entities are appropriate for:
- Site visit records and inspection checklists
- Survey responses and questionnaire answers
- Approval metadata and workflow step annotations
- Industry-specific classification fields
- Tenant-specific configuration objects
- Integration event logs (where volume is low)
- Draft/temporary records that may be discarded

---

## 8. No-Migration Schema Evolution

Because all fields are JSONB keys, adding a new field to a custom entity requires only an `EntityDefinition` change — no migration. Existing records simply lack the new key; the framework treats missing keys as NULL.

Removing a field from a custom entity:
- Remove from `EntityDefinition.Fields`
- Existing records retain the key in `data` (orphaned)
- A cleanup migration may remove the orphaned key: `UPDATE custom_entity_records SET data = data - 'old_field' WHERE entity_type = 'my_entity'`
- No schema migration required; this is a data migration

Renaming a field on a custom entity:
- Add the new field name to `EntityDefinition.Fields`
- Write a data migration: `UPDATE custom_entity_records SET data = jsonb_set(data, '{new_name}', data->'old_name') - 'old_name' WHERE entity_type = 'my_entity'`
- Remove the old field from `EntityDefinition.Fields` in a subsequent deployment

---

## Related Documents

- [System Entities](system-entities.md) — the alternative storage model for high-integrity data
- [EntityRepository](entity-repository.md) — the interface used to access custom entities
- [Filter DSL](filter-dsl.md) — querying JSONB fields
- [Fields](../04-domain/fields.md) — FieldType declarations for custom entities
- [Metadata Module](../10-modules/platform-modules.md) — runtime custom field management
- [Glossary](../GLOSSARY.md) — Custom Entity, System Entity, Custom Field, JSONB Storage
