# Chapter 13 — Dashboard Framework

> **Volume:** III — Component System
> **Phase:** 2
> **Audience:** Backend Engineers, Web/Mobile Engineers, Product Owners
> **Prerequisites:** Chapters 01–12

---

## Table of Contents

- [13.1 Dashboard Design Goals](#131-dashboard-design-goals)
- [13.2 Dashboard Node Schema](#132-dashboard-node-schema)
- [13.3 Dashboard Layouts](#133-dashboard-layouts)
- [13.4 Widget Types](#134-widget-types)
- [13.5 Dashboard Data Sources](#135-dashboard-data-sources)
- [13.6 Dashboard Personalization](#136-dashboard-personalization)
- [13.7 Dashboard Permissions](#137-dashboard-permissions)
- [13.8 Dashboard Refresh and Polling](#138-dashboard-refresh-and-polling)
- [13.9 Dashboard Export](#139-dashboard-export)

---

## 13.1 Dashboard Design Goals

Dashboards in AwoERP serve a different purpose than forms and tables. Forms collect input; tables browse records; dashboards provide **situational awareness** — the answer to "what do I need to know right now?" for a given role, department, or business function.

Three design goals drive the dashboard framework:

**Composability.** A dashboard is a composition of widgets. The platform provides a rich widget library; the backend composes them into role-appropriate layouts. A procurement manager's dashboard, a CFO's financial dashboard, and a warehouse supervisor's inventory dashboard are composed from the same widget library — they differ in which widgets appear, in what arrangement, with what data bindings.

**Personalizability without loss of control.** Tenants and users can customize their dashboards — reordering widgets, resizing them, hiding irrelevant ones, adding from a permitted set. But the backend retains control of which widgets are available and what data they can access. Personalization operates within a backend-defined permission envelope.

**Performance by design.** Dashboards aggregate data from many sources. Naive implementations lead to N+1 data fetching and slow initial renders. The dashboard framework is designed around parallel data fetching, aggressive widget-level caching, and skeleton loading states that give the user immediate feedback while data loads.

---

## 13.2 Dashboard Node Schema

### 13.2.1 Dashboard Container

**`dashboard.container`** is the root node of every dashboard surface.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `title` | LocalizedString | ✅ | — | Dashboard title |
| `personalization_key` | string | | — | If set, enables user personalization; key namespaces the saved layout |
| `date_range_picker` | boolean | | `false` | Whether to show a global date range filter above the widgets |
| `date_range_state_var` | string | | `"dashboard_date_range"` | State variable name for the selected date range |
| `refresh_config` | RefreshConfig | | — | Automatic refresh settings |
| `toolbar` | array of Node | | `[]` | Additional controls in the dashboard toolbar |

### 13.2.2 Widget Container

**`dashboard.widget`** wraps each individual widget, providing the resizable, movable container behavior.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `widget_id` | string | ✅ | — | Stable identifier for personalization persistence |
| `title` | LocalizedString | | — | Widget card title |
| `subtitle` | LocalizedString | | — | |
| `col_span` | integer\|ResponsiveMap | ✅ | — | Grid column span (responsive) |
| `row_span` | integer | | 1 | Grid row span |
| `min_col_span` | integer | | 1 | Minimum allowed span during user resize |
| `max_col_span` | integer | | 12 | Maximum allowed span during user resize |
| `resizable` | boolean | | `true` | Whether the user can resize this widget |
| `removable` | boolean | | `false` | Whether the user can remove this widget |
| `collapsible` | boolean | | `false` | Whether the widget card can be collapsed |
| `header_actions` | array of Node | | `[]` | Toolbar buttons in the widget's card header |
| `loading_strategy` | `eager`\|`lazy` | | `eager` | `lazy` widgets load only when scrolled into view |

### 13.2.3 Widget Manifest

For dashboards with a widget picker (user-addable widgets), the backend provides a widget manifest listing available widgets:

```go
ast.NewDashboardContainer("procurement-dashboard").
    WithWidgetManifest([]ast.WidgetManifestEntry{
        {
            WidgetID:    "spend-by-category",
            Title:       i18n.Key("widget.spend_by_category"),
            Description: i18n.Key("widget.spend_by_category.desc"),
            Permission:  "procurement:analytics:read",
            DefaultColSpan: 6,
            Component:   spendByCategoryWidget,
        },
        {
            WidgetID:    "vendor-scorecard",
            Title:       i18n.Key("widget.vendor_scorecard"),
            Description: i18n.Key("widget.vendor_scorecard.desc"),
            Permission:  "procurement:vendors:read",
            DefaultColSpan: 4,
            Component:   vendorScorecardWidget,
        },
    })
```

---

## 13.3 Dashboard Layouts

### 13.3.1 Fixed Grid Dashboard

A fixed-grid dashboard has a predetermined widget arrangement that does not change. Widgets are placed using `col_span` and `row_span` within a 12-column grid.

```go
ast.NewDashboardContainer("finance-overview").
    WithLayout(ast.NewGridLayout(12).WithGap(ast.GapMD)).
    AddWidget(ast.NewDashboardWidget("cashflow-widget").
        WithColSpan(ast.Responsive{XS: 12, MD: 6, LG: 8}).
        WithRowSpan(2).
        WithComponent(cashflowChartWidget),
    ).
    AddWidget(ast.NewDashboardWidget("ar-aging-widget").
        WithColSpan(ast.Responsive{XS: 12, MD: 6, LG: 4}).
        WithComponent(arAgingWidget),
    )
```

### 13.3.2 Drag-and-Drop Grid (User-Configurable)

When `personalization_key` is set on the dashboard container and widgets have `resizable: true`, the dashboard becomes user-configurable. The rendering engine enables drag-and-drop widget reordering and resize handles. The personalized layout is saved to the user preferences store under the `personalization_key` namespace.

### 13.3.3 Responsive Dashboard

All dashboard grids use the 12-column responsive grid system. Widget `col_span` values use the responsive map form:

```json
{ "col_span": { "xs": 12, "sm": 12, "md": 6, "lg": 4, "xl": 3 } }
```

On mobile, widgets stack vertically (all span 12 columns). On desktop, they tile in the grid.

---

## 13.4 Widget Types

### 13.4.1 KPI / Metric Widget

**`widget.kpi_card`** — Displays a single key performance indicator.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `value` | number\|string\|Binding | ✅ | — | The primary metric value |
| `label` | LocalizedString | ✅ | — | Metric name |
| `format` | `number`\|`currency`\|`percentage`\|`plain` | | `number` | |
| `currency_code` | string\|Binding | | — | Required for `format: "currency"` |
| `comparison_value` | number\|Binding | | — | Previous period value for trend comparison |
| `comparison_label` | LocalizedString | | — | E.g., "vs. last month" |
| `trend_direction` | `up_good`\|`down_good`\|`neutral` | | `neutral` | How to color the trend indicator |
| `icon` | IconRef | | — | Optional supporting icon |
| `click_action` | ActionRef | | — | Makes the card clickable |
| `target_value` | number\|Binding | | — | Shows a progress bar toward a target |

### 13.4.2 Trend Card

**`widget.trend_card`** — A KPI card with a sparkline chart showing historical trend.

Extends `widget.kpi_card` with:

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `sparkline_data_source` | DataSourceRef | ✅ | — | Time series data for the sparkline |
| `sparkline_field` | string | ✅ | — | The value field in the time series |
| `sparkline_color` | ColorToken | | `token:brand.primary` | |
| `sparkline_type` | `line`\|`bar`\|`area` | | `line` | |

### 13.4.3 Chart Widget

Chart widgets wrap the chart components from Chapter 14 in a dashboard widget container. Any chart component can be used as a dashboard widget by placing it inside a `dashboard.widget` node.

```go
ast.NewDashboardWidget("spend-trend").
    WithTitle(i18n.Key("widget.spend_trend")).
    WithColSpan(ast.Responsive{XS: 12, LG: 8}).
    WithComponent(
        ast.NewLineChart("spend-trend-chart").
            WithDataSource(ast.DataSourceRef("monthly-spend-source")).
            WithXAxis(ast.DateAxis("month")).
            WithYAxis(ast.CurrencyAxis("amount", "USD")).
            WithSeries("total_spend", i18n.Key("series.total_spend")),
    )
```

### 13.4.4 Activity Feed Widget

**`widget.activity_feed`** — A chronological list of recent system events.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `data_source` | DataSourceRef | ✅ | — | |
| `limit` | integer | | `10` | Number of items to display |
| `item_template` | Node | | *(default template)* | Custom item rendering |
| `group_by_date` | boolean | | `true` | Show date separators |
| `show_actor_avatar` | boolean | | `true` | |
| `empty_label` | LocalizedString | | *(platform default)* | |
| `view_all_action` | ActionRef | | — | "View all" link at the bottom |

### 13.4.5 Approval Pending Widget

**`widget.approval_queue`** — Shows documents awaiting the current user's approval.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `data_source` | DataSourceRef | ✅ | — | |
| `limit` | integer | | `5` | |
| `document_type_label` | LocalizedString | | — | E.g., "Purchase Orders" |
| `approve_action` | ActionRef | | — | Quick-approve action per item |
| `view_action` | ActionRef | | — | Navigate to detail for review |
| `show_sla_indicator` | boolean | | `true` | Shows urgency based on approval SLA |

### 13.4.6 Quick Action Widget

**`widget.quick_actions`** — A grid of shortcut buttons for common operations.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `actions` | array of QuickActionDef | ✅ | — | |
| `layout` | `grid`\|`list` | | `grid` | |
| `columns` | integer | | 3 | Columns in grid mode |

**QuickActionDef:** `{ label: LocalizedString, icon: IconRef, action_ref: ActionRef, permission?: PermissionRef, badge?: string|Binding }`

### 13.4.7 Calendar / Schedule Widget

**`widget.calendar`** — Displays events on a calendar view.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `data_source` | DataSourceRef | ✅ | — | |
| `view` | `month`\|`week`\|`day`\|`agenda` | | `month` | |
| `event_label_field` | string | ✅ | — | |
| `event_start_field` | string | ✅ | — | |
| `event_end_field` | string | | — | |
| `event_color_field` | string | | — | |
| `event_click_action` | ActionRef | | — | |

---

## 13.5 Dashboard Data Sources

Dashboard data sources are declared at the `dashboard.container` level and shared across widgets. Each widget binds to one or more shared data sources, rather than each widget declaring its own — this prevents duplicate API calls for data that multiple widgets consume.

```go
ast.NewDashboardContainer("procurement-dashboard").
    AddDataSource(ast.NewRESTDataSource("procurement-kpis").
        WithEndpoint("/api/procurement/dashboard/kpis").
        WithParam("date_range", ast.StateBinding("dashboard_date_range")).
        WithCacheTTL(60 * time.Second),
    ).
    AddDataSource(ast.NewRESTDataSource("pending-approvals").
        WithEndpoint("/api/procurement/approvals/pending").
        WithCacheTTL(30 * time.Second),
    )
```

All widget data sources load in parallel when the dashboard renders. The rendering engine tracks loading state per data source and shows individual widget loading skeletons until their data arrives.

---

## 13.6 Dashboard Personalization

When `personalization_key` is set, the user can:
- Reorder widgets (drag-and-drop)
- Resize widgets (within `min_col_span` / `max_col_span` bounds)
- Remove widgets marked `removable: true`
- Add widgets from the widget manifest

Personalization state is stored in the user preferences store under the key `dashboard.{personalization_key}.layout`. The backend loads this state and applies it during compilation — the compiled dashboard definition reflects the user's saved layout.

When a user has no saved layout (first visit) or their saved layout references a widget that no longer exists, the default layout from the AST is used.

### Role-Based Default Layouts

```go
// Choose the default layout based on the user's primary role
var defaultLayout *ast.DashboardLayoutConfig
switch compilationContext.PrimaryRole() {
case "procurement_officer":
    defaultLayout = procurementOfficerLayout
case "budget_approver":
    defaultLayout = budgetApproverLayout
default:
    defaultLayout = standardProcurementLayout
}
```

---

## 13.7 Dashboard Permissions

Dashboard widget visibility is controlled through `$if` permission directives on individual `dashboard.widget` nodes. A widget excluded by permission is absent from the compiled payload — the dashboard layout reflows to fill the space.

The widget manifest also respects permissions: manifest entries with `permission` set are excluded for users who do not have that permission.

---

## 13.8 Dashboard Refresh and Polling

```go
ast.NewDashboardContainer("finance-realtime").
    WithRefreshConfig(ast.RefreshConfig{
        AutoRefresh:    true,
        IntervalSeconds: 60,
        RefreshLabel:   i18n.Key("dashboard.refreshing"),
        ShowLastUpdated: true,
        ManualRefreshEnabled: true,
    })
```

Refresh triggers a reload of all data sources. Individual data sources can override the global refresh interval with their own `CacheTTL` — they will refresh at their individual TTL even if the global auto-refresh is longer.

---

## 13.9 Dashboard Export

```go
ast.NewDashboardContainer("finance-dashboard").
    WithExport(ast.DashboardExportConfig{
        Formats:   []ast.ExportFormat{ast.ExportPDF, ast.ExportPNG},
        Filename:  ast.Expr(`concat("finance_dashboard_", format_date(today(), "YYYY-MM-DD"))`),
        PageSize:  ast.PageA4Landscape,
    })
```

Dashboard export captures a server-side rendering of the dashboard (not a client-side screenshot) to ensure consistent output regardless of the user's screen resolution.

---

*End of Chapter 13*

---

# Chapter 14 — Charts and Analytics

> **Volume:** III — Component System
> **Phase:** 2
> **Audience:** Backend Engineers, Web/Mobile Engineers
> **Prerequisites:** Chapters 01–13

---

## Table of Contents

- [14.1 Chart System Overview](#141-chart-system-overview)
- [14.2 Chart Node Schema](#142-chart-node-schema)
- [14.3 Chart Types](#143-chart-types)
- [14.4 Chart Data Binding](#144-chart-data-binding)
- [14.5 Chart Interactivity](#145-chart-interactivity)
- [14.6 Chart Theming and Branding](#146-chart-theming-and-branding)
- [14.7 Chart Export](#147-chart-export)
- [14.8 Chart Performance for Large Datasets](#148-chart-performance-for-large-datasets)
- [14.9 Chart Accessibility](#149-chart-accessibility)

---

## 14.1 Chart System Overview

Charts in AwoERP are first-class components in the DSL — not iframed third-party embeds or raw HTML canvas elements managed by the frontend. A chart definition in the AST is as declarative and schema-validated as a form field definition.

Rendering engines implement chart rendering using their platform's best available library — the web renderer may use Recharts, Vega-Lite, or ECharts; the iOS renderer uses Swift Charts; the Android renderer uses MPAndroidChart or Compose Charts. The DSL abstracts over these implementations: a `chart.bar` node renders as the most appropriate bar chart on each platform.

Chart components require the `charts` client capability. Clients that do not declare this capability receive the chart node's fallback (typically a data table).

---

## 14.2 Chart Node Schema

All chart types share a common base schema, extended by type-specific props.

**Base chart props (inherited by all `chart.*` nodes):**

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `title` | LocalizedString | | — | Chart title |
| `subtitle` | LocalizedString | | — | |
| `data_source` | DataSourceRef | ✅ | — | |
| `height` | integer (px) | | `300` | |
| `legend` | LegendConfig | | *(show at bottom)* | |
| `tooltip` | TooltipConfig | | *(auto)* | |
| `animation` | boolean | | `true` | Whether to animate on load |
| `empty_state` | Node | | *(default)* | |
| `color_palette` | array of ColorToken | | *(brand palette)* | Ordered colors for series |

**LegendConfig:** `{ show: boolean, position: "top"|"bottom"|"left"|"right", align: "start"|"center"|"end" }`

**TooltipConfig:** `{ show: boolean, format: "auto"|"currency"|"percentage"|"number", shared: boolean }`

---

## 14.3 Chart Types

### 14.3.1 Bar Chart

**`chart.bar`** — Vertical, horizontal, stacked, and grouped variants.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `x_axis` | AxisConfig | ✅ | — | |
| `y_axis` | AxisConfig | ✅ | — | |
| `series` | array of SeriesDef | ✅ | — | |
| `orientation` | `vertical`\|`horizontal` | | `vertical` | |
| `mode` | `grouped`\|`stacked`\|`stacked_percent` | | `grouped` | |
| `bar_radius` | integer (px) | | `2` | Corner radius |
| `show_data_labels` | boolean | | `false` | |
| `reference_lines` | array of ReferenceLine | | — | Horizontal/vertical threshold lines |

### 14.3.2 Line Chart

**`chart.line`** — Single and multi-series line and area charts.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `x_axis` | AxisConfig | ✅ | — | |
| `y_axis` | AxisConfig | ✅ | — | |
| `series` | array of SeriesDef | ✅ | — | |
| `smooth` | boolean | | `false` | Curved vs. straight lines |
| `area` | boolean | | `false` | Fill area under the line |
| `area_opacity` | number (0–1) | | `0.2` | |
| `show_points` | boolean | | `true` | |
| `point_size` | integer (px) | | `4` | |
| `connect_nulls` | boolean | | `false` | Whether to connect across null data points |

### 14.3.3 Pie / Donut Chart

**`chart.pie`** — Pie and donut variants.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `value_field` | string | ✅ | — | |
| `label_field` | string | ✅ | — | |
| `inner_radius` | number (0–1) | | `0` | `0` = pie; `> 0` = donut (e.g., `0.6`) |
| `show_percentages` | boolean | | `true` | |
| `show_values` | boolean | | `false` | |
| `min_slice_percent` | number | | `1` | Slices below this threshold are grouped into "Other" |
| `center_label` | LocalizedString | | — | Center label for donut charts |
| `center_value` | any\|Binding | | — | Center value for donut charts |

### 14.3.4 Gauge / Speedometer

**`chart.gauge`** — A circular or arc gauge for displaying a value relative to a range.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `value` | number\|Binding | ✅ | — | |
| `min` | number | ✅ | — | |
| `max` | number | ✅ | — | |
| `thresholds` | array of GaugeThreshold | | — | Color zones on the gauge arc |
| `label` | LocalizedString | | — | Label below the value |
| `value_format` | `number`\|`percentage`\|`currency` | | `number` | |
| `arc_width` | number (0–1) | | `0.2` | |

**GaugeThreshold:** `{ value: number, color: ColorToken, label?: LocalizedString }`

### 14.3.5 Gantt / Timeline Chart

**`chart.gantt`** — A Gantt chart for project and process timelines.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `task_label_field` | string | ✅ | — | |
| `start_date_field` | string | ✅ | — | |
| `end_date_field` | string | ✅ | — | |
| `progress_field` | string | | — | Optional completion percentage (0–100) |
| `group_field` | string | | — | Groups tasks by this field |
| `color_field` | string | | — | Color token or status-mapped color |
| `today_marker` | boolean | | `true` | Vertical line at today's date |
| `time_scale` | `days`\|`weeks`\|`months` | | *(auto)* | |

### 14.3.6 Waterfall Chart

**`chart.waterfall`** — Shows cumulative effect of sequential values (budget variances, cash flow bridges).

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `label_field` | string | ✅ | — | |
| `value_field` | string | ✅ | — | Positive = increase, negative = decrease |
| `total_indices` | array of integer | | `[last]` | Indices that represent totals (rendered differently) |
| `show_connectors` | boolean | | `true` | Lines connecting bars |
| `positive_color` | ColorToken | | `token:status.success` | |
| `negative_color` | ColorToken | | `token:status.error` | |
| `total_color` | ColorToken | | `token:brand.primary` | |

---

## 14.4 Chart Data Binding

Charts bind to data sources that return arrays of records. The chart configuration maps record fields to visual encodings (axes, series, colors).

```go
ast.NewBarChart("spend-by-category").
    WithDataSource(ast.DataSourceRef("spend-by-category-source")).
    WithXAxis(ast.CategoryAxis("category_name").
        WithLabel(i18n.Key("axis.category")),
    ).
    WithYAxis(ast.ValueAxis("total_spend").
        WithFormat(ast.FormatCurrency).
        WithCurrencyCode("USD").
        WithLabel(i18n.Key("axis.spend")),
    ).
    WithSeries("total_spend", i18n.Key("series.total_spend")).
    WithSeries("budget", i18n.Key("series.budget")).
    WithMode(ast.ChartGrouped)
```

### Axis Configuration

**AxisConfig:**

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `field` | string | ✅ | — | Data field |
| `label` | LocalizedString | | — | Axis label |
| `type` | `category`\|`datetime`\|`numeric` | | *(inferred)* | |
| `format` | FormatConfig | | *(auto)* | |
| `min` | number | | *(auto)* | |
| `max` | number | | *(auto)* | |
| `tick_count` | integer | | *(auto)* | |
| `grid_lines` | boolean | | `true` | |
| `rotate_labels` | integer (degrees) | | `0` | |

### Series Definition

**SeriesDef:**

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `field` | string | ✅ | — | Data field for this series |
| `label` | LocalizedString | ✅ | — | Legend label |
| `color` | ColorToken | | *(from palette)* | |
| `type` | `bar`\|`line`\|`area` | | *(chart type)* | For combo charts |
| `y_axis` | `primary`\|`secondary` | | `primary` | |

---

## 14.5 Chart Interactivity

### 14.5.1 Click Events → Actions

Chart elements (bars, slices, points) can trigger actions when clicked. The click event payload includes the data point's full record, enabling drill-down navigation.

```go
ast.NewBarChart("spend-by-vendor").
    WithOnElementClick(
        ast.ActionRef("action-navigate-to-vendor-detail"),
        ast.ActionParams{"vendor_id": ast.EventBinding("clicked_element.vendor_id")},
    )
```

### 14.5.2 Drill-Down Navigation

A common pattern in ERP analytics: clicking a chart element navigates to a more detailed view of that data segment.

```go
// Clicking a bar navigates to filtered PO list for that vendor
ast.NewNavigateAction("action-drill-down-vendor").
    WithTarget("procurement.purchase-orders.list").
    WithParams(map[string]ast.Binding{
        "filter_vendor_id": ast.EventBinding("clicked_element.vendor_id"),
    })
```

### 14.5.3 Cross-Filter Between Charts

When multiple charts on a dashboard share a state variable, selecting an element on one chart can filter others. This is implemented through `action.set_variable` on chart click events, with other charts binding their data source params to the same state variable.

---

## 14.6 Chart Theming and Branding

Charts use the platform's design token system for all colors. The `color_palette` prop accepts an ordered array of color tokens:

```go
ast.NewBarChart("revenue-chart").
    WithColorPalette([]ast.ColorToken{
        ast.Token("token:brand.primary"),
        ast.Token("token:brand.secondary"),
        ast.Token("token:brand.accent"),
        ast.Token("token:status.info"),
    })
```

When the tenant overrides brand tokens, chart colors update automatically.

---

## 14.7 Chart Export

```go
ast.NewBarChart("spend-chart").
    WithExport(ast.ChartExportConfig{
        Formats:  []ast.ExportFormat{ast.ExportPNG, ast.ExportSVG, ast.ExportCSV},
        Filename: ast.Expr(`concat("spend_chart_", format_date(today(), "YYYY-MM-DD"))`),
    })
```

Export controls appear in the chart's toolbar when configured.

---

## 14.8 Chart Performance for Large Datasets

For charts rendering more than 1,000 data points:
- **Data aggregation must be server-side.** The data source endpoint aggregates before responding; the chart receives pre-aggregated data.
- **Downsampling.** For time series with high resolution (minute-by-minute for a year), the rendering engine may downsample to the visible resolution. The `max_data_points` prop controls this.
- **Progressive loading.** Large charts load in stages: the initial render shows the first segment; additional data loads as the user scrolls or zooms.

```go
ast.NewLineChart("sensor-readings").
    WithMaxDataPoints(500).
    WithDownsamplingMethod(ast.LargestTriangleThreeBuckets)
```

---

## 14.9 Chart Accessibility

Charts must provide non-visual alternatives for screen reader users:
- The `caption` prop provides a textual description of the chart's purpose and key findings
- Data tables are accessible via a "View data" toggle that appears as a chart toolbar action
- Chart elements receive `aria-label` attributes from the rendering engine based on the data point values
- Color choices must maintain WCAG AA contrast ratios (3:1 for adjacent series colors)

---

*End of Chapter 14*

---

# Chapter 15 — Workflow and Approval Components

> **Volume:** III — Component System
> **Phase:** 2
> **Audience:** Backend Engineers, Mobile/Web Engineers, ERP Implementers
> **Prerequisites:** Chapters 01–14

---

## Table of Contents

- [15.1 Workflow UI Philosophy](#151-workflow-ui-philosophy)
- [15.2 Workflow Component Taxonomy](#152-workflow-component-taxonomy)
- [15.3 Approval Actions](#153-approval-actions)
- [15.4 Workflow State Machine Representation](#154-workflow-state-machine-representation)
- [15.5 Temporal Signal Binding](#155-temporal-signal-binding)
- [15.6 Pending Approval Lists and Queues](#156-pending-approval-lists-and-queues)
- [15.7 SLA Timers and Deadline Indicators](#157-sla-timers-and-deadline-indicators)
- [15.8 Audit Trail Components](#158-audit-trail-components)

---

## 15.1 Workflow UI Philosophy

Workflow and approval UI sits at the intersection of the UI platform and the Temporal workflow orchestration engine. The business logic of an approval workflow — who must approve, in what sequence, with what escalation rules — lives entirely in Temporal workflows authored by backend engineers. The UI platform's role is to **surface the current workflow state** to users and to **capture user decisions** (approve, reject, delegate) and relay them to Temporal as signals.

This creates a clean boundary:
- **Temporal owns workflow state.** The current step, the assigned approver, the deadline, the historical decisions — all of this lives in Temporal's workflow state.
- **The UI platform surfaces workflow state.** The compilation pipeline subscribes to Temporal workflow state for the relevant document and incorporates it into the compiled UI definition.
- **User actions send Temporal signals.** An "Approve" button executes `action.workflow_signal`, which calls the Temporal signal endpoint. The Temporal workflow advances its state machine in response.

The UI is a view onto the workflow engine — not a workflow engine itself.

---

## 15.2 Workflow Component Taxonomy

### 15.2.1 Workflow Status Indicator

**`workflow.status_indicator`** — Displays the current workflow state as a status badge with contextual information.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `workflow_state` | string\|Binding | ✅ | — | Current state name from the workflow definition |
| `state_display_map` | map of state → StatusConfig | ✅ | — | Maps state names to display configuration |
| `show_assignee` | boolean | | `true` | Show who the current step is assigned to |
| `assignee_field` | string | | — | Data field containing assignee info |
| `show_deadline` | boolean | | `true` | Show the step deadline if one exists |
| `deadline_field` | string | | — | |
| `overdue_indicator` | boolean | | `true` | Highlight past-deadline items |

### 15.2.2 Workflow Progress Stepper

**`workflow.progress_stepper`** — Displays all workflow steps with their completion status, making the full approval path visible.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `steps` | array of WorkflowStepDef\|Binding | ✅ | — | Can be static (known steps) or bound to a data source (dynamic routing) |
| `current_step_index` | integer\|Binding | ✅ | — | |
| `orientation` | `horizontal`\|`vertical` | | `horizontal` | |
| `show_actor` | boolean | | `true` | Show who completed each step |
| `show_timestamp` | boolean | | `true` | Show completion timestamp for completed steps |
| `show_comments` | boolean | | `false` | Show approval/rejection comments inline |

**WorkflowStepDef:** `{ id: string, label: LocalizedString, status: "pending"|"active"|"complete"|"rejected"|"skipped", actor?: string, completed_at?: datetime, comment?: string }`

### 15.2.3 Approval Action Bar

**`workflow.approval_action_bar`** — The primary component for capturing approval decisions. Renders the available actions for the current workflow step.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `workflow_id` | string\|Binding | ✅ | — | Temporal workflow ID |
| `available_actions` | array of ApprovalActionDef\|Binding | ✅ | — | Actions the current user may take |
| `require_comment_for` | array of string | | `["reject", "request_revision"]` | Action types that require a comment |
| `comment_label` | LocalizedString | | `"Comment"` | |
| `comment_required_label` | LocalizedString | | `"Comment is required for this action"` | |
| `sticky` | boolean | | `true` | Whether the action bar sticks to the bottom of the screen |

**ApprovalActionDef:**
```go
type ApprovalActionDef struct {
    ID              string
    Label           LocalizedString
    Icon            *IconRef
    Variant         ActionVariant       // primary, secondary, danger
    SignalName      string              // Temporal signal name
    ConfirmRequired bool
    ConfirmMessage  *LocalizedString
}
```

### 15.2.4 Approval History Timeline

**`workflow.approval_timeline`** — Shows the complete decision history of a workflow instance.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `data_source` | DataSourceRef | ✅ | — | Returns ordered list of approval events |
| `show_comments` | boolean | | `true` | |
| `show_duration` | boolean | | `true` | Time taken per step |
| `show_attachments` | boolean | | `false` | Whether attachment links are shown |
| `limit` | integer | | — | Truncate to most recent N events |

### 15.2.5 Comment Thread

**`workflow.comment_thread`** — A threaded comment and note section on an approval document.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `data_source` | DataSourceRef | ✅ | — | |
| `submit_endpoint` | string | ✅ | — | POST endpoint for new comments |
| `allow_mentions` | boolean | | `true` | @-mention support |
| `allow_attachments` | boolean | | `false` | |
| `allow_reactions` | boolean | | `false` | |
| `comment_permission` | PermissionRef | | — | Permission required to post comments |
| `real_time` | boolean | | `true` | Subscribe to real-time comment updates |

---

## 15.3 Approval Actions

All approval actions are `action.workflow_signal` nodes:

```go
ast.NewWorkflowSignalAction("action-approve").
    WithWorkflowIDBinding(ast.DataBinding("datasource:po.workflow_id")).
    WithSignalName("approve").
    WithSignalPayload(map[string]ast.Binding{
        "approver_id":   ast.ContextBinding("user.id"),
        "comment":       ast.StateBinding("approval_comment"),
        "approved_at":   ast.ExprBinding("now()"),
    }).
    WithPermission("procurement:po:approve").
    WithConfirmLabel(i18n.Key("action.approve.confirm")).
    WithLoadingLabel(i18n.Key("action.approving")).
    WithOnSuccess(
        ast.ActionChain(
            ast.ActionRef("action-show-approve-success"),
            ast.ActionRef("action-navigate-back"),
        ),
    )
```

The `action.workflow_signal` type is the primary mechanism connecting UI interactions to Temporal. The signal name maps to a Temporal signal handler in the workflow definition.

---

## 15.4 Workflow State Machine Representation

The UI must accurately reflect the current workflow state. The compilation pipeline retrieves workflow state from Temporal before building the AST, making the state available as a data source.

```go
// In the procurement service's AST builder:
workflowState, err := s.temporalClient.QueryWorkflow(ctx,
    poWorkflowID, "", "current_state_query")

// The compilation context includes the workflow state
ast.NewWorkflowProgressStepper("approval-stepper").
    WithCurrentStepIndex(ast.DataBinding("datasource:workflow.current_step_index")).
    WithSteps(ast.DataBinding("datasource:workflow.steps"))
```

---

## 15.5 Temporal Signal Binding

The workflow_signal action automatically adds the current tenant ID, user ID, and request timestamp to the signal payload. Backend Temporal workflows can rely on these fields being present without the UI author explicitly binding them.

---

## 15.6 Pending Approval Lists and Queues

```go
ast.NewDataTable("approval-queue").
    WithDataSource(ast.NewRESTDataSource("approvals-source").
        WithEndpoint("/api/approvals/pending").
        WithParam("assignee_id", ast.ContextBinding("user.id")),
    ).
    WithColumn(ast.NewTextColumn("document_reference")).
    WithColumn(ast.NewStatusColumn("document_type")).
    WithColumn(ast.NewTextColumn("submitted_by_name")).
    WithColumn(ast.NewDateColumn("submitted_at")).
    WithColumn(ast.NewDateColumn("deadline").
        WithColorBinding(
            ast.Conditional(
                ast.RowBinding("is_overdue"),
            ).Then(ast.TokenStatusError).Else(ast.TokenTextPrimary),
        ),
    ).
    WithColumn(ast.NewActionsColumn("queue-actions").
        AddAction(ast.RowAction("review").
            WithLabel(i18n.Key("action.review")).
            WithIcon(ast.IconEye).
            WithActionRef("action-navigate-to-document"),
        ),
    )
```

---

## 15.7 SLA Timers and Deadline Indicators

**`workflow.sla_indicator`** — A visual countdown showing time remaining for a workflow step.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `deadline` | datetime\|Binding | ✅ | — | |
| `warn_before_hours` | integer | | `24` | Hours before deadline to show warning state |
| `critical_before_hours` | integer | | `4` | Hours before deadline to show critical state |
| `display` | `countdown`\|`progress_bar`\|`badge` | | `badge` | |
| `overdue_label` | LocalizedString | | `"Overdue"` | |

---

## 15.8 Audit Trail Components

**`workflow.audit_trail`** — A specialized timeline view for immutable audit records.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `data_source` | DataSourceRef | ✅ | — | |
| `event_type_map` | map of type → DisplayConfig | | — | Maps event type codes to human-readable labels and icons |
| `show_diff` | boolean | | `true` | Show field-level diff for modification events |
| `diff_format` | `inline`\|`side_by_side` | | `inline` | |
| `show_metadata` | boolean | | `false` | Show IP address, user agent, session ID |

---

*End of Chapter 15*

---

# Chapter 16 — ERP-Specific Components

> **Volume:** III — Component System
> **Phase:** 2
> **Audience:** Backend Engineers, ERP Implementers
> **Prerequisites:** Chapters 01–15

---

## Table of Contents

- [16.1 ERP Component Design Principles](#161-erp-component-design-principles)
- [16.2 Financial Components](#162-financial-components)
- [16.3 Procurement Components](#163-procurement-components)
- [16.4 Inventory Components](#164-inventory-components)
- [16.5 HR Components](#165-hr-components)
- [16.6 Sales Components](#166-sales-components)
- [16.7 Document Components](#167-document-components)

---

## 16.1 ERP Component Design Principles

ERP domain components encapsulate **recurring, complex, business-domain-specific** UI patterns. A component earns a place in the ERP component library when:

1. It appears in multiple, unrelated surfaces across the platform (not just in one module)
2. Its implementation is non-trivial and benefits from centralization
3. Its behavior encodes business rules that should not be re-implemented per surface
4. It has a stable, well-understood semantic meaning in ERP domain terminology

Components that fail criterion 1 belong in the specific module's surface definitions as inline AST subtrees, not in the shared component library.

---

## 16.2 Financial Components

### 16.2.1 Currency Display

**`erp.currency_amount`** — Displays a formatted monetary amount with currency symbol.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `amount` | number\|Binding | ✅ | — | |
| `currency_code` | string\|Binding | ✅ | — | ISO 4217 currency code |
| `show_symbol` | boolean | | `true` | |
| `show_code` | boolean | | `false` | |
| `decimal_places` | integer | | *(currency standard)* | |
| `negative_format` | `minus`\|`parentheses`\|`red` | | `minus` | |
| `base_currency` | string | | — | If set, shows FX converted amount in a tooltip |
| `fx_rate` | number\|Binding | | — | Required if `base_currency` is set |
| `size` | `sm`\|`md`\|`lg`\|`xl` | | `md` | Typography size |
| `weight` | `normal`\|`medium`\|`bold` | | `normal` | |

### 16.2.2 Amount Breakdown Panel

**`erp.amount_breakdown`** — Displays a subtotal, tax, discount, and total breakdown — the standard financial summary panel for invoices, purchase orders, and sales orders.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `subtotal` | number\|Binding | ✅ | — | |
| `currency_code` | string\|Binding | ✅ | — | |
| `discount_amount` | number\|Binding | | — | |
| `discount_label` | LocalizedString | | `"Discount"` | |
| `tax_lines` | array of TaxLine\|Binding | | `[]` | |
| `total` | number\|Binding | ✅ | — | |
| `total_label` | LocalizedString | | `"Total"` | |
| `base_currency_total` | number\|Binding | | — | FX-converted total in base currency |
| `base_currency_code` | string | | — | |
| `layout` | `compact`\|`detailed` | | `detailed` | |

**TaxLine:** `{ label: LocalizedString, rate: number, amount: number|Binding }`

### 16.2.3 GL Account Selector

**`erp.gl_account_selector`** — A specialized lookup for General Ledger accounts with account code, name, and account type display.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `search_endpoint` | string | ✅ | — | |
| `account_type_filter` | array of string | | — | Filter by account type (asset, liability, equity, revenue, expense) |
| `company_id` | string\|Binding | | — | Scope to a specific company's chart of accounts |
| `show_code` | boolean | | `true` | Show account code alongside name |
| `show_type` | boolean | | `false` | Show account type badge |
| `value_field` | string | | `"code"` | Whether to store the account code or ID |

### 16.2.4 Budget vs. Actual Bar

**`erp.budget_vs_actual`** — A visual comparison of budgeted vs. actual amounts for a cost center, project, or period.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `budgeted` | number\|Binding | ✅ | — | |
| `actual` | number\|Binding | ✅ | — | |
| `currency_code` | string\|Binding | ✅ | — | |
| `label` | LocalizedString | | — | |
| `warn_at_percent` | number | | `80` | Show warning color when actual ≥ this % of budget |
| `critical_at_percent` | number | | `100` | Show critical color when over budget |
| `show_remaining` | boolean | | `true` | |
| `show_percentage` | boolean | | `true` | |

---

## 16.3 Procurement Components

### 16.3.1 Purchase Order Header

**`erp.po_header`** — The standard summary header for a purchase order view — vendor name, PO number, status, dates, and total amount in a structured layout.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `po_number` | string\|Binding | ✅ | — | |
| `status` | string\|Binding | ✅ | — | |
| `status_map` | map of status → StatusConfig | | *(defaults)* | |
| `vendor_name` | string\|Binding | ✅ | — | |
| `vendor_code` | string\|Binding | | — | |
| `order_date` | date\|Binding | ✅ | — | |
| `required_date` | date\|Binding | | — | |
| `total_amount` | number\|Binding | ✅ | — | |
| `currency_code` | string\|Binding | ✅ | — | |
| `approver_name` | string\|Binding | | — | |
| `created_by_name` | string\|Binding | | — | |

### 16.3.2 Vendor Card

**`erp.vendor_card`** — A compact summary of a vendor's key information. Used in search results, lookup dropdowns, and procurement document headers.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `vendor_name` | string\|Binding | ✅ | — | |
| `vendor_code` | string\|Binding | ✅ | — | |
| `vendor_type` | string\|Binding | | — | |
| `rating` | number\|Binding | | — | 0–5 star rating |
| `payment_terms` | string\|Binding | | — | |
| `credit_status` | string\|Binding | | — | |
| `contact_name` | string\|Binding | | — | |
| `logo_url` | string\|Binding | | — | |
| `click_action` | ActionRef | | — | |

### 16.3.3 RFQ / Quotation Comparison Table

**`erp.quotation_comparison`** — Displays competing vendor quotations side-by-side for evaluation.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `data_source` | DataSourceRef | ✅ | — | Returns array of vendor quotations |
| `line_items_field` | string | ✅ | — | Field containing the line items for each quotation |
| `highlight_lowest` | boolean | | `true` | Visually highlight the lowest price per line item |
| `show_total_comparison` | boolean | | `true` | Summary row comparing total per vendor |
| `select_action` | ActionRef | | — | Action for "Select this vendor" |

---

## 16.4 Inventory Components

### 16.4.1 Stock Level Indicator

**`erp.stock_level`** — A visual indicator of current stock against reorder point and maximum levels.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `current_qty` | number\|Binding | ✅ | — | |
| `reorder_point` | number\|Binding | | — | |
| `max_qty` | number\|Binding | | — | |
| `unit_of_measure` | string\|Binding | | — | |
| `display` | `bar`\|`badge`\|`number` | | `bar` | |
| `show_numbers` | boolean | | `true` | |

### 16.4.2 Batch / Serial Number Panel

**`erp.batch_serial_panel`** — Manages batch or serial number tracking for inventory transactions.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `tracking_mode` | `batch`\|`serial`\|`none` | ✅ | — | |
| `quantity` | number\|Binding | ✅ | — | Total quantity to be allocated |
| `data_source` | DataSourceRef | | — | Available batches/serials for lookup |
| `read_only` | boolean | | `false` | |
| `allow_create` | boolean | | `false` | Whether new batch/serial numbers can be created |

---

## 16.5 HR Components

### 16.5.1 Employee Card

**`erp.employee_card`** — A compact employee profile card.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `employee_name` | string\|Binding | ✅ | — | |
| `employee_id` | string\|Binding | | — | |
| `job_title` | string\|Binding | | — | |
| `department` | string\|Binding | | — | |
| `avatar_url` | string\|Binding | | — | |
| `email` | string\|Binding | | — | |
| `phone` | string\|Binding | | — | |
| `employment_status` | string\|Binding | | — | |
| `click_action` | ActionRef | | — | |

### 16.5.2 Leave Balance Panel

**`erp.leave_balance`** — Displays an employee's leave balance across leave types.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `data_source` | DataSourceRef | ✅ | — | Returns array of leave type balances |
| `as_of_date` | date\|Binding | | *(today)* | |
| `show_accrual_rate` | boolean | | `false` | |
| `show_pending` | boolean | | `true` | Show pending/in-progress leave |

### 16.5.3 Org Chart

**`erp.org_chart`** — A hierarchical organizational chart.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `data_source` | DataSourceRef | ✅ | — | Returns flat list of nodes with `id` and `parent_id` |
| `root_id` | string\|Binding | | *(top of tree)* | Root node to display from |
| `depth_limit` | integer | | 4 | Maximum visible depth |
| `node_template` | Node | | *(default employee card)* | Custom node rendering |
| `click_action` | ActionRef | | — | |
| `collapse_at_depth` | integer | | 3 | Depth beyond which nodes are collapsed by default |

---

## 16.6 Sales Components

### 16.6.1 Customer Card

**`erp.customer_card`** — Analogous to `erp.vendor_card` for sales-side customers.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `customer_name` | string\|Binding | ✅ | — | |
| `customer_code` | string\|Binding | ✅ | — | |
| `account_type` | string\|Binding | | — | |
| `credit_limit` | number\|Binding | | — | |
| `credit_used` | number\|Binding | | — | |
| `currency_code` | string\|Binding | | — | |
| `payment_terms` | string\|Binding | | — | |
| `click_action` | ActionRef | | — | |

### 16.6.2 Credit Limit Warning

**`erp.credit_limit_warning`** — An inline alert shown on sales order and invoice forms when the customer is approaching or exceeding their credit limit.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `credit_limit` | number\|Binding | ✅ | — | |
| `credit_used` | number\|Binding | ✅ | — | |
| `current_order_amount` | number\|Binding | ✅ | — | |
| `currency_code` | string\|Binding | ✅ | — | |
| `warn_at_percent` | number | | `80` | |
| `override_permission` | PermissionRef | | — | If set, shows an override option for authorized users |
| `override_action` | ActionRef | | — | |

### 16.6.3 Pipeline Stage Indicator

**`erp.pipeline_stage`** — Shows a CRM opportunity's current stage in the sales pipeline.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `stages` | array of PipelineStage\|Binding | ✅ | — | Ordered stage definitions |
| `current_stage_id` | string\|Binding | ✅ | — | |
| `show_probability` | boolean | | `false` | |
| `show_expected_close` | boolean | | `false` | |
| `orientation` | `horizontal`\|`compact` | | `horizontal` | |

---

## 16.7 Document Components

### 16.7.1 PDF Viewer

**`erp.pdf_viewer`** — Embeds a PDF document inline in the surface.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `document_url` | string\|Binding | ✅ | — | URL of the PDF to display |
| `height` | integer (px) | | `600` | |
| `show_toolbar` | boolean | | `true` | |
| `allow_download` | boolean | | `true` | |
| `initial_page` | integer | | `1` | |

### 16.7.2 Document Status Badge

**`erp.document_status`** — A standardized status badge for ERP document states, with pre-configured display for common document lifecycle states.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `status` | string\|Binding | ✅ | — | |
| `document_type` | `po`\|`invoice`\|`payment`\|`delivery`\|`contract`\|`custom` | | `custom` | Pre-fills `status_map` with standard states for the document type |
| `status_map` | map of status → StatusConfig | | *(document type defaults)* | |
| `size` | `sm`\|`md`\|`lg` | | `md` | |

### 16.7.3 E-Signature Capture

**`erp.esignature`** — A complete e-signature workflow embedded in a surface — signature pad (or typed name fallback), legal consent text, and submission.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `signer_name` | string\|Binding | ✅ | — | Pre-filled signer name |
| `document_reference` | string\|Binding | ✅ | — | The document being signed |
| `consent_text` | LocalizedString | ✅ | — | Legal consent statement displayed above the signature pad |
| `submit_endpoint` | string | ✅ | — | |
| `completion_action` | ActionRef | | — | Action after successful signature submission |
| `allow_typed_signature` | boolean | | `true` | |
| `require_drawn_signature` | boolean | | `false` | |

### 16.7.4 QR Code Display

**`erp.qr_code`** — Generates and displays a QR code for a URL or structured data.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `content` | string\|Binding | ✅ | — | The content to encode (URL or text) |
| `size` | integer (px) | | `150` | |
| `error_correction` | `L`\|`M`\|`Q`\|`H` | | `M` | |
| `label` | LocalizedString | | — | Text below the QR code |

### 16.7.5 Barcode Scanner Input

**`erp.barcode_scanner`** — A field that accepts barcode or QR code input via device camera (mobile) or USB scanner (web).

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `formats` | array of BarcodeFormat | | *(all common)* | Supported formats: `qr_code`, `code_128`, `ean_13`, etc. |
| `on_scan_action` | ActionRef | ✅ | — | Action triggered when a barcode is successfully scanned |
| `scan_param_name` | string | | `"scanned_value"` | The param name under which the scanned value is passed to the action |
| `continuous` | boolean | | `false` | Whether to continue scanning after a successful scan |
| `beep_on_scan` | boolean | | `true` | |
| `manual_input_fallback` | boolean | | `true` | Shows a text input for manual entry |

---

*End of Chapter 16*

---

## Phase 2, Volume III Complete

Chapters 09–16 constitute the complete Component System documentation. Any engineer who has read Chapters 01–16 can:

- Understand the full component contract and registry system
- Author any standard ERP surface using the complete component vocabulary
- Make correct decisions about component composition, layout, and theming
- Understand how forms, tables, dashboards, charts, workflows, and ERP-specific patterns are expressed as AST nodes
- Implement rendering engine support for any component type, given its schema

**Continuing Phase 2 with:** Volume IV — Rendering Architecture (Chapters 17–21)
