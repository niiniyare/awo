# WidgetTree Specification

**Classification:** Specification — Tier 1
**Owner:** `10-sdui/WIDGET_TREE_SPEC.md`
**Status:** Frozen at v1.0 (ADR-006)
**Package:** `awo.so/awo/sdui/widget`

---

## Purpose

This document specifies the WidgetTree intermediate representation (IR) — the typed structure that sits between the SDUI generator and any UI renderer. The amis renderer converts WidgetTree to amis JSON; future renderers may target other platforms.

## Dependencies

None. The `widget` package has no internal dependencies.

---

## 1. ADR-006 Rationale

Before the WidgetTree IR, the SDUI generator produced amis JSON (`map[string]any`) directly. This coupled the framework permanently to amis. Every amis upgrade required auditing all generators. Any alternative renderer would require rewriting everything.

The IR decouples generation (EntityDefinition → WidgetTree) from rendering (WidgetTree → amis JSON). Adding a renderer is O(1 renderer); removing amis is O(1 renderer). Generators need not change.

---

## 2. Node Struct

```go
// Package: awo.so/awo/sdui/widget

// Node is a single widget in the WidgetTree.
// Nodes form a tree via Children. Each Node has a semantic Kind
// that renderers translate to renderer-specific types.
type Node struct {
    // Kind is the semantic widget type (required).
    Kind NodeKind

    // ID is the optional stable identifier for this node.
    // Used by renderers for reference (e.g., dialog target IDs).
    ID string

    // Label is the human-readable display text.
    Label string

    // Name is the field binding name.
    // For form fields: the field name in EntityRecord.Data.
    // For lists: not used.
    Name string

    // Required marks a form field as required in the UI.
    // Does not duplicate server-side validation — both apply independently.
    Required bool

    // ReadOnly marks a form field as non-editable.
    ReadOnly bool

    // Hidden marks a node as absent from the rendered output.
    // Used for permission-gated elements (absent, not just disabled).
    Hidden bool

    // Props contains renderer-specific properties not expressible
    // in the typed fields above. Use sparingly.
    Props map[string]any

    // Children are the nested nodes (for page, form, section, tabs, table).
    Children []*Node

    // DataSource configures remote data fetching for list and select nodes.
    DataSource *DataSource

    // Actions are the action buttons associated with this node.
    Actions []*ActionNode
}
```

---

## 3. NodeKind Constants

```go
type NodeKind string

const (
    // Structural
    NodePage     NodeKind = "page"      // top-level page container
    NodeForm     NodeKind = "form"      // form container
    NodeList     NodeKind = "list"      // paginated record list
    NodeSection  NodeKind = "section"   // visual grouping within a form
    NodeTabs     NodeKind = "tabs"      // tabbed navigation
    NodeTable    NodeKind = "table"     // inline child record table
    NodeDialog   NodeKind = "dialog"    // modal dialog

    // Input fields
    NodeField    NodeKind = "field"     // generic single-line text
    NodeText     NodeKind = "text"      // single-line text input (FieldTypeData)
    NodeTextArea NodeKind = "textarea"  // multi-line text (FieldTypeLongText, SmallText)
    NodeNumber   NodeKind = "number"    // numeric input (Int, Float, Currency)
    NodeSelect   NodeKind = "select"    // dropdown or autocomplete (Select, Link)
    NodeDate     NodeKind = "date"      // date picker (FieldTypeDate)
    NodeDateTime NodeKind = "datetime"  // datetime picker (FieldTypeDateTime)
    NodeSwitch   NodeKind = "switch"    // toggle switch (FieldTypeBool)
    NodeEditor   NodeKind = "editor"    // JSON editor (FieldTypeJSON)

    // Interactive
    NodeButton   NodeKind = "button"    // action trigger
)
```

---

## 4. DataSource Struct

Used by `NodeList` and `NodeSelect` (with server-side search) to configure remote data fetching:

```go
type DataSource struct {
    // URL is the API endpoint.
    // May contain amis-style template variables: ${keywords}, ${page}, ${tenant_id}
    URL string

    // Method is the HTTP method (default: "GET").
    Method string

    // SendOn is a condition expression for conditional data fetching.
    // Empty means always fetch.
    SendOn string

    // LabelField is the field name used as the display label in select options.
    LabelField string

    // ValueField is the field name used as the select option value (default: "id").
    ValueField string
}
```

---

## 5. ActionNode Struct

Represents an action button on a node (typically on list rows or detail views):

```go
type ActionNode struct {
    // Label is the button display text.
    Label string

    // ActionType is the semantic type: "submit", "dialog", "link", "ajax".
    ActionType string

    // Level controls the visual prominence: "primary", "default", "warning", "danger".
    Level string

    // Href is the navigation target for "link" type actions.
    Href string

    // API is the endpoint for "ajax" type actions.
    API string
}
```

---

## 6. Standard Page Views

The SDUI generator produces four standard views for each entity:

| View | Root NodeKind | Description |
|------|--------------|-------------|
| List | NodeList | Paginated table of records with column headers |
| Create | NodeForm | Empty form for new record creation |
| Edit | NodeForm | Pre-populated form for record modification |
| Detail | NodePage | Read-only display of a single record |

### Auto-Generated List View Structure

```
NodeList (DataSource: entity RoutePrefix + query params)
  ├── NodeButton "New {Label}" → navigates to Create view
  ├── Columns from Fields (non-Sensitive, non-LongText)
  └── Actions: [NodeButton "View", NodeButton "Edit", ...custom actions...]
```

### Auto-Generated Form View Structure

```
NodeForm (submit to RoutePrefix)
  ├── NodeSection (if fields are grouped)
  │   ├── {NodeKind for field 1}
  │   ├── {NodeKind for field 2}
  │   └── ...
  └── [child table NodeTable for OneToMany edges]
```

---

## 7. Permission-Gated Elements

When a viewer lacks a required permission for an action, the corresponding `ActionNode` MUST be absent from the tree — not present with `Hidden: true`, but completely omitted.

The generator checks `PolicyEvaluator.CanPerform()` at schema generation time and omits nodes the viewer cannot use.

This is security-critical: hidden elements can be made visible by client-side manipulation. Absent elements cannot.

---

## 8. Normative Requirements

- `NodeKind` values MUST be one of the defined constants. Custom NodeKinds are not permitted.
- `Node.Hidden: true` MUST NOT be used for permission-gating — absent nodes MUST be used instead.
- `Node.DataSource.URL` MUST use the compiled `RoutePrefix` from `EntitySchema`, not hard-coded paths.
- `Node.Props` MUST be used only for renderer-specific properties not covered by typed fields.
- The WidgetTree MUST be constructable without importing `awo/sdui/amis`.

---

## References

- `awo/sdui/widget/node.go` — Node struct and NodeKind constants
- [`10-sdui/AMIS_RENDERER_SPEC.md`](AMIS_RENDERER_SPEC.md) — NodeKind → amis type mapping
- [`10-sdui/SDUI_FIELD_WIDGET_MAP.md`](SDUI_FIELD_WIDGET_MAP.md) — FieldType → NodeKind mapping
- ADR-006 in [`00-overview/DECISION_REGISTER.md`](../00-overview/DECISION_REGISTER.md)
