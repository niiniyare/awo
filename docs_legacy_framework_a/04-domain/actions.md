> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Actions"
id: dom-005
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[EntityDefinition](../03-kernel/entity-def.md)"
  - "[Hooks](hooks.md)"
  - "[API Conventions](../11-api/conventions.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Actions

**DOM-005 | Status: Accepted | Stability: Stable**

This document specifies custom actions: `ActionDef`, the generated HTTP route, `ActionContext`, `ActionResult`, and patterns for implementing action handlers.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. What Actions Are

Standard CRUD operations (create, read, update, delete) are generated automatically from the `EntityDefinition`. Actions are operations beyond CRUD that represent domain-specific transitions on a record: submitting an invoice for approval, cancelling an order, marking a shift as complete, dispatching a stock picking.

Actions are declared in `EntityDefinition.Actions` and auto-generate HTTP routes. Unlike hooks (which participate in CRUD), actions are first-class HTTP endpoints with their own permission grants.

---

## 2. ActionDef

```go
type ActionDef struct {
    // Name is the stable identifier used in the URL path.
    // Format: snake_case. Example: "submit", "cancel", "dispatch_picking"
    // Immutable after first deployment.
    Name string

    // Method is the HTTP method. Default: Post.
    Method ActionMethod

    // Label is shown in SDUI action buttons.
    Label string

    // Description is shown as a tooltip in SDUI.
    Description string

    // Permission is the single Casbin subject required.
    // Example: "role:finance.accounts_payable"
    // Empty string means any authenticated actor may invoke the action.
    Permission string

    // HandlerFunc is the action implementation.
    HandlerFunc ActionHandlerFunc

    // ConfirmationRequired causes SDUI to show a confirmation dialog before invoking.
    ConfirmationRequired bool

    // ConfirmationMessage is shown in the SDUI confirmation dialog.
    ConfirmationMessage string
}

type ActionMethod int
const (
    ActionMethodPost ActionMethod = iota // default
    ActionMethodGet
)

type ActionHandlerFunc func(ctx context.Context, action ActionContext) (*ActionResult, error)
```

---

## 3. Generated HTTP Route

For an entity `finance_invoice` with action `Name: "submit"`:

```
POST /api/v1/entities/finance_invoice/{id}/submit
```

The framework:
1. Resolves `{id}` to a UUID
2. Checks RBAC: `actor` has `action.Permission` for `finance_invoice`
3. Fetches the record via `EntityRepository.Get()` with the entity's `PolicyFn` applied
4. Returns 404 if the record is not found or not visible to the actor
5. Calls `action.HandlerFunc(ctx, ActionContext{...})`
6. Serializes the returned `ActionResult` as the response body

If the action's `HandlerFunc` triggers a workflow, the response is HTTP 202 with the workflow ID. Otherwise HTTP 200.

---

## 4. ActionContext

```go
type ActionContext struct {
    // Repo is scoped to this tenant and actor. Permissions and PolicyFn applied.
    Repo EntityRepository

    // Record is the target entity record, already fetched.
    Record *EntityRecord

    // RecordID is the target record's UUID.
    RecordID uuid.UUID

    // Actor is the authenticated user from the session context.
    Actor Actor

    // Input is the parsed request body (if the action accepts a body).
    // Nil for actions with Method: ActionMethodGet.
    Input map[string]any

    // TriggerWorkflow is a function that enqueues a workflow start via the outbox.
    // Do not call temporal.StartWorkflow() directly.
    TriggerWorkflow func(trigger entity.WorkflowTrigger, input any) (workflowID string, err error)
}
```

The handler receives a pre-resolved, permission-checked context. It MUST NOT perform its own tenant wiring, session validation, or RBAC checks.

---

## 5. ActionResult

```go
type ActionResult struct {
    // Message is shown to the user in SDUI toast notifications.
    Message string

    // Data is additional structured data included in the API response body.
    // Optional.
    Data any

    // WorkflowID is set when the action triggers an async workflow.
    // Causes the HTTP response to be 202 Accepted instead of 200 OK.
    WorkflowID string

    // Reload causes SDUI to reload the current page after the action completes.
    Reload bool
}
```

---

## 6. Implementing Actions

### Synchronous Action (No Workflow)

```go
func CancelInvoiceAction(ctx context.Context, action entity.ActionContext) (*entity.ActionResult, error) {
    // Validate current state
    status := action.Record.Fields["status"].(string)
    if status == "Paid" || status == "Cancelled" {
        return nil, &entity.BusinessError{
            Code:    "invoice.cannot_cancel",
            Message: fmt.Sprintf("Cannot cancel an invoice in '%s' status", status),
            Status:  409,
        }
    }

    // Update the record
    _, err := action.Repo.Update(ctx, action.RecordID, entity.UpdateInput{
        Fields: map[string]any{"status": "Cancelled"},
    })
    if err != nil {
        return nil, fmt.Errorf("CancelInvoiceAction: update: %w", err)
    }

    return &entity.ActionResult{
        Message: "Invoice cancelled",
        Reload:  true,
    }, nil
}
```

### Workflow-Triggering Action

```go
func SubmitInvoiceAction(ctx context.Context, action entity.ActionContext) (*entity.ActionResult, error) {
    // Validate state
    if action.Record.Fields["status"] != "Draft" {
        return nil, &entity.BusinessError{
            Code:    "invoice.not_draft",
            Message: "Only Draft invoices can be submitted",
            Status:  409,
        }
    }

    // Update status to Submitted
    _, err := action.Repo.Update(ctx, action.RecordID, entity.UpdateInput{
        Fields: map[string]any{"status": "Submitted"},
    })
    if err != nil {
        return nil, fmt.Errorf("SubmitInvoiceAction: update status: %w", err)
    }

    // Trigger workflow via outbox (NOT temporal.StartWorkflow directly)
    workflowID, err := action.TriggerWorkflow(
        entity.WorkflowTrigger{
            WorkflowFn: "InvoiceApprovalWorkflow",
            TaskQueue:  "finance.invoice.submit",
        },
        finance.InvoiceApprovalInput{
            TenantID:  action.Actor.TenantID,
            InvoiceID: action.RecordID,
        },
    )
    if err != nil {
        // TriggerWorkflow writes to outbox — failure here is a system error
        return nil, fmt.Errorf("SubmitInvoiceAction: trigger workflow: %w", err)
    }

    return &entity.ActionResult{
        Message:    "Invoice submitted for approval",
        WorkflowID: workflowID,
        Reload:     true,
    }, nil
}
```

---

## 7. Action Naming Rules

Action names MUST:
- Be lowercase, snake_case
- Be stable after first deployment (embedded in URL paths and SDUI schemas)
- Be unique within the entity
- Use a verb or verb-noun: `submit`, `cancel`, `dispatch_picking`, `mark_complete`

Action names MUST NOT shadow standard CRUD operations or system-reserved names:
- Reserved: `create`, `read`, `update`, `delete`, `list`, `get`, `patch`

---

## 8. SDUI Integration

Declared actions appear as buttons in the SDUI detail view toolbar. The framework generates an amis `action` button for each `ActionDef`:
- `Label` is the button text
- `Permission` gates button visibility (the button is absent from the schema, not just disabled, for actors without the permission)
- `ConfirmationRequired: true` wraps the button in an amis confirm dialog
- Actions that trigger workflows cause the button to show a loading spinner and disable itself after click

Custom button positioning, styling, or conditional display requires a `PageBuilderSet.Detail` override.

---

## Related Documents

- [EntityDefinition](../03-kernel/entity-def.md) — Actions field
- [API Conventions](../11-api/conventions.md) — Generated route format and response envelope
- [Hooks](hooks.md) — Lifecycle hooks (separate from actions)
- [Workflow Layer](../09-workflow/temporal-integration.md) — Triggering workflows from actions
- [Glossary](../GLOSSARY.md) — Action, ActionDef, ActionContext, ActionResult
