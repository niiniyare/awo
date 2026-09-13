# Actor Specification

**Classification:** Specification — Tier 0
**Owner:** `03-auth/ACTOR_SPEC.md`
**Status:** Frozen at v1.0 (ADR-003)
**Package:** `awo.so/awo/def`

---

## Purpose

This document specifies `def.Actor` — the authenticated principal representation passed through the hook pipeline and action handlers. `Actor` carries identity for business logic; `ViewerContext` carries identity for authorization. They are constructed from the same `Session` and carry identical data.

## Scope

- `def.Actor` struct definition and all fields (ADR-003)
- `IsPlatformAdmin()`, `IsServiceAccount()`, `HasRole()` methods
- Identity invariants
- Construction from `auth.Session`
- Where Actor is accessible in the framework
- Distinction from `ViewerContext`

## Dependencies

- [`00-overview/DECISION_REGISTER.md`](../00-overview/DECISION_REGISTER.md) — ADR-003
- [`03-auth/VIEWER_CONTEXT.md`](VIEWER_CONTEXT.md) — ViewerContext carries Actor via Actor() method
- [`03-auth/SESSION_SPEC.md`](SESSION_SPEC.md) — Session.ToActor() constructs Actor

---

## 1. Struct Definition (ADR-003)

```go
// Package: awo.so/awo/def

// Actor represents the authenticated principal performing an operation.
// It is constructed from auth.Session at request time and embedded in the hook
// pipeline via RecordMeta.Actor and in action handlers via ActionContext.Actor.
//
// ADR-003 decisions:
//   - IsPlatformAdmin is a METHOD, not a bool field. Privileges belong in Roles.
//   - ServiceAccountID was added to distinguish human users from service accounts.
//   - Exactly one of UserID and ServiceAccountID is non-nil for any given Actor.
type Actor struct {
    // UserID is the UUID of the authenticated human user.
    // uuid.Nil for service account sessions.
    UserID uuid.UUID

    // ServiceAccountID is the UUID of the authenticated service account.
    // uuid.Nil for human user sessions.
    // Exactly one of UserID and ServiceAccountID is non-nil.
    ServiceAccountID uuid.UUID

    // TenantID is the tenant the actor is operating within.
    // Always non-nil for authenticated requests.
    TenantID uuid.UUID

    // Roles is the complete set of role names held by this principal within
    // the current tenant scope. Format: "role:{domain}.{name}".
    // Example: []string{"role:tenant.admin", "role:finance.accounts_payable"}
    // Loaded once at session time; stale after role changes until re-login.
    Roles []string
}
```

---

## 2. Methods

```go
// IsPlatformAdmin reports whether this actor holds "role:platform-admin".
// Platform admins bypass all PolicyEvaluator checks unconditionally (ADR-001).
//
// IsPlatformAdmin is a method, not a field. This is the ADR-003 decision.
// Having it as a bool field created two authorization paths and allowed
// privileged construction without proper role loading. The method form
// ensures privilege derives exclusively from the Roles slice.
func (a *Actor) IsPlatformAdmin() bool {
    for _, r := range a.Roles {
        if r == "role:platform-admin" {
            return true
        }
    }
    return false
}

// IsServiceAccount reports whether this actor is a machine/service principal.
// Returns true when ServiceAccountID is non-nil.
func (a *Actor) IsServiceAccount() bool {
    return a.ServiceAccountID != uuid.Nil
}

// HasRole reports whether this actor holds the named role (exact match).
// Comparison is case-sensitive. Linear scan — O(n) where n = len(Roles).
// Use ViewerContext.HasRole() for O(1) on the hot path (PolicyEvaluator).
func (a *Actor) HasRole(role string) bool {
    for _, r := range a.Roles {
        if r == role {
            return true
        }
    }
    return false
}
```

**Note on HasRole performance:** `Actor.HasRole` is O(n). This is acceptable for hook and action handler code where it is called infrequently. `ViewerContext.HasRole` is O(1) (pre-built map) and MUST be used by `PolicyEvaluator`.

---

## 3. Identity Invariants

The following invariants hold for every `Actor` constructed from a valid `Session`:

1. **Exactly one identity**: Either `UserID != uuid.Nil` or `ServiceAccountID != uuid.Nil`, never both, never neither.
2. **TenantID is always set**: `TenantID != uuid.Nil` for all authenticated actors. Background system actors use the target tenant's UUID.
3. **IsPlatformAdmin is derived from Roles**: `IsPlatformAdmin()` returns true if and only if `"role:platform-admin"` appears in `Roles`. It cannot be forced by constructing a struct literal with a bool field.
4. **Roles are loaded at session time**: The Roles slice reflects IAM state at login time. It does not update during the request. Role changes take effect at next login.
5. **Actor is immutable**: No code may mutate an Actor's fields after construction. Hooks and action handlers receive a pointer; they MUST treat it as read-only.

---

## 4. Construction

Actors are constructed from `auth.Session` by the session middleware:

```go
// Package: awo.so/awo/auth

func (s *Session) ToActor() *def.Actor {
    return &def.Actor{
        UserID:           s.UserID,
        ServiceAccountID: s.ServiceAccountID,
        TenantID:         s.TenantID,
        Roles:            append([]string(nil), s.Roles...),  // defensive copy
    }
}
```

`Session.Roles` is the source of truth. It is populated at login by loading `iam_user_roles` and resolving the role hierarchy.

---

## 5. Accessibility in the Framework

`Actor` is accessible at the following points in the request lifecycle:

| Location | Access pattern | Notes |
|---|---|---|
| `def.RecordMeta.Actor` | `record.Meta.Actor` | Available in all hook implementations |
| `def.ActionContext.Actor` | `ctx.Actor` | Available in custom action handlers |
| `def.TriggerContext.Actor` | `tc.Actor` | Available in WorkflowTrigger.InputBuilder |
| `def.ActionRuntime.Actor()` | `rt.Actor()` | Method on the runtime service |
| `auth.ViewerContext.Actor()` | `viewer.Actor()` | Projection from ViewerContext |

---

## 6. Distinction from ViewerContext

`def.Actor` and `auth.ViewerContext` carry the same identity but serve different layers:

| | `def.Actor` | `auth.ViewerContext` |
|---|---|---|
| **Package** | `awo/def` — lowest dependency level | `awo/auth` — higher level |
| **Primary consumer** | Hooks, action handlers, workflow triggers | PolicyEvaluator, middleware |
| **HasRole performance** | O(n) — acceptable for business logic | O(1) — required for auth hot path |
| **Source** | `Session.ToActor()` | `Session.ToViewer()` |

The separation exists because `awo/def` has no dependencies (Level 0 in the package DAG). Importing `awo/auth` from `awo/def` would introduce a cycle. `Actor` in `def` keeps the package boundary clean.

---

## 7. Service Account Actors

Service accounts are machine principals authenticated via API tokens. Their Actor has:

- `ServiceAccountID` — set to the service account UUID
- `UserID` — `uuid.Nil`
- `Roles` — loaded from `iam_service_account_roles` at token validation time
- `TenantID` — the tenant owning the service account

Service accounts participate in the same authorization model as human users. There is no special-casing in hooks or PolicyEvaluator.

---

## 8. Background Operation Actors

Temporal activities and scheduled jobs that operate without a user session construct a system Actor:

```go
// For framework-internal background operations:
actor := &def.Actor{
    TenantID: tenantID,
    Roles:    []string{"role:platform-admin"},
}
```

`RecordMeta.Actor` is `nil` only during bootstrap migrations that predate the IAM system. All other operations have a non-nil Actor.

---

## 9. Normative Requirements

- `IsPlatformAdmin` MUST be a method — never a bool field (ADR-003).
- `ServiceAccountID` MUST be `uuid.Nil` for human user sessions.
- `UserID` MUST be `uuid.Nil` for service account sessions.
- Hooks and action handlers MUST treat Actor as read-only.
- `Actor.Roles` MUST be a defensive copy of `Session.Roles` — not a shared slice.
- `Actor` MUST NOT be constructed with arbitrary roles outside of `Session.ToActor()` and controlled background operation paths.

---

## References

- `awo/def/record.go` — Actor struct definition
- `awo/auth/session.go` — Session.ToActor()
- [`03-auth/VIEWER_CONTEXT.md`](VIEWER_CONTEXT.md) — ViewerContext.Actor()
- [`03-auth/SESSION_SPEC.md`](SESSION_SPEC.md) — Session is the source of Actor
- ADR-003 in [`00-overview/DECISION_REGISTER.md`](../00-overview/DECISION_REGISTER.md)
