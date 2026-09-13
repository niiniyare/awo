# Registry Validation Rules

**Classification:** Reference — Tier 1
**Owner:** `05-compiler/VALIDATION_RULES.md`
**Status:** Frozen at v1.0
**Package:** `awo.so/awo/registry`, `awo.so/awo/compiler`

---

## Purpose

This document enumerates all validation rules applied during `registry.Build()` and `compiler.Compile()`. Violations at `registry.Build()` are fatal errors that terminate the startup sequence.

---

## 1. Entity-Level Rules

| Rule | Severity | Description |
|------|----------|-------------|
| `ENTITY_NAME_EMPTY` | Error | `EntityName()` returns empty string |
| `ENTITY_NAME_INVALID` | Error | `EntityName()` contains uppercase, spaces, hyphens, or module prefix |
| `ENTITY_MODULE_EMPTY` | Error | `EntityModule()` returns empty string |
| `ENTITY_MODULE_INVALID` | Error | `EntityModule()` contains uppercase, underscores, or digits |
| `ENTITY_DUPLICATE` | Error | Two definitions produce the same qualified name |
| `ENTITY_MANDATORY_SYSTEM` | Error | A mandatory system entity (LedgerEntry, etc.) is declared as `CustomDefinition` |
| `ENTITY_NO_FIELDS` | Warning | Entity has no fields declared (unusual; may be intentional) |

---

## 2. Field-Level Rules

| Rule | Severity | Description |
|------|----------|-------------|
| `FIELD_NAME_EMPTY` | Error | `FieldDef.Name` is empty |
| `FIELD_NAME_INVALID` | Error | Field name is not snake_case |
| `FIELD_NAME_RESERVED` | Error | Field name is a reserved system field (id, tenant_id, created_at, updated_at, custom_fields) |
| `FIELD_NAME_DUPLICATE` | Error | Two fields in the same entity share a name |
| `FIELD_TYPE_UNKNOWN` | Error | `FieldDef.Type` is not a recognized `FieldType` constant |
| `FIELD_LINK_TARGET_MISSING` | Error | `FieldTypeLink` or `FieldTypeLinkList` field has empty `LinkTarget` |
| `FIELD_LINK_TARGET_INVALID` | Error | `LinkTarget` does not match format `{module}_{name}` |
| `FIELD_LINK_TARGET_UNRESOLVED` | Error | `LinkTarget` refers to a qualified name not in the registry |
| `FIELD_SELECT_NO_OPTIONS` | Error | `FieldTypeSelect` or `FieldTypeMultiSelect` field has no `Options` |
| `FIELD_NAMING_SERIES_NO_PATTERN` | Error | `FieldTypeNamingSeries` field has empty `Series` |
| `FIELD_NAMING_SERIES_INVALID` | Error | `Series` pattern contains invalid tokens |
| `FIELD_CURRENCY_FLOAT` | Error | `FieldTypeFloat` is named in a way strongly suggesting monetary use (name contains "amount", "price", "cost", "total", "balance") |
| `FIELD_IMMUTABLE_AND_REQUIRED` | Warning | Field is both `Immutable: true` and `Required: true` — set-once required field may confuse UX |

---

## 3. Edge-Level Rules

| Rule | Severity | Description |
|------|----------|-------------|
| `EDGE_NAME_EMPTY` | Error | `EdgeDef.Name` is empty |
| `EDGE_NAME_DUPLICATE` | Error | Two edges in the same entity share a name |
| `EDGE_TARGET_MISSING` | Error | `EdgeDef.Target` is empty |
| `EDGE_TARGET_UNRESOLVED` | Error | `EdgeDef.Target` refers to a qualified name not in the registry |
| `EDGE_TYPE_UNKNOWN` | Error | `EdgeDef.Type` is not a recognized `EdgeType` constant |

---

## 4. Action-Level Rules

| Rule | Severity | Description |
|------|----------|-------------|
| `ACTION_NAME_EMPTY` | Error | `ActionDef.Name` is empty |
| `ACTION_NAME_DUPLICATE` | Error | Two actions in the same entity share a name |
| `ACTION_NAME_RESERVED` | Error | Action name conflicts with a CRUD operation name (list, get, create, update, delete) |
| `ACTION_HANDLER_NIL` | Error | `ActionDef.HandlerFunc` is nil |

---

## 5. Workflow Trigger Rules

| Rule | Severity | Description |
|------|----------|-------------|
| `WORKFLOW_FN_EMPTY` | Error | `WorkflowTrigger.WorkflowFn` is empty |
| `WORKFLOW_TASK_QUEUE_EMPTY` | Error | `WorkflowTrigger.TaskQueue` is empty |
| `WORKFLOW_INPUT_BUILDER_NIL` | Warning | `WorkflowTrigger.InputBuilder` is nil — workflow will receive nil input |
| `WORKFLOW_DUPLICATE_EVENT` | Warning | Two triggers on the same entity bind the same `EventType` |

---

## 6. Permission Rules

| Rule | Severity | Description |
|------|----------|-------------|
| `PERMISSION_EMPTY_ALL` | Warning | All of Create/Read/Write/Delete are empty — entity is inaccessible to non-admin actors |
| `PERMISSION_ROLE_INVALID` | Warning | A role name does not match `role:{domain}.{name}` or `role:{name}` format |

---

## 7. Mandatory System Entity Rules

The following entities MUST be declared as `SystemDefinition`. Declaring them as `CustomDefinition` is a fatal error:

| Qualified Name | Reason |
|---------------|--------|
| `accounting_ledger_entry` or equivalent | Double-entry accounting |
| `inventory_stock_move` or equivalent | Inventory accounting |
| `finance_payment` or equivalent | Financial transaction |
| `iam_user` | IAM data |
| `platform_tenant` | Platform identity |
| `accounting_journal_entry` or equivalent | Double-entry |
| `tax_entry` or equivalent | Regulatory compliance |

The registry recognizes these by their qualified names. Module authors MUST use the canonical names.

---

## References

- `awo/registry/validate.go` — Validation implementation
- [`05-compiler/COMPILE_SPEC.md`](COMPILE_SPEC.md) — How validation integrates with compilation
