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

## 5. Role-to-Permission Mapping

Role names MUST NOT appear in `PermissionSet` declarations. `PermissionSet` contains permission identifiers only (ADR-001). Role-to-permission mapping is data owned by the IAM module, not code in EntityDefinition.

The IAM module seeds role-to-permission bindings at tenant bootstrap via `iam_role_permissions`:

```sql
-- Example: which roles grant finance.invoice.create
-- This lives in a migration seed, NOT in any EntityDefinition
INSERT INTO iam_role_permissions (role_id, permission_identifier) VALUES
    ('role:tenant.admin',             'finance.invoice.create'),
    ('role:finance.manager',          'finance.invoice.create'),
    ('role:finance.accounts_payable', 'finance.invoice.create');
```

The corresponding `PermissionSet` declares only the permission identifier:

```go
// CORRECT: permission identifiers only
Permissions: def.PermissionSet{
    Create: []string{"finance.invoice.create"},
    Read:   []string{"finance.invoice.read"},
    Write:  []string{"finance.invoice.update"},
    Delete: []string{"finance.invoice.delete"},
    Actions: map[string][]string{
        "approve": {"finance.invoice.approve"},
    },
},

// WRONG: role names in PermissionSet — rejected at compile time
Permissions: def.PermissionSet{
    Create: []string{"role:finance.accounts_payable"},  // ← compiler error
},
```

See [`03-auth/AUTHORIZATION_SPEC.md`](AUTHORIZATION_SPEC.md) for the full two-layer authorization model.

---

## 6. Custom Roles

Tenants may define additional roles via the IAM admin UI. Custom roles MUST NOT conflict with built-in role names. Custom roles follow the same naming convention: `role:{module}.{name}`.

Custom roles are not part of the frozen architecture — they are tenant data, not framework data.

---

## References

- [`03-auth/AUTHORIZATION_SPEC.md`](AUTHORIZATION_SPEC.md) — How roles are enforced
- [`03-auth/CASBIN_ADAPTER.md`](CASBIN_ADAPTER.md) — Casbin configuration
