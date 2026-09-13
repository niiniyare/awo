> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Multi-Branch Tenancy"
id: ten-006
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Tenant Provisioning](tenant-provisioning.md)"
  - "[Settings Patterns](../10-modules/settings-patterns.md)"
  - "[RBAC](../07-iam/rbac.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Multi-Branch Tenancy

**TEN-006 | Status: Accepted | Stability: Stable**

Multi-branch support enables a single tenant to operate multiple physical locations (branches, outlets, warehouses) within one Awo account. All branches share the same tenant ID and RLS boundary — branch isolation is a domain concern, not a security boundary.

---

## 1. Branch Entity

```go
var BranchDefinition = def.SystemDefinition{
    Name:   "tenant_branch",
    Module: "tenant",
    Fields: []def.FieldDef{
        {Name: "name",       Type: def.FieldData, Required: true, Searchable: true},
        {Name: "code",       Type: def.FieldData, Required: true, Unique: true},
            // Short code used in NamingSeries prefix overrides (e.g. "NBI" for Nairobi)
        {Name: "address",    Type: def.FieldSmallText},
        {Name: "phone",      Type: def.FieldData},
        {Name: "manager",    Type: def.FieldLink, LinkTarget: "iam_user"},
        {Name: "active",     Type: def.FieldBool, Default: true},
        {Name: "timezone",   Type: def.FieldLink, LinkTarget: "timezone"},
    },
    Permissions: def.PermissionSet{
        Create: []string{"role:tenant.admin"},
        Read:   []string{"role:tenant.admin", "role:tenant.user"},
        Write:  []string{"role:tenant.admin"},
        Delete: []string{"role:tenant.admin"},
    },
}
```

---

## 2. Branch Context in Requests

Branch is identified via the `X-Branch-ID` header or subdomain pattern:

```
nbi.tenant.awo.app → Nairobi branch
msa.tenant.awo.app → Mombasa branch
```

Branch context is stored alongside tenant context in the request context:

```go
// Tenant middleware (after tenant context is set)
branchID := c.Get("X-Branch-ID")
if branchID != "" {
    branch, err := h.BranchService.Get(ctx, uuid.MustParse(branchID))
    if err != nil || !branch.Active {
        return fiber.NewError(fiber.StatusBadRequest, `{"code":"invalid_branch"}`)
    }
    ctx = session.WithBranchID(ctx, branch.ID)
    c.SetUserContext(ctx)
}
```

---

## 3. Branch-Scoped Policies

Entities that are branch-scoped use `BranchScoped` policy:

```go
// On entities that belong to a specific branch
Policy: def.PolicyFunc(func(ctx context.Context) def.Filter {
    branchID := session.BranchIDFromContext(ctx)
    if branchID == uuid.Nil {
        return filter.All()  // No branch context — see all branches (tenant admin)
    }
    return filter.Eq("branch", branchID)
}),
```

---

## 4. Branch-Level Settings Overrides

Settings follow `Branch override → Tenant override → System default`:

```go
// Branch-specific override
threshold, _ := settings.GetDecimalForBranch(ctx, "finance.invoice_approval_threshold", branchID)

// Falls back to tenant setting if no branch override exists
```

### Declaring Branch-Overridable Settings

```go
SettingInvoiceApprovalThreshold = def.Setting{
    Key:         "finance.invoice_approval_threshold",
    Label:       "Invoice Approval Threshold (KES)",
    Type:        def.SettingTypeCurrency,
    Default:     "50000.0000",
    Scope:       def.SettingScopeTenant,
    BranchOverridable: true,  // Branches can set their own threshold
}
```

---

## 5. Branch-Specific NamingSeries

When `TenantOverridable: true` is set on a NamingSeries field, and the entity has a `branch` link field, the framework also supports branch-level prefix overrides:

```
INV-2024-00001       (Nairobi branch default)
MSA-INV-2024-00001   (Mombasa branch override)
```

The prefix is configured in the Settings UI per branch.

---

## 6. Branch-Scoped RBAC

Roles can be assigned at the branch level:

```go
// iam_role_assignment supports branch scope
{
  "user":   "{user_uuid}",
  "role":   "role:forecourt.supervisor",
  "branch": "{branch_uuid}"  // optional — scopes role to this branch only
}
```

A user with `role:forecourt.supervisor` scoped to the Nairobi branch can only supervise shifts at that branch. The PolicyFunc on `forecourt_shift` enforces this:

```go
Policy: def.PolicyFunc(func(ctx context.Context) def.Filter {
    actor := session.ActorFromContext(ctx)
    if actor.HasGlobalRole("role:tenant.admin") {
        return filter.All()
    }
    // Return shifts in branches where actor has a supervisor role
    branchIDs := actor.RoleScopedBranches("role:forecourt.supervisor")
    return filter.In("station.branch", branchIDs)
}),
```

---

## 7. Cross-Branch Reporting

Tenant admins (no branch scope) see all branches' data in reports and dashboards. Branch-scoped users see only their branch.

The PolicyFunc pattern above handles this automatically — `actor.HasGlobalRole("role:tenant.admin")` returns `filter.All()` which the framework applies as no additional WHERE predicate.

---

## 8. Branch Inventory Isolation

Inventory stock is always branch-specific (tied to a `inventory_location` which belongs to a branch). Stock moves between branches are modeled as:
- OUT move from source branch location
- IN move to destination branch location

This ensures stock counts are always branch-accurate without any policy logic — inventory entities naturally isolate by location.

---

## Related Documents

- [Tenant Provisioning](tenant-provisioning.md) — branch creation during provisioning
- [Settings Patterns](../10-modules/settings-patterns.md) — branch-level settings overrides
- [RBAC](../07-iam/rbac.md) — branch-scoped role assignments
- [Inventory Patterns](../10-modules/inventory-patterns.md) — branch inventory isolation
