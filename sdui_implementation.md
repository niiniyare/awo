# SDUI Implementation Architecture — Parts 12–20

**Audience:** Principal Engineers and Senior Engineers implementing the SDUI framework.
**Prerequisite:** Parts 1–11 of the SDUI Architecture Blueprint are approved and frozen. This document does not revisit those decisions.
**Status:** Design review. All decisions here are binding for implementation.

---

## Part 12 — Package Architecture

### Package Inventory

The `awo/sdui/` tree is organized into three concentric zones: the stable core (frozen at v1.0), the extension surface (safe to evolve), and internal implementation details (unexported). The distinction matters because stable core packages will have breaking-change policies enforced through code review gates.

---

#### `awo/sdui/widget`

**Responsibility:** Defines the Widget IR — the canonical, renderer-independent tree of nodes and all types associated with it.

**Public API surface:** `Node`, `NodeKind`, `DataSource`, `ActionDef`, `ValidationRule`, `LayoutHint`, `ExpressionRef`, `ThemeToken`, and all enumerated constants for node kinds, field types, and layout modes. The `Walk` and `WalkDepth` traversal functions.

**Dependency rules:** Imports nothing from `awo/sdui/`. May import `awo/def` for field type constants referenced in node metadata, and standard library only. Must never import `generator`, `renderer`, `amis`, or any external rendering library.

**Zone:** Stable core. No changes without a deprecation cycle.

---

#### `awo/sdui/generator`

**Responsibility:** Transforms an `EntityDefinition` and a `GeneratorContext` into a Widget IR tree.

**Public API surface:** `Generator` interface, `GeneratorContext` struct, `NewFieldBuilder`, `NewLayoutBuilder`, `NewPageBuilder`, `NewDashboardBuilder`, `BuilderOptions`. The `Generate` function as the primary entry point.

**Dependency rules:** Imports `awo/sdui/widget`, `awo/sdui/expression`, `awo/sdui/validation`, `awo/def`, `awo/auth`, `awo/compiler`. Must never import `renderer`, `amis`, or any renderer-specific package.

**Zone:** Stable core.

---

#### `awo/sdui/renderer`

**Responsibility:** Defines the `Renderer` interface and shared rendering utilities usable by all renderer implementations.

**Public API surface:** `Renderer` interface, `RendererContext` struct, `RenderFunc` type, `NodeRendererMap` type, `ActionRenderer` interface, `LayoutRenderer` interface. Utility functions: `WalkAndRender`, `CollectActions`, `FlattenLayout`.

**Dependency rules:** Imports `awo/sdui/widget` only. Must never import `amis`, `generator`, or any concrete renderer package.

**Zone:** Stable core.

---

#### `awo/sdui/amis`

**Responsibility:** Implements the AMIS renderer — translates the Widget IR into AMIS-compatible JSON schema maps.

**Public API surface:** `AMISRenderer` struct implementing `renderer.Renderer`, `NewAMISRenderer(opts AMISOptions)`, `AMISOptions` for configuration.

**Dependency rules:** Imports `awo/sdui/widget`, `awo/sdui/renderer`. Must never import `generator`. Imports `awo/sdui/expression` only for expression-to-JS-string translation.

**Zone:** Extension surface. AMIS schema evolution is expected.

---

#### `awo/sdui/registry`

**Responsibility:** Maintains the global, read-only-after-bootstrap registry of widget kinds, renderers, and field renderer overrides.

**Public API surface:** `Register`, `RegisterRenderer`, `RegisterFieldRenderer`, `LookupNodeKind`, `LookupRenderer`, `LookupFieldRenderer`, `Validate`. The `WidgetRegistration` and `RendererRegistration` structs.

**Dependency rules:** Imports `awo/sdui/widget`, `awo/sdui/renderer`. Must never import `generator` or `amis`.

**Zone:** Stable core.

---

#### `awo/sdui/cache`

**Responsibility:** Manages the three-level cache for compiled widget trees and renderer output, including key construction, TTL management, and singleflight coordination.

**Public API surface:** `Cache` interface, `CacheKey` struct, `ComputeKey`, `NewRedisCache`, `NewNoopCache`. `Singleflight` wrapper.

**Dependency rules:** Imports `awo/sdui/widget`, standard library, `go-redis/v8`. Must never import `generator`, `renderer`, or `amis`.

**Zone:** Extension surface.

---

#### `awo/sdui/context`

**Responsibility:** Defines `GeneratorContext` propagation helpers and context key types for SDUI-specific values carried through `context.Context`.

**Public API surface:** `WithGeneratorContext`, `GeneratorContextFromContext`, `WithRendererContext`, `RendererContextFromContext`. Context key constants.

**Dependency rules:** Imports `awo/sdui/widget`, `awo/auth`, standard library only. Must never import `generator` or `renderer`.

**Zone:** Stable core.

---

#### `awo/sdui/expression`

**Responsibility:** Parses, validates, and translates SDUI expression strings into renderer-agnostic `ExpressionRef` nodes; provides renderer-specific serializers.

**Public API surface:** `Parse(raw string) (ExpressionRef, error)`, `Validate(expr ExpressionRef) error`, `AMISSerializer`, `NoopSerializer` (for PDF/print renderers). `ExpressionRef` is owned by `widget` but `expression` provides the construction logic.

**Dependency rules:** Imports `awo/sdui/widget`, standard library. Must never import `amis`, `renderer`, or `generator`.

**Zone:** Extension surface. Expression language may gain capabilities.

---

#### `awo/sdui/validation`

**Responsibility:** Builds and validates `ValidationRule` sets for widget nodes; detects rule conflicts at generation time.

**Public API surface:** `Builder`, `Required()`, `RequiredOn(expr)`, `MinLength`, `MaxLength`, `Pattern`, `Range`, `Custom(name, params)`, `Validate(rules []ValidationRule) error`.

**Dependency rules:** Imports `awo/sdui/widget`, `awo/sdui/expression`, standard library. Must never import `renderer`, `generator`, `amis`.

**Zone:** Extension surface.

---

#### `awo/sdui/layout`

**Responsibility:** Computes and validates layout trees, detects circular nesting, and resolves column grid constraints before the widget tree is finalized.

**Public API surface:** `Compute(root *widget.Node) (*widget.Node, error)`, `Validate(root *widget.Node) error`, `LayoutOptions`.

**Dependency rules:** Imports `awo/sdui/widget` only. Must never import `renderer`, `generator`, `amis`.

**Zone:** Extension surface.

---

#### `awo/sdui/dashboard`

**Responsibility:** Defines dashboard panel types, the dashboard layout model, and the dashboard generator that composes panels from multiple entity sources.

**Public API surface:** `DashboardDefinition`, `PanelDefinition`, `PanelKind`, `DashboardGenerator`, `NewDashboardGenerator`.

**Dependency rules:** Imports `awo/sdui/widget`, `awo/sdui/generator`, `awo/def`, `awo/sdui/registry`. Must never import `amis` directly.

**Zone:** Extension surface.

---

#### `awo/sdui/plugins`

**Responsibility:** Defines the plugin interfaces and registration mechanisms for all SDUI extension points.

**Public API surface:** All nine plugin interfaces (see Part 17), `RegisterPlugin`, `PluginRegistry`, `ExecutionOrder`, `PluginMetadata`.

**Dependency rules:** Imports `awo/sdui/widget`, `awo/sdui/renderer`, `awo/def`, standard library. Must never import `amis` or `generator` internals.

**Zone:** Extension surface.

---

#### `awo/sdui/internal/`

**Responsibility:** Implementation details shared across `sdui/` subpackages that must not be exported. Includes node pool allocation, tree diff utilities, fingerprint computation, and render dispatch internals.

**Public API surface:** None. Unexported to all external callers.

**Dependency rules:** May import any `awo/sdui/` package. Must not create cycles.

**Zone:** Internal only.

---

### Dependency Graph (strict)

The arrows represent permitted imports. A package must never import anything above its layer in this hierarchy.

```
widget
  ↑
expression  validation  layout
  ↑              ↑         ↑
context     registry
  ↑              ↑
generator        renderer
  ↑                  ↑
dashboard           amis
  ↑
plugins
  ↑
cache
  ↑
[sdui HTTP handler — outside sdui/ package]
```

The rule is absolute: no package at a lower layer may import a package at a higher layer. `widget` has zero `sdui/` imports. `renderer` never imports `generator`. `amis` never imports `generator`. This separation is what makes the architecture multi-renderer.

---

### Stable Core vs Extension Surface

**Stable core (frozen at v1.0):** `widget`, `generator`, `renderer`, `registry`, `context`. Breaking changes to these packages require a versioned deprecation path and team sign-off.

**Extension surface (safe to evolve):** `amis`, `cache`, `expression`, `validation`, `layout`, `dashboard`, `plugins`. These packages may gain new capabilities between minor versions. Backward compatibility is expected but not guaranteed across major versions.

**Internal:** `internal/`. No stability guarantee. Refactor freely.

---

## Part 13 — Generator Architecture

### Single Generator vs Multiple Specialised Generators

The decision is: one `Generator` interface with one concrete `EntityGenerator` implementation, parameterized by view mode in `GeneratorContext`. Specialised sub-builders (`ListBuilder`, `FormBuilder`, `DetailBuilder`) exist internally, but they are created and composed by the single `EntityGenerator` — they are not separate `Generator` implementations.

The rationale: view mode (list, form, detail, dashboard) does not change what data the generator needs from `EntityDefinition` — it changes which fields are included and how they are arranged. The same entity permissions, locale context, schema fingerprint, and tenant context apply regardless of view mode. Splitting into four generators creates four copies of the permission gating logic, four copies of locale resolution, and four points of divergence for GeneratorContext propagation — all of which have been failure modes in prior SDKs.

Dashboard generation is the one exception. `DashboardGenerator` is a separate type in `awo/sdui/dashboard` because it does not operate on a single `EntityDefinition` — it composes across multiple entities and does not fit the `EntityGenerator` contract.

---

### Generator Stages

Generation proceeds through six ordered stages. Each stage is a discrete transformation. Stage failures abort further stages; partial outputs are not returned.

**Stage 1 — Context resolution.** The `GeneratorContext` is validated and completed: missing locale defaults are resolved, the schema fingerprint is verified against the current `CompiledSchema`, and the viewer's permission fingerprint is computed and stored on context. This stage fails fast if the `CompiledSchema` is stale or the viewer context is missing.

**Stage 2 — Field selection.** All fields from the `EntityDefinition` are evaluated against view mode and permissions. Fields are either included or excluded — there is no "present but hidden" state in the widget tree. A field is excluded if the viewer lacks the read permission associated with it, if the field is marked `ExcludeFrom` for the current view mode, or if a pre-generation plugin excludes it. The output of this stage is an ordered `[]FieldDef` that the remaining stages operate on.

**Stage 3 — Field node construction.** Each selected field is converted to a `*widget.Node` by the `FieldBuilder`. This includes resolving the field's NodeKind from its `FieldType`, applying the field's label via locale resolution, constructing the field's `ValidationRule` set, constructing `ExpressionRef` nodes for VisibleOn/HiddenOn/DisabledOn/RequiredOn, and attaching `DataSource` for Link and Select fields. Field-level plugins may override the constructed node before it is added to the tree.

**Stage 4 — Layout computation.** The `LayoutBuilder` arranges field nodes into layout nodes (`NodeSection`, `NodeGrid`, `NodeTabs`) according to the entity's layout declarations. Layout is resolved through the `layout` package which validates for circular nesting and impossible column configurations. The output is a structured subtree rooted at a layout node.

**Stage 5 — Page construction.** The `PageBuilder` wraps the layout subtree in the appropriate page node (`NodeList`, `NodeForm`, `NodeDetail`), attaches actions (from `ActionDef` entries that pass permission gating), attaches the entity's `DataSource` (list endpoint, create endpoint, etc.), and sets page-level metadata: title, breadcrumb, empty-state configuration.

**Stage 6 — Post-generation transforms.** Post-generation plugins execute in deterministic order. Each plugin receives the complete widget tree and may return a modified tree. The final tree is validated by the `layout` package before the stage completes. If any post-generation plugin returns an error, the entire generation fails — there is no partial-success path.

---

### Builder Pattern

Builders are the internal construction API. They are not exported from the `generator` package — only the `Generator` interface and `GeneratorContext` are exported.

`FieldBuilder` is responsible for one field node. It receives a `FieldDef` and a `GeneratorContext`. Its methods set label, validation, expressions, data source, and metadata. It produces one `*widget.Node`.

`LayoutBuilder` receives a `[](*widget.Node)` and layout declarations from the entity schema. It creates section, grid, and tab nodes, assigns children, and delegates final validation to the `layout` package.

`PageBuilder` receives the layout root node and `GeneratorContext`. It creates the view-mode-specific page node, attaches actions, sets data source configuration, and returns the page root node.

`DashboardBuilder` (in `awo/sdui/dashboard`, not in `generator`) receives multiple `PanelDefinition` entries, each backed by a separate entity generator invocation, and assembles them into a dashboard root node.

---

### GeneratorContext Design

`GeneratorContext` is a value type (struct, not interface) passed by pointer through the generator pipeline. It is created once per request before generation begins and is not mutated after Stage 1 completes. Stages 2–6 read it; they do not write to it.

Fields on `GeneratorContext`:

- `ViewerContext auth.ViewerContext` — the authenticated viewer; used for all permission checks.
- `ViewMode ViewMode` — enumerated: List, Form, Detail, Dashboard.
- `Readonly bool` — when true, the entire tree is generated in read-only mode; no input widgets are generated.
- `Locale string` — BCP 47 locale tag for label and message resolution (e.g., `"en-US"`, `"fr-FR"`).
- `TenantID uuid.UUID` — the current tenant; used for tenant-specific label overrides and datasource URL construction.
- `SchemaFingerprint string` — the `CompiledSchema` fingerprint at the time of context creation; used in cache key computation and validated against the live schema in Stage 1.
- `PermissionFingerprint string` — computed in Stage 1 from the viewer's role set and the entity's permission identifiers. Stable for the same viewer+entity combination regardless of request ordering.
- `RequestID string` — for trace correlation in logs and spans.
- `RendererTarget string` — the intended renderer (e.g., `"amis"`, `"flutter"`, `"pdf"`). Generators must not use this to conditionally produce different nodes — it exists solely for cache key computation.
- `EntityName string` — the entity being generated.
- `Metadata map[string]any` — escape hatch for plugin-supplied context data. Plugins read this; they do not mutate it during generation.

**Propagation rules:** `GeneratorContext` is constructed by the SDUI request handler and passed to `generator.Generate`. Sub-builders receive a pointer to the same context. No sub-builder copies and modifies the context — they all read from the shared instance. The `Readonly` flag propagates to all builders automatically: `FieldBuilder` checks it before selecting input-capable node kinds; it never needs to be threaded explicitly.

**Locale resolution:** Labels are resolved in `FieldBuilder` during Stage 3. The `FieldDef` carries a label key (the field's Go name by convention). `GeneratorContext.Locale` is used to look up the localized string from the label registry. If the locale is not found, the fallback is `"en-US"`. If the key is not found, the raw field name is used. This never fails the generation.

**Permission gating:** In Stage 2, each field's associated read permission is checked against `ViewerContext`. The check calls `PolicyEvaluator.CanPerform` — this is the same evaluator used in the HTTP middleware. There is no secondary permission model in the generator. If `CanPerform` returns an error, Stage 2 fails and generation aborts.

**View mode field selection:** Each `FieldDef` may carry `ShowInList`, `ShowInForm`, `ShowInDetail` boolean hints. The generator enforces these alongside permissions. A `NamingSeries` field that is auto-generated is excluded from Form view (create mode) but included in Detail view.

---

### Extension Points in the Generator

Two extension points exist within the generator pipeline. Both are registered via the `plugins` package.

**Pre-generation transform (before Stage 2):** A plugin may inspect and modify the field list before field selection runs. It receives the full `[]FieldDef` from the entity definition and the `GeneratorContext`. It returns a modified `[]FieldDef`. This is the correct place for plugins that add virtual fields (computed fields, derived columns) or suppress fields based on tenant configuration.

**Field node override (between Stage 3 field construction and Stage 4):** A plugin registered for a specific entity and field name may replace the `*widget.Node` produced by `FieldBuilder`. The plugin receives the generated node and the `GeneratorContext` and returns a replacement node. The replacement must have the same NodeKind or a registered compatible kind — the registry validates this at plugin registration time.

There is no extension point inside Stage 4 (layout) or Stage 5 (page construction) during their internal execution. Post-generation plugins (Stage 6) are the correct hook for structural tree modifications.

---

## Part 14 — Renderer Architecture

### The Renderer Interface

The `renderer.Renderer` interface is not simply `Render(root *widget.Node) (map[string]any, error)`. That signature is adequate only for AMIS. The full interface must support:

- Multiple output formats (JSON map for AMIS, structured objects for Flutter, binary for PDF)
- A typed output that does not force all renderers to return `map[string]any`
- A renderer context that carries request-time configuration (theme, locale, renderer-specific options)

The interface is:

```
Renderer interface {
    Target() string
    Render(ctx RendererContext, root *widget.Node) (RenderedOutput, error)
}

RenderedOutput interface {
    MarshalJSON() ([]byte, error)
    Target() string
}
```

`Target()` returns the renderer's identifier string (`"amis"`, `"flutter"`, `"pdf"`). This is matched against `GeneratorContext.RendererTarget` for cache key computation and dispatch.

`RenderedOutput` is an interface rather than `map[string]any` so that the PDF renderer can return a byte buffer, the Flutter renderer can return a typed schema struct, and the AMIS renderer can return a JSON map — all satisfying the same contract. The HTTP handler calls `MarshalJSON()` only when writing the response. Internal code works with the typed output.

---

### RendererContext

`RendererContext` is a value type passed to `Render`. It carries everything the renderer needs that is not in the widget tree:

- `Locale string` — for renderer-specific label formatting (date formats, number formats)
- `Theme ThemeTokenSet` — resolved theme tokens for this tenant/request
- `RendererTarget string` — for self-validation
- `TenantID uuid.UUID` — for datasource URL construction (base URL may be tenant-scoped)
- `RequestID string` — for span correlation
- `Options map[string]any` — renderer-specific options passed through without coupling the interface to any renderer's details

`RendererContext` is created by the SDUI request handler immediately before renderer invocation. It is not the same as `GeneratorContext` — generation and rendering are separate phases with separate contexts.

---

### Common Rendering Utilities in `renderer/`

The `renderer` package provides utilities that are correct for any renderer and would otherwise be duplicated:

`WalkAndRender` traverses the widget tree depth-first and dispatches each node to the renderer's registered node handler. It handles nil nodes, enforces max depth (configurable, default 20), and collects errors without aborting on field-level failures (configurable).

`CollectActions` extracts all `ActionDef` nodes from the tree. Used by renderers that render actions separately from the main layout (AMIS toolbar, Flutter FAB).

`FlattenLayout` converts nested layout nodes to a flat list with depth metadata. Used by renderers that do not support nested layouts natively.

These utilities are pure functions operating on `*widget.Node`. They produce generic intermediate structures. Renderers use them; they do not depend on any renderer's output format.

---

### Node Renderer Dispatch

The AMIS renderer (and any renderer) must dispatch from `NodeKind` to a rendering function without a massive switch statement and without reflection. The dispatch mechanism is the `NodeRendererMap` — a map from `NodeKind` to `RenderFunc` that is constructed at renderer initialization time.

`RenderFunc` is: `func(ctx RendererContext, node *widget.Node, recurse RenderFunc) (RenderedOutput, error)`.

The `recurse` parameter is the dispatch function itself, passed so that layout nodes can invoke it on their children without calling back into the renderer object. This enables stateless render functions that are safe to call concurrently.

Renderers register their `RenderFunc` entries in `NewAMISRenderer` (or equivalent). Unknown NodeKinds are handled by the renderer's configured unknown-node strategy (see Part 18).

Extension of the dispatch table happens at renderer construction time by consulting the `registry` package for field renderer overrides registered for this renderer target. This means a plugin can register a custom render function for a NodeKind+renderer pair and it will be incorporated into the dispatch table when the renderer is instantiated.

---

### Action Renderer Design

Actions in the widget tree are `*widget.Node` nodes with `Kind == NodeAction`. They carry the action identifier, the HTTP method and path (for remote actions), the label (localized), confirmation requirements, and permission identifier.

The renderer is responsible for deciding where and how actions appear. AMIS renders list actions in a toolbar and row-level actions in a column. Flutter renders them as FAB or contextual buttons. PDF renders none of them.

The `renderer` package provides `CollectActions(root *widget.Node) []ActionNode` which separates page-level, record-level, and bulk actions by their `ActionScope` attribute set during generation. Renderers use this to place actions correctly in their output format.

Action rendering is a separate render pass in AMIS: after the main schema is rendered, actions are rendered and injected into the toolbar and operations columns. This two-pass approach is AMIS-specific and must not influence the widget tree structure.

---

### Layout Renderer Design

Layout nodes (`NodeSection`, `NodeGrid`, `NodeTabs`, `NodeColumns`) carry `LayoutHint` structs that describe intent, not renderer-specific CSS. `LayoutHint` fields:

- `Columns int` — number of columns (1–12 grid scale)
- `Gap Spacing` — enumerated: None, Small, Medium, Large
- `Align Alignment` — enumerated: Start, Center, End, Stretch
- `Collapsible bool` — whether the section can be collapsed
- `DefaultCollapsed bool`

The AMIS renderer translates these to AMIS-specific `body` arrays with `className` strings (`"grid grid-cols-2 gap-4"`). The Flutter renderer translates them to `Row`/`Column` widget descriptors. The PDF renderer uses them for table column widths.

The key invariant: `LayoutHint` values are semantic and portable. The renderer owns the translation. No AMIS class names, Flutter widget names, or CSS strings appear in the widget IR.

---

### Expression Rendering

`ExpressionRef` nodes in the widget tree represent conditional display/behavior logic. They are renderer-agnostic by design — they describe what the condition should compute, not how.

`ExpressionRef` carries:
- `Raw string` — the raw expression string as authored (e.g., `"status == 'draft'"`)
- `Kind ExpressionKind` — VisibleOn, HiddenOn, DisabledOn, RequiredOn
- `ParsedTokens []Token` — optional pre-parsed AST for renderers that compile expressions

AMIS renders expressions as JavaScript expression strings embedded in `visibleOn`, `hiddenOn`, `disabledOn`, `required` fields. The `amis` package provides the `expression.AMISSerializer` which converts `ExpressionRef.Raw` to the AMIS JS format (field references are prefixed with `data.`).

Flutter renders expressions as Dart evaluation stubs or sends them to a server-side evaluation endpoint.

PDF renderers use `expression.NoopSerializer` — conditions are ignored and all fields render unconditionally (correct for static output).

The `expression` package validates the raw expression string at generation time (Stage 3). If validation fails, the field node is generated without the expression and a warning is logged. Generation does not abort for an invalid expression — the field renders as always-visible.

---

### Validation Rendering

`ValidationRule` sets on widget nodes describe constraints. Renderers translate them to renderer-specific validation mechanisms.

AMIS: `required`, `minLength`, `maxLength`, `pattern`, `validations`, `validationErrors`. Custom rules become AMIS `validations` entries with custom message strings.

Flutter: validation functions in the form field widget descriptors.

PDF: validation is omitted entirely.

The `renderer` package provides no shared validation translation — each renderer implements its own because the output formats are incompatible. The widget IR validation rules are the single source of truth. If the AMIS validation format changes, only the `amis` package changes.

---

### Theme Rendering

`ThemeTokenSet` is a key-value map of semantic token names to values (`"color.primary"`, `"spacing.base"`, `"font.size.body"`). It is resolved by the SDUI request handler from the tenant's theme configuration before `RendererContext` is constructed.

AMIS translates `ThemeTokenSet` to CSS custom properties injected as a style block at the top of the rendered schema. No AMIS-specific token names appear in the widget IR.

Flutter translates them to `ThemeData` overrides.

The renderer is responsible for all theme translation. The widget tree carries no color values, no font sizes, no spacing values — only semantic token references on nodes that need them (which is rare; most theming is global).

---

### Future Renderer Plug-In

A future renderer (Flutter, PDF, CLI) registers by:

1. Implementing the `renderer.Renderer` interface.
2. Calling `registry.RegisterRenderer(r)` in its package `init()`.
3. Providing `RenderFunc` entries for all `NodeKind` values it supports.
4. Declaring an unknown-node strategy.

No existing code changes. The SDUI request handler resolves the renderer by calling `registry.LookupRenderer(target)` using the `RendererTarget` from the request. If no renderer is registered for the target, the handler returns 400.

The `amis` package is not special-cased anywhere in the dispatch path. It is one renderer among future peers.

---

## Part 15 — Widget Registry

### Registration Lifecycle

The widget registry is a global singleton, consistent with `def.Register`. Registration must occur only in package `init()` functions. The registry is validated at bootstrap by the SDUI bootstrap function (called from `awo/bootstrap`) and transitions to read-only. Any registration attempt after bootstrap panics — this is deliberate fail-fast behavior to prevent runtime mutation.

This mirrors the `def.Register` precedent and is justified for the same reasons: Go's `init()` ordering is deterministic within a binary, compile-time registration is verifiable, and runtime plugin loading is deferred to post-v1.0.

---

### What Is Registered

The registry stores three kinds of entries:

**`WidgetRegistration`** — keyed by `NodeKind`. Contains: the `NodeKind` constant, a display name for error messages, a set of compatible `FieldType` values (used by the generator to validate field→node assignments), a list of renderer targets that support this node kind (used to warn at bootstrap if a registered renderer is missing handlers for registered nodes).

**`RendererRegistration`** — keyed by renderer target string. Contains: the `renderer.Renderer` implementation, the set of `NodeKind` values it handles, and the unknown-node strategy (`StrategyError`, `StrategyOmit`, `StrategyFallback`).

**`FieldRendererOverride`** — keyed by `(NodeKind, rendererTarget)`. Contains a `RenderFunc` that replaces the renderer's default handler for that node kind. Used by plugins and modules to override rendering for specific widgets in specific renderers.

---

### Validation at Registration

At `RegisterWidget(r WidgetRegistration)`:
- Duplicate `NodeKind` is a panic with a descriptive message including the package that registered first.
- `CompatibleFieldTypes` must not be empty.
- `NodeKind` value must be in the valid range (prevents integer alias mistakes).

At `RegisterRenderer(r RendererRegistration)`:
- Duplicate target string is a panic.
- The `Renderer` implementation must not be nil.

At bootstrap validation (`registry.Validate()`):
- Every `WidgetRegistration` that lists a renderer target must have a corresponding `RendererRegistration` for that target.
- Every `RendererRegistration` must declare handlers for all `NodeKind` values registered with it in `WidgetRegistration` entries, or must have `StrategyFallback` configured.
- Any `FieldRendererOverride` that references an unregistered `NodeKind` or unregistered renderer target is a fatal bootstrap error.

If `Validate()` returns any errors, bootstrap aborts. Misconfigured SDUI is not a degraded-ok condition.

---

### Lookup API

`LookupNodeKind(kind NodeKind) (WidgetRegistration, bool)` — O(1) map lookup. Returns false if not found.

`LookupRenderer(target string) (RendererRegistration, bool)` — O(1) map lookup.

`LookupFieldRenderer(kind NodeKind, target string) (RenderFunc, bool)` — O(1) map lookup on a two-key composite. Returns false if no override is registered; the renderer uses its default handler.

All lookups are safe for concurrent access after bootstrap because the map is not written after bootstrap completes. No read locks are needed post-bootstrap. A `sync.RWMutex` is held only during the `init()` registration phase and released permanently when bootstrap calls `registry.Seal()`.

---

### Custom Widget Registration

A module author creating a new `NodeKind` (e.g., `NodeKindRichText`, `NodeKindGanttBar`) registers it by calling `registry.RegisterWidget` in their package `init()`. They must also register at least one `FieldRendererOverride` for each renderer target they intend to support, or provide a renderer implementation if the new kind requires one.

Custom `NodeKind` values must be in a reserved range defined by the registry (e.g., `NodeKind >= 1000`). Built-in kinds occupy `1–999`. This prevents collision without a central authority.

**Versioning:** `WidgetRegistration` carries a `TargetFrameworkVersion string` (semver). At bootstrap, the registry checks that the running framework version satisfies each registration's declared target. If a custom widget was built against `v1.2` but the running framework is `v1.0`, it is rejected at bootstrap with a clear error.

---

### Unknown Widget Handling

Unknown widget handling is renderer-specific, not registry-specific. The renderer's `StrategyError` causes `Render` to return an error for any node with an unregistered `NodeKind`. `StrategyOmit` silently drops the node. `StrategyFallback` renders a placeholder node (AMIS renders a static-text node showing the field name; Flutter shows a disabled text widget).

The default strategy for production renderers must be `StrategyError`. `StrategyFallback` is for development tools only. This enforces the invariant that incomplete renderer implementations are caught before production deployment.

---

### Concurrency Model

During `init()` registration: `sync.RWMutex` write-locked.
After `registry.Seal()` is called during bootstrap: read access only, no locks needed.
`Seal()` is idempotent. Calling it twice is a no-op.
Any call to `RegisterWidget`, `RegisterRenderer`, or `RegisterFieldRenderer` after `Seal()` panics immediately.

Test isolation concern: tests that need a clean registry must call `registry.Reset()` which is only compiled in `testing` builds (via build tag). `Reset()` clears all registrations and re-opens the registry for registration. This is the only exception to the "no mutation after seal" rule.

---

## Part 16 — SDUI Request Lifecycle

### Complete Request Lifecycle

The SDUI endpoint is `GET /api/v1/sdui/{entity}/{view}`. The `{view}` path segment is one of: `list`, `form`, `detail`, `dashboard`.

---

**Step 1 — HTTP Request arrives.**

The Fiber router matches the SDUI route group. Path parameters `entity` and `view` are extracted. Query parameters `locale` and `renderer` are read (defaulting to `"en-US"` and `"amis"`). If `view` is not a recognized value, a 400 is returned immediately.

**Error handling:** Malformed path parameters → 400, no further processing.

---

**Step 2 — Middleware stack.**

`TenantResolver` runs first: extracts tenant from the host header, validates that the tenant exists and is in `ACTIVE` status, injects `TenantID` into context. If the tenant is not found or not active, 404 is returned. If the tenant lookup fails due to a database error, 503 is returned.

`RequireAuth` runs second: validates the session token, builds `ViewerContext`, injects it via `auth.WithViewer`. If the session is missing or expired, 401 is returned.

**Error handling:** Any middleware failure aborts; no SDUI logic runs.

---

**Step 3 — Permission resolution.**

The handler computes the viewer's `PermissionFingerprint` for the requested entity. This is a stable hash of the sorted intersection of the viewer's roles and the entity's declared permission identifiers. The computation uses the `compiler`'s `CapabilityGrant` set for the entity.

This fingerprint is not used for authorization enforcement — that happens in the entity CRUD middleware. Its purpose is cache key differentiation: two viewers with the same effective permissions on this entity get the same cached widget tree.

**Error handling:** `PolicyEvaluator` error during fingerprint computation → 500. Missing `CompiledSchema` entry for the entity → 404.

---

**Step 4 — CompiledSchema lookup (Level 1 cache: in-process).**

The `CompiledSchema` is an in-process, read-only singleton populated at bootstrap. Lookup is O(1) by entity name with no I/O. The schema fingerprint (a hash of the entity's field definitions, permissions, and layout declarations at compile time) is read from the `CompiledSchema` entry.

This is not really a "cache" — it is the live compiled schema. It does not expire. It is invalidated only by server restart.

**Error handling:** Entity not found in `CompiledSchema` → 404.

---

**Step 5 — Cache key computation.**

The Level 2 and Level 3 cache key is computed as:

`sdui:{entity}:{view}:{renderer}:{locale}:{schema_fingerprint}:{permission_fingerprint}`

All components are URL-safe strings. `schema_fingerprint` ensures that a schema change (server restart with updated entity definitions) automatically invalidates all cached trees. `permission_fingerprint` ensures that viewers with different effective permissions see different trees.

Locale is part of the key because labels are resolved into the tree at generation time. A widget tree generated for `fr-FR` is not reusable for `en-US`.

`renderer` is part of the key because the widget tree is renderer-independent, but cached Level 3 output (rendered JSON) is renderer-specific.

---

**Step 6 — Widget tree cache lookup (Level 2: Redis).**

The handler checks Redis for a cached serialized widget tree. The key is the full cache key from Step 5 minus the renderer component (widget trees are renderer-independent):

`sdui:tree:{entity}:{view}:{locale}:{schema_fingerprint}:{permission_fingerprint}`

**TTL:** 5 minutes. This is intentionally short because permission changes (role assignment) must propagate within an acceptable window. The 5-minute window is a UX tradeoff, not a security boundary — the entity CRUD endpoints enforce authorization independently.

**Cache hit path:** Deserialize the widget tree, skip to Step 10.

**Cache miss path:** Continue to Step 7.

**Singleflight:** Before the Redis lookup, the handler checks a process-local `singleflight.Group` keyed by the tree cache key. If another goroutine is already generating this tree, the current goroutine blocks and reuses the result. This prevents thundering herd on cache miss for popular entities.

**Error handling:** Redis unavailable → log warning, treat as cache miss, continue. Cache deserialization failure → log error, treat as cache miss.

---

**Step 7 — Generator invocation with GeneratorContext.**

A `GeneratorContext` is constructed with all fields populated from the request context, viewer context, path parameters, query parameters, and the computed fingerprints. `generator.Generate(ctx, entityName, genCtx)` is called.

**Plugin extension point:** Pre-generation plugins registered for this entity execute before Stage 2.

**Error handling:** Generator Stage 1 failure (stale schema fingerprint, invalid viewer context) → 500. Generator Stage 2 failure (permission evaluation error) → 500. Generator Stage 3–6 failure → 500. No partial generation is returned.

---

**Step 8 — Widget tree construction.**

The generator runs all six stages (Part 13). The output is a `*widget.Node` tree.

A post-generation validation pass runs unconditionally after Stage 6: the `layout` package's `Validate` function checks for circular nesting and impossible configurations. If validation fails → 500. This is a programming error, not a user error.

**Plugin extension point:** Post-generation plugins execute as Stage 6 (Part 13).

---

**Step 9 — Widget tree cache write (Level 2).**

The widget tree is serialized and written to Redis with the tree cache key and a 5-minute TTL. This write is best-effort — if it fails, the request continues. The failure is logged at Warn level.

Write is performed in a goroutine only if the serialized tree exceeds a size threshold (configurable, default 4KB). Below the threshold, write is synchronous to avoid goroutine leak risk from fire-and-forget patterns.

---

**Step 10 — Renderer selection.**

`registry.LookupRenderer(rendererTarget)` retrieves the renderer. The renderer target comes from the `renderer` query parameter, defaulting to `"amis"`.

**Error handling:** Unknown renderer target → 400.

---

**Step 11 — Rendered output cache lookup (Level 3: Redis).**

The handler checks Redis for cached renderer output. Key:

`sdui:rendered:{entity}:{view}:{renderer}:{locale}:{schema_fingerprint}:{permission_fingerprint}`

**TTL:** 10 minutes. Renderer output is more expensive to produce than tree generation, so a longer TTL is appropriate.

**Singleflight:** Same pattern as Level 2 — a process-local `singleflight.Group` keyed by the rendered cache key.

**Cache hit path:** Return cached JSON directly as the HTTP response.

**Cache miss path:** Continue to Step 12.

**Error handling:** Redis unavailable → log warning, treat as cache miss.

---

**Step 12 — Renderer invocation.**

A `RendererContext` is constructed. `renderer.Render(rendCtx, root)` is called. The result is a `RenderedOutput`.

**Plugin extension point:** Post-render plugins execute on the `RenderedOutput` before caching.

**Error handling:** Renderer returns error → 500. If the renderer panics (caught by a deferred recover in the handler) → log error with stack trace → 500.

---

**Step 13 — Rendered output cache write (Level 3).**

The serialized rendered output is written to Redis with the rendered cache key and a 10-minute TTL. Best-effort; failures are logged at Warn level.

---

**Step 14 — HTTP Response.**

The handler writes the HTTP response: `Content-Type: application/json`, status 200, body is the JSON-serialized `RenderedOutput`. The response includes a header `X-Schema-Fingerprint` for client-side cache validation.

---

### Cache Invalidation

Level 2 and Level 3 caches invalidate automatically when the schema fingerprint changes (server restart with new entity definitions). No explicit invalidation mechanism is required for schema changes.

Permission changes (role assignment/revocation) invalidate caches indirectly: the new `PermissionFingerprint` produces a new cache key, so old cached trees expire naturally within their TTL window.

Explicit invalidation is provided for administrative use: `DELETE /api/v1/sdui/cache/{entity}` clears all Level 2 and Level 3 Redis keys matching `sdui:*:{entity}:*`. This is a platform-admin-only endpoint.

---

## Part 17 — Extension Pipeline

### Design Philosophy

Extension points are designed for tree decoration, not tree replacement. Plugins are guests — they may augment and modify; they may not take over the pipeline. Every extension point is invoked synchronously within the request lifecycle (no goroutines spawned by plugin execution). Plugins that block excessively will be visible in the generator/renderer duration histograms (Part 19).

---

### Extension Point 1 — Pre-Generation Transform

**When:** After `GeneratorContext` construction, before Stage 2 (field selection).

**Input:** The entity's `[]FieldDef` slice from the `CompiledSchema`, and the `GeneratorContext` (read-only pointer).

**Output:** A modified `[]FieldDef`. The plugin may reorder, add (virtual fields), or remove fields. It may not modify the `GeneratorContext`.

**Registration:** `plugins.RegisterPreGeneration(entityName string, p PreGenerationPlugin, order int)`. The `entityName` may be `"*"` to apply to all entities.

**Execution order:** Deterministic. Plugins for a specific entity name execute before wildcard plugins. Within each group, plugins execute in ascending `order` value. Equal `order` values are disambiguated by plugin registration order (which is `init()` order, which is deterministic within a binary). This determinism is a hard requirement — non-deterministic plugin ordering would make caching incorrect.

**Allowed side effects:** Logging only. No database writes, no network calls, no cache writes.

**Forbidden:** Modifying `GeneratorContext`, accessing the widget registry, calling other plugins.

---

### Extension Point 2 — Field Node Override

**When:** After Stage 3 for a specific field, before the node is added to the layout.

**Input:** The `*widget.Node` produced by `FieldBuilder`, the `FieldDef`, and the `GeneratorContext` (read-only).

**Output:** A replacement `*widget.Node`. Must have the same or a compatible `NodeKind`.

**Registration:** `plugins.RegisterFieldOverride(entityName, fieldName string, p FieldOverridePlugin)`. Both `entityName` and `fieldName` are required; wildcard field overrides are not supported (too broad).

**Execution order:** At most one override per `(entity, field)` pair. Registering a second override for the same pair is a bootstrap error.

**Allowed side effects:** Logging only.

**Forbidden:** Replacing the NodeKind with an incompatible kind (validated by registry at override registration time), accessing datasources, making HTTP calls.

---

### Extension Point 3 — Post-Generation Transform

**When:** Stage 6 of generation, after the complete widget tree is assembled.

**Input:** The root `*widget.Node` of the complete tree, and the `GeneratorContext` (read-only).

**Output:** A modified root `*widget.Node`. The plugin may restructure the tree freely.

**Registration:** `plugins.RegisterPostGeneration(entityName string, p PostGenerationPlugin, order int)`.

**Execution order:** Same deterministic ordering as Extension Point 1.

**Allowed side effects:** Logging only.

**Forbidden:** Mutating `GeneratorContext`, spawning goroutines, any I/O.

---

### Extension Point 4 — Pre-Renderer Transform

**When:** After widget tree cache write (Step 9), before renderer invocation (Step 12). Unlike Post-Generation, this executes for every renderer invocation including cache hits on the tree (because the tree came from cache but renderer output was a miss).

**Input:** The root `*widget.Node`, the `RendererContext` (read-only), and the renderer target string.

**Output:** A modified root `*widget.Node`. The plugin may add renderer-target-specific metadata to nodes via `node.Metadata` map without violating the renderer-independent tree invariant (metadata is opaque to the generator).

**Registration:** `plugins.RegisterPreRenderer(rendererTarget string, p PreRendererPlugin, order int)`. The `rendererTarget` must match a registered renderer.

**Execution order:** Deterministic, same pattern.

**Allowed side effects:** Logging only.

**Forbidden:** Reading or modifying `RendererContext`, adding renderer-specific schema fragments (node metadata is opaque, not AMIS JSON).

---

### Extension Point 5 — Custom Node Renderer

**When:** During renderer dispatch (Step 12), when the dispatch map encounters a registered `FieldRendererOverride`.

**Input:** `RendererContext`, `*widget.Node`, and the `recurse RenderFunc` for rendering children.

**Output:** A `RenderedOutput` for this node.

**Registration:** `registry.RegisterFieldRenderer(kind NodeKind, target string, fn RenderFunc)` in `init()`. This is actually a registry registration, not a plugin registration, because it must be available at renderer construction time.

**Execution order:** Only one `RenderFunc` per `(NodeKind, target)` pair. Duplicates are a bootstrap error.

**Allowed side effects:** None. Must be a pure function of inputs.

**Forbidden:** Accessing global state, logging (use structured attributes on the span instead), calling back into the generator.

---

### Extension Point 6 — Post-Render Transform

**When:** After renderer invocation (Step 12), before rendered output cache write (Step 13).

**Input:** The `RenderedOutput` and the `RendererContext` (read-only).

**Output:** A modified `RenderedOutput`. The plugin may add renderer-specific metadata, wrap the output, or modify values.

**Registration:** `plugins.RegisterPostRender(rendererTarget string, p PostRenderPlugin, order int)`.

**Execution order:** Deterministic.

**Allowed side effects:** Logging only.

**Forbidden:** Making the output invalid for the renderer's schema, changing the renderer target of the output.

---

### Extension Point 7 — Dashboard Panel Plugin

**When:** During `DashboardGenerator.Build()`, after panel definitions are resolved but before the dashboard node is assembled.

**Input:** `[]PanelDefinition` and the `GeneratorContext` (read-only).

**Output:** A modified `[]PanelDefinition`. Plugins may add, remove, or reorder panels. Panels may be sourced from custom data (not EntityDefinition-backed) if the plugin registers a corresponding `DataSource`.

**Registration:** `plugins.RegisterDashboardPanel(dashboardName string, p DashboardPanelPlugin, order int)`.

**Execution order:** Deterministic.

**Allowed side effects:** Logging only.

**Forbidden:** Generating widget trees inline (must use `generator.Generate` for entity-backed panels, which is itself an extension of the normal pipeline).

---

### Extension Point 8 — Validation Plugin

**When:** During Stage 3 (field node construction) by `FieldBuilder`, after built-in validation rules are generated.

**Input:** `[]ValidationRule` (the built-in rules), `FieldDef`, and `GeneratorContext` (read-only).

**Output:** A modified `[]ValidationRule`. Plugins may append custom rules.

**Registration:** `plugins.RegisterValidation(entityName, fieldName string, p ValidationPlugin)`. Both entity and field are required.

**Execution order:** One validation plugin per `(entity, field)`. Duplicate registrations are a bootstrap error.

**Allowed side effects:** Logging only.

**Forbidden:** Removing built-in `Required` rules (these are authoritative from the field definition).

---

### Extension Point 9 — Layout Transform Plugin

**When:** During Stage 4 (layout computation), after the `LayoutBuilder` produces the initial layout tree but before the `layout.Validate()` pass.

**Input:** The layout root `*widget.Node` and the `GeneratorContext` (read-only).

**Output:** A modified layout root `*widget.Node`. May restructure sections, move fields between sections, add decorative nodes.

**Registration:** `plugins.RegisterLayoutTransform(entityName string, p LayoutTransformPlugin, order int)`.

**Execution order:** Deterministic.

**Allowed side effects:** Logging only.

**Forbidden:** Introducing circular nesting (caught by subsequent `layout.Validate()` and returns an error attributed to the plugin). Adding layout nodes that are not registered widget kinds.

---

### Forbidden Extension Operations

**Direct EntityDefinition mutation from a plugin:** `EntityDefinition` is read-only after `def.Register` in `init()`. The `CompiledSchema` derived from it is read-only after bootstrap. No plugin interface provides write access to either. A plugin attempting to cast to a concrete type to mutate fields is a programming error discoverable in code review.

**Renderer injection into generator:** No plugin extension point provides a renderer instance to a generator-phase plugin. The generator must remain unaware of renderers. Any plugin at Extension Points 1–3 that holds a renderer reference and calls it during generation is violating the architecture — this cannot be statically enforced but is enforced in code review and documented as a forbidden pattern.

**Schema fingerprint bypass:** The cache key includes the `SchemaFingerprint`. A plugin may not modify the fingerprint. The fingerprint is computed from the live `CompiledSchema` by the handler before any plugin runs. It is read-only in `GeneratorContext`.

**Permission bypass:** No plugin extension point can grant or suppress permissions. The viewer's permissions are evaluated in Stage 2 using `PolicyEvaluator`. Plugins at Stage 2 receive the already-filtered field list — they cannot add back fields that were excluded by permission evaluation.

---

## Part 18 — Error Architecture

### Error Categories

**Configuration errors** are detected at bootstrap during `registry.Validate()` or `compiler` validation. They prevent server startup entirely. No request is ever served with a misconfigured SDUI stack.

**Fatal errors** abort the current request with a 500 response. They indicate programming errors or infrastructure failures. Examples: permission evaluator failure, schema compilation state corruption, renderer panic.

**Field errors** degrade the output: one field fails to generate or render, and the rest of the tree proceeds. The response is returned with the partial tree and a degraded flag in the response metadata.

**Recoverable errors** allow the request to succeed with a logged warning. Examples: cache write failure, plugin fallback, expression validation failure (field rendered without conditional logic).

---

### `SDUIError` Type Hierarchy

The root type is `SDUIError` which implements `error` and carries:
- `Code SDUIErrorCode` — enumerated; used for metrics labels
- `Message string` — human-readable, safe to log
- `EntityName string`
- `FieldName string` (optional)
- `NodeKind NodeKind` (optional)
- `Stage GeneratorStage` or `RenderStage` (optional)
- `Cause error` — wrapped underlying error; `errors.As` compatible
- `Severity ErrorSeverity` — Fatal, Field, Recoverable, Configuration

Sentinel errors are defined as `var ErrUnknownWidget = &SDUIError{Code: CodeUnknownWidget, ...}`. All callers use `errors.Is` or `errors.As` — never type switches.

---

### Specific Error Scenarios

**1. Unknown widget (NodeKind not in registry)**

Category: Configuration error if detected at bootstrap (a registered renderer lists a NodeKind it handles, but that NodeKind has no `WidgetRegistration`). Fatal error at request time if somehow bypassed (cannot happen if bootstrap validation is complete). If renderer `StrategyError` is set → Fatal, 500. `StrategyOmit` → Recoverable, field omitted, Warn log. `StrategyFallback` → Recoverable, placeholder rendered, Warn log.

**2. Invalid layout (circular nesting, impossible column configuration)**

Category: Configuration error if detected at bootstrap by the compiler. Field error at request time if introduced by a layout plugin (after the widget tree cache). The `layout.Validate()` pass attributes the error to the plugin that last modified the layout. Returns 500 if the root node is invalid; degrades if only a section is invalid (the section is replaced with a flat list of its children).

**3. Duplicate widget registration**

Category: Configuration error. Detected in `registry.RegisterWidget` in `init()`. Panics with a message identifying both registering packages. Server does not start.

**4. Invalid renderer (returns nil output)**

Category: Fatal. The renderer contract is that `Render` returns a non-nil `RenderedOutput` or a non-nil error. Returning `(nil, nil)` is a programming error in the renderer. Detected by the handler with a nil check; returns 500 and logs an Error with the renderer target and entity.

**5. Missing DataSource (NodeList/NodeSelect with no DataSource)**

Category: Configuration error if detected at bootstrap by the compiler report pass. Field error at request time if a plugin creates a list/select node without a data source. The field is omitted (Recoverable) with a Warn log identifying the plugin.

**6. Circular layout (section contains itself via plugin transform)**

Category: Field error. The `layout.Validate()` pass detects it. The offending section is replaced with a flat list of its children, the plugin that introduced the cycle is identified in the log entry (via the GeneratorContext's plugin execution trace), and a Warn is emitted. The rest of the page renders.

**7. Permission evaluation failure**

Category: Fatal. `PolicyEvaluator.CanPerform` returned an error (not false — an error). This is an infrastructure failure (Casbin database unreachable, etc.). Aborts with 500. The request is not served with degraded permission enforcement. Logged at Error level with the permission identifier and entity.

**8. Expression validation failure**

Category: Recoverable. The field is generated without the failing expression (rendered as always-visible/always-enabled). A Warn log identifies the entity, field, and raw expression string. Generation continues.

**9. Renderer failure (panic recovery)**

Category: Fatal. The renderer's `Render` call is wrapped in a `defer recover()` in the handler. A panic is converted to a `SDUIError` with `Severity: Fatal`, the stack trace is logged at Error level, and a 500 is returned. Partial renderer output is discarded.

**10. Validation rule conflict**

Category: Field error (detected at generation time by `validation.Validate(rules)`). Example conflict: `Required()` and `RequiredOn(expr)` are both set (ambiguous). The `Required()` rule wins (stricter), the conflicting rule is dropped, a Warn log is emitted. Generation continues.

---

### Logging Rules

**Error level:** Permission evaluation failure, renderer panic, schema state corruption, any error that results in a 500. Always includes `entity`, `view_mode`, `request_id`, `tenant_id`, and the full causal chain via `errors.As` unwrapping.

**Warn level:** Cache write failure, plugin fallback, expression validation failure, degraded output (field omitted), validation rule conflict.

**Info level:** Cache hit (with cache level), generation complete (with node count and duration), renderer complete.

**Debug level:** Per-field generation details, per-plugin execution, cache key computation. Debug is not enabled in production by default.

---

## Part 19 — Observability

### Metrics

All metrics use the `awo_sdui_` prefix to namespace them within the broader `awo_` metrics space.

---

**`awo_sdui_generator_duration_seconds` (Histogram)**

Measures: Time from `generator.Generate` call entry to return.

Labels: `entity` (entity name), `view_mode` (list/form/detail/dashboard), `cache_hit` (bool as string "true"/"false" — false when generation ran, always false for this metric since it only fires when generation runs). `result` (success/error).

Buckets: 5ms, 10ms, 25ms, 50ms, 100ms, 250ms, 500ms, 1s, 2.5s.

---

**`awo_sdui_renderer_duration_seconds` (Histogram)**

Measures: Time from `renderer.Render` call entry to return.

Labels: `entity`, `view_mode`, `renderer_target` (amis/flutter/pdf), `result` (success/error).

Buckets: Same as generator.

---

**`awo_sdui_cache_operations_total` (Counter)**

Measures: Cache hits and misses per level.

Labels: `level` (2/3 — Level 1 is in-process and not measured separately), `entity`, `view_mode`, `operation` (hit/miss/write/write_error).

---

**`awo_sdui_widget_tree_nodes` (Histogram)**

Measures: Number of nodes in the generated widget tree (depth-first count).

Labels: `entity`, `view_mode`.

Buckets: 5, 10, 20, 50, 100, 200, 500.

This metric is the primary indicator of tree complexity growth over time as modules add fields.

---

**`awo_sdui_plugin_duration_seconds` (Histogram)**

Measures: Time each plugin extension point takes.

Labels: `plugin_name` (registered plugin identifier), `extension_point` (pre_gen/field_override/post_gen/pre_render/custom_node/post_render/dashboard_panel/validation/layout_transform), `result` (success/error/fallback).

This metric is critical for identifying plugins that degrade request latency.

---

**`awo_sdui_errors_total` (Counter)**

Measures: Error frequency by category.

Labels: `error_code` (the `SDUIErrorCode` string), `entity`, `severity` (fatal/field/recoverable/configuration).

---

### Tracing

All spans use OpenTelemetry. Attribute names follow the `sdui.*` convention.

**`sdui.request` (root span)**

Created at the start of the SDUI handler. Ends when the HTTP response is written.

Required attributes: `sdui.entity`, `sdui.view_mode`, `sdui.renderer_target`, `sdui.locale`, `tenant.id`, `request.id`, `sdui.schema_fingerprint`, `sdui.permission_fingerprint`.

---

**`sdui.permission_resolve` (child of `sdui.request`)**

Created before cache key computation. Ends after `PermissionFingerprint` is computed.

Attributes: `sdui.permission_fingerprint`, `sdui.viewer.role_count` (number of roles), `sdui.entity_permission_count`.

---

**`sdui.cache.lookup` (child of `sdui.request`, one per cache level)**

Created at each cache level check. Attribute `sdui.cache.level` (2 or 3), `sdui.cache.result` (hit/miss/error).

---

**`sdui.generator` (child of `sdui.request`)**

Created at `generator.Generate` entry. Ends at return.

Attributes: `sdui.generator.field_count_input` (fields in entity definition), `sdui.generator.field_count_output` (fields in generated tree, after permission gating and view mode filtering), `sdui.generator.node_count` (total tree nodes), `sdui.generator.stage_count` (number of stages completed before error, if any).

Child spans for each Stage (1–6): `sdui.generator.stage.{n}`.

---

**`sdui.renderer` (child of `sdui.request`)**

Created at `renderer.Render` entry. Ends at return.

Attributes: `sdui.renderer.target`, `sdui.renderer.node_count_processed`, `sdui.renderer.output_bytes`.

---

### Logging

All SDUI log entries must include these structured fields: `entity`, `view_mode`, `renderer_target`, `tenant_id`, `request_id`, `schema_fingerprint`. These are set on a logger derived from the request context at handler entry. All sub-functions receive this logger via context.

Log entries must not include `permission_fingerprint` in any log at Info level or below (it encodes role membership, which is security-sensitive). It may appear in Error-level logs for debugging permission failures.

---

### Profiling

The `awo/observability` pprof endpoint exposes two SDUI-specific profiles:

**`/debug/pprof/sdui_generator`** — CPU profile captured during generator execution. Enabled by a build tag (`sdui_profile`). Not compiled into production builds by default. Used during development to identify hot paths in field selection and layout computation.

**`/debug/pprof/sdui_renderer`** — CPU profile captured during renderer execution.

**Memory allocation tracking:** The generator uses a node pool (`internal/nodepool`) with allocation tracking. When the `sdui_profile` build tag is active, allocation counts per entity+view_mode are reported as a gauge metric `awo_sdui_allocations_per_request`.

**Benchmark suite acceptance thresholds:**

The benchmark suite (in `awo/sdui/bench_test.go`) must include:
- `BenchmarkGenerateList_20Fields` — must complete in under 2ms per operation
- `BenchmarkGenerateForm_20Fields` — must complete in under 2ms per operation
- `BenchmarkRenderAMIS_20Fields` — must complete in under 5ms per operation
- `BenchmarkCacheKey_Compute` — must complete in under 50µs per operation
- `BenchmarkFullPipeline_CacheHit` — must complete in under 1ms per operation (cache hit path)

These thresholds are enforced in CI via `go test -bench -benchtime=5s -benchmem`. Regressions block merge.

---

## Part 20 — Final Architecture Review

### Concern 1 — Cache Key Correctness: Permission Fingerprint

**Severity: Critical**

**Root cause:** The `PermissionFingerprint` is computed as a hash of the viewer's roles intersected with the entity's permission identifiers. This assumes that effective permissions are entirely determined by role membership. If `PolicyEvaluator` applies attribute-based conditions (time-of-day rules, OU-scoped access, custom CEL expressions) then two viewers with identical role sets may have different effective permissions — but the same fingerprint. They would receive the same cached widget tree, which may include fields the second viewer should not see.

**Long-term impact:** As the IAM system matures (ltree OU scoping is already in scope), ABAC conditions will likely be introduced. A cache keying strategy based on role sets alone will silently over-serve cached trees. At the 5-year horizon, this becomes a correctness failure, not just a performance concern.

**Recommended solution:** The `PermissionFingerprint` computation must be delegated to the `PolicyEvaluator` itself via an interface method `ComputeFingerprint(ctx, viewer, entityPermissions) (string, error)`. The default Casbin implementation computes from roles. A future ABAC implementation computes from roles plus evaluated conditions. The SDUI pipeline treats it as an opaque string — it does not know or care how it is computed.

**Fix before implementation:** Yes. The `PolicyEvaluator` interface extension must be designed before the cache layer is built. Retrofitting this after the cache is in production is a correctness bug discovery scenario.

---

### Concern 2 — Plugin Ordering Determinism

**Severity: High**

**Root cause:** Deterministic plugin ordering is declared as a requirement but the mechanism (`order int` with `init()` tiebreaking) has a subtle problem: `init()` execution order between packages in Go is guaranteed within a dependency chain but not between independent packages that both depend on `sdui/plugins`. Two modules that both register plugins with `order: 100` and have no dependency relationship will have `init()` tiebreaking that depends on the linker's symbol ordering, which is not guaranteed to be stable across Go versions.

**Long-term impact:** At the 5-year horizon with many modules, plugin ordering bugs will appear intermittently on Go upgrades. These are the worst kind of bugs — they affect caching correctness (two orderings produce different trees that land in the same cache slot) and are invisible in tests that compile all modules in the same order.

**Recommended solution:** Require that `order` values be unique within the same extension point and entity scope. Registration of a duplicate `order` value is a bootstrap error. Document that `order` values are module-scoped integers (e.g., the Finance module uses 100–199, the Inventory module uses 200–299). Publish a module `order` range registry in `CLAUDE.md`.

**Fix before implementation:** Yes. The registration API must enforce this at the time it is designed.

---

### Concern 3 — GeneratorContext Mutability During Pipeline

**Severity: Medium**

**Root cause:** `GeneratorContext` is a struct passed by pointer and declared read-only after Stage 1. This is a convention, not a language enforcement. A plugin receiving a `*GeneratorContext` can mutate it. Go does not provide a `readonly` qualifier.

**Long-term impact:** A plugin that accidentally or maliciously mutates `GeneratorContext.PermissionFingerprint` or `GeneratorContext.Locale` mid-pipeline produces a widget tree that is cached under the wrong key. This is a correctness failure that is very difficult to debug.

**Recommended solution:** Pass `GeneratorContext` to plugins as a value type (copy), not a pointer. The struct is small enough that copying is not a performance concern. Sub-builders inside the generator that need to modify context (none do under the current design) can maintain their own local state. This change makes mutability physically impossible.

**Fix before implementation:** Yes. The plugin interface signatures must use value types from the beginning.

---

### Concern 4 — Widget Registry Global State and Test Isolation

**Severity: Medium**

**Root cause:** The widget registry is a package-level global. Tests that register widgets in `TestMain` or `init()` will pollute the registry for all tests in the package. Tests that need a clean registry (to test unknown-widget behavior, for example) cannot get one without the `registry.Reset()` mechanism, which is a testing-build-only escape hatch.

**Long-term impact:** As the test suite grows, test isolation failures will appear non-deterministically based on test execution order. `go test -shuffle=on` will surface these intermittently.

**Recommended solution:** Implement `registry.Reset()` gated by `//go:build testing` build tag (not `_test.go` suffix, which does not propagate to imported packages). Document that all widget registration in test packages must be preceded by `registry.Reset()` in `TestMain`. Provide a `registry.NewIsolated()` constructor for tests that need a fully isolated registry — this returns a non-global registry instance that test code can pass explicitly to generator and renderer constructors via dependency injection. The global registry remains for production use; the isolated registry is for tests.

**Fix before implementation:** The `registry.NewIsolated()` design must be in place before the first test is written for SDUI.

---

### Concern 5 — Expression Language Lock-In (AMIS JS vs Portable IR)

**Severity: High**

**Root cause:** `ExpressionRef.Raw` stores the expression as a string authored by the entity or plugin developer. The `expression` package validates and translates it. But the raw format is currently underspecified — if developers author expressions in AMIS JS syntax (`data.status == 'draft'`), the `expression.Parse` function will have to handle AMIS-specific syntax, making it impossible to correctly translate to Flutter Dart or PDF without a full JS parser.

**Long-term impact:** At the 5-year horizon, when the Flutter renderer is implemented, it will find that all existing expressions are AMIS JS strings that cannot be mechanically translated. Every entity definition with conditional display logic will need manual migration.

**Recommended solution:** Define a portable expression DSL for SDUI that is a proper subset of common CEL. The `expression` package owns this DSL. The AMIS renderer translates it to AMIS JS. The Flutter renderer translates it to Dart. The DSL must be defined and documented before any entity definitions use expressions. Example: `status == "draft"` in the DSL becomes `data.status == 'draft'` in AMIS JS and `record.status == "draft"` in Flutter Dart. The DSL is the source of truth; renderer-specific strings are derived artifacts.

**Fix before implementation:** Yes. The expression DSL must be specified before any entity definition authors expressions. This is a pre-freeze requirement.

---

### Concern 6 — NodeGrid Offline Persistence Design Complexity

**Severity: Low**

**Root cause:** `NodeGrid` (editable data grids) mentioned in earlier parts implies inline cell editing with optimistic updates. The request lifecycle in Part 16 covers read (schema generation). Write paths for NodeGrid inline edits go through the existing entity CRUD API. There is no special SDUI write protocol. This is correct but creates a UX complexity: the SDUI schema must embed per-row action endpoints, and the AMIS grid must be configured with the correct save/update URLs.

**Long-term impact:** Offline persistence (service workers, local-first) is not in scope for v1.0 but may be required for mobile Flutter clients. The current design has no opinion on this.

**Recommended solution:** At v1.0, document that NodeGrid requires live connectivity for writes and that the DataSource on NodeGrid nodes carries explicit `saveUrl` and `rowUpdateUrl` fields. Offline support is deferred and will require a separate design pass on the widget IR to add conflict-resolution metadata.

**Fix before implementation:** No. Document the v1.0 limitation and move on.

---

### Concern 7 — Dashboard Separation from EntityDefinition

**Severity: Low**

**Root cause:** Part 12 places `DashboardGenerator` in `awo/sdui/dashboard` as a separate generator that composes across multiple entities. This is architecturally correct. However, there is a question of ownership: who defines which dashboards exist? If dashboards are defined in module code (like entity definitions), they need a registration mechanism similar to `def.Register`. If they are defined dynamically by tenants, they are data, not code.

**Long-term impact:** Without a clear answer, module authors will improvise — some will define dashboards in code, some will store them in the database — leading to two incompatible dashboard sources.

**Recommended solution:** Establish that v1.0 dashboards are code-defined (registered via `dashboard.Register(&MyDashboard)` in `init()`, parallel to `def.Register`). Tenant-configurable dashboards are a post-v1.0 feature requiring a separate persistence model. Enforce this by making `DashboardDefinition` implement a `DashboardDefinition` interface (parallel to `EntityDefinition`) that `dashboard.Register` accepts.

**Fix before implementation:** Yes. The `DashboardDefinition` registration mechanism must be defined before any module author writes a dashboard.

---

### Concern 8 — Multi-Renderer Output Format Differences (Schema Versioning)

**Severity: Medium**

**Root cause:** The AMIS renderer produces JSON schemas that are version-coupled to the AMIS SDK version in `web/sdk/`. When the AMIS SDK is upgraded (a new sdk.js), the rendered output format may need to change. The Level 3 cache (rendered output) does not include the AMIS SDK version in its key. A cached schema generated against AMIS SDK v3.5 will be served to a client running AMIS SDK v3.6 if the cache has not expired.

**Long-term impact:** AMIS SDK upgrades that include breaking schema changes will cause subtle rendering bugs in the TTL window after upgrade. This is a 10-minute window (Level 3 TTL), which is acceptable but surprising.

**Recommended solution:** Include the AMIS SDK version hash in the Level 3 cache key for the AMIS renderer. The SDK version hash is computed at bootstrap from the `web/sdk/sdk.js` file content hash and stored in the `AMISRenderer` instance. When the SDK file changes (deployment), all Level 3 AMIS cache entries are automatically invalidated. Other renderers include their own version token in the same key slot.

**Fix before implementation:** Yes. The cache key computation in Part 16 Step 5 must include a renderer version token field. The token source is renderer-specific.

---

### Concern 9 — Missing Widget Tree Validation Pass

**Severity: High**

**Root cause:** Part 13 describes a post-generation validation pass by the `layout` package that checks for circular nesting and impossible configurations. However, the `layout` package only validates structural layout concerns. There is no validation that: all `DataSource` references point to reachable endpoints, all `ExpressionRef` nodes have valid parsed tokens, all `ActionDef` nodes reference existing entity actions, all `FieldTypeLink` nodes have a registered target entity.

**Long-term impact:** These failures will manifest as runtime errors in the renderer or as AMIS schemas that load but fail at user interaction time. Debugging will be difficult because the error appears in the browser, not in the server log.

**Recommended solution:** Implement a `widget.Validate(root *widget.Node, schema *compiler.CompiledSchema) error` function in the `widget` package. This function performs a depth-first walk and validates all DataSource, ExpressionRef, ActionDef, and Link references against the compiled schema. It is called after Stage 6 post-generation transforms. The compiler report pass (§11 item 6) is the design-time equivalent; `widget.Validate` is the request-time guard.

**Fix before implementation:** Yes. This validation function must exist before any test exercises the full pipeline.

---

### Concern 10 — Error Propagation from Plugins

**Severity: Medium**

**Root cause:** Plugins return errors. The pipeline must decide: fail the entire request, drop the field, or log and continue. Part 18 describes categories, but the plugin interfaces in Part 17 do not specify which error category applies to which extension point. A post-generation plugin that returns an error — is that Fatal or is it Recoverable?

**Long-term impact:** Without a clear per-extension-point error policy, plugin authors will not know how to signal degradation vs hard failure. Users of the ERP will see inconsistent behavior: some plugin failures cause 500, others cause silently degraded UI.

**Recommended solution:** Each plugin interface method signature includes a semantic in its return error: if the plugin wraps its error in `plugins.ErrFatal`, the pipeline treats it as Fatal (500). All other errors are treated as Recoverable (field/section omitted, Warn logged). Plugin authors who do not wrap get Recoverable behavior by default, which is the safe default. This is consistent with the Go error wrapping model and requires no changes to the `SDUIError` type hierarchy.

**Fix before implementation:** Yes. The plugin interface return value semantics must be documented before any plugin is written.

---

### Pre-Implementation Checklist

The following decisions must each have a definitive YES or NO answer from the team before any SDUI implementation code is written. These are binding architectural decisions.

1. **PolicyEvaluator.ComputeFingerprint extension:** Will the `PolicyEvaluator` interface be extended with a `ComputeFingerprint` method before the cache layer is implemented? (Concern 1)

2. **Plugin order uniqueness enforcement:** Will duplicate `order` values within the same extension point and entity scope be a bootstrap error, enforced at registration time? (Concern 2)

3. **GeneratorContext value type for plugins:** Will all plugin interface methods receive `GeneratorContext` as a value (copy), not a pointer? (Concern 3)

4. **Isolated registry for tests:** Will `registry.NewIsolated()` be implemented as part of the registry package before any SDUI tests are written? (Concern 4)

5. **Portable expression DSL:** Will the SDUI expression DSL be formally specified (with BNF or EBNF grammar) before any entity definition uses conditional expressions? (Concern 5)

6. **DashboardDefinition registration interface:** Will `DashboardDefinition` follow the `def.Register` / `init()` pattern with a `dashboard.Register` function, before any module implements a dashboard? (Concern 7)

7. **Renderer version token in cache key:** Will the Level 3 cache key include a renderer version token, and will each renderer be responsible for computing its own version token at instantiation? (Concern 8)

8. **Widget tree validation pass:** Will `widget.Validate(root, schema)` be implemented and called unconditionally after Stage 6 before any pipeline test is considered passing? (Concern 9)

9. **Plugin error propagation semantics:** Will the Recoverable-by-default / `plugins.ErrFatal`-for-hard-failure convention be enforced in all plugin interface documentation and validated in plugin registration (e.g., a linting check)? (Concern 10)

10. **AMIS as the only v1.0 renderer:** Is AMIS the only renderer that will be implemented before the v1.0 kernel freeze, with all other renderers deferred? The answer determines how much of the multi-renderer infrastructure must be tested before freeze.

11. **Dashboard data model (code vs database):** Are all v1.0 dashboards code-defined (registered in `init()`)? If any tenant-configurable dashboards are in scope for v1.0, the persistence model must be designed now.

12. **Widget tree serialization format for Level 2 cache:** Will the widget tree be serialized as JSON, Protobuf, or MessagePack for Redis storage? This decision affects cache key stability across schema changes and deserialization performance.

13. **NodeGrid inline edit write protocol:** Is the v1.0 NodeGrid write path exclusively through the standard entity CRUD API (`PATCH /api/v1/{module}/{resource}/:id`)? If a batch/inline-save protocol is needed, it must be designed as part of SDUI, not retrofitted.

14. **Singleflight scope:** Is the singleflight group process-local only (in-process deduplication), or is distributed singleflight (Redis-based locking across multiple server instances) required for v1.0? Distributed singleflight adds significant complexity.

15. **Error response format for SDUI endpoints:** When SDUI generation or rendering fails, what is the HTTP response body format? It must be consistent with the rest of the API's error response format (defined in `shared/errors/http.go`) and must include enough context for the client to distinguish a degraded response (partial tree, field errors) from a complete failure.
