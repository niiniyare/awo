> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Workflow Error Handling"
id: wf-008
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Saga Pattern](saga-pattern.md)"
  - "[Signal Patterns](signal-patterns.md)"
  - "[Temporal Integration](temporal-integration.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Workflow Error Handling

**WF-008 | Status: Accepted | Stability: Stable**

How to handle errors in Temporal workflows and activities: retry policies, non-retryable errors, application errors, and manual intervention.

---

## 1. Activity Retry Policy

Every activity should declare an explicit retry policy:

```go
ao := workflow.ActivityOptions{
    StartToCloseTimeout: 5 * time.Minute,
    RetryPolicy: &temporal.RetryPolicy{
        InitialInterval:    time.Second,
        BackoffCoefficient: 2.0,
        MaximumInterval:    time.Minute,
        MaximumAttempts:    5,
        NonRetryableErrorTypes: []string{
            "BusinessError",     // Domain rule violations — retrying won't help
            "ValidationError",   // Input problems — retrying won't help
            "NotFoundError",     // Record doesn't exist — retrying won't help
        },
    },
}
ctx = workflow.WithActivityOptions(ctx, ao)
```

---

## 2. Non-Retryable vs Retryable Errors

| Error type | Retryable? | Reason |
|---|---|---|
| Network timeout | Yes | Transient — retry will likely succeed |
| Database connection error | Yes | Transient |
| PostgreSQL unique violation | **No** | Structural — retrying inserts same data |
| External API rate limit (429) | Yes | Time-based — use `InitialInterval` to back off |
| External API bad request (400) | **No** | Our request is wrong — retry won't help |
| External API not found (404) | **No** | Resource doesn't exist |
| Business rule violation | **No** | Domain logic — retrying same input is pointless |
| Activity panic (unhandled) | Yes (limited) | May be a transient bug |

### Declaring Non-Retryable Errors in Activities

```go
func (a *Activities) SubmitToETIMSActivity(ctx context.Context, input ETIMSInput) error {
    resp, err := a.ETIMSClient.Submit(ctx, input)
    if err != nil {
        var apiErr *etims.APIError
        if errors.As(err, &apiErr) && apiErr.StatusCode == 400 {
            // Wrap as ApplicationError with non-retryable flag
            return temporal.NewApplicationErrorWithCause(
                "eTIMS rejected invoice data",
                "ETIMSValidationError",  // matches NonRetryableErrorTypes
                err,
                apiErr.Details,
            )
        }
        return fmt.Errorf("SubmitToETIMSActivity: submit: %w", err)
    }
    return nil
}
```

---

## 3. Workflow-Level Error Handling

```go
func InvoiceSubmissionWorkflow(ctx workflow.Context, input InvoiceSubmissionInput) error {
    // Each activity has its own options if needed
    criticalAO := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
        StartToCloseTimeout: 30 * time.Second,
        RetryPolicy: &temporal.RetryPolicy{
            MaximumAttempts: 3,
            NonRetryableErrorTypes: []string{"BusinessError", "ETIMSValidationError"},
        },
    })

    err := workflow.ExecuteActivity(criticalAO, a.SubmitToETIMSActivity, input).Get(criticalAO, nil)
    if err != nil {
        // Distinguish error types in workflow
        var appErr *temporal.ApplicationError
        if errors.As(err, &appErr) {
            switch appErr.Type() {
            case "ETIMSValidationError":
                // Record validation failure on invoice — do not compensate
                return workflow.ExecuteActivity(ctx, a.MarkInvoiceRejectedActivity,
                    MarkRejectedInput{InvoiceID: input.InvoiceID, Reason: appErr.Message()},
                ).Get(ctx, nil)
            case "BusinessError":
                return fmt.Errorf("InvoiceSubmissionWorkflow: business rule: %s", appErr.Message())
            }
        }
        // Unknown error — let workflow fail, Temporal will show in UI
        return fmt.Errorf("InvoiceSubmissionWorkflow: etims: %w", err)
    }

    return nil
}
```

---

## 4. Workflow Timeout Handling

Workflow timeout means the entire workflow did not complete within the allowed window:

```go
// In workflow trigger
WorkflowTriggers: []def.WorkflowTrigger{
    {
        On:         def.EventOnSubmit,
        WorkflowFn: "InvoiceSubmissionWorkflow",
        Options: workflow.StartWorkflowOptions{
            WorkflowExecutionTimeout: 24 * time.Hour,  // Max lifetime
            WorkflowTaskTimeout:      10 * time.Second, // Per-task timeout
        },
    },
},
```

If a workflow times out, Temporal sets it as `TimedOut`. The entity remains in its last-known state. A monitoring alert fires (`awo_workflow_failed_total` counter increases). On-call investigates.

---

## 5. Manual Intervention Pattern

For workflows that get stuck or require human intervention:

```go
// Worker provides an admin endpoint to send a "force continue" signal
func (h *WorkflowAdminHandler) ForceComplete(c *fiber.Ctx) error {
    workflowID := c.Params("workflow_id")

    err := h.TemporalClient.SignalWorkflow(c.UserContext(),
        workflowID, "",
        "admin_override",
        AdminOverrideSignal{Action: "force_complete", Reason: c.Query("reason")},
    )
    if err != nil {
        return fiber.NewError(fiber.StatusInternalServerError, err.Error())
    }
    return c.JSON(fiber.Map{"message": "Signal sent"})
}
```

In the workflow:

```go
// Listen for admin override alongside normal signals
adminCh := workflow.GetSignalChannel(ctx, "admin_override")
selector := workflow.NewSelector(ctx)
selector.AddReceive(approvalCh, func(c workflow.ReceiveChannel, more bool) {
    c.Receive(ctx, &approvalSignal)
})
selector.AddReceive(adminCh, func(c workflow.ReceiveChannel, more bool) {
    c.Receive(ctx, &adminSignal)
    workflow.GetLogger(ctx).Warn("admin override received", "action", adminSignal.Action)
    // Force the workflow to the desired state
})
selector.Select(ctx)
```

---

## 6. Activity Heartbeat for Long-Running Operations

For activities that take minutes (bulk imports, PDF generation, report compilation):

```go
func (a *Activities) GenerateBulkReportActivity(ctx context.Context, input ReportInput) error {
    records, _ := a.Repo.Query(ctx, filter.Eq("period", input.Period))

    for i, record := range records {
        // Heartbeat every 10 records — prevents Temporal marking activity as timed out
        if i%10 == 0 {
            activity.RecordHeartbeat(ctx, fmt.Sprintf("Processing record %d/%d", i, len(records)))
        }

        // Check for cancellation
        if err := ctx.Err(); err != nil {
            return fmt.Errorf("GenerateBulkReportActivity: cancelled at record %d: %w", i, err)
        }

        // Process record...
    }
    return nil
}
```

Activity options for heartbeat-based activities:

```go
workflow.ActivityOptions{
    HeartbeatTimeout:    30 * time.Second,  // Must heartbeat within this window
    StartToCloseTimeout: 4 * time.Hour,     // Overall activity timeout
}
```

---

## 7. Observability for Workflow Errors

All workflow failures increment `awo_workflow_failed_total{workflow_type, tenant_id}`. Alert rule:

```yaml
- alert: WorkflowFailureSurge
  expr: rate(awo_workflow_failed_total[10m]) > 1
  for: 5m
  annotations:
    summary: "Workflow failures above 1/min — possible systematic issue"
```

Failed workflows are also queryable via Temporal Web UI:

```
http://temporal-ui:8080/namespaces/awo-production/workflows?status=Failed
```

---

## Related Documents

- [Saga Pattern](saga-pattern.md) — compensation for multi-step failures
- [Signal Patterns](signal-patterns.md) — human approval and manual intervention signals
- [Metrics Reference](../13-observability/metrics-reference.md) — `awo_workflow_failed_total`
- [Troubleshooting](../14-operations/troubleshooting.md) — Temporal worker diagnosis
