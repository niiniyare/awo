> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Temporal Integration"
id: wf-001
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Activities](activities.md)"
  - "[Outbox Pattern](outbox-pattern.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Temporal Integration

**WF-001 | Status: Accepted | Stability: Stable**

This document specifies Temporal client setup, worker registration, workflow function rules, workflow ID format, and task queue conventions.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Temporal Client

The Temporal client is initialized during startup (Step 7 of the startup sequence). It uses a lazy client that connects on first use rather than at initialization:

```go
c, err := client.NewLazyClient(client.Options{
    HostPort:  config.TemporalHost,
    Namespace: config.TemporalNamespace,
})
```

The lazy client delays connection until the first workflow operation, allowing the process to start in degraded mode if Temporal is unavailable.

---

## 2. Worker Registration

The Temporal worker is registered with all workflows and activities from all modules:

```go
w := worker.New(c, "awo-main", worker.Options{
    MaxConcurrentActivityExecutionSize:      50,
    MaxConcurrentWorkflowTaskExecutionSize:  10,
})

// Framework registers all compiled workflow triggers
for _, trigger := range schema.WorkflowTriggers {
    w.RegisterWorkflow(trigger.WorkflowFn)
}

// Modules register their activities
finance.RegisterActivities(w)
hr.RegisterActivities(w)
// ...

w.Start()
```

Each module provides a `RegisterActivities(w worker.Worker)` function that registers all activity implementations.

---

## 3. Workflow Function Rules

Workflow functions MUST be deterministic. The following are PROHIBITED in workflow code:

| Prohibited | Required replacement |
|---|---|
| `time.Now()` | `workflow.Now(ctx)` |
| `time.Sleep(d)` | `workflow.Sleep(ctx, d)` |
| `rand.Int()` | `workflow.SideEffect(ctx, ...)` |
| Direct I/O (HTTP, DB, files) | Activity function |
| Standard goroutines `go func()` | `workflow.Go(ctx, ...)` |
| `select` on channels | `workflow.Channel` and `workflow.Go` |
| Global variable mutation | State in workflow variables only |
| Non-deterministic map iteration | Sort keys before iterating |

Violation of these rules causes replay divergence — a `NonDeterministicError` from Temporal — when a workflow is replayed after a worker restart.

### Minimal Workflow Structure

```go
// InvoiceApprovalWorkflow orchestrates the multi-step invoice approval process.
// Stability: STABLE
func InvoiceApprovalWorkflow(ctx workflow.Context, input InvoiceApprovalInput) error {
    logger := workflow.GetLogger(ctx)

    // Configure activity options
    ao := workflow.ActivityOptions{
        StartToCloseTimeout: 30 * time.Second,
        RetryPolicy: &temporal.RetryPolicy{
            MaxAttempts: 3,
            InitialInterval: time.Second,
        },
    }
    ctx = workflow.WithActivityOptions(ctx, ao)

    // Step 1: Notify approvers
    err := workflow.ExecuteActivity(ctx, activities.NotifyApproversActivity, input).Get(ctx, nil)
    if err != nil {
        return fmt.Errorf("InvoiceApprovalWorkflow: notify approvers: %w", err)
    }

    // Step 2: Wait for approval signal (up to 7 days)
    var approvalDecision ApprovalDecision
    signalChan := workflow.GetSignalChannel(ctx, "approval_decision")

    selector := workflow.NewSelector(ctx)
    selector.AddReceive(signalChan, func(c workflow.ReceiveChannel, more bool) {
        c.Receive(ctx, &approvalDecision)
    })

    // Timeout after 7 days
    timer := workflow.NewTimer(ctx, 7*24*time.Hour)
    selector.AddFuture(timer, func(f workflow.Future) {
        approvalDecision = ApprovalDecision{Approved: false, Reason: "timeout"}
    })

    selector.Select(ctx)

    // Step 3: Apply decision
    if approvalDecision.Approved {
        err = workflow.ExecuteActivity(ctx, activities.ApproveInvoiceActivity, input).Get(ctx, nil)
    } else {
        err = workflow.ExecuteActivity(ctx, activities.RejectInvoiceActivity, input, approvalDecision.Reason).Get(ctx, nil)
    }

    return err
}
```

---

## 4. Workflow ID Format

All workflow IDs MUST follow the canonical format (see [LAW-016](../02-architecture/laws.md#law-016-workflow-id-format-is-canonical)):

```
{tenant-uuid}.{entity-name}.{record-id}.{event}.{WorkflowFn}
```

The framework constructs this ID automatically from the WorkflowTrigger configuration. Module authors MUST NOT construct workflow IDs manually.

### WorkflowID Parsing

The canonical format allows parsing workflow IDs to extract entity information:

```go
func ParseWorkflowID(id string) (tenantID, entityName, recordID, event, workflowFn string) {
    parts := strings.SplitN(id, ".", 5)
    return parts[0], parts[1], parts[2], parts[3], parts[4]
}
```

---

## 5. Task Queue Conventions

Task queue names MUST follow the format: `{module}.{entity_noun}.{event}`:

```
finance.invoice.submit
finance.invoice.cancel
hr.payroll.run
inventory.picking.dispatch
```

All workers registered in the main process listen on the `"awo-main"` worker task queue by default. Module-specific task queues allow routing specific workflows to dedicated worker pools in high-volume deployments.

---

## 6. Signal Handling

Signals allow external systems or users to communicate with running workflows:

```go
// Sending a signal (from an action handler)
c.SignalWorkflow(ctx, workflowID, "", "approval_decision", ApprovalDecision{
    Approved: true,
    ApprovedBy: actor.UserID,
    Comment: "Approved after review",
})
```

Signal names MUST be lowercase, hyphen-separated: `approval_decision`, `payment_received`, `cancellation_requested`.

---

## 7. Queries

Temporal queries allow reading workflow state without sending a signal:

```go
// Register query handler (in workflow function)
workflow.SetQueryHandler(ctx, "get_status", func() (string, error) {
    return currentStatus, nil
})

// Querying from outside (in action handler)
value, err := c.QueryWorkflow(ctx, workflowID, "", "get_status")
```

---

## Related Documents

- [Activities](activities.md) — implementing workflow activities
- [Sagas](sagas.md) — compensation for failed workflow steps
- [Outbox Pattern](outbox-pattern.md) — how workflows are started reliably
- [Architecture Laws](../02-architecture/laws.md) — LAW-006, LAW-016
- [Glossary](../GLOSSARY.md) — Workflow, Temporal, Task Queue, Workflow ID
