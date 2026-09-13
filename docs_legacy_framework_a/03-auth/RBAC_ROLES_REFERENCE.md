> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# RBAC Roles Reference

**Classification:** Reference — Tier 1
**Owner:** `03-auth/RBAC_ROLES_REFERENCE.md`
**Status:** Frozen at v1.0
**Package:** `awo.so/awo/iam`

---

## Purpose

This document is the exhaustive reference for all built-in role names, their scopes, and their naming conventions. Role names are **PolicyEvaluator configuration** — they map roles to permission identifiers in the IAM module. They MUST NOT appear inside `PermissionSet` declarations on `EntityDefinition`.

---

## 1. Role Name Format

```
role:{domain}.{name}
```

- `{domain}` is the module name (`finance`, `inventory`, `iam`, `tenant`, `platform`) or `tenant` for tenant-scoped system roles.
- `{name}` is a descriptive role name in snake_case.
- Exception: `role:platform-admin` uses a hyphen by historical convention.

---

## 2. System Roles (Built-in, Cannot Be Deleted)

Seeded at tenant bootstrap. Cannot be modified or deleted.

### Platform-Scoped Roles

| Role | Scope | Description |
|------|-------|-------------|
| `role:platform-admin` | All tenants | Bypasses all Casbin checks. Full platform access. Assigned only to Awo framework operators. |

### Tenant-Scoped System Roles

| Role | Scope | Description |
|------|-------|-------------|
| `role:tenant.admin` | One tenant | Full access within the tenant. Can manage all entities, users, roles, and settings. |
| `role:tenant.user` | One tenant | Standard authenticated user. Actual permissions depend on additionally assigned module roles. |
| `role:api-client` | One tenant | Machine-to-machine service accounts. Minimal default permissions; expand via specific module roles. |

---

## 3. Module Role Naming Convention

Module roles MUST follow: `role:{module}.{capability}`

**Finance module examples:**
```
role:finance.viewer               — read-only access to all finance entities
role:finance.accounts_payable     — create/update invoices, payments
role:finance.accounts_receivable  — manage customer invoices
role:finance.accountant           — post journal entries, manage ledger
role:finance.approver             — approve submitted documents
role:finance.auditor              — read-only access including sensitive fields
```

**Inventory module examples:**
```
role:inventory.viewer             — read-only stock visibility
role:inventory.manager            — manage stock moves, adjustments
role:inventory.warehouse          — perform physical inventory operations
```

**IAM module examples:**
```
role:iam.user_manager             — create/update users within tenant
role:iam.role_manager             — assign roles to users
```

---

## 4. Role Hierarchy

Role inheritance is defined via Casbin `g` assertions. A role that inherits from another automatically receives all the inherited role's permissions.

Standard inheritance:
```
role:tenant.admin inherits from role:tenant.user
role:finance.accountant inherits from role:finance.viewer
role:inventory.manager inherits from role:inventory.viewer
```

This means assigning `role:finance.accountant` to a user also grants all `role:finance.viewer` permissions.

The inheritance hierarchy is seeded at module registration time and is not tenant-configurable in v1.0.

---

## 5. Declaring Roles in PermissionSet

```go
Permissions: def.PermissionSet{
    // Use exact role name strings. No typo tolerance at declaration time.
    Create: []string{"role:finance.accounts_payable", "role:tenant.admin"},
    Read:   []string{"role:finance.viewer", "role:tenant.admin"},
    Write:  []string{"role:finance.accounts_payable", "role:tenant.admin"},
    Delete: []string{"role:tenant.admin"},
    Actions: map[string][]string{
        "approve": {"role:finance.approver", "role:tenant.admin"},
    },
},
```

---

## 6. Custom Roles

Tenants may define additional roles via the IAM admin UI. Custom roles MUST NOT conflict with built-in role names. Custom roles follow the same naming convention: `role:{module}.{name}`.

Custom roles are not part of the frozen architecture — they are tenant data, not framework data.

---

## References

- [`03-auth/AUTHORIZATION_SPEC.md`](AUTHORIZATION_SPEC.md) — How roles are enforced
- [`03-auth/CASBIN_ADAPTER.md`](CASBIN_ADAPTER.md) — Casbin configuration
