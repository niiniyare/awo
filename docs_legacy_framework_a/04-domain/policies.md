> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Policy Functions"
id: dom-004
status: accepted
category: SPEC
stability: FROZEN
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[EntityDefinition](../03-kernel/entity-def.md)"
  - "[Hooks](hooks.md)"
  - "[Filter DSL](../05-persistence/filter-dsl.md)"
  - "[RBAC](../07-iam/rbac.md)"
  - "[Tenancy Model](../06-tenancy/tenant-model.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Policy Functions

**DOM-004 | Status: Accepted | Stability: Frozen**

This document specifies the `PolicyFunc` type, policy evaluation semantics, built-in policies, policy composition, and the distinction between RBAC (operation-level) and policy functions (row-level).

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Purpose and Scope

RBAC and Policy Functions are complementary, not alternatives:

| Mechanism | Question answered | Granularity |
|---|---|---|
| RBAC (Casbin) | May this actor perform this action on this entity type? | Operation-level |
| Policy Function | Which records may this actor see/modify? | Row-level |
| RLS (PostgreSQL) | Which rows belong to this tenant? | Tenant-level |

All three layers are applied independently. An actor must pass all three to access a record:
1. RLS — the record belongs to the actor's tenant
2. RBAC — the actor has permission to perform the operation
3. Policy Function — the predicate injection permits this specific record

---

## 2. PolicyFunc Type

```go
// PolicyFunc is a function that returns a Filter predicate injected into
// all repository operations for its entity.
//
// Must be pure: same actor state → same predicate.
// Must not query the database.
// Must not have side effects.
//
// Stability: FROZEN
type PolicyFunc func(ctx context.Context) Filter
```

`PolicyFunc` is assigned to `EntityDefinition.PolicyFn`. It is invoked by the `EntityRepository` implementation before every `Get`, `Query`, `Exists`, `Count`, and `Aggregate` call.

The returned `Filter` is ANDed with any caller-provided filter. The combined filter is what executes against the database.

---

## 3. Policy Evaluation Semantics

**AND composition:** All injected predicates MUST be satisfied. The policy function predicate is ANDed with:
- The tenant isolation predicate (from RLS — already at DB level)
- The caller-supplied filter (from the application code calling `repo.Query()`)

There is no OR composition across policy functions. An entity has exactly one `PolicyFn`. Multiple row-level constraints are expressed as a single compound filter within the `PolicyFn`.

**Pure function requirement:** `PolicyFunc` MUST be a pure function — same actor context → same filter predicate. It MUST NOT query the database, call external services, or produce side effects. Side effects in policy functions would make them unpredictable and impossible to test without infrastructure.

**Nil means no restriction:** When `PolicyFn` is nil (the default), no additional filter is injected. All records accessible to the tenant are accessible (subject to RBAC).

---

## 4. Accessing the Actor

The actor is extracted from `context.Context` using the IAM module's session context helper:

```go
import "awo.so/internal/platform/iam/session"

PolicyFn: entity.PolicyFunc(func(ctx context.Context) entity.Filter {
    actor := session.ActorFromContext(ctx)
    // actor.UserID    uuid.UUID
    // actor.TenantID  uuid.UUID
    // actor.Roles     []string
    // actor.HasRole(role string) bool
    // actor.Branch    string  — branch scope (if applicable)
    return filter.Eq("assigned_to", actor.UserID)
}),
```

If `ctx` does not carry a resolved actor (e.g., during a system operation via `SystemViewer`), `ActorFromContext()` returns the system actor, and the policy function should handle this case explicitly.

---

## 5. Built-in Policies

The framework provides commonly-used policy implementations:

### TenantIsolation

Applied automatically by RLS. Not a `PolicyFunc` — enforced at the database level. No declaration required.

### OwnerOnly

```go
// Records are visible only to the actor who created them.
PolicyFn: policy.OwnerOnly(),
```

Equivalent to:
```go
PolicyFn: entity.PolicyFunc(func(ctx context.Context) entity.Filter {
    return filter.Eq("created_by", session.ActorFromContext(ctx).UserID)
}),
```

### BranchScoped

```go
// Records are scoped to the actor's branch.
// For actors without a branch scope (e.g., tenant.admin), no restriction applies.
PolicyFn: policy.BranchScoped("branch_id"),
```

Equivalent to:
```go
PolicyFn: entity.PolicyFunc(func(ctx context.Context) entity.Filter {
    actor := session.ActorFromContext(ctx)
    if actor.Branch == "" || actor.HasRole("role:tenant.admin") {
        return filter.None()
    }
    return filter.Eq("branch_id", actor.Branch)
}),
```

### SensitiveFieldMask

Not a row-level filter. Applied at serialization time to exclude sensitive field values from responses for actors without explicit sensitive-read permission. The `Sensitive: true` field constraint handles most cases; `SensitiveFieldMask` is for dynamic sensitivity based on actor roles.

---

## 6. Policy Composition Pattern

When a single entity requires multiple row-level constraints depending on actor role:

```go
PolicyFn: entity.PolicyFunc(func(ctx context.Context) entity.Filter {
    actor := session.ActorFromContext(ctx)

    switch {
    case actor.HasRole("role:tenant.admin"):
        return filter.None() // no restriction

    case actor.HasRole("role:finance.accounts_payable"):
        // AP role sees only their own department's invoices
        return filter.Eq("department_id", actor.DepartmentID)

    case actor.HasRole("role:finance.viewer"):
        // Viewers see only submitted and above
        return filter.In("status", []string{"Submitted", "Approved", "Paid"})

    default:
        // Unknown roles see nothing
        return filter.Impossible() // always-false predicate → zero rows
    }
}),
```

`filter.None()` — no predicate injected (actor sees all tenant-scoped records).
`filter.Impossible()` — always-false predicate (actor sees zero records).

---

## 7. Policy Functions and Mutations

`PolicyFunc` is applied to read operations and also to the pre-mutation read in update and delete operations.

For `Update(ctx, id, input)`:
1. The framework first fetches the record using `Get(ctx, id)` with the policy predicate injected
2. If the record is not found (does not exist or policy excludes it), a `NotFoundError` is returned
3. If found, the update proceeds

This means a policy that restricts visibility also restricts mutability. An actor who cannot see a record cannot update or delete it. This is correct behavior — it prevents privilege escalation via blind mutations.

---

## 8. Policy Functions and SystemViewer

When an operation is performed with a `SystemViewer` context (background jobs, migration scripts, admin tooling), the `PolicyFn` must handle the system actor case:

```go
PolicyFn: entity.PolicyFunc(func(ctx context.Context) entity.Filter {
    actor := session.ActorFromContext(ctx)

    if actor.IsSystem() {
        return filter.None() // system operations bypass row-level restrictions
    }

    return filter.Eq("assigned_to", actor.UserID)
}),
```

System actors bypass row-level policy restrictions but still respect RLS (they operate within a specific tenant context).

---

## 9. Testing Policy Functions

Policy functions are pure functions over `context.Context`. They are tested without infrastructure:

```go
func TestInvoicePolicy_ViewerRole(t *testing.T) {
    ctx := session.WithActor(context.Background(), testActor(
        withRole("role:finance.viewer"),
    ))

    pred := InvoiceDefinition.PolicyFn(ctx)

    // Inspect the predicate
    sql, args := pred.ToSQL()
    assert.Contains(t, sql, "status IN")
    assert.Equal(t, []string{"Submitted", "Approved", "Paid"}, args[0])
}
```

---

## Related Documents

- [EntityDefinition](../03-kernel/entity-def.md) — PolicyFn field
- [RBAC](../07-iam/rbac.md) — operation-level permission checks (complementary)
- [Tenancy Model](../06-tenancy/tenant-model.md) — RLS tenant isolation (foundational)
- [Filter DSL](../05-persistence/filter-dsl.md) — filter.Eq, filter.In, filter.None, filter.Impossible
- [Hooks](hooks.md) — lifecycle hooks (separate from policies)
- [Glossary](../GLOSSARY.md) — PolicyFunc, Privacy Policy, Actor, SystemViewer
