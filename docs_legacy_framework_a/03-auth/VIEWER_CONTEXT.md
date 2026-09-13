> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# ViewerContext Specification

**Classification:** Specification — Tier 1
**Owner:** `03-auth/VIEWER_CONTEXT.md`
**Status:** Frozen at v1.0 (ADR-002)
**Package:** `awo.so/awo/auth`

---

## Purpose

This document specifies the `ViewerContext` interface, its lifecycle, propagation mechanism, and the invariants the framework guarantees about its presence. This is the single authorization surface for all operations.

## Scope

This specification covers:
- The `ViewerContext` interface and all methods
- The context embedding mechanism (`WithViewer` / `ViewerFromContext`)
- The `DefaultViewer` concrete implementation
- When ViewerContext is constructed and by whom
- How hooks and action handlers access it

## Dependencies

- [`03-auth/SESSION_MODEL.md`](SESSION_MODEL.md) — Session → ViewerContext construction
- [`03-auth/ACTOR_MODEL.md`](ACTOR_MODEL.md) — Actor (a projection of ViewerContext)

---

## 1. Interface

```go
// Package: awo.so/awo/auth

// ViewerContext is the authorization surface for every operation within
// a request context. The middleware pipeline constructs it from the validated
// session and injects it into context.Context before any handler, hook, or
// repository operation executes.
//
// All methods MUST be goroutine-safe. The implementation MUST be immutable
// after construction.
type ViewerContext interface {
    // TenantID returns the UUID of the tenant this viewer operates within.
    // Never uuid.Nil for authenticated requests.
    TenantID() uuid.UUID

    // UserID returns the UUID of the authenticated user.
    // Returns uuid.Nil for service account sessions.
    UserID() uuid.UUID

    // ServiceAccountID returns the UUID of the service account.
    // Returns uuid.Nil for human user sessions.
    ServiceAccountID() uuid.UUID

    // Roles returns the complete set of role names assigned to this viewer.
    // Example: ["role:tenant.admin", "role:finance.accounts_payable"]
    Roles() []string

    // HasRole returns true if the viewer holds the named role.
    // Role name comparison is case-sensitive.
    HasRole(role string) bool

    // IsPlatformAdmin returns true if the viewer holds "role:platform-admin".
    // Platform admins bypass all PolicyEvaluator checks.
    IsPlatformAdmin() bool
}
```

---

## 2. Context Propagation

### Embedding

```go
// WithViewer embeds a ViewerContext into ctx and returns the new context.
// Called by the middleware pipeline after session validation.
func WithViewer(ctx context.Context, v ViewerContext) context.Context {
    return context.WithValue(ctx, viewerKey{}, v)
}

// viewerKey is an unexported type, preventing collisions with other packages.
type viewerKey struct{}
```

### Extraction

```go
// ViewerFromContext extracts the ViewerContext from ctx.
//
// PANICS if ViewerContext is absent from ctx. This is intentional:
// ViewerContext absence means the middleware guarantee was violated.
// A panic at development time is preferable to a silent authorization bypass
// at production time.
func ViewerFromContext(ctx context.Context) ViewerContext {
    v, ok := ctx.Value(viewerKey{}).(ViewerContext)
    if !ok {
        panic("auth: ViewerContext missing from context — middleware not applied or wrong context passed")
    }
    return v
}
```

### When It Panics

`ViewerFromContext` panics when called on a context that has not passed through the session validation middleware. This occurs only when:

1. A handler bypasses the middleware pipeline (programming error)
2. A background goroutine is started with a fresh `context.Background()` instead of deriving from the request context (programming error)
3. A test does not inject a ViewerContext (test must inject one explicitly)

The panic is a programming error signal, not a runtime operational failure.

---

## 3. DefaultViewer

The concrete implementation constructed by the middleware from a `Session`:

```go
// DefaultViewer is the concrete ViewerContext constructed from a Session.
// It is constructed once per request by the session middleware and is immutable.
type DefaultViewer struct {
    tenantID         uuid.UUID
    userID           uuid.UUID
    serviceAccountID uuid.UUID
    roles            []string
    roleSet          map[string]bool  // O(1) HasRole lookup
}

func NewViewer(s *Session) *DefaultViewer {
    roleSet := make(map[string]bool, len(s.Roles))
    for _, r := range s.Roles {
        roleSet[r] = true
    }
    return &DefaultViewer{
        tenantID:         s.TenantID,
        userID:           s.UserID,
        serviceAccountID: s.ServiceAccountID,
        roles:            append([]string(nil), s.Roles...),
        roleSet:          roleSet,
    }
}

func (v *DefaultViewer) TenantID() uuid.UUID         { return v.tenantID }
func (v *DefaultViewer) UserID() uuid.UUID            { return v.userID }
func (v *DefaultViewer) ServiceAccountID() uuid.UUID  { return v.serviceAccountID }
func (v *DefaultViewer) Roles() []string              { return v.roles }
func (v *DefaultViewer) HasRole(role string) bool     { return v.roleSet[role] }
func (v *DefaultViewer) IsPlatformAdmin() bool        { return v.roleSet["role:platform-admin"] }
```

`HasRole` is O(1) via the pre-built map.

---

## 4. Lifecycle

```
HTTP Request arrives
        │
        ▼
Session middleware:
  1. Extract token from Authorization header
  2. Fetch Session from Redis (session:{token})
  3. Validate expiry: Session.IsExpired(time.Now())
  4. Validate tenant status: must be ACTIVE
  5. Construct DefaultViewer from Session
  6. auth.WithViewer(ctx, viewer) → new ctx
        │
        ▼
Request context carries ViewerContext for all downstream code
        │
        ├── Route handler: auth.ViewerFromContext(ctx)
        ├── Hook implementations: auth.ViewerFromContext(ctx)
        ├── Action handlers: action.Runtime.Actor() (derived from ViewerContext)
        └── Repository layer: extracts viewer for authorization checks
```

---

## 5. SystemViewer (Background Operations)

For background operations (Temporal activities, scheduled jobs, migration scripts) that do not have a user session, a `SystemViewer` MUST be used:

```go
// SystemViewer represents a platform-level background process.
// It operates within a specific tenant context or without a tenant (for cross-tenant ops).
type SystemViewer struct {
    tenantID uuid.UUID
    // userID and serviceAccountID are both uuid.Nil for system operations
}

func NewSystemViewer(tenantID uuid.UUID) *SystemViewer {
    return &SystemViewer{tenantID: tenantID}
}

func (v *SystemViewer) TenantID() uuid.UUID         { return v.tenantID }
func (v *SystemViewer) UserID() uuid.UUID            { return uuid.Nil }
func (v *SystemViewer) ServiceAccountID() uuid.UUID  { return uuid.Nil }
func (v *SystemViewer) Roles() []string              { return []string{"role:platform-admin"} }
func (v *SystemViewer) HasRole(role string) bool     { return role == "role:platform-admin" }
func (v *SystemViewer) IsPlatformAdmin() bool        { return true }
```

`SystemViewer` carries `"role:platform-admin"` because background platform operations bypass authorization. Use it only for legitimate platform operations, not as an authorization bypass in module code.

---

## 6. Testing

Tests that exercise hooks or handlers MUST inject a ViewerContext:

```go
func TestInvoiceCreate_RequiresAccountsPayable(t *testing.T) {
    viewer := &testViewer{
        tenantID: testTenantID,
        userID:   testUserID,
        roles:    []string{"role:tenant.user"},  // no accounts_payable role
    }
    ctx := auth.WithViewer(context.Background(), viewer)
    // ... test that create is denied
}

// testViewer implements auth.ViewerContext for tests
type testViewer struct {
    tenantID uuid.UUID
    userID   uuid.UUID
    roles    []string
}
func (v *testViewer) TenantID() uuid.UUID        { return v.tenantID }
func (v *testViewer) UserID() uuid.UUID           { return v.userID }
func (v *testViewer) ServiceAccountID() uuid.UUID { return uuid.Nil }
func (v *testViewer) Roles() []string             { return v.roles }
func (v *testViewer) HasRole(role string) bool {
    for _, r := range v.roles { if r == role { return true } }
    return false
}
func (v *testViewer) IsPlatformAdmin() bool { return v.HasRole("role:platform-admin") }
```

---

## 7. Normative Requirements

- The middleware pipeline MUST call `auth.WithViewer(ctx, viewer)` before any handler, hook, or repository operation.
- `auth.ViewerFromContext(ctx)` MUST panic when ViewerContext is absent.
- `DefaultViewer` MUST be immutable after construction.
- `HasRole` MUST be O(1) — the implementation MUST use a pre-built set.
- `ViewerContext` MUST NOT be stored beyond the lifetime of the request.
- Module authors MUST NOT construct `DefaultViewer` directly — it is constructed by the framework middleware.
- `SystemViewer` MUST be used for background operations that require a ViewerContext.

---

## References

- `awo/auth/viewer.go` — ViewerContext interface and DefaultViewer
- [`03-auth/SESSION_MODEL.md`](SESSION_MODEL.md) — Session → DefaultViewer construction
- [`03-auth/ACTOR_MODEL.md`](ACTOR_MODEL.md) — Actor (hook/action handler projection)
- [`03-auth/AUTHORIZATION_SPEC.md`](AUTHORIZATION_SPEC.md) — PolicyEvaluator using ViewerContext
- ADR-002 in [`00-overview/DECISION_REGISTER.md`](../00-overview/DECISION_REGISTER.md)
