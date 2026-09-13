> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Settings Module Patterns"
id: mod-007
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Platform Modules](platform-modules.md)"
  - "[Feature Flags Configuration](../12-configuration/feature-flags-config.md)"
  - "[Tenant Provisioning](../06-tenancy/tenant-provisioning.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Settings Module Patterns

**MOD-007 | Status: Accepted | Stability: Stable**

Patterns for declaring, reading, and overriding tenant-configurable settings in business modules.

---

## 1. Settings vs Feature Flags vs Configuration

| Concern | Mechanism | Changed by | Scope |
|---|---|---|---|
| Process-level settings (DB URL, port) | Environment variables | Operator | Deployment |
| On/off capability switches | Feature flags | Tenant admin / platform admin | Per-tenant |
| Numeric/string behavioral parameters | Settings module | Tenant admin | Per-tenant (overrides system default) |

Use the Settings module for parameters that:
- Have a system-wide default
- Can be overridden per tenant or per branch
- Are not boolean on/off (those belong in feature flags)

---

## 2. Declaring a Setting

Settings are declared in the module that owns them:

```go
// internal/core/finance/settings.go

var (
    SettingInvoiceApprovalThreshold = def.Setting{
        Key:         "finance.invoice_approval_threshold",
        Label:       "Invoice Approval Threshold (KES)",
        Description: "Invoices above this amount require manager approval before submission.",
        Type:        def.SettingTypeCurrency,
        Default:     "50000.0000",
        Scope:       def.SettingScopeTenant,
    }

    SettingPaymentTermsDays = def.Setting{
        Key:         "finance.payment_terms_days",
        Label:       "Default Payment Terms (days)",
        Type:        def.SettingTypeInt,
        Default:     "30",
        Scope:       def.SettingScopeTenant,
    }

    SettingDefaultCurrency = def.Setting{
        Key:         "finance.default_currency",
        Label:       "Default Currency",
        Type:        def.SettingTypeSelect,
        Options:     []string{"KES", "USD", "EUR", "GBP"},
        Default:     "KES",
        Scope:       def.SettingScopeTenant,
    }
)

func init() {
    def.RegisterSetting(&SettingInvoiceApprovalThreshold)
    def.RegisterSetting(&SettingPaymentTermsDays)
    def.RegisterSetting(&SettingDefaultCurrency)
}
```

---

## 3. Reading Settings in Hooks

```go
// In a BeforeCreate hook on finance_invoice
func (h *InvoiceApprovalGuard) BeforeCreate(ctx context.Context, record *entity.EntityRecord) error {
    amount, _ := record.Fields["total_kes"].(decimal.Decimal)

    // Read tenant's approval threshold setting
    threshold, err := h.Settings.GetDecimal(ctx, "finance.invoice_approval_threshold")
    if err != nil {
        return fmt.Errorf("InvoiceApprovalGuard: read threshold setting: %w", err)
    }

    if amount.GreaterThan(threshold) {
        // Flag for approval workflow — don't reject, just mark as requiring approval
        record.Fields["requires_approval"] = true
    }

    return nil
}
```

`h.Settings` is the Settings service — injected via wire at startup.

### Settings Service Interface

```go
type SettingsService interface {
    Get(ctx context.Context, key string) (string, error)
    GetInt(ctx context.Context, key string) (int64, error)
    GetDecimal(ctx context.Context, key string) (decimal.Decimal, error)
    GetBool(ctx context.Context, key string) (bool, error)
}
```

`Get` returns the tenant's override if set, otherwise the system default. Module code does not distinguish between them.

---

## 4. Setting Hierarchy

Settings follow a hierarchical evaluation order (highest priority wins):

```
Branch override  →  Tenant override  →  System default
```

Branch overrides are for multi-branch tenants (e.g., a chain with different invoice thresholds per branch).

```go
// Branch-specific override (if branch context is in ctx)
threshold, _ := settings.GetDecimalForBranch(ctx, "finance.invoice_approval_threshold", branchID)
```

---

## 5. Reading Settings in SDUI

Settings can surface in page schemas for display or as form defaults:

```go
func BuildInvoiceCreateForm(ctx context.Context, actor session.Actor) ([]byte, error) {
    psc := sdui.NewPageSchemaContext(ctx, actor)

    // Read default payment terms to pre-populate the form
    defaultTerms, _ := psc.SettingInt("finance.payment_terms_days")

    form := amis.Form(amis.FormProps{
        API: "POST /api/v1/entities/finance_invoice",
        Fields: []amis.FormField{
            // ...
            amis.IntField("payment_terms_days", "Payment Terms (days)").
                DefaultValue(defaultTerms),
        },
    })

    return psc.Marshal(form)
}
```

---

## 6. Tenant Admin Settings UI

The Settings module auto-generates an admin UI listing all registered settings for the tenant:

```
/settings → Lists all settings grouped by module
/settings/finance → Finance module settings
```

Settings with `Scope: SettingScopePlatform` are only visible to platform admins.
Settings with `Scope: SettingScopeTenant` are visible and editable by tenant admins.

---

## 7. NamingSeries Prefix as a Setting

NamingSeries fields with `TenantOverridable: true` automatically register a setting:

```go
{
    Name:              "number",
    Type:              def.FieldNamingSeries,
    Series:            "INV-{YYYY}-{SEQ:5}",
    TenantOverridable: true,
}
// Registers setting: finance_invoice.number.series_prefix (default: "INV")
```

Tenant admin can change the prefix from "INV" to "ACME-INV" via the settings UI. The framework handles prefix application automatically.

---

## 8. Anti-Patterns

### Hard-Coding Thresholds in Hook Logic

```go
// WRONG: threshold hard-coded
if amount.GreaterThan(decimal.NewFromFloat(50000)) { ... }

// CORRECT: read from settings
threshold, _ := h.Settings.GetDecimal(ctx, "finance.invoice_approval_threshold")
if amount.GreaterThan(threshold) { ... }
```

### Reading Settings in a Tight Loop

```go
// WRONG: reads DB on every iteration
for _, invoice := range invoices {
    threshold, _ := settings.GetDecimal(ctx, "finance.invoice_approval_threshold")
    // ...
}

// CORRECT: read once before loop
threshold, _ := settings.GetDecimal(ctx, "finance.invoice_approval_threshold")
for _, invoice := range invoices {
    // use threshold
}
```

The Settings service caches in Redis (5-minute TTL), but looping DB calls still add latency. Read once, use many times.

---

## Related Documents

- [Platform Modules](platform-modules.md) — §5 Settings module internals
- [Feature Flags Configuration](../12-configuration/feature-flags-config.md) — for boolean switches
- [Tenant Provisioning](../06-tenancy/tenant-provisioning.md) — default settings seeded at provisioning
- [Naming Series](../04-domain/naming-series.md) — `TenantOverridable` setting integration
