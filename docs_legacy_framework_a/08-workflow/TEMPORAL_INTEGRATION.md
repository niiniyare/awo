> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Temporal Integration

**Classification:** Specification — Tier 1
**Owner:** `08-workflow/TEMPORAL_INTEGRATION.md`
**Status:** Frozen at v1.0

---

## Purpose

This document specifies how the Awo Framework integrates with Temporal: how workflow triggers are declared, how workflows start, determinism requirements, the activity pattern, and the saga pattern.

---

## 1. WorkflowTrigger Declaration

```go
// Package: awo.so/awo/def

type WorkflowTrigger struct {
    // On is the entity lifecycle event that triggers this workflow.
    On EventType

    // WorkflowFn is the registered Temporal workflow function name.
    WorkflowFn string

    // TaskQueue is the Temporal task queue to route the workflow to.
    TaskQueue string

    // WorkflowID is an optional stable ID template.
    // When empty, the runtime generates one using the convention:
    // {tenant}.{entity}.{record_id}.{event}
    WorkflowID string

    // InputBuilder constructs the workflow input from the EntityRecord
    // and TriggerContext. Called at workflow dispatch time.
    InputBuilder func(rec *EntityRecord, tc TriggerContext) (any, error)
}
```

### EventType Constants

| Constant | Trigger |
|----------|---------|
| `EventOnCreate` | After every create |
| `EventOnUpdate` | After every update |
| `EventOnDelete` | After every delete |
| `EventOnSubmit` | When action `"submit"` completes |
| `EventOnApprove` | When action `"approve"` completes |
| `EventOnCancel` | When action `"cancel"` completes |

Custom action events can be declared for any custom action name.

---

## 2. Workflow Start Protocol (ADR-007)

Workflow starts are NOT direct calls to Temporal. They go through the workflow outbox:

```
Entity TX commits
        │
        ▼
Runtime writes WorkflowOutboxRecord to workflow_outbox
(outside entity TX — see OUTBOX_SPEC.md)
        │
        ▼
Outbox worker reads pending records
        │
        ▼
Worker calls Temporal.StartWorkflow()
        │
  ┌─────┴──────┐
  │ Success    │ Failure
  ▼            ▼
Updates record Updates record
status='dispatched'  status='pending'
             Schedules retry with backoff
```

See [`08-workflow/OUTBOX_SPEC.md`](OUTBOX_SPEC.md) for the complete outbox specification.

---

## 3. Determinism Requirements

Temporal achieves durability by replaying workflow history. Replay works only when workflow functions are **deterministic** — the same event history must always produce the same decisions.

### Prohibited in Workflow Code

| Prohibited | Use instead |
|-----------|-------------|
| `time.Now()` | `workflow.Now(ctx)` |
| `time.Sleep(d)` | `workflow.Sleep(ctx, d)` |
| `rand.Read()`, `rand.Int()` | `workflow.SideEffect(ctx, func() any { return rand.Int() })` |
| Direct HTTP calls | Temporal activity |
| Direct database access | Temporal activity |
| `log.Println()` | `workflow.GetLogger(ctx).Info()` |
| Goroutines | `workflow.Go(ctx, func(ctx workflow.Context) {...})` |
| Global variables (mutable) | Inject via workflow input |
| System clock | `workflow.Now(ctx)` |

**These rules are not suggestions.** Violating them produces non-determinism bugs that corrupt workflow history. Temporal may panic on replay or silently produce wrong results.

### Allowed in Workflow Code

- Pure computation (arithmetic, string manipulation, type conversions)
- Calling activities via `workflow.ExecuteActivity()`
- Starting child workflows via `workflow.ExecuteChildWorkflow()`
- Using `workflow.Now(ctx)` for time
- Using `workflow.GetLogger(ctx)` for logging
- Using `workflow.SideEffect()` for non-deterministic values

---

## 4. Activity Pattern

All I/O MUST be in activities. Activities are injected via struct receivers:

```go
// Activities struct allows dependency injection
type FinanceActivities struct {
    Repo   *pgxpool.Pool
    Mailer notifications.Mailer
    Logger *slog.Logger
}

// Naming: {Verb}{Noun}Activity
func (a *FinanceActivities) PostJournalEntryActivity(
    ctx context.Context,
    input PostJournalEntryInput,
) (PostJournalEntryResult, error) {
    // Direct database access is allowed in activities
    // HTTP calls are allowed in activities
    // I/O of all kinds is allowed in activities
    return PostJournalEntryResult{EntryID: entryID}, nil
}
```

**Activity registration:**
```go
worker.RegisterActivity(financeActivities.PostJournalEntryActivity)
```

**Calling from workflow:**
```go
var result PostJournalEntryResult
err := workflow.ExecuteActivity(ctx,
    financeActivities.PostJournalEntryActivity,
    input,
).Get(ctx, &result)
```

---

## 5. Workflow Example

```go
// InvoiceSubmissionWorkflow is the canonical Finance module workflow.
// Naming: {Entity}{Event}Workflow
func InvoiceSubmissionWorkflow(ctx workflow.Context, input InvoiceSubmissionInput) error {
    logger := workflow.GetLogger(ctx)

    ao := workflow.ActivityOptions{
        StartToCloseTimeout: 30 * time.Second,
        RetryPolicy: &temporal.RetryPolicy{
            MaxAttempts: 3,
        },
    }
    ctx = workflow.WithActivityOptions(ctx, ao)

    // Step 1: Post journal entries
    var journalResult PostJournalEntryResult
    err := workflow.ExecuteActivity(ctx,
        financeActivities.PostJournalEntryActivity,
        PostJournalEntryInput{InvoiceID: input.InvoiceID},
    ).Get(ctx, &journalResult)
    if err != nil {
        logger.Error("Failed to post journal entries", "err", err)
        return err
    }

    // Step 2: Send notification email
    err = workflow.ExecuteActivity(ctx,
        notificationActivities.SendInvoiceSubmittedEmail,
        SendEmailInput{InvoiceID: input.InvoiceID},
    ).Get(ctx, nil)
    if err != nil {
        // Non-fatal: log and continue
        logger.Warn("Failed to send notification", "err", err)
    }

    return nil
}
```

---

## 6. Saga Pattern

For workflows that call multiple activities and need compensating transactions:

```go
func PaymentProcessingWorkflow(ctx workflow.Context, input PaymentInput) error {
    var compensations []func(workflow.Context)

    // Step 1: Reserve funds
    err := workflow.ExecuteActivity(ctx, activities.ReserveFunds, input).Get(ctx, nil)
    if err != nil {
        return err
    }
    compensations = append(compensations, func(ctx workflow.Context) {
        workflow.ExecuteActivity(ctx, activities.ReleaseFundReservation, input).Get(ctx, nil)
    })

    // Step 2: Process payment gateway
    err = workflow.ExecuteActivity(ctx, activities.ChargePaymentGateway, input).Get(ctx, nil)
    if err != nil {
        // Compensate in reverse order
        for i := len(compensations) - 1; i >= 0; i-- {
            compensations[i](ctx)
        }
        return err
    }

    return nil
}
```

See [`08-workflow/SAGA_PATTERN.md`](SAGA_PATTERN.md) for the `SagaCompensator` helper.

---

## 7. Workflow Registration

All workflow functions MUST be registered with the Temporal worker at startup:

```go
// cmd/server/main.go or equivalent
w := worker.New(temporalClient, "finance.invoice.submit", worker.Options{})

w.RegisterWorkflow(InvoiceSubmissionWorkflow)
w.RegisterActivity(financeActivities.PostJournalEntryActivity)
w.RegisterActivity(notificationActivities.SendInvoiceSubmittedEmail)
```

Task queues MUST match the `TaskQueue` value in `WorkflowTrigger` declarations.

---

## 8. Normative Requirements

- Workflow functions MUST be deterministic.
- All I/O MUST be in Temporal activities.
- `time.Now()` MUST NOT be called in workflow code — use `workflow.Now(ctx)`.
- `time.Sleep()` MUST NOT be called in workflow code — use `workflow.Sleep(ctx, d)`.
- Workflow starts MUST go through the `workflow_outbox` table (ADR-007).
- Temporal activities MUST be idempotent (they may be retried).
- Workflow function naming MUST follow `{Entity}{Event}Workflow`.
- Activity function naming MUST follow `{Verb}{Noun}Activity`.

---

## References

- [`08-workflow/OUTBOX_SPEC.md`](OUTBOX_SPEC.md) — Workflow outbox
- [`08-workflow/SAGA_PATTERN.md`](SAGA_PATTERN.md) — Compensating transactions
- [`08-workflow/WORKFLOW_ID_CONVENTION.md`](WORKFLOW_ID_CONVENTION.md) — Workflow ID naming
