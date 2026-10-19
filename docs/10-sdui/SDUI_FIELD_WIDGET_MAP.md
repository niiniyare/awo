# SDUI Field-Widget Mapping

**Classification:** Reference — Tier 1
**Owner:** `10-sdui/SDUI_FIELD_WIDGET_MAP.md`
**Status:** Frozen at v1.0

---

## Purpose

This document is the exhaustive mapping from `FieldType` constants to `NodeKind` constants (WidgetTree) and amis widget types.

---

## Field Type → NodeKind → amis Mapping

> **Corrected 2026-10-19 (consistency gate, `tasks.md` R.21):** three rows below were wrong against the real `fieldTypeToNodeKind` switch in `sdui/generator/generator.go` (verified this pass, not assumed) — `currency`, `multi_select`, and `link` are marked below. This table is also **not exhaustive** against that function: `rich_text`→`NodeRichText`, `tree_link`→`NodeTreeSelect`, `duration`→`NodeDuration`, `color`→`NodeColor`, `signature`→`NodeSignature`, and `file`/`image`/`attach`→`NodeFileUpload` are real, current mappings not listed here — adding them is a content-completion task, not performed in this pass. Separately, **list-view columns use a different, simpler mapping function** (`fieldTypeToColumnNodeKind`), which collapses `multi_select`, `link`, and `tree_link` all to `NodeSelect` for column display specifically — the table below describes the *form-field* mapping only; do not assume it applies uniformly to list columns too.

| FieldType | NodeKind | amis type | Notes |
|-----------|----------|-----------|-------|
| `FieldTypeData` | `NodeText` | `input-text` | MaxLen applies to amis `maxLength` |
| `FieldTypeSmallText` | `NodeTextArea` | `textarea` | rows=2 |
| `FieldTypeLongText` | `NodeTextArea` | `textarea` | rows=4 |
| `FieldTypeInt` | `NodeNumber` | `input-number` | `precision: 0` |
| `FieldTypeFloat` | `NodeNumber` | `input-number` | `precision: 6` |
| `FieldTypeCurrency` | **`NodeMoney`** (corrected — was `NodeNumber`) | `input-number` | `precision: 4`, prefix: currency symbol |
| `FieldTypeBool` | `NodeSwitch` | `switch` | |
| `FieldTypeDate` | `NodeDate` | `input-date` | `format: YYYY-MM-DD` |
| `FieldTypeDateTime` | `NodeDateTime` | `input-datetime` | `format: YYYY-MM-DDTHH:mm:ssZ` |
| `FieldTypeTime` | `NodeDateTime` | `input-time` | `format: HH:mm` |
| `FieldTypeSelect` | `NodeSelect` | `select` | Static options from `FieldDef.Options` |
| `FieldTypeMultiSelect` | **`NodeMultiSelect`** (corrected — was `NodeSelect` + `multiple: true`) | `select` | `multiple: true` |
| `FieldTypeNamingSeries` | `NodeText` | `input-text` | `disabled: true` on edit; empty on create |
| `FieldTypeJSON` | `NodeEditor` | `json-editor` | |
| `FieldTypeLink` | **`NodeLookup` when a `DataSource` is configured, `NodeSelect` otherwise** (corrected — was unconditionally `NodeSelect`) | `select` | `searchable: true`, server-side search via `CompiledLookup.SearchURL` |
| `FieldTypeLinkList` | `NodeSelect` | `select` | `multiple: true`, server-side search |
| `FieldTypeDynamicLink` | `NodeField` | Two fields: `input-text` (type) + `input-text` (name) | |

---

## Field Constraint → Widget Property Mapping

| Constraint | Widget Property |
|-----------|----------------|
| `Required: true` | `required: true` |
| `Immutable: true` | `disabled: true` in Edit view |
| `Sensitive: true` | Omitted from List view; password-type in Form if applicable |
| `Searchable: true` | Enables trigram search in List filter |
| `MaxLen` | `maxLength` on text inputs |
| `Min` | `min` on number inputs |
| `Max` | `max` on number inputs |
| `Options` | Static `options` array on select |

---

## List View Column Exclusions

The following fields are excluded from auto-generated list view columns:

| Exclusion Reason | Fields |
|----------------|--------|
| Always excluded | `id`, `tenant_id`, `custom_fields` |
| Excluded by type | `FieldTypeLongText` (too wide), `FieldTypeJSON` (not renderable as column) |
| Excluded by flag | `Sensitive: true` (hidden from list by default) |

---

## References

- [`10-sdui/WIDGET_TREE_SPEC.md`](WIDGET_TREE_SPEC.md) — NodeKind definitions
- [`10-sdui/AMIS_RENDERER_SPEC.md`](AMIS_RENDERER_SPEC.md) — amis type rendering
- [`01-entity/FIELD_TYPES_REFERENCE.md`](../01-entity/FIELD_TYPES_REFERENCE.md) — FieldType definitions
