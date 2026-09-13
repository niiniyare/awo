# Action Handler Guide

**Classification:** Guide — Tier 2
**Owner:** `13-actions/ACTION_HANDLER_GUIDE.md`
**Status:** Living document

---

## Purpose

This guide explains how to write custom action handlers — the `ActionHandlerFunc` implementations that power non-CRUD operations (submit, approve, cancel, post, etc.). It covers the `ActionDef` declaration, `ActionContext`, `ActionRuntime` usage patterns, and error handling.

---

## 1. What Custom Actions Are

Standard CRUD routes are auto-generated from `EntityDefinition`. Custom actions extend the entity API with business-specific operations that go beyond create/read/update/delete.

Each custom action maps to:
- A route: `POST /api/v1/entities/{entity-type}/{id}/{action-name}`
- A permission check (before the handler runs)
- A handler function that receives `ActionContext` and returns `ActionResult`

---

## 2. Declaring an Action

```go
var InvoiceDefinition = def.SystemDefinition{
    // ... standard fields ...
    Actions: []def.ActionDef{
        {
            Name:        "submit",
            Method:      def.ActionMethodPost,
            Label:       "Submit for Approval",
            Permission:  "finance.invoice.submit",
            HandlerFunc: SubmitInvoiceAction,
        },
        {
            Name:        "approve",
            Method:      def.ActionMethodPost,
            Label:       "Approve Invoice",
            Permission:  "finance.invoice.approve",
            HandlerFunc: ApproveInvoiceAction,
        },
        {
            Name:        "cancel",
            Method:      def.ActionMethodPost,
            Label:       "Cancel Invoice",
            Permission:  "finance.invoice.cancel",
            HandlerFunc: CancelInvoiceAction,
        },
    },
}
```

`ActionDef` fields:
- `Name`: snake_case action name. Used in route path and workflow IDs.
- `Method`: `ActionMethodPost` (default and required for mutations) or `ActionMethodGet` (read-only actions).
- `Label`: human-readable label. Used in SDUI action buttons.
- `Permission`: permission identifier checked by `PolicyEvaluator` before the handler is called. Format: `"{module}.{entity}.{action}"`. MUST NOT be a role name.
- `HandlerFunc`: the implementation.

---

## 3. ActionHandlerFunc Signature

```go
type ActionHandlerFunc func(
    ctx    context.Context,
    action ActionContext,
) (*ActionResult, error)
```

`ActionContext` provides:
- `action.Runtime` — the `ActionRuntime` with all framework services
- `action.RecordID` — the target record's UUID
- `action.Body` — request body as `map[string]any` (for actions that accept a payload)
- `action.Entity` — the `EntitySchema` for the target entity

---

## 4. Minimal Action Handler

```go
func SubmitInvoiceAction(ctx context.Context, action def.ActionContext) (*def.ActionResult, error) {
    repo := action.Runtime.Repo("finance_invoice")

    // 1. Load the record
    rec, err := repo.Get(ctx, action.RecordID)
    if err != nil {
        return nil, err
    }

    // 2. Business rule validation
    if rec.GetString("status") != "Draft" {
        return nil, &def.BusinessError{
            Code:    "invoice.already_submitted",
            Message: "Only Draft invoices can be submitted.",
            Status:  409,
        }
    }

    // 3. Mutate inside a transaction
    err = action.Runtime.Tx(ctx, func(ctx context.Context) error {
        _, err := repo.Update(ctx, action.RecordID, map[string]any{
            "status":       "Submitted",
            "submitted_by": action.Runtime.Actor().UserID,
            "submitted_at": action.Runtime.Clock(),
        })
        if err != nil {
            return err
        }

        // 4. Publish domain event (inside TX — rolls back with TX on failure)
        return action.Runtime.Publish(ctx, def.ActionEvent{
            Topic: "finance.invoice.submitted",
            Payload: map[string]any{
                "invoice_id":   action.RecordID,
                "submitted_by": action.Runtime.Actor().UserID,
                "total":        rec.GetDecimal("total"),
            },
        })
    })
    if err != nil {
        return nil, err
    }

    // 5. Start workflow (outside TX — dispatched after commit)
    workflowID, err := action.Runtime.StartWorkflow(ctx, def.ActionWorkflowSpec{
        WorkflowFn: "InvoiceApprovalWorkflow",
        TaskQueue:  "finance.invoice.approval",
        Input: InvoiceApprovalInput{
            TenantID:  action.Runtime.TenantID(),
            InvoiceID: action.RecordID,
        },
    })
    if err != nil {
        // Log but don't fail the action — entity is already saved
        action.Runtime.Logger().Error("failed to start approval workflow",
            "err", err,
            "invoice_id", action.RecordID,
        )
    }

    return &def.ActionResult{
        Message:    "Invoice submitted for approval.",
        WorkflowID: workflowID,
    }, nil
}
```

---

## 5. ActionResult

```go
type ActionResult struct {
    // Message is the human-readable result shown in the SDUI.
    Message string

    // Data contains additional response fields serialised to JSON.
    // Optional.
    Data map[string]any

    // WorkflowID is the started workflow ID, if any.
    // Triggers the API to return HTTP 202 Accepted instead of 200.
    WorkflowID string

    // RedirectTo is an SDUI navigation target after success.
    // Optional. E.g. "../" to return to list view.
    RedirectTo string
}
```

When `WorkflowID` is non-empty, the framework returns HTTP 202 with:
```json
{
  "data": { "id": "...", "workflow_id": "..." },
  "meta": { "message": "Invoice submitted for approval." }
}
```

---

## 6. Error Handling in Action Handlers

Return `*def.BusinessError` for domain rule violations:

```go
return nil, &def.BusinessError{
    Code:    "invoice.already_paid",
    Message: "Cannot cancel a paid invoice.",
    Status:  409,
}
```

Return `*def.ValidationError` for input validation failures:

```go
return nil, &def.ValidationError{
    Fields: map[string]string{
        "reason": "Cancellation reason is required.",
    },
}
```

Return raw errors for unexpected failures — the framework maps them to HTTP 500 and logs internally. Never expose raw error messages to clients.

---

## 7. Using the Request Body

Actions that accept a payload declare the body fields:

```go
func CancelInvoiceAction(ctx context.Context, action def.ActionContext) (*def.ActionResult, error) {
    reason, ok := action.Body["reason"].(string)
    if !ok || reason == "" {
        return nil, &def.ValidationError{
            Fields: map[string]string{"reason": "Cancellation reason is required."},
        }
    }
    // ... rest of handler
}
```

`action.Body` is `map[string]any` from JSON decoding. Always type-assert defensively.

---

## 8. Read-Only Actions

`ActionMethodGet` actions do not require a request body and SHOULD NOT mutate state:

```go
{
    Name:        "preview_pdf",
    Method:      def.ActionMethodGet,
    Label:       "Preview PDF",
    Permission:  "finance.invoice.read",
    HandlerFunc: PreviewInvoicePDF,
},
```

GET actions return HTTP 200 with their `ActionResult.Data` serialised to JSON.

---

## 9. Testing Action Handlers

See [`16-testing/ACTION_TEST_PATTERNS.md`](../16-testing/ACTION_TEST_PATTERNS.md) for test patterns using mock `ActionRuntime`.

Key testing principles:
- Inject mock `ActionRuntime` via the test helper
- Test each business rule as an independent case
- Verify `Publish` and `StartWorkflow` are called with expected arguments
- Verify the correct HTTP status code via `ActionResult.WorkflowID`

---

## References

- [`13-actions/ACTION_RUNTIME_REFERENCE.md`](ACTION_RUNTIME_REFERENCE.md) — Full ActionRuntime method contracts
- [`13-actions/CUSTOM_ACTIONS_EXAMPLES.md`](CUSTOM_ACTIONS_EXAMPLES.md) — Finance module worked examples
- [`02-pipeline/ERROR_MODEL.md`](../02-pipeline/ERROR_MODEL.md) — BusinessError, ValidationError
- [`16-testing/ACTION_TEST_PATTERNS.md`](../16-testing/ACTION_TEST_PATTERNS.md) — Test patterns
