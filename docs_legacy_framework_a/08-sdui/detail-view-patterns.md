> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Detail View Patterns"
id: sdui-008
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Page Builders](page-builders.md)"
  - "[List Patterns](list-patterns.md)"
  - "[Form Patterns](form-patterns.md)"
  - "[Action Buttons](action-buttons.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Detail View Patterns

**SDUI-008 | Status: Accepted | Stability: Stable**

Patterns for entity detail views: field groups, related records panels, action buttons, and tabbed layouts.

---

## 1. Basic Detail View

The auto-generated detail view shows all fields. To customize:

```go
PageBuilders: def.PageBuilderSet{
    Detail: BuildInvoiceDetailPage,
},
```

```go
func BuildInvoiceDetailPage(ctx context.Context, actor session.Actor) ([]byte, error) {
    psc := sdui.NewPageSchemaContext(ctx, actor)

    return psc.Marshal(amis.Page(amis.PageProps{
        Title: "Invoice ${number}",
        Body: amis.Detail(amis.DetailProps{
            API: "GET /api/v1/entities/finance_invoice/${id}",
            Body: []amis.DetailField{
                amis.DetailField("number", "Invoice #"),
                amis.DetailField("customer_name", "Customer"),  // from linked entity
                amis.DetailField("status", "Status", amis.WithBadge()),
                amis.DetailField("total_kes", "Total", amis.WithCurrencyFormat()),
                amis.DetailField("created_at", "Created", amis.WithDateTimeFormat()),
                amis.DetailField("notes", "Notes"),
            },
        }),
    }))
}
```

---

## 2. Field Groups

Organize related fields visually:

```go
amis.Detail(amis.DetailProps{
    API: "GET /api/v1/entities/finance_invoice/${id}",
    Body: []amis.Component{
        amis.FieldGroup("Invoice Details", []amis.DetailField{
            amis.DetailField("number", "Invoice #"),
            amis.DetailField("status", "Status"),
            amis.DetailField("invoice_date", "Invoice Date"),
        }),
        amis.FieldGroup("Customer", []amis.DetailField{
            amis.DetailField("customer_name", "Name"),
            amis.DetailField("customer_email", "Email"),
            amis.DetailField("payment_terms", "Payment Terms (days)"),
        }),
        amis.FieldGroup("Amounts", []amis.DetailField{
            amis.DetailField("subtotal_kes", "Subtotal"),
            amis.DetailField("vat_kes", "VAT (16%)"),
            amis.DetailField("total_kes", "Total"),
        }),
    },
})
```

---

## 3. Tabbed Detail with Related Records

```go
return psc.Marshal(amis.Page(amis.PageProps{
    Title: "Invoice ${number}",
    Toolbar: buildInvoiceToolbar(psc),
    Body: amis.Tabs([]amis.Tab{
        {
            Title: "Details",
            Body: amis.Detail(amis.DetailProps{
                API: "GET /api/v1/entities/finance_invoice/${id}?with=customer",
            }),
        },
        {
            Title: "Line Items",
            Body: amis.CRUD(amis.CRUDProps{
                API: "GET /api/v1/entities/finance_invoice_line?invoice=${id}",
                Columns: []amis.Column{
                    {Name: "description", Label: "Description"},
                    {Name: "quantity", Label: "Qty", Type: "number"},
                    {Name: "unit_price", Label: "Unit Price", Type: "currency"},
                    {Name: "total", Label: "Total", Type: "currency"},
                },
                // Read-only in context of invoice detail
                ReadOnly: true,
            }),
        },
        {
            Title: "Change History",
            Body: amis.CRUD(amis.CRUDProps{
                API: "GET /api/v1/entities/iam_audit_log?entity_type=finance_invoice&entity_id=${id}",
                Columns: []amis.Column{
                    {Name: "occurred_at", Label: "When", Type: "datetime"},
                    {Name: "actor_id", Label: "By"},
                    {Name: "action", Label: "Action"},
                    {Name: "changed_fields", Label: "Fields", Type: "json"},
                },
                // No create/edit/delete in audit log view
                ReadOnly: true,
            }),
        },
    }),
}))
```

---

## 4. Status Badge with Color Mapping

```go
amis.DetailField("status", "Status",
    amis.WithMapping(map[string]amis.MappingValue{
        "Draft":     {Label: "Draft",     Level: "default"},
        "Submitted": {Label: "Submitted", Level: "info"},
        "Approved":  {Label: "Approved",  Level: "success"},
        "Rejected":  {Label: "Rejected",  Level: "danger"},
        "Cancelled": {Label: "Cancelled", Level: "warning"},
    }),
)
```

---

## 5. Computed Summary Row

For entities with child line items, show a summary:

```go
amis.Panel("Summary",
    amis.Container([]amis.Component{
        amis.Stat(amis.StatProps{
            API:   "GET /api/v1/entities/finance_invoice/${id}",
            Items: []amis.StatItem{
                {Name: "total_kes", Label: "Invoice Total", Format: "currency"},
                {Name: "paid_kes", Label: "Paid", Format: "currency"},
                {Name: "balance_kes", Label: "Balance Due", Format: "currency"},
            },
        }),
    }),
)
```

---

## 6. Inline Edit (Quick Update)

For editable fields in the detail view without navigating to full edit form:

```go
amis.DetailField("payment_terms", "Payment Terms",
    amis.Editable(amis.EditableProps{
        API:    "PATCH /api/v1/entities/finance_invoice/${id}",
        Field:  "payment_terms",
        Type:   "number",
    }),
)
```

Only use for simple fields. Complex updates require the full edit form.

---

## 7. Print / PDF Button

```go
amis.Button(amis.ButtonProps{
    Label:      "Print Invoice",
    ActionType: "url",
    URL:        "/api/v1/entities/finance_invoice/${id}/pdf",
    Blank:      true,  // Opens in new tab
    Level:      "default",
})
```

The PDF endpoint is a custom route on the entity returning `Content-Type: application/pdf`.

---

## 8. Breadcrumb Navigation

```go
amis.Page(amis.PageProps{
    Title: "Invoice ${number}",
    Breadcrumb: []amis.BreadcrumbItem{
        {Label: "Finance", Link: "/finance"},
        {Label: "Invoices", Link: "/finance/invoice"},
        {Label: "${number}"},  // Current page — no link
    },
    Body: /* ... */,
})
```

---

## Related Documents

- [Page Builders](page-builders.md) — PageBuilderSet and the `Detail` builder slot
- [List Patterns](list-patterns.md) — how users navigate to the detail view
- [Form Patterns](form-patterns.md) — edit form accessed from detail view
- [Action Buttons](action-buttons.md) — toolbar and detail-level action buttons
