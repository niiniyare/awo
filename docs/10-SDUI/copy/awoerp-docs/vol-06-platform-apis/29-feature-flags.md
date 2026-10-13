# Chapter 29 — Feature Flags

> **Volume:** VI — Platform APIs & Configuration
> **Phase:** 3
> **Audience:** Backend Engineers, Platform Engineers, Product Engineers
> **Prerequisites:** Chapters 08, 22, 25, 28

---

## Table of Contents

- [29.1 Feature Flag Philosophy](#291-feature-flag-philosophy)
- [29.2 Flag Types](#292-flag-types)
- [29.3 Flag Declaration and Storage](#293-flag-declaration-and-storage)
- [29.4 Flag Evaluation Engine](#294-flag-evaluation-engine)
- [29.5 Targeting Rules](#295-targeting-rules)
- [29.6 Gradual Rollout](#296-gradual-rollout)
- [29.7 Flags in SDUI Compilation](#297-flags-in-sdui-compilation)
- [29.8 Client-Side Flag Binding](#298-client-side-flag-binding)
- [29.9 Flag Lifecycle](#299-flag-lifecycle)

---

## 29.1 Feature Flag Philosophy

Feature flags in AwoERP serve three distinct purposes:

**Feature gating.** New product features are developed behind flags and enabled progressively — first to internal users, then to beta tenants, then to general availability. This decouples deployment from release.

**Tenant entitlements.** Plan-based capabilities are implemented as flags. A tenant on the `enterprise` plan has `advanced_reporting: true`; a tenant on `starter` has `advanced_reporting: false`. This avoids separate code paths for different plans.

**Operational controls.** Flags enable/disable specific behaviors for incident response — disabling a data export feature when a downstream service is degraded, or enabling verbose logging for a specific tenant for debugging.

**Flags are not A/B tests.** AwoERP does not use flags for UI experiments. All users on the same tenant see the same UI. Flags change capability, not presentation.

---

## 29.2 Flag Types

| Flag Type | Value Type | Use Case |
|-----------|-----------|---------|
| `boolean` | `true` / `false` | Feature on/off |
| `string` | Arbitrary string | Variant selection, config value |
| `number` | Integer or float | Limits, thresholds, percentages |
| `json` | JSON object | Complex configuration |

Most flags are `boolean`. Non-boolean flags are used for tenant-specific configuration that does not warrant a dedicated settings field.

---

## 29.3 Flag Declaration and Storage

### 29.3.1 Flag Registry

All flags are declared in the platform's flag registry before they can be used in surface definitions or application code. The registry is the source of truth for:
- Flag metadata (name, description, type, default value)
- Which surface definitions reference the flag
- Current rollout status

```sql
CREATE TABLE feature_flags (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key         TEXT NOT NULL UNIQUE,   -- e.g. 'advanced_reporting'
    name        TEXT NOT NULL,
    description TEXT NOT NULL,
    flag_type   TEXT NOT NULL,          -- boolean | string | number | json
    default_value JSONB NOT NULL,
    status      TEXT NOT NULL DEFAULT 'active',  -- active | deprecated | archived
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Per-tenant flag overrides
CREATE TABLE tenant_flag_overrides (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    flag_key    TEXT NOT NULL REFERENCES feature_flags(key),
    value       JSONB NOT NULL,
    enabled_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ,            -- NULL = permanent
    UNIQUE(tenant_id, flag_key)
);

-- Per-user flag overrides (for beta users within a tenant)
CREATE TABLE user_flag_overrides (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL,
    user_id     UUID NOT NULL,
    flag_key    TEXT NOT NULL REFERENCES feature_flags(key),
    value       JSONB NOT NULL,
    enabled_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ,
    UNIQUE(tenant_id, user_id, flag_key)
);
```

### 29.3.2 Flag Definition Example

```json
{
  "key": "advanced_reporting",
  "name": "Advanced Reporting",
  "description": "Enables the advanced reporting module with custom report builder and scheduled exports",
  "flag_type": "boolean",
  "default_value": false,
  "status": "active"
}
```

---

## 29.4 Flag Evaluation Engine

### 29.4.1 Evaluation Order

Flag values are resolved using the following precedence order (highest to lowest):

```
1. User override (if set for this user in this tenant)
2. Tenant override (if set for this tenant)
3. Rollout rule (if a targeting rule matches)
4. Default value (from flag registry)
```

### 29.4.2 Go Evaluation Function

```go
type FlagEvaluator struct {
    store FlagStore
    cache *redis.Client
}

type EvaluationContext struct {
    TenantID string
    UserID   string
    Roles    []string
    Plan     string   // starter | growth | enterprise
}

func (e *FlagEvaluator) Evaluate(ctx context.Context,
    flagKey string,
    evalCtx EvaluationContext) (FlagValue, error) {

    cacheKey := fmt.Sprintf("flag:%s:tenant:%s:user:%s",
        flagKey, evalCtx.TenantID, evalCtx.UserID)

    // Check cache
    if cached, err := e.cache.Get(ctx, cacheKey).Result(); err == nil {
        var v FlagValue
        json.Unmarshal([]byte(cached), &v)
        return v, nil
    }

    // 1. User override
    if override, err := e.store.GetUserOverride(ctx, evalCtx.TenantID, evalCtx.UserID, flagKey); err == nil {
        return e.cacheAndReturn(ctx, cacheKey, override.Value)
    }

    // 2. Tenant override
    if override, err := e.store.GetTenantOverride(ctx, evalCtx.TenantID, flagKey); err == nil {
        if override.ExpiresAt == nil || override.ExpiresAt.After(time.Now()) {
            return e.cacheAndReturn(ctx, cacheKey, override.Value)
        }
    }

    // 3. Rollout rules
    flag, err := e.store.GetFlag(ctx, flagKey)
    if err != nil {
        return FlagValue{}, err
    }

    for _, rule := range flag.RolloutRules {
        if e.evaluateRule(rule, evalCtx) {
            return e.cacheAndReturn(ctx, cacheKey, rule.Value)
        }
    }

    // 4. Default
    return e.cacheAndReturn(ctx, cacheKey, flag.DefaultValue)
}
```

### 29.4.3 Bulk Evaluation for Session Context

Rather than evaluating flags one at a time, the session context endpoint evaluates all active flags for the user in a single call:

```go
func (e *FlagEvaluator) EvaluateAll(ctx context.Context,
    evalCtx EvaluationContext) (map[string]FlagValue, error) {

    flags, err := e.store.ListActiveFlags(ctx)
    if err != nil {
        return nil, err
    }

    result := make(map[string]FlagValue, len(flags))
    for _, flag := range flags {
        val, err := e.Evaluate(ctx, flag.Key, evalCtx)
        if err != nil {
            // Non-fatal: use default value
            val = FlagValue{Value: flag.DefaultValue}
        }
        result[flag.Key] = val
    }
    return result, nil
}
```

---

## 29.5 Targeting Rules

Targeting rules allow flags to be enabled for specific subsets of users without setting individual overrides.

### 29.5.1 Rule Schema

```go
type RolloutRule struct {
    ID         string      `json:"id"`
    Conditions []Condition `json:"conditions"`
    Operator   string      `json:"operator"`   // "and" | "or"
    Value      FlagValue   `json:"value"`
    Priority   int         `json:"priority"`   // Higher priority rules evaluated first
}

type Condition struct {
    Attribute string      `json:"attribute"` // "plan" | "role" | "tenant_id" | "user_id"
    Operator  string      `json:"operator"`  // "eq" | "in" | "not_in" | "contains"
    Value     interface{} `json:"value"`
}
```

### 29.5.2 Example Targeting Rules

**Enable for enterprise plan tenants:**
```json
{
  "id": "rule_enterprise_only",
  "conditions": [
    { "attribute": "plan", "operator": "eq", "value": "enterprise" }
  ],
  "operator": "and",
  "value": true,
  "priority": 10
}
```

**Enable for specific beta tenants:**
```json
{
  "id": "rule_beta_tenants",
  "conditions": [
    {
      "attribute": "tenant_id",
      "operator": "in",
      "value": ["ten-001", "ten-002", "ten-007"]
    }
  ],
  "operator": "and",
  "value": true,
  "priority": 20
}
```

**Enable for users with a specific role across all tenants:**
```json
{
  "id": "rule_beta_users",
  "conditions": [
    { "attribute": "role", "operator": "contains", "value": "beta_tester" }
  ],
  "operator": "and",
  "value": true,
  "priority": 30
}
```

### 29.5.3 Rule Evaluation

```go
func (e *FlagEvaluator) evaluateRule(rule RolloutRule, ctx EvaluationContext) bool {
    results := make([]bool, len(rule.Conditions))
    for i, cond := range rule.Conditions {
        results[i] = e.evaluateCondition(cond, ctx)
    }

    if rule.Operator == "or" {
        for _, r := range results {
            if r { return true }
        }
        return false
    }

    // Default: "and"
    for _, r := range results {
        if !r { return false }
    }
    return true
}

func (e *FlagEvaluator) evaluateCondition(cond Condition, ctx EvaluationContext) bool {
    var actual interface{}
    switch cond.Attribute {
    case "plan":
        actual = ctx.Plan
    case "tenant_id":
        actual = ctx.TenantID
    case "user_id":
        actual = ctx.UserID
    case "role":
        actual = ctx.Roles
    }

    switch cond.Operator {
    case "eq":
        return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", cond.Value)
    case "in":
        values, _ := cond.Value.([]interface{})
        for _, v := range values {
            if fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", v) { return true }
        }
        return false
    case "contains":
        roles, ok := actual.([]string)
        if !ok { return false }
        target := fmt.Sprintf("%v", cond.Value)
        for _, r := range roles {
            if r == target { return true }
        }
        return false
    }
    return false
}
```

---

## 29.6 Gradual Rollout

### 29.6.1 Percentage Rollout

For progressive rollouts, a flag can be enabled for a percentage of tenants based on a hash of their tenant ID:

```go
type PercentageRollout struct {
    Percentage int    `json:"percentage"` // 0-100
    Salt       string `json:"salt"`       // Randomizes which tenants are included
}

func (e *FlagEvaluator) inRolloutPercentage(tenantID string, rollout PercentageRollout) bool {
    hash := fnv.New32a()
    hash.Write([]byte(tenantID + rollout.Salt))
    bucket := int(hash.Sum32() % 100)
    return bucket < rollout.Percentage
}
```

The `salt` ensures that different flags include different tenants at the same percentage. Without a salt, the same 10% of tenants would always be the first to receive every gradual rollout.

### 29.6.2 Rollout Schedule

Gradual rollouts follow a standard schedule:

```
Day 0:   0% → Internal users (user overrides)
Day 3:   5% → Early access tenants (targeting rule)
Day 7:  20% → Expanded beta
Day 14: 50% → Majority rollout
Day 21: 100% → General availability
```

Each stage is monitored in the observability dashboard (Chapter 38) before advancing. Rollout can be paused or reversed by adjusting the percentage rule without a deployment.

---

## 29.7 Flags in SDUI Compilation

### 29.7.1 Flag-Gated Components

Surface definitions can gate entire component trees on a flag value:

```json
{
  "type": "panel",
  "show_if": "$flag.advanced_reporting",
  "children": [
    {
      "type": "button",
      "label": "Build Custom Report",
      "on_click_action_id": "action_open_report_builder"
    }
  ]
}
```

### 29.7.2 Flag Values in Expressions

Flag bindings (`$flag.*`) are available in all expression contexts within the surface definition:

```json
{
  "type": "field.number",
  "label": "Export Limit",
  "value": "$expr{ $flag.export_row_limit || 1000 }",
  "max": "$expr{ $flag.export_row_limit || 1000 }"
}
```

### 29.7.3 Compilation-Time Optimization

At compile time, boolean flags that are `false` can be used to statically remove component trees from the surface definition, reducing payload size. Components gated by a `false` flag are not included in the compiled definition — the client never sees them, and they consume no payload budget.

The `flag_set_hash` in the cache key (Chapter 8) ensures each unique flag combination produces a distinct cached surface definition. When a tenant's flag values change, their flag set hash changes, and they receive a freshly compiled definition on the next surface fetch.

---

## 29.8 Client-Side Flag Binding

Flags are loaded into session state from the `/api/session` response (Chapter 25) and are accessible via the `$flag.*` binding scope across all surfaces:

```json
{
  "type": "nav.sidebar",
  "items": [
    {
      "label": "Reports",
      "surface_id": "reports_list",
      "visible_if": "$flag.advanced_reporting"
    }
  ]
}
```

The `$flag.*` scope is read-only. Clients cannot set flag values; flags are always server-evaluated.

### 29.8.1 Flag Change Propagation

When a tenant's flag values change (due to a rollout advance or a manual override), the platform emits a `session_config_changed` SSE event. The rendering engine re-fetches the session context and recompiles the navigation and any surfaces that reference the changed flags.

---

## 29.9 Flag Lifecycle

### 29.9.1 States

```
draft → active → deprecated → archived
```

| State | Behavior |
|-------|----------|
| `draft` | Declared but not evaluatable; compile-time warning if referenced in a surface |
| `active` | Evaluatable; used in compilation |
| `deprecated` | Still evaluatable; compile-time warning in surfaces that reference it |
| `archived` | Not evaluatable; compile-time error if referenced |

### 29.9.2 Permanent Flag Cleanup

When a flag reaches 100% rollout and is stable, it should be cleaned up:

1. Set flag to `deprecated`
2. Remove all references from surface definitions (compile-time warnings guide this)
3. Remove flag evaluation from Go application code
4. Set flag to `archived`
5. (Optional) Delete flag record after 90 days

Permanent flags that are plan entitlements (e.g., `advanced_reporting`) are never cleaned up — they remain `active` and are evaluated for the life of the platform.

---

*End of Chapter 29*

**Previous:** [Chapter 28 — Multi-Tenancy](./28-multi-tenancy.md)
**Next:** [Chapter 30 — Customization](./30-customization.md)
