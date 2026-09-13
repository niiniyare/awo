> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Metadata Platform Module"
id: mod-012
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Platform Modules](platform-modules.md)"
  - "[Custom Entity Registry](../03-kernel/custom-entity-registry.md)"
  - "[Fields](../04-domain/fields.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Metadata Platform Module

**MOD-012 | Status: Accepted | Stability: Stable**

The Metadata module enables tenants to extend any entity with custom fields at runtime — no migration, no redeploy, no code change.

---

## 1. Purpose

Business modules ship with a fixed set of fields defined at compile time. But tenants always have domain-specific data that doesn't fit the standard schema: a logistics company needs "Container Number" on purchase orders; a hospital needs "Ward" on employee records.

The Metadata module solves this without:
- Forking the codebase
- Writing a migration
- Deploying new code

---

## 2. Custom Field Definition Entity

```go
var CustomFieldDefDefinition = def.SystemDefinition{
    Name:   "metadata_custom_field",
    Module: "metadata",
    Fields: []def.FieldDef{
        {Name: "entity_type",   Type: def.FieldData, Required: true, Immutable: true},
            // e.g. "finance_invoice", "crm_customer"
        {Name: "field_name",    Type: def.FieldData, Required: true, Immutable: true},
            // snake_case, becomes the key in custom_fields JSONB
        {Name: "field_label",   Type: def.FieldData, Required: true},
        {Name: "field_type",    Type: def.FieldSelect,
            Options: []string{"text", "number", "date", "boolean", "select", "multiselect"},
            Required: true, Immutable: true},
        {Name: "options",       Type: def.FieldJSON},
            // For select/multiselect: []string of allowed values
        {Name: "required",      Type: def.FieldBool, Default: false},
        {Name: "searchable",    Type: def.FieldBool, Default: false},
            // If true, framework adds GIN index path on custom_fields
        {Name: "display_order", Type: def.FieldInt, Default: 0},
        {Name: "active",        Type: def.FieldBool, Default: true},
        {Name: "description",   Type: def.FieldSmallText},
    },
    Hooks: def.HookSet{
        BeforeCreate: []def.BeforeCreateHook{&CustomFieldNameValidator{}},
        AfterCreate:  []def.AfterCreateHook{&CustomFieldRegistrar{}},
        AfterUpdate:  []def.AfterUpdateHook{&CustomFieldRegistrar{}},
    },
    Permissions: def.PermissionSet{
        Create: []string{"role:tenant.admin"},
        Read:   []string{"role:tenant.admin", "role:tenant.user"},
        Write:  []string{"role:tenant.admin"},
        Delete: []string{"role:tenant.admin"},
    },
}
```

---

## 3. Field Name Validation

```go
type CustomFieldNameValidator struct {
    Repo def.EntityRepository[CustomFieldDef]
}

func (h *CustomFieldNameValidator) BeforeCreate(ctx context.Context, record *def.EntityRecord) error {
    fieldName, _ := record.Fields["field_name"].(string)
    entityType, _ := record.Fields["entity_type"].(string)

    // Must be valid snake_case identifier
    if !regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`).MatchString(fieldName) {
        return &def.ValidationError{
            Fields: map[string]string{
                "field_name": "Must be lowercase snake_case, 1-63 characters, starting with a letter",
            },
        }
    }

    // Must not conflict with system field names
    systemDef, err := registry.GetDefinition(entityType)
    if err != nil {
        return &def.ValidationError{
            Fields: map[string]string{
                "entity_type": fmt.Sprintf("Unknown entity type: %s", entityType),
            },
        }
    }
    for _, f := range systemDef.Fields {
        if f.Name == fieldName {
            return &def.ValidationError{
                Fields: map[string]string{
                    "field_name": fmt.Sprintf("Field name '%s' conflicts with a system field", fieldName),
                },
            }
        }
    }

    // Must be unique per entity_type within this tenant
    exists, err := h.Repo.Exists(ctx, filter.And(
        filter.Eq("entity_type", entityType),
        filter.Eq("field_name", fieldName),
    ))
    if err != nil {
        return fmt.Errorf("CustomFieldNameValidator: check unique: %w", err)
    }
    if exists {
        return &def.ValidationError{
            Fields: map[string]string{
                "field_name": "A custom field with this name already exists for this entity type",
            },
        }
    }

    return nil
}
```

---

## 4. Registration After Creation

```go
type CustomFieldRegistrar struct {
    CustomRegistry *registry.CustomEntityRegistry
}

func (h *CustomFieldRegistrar) AfterCreate(ctx context.Context, record *def.EntityRecord) error {
    tenantID := session.TenantIDFromContext(ctx)
    entityType, _ := record.Fields["entity_type"].(string)

    // Reload all custom fields for this entity type + tenant
    // CustomEntityRegistry handles the schema update with proper locking
    return h.CustomRegistry.ReloadForTenant(ctx, tenantID, entityType)
}
```

`ReloadForTenant` uses a Redis-distributed lock to prevent concurrent schema mutations. See [Custom Entity Registry](../03-kernel/custom-entity-registry.md) for locking details.

---

## 5. Runtime Field Behavior

After registration, custom fields behave identically to system fields in:

| Subsystem | Custom field behavior |
|---|---|
| API Create/Update | Accepted in request body, validated against `field_type` and `required` |
| API Read | Included in response alongside system fields |
| Filter DSL | Filterable via `filter.Eq("custom_fields.field_name", value)` |
| SDUI list | Configurable as visible columns via column chooser |
| SDUI form | Auto-generated form field with correct input type |
| Audit log | Changes captured in `changed_fields` list |

### Storage

On system entities: custom field values are stored in the `custom_fields jsonb` column.

```sql
-- Every system entity table has this column
ALTER TABLE finance_invoice ADD COLUMN custom_fields jsonb DEFAULT '{}';
CREATE INDEX ON finance_invoice USING gin(custom_fields);
```

GIN index path for `searchable: true` fields:

```sql
CREATE INDEX ON finance_invoice ((custom_fields->>'container_number'));
```

This index is created at the time the custom field is registered (via `AfterCreate` hook triggering an async migration activity).

---

## 6. Custom Field UI (Auto-Generated)

The Settings module provides the Tenant Admin UI for managing custom fields:

```
/settings/custom-fields → Lists all entity types
/settings/custom-fields/finance_invoice → Lists custom fields for invoices
/settings/custom-fields/finance_invoice/new → Add custom field form
```

This UI is auto-generated from the `metadata_custom_field` entity definition — no custom page builder code.

---

## 7. Reading Custom Fields in Hooks

Module hooks can read custom field values like any other field:

```go
func (h *InvoiceValidator) BeforeCreate(ctx context.Context, record *def.EntityRecord) error {
    // Custom field access — same as system field
    containerNum, hasContainerNum := record.Fields["container_number"]
    if hasContainerNum && containerNum == "" {
        return &def.ValidationError{
            Fields: map[string]string{
                "container_number": "Container number cannot be empty when provided",
            },
        }
    }
    return nil
}
```

The framework merges custom fields into `record.Fields` before hook execution. Hooks do not distinguish between system fields and custom fields.

---

## 8. Limitations vs System Fields

| Capability | System field | Custom field |
|---|---|---|
| SQL FK constraint | Yes | No |
| Referential integrity | Yes | No |
| B-tree index | Yes | GIN path index only |
| Used in financial calculations | Yes | No — use `decimal.Decimal` system fields for money |
| Available before per-tenant schema load | Yes | No — requires tenant context |
| Cross-tenant schema sharing | Yes | No — per-tenant definition |

Custom fields with `field_type: number` store as JSON number (float64 precision). For financial values requiring `decimal.Decimal`, use a system entity field — do not use custom fields for money amounts.

---

## Related Documents

- [Platform Modules](platform-modules.md) — metadata module in context
- [Custom Entity Registry](../03-kernel/custom-entity-registry.md) — runtime schema management and locking
- [Fields](../04-domain/fields.md) — system field types and constraints
- [SDUI Form Patterns](../08-sdui/form-patterns.md) — auto-generated forms for custom fields
