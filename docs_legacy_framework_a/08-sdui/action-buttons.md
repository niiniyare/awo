> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Action Buttons in SDUI"
id: sdui-007
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Actions](../04-domain/actions.md)"
  - "[Permission Sets](../04-domain/permissions.md)"
  - "[List Patterns](list-patterns.md)"
  - "[Form Patterns](form-patterns.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Action Buttons in SDUI

**SDUI-007 | Status: Accepted | Stability: Stable**

Patterns for surfacing custom actions in amis schemas: toolbar buttons, row-level actions, confirmation dialogs, and permission gates.

---

## 1. Toolbar Action Button (List View)

```go
func BuildInvoiceListPage(ctx context.Context, actor session.Actor) ([]byte, error) {
    psc := sdui.NewPageSchemaContext(ctx, actor)

    var toolbar []amis.Component

    // New Invoice — gated on create permission
    if psc.IfPermitted("finance_invoice", "create") {
        toolbar = append(toolbar, amis.Button(amis.ButtonProps{
            Label:     "New Invoice",
            ActionType: "link",
            Link:      "/finance/invoice/new",
            Level:     "primary",
        }))
    }

    // Bulk Export — available to all readers
    if psc.IfPermitted("finance_invoice", "read") {
        toolbar = append(toolbar, amis.Button(amis.ButtonProps{
            Label:      "Export CSV",
            ActionType: "ajax",
            API:        "POST /api/v1/entities/finance_invoice/bulk-export",
            Level:      "default",
        }))
    }

    return psc.Marshal(amis.Page(amis.PageProps{
        Body: amis.CRUD(amis.CRUDProps{
            API:     "GET /api/v1/entities/finance_invoice",
            Toolbar: toolbar,
        }),
    }))
}
```

---

## 2. Row-Level Action (Custom Action Button)

```go
columns := []amis.Column{
    {Name: "number", Label: "Invoice #"},
    {Name: "status", Label: "Status"},
    {Name: "total_kes", Label: "Amount", Type: "currency"},
    // Action column — conditionally shown
    amis.ActionColumn([]amis.RowAction{
        // Submit action — only for Draft invoices, only if permitted
        amis.RowAction(amis.RowActionProps{
            Label:      "Submit",
            ActionType: "ajax",
            API:        "POST /api/v1/entities/finance_invoice/${id}/submit",
            Level:      "success",
            VisibleWhen: "${status === 'Draft'}",
            // Note: VisibleWhen is UX-only. Permission is enforced server-side.
            // Do not use VisibleWhen as a security control.
            ConfirmText: "Submit this invoice for approval?",
        }),
        // Approve action — only for Submitted invoices
        amis.RowAction(amis.RowActionProps{
            Label:      "Approve",
            ActionType: "ajax",
            API:        "POST /api/v1/entities/finance_invoice/${id}/approve",
            Level:      "primary",
            VisibleWhen: "${status === 'Submitted'}",
            ConfirmText: "Approve and post this invoice?",
        }),
        // Edit — always available
        amis.RowAction(amis.RowActionProps{
            Label:      "Edit",
            ActionType: "link",
            Link:       "/finance/invoice/${id}/edit",
        }),
    }),
}
```

---

## 3. Permission Gate on Row Actions

For actions that require specific roles, gate the entire action column:

```go
var actionColumn amis.Component
if psc.IfPermitted("finance_invoice", "submit") {
    // Include submit button in action column
    actionColumn = amis.ActionColumn([]amis.RowAction{
        submitAction,
        editAction,
    })
} else {
    // Read-only users get edit-only actions
    actionColumn = amis.ActionColumn([]amis.RowAction{
        viewAction,
    })
}
```

This ensures users without the submit permission never see the Submit button — it is **absent from the schema**, not just grayed out.

---

## 4. Confirmation Dialog Pattern

For destructive or irreversible actions:

```go
amis.Button(amis.ButtonProps{
    Label:      "Cancel Invoice",
    ActionType: "dialog",
    Dialog: amis.Dialog(amis.DialogProps{
        Title: "Cancel Invoice",
        Body: amis.Tpl("Are you sure you want to cancel invoice **${number}**? This cannot be undone."),
        Actions: []amis.DialogAction{
            {
                Label:      "Yes, Cancel",
                ActionType: "ajax",
                API:        "POST /api/v1/entities/finance_invoice/${id}/cancel",
                Level:      "danger",
                Close:      true,
            },
            {
                Label:      "Keep Invoice",
                ActionType: "close",
            },
        },
    }),
})
```

---

## 5. Action with Input Form

For actions that require additional input:

```go
// "Reject" action with required reason field
amis.Button(amis.ButtonProps{
    Label:      "Reject",
    ActionType: "dialog",
    Dialog: amis.Dialog(amis.DialogProps{
        Title: "Reject Invoice",
        Body: amis.Form(amis.FormProps{
            API: "POST /api/v1/entities/finance_invoice/${id}/reject",
            Body: []amis.FormField{
                amis.TextareaField("reason", "Rejection Reason").
                    Required(true).
                    Placeholder("Explain why this invoice is being rejected..."),
            },
            SubmitText: "Reject Invoice",
        }),
    }),
})
```

The `HandlerFunc` for the `reject` action receives the form data in `action.Input`:

```go
func RejectInvoiceAction(ctx context.Context, action def.ActionContext) (*def.ActionResult, error) {
    reason, _ := action.Input["reason"].(string)
    if reason == "" {
        return nil, &def.ValidationError{
            Fields: map[string]string{"reason": "Rejection reason is required"},
        }
    }
    // ... update invoice status and store reason
}
```

---

## 6. Bulk Action Button

For operating on selected list rows:

```go
amis.CRUD(amis.CRUDProps{
    API: "GET /api/v1/entities/finance_invoice",
    BulkActions: []amis.BulkAction{
        {
            Label:      "Bulk Submit",
            ActionType: "ajax",
            API:        "POST /api/v1/entities/finance_invoice/bulk",
            // amis sends: {"ids": ["uuid1", "uuid2", ...], "action": "submit"}
            ConfirmText: "Submit ${selectedItems.length} invoices for approval?",
        },
    },
    Checkable: true,
})
```

---

## 7. Status-Driven Button Visibility

For state-machine-based entities, use `VisibleWhen` to show only relevant actions:

```go
// Only show "Approve" when status is "Submitted" AND actor has approve role
// The role check is handled by psc.IfPermitted() in the page builder
// The status check is handled by VisibleWhen in amis
amis.RowAction(amis.RowActionProps{
    Label:       "Approve",
    VisibleWhen: "${status === 'Submitted'}",
    API:         "POST /api/v1/entities/finance_invoice/${id}/approve",
})
```

**Security note**: `VisibleWhen` is purely cosmetic. The server-side `HandlerFunc` MUST check the entity status and actor permissions independently. Never rely on `VisibleWhen` as a security gate.

---

## Related Documents

- [Actions](../04-domain/actions.md) — ActionDef and HandlerFunc implementation
- [Permission Sets](../04-domain/permissions.md) — `psc.IfPermitted` permission gating
- [List Patterns](list-patterns.md) — full list view configuration
- [Form Patterns](form-patterns.md) — input forms for action dialogs
