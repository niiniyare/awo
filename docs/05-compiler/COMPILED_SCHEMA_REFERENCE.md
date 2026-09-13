# CompiledSchema Reference

**Classification:** Reference — Tier 1
**Owner:** `05-compiler/COMPILED_SCHEMA_REFERENCE.md`
**Status:** Frozen at v1.0
**Package:** `awo.so/awo/compiler`

---

## Purpose

Complete reference for all types in `CompiledSchema`. Runtime subsystems read these types directly; they MUST NOT call back to `EntityDefinition` methods after startup.

---

## CompiledSchema

```go
type CompiledSchema struct {
    Entities         []*EntitySchema     // all entities, ordered by QualifiedName
    ByName           map[string]*EntitySchema  // O(1) lookup by QualifiedName
    Routes           []RouteDescriptor   // all auto-generated routes
    CapabilityGrants []CapabilityGrant   // all authorization assertions
    Diagnostics      Diagnostics         // warnings from compilation
}
```

**Immutability:** `CompiledSchema` MUST NOT be mutated after `Compile()` returns. It is shared across all goroutines concurrently.

---

## EntitySchema

The compiled representation of one `EntityDefinition`.

```go
type EntitySchema struct {
    // Identity
    QualifiedName string   // "finance_invoice"
    LocalName     string   // "invoice"
    Module        string   // "finance"
    IsSystem      bool

    // Display
    Label       string
    LabelPlural string
    Description string

    // HTTP / API
    APIResource  string   // "invoices" (plural URL segment)
    APISingular  string   // "invoice"
    RoutePrefix  string   // "/api/v1/finance/invoices"
    OpenAPITag   string   // "Finance"

    // Namespace identifiers
    EventNamespace      string  // "finance.invoice"
    WorkflowNamespace   string  // "finance.invoice"
    PermissionNamespace string  // "finance_invoice"
    MetricNamespace     string  // "finance_invoice"
    CacheNamespace      string  // "finance:invoice"
    TableName           string  // "finance_invoice" (or "custom_entity_records")

    // Structural metadata
    Fields           []def.FieldDef
    Edges            []def.EdgeDef
    Actions          []def.ActionDef
    Permissions      def.PermissionSet
    Hooks            def.HookSet
    WorkflowTriggers []def.WorkflowTrigger
    PageBuilders     def.PageBuilderSet

    // O(1) lookup maps
    FieldsByName   map[string]def.FieldDef
    EdgesByName    map[string]def.EdgeDef
    ActionsByName  map[string]def.ActionDef
    DefaultValues  map[string]func() any
    LinkTargets    map[string]*EntitySchema   // resolved in Phase 2
    FieldLookups   map[string]*CompiledLookup // resolved in Phase 2.5

    // Field constraint sets
    RequiredFields   map[string]bool
    ImmutableFields  map[string]bool
    SensitiveFields  map[string]bool
    SearchableFields map[string]bool
    FieldValidators  map[string][]def.FieldValidator
}
```

**Runtime rule:** Code accessing `EntitySchema` MUST use the promoted fields. It MUST NOT call through to the original `EntityDefinition` after startup. The private `def` field is retained only for compile-phase link resolution and is inaccessible after `Compile` returns.

---

## RouteDescriptor

One auto-generated HTTP route.

```go
type RouteDescriptor struct {
    Method              string  // "GET", "POST", "PATCH", "DELETE"
    Path                string  // "/api/v1/finance/invoices/:id"
    EntityQualifiedName string  // "finance_invoice"
    Module              string  // "finance"
    Resource            string  // "invoices"
    Operation           string  // "list", "get", "create", "update", "delete", "action"
    ActionName          string  // non-empty for action routes
    RequiredPermission  string  // "read", "write", "create", "delete", or action name
}
```

The API layer iterates `CompiledSchema.Routes` at startup to register Fiber route handlers.

---

## CapabilityGrant

One authorization assertion (renamed from `CasbinPolicy` per ADR-011).

```go
type CapabilityGrant struct {
    Subject string  // "role:finance.accounts_payable"
    Object  string  // "finance_invoice"
    Action  string  // "create"
}
```

The authorization engine implementation (Casbin by default) loads all `CapabilityGrant` values from `CompiledSchema.CapabilityGrants` at startup.

---

## CompiledLookup

Metadata for rendering a `FieldTypeLink` field as an autocomplete widget.

```go
type CompiledLookup struct {
    TargetQualifiedName string   // "crm_customer"
    TargetLabel         string   // "Customer"
    SearchURL           string   // "/api/v1/crm/customers?q=${keywords}"
    ValueField          string   // "id" (always)
    LabelField          string   // "name" (heuristic: first of name/code/title/label)
    Multiple            bool     // true for FieldTypeLinkList
}
```

---

## Diagnostics

```go
type Diagnostics []Diagnostic

type Diagnostic struct {
    Severity string  // "error" or "warning"
    Entity   string  // qualified entity name
    Field    string  // field name (may be empty for entity-level diagnostics)
    Message  string
}

func (d Diagnostics) HasErrors() bool
func (d Diagnostics) AsError() error  // joins all error diagnostics
```

---

## References

- `awo/compiler/schema.go` — Type declarations and Compile()
- [`05-compiler/COMPILE_SPEC.md`](COMPILE_SPEC.md) — Compilation phases
- [`05-compiler/VALIDATION_RULES.md`](VALIDATION_RULES.md) — Validation producing Diagnostics
