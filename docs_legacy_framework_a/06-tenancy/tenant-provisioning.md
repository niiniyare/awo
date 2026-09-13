> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Tenant Provisioning"
id: ten-005
status: accepted
category: SPEC
stability: STABLE
audience: [operators, module-authors]
since: "1.0"
normative-level: normative
related:
  - "[Tenant Lifecycle](tenant-lifecycle.md)"
  - "[Tenant Model](tenant-model.md)"
  - "[IAM](../07-iam/README.md)"
  - "[Module System](../10-modules/module-system.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Tenant Provisioning

**TEN-005 | Status: Accepted | Stability: Stable**

This document specifies the tenant provisioning workflow: what happens when a new tenant is created, the provisioning steps, default data seeding, and module activation.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Provisioning Overview

Tenant provisioning transitions a tenant from `PENDING` to `ACTIVE`. It is implemented as a Temporal workflow to ensure durability — if provisioning fails partway through, it can be retried from the failed step.

```
Tenant record created (PENDING)
    ↓ (TenantProvisioningWorkflow starts)
Seed system roles (tenant.admin, tenant.user, api-client)
    ↓
Seed default settings
    ↓
Activate registered modules (per plan)
    ↓
Create initial admin user
    ↓
Send welcome email
    ↓
Set tenant status → ACTIVE
```

---

## 2. TenantProvisioningWorkflow

```go
// internal/platform/tenant/workflows/provisioning.go

type ProvisioningInput struct {
    TenantID    uuid.UUID
    Plan        string
    AdminEmail  string
    AdminName   string
}

func TenantProvisioningWorkflow(ctx workflow.Context, input ProvisioningInput) error {
    ao := workflow.ActivityOptions{
        StartToCloseTimeout: 60 * time.Second,
        RetryPolicy: &temporal.RetryPolicy{MaxAttempts: 5},
    }
    ctx = workflow.WithActivityOptions(ctx, ao)

    var activities *ProvisioningActivities

    // 1. Seed system roles
    err := workflow.ExecuteActivity(ctx, activities.SeedSystemRolesActivity, input.TenantID).Get(ctx, nil)
    if err != nil {
        return fmt.Errorf("TenantProvisioningWorkflow: seed roles: %w", err)
    }

    // 2. Seed default settings
    err = workflow.ExecuteActivity(ctx, activities.SeedDefaultSettingsActivity, SeedSettingsInput{
        TenantID: input.TenantID,
        Plan:     input.Plan,
    }).Get(ctx, nil)
    if err != nil {
        return fmt.Errorf("TenantProvisioningWorkflow: seed settings: %w", err)
    }

    // 3. Activate modules for plan
    err = workflow.ExecuteActivity(ctx, activities.ActivateModulesActivity, ActivateModulesInput{
        TenantID: input.TenantID,
        Plan:     input.Plan,
    }).Get(ctx, nil)
    if err != nil {
        return fmt.Errorf("TenantProvisioningWorkflow: activate modules: %w", err)
    }

    // 4. Create admin user
    var adminUserID uuid.UUID
    err = workflow.ExecuteActivity(ctx, activities.CreateAdminUserActivity, CreateAdminInput{
        TenantID: input.TenantID,
        Email:    input.AdminEmail,
        Name:     input.AdminName,
    }).Get(ctx, &adminUserID)
    if err != nil {
        return fmt.Errorf("TenantProvisioningWorkflow: create admin user: %w", err)
    }

    // 5. Send welcome email (non-fatal)
    _ = workflow.ExecuteActivity(ctx, activities.SendWelcomeEmailActivity, WelcomeEmailInput{
        TenantID: input.TenantID,
        UserID:   adminUserID,
        Email:    input.AdminEmail,
        Name:     input.AdminName,
    }).Get(ctx, nil)

    // 6. Activate tenant
    err = workflow.ExecuteActivity(ctx, activities.ActivateTenantActivity, input.TenantID).Get(ctx, nil)
    if err != nil {
        return fmt.Errorf("TenantProvisioningWorkflow: activate tenant: %w", err)
    }

    return nil
}
```

---

## 3. System Role Seeding

Each tenant receives three built-in roles that MUST exist before any user is created:

```go
func (a *ProvisioningActivities) SeedSystemRolesActivity(ctx context.Context, tenantID uuid.UUID) error {
    tenantCtx, err := a.TenantStore.SetTenantContext(ctx, tenantID)
    if err != nil { return err }

    roles := []struct{ name, description string }{
        {"role:tenant.admin", "Full access within tenant. Cannot modify platform configuration."},
        {"role:tenant.user",  "Standard user. Permissions customized per tenant."},
        {"role:api-client",   "Machine-to-machine. Limited to declared scopes."},
    }

    for _, role := range roles {
        // Idempotent: skip if already exists
        exists, err := a.RoleRepo.Exists(tenantCtx, filter.Eq("name", role.name))
        if err != nil { return err }
        if exists { continue }

        _, err = a.RoleRepo.Create(tenantCtx, entity.CreateInput{
            Fields: map[string]any{
                "name":        role.name,
                "description": role.description,
                "system_role": true,  // cannot be deleted
            },
        })
        if err != nil {
            return fmt.Errorf("SeedSystemRolesActivity: create role %s: %w", role.name, err)
        }
    }
    return nil
}
```

System roles (`system_role: true`) are protected from deletion at the API and hook layer.

---

## 4. Default Settings Seeding

Default settings are seeded from a plan-based template:

```go
func (a *ProvisioningActivities) SeedDefaultSettingsActivity(ctx context.Context, input SeedSettingsInput) error {
    tenantCtx, err := a.TenantStore.SetTenantContext(ctx, input.TenantID)
    if err != nil { return err }

    defaults := planDefaults(input.Plan)  // map[string]string

    for key, value := range defaults {
        exists, err := a.SettingsRepo.Exists(tenantCtx, filter.Eq("key", key))
        if err != nil { return err }
        if exists { continue }  // idempotent

        _, err = a.SettingsRepo.Create(tenantCtx, entity.CreateInput{
            Fields: map[string]any{"key": key, "value": value, "scope": "tenant"},
        })
        if err != nil {
            return fmt.Errorf("SeedDefaultSettingsActivity: set %s: %w", key, err)
        }
    }
    return nil
}

func planDefaults(plan string) map[string]string {
    base := map[string]string{
        "iam.session_ttl_seconds":    "28800",
        "iam.password_min_length":    "8",
        "iam.mfa_required":           "false",
        "finance.default_currency":   "KES",
        "platform.timezone":          "Africa/Nairobi",
    }
    if plan == "Enterprise" {
        base["finance.etims_integration"]    = "true"
        base["finance.advanced_analytics"]   = "true"
        base["finance.multi_currency"]       = "true"
        base["iam.mfa_required"]             = "true"
    }
    return base
}
```

---

## 5. Admin User Creation

The initial admin user is created with a temporary password that requires change on first login:

```go
func (a *ProvisioningActivities) CreateAdminUserActivity(ctx context.Context, input CreateAdminInput) (uuid.UUID, error) {
    tenantCtx, err := a.TenantStore.SetTenantContext(ctx, input.TenantID)
    if err != nil { return uuid.Nil, err }

    // Idempotency: return existing admin user ID if already created
    users, _, err := a.UserRepo.Query(tenantCtx, filter.And(
        filter.Eq("email", input.Email),
        filter.Eq("role", "role:tenant.admin"),
    ))
    if err != nil { return uuid.Nil, err }
    if len(users) > 0 { return users[0].ID, nil }

    // Generate temporary password
    tempPassword, err := generateTempPassword(16)
    if err != nil { return uuid.Nil, err }

    hash, err := bcrypt.GenerateFromPassword([]byte(tempPassword), 12)
    if err != nil { return uuid.Nil, err }

    user, err := a.UserRepo.Create(tenantCtx, entity.CreateInput{
        Fields: map[string]any{
            "email":                    input.Email,
            "name":                     input.Name,
            "password_hash":            string(hash),
            "role":                     "role:tenant.admin",
            "requires_password_change": true,
            "status":                   "Active",
        },
    })
    if err != nil {
        return uuid.Nil, fmt.Errorf("CreateAdminUserActivity: %w", err)
    }

    // Store temp password for welcome email
    // (stored temporarily in Redis — TTL 24 hours — not in DB)
    err = a.Redis.Set(ctx,
        fmt.Sprintf("temp_password:%s", user.ID),
        tempPassword,
        24*time.Hour,
    ).Err()

    return user.ID, err
}
```

The temporary password is passed to the welcome email via Redis (not stored in DB). After the welcome email is sent, the Redis key is deleted.

---

## 6. Module Activation

Modules are activated per tenant based on their subscription plan:

```go
func (a *ProvisioningActivities) ActivateModulesActivity(ctx context.Context, input ActivateModulesInput) error {
    tenantCtx, err := a.TenantStore.SetTenantContext(ctx, input.TenantID)
    if err != nil { return err }

    modulesForPlan := map[string][]string{
        "Starter":    {"crm", "finance"},
        "Growth":     {"crm", "finance", "inventory", "hr"},
        "Enterprise": {"crm", "finance", "inventory", "hr", "payroll", "forecourt", "projects"},
    }

    for _, module := range modulesForPlan[input.Plan] {
        exists, err := a.ModuleRegistryRepo.Exists(tenantCtx, filter.Eq("module_key", module))
        if err != nil { return err }
        if exists { continue }

        _, err = a.ModuleRegistryRepo.Create(tenantCtx, entity.CreateInput{
            Fields: map[string]any{
                "module_key": module,
                "status":     "Active",
                "activated_at": time.Now().UTC(),
            },
        })
        if err != nil {
            return fmt.Errorf("ActivateModulesActivity: activate %s: %w", module, err)
        }
    }
    return nil
}
```

---

## 7. Provisioning Failure Recovery

If provisioning fails partway through, the workflow retries from the failed activity (Temporal auto-retry). Activities are idempotent — re-running them is safe.

To manually retry a stuck provisioning workflow:

```bash
# Check provisioning workflow status
temporal workflow describe --workflow-id "{tenant-id}.tenant.{tenant-id}.provision"

# If stuck, signal to resume or terminate and restart
temporal workflow terminate --workflow-id "..." --reason "Manual restart"

# Then re-trigger provisioning via API
POST /api/v1/admin/tenants/{id}/reprovision
```

---

## Related Documents

- [Tenant Lifecycle](tenant-lifecycle.md) — PENDING → ACTIVE state transition
- [IAM Authentication](../07-iam/authentication.md) — temporary password and requires_password_change flag
- [Module System](../10-modules/module-system.md) — module registry
- [Scheduled Workflows](../09-workflow/scheduled-workflows.md) — background job tenant context
