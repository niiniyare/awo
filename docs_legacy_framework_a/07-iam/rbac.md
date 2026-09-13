> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "RBAC"
id: iam-001
status: accepted
category: SPEC
stability: FROZEN
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Sessions](sessions.md)"
  - "[Policy Functions](../04-domain/policies.md)"
  - "[EntityDefinition](../03-kernel/entity-def.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# RBAC

**IAM-001 | Status: Accepted | Stability: Frozen**

This document specifies Awo's role-based access control model: the Casbin policy format, role hierarchy, permission evaluation, system roles, and the relationship between RBAC and row-level policy functions.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. RBAC Model

Awo uses Casbin for permission evaluation. The policy model is:

```
(subject, domain, object, action)
```

| Field | Type | Examples |
|---|---|---|
| subject | `role:{name}` or `user:{uuid}` | `role:finance.accounts_payable`, `user:abc123` |
| domain | Tenant UUID or `_platform_` | `a1b2c3d4-...`, `_platform_` |
| object | Entity type name | `finance_invoice`, `hr_employee` |
| action | Operation name | `read`, `write`, `create`, `delete`, `submit`, `cancel` |

The policy model uses RBAC with domain (multi-tenancy) and role inheritance:

```ini
[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = sub, dom, obj, act

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub, r.dom) && r.dom == p.dom && r.obj == p.obj && r.act == p.act
```

---

## 2. Permission Evaluation

Permission is checked before any application logic executes, after tenant resolution and session validation:

```go
// In route handler (auto-generated CRUD or custom action):
ok, err := casbin.Enforce(actor.Subject(), tenantID.String(), entityName, action)
if !ok {
    return c.Status(403).JSON(PermissionError{Code: "permission.denied"})
}
```

Evaluation is OR-based across roles: an actor is permitted if ANY policy tuple grants them the action. Multiple roles accumulate permissions — holding two roles grants the union of both roles' permissions.

Permission evaluation answers: "may this actor perform this action on this entity type?" It does not answer "on which specific records?" Record-level filtering is handled by [Policy Functions](../04-domain/policies.md).

---

## 3. System Roles

System roles are seeded at tenant bootstrap and cannot be deleted. They are defined in the IAM platform module and do not require `EntityDefinition.Permissions` declarations:

| Role | Scope | Description |
|---|---|---|
| `role:platform-admin` | `_platform_` domain | Bypasses Casbin entirely. All tenants, all entities, all actions. Not in policy table. |
| `role:tenant.admin` | Tenant domain | Full access within one tenant. Granted all actions on all entities. |
| `role:tenant.user` | Tenant domain | Standard user. Permissions customized per tenant via role assignments. |
| `role:api-client` | Tenant domain | Machine-to-machine. Limited to explicitly granted scopes. |

Platform admin bypass:

```go
// Before Casbin evaluation:
if actor.HasRole("role:platform-admin") {
    return true  // bypass Casbin
}
```

Platform admin is never stored in the Casbin policy table. The bypass is hard-coded in the middleware to prevent accidental removal.

---

## 4. Module Roles

Module-specific roles follow the format `role:{module}.{role_name}`:

```
role:finance.accounts_payable
role:finance.viewer
role:inventory.warehouse_manager
role:hr.payroll_officer
role:crm.sales_rep
```

These roles are declared in `EntityDefinition.Permissions` and compiled into Casbin policy tuples at framework compilation. They are seeded into the Casbin policy during tenant provisioning.

---

## 5. Role Hierarchy

Casbin `g` (group) assertions define role inheritance. A role that inherits from another accumulates all policies of the parent:

```
g, role:tenant.admin, role:finance.accounts_payable, {tenant_uuid}
g, role:tenant.admin, role:finance.viewer, {tenant_uuid}
g, role:tenant.admin, role:inventory.warehouse_manager, {tenant_uuid}
```

These assertions mean: `role:tenant.admin` inherits all permissions of the listed roles. When evaluating permissions for an actor with `role:tenant.admin`, Casbin resolves all inherited policies.

Role hierarchy is per-tenant (scoped by domain).

---

## 6. Policy Compilation

`EntityDefinition.Permissions` is compiled into Casbin policy tuples at `Registry.Compile()`:

```go
// PermissionSet
Permissions: entity.PermissionSet{
    Create: []string{"role:finance.accounts_payable", "role:tenant.admin"},
    Read:   []string{"role:finance.viewer", "role:tenant.admin"},
    Write:  []string{"role:finance.accounts_payable", "role:tenant.admin"},
    Delete: []string{"role:tenant.admin"},
},
```

Compiles to Casbin policy tuples (applied at tenant provisioning per tenant):
```
p, role:finance.accounts_payable, {tenant}, finance_invoice, create
p, role:tenant.admin,             {tenant}, finance_invoice, create
p, role:finance.viewer,           {tenant}, finance_invoice, read
p, role:tenant.admin,             {tenant}, finance_invoice, read
p, role:finance.accounts_payable, {tenant}, finance_invoice, write
p, role:tenant.admin,             {tenant}, finance_invoice, write
p, role:tenant.admin,             {tenant}, finance_invoice, delete
```

Custom action permissions (`ActionDef.Permission`) compile into additional `p` tuples for the action name.

---

## 7. SDUI Permission Gating

Permissions are checked at page schema generation time, not only at data access time. UI elements requiring a permission the actor lacks are absent from the generated page schema — not just disabled.

This prevents UI-layer information leakage: an actor without `delete` permission does not see a Delete button in the schema. An actor without `create` permission does not see the New button.

---

## Related Documents

- [Sessions](sessions.md) — Actor extraction from session context
- [Policy Functions](../04-domain/policies.md) — row-level filtering (complementary to RBAC)
- [EntityDefinition](../03-kernel/entity-def.md) — PermissionSet declaration
- [Glossary](../GLOSSARY.md) — RBAC, Casbin, Actor, Role, Permission, Platform Admin
