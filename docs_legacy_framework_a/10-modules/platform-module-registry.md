> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Module Registry Platform Module"
id: mod-015
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors, operators]
since: "1.0"
normative-level: normative
related:
  - "[Module System](module-system.md)"
  - "[Platform Modules](platform-modules.md)"
  - "[Feature Flags Module](platform-flags-module.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Module Registry Platform Module

**MOD-015 | Status: Accepted | Stability: Stable**

The Module Registry tracks which business modules are installed and activated per tenant. It controls module provisioning, tenant-level activation, and dependency enforcement.

---

## 1. Entities

### `platform_installed_module`

Records that a module binary exists in this deployment — set at boot time, not by tenants.

```go
var InstalledModuleDefinition = def.SystemDefinition{
    Name:        "platform_installed_module",
    Module:      "platform",
    Label:       "Installed Module",
    LabelPlural: "Installed Modules",
    Fields: []def.FieldDef{
        {Name: "module_name",    Type: def.FieldData,   Required: true, Immutable: true, Unique: true},
        {Name: "display_name",   Type: def.FieldData,   Required: true},
        {Name: "version",        Type: def.FieldData,   Required: true},
        {Name: "description",    Type: def.FieldSmallText},
        {Name: "dependencies",   Type: def.FieldJSON},  // []string of module_name
        {Name: "registered_at",  Type: def.FieldDateTime, Required: true},
    },
    Permissions: def.PermissionSet{
        Read:   []string{"role:platform-admin", "role:tenant.admin"},
        Create: []string{"role:platform-admin"},
        Write:  []string{"role:platform-admin"},
        Delete: []string{"role:platform-admin"},
    },
}
```

### `tenant_module_activation`

Records that a specific tenant has activated a module.

```go
var TenantModuleActivationDefinition = def.SystemDefinition{
    Name:        "tenant_module_activation",
    Module:      "platform",
    Label:       "Module Activation",
    LabelPlural: "Module Activations",
    Fields: []def.FieldDef{
        {Name: "module_name",    Type: def.FieldData,   Required: true, Immutable: true},
        {Name: "status",         Type: def.FieldSelect, Required: true,
            Options: []string{"active", "suspended", "deactivating"},
            Default: "active"},
        {Name: "activated_at",   Type: def.FieldDateTime, Required: true},
        {Name: "activated_by",   Type: def.FieldLink,   LinkTarget: "iam_user"},
        {Name: "deactivated_at", Type: def.FieldDateTime},
        {Name: "config",         Type: def.FieldJSON},  // module-specific activation config
    },
    Permissions: def.PermissionSet{
        Read:   []string{"role:tenant.admin"},
        Create: []string{"role:platform-admin"},
        Write:  []string{"role:platform-admin"},
        Delete: []string{"role:platform-admin"},
    },
}
```

---

## 2. Module Registration at Boot

Every business module calls `registry.RegisterModule` in its `init()`:

```go
// internal/core/finance/finance.go
func init() {
    registry.RegisterModule(registry.ModuleManifest{
        Name:         "finance",
        DisplayName:  "Finance",
        Version:      "1.0.0",
        Description:  "Double-entry accounting, invoicing, payments",
        Dependencies: []string{"crm"}, // crm must be active before finance can activate
    })
    def.Register(&InvoiceDefinition)
    def.Register(&JournalEntryDefinition)
    // ...
}
```

The registry upserts a `platform_installed_module` record at startup using the migration role (not the app role). This is idempotent — restart-safe.

---

## 3. Tenant Module Activation

### Activating a Module

Platform admin activates a module for a tenant. This triggers `ActivateModuleWorkflow`:

```go
// POST /api/v1/platform/tenants/{tenant-id}/modules/{module-name}/activate
func ActivateModuleAction(ctx context.Context, action def.ActionContext) (*def.ActionResult, error) {
    moduleName := action.Params["module_name"]

    // 1. Verify module is installed
    installed, err := action.Repo.Exists(ctx,
        filter.Eq("module_name", moduleName))
    if !installed {
        return nil, &errors.BusinessError{
            Code: "module.not_installed", Message: "Module not found in this deployment", Status: 404}
    }

    // 2. Check dependencies are already active for this tenant
    manifest := registry.GetManifest(moduleName)
    for _, dep := range manifest.Dependencies {
        active, _ := action.ActivationRepo.Exists(ctx,
            filter.And(filter.Eq("module_name", dep), filter.Eq("status", "active")))
        if !active {
            return nil, &errors.BusinessError{
                Code:    "module.dependency_not_active",
                Message: fmt.Sprintf("Dependency %q must be activated first", dep),
                Status:  422,
            }
        }
    }

    // 3. Start activation workflow (provisions settings, seeds data, runs module migrations)
    wfID := fmt.Sprintf("%s.module.%s.activate", action.Actor.TenantID, moduleName)
    _, err = action.TemporalClient.ExecuteWorkflow(ctx,
        client.StartWorkflowOptions{ID: wfID, TaskQueue: "platform.module"},
        "ActivateModuleWorkflow",
        ActivateModuleInput{TenantID: action.Actor.TenantID, ModuleName: moduleName},
    )
    if err != nil {
        return nil, fmt.Errorf("ActivateModuleAction: start workflow: %w", err)
    }

    return &def.ActionResult{
        Message:    "Module activation started",
        WorkflowID: wfID,
    }, nil
}
```

### Activation Workflow

```go
func ActivateModuleWorkflow(ctx workflow.Context, input ActivateModuleInput) error {
    saga := registry.NewSagaCompensator()

    // 1. Create activation record
    var activationID uuid.UUID
    if err := workflow.ExecuteActivity(ctx, a.CreateActivationRecordActivity, input).Get(ctx, &activationID); err != nil {
        return err
    }
    saga.Add(func(ctx workflow.Context) error {
        return workflow.ExecuteActivity(ctx, a.DeleteActivationRecordActivity, activationID).Get(ctx, nil)
    })

    // 2. Provision module-specific settings defaults
    if err := workflow.ExecuteActivity(ctx, a.ProvisionModuleSettingsActivity, input).Get(ctx, nil); err != nil {
        saga.Compensate(ctx)
        return err
    }

    // 3. Seed module reference data for tenant
    if err := workflow.ExecuteActivity(ctx, a.SeedModuleDataActivity, input).Get(ctx, nil); err != nil {
        saga.Compensate(ctx)
        return err
    }

    // 4. Invalidate module cache for tenant
    return workflow.ExecuteActivity(ctx, a.InvalidateModuleCacheActivity, input).Get(ctx, nil)
}
```

---

## 4. Runtime Module Check

Middleware and hooks check module activation before allowing access to module-specific routes:

```go
// ModuleRequired middleware — used on all finance routes
func ModuleRequired(moduleName string) fiber.Handler {
    return func(c *fiber.Ctx) error {
        tenantID := session.TenantIDFromContext(c.UserContext())
        active, err := moduleRegistry.IsActive(c.UserContext(), tenantID, moduleName)
        if err != nil {
            return mapError(c, err)
        }
        if !active {
            return c.Status(402).JSON(fiber.Map{
                "error": fiber.Map{
                    "code":    "module.not_activated",
                    "message": "This feature requires the " + moduleName + " module",
                },
            })
        }
        return c.Next()
    }
}
```

Module status is Redis-cached with a 5-minute TTL per tenant, keyed as `module:active:{tenant_id}:{module_name}`.

---

## 5. Module Status API

```
GET  /api/v1/platform/modules                              → list all installed modules
GET  /api/v1/platform/tenants/{id}/modules                 → list activations for tenant
POST /api/v1/platform/tenants/{id}/modules/{name}/activate → start activation
POST /api/v1/platform/tenants/{id}/modules/{name}/suspend  → suspend (retain data)
POST /api/v1/platform/tenants/{id}/modules/{name}/deactivate → begin deactivation (data warning)
```

Deactivation is a two-step process: `suspend` (blocks new data, existing data read-only), then `deactivate` after confirmation. Hard delete of tenant module data is never automatic — requires explicit data export or purge confirmation.

---

## 6. Dependency Graph Example

```
platform (always active)
    └── iam (always active)
    └── audit (always active)
    └── metadata (always active)
    └── flags (always active)
    └── settings (always active)

crm
    └── finance
        └── payroll
        └── projects

inventory
    └── forecourt
        └── payroll (for payroll of shift workers)
```

A tenant activating `payroll` requires both `finance` and either `crm` or `inventory` to be active, depending on the employee source.

---

## Related Documents

- [Module System](module-system.md) — module manifest structure
- [Platform Modules](platform-modules.md) — all built-in platform modules
- [Feature Flags Module](platform-flags-module.md) — per-tenant feature gating
- [Tenant Model](../06-tenancy/tenant-model.md) — tenant lifecycle
