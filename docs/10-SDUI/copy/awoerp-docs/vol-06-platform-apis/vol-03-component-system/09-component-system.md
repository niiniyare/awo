# Chapter 09 — Component System

> **Volume:** III — Component System
> **Phase:** 2
> **Audience:** Platform Engineers, Web/Mobile Engineers, UI Framework Developers, Backend Engineers
> **Prerequisites:** Chapters 01–08

---

## Table of Contents

- [9.1 Component Philosophy](#91-component-philosophy)
- [9.2 Component Identity and Registry](#92-component-identity-and-registry)
- [9.3 Component Category Taxonomy](#93-component-category-taxonomy)
- [9.4 Component Contract](#94-component-contract)
- [9.5 Component Versioning](#95-component-versioning)
- [9.6 Primitive Component Library](#96-primitive-component-library)
- [9.7 Compound Component Library](#97-compound-component-library)
- [9.8 Theming and Style Props](#98-theming-and-style-props)
- [9.9 Conditional Rendering on Components](#99-conditional-rendering-on-components)
- [9.10 Component Rendering Fallbacks](#910-component-rendering-fallbacks)

---

## 9.1 Component Philosophy

A component is the atomic unit of the AwoERP UI platform. Everything visible on screen is a component or a composition of components. The component system is the vocabulary of the platform — the set of named, typed, reusable building blocks from which all interfaces are constructed.

Three philosophical commitments govern the component system:

**Components are contracts, not implementations.** A component is defined by its schema: what props it accepts, what slots it exposes, what events it emits, what actions it supports. The schema is the contract between the backend (which authors component usage) and the rendering engines (which implement component rendering). The Go AST builder API is generated from the schema. The rendering engine's component registry validates against the schema. The schema is the ground truth — not the web implementation, not the iOS implementation.

**Components speak the language of ERP, not the language of HTML.** The platform's component library is organized around business concepts — `VendorSelector`, `CurrencyAmount`, `ApprovalStatusBadge`, `LineItemGrid` — not around HTML primitives. This vocabulary is shared between backend engineers authoring UI definitions and product stakeholders reviewing them. A product manager can read an AST and recognize the business concepts; they cannot be expected to read flexbox layout code and infer business intent.

**Components are platform-agnostic at the definition level.** A `DateField` component in the AST does not specify whether it renders as a native iOS `DatePicker`, an Android `DatePickerDialog`, or a browser `<input type="date">`. The component type carries semantic intent; the rendering engine maps that intent to the most appropriate native control for its platform. This separation enables each platform to provide the best possible user experience without requiring backend changes.

---

## 9.2 Component Identity and Registry

### 9.2.1 Component Type Identifiers (Namespaced)

Every component has a globally unique type identifier, expressed as a dot-separated namespace path. The namespace structure is:

```
{category}.{subcategory?}.{name}

Examples:
  layout.grid
  layout.tabs
  form.container
  form.section
  field.text
  field.currency_amount
  field.vendor_selector
  action.button
  action.http_request
  widget.kpi_card
  widget.chart.bar
  erp.procurement.po_header
  plugin.acme_corp.custom_scorecard
```

**Namespace conventions:**

| Prefix | Category | Authorship |
|--------|----------|------------|
| `layout.*` | Layout containers | Platform |
| `form.*` | Form structure nodes | Platform |
| `field.*` | Form input fields | Platform |
| `ui.*` | Generic display components | Platform |
| `action.*` | Action nodes | Platform |
| `data.*` | Data source nodes | Platform |
| `flow.*` | Control flow nodes | Platform |
| `widget.*` | Dashboard widgets | Platform |
| `chart.*` | Chart components | Platform |
| `nav.*` | Navigation components | Platform |
| `erp.*` | ERP domain components | Platform (domain teams) |
| `workflow.*` | Workflow/approval components | Platform (workflow team) |
| `plugin.*` | Third-party custom components | External / Registered plugins |

The `plugin.*` namespace is reserved for external contributors. All `plugin.*` components must be registered through the governance process before they can be used in compiled definitions.

### 9.2.2 The Component Registry Service

The Component Registry is the authoritative catalog of all known component types. It is a platform service with a PostgreSQL-backed store and a Redis cache layer.

```go
type ComponentRegistry interface {
    // Get the schema for a component type
    GetSchema(ctx context.Context, typeID ComponentTypeID) (*ComponentSchema, error)

    // Get the schema for a specific version of a component type
    GetSchemaVersion(ctx context.Context, typeID ComponentTypeID, version semver.Version) (*ComponentSchema, error)

    // List all registered component types
    ListComponents(ctx context.Context, filter ComponentFilter) ([]*ComponentSummary, error)

    // Register a new component type or a new version of an existing type
    Register(ctx context.Context, schema *ComponentSchema) error

    // Deprecate a component type version
    Deprecate(ctx context.Context, typeID ComponentTypeID, version semver.Version, reason string) error

    // Check if a component type is known
    Exists(ctx context.Context, typeID ComponentTypeID) (bool, error)
}
```

### 9.2.3 Registry Lookup and Resolution

During AST validation (Stage 4 of the compilation pipeline), every node's type is resolved against the Component Registry. Resolution follows this order:

1. **Exact version match:** If the node specifies `type_version`, match that exact version
2. **Latest compatible version:** If no version is specified, use the latest non-deprecated version of the type that is compatible with the client's declared `schema_version`
3. **Unknown type:** If the type is not found in the registry, emit a validation error and replace the node with its declared `$fallback` node (if any), or a generic unknown-component placeholder

Registry lookups during compilation are served from the in-process LRU cache (populated from Redis) to avoid network round-trips per node. The cache is refreshed when a new component version is registered.

### 9.2.4 Custom Component Registration

Third-party components are registered by platform engineers after the governance review process (Chapter 45). Registration requires:

1. A complete `ComponentSchema` JSON document
2. A rendering engine implementation for each supported platform (documented in Chapter 50 — SDK Development)
3. A Go builder implementation (or generated builder from the schema)
4. Approval from the platform governance committee
5. Version tagging (semver) and changelog entry

```go
// Component schema document structure
type ComponentSchema struct {
    TypeID          ComponentTypeID `json:"type_id"`
    Version         semver.Version  `json:"version"`
    DisplayName     string          `json:"display_name"`
    Description     string          `json:"description"`
    Category        ComponentCategory `json:"category"`
    MinSchemaVersion semver.Version `json:"min_schema_version"`
    Props           JSONSchema      `json:"props"`
    Slots           []SlotDef       `json:"slots"`
    Events          []EventDef      `json:"events"`
    SupportedActions []ActionTypeID `json:"supported_actions"`
    Capabilities    []Capability    `json:"required_capabilities"`
    Fallback        *ComponentSchema `json:"fallback,omitempty"`
    Deprecated      bool            `json:"deprecated"`
    DeprecatedReason string         `json:"deprecated_reason,omitempty"`
    ReplacedBy      *ComponentTypeID `json:"replaced_by,omitempty"`
}
```

---

## 9.3 Component Category Taxonomy

### 9.3.1 Primitive / Atomic Components

Primitive components are the smallest renderable units. They have no children and no slots. They render exactly one visible element.

Full list: `ui.text`, `ui.heading`, `ui.label`, `ui.icon`, `ui.badge`, `ui.tag`, `ui.chip`, `ui.avatar`, `ui.image`, `ui.divider`, `ui.spacer`, `ui.progress_bar`, `ui.progress_circle`, `ui.spinner`, `ui.skeleton`, `ui.qr_code`, `ui.barcode`.

### 9.3.2 Compound Components

Compound components contain structured internal layouts and optional slots. They render multiple elements as a cohesive unit.

Full list: `ui.card`, `ui.list_item`, `ui.alert`, `ui.banner`, `ui.empty_state`, `ui.modal`, `ui.drawer`, `ui.bottom_sheet`, `ui.tooltip`, `ui.popover`, `ui.stepper`, `ui.accordion`, `ui.tab_container`, `ui.breadcrumb`, `ui.timeline`, `ui.comment_thread`.

### 9.3.3 Layout Components

Layout components arrange their children spatially. They have no semantic meaning beyond arrangement.

Full list: `layout.page`, `layout.grid`, `layout.stack`, `layout.split`, `layout.scroll`, `layout.tabs`, `layout.accordion`, `layout.masonry`, `layout.overlay`.

### 9.3.4 Data Components

Data components declare data sources but render no visible content.

Full list: `data.rest_source`, `data.grpc_source`, `data.static_source`, `data.computed_source`, `data.realtime_source`, `data.file_source`.

### 9.3.5 Form Components

Form components collect user input. They are the most complex category — each has a distinct input model, validation contract, and accessibility requirements.

Full list: See Chapter 11 — Forms Framework.

### 9.3.6 Navigation Components

Navigation components manage the user's movement between surfaces.

Full list: `nav.tab_bar`, `nav.sidebar`, `nav.top_bar`, `nav.breadcrumb`, `nav.back_button`, `nav.drawer`, `nav.bottom_sheet_handle`.

### 9.3.7 Feedback & Status Components

Feedback components communicate system state, operation outcomes, and asynchronous activity.

Full list: `ui.toast`, `ui.notification_banner`, `ui.error_state`, `ui.loading_state`, `ui.empty_state`, `ui.confirmation_dialog`, `ui.progress_overlay`.

### 9.3.8 ERP Domain Components

ERP domain components encapsulate complex, domain-specific UI patterns. See Chapter 16.

### 9.3.9 Chart / Analytics Components

Chart components render data visualizations. See Chapter 14.

### 9.3.10 Workflow Components

Workflow components surface the state and interaction model of long-running processes. See Chapter 15.

---

## 9.4 Component Contract

The component contract is the formal agreement between the backend (which uses the component) and the rendering engine (which implements the component). The contract is expressed in the `ComponentSchema` and consists of five dimensions.

### 9.4.1 Props Schema (Required vs. Optional)

The props schema is a JSON Schema document that defines all properties the component accepts. Each prop has:
- A name (camelCase in JSON Schema, snake_case in the JSON wire format)
- A type (string, integer, number, boolean, array, object, or a reference to a shared type)
- Required or optional status
- A default value (for optional props)
- A description (used in generated documentation and IDE tooltips)
- Constraints (minimum, maximum, enum values, pattern)

```json
{
  "type": "object",
  "properties": {
    "label": {
      "type": "string",
      "description": "Field label displayed above the input",
      "examples": ["Vendor Name", "Purchase Order Number"]
    },
    "required": {
      "type": "boolean",
      "default": false,
      "description": "Whether the field must be filled before form submission"
    },
    "max_length": {
      "type": "integer",
      "minimum": 1,
      "maximum": 10000,
      "description": "Maximum number of characters allowed"
    },
    "placeholder": {
      "type": "string",
      "description": "Hint text displayed when the field is empty"
    }
  },
  "required": ["label"]
}
```

### 9.4.2 Slot Definitions

Slot definitions specify the named insertion points the component exposes:

```go
type SlotDef struct {
    Name         string          // slot identifier (e.g., "header", "footer", "actions")
    Required     bool            // must the slot be populated?
    AcceptedTypes []ComponentTypeID // which component types may be placed in this slot
    AcceptMultiple bool          // does the slot accept multiple nodes, or exactly one?
    Description  string          // developer documentation
}
```

### 9.4.3 Emitted Events

Components emit events when user interactions or state changes occur. Each emitted event is defined in the contract:

```go
type EventDef struct {
    Name        string      // event identifier (e.g., "on_click", "on_change", "on_row_select")
    PayloadType JSONSchema   // schema of the event payload passed to action handlers
    Description string
}
```

Event payload schemas are important for type-safe action handler design. A table's `on_row_select` event might carry `{ "selected_ids": ["string"] }` as its payload, which action handlers can reference in their parameter bindings.

### 9.4.4 Supported Actions

Some components have built-in action support beyond their event bindings — for example, a table component supports an `action.table_scroll_to_row` action that can be dispatched programmatically from another component's event handler. The contract declares which action types the component accepts.

### 9.4.5 Permission Surface

The permission surface documents which props, slots, and events can be individually protected by permission directives. Not all props support per-prop permissions — some are structural and cannot be partially applied.

### 9.4.6 Rendering Constraints

Rendering constraints document platform-specific limitations:
- **Minimum supported schema version** for this component
- **Required client capabilities** (e.g., `charts` capability required for all chart components)
- **Platform exclusions** (a component available only on mobile, or only on web)
- **Context restrictions** (a component valid only as a direct child of `form.section`, not in arbitrary positions)

---

## 9.5 Component Versioning

### 9.5.1 Component Version Lifecycle

```
Draft → Active → Deprecated → Retired
```

- **Draft:** The component schema is registered but not yet usable in production definitions. Used during development and review.
- **Active:** The component is available for use in compiled definitions. This is the normal operating state.
- **Deprecated:** The component is still rendered by conformant rendering engines, but new usage is discouraged. The compiler emits warnings for definitions using deprecated components.
- **Retired:** The component is no longer rendered by current rendering engines. Definitions using retired components fail schema validation.

### 9.5.2 Breaking vs. Non-Breaking Component Changes

**Non-breaking (minor version bump):**
- Adding a new optional prop with a default value
- Adding a new slot (optional)
- Adding a new event
- Adding a new supported action type
- Expanding an enum value set

**Breaking (major version bump, with migration path):**
- Removing a prop
- Renaming a prop
- Changing a prop's type
- Removing a slot
- Changing a slot's accepted types
- Removing an event
- Changing an event's payload schema in a non-additive way

### 9.5.3 Deprecation Strategy

When a component must be replaced:

1. Register the new component type (e.g., `field.vendor_selector_v2`) as `Active`
2. Mark the old type (e.g., `field.vendor_selector`) as `Deprecated` with `replaced_by: "field.vendor_selector_v2"`
3. Publish a migration guide documenting prop mapping from old to new
4. Provide an automated migration script in the platform CLI that rewrites AST builder calls
5. After the support window (minimum 180 days), retire the old type

---

## 9.6 Primitive Component Library

The following tables document every primitive component — its type identifier, purpose, key props, and notes.

### 9.6.1 Text, Label, Heading

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|

**`ui.text`** — Inline or block text display.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `content` | LocalizedString | ✅ | — | The text content to display |
| `variant` | `body1`\|`body2`\|`caption`\|`overline`\|`code` | | `body1` | Typography variant |
| `color` | ColorToken | | `token:text.primary` | Text color |
| `align` | `left`\|`center`\|`right`\|`justify` | | `left` | Text alignment |
| `truncate` | integer | | — | Max lines before ellipsis |
| `selectable` | boolean | | `false` | Whether text is user-selectable |

**`ui.heading`** — Section or page heading.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `content` | LocalizedString | ✅ | — | Heading text |
| `level` | `1`\|`2`\|`3`\|`4`\|`5`\|`6` | | `2` | Semantic heading level (maps to h1–h6 on web, equivalent on mobile) |
| `color` | ColorToken | | `token:text.heading` | |
| `align` | `left`\|`center`\|`right` | | `left` | |

### 9.6.2 Button, IconButton

**`action.button`** — The primary interaction element.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `label` | LocalizedString | ✅ | — | Button text |
| `action_ref` | ActionRef | ✅ | — | The action to execute on press |
| `variant` | `primary`\|`secondary`\|`tertiary`\|`danger`\|`ghost` | | `secondary` | Visual style |
| `icon` | IconRef | | — | Optional leading icon |
| `icon_position` | `leading`\|`trailing` | | `leading` | |
| `disabled` | boolean\|Binding | | `false` | |
| `loading` | boolean\|Binding | | `false` | Shows loading spinner when true |
| `size` | `sm`\|`md`\|`lg` | | `md` | |
| `full_width` | boolean | | `false` | Stretches to container width |

**`action.icon_button`** — An icon-only interactive button.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `icon` | IconRef | ✅ | — | The icon to display |
| `label` | LocalizedString | ✅ | — | Accessibility label (not visible) |
| `action_ref` | ActionRef | ✅ | — | |
| `variant` | `default`\|`primary`\|`danger`\|`ghost` | | `default` | |
| `size` | `sm`\|`md`\|`lg` | | `md` | |
| `disabled` | boolean\|Binding | | `false` | |

### 9.6.3 Icon

**`ui.icon`** — Displays a named icon from the platform icon set.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `name` | IconRef | ✅ | — | Icon identifier (e.g., `icon:check-circle`) |
| `size` | `xs`\|`sm`\|`md`\|`lg`\|`xl` | | `md` | |
| `color` | ColorToken | | `token:icon.default` | |
| `label` | LocalizedString | | — | Accessibility label for screen readers |

### 9.6.4 Badge, Tag, Chip

**`ui.badge`** — A small status or count indicator.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `label` | LocalizedString | ✅ | — | Badge text |
| `variant` | `default`\|`success`\|`warning`\|`error`\|`info`\|`neutral` | | `default` | Semantic color variant |
| `size` | `sm`\|`md` | | `md` | |
| `dot` | boolean | | `false` | Show dot only (no label) |
| `max_count` | integer | | — | If label is a number, cap display at this value (shows "99+" etc.) |

**`ui.tag`** — A removable or static label applied to an entity.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `label` | LocalizedString | ✅ | — | |
| `color` | ColorToken | | — | Custom color override |
| `removable` | boolean | | `false` | Shows a remove (×) button |
| `remove_action` | ActionRef | | — | Required if `removable: true` |

### 9.6.5 Avatar

**`ui.avatar`** — Displays a user or entity representation.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `src` | string\|Binding | | — | Image URL; if absent, falls back to initials |
| `initials` | string\|Binding | | — | 1–2 character initials |
| `alt` | LocalizedString | | — | Accessibility description |
| `size` | `xs`\|`sm`\|`md`\|`lg`\|`xl` | | `md` | |
| `shape` | `circle`\|`square`\|`rounded` | | `circle` | |
| `status` | `online`\|`offline`\|`away`\|`busy` | | — | Optional status indicator dot |

### 9.6.6 Progress Bar

**`ui.progress_bar`** — A linear progress indicator.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `value` | number\|Binding | ✅ | — | Current value |
| `max` | number | | `100` | Maximum value |
| `variant` | `default`\|`success`\|`warning`\|`error` | | `default` | |
| `show_label` | boolean | | `false` | Displays percentage text |
| `label` | LocalizedString | | — | Optional descriptive label |
| `indeterminate` | boolean | | `false` | Animated indeterminate mode (for unknown progress) |

### 9.6.7 Spinner

**`ui.spinner`** — An animated loading indicator.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `size` | `sm`\|`md`\|`lg` | | `md` | |
| `label` | LocalizedString | | — | Accessibility label |
| `color` | ColorToken | | `token:brand.primary` | |

---

## 9.7 Compound Component Library

### 9.7.1 Card

**`ui.card`** — A contained, elevated surface for grouping related content.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `elevation` | `flat`\|`raised`\|`floating` | | `raised` | Visual depth |
| `padding` | `none`\|`sm`\|`md`\|`lg` | | `md` | Internal padding |
| `clickable` | boolean | | `false` | Adds hover/press affordance |
| `click_action` | ActionRef | | — | Required if `clickable: true` |

**Slots:** `header` (optional), `body` (required), `footer` (optional), `actions` (optional)

### 9.7.2 Alert, Banner

**`ui.alert`** — An inline message communicating status.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `title` | LocalizedString | | — | |
| `message` | LocalizedString | ✅ | — | |
| `variant` | `info`\|`success`\|`warning`\|`error` | ✅ | — | |
| `icon` | IconRef | | *(variant default)* | |
| `dismissible` | boolean | | `false` | |
| `dismiss_action` | ActionRef | | — | Required if `dismissible: true` |

**`ui.banner`** — A full-width persistent notification, typically at the top of a surface.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `message` | LocalizedString | ✅ | — | |
| `variant` | `info`\|`success`\|`warning`\|`error` | ✅ | — | |
| `action_label` | LocalizedString | | — | Optional CTA button label |
| `action_ref` | ActionRef | | — | |
| `dismissible` | boolean | | `true` | |

### 9.7.3 Modal / Dialog

**`ui.modal`** — A floating dialog that overlays the current surface.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `title` | LocalizedString | ✅ | — | |
| `size` | `sm`\|`md`\|`lg`\|`fullscreen` | | `md` | |
| `dismissible` | boolean | | `true` | Whether the backdrop dismisses the modal |
| `surface_ref` | SurfaceID | | — | Load a sub-surface as modal content |

**Slots:** `body` (required or `surface_ref`), `footer` (optional)

> 📘 **Note:** Modals in AwoERP can either embed an inline `body` slot subtree or reference a named sub-surface via `surface_ref`. The sub-surface is compiled independently and loaded lazily when the modal opens. Use `surface_ref` for complex modal content to keep the parent surface's payload budget manageable.

### 9.7.4 Drawer / Side Panel

**`ui.drawer`** — A sliding panel that appears from a screen edge.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `title` | LocalizedString | | — | |
| `position` | `left`\|`right`\|`bottom` | | `right` | Edge from which the drawer appears |
| `size` | `sm`\|`md`\|`lg`\|`fullscreen` | | `md` | Width (or height for bottom drawers) |
| `surface_ref` | SurfaceID | | — | Load sub-surface as drawer content |

**Slots:** `body` (required or `surface_ref`), `footer` (optional)

### 9.7.5 Stepper / Wizard

**`ui.stepper`** — A multi-step progress indicator for wizard-style flows.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `steps` | array of StepDef | ✅ | — | Ordered step definitions |
| `current_step` | integer\|Binding | ✅ | — | 0-indexed current step |
| `orientation` | `horizontal`\|`vertical` | | `horizontal` | |
| `allow_step_click` | boolean | | `false` | Whether past steps are directly navigable |

**StepDef:** `{ label: LocalizedString, description?: LocalizedString, status: "pending"|"active"|"complete"|"error" }`

### 9.7.6 Accordion / Collapsible

**`ui.accordion`** — A vertically stacked set of collapsible sections.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `items` | array of AccordionItem | ✅ | — | |
| `allow_multiple` | boolean | | `false` | Whether multiple sections can be open simultaneously |
| `default_open` | array of integer | | `[]` | Indices of initially open sections |

**AccordionItem:** `{ id: string, title: LocalizedString, body: Node }`

### 9.7.7 Tab Container

**`ui.tab_container`** — A set of horizontally navigable tabs.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `tabs` | array of TabDef | ✅ | — | |
| `active_tab` | string\|Binding | | *(first tab)* | ID of the active tab |
| `tab_position` | `top`\|`bottom`\|`left` | | `top` | |
| `lazy_load` | boolean | | `true` | Whether inactive tab contents are rendered |
| `on_tab_change` | array of ActionRef | | — | Actions to execute when the active tab changes |

**TabDef:** `{ id: string, label: LocalizedString, icon?: IconRef, badge?: string, body: Node, disabled?: boolean }`

### 9.7.8 Breadcrumb

**`ui.breadcrumb`** — A hierarchical navigation trail.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `items` | array of BreadcrumbItem | ✅ | — | Ordered from root to current |
| `max_visible` | integer | | — | Collapses middle items when exceeded |

**BreadcrumbItem:** `{ label: LocalizedString, action_ref?: ActionRef }` — the last item in the array is the current page (no action).

---

## 9.8 Theming and Style Props

### 9.8.1 Design Token References

Visual style props in AwoERP components reference **design tokens** rather than raw CSS values or hex colors. A design token is a named, semantic reference to a visual attribute defined in the platform's design system.

```json
// ✅ Correct — semantic token reference
{ "color": "token:status.success" }
{ "color": "token:text.secondary" }
{ "color": "token:brand.primary" }

// 🚫 Incorrect — raw value
{ "color": "#22c55e" }
{ "color": "rgba(0, 0, 0, 0.6)" }
```

Token references are resolved by the rendering engine to platform-appropriate values. On the web, they map to CSS custom properties (`--color-status-success`). On iOS, they map to `UIColor` semantic colors. On Android, they map to Material Design color roles. This resolution ensures that dark mode, high-contrast mode, and tenant theme overrides work correctly across all platforms without changes to individual component definitions.

**Token categories:**

| Category | Examples | Purpose |
|----------|---------|---------|
| `token:brand.*` | `primary`, `secondary`, `accent` | Tenant-branded colors |
| `token:text.*` | `primary`, `secondary`, `disabled`, `inverse`, `heading`, `link` | Text colors |
| `token:icon.*` | `default`, `muted`, `active`, `danger` | Icon colors |
| `token:status.*` | `success`, `warning`, `error`, `info` | Semantic status colors |
| `token:surface.*` | `default`, `raised`, `overlay`, `sunken` | Background colors |
| `token:border.*` | `default`, `strong`, `focus`, `error` | Border colors |
| `token:spacing.*` | `xs`, `sm`, `md`, `lg`, `xl`, `2xl` | Spacing values |
| `token:radius.*` | `sm`, `md`, `lg`, `full` | Border radius values |
| `token:shadow.*` | `sm`, `md`, `lg` | Elevation shadows |

### 9.8.2 Tenant-Level Theming

Tenants can override brand tokens through their tenant configuration. Brand token overrides cascade to all components automatically — no component needs to be individually updated when a tenant's primary color changes.

```json
// Tenant configuration — brand token overrides
{
  "theme_tokens": {
    "token:brand.primary": "#0066CC",
    "token:brand.secondary": "#004C99",
    "token:brand.accent": "#FF6600"
  }
}
```

Non-brand tokens (text, status, surface, border) cannot be overridden at the tenant level to ensure accessibility standards are maintained.

### 9.8.3 Component Style Overrides

Components accept a `style_overrides` prop (type: `StyleOverrideMap`) that allows per-instance token overrides for specific visual needs. This is an escape hatch — it should not be the primary theming mechanism.

```json
{
  "type": "ui.badge",
  "props": {
    "label": "URGENT",
    "style_overrides": {
      "token:brand.primary": "token:status.error"
    }
  }
}
```

Style overrides are validated against the component schema's declared overrideable tokens. Only tokens explicitly listed as overrideable can be overridden; attempting to override a non-overrideable token is a validation warning (not an error — the override is silently ignored).

---

## 9.9 Conditional Rendering on Components

Any component can be made conditionally rendered using the `$if` and `$show` directives (Chapter 06, Section 6.6). This section documents the rendering behavior implications of these directives.

**`$if` (compilation-time exclusion):**
- Evaluated during the permission pruning or flag resolution pass
- If false, the node is entirely absent from the compiled JSON payload
- The rendering engine has no knowledge of the excluded node
- The surrounding layout reflows as if the node never existed
- Use for: permission-based exclusion, flag-gated features, context-dependent fields

**`$show` (runtime visibility):**
- Evaluated by the rendering engine at render time
- If false, the node is in the payload but not rendered
- The surrounding layout should not leave a gap — the rendering engine must not reserve space for hidden nodes
- The node's data sources are still loaded (the data may be needed for other bindings)
- Use for: user-toggleable sections, conditional content based on form state

> ⚠️ **Warning:** Do not use `$show` for permission-based visibility. Permission decisions must use `$if` to ensure the excluded content is not present in the payload at all. A component hidden with `$show: false` but present in the JSON payload is a security risk — its props (which may include sensitive data bound from a data source) are visible in the payload.

---

## 9.10 Component Rendering Fallbacks

Every component type can declare a `fallback` in its registry schema. The fallback is a simpler component subtree that rendering engines use when they encounter a component type they do not recognize.

**Platform-declared fallbacks** are registered by the platform team in the component registry:

```json
{
  "type_id": "widget.chart.bar",
  "fallback": {
    "type_id": "widget.data_table",
    "props_mapping": {
      "data_source": "data_source",
      "title": "title"
    }
  }
}
```

When an older rendering engine (one that does not support `widget.chart.bar`) encounters a bar chart node, it substitutes the `widget.data_table` fallback, mapping the chart's data source binding to the table's data source prop. The user sees a data table instead of a bar chart — a functional, if less visually rich, representation.

**Node-level fallbacks** are declared inline by the backend engineer authoring the AST:

```go
ast.NewBarChart("revenue-chart").
    WithDataSource("datasource:monthly_revenue").
    WithTitle(i18n.Key("dashboard.revenue_chart.title")).
    WithFallback(
        // Clients that can't render the bar chart get this instead
        ast.NewDataTable("revenue-table").
            WithDataSource("datasource:monthly_revenue").
            WithTitle(i18n.Key("dashboard.revenue_chart.title")),
    )
```

Node-level fallbacks override platform-declared fallbacks. They allow backend engineers to provide a more semantically appropriate fallback for a specific usage than the generic platform default.

**The unknown-component placeholder** is the last-resort fallback when no explicit fallback is declared and the component type is unrecognized. The rendering engine renders a neutral, non-interactive box with:
- An informational icon
- The text "This component is not available in your current version"
- The component's type ID and node ID (in development/debug mode only)
- A telemetry event recording the fallback occurrence

The telemetry event is the critical element: it provides the platform team with visibility into which rendering engine versions are encountering unknown components, enabling proactive release coordination.

---

*End of Chapter 09*

**Previous:** [Chapter 08 — JSON Compilation Pipeline](../vol-02-dsl-and-ast/08-json-compilation-pipeline.md)
**Next:** [Chapter 10 — Layout System](./10-layout-system.md)
