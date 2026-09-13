# AWO SDUI Architecture — Phase 1 Implementation Blueprint
### SDUI Principal Engineering Document

---

## PART 1 — SDUI Architecture Review

### Current layering

```
EntityDefinition
      ↓
CompiledSchema
      ↓
SDUI Generator
      ↓
Widget Tree (widget.Node)
      ↓
Renderer (amis.DefaultRenderer)
      ↓
AMIS JSON
```

### Is the layering correct?

Yes — structurally. Each layer has a clear transformation role. The violations are within layers, not between them.

### Mixed responsibility violations

**Generator mixes structural and presentational concerns.**
`buildList` constructs `map[string]any` AMIS button schemas directly (rowActions). This is a renderer concern inside the generator. The generator must emit `widget.ActionNode` values only. The renderer converts them to renderer-specific button schemas.

**`NodeSection` has two semantic roles.**
Inside `NodeTabs`, `NodeSection` acts as a tab pane container. Outside `NodeTabs`, it acts as a form section (fieldSet or group). A renderer must inspect the parent to distinguish these roles — fragile. Fix: introduce `NodeTabPane`. `NodeSection` = form section only. `NodeTabPane` = tab pane container only.

**`LayoutDef` contains visual grid properties.**
`ColumnDef.Span int` and derived `columnClassName` are visual properties (CSS grid) inside a semantic metadata type. Visual layout decisions belong in the renderer, not in `awo/def`. The semantic concept (field grouping) belongs in `def`. The visual expression (col-md-6) belongs in AMIS renderer hints.

**`setAllReadOnly` is a post-processing hack.**
Detail page read-only state is applied by a recursive walk over the already-built widget tree. A node added after the walk (e.g., by a future plugin hook) is not read-only. The correct model: read-only intent propagates during tree construction via a generator context parameter, not post-processing.

### Corrected layer responsibilities

```
EntityDefinition          — domain metadata, field groupings (semantic only)
      ↓
CompiledSchema            — resolved, validated, fingerprinted schema IR
      ↓
GeneratorContext          — view mode (create/edit/detail/list), viewer permissions,
                           read-only flag, locale — drives generator decisions
      ↓
SDUI Generator            — entity schema → widget.Node tree (semantic decisions only)
                           NO renderer-specific constructs
      ↓
Widget Tree               — renderer-independent IR
                           widget.Node, widget.NodeKind, widget.ActionNode
      ↓
RendererContext           — renderer hints (grid system, theme, locale formatting)
      ↓
Renderer                  — widget.Node tree → renderer-specific output
                           amis.DefaultRenderer → map[string]any (AMIS JSON)
                           mobile.Renderer      → Flutter widget tree (Phase C)
                           pdf.Renderer         → PDF element tree  (Phase C)
```

### Key invariant

**The generator must be testable without importing any renderer package.**
**The renderer must be testable without importing the generator package.**
The widget tree IR is the only shared contract.

---

## PART 2 — Widget System

### Current NodeKind inventory

Structural: `page`, `form`, `list`, `section`, `tabs`, `table`, `dialog`
Input fields: `field`, `text`, `textarea`, `number`, `select`, `date`, `datetime`, `switch`, `editor`
Interactive: `button`

### Missing node types

**`tab_pane`** — currently abused as `section` inside tabs. Add `NodeTabPane`.

**`rich_text`** — multi-format text editor (Quill, TipTap). Required for invoice notes, HR descriptions. Not the same as `textarea` (plain text).

**`file_upload`** — file and image attachment. Required for invoice attachments, product images. Distinct from text — has upload API, preview, file list.

**`money`** — currency amount with paired currency selector. ERP-critical. A number field cannot represent a currency amount with currency selector as a unit.

**`duration`** — HH:MM:SS duration input. Required for HR timesheets. Not a datetime.

**`color`** — color picker. Required for category color coding, status badges.

**`signature`** — signature capture pad. Required for delivery notes, approval workflows.

**`static_text`** — display-only text node (label, description block, divider). Not an input. Required for form instructions, section headers without grouping.

**`badge`** — status badge (colored pill). Required in list columns and detail summaries. Maps to AMIS `mapping` column type with color.

**`progress`** — progress bar (0–100). Required for workflow completion, budget consumption.

**`chart`** — inline chart within a form or detail panel (sparkline, donut). Required for dashboard panels and detail summary cards.

**`timeline`** — activity/audit timeline. Required for detail pages. Structured, not renderable as a list.

**`attachment_list`** — file attachment display and upload. Distinct from `file_upload` — this is a managed list with download, delete, metadata.

**`related_list`** — paginated list of related records (e.g., invoice payments, journal entries for an invoice). Mini-list within a detail page.

**`summary_card`** — key-value summary block at the top of a detail page (record ID, status, total amount, due date). Not a form — display only.

**`kanban`** — kanban board column layout. Required for CRM pipeline, HR leave calendar.

**`grid`** — editable data grid for line items. The most complex widget — covered in Part 5. Distinct from `table` (read-only display) and `list` (full crud2 list).

### Unnecessary node types

**`NodeField` (`field` kind)** — described as "generic single-line text for DynamicLink." This is redundant with `NodeText`. DynamicLink is a relationship concept — it should be `NodeSelect` with a dynamic data source, not a special field kind. Remove `NodeField`. If DynamicLink rendering differs from select, use `Props` to override until a dedicated `NodeLookup` type is introduced.

### Node extensibility

Current `Props map[string]any` is the escape hatch for renderer-specific properties not covered by typed fields. This is correct for v1.0. It enables renderer customization without polluting the widget IR with renderer-specific fields.

For Phase C (plugin architecture), introduce a `WidgetRegistration` mechanism — a third-party module can register a new `NodeKind` constant and a corresponding renderer function. Until Phase C, `Props` is the extension mechanism.

### Composition model

The current `Children []*Node` tree is correct. Composition by containment. No message-passing, no event bus. The tree is constructed once per request (or once per cache entry) and serialized.

The composition model must support:
- Variable-depth nesting (tabs → section → group → field)
- Sibling ordering (fields within a section appear in declaration order)
- Conditional omission (permission-gated nodes are absent, not hidden)

All three are satisfied by the current model.

### Renderer independence test

Ask: can the widget tree be rendered by a non-AMIS renderer without modification?

Currently: **No.** Reasons:
1. `NodeSection` dual role requires parent inspection (AMIS-specific knowledge)
2. `rowActions` are `map[string]any` (AMIS-specific) in the generator output
3. `columnClassName` prop (CSS grid class) is AMIS/Bootstrap-specific
4. Expression strings (`VisibleOn`) are JavaScript — non-AMIS renderers cannot evaluate them

After Phase A B-03 (renderButton), B-07 (toolbar split), and the `NodeTabPane` fix, items 1 and 2 are resolved. Items 3 and 4 require the `FieldGroupDef` refactor and expression IR — Phase B work.

### Widget hierarchy (target)

```
Node
├── Structural
│   ├── NodePage         — top-level page container
│   ├── NodeForm         — data-entry form
│   ├── NodeList         — paginated record list (crud2)
│   ├── NodeTable        — read-only inline table
│   ├── NodeGrid         — editable line-item grid
│   ├── NodeSection      — labeled/collapsible form section (fieldSet)
│   ├── NodeTabs         — tab container
│   ├── NodeTabPane      — single tab pane (child of NodeTabs)
│   ├── NodeDialog       — modal dialog
│   └── NodeSummaryCard  — key-value display header
├── Display
│   ├── NodeStaticText   — display-only text/label
│   ├── NodeBadge        — colored status pill
│   ├── NodeProgress     — progress bar
│   ├── NodeChart        — inline chart
│   ├── NodeTimeline     — activity/audit timeline
│   ├── NodeAttachmentList — file attachment display + upload
│   └── NodeRelatedList  — mini paginated related-record list
├── Input — simple
│   ├── NodeText         — single-line text
│   ├── NodeTextArea     — multi-line text
│   ├── NodeRichText     — formatted text editor
│   ├── NodeNumber       — numeric
│   ├── NodeMoney        — currency amount + currency selector
│   ├── NodeDate         — date picker
│   ├── NodeDateTime     — datetime picker
│   ├── NodeDuration     — HH:MM:SS
│   ├── NodeSwitch       — boolean toggle
│   ├── NodeColor        — color picker
│   ├── NodeSignature    — signature pad
│   └── NodeEditor       — JSON/code editor
├── Input — relational
│   ├── NodeSelect       — static or dynamic dropdown
│   ├── NodeLookup       — searchable FK reference selector
│   ├── NodeTreeSelect   — hierarchical selector (OU, COA)
│   └── NodeMultiSelect  — many-to-many tag selector
├── Upload
│   └── NodeFileUpload   — file/image upload
└── Interactive
    └── NodeButton       — action trigger
```

---

## PART 3 — Layout Engine

### Semantic vs visual distinction

**Semantic (belongs in `awo/def`):**
- Tabs as logical navigation units (General, Line Items, Notes)
- Sections as named field groups (Vendor Details, Billing Address)
- Field ordering within a section
- Section collapsibility (informational — "this section starts collapsed")
- Which fields belong together

**Visual (belongs in renderer/renderer hints):**
- Column count per section (2-column, 3-column, full-width)
- Grid span per field (col-md-6, col-md-4)
- Responsive breakpoints
- Section gap/spacing
- Tab orientation (top, left, bottom)

### Current problem

`ColumnDef.Span int` and the derived `columnClassName: "col-md-6"` prop are visual properties in `awo/def`. They belong in the renderer.

### Correct architecture

**`awo/def` retains (semantic):**

```go
type LayoutDef struct {
    Tabs     []TabDef
    Sections []SectionDef  // top-level sections if no tabs
}

type TabDef struct {
    Name     string
    Label    string
    Sections []SectionDef
}

type SectionDef struct {
    Name        string
    Label       string
    Collapsible bool
    Collapsed   bool      // initial state
    Columns     int       // number of columns (0 = auto)
    Fields      []string  // field names in order
}
```

`SectionDef.Columns int` is retained — it is a semantic intent ("lay these fields in 2 columns") not a visual detail ("use col-md-6"). The renderer interprets this intent in its grid system. AMIS uses `col-md-{12/columns}`. A mobile renderer uses a 2-column flex layout. A PDF renderer uses table cells. Same intent, different expression.

`ColumnDef` (the current struct with per-column field lists) is removed. Replace with flat `Fields []string` per section plus `Columns int`. This is simpler, renderer-neutral, and covers all ERP layout cases.

**Renderer translates:**

```
SectionDef.Columns == 1 → single field per row (full width)
SectionDef.Columns == 2 → AMIS: col-md-6 per field
SectionDef.Columns == 3 → AMIS: col-md-4 per field
SectionDef.Columns == 0 → renderer decides (auto)
```

The renderer is the only place `col-md-N` appears.

### Generator role in layout

The generator reads `EntitySchema.Layout` (populated by compiler from `EntityDefinition.EntityLayout()`). It constructs the widget tree according to the layout:

```
Layout present:
  → NodeTabs (if tabs declared)
    → NodeTabPane per TabDef
      → NodeSection per SectionDef (with label, collapsible flag)
        → field nodes in SectionDef.Fields order

Layout absent:
  → flat list of field nodes (existing flat fallback behavior)
```

The generator does NOT emit column class props. The renderer's `renderSection` function reads `n.Props["columns"]` (set by generator from `SectionDef.Columns`) and computes the grid class.

Generator sets: `out.Props["columns"] = sectionDef.Columns`
Renderer reads: `columns := n.Props["columns"].(int)` → emits `col-md-{12/columns}` on child nodes.

### Responsive layout

The renderer is responsible for responsive behavior, not the generator. The AMIS renderer emits:

```json
{
  "type": "group",
  "body": [...],
  "columnClassName": "col-md-6 col-sm-12"
}
```

The `col-sm-12` fallback (full width on small screens) is a renderer default — not a generator concern.

### `NodeTabPane` layout

Each `NodeTabPane` node carries a `Label` (tab title). Its `Children` are `NodeSection` nodes. The renderer maps this to AMIS `tabs` → `tabs[i]` with `title` and `body`.

The `NodeTabPane` ID field is used for direct tab navigation (deep linking). Generator sets `ID = tabDef.Name`.

---

## PART 4 — Relationship Components

### Current state

`NodeSelect` with a `DataSource` handles simple FK dropdowns. This covers single-FK fields with small option sets. It does not cover:

- Large option sets requiring search (>50 options)
- Hierarchical selections (chart of accounts, org unit)
- Many-to-many selections (tags, permissions)
- Master-detail relationships (invoice header + line items)
- Inline child record creation (add new supplier while creating invoice)

### Widget taxonomy for relationships

**`NodeSelect`** — unchanged. Static options or small dynamic sets. `DataSource.URL` populates options. `DataSource.SendOn` for conditional loading. No search input. ≤50 options.

**`NodeLookup`** — searchable FK reference selector. Renders as search-as-you-type input. User types → debounced API call → options appear. Selected value stored as UUID. Display value stored as label string (for form pre-population). Required for all FieldTypeLink fields with large record sets (vendors, customers, accounts). Maps to AMIS `picker` or `select` with `searchable: true`.

Properties on `widget.Node`:
```
DataSource.URL          — search endpoint, accepts ?q= parameter
DataSource.LabelField   — field name to display in option
DataSource.ValueField   — field name to store (default: id)
DataSource.SendOn       — condition for search (e.g., tenant filter)
Props["minLength"]      — minimum chars before search fires (default: 1)
Props["pageSize"]       — results per page (default: 20)
```

**`NodeTreeSelect`** — hierarchical option selector. Required for:
- Chart of accounts (account hierarchy)
- Org unit (ltree hierarchy)
- Category tree (product categories)

The data source returns a flat list with `parent_id` fields. The renderer builds the tree client-side. Maps to AMIS `nested-select` or `tree-select`.

Properties:
```
DataSource.URL          — returns [{id, label, parent_id}]
Props["valueField"]     — value to store (default: id)
Props["labelField"]     — display label
Props["rootValue"]      — root node parent_id (null or "")
Props["cascade"]        — whether selecting parent selects children
```

**`NodeMultiSelect`** — many-to-many tag selector. User selects multiple values from a list. Stored as array of IDs. Required for:
- Invoice tags
- User roles
- Product attributes

Maps to AMIS `select` with `multiple: true`. Or `checkboxes` for small static sets.

Properties:
```
DataSource.URL          — options source (optional; static Options otherwise)
Props["delimiter"]      — value delimiter for storage (default: comma)
Props["maxTagCount"]    — max displayed tags before "+N more"
```

**`NodeGrid`** — editable child record table (line items). Covered fully in Part 5.

**`NodeRelatedList`** — read-only mini paginated list of related records. Used in detail pages. Not editable inline — links to the related entity's own list view. Maps to AMIS `crud2` with limited toolbar.

Properties:
```
DataSource.URL          — filtered list endpoint (e.g., /invoices?vendor_id=${id})
Props["pageSize"]       — rows per page (default: 5)
Props["columns"]        — column definitions (reuses renderListColumns)
Props["viewLink"]       — URL template to open related record detail
```

### Generator responsibility for relationships

The generator inspects `FieldDef.Type`:

```
FieldTypeLink → inspect Options field
  → if linked entity has >50 records (compiler hint) → NodeLookup
  → if linked entity is hierarchical (ltree parent) → NodeTreeSelect
  → otherwise → NodeSelect with DataSource

FieldTypeLink + many=true → NodeMultiSelect

FieldTypeChildTable → NodeGrid (Part 5)
```

The compiler annotation ">50 records" is a hint, not a hard rule. A `FieldDef` option `LargeSet: bool` declared by the module author communicates this intent without runtime count queries.

### Autocomplete vs lookup distinction

Autocomplete = user types, suggestions appear, value is the typed text (freeform).
Lookup = user types, suggestions appear, value is selected from the suggestion (constrained to existing records).

AWO relationship widgets are always Lookup (constrained). Never allow freeform text in a FK field. If the user needs to create a new record inline, use a `NodeDialog` triggered by an "Add New" action on the lookup — not freeform text.

---

## PART 5 — Editable Grid

### Use cases in AWO ERP

- Invoice line items (quantity, unit price, tax, total)
- Journal entry lines (account, debit, credit, description)
- Payment allocations (invoice reference, amount)
- Purchase order items
- Inventory adjustments

All share: variable row count, row-level validation, column totals, bulk operations.

### Widget node design

`NodeGrid` is a `widget.Node` with `Kind: NodeGrid`. Its children are column definition nodes (same as `renderListColumns` — `NodeText`, `NodeNumber`, `NodeSelect`, `NodeDate` etc. describe column types). The grid itself manages rows.

```
Node{
    Kind:       NodeGrid,
    Name:       "lines",          // field name in form data
    DataSource: &DataSource{      // for existing record: loads current lines
        ReadURL: "/invoice/:id/lines",
        URL:     "/invoice/:id/lines", // submit endpoint
    },
    Children: []*Node{            // column definitions
        {Kind: NodeText,   Name: "description", Label: "Description"},
        {Kind: NodeSelect, Name: "account_id",  Label: "Account",
            DataSource: &DataSource{URL: "/accounts?q=${query}"}},
        {Kind: NodeNumber, Name: "quantity",    Label: "Qty"},
        {Kind: NodeNumber, Name: "unit_price",  Label: "Unit Price"},
        {Kind: NodeNumber, Name: "line_total",  Label: "Total",
            ReadOnly: true},      // computed — ReadOnly in grid
    },
    Actions: []*ActionNode{
        {Label: "Add Line",    ActionType: "addRow"},
        {Label: "Delete",      ActionType: "deleteRow", Level: "danger"},
        {Label: "Bulk Delete", ActionType: "bulkDelete"},
    },
    Props: map[string]any{
        "minRows":          1,
        "maxRows":          500,
        "showRowNumbers":   true,
        "showColumnTotals": []string{"quantity", "unit_price", "line_total"},
        "pageSize":         50,         // rows before pagination
        "virtualScroll":    true,       // for large grids
        "keyboardNav":      true,
        "dirtyTracking":    true,       // tracks unsaved changes
        "addRowPosition":   "bottom",   // or "top"
    },
}
```

### Renderer mapping (AMIS)

AMIS `editable-table` or `input-table` component. The AWO grid renderer:

1. Maps each child `Node` column definition to an AMIS column with `quickEdit: true` (inline editing)
2. Maps `NodeSelect` columns to AMIS `select` with `searchable: true` in the quick edit cell
3. Maps `ReadOnly: true` columns to display-only cells (no quick edit)
4. Computed columns (`ComputedFrom`) are display-only with value refreshed on dependency change
5. Row actions map to AMIS per-row action buttons

AMIS-specific grid config (emitted by renderer, not generator):
```json
{
  "type": "input-table",
  "name": "lines",
  "columns": [...],
  "addable": true,
  "removable": true,
  "showTableFooter": true,
  "footerSummary": [{"column": "line_total", "type": "sum"}]
}
```

### Validation

Row-level validation: each cell validates on blur (field-level). Row validates on row save (cross-field — quantity > 0, unit_price > 0). Grid validates on form submit (all rows must be valid, minimum row count met).

Validation errors attach to `{fieldName}[rowIndex].{columnName}`. The HTTP 422 response body carries these paths. The AMIS renderer maps them to cell-level error highlights.

### Change tracking

`dirtyTracking: true` prop causes the renderer to:
1. Mark modified cells with a visual indicator
2. Collect only dirty rows in the form submit payload (not all rows)
3. Distinguish "new row" from "modified row" from "deleted row" in the payload

Payload shape:
```json
{
  "lines": {
    "added":   [{...}],
    "modified": [{"id": "uuid", "quantity": 5}],
    "deleted":  ["uuid1", "uuid2"]
  }
}
```

The API handler maps this to three repository operations: Create, Update (FieldPatch), Delete.

### Keyboard navigation

Tab key: move to next editable cell in row.
Enter key: move to next row, same column.
Shift+Enter: add new row.
Delete key (on row): prompt confirm delete.
Ctrl+Z: undo last cell change (renderer-level, AMIS provides this).
Ctrl+D: duplicate row.

Keyboard navigation is a renderer concern — not in `widget.Node`. The `Props["keyboardNav"]` flag instructs the renderer to enable this behavior.

### Offline edits and error recovery

ERP usage patterns: user fills 50 line items. Network drops. They must not lose work.

The AMIS renderer supports form dirty-state persistence to `sessionStorage`. The grid component serializes dirty rows to session storage on every change. On page reload (after network recovery), the form restores from session storage and prompts: "You have unsaved changes. Restore?"

This is a renderer-level feature controlled by `Props["offlinePersistence"]` flag on the grid node. The generator sets this to `true` for any `NodeGrid` with `maxRows > 10` (heuristic: large grids are the ones users will lose work on).

### Nested tables

Some ERP structures have three levels: Order → Line Item → Tax Breakdown. The grid does not support recursive nesting (it leads to UX nightmares). Instead: clicking a line item row opens a dialog (`NodeDialog`) containing a secondary grid for the nested level. The generator produces this pattern when `FieldTypeChildTable` is nested inside another `FieldTypeChildTable`.

---

## PART 6 — Detail Pages

### Current state

`buildDetail` produces a read-only form. This is insufficient for a production ERP detail page. A vendor detail page needs: record summary header, action toolbar, activity timeline, attachments panel, related invoices list, audit history.

### Detail page structure

The detail page is a `NodePage` with a structured set of child nodes:

```
NodePage
├── NodeSummaryCard          — key-value header (status, ID, amounts)
├── NodeButton (toolbar)     — action buttons (Edit, Submit, Cancel, Print)
├── NodeTabs
│   ├── NodeTabPane "Details"
│   │   └── [read-only form fields per LayoutDef]
│   ├── NodeTabPane "Line Items" (if entity has child tables)
│   │   └── NodeTable (read-only grid of child records)
│   ├── NodeTabPane "Related"
│   │   └── NodeRelatedList per configured related entity
│   ├── NodeTabPane "Attachments"
│   │   └── NodeAttachmentList
│   ├── NodeTabPane "Activity"
│   │   └── NodeTimeline
│   └── NodeTabPane "Audit"
│       └── NodeTable (field-change audit records)
└── NodeWorkflowPanel (if entity has StateMachineDef)
```

### Framework primitives vs optional widgets

**Framework primitives (generated automatically from EntityDefinition):**
- Summary card (generated from entity's `SummaryFields []string` annotation)
- Action toolbar (generated from `EntityPermissions` + `EntityStateMachine`)
- Read-only field sections (generated from `LayoutDef`)
- Child record tables (generated from `FieldTypeChildTable` fields)
- Workflow panel (generated from `StateMachineDef`)

**Optional widgets (declared by module author in `EntityDefinition`):**
- Activity timeline (requires `Timeline: bool` flag — not every entity has activity)
- Attachment list (requires `Attachments: bool` flag — not every entity supports attachments)
- Related records tabs (requires explicit `RelatedEntities []RelatedEntityDef` declaration)
- Audit tab (requires `Audited: bool` on fields)

### `SummaryCardDef` in `EntityDefinition`

Module authors declare which fields appear in the summary card:

```go
type SummaryCardDef struct {
    TitleField  string    // e.g., "name" or "invoice_number"
    StatusField string    // e.g., "status" — rendered as NodeBadge
    Fields      []string  // key-value pairs to display
}
```

`EntityDefinition` gains `EntitySummaryCard() SummaryCardDef`. Default (empty): generator uses entity name field + status field if detected.

### Workflow panel

When `EntityStateMachine()` returns a non-nil `*StateMachineDef`, the generator includes a `NodeWorkflowPanel` in the detail page. This node renders:
- Current state (highlighted)
- Available transitions from current state (as action buttons, filtered by viewer permissions)
- Transition history (last N state changes with actor and timestamp)

`NodeWorkflowPanel` is a new `NodeKind`. The AMIS renderer maps it to a custom AMIS component (or a styled `steps` component). The node carries:
```
Node{
    Kind:  NodeWorkflowPanel,
    Name:  "workflow",
    Props: {
        "currentState":   "submitted",
        "states":         ["draft", "submitted", "posted", "reversed"],
        "transitions":    [{"from":"submitted","to":"posted","label":"Post","api":"/..."}],
        "history":        DataSource{URL: "/entity/:id/transitions"},
    },
}
```

### Activity timeline

`NodeTimeline` renders a chronological list of domain events for the record. Each entry has: actor, timestamp, action description, optional changed fields.

The data source for the timeline is the entity's outbox events (filtered by record ID). The generator sets `DataSource.URL = "/{entity}/{id}/activity"`. This endpoint is automatically registered by the router when `Timeline: true` is set on the entity definition.

---

## PART 7 — Dashboard Architecture

### Dashboard primitives

Dashboards in AWO are not freeform canvases — they are role-driven, schema-derived aggregation views. A finance manager's dashboard is derived from Finance entity schemas. An inventory manager's dashboard is derived from Inventory schemas.

### Widget taxonomy

**`NodeDashboardPage`** — top-level dashboard container. Has a filter toolbar and a grid of panels. Not a `NodePage` — dashboards are a distinct layout type.

**`NodeKPICard`** — single metric display (Total Revenue, Open Invoices, Overdue Amount). Shows: value, trend (vs previous period), label, icon. Data from a single aggregation API endpoint.

**`NodeChartPanel`** — chart widget. Supports: line, bar, pie, donut, area. Data from a time-series API endpoint. Configurable X/Y axes, groupBy field, date range.

**`NodeTablePanel`** — summary table widget. Paginated, sortable. Not a full crud2 — no edit, no filters. Data from a read-only list endpoint. Configurable columns.

**`NodeFilterBar`** — dashboard filter controls (date range, entity selectors). Filter values propagate to all panels on the dashboard via AMIS data scope.

**`NodeSavedViews`** — saved filter preset selector. User saves "Q4 2026 Finance" preset. Restored on next visit. Stored in user preferences (platform settings table).

### Dashboard definition in EntityDefinition

Module authors declare dashboard definitions as part of their module, not per-entity:

```go
type DashboardDef struct {
    Name    string
    Label   string
    Roles   []string          // permission identifiers that can see this dashboard
    Panels  []DashboardPanel
}

type DashboardPanel struct {
    Kind     DashboardPanelKind  // KPI, Chart, Table
    Label    string
    DataURL  string             // aggregation API endpoint
    Span     int                // 1–12 grid span
    Config   map[string]any     // chart type, axes, etc.
}
```

`EntityDefinition` does NOT gain dashboard methods — dashboards span multiple entities. A separate `DashboardDefinition` interface (registered separately in the registry) declares dashboards. The SDUI generator has a `GenerateDashboard(dash *DashboardDef, viewer ViewerContext)` method alongside the existing entity page generation.

### Role-based layouts

`DashboardDef.Roles` contains permission identifiers (not role names). The generator filters panels: if the viewer lacks a panel's required permission, the panel is absent from the dashboard widget tree (not hidden — absent).

### Live updates

`NodeKPICard` and `NodeChartPanel` support polling via `Props["refreshInterval"]` (seconds). The AMIS renderer emits `interval: N` on the data source. For real-time requirements (Phase D): SSE or WebSocket endpoint — the renderer subscribes and updates in-place.

### Drill-down

`NodeChartPanel` drill-down: clicking a bar in "Revenue by Customer" chart navigates to the customer detail page or a filtered list of invoices for that customer. The generator sets `Props["drillDownURL"]` on the chart panel. The renderer maps clicks to navigation actions.

---

## PART 8 — Plugin Architecture

### Design constraints

- Third-party module registers widgets, renderers, layout transforms **without modifying framework source code**
- Registration happens at startup via `init()` (consistent with EntityDefinition registration)
- The framework does not dynamically load `.so` files at v1.0 (ADR decision)
- Plugin registration must be type-safe at compile time

### Extension points

**1. Custom NodeKind registration**

```go
// In awo/sdui package
type WidgetDescriptor struct {
    Kind         widget.NodeKind
    DisplayName  string
    // Validates that a Node of this Kind has required Props
    Validate     func(n *widget.Node) error
}

func RegisterWidget(d WidgetDescriptor)
```

A module that introduces `NodeSignature` calls `sdui.RegisterWidget(WidgetDescriptor{Kind: "signature", ...})` in `init()`. The generator can then emit `NodeSignature` nodes. Existing renderers that don't know the kind return an error (per ADR: unknown kinds are errors, not silent drops).

**2. Custom renderer extension**

```go
// In awo/sdui/amis package
type NodeRenderer func(r *DefaultRenderer, n *widget.Node) (map[string]any, error)

func RegisterNodeRenderer(kind widget.NodeKind, renderer NodeRenderer)
```

A module registering `NodeSignature` also registers its AMIS renderer. The `DefaultRenderer.renderNode` switch falls through to the registered renderer map before returning the "unknown kind" error. This avoids modifying the framework's switch statement.

**3. Layout transform**

```go
// In awo/sdui package
type LayoutTransform func(schema *compiler.EntitySchema, view ViewMode) *LayoutOverride

func RegisterLayoutTransform(entityName string, transform LayoutTransform)
```

A module can override the layout for a specific entity (e.g., a Finance module providing a specialized invoice form layout that differs from the generic layout engine output). The generator applies registered transforms after the default layout engine, before emitting nodes.

**4. Field renderer override**

```go
// In awo/sdui package
type FieldNodeBuilder func(field *def.FieldDef, schema *compiler.EntitySchema) *widget.Node

func RegisterFieldRenderer(fieldType def.FieldType, builder FieldNodeBuilder)
```

If a module introduces `FieldTypeMoney`, it registers a field renderer that returns `NodeMoney` instead of the default `NodeNumber`. The generator calls registered field renderers before the built-in `fieldToNode` switch.

**5. Dashboard component registration**

```go
// In awo/sdui package
type DashboardPanelRenderer func(panel DashboardPanel, viewer auth.ViewerContext) *widget.Node

func RegisterDashboardPanel(kind DashboardPanelKind, renderer DashboardPanelRenderer)
```

### Plugin registration lifecycle

All `RegisterWidget`, `RegisterNodeRenderer`, `RegisterLayoutTransform`, `RegisterFieldRenderer` calls must happen in `init()` — before the SDUI generator is constructed. The generator captures the registered state at construction time. Registrations after generator construction are ignored with a logged warning.

This mirrors the EntityDefinition registration pattern — consistent developer experience.

### Anti-patterns to forbid

- Do not allow plugins to replace core renderers (`renderForm`, `renderList`) — only extend
- Do not allow plugins to inject into the compiler pipeline (that is a Phase C compiler extension point)
- Do not allow plugins to modify the widget tree after generation — transforms happen during generation, not after

---

## PART 9 — Performance

### Caching strategy

Three levels of caching:

**Level 1: Compiled schema cache (in-process)**
`CompiledSchema` is computed once at startup and held in memory. It never changes during the process lifetime. No TTL. Invalidated only by process restart (new deployment). This is already the current behavior.

**Level 2: Widget tree cache (Redis)**
Key: `sdui:tree:{entity}:{view}:{schema_fingerprint}:{roles_sha256_prefix16}:{tenant_id}`

The widget tree (`[]*widget.Node`) is the output of the generator. For a given entity + view mode + permission set + tenant, the widget tree is identical (assuming no tenant-specific layout overrides). Cache the serialized widget tree. Skip the generator on cache hit.

TTL: 24 hours. Invalidated by schema fingerprint change (new deployment automatically invalidates all entries for old fingerprint).

Serialization: `encoding/json` (the widget tree is already JSON-serializable). A binary serialization (`encoding/gob` or protobuf) would be faster but adds complexity — defer to Phase E if profiling reveals widget tree deserialization as a bottleneck.

**Level 3: AMIS JSON cache (Redis)**
Key: `sdui:amis:{entity}:{view}:{schema_fingerprint}:{roles_sha256_prefix16}:{tenant_id}`

The AMIS JSON (`map[string]any` → marshaled JSON bytes) is the renderer output. Cache the final JSON bytes. The HTTP handler writes cached bytes directly to the response with `Content-Type: application/json`. Zero allocation on cache hit.

TTL: 24 hours. Same fingerprint-based invalidation.

**Cache warming:** On startup, pre-generate SDUI JSON for common view/entity combinations. Use a goroutine pool to generate in background. Common = any entity in `DependencyOrder` × {list, create, edit, detail} views × active role combinations. This eliminates cold-start latency on first deployment.

### What NOT to cache

Tenant-specific layout overrides (Phase C plugin feature): the cache key must include a "layout override version" component if tenant-level customization is enabled. At v1.0, no tenant-level overrides — cache key is safe.

Records with row-level `PolicyFunc` that affects field visibility: the cache key must include viewer identity (not just roles). At v1.0, field visibility is role-based only — cache key is safe.

Dashboard panels with live data: never cache the data payload, only the widget tree (schema). Data is fetched client-side on each dashboard load.

### Generator performance

The generator walks the entity schema to build the widget tree. The dominant cost is:
1. Iterating fields to call `fieldToNode` (O(N) where N = fields)
2. Resolving layout (O(M) where M = layout nodes)
3. Permission checks per field (O(F) where F = permission identifiers)

For an entity with 50 fields and a complex layout, tree generation should complete in <1ms. The Redis cache exists for the rare large entities (100+ fields) where generation overhead accumulates over many requests.

Benchmark target: `BenchmarkGenerator_LargeEntity` (100 fields, 5 tabs, 20 sections) < 2ms per operation, < 10KB allocations.

### Incremental generation

At v1.0, generation is all-or-nothing (full tree per request). For Phase E (incremental compilation), the generator could produce partial trees (one tab at a time) for lazy rendering. The widget tree structure supports this — `NodeTabPane` children are naturally independent subtrees.

### Large forms and virtual scrolling

Forms with >50 fields should use `NodeTabs` to split into logical sections — never a single vertical scroll. The layout engine handles this automatically when the entity's `LayoutDef` declares tabs.

Tables with >100 rows use virtual scrolling (`Props["virtualScroll"]: true`). The AMIS renderer enables virtualization on `crud2` and `input-table` components. The generator sets this prop automatically when `NodeList` or `NodeGrid` has no explicit page size or when `Props["maxRows"] > 100`.

### Cache invalidation triggers

| Event | Cache invalidated |
|---|---|
| New deployment (fingerprint changes) | All Level 2 + Level 3 entries for old fingerprint (expire via TTL) |
| Tenant-specific setting change | Tenant-specific entries only (Phase C — not at v1.0) |
| Role assignment change | Role-specific entries only (via `roles_sha256` component change on next request) |
| Manual cache flush (`/admin/cache/flush`) | All SDUI entries for the tenant |

---

## PART 10 — Implementation Sequence

### Phase S1 — Foundation correctness (immediate, 1–2 weeks)
**Prerequisite for all subsequent SDUI work.**

**Goal:** Fix the renderer-independence violations and semantic ambiguities that make the widget tree an unreliable contract.

**Deliverables:**
1. `NodeTabPane` introduced in `awo/sdui/widget/node.go`
2. Generator emits `NodeTabPane` for tab pane containers (not `NodeSection`)
3. `NodeSection` reserved for form sections only
4. `renderButton` calls `applyCommon` (B-03 fix)
5. `sanitizeText()` applied to all label/description emissions (B-01)
6. `rowActions` refactored: generator emits `widget.ActionNode`; renderer converts to AMIS buttons
7. `buildEdgeTables()` extracted; called from both `buildForm` and `buildDetail` (B-06)
8. Toolbar duplication fixed: `EntityPageActions`/`EntityListActions` split (B-07)
9. Race condition test: `TestGenerator_ConcurrentRender`

**Packages affected:** `awo/sdui/widget`, `awo/sdui`, `awo/sdui/amis`, `awo/def`

**Dependencies:** Phase A B-series items (overlap intentional)

**Breaking changes:**
- `NodeSection` in `NodeTabs` is now `NodeTabPane` — any renderer testing for `NodeSection` inside tabs must update
- `EntityDefinition` interface gains `EntityPageActions()` and `EntityListActions()`

**Tests:**
- `TestRenderer_NodeTabPane_renders_as_tab`
- `TestRenderer_NodeSection_never_in_tabs`
- `TestGenerator_RowActions_are_ActionNodes_not_maps`
- `TestGenerator_Toolbar_appears_once_in_list`
- `TestGenerator_Detail_includes_edge_tables`

**Completion criteria:** Widget tree is renderer-independent for all currently supported node types. No `map[string]any` in generator output. `-race` clean.

**Risk: Low**

---

### Phase S2 — Layout engine hardening (2–3 weeks)
**Goal:** Make the layout engine the definitive form layout mechanism. Remove visual properties from `awo/def`.

**Deliverables:**
1. `ColumnDef` removed from `awo/def`; replaced with `SectionDef.Columns int` + `SectionDef.Fields []string`
2. `columnClassName` prop removed from generator; moved to renderer
3. AMIS renderer computes `col-md-{12/columns}` from `n.Props["columns"]`
4. `setAllReadOnly` replaced with `buildLayoutNodes(ctx GeneratorContext)` where `GeneratorContext.ReadOnly bool` propagates during tree construction
5. `GeneratorContext` type introduced: `{ViewMode, ReadOnly, Viewer, Locale, Fingerprint}`
6. Generator refactored: all functions accept `GeneratorContext` instead of ad hoc parameters
7. Layout engine handles entities without `LayoutDef` (flat fallback) and with `LayoutDef` (structured)

**Packages affected:** `awo/def`, `awo/sdui`, `awo/sdui/amis`

**Dependencies:** Phase S1 complete

**Breaking changes:**
- `ColumnDef` type removed from `awo/def` — module authors using it must update to `SectionDef.Columns`
- `EntityDefinition.EntityLayout()` signature unchanged; internal `LayoutDef` structure changes

**Tests:**
- `TestLayout_SingleColumn_no_span_props`
- `TestLayout_TwoColumn_AMIS_emits_col_md_6`
- `TestLayout_ThreeColumn_AMIS_emits_col_md_4`
- `TestLayout_ZeroColumn_renderer_decides`
- `TestLayout_Detail_all_fields_readonly_via_context`
- `TestLayout_Detail_no_setAllReadOnly_post_processing`

**Completion criteria:** Generator emits no CSS class strings. Renderer tests for column layout pass. `GeneratorContext` is the single source of view-mode truth.

**Risk: Medium** (ColumnDef removal is a breaking def change; coordinate with all entity authors)

---

### Phase S3 — Relationship widgets (3–4 weeks)
**Goal:** Full relationship widget coverage for FK fields.

**Deliverables:**
1. `NodeLookup` — searchable FK reference selector
2. `NodeTreeSelect` — hierarchical selector
3. `NodeMultiSelect` — many-to-many tag selector
4. Generator logic: `FieldTypeLink` → `NodeLookup` (when `LargeSet: true`) or `NodeSelect`
5. Generator logic: `FieldTypeLink` with `HierarchicalParent: string` → `NodeTreeSelect`
6. `FieldDef` gains `LargeSet bool` and `HierarchicalParent string` optional fields
7. `NodeLookup` AMIS renderer: `select` with `searchable: true`, `source` as API
8. `NodeTreeSelect` AMIS renderer: `nested-select` with flat-to-tree transform
9. `NodeMultiSelect` AMIS renderer: `select` with `multiple: true`
10. `NodeRelatedList` for detail pages
11. AMIS renderer for `NodeRelatedList`: mini `crud2` without toolbar

**Packages affected:** `awo/sdui/widget`, `awo/sdui`, `awo/sdui/amis`, `awo/def`

**Dependencies:** Phase S2 complete

**Breaking changes:**
- `FieldDef` gains new optional fields (additive, backward compatible)
- Entities with large FK sets may see generator produce `NodeLookup` instead of `NodeSelect` — behavior change if `LargeSet: true` is set

**Tests:**
- `TestGenerator_LargeSet_Link_becomes_NodeLookup`
- `TestGenerator_Hierarchical_Link_becomes_NodeTreeSelect`
- `TestRenderer_NodeLookup_emits_searchable_select`
- `TestRenderer_NodeTreeSelect_emits_nested_select`
- `TestRenderer_NodeMultiSelect_emits_multiple_select`
- `TestRenderer_NodeRelatedList_emits_crud2_without_toolbar`

**Completion criteria:** Finance entity FK fields (account_id, vendor_id, currency_id) all resolve to the correct widget kind. Tree selector renders chart of accounts hierarchy.

**Risk: Medium**

---

### Phase S4 — Editable grid (4–5 weeks)
**Goal:** Production-grade editable grid for ERP line items.

**Deliverables:**
1. `NodeGrid` NodeKind added
2. Generator: `FieldTypeChildTable` → `NodeGrid` in form views, `NodeTable` in detail views
3. Grid node structure: children as column definition nodes, `DataSource` for load/submit, `Props` for grid behavior
4. AMIS renderer: `NodeGrid` → `input-table` with `quickEdit: true` columns
5. Column type mapping: each column child node type maps to AMIS `quickEdit` component type
6. Row validation: field-level on blur, row-level on row save, grid-level on form submit
7. `showColumnTotals` support: sum/count per designated columns
8. Dirty tracking: payload shape with `added`, `modified`, `deleted`
9. Keyboard navigation props
10. `offlinePersistence` prop: session storage restore on reload
11. Nested child table: `NodeGrid` column with `NodeDialog` trigger for secondary grid

**Packages affected:** `awo/sdui/widget`, `awo/sdui`, `awo/sdui/amis`

**Dependencies:** Phase S3 complete (grid cells use relationship widgets for FK columns)

**Breaking changes:** None (new NodeKind addition)

**Tests:**
- `TestGenerator_ChildTable_becomes_NodeGrid_in_form`
- `TestGenerator_ChildTable_becomes_NodeTable_in_detail`
- `TestRenderer_NodeGrid_emits_input_table`
- `TestRenderer_NodeGrid_quickEdit_column_types`
- `TestRenderer_NodeGrid_computed_column_readonly`
- `TestGrid_DirtyPayload_shape`

**Completion criteria:** Invoice line items form renders as editable grid. Column totals display. Dirty rows-only submit works. Offline persistence restores on reload.

**Risk: High** (most complex SDUI widget; AMIS input-table has known limitations)

---

### Phase S5 — Detail page architecture (2–3 weeks)
**Goal:** Detail pages include summary card, workflow panel, activity timeline, attachments.

**Deliverables:**
1. `NodeSummaryCard` NodeKind and AMIS renderer
2. `SummaryCardDef` in `awo/def`; `EntityDefinition` gains `EntitySummaryCard() SummaryCardDef`
3. `NodeWorkflowPanel` NodeKind and AMIS renderer
4. `NodeTimeline` NodeKind and AMIS renderer
5. `NodeAttachmentList` NodeKind and AMIS renderer
6. `buildDetail` refactored: produces full detail page structure (summary card + tabs + workflow panel)
7. Detail tabs: Details, Line Items (if child tables), Related (if `RelatedEntities` declared), Attachments (if enabled), Activity (if enabled), Audit (if audited fields)
8. `EntityDefinition` gains `EntityDetailConfig() DetailConfig` returning optional feature flags
9. `DetailConfig` struct: `{Timeline bool, Attachments bool, RelatedEntities []RelatedEntityDef}`

**Packages affected:** `awo/sdui/widget`, `awo/sdui`, `awo/sdui/amis`, `awo/def`

**Dependencies:** Phase S4 complete (detail page may include read-only grid for child tables)

**Breaking changes:**
- `EntityDefinition` interface gains `EntitySummaryCard()` and `EntityDetailConfig()`
- `buildDetail` output changes significantly (new node structure)

**Tests:**
- `TestDetail_includes_summary_card`
- `TestDetail_includes_workflow_panel_when_state_machine`
- `TestDetail_no_workflow_panel_without_state_machine`
- `TestDetail_timeline_tab_when_enabled`
- `TestDetail_attachments_tab_when_enabled`
- `TestDetail_audit_tab_when_audited_fields`

**Completion criteria:** Finance invoice detail page renders with summary card, state machine panel (submit/post/cancel buttons), line items table, activity timeline.

**Risk: Medium**

---

### Phase S6 — Dashboard architecture (3–4 weeks)
**Goal:** Role-based dashboard page generation from DashboardDef.

**Deliverables:**
1. `DashboardDef`, `DashboardPanel`, `DashboardPanelKind` types in `awo/def`
2. `DashboardDefinition` interface and registry (separate from `EntityDefinition` registry)
3. `NodeDashboardPage`, `NodeKPICard`, `NodeChartPanel`, `NodeTablePanel`, `NodeFilterBar` NodeKinds
4. Dashboard generator: `GenerateDashboard(dash *DashboardDef, viewer ViewerContext) (*widget.Node, error)`
5. AMIS renderers for all dashboard node kinds
6. Permission filtering: panels absent for viewers without required permissions
7. Filter bar: date range + entity selectors that propagate to all panels
8. Saved views: `Props["savedViewsKey"]` → user preference key for save/restore
9. Dashboard cache: same fingerprint-based cache as entity SDUI pages
10. `GET /api/v1/dashboard/{name}` endpoint auto-registered for each `DashboardDef`

**Packages affected:** `awo/sdui/widget`, `awo/sdui`, `awo/sdui/amis`, `awo/def`, `awo/registry`, `awo/api/router`

**Dependencies:** Phase S5 complete; Phase A complete (for permission filtering)

**Breaking changes:** New registry for `DashboardDefinition` — additive

**Tests:**
- `TestDashboard_panels_filtered_by_permission`
- `TestDashboard_KPI_card_renders`
- `TestDashboard_chart_panel_renders`
- `TestDashboard_filter_bar_propagates_to_panels`
- `TestDashboard_cache_key_includes_roles`

**Completion criteria:** Finance dashboard with revenue KPI, invoice aging chart, overdue payments table renders correctly for finance-manager role. Non-finance roles see empty dashboard.

**Risk: Medium**

---

### Phase S7 — Plugin architecture (2–3 weeks)
**Goal:** Third-party modules can register custom widgets without modifying framework source.

**Deliverables:**
1. `sdui.RegisterWidget(WidgetDescriptor)` — custom NodeKind registration
2. `amis.RegisterNodeRenderer(kind, NodeRenderer)` — custom AMIS renderer registration
3. `sdui.RegisterLayoutTransform(entityName, LayoutTransform)` — per-entity layout override
4. `sdui.RegisterFieldRenderer(fieldType, FieldNodeBuilder)` — per-FieldType node builder
5. `sdui.RegisterDashboardPanel(kind, DashboardPanelRenderer)` — custom dashboard component
6. All registrations captured at generator construction time
7. Documentation: plugin authoring guide with example (NodeSignature widget)

**Packages affected:** `awo/sdui`, `awo/sdui/amis`

**Dependencies:** Phase S6 complete (all built-in widgets exist before extension points are designed)

**Breaking changes:** None (new additive APIs)

**Tests:**
- `TestPlugin_custom_node_kind_renders`
- `TestPlugin_unknown_kind_without_registration_returns_error`
- `TestPlugin_field_renderer_override`
- `TestPlugin_layout_transform_applied`
- `TestPlugin_dashboard_panel_registered`

**Completion criteria:** A test module registers `NodeSignature` widget with AMIS renderer. Generator emits it for a `FieldTypeSignature` field. AMIS renderer produces correct JSON. No framework source modified.

**Risk: Low**

---

### Phase S8 — Performance hardening (2 weeks)
**Goal:** Widget tree cache and AMIS JSON cache with cache warming.

**Deliverables:**
1. Level 2 cache: widget tree serialized to Redis (`sdui:tree:{...}` key)
2. Level 3 cache: AMIS JSON bytes in Redis (`sdui:amis:{...}` key)
3. Cache key construction uses `CompiledSchema.SchemaFingerprint`
4. Cache warming goroutine at startup: pre-generates common view/entity combinations
5. `BenchmarkGenerator_LargeEntity` benchmark added (100 fields, 5 tabs)
6. `BenchmarkRenderer_LargeForm` benchmark added
7. Virtual scroll auto-enabled for `NodeList` and `NodeGrid` with `maxRows > 100`
8. `/admin/cache/flush?scope=sdui` endpoint for manual cache invalidation
9. `authz_sdui_cache_hit_total`, `authz_sdui_cache_miss_total`, `authz_sdui_generate_duration_seconds` metrics

**Packages affected:** `awo/sdui`, `awo/sdui/amis`, `awo/observability`

**Dependencies:** Phase A2 (fingerprint), Phase S7 complete

**Breaking changes:** None

**Tests:**
- `TestCache_hit_returns_same_JSON`
- `TestCache_fingerprint_change_causes_miss`
- `TestCache_warming_pre_generates_entries`
- `BenchmarkGenerator_LargeEntity` < 2ms
- `BenchmarkRenderer_LargeForm` < 1ms

**Completion criteria:** Cache hit rate >95% after warm-up. Cold start (post-deploy) recovers within 60 seconds via warming. Benchmarks within targets.

**Risk: Low**

---

## PART 11 — Readiness Assessment

### 1. Which SDUI feature blocks Finance?

**Editable grid (Phase S4) is the Finance blocker.**

Invoice line items are the core of the Finance module's UI. Without `NodeGrid`, the invoice form either has no line items UI or uses a workaround (flat form with fixed number of line item fields). Neither is acceptable for a production ERP. Every other Finance form (COA, vendor, payment) can be built with existing widgets.

Second blocker: **relationship widgets (Phase S3)**. Account selectors (chart of accounts), vendor selectors, and currency selectors in invoice forms require `NodeLookup` and `NodeTreeSelect`. The current `NodeSelect` is insufficient for large record sets.

Third blocker: **workflow panel (Phase S5, partial)**. Journal entry and payment forms need state machine UI (submit/post/cancel buttons). This is a subset of S5 — only the `NodeWorkflowPanel` generation is needed, not the full detail page refactor.

**Finance can begin after S4 + S3 + partial S5 (workflow panel only).**

### 2. Which SDUI feature blocks Inventory?

**Kanban view blocks Inventory** — not in the current roadmap.

Inventory management requires: stock movement tracking (editable grid — already in S4), product category tree (tree selector — S3), and warehouse/bin location selection (tree selector — S3).

The real Inventory blocker is **`NodeTreeSelect` for hierarchical location selection** (warehouse → zone → bin). This is part of S3.

Inventory can begin after S3. The kanban board (for demand planning / replenishment workflow visualization) is a separate widget not in the current roadmap — add it as S6b or defer to a module-authored plugin.

### 3. Which SDUI feature should be removed?

**`NodeField` (the `field` kind).**

It is described as "generic single-line text for DynamicLink" but is functionally identical to `NodeText`. DynamicLink fields should use `NodeLookup` — a properly typed relationship widget. `NodeField` represents an architectural ambiguity (what is a "dynamic link" at the widget level?). Remove it. Any entity using it should migrate to `NodeText` or `NodeLookup` depending on the field type. This is a one-commit change.

### 4. Which feature should be promoted earlier?

**`GeneratorContext` (Phase S2) should be part of Phase S1.**

The `GeneratorContext` struct — carrying `{ViewMode, ReadOnly, Viewer, Locale, Fingerprint}` — is the correct way to eliminate all the ad hoc parameters currently passed through the generator's call stack (`isEdit bool`, implicit viewer from closure). Every Phase S1 fix involves some parameter threading issue that `GeneratorContext` would resolve.

Implementing `GeneratorContext` as part of S1 means S2–S8 are built on a clean foundation. Without it, each subsequent phase inherits the parameter threading debt and creates its own workarounds.

Promote `GeneratorContext` to the first deliverable of Phase S1.

### 5. Which architectural decision will matter most over the next ten years?

**The widget tree IR (`widget.Node`) as the renderer contract.**

This is the single decision that determines whether AWO's SDUI remains a sophisticated AMIS configuration generator or becomes a true multi-renderer SDUI framework.

If the widget tree IR is maintained as a strict, renderer-independent contract:
- Mobile renderers (Flutter) can be added in Phase C without touching the generator
- PDF renderers can be added for document generation without touching the generator
- Accessibility trees can be generated from the same IR
- AI-generated UI (Phase D) outputs to the widget tree IR — any downstream renderer benefits automatically
- Third-party renderers (Retool, Appsmith connectors) are feasible

If the widget tree IR accumulates renderer-specific concerns (AMIS expressions as strings, CSS class props, AMIS component names in NodeKind values), it becomes an AMIS schema generator. Replacing AMIS becomes a framework rewrite, not a renderer swap.

The `NodeTabPane` fix (Phase S1), the `columnClassName` removal (Phase S2), and the expression IR (deferred but necessary by Phase C) are all instances of this same decision. Each time a renderer-specific concern is kept out of the widget tree, the ten-year architecture is preserved. Each time it leaks in, the cost of future renderer independence increases.

**This decision must be made correctly in Phase S1 and defended in every subsequent code review.**
