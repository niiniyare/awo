# Actor Model

**Classification:** Specification — Tier 1
**Owner:** `03-auth/ACTOR_MODEL.md`
**Status:** Frozen at v1.0 (ADR-003)
**Package:** `awo.so/awo/def`

---

## Purpose

This document specifies the `Actor` struct — the identity representation injected into hooks and action handlers. Actor is distinct from `ViewerContext`: ViewerContext is the request-lifecycle authorization surface; Actor is the identity record passed to domain code.

## Scope

- The `Actor` struct fields and methods
- How Actor relates to Session and ViewerContext
- Service account identity
- Platform admin detection via roles (not a boolean field)

## Dependencies

- [`03-auth/VIEWER_CONTEXT.md`](VIEWER_CONTEXT.md) — ViewerContext interface
- [`03-auth/SESSION_MODEL.md`](SESSION_MODEL.md) — Session → Actor conversion

---

## 1. Actor Struct

```go
// Package: awo.so/awo/def

// Actor represents the authenticated principal who initiated an operation.
// Injected into EntityRecord.Meta and ActionContext by the framework before
// any hook or action handler executes.
//
// Exactly one of UserID or ServiceAccountID is non-nil. Both cannot be set
// simultaneously.
type Actor struct {
    // UserID is the UUID of the authenticated human user.
    // uuid.Nil for service account actors.
    UserID uuid.UUID

    // ServiceAccountID is the UUID of the service account.
    // uuid.Nil for human user actors.
    ServiceAccountID uuid.UUID

    // TenantID is the UUID of the tenant this actor operates within.
    // Never uuid.Nil for authenticated actors.
    TenantID uuid.UUID

    // Roles is the complete set of role names assigned to this actor.
    // Platform admin actors carry "role:platform-admin" in this slice.
    Roles []string
}
```

---

## 2. Methods

```go
// IsPlatformAdmin returns true when the actor holds "role:platform-admin".
// This replaces the deprecated IsPlatformAdmin bool field (ADR-003).
func (a *Actor) IsPlatformAdmin() bool {
    for _, r := range a.Roles {
        if r == "role:platform-admin" {
            return true
        }
    }
    return false
}

// IsServiceAccount returns true when the actor is a machine identity
// (ServiceAccountID is set) rather than a human user.
func (a *Actor) IsServiceAccount() bool {
    return a.ServiceAccountID != uuid.Nil
}
```

---

## 3. ADR-003 Rationale

**Before ADR-003:** `Actor{UserID, TenantID, Roles []string, IsPlatformAdmin bool}`

**After ADR-003:** `IsPlatformAdmin bool` removed; `ServiceAccountID uuid.UUID` added; `IsPlatformAdmin()` is now a method that checks `Roles`.

**Why the boolean was wrong:** `IsPlatformAdmin bool` created two authorization paths in the middleware — one for the boolean check and one for role-based Casbin checks. This divergence could produce silent inconsistencies where the boolean said "yes" but the Casbin role said "no" (or vice versa). As a role in the `Roles` slice, platform admin status flows through the same `PolicyEvaluator.CanPerform()` bypass check as all other role checks.

**Why ServiceAccountID was added:** Without a first-class service account type, machine integrations had to impersonate human users. This made audit logs uninterpretable ("invoice submitted by user XYZ" when XYZ was actually a Temporal activity). Service accounts have their own UUID identity, appear correctly in audit records, and can be assigned specific, minimal roles.

---

## 4. Actor vs. ViewerContext

| Concern | ViewerContext | Actor |
|---------|--------------|-------|
| Package | `awo/auth` | `awo/def` |
| Who uses it | Repository layer, PolicyEvaluator | Hooks, action handlers |
| Lifetime | Request context | Embedded in EntityRecord.Meta |
| Construction | Session middleware | Derived from Session via ToActor() |
| Authorization | Yes — used by PolicyEvaluator | No — read-only identity record |

**Rule:** Code in `awo/auth`, `awo/runtime`, and the store layer uses `ViewerContext`. Code in hooks and action handlers uses `Actor` (accessed via `record.Meta.Actor` or `actionCtx.Actor`).

---

## 5. Construction

Actor is constructed by `Session.ToActor()`:

```go
// awo/auth/session.go

func (s *Session) ToActor() *def.Actor {
    return &def.Actor{
        UserID:           s.UserID,
        ServiceAccountID: s.ServiceAccountID,
        TenantID:         s.TenantID,
        Roles:            append([]string(nil), s.Roles...),
    }
}
```

The framework constructs the Actor from the Session and injects it into:
- `EntityRecord.Meta.Actor` — available in all hooks
- `ActionContext.Actor` — available in action handlers

Module authors MUST NOT construct `Actor` manually except in tests.

---

## 6. Service Account Identity

A service account actor has:
- `ServiceAccountID != uuid.Nil`
- `UserID == uuid.Nil`
- `IsServiceAccount()` returns `true`
- `Roles` contains the service account's assigned roles (typically `"role:api-client"`)

Audit records for service account operations show `ServiceAccountID` as the actor identifier.

**Use cases:**
- Temporal activities calling the ERP API
- External integrations (accounting software, payment gateways)
- CLI migration scripts operating on tenant data

---

## 7. Normative Requirements

- Exactly one of `Actor.UserID` and `Actor.ServiceAccountID` MUST be non-nil for authenticated actors.
- `Actor.TenantID` MUST NOT be `uuid.Nil` for authenticated actors.
- `IsPlatformAdmin` MUST check `Roles` — never a dedicated boolean field.
- Module authors MUST NOT set `Actor.IsPlatformAdmin` manually (the field does not exist; this is a method).
- Hook and action handler code MUST read actor identity from the injected `Actor`, never from context directly.

---

## References

- `awo/def/record.go` — Actor struct declaration
- [`03-auth/SESSION_MODEL.md`](SESSION_MODEL.md) — Session and ToActor()
- [`03-auth/VIEWER_CONTEXT.md`](VIEWER_CONTEXT.md) — ViewerContext (distinct concept)
- ADR-003 in [`00-overview/DECISION_REGISTER.md`](../00-overview/DECISION_REGISTER.md)
