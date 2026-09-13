> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Report View Patterns"
id: sdui-009
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: normative
related:
  - "[SDUI Overview](sdui-overview.md)"
  - "[CRUD Patterns](crud-patterns.md)"
  - "[Detail View Patterns](detail-view-patterns.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Report View Patterns

**SDUI-009 | Status: Accepted | Stability: Stable**

Building report pages in amis: summary cards, data tables with aggregations, charts, date range filters, and export buttons.

---

## 1. Basic Report Page Structure

A report page is a `page` schema with a filter toolbar, summary stats row, and a data table:

```go
func BuildInvoiceReportPage(ctx context.Context, actor session.Actor) ([]byte, error) {
    psc := sdui.NewPageSchemaContext(ctx, actor)

    page := amis.Page{
        Title: "Invoice Report",
        Body: []any{
            buildReportFilterToolbar(),
            buildSummaryStats(),
            buildInvoiceReportTable(psc),
        },
    }

    return json.Marshal(page)
}
```

---

## 2. Filter Toolbar

Use an `amis.Form` with `wrapWithPanel: false` and `submitOnChange: true` so filters apply immediately:

```go
func buildReportFilterToolbar() amis.Form {
    return amis.Form{
        Type:           "form",
        WrapWithPanel:  false,
        SubmitOnChange: true,
        Target:         "report-table",     // drives the CRUD component below
        Body: []any{
            amis.DateRangePicker{
                Type:        "input-date-range",
                Name:        "date_range",
                Label:       "Invoice Date",
                StartPlaceholder: "From",
                EndPlaceholder:   "To",
                Format:      "YYYY-MM-DD",
                Value:       "thisMonth",   // default: current month
            },
            amis.Select{
                Type:    "select",
                Name:    "status",
                Label:   "Status",
                Options: []amis.Option{
                    {Label: "All",       Value: ""},
                    {Label: "Draft",     Value: "Draft"},
                    {Label: "Submitted", Value: "Submitted"},
                    {Label: "Paid",      Value: "Paid"},
                    {Label: "Cancelled", Value: "Cancelled"},
                },
                Value: "",
            },
            amis.SelectRemote{
                Type:      "select",
                Name:      "customer",
                Label:     "Customer",
                Source:    "/api/v1/entities/crm_customer?fields=id,name",
                LabelField: "name",
                ValueField: "id",
                Clearable:  true,
            },
        },
    }
}
```

---

## 3. Summary Stats Row

Use `amis.Grid` with `amis.Stat` components. Values come from a service endpoint:

```go
func buildSummaryStats() amis.Service {
    return amis.Service{
        Type: "service",
        API: amis.API{
            URL:    "/api/v1/reports/finance/invoice-summary",
            Method: "get",
            Data: map[string]any{
                "date_range": "${date_range}",
                "status":     "${status}",
                "customer":   "${customer}",
            },
        },
        Body: amis.Grid{
            Type: "grid",
            Columns: []amis.GridColumn{
                {Body: amis.Stat{Type: "stat", Title: "Total Invoices", Value: "${total_count}"}},
                {Body: amis.Stat{Type: "stat", Title: "Total Amount", Value: "${total_amount_formatted}"}},
                {Body: amis.Stat{Type: "stat", Title: "Paid", Value: "${paid_amount_formatted}", Style: map[string]any{"color": "#52c41a"}}},
                {Body: amis.Stat{Type: "stat", Title: "Outstanding", Value: "${outstanding_formatted}", Style: map[string]any{"color": "#ff4d4f"}}},
            },
        },
    }
}
```

The `/api/v1/reports/finance/invoice-summary` handler uses `EntityRepository.Aggregate`:

```go
func (h *ReportHandler) InvoiceSummary(c *fiber.Ctx) error {
    ctx := c.UserContext()
    f := buildInvoiceFilter(c)     // parse date_range, status, customer from query

    total, _, _ := h.invoiceRepo.Query(ctx, f, def.WithCount())
    sumResult, _ := h.invoiceRepo.Aggregate(ctx, f, def.AggregateSpec{
        Sums: []string{"total_kes"},
        GroupBy: []def.GroupBySpec{
            {Field: "status"},
        },
    })

    return c.JSON(fiber.Map{
        "data": fiber.Map{
            "total_count":           total,
            "total_amount_formatted": formatKES(sumResult.Sums["total_kes"]),
            "paid_amount_formatted":  formatKES(sumResult.GroupSums["Paid"]["total_kes"]),
            "outstanding_formatted":  formatKES(sumResult.GroupSums["Submitted"]["total_kes"]),
        },
    })
}
```

---

## 4. Report Table with Aggregation Footer

```go
func buildInvoiceReportTable(psc *sdui.PageSchemaContext) amis.CRUD {
    return amis.CRUD{
        Type:    "crud",
        Name:    "report-table",
        API: amis.API{
            URL:    "/api/v1/entities/finance_invoice",
            Method: "get",
            Data: map[string]any{
                "filter": map[string]any{
                    "and": []any{
                        map[string]any{"field": "invoice_date", "op": "between", "value": "${date_range}"},
                        map[string]any{"field": "status", "op": "eq", "value": "${status}"},
                        map[string]any{"field": "customer", "op": "eq", "value": "${customer}"},
                    },
                },
                "sort":  "invoice_date",
                "order": "desc",
            },
        },
        Columns: []amis.Column{
            {Name: "number",       Label: "Invoice No.",   Sortable: true},
            {Name: "customer_name", Label: "Customer"},
            {Name: "invoice_date", Label: "Date",          Type: "date", Format: "DD MMM YYYY", Sortable: true},
            {Name: "status",       Label: "Status",        Type: "mapping", Map: invoiceStatusBadges()},
            {Name: "total_kes",    Label: "Amount (KES)",  Type: "number", Precision: 2, Sortable: true, Align: "right"},
        },
        FooterToolbar: []any{
            amis.ColumnToggle{Type: "column-toggler"},
            "statistics",  // built-in amis footer showing count + sum
            "pagination",
        },
    }
}
```

---

## 5. Chart Integration

For trend charts use `amis.Chart` (ECharts wrapper). Data comes from a dedicated aggregation endpoint:

```go
amis.Chart{
    Type: "chart",
    API:  "/api/v1/reports/finance/invoice-trend?date_range=${date_range}",
    Config: map[string]any{
        "xAxis": map[string]any{
            "type": "category",
            "data": "${xAxis}",
        },
        "yAxis": map[string]any{"type": "value"},
        "series": []map[string]any{
            {
                "name": "Invoiced",
                "type": "bar",
                "data": "${invoiced}",
            },
            {
                "name": "Paid",
                "type": "bar",
                "data": "${paid}",
            },
        },
        "legend": map[string]any{"show": true},
        "tooltip": map[string]any{"trigger": "axis"},
    },
}
```

The handler returns:

```json
{
  "data": {
    "xAxis": ["Jan", "Feb", "Mar", "Apr", "May"],
    "invoiced": [450000, 520000, 380000, 610000, 490000],
    "paid": [420000, 500000, 350000, 590000, 460000]
  }
}
```

---

## 6. Export Button

Add a CSV/Excel export button to the toolbar:

```go
amis.Button{
    Type:  "button",
    Label: "Export CSV",
    Icon:  "fa fa-download",
    OnClick: amis.Action{
        ActionType: "ajax",
        API: amis.API{
            URL:          "/api/v1/reports/finance/invoice-export",
            Method:       "get",
            ResponseType: "blob",
            Data: map[string]any{
                "date_range": "${date_range}",
                "status":     "${status}",
                "customer":   "${customer}",
                "format":     "csv",
            },
        },
        FileName: "invoices_${date_range}.csv",
    },
}
```

Export handler streams CSV via Fiber's streaming response — runs in a Temporal activity for large exports (>10K rows).

---

## 7. Date Range Presets

amis `input-date-range` supports shorthand presets:

| Value | Meaning |
|---|---|
| `today` | Current day |
| `yesterday` | Previous day |
| `thisWeek` | Mon–today |
| `lastWeek` | Full previous week |
| `thisMonth` | 1st–today |
| `lastMonth` | Full previous month |
| `thisQuarter` | Current quarter |
| `thisYear` | 1 Jan–today |

Default: `thisMonth` — sensible for most financial reports.

---

## 8. Print / PDF

For printable reports, add a print button that opens a `drawer` with a print-optimized layout:

```go
amis.Button{
    Type:  "button",
    Label: "Print Report",
    Icon:  "fa fa-print",
    OnClick: amis.Action{
        ActionType: "drawer",
        Drawer: amis.Drawer{
            Title: "Print Preview",
            Size:  "xl",
            Body: amis.IFrame{
                Type:   "iframe",
                Src:    "/api/v1/reports/finance/invoice-print?${filters}",
                Height: "80vh",
            },
            Actions: []amis.Action{
                {ActionType: "custom", Label: "Print",
                    OnClick: "window.frames[0].print()"},
                {ActionType: "close", Label: "Close"},
            },
        },
    },
}
```

The print endpoint returns a standalone HTML page with print CSS — no amis runtime needed in print context.

---

## Related Documents

- [SDUI Overview](sdui-overview.md) — amis architecture, rendering pipeline
- [CRUD Patterns](crud-patterns.md) — list views with filters
- [Detail View Patterns](detail-view-patterns.md) — record detail customization
- [Action Buttons](action-buttons.md) — toolbar and row actions
- [Metrics Reference](../13-observability/metrics-reference.md) — instrument report handler latency
