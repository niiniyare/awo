# Compiler Specification

**Classification:** Specification — Tier 1
**Owner:** `05-registry/COMPILER_SPEC.md`
**Status:** Frozen at v1.0 (ADR-011)
**Package:** `awo.so/awo/compiler`

---

## Purpose

This document specifies the compilation phase: how a sealed `registry.Registry` is transformed into a `CompiledSchema`. The `CompiledSchema` is the single runtime authority for all subsystems — no subsystem reads `EntityDefinition` after compilation.

## Scope

- `compiler.Compile()` entry point and compilation phases
- `CompiledSchema` struct — the runtime authority
- `EntitySchema` struct — per-entity compiled metadata
- `CapabilityGrant` emission (ADR-011)
- Route derivation
- Link resolution and `CompiledLookup`

## Dependencies

- [`05-registry/REGISTRY_SPEC.md`](REGISTRY_SPEC.md) — Registry must be sealed before Compile()
- [`01-entity/ENTITY_DEFINITION_SPEC.md`](../01-entity/ENTITY_DEFINITION_SPEC.md) — Input to compilation
- [`03-auth/AUTHORIZATION_SPEC.md`](../03-auth/AUTHORIZATION_SPEC.md) — CapabilityGrant output (ADR-011)
- [`00-overview/DECISION_REGISTER.md`](../00-overview/DECISION_REGISTER.md) — ADR-011

---

## 1. Entry Point

```go
// Package: awo.so/awo/compiler

// Compile transforms a sealed Registry into a CompiledSchema.
//
// Compile runs four phases in order:
//   1. Entity schema construction (stubs without link resolution)
//   2. Link target resolution
//   3. Route emission
//   4. CapabilityGrant emission
//
// On any error the caller MUST treat the error as fatal and exit the process.
// A partially compiled schema is never returned.
//
// Compile is called once at startup. The resulting *CompiledSchema is
// immutable and safe for concurrent reads for the lifetime of the process.
func Compile(reg *registry.Registry) (*CompiledSchema, error)
```

---

## 2. CompiledSchema

`CompiledSchema` is the immutable output of compilation and the sole runtime authority. Every subsystem receives `*CompiledSchema` at startup and reads from it for the process lifetime.

```go
// CompiledSchema is the immutable product of the compilation phase.
// It is safe for concurrent read access. It is never mutated after Compile returns.
type CompiledSchema struct {
    // Entities is the ordered list of compiled entity schemas.
    // Order matches the registration order in the Registry.
    Entities []*EntitySchema

    // ByName provides O(1) lookup of EntitySchema by qualified entity name.
    // e.g. schema.ByName["finance_invoice"]
    ByName map[string]*EntitySchema

    // Routes is the flat list of all auto-generated HTTP route descriptors.
    // Consumed by the API layer at startup to register Fiber routes.
    Routes []RouteDescriptor

    // CapabilityGrants is the engine-agnostic list of capability assertions
    // derived from PermissionSet declarations (ADR-011).
    // Each grant binds a permission identifier to an entity+action pair.
    // Consumed by PolicyEvaluator at startup (Phase 1 of Casbin loading).
    CapabilityGrants []CapabilityGrant

    // Diagnostics contains warnings from compilation.
    // Error-severity diagnostics cause Compile to return an error.
    // Warning-severity diagnostics are preserved here for tooling.
    Diagnostics Diagnostics
}
```

---

## 3. EntitySchema

`EntitySchema` is the compiled representation of a single `EntityDefinition`. All fields are derived at compile time. Runtime code MUST read from `EntitySchema`, never from the original `EntityDefinition`.

```go
// EntitySchema is the compiled representation of a single EntityDefinition.
// It is the sole runtime metadata authority for its entity.
// Runtime code MUST NOT call through to EntityDefinition after compilation.
type EntitySchema struct {
    // ── Identity ─────────────────────────────────────────────────────────────

    // QualifiedName is the globally unique entity identifier.
    // Format: "{module}_{local_name}"  e.g. "finance_invoice", "iam_user"
    // Used as: DB table name, Casbin object, metric label, cache key prefix.
    QualifiedName string

    // LocalName is the module-local identifier.
    // e.g. "invoice", "user"  (EntityDefinition.Name without module prefix)
    LocalName string

    // Module is the owning module. e.g. "finance", "iam"
    Module string

    // IsSystem distinguishes SQL-backed system entities from JSONB custom entities.
    IsSystem bool

    // ── Display ──────────────────────────────────────────────────────────────

    Label       string  // singular human-readable name (e.g. "Invoice")
    LabelPlural string  // plural human-readable name (e.g. "Invoices")
    Description string  // optional entity description

    // ── HTTP / API ───────────────────────────────────────────────────────────

    // APIResource is the plural URL path segment. e.g. "invoices", "users"
    APIResource string

    // RoutePrefix is the base HTTP path for this entity's CRUD routes.
    // Format: /api/v1/{module}/{api_resource}
    RoutePrefix string

    // APISingular is the singular URL path segment. e.g. "invoice", "user"
    APISingular string

    // OpenAPITag is the human-readable OpenAPI grouping tag. e.g. "Finance"
    OpenAPITag string

    // ── Derived namespace identifiers ─────────────────────────────────────────

    EventNamespace      string  // "{module}.{local_name}"    e.g. "finance.invoice"
    WorkflowNamespace   string  // "{module}.{local_name}"    e.g. "finance.invoice"
    PermissionNamespace string  // equals QualifiedName       e.g. "finance_invoice"
    MetricNamespace     string  // equals QualifiedName       e.g. "finance_invoice"
    CacheNamespace      string  // "{module}:{local_name}"    e.g. "finance:invoice"
    TableName           string  // QualifiedName for system; "custom_entity_records" for custom

    // ── Structural metadata ──────────────────────────────────────────────────

    Fields           []def.FieldDef
    Edges            []def.EdgeDef
    Actions          []def.ActionDef
    Permissions      def.PermissionSet
    Hooks            def.HookSet
    WorkflowTriggers []def.WorkflowTrigger
    PageBuilders      def.PageBuilderSet

    // ── O(1) lookup maps ─────────────────────────────────────────────────────

    FieldsByName    map[string]def.FieldDef
    EdgesByName     map[string]def.EdgeDef
    ActionsByName   map[string]def.ActionDef
    DefaultValues   map[string]func() any
    RequiredFields  map[string]bool
    ImmutableFields map[string]bool
    SensitiveFields map[string]bool
    SearchableFields map[string]bool
    FieldValidators  map[string][]def.FieldValidator

    // LinkTargets maps FieldTypeLink field names to their target EntitySchema.
    // Resolved at compile time; all link targets are guaranteed to exist.
    LinkTargets map[string]*EntitySchema

    // FieldLookups maps Link/LinkList field names to amis select control metadata.
    FieldLookups map[string]*CompiledLookup
}
```

---

## 4. Compilation Phases

```
Compile(reg)
    │
    ├── Phase 0: Validate registry
    │       Validate(reg) → Diagnostics
    │       HasErrors() → return error (fatal)
    │
    ├── Phase 1: Build EntitySchema stubs
    │       For each EntityDefinition:
    │         buildEntitySchema(d) → EntitySchema
    │         Derive QualifiedName, RoutePrefix, namespaces, TableName
    │         Build FieldsByName, RequiredFields, ImmutableFields, etc.
    │         schema.ByName[es.QualifiedName] = es
    │
    ├── Phase 2: Resolve link targets
    │       For each EntitySchema, for each FieldTypeLink/LinkList field:
    │         es.LinkTargets[fieldName] = schema.ByName[f.LinkTarget]
    │
    ├── Phase 2.5: Build CompiledLookup for SDUI
    │       For each Link/LinkList field:
    │         buildLookup(target, multiple) → CompiledLookup
    │         Heuristic: pick label field (name, code, title, label, full_name)
    │
    ├── Phase 3: Emit HTTP routes
    │       For each EntitySchema:
    │         emitRoutes(es) → []RouteDescriptor
    │         5 standard CRUD routes + N custom action routes
    │
    └── Phase 4: Emit CapabilityGrants
            For each EntitySchema:
              emitCapabilityGrants(es) → []CapabilityGrant
              One grant per permission identifier per operation
```

---

## 5. CapabilityGrant (ADR-011)

```go
// CapabilityGrant is a compiled authorization assertion (ADR-011).
// One CapabilityGrant is emitted per (permission identifier, entity, action) triple
// declared in PermissionSet. The compiler output is engine-agnostic — it carries
// no reference to Casbin, OPA, or any specific backend.
//
// Renamed from CasbinPolicy in ADR-011. The field names were simultaneously
// renamed: Subject→Permission, Object→Entity.
type CapabilityGrant struct {
    // Permission is the permission identifier from PermissionSet.
    // Format: "{module}.{entity}.{operation}"
    // e.g. "finance.invoice.create"
    Permission string

    // Entity is the qualified entity name.
    // e.g. "finance_invoice"
    Entity string

    // Action is the operation name.
    // Standard: "create", "read", "write", "delete"
    // Custom: any action name declared in ActionDef.Name
    Action string
}
```

**Emission logic:**

```
PermissionSet.Create  = ["finance.invoice.create"]
  → CapabilityGrant{Permission: "finance.invoice.create",  Entity: "finance_invoice", Action: "create"}

PermissionSet.Read    = ["finance.invoice.read"]
  → CapabilityGrant{Permission: "finance.invoice.read",    Entity: "finance_invoice", Action: "read"}

PermissionSet.Write   = ["finance.invoice.update"]
  → CapabilityGrant{Permission: "finance.invoice.update",  Entity: "finance_invoice", Action: "write"}

PermissionSet.Delete  = ["finance.invoice.delete"]
  → CapabilityGrant{Permission: "finance.invoice.delete",  Entity: "finance_invoice", Action: "delete"}

PermissionSet.Actions["submit"] = ["finance.invoice.submit"]
  → CapabilityGrant{Permission: "finance.invoice.submit",  Entity: "finance_invoice", Action: "submit"}
```

`CompiledSchema.CapabilityGrants` is consumed at startup by `PolicyEvaluator` (Phase 1 of Casbin loading). See [`03-auth/CASBIN_ADAPTER.md`](../03-auth/CASBIN_ADAPTER.md).

---

## 6. Route Derivation

For each `EntitySchema`, five standard routes are emitted:

| Method | Path | Operation | RequiredPermission |
|---|---|---|---|
| `GET` | `{RoutePrefix}` | `list` | `"read"` |
| `GET` | `{RoutePrefix}/:id` | `get` | `"read"` |
| `POST` | `{RoutePrefix}` | `create` | `"create"` |
| `PATCH` | `{RoutePrefix}/:id` | `update` | `"write"` |
| `DELETE` | `{RoutePrefix}/:id` | `delete` | `"delete"` |

Plus one route per `ActionDef`:

| Method | Path | Operation | RequiredPermission |
|---|---|---|---|
| `{ActionDef.Method}` | `{RoutePrefix}/:id/{action.Name}` | `action` | `action.Permission` |

`RouteDescriptor.RequiredPermission` carries the permission identifier. The API middleware passes it to `authz.RequirePermission`.

---

## 7. CompiledLookup

```go
// CompiledLookup carries SDUI metadata for rendering FieldTypeLink as a
// server-side-searched select control.
type CompiledLookup struct {
    TargetQualifiedName string  // "finance_currency"
    TargetLabel         string  // "Currency"
    SearchURL           string  // "/api/v1/finance/currencies?q=${keywords}"
    ValueField          string  // always "id"
    LabelField          string  // heuristic: "name", "code", "title", "label", "full_name"
    Multiple            bool    // true for FieldTypeLinkList
}
```

`LabelField` heuristic: try fields named `"name"`, `"code"`, `"title"`, `"label"`, `"full_name"`, `"account_name"` in that priority order. Fall back to any `Searchable: true` field. Fall back to `"id"`.

---

## 8. Namespace Derivation Table

All identifiers derived from `{module}` + `{local_name}`:

| Namespace | Format | Example |
|---|---|---|
| `QualifiedName` | `{module}_{local_name}` | `finance_invoice` |
| `TableName` (system) | `{module}_{local_name}` | `finance_invoice` |
| `TableName` (custom) | `custom_entity_records` | `custom_entity_records` |
| `RoutePrefix` | `/api/v1/{module}/{api_resource}` | `/api/v1/finance/invoices` |
| `EventNamespace` | `{module}.{local_name}` | `finance.invoice` |
| `WorkflowNamespace` | `{module}.{local_name}` | `finance.invoice` |
| `PermissionNamespace` | `{module}_{local_name}` | `finance_invoice` |
| `MetricNamespace` | `{module}_{local_name}` | `finance_invoice` |
| `CacheNamespace` | `{module}:{local_name}` | `finance:invoice` |

---

## 9. Normative Requirements

- `Compile` MUST return a complete `*CompiledSchema` or a non-nil error — never a partial result.
- On error, `Compile` MUST NOT modify global state or partially register routes.
- All link targets MUST exist in the registry before `Compile` is called; dangling links are a compile-time error.
- `CompiledSchema` MUST be immutable after `Compile` returns — no field mutations.
- Runtime subsystems MUST read from `EntitySchema`, MUST NOT call through to the original `EntityDefinition`.
- `CapabilityGrant.Permission` MUST be a permission identifier — never a role name or subject identifier (ADR-011).
- `CompiledSchema.CapabilityGrants` MUST NOT be modified after `Compile` returns.
- The `CasbinPolicy` type and `CasbinPolicies` field are permanently retired (ADR-011).

---

## References

- `awo/compiler/schema.go` — CompiledSchema, EntitySchema, CapabilityGrant, Compile()
- `awo/compiler/capability_grants.go` — emitCapabilityGrants()
- [`05-registry/REGISTRY_SPEC.md`](REGISTRY_SPEC.md) — Registry (input to Compile)
- [`03-auth/AUTHORIZATION_SPEC.md`](../03-auth/AUTHORIZATION_SPEC.md) — CapabilityGrant consumer
- [`03-auth/CASBIN_ADAPTER.md`](../03-auth/CASBIN_ADAPTER.md) — Phase 1 policy loading
- ADR-011 in [`00-overview/DECISION_REGISTER.md`](../00-overview/DECISION_REGISTER.md)
