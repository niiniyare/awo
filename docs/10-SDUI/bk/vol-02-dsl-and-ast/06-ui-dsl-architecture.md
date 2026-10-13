# Chapter 06 — UI DSL Architecture

> **Volume:** II — DSL & AST
> **Phase:** 1 (Foundation)
> **Audience:** Platform Engineers, UI Framework Developers, Senior Backend Engineers
> **Prerequisites:** Chapters 01–05

---

## Table of Contents

- [6.1 What Is the AwoERP UI DSL?](#61-what-is-the-awoerp-ui-dsl)
- [6.2 DSL Design Goals](#62-dsl-design-goals)
- [6.3 DSL Grammar Overview](#63-dsl-grammar-overview)
- [6.4 Type System for the DSL](#64-type-system-for-the-dsl)
- [6.5 Binding Expressions](#65-binding-expressions)
- [6.6 Directives](#66-directives)
- [6.7 DSL Extensibility — Defining Custom Node Types](#67-dsl-extensibility--defining-custom-node-types)
- [6.8 DSL Validation Rules](#68-dsl-validation-rules)
- [6.9 DSL Versioning](#69-dsl-versioning)

---

## 6.1 What Is the AwoERP UI DSL?

A **Domain-Specific Language (DSL)** is a language designed for a specific problem domain. The AwoERP UI DSL is the formal language in which backend engineers express user interface intent. It is not a general-purpose programming language; it is a constrained vocabulary of concepts — components, layouts, bindings, actions, events, directives — that together express everything a UI can be or do within AwoERP.

The DSL exists at two levels simultaneously:

**The Go API Level (authoring time).** Backend engineers interact with the DSL through the Go AST builder library — a set of typed constructors, builder methods, and validation helpers that let engineers express UI definitions as Go code. At this level, the DSL benefits from Go's type system: incorrect usage fails at compile time, not at runtime.

```go
// DSL at the Go API level — typed, IDE-assisted, compile-time safe
ast.NewForm("po-create-form").
    AddSection(
        ast.NewFormSection("header").
            AddField(ast.NewTextField("notes").
                WithLabel(i18n.Key("po.field.notes")).
                WithMaxLength(500).
                WithRequired(false),
            ),
    )
```

**The JSON Level (transport time).** The compiled AST is serialized to JSON for transport. The JSON is the DSL expressed as data — the same structure and concepts, but as a portable document rather than a Go program. This is what the rendering engine receives.

```json
{
  "id": "po-create-form",
  "type": "form.container",
  "sections": [
    {
      "id": "header",
      "type": "form.section",
      "fields": [
        {
          "id": "notes",
          "type": "field.text",
          "props": {
            "label": "Notes",
            "max_length": 500,
            "required": false
          }
        }
      ]
    }
  ]
}
```

The two levels are isomorphic: every valid Go AST produces a corresponding valid JSON structure, and every valid JSON structure has a corresponding Go AST. This isomorphism is enforced by the schema system — the same component schemas that validate the JSON also constrain the Go builder API.

---

## 6.2 DSL Design Goals

The DSL was designed against the following explicit goals. Trade-offs that had to be made between conflicting goals are documented in Section 6.2.x.

**Goal 1: Expressiveness for ERP.** The DSL must be capable of expressing every UI pattern required by an enterprise ERP system: complex multi-step forms with conditional field visibility, multi-currency financial displays, approval workflow status indicators, hierarchical drill-down navigation, real-time KPI dashboards, audit trails, and document attachment workflows. Gaps in expressiveness force engineers to work around the DSL — a failure mode that degrades platform coherence over time.

**Goal 2: Comprehensibility for backend engineers.** The DSL is authored by backend engineers — Go developers who understand their domain deeply but may not be frontend specialists. The DSL must not require frontend expertise. A backend engineer reading a form definition should understand immediately what it describes. A backend engineer writing a form definition should not need to understand CSS flexbox, React lifecycle methods, or SwiftUI view modifiers.

**Goal 3: Machine-validatable at every stage.** Every DSL construct must be formally defined — with a schema that specifies required and optional properties, type constraints, and structural rules. This enables automated validation at authoring time (Go type system + builder validation), at compilation time (AST schema validation), and at rendering time (client-side schema validation). Any definition that passes all three validation stages is guaranteed to be renderable by a conformant rendering engine.

**Goal 4: Extensible without forking.** Third parties and platform contributors must be able to add new component types, new action types, and new data source adapters without modifying the core DSL grammar. The extension mechanism must be formal (registered with the component registry, validated against a provided schema) rather than informal (undocumented JSON properties passed through unvalidated).

**Goal 5: Versionable with backward compatibility.** The DSL must support a formal versioning scheme that allows the language to evolve — adding new constructs, deprecating old ones — without breaking existing definitions or existing rendering engines that have not yet been updated.

**Goal 6: Testable in isolation.** It must be possible to test a UI definition without a running rendering engine. The compiled JSON output of a business service's AST builder call is a value that can be asserted against in unit tests. The definition is self-contained: it does not reference external state that would make tests non-deterministic.

### Trade-offs Made

**Expressiveness vs. Comprehensibility.** A more expressive DSL (one that could express arbitrary UI patterns) would be less comprehensible. The DSL deliberately excludes constructs that require deep frontend knowledge — absolute positioning, z-index management, animation keyframes, custom CSS properties — and provides semantic abstractions instead (`state: "error"`, `variant: "primary"`, `density: "compact"`). Some UI patterns are deliberately inexpressible; they require a custom component registered through the extension mechanism.

**Flexibility vs. Validation.** A schema-validated DSL is less flexible than an unvalidated JSON object. Some properties that would be useful in specific edge cases cannot be expressed because they have not been added to the schema. This is intentional: unschematized properties cannot be validated, documented, or maintained. Every necessary property goes through the schema update process.

---

## 6.3 DSL Grammar Overview

The DSL grammar defines what constructs exist and how they may be combined. This section provides a conceptual overview; the formal grammar is expressed in the JSON Schema definitions in the `/schemas` directory of the platform repository.

### 6.3.1 Nodes

A **node** is the fundamental unit of the DSL. Every element in a UI definition — a component, a layout container, a data binding, an action reference — is a node. Nodes have:
- A unique `id` within their surface
- A `type` that identifies the node's kind and determines which schema governs its structure
- A `props` object containing the node's configuration (typed according to the schema for its type)
- Optional child nodes, slots, events, and metadata

Node types are namespaced using dot notation to prevent collisions: `layout.grid`, `form.section`, `field.currency_amount`, `action.http_request`, `widget.kpi_card`. The namespace prefix identifies the node's category; the suffix identifies the specific type within that category.

```json
{
  "id": "total-amount-field",
  "type": "field.currency_amount",
  "props": {
    "label": "Total Amount",
    "currency_field": "currency_code",
    "required": true,
    "read_only": false
  }
}
```

### 6.3.2 Edges (Parent-Child Relationships)

The DSL forms a tree structure through parent-child relationships. Not all child relationships are the same — the DSL distinguishes between:

**Positional children** (`children` array): An ordered list of child nodes placed in the parent's default content area. The order is significant — the rendering engine renders children in the specified order.

```json
{
  "type": "layout.stack",
  "props": { "direction": "vertical", "gap": "md" },
  "children": [
    { "type": "widget.kpi_card", "props": { "..." } },
    { "type": "widget.activity_feed", "props": { "..." } }
  ]
}
```

**Slot children** (`slots` object): Named child placements in specific positions within a component. A component may define multiple named slots — for example, a `Card` component may have `header`, `body`, and `footer` slots. Each slot accepts a specific type of child.

```json
{
  "type": "ui.card",
  "slots": {
    "header": { "type": "ui.card_header", "props": { "title": "Order Summary" } },
    "body": { "type": "layout.stack", "children": [ "..." ] },
    "footer": { "type": "ui.card_footer", "props": { "..." } }
  }
}
```

The distinction between positional children and slot children is meaningful: positional children are interchangeable items in a list; slot children are specifically placed elements in a defined structure. A component schema specifies which relationship type it uses for each of its child-accepting fields.

### 6.3.3 Slots

A **slot** is a named insertion point within a component where a specific type of child node can be placed. Slots are defined by the component schema; they specify:
- The slot's name
- Whether the slot is required or optional
- Which node types are accepted in the slot
- Whether the slot accepts a single node or multiple nodes

Slots enable components to have complex internal structures without requiring every variation to be a separate component type. A `Form.Section` with a custom header is expressed by placing a custom node in the section's `header` slot — rather than requiring a `FormSectionWithCustomHeader` component.

### 6.3.4 Bindings

A **binding** is an expression that computes a property value dynamically, rather than providing a static literal value. Bindings connect component properties to data sources, state variables, and computed expressions.

A binding is distinguished from a literal value by its structure: a literal value is a JSON primitive (string, number, boolean, null) or a plain object; a binding is an object with a `$` prefix key that identifies the binding type.

```json
// Static literal value
{ "label": "Total Amount" }

// Data source binding — value comes from a named data source
{ "value": { "$data": "datasource:po_totals.total_amount" } }

// Expression binding — value is computed
{ "value": { "$expr": "sum(line_items[*].amount)" } }

// State binding — value comes from a local state variable
{ "visible": { "$state": "show_advanced_fields" } }

// Conditional binding — value depends on a condition
{ "required": { "$if": "order_type == 'external'", "$then": true, "$else": false } }
```

See Section 6.5 for the complete binding expression specification.

### 6.3.5 Directives

A **directive** is a special instruction to the compilation pipeline or rendering engine that modifies how a node is processed, rather than what it renders. Directives are prefixed with `$` to distinguish them from regular props.

```json
{
  "type": "field.text",
  "props": { "label": "GL Code", "required": true },
  "$if": { "$permission": "finance:gl_code:write" },
  "$locale_key": "field.gl_code",
  "$feature_flag": "finance.enhanced-gl-picker"
}
```

The `$if` directive is a conditional: if the condition evaluates to false, the node is excluded from the compiled output. The `$locale_key` directive marks the node as having locale-dependent content. The `$feature_flag` directive makes the node's inclusion conditional on the named flag being active.

See Section 6.6 for the complete directive specification.

---

## 6.4 Type System for the DSL

### 6.4.1 Primitive Types

The DSL supports the following primitive property value types:

| Type | JSON Representation | Go Type | Example |
|------|---------------------|---------|---------|
| `string` | JSON string | `string` | `"label": "Vendor Name"` |
| `i18n_key` | JSON string with key prefix | `i18n.Key` | `"label": { "$i18n": "vendor.name.label" }` |
| `integer` | JSON number (no decimal) | `int64` | `"max_length": 500` |
| `float` | JSON number | `float64` | `"min_value": 0.01` |
| `boolean` | JSON boolean | `bool` | `"required": true` |
| `null` | JSON null | `nil` pointer | `"default_value": null` |
| `duration` | JSON string (ISO 8601) | `time.Duration` | `"timeout": "PT30S"` |
| `date` | JSON string (ISO 8601 date) | `civil.Date` | `"min_date": "2026-01-01"` |
| `datetime` | JSON string (RFC 3339) | `time.Time` | `"created_after": "2026-01-01T00:00:00Z"` |
| `color_token` | JSON string (design token ref) | `design.ColorToken` | `"color": "token:status.error"` |
| `icon_ref` | JSON string (icon identifier) | `icon.Ref` | `"icon": "icon:check-circle"` |

### 6.4.2 Composite Types

Composite types are structured objects used as property values within node props:

**`LocalizedString`**: A string value that may be either a literal string or an internationalization key reference.

```go
// Go — either literal or i18n key
type LocalizedString struct {
    Literal  *string  // if set, use this literal value
    I18nKey  *i18nKey // if set, resolve via localization service
}
```

```json
// JSON — literal form
{ "label": "Vendor" }

// JSON — i18n key form
{ "label": { "$i18n": "po.field.vendor" } }
```

**`ValidationRule`**: Specifies a single validation constraint on a field.

```json
{
  "type": "min_length",
  "value": 3,
  "message": { "$i18n": "validation.min_length" }
}
```

**`DataSourceRef`**: A reference to a named data source, with optional field path.

```json
{ "$data": "datasource:vendors.name" }
```

**`ActionRef`**: A reference to a named action defined in the same surface.

```json
{ "$action": "action-submit-po" }
```

**`PermissionRef`**: A permission identifier for the authorization system.

```json
{ "$permission": "procurement:po:approve" }
```

### 6.4.3 Reference Types

Reference types are values that point to other constructs within the same surface definition. They are resolved at compilation time (for permission references, flag references) or at rendering time (for data source and action references).

**Data source references** (`$data:`) point to a declared data source in the surface's data source registry and optionally specify a field path within the data source's response schema.

**Action references** (`$action:`) point to a named action node within the same surface. Used in event handler bindings.

**Component references** (`$component:`) point to a named, reusable component template registered in the component library (for template composition patterns — see Chapter 07).

**State variable references** (`$state:`) point to a named state variable in the current scope or a parent scope.

### 6.4.4 Conditional Types

Conditional types express values that depend on a runtime condition. They are evaluated by the rendering engine when the component is rendered.

```json
{
  "required": {
    "$if": "form.order_type == 'external'",
    "$then": true,
    "$else": false
  }
}
```

A conditional type has three fields:
- `$if`: A binding expression that evaluates to a boolean
- `$then`: The value to use if the condition is true
- `$else`: The value to use if the condition is false (defaults to `null` / the type's zero value if omitted)

Conditional types can be nested: the `$then` or `$else` value may itself be a conditional type. The maximum nesting depth is 5 (enforced at validation time) to prevent incomprehensible condition trees.

### 6.4.5 Expression Types

Expression types are values computed by the expression evaluator at rendering time. They are the most dynamic construct in the DSL — capable of performing computations, aggregations, and string transformations on live data.

```json
{
  "value": { "$expr": "format_currency(sum(line_items[*].unit_price * line_items[*].quantity), currency_code)" }
}
```

The expression language is formally specified in Appendix A of this documentation. Key characteristics:
- **Purely functional**: expressions have no side effects
- **Sandboxed**: no access to APIs, file system, or platform capabilities
- **Typed**: the expression evaluator performs type checking at evaluation time
- **Bounded**: maximum depth 20, maximum string length 10,000 characters, timeout 100ms

---

## 6.5 Binding Expressions

Binding expressions are the mechanism by which component properties become dynamic — connected to live data, computed from state, or conditioned on runtime values. This section specifies all supported binding types.

### 6.5.1 Static Bindings

A static binding is simply a literal value. It is the default and requires no special syntax.

```json
{ "label": "Purchase Order Number", "required": true, "max_length": 20 }
```

Static bindings are resolved at compilation time. They do not change after the definition is delivered to the client. Use static bindings for values that do not depend on runtime data — labels, structural configuration, validation rules.

### 6.5.2 Data Bindings (`$data`)

A data binding connects a property to a value within a named data source.

```
{ "$data": "<datasource_id>.<field_path>" }
```

```json
{
  "id": "vendor-name-display",
  "type": "ui.text",
  "props": {
    "content": { "$data": "datasource:purchase_order.vendor.name" }
  }
}
```

The `datasource_id` must match a data source declared in the surface's `data_sources` block. The `field_path` is a dot-separated path into the data source's response schema. Array indexing uses bracket notation: `line_items[0].description`.

Data bindings are reactive: when the data source refreshes, the bound property updates automatically, triggering a re-render of the affected component.

### 6.5.3 Conditional Bindings (`$if`)

A conditional binding evaluates a boolean condition and provides one of two values.

```json
{
  "visible": {
    "$if": "form.order_type == 'external'",
    "$then": true,
    "$else": false
  }
}
```

The condition expression has access to:
- All declared state variables in scope
- All loaded data source values
- Built-in context values (`user.id`, `tenant.id`, `now()`, `today()`)

### 6.5.4 Computed Bindings (`$expr`)

A computed binding evaluates an expression and uses the result as the property value.

```json
{
  "value": { "$expr": "sum(line_items[*].amount)" },
  "formatted_value": { "$expr": "format_currency(total, currency_code, locale)" }
}
```

Built-in expression functions include:

| Category | Functions |
|----------|-----------|
| **Math** | `sum()`, `avg()`, `min()`, `max()`, `abs()`, `round()`, `ceil()`, `floor()` |
| **Array** | `count()`, `filter()`, `map()`, `find()`, `sort()`, `group_by()`, `distinct()` |
| **String** | `concat()`, `trim()`, `upper()`, `lower()`, `substring()`, `replace()`, `format()` |
| **Date** | `now()`, `today()`, `date_diff()`, `date_add()`, `format_date()` |
| **Finance** | `format_currency()`, `convert_currency()`, `format_percentage()` |
| **Logic** | `if()`, `coalesce()`, `not()`, `and()`, `or()` |
| **Type** | `is_null()`, `is_empty()`, `to_string()`, `to_number()`, `to_boolean()` |

### 6.5.5 Permission Bindings (`$permission`)

A permission binding evaluates a permission check for the current user and returns a boolean.

```json
{
  "disabled": {
    "$if": { "$not": { "$permission": "procurement:po:edit" } },
    "$then": true,
    "$else": false
  }
}
```

> ⚠️ **Warning:** Permission bindings in client-side props are for **UI enhancement only** — for example, disabling a field for users who cannot edit it (even though the backend would reject their edit anyway). They must never be the sole enforcement mechanism. All authorization enforcement happens server-side, both at the API level and in the compilation pipeline's permission pruning pass.

### 6.5.6 State Bindings (`$state`)

A state binding reads the current value of a named state variable.

```json
{
  "visible": { "$state": "page.show_advanced_section" }
}
```

State variables are declared in the surface definition's `state` block. They have an initial value (which may itself be a binding) and are mutated by `action.set_variable` actions.

---

## 6.6 Directives

Directives modify how a node is processed by the compilation pipeline or the rendering engine. They are attached to nodes alongside (not inside) the `props` object.

### 6.6.1 Visibility Directives (`$if`, `$show`)

The `$if` directive controls whether a node is **included** in the compiled output. It is evaluated at compilation time. If the condition is false, the node is completely absent from the JSON payload.

```json
{
  "type": "field.text",
  "props": { "label": "Internal Reference" },
  "$if": { "$permission": "finance:internal_ref:read" }
}
```

The `$show` directive controls whether a node is **rendered** at runtime. It is evaluated by the rendering engine. If the condition is false, the node is in the payload but not displayed. Use `$show` (not `$if`) for visibility that changes based on runtime state — for example, showing an "advanced options" section when the user toggles it.

```json
{
  "type": "form.section",
  "props": { "title": "Advanced Options" },
  "$show": { "$state": "page.show_advanced" }
}
```

> ✅ **Best Practice:** Prefer `$if` over `$show` for permission-based visibility — it ensures that unauthorized content never leaves the backend. Use `$show` only for user-controlled, non-authorization-related visibility toggling.

### 6.6.2 Iteration Directives (`$for`)

The `$for` directive repeats a node for each item in a collection.

```json
{
  "type": "ui.badge",
  "props": {
    "label": { "$data": "$item.name" },
    "color": { "$data": "$item.color" }
  },
  "$for": {
    "items": { "$data": "datasource:po.tags" },
    "item_key": "id",
    "item_alias": "$item"
  }
}
```

The `$for` directive provides:
- `items`: The collection to iterate over (a data binding resolving to an array)
- `item_key`: The field name to use as the unique key for each item (for efficient re-rendering)
- `item_alias`: The name by which the current item is referenced in child bindings

The `$for` directive is evaluated by the rendering engine at render time. The node is rendered once for each item in the collection.

### 6.6.3 Permission Directives (`$if` + `$permission`)

Permission directives are the most common use of the `$if` directive. They restrict node inclusion to users with a specific permission.

```json
{
  "type": "action.button",
  "props": { "label": "Approve", "variant": "primary" },
  "$if": { "$permission": "procurement:po:approve" }
}
```

Multiple permissions can be combined:

```json
// AND — user must have ALL permissions
"$if": { "$and": [
  { "$permission": "procurement:po:approve" },
  { "$permission": "procurement:budget:check" }
]}

// OR — user must have ANY permission
"$if": { "$or": [
  { "$permission": "procurement:po:approve" },
  { "$permission": "admin:override" }
]}
```

### 6.6.4 Feature Flag Directives (`$flag`)

The `$flag` directive makes a node conditional on a feature flag being active. Evaluated at compilation time.

```json
{
  "type": "field.vendor_selector",
  "props": { "variant": "enhanced" },
  "$flag": "procurement.enhanced-vendor-search"
}
```

For multi-variant flags, use the `$flag_variant` form:

```json
{
  "type": "widget.analytics_panel",
  "props": { "variant": "v2" },
  "$flag": { "name": "analytics.panel-variant", "value": "v2" }
}
```

When a flag directive evaluates to false, the node is replaced by its `$fallback` sibling (if one is declared) or removed entirely.

### 6.6.5 Locale Directives (`$locale`)

The `$locale` directive marks a node as locale-sensitive. It informs the localization pass that the node's string properties should be resolved against the translation catalog.

In practice, most localization happens automatically through the `LocalizedString` type system — string props declared as `LocalizedString` in the component schema are automatically resolved. The explicit `$locale` directive is used for cases where the localization behavior needs to be overridden or the locale context needs to be explicitly set.

```json
{
  "type": "ui.text",
  "props": { "content": { "$i18n": "dashboard.welcome_message" } },
  "$locale": "en-US"
}
```

---

## 6.7 DSL Extensibility — Defining Custom Node Types

The DSL can be extended with new node types through the Component Registry. A custom node type is defined by:

**1. A JSON Schema** describing the node's property structure:

```json
{
  "$schema": "http://json-schema.org/draft-07/schema",
  "title": "CustomVendorScorecard",
  "type": "object",
  "properties": {
    "vendor_id": { "type": "string", "description": "The vendor's unique identifier" },
    "show_history": { "type": "boolean", "default": false },
    "max_periods": { "type": "integer", "minimum": 1, "maximum": 24, "default": 6 }
  },
  "required": ["vendor_id"]
}
```

**2. A Go builder** implementing the `ast.NodeBuilder` interface:

```go
// Custom node builder registered with the platform
type VendorScorecardBuilder struct {
    vendorID   string
    showHistory bool
    maxPeriods  int
}

func NewVendorScorecard(vendorID string) *VendorScorecardBuilder {
    return &VendorScorecardBuilder{vendorID: vendorID, maxPeriods: 6}
}

func (b *VendorScorecardBuilder) WithHistory(show bool) *VendorScorecardBuilder {
    b.showHistory = show
    return b
}

func (b *VendorScorecardBuilder) Build() (*ast.Node, error) {
    return ast.NewCustomNode("plugin.acme.vendor_scorecard", map[string]any{
        "vendor_id":    b.vendorID,
        "show_history": b.showHistory,
        "max_periods":  b.maxPeriods,
    }), nil
}
```

**3. A rendering engine implementation** for each supported platform (web, iOS, Android). This is the responsibility of the extension's author, documented in Chapter 50 (SDK Development).

**4. Registration in the Component Registry** via the platform's governance process (Chapter 45). Custom node types cannot be used in compiled definitions until they are registered — unregistered types fail schema validation.

---

## 6.8 DSL Validation Rules

The DSL enforces the following categories of validation at compilation time. Violations are returned as structured errors with node path, field name, and human-readable messages.

### Structural Validation

- Every node must have a non-empty `id` that is unique within its surface
- Every `id` must match the pattern `[a-z][a-z0-9\-]*` (lowercase, alphanumeric, hyphens)
- The `type` field must match a registered component type
- Required props (as defined by the component schema) must be present
- Prop values must conform to their declared types
- Slot contents must match the slot's declared accepted types
- Node nesting must not exceed the maximum depth (default: 50 levels)

### Reference Integrity Validation

- Data source references (`$data`) must point to a data source declared in the same surface's `data_sources` block
- Action references (`$action`) must point to an action node declared in the same surface
- State variable references (`$state`) must point to a variable declared in the same surface's `state` block or a parent scope
- Permission references (`$permission`) must match the format `namespace:resource:action` and must be registered in the permission catalog

### Semantic Validation

- A `$for` directive's `items` binding must resolve to an array type
- A `$if` directive's condition must resolve to a boolean type
- Expression bindings must be syntactically valid (parsed by the expression language parser)
- Fields with `read_only: true` must not also have `required: true` (a read-only field cannot be user-provided)
- Action `method` fields on `action.http_request` nodes must be one of: GET, POST, PUT, PATCH, DELETE

### Permission Consistency Validation

- A field marked as `required: true` that is also protected by a `$if: { $permission: "..." }` directive should be accompanied by a warning: if the field is required but conditionally included, form submission may fail for users who do not have the permission to see the field
- Actions with permission directives must also have matching server-side authorization on their target endpoint (this is a lint warning, not a hard error, since the server-side authorization is not visible to the compilation service)

---

## 6.9 DSL Versioning

The DSL version is tracked independently of the platform software version. A DSL version increment indicates that the formal language has changed — new node types added, existing node types modified, deprecated types removed.

### Version Format

DSL versions follow semantic versioning:
- **MAJOR**: Breaking changes — existing valid definitions may become invalid, or existing definitions may change semantics
- **MINOR**: Backward-compatible additions — new node types, new optional props, new directives
- **PATCH**: Corrections — bug fixes to the schema definitions, documentation corrections, validation rule adjustments that don't change the language

### Version Declaration

Every surface definition includes the DSL version it was compiled against:

```json
{
  "schema_version": "2.1.0",
  "surface_id": "procurement.purchase-order.create",
  "..."
}
```

### Version Negotiation

Rendering engines declare their maximum supported DSL version in their capability declaration. The compilation service compiles definitions against the highest DSL version that both the client and the current backend support. This allows old clients to continue receiving definitions they can handle while new clients receive definitions that take advantage of new features.

### Deprecation Process

When a DSL construct is deprecated:
1. The deprecation is announced in the DSL changelog with a minimum 90-day notice period
2. Compilation of definitions using the deprecated construct emits a deprecation warning in the compilation log
3. After the notice period, the deprecated construct is removed from the supported construct set for new compilations, but continues to be recognized in existing cached definitions for a further 90 days
4. After the total 180-day period, the construct is retired. Definitions referencing it fail schema validation.

---

*End of Chapter 06*

**Previous:** [Chapter 05 — SDUI Fundamentals](./05-sdui-fundamentals.md)
**Next:** [Chapter 07 — AST Design](./07-ast-design.md)
