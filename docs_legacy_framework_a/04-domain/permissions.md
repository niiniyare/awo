> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Permission Sets"
id: dom-009
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: normative
related:
  - "[Policy Functions](policies.md)"
  - "[RBAC](../07-iam/rbac.md)"
  - "[Actions](actions.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Permission Sets

**DOM-009 | Status: Accepted | Stability: Stable**

This document specifies the `PermissionSet` on `EntityDefinition`: what permissions control, how they are declared, how they interact with RBAC, and how custom action permissions are declared.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. PermissionSet Structure

```go
Permissions: entity.PermissionSet{
    Create: []string{"role:finance.accounts_payable", "role:tenant.admin"},
    Read:   []string{"role:finance.viewer", "role:finance.accounts_payable", "role:tenant.admin"},
    Write:  []string{"role:finance.accounts_payable", "role:tenant.admin"},
    Delete: []string{"role:tenant.admin"},
    // Custom actions declared per ActionDef, not here
},
```

Each field is a list of role names. An actor with ANY of the listed roles is granted the permission. This is an OR condition — role membership is checked against Casbin's policy store.

---

## 2. Permission Granularity

| Permission field | Controls |
|---|---|
| `Create` | `POST /api/v1/entities/{type}` |
| `Read` | `GET /api/v1/entities/{type}` and `GET /api/v1/entities/{type}/{id}` |
| `Write` | `PATCH /api/v1/entities/{type}/{id}` |
| `Delete` | `DELETE /api/v1/entities/{type}/{id}` |
| `Submit` | `POST /api/v1/entities/{type}/{id}/submit` (if declared as action) |

Action-specific permissions are declared on `ActionDef`, not on `PermissionSet`.

---

## 3. System Roles

System roles are always present after tenant provisioning:

| Role | Typical permissions |
|---|---|
| `role:tenant.admin` | Full CRUD on all entities within the tenant |
| `role:tenant.user` | Read-only on most; write on entities in their domain |
| `role:api-client` | Limited to declared scopes |
| `role:platform-admin` | Bypasses Casbin entirely — all operations, all tenants |

Module authors MUST include `role:tenant.admin` in all `Create`, `Write`, and `Delete` permission lists. Excluding it would prevent tenant admins from managing their own data.

---

## 4. Custom Roles

Tenants can create custom roles via the IAM module:

```
POST /api/v1/entities/iam_role
Body: {
  "name": "role:finance.accounts_payable",
  "description": "Can create and submit invoices."
}
```

The role is assigned to users via the role assignment API. Module authors declare custom role names in `PermissionSet` — the role itself is created at tenant provisioning or by the tenant admin.

Custom role names MUST follow `role:{module}.{function}` format:
- `role:finance.accounts_payable`
- `role:crm.sales_rep`
- `role:hr.manager`

---

## 5. Permission Compilation

At registration, the framework compiles `PermissionSet` into Casbin policy assertions:

```
p role:finance.accounts_payable, {tenant_id}, finance_invoice, create
p role:finance.accounts_payable, {tenant_id}, finance_invoice, write
p role:finance.viewer,           {tenant_id}, finance_invoice, read
p role:tenant.admin,             {tenant_id}, finance_invoice, create
p role:tenant.admin,             {tenant_id}, finance_invoice, read
p role:tenant.admin,             {tenant_id}, finance_invoice, write
p role:tenant.admin,             {tenant_id}, finance_invoice, delete
```

These policies are seeded at tenant provisioning and updated when module permissions change (deployment).

---

## 6. SDUI Permission Gating

The page schema server applies permission checks when generating schemas:

```go
// In page builder
if psc.IfPermitted("finance_invoice", "create") {
    // Include "New Invoice" button in schema
}
if psc.IfPermitted("finance_invoice", "delete") {
    // Include row-level delete button
}
```

Components that require a permission the actor lacks are **absent from the schema** — not just visually hidden. A user cannot see a "Delete" button whose permission they lack.

---

## 7. Action Permissions

Custom action permissions are declared on `ActionDef`:

```go
Actions: []entity.ActionDef{
    {
        Name:       "submit",
        Permission: "role:finance.accounts_payable",  // single role
        // ...
    },
    {
        Name:       "approve",
        Permission: "role:finance.manager",
        // ...
    },
    {
        Name:       "cancel",
        Permissions: []string{"role:finance.manager", "role:tenant.admin"},  // multiple roles
        // ...
    },
},
```

For actions with a single permission string, `role:tenant.admin` is automatically included as an additional allowed role — tenant admins can always execute any action within their tenant.

---

## 8. Read-Only Override

For view-only contexts (embedded tables in dashboards, audit views), mark all write operations as requiring a role that no standard user has:

```go
Permissions: entity.PermissionSet{
    Create: []string{"role:system.internal_only"},  // no user has this
    Read:   []string{"role:tenant.user"},
    Write:  []string{"role:system.internal_only"},
    Delete: []string{"role:system.internal_only"},
},
```

This creates an effectively read-only entity that all users can read but none can modify through the API.

---

## 9. Permission Check Flow

```
HTTP request arrives
    ↓
Session middleware: validate token → extract Actor (roles list)
    ↓
Tenant middleware: set tenant context
    ↓
Route handler: call Casbin.Enforce(actor.UserID, tenantID, entityType, action)
    ↓
If denied: HTTP 403 {"code": "forbidden"}
    ↓
If permitted: execute handler → apply PolicyFunc (row-level filter)
    ↓
Return data filtered by PolicyFunc
```

RBAC (Casbin) = "can this actor do this operation type?"
PolicyFunc = "which specific records can this actor access?"

Both must pass. RBAC failure returns 403. PolicyFunc silently filters rows (0 results if no rows match — not 403).

---

## Related Documents

- [Policy Functions](policies.md) — row-level filter complementing RBAC
- [RBAC](../07-iam/rbac.md) — Casbin model, role hierarchy, policy storage
- [Actions](actions.md) — action-specific permissions
- [SDUI Page Builders](../08-sdui/page-builders.md) — `psc.IfPermitted` schema gating
