# Chapter 12 — Tables and Data Grids

> **Volume:** III — Component System
> **Phase:** 2
> **Audience:** Backend Engineers, Web/Mobile Engineers, ERP Implementers
> **Prerequisites:** Chapters 01–11

---

## Table of Contents

- [12.1 Table vs. Data Grid Philosophy](#121-table-vs-data-grid-philosophy)
- [12.2 Table Node Schema](#122-table-node-schema)
- [12.3 Column Types](#123-column-types)
- [12.4 Table Features](#124-table-features)
- [12.5 Table Permissions](#125-table-permissions)
- [12.6 ERP-Specific Table Patterns](#126-erp-specific-table-patterns)
- [12.7 Table Performance Considerations](#127-table-performance-considerations)
- [12.8 Mobile Table Adaptations](#128-mobile-table-adaptations)

---

## 12.1 Table vs. Data Grid Philosophy

AwoERP provides a single, unified table component (`ui.data_table`) rather than separate "table" and "data grid" components. The distinction between a simple display table and an interactive data grid is made through the table's feature configuration — not through separate component types. This prevents the proliferation of similar components that diverge over time.

A table with no sorting, no filtering, no selection, and no inline editing is a display table. The same component with those features enabled becomes a full data grid. The backend engineer enables features by including their configuration in the table definition; the rendering engine activates those features in its implementation.

This unified approach has one important consequence: the rendering engine must implement the full feature set. A rendering engine cannot claim to support `ui.data_table` if it only supports the display-only subset. Feature configuration in the AST declares intent; the rendering engine either supports the declared features or reports an unsupported fallback.

---

## 12.2 Table Node Schema

### 12.2.1 Column Definitions

Columns are the heart of a table definition. Each column defines what data it displays, how it renders, and how it behaves.

**`ui.data_table`** — The unified table/data grid component.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `columns` | array of ColumnDef | ✅ | — | Ordered column definitions |
| `data_source` | DataSourceRef | ✅ | — | The data source providing rows |
| `row_key` | string | ✅ | — | The field name used as a stable unique key for each row |
| `empty_state` | Node | | *(default empty state)* | Rendered when the data source returns no rows |
| `loading_state` | Node | | *(default loading state)* | Rendered while the data source is loading |
| `caption` | LocalizedString | | — | Accessibility caption for the table |
| `density` | `compact`\|`normal`\|`comfortable` | | *(inherits from page)* | Row height and padding |
| `striped` | boolean | | `false` | Alternating row background colors |
| `bordered` | boolean | | `false` | Cell borders |
| `hover_highlight` | boolean | | `true` | Row highlight on hover |
| `sticky_header` | boolean | | `true` | Whether the column header row remains visible while scrolling |
| `sticky_first_column` | boolean | | `false` | Freeze the first column during horizontal scroll |
| `max_height` | string | | — | If set, the table scrolls vertically within this height |

### 12.2.2 Row Data Source Binding

The table's `data_source` prop references a `data.*` node declared in the surface. The data source is responsible for fetching, paginating, filtering, and sorting the row data. The table is a display component — it renders what the data source provides.

```go
// Declare the data source
ast.NewRESTDataSource("purchase-orders-source").
    WithEndpoint("/api/procurement/purchase-orders").
    WithPaginationMode(ast.CursorPagination).
    WithDefaultSort("created_at", ast.Descending).
    WithParams(map[string]any{
        "tenant_id": ast.ContextBinding("tenant.id"),
        "status": ast.StateBinding("filter_status"),
    }),

// Reference it from the table
ast.NewDataTable("po-list-table").
    WithDataSource(ast.DataSourceRef("purchase-orders-source")).
    WithRowKey("id").
    WithColumns( ...column definitions... )
```

### 12.2.3 Key Field Configuration

The `row_key` field must uniquely identify each row within the current dataset. It is used by the rendering engine for:
- Efficient re-rendering (only changed rows are updated)
- Row selection state tracking (selected row IDs are stable even after sort/filter changes)
- Inline editing state (edit state is keyed by row ID)
- Accessibility (`aria-rowindex` and keyboard navigation)

---

## 12.3 Column Types

All column definitions share a base set of props. Individual column types extend this base.

**Base column props (inherited by all column types):**

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | ✅ | — | Unique column identifier within the table |
| `header` | LocalizedString | ✅ | — | Column header label |
| `field` | string | ✅ | — | The row data field this column displays |
| `width` | string | | — | Fixed column width (`"px"` or `"%"`). If unset, column shares remaining space |
| `min_width` | string | | `"80px"` | |
| `max_width` | string | | — | |
| `align` | `left`\|`center`\|`right` | | `left` | |
| `sortable` | boolean | | `false` | |
| `resizable` | boolean | | `true` | |
| `hidden` | boolean\|Binding | | `false` | |
| `frozen` | `left`\|`right`\|`none` | | `none` | Sticky column (frozen during horizontal scroll) |

### 12.3.1 Text Column

**`column.text`** — Displays a plain text value.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `truncate` | boolean | | `true` | Truncate with ellipsis if text overflows cell |
| `tooltip_on_truncate` | boolean | | `true` | Show full value in tooltip when truncated |
| `copyable` | boolean | | `false` | Shows a copy-to-clipboard icon on hover |
| `prefix` | LocalizedString | | — | |
| `suffix` | LocalizedString | | — | |
| `empty_value` | LocalizedString | | `"—"` | Displayed when the field value is null or empty |
| `transform` | `none`\|`uppercase`\|`lowercase`\|`capitalize` | | `none` | |

### 12.3.2 Number / Currency Column

**`column.number`** — Displays a numeric value with formatting.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `format` | `plain`\|`currency`\|`percentage`\|`scientific` | | `plain` | |
| `currency_field` | string | | — | Row field containing the currency code (for `format: "currency"`) |
| `currency_code` | string | | — | Static currency code |
| `decimal_places` | integer | | 2 | |
| `show_sign` | boolean | | `false` | Always show +/– sign |
| `color_negative` | boolean | | `false` | Render negative values in `token:status.error` color |
| `aggregate` | `sum`\|`avg`\|`min`\|`max`\|`count` | | — | Aggregate function for the totals row |

### 12.3.3 Date / DateTime Column

**`column.date`** — Displays a date or datetime value.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `format` | `date`\|`datetime`\|`time`\|`relative`\|`calendar` | | `date` | |
| `relative_threshold_days` | integer | | 7 | Days within which `relative` format shows "2 days ago" instead of the full date |
| `timezone` | IANA timezone | | *(user locale timezone)* | |
| `tooltip` | `full_datetime`\|`none` | | `full_datetime` | Tooltip showing full datetime when format is abbreviated |

### 12.3.4 Boolean / Status Column

**`column.boolean`** — Renders a boolean value as an icon or badge.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `true_label` | LocalizedString | | `"Yes"` | |
| `false_label` | LocalizedString | | `"No"` | |
| `display` | `icon`\|`badge`\|`text`\|`check` | | `check` | |
| `true_icon` | IconRef | | `icon:check` | |
| `false_icon` | IconRef | | `icon:x` | |

**`column.status`** — Renders an enum status value as a colored badge. The most common column type for ERP document lists (PO Status, Invoice Status, etc.).

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `status_map` | map of value → StatusConfig | ✅ | — | Maps each possible status value to a display configuration |
| `default_status` | StatusConfig | | — | Fallback for unrecognized values |

**StatusConfig:** `{ label: LocalizedString, variant: "default"|"success"|"warning"|"error"|"info"|"neutral", icon?: IconRef }`

```go
ast.NewStatusColumn("status").
    WithHeader(i18n.Key("column.status")).
    WithStatusMap(map[string]ast.StatusConfig{
        "draft":     {Label: i18n.Key("po.status.draft"),     Variant: ast.StatusNeutral},
        "pending":   {Label: i18n.Key("po.status.pending"),   Variant: ast.StatusWarning},
        "approved":  {Label: i18n.Key("po.status.approved"),  Variant: ast.StatusSuccess},
        "rejected":  {Label: i18n.Key("po.status.rejected"),  Variant: ast.StatusError},
        "cancelled": {Label: i18n.Key("po.status.cancelled"), Variant: ast.StatusNeutral},
    })
```

### 12.3.5 Link Column

**`column.link`** — Renders a clickable text or button that triggers a navigation action.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `action_ref` | ActionRef | ✅ | — | Action to execute on click (typically `action.navigate`) |
| `action_params` | map of string → Binding | | — | Dynamic params derived from row data bound to the action |
| `display_field` | string | | *(same as `field`)* | The field to display as link text (if different from the value field) |
| `underline` | boolean | | `true` | |

### 12.3.6 Action Column

**`column.actions`** — Renders a set of per-row action buttons or an action menu.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `actions` | array of RowActionDef | ✅ | — | |
| `display` | `buttons`\|`menu`\|`auto` | | `auto` | `auto`: buttons if ≤2 actions, menu if >2 |
| `menu_trigger_icon` | IconRef | | `icon:ellipsis-vertical` | |
| `header` | LocalizedString | | `""` | Empty header is standard for action columns |

**RowActionDef:**
```go
type RowActionDef struct {
    ID          string
    Label       LocalizedString
    Icon        *IconRef
    ActionRef   ActionRef           // references an action with row data params
    Variant     ActionVariant       // default, danger
    Permission  *PermissionRef      // per-row permission check using row data
    ShowIf      *Expression         // show condition evaluated against row data
    ConfirmLabel *LocalizedString   // if set, shows a confirmation dialog before executing
}
```

### 12.3.7 Badge / Tag Column

**`column.badge`** — Renders a single badge value.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `variant` | `default`\|`success`\|`warning`\|`error`\|`info`\|Binding | | `default` | Can be bound to a row field for dynamic variant |

**`column.tags`** — Renders multiple badge values from an array field.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `max_visible` | integer | | 3 | Tags beyond this count are collapsed into a "+N more" indicator |
| `tag_label_field` | string | | `"label"` | Field within each tag object for display text |
| `tag_color_field` | string | | — | Field within each tag object for color token |

### 12.3.8 Custom Render Column

**`column.custom`** — Renders an arbitrary node template per row. The most flexible column type — use when no standard column type meets the requirement.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `cell_template` | Node | ✅ | — | A node subtree rendered for each row. Row field values are accessible via `$row.field_name` bindings within the template |

```go
ast.NewCustomColumn("vendor-info").
    WithHeader(i18n.Key("column.vendor")).
    WithCellTemplate(
        ast.NewStackLayout(ast.Horizontal).WithGap(ast.GapSM).
            AddChild(
                ast.NewAvatar("vendor-avatar").
                    WithSrc(ast.RowBinding("vendor.logo_url")).
                    WithInitials(ast.RowBinding("vendor.initials")).
                    WithSize(ast.AvatarSM),
            ).
            AddChild(
                ast.NewStackLayout(ast.Vertical).WithGap(ast.GapXS).
                    AddChild(ast.NewText("vendor-name").
                        WithContent(ast.RowBinding("vendor.name")).
                        WithVariant(ast.TextBody1),
                    ).
                    AddChild(ast.NewText("vendor-code").
                        WithContent(ast.RowBinding("vendor.code")).
                        WithVariant(ast.TextCaption).
                        WithColor(ast.TokenTextSecondary),
                    ),
            ),
    )
```

---

## 12.4 Table Features

### 12.4.1 Sorting (Client vs. Server)

```go
ast.NewDataTable("po-list").
    WithSorting(ast.SortingConfig{
        Mode:           ast.ServerSort,      // or ast.ClientSort
        DefaultField:   "created_at",
        DefaultOrder:   ast.Descending,
        MultiSort:      false,               // whether multiple columns can be sorted simultaneously
        SortParamName:  "sort_by",           // query param name sent to the data source
        OrderParamName: "sort_order",        // query param name for asc/desc
    })
```

**Client sort:** The rendering engine sorts the already-loaded rows. Appropriate for small datasets (< 500 rows) where all rows are loaded at once.

**Server sort:** The rendering engine updates the data source's query params and triggers a reload. Required for paginated datasets.

### 12.4.2 Filtering (Column-Level, Global)

```go
ast.NewDataTable("po-list").
    WithFiltering(ast.FilteringConfig{
        GlobalSearch:    true,
        GlobalSearchParamName: "q",
        GlobalSearchPlaceholder: i18n.Key("table.search.placeholder"),
        ColumnFilters:   true,
        FilterParamPrefix: "filter_",       // column filter params: filter_status, filter_vendor_id
        FilterDebounceMs: 400,
    }).
    WithColumnFilter("status",
        ast.SelectFilter().WithOptions([]ast.SelectOption{
            {Value: "pending",  Label: i18n.Key("po.status.pending")},
            {Value: "approved", Label: i18n.Key("po.status.approved")},
        }),
    ).
    WithColumnFilter("created_at",
        ast.DateRangeFilter().WithPresets(ast.StandardDatePresets()),
    ).
    WithColumnFilter("vendor_id",
        ast.LookupFilter().WithSearchEndpoint("/api/vendors/search"),
    )
```

### 12.4.3 Pagination (Offset, Cursor, Infinite Scroll)

```go
// Offset pagination — traditional page numbers
ast.PaginationConfig{
    Mode:           ast.OffsetPagination,
    PageSizes:      []int{25, 50, 100},
    DefaultPageSize: 25,
    PageParamName:  "page",
    SizeParamName:  "page_size",
    ShowPageInfo:   true,           // "Showing 1–25 of 142"
}

// Cursor pagination — "Load more" / infinite scroll
ast.PaginationConfig{
    Mode:           ast.CursorPagination,
    CursorParamName: "cursor",
    PageSize:       50,
    InfiniteScroll: true,           // auto-load on scroll near bottom
}
```

### 12.4.4 Row Selection (Single, Multi)

```go
ast.NewDataTable("po-list").
    WithSelection(ast.SelectionConfig{
        Mode:           ast.MultiSelect,    // or ast.SingleSelect
        SelectAllEnabled: true,
        SelectAllScope:  ast.SelectAllPage, // or ast.SelectAllAll (selects across pages)
        SelectedStateVariable: "selected_po_ids",
        OnSelectionChange: []ast.ActionRef{ast.ActionRef("action-update-bulk-bar")},
    })
```

Selected row IDs are stored in the named state variable and available to bulk action bindings.

### 12.4.5 Row Expansion / Details Panel

```go
ast.NewDataTable("po-list").
    WithRowExpansion(ast.RowExpansionConfig{
        Mode:            ast.SingleExpand,  // or ast.MultiExpand
        ExpansionTemplate: ast.NewStackLayout(ast.Vertical).
            AddChild( ...inline detail content... ),
        LazyLoad:        true,             // load expansion content on-demand
        ExpansionSurfaceRef: "procurement.purchase-order.inline-detail",
    })
```

When `ExpansionSurfaceRef` is set, the expanded row content is loaded from a named sub-surface, passing the row's key as a parameter. This allows complex expansion content without bloating the parent table's payload.

### 12.4.6 Frozen Columns

Any column can be frozen to the left or right edge of the table, remaining visible during horizontal scroll. Configured per-column via the `frozen` prop.

```go
ast.NewTextColumn("po_number").WithFrozen(ast.FrozenLeft)
ast.NewActionsColumn("row-actions").WithFrozen(ast.FrozenRight)
```

### 12.4.7 Column Reordering (Persisted per User)

```go
ast.NewDataTable("po-list").
    WithColumnReordering(ast.ColumnReorderingConfig{
        Enabled:         true,
        PersistKey:      "procurement.po-list.column-order",
        // User's saved column order is stored in the user preferences store
        // and loaded on subsequent visits
    })
```

### 12.4.8 Column Visibility Toggle

```go
ast.NewDataTable("po-list").
    WithColumnVisibility(ast.ColumnVisibilityConfig{
        Enabled:         true,
        PersistKey:      "procurement.po-list.column-visibility",
        DefaultHidden:   []string{"internal_reference", "created_by"},
    })
```

When column visibility is enabled, the table renders a column picker control (a button that opens a checklist of column names) in the table toolbar.

### 12.4.9 Row Grouping

```go
ast.NewDataTable("ledger-table").
    WithGrouping(ast.GroupingConfig{
        GroupByField:    "account_code",
        GroupHeaderTemplate: ast.NewStackLayout(ast.Horizontal).
            AddChild(ast.NewText("group-label").
                WithContent(ast.RowBinding("$group.key")).
                WithVariant(ast.TextBody2Bold),
            ).
            AddChild(ast.NewText("group-count").
                WithContent(ast.Expr(`concat("(", count($group.rows), " entries)")`)),
            ),
        DefaultExpanded: true,
        SubtotalsEnabled: true,
    })
```

### 12.4.10 Aggregation Rows

```go
ast.NewDataTable("po-line-items").
    WithAggregationRow(ast.AggregationRowConfig{
        Position:  ast.AggregationBottom,
        Label:     i18n.Key("table.totals"),
        Columns: map[string]ast.AggregationFunc{
            "quantity":   ast.AggSum,
            "unit_price": ast.AggAvg,
            "line_total": ast.AggSum,
        },
    })
```

### 12.4.11 Inline Editing

```go
ast.NewDataTable("inventory-adjustments").
    WithInlineEditing(ast.InlineEditingConfig{
        Mode:             ast.ClickToEdit,     // or ast.AlwaysEdit
        EditableColumns:  []string{"quantity", "unit_cost"},
        SaveMode:         ast.SaveOnBlur,      // or ast.SaveOnEnter, ast.SaveManually
        SaveEndpoint:     "PATCH /api/inventory/adjustments/{id}",
        OptimisticUpdate: true,
    }).
    WithColumn(ast.NewNumberColumn("quantity").WithInlineEdit(
        ast.InlineEditConfig{
            FieldType:      ast.FieldNumber,
            Min:            0,
            ValidationRules: []ast.ValidationRule{ast.RuleMin(0)},
        },
    ))
```

### 12.4.12 Row-Level Actions

Row-level actions are defined in `column.actions` (see Section 12.3.6). Actions receive the row's key and optionally the full row data as parameters, passed via `action_params`.

```go
ast.NewActionsColumn("po-row-actions").
    AddAction(ast.RowAction("view-po").
        WithLabel(i18n.Key("action.view")).
        WithIcon(ast.IconEye).
        WithActionRef("action-navigate-to-po-detail").
        WithActionParams(map[string]ast.Binding{
            "po_id": ast.RowBinding("id"),
        }),
    ).
    AddAction(ast.RowAction("approve-po").
        WithLabel(i18n.Key("action.approve")).
        WithIcon(ast.IconCheck).
        WithVariant(ast.ActionDanger).
        WithActionRef("action-approve-po").
        WithPermission("procurement:po:approve").
        WithShowIf(ast.RowBinding("status").EqualTo("pending")).
        WithConfirmLabel(i18n.Key("action.approve.confirm")),
    )
```

### 12.4.13 Bulk Actions on Selected Rows

```go
ast.NewDataTable("po-list").
    WithBulkActions(ast.BulkActionsConfig{
        StateVariable:   "selected_po_ids",
        Actions: []ast.BulkActionDef{
            {
                ID:          "bulk-approve",
                Label:       i18n.Key("action.bulk_approve"),
                Icon:        ast.IconCheckCircle,
                Permission:  "procurement:po:approve",
                ActionRef:   "action-bulk-approve-pos",
                ConfirmLabel: i18n.Key("action.bulk_approve.confirm"),
                MinSelected: 1,
            },
            {
                ID:          "bulk-export",
                Label:       i18n.Key("action.export_selected"),
                Icon:        ast.IconDownload,
                ActionRef:   "action-export-pos",
                MinSelected: 1,
            },
        },
        ShowCountLabel: true,
        ClearOnAction:  true,
    })
```

The bulk action bar appears above the table when one or more rows are selected. It disappears when selection is cleared.

### 12.4.14 Export (CSV, Excel, PDF)

```go
ast.NewDataTable("po-list").
    WithExport(ast.ExportConfig{
        Formats:        []ast.ExportFormat{ast.ExportCSV, ast.ExportXLSX},
        Filename:       ast.Expr(`concat("purchase_orders_", format_date(today(), "YYYY-MM-DD"))`),
        ExportEndpoint: "/api/procurement/purchase-orders/export",
        ExportAllRows:  true,   // export all rows, not just the current page
        IncludeHeaders: true,
    })
```

---

## 12.5 Table Permissions

### 12.5.1 Column-Level Visibility

Columns can be conditionally included using the `$if` permission directive:

```go
ast.NewNumberColumn("cost_price").
    WithHeader(i18n.Key("column.cost_price")).
    WithIf(ast.Permission("finance:cost_price:read"))
```

Columns excluded by permission are absent from the column definitions in the compiled payload. The table renders without them — column widths reflow automatically.

### 12.5.2 Row-Level Visibility (FGA-Driven)

Row-level visibility is enforced at the data source layer, not at the table component layer. The data source endpoint must filter rows based on the user's authorization. The table component renders whatever rows the data source returns. This is consistent with the principle that authorization is enforced server-side.

The table component does not support a row-level `$if` directive — such a directive would require the full set of rows to be delivered to the client and then hidden, which violates the least-privilege principle.

### 12.5.3 Action Column Permissions

Row actions are conditionally shown using the `permission` and `show_if` fields on `RowActionDef`. The permission check is evaluated against the current user's permissions at compilation time (for static permission checks) or at render time (for row-data-dependent permission checks, where the OpenFGA tuple includes the specific row's ID).

---

## 12.6 ERP-Specific Table Patterns

### 12.6.1 Line Item Grid

The line item grid is a specialized form of the data table used within forms (see Chapter 11, Section 11.5.4). It combines inline editing with aggregation rows and is the primary data entry mechanism for document lines.

### 12.6.2 Ledger / Journal Table

Ledger tables display double-entry accounting records. They require:
- Debit and credit columns that are never both populated in the same row
- Running balance computation
- Color-coded debit/credit distinction
- Account code linking
- Totals rows ensuring debit total = credit total

```go
ast.NewDataTable("journal-ledger").
    WithDensity(ast.DensityCompact).
    WithColumn(ast.NewTextColumn("account_code").WithFrozen(ast.FrozenLeft)).
    WithColumn(ast.NewTextColumn("description")).
    WithColumn(ast.NewNumberColumn("debit").
        WithFormat(ast.FormatCurrency).
        WithCurrencyField("currency_code").
        WithColorNegative(false).
        WithAggregate(ast.AggSum),
    ).
    WithColumn(ast.NewNumberColumn("credit").
        WithFormat(ast.FormatCurrency).
        WithCurrencyField("currency_code").
        WithAggregate(ast.AggSum),
    ).
    WithColumn(ast.NewNumberColumn("running_balance").
        WithFormat(ast.FormatCurrency).
        WithReadOnly(true),
    ).
    WithAggregationRow(ast.AggregationRowConfig{
        Label:    i18n.Key("ledger.totals"),
        Position: ast.AggregationBottom,
        ValidationRule: ast.AggValidationEqual("debit", "credit",
            i18n.Key("ledger.validation.unbalanced")),
    })
```

### 12.6.3 Audit Log Table

Audit log tables display immutable records of system events. They are always read-only, sorted newest-first, and include actor, timestamp, event type, and a diff of changed values.

```go
ast.NewDataTable("audit-log").
    WithDataSource(ast.DataSourceRef("audit-log-source")).
    WithRowKey("event_id").
    WithSorting(ast.SortingConfig{DefaultField: "occurred_at", DefaultOrder: ast.Descending}).
    WithColumn(ast.NewDateColumn("occurred_at").WithFormat(ast.FormatDatetime)).
    WithColumn(ast.NewTextColumn("actor_name")).
    WithColumn(ast.NewStatusColumn("event_type").WithStatusMap(auditEventStatusMap)).
    WithColumn(ast.NewCustomColumn("changes").WithCellTemplate(auditDiffTemplate)).
    WithRowExpansion(ast.RowExpansionConfig{
        Mode:              ast.MultiExpand,
        ExpansionTemplate: fullAuditDetailTemplate,
    })
```

### 12.6.4 Hierarchical / Tree Table

Tree tables display data with parent-child relationships — org charts in table form, bill of materials, GL account hierarchies.

```go
ast.NewDataTable("gl-accounts").
    WithTreeMode(ast.TreeTableConfig{
        ParentField:     "parent_account_id",
        ChildrenField:   "children",       // if pre-nested; omit for flat + parent_field mode
        DefaultExpanded: 2,                // expand 2 levels by default
        IndentPxPerLevel: 20,
        ExpandColID:     "account_code",   // which column gets the expand/collapse control
    })
```

---

## 12.7 Table Performance Considerations

### Virtual Rendering (Row Virtualization)

For tables displaying more than 100 rows, rendering engines must implement row virtualization: only the rows visible in the viewport (plus a buffer above and below) are rendered in the DOM/view hierarchy. Off-screen rows are represented by placeholder elements of the correct height.

Virtualization is triggered automatically by the rendering engine when row count exceeds the virtualization threshold (default: 100 rows). Backend engineers do not need to configure it explicitly.

### Column Width Calculation

Column width calculation is a significant source of rendering performance issues in wide tables. Rendering engines must avoid measuring DOM elements to determine column widths — all width calculations must be based on explicitly specified widths or computed from the container width and `fr` fractions. A table that requires layout reflow to determine column widths is a performance regression.

### Data Source Pagination

Tables displaying large datasets must use server-side pagination (offset or cursor). Client-side pagination (load all rows, paginate in-memory) is only acceptable for datasets confirmed to be under 500 rows. The compilation pipeline enforces a data source `max_rows` limit of 2,000 for any data source used by a table — requests exceeding this limit are rejected with a validation error.

---

## 12.8 Mobile Table Adaptations

Full-featured data grids with many columns are not practical on mobile screens. The rendering engine applies the following adaptations automatically, unless the backend overrides them.

### Column Priority and Hiding

Each column definition may specify a `mobile_priority` value (`essential`, `important`, `supplemental`). On mobile (`xs` breakpoint), only `essential` columns are shown by default. `important` columns are shown if screen width allows. `supplemental` columns are hidden.

```go
ast.NewTextColumn("po_number").WithMobilePriority(ast.MobileEssential)
ast.NewStatusColumn("status").WithMobilePriority(ast.MobileEssential)
ast.NewTextColumn("vendor_name").WithMobilePriority(ast.MobileImportant)
ast.NewDateColumn("created_at").WithMobilePriority(ast.MobileSupplemental)
ast.NewNumberColumn("total_amount").WithMobilePriority(ast.MobileImportant)
```

### Card Layout Mode

On mobile, tables can render in card layout mode instead of the standard grid layout. Each row is rendered as a card with the essential columns displayed as labeled fields.

```go
ast.NewDataTable("po-list").
    WithMobileLayout(ast.MobileCardLayout{
        PrimaryField:   "po_number",
        SecondaryField: "vendor_name",
        StatusField:    "status",
        MetaFields:     []string{"created_at", "total_amount"},
    })
```

### Tap-to-Expand

On mobile, rows that have expansion content configured show a chevron on the right. A tap on the row body (not just the chevron) expands the row to show additional columns that were hidden due to screen size.

---

*End of Chapter 12*

**Previous:** [Chapter 11 — Forms Framework](./11-forms-framework.md)
**Next:** [Chapter 13 — Dashboard Framework](./13-dashboard-framework.md)
