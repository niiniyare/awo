> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "List and Table Patterns"
id: sdui-006
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Page Builders](page-builders.md)"
  - "[Page Schema](page-schema.md)"
  - "[Cursors and Pagination](../05-persistence/cursors-and-pagination.md)"
  - "[Filter DSL](../05-persistence/filter-dsl.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# List and Table Patterns

**SDUI-006 | Status: Accepted | Stability: Stable**

This document describes patterns for list views, search bars, bulk actions, column customization, and row-level actions in Awo's SDUI layer.

---

## 1. Default List Generation

Every `EntityDefinition` auto-generates a list view displaying all non-`LongText`, non-`Sensitive` fields as columns. The auto-generated list includes:

- Pagination (page-number mode, 20 rows per page)
- Sort by any field (click column header)
- Global text search across all `Searchable()` fields
- Create button (if actor has `create` permission)
- Row-level edit and delete buttons (permission-gated)

Override only when the auto-generated list is insufficient.

---

## 2. Custom Column Configuration

```go
func BuildInvoiceListPage(ctx context.Context, actor session.Actor) ([]byte, error) {
    psc := sdui.NewPageSchemaContext(ctx, actor)

    crud := amis.CRUD(amis.CRUDProps{
        API:       "/api/v1/entities/finance_invoice",
        PageSize:  25,
        OrderBy:   "created_at",
        OrderDir:  "desc",
        Columns: []amis.Column{
            {Name: "number",     Label: "Invoice #",  Sortable: true, Width: 140},
            {Name: "customer",   Label: "Customer",   Sortable: true},
            {Name: "total_kes",  Label: "Total",      Sortable: true, Type: "currency", Align: "right"},
            {Name: "status",     Label: "Status",     Sortable: true, Type: "tag",
             BadgeMap: map[string]string{
                 "Draft":     "info",
                 "Submitted": "warning",
                 "Paid":      "success",
                 "Cancelled": "danger",
             }},
            {Name: "created_at", Label: "Created",    Sortable: true, Type: "datetime"},
            {Name: "_actions",   Label: "Actions",    Type: "operation",
             Buttons: buildRowActions(psc)},
        },
    })

    return psc.Marshal(crud)
}
```

---

## 3. Row-Level Actions

```go
func buildRowActions(psc *sdui.PageSchemaContext) []amis.RowButton {
    buttons := []amis.RowButton{
        {Label: "View", Type: "button", OnClick: amis.LinkAction{Href: "/finance/invoices/${id}"}},
    }

    if psc.IfPermitted("finance_invoice", "write") {
        buttons = append(buttons, amis.RowButton{
            Label: "Edit", Type: "button",
            OnClick: amis.LinkAction{Href: "/finance/invoices/${id}/edit"},
        })
    }

    if psc.IfPermitted("finance_invoice", "submit") {
        buttons = append(buttons, amis.RowButton{
            Label:   "Submit",
            Type:    "button",
            OnClick: amis.AjaxAction{
                API:     "POST /api/v1/entities/finance_invoice/${id}/submit",
                Confirm: &amis.ConfirmProps{Title: "Submit Invoice?"},
                Reload:  "self",
            },
            // Only show when status = "Draft"
            VisibleWhen: "${status === 'Draft'}",
        })
    }

    if psc.IfPermitted("finance_invoice", "delete") {
        buttons = append(buttons, amis.RowButton{
            Label:   "Delete",
            Type:    "danger",
            OnClick: amis.AjaxAction{
                API:     "DELETE /api/v1/entities/finance_invoice/${id}",
                Confirm: &amis.ConfirmProps{
                    Title:   "Delete Invoice?",
                    Content: "This will permanently delete the invoice.",
                },
                Reload: "self",
            },
            VisibleWhen: "${status === 'Draft'}",
        })
    }

    return buttons
}
```

Row buttons with `VisibleWhen` are evaluated client-side. Use them for UX convenience (hide irrelevant actions) but do not rely on them for authorization — the server enforces permissions on every action call.

---

## 4. Search and Filters

```go
crud := amis.CRUD(amis.CRUDProps{
    // ...
    FilterToolbar: amis.FilterToolbar{
        SearchFields: []string{"number", "customer.name"},  // global search targets
        Filters: []amis.FilterField{
            {
                Name:    "status",
                Label:   "Status",
                Type:    "select",
                Options: []string{"Draft", "Submitted", "Paid", "Cancelled"},
                Multiple: true,  // multi-select filter
            },
            {
                Name:  "created_at_gte",
                Label: "From Date",
                Type:  "date",
            },
            {
                Name:  "created_at_lte",
                Label: "To Date",
                Type:  "date",
            },
            {
                Name:    "customer",
                Label:   "Customer",
                Type:    "link",
                Entity:  "crm_customer",
            },
        },
    },
})
```

Filter values are serialized as query parameters on the CRUD API call. The entity list endpoint translates query parameters into `Filter` DSL predicates automatically.

Filter parameter conventions:
- `{field}` — exact match
- `{field}_gte` / `{field}_lte` — range bounds
- `{field}_contains` — substring match (requires `Searchable()` on the field)
- `{field}` as array — `IN` operator

---

## 5. Bulk Actions

```go
crud := amis.CRUD(amis.CRUDProps{
    // ...
    BulkActions: []amis.BulkAction{
        {
            Label:    "Export Selected",
            Icon:     "fa fa-download",
            OnClick:  amis.AjaxAction{
                API:    "POST /api/v1/entities/finance_invoice/export",
                Body:   "${selectedItems | pluck:'id'}",
                Download: true,
            },
        },
    },
})
```

Permission check on the bulk action endpoint: verify all selected IDs belong to the current tenant and actor has permission. The entity IDs are tenant-scoped by RLS, but the action endpoint must validate them explicitly.

---

## 6. Cursor Pagination for Large Datasets

For lists expected to exceed 10,000 records (data export, audit views, full ledger history):

```go
crud := amis.CRUD(amis.CRUDProps{
    API:            "/api/v1/entities/finance_ledger_entry",
    PaginationMode: "cursor",  // keyset pagination, no COUNT(*)
    PageSize:       100,
    OrderBy:        "created_at",
    OrderDir:       "desc",
    // No "page N of M" display — cursor mode has no total count
})
```

Use cursor mode explicitly. The default is page-number mode. Do not use cursor mode for tables where "showing X of Y" navigation is required.

---

## 7. Expandable Row Detail

For showing child records without navigating to a detail page:

```go
amis.Column{
    Name:      "_expand",
    Type:      "expand",
    Label:     "",
    Expandable: amis.ExpandableConfig{
        Body: amis.CRUD(amis.CRUDProps{
            API:      "/api/v1/entities/finance_invoice_line?invoice=${id}",
            ReadOnly: true,
            Columns:  []amis.Column{
                {Name: "description", Label: "Description"},
                {Name: "quantity",    Label: "Qty"},
                {Name: "unit_price",  Label: "Unit Price", Type: "currency"},
                {Name: "amount",      Label: "Amount",     Type: "currency"},
            },
        }),
    },
}
```

The expanded row's API call is made on demand (when the user clicks expand), not preloaded. This avoids N+1 on list load.

---

## 8. Column Visibility and Density Control

```go
crud := amis.CRUD(amis.CRUDProps{
    // ...
    HeaderToolbar: amis.HeaderToolbar{
        ColumnToggle:  true,   // user can show/hide columns
        DensityToggle: true,   // user can switch compact/comfortable/spacious
        ExportButton:  psc.IfPermitted("finance_invoice", "read"),
    },
})
```

Column visibility preferences are stored in browser local storage by amis — they persist between sessions without any server-side storage.

---

## 9. Linked Column Values

Show a column value as a clickable link:

```go
amis.Column{
    Name:  "number",
    Label: "Invoice #",
    Type:  "link",
    Href:  "/finance/invoices/${id}",
}
```

The `${id}` expression refers to the row's `id` field. Any field in the row can be referenced in `Href` using `${fieldName}`.

---

## 10. Anti-Patterns

### Fetching All Records

```go
// WRONG: no pagination = full table scan returned to browser
API: "/api/v1/entities/finance_invoice"
// With 500,000 rows this times out and crashes the browser tab

// CORRECT: always set PageSize
API: "/api/v1/entities/finance_invoice",
PageSize: 25,
```

### Authorization in VisibleWhen

```go
// WRONG: hiding delete button with VisibleWhen based on role
amis.RowButton{VisibleWhen: "${actor.roles.includes('admin')}"}
// VisibleWhen is client-side and can be bypassed in browser DevTools

// CORRECT: exclude the button from schema entirely if no permission
if psc.IfPermitted("finance_invoice", "delete") {
    buttons = append(buttons, deleteButton)
}
```

### Custom Columns for Sensitive Fields

```go
// WRONG: custom column showing sensitive field
amis.Column{Name: "password_hint", Label: "Password Hint"}
// Fields marked Sensitive are excluded from list API responses; column will always be blank

// Fields marked Sensitive are excluded from all API responses unless
// explicitly masked — they are not fetchable in list views
```

---

## Related Documents

- [Page Builders](page-builders.md) — `PageBuilderSet`, `PageSchemaContext`
- [Cursors and Pagination](../05-persistence/cursors-and-pagination.md) — cursor vs page-number modes
- [Filter DSL](../05-persistence/filter-dsl.md) — how filter query params are interpreted
- [API Conventions](../11-api/conventions.md) — list endpoint response envelope
- [Glossary](../GLOSSARY.md) — SDUI, amis, Cursor Pagination, Page-Number Pagination
