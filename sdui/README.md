# SDUI Framework — Developer Guide

Server-Driven UI (SDUI) generates complete, permission-aware web interfaces from `EntityDefinition` metadata. Zero handwritten presentation code per entity.

**One registration drives five subsystems:** persistence, API, SDUI, authorization, workflow.

---

## Table of Contents

1. [Architecture Overview](#1-architecture-overview)
2. [Package Responsibilities](#2-package-responsibilities)
3. [End-to-End Request Pipeline](#3-end-to-end-request-pipeline)
4. [Defining an EntityDefinition for SDUI](#4-defining-an-entitydefinition-for-sdui)
5. [Generator](#5-generator)
6. [Engine](#6-engine)
7. [Renderer](#7-renderer)
8. [Layout Engine](#8-layout-engine)
9. [Cache](#9-cache)
10. [Validation](#10-validation)
11. [Plugins](#11-plugins)
12. [Dashboard Framework](#12-dashboard-framework)
13. [API Handler](#13-api-handler)
14. [Adding New Widgets](#14-adding-new-widgets)
15. [Adding New Renderers](#15-adding-new-renderers)
16. [Adding Plugins](#16-adding-plugins)
17. [Integrating SDUI into New Modules](#17-integrating-sdui-into-new-modules)
18. [Extension Points](#18-extension-points)
19. [Best Practices](#19-best-practices)
20. [Common Pitfalls](#20-common-pitfalls)
21. [Minimal Working Examples](#21-minimal-working-examples)

---

## 1. Architecture Overview

```
EntityDefinition
    │
    ▼ compiler.Compile()
compiler.EntitySchema        ← single source of truth for all runtime subsystems
    │
    ▼ adapt.FromCompiled()
generator.EntitySchema       ← renderer-agnostic entity projection
    │
    ▼ engine.Handle()
*widget.Node                 ← Widget IR (intermediate representation)
    │
    ▼ Renderer.Render()
RenderedOutput               ← AMIS JSON / Flutter widget tree / PDF bytes
    │
    ▼ HTTP response
browser
```

**Key architectural invariants:**

- `generator` must NOT import `compiler` or `expression` — import direction is one-way through `adapt`.
- `widget` has no dependencies — pure data types only.
- `amis` (renderer) imports `widget` and `expression`, but NOT `generator` or `compiler`.
- Permission-gated nodes are **absent** from the tree — never present with `Hidden: true`.
- The engine is the only public entry point; HTTP handlers call only `engine.Handle()`.

---

## 2. Package Responsibilities

| Package | Responsibility |
|---|---|
| `awo/def` | DSL — `EntityDefinition`, `FieldDef`, `PermissionSet`, `LayoutDef` |
| `awo/compiler` | Validates `EntityDefinition` → `CompiledSchema` |
| `awo/sdui/adapt` | Bridges `compiler.EntitySchema` → `generator.EntitySchema` |
| `awo/sdui/generator` | Builds widget IR trees from `EntitySchema` + `GeneratorContext` |
| `awo/sdui/widget` | Widget IR types: `Node`, `NodeKind`, `LayoutHint`, `ActionNode`, `DataSource` |
| `awo/sdui/expression` | Portable expression AST; serializes to renderer-specific strings |
| `awo/sdui/sduictx` | `GeneratorContext` — immutable request context for generation |
| `awo/sdui/engine` | Orchestrates full pipeline: generate → validate → layout → render → cache |
| `awo/sdui/renderer` | `Renderer` interface, `RendererContext`, `RenderedOutput`, locale table |
| `awo/sdui/amis` | AMIS renderer: `*widget.Node` → AMIS JSON `map[string]any` |
| `awo/sdui/layout` | Renderer-independent column span / row computation |
| `awo/sdui/validation` | Widget tree semantic validation (hard gate before render) |
| `awo/sdui/cache` | L2 (widget tree) + L3 (rendered output) Redis caching |
| `awo/sdui/plugins` | Plugin pipeline: pre/post-generation tree transforms |
| `awo/sdui/dashboard` | Dashboard panel registry |
| `awo/sdui/registry` | `NodeKind` registration; sealed at bootstrap |
| `awo/sdui/observability` | Metrics and tracing for the SDUI pipeline |
| `awo/api/sdui` | HTTP handler: extracts params, calls `engine.Handle`, returns JSON |

---

## 3. End-to-End Request Pipeline

```
GET /api/v1/ui/finance/invoices
    │
    ▼ Fiber middleware (TenantResolver → RequireAuth → RateLimit)
    │
    ▼ api/sdui.Handler.handle()
    │   - extract module, resource, locale, renderer ID
    │   - build sduictx.GeneratorContext
    │   - call adapt.FromCompiled(es) → generator.EntitySchema
    │   - call engine.Handle(ctx, Request{...})
    │
    ▼ engine.Engine.Handle()
    │
    ├── L3 cache lookup (rendered output) ──────────────── HIT → return cached JSON
    │
    ├── L2 cache lookup (widget tree) ──────────────────── HIT → skip generation
    │
    ├── generator.EntityGenerator.Generate()
    │       EntitySchema + GeneratorContext → *widget.Node
    │       Permission-gate: absent nodes for denied fields
    │       View mode: list / create / edit / detail / dashboard
    │
    ├── L2 cache store (widget tree JSON)
    │
    ├── validation.Validator.Validate()
    │       Fatal issues abort pipeline (e.g. nil nodes, unknown NodeKind)
    │
    ├── layout.Engine.Compute()
    │       Resolve column spans, group fields into rows
    │
    ├── Renderer.Render(*widget.Node, RendererContext)
    │       amis renderer: *widget.Node → map[string]any
    │
    ├── L3 cache store (rendered JSON)
    │
    └── return RenderedOutput
    │
    ▼ api/sdui.Handler
        - set ETag, Cache-Control headers
        - c.JSON(resp.Output.AMISSchema)
```

**Cache key dimensions (9):** entity name, view mode, renderer ID, renderer version, locale, tenant ID hash, schema fingerprint, permission fingerprint, level (L2/L3).

---

## 4. Defining an EntityDefinition for SDUI

SDUI is driven entirely from `EntityDefinition`. No hand-crafted widget code needed.

### Minimal definition

```go
var InvoiceDefinition = def.SystemDefinition{
    Name:        "invoice",
    Module:      "finance",
    Label:       "Invoice",
    LabelPlural: "Invoices",
    Icon:        "file-text",  // optional; shown in nav + list header

    Fields: []def.FieldDef{
        {Name: "number", Type: def.FieldTypeNamingSeries, Series: "INV-{YYYY}-{SEQ:5}", Required: true},
        {Name: "status", Type: def.FieldTypeSelect, Options: []string{"draft","submitted","posted"}, Required: true},
        {Name: "amount", Type: def.FieldTypeCurrency, Required: true},
        {Name: "due_date", Type: def.FieldTypeDate},
        {Name: "notes", Type: def.FieldTypeLongText},
    },

    Permissions: def.PermissionSet{
        Create: []string{"finance.invoice.create"},
        Read:   []string{"finance.invoice.read"},
        Update: []string{"finance.invoice.update"},
        Delete: []string{"finance.invoice.delete"},
    },
}

func init() {
    def.Register(&InvoiceDefinition)
}
```

This single registration automatically produces:
- `GET /api/v1/finance/invoices` → list page with columns
- `GET /api/v1/ui/finance/invoices` → SDUI list view JSON
- `GET /api/v1/ui/finance/invoices/create` → SDUI create form JSON
- `GET /api/v1/ui/finance/invoices/:id` → SDUI detail view JSON
- `GET /api/v1/ui/finance/invoices/:id/edit` → SDUI edit form JSON

### Field display hints

```go
def.FieldDef{
    Name:        "customer_id",
    Type:        def.FieldTypeLink,
    LinkTarget:  "finance_customer",
    Label:       "Customer",
    Icon:        "user",            // fa-user prefix on input
    Width:       "lg",              // xs|sm|md|lg|xl|full
    Placeholder: "Search customers…",
    Searchable:  true,              // include in list filter bar
}
```

### Conditional expressions

```go
def.FieldDef{
    Name:       "discount_reason",
    Type:       def.FieldTypeSmallText,
    VisibleOn:  "data.has_discount === true",   // raw AMIS JS expression
    RequiredOn: "data.has_discount === true",
}
```

### Layout (sections and tabs)

```go
Layout: def.LayoutDef{
    Tabs: []def.TabDef{
        {
            Name:  "main",
            Label: "Main",
            Icon:  "info",
            Sections: []def.SectionDef{
                {
                    Name:  "details",
                    Label: "Invoice Details",
                    Columns: []def.ColumnDef{
                        {Fields: []string{"number", "status", "due_date"}},
                        {Fields: []string{"amount", "currency_id"}},
                    },
                },
            },
        },
        {
            Name:  "notes",
            Label: "Notes",
            Sections: []def.SectionDef{
                {Name: "notes", Columns: []def.ColumnDef{{Fields: []string{"notes"}}}},
            },
        },
    },
},
```

### Actions

```go
Actions: []def.ActionDef{
    {
        Name:    "submit",
        Label:   "Submit",
        Method:  "POST",
        Permissions: []string{"finance.invoice.submit"},
        ConfirmMessage: "Submit this invoice for approval?",
    },
},
```

### Workflow triggers

```go
Workflows: []def.WorkflowDef{
    {Name: "approve", Label: "Approval Workflow"},
},
```

When `len(WorkflowDef) > 0`, the detail view automatically emits a `NodeWorkflowPanel` at `{DetailURL}/workflow-state`.

---

## 5. Generator

**Package:** `awo/sdui/generator`

Transforms `EntitySchema` + `GeneratorContext` into a `*widget.Node` tree.

```go
g := generator.New()
tree, err := g.Generate(schema, ctx)
```

### View mode builders

| ViewMode | Builder | Output |
|---|---|---|
| `ViewModeList` | `buildList` | `NodePage > [NodeFilterBar?,] NodeList(columns)` |
| `ViewModeCreate` | `buildForm` | `NodePage > NodeForm(fields)` |
| `ViewModeEdit` | `buildForm` | same; initApi populated from DetailURL |
| `ViewModeDetail` | `buildDetail` | `NodePage > NodeSummaryCard, [NodeWorkflowPanel?,] NodeForm(read-only), NodeRelatedList*` |
| `ViewModeDashboard` | `buildDashboard` | `NodePage > NodeKPICard|NodeChartPanel|NodeTablePanel|NodeFilterBar*` |

### Filter bar

Emitted before `NodeList` when at least one field has `Searchable: true`. Fields without `Searchable` never appear in the filter bar. This prevents accidental filter bars on entities with no searchable fields.

### Permission gating

The generator calls `ctx.Viewer.HasPermission(f.Permission)` for each field. Fields denied are **absent** from the tree — they do not appear with `Hidden: true`. This is enforced by ADR-006 and is the correct behavior: hidden elements can be revealed by client manipulation; absent elements cannot.

### Plugin hooks

Pre-generation schema transforms (`ExtPreGeneration`) and post-generation tree transforms (`ExtPostGeneration`) run at the start and end of `Generate()`. See §11.

---

## 6. Engine

**Package:** `awo/sdui/engine`

The sole entry point for the SDUI pipeline. HTTP handlers call only `engine.Handle()`.

### Construction

```go
eng := engine.New(engine.Options{
    Generator: generator.New(),
    Validator: validation.New(),
    Layout:    layout.New(),
    Renderers: map[string]renderer.Renderer{
        amis.RendererID: amis.New(),
    },
    DefaultRendererID: amis.RendererID,
    Cache:             sduiCacheFor(redisClient), // nil for no-op cache
    Obs:               myObsMetrics,              // nil for no-op
})
```

### Calling the engine

```go
resp, err := eng.Handle(ctx, engine.Request{
    Ctx:    generatorCtx,
    Schema: generatorSchema,
    RendererID:  "amis",
    RendererCtx: renderer.ApplyLocale(renderer.RendererContext{GenCtx: generatorCtx}, locale),
})
// resp.Output.AMISSchema is map[string]any
// resp.CacheHit is true on cache hit
```

---

## 7. Renderer

**Package:** `awo/sdui/renderer` (interface), `awo/sdui/amis` (AMIS implementation)

```go
type Renderer interface {
    ID() string
    Version() string
    Render(root *widget.Node, ctx RendererContext) (RenderedOutput, error)
}
```

### RendererContext

Carries renderer-specific hints. Constructed by the HTTP handler and passed only to the renderer.

```go
rctx := renderer.ApplyLocale(renderer.RendererContext{
    GenCtx: generatorCtx,
    Theme:  "dark",
}, locale)
```

`ApplyLocale` looks up the BCP 47 locale in a static table and populates `DateFormat`, `DateTimeFormat`, `DecimalSeparator`, `ThousandSeparator`, `RTL`.

### Locale table

`renderer.LocaleFormatsFor(locale)` covers 30+ locales. Falls back to language subtag (`"de"` → `"de-DE"`), then `"en-US"` for unknown tags.

### Theme

`amis.ThemeConfigFor(theme)` maps theme names:

| Theme name | AMIS theme | Dark mode CSS overlay |
|---|---|---|
| `""` / `"default"` | `cxd` | no |
| `"antd"` | `antd` | no |
| `"ang"` | `ang` | no |
| `"dark"` | `cxd` | yes — `html.dark` + CSS custom props |
| `"compact"` | `cxd` | no |

Dark mode has no built-in AMIS CSS. Set `html.dark` class in the web shell and scale `--colors-neutral-*` CSS custom properties. Never override `.cxd-*` backgrounds with `!important`.

---

## 8. Layout Engine

**Package:** `awo/sdui/layout`

Sits between generator and renderer. Resolves column spans, groups fields into rows, computes breakpoint-aware grid positions — without knowing about AMIS, Flutter, or PDF.

```go
eng := layout.New()
computed, err := eng.Compute(tree)
// computed.Rows[i] contains grouped field nodes
```

Grid model: 12-column desktop, 8-column tablet, 4-column mobile. Breakpoints configurable via `layout.Options`.

---

## 9. Cache

**Package:** `awo/sdui/cache`

Three-level cache:

| Level | What | TTL |
|---|---|---|
| L1 | In-process CompiledSchema (compiler owns this) | process lifetime |
| L2 | Widget tree JSON | 5 minutes |
| L3 | Rendered output JSON | 10 minutes |

**Cache key dimensions (9):** `entity`, `view`, `renderer_id`, `renderer_version`, `locale`, `tenant_id_hash`, `schema_fp`, `perm_fp`, `level`.

Any dimension change → cache miss → regeneration. Schema fingerprint changes on field/layout/action changes. Permission fingerprint changes on role assignment changes.

```go
// Construction
sduiCache := sdui_cache.New(redisClient) // nil → no-op cache

// Key construction (done internally by engine)
key := sdui_cache.Key(sdui_cache.KeyParams{
    Level:           "l2",
    EntityName:      "finance_invoice",
    ViewMode:        "list",
    RendererID:      "amis",
    RendererVersion: "1.0.0",
    Locale:          "en-US",
    TenantIDHash:    sdui_cache.HashTenantID(tenantID.String()),
    SchemaFP:        schemaFingerprint,
    PermFP:          permFingerprint,
})
```

---

## 10. Validation

**Package:** `awo/sdui/validation`

Hard gate before render. Fatal `ValidationError` aborts the pipeline — no partial rendering.

Checks performed:
- No nil nodes in tree
- All `NodeKind` values registered in `awo/sdui/registry`
- Input fields have non-empty `Name`
- Nodes requiring `DataSource` have one
- `NodeTabPane` appears only as child of `NodeTabs`
- `NodeSection` does not appear as child of `NodeTabs`
- No direct self-reference via ID (cycle guard)
- `ExpressionRef` values have non-nil `Expr`

```go
v := validation.New()
errs := v.Validate(tree)
for _, e := range errs {
    if e.Fatal {
        // pipeline aborts
    }
}
```

---

## 11. Plugins

**Package:** `awo/sdui/plugins`

Two extension points:

| Extension | When | Input/Output |
|---|---|---|
| `ExtPreGeneration` | Before `Generate()` | schema transform |
| `ExtPostGeneration` | After `Generate()` | tree transform |

```go
// Register a plugin
plugins.GlobalPipeline().Register(plugins.Plugin{
    ID:        "my-plugin",
    Ext:       plugins.ExtPostGeneration,
    Priority:  100, // lower = runs first
    Transform: func(root *widget.Node, ctx sduictx.GeneratorContext) (*widget.Node, error) {
        // Walk tree and modify nodes
        widget.WalkAll(root, func(n *widget.Node) bool {
            if n.Kind == widget.NodeText && n.Name == "amount" {
                n.Label = "Total Amount"
            }
            return true
        })
        return root, nil
    },
})
```

Register in `init()` before bootstrap completes. Plugin IDs must be unique — duplicate registration is a bootstrap error.

Use `generator.NewWithPipeline(p)` in tests to inject an isolated pipeline.

---

## 12. Dashboard Framework

**Package:** `awo/sdui/dashboard`

Register dashboard panel definitions for a module:

```go
dashboard.Register(dashboard.ModuleDef{
    Module: "finance",
    Panels: []dashboard.PanelDef{
        {
            ID:          "total-revenue",
            Title:       "Total Revenue",
            PanelType:   "kpi",
            DataURL:     "/api/v1/finance/metrics/revenue",
            KPIFormat:   "currency",
            ColSpan:     4,
            Permissions: []string{"finance.metrics.read"},
        },
        {
            ID:        "invoice-trend",
            Title:     "Invoice Trend",
            PanelType: "chart",
            ChartType: "line",
            DataURL:   "/api/v1/finance/metrics/invoice-trend",
            ColSpan:   8,
        },
    },
})
```

The `adapt` layer calls `dashboard.Registry.Lookup(module)` during `FromCompiled()` and populates `generator.EntitySchema.DashboardPanels`. Panels render as `NodeKPICard`, `NodeChartPanel`, `NodeTablePanel`, or `NodeFilterBar` in `ViewModeDashboard`.

Permission-gate: panels with `Permissions` are absent when the viewer lacks any listed permission.

---

## 13. API Handler

**Package:** `awo/api/sdui`

Thin HTTP layer. No SDUI logic — all delegated to `engine.Engine`.

### Endpoints

| Method | Path | View mode |
|---|---|---|
| GET | `/api/v1/ui/nav` | Navigation schema (sidebar) |
| GET | `/api/v1/ui/:module/:resource` | List |
| GET | `/api/v1/ui/:module/:resource/create` | Create form |
| GET | `/api/v1/ui/:module/:resource/:id` | Detail |
| GET | `/api/v1/ui/:module/:resource/:id/edit` | Edit form |

### Headers

| Request header | Effect |
|---|---|
| `Accept-SDUI-Renderer` | Renderer selection (default: `amis`) |
| `Accept-Language` | Locale (first tag; default: `en-US`) |
| `If-None-Match` | Conditional request → 304 on match |

Response headers: `ETag: "{schemaFP}-{rendererID}-{locale}"`, `Cache-Control: private, max-age=300`.

### Registration

```go
h := api_sdui.New(compiledSchema, sduiEngine, evaluator)
uiGroup := app.Group("/api/v1/ui")
uiGroup.Use(middleware.RequireAuth(...))
h.Register(uiGroup)
```

---

## 14. Adding New Widgets

**1. Add `NodeKind` constant to `widget/node.go`:**

```go
// NodeRating is a star-rating input.
NodeRating NodeKind = "rating"
```

**2. Add any required fields to `widget.Node`:**

```go
// RatingMax is the maximum star count (default 5).
RatingMax int
```

**3. Register in `registry/builtin.go`:**

```go
r.Register(widget.WidgetDef{
    Kind:        widget.NodeRating,
    Description: "Star rating input",
    InputField:  true,
})
```

**4. Handle in `amis/renderer.go`:**

```go
case widget.NodeRating:
    out, err = r.renderRating(n)
```

```go
func (r *DefaultRenderer) renderRating(n *widget.Node) (map[string]any, error) {
    out := map[string]any{
        "type":  "rate",
        "count": 5,
    }
    if n.RatingMax > 0 {
        out["count"] = n.RatingMax
    }
    applyCommon(n, out)
    return out, nil
}
```

**5. Emit from generator in `generator/generator.go`** (add field type → NodeKind mapping):

```go
case "rating":
    n.Kind = widget.NodeRating
```

**6. Add to conformance suite** (`conformance/conformance_test.go` `allNodeKinds()` list).

---

## 15. Adding New Renderers

Implement `renderer.Renderer`:

```go
type PDFRenderer struct{}

func (r *PDFRenderer) ID() string      { return "pdf" }
func (r *PDFRenderer) Version() string { return "1.0.0" }

func (r *PDFRenderer) Render(root *widget.Node, ctx renderer.RendererContext) (renderer.RenderedOutput, error) {
    var buf bytes.Buffer
    // walk root, emit PDF...
    return renderer.NewRawOutput(renderer.FormatPDFBytes, buf.Bytes()), nil
}
```

Register with the engine at construction:

```go
engine.New(engine.Options{
    Renderers: map[string]renderer.Renderer{
        amis.RendererID:  amis.New(),
        "pdf":            &PDFRenderer{},
    },
})
```

Clients select it via `Accept-SDUI-Renderer: pdf`.

---

## 16. Adding Plugins

```go
func init() {
    plugins.GlobalPipeline().Register(plugins.Plugin{
        ID:       "finance-amount-label",
        Ext:      plugins.ExtPostGeneration,
        Priority: 200,
        Transform: func(root *widget.Node, ctx sduictx.GeneratorContext) (*widget.Node, error) {
            if ctx.EntityName != "finance_invoice" {
                return root, nil // skip other entities
            }
            widget.WalkAll(root, func(n *widget.Node) bool {
                if n.Name == "amount" {
                    n.Label = "Invoice Amount (USD)"
                }
                return true
            })
            return root, nil
        },
    })
}
```

Rules:
- Register in `init()` only — never after bootstrap.
- Plugin IDs must be globally unique.
- Return the (possibly modified) root, never nil.
- Use `ctx.EntityName` and `ctx.ViewMode` to scope transforms.
- For tests: `generator.NewWithPipeline(plugins.NewIsolated())` to avoid global state.

---

## 17. Integrating SDUI into New Modules

When building a new module (e.g. `inventory`):

**1. Declare entities with SDUI hints:**

```go
var ProductDefinition = def.SystemDefinition{
    Name:        "product",
    Module:      "inventory",
    Label:       "Product",
    LabelPlural: "Products",
    Icon:        "box",

    Fields: []def.FieldDef{
        {Name: "sku",   Type: def.FieldTypeData,     Required: true, Searchable: true},
        {Name: "name",  Type: def.FieldTypeData,     Required: true, Searchable: true},
        {Name: "price", Type: def.FieldTypeCurrency, Required: true},
        {Name: "category_id", Type: def.FieldTypeLink, LinkTarget: "inventory_category",
         Label: "Category", ClearOn: []string{"subcategory_id"}},
        {Name: "subcategory_id", Type: def.FieldTypeLink, LinkTarget: "inventory_subcategory",
         Label: "Subcategory", VisibleOn: "!!data.category_id"},
    },

    Permissions: def.PermissionSet{
        Create: []string{"inventory.product.create"},
        Read:   []string{"inventory.product.read"},
        Update: []string{"inventory.product.update"},
        Delete: []string{"inventory.product.delete"},
    },
}

func init() { def.Register(&ProductDefinition) }
```

**2. (Optional) Register a dashboard:**

```go
func init() {
    dashboard.Register(dashboard.ModuleDef{
        Module: "inventory",
        Panels: []dashboard.PanelDef{
            {ID: "total-skus", Title: "Total SKUs", PanelType: "kpi",
             DataURL: "/api/v1/inventory/metrics/skus", KPIFormat: "number", ColSpan: 4},
        },
    })
}
```

**3. Import the module in `cmd/server/main.go`** (init side effect):

```go
_ "awo.so/modules/inventory"
```

No HTTP handler code, no form builder, no navigation entry needed — all auto-generated.

---

## 18. Extension Points

| Point | Mechanism | When to use |
|---|---|---|
| Field display | `def.FieldDef.Icon`, `Width`, `Placeholder`, `VisibleOn`, etc. | Per-field UI hints |
| Conditional logic | `VisibleOn`, `HiddenOn`, `DisabledOn`, `RequiredOn` | Client-side state |
| Cascading selects | `ClearOn []string` | Reset dependent fields |
| Layout | `def.LayoutDef` (sections, tabs) | Multi-column / tabbed forms |
| Custom widgets | New `NodeKind` + renderer case | Widget types not in the library |
| Custom renderers | Implement `renderer.Renderer` | Non-AMIS targets (Flutter, PDF, tests) |
| Tree transforms | `plugins.GlobalPipeline().Register()` | Cross-cutting UI concerns |
| Dashboard panels | `dashboard.Register()` | Per-module KPI / chart panels |
| Permission gating | `def.FieldDef.Permission` | Field-level access control |
| Section/tab gating | `def.TabDef.Permission` | Tab-level access control |

---

## 19. Best Practices

**EntityDefinition:**
- Use `Searchable: true` only on fields users actually filter by — filter bars with 10+ fields are unusable.
- Set `Icon` on entities and key fields — improves nav + form scanability.
- Prefer `VisibleOn` over `Hidden` — `Hidden` is a static DSL-time decision; `VisibleOn` responds to user input.
- `Computed: true` on server-derived fields — renders read-only and auto-refreshes on form changes.
- `ClearOn` for cascading selects — prevents stale dependent values.

**Generator:**
- Never import `compiler` or `expression` from `generator`.
- Expression strings passed as plain `string` in `ExpressionRef.Expr` are passed through unchanged. Use `expression.RawExpr()` for typed AST nodes.

**Renderer:**
- `applyCommon(n, out)` handles `name`, `label`, `required`, `disabled`, `description`, `placeholder`, `maxLength`, `icon`, `size`, `clearOn`. Call it first in every field renderer.
- `n.Props` merges last and overrides everything — use sparingly, only for renderer-specific properties not in the typed Node fields.

**Cache:**
- Always set `SchemaFingerprint` in `GeneratorContext` — without it, cache keys collide across schema versions.
- Bump `RendererVersion` in `amis/renderer.go` whenever the renderer output format changes.
- `PermFP` is opaque — provided by `PolicyEvaluator.ComputeFingerprint()`. Never compute it yourself.

**Security:**
- Permission-gated fields must be absent (not hidden) — the generator enforces this when `f.Permission` is set. Never add `Hidden: true` to permission-gate a node.
- `IsPlatformAdmin()` bypass is in `authz` middleware only — never replicate in generator or plugin code.

---

## 20. Common Pitfalls

**Import cycle:**
Generator importing `compiler` or `expression` causes a cycle. All bridging goes through `adapt`.

**Filter bar always empty:**
`buildFilterBar` gates on `f.Searchable`. If the filter bar is missing, check that at least one list field has `Searchable: true` in the `generator.FieldDef`.

**Dark mode breaks AMIS styling:**
AMIS has no built-in dark CSS. Using `theme("dark")` in the SDK init switches to `dark-` prefix which has zero CSS rules. Fix: use `ThemeConfigFor("dark")` which sets `DarkMode: true` and keeps the `cxd` prefix, then apply `html.dark` + CSS custom property overrides in the web shell.

**Cache collision across tenants:**
`RendererContext.GenCtx.TenantID` must be set — the cache key includes a SHA-256 of the tenant UUID. If `TenantID` is zero, all tenants share one cache entry.

**Widget tree not deterministic:**
`Generate()` must be a pure function. Map iteration order is non-deterministic in Go — avoid iterating over maps to build the tree. Use the ordered `schema.Fields` slice.

**Unknown NodeKind in renderer:**
The AMIS renderer returns an error for unknown `NodeKind` values. The conformance suite catches this. Add new `NodeKind` cases to the renderer before registering them in `registry/builtin.go`.

**`def.PageKind` vs `sduictx.ViewMode`:**
The legacy `def.PageKind` type is a compiler concept. The SDUI generator uses `sduictx.ViewMode`. They overlap but are not the same type. `adapt.FromCompiled` bridges between them.

---

## 21. Minimal Working Examples

### Render a list view in a test

```go
import (
    "testing"

    "github.com/google/uuid"

    "awo.so/awo/sdui/amis"
    "awo.so/awo/sdui/generator"
    "awo.so/awo/sdui/renderer"
    "awo.so/awo/sdui/sduictx"
)

func TestInvoiceListView(t *testing.T) {
    schema := generator.EntitySchema{
        Name:        "finance_invoice",
        Title:       "Invoice",
        PluralTitle: "Invoices",
        ListURL:     "/api/v1/finance/invoices",
        Fields: []generator.FieldDef{
            {Name: "number", Label: "Number", FieldType: "data",
             InList: true, InForm: true, InDetail: true},
            {Name: "status", Label: "Status", FieldType: "select",
             InList: true, InForm: true, InDetail: true,
             Searchable: true},
        },
    }

    ctx, _ := sduictx.NewGeneratorContext(
        uuid.New(), &openViewer{}, "finance_invoice",
        sduictx.ViewModeList, "amis",
    ).WithSchemaFingerprint("fp1").Build()

    g := generator.New()
    tree, err := g.Generate(schema, ctx)
    if err != nil {
        t.Fatal(err)
    }

    r := amis.New()
    rctx := renderer.ApplyLocale(renderer.RendererContext{GenCtx: ctx}, "en-US")
    out, err := r.Render(tree, rctx)
    if err != nil {
        t.Fatal(err)
    }

    // out.AMISSchema is map[string]any ready for JSON serialisation
    _ = out.AMISSchema
}

type openViewer struct{}
func (v *openViewer) TenantID() uuid.UUID       { return uuid.New() }
func (v *openViewer) Roles() []string           { return []string{"admin"} }
func (v *openViewer) IsPlatformAdmin() bool     { return true }
func (v *openViewer) HasPermission(string) bool { return true }
```

### Full engine construction (production)

```go
eng := engine.New(engine.Options{
    Generator:         generator.New(),
    Validator:         validation.New(),
    Layout:            layout.New(),
    Renderers:         map[string]renderer.Renderer{amis.RendererID: amis.New()},
    DefaultRendererID: amis.RendererID,
    Cache:             sdui_cache.New(redisClient),
})

// In HTTP handler:
gSchema := adapt.FromCompiled(compiledES)
rctx := renderer.ApplyLocale(renderer.RendererContext{GenCtx: genCtx}, locale)
resp, err := eng.Handle(ctx, engine.Request{
    Ctx:         genCtx,
    Schema:      gSchema,
    RendererID:  "amis",
    RendererCtx: rctx,
})
```

---

## Legacy Notes

The following files in `awo/sdui/` are empty stubs retained for package identity only. Their functionality has been superseded by the packages documented above:

| File | Superseded by |
|---|---|
| `sdui/generator.go` | `sdui/generator/`, `sdui/engine/`, `api/sdui/` |
| `sdui/label.go` | `def.DeriveLabel()`, `adapt.labelFor()` |
| `sdui/builder.go` | `sdui/generator/` + `sdui/amis/` pipeline |
| `sdui/schema.go` | `sdui/widget/node.go` (typed IR) + `sdui/amis/` (renderer) |
| `sdui/nav.go` | `api/sdui.Handler.nav()` endpoint |
| `api/handler/sdui.go` | `api/sdui.Handler` (nav + all views) |
