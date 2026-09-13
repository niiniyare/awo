> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Filter Policy Patterns

**Classification:** Reference — Tier 1
**Owner:** `06-filter/FILTER_POLICY_PATTERNS.md`
**Status:** Frozen at v1.0

---

## Purpose

This document provides canonical patterns for `PolicyFunc` implementations — row-level visibility filters declared on `EntityDefinition`.

---

## 1. PolicyFunc Type

```go
// Package: awo.so/awo/def

// PolicyFunc is a function that returns a *filter.Filter controlling row-level
// visibility for authenticated requests. It is called on every query.
//
// The returned filter is AND'd with the caller's query filter.
// Return nil to apply no additional row-level filter.
type PolicyFunc func(ctx context.Context) Filter
```

Declared on `EntityDefinition`:
```go
var MyEntityDef = def.SystemDefinition{
    // ...
    Policy: def.PolicyFunc(func(ctx context.Context) def.Filter {
        // return a filter or nil
    }),
}
```

---

## 2. Owner-Only Policy

Records are visible only to the user who created them:

```go
Policy: def.PolicyFunc(func(ctx context.Context) def.Filter {
    viewer := auth.ViewerFromContext(ctx)
    return filter.Eq("created_by", viewer.UserID())
}),
```

**Requirement:** Entity MUST have a `created_by` field of type `FieldTypeLink` targeting `iam_user`.

---

## 3. Assigned-To Policy

Records are visible only to the user they are assigned to:

```go
Policy: def.PolicyFunc(func(ctx context.Context) def.Filter {
    viewer := auth.ViewerFromContext(ctx)
    return filter.Eq("assigned_to", viewer.UserID())
}),
```

---

## 4. Team-Scoped Policy

Records are visible to members of the actor's team:

```go
Policy: def.PolicyFunc(func(ctx context.Context) def.Filter {
    viewer := auth.ViewerFromContext(ctx)
    // teamIDs would be fetched from a session extension or derived from roles
    teamIDs := extractTeamIDs(viewer.Roles())
    if len(teamIDs) == 0 {
        return filter.IsNull("id")  // deny all — no teams
    }
    return filter.InUUIDs("team_id", teamIDs)
}),
```

---

## 5. Role-Based Visibility Policy

Admins see all records; regular users see only their own:

```go
Policy: def.PolicyFunc(func(ctx context.Context) def.Filter {
    viewer := auth.ViewerFromContext(ctx)
    if viewer.HasRole("role:tenant.admin") || viewer.HasRole("role:finance.viewer") {
        return nil  // no additional filter — see all records
    }
    return filter.Eq("created_by", viewer.UserID())
}),
```

---

## 6. Status-Based Visibility Policy

Archived records are invisible to non-admin users:

```go
Policy: def.PolicyFunc(func(ctx context.Context) def.Filter {
    viewer := auth.ViewerFromContext(ctx)
    if viewer.HasRole("role:tenant.admin") {
        return nil  // admins see all statuses
    }
    return filter.NotIn("status", "Archived", "Deleted")
}),
```

---

## 7. Policy Interaction with RBAC

PolicyFunc and RBAC are complementary:

- **RBAC (PermissionSet):** Controls whether you can access the entity operation at all (list, create, etc.).
- **PolicyFunc:** Controls which rows you can see within that access.

Both checks always run. A user with `read` permission but a restrictive `PolicyFunc` will get an empty result set, not a 403.

---

## 8. PolicyFunc and Platform Admin

`PolicyFunc` is NOT bypassed for platform admins. If a platform admin should see all records, the `PolicyFunc` must check:

```go
Policy: def.PolicyFunc(func(ctx context.Context) def.Filter {
    viewer := auth.ViewerFromContext(ctx)
    if viewer.IsPlatformAdmin() {
        return nil
    }
    return filter.Eq("created_by", viewer.UserID())
}),
```

---

## References

- [`06-filter/FILTER_DSL_REFERENCE.md`](FILTER_DSL_REFERENCE.md) — Filter constructors
- [`03-auth/AUTHORIZATION_SPEC.md`](../03-auth/AUTHORIZATION_SPEC.md) — RBAC vs policy filters
- [`03-auth/VIEWER_CONTEXT.md`](../03-auth/VIEWER_CONTEXT.md) — ViewerContext in PolicyFunc
