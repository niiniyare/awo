# Chapter 11 — Forms Framework

> **Volume:** III — Component System
> **Phase:** 2
> **Audience:** Backend Engineers, Web/Mobile Engineers, ERP Implementers
> **Prerequisites:** Chapters 01–10

---

## Table of Contents

- [11.1 Forms Philosophy in SDUI](#111-forms-philosophy-in-sdui)
- [11.2 Form Node Types](#112-form-node-types)
- [11.3 Field Input Types](#113-field-input-types)
- [11.4 Form Layout](#114-form-layout)
- [11.5 Dynamic Forms](#115-dynamic-forms)
- [11.6 Form State Management](#116-form-state-management)
- [11.7 Form Validation](#117-form-validation)
- [11.8 Form Actions](#118-form-actions)
- [11.9 Form Permissions](#119-form-permissions)
- [11.10 ERP-Specific Form Patterns](#1110-erp-specific-form-patterns)

---

## 11.1 Forms Philosophy in SDUI

Forms are the primary mechanism through which ERP users create, modify, and approve business documents. They are also the most complex category of UI in the platform — they involve state management, validation, conditional logic, data binding, permission enforcement, and multi-step workflows, all within a single surface.

Three principles govern AwoERP's forms framework:

**The form is a contract, not a template.** A form definition specifies precisely what the user may provide, what is required, and what business rules constrain their input. This specification is authored in the backend — in the same service that will validate and process the submission. There is no gap between "what the form accepts" and "what the service validates." They are the same definition, expressed at different stages of the same pipeline.

**State lives in the rendering engine; shape lives in the backend.** The backend defines the form's structure — fields, sections, validation rules, conditional visibility, permitted actions. The rendering engine owns the transient runtime state — current field values, dirty flags, touched state, error display. This division means the backend does not need to be aware of in-progress edits; the rendering engine does not need to re-validate against business rules until submission.

**Field types carry business semantics.** A `field.currency_amount` is not a text field that happens to hold money. It is a semantic type that carries currency awareness, locale-sensitive formatting, multi-currency support, and ERP-specific validation (non-negative, maximum precision). The rendering engine maps this semantic type to the most appropriate input experience on its platform. The backend engineer expresses intent; the rendering engine handles realization.

---

## 11.2 Form Node Types

### 11.2.1 Form Container

**`form.container`** is the root of every form. It is a structural node that groups all form elements, declares the submit action, and owns the top-level form state.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `submit_action` | ActionRef | ✅ | — | The action to execute on successful form submission |
| `reset_on_success` | boolean | | `false` | Whether form fields are cleared after successful submission |
| `track_dirty` | boolean | | `true` | Whether to warn the user if they navigate away with unsaved changes |
| `dirty_warning_message` | LocalizedString | | *(platform default)* | The unsaved-changes warning message |
| `auto_save` | AutoSaveConfig | | — | If set, form data is periodically saved as a draft |
| `layout` | `single_column`\|`two_column`\|`custom` | | `single_column` | Default column layout applied to all child sections |

**`AutoSaveConfig`:**
```json
{
  "interval_seconds": 30,
  "save_action": "action-save-draft",
  "indicator_label": { "$i18n": "form.auto_save.indicator" }
}
```

**Events emitted by `form.container`:**

| Event | Payload | When |
|-------|---------|------|
| `on_submit_start` | `{}` | User triggers submit; before validation |
| `on_submit_success` | `{ response: object }` | Submit action completes successfully |
| `on_submit_error` | `{ error: object }` | Submit action fails |
| `on_reset` | `{}` | Form is reset |
| `on_dirty_change` | `{ is_dirty: boolean }` | Form transitions between clean and dirty state |

### 11.2.2 Form Section / Group

**`form.section`** groups related fields under an optional heading. Sections provide visual organization and can be individually collapsible, permissioned, and conditionally visible.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `title` | LocalizedString | | — | Section heading |
| `subtitle` | LocalizedString | | — | Descriptive text below the heading |
| `columns` | integer (1–4) | | *(inherits from form)* | Number of field columns in this section |
| `collapsible` | boolean | | `false` | |
| `default_collapsed` | boolean | | `false` | |
| `divider` | `top`\|`bottom`\|`both`\|`none` | | `top` | Visual separator |
| `padding` | `none`\|`sm`\|`md`\|`lg` | | `md` | |

### 11.2.3 Form Field

All field types share a base set of props that every form field supports. Individual field types extend this base with type-specific props.

**Base field props (inherited by all `field.*` nodes):**

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `label` | LocalizedString | ✅ | — | Field label |
| `help_text` | LocalizedString | | — | Explanatory text shown below the field |
| `placeholder` | LocalizedString | | — | Input hint text (when empty) |
| `required` | boolean\|Binding | | `false` | |
| `read_only` | boolean\|Binding | | `false` | Field is displayed but not editable |
| `disabled` | boolean\|Binding | | `false` | Field is greyed out and not interactive |
| `default_value` | any | | — | Initial value (may be a Binding) |
| `col_span` | integer (1–4) | | 1 | How many grid columns this field occupies |
| `validation_rules` | array of ValidationRule | | `[]` | See Chapter 23 |
| `name` | string | | *(same as node id)* | The key used in the form submission payload |

### 11.2.4 Field Label, Help Text, Error Text

Labels and help text are rendered by the rendering engine using platform conventions — above the field on web, inside the field until focused on some mobile platforms. Error text replaces help text when a validation error is active.

Error text is sourced from two places:
- **Client-side validation errors:** Generated from the `validation_rules` by the rendering engine's validation executor. Displayed immediately when the field is blurred (on-blur validation) or when the form is submitted.
- **Server-side validation errors:** Returned in the submit action's error response, mapped to specific field `name` values. Displayed after a failed submission.

The server-side error mapping contract: the submit endpoint must return errors in the following structure for them to be automatically mapped to form fields:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "field_errors": [
      { "field": "vendor_id", "message": "Vendor account is suspended" },
      { "field": "line_items[0].unit_price", "message": "Price cannot exceed approved budget" }
    ]
  }
}
```

The `field` value uses dot notation and bracket indexing to address nested fields within the form's submission payload. The rendering engine maps these errors to the corresponding field nodes by matching the `name` prop.

---

## 11.3 Field Input Types

### 11.3.1 Text Input

**`field.text`** — Single-line and multi-line plain text input.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `multiline` | boolean | | `false` | Multi-line textarea mode |
| `min_rows` | integer | | 3 | Minimum visible rows (multiline only) |
| `max_rows` | integer | | 10 | Maximum rows before scroll (multiline only) |
| `max_length` | integer | | — | Character limit |
| `min_length` | integer | | — | Minimum characters required |
| `pattern` | string (regex) | | — | Validation regex pattern |
| `autocomplete` | string | | — | HTML autocomplete hint (`email`, `tel`, `name`, etc.) |
| `input_type` | `text`\|`email`\|`url`\|`tel`\|`search` | | `text` | Semantic input type (affects mobile keyboard) |
| `prefix` | LocalizedString | | — | Static text shown before the input |
| `suffix` | LocalizedString | | — | Static text shown after the input |
| `prefix_icon` | IconRef | | — | |
| `suffix_icon` | IconRef | | — | |

### 11.3.2 Number Input

**`field.number`** — Numeric input with integer or decimal support.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `number_type` | `integer`\|`decimal` | | `decimal` | |
| `min` | number | | — | |
| `max` | number | | — | |
| `step` | number | | `1` (integer), `0.01` (decimal) | Increment/decrement step |
| `decimal_places` | integer | | `2` | Decimal precision |
| `show_stepper` | boolean | | `false` | Whether to show +/– stepper buttons |
| `format` | `plain`\|`percentage`\|`scientific` | | `plain` | Display format |
| `thousands_separator` | boolean | | `true` | Show thousands separators |
| `prefix` | LocalizedString | | — | E.g., "#" for order numbers |
| `suffix` | LocalizedString | | — | E.g., "kg", "units" |

### 11.3.3 Currency Amount

**`field.currency_amount`** — A financial amount with currency awareness. One of the most important ERP-specific field types.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `currency_field` | string | | — | The `name` of another field in the same form whose value provides the currency code. When set, this field renders as an amount+currency pair |
| `currency_code` | string | | — | Static currency code (e.g., `"USD"`). Used when `currency_field` is not set |
| `allow_negative` | boolean | | `false` | Whether negative amounts are valid |
| `decimal_places` | integer | | 2 | Override for currencies with non-standard precision |
| `max_value` | number | | — | |
| `show_currency_symbol` | boolean | | `true` | Display the currency symbol (e.g., "$", "€") |
| `show_currency_code` | boolean | | `false` | Display the ISO currency code |

> 📘 **Note:** When `currency_field` is set, the rendering engine displays an integrated amount + currency selector. The currency selector shows the ISO code and flag of the selected currency. Changes to the currency selector update the referenced currency field's value and re-format the amount according to the new currency's decimal convention.

### 11.3.4 Date / Time / DateTime Pickers

**`field.date`** — Date selection.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `min_date` | date\|Binding | | — | Earliest selectable date (supports `$expr: "today()"`) |
| `max_date` | date\|Binding | | — | Latest selectable date |
| `disabled_dates` | array of date\|Binding | | — | Specific dates to disable |
| `disabled_days_of_week` | array of integer (0–6) | | — | 0=Sunday |
| `date_format` | string | | *(locale default)* | Display format (e.g., `"DD/MM/YYYY"`) |
| `input_hint` | `native_date_picker`\|`calendar_popup`\|`text_input` | | *(platform default)* | Rendering hint |

**`field.time`** — Time selection.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `min_time` | time string | | — | |
| `max_time` | time string | | — | |
| `step_minutes` | integer | | `15` | Minute increment in the picker |
| `use_24h` | boolean | | *(locale default)* | 24-hour vs. 12-hour display |

**`field.datetime`** — Combined date and time selection. Props are a union of `field.date` and `field.time` props, plus:

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `timezone` | IANA timezone string | | *(user locale timezone)* | The timezone in which the value is interpreted |
| `show_timezone` | boolean | | `false` | Display the timezone abbreviation next to the input |

### 11.3.5 Date Range Picker

**`field.date_range`** — Selects a start and end date as a pair.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `start_label` | LocalizedString | | `"From"` | Label for the start date input |
| `end_label` | LocalizedString | | `"To"` | Label for the end date input |
| `min_date` | date\|Binding | | — | |
| `max_date` | date\|Binding | | — | |
| `min_range_days` | integer | | — | Minimum span between start and end |
| `max_range_days` | integer | | — | Maximum span |
| `presets` | array of DateRangePreset | | — | Quick-select options (e.g., "This Month", "Last Quarter") |
| `layout` | `inline`\|`side_by_side`\|`stacked` | | `side_by_side` | How the two pickers are arranged |

**DateRangePreset:** `{ label: LocalizedString, start_offset_days: integer, end_offset_days: integer }` — offsets relative to today.

### 11.3.6 Select / Dropdown

**`field.select`** — Single-value selection from a list of options.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `options` | array of SelectOption\|DataSourceRef | ✅ | — | Static list or dynamic data source reference |
| `allow_clear` | boolean | | `false` | Whether the selection can be cleared |
| `searchable` | boolean | | `false` | Whether the dropdown is filterable by typing |
| `search_placeholder` | LocalizedString | | — | |
| `option_label_field` | string | | `"label"` | Field name for display text when options come from a data source |
| `option_value_field` | string | | `"value"` | Field name for the submitted value |
| `group_by_field` | string | | — | Field name for option grouping |
| `max_dropdown_height` | string | | `"300px"` | |

**SelectOption:** `{ value: string|integer, label: LocalizedString, icon?: IconRef, disabled?: boolean, description?: LocalizedString }`

**`field.multi_select`** — Multi-value selection. Extends `field.select` with:

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `min_selections` | integer | | — | |
| `max_selections` | integer | | — | |
| `display_mode` | `tags`\|`count`\|`list` | | `tags` | How selected values are displayed in the closed state |

### 11.3.7 Checkbox, Checkbox Group

**`field.checkbox`** — A single boolean toggle.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `checked_value` | any | | `true` | The value submitted when checked |
| `unchecked_value` | any | | `false` | The value submitted when unchecked |
| `indeterminate` | boolean\|Binding | | `false` | Three-state mode (for "select all" patterns) |

**`field.checkbox_group`** — Multiple checkboxes sharing a field name.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `options` | array of SelectOption\|DataSourceRef | ✅ | — | |
| `layout` | `vertical`\|`horizontal`\|`grid` | | `vertical` | |
| `grid_columns` | integer | | 2 | Columns when `layout: "grid"` |
| `min_selections` | integer | | — | |
| `max_selections` | integer | | — | |

### 11.3.8 Radio Group

**`field.radio_group`** — Single-selection from a set of visible options.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `options` | array of SelectOption\|DataSourceRef | ✅ | — | |
| `layout` | `vertical`\|`horizontal`\|`card` | | `vertical` | `card` renders each option as a selectable card |
| `allow_deselect` | boolean | | `false` | Whether clicking the selected option deselects it |

### 11.3.9 Toggle / Switch

**`field.toggle`** — A binary on/off switch. Semantically equivalent to `field.checkbox` but with a different visual affordance — use for settings and preferences rather than for form data collection.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `on_label` | LocalizedString | | — | Label when toggled on |
| `off_label` | LocalizedString | | — | Label when toggled off |
| `size` | `sm`\|`md`\|`lg` | | `md` | |

### 11.3.10 File Upload

**`field.file_upload`** — File attachment input.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `accept` | array of MIME type or extension | | — | Allowed file types (e.g., `[".pdf", ".xlsx", "image/*"]`) |
| `max_file_size_mb` | number | | `10` | |
| `max_files` | integer | | `1` | `1` for single file; >1 for multi-file |
| `upload_endpoint` | string | ✅ | — | The API endpoint to which files are POSTed |
| `upload_field_name` | string | | `"file"` | The multipart field name |
| `upload_extra_data` | object | | — | Additional fields included in the upload request |
| `show_preview` | boolean | | `true` | Whether to show a preview thumbnail for images |
| `storage_mode` | `immediate`\|`on_submit` | | `immediate` | Whether files are uploaded immediately or held until form submission |

### 11.3.11 Signature Input

**`field.signature`** — Captures a handwritten signature on touch devices; falls back to typed name on non-touch.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `output_format` | `base64_png`\|`svg` | | `base64_png` | |
| `min_stroke_count` | integer | | 3 | Minimum drawing strokes before the signature is considered valid |
| `canvas_height` | integer (px) | | `200` | |
| `clear_label` | LocalizedString | | `"Clear"` | |

### 11.3.12 Rich Text Editor

**`field.rich_text`** — A WYSIWYG text editor with formatting support.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `toolbar` | array of ToolbarItem | | *(default toolbar)* | Which formatting controls to show |
| `max_length` | integer | | — | Character limit (applied to plain-text equivalent) |
| `output_format` | `html`\|`markdown`\|`delta` | | `html` | The format of the submitted value |
| `placeholder` | LocalizedString | | — | |
| `min_height` | integer (px) | | `120` | |
| `max_height` | integer (px) | | `400` | |
| `allow_images` | boolean | | `false` | Whether inline images can be inserted |
| `image_upload_endpoint` | string | | — | Required if `allow_images: true` |

**Default toolbar items:** `bold`, `italic`, `underline`, `strikethrough`, `ordered_list`, `unordered_list`, `link`, `clear_formatting`.

### 11.3.13 Lookup / Entity Picker

**`field.lookup`** — A search-based selector for ERP entities (vendors, employees, GL accounts, cost centers, etc.). This is one of the most critical ERP-specific field types — it provides an inline search-as-you-type experience for large entity catalogs.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `search_endpoint` | string | ✅ | — | API endpoint for search requests (`GET ?q={query}&tenant={tenant_id}`) |
| `value_field` | string | ✅ | — | The entity field used as the submitted value (typically `id`) |
| `display_field` | string | ✅ | — | The entity field shown in the input after selection |
| `search_fields` | array of string | | `[display_field]` | Fields searched on the backend |
| `result_template` | Node | | — | A component subtree rendered for each search result |
| `min_search_length` | integer | | `2` | Minimum characters before search is triggered |
| `debounce_ms` | integer | | `300` | Debounce delay for search requests |
| `max_results` | integer | | `10` | Maximum results shown in the dropdown |
| `allow_create` | boolean | | `false` | Whether the user can create a new entity from the picker |
| `create_action` | ActionRef | | — | Required if `allow_create: true` |
| `preload` | boolean | | `false` | Whether to load all options upfront (for small catalogs only) |
| `multi` | boolean | | `false` | Multiple-entity selection mode |

### 11.3.14 Cascading Selects

**`field.cascading_select`** — A chain of dependent selects where each level's options depend on the selection of the previous level (e.g., Country → State → City; Company → Division → Department).

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `levels` | array of CascadeLevel | ✅ | — | Ordered level definitions |
| `layout` | `horizontal`\|`vertical` | | `horizontal` | |
| `clear_downstream_on_change` | boolean | | `true` | Whether selecting a level clears all downstream selections |

**CascadeLevel:**
```json
{
  "name": "department_id",
  "label": { "$i18n": "field.department" },
  "options_endpoint": "/api/org/departments?parent_id={parent_value}",
  "value_field": "id",
  "display_field": "name",
  "placeholder": { "$i18n": "field.department.placeholder" }
}
```

The `{parent_value}` token in `options_endpoint` is replaced at runtime with the selected value of the previous level.

### 11.3.15 Slider

**`field.slider`** — A draggable range input.

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `min` | number | ✅ | — | |
| `max` | number | ✅ | — | |
| `step` | number | | `1` | |
| `show_value` | boolean | | `true` | Display current value above the thumb |
| `show_ticks` | boolean | | `false` | |
| `range` | boolean | | `false` | Dual-thumb range selection (submits `{ min, max }`) |
| `marks` | array of SliderMark | | — | Labeled positions on the track |

---

## 11.4 Form Layout

### 11.4.1 Field Grid Placement

Fields within a `form.section` are placed in a grid whose column count is defined by the section's `columns` prop (default: inherits from the `form.container`). Each field occupies one column by default. Fields can span multiple columns using the `col_span` prop.

```go
ast.NewFormSection("address-section").
    WithColumns(2).
    AddField(ast.NewTextField("street_address").
        WithColSpan(2),  // full width — spans both columns
    ).
    AddField(ast.NewTextField("city")).          // col 1
    AddField(ast.NewTextField("postal_code")).   // col 2
    AddField(ast.NewSelectField("country").
        WithColSpan(2),  // full width
    )
```

### 11.4.2 Multi-Column Forms

Forms with multiple columns are the standard layout for ERP data-entry screens. The section `columns` prop sets the column count for the section. A `form.container` with `layout: "two_column"` sets the default to 2 for all sections that do not override it.

Column widths within a section are equal by default. When a section contains fields with mixed `col_span` values, the grid algorithm fills remaining space with subsequent fields — the same algorithm used by CSS Grid auto-placement.

### 11.4.3 Collapsible Sections

Sections with `collapsible: true` render with an expand/collapse chevron in the section header. The initial state is set by `default_collapsed`. Collapsed sections are fully present in the DOM/view tree — they are not unmounted. Their fields are still included in form validation and submission.

> ⚠️ **Warning:** Do not use collapsible sections to hide required fields by default. A form where the user cannot see a required field and does not know to expand its section produces a confusing validation failure. If a section contains required fields, set `default_collapsed: false` or use permission-based exclusion (`$if`) to hide the section from users who don't need it.

### 11.4.4 Tab-Based Form Sections

Large forms can be organized into tabs using `layout.tab_container` as a direct child of `form.container`. Each tab contains one or more `form.section` nodes.

```go
ast.NewForm("vendor-onboarding").
    AddChild(
        ast.NewTabContainer("vendor-tabs").
            AddTab("basic-info", i18n.Key("tab.basic_info"),
                ast.NewFormSection("basic").AddField(...),
            ).
            AddTab("banking", i18n.Key("tab.banking"),
                ast.NewFormSection("banking").AddField(...).
                    WithPermission("finance:banking:read"),
            ).
            AddTab("compliance", i18n.Key("tab.compliance"),
                ast.NewFormSection("compliance").AddField(...),
            ),
    ).
    WithSubmitAction("action-save-vendor")
```

> 📘 **Note:** When a form is organized into tabs, server-side validation errors must specify their `field` paths clearly so the rendering engine can surface the error on the correct tab. If a field on an inactive tab has a validation error after submission, the rendering engine must navigate to that tab and display the error. The tab's `error` state (a visual indicator on the tab label) is set automatically by the rendering engine when any field within that tab has a validation error.

### 11.4.5 Wizard / Multi-Step Forms

Multi-step forms use `ui.stepper` combined with a state variable that tracks the current step, and `$show` directives on `form.section` groups to display one step at a time.

```go
// State variable for current step
ast.NewStateVariable("current_step").WithInitialValue(0),

// Stepper component
ast.NewStepper("po-wizard-stepper").
    WithCurrentStep(ast.StateBinding("current_step")).
    WithSteps(
        ast.Step("vendor-step",    i18n.Key("step.vendor")),
        ast.Step("items-step",     i18n.Key("step.items")),
        ast.Step("approval-step",  i18n.Key("step.approval")),
        ast.Step("review-step",    i18n.Key("step.review")),
    ),

// Each section shown for its step only
ast.NewFormSection("vendor-section").
    WithShow(ast.StateBinding("current_step").EqualTo(0)),

ast.NewFormSection("items-section").
    WithShow(ast.StateBinding("current_step").EqualTo(1)),
```

Navigation between steps is handled by `action.set_variable` actions on "Next" and "Back" buttons, with optional per-step validation via `action.validate_section` before advancing.

---

## 11.5 Dynamic Forms

### 11.5.1 Conditionally Visible Fields

Fields are conditionally shown (not conditionally included in the payload — use `$if` for that) using the `$show` directive bound to form state:

```go
// Show the "approval_reason" field only when order type is "emergency"
ast.NewTextField("approval_reason").
    WithLabel(i18n.Key("po.field.approval_reason")).
    WithRequired(true).
    WithShow(
        ast.Conditional(
            ast.FieldBinding("order_type").EqualTo("emergency"),
        ),
    )
```

### 11.5.2 Conditionally Required Fields

Fields can be made dynamically required based on other field values:

```go
ast.NewTextField("justification").
    WithLabel(i18n.Key("po.field.justification")).
    WithRequired(
        ast.Conditional(
            ast.FieldBinding("estimated_total").GreaterThan(10000),
        ),
    )
```

The rendering engine enforces the conditional required rule at validation time — the field is required only when the condition is true.

### 11.5.3 Dynamically Populated Options

Select fields can load their options from a data source that responds to other field values:

```go
ast.NewSelectField("cost_center_id").
    WithLabel(i18n.Key("po.field.cost_center")).
    WithOptionsSource(
        ast.NewRESTDataSource("cost-centers-source").
            WithEndpoint("/api/finance/cost-centers").
            WithParam("department_id", ast.FieldBinding("department_id")).
            WithParam("active_only", true),
    )
```

When `department_id` changes, the data source is automatically refreshed, and the cost center options update. The current cost center selection is cleared when the options change (configurable with `clear_on_reload: false`).

### 11.5.4 Repeating Field Groups (Line Items)

Line item patterns — the core data entry model for purchase orders, invoices, journal entries, and stock transfers — use the `form.line_item_section` component.

```go
ast.NewLineItemSection("line-items").
    WithLabel(i18n.Key("po.section.line_items")).
    WithMinItems(1).
    WithMaxItems(500).
    WithAddLabel(i18n.Key("po.line_items.add")).
    WithRemoveLabel(i18n.Key("po.line_items.remove")).
    AddColumn(ast.NewLookupField("item_id").WithLabel(i18n.Key("field.item"))).
    AddColumn(ast.NewTextField("description").WithLabel(i18n.Key("field.description"))).
    AddColumn(ast.NewNumberField("quantity").WithLabel(i18n.Key("field.quantity")).WithMin(0.001)).
    AddColumn(ast.NewCurrencyAmountField("unit_price").WithLabel(i18n.Key("field.unit_price"))).
    AddColumn(ast.NewCurrencyAmountField("line_total").
        WithLabel(i18n.Key("field.line_total")).
        WithReadOnly(true).
        WithValue(ast.Expr("$item.quantity * $item.unit_price")),
    ).
    WithTotalsRow(
        ast.LineItemTotal("total_amount",
            i18n.Key("po.line_items.total"),
            ast.Expr("sum(line_items[*].line_total)"),
        ),
    )
```

The submission payload for a line item section is an array of objects, keyed by the line item's internal identifier:

```json
{
  "line_items": [
    { "item_id": "ITEM-001", "description": "Steel Pipe 2\"", "quantity": 100, "unit_price": 4.50 },
    { "item_id": "ITEM-002", "description": "Steel Pipe 3\"", "quantity": 50,  "unit_price": 6.75 }
  ]
}
```

### 11.5.5 Computed / Read-Only Fields

Computed fields display a value derived from other fields but are not user-editable and are not included in the submission payload (unless `include_in_submission: true` is set):

```go
ast.NewCurrencyAmountField("vat_amount").
    WithLabel(i18n.Key("po.field.vat_amount")).
    WithReadOnly(true).
    WithValue(ast.Expr("total_amount * (vat_rate / 100)")).
    WithCurrencyField("currency_code")
```

---

## 11.6 Form State Management

The rendering engine owns form state. The backend defines the form's structure and initial state; the rendering engine tracks mutations from that initial state.

### 11.6.1 Initial Values

Initial field values are specified either as static `default_value` props or as data bindings:

```go
// Static default
ast.NewSelectField("currency_code").
    WithDefaultValue("USD")

// Data binding — pre-populate from a data source
ast.NewTextField("vendor_name").
    WithDefaultValue(ast.DataBinding("datasource:vendor.name"))
```

When an initial value comes from a data source (e.g., loading an existing purchase order for editing), the form is considered "clean" until the user makes a change — the data source values are treated as the original state, not as pending edits.

### 11.6.2 Dirty State

A form is "dirty" when any field's current value differs from its initial value. Dirty state is tracked per-field and at the form level. The rendering engine:
- Shows unsaved-changes indicators when `track_dirty: true`
- Warns the user before navigation when the form is dirty
- Includes a `_dirty_fields` array in the submission payload (containing the names of changed fields) when `track_dirty: true`, enabling the backend to perform partial updates efficiently

### 11.6.3 Touched State

A field is "touched" once it has received focus and then been blurred (the user interacted with it). Validation errors are only shown for touched fields (on-blur validation) or all fields (on-submit validation). This prevents showing error states on fields the user has not yet interacted with.

### 11.6.4 Error State

Fields enter error state when they fail validation. Error state:
- Displays the error message below the field (replacing help text)
- Changes the field's border color to `token:border.error`
- Sets `aria-invalid: true` for accessibility
- Contributes to the form's overall error count (preventing submission when `> 0`)

### 11.6.5 Submitting State

When the form's submit action is executing, the form enters submitting state:
- All fields become read-only
- The submit button shows a loading indicator
- All other actions (except Cancel) are disabled
- The form exits submitting state when the action completes (success or error)

---

## 11.7 Form Validation

Form validation is covered in full in Chapter 23 — Validation Framework. This section provides a summary of how validation integrates with the forms framework specifically.

Validation rules are attached to individual field nodes as the `validation_rules` array. Rules are specified in the AST by the backend engineer, delivered to the client in the compiled form definition, and executed by the rendering engine on the client side for UX responsiveness. The backend always re-validates on submission.

```go
ast.NewTextField("po_number").
    WithLabel(i18n.Key("po.field.po_number")).
    WithValidationRules(
        ast.Rule().Required(),
        ast.Rule().MinLength(6),
        ast.Rule().MaxLength(20),
        ast.Rule().Pattern(`^PO-\d{4,}$`, i18n.Key("validation.po_number.format")),
        ast.Rule().RemoteUnique(
            "/api/procurement/purchase-orders/validate-number",
            i18n.Key("validation.po_number.duplicate"),
        ),
    )
```

---

## 11.8 Form Actions

### 11.8.1 Submit Action

The primary form action. Collects all field values, runs client-side validation, and dispatches the submit endpoint with the serialized form payload.

The submit action is declared as an `action.http_request` node with `form_submit: true`:

```go
ast.NewHTTPAction("action-submit-po").
    WithMethod("POST").
    WithEndpoint("/api/procurement/purchase-orders").
    WithFormSubmit(true).           // uses form data as the request body
    WithLoadingLabel(i18n.Key("po.action.submitting")).
    WithSuccessAction(ast.ActionRef("action-navigate-to-detail")).
    WithErrorAction(ast.ActionRef("action-show-error-toast"))
```

### 11.8.2 Save Draft Action

A secondary action that saves the current form state without validation. Draft saves use the same endpoint as submit but with a different status marker in the payload:

```go
ast.NewHTTPAction("action-save-draft").
    WithMethod("POST").
    WithEndpoint("/api/procurement/purchase-orders/draft").
    WithFormSubmit(true).
    WithSkipValidation(true).       // bypass client-side validation
    WithLoadingLabel(i18n.Key("po.action.saving_draft"))
```

### 11.8.3 Reset Action

Resets all fields to their initial values. Clears dirty state, touched state, and error state.

```go
ast.NewResetAction("action-reset-form").
    WithConfirmation(i18n.Key("form.reset.confirmation"))  // shows a confirmation dialog
```

### 11.8.4 Cancel Action

Navigates away from the form. If the form is dirty, shows the dirty warning before navigating (when `track_dirty: true`).

```go
ast.NewNavigateAction("action-cancel").
    WithTarget("procurement.purchase-orders.list").
    WithDirtyCheck(true)
```

### 11.8.5 Custom Form Actions

Additional actions can be attached to the form for operations that do not fit the submit/save/reset model — for example, an "Import from CSV" action that populates line items from an uploaded file, or a "Copy from Previous Order" action.

```go
ast.NewHTTPAction("action-import-items").
    WithMethod("POST").
    WithEndpoint("/api/procurement/line-items/import").
    WithRequestBody(ast.StaticBody(map[string]any{"source": "csv"})).
    WithOnSuccess(
        ast.ActionChain(
            ast.ActionRef("action-refresh-line-items"),
            ast.ActionRef("action-show-import-success-toast"),
        ),
    )
```

---

## 11.9 Form Permissions

### 11.9.1 Field-Level Read/Write Permissions

Fields can be restricted to read-only or hidden entirely using permission directives:

```go
// Visible but read-only for users who can see but not change GL codes
ast.NewLookupField("gl_account_id").
    WithLabel(i18n.Key("po.field.gl_account")).
    WithReadOnly(
        ast.Conditional(
            ast.Not(ast.Permission("finance:gl_code:write")),
        ),
    )

// Completely hidden from users without the banking permission
ast.NewFormSection("banking-section").
    WithTitle(i18n.Key("vendor.section.banking")).
    WithIf(ast.Permission("finance:banking:read"))
```

### 11.9.2 Section-Level Visibility

Entire form sections can be protected with permission directives. When a section is excluded via `$if`, its fields are neither sent to the client nor included in validation or submission. The form layout reflows to fill the space.

### 11.9.3 Submit Permission

The submit action itself should be protected to prevent unauthorized form submission. The submit action node carries a permission directive that causes it to be excluded from the compiled payload for unauthorized users. Without a submit action in the definition, the rendering engine renders the form in read-only mode.

```go
ast.NewHTTPAction("action-submit-po").
    WithPermission("procurement:po:create").
    WithMethod("POST").
    WithEndpoint("/api/procurement/purchase-orders")
```

---

## 11.10 ERP-Specific Form Patterns

### 11.10.1 Header + Line Items Pattern

The most prevalent ERP data-entry pattern: a document header section (vendor, dates, reference number, totals) combined with a repeating line items section. See Section 11.5.4 for the line items implementation. The combination looks like this in the AST:

```go
ast.NewForm("purchase-order-form").
    AddSection(ast.NewFormSection("po-header"). ...header fields... ).
    AddSection(ast.NewLineItemSection("po-lines"). ...column definitions... ).
    AddSection(ast.NewFormSection("po-totals"). ...computed totals... ).
    AddSection(ast.NewFormSection("po-notes"). ...notes and attachments... ).
    WithActions(submitAction, saveDraftAction, cancelAction)
```

### 11.10.2 Approval Routing Fields

Forms that initiate approval workflows include fields that configure the routing:

```go
ast.NewFormSection("approval-routing").
    WithTitle(i18n.Key("po.section.approval_routing")).
    WithIf(ast.Permission("procurement:po:set_approver")).
    AddField(
        ast.NewLookupField("approver_id").
            WithLabel(i18n.Key("po.field.approver")).
            WithSearchEndpoint("/api/users/approvers").
            WithRequired(true),
    ).
    AddField(
        ast.NewTextField("approval_note").
            WithLabel(i18n.Key("po.field.approval_note")).
            WithMultiline(true),
    )
```

### 11.10.3 Document Attachment Fields

ERP documents commonly require file attachments (supporting invoices, delivery notes, signed contracts):

```go
ast.NewFormSection("attachments-section").
    WithTitle(i18n.Key("section.attachments")).
    AddField(
        ast.NewFileUploadField("attachments").
            WithLabel(i18n.Key("field.attachments")).
            WithAccept([]string{".pdf", ".jpg", ".png", ".xlsx"}).
            WithMaxFiles(10).
            WithMaxFileSizeMB(25).
            WithUploadEndpoint("/api/documents/upload").
            WithStorageMode(ast.UploadImmediate),
    )
```

### 11.10.4 Audit Trail Display in Forms

Edit forms for existing documents display a read-only audit trail at the bottom of the form, showing who created, modified, and approved the document:

```go
ast.NewFormSection("audit-section").
    WithTitle(i18n.Key("section.audit_trail")).
    WithDefaultCollapsed(true).
    AddField(
        ast.NewTextField("created_by_name").
            WithLabel(i18n.Key("field.created_by")).
            WithReadOnly(true).
            WithDefaultValue(ast.DataBinding("datasource:po.created_by.full_name")),
    ).
    AddField(
        ast.NewDatetimeField("created_at").
            WithLabel(i18n.Key("field.created_at")).
            WithReadOnly(true).
            WithDefaultValue(ast.DataBinding("datasource:po.created_at")),
    ).
    AddField(
        ast.NewTextField("last_modified_by_name").
            WithLabel(i18n.Key("field.last_modified_by")).
            WithReadOnly(true).
            WithDefaultValue(ast.DataBinding("datasource:po.updated_by.full_name")),
    )
```

---

*End of Chapter 11*

**Previous:** [Chapter 10 — Layout System](./10-layout-system.md)
**Next:** [Chapter 12 — Tables and Data Grids](./12-tables-and-data-grids.md)
