> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Page Schema"
id: sdui-001
status: accepted
category: SPEC
stability: FROZEN
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Page Builders](page-builders.md)"
  - "[amis Integration](amis-integration.md)"
  - "[EntityDefinition](../03-kernel/entity-def.md)"
  - "[RBAC](../07-iam/rbac.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Page Schema

**SDUI-001 | Status: Accepted | Stability: Frozen**

This document specifies how page schemas are generated, cached, served, and permission-gated.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Overview

Every [EntityDefinition](../GLOSSARY.md#entitydefinition) produces four page schemas automatically:
- **List** — a searchable, paginated table of records
- **Create** — a form for creating a new record
- **Edit** — a form for updating an existing record
- **Detail** — a read-only view of a single record with all edges

Schemas are JSON documents conforming to the amis schema format. They are generated once (at compilation or on first request) and cached in Redis. The browser receives the schema and renders it using the pinned amis SDK — no custom JavaScript required.

---

## 2. Schema Generation

### Default Generation

The framework generates default schemas from the CompiledSchema. For each field in `EntityDefinition.Fields`, a corresponding amis form control or table column is generated. For each edge in `EntityDefinition.Edges`, an embedded sub-table or link field is generated.

The generator uses the FieldType to select the correct amis control:

| FieldType | List column | Form control |
|---|---|---|
| Data | Text column | Text input |
| SmallText | Text column | Text area (1 row) |
| LongText | Truncated text | Text area (multi-row) |
| Int | Number column | Number input |
| Float | Number column | Number input |
| Currency | Currency column | Currency input |
| Bool | Checkbox column | Switch |
| Date | Date column | Date picker |
| DateTime | DateTime column | DateTime picker |
| Time | Time column | Time picker |
| Select | Select column | Select dropdown |
| MultiSelect | Tag column | Multi-select |
| NamingSeries | Text column (read-only) | Read-only text |
| Link | Text (label of linked record) | Select with search |
| DynamicLink | Type + label | Dynamic select |

### Permission-Gated Schema Elements

Schema elements requiring permissions that the requesting actor lacks are **absent** from the schema — not disabled. This applies to:
- Create button: absent if actor lacks `create` permission
- Edit button: absent if actor lacks `write` permission
- Delete button: absent if actor lacks `delete` permission
- Action buttons: absent if actor lacks the action's declared permission
- Fields declared `Sensitive: true`: absent unless actor has explicit sensitive-read permission

This behavior is intentional: a disabled button leaks information about what operations exist. An absent element does not.

---

## 3. Schema Endpoints

```
GET /api/v1/schemas/{entity-name}/list
GET /api/v1/schemas/{entity-name}/create
GET /api/v1/schemas/{entity-name}/edit
GET /api/v1/schemas/{entity-name}/detail
```

These endpoints:
1. Require a valid session (standard session validation)
2. Resolve the actor's permissions for the entity
3. Check Redis cache: `page:{entity}:{version}:{tenant}:{actor_roles_hash}`
4. On cache miss: invoke the PageBuilderFunc (or use the default generator)
5. Store in Redis with 5-minute TTL
6. Return the schema as `application/json`

The cache key includes `actor_roles_hash` — a hash of the actor's role set — because different roles produce different schemas (due to permission-gated elements). This means schemas are cached per-role-combination, not per-user.

---

## 4. Cache Invalidation

The page schema cache is invalidated when:
- The tenant's feature flags change (a flag gate may add or remove elements)
- The tenant's permission assignments change (role grants/revocations)
- The framework is redeployed (content hash changes)

Cache invalidation is performed by deleting all keys matching `page:{entity}:*:{tenant}:*`. This is a Redis `SCAN` + `DEL` operation — not a blocking `KEYS` scan.

The 5-minute TTL provides eventual consistency: stale schemas expire automatically. Cache invalidation provides immediate consistency for critical changes (security-relevant permission changes should be invalidated immediately, not waited out).

---

## 5. Schema Versioning

The schema version embedded in the cache key is derived from the [CompiledSchema content hash](../GLOSSARY.md#content-hash-schema). When a new binary is deployed with a different schema hash, old cached schemas are automatically bypassed (different version key).

Old version keys are not explicitly deleted; they expire via TTL.

---

## 6. Schema Response Format

```json
{
  "type": "page",
  "title": "Invoices",
  "body": {
    "type": "crud",
    "api": "/api/v1/entities/finance_invoice",
    "columns": [...],
    "filter": {...},
    "toolbar": [...]
  }
}
```

The schema is a valid amis JSON document. All amis features may be used; the only constraint is that the schema must be generatable from Go code (no hardcoded JavaScript expressions).

---

## Related Documents

- [Page Builders](page-builders.md) — customizing generated schemas
- [amis Integration](amis-integration.md) — amis SDK and rendering
- [RBAC](../07-iam/rbac.md) — permission evaluation for schema element gating
- [Glossary](../GLOSSARY.md) — Page Schema, Page Builder, SDUI, amis
