> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Signal and Query Patterns"
id: wf-005
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Temporal Integration](temporal-integration.md)"
  - "[Activities](activities.md)"
  - "[Sagas](sagas.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Signal and Query Patterns

**WF-005 | Status: Accepted | Stability: Stable**

This document describes Temporal signal and query patterns used in Awo for long-running workflows: approval gates, human-in-the-loop steps, workflow status queries, and signal-driven state machines.

---

## 1. What Signals and Queries Are

**Signal**: An asynchronous message sent to a running workflow from outside (HTTP handler, scheduled job, another workflow). Signals allow external events to advance a waiting workflow without polling.

**Query**: A synchronous, read-only call to a running workflow to inspect its current state. Queries do not advance the workflow — they return the workflow's in-memory state at the moment of the call.

Temporal guarantees signal delivery — a signal sent to a running workflow will eventually be processed even if the worker crashes immediately after. Queries are answered only if the workflow is currently running (not completed or terminated).

---

## 2. Multi-Step Approval Workflow

Approval chains are the canonical Temporal signal use case:

```go
// internal/core/finance/workflows/invoice_approval.go

const (
    SignalApprove = "approve"
    SignalReject  = "reject"
)

type ApprovalInput struct {
    TenantID  uuid.UUID
    InvoiceID uuid.UUID
    Amount    decimal.Decimal
    Requester string
}

type ApprovalDecision struct {
    Approved   bool
    ApproverID uuid.UUID
    Comment    string
    DecidedAt  time.Time
}

func InvoiceApprovalWorkflow(ctx workflow.Context, input ApprovalInput) error {
    logger := workflow.GetLogger(ctx)

    // Step 1: Notify approver
    ao := workflow.ActivityOptions{
        StartToCloseTimeout: 30 * time.Second,
        RetryPolicy:         &temporal.RetryPolicy{MaxAttempts: 3},
    }
    ctx = workflow.WithActivityOptions(ctx, ao)

    var activities *InvoiceActivities
    err := workflow.ExecuteActivity(ctx, activities.NotifyApproverActivity, NotifyApproverInput{
        TenantID:  input.TenantID,
        InvoiceID: input.InvoiceID,
        Amount:    input.Amount,
        Requester: input.Requester,
    }).Get(ctx, nil)
    if err != nil {
        return fmt.Errorf("InvoiceApprovalWorkflow: notify approver: %w", err)
    }

    // Step 2: Wait for approval signal (up to 7 days)
    var decision ApprovalDecision
    signalCh := workflow.GetSignalChannel(ctx, SignalApprove)
    rejectCh  := workflow.GetSignalChannel(ctx, SignalReject)

    // Selector blocks until one of: approve signal, reject signal, or 7-day timeout
    selector := workflow.NewSelector(ctx)
    var timedOut bool

    timer := workflow.NewTimer(ctx, 7*24*time.Hour)
    selector.AddFuture(timer, func(f workflow.Future) {
        timedOut = true
    })
    selector.AddReceive(signalCh, func(c workflow.ReceiveChannel, more bool) {
        c.Receive(ctx, &decision)
        decision.Approved = true
        decision.DecidedAt = workflow.Now(ctx)
    })
    selector.AddReceive(rejectCh, func(c workflow.ReceiveChannel, more bool) {
        c.Receive(ctx, &decision)
        decision.Approved = false
        decision.DecidedAt = workflow.Now(ctx)
    })

    selector.Select(ctx)

    // Step 3: Handle outcome
    if timedOut {
        logger.Info("approval timed out, auto-rejecting", "invoice_id", input.InvoiceID)
        return workflow.ExecuteActivity(ctx, activities.RejectInvoiceActivity, RejectInput{
            TenantID:  input.TenantID,
            InvoiceID: input.InvoiceID,
            Reason:    "Approval request expired after 7 days.",
        }).Get(ctx, nil)
    }

    if decision.Approved {
        return workflow.ExecuteActivity(ctx, activities.ApproveInvoiceActivity, ApproveInput{
            TenantID:   input.TenantID,
            InvoiceID:  input.InvoiceID,
            ApproverID: decision.ApproverID,
        }).Get(ctx, nil)
    }

    return workflow.ExecuteActivity(ctx, activities.RejectInvoiceActivity, RejectInput{
        TenantID:  input.TenantID,
        InvoiceID: input.InvoiceID,
        Reason:    decision.Comment,
    }).Get(ctx, nil)
}
```

Key rules:
- Use `workflow.Now(ctx)` — not `time.Now()` — for determinism
- Use `workflow.NewTimer(ctx, duration)` — not `time.After` — for determinism
- `workflow.NewSelector(ctx)` blocks until one branch fires

---

## 3. Sending Signals from an HTTP Handler

```go
// handler.go — called when approver clicks "Approve" in the UI

func ApproveInvoiceHandler(c *fiber.Ctx) error {
    invoiceID, err := uuid.Parse(c.Params("id"))
    if err != nil {
        return c.Status(400).JSON(ErrorEnvelope{Error: ErrorBody{Code: "invalid_id"}})
    }

    var body struct {
        Comment string `json:"comment"`
    }
    if err := c.BodyParser(&body); err != nil {
        return c.Status(400).JSON(ErrorEnvelope{Error: ErrorBody{Code: "invalid_body"}})
    }

    actor := session.ActorFromContext(c.Context())
    tenantID := tenant.IDFromContext(c.Context())

    // Workflow ID matches the convention: {tenant}.{entity}.{id}.{event}
    workflowID := fmt.Sprintf("%s.finance_invoice.%s.on_submit", tenantID, invoiceID)

    signal := ApprovalDecision{
        ApproverID: actor.UserID,
        Comment:    body.Comment,
    }

    err = h.TemporalClient.SignalWorkflow(
        c.Context(),
        workflowID,
        "",  // empty runID = latest run
        SignalApprove,
        signal,
    )
    if err != nil {
        // Workflow may have already completed or been terminated
        var notFound *serviceerror.NotFound
        if errors.As(err, &notFound) {
            return c.Status(409).JSON(ErrorEnvelope{Error: ErrorBody{
                Code:    "workflow_not_found",
                Message: "The approval workflow for this invoice is no longer active.",
            }})
        }
        slog.Error("signal workflow failed",
            "workflow_id", workflowID,
            "err", err,
        )
        return c.Status(500).JSON(ErrorEnvelope{Error: ErrorBody{Code: "internal_error"}})
    }

    return c.Status(200).JSON(SuccessEnvelope{Data: fiber.Map{"message": "Approval recorded."}})
}
```

---

## 4. Workflow Status Queries

Expose workflow state to the UI without polling the entity record:

```go
// Workflow registers a query handler
func InvoiceApprovalWorkflow(ctx workflow.Context, input ApprovalInput) error {
    // In-memory state
    var currentState string = "pending_approval"
    var approver uuid.UUID

    // Register query handler — callable from outside at any time
    if err := workflow.SetQueryHandler(ctx, "status", func() (map[string]any, error) {
        return map[string]any{
            "state":    currentState,
            "approver": approver,
        }, nil
    }); err != nil {
        return fmt.Errorf("InvoiceApprovalWorkflow: set query handler: %w", err)
    }

    // ... selector waits for signal ...

    // Update in-memory state as workflow progresses
    currentState = "approved"
    // ... rest of workflow ...
}
```

```go
// Handler queries running workflow status
func InvoiceApprovalStatusHandler(c *fiber.Ctx) error {
    invoiceID := c.Params("id")
    tenantID := tenant.IDFromContext(c.Context())
    workflowID := fmt.Sprintf("%s.finance_invoice.%s.on_submit", tenantID, invoiceID)

    resp, err := h.TemporalClient.QueryWorkflow(
        c.Context(),
        workflowID,
        "",
        "status",
    )
    if err != nil {
        // Handle: workflow not found (completed), no query handler, etc.
        return mapWorkflowQueryError(c, err)
    }

    var status map[string]any
    if err := resp.Get(&status); err != nil {
        return c.Status(500).JSON(internalError())
    }

    return c.JSON(SuccessEnvelope{Data: status})
}
```

Queries are answered synchronously. The Temporal server routes the query to the worker holding the workflow's goroutine and returns the result directly. Query handlers must be registered before the workflow blocks on the first selector or timer.

---

## 5. Child Workflows

For complex approval chains involving multiple sequential approvers:

```go
func InvoiceMultiApprovalWorkflow(ctx workflow.Context, input ApprovalInput) error {
    // Stage 1: Finance manager approval (for amounts > 50,000)
    if input.Amount.GreaterThan(decimal.NewFromFloat(50000)) {
        childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
            WorkflowID: fmt.Sprintf("%s.level1_approval", workflow.GetInfo(ctx).WorkflowID),
        })
        err := workflow.ExecuteChildWorkflow(childCtx, InvoiceLevelOneApprovalWorkflow, input).Get(ctx, nil)
        if err != nil {
            return fmt.Errorf("level 1 approval failed: %w", err)
        }
    }

    // Stage 2: CFO approval (for amounts > 500,000)
    if input.Amount.GreaterThan(decimal.NewFromFloat(500000)) {
        childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
            WorkflowID: fmt.Sprintf("%s.level2_approval", workflow.GetInfo(ctx).WorkflowID),
        })
        err := workflow.ExecuteChildWorkflow(childCtx, InvoiceLevelTwoApprovalWorkflow, input).Get(ctx, nil)
        if err != nil {
            return fmt.Errorf("level 2 approval failed: %w", err)
        }
    }

    // Final: create payment after all approvals pass
    var activities *InvoiceActivities
    return workflow.ExecuteActivity(ctx, activities.InitiatePaymentActivity, input).Get(ctx, nil)
}
```

Child workflow IDs must be deterministically derived from the parent's workflow ID (use the parent's ID as a prefix). This ensures uniqueness and allows the parent to reference the child in signals/queries.

---

## 6. Signal-Driven State Machine

For entities with complex state transitions driven by external events:

```go
type LeaveRequestWorkflow struct{}

const (
    SignalManagerApprove  = "manager_approve"
    SignalManagerReject   = "manager_reject"
    SignalHRApprove       = "hr_approve"
    SignalHRReject        = "hr_reject"
    SignalWithdraw        = "withdraw"
)

func (w *LeaveRequestWorkflow) Run(ctx workflow.Context, input LeaveRequestInput) error {
    var activities *LeaveActivities
    state := "pending_manager"

    if err := workflow.SetQueryHandler(ctx, "state", func() (string, error) {
        return state, nil
    }); err != nil {
        return err
    }

    // Notify manager
    ao := workflow.ActivityOptions{StartToCloseTimeout: 30 * time.Second}
    ctx = workflow.WithActivityOptions(ctx, ao)
    _ = workflow.ExecuteActivity(ctx, activities.NotifyManagerActivity, input).Get(ctx, nil)

    // Wait for manager decision
    selector := workflow.NewSelector(ctx)
    var decided bool

    // All signal channels registered upfront
    managerApproveCh := workflow.GetSignalChannel(ctx, SignalManagerApprove)
    managerRejectCh  := workflow.GetSignalChannel(ctx, SignalManagerReject)
    withdrawCh       := workflow.GetSignalChannel(ctx, SignalWithdraw)

    selector.AddReceive(managerApproveCh, func(c workflow.ReceiveChannel, _ bool) {
        c.Receive(ctx, nil)
        state = "pending_hr"
        decided = true
    })
    selector.AddReceive(managerRejectCh, func(c workflow.ReceiveChannel, _ bool) {
        c.Receive(ctx, nil)
        state = "rejected"
        decided = true
    })
    selector.AddReceive(withdrawCh, func(c workflow.ReceiveChannel, _ bool) {
        c.Receive(ctx, nil)
        state = "withdrawn"
        decided = true
    })

    selector.Select(ctx)

    if state == "rejected" || state == "withdrawn" {
        return workflow.ExecuteActivity(ctx, activities.FinalizeLeaveActivity,
            FinalizeInput{TenantID: input.TenantID, LeaveID: input.LeaveID, Status: state}).Get(ctx, nil)
    }

    // Pending HR approval
    _ = workflow.ExecuteActivity(ctx, activities.NotifyHRActivity, input).Get(ctx, nil)

    // New selector for HR decision
    decided = false
    selector2 := workflow.NewSelector(ctx)
    hrApproveCh := workflow.GetSignalChannel(ctx, SignalHRApprove)
    hrRejectCh  := workflow.GetSignalChannel(ctx, SignalHRReject)

    selector2.AddReceive(hrApproveCh, func(c workflow.ReceiveChannel, _ bool) {
        c.Receive(ctx, nil)
        state = "approved"
        decided = true
    })
    selector2.AddReceive(hrRejectCh, func(c workflow.ReceiveChannel, _ bool) {
        c.Receive(ctx, nil)
        state = "rejected"
        decided = true
    })
    selector2.AddReceive(withdrawCh, func(c workflow.ReceiveChannel, _ bool) {
        c.Receive(ctx, nil)
        state = "withdrawn"
        decided = true
    })

    selector2.Select(ctx)

    return workflow.ExecuteActivity(ctx, activities.FinalizeLeaveActivity,
        FinalizeInput{TenantID: input.TenantID, LeaveID: input.LeaveID, Status: state}).Get(ctx, nil)
}
```

---

## 7. Workflow ID Lookup for Signal Routing

When an HTTP handler sends a signal, it must know the workflow ID to target. Always derive the workflow ID deterministically from entity identifiers (per LAW-016 convention):

```go
// Convention: {tenant-uuid}.{entity-type}.{record-id}.{event}
func workflowID(tenantID, entityType, recordID, event string) string {
    return fmt.Sprintf("%s.%s.%s.%s", tenantID, entityType, recordID, event)
}
```

This allows any handler, given only a tenant ID and record ID, to construct the correct workflow ID without a database lookup. The convention is a framework-level invariant (LAW-016).

---

## 8. Anti-Patterns

### Polling Entity Status Instead of Querying Workflow

```go
// WRONG: polling entity record for workflow state
for {
    invoice, _ := repo.Get(ctx, invoiceID)
    if invoice.Status != "Submitted" { break }
    time.Sleep(1 * time.Second)
}

// CORRECT: query the running workflow directly
resp, _ := temporalClient.QueryWorkflow(ctx, workflowID, "", "status")
```

### Signals Without Idempotency Guard

Signals are delivered at-least-once. If a signal triggers an activity (e.g., send approval email), the activity must be idempotent — safe to run more than once:

```go
// CORRECT: idempotent notify activity
func (a *Activities) NotifyApproverActivity(ctx context.Context, input NotifyInput) error {
    // Check: was notification already sent?
    alreadySent, err := a.NotificationLog.Exists(ctx, input.TenantID, input.InvoiceID, "approval_notify")
    if err != nil { return err }
    if alreadySent { return nil }  // idempotent: skip if already done

    if err := a.EmailClient.Send(ctx, buildApprovalEmail(input)); err != nil {
        return err
    }
    return a.NotificationLog.Record(ctx, input.TenantID, input.InvoiceID, "approval_notify")
}
```

---

## Related Documents

- [Temporal Integration](temporal-integration.md) — client setup, worker registration, workflow ID format
- [Activities](activities.md) — activity pattern, retry configuration, idempotency
- [Sagas](sagas.md) — compensation-based error handling
- [Architecture Laws](../02-architecture/laws.md) — LAW-016 (workflow ID convention)
- [Glossary](../GLOSSARY.md) — Workflow, Signal, Query, Temporal, Selector, Child Workflow
