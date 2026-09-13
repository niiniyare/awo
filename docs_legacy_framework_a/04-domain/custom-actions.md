> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Custom Actions"
id: dom-012
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: normative
related:
  - "[EntityDefinition](../03-kernel/entity-def.md)"
  - "[Hook Pipeline](../03-kernel/hook-pipeline.md)"
  - "[Route Generation](../03-kernel/route-generation.md)"
  - "[Workflow Engine](../09-workflow/workflow-engine.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Custom Actions

**DOM-012 | Status: Accepted | Stability: Stable**

Declaring and implementing custom actions on entity definitions: lifecycle actions, state transitions, and workflow-triggering actions.

---

## 1. What is an Action

An action is a named operation on a specific entity record that goes beyond standard CRUD. Examples: `submit`, `approve`, `cancel`, `qualify`, `close_shift`, `generate_payslip`.

Actions are declared on `EntityDefinition.Actions` and automatically generate routes:

```
POST /api/v1/entities/{entity-type}/{id}/{action-name}
```

Standard CRUD is never replaced — actions extend it.

---

## 2. Declaring an Action

```go
var InvoiceDefinition = def.SystemDefinition{
    Name:   "finance_invoice",
    Module: "finance",
    // ...
    Actions: []def.ActionDef{
        {
            Name:        "submit",
            Method:      def.ActionMethodPost,
            Label:       "Submit for Approval",
            Permission:  "role:finance.accounts_payable",
            HandlerFunc: SubmitInvoiceAction,
        },
        {
            Name:        "approve",
            Method:      def.ActionMethodPost,
            Label:       "Approve",
            Permission:  "role:finance.approver",
            HandlerFunc: ApproveInvoiceAction,
        },
        {
            Name:        "cancel",
            Method:      def.ActionMethodPost,
            Label:       "Cancel",
            Permission:  "role:finance.accounts_payable",
            HandlerFunc: CancelInvoiceAction,
        },
    },
}
```

---

## 3. ActionContext

The framework resolves and passes `ActionContext` to every handler. No manual wiring:

```go
type ActionContext struct {
    // Record being acted on (already loaded, RLS-checked)
    Record   *def.EntityRecord
    RecordID uuid.UUID

    // Pre-scoped repository — tenant + permissions already applied
    Repo def.EntityRepository[def.EntityRecord]

    // Authenticated actor from session
    Actor session.Actor

    // Raw fiber context — for body parsing and response writing
    FiberCtx *fiber.Ctx

    // Temporal client — for starting workflows
    TemporalClient client.Client

    // Request body (if submitted)
    Body map[string]any

    // URL params (action-specific, declared in ActionDef.Params)
    Params map[string]string
}
```

---

## 4. Simple State Transition

```go
func SubmitInvoiceAction(ctx context.Context, action def.ActionContext) (*def.ActionResult, error) {
    record := action.Record

    // Guard: only Draft invoices can be submitted
    if record.Get("status") != "Draft" {
        return nil, &errors.BusinessError{
            Code:    "invoice.already_submitted",
            Message: "Only Draft invoices can be submitted",
            Status:  409,
        }
    }

    // Guard: must have at least one line
    lineCount, err := action.Repo.Count(ctx,
        filter.Eq("invoice", action.RecordID))
    if err != nil {
        return nil, fmt.Errorf("SubmitInvoiceAction: count lines: %w", err)
    }
    if lineCount == 0 {
        return nil, &errors.ValidationError{
            Fields: map[string]string{"lines": "Invoice must have at least one line"},
        }
    }

    // Transition
    updated, err := action.Repo.Update(ctx, action.RecordID, def.UpdateInput{
        Fields: map[string]any{"status": "Submitted"},
    })
    if err != nil {
        return nil, fmt.Errorf("SubmitInvoiceAction: update: %w", err)
    }

    return &def.ActionResult{
        Record:  updated,
        Message: "Invoice submitted for approval",
    }, nil
}
```

---

## 5. Action with Workflow Trigger

Actions that start long-running processes return `WorkflowID`:

```go
func ApproveInvoiceAction(ctx context.Context, action def.ActionContext) (*def.ActionResult, error) {
    record := action.Record

    if record.Get("status") != "Submitted" {
        return nil, &errors.BusinessError{
            Code: "invoice.not_submitted", Message: "Invoice must be Submitted to approve", Status: 409}
    }

    // Update status immediately
    updated, err := action.Repo.Update(ctx, action.RecordID, def.UpdateInput{
        Fields: map[string]any{
            "status":      "Approved",
            "approved_by": action.Actor.UserID,
            "approved_at": time.Now().UTC(),
        },
    })
    if err != nil {
        return nil, fmt.Errorf("ApproveInvoiceAction: update: %w", err)
    }

    // Trigger payment processing workflow (outside transaction — see hook-pipeline.md)
    wfID := fmt.Sprintf("%s.finance_invoice.%s.on_approve",
        action.Actor.TenantID, action.RecordID)

    _, err = action.TemporalClient.ExecuteWorkflow(ctx,
        client.StartWorkflowOptions{
            ID:        wfID,
            TaskQueue: "finance.invoice.approve",
        },
        "InvoiceApprovalWorkflow",
        finance.InvoiceApprovalInput{
            TenantID:  action.Actor.TenantID,
            InvoiceID: action.RecordID,
        },
    )
    if err != nil {
        // Log but don't fail — record is saved, workflow retry will handle it
        slog.Error("ApproveInvoiceAction: start workflow",
            "invoice_id", action.RecordID,
            "err", err)
    }

    return &def.ActionResult{
        Record:     updated,
        Message:    "Invoice approved",
        WorkflowID: wfID,
        HTTPStatus: 202, // Accepted — async processing
    }, nil
}
```

Framework maps `ActionResult.HTTPStatus` to response status. Default is 200.

---

## 6. Action with Input Body

For actions requiring additional input (reason, date, amount):

```go
var InvoiceDefinition = def.SystemDefinition{
    Actions: []def.ActionDef{
        {
            Name:       "partial_payment",
            Method:     def.ActionMethodPost,
            Label:      "Record Partial Payment",
            Permission: "role:finance.cashier",
            InputSchema: map[string]any{    // validates body before handler runs
                "type": "object",
                "properties": map[string]any{
                    "amount_kes": map[string]any{"type": "number", "minimum": 0.01},
                    "payment_date": map[string]any{"type": "string", "format": "date"},
                    "reference":   map[string]any{"type": "string"},
                },
                "required": []string{"amount_kes", "payment_date"},
            },
            HandlerFunc: RecordPartialPaymentAction,
        },
    },
}

func RecordPartialPaymentAction(ctx context.Context, action def.ActionContext) (*def.ActionResult, error) {
    amountKES := action.Body["amount_kes"].(float64)
    paymentDate := action.Body["payment_date"].(string)

    // ... business logic using amountKES and paymentDate
}
```

Framework validates `InputSchema` (JSON Schema) before calling `HandlerFunc`. Invalid body → `422` with field errors, handler never called.

---

## 7. Bulk Action

Actions can operate on multiple records. Declared with `Bulk: true`:

```go
{
    Name:        "bulk_cancel",
    Method:      def.ActionMethodPost,
    Label:       "Cancel Selected",
    Permission:  "role:finance.accounts_payable",
    Bulk:        true,                   // route: POST /entities/{type}/bulk/{action-name}
    HandlerFunc: BulkCancelInvoiceAction,
}
```

Bulk action handler receives a slice of IDs:

```go
func BulkCancelInvoiceAction(ctx context.Context, action def.ActionContext) (*def.ActionResult, error) {
    ids := action.BulkRecordIDs   // []uuid.UUID

    count, err := action.Repo.BulkUpdate(ctx,
        filter.And(
            filter.In("id", ids),
            filter.Eq("status", "Draft"),
        ),
        def.Patch{"status": "Cancelled"},
    )
    if err != nil {
        return nil, fmt.Errorf("BulkCancelInvoiceAction: %w", err)
    }

    return &def.ActionResult{
        Message: fmt.Sprintf("%d invoices cancelled", count),
    }, nil
}
```

---

## 8. Action Visibility in SDUI

SDUI automatically renders action buttons based on `Permission` and optional `VisibleWhen`:

```go
{
    Name:        "approve",
    Permission:  "role:finance.approver",
    VisibleWhen: "status == 'Submitted'",   // amis expression — button absent otherwise
    HandlerFunc: ApproveInvoiceAction,
}
```

`VisibleWhen` is a client-side display hint only — the server enforces `Permission` on every request regardless of whether the button was shown.

---

## 9. Action Response Envelope

```json
{
  "data": {
    "id": "018e...",
    "status": "Approved",
    // ... updated record fields
  },
  "meta": {
    "message": "Invoice approved",
    "workflow_id": "tenantid.finance_invoice.invoiceid.on_approve"
  }
}
```

On error:

```json
{
  "error": {
    "code": "invoice.not_submitted",
    "message": "Invoice must be Submitted to approve"
  }
}
```

---

## Related Documents

- [EntityDefinition](../03-kernel/entity-def.md) — `Actions` field reference
- [Hook Pipeline](../03-kernel/hook-pipeline.md) — how actions relate to the lifecycle
- [Route Generation](../03-kernel/route-generation.md) — auto-generated action routes
- [Workflow Engine](../09-workflow/workflow-engine.md) — triggering from action handlers
- [Action Buttons](../08-sdui/action-buttons.md) — SDUI rendering of action buttons
