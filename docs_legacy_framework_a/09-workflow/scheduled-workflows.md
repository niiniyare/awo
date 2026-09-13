> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Scheduled Workflows"
id: wf-006
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: normative
related:
  - "[Temporal Integration](temporal-integration.md)"
  - "[Activities](activities.md)"
  - "[Multi-Tenancy Patterns](../06-tenancy/multi-tenancy-patterns.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Scheduled Workflows

**WF-006 | Status: Accepted | Stability: Stable**

This document specifies patterns for recurring scheduled workflows: Temporal Schedules, per-tenant job dispatch, and the fan-out pattern for multi-tenant scheduled tasks.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Temporal Schedules

Recurring workflows are managed via Temporal Schedules — not cron jobs or time.Sleep loops.

```go
// Register a schedule at worker startup
func RegisterSchedules(temporalClient client.Client) error {
    _, err := temporalClient.ScheduleClient().Create(context.Background(), client.ScheduleOptions{
        ID: "monthly-invoice-run",
        Spec: client.ScheduleSpec{
            CronExpressions: []string{"0 6 1 * *"},  // 06:00 on the 1st of each month
            TimezoneName:    "Africa/Nairobi",
        },
        Action: &client.ScheduleWorkflowAction{
            Workflow:  MonthlyInvoiceRunWorkflow,
            TaskQueue: "finance.scheduled",
            Args:      []any{MonthlyRunInput{}},
        },
    })
    if err != nil {
        // Schedule already exists — not an error; schedules persist across restarts
        var alreadyExists *serviceerror.WorkflowExecutionAlreadyStarted
        if !errors.As(err, &alreadyExists) {
            return fmt.Errorf("RegisterSchedules: create monthly invoice schedule: %w", err)
        }
    }
    return nil
}
```

Temporal Schedules persist in the Temporal server — they survive worker restarts. `Create` is idempotent: calling it on an already-existing schedule ID is a no-op.

---

## 2. Multi-Tenant Fan-Out

Most scheduled jobs must run per-tenant. The pattern:

1. A single **orchestrator workflow** fetches all active tenants
2. It starts a **per-tenant child workflow** for each tenant
3. Each child workflow sets its tenant context and performs the work

```go
// Step 1: Orchestrator
func MonthlyInvoiceRunWorkflow(ctx workflow.Context, input MonthlyRunInput) error {
    ao := workflow.ActivityOptions{
        StartToCloseTimeout: 60 * time.Second,
    }
    ctx = workflow.WithActivityOptions(ctx, ao)

    // Get all active tenants
    var activities *ScheduledActivities
    var tenants []uuid.UUID
    err := workflow.ExecuteActivity(ctx, activities.ListActiveTenantsActivity).Get(ctx, &tenants)
    if err != nil {
        return fmt.Errorf("MonthlyInvoiceRunWorkflow: list tenants: %w", err)
    }

    // Fan out — start child workflow per tenant
    var futures []workflow.Future
    for _, tenantID := range tenants {
        childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
            WorkflowID: fmt.Sprintf("monthly-invoice-run.%s.%s",
                tenantID,
                workflow.Now(ctx).Format("2006-01")),
            TaskQueue: "finance.scheduled",
        })
        future := workflow.ExecuteChildWorkflow(childCtx, MonthlyInvoiceRunForTenantWorkflow,
            TenantRunInput{TenantID: tenantID, Month: workflow.Now(ctx)})
        futures = append(futures, future)
    }

    // Wait for all children (or collect errors)
    var errs []error
    for _, f := range futures {
        if err := f.Get(ctx, nil); err != nil {
            errs = append(errs, err)
        }
    }

    if len(errs) > 0 {
        // Log all failures; don't fail the orchestrator (other tenants succeeded)
        workflow.GetLogger(ctx).Error("some tenant runs failed", "error_count", len(errs))
    }
    return nil
}
```

```go
// Step 2: Per-tenant workflow
func MonthlyInvoiceRunForTenantWorkflow(ctx workflow.Context, input TenantRunInput) error {
    var activities *InvoiceActivities
    return workflow.ExecuteActivity(
        workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
            StartToCloseTimeout: 10 * time.Minute,
        }),
        activities.GenerateMonthlyInvoicesActivity,
        input,
    ).Get(ctx, nil)
}
```

---

## 3. Tenant Context in Scheduled Activities

Scheduled activities do not have an HTTP request context. Tenant context MUST be set explicitly:

```go
func (a *InvoiceActivities) GenerateMonthlyInvoicesActivity(ctx context.Context, input TenantRunInput) error {
    // Set tenant context before any repository operation
    tenantCtx, err := a.TenantStore.SetTenantContext(ctx, input.TenantID)
    if err != nil {
        return fmt.Errorf("GenerateMonthlyInvoicesActivity: set tenant context: %w", err)
    }

    // All subsequent repo calls use tenantCtx — RLS applies
    customers, _, err := a.CustomerRepo.Query(tenantCtx, filter.Eq("billing_cycle", "Monthly"))
    if err != nil {
        return fmt.Errorf("GenerateMonthlyInvoicesActivity: query customers: %w", err)
    }

    for _, customer := range customers {
        _, err := a.InvoiceRepo.Create(tenantCtx, entity.CreateInput{
            Fields: map[string]any{
                "customer": customer.ID,
                "period":   input.Month.Format("2006-01"),
                "status":   "Draft",
            },
        })
        if err != nil {
            // Log and continue — don't fail entire tenant run for one invoice
            slog.Error("failed to generate monthly invoice",
                "tenant_id",   input.TenantID,
                "customer_id", customer.ID,
                "err",         err,
            )
        }
    }
    return nil
}
```

---

## 4. ListActiveTenants Activity

The orchestrator needs to list all active tenants. This is the one place where a global table query is required:

```go
func (a *ScheduledActivities) ListActiveTenantsActivity(ctx context.Context) ([]uuid.UUID, error) {
    // Tenants table is global — no RLS, no tenant context needed
    tenants, _, err := a.TenantRepo.QueryGlobal(ctx, filter.Eq("status", "Active"))
    if err != nil {
        return nil, fmt.Errorf("ListActiveTenantsActivity: %w", err)
    }

    ids := make([]uuid.UUID, len(tenants))
    for i, t := range tenants {
        ids[i] = t.ID
    }
    return ids, nil
}
```

`QueryGlobal` is available only on the Tenant repository — not on the general `EntityRepository[T]` interface (see TEN-004).

---

## 5. Schedule Management

```
# List all schedules
temporal schedule list

# View schedule details and next runs
temporal schedule describe --schedule-id monthly-invoice-run

# Trigger a manual run (e.g., for a missed run)
temporal schedule trigger --schedule-id monthly-invoice-run

# Pause a schedule (e.g., during maintenance)
temporal schedule pause --schedule-id monthly-invoice-run --reason "Maintenance window"

# Resume
temporal schedule unpause --schedule-id monthly-invoice-run
```

Schedules can also be managed via the Temporal Web UI.

---

## 6. Idempotency in Scheduled Activities

Scheduled activities MUST be idempotent — Temporal may retry them on failure, and manual triggers may cause duplicate runs:

```go
func (a *InvoiceActivities) GenerateMonthlyInvoicesActivity(ctx context.Context, input TenantRunInput) error {
    tenantCtx, err := a.TenantStore.SetTenantContext(ctx, input.TenantID)
    if err != nil { return err }

    period := input.Month.Format("2006-01")

    // Idempotency: check if this period was already processed
    exists, err := a.InvoiceRepo.Exists(tenantCtx, filter.And(
        filter.Eq("period", period),
        filter.Eq("source", "monthly_auto"),
    ))
    if err != nil { return err }
    if exists {
        // Already processed — safe to return nil
        return nil
    }

    // ... generate invoices
}
```

The child workflow ID (`monthly-invoice-run.{tenantID}.{month}`) also provides deduplication at the workflow level — Temporal rejects duplicate workflow ID starts by default.

---

## 7. Anti-Patterns

### time.Sleep in Workflow Code

```go
// WRONG: time.Sleep blocks the goroutine and is not deterministic on replay
time.Sleep(24 * time.Hour)

// CORRECT: workflow.Sleep is durable across worker restarts
workflow.Sleep(ctx, 24 * time.Hour)
```

### Cron Job Instead of Temporal Schedule

```go
// WRONG: a Go cron job that directly calls business logic
cron.AddFunc("0 6 1 * *", func() {
    // No retry, no audit trail, no durability
    generateMonthlyInvoices()
})

// CORRECT: Temporal Schedule that starts a durable workflow
```

### Running All Tenants Serially in One Activity

```go
// WRONG: single activity processes all tenants — slow and non-resumable
func (a *Activities) RunForAllTenantsActivity(ctx context.Context) error {
    tenants := listAllTenants()
    for _, t := range tenants {
        // If this crashes at tenant 47 of 200, restarts from tenant 1
        processForTenant(t)
    }
}

// CORRECT: fan-out to per-tenant child workflows — each tenant is independently resumable
```

---

## Related Documents

- [Temporal Integration](temporal-integration.md) — worker setup, task queues
- [Activities](activities.md) — activity pattern, retry configuration
- [Multi-Tenancy Patterns](../06-tenancy/multi-tenancy-patterns.md) — §3 tenant context in background jobs
- [Signal Patterns](signal-patterns.md) — signal-driven workflows
