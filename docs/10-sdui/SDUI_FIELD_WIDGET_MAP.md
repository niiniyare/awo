# SDUI Field-Widget Mapping

**Classification:** Reference — Tier 1
**Owner:** `10-sdui/SDUI_FIELD_WIDGET_MAP.md`
**Status:** Frozen at v1.0

---

## Purpose

This document is the exhaustive mapping from `FieldType` constants to `NodeKind` constants (WidgetTree) and amis widget types.

---

## Field Type → NodeKind → amis Mapping

| FieldType | NodeKind | amis type | Notes |
|-----------|----------|-----------|-------|
| `FieldTypeData` | `NodeText` | `input-text` | MaxLen applies to amis `maxLength` |
| `FieldTypeSmallText` | `NodeTextArea` | `textarea` | rows=2 |
| `FieldTypeLongText` | `NodeTextArea` | `textarea` | rows=4 |
| `FieldTypeInt` | `NodeNumber` | `input-number` | `precision: 0` |
| `FieldTypeFloat` | `NodeNumber` | `input-number` | `precision: 6` |
| `FieldTypeCurrency` | `NodeNumber` | `input-number` | `precision: 4`, prefix: currency symbol |
| `FieldTypeBool` | `NodeSwitch` | `switch` | |
| `FieldTypeDate` | `NodeDate` | `input-date` | `format: YYYY-MM-DD` |
| `FieldTypeDateTime` | `NodeDateTime` | `input-datetime` | `format: YYYY-MM-DDTHH:mm:ssZ` |
| `FieldTypeTime` | `NodeDateTime` | `input-time` | `format: HH:mm` |
| `FieldTypeSelect` | `NodeSelect` | `select` | Static options from `FieldDef.Options` |
| `FieldTypeMultiSelect` | `NodeSelect` | `select` | `multiple: true` |
| `FieldTypeNamingSeries` | `NodeText` | `input-text` | `disabled: true` on edit; empty on create |
| `FieldTypeJSON` | `NodeEditor` | `json-editor` | |
| `FieldTypeLink` | `NodeSelect` | `select` | `searchable: true`, server-side search via `CompiledLookup.SearchURL` |
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
