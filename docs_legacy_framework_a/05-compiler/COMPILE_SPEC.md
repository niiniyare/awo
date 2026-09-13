> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Compiler Specification

**Classification:** Specification — Tier 1
**Owner:** `05-compiler/COMPILE_SPEC.md`
**Status:** Frozen at v1.0
**Package:** `awo.so/awo/compiler`

---

## Purpose

This document specifies the compilation process that transforms a sealed `Registry` of `EntityDefinition` values into a `CompiledSchema`. The `CompiledSchema` is the single source of truth for all runtime subsystems.

## Dependencies

- [`01-entity/ENTITY_DEFINITION_SPEC.md`](../01-entity/ENTITY_DEFINITION_SPEC.md) — Input to compilation
- [`05-compiler/COMPILED_SCHEMA_REFERENCE.md`](COMPILED_SCHEMA_REFERENCE.md) — Output structures
- [`05-compiler/VALIDATION_RULES.md`](VALIDATION_RULES.md) — Registry validation rules

---

## 1. Compilation Entry Point

```go
// compiler.Compile transforms a sealed registry into a CompiledSchema.
// Returns error only for semantic analysis failures not caught by the registry
// validator (circular edge references, impossible defaults).
//
// On error, treat as fatal — the process cannot serve requests without
// a valid CompiledSchema.
func Compile(reg *registry.Registry) (*CompiledSchema, error)
```

`Compile` is called once at startup, after `registry.Build()` returns a valid sealed registry. The resulting `CompiledSchema` is shared across all goroutines for the lifetime of the process. It is never mutated after `Compile` returns.

---

## 2. Compilation Phases

### Phase 0 — Validation

Before building any schema structures, the compiler runs the full validation suite on the registry. See [`05-compiler/VALIDATION_RULES.md`](VALIDATION_RULES.md) for the complete rule set.

If validation produces any error-severity diagnostics, `Compile` returns an error and a nil schema.

Warning-severity diagnostics are preserved in `CompiledSchema.Diagnostics` for tooling.

### Phase 1 — EntitySchema Stubs

For each `EntityDefinition` in the registry (in deterministic order):

1. Derive `QualifiedName` = `module + "_" + localName`
2. Derive `LocalName` from `EntityName()`
3. Derive `APIResource` — plural of local name, or `PluralName` override
4. Derive `RoutePrefix` = `/api/v1/{module}/{apiResource}`
5. Derive all namespace identifiers: `EventNamespace`, `WorkflowNamespace`, `PermissionNamespace`, `MetricNamespace`, `CacheNamespace`
6. Derive `TableName` — `QualifiedName` for system entities; `"custom_entity_records"` for custom
7. Populate lookup maps: `FieldsByName`, `EdgesByName`, `ActionsByName`
8. Populate boolean index maps: `RequiredFields`, `ImmutableFields`, `SensitiveFields`, `SearchableFields`
9. Populate `FieldValidators` from `FieldDef.Validators`
10. Populate `DefaultValues` from `FieldDef.Default`

At the end of Phase 1, all `EntitySchema` objects exist with stubs for `LinkTargets` and `FieldLookups` (resolved in Phase 2).

### Phase 2 — Link Target Resolution

For each `EntitySchema`, for each `FieldTypeLink` or `FieldTypeLinkList` field:

1. Look up `FieldDef.LinkTarget` in `CompiledSchema.ByName`
2. Set `EntitySchema.LinkTargets[fieldName]` = the resolved `*EntitySchema`

If a link target does not exist in `ByName`, this is a compilation error (should have been caught by validation, but defense in depth applies).

### Phase 2.5 — CompiledLookup Generation

For each `FieldTypeLink` or `FieldTypeLinkList` field with a resolved target:

1. Determine `LabelField` from the target entity using priority heuristic: `name` → `code` → `title` → `label` → `full_name` → `account_name` → first `Searchable` field → `"id"`
2. Build `SearchURL` = `{target.RoutePrefix}?q=${keywords}`
3. Populate `EntitySchema.FieldLookups[fieldName]`

`CompiledLookup` values are consumed by the SDUI generator to render autocomplete widgets for Link fields.

### Phase 3 — Route Emission

For each `EntitySchema`, emit standard CRUD routes:

| Method | Path | Operation | Permission |
|--------|------|-----------|-----------|
| GET | `{RoutePrefix}` | `list` | `read` |
| GET | `{RoutePrefix}/:id` | `get` | `read` |
| POST | `{RoutePrefix}` | `create` | `create` |
| PATCH | `{RoutePrefix}/:id` | `update` | `write` |
| DELETE | `{RoutePrefix}/:id` | `delete` | `delete` |

For each `ActionDef` on the entity, emit an action route:

| Method | Path | Operation |
|--------|------|-----------|
| `action.Method` (default POST) | `{RoutePrefix}/:id/{action.Name}` | `action` |

All routes are accumulated in `CompiledSchema.Routes`.

### Phase 4 — CapabilityGrant Emission

For each `EntitySchema`, read `EntitySchema.Permissions` and emit `CapabilityGrant` values:

```
for each subject in Permissions.Create: CapabilityGrant{subject, qualifiedName, "create"}
for each subject in Permissions.Read:   CapabilityGrant{subject, qualifiedName, "read"}
for each subject in Permissions.Write:  CapabilityGrant{subject, qualifiedName, "write"}
for each subject in Permissions.Delete: CapabilityGrant{subject, qualifiedName, "delete"}
for each (action, subjects) in Permissions.Actions:
    for each subject in subjects: CapabilityGrant{subject, qualifiedName, action}
```

All `CapabilityGrant` values are accumulated in `CompiledSchema.CapabilityGrants`.

---

## 3. Deterministic Output

The compilation output MUST be deterministic across identical inputs. This means:

- `CompiledSchema.Entities` is ordered by `QualifiedName` lexicographically
- `CompiledSchema.Routes` is ordered by entity then by CRUD operation order (list, get, create, update, delete, then actions in declaration order)
- `CompiledSchema.CapabilityGrants` is ordered by entity then by operation

Determinism is required because the `CompiledSchema` is compared across deployments to detect unintended changes.

---

## 4. Error Semantics

`Compile` returns an error when:
- Validation produces error-severity diagnostics
- A `LinkTarget` refers to a non-existent entity (should be caught by validation)
- Default value functions panic (recovered and converted to errors)

`Compile` returns a nil error with diagnostics when:
- Validation produces only warning-severity diagnostics

The caller MUST treat a non-nil error as fatal.

---

## References

- `awo/compiler/schema.go` — Compile() implementation
- [`05-compiler/COMPILED_SCHEMA_REFERENCE.md`](COMPILED_SCHEMA_REFERENCE.md) — Output structures
- [`05-compiler/VALIDATION_RULES.md`](VALIDATION_RULES.md) — Validation rules
- [`01-entity/ENTITY_DEFINITION_SPEC.md`](../01-entity/ENTITY_DEFINITION_SPEC.md) — Input format
