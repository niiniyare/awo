> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Authorization Specification

**Classification:** Specification — Tier 0
**Owner:** `03-auth/AUTHORIZATION_SPEC.md`
**Status:** Frozen at v1.0 (ADR-001, ADR-011)
**Package:** `awo.so/awo/auth`, `awo.so/awo/compiler`

---

## Purpose

This document specifies the complete authorization model: how module authors declare permissions, how the compiler compiles them to `CapabilityGrant` values, and how the `PolicyEvaluator` enforces them at runtime.

## Scope

- `PermissionSet` declaration on EntityDefinition
- `CapabilityGrant` struct (compiler output)
- `PolicyEvaluator` interface and its Casbin default implementation
- Authorization flow for every request
- Platform admin bypass
- Policy function (row-level authorization filter)

## Dependencies

- [`03-auth/VIEWER_CONTEXT.md`](VIEWER_CONTEXT.md) — ViewerContext is the authorization subject
- [`01-entity/ENTITY_DEFINITION_SPEC.md`](../01-entity/ENTITY_DEFINITION_SPEC.md) — PermissionSet on EntityDefinition
- [`00-overview/DECISION_REGISTER.md`](../00-overview/DECISION_REGISTER.md) — ADR-001, ADR-011

---

## 1. Two-Layer Authorization Architecture (ADR-001)

Authorization in Awo has two distinct layers:

**Layer 1 — Declaration (in `awo/def`, frozen):**
`PermissionSet` on `EntityDefinition`. Module authors declare **permission identifiers** — stable names for the capabilities required to perform each operation. `PermissionSet` is a Go struct: statically declared, validated at startup. It references no roles, no subjects, no authorization backend.

**Layer 2 — Enforcement (in `awo/auth`, replaceable):**
`PolicyEvaluator` interface. The runtime consults it on every request. The default implementation is Casbin-backed. It can be replaced with OPA, ReBAC, or a custom engine without touching any EntityDefinition.

**Critical invariant:** `EntityDefinition` MUST NOT reference roles, subjects, role names, RBAC constructs, ABAC attributes, JWT claims, or any authorization backend detail. These are implementation decisions owned by `PolicyEvaluator`.

**Why this matters:** Frappe (ERPNext) conflated declaration and enforcement — roles were hardcoded into entity definitions. Swapping the authorization backend required touching hundreds of entity definitions. Awo prevents this by design.

---

## 2. PermissionSet Declaration

```go
// Package: awo.so/awo/def

type PermissionSet struct {
    // Create lists permission identifiers required to create records of this entity.
    // An actor must hold at least one listed permission to perform the operation.
    Create []string

    // Read lists permission identifiers required to read records of this entity.
    // Applies to Get, Query, Count, Exists, and related operations.
    Read []string

    // Update lists permission identifiers required to update records of this entity.
    Update []string

    // Delete lists permission identifiers required to delete records of this entity.
    Delete []string

    // Actions maps custom action names to their required permission identifiers.
    // Keys are action names as declared in ActionDef.Name.
    Actions map[string][]string
}
```

Permission identifier format: `"{module}.{entity}.{operation}"`.

Examples: `"finance.invoice.create"`, `"finance.invoice.submit"`, `"inventory.stock_item.delete"`.

**Required declaration example:**

```go
Permissions: def.PermissionSet{
    Create: []string{"finance.invoice.create"},
    Read:   []string{"finance.invoice.read"},
    Update: []string{"finance.invoice.update"},
    Delete: []string{"finance.invoice.delete"},
    Actions: map[string][]string{
        "submit":         {"finance.invoice.submit"},
        "approve":        {"finance.invoice.approve"},
        "cancel":         {"finance.invoice.cancel"},
        "record_payment": {"finance.invoice.record_payment"},
    },
},
```

**MUST NOT appear inside PermissionSet:**

- Role names (`role:finance.accounts_payable`, `role:tenant.admin`)
- Subject identifiers (`user:`, `group:`)
- Any string starting with `role:`, `user:`, `group:`
- Any Casbin, OPA, or ABAC backend construct

**Empty PermissionSet:** An entity with no permission identifiers declared for an operation is inaccessible for that operation to all non-platform-admin actors (deny by default).

---

## 3. CapabilityGrant (Compiler Output, ADR-011)

The compiler reads `PermissionSet` declarations and produces `CapabilityGrant` slices:

```go
// Package: awo.so/awo/compiler

// CapabilityGrant is a compiled authorization assertion.
// One CapabilityGrant is emitted per (permission, entity, action) triple.
// Renamed from CasbinPolicy — the compiler output is engine-agnostic.
type CapabilityGrant struct {
    Permission string  // e.g. "finance.invoice.create"
    Entity     string  // qualified entity name, e.g. "finance_invoice"
    Action     string  // "create", "read", "update", "delete", or custom action name
}
```

`CompiledSchema.CapabilityGrants` contains all grants from all registered entities.

At startup the runtime loads `CapabilityGrants` into the `PolicyEvaluator` implementation. Each engine translates them into its native format:
- **Casbin**: `CapabilityGrant{Permission: "finance.invoice.create", Entity: "finance_invoice", Action: "create"}` becomes a Casbin `p` assertion binding the permission identifier to the entity+action pair.
- **OPA**: loaded as a Rego data document.
- **Custom**: implementation-defined.

**Role-to-permission mapping is external to EntityDefinition.** The IAM module manages which roles grant which permissions. This mapping lives in `iam_role_permissions` (a separate table), not in any EntityDefinition.

---

## 4. Authorization Flow

For every authenticated request:

```
1. Middleware extracts Session → constructs ViewerContext

2. viewer.IsPlatformAdmin() == true?
   → YES: AUTHORIZE stage is skipped entirely
   → NO: continue

3. PolicyEvaluator.CanPerform(ctx, viewer, qualifiedEntityName, action)
   → (true, nil):  proceed to pipeline
   → (false, nil): HTTP 403 PermissionError
   → (_, error):   HTTP 500 internal error

4. Policy filter (privacy policy) applied to query predicates:
   → PolicyFunc returns a *filter.Filter
   → filter is AND'd with the request predicate
   → Data that does not match the filter is invisible to this viewer
```

---

## 5. PolicyEvaluator Interface

```go
// Package: awo.so/awo/auth

// PolicyEvaluator is the single enforcement point for all capability decisions.
// The framework consults it at the AUTHORIZE pipeline stage for every mutation
// and before every route handler.
//
// The default implementation is Casbin-backed. Replace with OPA, ReBAC,
// or a custom engine by providing an alternative implementation at startup.
type PolicyEvaluator interface {
    // CanPerform returns true if the viewer may perform action on object.
    //
    // object is the qualified entity name (e.g., "finance_invoice").
    // action is one of: "create", "read", "update", "delete", or a custom action name.
    //
    // Implementations MUST be goroutine-safe.
    // Implementations MUST be fast (called on every request) — avoid I/O.
    CanPerform(ctx context.Context, viewer ViewerContext, object string, action string) (bool, error)
}
```

`PolicyEvaluator` is the sole owner of the question "does this viewer have this permission?". It may consult roles, attributes, claims, tenant-specific overrides, or any combination thereof. This decision is invisible to EntityDefinition.

---

## 6. Casbin Default Implementation

The default `PolicyEvaluator` uses Casbin with the following configuration:

**Model:**
```
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
```

**Policy loading (two-phase):**

Phase 1 — permission-to-entity-action bindings (from `CompiledSchema.CapabilityGrants`):
```
p, finance.invoice.create, finance_invoice, create
p, finance.invoice.submit, finance_invoice, submit
```

Phase 2 — role-to-permission bindings (from IAM `iam_role_permissions` table):
```
g, role:finance.accounts_payable, finance.invoice.create
g, role:finance.accounts_payable, finance.invoice.submit
g, role:finance.manager,          finance.invoice.approve
g, role:tenant.admin,             finance.invoice.create
g, role:tenant.admin,             finance.invoice.submit
g, role:tenant.admin,             finance.invoice.approve
```

**Thread safety:** Casbin enforcer is safe for concurrent reads after policy loading. Policy reload (for dynamic role changes) uses write-lock swap.

**Consequence:** Changing which roles have which permissions requires updating `iam_role_permissions` and reloading the Casbin evaluator — zero EntityDefinition changes required.

---

## 7. Privacy Policies (Row-Level Filters)

`PolicyEvaluator` controls operation-level access. Privacy policies control which rows are visible within an operation.

```go
// Declared on EntityDefinition — PolicyFunc for row filtering:
Policy: def.PolicyFunc(func(ctx context.Context) def.Filter {
    viewer := auth.ViewerFromContext(ctx)
    return filter.Eq("assigned_to", viewer.UserID())
})
```

The runtime calls `PolicyFunc` on every query and AND's its result with the caller's filter. Records that don't match are invisible — not forbidden, just absent.

**Built-in policies:**

| Policy | Filter applied |
|--------|--------------|
| `TenantIsolation` | Applied automatically by RLS — not a PolicyFunc |
| `OwnerOnly` | `filter.Eq("created_by", viewer.UserID())` |
| `BranchScoped` | `filter.Eq("branch_id", viewer.BranchID())` (v1.1) |

**Difference from operation-level authorization:** `PolicyEvaluator` determines whether you can access the entity at all. `PolicyFunc` determines which records you can see within that access. Both are required and complementary.

---

## 8. Normative Requirements

- `PermissionSet` MUST contain permission identifiers only — never roles, subjects, or backend constructs.
- `EntityDefinition` MUST NOT reference roles, subjects, or authorization engine details.
- The AUTHORIZE pipeline stage MUST consult `PolicyEvaluator.CanPerform()` for every mutation.
- Platform admin actors (`IsPlatformAdmin() == true`) MUST bypass the AUTHORIZE stage.
- `PolicyEvaluator` implementations MUST be goroutine-safe.
- `PolicyEvaluator.CanPerform()` MUST NOT perform database I/O on the hot path.
- The `CapabilityGrant` type MUST be the exclusive output format from the compiler — `CasbinPolicy` is permanently retired.
- Privacy policies (`PolicyFunc`) MUST be AND'd with every query predicate — never OR'd.
- Deny-by-default: operations with no declared permission identifiers in `PermissionSet` MUST be inaccessible to non-platform-admin actors.
- Role-to-permission mapping MUST live in the IAM module — never in EntityDefinition.

---

## References

- `awo/auth/evaluator.go` — PolicyEvaluator interface
- `awo/compiler/schema.go` — CapabilityGrant struct, emitGrants()
- [`03-auth/VIEWER_CONTEXT.md`](VIEWER_CONTEXT.md) — ViewerContext
- [`03-auth/RBAC_ROLES_REFERENCE.md`](RBAC_ROLES_REFERENCE.md) — Built-in roles (PolicyEvaluator configuration)
- [`06-filter/FILTER_POLICY_PATTERNS.md`](../06-filter/FILTER_POLICY_PATTERNS.md) — PolicyFunc patterns
- ADR-001 and ADR-011 in [`00-overview/DECISION_REGISTER.md`](../00-overview/DECISION_REGISTER.md)
