> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Activities"
id: wf-002
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: normative
related:
  - "[Temporal Integration](temporal-integration.md)"
  - "[Sagas](sagas.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Activities

**WF-002 | Status: Accepted | Stability: Stable**

This document specifies the activity pattern, dependency injection via struct receivers, retry configuration, error classification, and activity registration conventions.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. What Activities Are

An activity is a Go function (or method) that performs a single, side-effectful operation on behalf of a workflow. Activities are the only place where I/O is permitted in the workflow layer:

- Database reads and writes
- HTTP calls to external services
- Email and SMS dispatch
- File storage operations
- Audit log writes

Activities execute outside the workflow's event-sourced execution context. They may fail, and Temporal will retry them according to the configured retry policy. They MUST be idempotent when retried.

---

## 2. Activity Struct Pattern

Activities MUST be declared as methods on a struct that carries injected dependencies. This is the only supported pattern — standalone functions are not acceptable because they cannot carry dependencies without global state.

```go
// Activities groups all activities for the finance module's invoice workflows.
// Each field is a dependency injected at worker startup.
type InvoiceActivities struct {
    Repo         entity.EntityRepository[Invoice]
    EmailClient  notifications.EmailClient
    AuditLog     audit.Logger
    TemporalClient client.Client
}
```

The struct is registered with the Temporal worker at startup:

```go
invoiceActivities := &finance.InvoiceActivities{
    Repo:        invoiceRepo,
    EmailClient: emailClient,
    AuditLog:    auditLogger,
}
w.RegisterActivity(invoiceActivities)
```

Temporal reflects over the struct and registers all exported methods as individual activities.

---

## 3. Activity Function Signature

Every activity method MUST follow this signature:

```go
func (a *MyActivities) VerbNounActivity(ctx context.Context, input InputType) (OutputType, error)
```

Rules:
- First argument MUST be `context.Context`
- Input MUST be a serializable Go struct (JSON-serializable; no channels, funcs, or interfaces)
- Output MUST be a serializable Go struct or `error` only (if no output needed: `(error)`)
- Method name MUST follow the `{Verb}{Noun}Activity` convention

```go
// Correct naming examples
func (a *InvoiceActivities) NotifyApproversActivity(ctx context.Context, input NotifyApproversInput) error
func (a *InvoiceActivities) ApproveInvoiceActivity(ctx context.Context, input ApproveInvoiceInput) (*ApproveInvoiceResult, error)
func (a *InvoiceActivities) SendRejectionEmailActivity(ctx context.Context, input RejectionEmailInput) error
```

---

## 4. Activity Input and Output Structs

Activity inputs and outputs MUST be defined as named structs — never raw primitive types or `map[string]any`. Named structs allow versioned schema evolution and are self-documenting.

```go
type NotifyApproversInput struct {
    TenantID  uuid.UUID `json:"tenant_id"`
    InvoiceID uuid.UUID `json:"invoice_id"`
    Amount    string    `json:"amount"`   // string, not decimal — safe for JSON round-trip
    Currency  string    `json:"currency"`
}

type ApproveInvoiceResult struct {
    WorkflowID   string    `json:"workflow_id"`
    ApprovedAt   time.Time `json:"approved_at"`
    JournalEntryID uuid.UUID `json:"journal_entry_id"`
}
```

For monetary values in activity inputs and outputs, use string representation (`"1234.5600"`) not `decimal.Decimal` — `decimal.Decimal` serializes correctly but string removes dependency on the decimal library from the activity input spec.

---

## 5. Retry Configuration

Retry policy MUST be configured on the `workflow.ActivityOptions` in the workflow function, not in the activity itself. Activities do not declare their own retry policy — this allows the workflow to set context-appropriate retries.

```go
// In the workflow function:
ao := workflow.ActivityOptions{
    StartToCloseTimeout: 30 * time.Second,
    HeartbeatTimeout:    10 * time.Second,
    RetryPolicy: &temporal.RetryPolicy{
        InitialInterval:        time.Second,
        BackoffCoefficient:     2.0,
        MaximumInterval:        30 * time.Second,
        MaximumAttempts:        3,
        NonRetryableErrorTypes: []string{"finance.invoice_not_found", "finance.already_approved"},
    },
}
ctx = workflow.WithActivityOptions(ctx, ao)
```

### Timeout Selection Guide

| Timeout Type | When to Use | Typical Value |
|---|---|---|
| `StartToCloseTimeout` | Maximum time from start to completion | 30s–5min |
| `ScheduleToCloseTimeout` | Maximum time including scheduling delay | Use for SLA enforcement |
| `HeartbeatTimeout` | Long-running activities that heartbeat | 30s–2min |
| `ScheduleToStartTimeout` | Maximum wait in task queue | Rarely needed |

`StartToCloseTimeout` MUST always be set — it has no default.

---

## 6. Error Classification

Activities classify errors to control retry behavior.

### Retryable Errors (default)

Transient failures that SHOULD be retried:
- Network timeouts
- Database connection errors
- Rate limiting (HTTP 429)
- Temporary external service unavailability

Return these as plain Go errors:

```go
func (a *InvoiceActivities) NotifyApproversActivity(ctx context.Context, input NotifyApproversInput) error {
    err := a.EmailClient.Send(ctx, ...)
    if err != nil {
        // Retryable — Temporal will retry per RetryPolicy
        return fmt.Errorf("NotifyApproversActivity: send email: %w", err)
    }
    return nil
}
```

### Non-Retryable Errors

Business rule violations or permanent failures that MUST NOT be retried:

```go
import "go.temporal.io/sdk/temporal"

func (a *InvoiceActivities) ApproveInvoiceActivity(ctx context.Context, input ApproveInvoiceInput) error {
    invoice, err := a.Repo.Get(ctx, input.InvoiceID)
    if err != nil {
        return fmt.Errorf("ApproveInvoiceActivity: get invoice: %w", err)
    }

    if invoice.Status != "Submitted" {
        // Non-retryable: business rule violation
        return temporal.NewNonRetryableApplicationError(
            "invoice is not in Submitted status",
            "finance.invalid_status",
            nil,
        )
    }
    // ...
}
```

Non-retryable error types declared in `RetryPolicy.NonRetryableErrorTypes` MUST match the error type string exactly.

---

## 7. Idempotency

Every activity that writes data MUST be idempotent — executing it twice with the same input MUST produce the same result. Temporal may re-execute activities on worker restart or transient failure.

### Idempotency Patterns

**Database upsert (preferred for DB writes):**

```go
func (a *InvoiceActivities) RecordApprovalActivity(ctx context.Context, input RecordApprovalInput) error {
    // Upsert — safe to call multiple times
    _, err := a.Repo.Create(ctx, entity.CreateInput{
        Fields: map[string]any{
            "invoice_id":   input.InvoiceID,
            "approved_by":  input.ApprovedBy,
            "workflow_id":  input.WorkflowID,  // idempotency key
        },
        ConflictField: "workflow_id",  // ON CONFLICT DO NOTHING
        ConflictAction: entity.ConflictIgnore,
    })
    return err
}
```

**Check-before-act (for external calls):**

```go
func (a *InvoiceActivities) SendApprovalEmailActivity(ctx context.Context, input ApprovalEmailInput) error {
    // Check if already sent using an idempotency record
    sent, err := a.Repo.Exists(ctx, filter.And(
        filter.Eq("workflow_id", input.WorkflowID),
        filter.Eq("event", "approval_email_sent"),
    ))
    if err != nil {
        return fmt.Errorf("SendApprovalEmailActivity: check sent: %w", err)
    }
    if sent {
        return nil  // Already sent — idempotent return
    }

    err = a.EmailClient.Send(ctx, buildApprovalEmail(input))
    if err != nil {
        return fmt.Errorf("SendApprovalEmailActivity: send: %w", err)
    }

    // Record that email was sent
    _, err = a.AuditLog.Record(ctx, "approval_email_sent", input.WorkflowID)
    return err
}
```

---

## 8. Heartbeating

Long-running activities MUST heartbeat periodically so Temporal can detect worker failures:

```go
func (a *ReportActivities) GenerateMonthlyReportActivity(ctx context.Context, input ReportInput) error {
    records, err := a.Repo.Query(ctx, filter.Eq("month", input.Month))
    if err != nil {
        return err
    }

    for i, record := range records {
        // Heartbeat every 100 records
        if i%100 == 0 {
            activity.RecordHeartbeat(ctx, fmt.Sprintf("processed %d/%d", i, len(records)))
        }

        // Check for cancellation
        if ctx.Err() != nil {
            return ctx.Err()
        }

        if err := processRecord(record); err != nil {
            return fmt.Errorf("GenerateMonthlyReportActivity: record %d: %w", i, err)
        }
    }
    return nil
}
```

`HeartbeatTimeout` on the ActivityOptions MUST be shorter than the heartbeat interval. If the worker crashes, Temporal reschedules the activity after `HeartbeatTimeout` elapses.

---

## 9. Activity Registration

Each module provides a `RegisterActivities(w worker.Worker)` function:

```go
// finance/workflows/activities.go

func RegisterActivities(w worker.Worker, deps Dependencies) {
    invoiceActs := &InvoiceActivities{
        Repo:        deps.InvoiceRepo,
        EmailClient: deps.EmailClient,
        AuditLog:    deps.AuditLog,
    }
    w.RegisterActivity(invoiceActs)

    paymentActs := &PaymentActivities{
        Repo:         deps.PaymentRepo,
        LedgerRepo:   deps.LedgerRepo,
        PaymentGW:    deps.PaymentGateway,
    }
    w.RegisterActivity(paymentActs)
}
```

The main worker startup calls each module's registration function:

```go
finance.RegisterActivities(w, financeDeps)
hr.RegisterActivities(w, hrDeps)
inventory.RegisterActivities(w, inventoryDeps)
```

---

## 10. Calling Activities from Workflows

Activities are invoked via `workflow.ExecuteActivity`. The activity is referenced by function value:

```go
err := workflow.ExecuteActivity(ctx, activities.NotifyApproversActivity, input).Get(ctx, nil)
```

For activities with output:

```go
var result finance.ApproveInvoiceResult
err := workflow.ExecuteActivity(ctx, activities.ApproveInvoiceActivity, input).Get(ctx, &result)
if err != nil {
    return fmt.Errorf("InvoiceApprovalWorkflow: approve: %w", err)
}
```

Activity references MUST use the actual function value (not a string name). This preserves compile-time type checking.

---

## 11. Testing Activities

Activities are tested independently of Temporal using standard Go tests with injected mock dependencies.

```go
func TestApproveInvoiceActivity_Success(t *testing.T) {
    repo := &mockInvoiceRepo{
        invoice: &Invoice{ID: testID, Status: "Submitted"},
    }
    acts := &InvoiceActivities{
        Repo:     repo,
        AuditLog: &mockAuditLog{},
    }

    err := acts.ApproveInvoiceActivity(context.Background(), ApproveInvoiceInput{
        InvoiceID:  testID,
        ApprovedBy: approverID,
        WorkflowID: "test-workflow-1",
    })

    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if repo.lastUpdate.Fields["status"] != "Approved" {
        t.Errorf("expected status Approved, got %v", repo.lastUpdate.Fields["status"])
    }
}

func TestApproveInvoiceActivity_WrongStatus(t *testing.T) {
    repo := &mockInvoiceRepo{
        invoice: &Invoice{ID: testID, Status: "Draft"},  // Not Submitted
    }
    acts := &InvoiceActivities{Repo: repo}

    err := acts.ApproveInvoiceActivity(context.Background(), ApproveInvoiceInput{
        InvoiceID: testID,
    })

    var appErr *temporal.ApplicationError
    if !errors.As(err, &appErr) {
        t.Fatalf("expected ApplicationError, got %T: %v", err, err)
    }
    if appErr.Type() != "finance.invalid_status" {
        t.Errorf("expected type finance.invalid_status, got %s", appErr.Type())
    }
}
```

For workflow-level integration tests (activity execution within the workflow event loop), use `testsuite.WorkflowTestSuite` with activity mocks.

---

## Related Documents

- [Temporal Integration](temporal-integration.md) — worker registration, workflow determinism rules
- [Sagas](sagas.md) — compensation for failed activities
- [Outbox Pattern](outbox-pattern.md) — reliable workflow dispatch
- [Architecture Laws](../02-architecture/laws.md) — LAW-006 (outbox), LAW-016 (workflow ID)
- [Glossary](../GLOSSARY.md) — Activity, Temporal, Workflow, Idempotency
