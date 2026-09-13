# EntityDefinition Specification

**Classification:** Specification — Tier 0
**Owner:** `01-entity/ENTITY_DEFINITION_SPEC.md`
**Status:** Active (ADR-022 amended interface to 16 methods)
**Package:** `awo.so/awo/def`

---

## Purpose

This document specifies the `EntityDefinition` interface, `SystemDefinition`, and `CustomDefinition` — the central primitive of the Awo Framework. It defines all methods, fields, contracts, and constraints that module authors rely on when declaring entities.

## Scope

This specification covers:
- The `EntityDefinition` interface and all 16 methods
- The `SystemDefinition` struct and its concrete implementation
- The `CustomDefinition` struct and its concrete implementation
- Entity naming rules and qualified name derivation
- The distinction between system and custom entities
- Registration semantics

## Dependencies

None. This is a Tier 0 specification.

## Related Specifications

- [`01-entity/FIELD_TYPES_REFERENCE.md`](FIELD_TYPES_REFERENCE.md) — FieldDef and all FieldType constants
- [`01-entity/EDGE_TYPES_REFERENCE.md`](EDGE_TYPES_REFERENCE.md) — EdgeDef and EdgeType semantics
- [`02-pipeline/LIFECYCLE_SPEC.md`](../02-pipeline/LIFECYCLE_SPEC.md) — How EntityDefinition drives the pipeline
- [`05-compiler/COMPILE_SPEC.md`](../05-compiler/COMPILE_SPEC.md) — How EntityDefinition becomes a runtime descriptor

---

## 1. The EntityDefinition Interface

```go
// package awo.so/awo/def

type EntityDefinition interface {
    EntityName() string
    EntityModule() string
    EntityLabel() string
    EntityLabelPlural() string
    EntityPluralName() string
    EntityDescription() string
    EntityFields() []FieldDef
    EntityEdges() []EdgeDef
    EntityHooks() HookSet
    EntityPermissions() PermissionSet
    EntityActions() []ActionDef
    EntityWorkflowTriggers() []WorkflowTrigger
    EntityPageBuilders() PageBuilderSet
    EntityLayout() LayoutDef
    EntityIcon() string
    EntityScope() Scope   // ADR-022; default ScopeTenant
    AllowAudit() bool     // ADR-023; default true
    IsSystem() bool
}
```

### Method Contracts

**`EntityName() string`**
Returns the module-local identifier. MUST be snake_case with no module prefix. Examples: `"invoice"`, `"org_assignment"`, `"customer"`. The compiler derives the globally unique qualified name as `module + "_" + name`. MUST be stable forever after any data is persisted — this value is embedded in migration filenames, Temporal workflow IDs, and Redis cache keys.

**`EntityModule() string`**
Returns the owning business domain. Examples: `"finance"`, `"inventory"`, `"iam"`, `"platform"`. Combined with `EntityName()` to derive all namespace identifiers.

**`EntityLabel() string`**
Returns the human-readable singular display name for UI rendering. If not explicitly set, the implementation MUST derive it from the local name by converting underscores to spaces and title-casing. Example: `"org_assignment"` → `"Org Assignment"`.

**`EntityLabelPlural() string`**
Returns the human-readable plural display name. If not explicitly set, the implementation MUST derive it from `EntityLabel()` by appending `"s"` (with standard English pluralization rules).

**`EntityPluralName() string`**
Returns an explicit override for the URL plural path segment. Returns empty string if automatic pluralization is acceptable. SHOULD only be set when standard pluralization produces incorrect results (e.g., `"sheep"` would pluralize incorrectly).

**`EntityDescription() string`**
Returns an optional description for documentation and OpenAPI output. MAY return empty string.

**`EntityFields() []FieldDef`**
Returns the ordered slice of field declarations. The order is significant — it determines the order in which fields appear in auto-generated list views and forms. MUST return a non-nil slice (empty slice is acceptable; nil slice is not).

**`EntityEdges() []EdgeDef`**
Returns the ordered slice of relationship declarations. MAY return nil or empty.

**`EntityHooks() HookSet`**
Returns the hook implementations for this entity. MAY return zero-value `HookSet`.

**`EntityPermissions() PermissionSet`**
Returns the RBAC permission declarations for this entity. An entity with no permissions declared for an operation is inaccessible for that operation.

**`EntityActions() []ActionDef`**
Returns custom action declarations beyond standard CRUD. MAY return nil or empty.

**`EntityWorkflowTriggers() []WorkflowTrigger`**
Returns Temporal workflow bindings. MAY return nil or empty.

**`EntityPageBuilders() PageBuilderSet`**
Returns optional SDUI page builder overrides. Zero-value means all views use auto-generated schemas.

**`EntityLayout() LayoutDef`**
Returns the SDUI layout declaration. Zero value produces a flat field list. Set Tabs or Sections to group fields.

**`EntityIcon() string`**
Returns the semantic icon name for navigation menus and breadcrumbs. Use generic names: `"document"`, `"money"`, `"user"`. Empty string means no icon.

**`EntityScope() Scope`** *(ADR-022)*
Returns the data isolation boundary. Default (zero value): `ScopeTenant`. See `def.Scope` constants:

| Scope | Meaning |
|-------|---------|
| `ScopeSystem` | Global rows; no `tenant_id`, no RLS |
| `ScopeTenant` | Default; RLS on `tenant_id` |
| `ScopeOrganization` | RLS on `tenant_id` + org filter |
| `ScopeOrganizationTree` | RLS on `tenant_id` + ltree ancestor |
| `ScopeUser` | Private to creating user |

**`AllowAudit() bool`** *(ADR-023)*
Returns `false` only when `DisableAudit: true` is set on the definition struct. Default: `true`. Use `DisableAudit: true` only for high-frequency internal bookkeeping entities where audit volume would be prohibitive (e.g. session event counters). Do NOT set for any entity holding financial, HR, or compliance-relevant data.

**`IsSystem() bool`**
Returns `true` for SQL-backed system entities. Returns `false` for JSONB-backed custom entities. MUST be deterministic — the value MUST NOT change at runtime.

---

## 2. SystemDefinition

`SystemDefinition` declares a system entity with typed SQL columns.

```go
type SystemDefinition struct {
    Name             string
    Module           string
    Label            string         // optional; derived from Name if empty
    LabelPlural      string         // optional; derived from Label if empty
    PluralName       string         // optional; overrides URL plural segment
    Description      string         // optional
    Fields           []FieldDef
    Edges            []EdgeDef
    Hooks            HookSet
    Permissions      PermissionSet
    Actions          []ActionDef
    WorkflowTriggers []WorkflowTrigger
    PageBuilders     PageBuilderSet
    AuditEnabled     bool           // default: true
}
```

`IsSystem()` returns `true`. The compiler generates typed PostgreSQL columns for each `FieldDef`. The table name equals `QualifiedName`.

### When to Use SystemDefinition

MUST be used when ANY of the following apply:

| Condition | Reason |
|-----------|--------|
| Financial data (amounts, balances) | SQL numeric constraints prevent floating-point errors |
| Inventory data (quantities, stock levels) | SQL constraints prevent negative stock |
| IAM data (users, roles, sessions) | JSONB corruption risk; FK constraints required |
| High-frequency writes (>hundreds/sec) | JSONB document parsing overhead is prohibitive |
| FK constraints to system entity PKs | JSONB fields cannot be FK targets |
| >10M records | JSONB GIN index performance degrades at scale |

**Mandatory system entities** (non-negotiable; Registry enforces this):

| Entity | Reason |
|--------|--------|
| `LedgerEntry` | Double-entry accounting; SQL numeric constraints |
| `StockMove` | Inventory accounting; SQL quantity constraints |
| `Payment` | Financial transaction; SQL + audit triggers |
| `User` | IAM data; JSONB corruption risk |
| `Tenant` | Platform identity; accessible before per-tenant schemas load |
| `JournalEntry` | Double-entry; debit/credit balance enforced at DB |
| `TaxEntry` | KRA eTIMS record; regulatory compliance |

---

## 3. CustomDefinition

`CustomDefinition` declares a JSONB-backed entity.

```go
type CustomDefinition struct {
    Name             string
    Module           string
    Label            string
    LabelPlural      string
    PluralName       string
    Description      string
    Fields           []FieldDef
    Edges            []EdgeDef
    Hooks            HookSet
    Permissions      PermissionSet
    Actions          []ActionDef
    WorkflowTriggers []WorkflowTrigger
    PageBuilders     PageBuilderSet
    AuditEnabled     bool  // default: true
}
```

`IsSystem()` returns `false`. Fields are stored as keys in a `jsonb` column in the `custom_entity_records` shared table. The compiler generates GIN indexes for `Searchable: true` fields.

### When to Use CustomDefinition

Appropriate when ALL of the following apply:

- Tenant-specific or frequently evolving schema
- No financial or inventory accounting participation
- No FK constraints to system entity PKs required
- Write rate under hundreds per second
- Record count likely to stay below 10M

### Escalation to SystemDefinition

Custom entities MUST be escalated to system entities when:
- Record count exceeds 10 million
- Fields are used in financial calculations requiring SQL-level precision
- FK constraints to system entity primary keys become necessary

Escalation requires a migration and a definition change. The entity name MUST remain stable.

---

## 4. Entity Naming

### Format

`{module}_{noun}` — both parts are snake_case.

Valid examples:
- `finance_invoice`
- `inventory_stock_move`
- `iam_user`
- `platform_organization`
- `forecourt_shift`

Invalid examples:
- `Invoice` — not snake_case, no module prefix
- `financeInvoice` — camelCase
- `finance-invoice` — hyphens not allowed
- `invoice` — no module prefix

### Qualified Name Derivation

The compiler derives the qualified name as:
```
qualified_name = module + "_" + local_name
```

The `local_name` (as returned by `EntityName()`) MUST NOT include the module prefix. The compiler prepends it.

### Stability Guarantee

The local name and module MUST NEVER change after any data has been persisted. The qualified name is embedded in:
- PostgreSQL table names (system entities)
- Migration filenames (`db/migration/YYYYMMDDHHMMSS_create_{qualified_name}.up.sql`)
- Temporal workflow IDs stored in Temporal history for years
- Redis cache keys (`{module}:{local_name}:*`)
- Casbin policy objects
- Prometheus metric labels

Renaming requires a complete data migration, Temporal history migration, and Redis key migration. This is operationally infeasible after production launch.

---

## 5. Registration

Module authors MUST register entity definitions in their module's `init()` function:

```go
// finance/invoice.go

var InvoiceDefinition = def.SystemDefinition{
    Name:   "invoice",
    Module: "finance",
    // ...
}

func init() {
    def.Register(&InvoiceDefinition)
}
```

`def.Register()` adds the definition to the global registry. It MUST be called from `init()` to guarantee registration before `registry.Build()` is called during startup.

`def.Register()` panics if:
- A definition with the same qualified name is already registered
- Called after `def.Seal()` has been invoked

**Never call `def.Register()` from a request handler.** It races with concurrent reads from the sealed registry.

---

## 6. AuditEnabled Field

Both `SystemDefinition` and `CustomDefinition` include `AuditEnabled bool` (default: `true`).

When `AuditEnabled` is `true` (the default), the runtime MUST write an `AuditRecord` in the AUDIT RECORD pipeline stage for every Create, Update, and Delete operation.

When `AuditEnabled` is `false`, the runtime MUST skip the AUDIT RECORD stage for this entity.

Setting `AuditEnabled: false` is permitted ONLY for high-frequency non-sensitive entities where audit volume would be operationally prohibitive. Examples: realtime metric snapshots, counter tables, session heartbeat records.

Setting `AuditEnabled: false` for any entity that handles financial, IAM, or tenant data is an architecture violation.

---

## 7. Normative Requirements

- EntityDefinition implementors MUST return consistent values across calls. The runtime caches method results after compilation.
- `EntityName()` MUST return a non-empty string containing only lowercase ASCII letters, digits, and underscores.
- `EntityModule()` MUST return a non-empty string containing only lowercase ASCII letters.
- `EntityFields()` MUST return a non-nil slice.
- `IsSystem()` MUST be deterministic (same value on every call, forever).
- Module authors MUST NOT implement `EntityDefinition` directly. They MUST use `SystemDefinition` or `CustomDefinition`.

---

## 8. Examples

### Minimal System Entity

```go
var CustomerDefinition = def.SystemDefinition{
    Name:   "customer",
    Module: "crm",
    Fields: []def.FieldDef{
        {Name: "name", Type: def.FieldTypeData, Required: true, Searchable: true},
        {Name: "email", Type: def.FieldTypeData, Unique: true},
    },
    Permissions: def.PermissionSet{
        // Permission identifiers only — NEVER role names (ADR-011).
        // Role-to-permission mapping is managed by the IAM module separately.
        Create: []string{"crm.customer.create"},
        Read:   []string{"crm.customer.read"},
        Write:  []string{"crm.customer.update"},
        Delete: []string{"crm.customer.delete"},
    },
}
```

### Full System Entity with Hooks and Workflow

```go
var InvoiceDefinition = def.SystemDefinition{
    Name:        "invoice",
    Module:      "finance",
    Label:       "Invoice",
    LabelPlural: "Invoices",
    Fields: []def.FieldDef{
        {Name: "number",      Type: def.FieldTypeNamingSeries, Series: "INV-{YYYY}-{SEQ:5}"},
        {Name: "customer_id", Type: def.FieldTypeLink, LinkTarget: "crm_customer", Required: true},
        {Name: "status",      Type: def.FieldTypeSelect, Options: []string{"Draft","Submitted","Paid","Cancelled"}, Default: func() any { return "Draft" }},
        {Name: "total",       Type: def.FieldTypeCurrency, Required: true},
        {Name: "notes",       Type: def.FieldTypeLongText},
    },
    Edges: []def.EdgeDef{
        {Name: "lines", Target: "finance_invoice_line", Type: def.EdgeOneToMany, CascadeDelete: true},
    },
    Hooks: def.HookSet{
        BeforeCreate: []def.BeforeCreateHook{&InvoiceValidator{}},
        AfterCreate:  []def.AfterCreateHook{&InvoiceNumberAssigner{}},
    },
    Permissions: def.PermissionSet{
        // Permission identifiers only — NEVER role names (ADR-011).
        Create: []string{"finance.invoice.create"},
        Read:   []string{"finance.invoice.read"},
        Write:  []string{"finance.invoice.update"},
        Delete: []string{"finance.invoice.delete"},
        Actions: map[string][]string{
            "submit": {"finance.invoice.submit"},
        },
    },
    Actions: []def.ActionDef{
        {Name: "submit", Method: def.ActionMethodPost, Label: "Submit", Permission: "submit", HandlerFunc: SubmitInvoiceAction},
    },
    WorkflowTriggers: []def.WorkflowTrigger{
        {
            On:         def.EventOnSubmit,
            WorkflowFn: "InvoiceSubmissionWorkflow",
            TaskQueue:  "finance.invoice.submit",
            InputBuilder: func(rec *def.EntityRecord, tc def.TriggerContext) (any, error) {
                return InvoiceSubmissionInput{TenantID: rec.TenantID, InvoiceID: rec.ID}, nil
            },
        },
    },
    AuditEnabled: true,
}

func init() {
    def.Register(&InvoiceDefinition)
}
```

---

## References

- `awo/def/entity.go` — Implementation
- [`01-entity/FIELD_TYPES_REFERENCE.md`](FIELD_TYPES_REFERENCE.md)
- [`01-entity/NAMING_CONVENTIONS.md`](NAMING_CONVENTIONS.md)
- [`05-compiler/COMPILE_SPEC.md`](../05-compiler/COMPILE_SPEC.md)
- [`02-pipeline/LIFECYCLE_SPEC.md`](../02-pipeline/LIFECYCLE_SPEC.md)
