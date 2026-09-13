# Casbin Adapter — Implementation Reference

**Classification:** Reference — Tier 2
**Owner:** `03-auth/CASBIN_ADAPTER.md`
**Status:** Frozen at v1.0

---

## Purpose

This document describes the default Casbin-backed `PolicyEvaluator` implementation. It is an implementation reference, not an architecture specification. The architecture is in [`03-auth/AUTHORIZATION_SPEC.md`](AUTHORIZATION_SPEC.md).

---

## 1. Casbin Model

```ini
# awo/auth/casbin/model.conf

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

- `sub` — role name string (e.g., `"role:finance.accounts_payable"`)
- `obj` — qualified entity name (e.g., `"finance_invoice"`)
- `act` — operation name (e.g., `"create"`, `"read"`, `"submit"`)

---

## 2. Policy Loading

At startup, the Casbin adapter loads `CapabilityGrant` values from `CompiledSchema.CapabilityGrants`:

```go
func (a *CasbinAdapter) LoadCapabilityGrants(grants []compiler.CapabilityGrant) error {
    for _, g := range grants {
        // g.Permission is the policy subject: "finance.invoice.create"
        // g.Entity    is the policy object:  "finance_invoice"
        // g.Action    is the policy action:  "create"
        if _, err := a.enforcer.AddPolicy(g.Permission, g.Entity, g.Action); err != nil {
            return fmt.Errorf("casbin load grant: %w", err)
        }
    }
    return nil
}
```

Role inheritance is loaded from the IAM module's role hierarchy table:

```go
// For each role inheritance pair (child, parent):
a.enforcer.AddRoleForUser(childRole, parentRole)
```

---

## 3. CanPerform Implementation

```go
func (a *CasbinAdapter) CanPerform(
    ctx context.Context,
    viewer auth.ViewerContext,
    object string,
    action string,
) (bool, error) {
    for _, role := range viewer.Roles() {
        ok, err := a.enforcer.Enforce(role, object, action)
        if err != nil {
            return false, fmt.Errorf("casbin enforce: %w", err)
        }
        if ok {
            return true, nil
        }
    }
    return false, nil
}
```

Roles are checked independently. If any role in `viewer.Roles()` grants the capability, the operation is allowed.

---

## 4. Concurrency

The Casbin enforcer MUST be configured for concurrent read access. Policy updates (e.g., when a tenant changes role assignments) use a read-write mutex:

```go
type CasbinAdapter struct {
    enforcer *casbin.Enforcer
    mu       sync.RWMutex
}

func (a *CasbinAdapter) CanPerform(...) (bool, error) {
    a.mu.RLock()
    defer a.mu.RUnlock()
    // ... enforce
}

func (a *CasbinAdapter) ReloadPolicies(...) error {
    a.mu.Lock()
    defer a.mu.Unlock()
    // ... reload
}
```

---

## 5. Replacing Casbin

To replace Casbin with OPA or a custom engine:

1. Implement `auth.PolicyEvaluator`
2. Load `CompiledSchema.CapabilityGrants` into the new engine at startup
3. Pass the new implementation to `RuntimeFactory`

No EntityDefinition changes are required. The `PermissionSet` and `CapabilityGrant` types remain unchanged.

---

## References

- [`03-auth/AUTHORIZATION_SPEC.md`](AUTHORIZATION_SPEC.md) — PolicyEvaluator interface
- `awo/auth/casbin/` — Casbin adapter implementation
