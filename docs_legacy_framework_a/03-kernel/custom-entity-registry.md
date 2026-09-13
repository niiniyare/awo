> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Custom Entity Registry"
id: kern-005
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Entity Registry](registry.md)"
  - "[Tenant Lifecycle](../06-tenancy/tenant-lifecycle.md)"
  - "[Module System](../10-modules/module-system.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Custom Entity Registry

**KERN-005 | Status: Accepted | Stability: Stable**

This document specifies the custom entity registry: how tenants register custom entity schemas at runtime, the concurrency constraints, and the lifecycle of custom schemas.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Two Registries

Awo has two distinct registries:

| Registry | Populated when | Contents | Immutable after |
|---|---|---|---|
| **Entity Registry** | Startup (Initialization Phase) | System + Custom entity type definitions from `def.Register()` | Registry seal at startup |
| **Custom Entity Registry** | Runtime, per tenant | Tenant-specific custom entity schemas | Not immutable — per-tenant mutations possible |

The Entity Registry governs the framework's type system. The Custom Entity Registry governs per-tenant runtime schema extensions (custom field additions, custom entity definitions added by tenants via admin UI).

---

## 2. Custom Entity Schema

Tenants can define custom entities via the Metadata module. A custom entity schema is stored in `custom_entity_definitions`:

```go
type CustomEntitySchema struct {
    ID          uuid.UUID
    TenantID    uuid.UUID
    EntityKey   string          // stable, used in URLs and API — never rename
    Label       string
    LabelPlural string
    Fields      []CustomFieldDef
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

Custom entity schemas are compiled into a runtime `EntityDefinition` equivalent per tenant. The compilation is cached in Redis (key: `custom_schema:{tenant_id}:{entity_key}:{hash}`).

---

## 3. RegisterCustomForTenant — Concurrency Rule

The critical rule (referenced in CLAUDE.md and LAW-012):

> **`registry.RegisterCustomForTenant` MUST NOT be called from a request handler.**

The custom entity registry is a shared data structure read by every request. Concurrent reads are safe. Concurrent writes race with concurrent reads.

The only safe call sites for `RegisterCustomForTenant` are:
- At startup, for schemas seeded during provisioning
- From a dedicated schema-mutation goroutine that holds a write lock
- From an admin action that acquires the schema mutation lock before modifying

```go
// WRONG: called from request handler
func CreateCustomEntityHandler(c *fiber.Ctx) error {
    schema := parseSchema(c)
    registry.RegisterCustomForTenant(tenantID, schema)  // RACE — concurrent reads in flight
    return c.SendStatus(200)
}

// CORRECT: schema changes go through the schema mutation queue
func CreateCustomEntityHandler(c *fiber.Ctx) error {
    schema := parseSchema(c)
    // Enqueue; applied asynchronously with proper locking
    err := h.SchemaMutationQueue.Enqueue(c.Context(), tenantID, schema)
    if err != nil {
        return mapError(c, err)
    }
    return c.Status(202).JSON(SuccessEnvelope{Data: fiber.Map{"message": "Schema update queued."}})
}
```

---

## 4. Schema Mutation Lock

The framework provides a schema mutation lock using Redis for distributed coordination:

```go
// internal/platform/metadata/registry.go

func (r *CustomEntityRegistry) ApplySchemaChange(ctx context.Context, tenantID uuid.UUID, schema CustomEntitySchema) error {
    // Acquire distributed lock — only one schema change at a time per tenant
    lock, err := r.Redis.SetNX(ctx,
        fmt.Sprintf("schema_lock:%s", tenantID),
        "locked",
        30*time.Second,
    )
    if err != nil { return err }
    if !lock {
        return &errors.BusinessError{
            Code:    "schema.mutation_in_progress",
            Message: "A schema change is already in progress for this tenant. Please retry.",
            Status:  409,
        }
    }
    defer r.Redis.Del(ctx, fmt.Sprintf("schema_lock:%s", tenantID))

    // Acquire read-write lock on the in-memory registry
    r.mu.Lock()
    defer r.mu.Unlock()

    // Apply the change
    r.schemas[tenantID][schema.EntityKey] = schema

    // Invalidate Redis schema cache for this tenant
    r.Redis.Del(ctx, fmt.Sprintf("custom_schema:%s:*", tenantID))

    return nil
}
```

---

## 5. Custom Schema Caching

Compiled custom schemas are cached per tenant in Redis:

```
Redis key: custom_schema:{tenant_id}:{entity_key}:{schema_hash}
TTL:       15 minutes
```

On cache miss, the schema is loaded from the database and compiled. Compilation validates field types, checks for conflicting field names with system fields, and generates the SQL column expressions for JSONB queries.

Cache invalidation occurs on:
- Schema mutation (explicit DEL after applying change)
- Tenant deactivation (all tenant cache keys purged)

---

## 6. Custom Entity Limitations

Custom entities stored in JSONB have constraints compared to system entities:

| Capability | System Entity | Custom Entity |
|---|---|---|
| FK constraints to other tables | Yes | No (link fields stored as UUID string) |
| DB-level CHECK constraints | Yes | No (validated at app layer only) |
| Triggers and stored procedures | Yes | No |
| Full-text search (tsvector) | Yes (manual) | GIN trgm only |
| Aggregate performance | O(n) with indexes | O(n) — JSONB extract overhead |
| Maximum fields | Unlimited | 200 (configurable per plan) |
| Financial amounts | Yes (numeric column) | Stored as text in JSONB — validate as decimal, do not use for financial calculations |

---

## 7. Escalation from Custom to System Entity

When a custom entity needs capabilities that JSONB cannot provide (financial calculations, FK constraints, >10M records), it must be escalated to a system entity:

1. Write a migration creating the proper SQL table
2. Backfill data from the JSONB custom entity to the new SQL table
3. Register a `SystemDefinition` for the entity
4. Deprecate and archive the custom entity schema
5. Update all module code to use the new system entity

Entity names MUST NOT change during escalation — the name is embedded in URLs, Temporal workflow IDs, and audit log entries.

---

## Related Documents

- [Entity Registry](registry.md) — startup-time registry for system entities
- [Compilation Pipeline](compilation-pipeline.md) — how EntityDefinitions become CompiledSchemas
- [Custom Entities](../05-persistence/custom-entities.md) — JSONB storage model
- [Architecture Laws](../02-architecture/laws.md) — LAW-012 (no RegisterCustomForTenant from handlers)
