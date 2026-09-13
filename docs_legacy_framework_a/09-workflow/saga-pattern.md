> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Saga Pattern"
id: wf-007
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Signal Patterns](signal-patterns.md)"
  - "[Scheduled Workflows](scheduled-workflows.md)"
  - "[Finance Patterns](../10-modules/finance-patterns.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Saga Pattern

**WF-007 | Status: Accepted | Stability: Stable**

The Saga pattern provides coordinated, compensatable multi-step operations across distributed activities. Awo provides a `SagaCompensator` helper for implementing sagas in Temporal workflows.

---

## 1. Why Sagas

Multi-step workflows that span multiple systems (database, email, payment gateway, external APIs) cannot use a single database transaction. If step 3 fails, steps 1 and 2 have already committed — they must be explicitly reversed via compensating operations.

Examples in Awo:
- Invoice submission: save → post journal → notify approval chain → send to eTIMS
- Tenant provisioning: create DB record → seed roles → create admin user → activate
- Payroll processing: compute → post journal → generate payslips → send emails

---

## 2. SagaCompensator

```go
// framework/saga/compensator.go

type Compensator struct {
    compensations []CompensationFunc
}

type CompensationFunc func(ctx workflow.Context) error

func NewCompensator() *Compensator {
    return &Compensator{}
}

// Add registers a compensation function.
// Compensations run in LIFO order (last registered, first executed).
func (c *Compensator) Add(fn CompensationFunc) {
    c.compensations = append(c.compensations, fn)
}

// Compensate runs all registered compensations in reverse order.
// Errors are logged but do not stop compensation chain.
func (c *Compensator) Compensate(ctx workflow.Context) {
    for i := len(c.compensations) - 1; i >= 0; i-- {
        if err := c.compensations[i](ctx); err != nil {
            // Compensation failure is alarmed but does not abort
            workflow.GetLogger(ctx).Error("saga compensation failed",
                "step", i, "err", err)
        }
    }
}
```

---

## 3. Saga Workflow Pattern

```go
func InvoiceSubmissionWorkflow(ctx workflow.Context, input InvoiceSubmissionInput) error {
    ao := workflow.ActivityOptions{
        StartToCloseTimeout: 5 * time.Minute,
        RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 3},
    }
    ctx = workflow.WithActivityOptions(ctx, ao)

    compensator := saga.NewCompensator()

    // Step 1: Post journal entry
    var journalID uuid.UUID
    err := workflow.ExecuteActivity(ctx, a.PostJournalEntryActivity, input).Get(ctx, &journalID)
    if err != nil {
        return fmt.Errorf("InvoiceSubmissionWorkflow: post journal: %w", err)
    }
    compensator.Add(func(ctx workflow.Context) error {
        return workflow.ExecuteActivity(ctx, a.ReverseJournalEntryActivity, journalID).Get(ctx, nil)
    })

    // Step 2: Send to eTIMS (KRA tax authority)
    var etimsRef string
    err = workflow.ExecuteActivity(ctx, a.SubmitToETIMSActivity, input).Get(ctx, &etimsRef)
    if err != nil {
        compensator.Compensate(ctx)  // reverses Step 1
        return fmt.Errorf("InvoiceSubmissionWorkflow: etims submit: %w", err)
    }
    compensator.Add(func(ctx workflow.Context) error {
        return workflow.ExecuteActivity(ctx, a.CancelETIMSSubmissionActivity, etimsRef).Get(ctx, nil)
    })

    // Step 3: Notify approval chain
    err = workflow.ExecuteActivity(ctx, a.NotifyApprovalChainActivity, input).Get(ctx, nil)
    if err != nil {
        compensator.Compensate(ctx)  // reverses Steps 2 and 1 (LIFO)
        return fmt.Errorf("InvoiceSubmissionWorkflow: notify approvers: %w", err)
    }

    // Step 4: Update invoice status to "Submitted"
    err = workflow.ExecuteActivity(ctx, a.UpdateInvoiceStatusActivity, InvoiceStatusInput{
        InvoiceID: input.InvoiceID,
        Status:    "Submitted",
        ETIMSRef:  etimsRef,
    }).Get(ctx, nil)
    if err != nil {
        compensator.Compensate(ctx)
        return fmt.Errorf("InvoiceSubmissionWorkflow: update status: %w", err)
    }

    return nil
}
```

---

## 4. Activity Idempotency

Compensating activities must also be idempotent. If a compensation is retried (e.g., after worker restart), it must be safe to run twice:

```go
func (a *Activities) ReverseJournalEntryActivity(ctx context.Context, journalID uuid.UUID) error {
    // Check if already reversed
    journal, err := a.JournalRepo.Get(ctx, journalID)
    if err != nil {
        return fmt.Errorf("ReverseJournalEntryActivity: get journal: %w", err)
    }

    status, _ := journal.Fields["status"].(string)
    if status == "Reversed" {
        return nil  // Already reversed — idempotent
    }

    // Create reversing journal entry
    _, err = a.JournalRepo.Create(ctx, def.CreateInput{
        Fields: map[string]any{
            "reference":        fmt.Sprintf("REV/%s", journal.Fields["reference"]),
            "reversal_of":      journalID,
            // Swap debit/credit amounts
        },
    })
    if err != nil {
        return fmt.Errorf("ReverseJournalEntryActivity: create reversal: %w", err)
    }

    // Mark original as reversed
    _, err = a.JournalRepo.Update(ctx, journalID, def.UpdateInput{
        Fields: map[string]any{"status": "Reversed"},
    })
    return err
}
```

---

## 5. Compensation Failure Handling

If a compensation step fails (e.g., external API unavailable), the `SagaCompensator` logs the error and continues with remaining compensations. This is intentional:

- One failed compensation must not prevent other compensations from running
- Failed compensations are alarmed (metrics + structured log at ERROR level)
- An operator or support workflow handles the failed compensation manually

For critical compensations (money reversal), failed compensations create an alert in the `operations_alert` entity with all context needed for manual resolution.

---

## 6. Saga vs Transaction Decision

| Scenario | Use |
|---|---|
| All operations in same PostgreSQL DB | `WithTx` (database transaction) |
| Operations span multiple services/APIs | Saga |
| Long-running process (minutes to hours) | Saga (DB transaction cannot be held) |
| Need rollback on failure | Saga with compensations |
| Need exactly-once delivery | Saga + idempotency keys |

---

## 7. Visualizing Saga State

Temporal Web UI shows the complete saga execution history:
- Each activity is a node in the workflow event history
- Compensating activities are clearly labeled
- Failures and retries are timestamped
- The workflow result (success or compensated failure) is the final state

This provides a complete audit trail of multi-step operations without any additional instrumentation.

---

## Related Documents

- [Finance Patterns](../10-modules/finance-patterns.md) — InvoiceSubmissionWorkflow using SagaCompensator
- [Payroll Patterns](../10-modules/payroll-patterns.md) — PayrollProcessingWorkflow saga
- [Signal Patterns](signal-patterns.md) — signal-driven approval within sagas
- [Troubleshooting](../14-operations/troubleshooting.md) — diagnosing Temporal workflow failures
