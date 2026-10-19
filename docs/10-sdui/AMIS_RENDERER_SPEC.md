# Amis Renderer Specification

**Classification:** Specification — Tier 1
**Owner:** `10-sdui/AMIS_RENDERER_SPEC.md`
**Status:** Frozen at v1.0 (ADR-006)
**Package:** `awo.so/awo/sdui/amis`

---

## Purpose

This document specifies the `awo/sdui/amis` package — the renderer that converts a WidgetTree (`[]*widget.Node`) into amis JSON (`map[string]any`) ready for serialization and delivery to the browser.

The amis renderer is one possible renderer for the WidgetTree IR. Its inputs are always `widget.Node` values; it MUST NOT accept raw field definitions or entity definitions directly.

---

## 1. ADR-006 Rationale

The WidgetTree IR (defined in `awo/sdui/widget`) decouples page generation from rendering. The amis renderer translates the IR to amis-specific JSON. Any future renderer (React Native, PDF, accessibility tree) MUST implement the same translation contract against the same WidgetTree IR.

---

## 2. Renderer Contract

> **Corrected 2026-10-19 (consistency gate, `tasks.md` R.21):** the interface shown below previously omitted the `ctx RendererContext` parameter and returned `map[string]any` instead of the real typed `RenderedOutput`. It is also defined in `awo.so/awo/sdui/renderer`, not `awo.so/awo/sdui/amis` — `sdui/amis` *implements* this interface, it does not declare it. Verified directly against `sdui/renderer/renderer.go` this pass.

```go
// Package: awo.so/awo/sdui/renderer

// Renderer converts a WidgetTree root node into target-specific output.
// sdui/amis is the one concrete implementation today; RenderedOutput is a
// typed union (not map[string]any) so a future non-JSON renderer (e.g. PDF)
// can implement this same interface without reshaping the contract.
type Renderer interface {
    ID() string
    Version() string
    // Render translates root and all its descendants into target-specific output.
    Render(root *widget.Node, ctx RendererContext) (RenderedOutput, error)
}
```

The default AMIS implementation is `amis.DefaultRenderer` (package `awo.so/awo/sdui/amis`). It handles all `NodeKind` constants defined in `awo/sdui/widget`. Unknown `NodeKind` values MUST cause `Render` to return an error — they MUST NOT be silently dropped or rendered as empty divs.

---

## 3. NodeKind → amis type Mapping

| NodeKind | amis type | Notes |
|----------|-----------|-------|
| `NodePage` | `page` | Top-level page container. `body` = rendered children. |
| `NodeForm` | `form` | `api` set from `Node.DataSource.URL`. `body` = rendered children. |
| `NodeList` | `crud2` | amis v2 CRUD. `api` from `DataSource`. Columns from children. |
| `NodeSection` | `group` | Visual grouping. `label` from `Node.Label`. Children rendered into `body`. |
| `NodeTabs` | `tabs` | Each child `NodeSection` becomes one tab item. |
| `NodeTable` | `table` | Inline sub-table. `source` from `DataSource.URL`. |
| `NodeDialog` | `dialog` | Modal dialog. `title` from `Node.Label`. Body = children. |
| `NodeButton` | `button` | `label`, `actionType`, `level` from `ActionNode`. |
| `NodeText` | `input-text` | `name`, `label`, `required`, `disabled`, `maxLength` from Node. |
| `NodeTextArea` | `textarea` | `rows` set by Props (`rows: 2` for SmallText, `rows: 4` for LongText). |
| `NodeNumber` | `input-number` | `precision` from Props. `min`, `max` if set. |
| `NodeSelect` | `select` | `options` (static) or `source` (DataSource URL). `multiple` from Props. |
| `NodeDate` | `input-date` | `format: "YYYY-MM-DD"`. |
| `NodeDateTime` | `input-datetime` | `format: "YYYY-MM-DDTHH:mm:ssZ"`. |
| `NodeSwitch` | `switch` | Boolean toggle. |
| `NodeEditor` | `json-editor` | Full JSON editor. |
| `NodeField` | `input-text` | Generic; used for DynamicLink dual-field rendering. |

---

## 4. Common Property Translations

The renderer applies the following translations from `widget.Node` typed fields to amis JSON properties:

| Node Field | amis JSON Key | Condition |
|-----------|--------------|-----------|
| `Node.Name` | `name` | Always |
| `Node.Label` | `label` | Always |
| `Node.Required` | `required: true` | When `true` |
| `Node.ReadOnly` | `disabled: true` | When `true` |
| `Node.Hidden` | omit from output | When `true` — do not emit the node |
| `Node.DataSource.URL` | `api` or `source` | Form/List: `api`; Select/Table: `source` |
| `Node.DataSource.Method` | `api.method` | Default: `"GET"` |
| `Node.DataSource.SendOn` | `api.sendOn` | Empty = always fetch |
| `Node.DataSource.LabelField` | `labelField` | Select nodes |
| `Node.DataSource.ValueField` | `valueField` | Select nodes; default `"id"` |
| `Node.Props` | merged into output | Renderer-specific overrides |
| `Node.Actions` | `actions` | Action button list |
| `Node.ID` | `id` | When non-empty |

`Node.Props` values MUST be merged last — they MUST override all generated defaults. This is the intended extension point for renderer-specific behaviour not expressible in typed fields.

---

## 5. Action Node Translation

Each `widget.ActionNode` becomes an amis `button` schema object:

```json
{
  "type": "button",
  "label": "Submit for Approval",
  "actionType": "ajax",
  "level": "primary",
  "api": "POST:/api/v1/entities/finance_invoice/${id}/submit"
}
```

| ActionNode Field | amis Key | Notes |
|-----------------|---------|-------|
| `Label` | `label` | Display text |
| `ActionType` | `actionType` | `"submit"` / `"dialog"` / `"link"` / `"ajax"` |
| `Level` | `level` | `"primary"` / `"default"` / `"warning"` / `"danger"` |
| `Href` | `link` | Only for `actionType: "link"` |
| `API` | `api` | Only for `actionType: "ajax"` |

---

## 6. List View Column Generation

For `NodeList`, the renderer reads `Node.Children` as column definitions. Each child `Node` produces one column object:

```json
{
  "name": "number",
  "label": "Invoice Number",
  "type": "text"
}
```

Column amis types are determined by the child `NodeKind`:

| NodeKind | Column type |
|----------|------------|
| `NodeText`, `NodeField` | `text` |
| `NodeNumber` | `tpl` with number formatting |
| `NodeDate` | `date` with `format: "YYYY-MM-DD"` |
| `NodeDateTime` | `datetime` |
| `NodeSwitch` | `status` |
| `NodeSelect` | `mapping` (maps option values to labels) |

---

## 7. Form API Configuration

For `NodeForm`, the amis `api` is built from `Node.DataSource`:

- **Create view**: `POST:{DataSource.URL}`
- **Edit view**: `PATCH:{DataSource.URL}/${id}`

The `initApi` (used to populate the edit form) is set to `GET:{DataSource.URL}/${id}`.

---

## 8. Schema Caching

Rendered amis JSON MUST be cached in Redis. The cache key is:

```
page:{entity_qualified_name}:{view}:{viewer_roles_hash}:{tenant_id}
```

TTL: 5 minutes.

Cache is invalidated on:
- `ActionRuntime.InvalidateCache(ctx, entityName)` call
- Feature flag change affecting SDUI elements
- Permission assignment change for the tenant

The viewer roles hash is an SHA-256 of the sorted role list, truncated to 16 hex characters. This ensures that permission-gated elements (which differ per viewer role set) are cached separately per role set.

---

## 9. amis SDK Version Constraint

**Corrected 2026-10-19 (ADR-026, `docs/00-overview/DECISION_REGISTER.md`):** the amis renderer produces JSON for `amis@6.7.0`, the version the served frontend actually loads at runtime from `unpkg.com` (`web/dist/index.html`) — confirmed present in this exact version since the project's first commit and continuously working since. This is **not** the vendored bundle in `web/sdk/` (which is actually `amis@6.13.0`, not 6.7.0, and has no live route or build step referencing it) and **not** `web/pages/index.html`'s `amis@3.6.6` (also unreferenced by any served route or build step). Both `web/sdk/` and `web/pages/index.html` are retired, abandoned-scaffold artifacts, not the pinned version — see ADR-026 and `tasks.md` **R.18** for the full evidence trail. The SDK version MUST NOT be changed without a full compatibility audit of all generated schemas.

Amis breaking changes that require renderer updates:
- Component type renames
- Required property additions
- Data binding expression syntax changes

---

## 10. Normative Requirements

- `Renderer.Render()` MUST return an error for any unknown `NodeKind` value.
- `Node.Hidden: true` MUST result in the node being omitted from output entirely — not rendered with `hidden: true`.
- `Node.Props` MUST be merged last and MUST override computed defaults.
- The renderer MUST NOT import `awo/def`, `awo/registry`, or `awo/compiler` — it receives only `widget.Node`.
- Rendered schemas MUST be cached per `{entity}:{view}:{roles_hash}:{tenant_id}`.
- The renderer MUST produce deterministic output for the same input tree.

---

## References

- `awo/sdui/amis/renderer.go` — Renderer interface and DefaultRenderer
- [`10-sdui/WIDGET_TREE_SPEC.md`](WIDGET_TREE_SPEC.md) — Node struct and NodeKind constants
- [`10-sdui/SDUI_FIELD_WIDGET_MAP.md`](SDUI_FIELD_WIDGET_MAP.md) — FieldType → NodeKind mapping
- ADR-006 in [`00-overview/DECISION_REGISTER.md`](../00-overview/DECISION_REGISTER.md)
