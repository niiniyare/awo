> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Module Author Guide

**Classification:** Guide — Tier 2
**Owner:** `99-modules/MODULE_AUTHOR_GUIDE.md`
**Status:** Living document

---

## Purpose

This guide explains how to build a new business module for the Awo Framework — directory structure, entity definition patterns, registration, migrations, and the checklist before merge.

---

## 1. What Is a Module

A module is a Go package under `internal/core/{module_name}/` that defines a set of related entities using `def.EntityDefinition`. Each module's entities use a shared `module` name prefix (e.g., `finance`, `inventory`, `hrm`).

Modules are self-contained: a module declares its own entities, hooks, policies, actions, and migrations. The framework registry (`def.Register`) discovers them at startup via `init()`.

---

## 2. Directory Structure

```
internal/
  core/
    {module}/
      {module}.go         ← package doc + module-level constants
      def.go       ← EntityDefinition variable declarations
      hooks.go            ← BeforeCreate/AfterCreate/etc. implementations
      policy.go           ← PolicyFunc implementations
      actions.go          ← ActionHandlerFunc implementations
      workflows.go        ← WorkflowTrigger InputBuilders
      service.go          ← optional thin service helpers
      {module}_test.go    ← unit tests
      integration_test.go ← integration tests (build tag: integration)
      migrations/
          {timestamp}_create_{entity}.up.sql
          {timestamp}_create_{entity}.down.sql
```

Do not create `repository/`, `domain/`, `handler/`, or `service/` subdirectories — those are Framework A concepts. All persistence goes through `ActionEntityRepo` or `EntityRepository`.

---

## 3. Registering an Entity

```go
// internal/core/inventory/inventory.go
package inventory

import "awo.so/awo/def"

func init() {
    def.Register(&StockItemDefinition)
    def.Register(&StockMoveDefinition)
    def.Register(&WarehouseDefinition)
}
```

`def.Register` MUST be called in `init()`. It MUST NOT be called from request handlers or goroutines — this races with concurrent reads during startup.

Import the module package from `cmd/server/main.go` (blank import):

```go
import (
    _ "awo.so/internal/core/inventory"   // register inventory module
)
```

---

## 4. Minimal Entity Definition

```go
// def.go
var StockItemDefinition = def.SystemDefinition{
    Name:        "stock_item",
    Module:      "inventory",
    Label:       "Stock Item",
    LabelPlural: "Stock Items",
    Fields: []def.FieldDef{
        {Name: "sku",         Type: def.FieldTypeData,   Required: true, Searchable: true},
        {Name: "name",        Type: def.FieldTypeData,   Required: true, Searchable: true},
        {Name: "unit",        Type: def.FieldTypeSelect,
            Options: []string{"each", "kg", "litre", "metre"}},
        {Name: "reorder_qty", Type: def.FieldTypeFloat},
        {Name: "cost_price",  Type: def.FieldTypeCurrency},
    },
    Permissions: def.PermissionSet{
        Create: []string{"inventory.stock_item.create"},
        Read:   []string{"inventory.stock_item.read"},
        Update: []string{"inventory.stock_item.update"},
        Delete: []string{"inventory.stock_item.delete"},
    },
    AuditEnabled: true,
}
```

---

## 5. Entity Naming Rules

Format: `{module}_{noun}` — all lowercase, snake_case.

- `inventory_stock_item` ✓
- `inventoryStockItem` ✗ (no camelCase)
- `inv_stock` ✗ (no abbreviations for module name)
- `stock_item` ✗ (missing module prefix)

**Never rename entity names** — they appear in migration filenames, Temporal workflow IDs, Redis cache keys, and audit records. Renaming breaks all of these permanently.

---

## 6. System vs Custom Entity Decision

Use `SystemDefinition` (typed SQL columns) when any of:
- Participates in financial calculations (JournalEntry, LedgerEntry, etc.)
- Requires FK constraints to other system entity PKs
- Write rate > hundreds/sec
- Fields need SQL-level constraints (CHECK, UNIQUE, FK)

Use `CustomDefinition` (JSONB storage) when all of:
- Tenant-specific, schema evolves frequently
- No financial/inventory/IAM participation
- Write rate < hundreds/sec
- No FK constraints needed

---

## 7. Migrations

Every `SystemDefinition` requires a migration. Use the template in [`15-migrations/RLS_TABLE_TEMPLATE.md`](../15-migrations/RLS_TABLE_TEMPLATE.md).

File naming:
```
{14-digit-timestamp}_{description}.up.sql
{14-digit-timestamp}_{description}.down.sql
```

Location: module's `migrations/` directory, or global `db/migration/` for framework-level tables.

---

## 8. Hook Pattern

```go
// hooks.go

// StockMoveValidator validates stock move requests before create.
type StockMoveValidator struct{}

func (v *StockMoveValidator) BeforeCreate(
    ctx context.Context,
    rec *def.EntityRecord,
) error {
    qty := rec.GetFloat("quantity")
    if qty <= 0 {
        return &def.ValidationError{
            Fields: map[string]string{
                "quantity": "Quantity must be greater than zero.",
            },
        }
    }
    return nil
}
```

Register on the definition:

```go
Hooks: def.HookSet{
    BeforeCreate: []def.BeforeCreateHook{&StockMoveValidator{}},
},
```

---

## 9. Policy Pattern

```go
// policy.go

// StockItemWarehousePolicy restricts stock items to the viewer's assigned warehouse.
// Managers (identified by the absence of a warehouse_id on their Actor) see all items.
func StockItemWarehousePolicy(ctx context.Context) def.Filter {
    viewer := auth.ViewerFromContext(ctx)
    warehouseID := viewer.Actor().WarehouseID  // set by IAM provisioning for warehouse staff
    if warehouseID == uuid.Nil {
        return nil // no warehouse restriction — manager or platform actor
    }
    return filter.Eq("warehouse_id", warehouseID)
}
```

---

## 10. Action Pattern

```go
// actions.go

func AdjustStockAction(ctx context.Context, action def.ActionContext) (*def.ActionResult, error) {
    // load, validate, update, publish — see ACTION_HANDLER_GUIDE.md
    return &def.ActionResult{Message: "Stock adjusted."}, nil
}
```

---

## 11. Module Author Checklist

Before merging a new module:

- [ ] Entity names follow `{module}_{noun}` format
- [ ] All `SystemDefinition` entities have migration files (up + down)
- [ ] Migrations include RLS setup (ENABLE, FORCE, CREATE POLICY)
- [ ] `def.Register()` called in `init()` only
- [ ] Module package imported (blank import) in `cmd/server/main.go`
- [ ] All `FieldTypeCurrency` fields use `numeric(20,4)` in migration
- [ ] `AuditEnabled: true` on all financial/IAM entities
- [ ] Unit tests for every hook and policy
- [ ] Integration tests for entity CRUD against real PostgreSQL
- [ ] Module permission identifiers declared in `PermissionSet` use `{module}.{entity}.{action}` format — no `role:` strings
- [ ] Role-to-permission mappings for module added to [`03-auth/RBAC_ROLES_REFERENCE.md`](../03-auth/RBAC_ROLES_REFERENCE.md)
- [ ] Domain events added to [`09-events/DOMAIN_EVENTS_REFERENCE.md`](../09-events/DOMAIN_EVENTS_REFERENCE.md)

---

## References

- [`01-entity/ENTITY_DEFINITION_SPEC.md`](../01-entity/ENTITY_DEFINITION_SPEC.md) — EntityDefinition interface
- [`02-pipeline/HOOK_CONTRACT.md`](../02-pipeline/HOOK_CONTRACT.md) — Hook interfaces
- [`13-actions/ACTION_HANDLER_GUIDE.md`](../13-actions/ACTION_HANDLER_GUIDE.md) — Action handler patterns
- [`99-modules/FINANCE_MODULE_SPEC.md`](FINANCE_MODULE_SPEC.md) — Finance as canonical example
