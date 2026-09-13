> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "ADR-001: Entity Identity as Compiler Artifacts"
id: adr-001
status: accepted
category: ADR
stability: FROZEN
audience: [framework-authors, module-authors, contributors]
since: "1.1"
normative-level: normative
related:
  - "[EntityDefinition](entity-def.md)"
  - "[Compilation Pipeline](compilation-pipeline.md)"
  - "[Entity Registry](registry.md)"
---

# ADR-001: Entity Identity as Compiler Artifacts

**Status: Accepted**

---

## Context

Prior to v1.1, `EntityDefinition.Name` was the globally-qualified entity
identifier: `"finance_invoice"`, `"iam_user"`, `"platform_organization"`.

This created three problems:

1. **Redundancy.** Both `Module` and `Name` encoded the same module
   information. `Module: "finance"`, `Name: "finance_invoice"` — the
   `finance_` prefix is pure duplication.

2. **Manual construction of qualified names.** Runtime packages concatenated
   `module + "_" + name` themselves. This scattered identity logic across
   the codebase with no single authoritative source.

3. **Route coupling.** Routes were `/api/v1/entities/{name}` — globally
   flat, not module-oriented. Adding a module hierarchy required a breaking
   change to the URL scheme.

---

## Decision

**`EntityDefinition.Name` is the module-local identifier.**

Entity authors declare only `Module` and local `Name`:

```go
var InvoiceDefinition = def.SystemDefinition{
    Module: "finance",
    Name:   "invoice",   // local — no "finance_" prefix
}
```

**All globally-unique identifiers are compiler artifacts.**

The compiler produces `EntitySchema` at startup, which exposes:

| Field | Value (for finance / invoice) |
|---|---|
| `QualifiedName` | `finance_invoice` |
| `TableName` | `finance_invoice` |
| `RoutePrefix` | `/api/v1/finance/invoices` |
| `APIResource` | `invoices` |
| `APISingular` | `invoice` |
| `OpenAPITag` | `Finance` |
| `EventNamespace` | `finance.invoice` |
| `WorkflowNamespace` | `finance.invoice` |
| `PermissionNamespace` | `finance_invoice` |
| `MetricNamespace` | `finance_invoice` |
| `CacheNamespace` | `finance:invoice` |

**No runtime package may concatenate `module + "_" + name` manually.**
The compiler's `EntitySchema` is the single source of truth for all
identity-derived strings.

---

## Consequences

### Positive

- Module-oriented REST routes (`/api/v1/finance/invoices`) replace the flat
  entity-namespace routes (`/api/v1/entities/finance_invoice`). Public APIs
  are semantically clearer.

- All namespaces (events, workflows, metrics, cache, permissions) are
  derived consistently from the same formula, in one place.

- Entity authors write less: no need to manually prefix every `Name` with
  the module name.

- The compiler can detect naming collisions (`LocalName` must be unique
  within a module; `QualifiedName` must be unique globally).

- Explicit `PluralName` override provides an escape hatch for English
  irregularities without changing the pluralization logic.

### Negative / Migration

- All existing definitions required a one-time `Name` field update (strip
  module prefix). This was a mechanical, non-breaking change: database table
  names (QualifiedName) and all external identifiers are unchanged.

- Tests that checked `EntityName()` against qualified names (e.g.
  `"demo_customer"`) required updating to check local names (`"customer"`).

### Backward Compatibility

- During the transitional period, the `QualifiedName()` function in
  `awo/def` detects old-style names (where `Name` already contains the
  module prefix) and returns them unchanged. This prevents double-prefixing.

- The compiler emits `SeverityWarning` for any definition still using
  prefixed names, with a suggested migration.

- This compatibility shim is temporary and will be removed in v2.0.

---

## Alternatives Considered

### Keep Name as qualified identifier

Rejected. Redundancy between `Module` and `Name` would remain. Route
generation and namespace derivation would still be scattered.

### Derive Module from Name prefix

Rejected. Would make `Module` redundant and break the ability to have
meaningful module names that differ from the entity name prefix (e.g. a
module named `platform` might contain entities named `tenant`, `organization`
— the prefix is the module name, not part of the entity concept).

### Use a separate `QualifiedName` field on EntityDefinition

Rejected. Having entity authors maintain both a local name and a qualified
name is more error-prone than deriving the qualified name at compile time.

---

## Implementation

See `awo/def/naming.go` for `QualifiedName()`, `LocalName()`,
`HasModulePrefix()`, `PluralizeLocal()`, `DeriveLabel()`,
`DerivePluralLabel()`, `OpenAPITag()`.

See `awo/compiler/schema.go` for `EntitySchema` with all derived namespace
fields populated in `buildEntitySchema()`.
