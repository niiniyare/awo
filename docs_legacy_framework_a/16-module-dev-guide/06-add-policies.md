> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Add Policies"
id: mdg-06
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Add Hooks](05-add-hooks.md)"
  - "[Add Actions](07-add-actions.md)"
  - "[Policies](../04-domain/policies.md)"
  - "[RBAC](../07-iam/rbac.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Add Policies

**MDG-06 | Module Developer Guide**

This document adds RBAC permissions and a `PolicyFunc` row-level filter to `crm_contact`. RBAC controls what operations actors can perform; PolicyFunc controls which records they can see.

---

## 1. Add Permissions to ContactDefinition

```go
// internal/core/crm/def.go
var ContactDefinition = def.EntityDefinition{
    // ... Name, Module, Label, StorageModel, Fields, Edges, Hooks ...

    Permissions: def.PermissionSet{
        // Who can create contacts
        Create: []string{
            "role:crm.sales_rep",
            "role:crm.manager",
            "role:tenant.admin",
        },
        // Who can read contacts (read permission is checked on list and get)
        Read: []string{
            "role:crm.sales_rep",
            "role:crm.manager",
            "role:crm.viewer",
            "role:tenant.admin",
        },
        // Who can update contacts
        Write: []string{
            "role:crm.sales_rep",
            "role:crm.manager",
            "role:tenant.admin",
        },
        // Who can delete contacts
        Delete: []string{
            "role:crm.manager",
            "role:tenant.admin",
        },
    },
}
```

Permissions use OR evaluation — an actor needs **any one** of the listed roles. Role names follow `role:{module}.{role_name}` convention for module roles.

---

## 2. Add a PolicyFunc for Row-Level Access

Sales reps should only see contacts assigned to them. Managers see all contacts.

```go
// internal/core/crm/policy.go
package crm

import (
    "context"
    "awo.so/awo/def"
    "awo.so/awo/filter"
    "awo.so/internal/platform/iam/session"
)

// ContactOwnerPolicy restricts contact visibility to the assigned sales rep.
// Managers and admins bypass the filter.
func ContactOwnerPolicy(ctx context.Context) def.Filter {
    actor := session.ActorFromContext(ctx)

    // Managers and admins see all contacts
    if actor.HasRole("role:crm.manager") || actor.HasRole("role:tenant.admin") {
        return filter.All()  // No additional filter — sees everything RBAC permits
    }

    // Sales reps see only their assigned contacts
    return filter.Eq("assigned_to", actor.UserID)
}
```

Register the policy on the definition:

```go
var ContactDefinition = def.EntityDefinition{
    // ... previous fields ...

    Policy: def.PolicyFunc(ContactOwnerPolicy),
}
```

---

## 3. How PolicyFunc Composes with RBAC

The execution order for a `GET /api/v1/entities/crm_contact` request:

1. **Session middleware**: validates session, resolves actor
2. **RBAC gate**: verifies actor has `role:crm.sales_rep` (or manager/admin) — if not, HTTP 403
3. **PolicyFunc injection**: `filter.Eq("assigned_to", actor.UserID)` appended to query filters
4. **Store query**: `SELECT ... WHERE tenant_id = $1 AND assigned_to = $2`
5. **RLS**: `WHERE tenant_id = current_tenant_id()` enforced by PostgreSQL regardless

RBAC gates the operation. PolicyFunc gates the rows. RLS is the backstop.

---

## 4. Testing the Policy

Policies are tested by calling the PolicyFunc directly — no database needed:

```go
// internal/core/crm/crm_test.go
func TestContactOwnerPolicy_SalesRep(t *testing.T) {
    repID := uuid.MustParse("018e0000-0000-7000-8000-000000000001")
    ctx := session.WithActor(context.Background(), session.Actor{
        UserID: repID,
        Roles:  []string{"role:crm.sales_rep"},
    })

    f := ContactOwnerPolicy(ctx)

    // Policy should restrict to assigned_to = repID
    expected := filter.Eq("assigned_to", repID)
    if !f.Equals(expected) {
        t.Errorf("expected owner filter, got %v", f)
    }
}

func TestContactOwnerPolicy_Manager(t *testing.T) {
    ctx := session.WithActor(context.Background(), session.Actor{
        UserID: uuid.New(),
        Roles:  []string{"role:crm.manager"},
    })

    f := ContactOwnerPolicy(ctx)

    // Manager should see all contacts
    if !f.IsAll() {
        t.Errorf("expected unrestricted filter for manager, got %v", f)
    }
}
```

---

## 5. Sensitive Field Policy

If some fields should be hidden from non-admin roles, use `SensitiveFieldMask`:

```go
Policy: def.ComposePolicy(
    ContactOwnerPolicy,
    def.SensitiveFieldMask([]string{"national_id", "salary"}, []string{
        "role:crm.manager",
        "role:tenant.admin",
    }),
),
```

`SensitiveFieldMask` removes listed fields from responses for actors without the specified roles, regardless of the field's `Sensitive` declaration. Use this for role-gated field visibility beyond the blanket `Sensitive: true` flag.

---

## 6. Declaring Module Roles

The roles referenced in `PermissionSet` (`role:crm.sales_rep`, `role:crm.manager`) must be seeded when the module is installed. Add them to the module's provisioning workflow (invoked by the Module Registry on tenant installation):

```go
// internal/core/crm/workflows/workflows.go
func ProvisionCRMModuleWorkflow(ctx workflow.Context, input ProvisionInput) error {
    ao := workflow.WithActivityOptions(ctx, defaultActivityOptions)

    // Seed CRM roles
    return workflow.ExecuteActivity(ctx, acts.SeedCRMRolesActivity, input).Get(ctx, nil)
}
```

```go
// Activity seeds roles in Casbin
func (a *CRMActivities) SeedCRMRolesActivity(ctx context.Context, input ProvisionInput) error {
    roles := []iam.RoleDefinition{
        {Name: "role:crm.sales_rep", Label: "CRM Sales Representative", TenantID: input.TenantID},
        {Name: "role:crm.manager",   Label: "CRM Manager",              TenantID: input.TenantID},
        {Name: "role:crm.viewer",    Label: "CRM Viewer (read-only)",   TenantID: input.TenantID},
    }
    return a.IAMService.SeedRoles(ctx, roles)
}
```

---

## Next: [Add Actions →](07-add-actions.md)
