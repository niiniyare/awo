> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Sagas"
id: wf-003
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: normative
related:
  - "[Temporal Integration](temporal-integration.md)"
  - "[Activities](activities.md)"
  - "[Outbox Pattern](outbox-pattern.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Sagas

**WF-003 | Status: Accepted | Stability: Stable**

This document specifies the Saga pattern, the `SagaCompensator` helper, compensation ordering rules, and when to use sagas versus simpler retry strategies.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. When to Use Sagas

A saga is a sequence of activities where each step has a defined compensation action. If any step fails after previous steps have committed side effects, the saga runs compensations in reverse order to undo those side effects.

Use sagas when:
- Multiple external systems must be updated in a consistent sequence
- Each step produces side effects that are visible to other processes (written to DB, sent to external API)
- Any step can fail after previous steps have already committed
- Full ACID transactions spanning multiple services are not available

**Do not** use a saga when:
- A single activity is sufficient (no multi-step coordination needed)
- The workflow can be fully retried from the start safely (all steps are idempotent and repeatable)
- The failure mode is purely transient (network hiccup, rate limit) — configure retry policy instead

---

## 2. SagaCompensator

The framework provides `SagaCompensator` — a helper that accumulates compensation functions and runs them in reverse on demand:

```go
type SagaCompensator struct {
    compensations []func(ctx workflow.Context) error
}

// AddCompensation registers a compensation to run on failure.
// Compensations run in LIFO order.
func (s *SagaCompensator) AddCompensation(fn func(ctx workflow.Context) error)

// Compensate runs all registered compensations in reverse order.
// All compensations are attempted regardless of individual failures.
// Returns the first error encountered (others are logged).
func (s *SagaCompensator) Compensate(ctx workflow.Context) error
```

---

## 3. Saga Pattern Implementation

```go
func InvoicePaymentSagaWorkflow(ctx workflow.Context, input PaymentSagaInput) error {
    logger := workflow.GetLogger(ctx)

    ao := workflow.ActivityOptions{
        StartToCloseTimeout: 30 * time.Second,
        RetryPolicy: &temporal.RetryPolicy{
            MaximumAttempts: 3,
        },
    }
    ctx = workflow.WithActivityOptions(ctx, ao)

    saga := &workflows.SagaCompensator{}

    // Step 1: Reserve funds in accounts payable
    var reservation ReserveFundsResult
    err := workflow.ExecuteActivity(ctx, acts.ReserveFundsActivity, ReserveFundsInput{
        TenantID:  input.TenantID,
        InvoiceID: input.InvoiceID,
        Amount:    input.Amount,
    }).Get(ctx, &reservation)
    if err != nil {
        return fmt.Errorf("InvoicePaymentSagaWorkflow: reserve funds: %w", err)
    }

    // Register compensation: release the reservation if subsequent steps fail
    saga.AddCompensation(func(ctx workflow.Context) error {
        return workflow.ExecuteActivity(ctx, acts.ReleaseFundsReservationActivity, ReleaseFundsInput{
            TenantID:      input.TenantID,
            ReservationID: reservation.ReservationID,
        }).Get(ctx, nil)
    })

    // Step 2: Initiate external payment
    var payment InitiatePaymentResult
    err = workflow.ExecuteActivity(ctx, acts.InitiatePaymentActivity, InitiatePaymentInput{
        TenantID:      input.TenantID,
        InvoiceID:     input.InvoiceID,
        ReservationID: reservation.ReservationID,
        Amount:        input.Amount,
        BankAccount:   input.BankAccount,
    }).Get(ctx, &payment)
    if err != nil {
        logger.Error("payment initiation failed; compensating", "error", err)
        saga.Compensate(ctx)  // releases funds reservation
        return fmt.Errorf("InvoicePaymentSagaWorkflow: initiate payment: %w", err)
    }

    // Register compensation: cancel the payment if ledger posting fails
    saga.AddCompensation(func(ctx workflow.Context) error {
        return workflow.ExecuteActivity(ctx, acts.CancelPaymentActivity, CancelPaymentInput{
            TenantID:  input.TenantID,
            PaymentID: payment.PaymentID,
        }).Get(ctx, nil)
    })

    // Step 3: Post to general ledger
    err = workflow.ExecuteActivity(ctx, acts.PostToLedgerActivity, PostToLedgerInput{
        TenantID:  input.TenantID,
        InvoiceID: input.InvoiceID,
        PaymentID: payment.PaymentID,
        Amount:    input.Amount,
    }).Get(ctx, nil)
    if err != nil {
        logger.Error("ledger posting failed; compensating", "error", err)
        saga.Compensate(ctx)  // cancels payment AND releases reservation (reverse order)
        return fmt.Errorf("InvoicePaymentSagaWorkflow: post to ledger: %w", err)
    }

    // Step 4: Mark invoice paid — no compensation needed (desired end state)
    err = workflow.ExecuteActivity(ctx, acts.MarkInvoicePaidActivity, MarkInvoicePaidInput{
        TenantID:  input.TenantID,
        InvoiceID: input.InvoiceID,
        PaymentID: payment.PaymentID,
    }).Get(ctx, nil)
    if err != nil {
        logger.Error("mark paid failed; compensating", "error", err)
        saga.Compensate(ctx)
        return fmt.Errorf("InvoicePaymentSagaWorkflow: mark paid: %w", err)
    }

    return nil
}
```

---

## 4. Compensation Ordering

Compensations execute in **last-in-first-out (LIFO)** order — the reverse of registration order. This mirrors how distributed transaction rollback works: undo the most recent committed operation first.

```
Registration order:       Step 1 → Step 2 → Step 3
Compensation order:       Step 3 → Step 2 → Step 1
```

In the example above, if Step 3 (ledger posting) fails:
1. Cancel payment (undoes Step 2)
2. Release funds reservation (undoes Step 1)

Step 3 itself never completed, so it has no compensation.

---

## 5. Compensation Failure Handling

Compensations can themselves fail. `SagaCompensator.Compensate` attempts all compensations regardless of individual failure — it does not short-circuit. The first compensation error is returned; subsequent errors are logged via `workflow.GetLogger`.

This is intentional: partial compensation is better than no compensation. Manual intervention resolving the remaining partial state is preferable to leaving all compensations unrun.

When compensation fails:
1. Log the failure with sufficient context (workflow ID, compensation step, error)
2. Return the error to the workflow function — the workflow fails
3. Temporal records the failure in workflow history
4. Operations receives an alert (via failed workflow monitoring)
5. Manual remediation is performed using the workflow history as an audit trail

### Compensation Activity Configuration

Compensation activities SHOULD use a more permissive retry policy than the forward activities:

```go
// In SagaCompensator.Compensate, use separate compensation context
compensationCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
    StartToCloseTimeout: 2 * time.Minute,  // longer than forward steps
    RetryPolicy: &temporal.RetryPolicy{
        MaximumAttempts:    10,             // more retries for compensation
        InitialInterval:    2 * time.Second,
        BackoffCoefficient: 2.0,
        MaximumInterval:    time.Minute,
    },
})
```

Compensations are more critical than forward steps — a failed compensation leaves the system in an inconsistent state.

---

## 6. Idempotency of Compensations

All compensation activities MUST be idempotent. Temporal may re-execute compensation activities on worker restart (compensation activities are just activities — they follow the same retry rules).

```go
func (a *FinanceActivities) ReleaseFundsReservationActivity(ctx context.Context, input ReleaseFundsInput) error {
    reservation, err := a.LedgerRepo.Get(ctx, input.ReservationID)
    if errors.Is(err, entity.ErrNotFound) {
        return nil  // Already released — idempotent return
    }
    if err != nil {
        return fmt.Errorf("ReleaseFundsReservationActivity: get reservation: %w", err)
    }

    if reservation.Status == "Released" {
        return nil  // Already released — idempotent return
    }

    return a.LedgerRepo.Update(ctx, input.ReservationID, entity.UpdateInput{
        Fields: map[string]any{"status": "Released"},
    })
}
```

---

## 7. Saga vs. Retry-Only

| Scenario | Strategy |
|---|---|
| External API sometimes fails (network) | Retry with backoff — no saga needed |
| Two DB writes that must both succeed | Use `WithTx` — single transaction is sufficient |
| DB write + external API call that must both succeed | Saga — transaction can't span both |
| Three external services must all be updated | Saga with compensation for each service |
| Long approval chain with human gates | Temporal signals — sagas are for automated compensation |

Sagas add complexity. Prefer simpler strategies when they are sufficient.

---

## 8. Observability

Every saga workflow SHOULD log:
- Each step's start and completion (using `workflow.GetLogger`)
- Compensation trigger (which step failed, triggering compensation)
- Each compensation's execution

Temporal Web UI shows the complete event history, including all activity executions and their results, making post-incident reconstruction straightforward.

Recommended Prometheus metrics for saga monitoring:
- `workflow_saga_started_total{workflow_type, tenant}`
- `workflow_saga_completed_total{workflow_type, tenant, outcome}` (outcome: success, compensated, partial_failure)

---

## Related Documents

- [Activities](activities.md) — implementing individual saga steps and compensations
- [Temporal Integration](temporal-integration.md) — worker setup, determinism rules
- [Outbox Pattern](outbox-pattern.md) — reliable workflow dispatch from entity events
- [Architecture Laws](../02-architecture/laws.md) — LAW-006 (outbox + TX boundaries)
- [Glossary](../GLOSSARY.md) — Saga Pattern, SagaCompensator, Activity, Temporal
