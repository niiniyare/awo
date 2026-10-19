# Page Builder Guide

**Classification:** Guide — Tier 2
**Owner:** `10-sdui/PAGE_BUILDER_GUIDE.md`
**Status:** Living document

---

## Purpose

This guide explains how to write custom `PageBuilder` functions for entities that need layouts beyond the auto-generated defaults. It covers the `PageBuilderSet` type, the WidgetTree construction API, permission-gated elements, and caching.

---

## 1. When to Write a Custom Page Builder

The auto-generated views (List, Create, Edit, Detail) satisfy 90% of ERP use cases. Write a custom `PageBuilder` only when:

- The form requires conditional sections (e.g., section B appears only if field A = "approved")
- The detail view must inline a computed summary panel
- The list view needs non-standard row actions or column groupings
- The entity has tab-based layout with heterogeneous content per tab

Do **not** write custom builders to change field labels, reorder standard fields, or add help text — those are field-level properties configured in `FieldDef`.

---

## 2. PageBuilderSet

Declare custom builders on `EntityDefinition.PageBuilders`:

```go
PageBuilders: def.PageBuilderSet{
    List:   BuildInvoiceListPage,   // override only what differs
    Detail: BuildInvoiceDetailPage,
    // Create and Edit: auto-generated
},
```

Unset fields use the auto-generated builder. Override only the views that need custom layout.

---

## 3. PageBuilder Function Signature

> **Corrected 2026-10-19 (consistency gate, `tasks.md` R.21):** the signature previously shown here did not match the real `def.PageBuilder` type and has never matched it in this repository's history. The real contract, verified against `def/page.go:29-40` this pass:
>
> ```go
> // PageBuilder is a function that produces an amis JSON schema for a specific
> // view. It receives the rendering context (actor, feature flags, tenant
> // settings) and returns the raw schema as a map.
> //
> // The schema must conform to the AMIS SDK version ratified by ADR-026
> // (amis@6.7.0) — def/page.go's own doc comment currently still says "the
> // pinned amis SDK version in web/sdk/", which is the same stale claim
> // ADR-026 already corrected in AMIS_RENDERER_SPEC.md; that Go source
> // comment itself is not corrected by this documentation-only pass.
> //
> // Returning nil instructs the framework to fall back to the auto-generated
> // schema for this page kind.
> type PageBuilder func(ctx context.Context, pctx PageContext) (map[string]any, error)
> ```
>
> **This means `PageBuilder` returns raw amis JSON directly, not a `widget.Node`** — the handler serves whatever a non-nil result contains straight back as the HTTP response, bypassing the generator, validation, layout, and renderer stages entirely (`api/sdui/handler.go:378-396`). **§4 below's worked examples, which construct and return a `*widget.Node` tree, do not match this real signature** and will not compile against the current `def.PageBuilder` type. They remain useful as illustrations of the *intent* (build a structured page from field/section definitions) but are not a drop-in template. Whether `def.PageBuilder` should instead be changed to return `*widget.Node` (restoring renderer-neutrality for custom builders) is an open architectural question, not resolved by this documentation pass — see `tasks.md` **R.22**.
>
> `PageContext` (not `auth.ViewerContext` + `*compiler.EntitySchema` as separate parameters) carries `Actor`, `EntityName`, `Kind`, `TenantSettings`, `EnabledFeatureFlags`, `RecordID`, and `RecordState` — see `def/page.go:44-71`.

The rest of this section's original content is retained below as a historical/illustrative example of WidgetTree construction patterns — **not a currently buildable example against `def.PageBuilder`'s real signature** (see the correction above):

```go
// PageBuilder constructs a WidgetTree for one view of one entity.
// viewer is the current authenticated principal — use it for permission gating.
// schema is the compiled EntitySchema — use it for route URLs and field metadata.
type PageBuilder func(
    ctx    context.Context,
    viewer auth.ViewerContext,
    schema *compiler.EntitySchema,
) (*widget.Node, error)
```

The function MUST return a `widget.Node` with a structural `NodeKind` at the root (`NodePage`, `NodeForm`, or `NodeList`). It MUST NOT return nil without an error.

---

## 4. Constructing a WidgetTree

### 4.1 Basic Form with Sections

```go
func BuildInvoiceDetailPage(
    ctx    context.Context,
    viewer auth.ViewerContext,
    schema *compiler.EntitySchema,
) (*widget.Node, error) {
    return &widget.Node{
        Kind:  widget.NodePage,
        Label: "Invoice Detail",
        Children: []*widget.Node{
            {
                Kind:  widget.NodeSection,
                Label: "Invoice Header",
                Children: []*widget.Node{
                    {Kind: widget.NodeText,   Name: "number", Label: "Invoice Number", ReadOnly: true},
                    {Kind: widget.NodeSelect, Name: "status", Label: "Status",         ReadOnly: true},
                    {Kind: widget.NodeNumber, Name: "total",  Label: "Total",          ReadOnly: true},
                },
            },
            {
                Kind:  widget.NodeSection,
                Label: "Customer",
                Children: []*widget.Node{
                    {Kind: widget.NodeText, Name: "customer_name", Label: "Customer", ReadOnly: true},
                },
            },
        },
        Actions: buildInvoiceActions(viewer, schema),
    }, nil
}
```

### 4.2 Tabbed Layout

```go
Children: []*widget.Node{
    {
        Kind:  widget.NodeTabs,
        Children: []*widget.Node{
            {
                Kind:  widget.NodeSection,
                Label: "Invoice Lines",
                Children: []*widget.Node{
                    buildInvoiceLinesTable(schema),
                },
            },
            {
                Kind:  widget.NodeSection,
                Label: "Payments",
                Children: []*widget.Node{
                    buildPaymentsTable(schema),
                },
            },
        },
    },
},
```

### 4.3 Inline Child Table (OneToMany edge)

```go
func buildInvoiceLinesTable(schema *compiler.EntitySchema) *widget.Node {
    lookup := schema.Lookups["invoice_line"]  // CompiledLookup for child entity
    return &widget.Node{
        Kind:  widget.NodeTable,
        Label: "Invoice Lines",
        DataSource: &widget.DataSource{
            URL:        lookup.SearchURL,
            Method:     "GET",
            LabelField: "description",
            ValueField: "id",
        },
        Children: []*widget.Node{
            {Kind: widget.NodeText,   Name: "description", Label: "Description"},
            {Kind: widget.NodeNumber, Name: "quantity",    Label: "Quantity"},
            {Kind: widget.NodeNumber, Name: "unit_price",  Label: "Unit Price"},
            {Kind: widget.NodeNumber, Name: "line_total",  Label: "Total"},
        },
    }
}
```

---

## 5. Permission-Gated Elements

**Security rule:** Absent elements CANNOT be revealed by client-side manipulation. Hidden elements CAN. Always use absent (not hidden) for permission-gated elements.

```go
func buildInvoiceActions(viewer auth.ViewerContext, schema *compiler.EntitySchema) []*widget.ActionNode {
    var actions []*widget.ActionNode

    // Submit action — only for accounts payable role
    if viewer.HasRole("role:finance.accounts_payable") {
        actions = append(actions, &widget.ActionNode{
            Label:      "Submit for Approval",
            ActionType: "ajax",
            Level:      "primary",
            API:        schema.RoutePrefix + "/${id}/submit",
        })
    }

    // Approve action — only for finance manager
    if viewer.HasRole("role:finance.manager") {
        actions = append(actions, &widget.ActionNode{
            Label:      "Approve",
            ActionType: "ajax",
            Level:      "primary",
            API:        schema.RoutePrefix + "/${id}/approve",
        })
    }

    // Cancel action — tenant admin only
    if viewer.HasRole("role:tenant.admin") || viewer.HasRole("role:finance.manager") {
        actions = append(actions, &widget.ActionNode{
            Label:      "Cancel",
            ActionType: "ajax",
            Level:      "danger",
            API:        schema.RoutePrefix + "/${id}/cancel",
        })
    }

    // View action — always present (every authenticated user with read permission)
    actions = append(actions, &widget.ActionNode{
        Label:      "Back to List",
        ActionType: "link",
        Level:      "default",
        Href:       schema.RoutePrefix,
    })

    return actions
}
```

**MUST NOT** use `Node.Hidden: true` for permission gates — the node MUST be absent from the tree if the viewer lacks the required permission. This is enforced at schema-serve time, not at render time.

---

## 6. Using EntitySchema in Builders

`compiler.EntitySchema` provides pre-compiled data needed by builders:

```go
schema.RoutePrefix          // e.g. "/api/v1/entities/finance_invoice"
schema.LocalName            // e.g. "invoice"
schema.QualifiedName        // e.g. "finance_invoice"
schema.Label                // e.g. "Invoice"
schema.LabelPlural          // e.g. "Invoices"
schema.Lookups["customer"]  // CompiledLookup{SearchURL: "...", LabelField: "name", ValueField: "id"}
```

Always use `schema.RoutePrefix` for API URLs — never hard-code paths. Hard-coded paths break if the module prefix changes.

---

## 7. DataSource Configuration

```go
DataSource: &widget.DataSource{
    URL:        schema.RoutePrefix + "?customer_id=${customer_id}",
    Method:     "GET",
    SendOn:     "${customer_id}",    // only fetch when customer_id is non-empty
    LabelField: "name",
    ValueField: "id",
},
```

`SendOn` prevents fetching until a prerequisite field is populated. Use amis expression syntax (`${fieldName}`).

---

## 8. Props for Renderer-Specific Properties

Use `Node.Props` for amis-specific properties not expressible in typed fields:

```go
&widget.Node{
    Kind:  widget.NodeNumber,
    Name:  "total",
    Label: "Total (KES)",
    Props: map[string]any{
        "prefix":    "KES ",
        "precision": 4,
        "step":      0.0001,
    },
},
```

`Props` are merged last by the renderer and override all computed defaults. Keep `Props` usage sparse — prefer typed fields for cross-renderer portability.

---

## 9. Caching Behaviour

Custom page builders are cached identically to auto-generated views:

```
page:{entity_qualified_name}:{view}:{roles_hash}:{tenant_id}
```

TTL: 5 minutes. The builder function is not invoked on cache hit.

If the builder's output depends on data that changes more frequently than 5 minutes, call `ActionRuntime.InvalidateCache(ctx, entityName)` after mutations that should trigger a fresh build.

---

## 10. Auto-Generated View Reference

For reference, the auto-generated List view structure is:

```
NodeList (DataSource: schema.RoutePrefix)
  ├── NodeButton "New {LabelPlural}" → link to Create view
  ├── Columns: all non-excluded fields (see SDUI_FIELD_WIDGET_MAP.md)
  └── Row actions: NodeButton "View", NodeButton "Edit" (permission-gated "Delete")
```

Auto-generated Form (Create/Edit):

```
NodeForm (DataSource: schema.RoutePrefix)
  ├── NodeSection (if fields declare Group)
  │   └── {NodeKind per field}
  └── NodeTable per OneToMany edge
```

---

## References

- [`10-sdui/WIDGET_TREE_SPEC.md`](WIDGET_TREE_SPEC.md) — Node struct, NodeKind constants
- [`10-sdui/AMIS_RENDERER_SPEC.md`](AMIS_RENDERER_SPEC.md) — NodeKind → amis type mapping
- [`10-sdui/SDUI_FIELD_WIDGET_MAP.md`](SDUI_FIELD_WIDGET_MAP.md) — FieldType → NodeKind table
- [`05-compiler/COMPILED_SCHEMA_REFERENCE.md`](../05-compiler/COMPILED_SCHEMA_REFERENCE.md) — EntitySchema fields
- [`03-auth/VIEWER_CONTEXT.md`](../03-auth/VIEWER_CONTEXT.md) — ViewerContext.HasRole()
