> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Feature Flags Configuration"
id: cfg-002
status: accepted
category: GUIDE
stability: STABLE
audience: [operators, module-authors]
since: "1.0"
normative-level: normative
related:
  - "[Configuration](configuration.md)"
  - "[Platform Modules](../10-modules/platform-modules.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Feature Flags Configuration

**CFG-002 | Status: Accepted | Stability: Stable**

This document specifies how feature flags are declared, evaluated, cached, and managed. Feature flags control capability availability per tenant without code changes or redeployment.

---

## 1. Flag Declaration

Flags are declared in the module that owns the feature:

```go
// internal/core/finance/flags.go

var (
    FlagAdvancedAnalytics = def.FeatureFlag{
        Key:         "finance.advanced_analytics",
        Label:       "Advanced Analytics Dashboard",
        Description: "Enables the profitability analysis and forecasting charts.",
        Default:     false,  // off by default; must be enabled per tenant
    }

    FlagETIMS = def.FeatureFlag{
        Key:         "finance.etims_integration",
        Label:       "KRA eTIMS Integration",
        Description: "Automatically submits tax entries to KRA eTIMS API.",
        Default:     false,
    }
)

func init() {
    def.RegisterFlag(&FlagAdvancedAnalytics)
    def.RegisterFlag(&FlagETIMS)
}
```

Flag keys MUST follow `{module}.{feature}` format. Keys are stable — changing a key after deployment loses the stored override values.

---

## 2. Evaluation Order

Flags evaluate in priority order (highest wins):

1. **User override** — per-user toggle (rare; used for beta testers)
2. **Tenant override** — per-tenant toggle (most common)
3. **System default** — from `FeatureFlag.Default` declaration

```go
// Middleware evaluates flag for current actor and tenant
func FlagEnabled(ctx context.Context, flagKey string) bool {
    actor := session.ActorFromContext(ctx)
    tenantID := tenant.IDFromContext(ctx)

    // Cached in Redis: eval:{sha256(flagKey+tenantID+userID)}
    // TTL: 5 minutes
    return flags.Evaluate(ctx, flagKey, tenantID, actor.UserID)
}
```

---

## 3. Setting Flag Values

### Via Settings Module API

```
POST /api/v1/entities/platform_setting
Authorization: Bearer {admin_session_token}
Body: {
  "key":   "finance.advanced_analytics",
  "value": "true",
  "scope": "tenant"
}
```

### Via Admin UI

Settings module provides a flag management UI for tenant admins. Platform admins can set system-wide defaults.

### Via Provisioning

At tenant provisioning, flags can be set based on the tenant's plan:

```go
// internal/platform/tenant/provisioning.go

func ProvisionTenant(ctx context.Context, input ProvisionInput) error {
    // ... create tenant ...

    // Enable features based on plan
    if input.Plan == "Enterprise" {
        err := flagService.SetTenantFlag(ctx, tenantID, "finance.advanced_analytics", true)
        if err != nil { return err }
        err = flagService.SetTenantFlag(ctx, tenantID, "finance.etims_integration", true)
        if err != nil { return err }
    }

    return nil
}
```

---

## 4. Using Flags in Code

### In Route Handlers

```go
func AnalyticsDashboardHandler(c *fiber.Ctx) error {
    if !flags.FlagEnabled(c.Context(), "finance.advanced_analytics") {
        return c.Status(404).JSON(ErrorEnvelope{Error: ErrorBody{
            Code:    "feature_not_available",
            Message: "Advanced analytics is not enabled for your account.",
        }})
    }
    // ... serve dashboard
}
```

### In Page Builders (SDUI)

```go
func buildAnalyticsSection(psc *sdui.PageSchemaContext) amis.Component {
    if !psc.FeatureFlagEnabled("finance.advanced_analytics") {
        return amis.Empty()  // absent from schema — not just hidden
    }
    return buildAnalyticsChart(psc)
}
```

### In Hooks

```go
func (h *ETIMSHook) AfterCreate(ctx context.Context, record *entity.EntityRecord) error {
    if !flags.FlagEnabled(ctx, "finance.etims_integration") {
        return nil  // skip eTIMS submission if flag is off
    }
    return h.ETIMSService.Submit(ctx, record)
}
```

---

## 5. Cache Invalidation

Flag evaluation results are cached in Redis for 5 minutes. Changes take effect within 5 minutes.

For immediate invalidation (e.g., emergency disable):

```
POST /api/v1/admin/flags/invalidate
Body: {"key": "finance.etims_integration", "scope": "tenant", "tenant_id": "..."}
```

This deletes the affected Redis eval key immediately. Next request evaluates from DB.

---

## 6. Flag Lifecycle

| Stage | Description |
|---|---|
| **Experimental** | Default: off. Only enabled for specific tenants via override. |
| **Beta** | Default: off. Available to self-service enable in tenant admin settings. |
| **GA** | Default: on. Tenants can disable via override (opt-out). |
| **Deprecated** | Default: on. Override to disable no longer available. Will be removed. |
| **Removed** | Code path deleted. Flag key no longer registered. |

Flag lifecycle changes are communicated via changelog and documented in ADRs for GA transitions.

---

## 7. Standard Platform Flags

| Flag Key | Default | Description |
|---|---|---|
| `platform.tenant_id_query_param` | true | Allow `?tenant_id=` in requests (disable in prod) |
| `platform.iam_audit_log_enabled` | true | Record audit log entries (cannot be disabled) |
| `iam.mfa_required` | false | Require MFA for all tenant users |
| `iam.mfa_email_otp_enabled` | true | Allow email OTP as MFA method |
| `iam.session_ttl_seconds` | 28800 | Session TTL in seconds |
| `finance.etims_integration` | false | KRA eTIMS submission |
| `finance.advanced_analytics` | false | Advanced analytics dashboard |
| `finance.multi_currency` | false | Multi-currency invoicing |

---

## Related Documents

- [Configuration](configuration.md) — environment-based configuration
- [Platform Modules](../10-modules/platform-modules.md#3-feature-flags-module) — flags module internals
- [SDUI Form Patterns](../08-sdui/form-patterns.md) — `psc.FeatureFlagEnabled` in page builders
- [Glossary](../GLOSSARY.md) — Feature Flag, Evaluation Order
