# Chapter 10 — Layout System

> **Volume:** III — Component System
> **Phase:** 2
> **Audience:** Web/Mobile Engineers, UI Framework Developers, Backend Engineers
> **Prerequisites:** Chapters 01–09

---

## Table of Contents

- [10.1 Layout Philosophy](#101-layout-philosophy)
- [10.2 Layout Model Overview](#102-layout-model-overview)
- [10.3 Grid System](#103-grid-system)
- [10.4 Responsive Breakpoints](#104-responsive-breakpoints)
- [10.5 Layout Containers](#105-layout-containers)
- [10.6 Spacing System](#106-spacing-system)
- [10.7 Layout for Mobile](#107-layout-for-mobile)
- [10.8 Layout for Web](#108-layout-for-web)
- [10.9 Layout Serialization in AST](#109-layout-serialization-in-ast)

---

## 10.1 Layout Philosophy

Layout in AwoERP is governed by two principles that are in deliberate tension with each other, and whose balance defines the platform's approach.

**Principle 1: The backend controls spatial intent.** The backend decides that a form has two columns, that a dashboard has a three-column grid, that a detail view uses a master-detail split. These are business and UX decisions encoded in the AST as layout nodes. They are not left to the rendering engine's discretion.

**Principle 2: The rendering engine controls physical realization.** Two columns at 800px screen width is a sensible layout. Two columns at 375px is unusable. The rendering engine knows the physical constraints of its environment and is authorized — indeed, required — to adapt the backend's spatial intent to those constraints. It does not override the intent; it realizes it appropriately.

The boundary between these two principles is expressed through the **responsive rules system**: the backend specifies the intended layout for each breakpoint category, and the rendering engine selects the appropriate specification for its actual screen size. The backend's intent is complete (it covers all breakpoint categories); the rendering engine's role is selection, not invention.

This makes AwoERP's layout model fundamentally different from a CSS framework that delegates all layout decisions to the frontend. The backend always has an answer for every screen size. The rendering engine never has to guess.

---

## 10.2 Layout Model Overview

AwoERP provides six layout primitives. Every spatial arrangement of components is expressed as a composition of these primitives.

### 10.2.1 Flow Layout

Flow layout arranges children in a left-to-right, top-to-bottom flow, wrapping to new lines when the container width is exceeded. It is the default layout for simple, linear content.

```
┌─────────────────────────────────┐
│ [Child A] [Child B] [Child C]   │
│ [Child D] [Child E]             │
└─────────────────────────────────┘
```

Use flow layout for: tag lists, button groups, inline badge collections, breadcrumbs.

### 10.2.2 Grid Layout

Grid layout places children in a defined grid of columns and rows. Children can span multiple columns and rows. Grid is the primary layout for forms, dashboards, and structured content displays.

```
┌─────────────────────────────────┐
│ Col 1       │ Col 2  │ Col 3   │
│─────────────┼────────┼─────────│
│ [Child A    │ [B]    │ [C]     │
│  spans 1]   │        │         │
│─────────────┼────────┴─────────│
│ [Child D spans 2]    │ [E]     │
└─────────────────────────────────┘
```

### 10.2.3 Flex Layout

Flex layout arranges children along a primary axis (horizontal or vertical) with control over alignment, justification, and spacing. It is appropriate for toolbars, action bars, header rows, and any arrangement that needs precise control along a single axis.

```
Horizontal flex, space-between:
┌─────────────────────────────────┐
│ [Child A]        [B]     [C]    │
└─────────────────────────────────┘

Vertical flex, gap md:
┌─────────────┐
│ [Child A]   │
│             │
│ [Child B]   │
│             │
│ [Child C]   │
└─────────────┘
```

### 10.2.4 Stack Layout (Vertical / Horizontal)

Stack layout is a simplified flex layout for the most common case: a sequence of children arranged vertically or horizontally with a uniform gap. It does not expose the full flex axis control surface — it trades configurability for simplicity.

```go
// Vertical stack — the most common layout primitive in ERP forms
ast.NewStackLayout(ast.Vertical).
    WithGap(ast.GapMD).
    AddChild(sectionHeader).
    AddChild(fieldGroup).
    AddChild(actionBar)
```

Use stack layout for: form sections, detail panels, any sequential arrangement with consistent spacing.

### 10.2.5 Absolute/Overlay Layout

Overlay layout positions children absolutely within the container, enabling z-axis layering. Used sparingly — primarily for tooltip anchors, dropdown triggers, and the modal/drawer system's backdrop.

> ⚠️ **Warning:** Avoid overlay layout for primary content arrangement. Overlaid elements are invisible to linear screen reader navigation and require explicit accessibility handling. The modal and drawer components manage their own overlay positioning internally; you should not need to use `layout.overlay` for those use cases.

### 10.2.6 Responsive Layout

Responsive layout is a meta-layout: it wraps a set of layout specifications, one per breakpoint, and instructs the rendering engine to use the specification that matches the current screen size. It is the mechanism by which the backend specifies different layouts for different screen categories without conditionality in the AST.

```json
{
  "type": "layout.responsive",
  "breakpoints": {
    "mobile": { "type": "layout.stack", "props": { "direction": "vertical", "gap": "md" } },
    "tablet": { "type": "layout.grid", "props": { "columns": 2, "gap": "md" } },
    "desktop": { "type": "layout.grid", "props": { "columns": 3, "gap": "lg" } }
  },
  "children": [ "..." ]
}
```

The children are shared across all breakpoint specifications. Only the layout container changes; the content is the same.

---

## 10.3 Grid System

The grid system is the most expressive and most commonly used layout primitive for ERP interfaces. It models a multi-column, multi-row grid where children are placed in defined cells.

### 10.3.1 Column Definitions

Columns are defined by count (integer) for equal-width columns, or by explicit column template for mixed-width columns.

**Equal columns:**
```json
{ "type": "layout.grid", "props": { "columns": 3 } }
```
Produces three equal-width columns, each occupying 33.33% of the container width.

**Explicit column template:**
```json
{
  "type": "layout.grid",
  "props": {
    "column_template": ["2fr", "1fr", "1fr"]
  }
}
```
Produces three columns where the first is twice the width of the other two. The `fr` unit is a fractional unit — `2fr` receives twice the available space of `1fr`.

**Fixed + flexible columns:**
```json
{
  "type": "layout.grid",
  "props": {
    "column_template": ["240px", "1fr"]
  }
}
```
Produces a fixed 240px left column (for navigation or labels) and a flexible right column (for content).

### 10.3.2 Row Definitions

By default, rows are sized to fit their content (`auto` height). Fixed or minimum row heights can be specified for dashboard grids where visual consistency between widget cards matters.

```json
{
  "type": "layout.grid",
  "props": {
    "columns": 3,
    "row_height": "200px"
  }
}
```

### 10.3.3 Span and Offset

Each child of a grid layout can specify how many columns and rows it spans, and its starting column position.

```go
ast.NewGridLayout(3).
    AddChild(
        ast.NewKPICard("total-revenue").
            WithGridPlacement(ast.GridPlacement{
                ColStart: 1, ColSpan: 2,  // occupies columns 1 and 2
                RowStart: 1, RowSpan: 1,
            }),
    ).
    AddChild(
        ast.NewKPICard("open-orders").
            WithGridPlacement(ast.GridPlacement{
                ColStart: 3, ColSpan: 1,  // occupies column 3
                RowStart: 1, RowSpan: 1,
            }),
    )
```

**JSON representation:**
```json
{
  "type": "layout.grid",
  "props": { "columns": 3, "gap": "md" },
  "children": [
    {
      "type": "widget.kpi_card",
      "props": { "..." },
      "grid_placement": { "col_start": 1, "col_span": 2, "row_start": 1, "row_span": 1 }
    },
    {
      "type": "widget.kpi_card",
      "props": { "..." },
      "grid_placement": { "col_start": 3, "col_span": 1, "row_start": 1, "row_span": 1 }
    }
  ]
}
```

Children without a `grid_placement` are placed using auto-placement: the grid engine fills available cells left-to-right, top-to-bottom.

### 10.3.4 Gap and Gutter

Grid gap (the space between cells) is specified as a spacing token:

```json
{ "gap": "md" }              // equal gap in both row and column directions
{ "row_gap": "sm", "col_gap": "lg" }  // different gaps per axis
```

Gutter (the space between the grid and its container edges) uses the standard `padding` prop on the grid node.

### 10.3.5 Nested Grids

Grids can be nested to any depth. A grid cell's content may itself be a grid. This enables complex layouts like a dashboard where each widget card has its own internal grid for its header, metrics, and chart areas.

```
Outer grid (3 columns):
┌──────────┬──────────┬──────────┐
│ Widget A │ Widget B │ Widget C │
│          │          │          │
│  Inner   │          │          │
│  grid    │          │          │
│  (2×2)   │          │          │
└──────────┴──────────┴──────────┘
```

There is no artificial limit on nesting depth, but deeply nested grids (more than 4 levels) are a signal that the layout structure should be simplified or extracted into a component template.

---

## 10.4 Responsive Breakpoints

### 10.4.1 Breakpoint Definitions

AwoERP defines five named breakpoints. These names are used throughout the DSL wherever breakpoint-specific values are needed.

| Name | Alias | Minimum Width | Typical Device |
|------|-------|---------------|----------------|
| `xs` | `mobile` | 0px | Phones (portrait) |
| `sm` | `mobile_landscape` | 480px | Phones (landscape), small tablets |
| `md` | `tablet` | 768px | Tablets (portrait), large phones |
| `lg` | `desktop` | 1024px | Tablets (landscape), laptops |
| `xl` | `wide` | 1440px | Desktops, wide monitors |

Breakpoints are **inclusive upward**: a spec declared for `md` applies to all screen widths ≥ 768px unless overridden by a larger breakpoint spec. The rendering engine selects the largest breakpoint spec that is ≤ the current screen width.

### 10.4.2 Per-Breakpoint Visibility

Any node in the AST can be hidden at specific breakpoints using the `$breakpoint_show` directive:

```json
{
  "type": "ui.text",
  "props": { "content": "Detailed description visible on desktop only" },
  "$breakpoint_show": { "lg": true, "xl": true, "xs": false, "sm": false, "md": false }
}
```

A shorthand form is also available:

```json
{ "$breakpoint_show": "lg+" }
// Equivalent to: show at lg and xl, hide at xs, sm, md
```

Supported shorthand values: `xs+`, `sm+`, `md+`, `lg+`, `xl`, `xs-only`, `sm-only`, `md-only`, `lg-only`, `xl-only`.

### 10.4.3 Per-Breakpoint Column Spans

Grid children can specify different column spans at different breakpoints:

```json
{
  "type": "widget.kpi_card",
  "grid_placement": {
    "col_span": { "xs": 12, "sm": 6, "md": 4, "lg": 3, "xl": 2 }
  }
}
```

This is the most common form of responsive grid adaptation in AwoERP: a 12-column grid where items span all 12 columns on mobile (full width), 6 on small screens (2 per row), 4 on tablets (3 per row), and so on.

### 10.4.4 Breakpoint Overrides from Backend

The backend can explicitly provide breakpoint specifications rather than relying on defaults. This is necessary when the backend knows that a specific surface has non-standard responsive requirements — for example, a form that is always single-column regardless of screen size (because it is embedded in a narrow panel).

```go
ast.NewGridLayout(12).
    WithResponsiveRules(ast.ResponsiveRules{
        XS: ast.GridConfig{Columns: 1},
        SM: ast.GridConfig{Columns: 1},  // force single column even on larger mobile
        MD: ast.GridConfig{Columns: 2},
        LG: ast.GridConfig{Columns: 3},
        XL: ast.GridConfig{Columns: 4},
    })
```

---

## 10.5 Layout Containers

Layout containers are the named, semantically meaningful containers that structure a full page or surface. They differ from the layout primitives (grid, stack, flex) in that they carry semantic meaning — a `layout.page` is the root of a surface; a `layout.split` communicates a master-detail relationship.

### 10.5.1 Page Container

**`layout.page`** — The root container of every surface. Every compiled surface has exactly one `layout.page` as its root node.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `title` | LocalizedString | ✅ | — | Surface title (used in browser tab, navigation header, accessibility) |
| `subtitle` | LocalizedString | | — | Optional subtitle shown below the title |
| `layout_mode` | `standard`\|`full_bleed`\|`centered`\|`narrow` | | `standard` | Controls the page's maximum content width and padding |
| `back_navigation` | boolean | | `true` | Whether a back button is shown in the navigation header |
| `back_target` | SurfaceID | | — | Override the default back navigation target |
| `scroll_behavior` | `standard`\|`sticky_header`\|`collapsing_header` | | `standard` | |
| `pull_to_refresh` | boolean | | `false` | Mobile pull-to-refresh gesture (triggers refresh of all page data sources) |

**Slots:** `header_actions` (optional — buttons in the top-right navigation area), `body` (required), `fab` (optional — Floating Action Button, mobile only)

### 10.5.2 Section Container

**`layout.section`** — A semantically distinct region within a page. Sections add visual separation (border, background, or heading) without the full elevation of a card.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `title` | LocalizedString | | — | Section heading |
| `subtitle` | LocalizedString | | — | |
| `collapsible` | boolean | | `false` | Whether the section can be collapsed |
| `default_collapsed` | boolean | | `false` | Initial collapsed state |
| `padding` | `none`\|`sm`\|`md`\|`lg` | | `md` | |
| `divider` | boolean | | `true` | Shows a top border |

### 10.5.3 Panel Container

**`layout.panel`** — An elevated, bordered container. Equivalent to `ui.card` but used as a layout primitive (structured with an internal layout) rather than a display component.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `elevation` | `flat`\|`raised`\|`floating` | | `raised` | |
| `padding` | `none`\|`sm`\|`md`\|`lg` | | `md` | |
| `fill_height` | boolean | | `false` | Whether the panel expands to fill its grid cell height |

### 10.5.4 Split Pane

**`layout.split`** — A two-pane layout communicating a master-detail or primary-secondary relationship.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `direction` | `horizontal`\|`vertical` | | `horizontal` | Axis of the split |
| `ratio` | string (`"1:3"`, `"1:2"`, `"2:3"`, `"1:1"`) | | `"1:3"` | Width ratio of primary to secondary pane |
| `resizable` | boolean | | `false` | Whether the user can drag the split divider |
| `min_primary_width` | string (`"px"` or `"%"`) | | `"240px"` | Minimum width of the primary pane |
| `collapse_on` | breakpoint name | | `"sm"` | Breakpoint at which the split collapses to stacked layout |
| `collapsed_primary_label` | LocalizedString | | — | Label for the collapsed primary pane toggle button |

**Slots:** `primary` (required), `secondary` (required)

### 10.5.5 Scrollable Container

**`layout.scroll`** — A container with an explicit scroll region. The content overflows within this container rather than causing the page to scroll.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `direction` | `vertical`\|`horizontal`\|`both` | | `vertical` | |
| `max_height` | string (`"px"` or `"vh"`) | | — | If set, limits the container height and enables vertical scroll |
| `max_width` | string | | — | |
| `show_scrollbar` | `auto`\|`always`\|`never` | | `auto` | |
| `snap_to_children` | boolean | | `false` | Scroll snapping (for carousel-style layouts) |

### 10.5.6 Sticky Header / Footer

**`layout.sticky`** — A container whose header or footer remains visible as the user scrolls the body content.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `sticky_position` | `top`\|`bottom` | | `top` | |
| `z_index_level` | `default`\|`elevated` | | `default` | Stacking context |

**Slots:** `sticky` (required — the element that remains fixed), `body` (required — the scrollable content)

---

## 10.6 Spacing System

### 10.6.1 Margin and Padding Tokens

All spacing in AwoERP uses named tokens from the spacing scale rather than pixel values. This ensures consistent visual rhythm and allows the design system to adjust spacing globally.

| Token | Value (Default) | Use |
|-------|----------------|-----|
| `token:spacing.xs` | 4px | Icon-to-label gap, tight list item spacing |
| `token:spacing.sm` | 8px | Compact field spacing, badge padding |
| `token:spacing.md` | 16px | Standard field spacing, card padding |
| `token:spacing.lg` | 24px | Section spacing, dialog padding |
| `token:spacing.xl` | 32px | Page-level section separation |
| `token:spacing.2xl` | 48px | Hero sections, large whitespace |

Components accept spacing tokens in `padding`, `margin`, `gap`, `row_gap`, and `col_gap` props. Raw pixel values are rejected by schema validation.

```json
// ✅ Correct
{ "padding": "md", "gap": "sm" }

// 🚫 Rejected by schema validation
{ "padding": "16px", "gap": "8" }
```

### 10.6.2 Density Modes

AwoERP supports three density modes that globally adjust the spacing scale for the current surface:

| Mode | Scale Factor | Use Case |
|------|-------------|----------|
| `compact` | ×0.75 | Data-dense views: audit tables, ledger grids, large data tables |
| `normal` | ×1.0 | Standard ERP forms and dashboards |
| `comfortable` | ×1.25 | Onboarding flows, approval forms, mobile-first surfaces |

Density is set at the `layout.page` level and applies to all descendants:

```go
ast.NewPage("purchase-order-detail").
    WithDensity(ast.DensityCompact)
```

Rendering engines translate density mode into adjusted spacing values by scaling the base token values. The token names remain the same; only their realized values change.

---

## 10.7 Layout for Mobile

Mobile layout has specific considerations that differ from web. These are honored by mobile rendering engines without requiring the backend to specify mobile-specific instructions, except where the backend has an explicit preference.

### 10.7.1 Single Column Default

On `xs` breakpoint, all grid layouts default to single-column unless a breakpoint override is explicitly specified. This means a backend engineer authoring a 3-column grid does not need to add a mobile override — the mobile rendering engine handles the collapse automatically.

The auto-collapse behavior: each grid child occupies its full column width on mobile, stacked vertically in the order they appear in the `children` array. Grid children with `grid_placement` props have their placements ignored on mobile (they are all treated as `col_span: 1`).

### 10.7.2 Safe Area Insets

Mobile rendering engines automatically apply safe area insets — the system-defined padding that prevents content from being obscured by the device's notch, home indicator, status bar, and navigation bar. AwoERP layout containers do not need to specify safe area padding; it is applied by the rendering engine.

The `layout.page` node's `layout_mode: "full_bleed"` disables automatic safe area padding, allowing content (typically a hero image or a colored header) to extend to the screen edges. Backgrounds extend to edges; interactive content within a full-bleed page still respects safe areas.

### 10.7.3 Bottom Sheet Layouts

The `layout.split` container, when collapsed at the `sm` breakpoint, renders on mobile as a bottom sheet pattern: the secondary pane appears as a bottom sheet that can be pulled up to full height. This provides the mobile-native alternative to a side-by-side split layout.

```
Desktop:                     Mobile:
┌────────┬───────────────┐   ┌───────────────────┐
│ List   │ Detail Panel  │   │ List              │
│        │               │   │                   │
│        │               │   │ [Detail Panel     │
│        │               │   │  appears as       │
└────────┴───────────────┘   │  bottom sheet]    │
                             └───────────────────┘
```

The backend does not need to specify this — it is the standard mobile realization of `layout.split`. If the backend requires a different mobile behavior (e.g., full navigation to a separate screen rather than a bottom sheet), it specifies `mobile_split_mode: "navigate"` in the split layout's props.

---

## 10.8 Layout for Web

### 10.8.1 Sidebar + Content Shell

The standard AwoERP web shell — sidebar navigation on the left, main content area on the right — is rendered by the web application shell, not by individual surface layouts. Surface-level layouts only describe the content area. The shell is controlled by the navigation definition (Chapter 19).

Within the content area, surfaces use the standard layout primitives. The content area has a maximum width (`max_content_width` in the app shell configuration) that prevents content from becoming uncomfortably wide on large monitors.

### 10.8.2 Master-Detail Layout

The `layout.split` with `direction: "horizontal"` and `ratio: "1:3"` is the standard web master-detail pattern. The left pane contains a list or navigation tree; the right pane contains the detail view for the selected item.

```
┌───────────────────────────────────────────────┐
│ Navigation Shell (sidebar)                    │
│  ┌──────────────────────────────────────────┐ │
│  │ Content Area (surface layout)            │ │
│  │                                          │ │
│  │ ┌─────────────┐  ┌─────────────────────┐ │ │
│  │ │ PO List     │  │ PO Detail           │ │ │
│  │ │ (primary)   │  │ (secondary)         │ │ │
│  │ │             │  │                     │ │ │
│  │ └─────────────┘  └─────────────────────┘ │ │
│  └──────────────────────────────────────────┘ │
└───────────────────────────────────────────────┘
```

### 10.8.3 Embedded / Portlet Layouts

AwoERP surfaces can be embedded in external web portals or third-party dashboards as portlets — iframed SDUI surfaces rendered without the full application shell. Portlet layouts use `layout_mode: "full_bleed"` on the `layout.page` node to disable the standard page margins, and `back_navigation: false` to suppress the navigation header.

---

## 10.9 Layout Serialization in AST

Layout nodes follow the standard node schema (Chapter 07, Section 7.3). The following JSON examples illustrate the canonical serialization for the most commonly used layout patterns.

### Standard Form Layout (Two-Column)

```json
{
  "id": "po-create-page",
  "type": "layout.page",
  "props": {
    "title": "Create Purchase Order",
    "layout_mode": "standard",
    "density": "normal"
  },
  "children": [
    {
      "id": "po-create-form",
      "type": "form.container",
      "props": { "submit_action": "action-submit" },
      "children": [
        {
          "id": "header-section",
          "type": "form.section",
          "props": { "title": "Order Details", "columns": 2 },
          "children": [
            {
              "id": "vendor-field",
              "type": "field.vendor_selector",
              "props": { "label": "Vendor", "required": true },
              "grid_placement": { "col_span": 2 }
            },
            {
              "id": "order-date-field",
              "type": "field.date",
              "props": { "label": "Order Date", "required": true }
            },
            {
              "id": "required-date-field",
              "type": "field.date",
              "props": { "label": "Required By", "required": true }
            }
          ]
        }
      ]
    }
  ]
}
```

### Dashboard Grid Layout

```json
{
  "id": "procurement-dashboard",
  "type": "layout.page",
  "props": { "title": "Procurement Dashboard", "layout_mode": "standard" },
  "children": [
    {
      "id": "kpi-row",
      "type": "layout.grid",
      "props": { "columns": 4, "gap": "md" },
      "children": [
        { "id": "w-open-pos", "type": "widget.kpi_card", "props": { "..." } },
        { "id": "w-pending-approval", "type": "widget.kpi_card", "props": { "..." } },
        { "id": "w-overdue", "type": "widget.kpi_card", "props": { "..." } },
        { "id": "w-spend-mtd", "type": "widget.kpi_card", "props": { "..." } }
      ]
    },
    {
      "id": "main-content",
      "type": "layout.grid",
      "props": { "column_template": ["2fr", "1fr"], "gap": "md" },
      "children": [
        {
          "id": "spend-chart",
          "type": "widget.chart.bar",
          "props": { "..." },
          "grid_placement": { "col_span": 1, "row_span": 2 }
        },
        { "id": "pending-approvals", "type": "widget.data_table", "props": { "..." } },
        { "id": "recent-pos", "type": "widget.activity_feed", "props": { "..." } }
      ]
    }
  ]
}
```

### Responsive Layout Switching

```json
{
  "id": "adaptive-section",
  "type": "layout.responsive",
  "breakpoints": {
    "xs": {
      "type": "layout.stack",
      "props": { "direction": "vertical", "gap": "md" }
    },
    "md": {
      "type": "layout.grid",
      "props": { "columns": 2, "gap": "md" }
    },
    "lg": {
      "type": "layout.grid",
      "props": { "columns": 3, "gap": "lg" }
    }
  },
  "children": [
    { "id": "card-a", "type": "ui.card", "props": { "..." } },
    { "id": "card-b", "type": "ui.card", "props": { "..." } },
    { "id": "card-c", "type": "ui.card", "props": { "..." } }
  ]
}
```

---

*End of Chapter 10*

**Previous:** [Chapter 09 — Component System](./09-component-system.md)
**Next:** [Chapter 11 — Forms Framework](./11-forms-framework.md)
