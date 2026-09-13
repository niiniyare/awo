> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Saga Pattern

**Classification:** Reference — Tier 2
**Owner:** `08-workflow/SAGA_PATTERN.md`
**Status:** Frozen at v1.0

---

## Purpose

This document specifies the saga pattern for compensating transactions in Temporal workflows.

---

## 1. What Is a Saga

A saga is a sequence of steps where each step has a corresponding compensating action. If any step fails, the workflow executes the compensations in reverse order to undo the effects of the successful steps.

In Awo workflows, sagas are used when a business operation spans multiple external systems (e.g., reserve funds → charge payment gateway → update inventory → send receipt). Each step may succeed or fail independently. A failure mid-saga requires undoing the successful steps.

---

## 2. SagaCompensator Helper

```go
// Package: awo.so/awo/workflow

type SagaCompensator struct {
    steps []func(ctx workflow.Context)
}

func (s *SagaCompensator) AddCompensation(fn func(ctx workflow.Context)) {
    s.steps = append(s.steps, fn)
}

// Compensate runs all registered compensations in reverse order.
// Called when a saga step fails.
func (s *SagaCompensator) Compensate(ctx workflow.Context) {
    for i := len(s.steps) - 1; i >= 0; i-- {
        s.steps[i](ctx)
    }
}
```

---

## 3. Saga Pattern in a Workflow

```go
func PaymentProcessingWorkflow(ctx workflow.Context, input PaymentInput) (err error) {
    saga := &workflow.SagaCompensator{}

    ao := workflow.ActivityOptions{StartToCloseTimeout: 30 * time.Second}
    ctx = workflow.WithActivityOptions(ctx, ao)

    // Step 1: Reserve funds
    err = workflow.ExecuteActivity(ctx, activities.ReserveFunds, input).Get(ctx, nil)
    if err != nil {
        return fmt.Errorf("reserve funds: %w", err)
    }
    saga.AddCompensation(func(ctx workflow.Context) {
        workflow.ExecuteActivity(ctx, activities.ReleaseFundReservation, input).Get(ctx, nil)
    })

    // Step 2: Charge payment gateway
    var gatewayResult ChargeResult
    err = workflow.ExecuteActivity(ctx, activities.ChargeGateway, input).Get(ctx, &gatewayResult)
    if err != nil {
        saga.Compensate(ctx)  // release fund reservation
        return fmt.Errorf("charge gateway: %w", err)
    }
    saga.AddCompensation(func(ctx workflow.Context) {
        workflow.ExecuteActivity(ctx, activities.RefundGateway, gatewayResult).Get(ctx, nil)
    })

    // Step 3: Update inventory
    err = workflow.ExecuteActivity(ctx, activities.DeductStock, input).Get(ctx, nil)
    if err != nil {
        saga.Compensate(ctx)  // refund gateway, release fund reservation
        return fmt.Errorf("deduct stock: %w", err)
    }

    return nil
}
```

---

## 4. Compensation Rules

- Compensating activities MUST be idempotent — they may be called multiple times if the workflow is retried.
- Compensation errors SHOULD be logged but MUST NOT prevent other compensations from running.
- Compensations are best-effort — in cases where a compensation fails (e.g., payment gateway unreachable), the failure MUST be logged with enough context for manual resolution.
- Do not attempt compensation for steps that never started.

---

## References

- [`08-workflow/TEMPORAL_INTEGRATION.md`](TEMPORAL_INTEGRATION.md) — Activity pattern
- `awo/workflow/saga.go` — SagaCompensator implementation
