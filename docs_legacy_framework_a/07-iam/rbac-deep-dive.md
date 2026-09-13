> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "RBAC Deep Dive"
id: iam-010
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[RBAC](rbac.md)"
  - "[Policy Functions](../04-domain/policies.md)"
  - "[ADR-014](../17-adr/adr-014-casbin-rbac.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# RBAC Deep Dive

**IAM-010 | Status: Accepted | Stability: Stable**

Casbin policy model internals, role hierarchy, permission evaluation flow, storage, and caching.

---

## 1. Policy Model

Awo uses Casbin with the `(sub, dom, obj, act)` model:

```
[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = sub, dom, obj, act

[role_definition]
g = _, _, _           # (user, role, domain) — role inheritance

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub, r.dom) && r.dom == p.dom && r.obj == p.obj && r.act == p.act
```

| Field | Meaning | Example |
|---|---|---|
| `sub` | Subject — user ID or role name | `user:018e...` or `role:finance.viewer` |
| `dom` | Domain — tenant UUID or `_platform_` | `018e1234-5678-...` |
| `obj` | Object — entity type name | `finance_invoice` |
| `act` | Action | `read`, `create`, `write`, `delete`, `submit` |

---

## 2. Policy Storage

Policies are stored in the `iam_casbin_policy` table:

```sql
CREATE TABLE iam_casbin_policy (
    id         uuid PRIMARY KEY,
    ptype      varchar(10) NOT NULL,  -- 'p' (policy) or 'g' (role assignment)
    v0         text NOT NULL,         -- sub
    v1         text NOT NULL,         -- dom
    v2         text NOT NULL,         -- obj
    v3         text NOT NULL,         -- act
    v4         text,                  -- unused (Casbin reserves v4-v5)
    v5         text
);
```

The Casbin adapter reads all policies at startup and caches them in memory. Policy changes invalidate the cache (next request reloads). For production with many tenants, use the Redis-backed Casbin adapter (policies cached per domain).

---

## 3. Role Hierarchy

Role inheritance uses Casbin `g` assertions:

```
g, role:finance.accounts_payable, role:finance.viewer, {tenant_id}
```

This means: `role:finance.accounts_payable` inherits all permissions of `role:finance.viewer` within `{tenant_id}`.

Standard hierarchy per tenant:

```
role:tenant.admin
    └── role:finance.approver
        └── role:finance.accounts_payable
            └── role:finance.viewer
    └── role:hr.admin
        └── role:hr.payroll
            └── role:hr.viewer
    └── role:inventory.manager
        └── role:inventory.viewer
```

Platform roles:
```
role:platform-admin  (bypasses Casbin entirely — checked separately)
```

---

## 4. Permission Evaluation Flow

```
HTTP Request
    ↓
Middleware: extract tenant_id + session token
    ↓
Redis: validate session → load actor (user_id + roles)
    ↓
Casbin: enforce(actor.UserID, tenantID, entityType, action)
    ↓
Casbin evaluates: does user or any of their roles have this permission in this domain?
    ↓
Yes → proceed to handler
No  → 403 Forbidden
    ↓
Handler calls repo → PolicyFunc injects row-level filter
    ↓
PostgreSQL: RLS enforces tenant_id isolation
```

Three independent checks:
1. Session validity (Redis)
2. Operation permission (Casbin)
3. Row filter (PolicyFunc)
4. Row isolation (PostgreSQL RLS)

All four must pass for data to be returned.

---

## 5. Platform Admin Bypass

`role:platform-admin` bypasses Casbin entirely:

```go
func checkPermission(actor session.Actor, tenantID uuid.UUID, obj, act string) error {
    // Platform admin: skip Casbin
    if actor.HasRole("role:platform-admin") {
        return nil
    }
    // Regular check
    ok, err := enforcer.Enforce(actor.UserID.String(), tenantID.String(), obj, act)
    if err != nil || !ok {
        return errors.ErrForbidden
    }
    return nil
}
```

Platform admin still passes through RLS — they must also call `set_tenant_context()` to access tenant-scoped data. Platform admin is not a database superuser.

---

## 6. Granting Permissions

```go
// Grant a user a role (most common)
enforcer.AddRoleForUserInDomain(userID.String(), "role:finance.viewer", tenantID.String())

// Grant a role a specific permission directly
enforcer.AddPolicy("role:finance.accounts_payable", tenantID.String(), "finance_invoice", "create")

// Grant a user a direct permission (avoid — use roles instead)
enforcer.AddPolicy(userID.String(), tenantID.String(), "finance_invoice", "read")
```

Direct user permissions are supported but discouraged. Use roles for maintainability.

---

## 7. Custom Actions in Casbin

Custom actions (`submit`, `approve`, `cancel`) are first-class Casbin actions:

```go
// EntityDefinition declares which roles can call this action
Actions: []def.ActionDef{
    {
        Name:       "submit",
        Permission: "role:finance.accounts_payable",  // role that can call this action
    },
}
```

The framework auto-creates Casbin policies from `ActionDef.Permission` at startup:

```
p, role:finance.accounts_payable, {tenant_id}, finance_invoice, submit
```

---

## 8. Casbin Performance

With many tenants, Casbin memory usage scales with:
```
policies ≈ tenants × (roles × objects × actions)
```

For 100 tenants × 20 roles × 30 entity types × 5 actions = 300,000 policies — manageable in memory.

For 1000+ tenants, use Redis-backed Casbin adapter with domain-scoped caching:

```go
// Domain-scoped cache: only load policies for the current tenant
adapter := casbinredis.NewAdapter(redisClient,
    casbinredis.WithTenantScoped(true))
```

Cache TTL: 5 minutes per domain. Invalidated on any policy change for that tenant.

---

## 9. Testing RBAC

```go
func TestInvoiceCreate_RequiresAccountsPayableRole(t *testing.T) {
    enforcer := setupTestEnforcer()
    tenantID := uuid.New()

    // Setup: finance.viewer cannot create
    enforcer.AddRoleForUserInDomain(viewerUserID.String(), "role:finance.viewer", tenantID.String())

    ok, _ := enforcer.Enforce(viewerUserID.String(), tenantID.String(), "finance_invoice", "create")
    assert.False(t, ok)

    // Setup: accounts_payable can create (via role hierarchy)
    enforcer.AddRoleForUserInDomain(apUserID.String(), "role:finance.accounts_payable", tenantID.String())

    ok, _ = enforcer.Enforce(apUserID.String(), tenantID.String(), "finance_invoice", "create")
    assert.True(t, ok)
}
```

---

## Related Documents

- [RBAC](rbac.md) — overview and system roles
- [Policy Functions](../04-domain/policies.md) — row-level filtering (complements Casbin)
- [Session Management](session-management.md) — how actor roles are loaded from session
- [ADR-014](../17-adr/adr-014-casbin-rbac.md) — why Casbin was chosen
