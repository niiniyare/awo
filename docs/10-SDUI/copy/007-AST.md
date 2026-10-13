# Chapter 07 — AST Design

> **Volume:** II — DSL & AST
> **Phase:** 1 (Foundation)
> **Audience:** Platform Engineers, Backend Engineers
> **Prerequisites:** Chapters 01–06

---

## Table of Contents

- [7.1 What Is the UI AST?](#71-what-is-the-ui-ast)
- [7.2 AST Node Taxonomy](#72-ast-node-taxonomy)
- [7.3 Node Schema — Universal Fields](#73-node-schema--universal-fields)
- [7.4 AST Construction in Go](#74-ast-construction-in-go)
- [7.5 AST Validation](#75-ast-validation)
- [7.6 AST Transformation Passes](#76-ast-transformation-passes)
- [7.7 AST Diffing and Patching](#77-ast-diffing-and-patching)
- [7.8 AST Serialization Formats](#78-ast-serialization-formats)
- [7.9 AST Canonical Form](#79-ast-canonical-form)

---

## 7.1 What Is the UI AST?

The UI Abstract Syntax Tree (AST) is the **in-memory, typed, structured representation** of a UI surface definition. It is the form in which the UI definition exists inside the Go backend — as a tree of strongly-typed Go structs — before it is serialized to JSON for transport.

The term "Abstract Syntax Tree" is borrowed from compiler design, where an AST is the structured representation of a program's source code after parsing, before code generation. In AwoERP, the analogy holds:

| Compiler Concept | AwoERP Equivalent |
|------------------|-------------------|
| Source code | Go AST builder API calls (the "source" a developer writes) |
| Parsing | The AST builder constructs the tree as the developer calls builder methods |
| AST | The `ast.Surface` struct and its tree of `ast.Node` children |
| Semantic analysis | AST validation passes |
| Code generation | JSON serialization (compilation output) |
| Target machine code | The rendered UI on client devices |

The AST is the pivot point in the compilation pipeline: it is the form produced by business services and the form consumed by all transformation passes. JSON is only produced at the very end of the pipeline, by the serialization stage.

### Why Not Write JSON Directly?

A reasonable question: why maintain a typed Go AST at all? Why not have business services write JSON directly?

**Type safety.** The Go type system enforces the structural rules of the DSL at the point of authoring. If a component requires a `vendor_id` prop of type `string`, the Go builder will not compile if you pass an integer. If a node type does not accept children, the builder's `AddChild` method does not exist on that type. Errors are caught by the Go compiler, not at runtime in a production compilation request.

**Programmatic construction.** Go code can construct AST subtrees using loops, conditionals, and helper functions. A `LineItemSection` with a variable number of columns based on business configuration is straightforward Go code; it would be complex and error-prone as a JSON template with runtime interpolation.

**Testability.** The output of a business service's AST builder call is a Go struct value. Unit tests can make precise structural assertions: "this form should have 3 sections," "the vendor_id field should be required when order_type is external," "the approve action should be absent when the PO is in draft state." These assertions work against Go struct values — they do not require JSON parsing or XPath-style traversal.

**Transformation as first-class operations.** All transformation passes (permission pruning, flag resolution, localization injection) are functions that accept an `*ast.Node` and return a (possibly modified) `*ast.Node`. Working with a typed tree makes these transformations safe, composable, and testable in isolation.

---

## 7.2 AST Node Taxonomy

Every element in a UI definition is a node. Nodes are organized into a taxonomy based on their role in the UI.

### 7.2.1 Container Nodes

Container nodes hold other nodes. They define structure and layout but do not themselves render visible content. Their primary purpose is to arrange their children.

| Node Type | Purpose | Key Props |
|-----------|---------|-----------|
| `layout.page` | Top-level surface container | `title`, `subtitle`, `layout_mode` |
| `layout.grid` | CSS-grid-style multi-column layout | `columns`, `gap`, `responsive_rules` |
| `layout.stack` | Vertical or horizontal stack | `direction`, `gap`, `align`, `justify` |
| `layout.split` | Two-pane split layout | `ratio`, `direction`, `resizable` |
| `layout.scroll` | Scrollable content area | `direction`, `snap_enabled` |
| `layout.tabs` | Tabbed content container | `tab_position`, `lazy_load` |
| `layout.accordion` | Collapsible sections | `allow_multiple_open`, `default_open` |
| `form.container` | Form root | `submit_action`, `reset_on_success` |
| `form.section` | Grouped form fields | `title`, `collapsible`, `columns` |
| `dashboard.grid` | Dashboard widget grid | `columns`, `row_height`, `gap` |

### 7.2.2 Leaf Nodes (Atoms)

Leaf nodes render visible, discrete UI elements. They do not have children. They are the atoms from which all complex UI is built.

| Node Type | Purpose | Key Props |
|-----------|---------|-----------|
| `ui.text` | Inline or block text | `content`, `variant`, `color`, `truncate` |
| `ui.heading` | Section or page heading | `content`, `level` (h1–h6) |
| `ui.icon` | Icon display | `name`, `size`, `color` |
| `ui.badge` | Status or count badge | `label`, `variant`, `color` |
| `ui.avatar` | User or entity avatar | `src`, `initials`, `size` |
| `ui.divider` | Visual separator | `orientation`, `spacing` |
| `ui.spacer` | Empty space | `size` |
| `ui.image` | Image display | `src`, `alt`, `aspect_ratio`, `fit` |
| `ui.progress_bar` | Linear progress indicator | `value`, `max`, `variant` |
| `ui.spinner` | Loading indicator | `size`, `label` |
| `action.button` | Clickable button | `label`, `icon`, `variant`, `action_ref` |
| `action.icon_button` | Icon-only button | `icon`, `label` (a11y), `action_ref` |
| `action.link` | Inline navigation link | `label`, `target` |

### 7.2.3 Control Flow Nodes

Control flow nodes are not rendered directly — they control the rendering of their children based on conditions or data.

| Node Type | Purpose |
|-----------|---------|
| `flow.conditional` | Renders one of two child subtrees based on a condition |
| `flow.switch` | Renders one of N child subtrees based on a value |
| `flow.repeat` | Renders a child template once per item in a collection |
| `flow.async` | Renders a loading state while a data source loads, then the content |
| `flow.error_boundary` | Renders a fallback subtree if the main subtree fails to render |

### 7.2.4 Data Nodes

Data nodes do not render visible content. They declare and configure data sources that are available to descendant nodes through data bindings.

| Node Type | Purpose |
|-----------|---------|
| `data.rest_source` | Declares an HTTP REST data source |
| `data.grpc_source` | Declares a gRPC data source |
| `data.static_source` | Declares an inline static data payload |
| `data.computed_source` | Declares a data source computed from other sources |
| `data.realtime_source` | Declares a WebSocket or SSE real-time data source |

Data nodes are typically placed as direct children of the surface's root `layout.page` node, making their data available to the entire surface.

### 7.2.5 Action Nodes

Action nodes define operations that the client can execute. They are referenced by `action_ref` bindings on interactive components and by event handler bindings.

| Node Type | Purpose |
|-----------|---------|
| `action.http_request` | Executes an HTTP request to a backend endpoint |
| `action.navigate` | Navigates to another surface or external URL |
| `action.open_modal` | Opens a modal surface |
| `action.close_modal` | Closes the current modal |
| `action.set_variable` | Sets a state variable value |
| `action.refresh_source` | Triggers a data source refresh |
| `action.show_notification` | Displays a toast or banner notification |
| `action.download_file` | Initiates a file download |
| `action.workflow_signal` | Sends a signal to a Temporal workflow |
| `action.chain` | Executes a sequence of actions |
| `action.conditional` | Executes one of two action chains based on a condition |

### 7.2.6 Layout Nodes

Layout nodes (see 7.2.1 for the Container Nodes superset) specifically control the spatial arrangement of their children. Distinct from container nodes in that layout nodes have no semantic meaning beyond arrangement.

### 7.2.7 Composite Nodes (Templates)

Composite nodes are references to reusable component templates registered in the component registry. They allow complex, frequently-used UI patterns to be expressed as single nodes rather than repeated subtrees.

```go
// Instead of repeating the entire approval action bar AST in every approval surface:
ast.NewTemplateRef("erp.approval_action_bar").
    WithProp("document_id", ast.DataBinding("datasource:po.id")).
    WithProp("workflow_id", ast.DataBinding("datasource:po.workflow_id")).
    WithProp("available_actions", ast.DataBinding("datasource:po.available_actions"))
```

The template reference is expanded to its full subtree during the compilation pipeline's template instantiation pass (see Section 7.4.3).

---

## 7.3 Node Schema — Universal Fields

Every node in the AST — regardless of its type — shares a common set of fields. These universal fields are defined on the base `ast.Node` struct and are inherited by all specific node types.

### 7.3.1 `id` — Unique Node Identity

```go
type Node struct {
    ID string // required; unique within the surface
}
```

The `id` field uniquely identifies a node within its surface. IDs must be:
- Non-empty
- Unique within the surface (two nodes in the same surface cannot share an ID)
- Matching the pattern `[a-z][a-z0-9\-_]*` (lowercase alphanumeric with hyphens and underscores)
- Stable across compilations for the same logical element (the same field in the same form should always have the same ID, to enable accurate diffing and caching)

IDs are used by:
- The rendering engine to identify components for efficient reconciliation (re-rendering only changed nodes)
- Action references and event handler bindings to target specific components
- The compilation audit log to identify pruned, modified, or replaced nodes
- Debugging tools to locate specific elements in the definition

**ID generation convention:**

```go
// Prefer semantic, stable IDs derived from the business concept
"vendor-selector-field"   // ✅ semantic, stable
"po-submit-action"        // ✅ semantic, stable
"node-7f3a2b"             // 🚫 generated, unstable (breaks caching and diffing)
"field_1"                 // 🚫 positional, fragile (breaks when fields are reordered)
```

### 7.3.2 `type` — Node Type Discriminator

```go
type Node struct {
    Type NodeType // required; must match a registered component type
}
```

The `type` field is the discriminator that determines which schema governs the node's structure and which component implementation the rendering engine uses. Types are namespaced dot-separated strings.

### 7.3.3 `props` — Node Properties

```go
type Node struct {
    Props map[string]any // typed according to the component schema
}
```

The `props` field contains the node's configuration — all properties that control its appearance and behavior. The specific set of allowed and required props is defined by the component schema for the node's type.

In the Go builder API, props are set through typed builder methods rather than through the generic `map[string]any` directly:

```go
// ✅ Typed builder methods — compile-time safety
ast.NewTextField("notes").
    WithLabel("Notes").
    WithMaxLength(500).
    WithRequired(false)

// 🚫 Direct map construction — runtime errors, no IDE support
ast.NewNode("field.text", map[string]any{
    "label":      "Notes",
    "max_length": "500",   // type error: should be int, not string
    "required":   false,
})
```

### 7.3.4 `children` — Child Node Array

```go
type Node struct {
    Children []*Node // ordered; nil for leaf nodes
}
```

The ordered list of child nodes for container-type nodes. The order is semantically significant — the rendering engine renders children in this order. For leaf nodes, `Children` is nil (not an empty slice — this distinction is meaningful for serialization: nil `children` is omitted from JSON; an empty slice `[]` serializes as an empty array and may trigger validation errors on leaf node schemas).

### 7.3.5 `slots` — Named Child Slots

```go
type Node struct {
    Slots map[string]*Node // named insertion points
}
```

Named slot placements for components that define specific content areas. See Section 6.3.2 for the distinction between positional children and slot children.

### 7.3.6 `events` — Event Handler Bindings

```go
type Node struct {
    Events map[EventName][]*ActionRef // event name → ordered list of action references
}
```

The event bindings map event names (as defined by the component schema) to ordered lists of action references. When the specified event fires on this component, the listed actions are executed in order.

```go
// Bind the "on_row_click" event of a table to a navigation action
ast.NewDataTable("po-list-table").
    OnEvent("on_row_click",
        ast.ActionRef("navigate-to-po-detail"),
    )
```

### 7.3.7 `actions` — Action Node Declarations

```go
type Node struct {
    Actions []*ActionNode // action nodes declared in the scope of this node
}
```

Action nodes may be declared at any scope in the tree. An action declared on a node is available to that node and all of its descendants. Actions declared at the surface root are available everywhere in the surface.

### 7.3.8 `permissions` — Authorization Metadata

```go
type Node struct {
    Permissions *PermissionSpec // optional; controls permission pruning behavior
}
```

```go
type PermissionSpec struct {
    Required  []PermissionRef // all must be satisfied (AND)
    AnyOf     []PermissionRef // at least one must be satisfied (OR)
    Condition *Expression     // arbitrary expression using $permission() calls
}
```

The `permissions` field instructs the permission pruning pass on how to evaluate access for this node. If the permissions are not satisfied for the current user, the node is removed from the compiled output.

### 7.3.9 `flags` — Feature Flag Conditions

```go
type Node struct {
    Flags *FlagSpec // optional; controls flag-based inclusion
}
```

```go
type FlagSpec struct {
    Required  string      // flag name; node included only if flag is active
    Variant   *string     // if set, flag must match this variant value
    Fallback  *Node       // rendered instead of this node if flag is inactive
}
```

### 7.3.10 `metadata` — Debug / Tracing Fields

```go
type Node struct {
    Metadata *NodeMetadata // optional; stripped from production payloads by default
}

type NodeMetadata struct {
    SourceLocation string  // "procurement/po_service.go:142" — for debugging
    Description    string  // human-readable note for developers
    Tags           []string // categorization tags for tooling
    TestID         string  // stable identifier for automated UI tests
}
```

Metadata is included in development and staging compilations to assist debugging. In production compilations, metadata is stripped from the serialized JSON by default (configurable). The `test_id` field is always preserved in non-production environments for use by end-to-end test selectors.

---

## 7.4 AST Construction in Go

### 7.4.1 Builder Pattern

The AST builder library uses the fluent builder pattern throughout. Each node type has a corresponding builder struct with typed setter methods. Builders are immutable-by-convention: each setter returns a new builder (or the same builder with the field set — implementations may vary for performance, but callers treat them as immutable).

```go
// Fluent builder chain
field := ast.NewCurrencyAmountField("total-amount").
    WithLabel(i18n.Key("po.total_amount")).
    WithCurrencyField("currency_code").
    WithRequired(true).
    WithReadOnly(false).
    WithPermissions(ast.RequirePermission("finance:amount:read")).
    WithMetadata(ast.Meta().Source("po_service.go:88"))
```

The builder's `Build()` method (or implicit build on `AddChild()`) validates the constructed node against its schema and returns either a valid `*ast.Node` or an error. In the common case where the node is immediately added to a parent, the parent's `AddChild()` method calls `Build()` internally and panics on error (ensuring that build-time schema errors surface immediately in development).

> ⚠️ **Warning:** The panic-on-error behavior of `AddChild()` is intentional for development ergonomics — it surfaces schema errors at the exact line of code that caused them. In production, the compilation pipeline wraps AST construction in a recovery handler. Engineers should still treat builder panics as bugs to fix, not expected behavior to handle.

### 7.4.2 Typed Node Factories

Each node category has a package-level factory function:

```go
// Layout factories
ast.NewGridLayout(columns int) *GridLayoutBuilder
ast.NewStackLayout(direction Direction) *StackLayoutBuilder
ast.NewTabsLayout() *TabsLayoutBuilder

// Form factories
ast.NewForm(id string) *FormBuilder
ast.NewFormSection(id string) *FormSectionBuilder
ast.NewTextField(id string) *TextFieldBuilder
ast.NewCurrencyAmountField(id string) *CurrencyAmountFieldBuilder
ast.NewVendorSelector(id string) *VendorSelectorBuilder
ast.NewDateField(id string) *DateFieldBuilder

// Action factories
ast.NewHTTPAction(id string) *HTTPActionBuilder
ast.NewNavigateAction(id string) *NavigateActionBuilder
ast.NewWorkflowSignalAction(id string) *WorkflowSignalActionBuilder

// Data source factories
ast.NewRESTDataSource(id string) *RESTDataSourceBuilder
ast.NewRealtimeDataSource(id string) *RealtimeDataSourceBuilder
```

The factory functions return concrete builder types (not the generic `*ast.Node`), enabling IDE auto-completion and compile-time type checking for all setter methods.

### 7.4.3 Slot Composition

Slots are populated using the `WithSlot()` builder method:

```go
ast.NewCard("summary-card").
    WithSlot("header",
        ast.NewCardHeader("summary-header").
            WithTitle(i18n.Key("po.summary.title")).
            WithSubtitle(ast.DataBinding("datasource:po.reference_number")),
    ).
    WithSlot("body",
        ast.NewStackLayout(ast.Vertical).
            AddChild(ast.NewTextField("po-status").WithReadOnly(true)).
            AddChild(ast.NewCurrencyAmountField("po-total").WithReadOnly(true)),
    ).
    WithSlot("footer",
        ast.NewCardFooter("summary-footer").
            WithActionRefs("action-view-detail"),
    )
```

### 7.4.4 Recursive AST Construction

Complex ERP forms involve recursive or deeply nested structures. The builder pattern supports this naturally because each builder call is a Go expression — builders can be composed using helper functions, loops, and conditionals.

```go
// Build a dynamic line item section from business configuration
func buildLineItemSection(cfg *POLineItemConfig) *ast.FormSectionBuilder {
    section := ast.NewFormSection("line-items").
        WithTitle(i18n.Key("po.section.line_items")).
        WithRepeatable(true).
        WithMinItems(1)

    for _, col := range cfg.Columns {
        section.AddField(buildLineItemField(col))
    }

    if cfg.AllowTaxOverride {
        section.AddField(
            ast.NewCurrencyAmountField("tax-override").
                WithLabel(i18n.Key("po.line_item.tax_override")).
                WithPermissions(ast.RequirePermission("finance:tax:override")),
        )
    }

    return section
}
```

---

## 7.5 AST Validation

AST validation runs after construction and before any transformation passes. It is a deterministic, synchronous operation that either returns the validated AST or a list of structured validation errors.

### 7.5.1 Structural Validation

Verifies that the tree's shape conforms to the component registry's structural rules:
- Required slots are populated
- Slot contents match the accepted node types for that slot
- Positional children are of accepted types for that container
- Node nesting depth does not exceed the maximum (default: 50)
- No cycles exist in the node graph

### 7.5.2 Type Validation

Verifies that all prop values conform to their declared types:
- String props contain strings (not numbers passed as strings — these are a common error)
- Integer props contain integers within declared min/max bounds
- Boolean props contain booleans
- Enum props contain one of the declared enum values
- Required props are present and non-null

### 7.5.3 Reference Integrity Validation

Verifies that all references within the tree resolve to declared targets:
- `$data:` bindings reference declared data source IDs
- `$action:` refs reference declared action node IDs
- `$state:` refs reference declared state variable names
- Template refs reference registered component templates

### 7.5.4 Permission Consistency Validation

Validates the logical consistency of permission specifications:
- Warning: A required field protected by a permission directive (see Section 6.8)
- Warning: A submit action with a permission directive that differs from the form's own permission requirement (the form may be visible but the submit blocked — ensure the UX handles this)
- Error: A `$permission` reference that does not match the permission catalog format

### 7.5.5 Cyclic Reference Detection

Ensures that the AST forms a tree (not a graph with cycles). Template references must not form circular reference chains: Template A → Template B → Template A is an error. The validator performs a depth-first traversal and detects cycles by tracking the set of nodes currently in the traversal stack.

### Validation Error Structure

```go
type ValidationError struct {
    NodeID    string          // ID of the node with the error
    NodePath  string          // dot-separated path from root: "root.form.section-1.field-vendor"
    Field     string          // which field on the node is invalid: "props.max_length"
    Code      ValidationCode  // machine-readable error code: REQUIRED_PROP_MISSING
    Message   string          // human-readable message for developers
    Severity  Severity        // ERROR or WARNING
}
```

Errors halt compilation; warnings are collected and included in the compilation response headers (for development) and the compilation audit log (for all environments).

---

## 7.6 AST Transformation Passes

After validation, the AST passes through a sequence of transformation operations before serialization. Each pass is a pure function: it takes an `*ast.Surface` and returns a (possibly modified) `*ast.Surface`. Passes do not mutate their input; they return a new surface with modified subtrees.

### 7.6.1 Permission Pruning Pass

**Purpose:** Remove all nodes that the current user is not authorized to see or interact with.

**Algorithm:** Depth-first post-order traversal. For each node:
1. Evaluate the node's `permissions` spec against the current user using the Authorization Resolver
2. If access is denied: remove the node from its parent's children or slots; record the removal in the audit log
3. If access is granted: recurse into children and slots

**Output:** An AST containing only nodes the current user may access.

**Performance:** Authorization checks are batched before the pass begins. The pass itself makes no network calls — it reads from a pre-populated permission map.

### 7.6.2 Feature Flag Resolution Pass

**Purpose:** Resolve all feature flag directives, substituting flag-conditional nodes with their active variant or fallback.

**Algorithm:** Depth-first pre-order traversal. For each node with a `flags` spec:
1. Look up the flag's current value in the pre-resolved flag map
2. If the flag condition is satisfied: keep the node; remove its `flags` spec (it has been resolved)
3. If the flag condition is not satisfied: replace the node with its `fallback` node (if declared) or remove it

### 7.6.3 Localization Injection Pass

**Purpose:** Replace all `i18n.Key` references in string props with their resolved locale strings.

**Algorithm:** Depth-first traversal of all nodes. For each prop value of type `LocalizedString` with an `I18nKey`:
1. Look up the key in the pre-fetched translation map
2. Replace the `I18nKey` with the resolved string
3. Apply pluralization if the key references a count variable
4. If the key is missing in the target locale, apply the fallback locale chain

**Output:** An AST where all string props contain resolved locale strings. No `i18n.Key` references remain.

### 7.6.4 Default Value Injection Pass

**Purpose:** Ensure all optional props with declared defaults have their default values populated, so the rendering engine does not need to know about defaults.

**Algorithm:** For each node, consult the component schema for default values of optional props that are absent from the node's props. Inject the defaults.

**Rationale:** Moving default value knowledge to the backend (rather than requiring each rendering engine platform to duplicate the defaults) ensures consistency. If a component's default `max_length` changes, it changes in the backend — all platforms receive the new default without any client update.

### 7.6.5 Tenant Override Application Pass

**Purpose:** Apply tenant-specific customizations to the base AST.

**Algorithm:** The tenant configuration record contains an ordered list of override operations. Each operation targets a node by ID and specifies a modification:

```go
type OverrideOperation struct {
    TargetNodeID string
    Operation    OverrideOp // SET_PROP, REMOVE_NODE, ADD_CHILD, REPLACE_NODE
    Field        string     // for SET_PROP: which prop to set
    Value        any        // for SET_PROP: the new value
    Node         *ast.Node  // for ADD_CHILD, REPLACE_NODE: the new node
    Position     int        // for ADD_CHILD: insertion position
}
```

**Example:** ACME Corp has configured:
```json
[
  { "target": "vendor-selector-field", "op": "SET_PROP", "field": "props.label", "value": "Supplier" },
  { "target": "preferred-vendor-field", "op": "REMOVE_NODE" },
  { "target": "header-section", "op": "ADD_CHILD", "position": 3,
    "node": { "type": "field.text", "id": "internal-cost-code", "props": { "label": "Cost Code", "required": true } } }
]
```

**Validation:** Override operations are validated against the base AST before application. An override targeting a node ID that does not exist in the base AST is a configuration error (logged and skipped, not a compilation failure — graceful degradation for misconfigured tenant overrides).

---

## 7.7 AST Diffing and Patching

When a surface definition changes due to an event (a workflow step advances, a data source updates, a real-time notification arrives), sending the complete new definition is wasteful. The AST diffing system computes a **minimal patch** that transforms the old definition into the new one.

### Diff Algorithm

The diff algorithm operates on the compiled AST (not the JSON, which is harder to diff semantically). It performs a depth-first comparison of two AST trees:

1. **Same ID, same type, props differ** → `SET_PROPS` patch: only the changed props
2. **Same ID, type changed** → `REPLACE` patch: the entire node subtree is replaced
3. **Node present in old, absent in new** → `REMOVE` patch
4. **Node absent in old, present in new** → `INSERT` patch with the new subtree and its position
5. **Same ID, same type, same props, children differ** → recurse into children

The diff produces a **patch document**:

```json
{
  "schema_version": "2.1.0",
  "base_etag": "sha256:a3f9b2...",
  "patches": [
    { "op": "set_props", "node_id": "action-approve", "props": { "disabled": false } },
    { "op": "remove", "node_id": "action-request-revision" },
    { "op": "insert", "parent_id": "actions-container", "position": 2,
      "node": { "id": "action-countersign", "type": "action.button", "props": { "..." } } }
  ]
}
```

The rendering engine applies the patch atomically — from the user's perspective, the UI updates in a single frame.

### When Diffing Is Used

- Real-time workflow state changes (Temporal signals triggering UI updates)
- Data source refresh without full page reload
- Background permission changes (a role is modified while the user has the session open)
- Feature flag changes pushed to active sessions

Diffing is **not** used for navigation — navigating to a new surface always delivers the full definition. Diffing is only used for in-place updates to an already-rendered surface.

---

## 7.8 AST Serialization Formats

The AST can be serialized to multiple formats depending on the client's preference and the transport mechanism.

### JSON (Default)

Standard JSON serialization, gzip-compressed for transport. Used for REST API responses to all client types.

```
Content-Type: application/json
Content-Encoding: gzip
```

Field naming convention: `snake_case` throughout. No camelCase in the JSON output — this is an intentional divergence from JavaScript conventions to maintain consistency with the Go and PostgreSQL layers.

### Protocol Buffer Binary (gRPC)

For gRPC transport (used for streaming and server-push scenarios), the AST is serialized using Protocol Buffers. The proto schema is generated from the same component schemas that generate JSON Schema definitions — a single source of truth with multiple serialization targets.

```
Content-Type: application/grpc
```

Protocol Buffer serialization is approximately 30–60% smaller than gzip-compressed JSON for typical UI definitions. For real-time streaming scenarios where many small patch documents are delivered, the size reduction is meaningful.

### Debug JSON (Development Only)

An extended JSON format that includes compilation metadata — the full `compilation_trace` alongside the definition. Available only in non-production environments and to users with the `platform:debug` permission.

---

## 7.9 AST Canonical Form

The canonical form is a deterministic, normalized JSON representation of an AST used for cache key computation, snapshot testing, and change detection.

**Canonicalization rules:**
1. Object keys sorted alphabetically at every level
2. All optional fields with default values explicitly included (no omitted defaults)
3. Arrays maintained in their defined order (arrays are ordered by design)
4. No whitespace in the output (minified)
5. Null values for absent optional fields (no field omission)
6. Metadata fields stripped

The canonical form is used to compute the `sha256` hash that forms the ETag for HTTP caching. Two compilations that produce the same canonical form have the same ETag and result in a `304 Not Modified` response to clients that present the ETag in subsequent requests.

```go
// Canonical form computation
canonical, err := ast.Canonicalize(surface)
if err != nil {
    return nil, fmt.Errorf("canonicalization failed: %w", err)
}
etag := fmt.Sprintf(`"sha256:%s"`, sha256hex(canonical))
```

For snapshot testing, the canonical form is what test assertions compare against:

```go
func TestPurchaseOrderCreateForm(t *testing.T) {
    surface, err := procurementService.BuildPurchaseOrderCreateAST(
        context.Background(),
        testContext(userJane, tenantACME),
    )
    require.NoError(t, err)

    canonical, err := ast.Canonicalize(surface)
    require.NoError(t, err)

    // Golden file assertion — fails if the form definition changes unexpectedly
    golden.Assert(t, canonical, "testdata/po_create_form_jane_acme.golden.json")
}
```

Golden file tests are the primary mechanism for catching unintended changes to UI definitions. When a change is intentional (a new field added, a label updated), the developer updates the golden file as part of their change — making the change visible in code review.

---

*End of Chapter 07*

**Previous:** [Chapter 06 — UI DSL Architecture](./06-ui-dsl-architecture.md)
**Next:** [Chapter 08 — JSON Compilation Pipeline](./08-json-compilation-pipeline.md)
