# Chapter 22 — Client State Management

> **Volume:** V — State, Validation & Data
> **Phase:** 3
> **Audience:** Platform Engineers, Mobile Engineers, Web Engineers
> **Prerequisites:** Chapters 17–21

---

## Table of Contents

- [22.1 State Management Philosophy](#221-state-management-philosophy)
- [22.2 State Taxonomy](#222-state-taxonomy)
- [22.3 State Variables — Declaration](#223-state-variables--declaration)
- [22.4 State Variables — Initialization](#224-state-variables--initialization)
- [22.5 State Mutation via Actions](#225-state-mutation-via-actions)
- [22.6 State Bindings in the View Tree](#226-state-bindings-in-the-view-tree)
- [22.7 Derived State and Computed Expressions](#227-derived-state-and-computed-expressions)
- [22.8 Form State](#228-form-state)
- [22.9 Data Source State](#229-data-source-state)
- [22.10 Session State](#2210-session-state)
- [22.11 Persistent State](#2211-persistent-state)
- [22.12 State Debugging](#2212-state-debugging)

---

## 22.1 State Management Philosophy

**State lives where it is owned.** The backend owns business state — whether an invoice is approved, the current balance of an account, the items in a purchase order. The client owns UI state — which tab is active, whether a panel is expanded, the current value of a form field before it is submitted.

This separation has a practical consequence: the client should not attempt to replicate or infer business state from UI behavior. When a user approves an invoice by clicking "Approve" and the action succeeds, the client does not flip `invoice.status = "approved"` in its own model. It invalidates the `invoice` data source and lets the server's authoritative response populate the new status. Optimistic updates (Chapter 20, Section 20.5) are the one exception, and they are always backed by a server confirmation.

**State is always readable as bindings.** There is no imperative state-read API in the DSL. UI state is accessed through `$state.*`, `$form.*`, `$data.*` binding expressions, which are evaluated reactively by the rendering engine. This constraint ensures the view always reflects the current state and eliminates a class of synchronization bugs.

**State mutation is always via actions.** A component cannot directly modify state — it emits an event that triggers an action that calls `action.set_state`. This indirection makes every state change traceable, debounce-able, and auditable.

---

## 22.2 State Taxonomy

```
┌─────────────────────────────────────────────────────────────────────┐
│  CLIENT STATE                                                        │
│                                                                      │
│  ┌─────────────────────────┐  ┌───────────────────────────────────┐ │
│  │  SURFACE-SCOPED         │  │  SESSION-SCOPED                   │ │
│  │                         │  │                                   │ │
│  │  • State variables      │  │  • Auth token                     │ │
│  │  • Form state           │  │  • Tenant / user context          │ │
│  │  • Data source state    │  │  • Feature flags                  │ │
│  │  • Component state      │  │  • Global notifications           │ │
│  │    (open/closed,        │  │  • Offline action queue           │ │
│  │     selected item, etc.)│  │  • Navigation history             │ │
│  │                         │  │                                   │ │
│  │  Lifetime: surface mount│  │  Lifetime: app session            │ │
│  │  to unmount             │  │  (cleared on logout)              │ │
│  └─────────────────────────┘  └───────────────────────────────────┘ │
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────────┐ │
│  │  PERSISTENT (survives app termination)                          │ │
│  │                                                                 │ │
│  │  • User preferences (dashboard layouts, column visibility)      │ │
│  │  • Cached surface definitions (with TTL)                        │ │
│  │  • Cached data source responses (with TTL)                      │ │
│  │  • Offline action queue (serialized to encrypted storage)       │ │
│  └─────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 22.3 State Variables — Declaration

State variables are declared in the surface definition's `state` block. The declaration specifies the variable's name, type, and initial value.

```json
{
  "surface": {
    "id": "invoice_list",
    "state": [
      {
        "name": "selected_status_filter",
        "type": "string",
        "initial_value": "all"
      },
      {
        "name": "date_range",
        "type": "object",
        "initial_value": {
          "start": null,
          "end": null
        }
      },
      {
        "name": "is_filter_panel_open",
        "type": "boolean",
        "initial_value": false
      },
      {
        "name": "selected_invoice_ids",
        "type": "array",
        "initial_value": []
      },
      {
        "name": "active_tab",
        "type": "string",
        "initial_value": { "$source": "route_param", "key": "tab", "default": "details" }
      }
    ]
  }
}
```

### 22.3.1 State Variable Types

| `type` | TypeScript equivalent | JSON default |
|--------|----------------------|--------------|
| `"string"` | `string \| null` | `null` |
| `"number"` | `number \| null` | `null` |
| `"boolean"` | `boolean` | `false` |
| `"array"` | `unknown[]` | `[]` |
| `"object"` | `Record<string, unknown> \| null` | `null` |

All types are nullable unless the declaration includes `"required": true` with a non-null `initial_value`.

---

## 22.4 State Variables — Initialization

State variable initial values can be one of four forms:

**Literal value:** A static JSON value.
```json
{ "name": "page_size", "type": "number", "initial_value": 25 }
```

**Route parameter:** The value of a URL/route parameter.
```json
{
  "name": "active_tab",
  "type": "string",
  "initial_value": { "$source": "route_param", "key": "tab", "default": "details" }
}
```

**Data source field:** A field from a data source that loads on surface mount.
```json
{
  "name": "current_currency",
  "type": "string",
  "initial_value": { "$source": "data", "source_id": "tenant_settings", "field": "default_currency" }
}
```

**User preference:** A value from the user's persisted preference store.
```json
{
  "name": "table_density",
  "type": "string",
  "initial_value": { "$source": "preference", "key": "invoice_list.table_density", "default": "comfortable" }
}
```

Initialization from a data source or user preference is deferred — the state variable starts as its `default` value until the source resolves.

---

## 22.5 State Mutation via Actions

State variables are mutated exclusively through `action.set_state` (Chapter 20, Section 20.3.3). There is no two-way binding between a component and a state variable — state changes flow top-down.

### 22.5.1 Wiring a Toggle

```json
{
  "type": "action.button",
  "props": {
    "label": "$expr{ $state.is_filter_panel_open ? 'Hide Filters' : 'Show Filters' }",
    "variant": "ghost",
    "icon": "filter",
    "events": {
      "on_click": "toggle_filter_panel"
    }
  }
}
```

```json
{
  "id": "toggle_filter_panel",
  "chain": [
    {
      "type": "action.set_state",
      "props": {
        "assignments": [
          {
            "var": "is_filter_panel_open",
            "value": "$expr{ !$state.is_filter_panel_open }"
          }
        ]
      }
    }
  ]
}
```

### 22.5.2 Persisting State to User Preferences

After setting a state variable that should persist across sessions, an additional `action.save_preference` step writes the value to the persistent preference store:

```json
{
  "id": "set_table_density",
  "chain": [
    {
      "type": "action.set_state",
      "props": {
        "assignments": [{ "var": "table_density", "value": "$event.value" }]
      }
    },
    {
      "type": "action.save_preference",
      "props": {
        "key": "invoice_list.table_density",
        "value": "$state.table_density"
      }
    }
  ]
}
```

---

## 22.6 State Bindings in the View Tree

State variables are accessed in component props via `$state.<name>` bindings. Any component prop that accepts a `Binding` type can use a state binding.

### 22.6.1 Conditional Visibility

```json
{
  "type": "layout.panel",
  "props": { "id": "filter_panel" },
  "$show": "$state.is_filter_panel_open",
  "children": [ ... ]
}
```

### 22.6.2 Dynamic Data Source Parameters

State variables bound to data source parameters cause the data source to refetch when the state changes:

```json
{
  "type": "data.source",
  "props": {
    "id": "invoices_list",
    "endpoint": "/api/invoices",
    "params": {
      "status": "$state.selected_status_filter",
      "date_from": "$state.date_range.start",
      "date_to": "$state.date_range.end",
      "page_size": "$state.page_size"
    }
  }
}
```

When `selected_status_filter` changes, the data source automatically refetches with the new parameter. The loading state is reflected in the table via `$data.invoices_list.$loading`.

### 22.6.3 Reactive Computed Labels

```json
{
  "type": "ui.text",
  "props": {
    "content": "$expr{ $state.selected_invoice_ids.length > 0 ? $state.selected_invoice_ids.length + ' selected' : 'Select invoices' }"
  }
}
```

---

## 22.7 Derived State and Computed Expressions

Complex derived values are expressed with `$expr{ }` bindings. The expression evaluator (Chapter 18, Section 18.5.3) has access to all binding scopes and supports the following operations:

**Arithmetic:** `+`, `-`, `*`, `/`, `%`
**Comparison:** `===`, `!==`, `<`, `>`, `<=`, `>=`
**Logical:** `&&`, `||`, `!`
**Ternary:** `condition ? value_if_true : value_if_false`
**String concatenation:** `"prefix " + $state.name`
**Null coalescing:** `$data.invoice.discount ?? 0`
**Array operations:** `.length`, `.map()`, `.filter()`, `.find()`, `.some()`, `.every()`, `.includes()`
**Math:** `Math.max()`, `Math.min()`, `Math.round()`, `Math.floor()`, `Math.ceil()`, `Math.abs()`
**String methods:** `.toUpperCase()`, `.toLowerCase()`, `.trim()`, `.startsWith()`, `.endsWith()`, `.includes()`, `.split()`, `.join()`
**Date formatting:** `formatDate($state.date, 'DD MMM YYYY')` — uses tenant locale
**Currency formatting:** `formatCurrency($data.invoice.amount, $data.invoice.currency)` — uses tenant locale

### 22.7.1 Expression Caching

Expressions are compiled to evaluator functions at parse time and cached by expression text. The compiled function is re-executed whenever any of its scope dependencies change. The rendering engine tracks which scope keys each expression reads (via a proxy during the first evaluation) and re-evaluates only when those keys change.

---

## 22.8 Form State

Form state is managed by the form library (`react-hook-form` on web, a custom equivalent on mobile) and mirrored to the `SurfaceStore` for cross-component `$form.*` bindings.

### 22.8.1 Form State Shape

```typescript
interface FormState {
  values: Record<string, unknown>;          // Current field values
  errors: Record<string, FieldError>;       // Field-level validation errors
  dirty: Record<string, boolean>;           // Fields changed from initial value
  touched: Record<string, boolean>;         // Fields the user has interacted with
  isSubmitting: boolean;                    // Submit in progress
  isValidating: boolean;                    // Async validation in progress
  submitCount: number;                      // Number of submit attempts
  isDirty: boolean;                         // Any field changed from initial
  isValid: boolean;                         // No validation errors present
}
```

### 22.8.2 Accessing Form State in Bindings

| Binding | Resolves to |
|---------|-------------|
| `$form.field_name` | Current value of `field_name` |
| `$form.$errors.field_name` | Error message for `field_name` |
| `$form.$dirty.field_name` | `true` if field has been changed |
| `$form.$touched.field_name` | `true` if field has been interacted with |
| `$form.$isSubmitting` | `true` while submit action is running |
| `$form.$isDirty` | `true` if any field changed from initial |
| `$form.$isValid` | `true` if no validation errors |

### 22.8.3 Multi-Form Surfaces

A surface can contain multiple independent forms. Each `form.container` is identified by its `id`. Form state bindings are qualified by form ID when multiple forms are present:

```json
"$form.invoice_form.amount"       // Value of 'amount' field in 'invoice_form'
"$form.line_items_form.$isDirty"  // isDirty flag for 'line_items_form'
```

---

## 22.9 Data Source State

Data source state tracks the loading lifecycle for each data source declared on the surface.

### 22.9.1 Data Source State Shape

```typescript
interface DataSourceState {
  status: 'idle' | 'loading' | 'success' | 'error' | 'refreshing';
  data: unknown;           // Latest successful response data
  error: DataSourceError | null;
  fetchedAt: Date | null;  // Timestamp of last successful fetch
  rowCount: number;        // Number of rows (for list data sources)
}
```

### 22.9.2 Accessing Data Source State in Bindings

| Binding | Resolves to |
|---------|-------------|
| `$data.source_id.field_name` | Field from the data source response |
| `$data.source_id.$loading` | `true` while loading or refreshing |
| `$data.source_id.$error` | Error object if the last fetch failed |
| `$data.source_id.$error.message` | Human-readable error message |
| `$data.source_id.$fetchedAt` | ISO timestamp of last successful fetch |
| `$data.source_id.$rowCount` | Number of rows in the response |
| `$data.source_id.rows` | Array of all rows (for list data sources) |
| `$data.source_id.rows[0].field` | First row, specific field |

### 22.9.3 Loading and Error States in Components

```json
{
  "type": "ui.data_table",
  "props": {
    "data_source_id": "invoices_list",
    "loading": "$data.invoices_list.$loading",
    "error_message": "$data.invoices_list.$error.message"
  }
}
```

---

## 22.10 Session State

Session state is managed by the application shell and is not defined in surface definitions — surfaces access it through read-only bindings.

| Binding | Resolves to |
|---------|-------------|
| `$user.id` | Authenticated user ID |
| `$user.display_name` | User's display name |
| `$user.email` | User's email address |
| `$user.roles` | Array of role IDs |
| `$user.can('permission')` | Permission check (returns boolean) |
| `$tenant.id` | Tenant ID |
| `$tenant.name` | Tenant display name |
| `$tenant.logo_url` | Tenant logo URL |
| `$tenant.currency` | Tenant default currency code |
| `$tenant.locale` | Tenant default locale |
| `$tenant.timezone` | Tenant timezone |
| `$flag.flag_name` | Feature flag value (boolean/string/number) |

Session state bindings are read-only from within surface definitions. They cannot be set via `action.set_state`.

---

## 22.11 Persistent State

Persistent state survives app termination and is restored on the next session.

### 22.11.1 User Preferences

User preferences are key-value pairs stored per user, per surface (or globally). They are fetched as part of the session initialization and made available as `{ "$source": "preference" }` initial values on state variables.

Preference keys follow the convention `<surface_id>.<preference_name>` for surface-scoped preferences and `<preference_name>` for global preferences.

| Preference | Default | Description |
|-----------|---------|-------------|
| `<surface_id>.table_density` | `"comfortable"` | Row density for data tables on this surface |
| `<surface_id>.column_visibility` | (all visible) | Hidden column IDs |
| `<surface_id>.column_order` | (declaration order) | Column reorder positions |
| `<surface_id>.page_size` | `25` | Default page size |
| `dashboard.layout` | (default layout) | Widget positions and sizes |
| `locale` | (tenant default) | User's preferred locale |
| `theme` | `"system"` | `"light"`, `"dark"`, or `"system"` |

### 22.11.2 Preference Storage Backend

Preferences are stored server-side, keyed by `user_id + preference_key`. They are fetched once at session start and cached in session state. The `action.save_preference` step PATCHes the preference to the server and updates the local cache.

Server-side storage ensures preferences are consistent across devices and sessions — a user's column configuration on their laptop is the same on their mobile.

### 22.11.3 Cached Surface Definitions and Data

Cached definitions and data source responses use platform-appropriate encrypted local storage:
- **iOS:** `FileManager` in the application support directory, files encrypted with `NSFileProtectionComplete`
- **Android:** `EncryptedFile` via Jetpack Security
- **Web:** `localStorage` for definitions (non-sensitive); `sessionStorage` for sensitive data source responses; `IndexedDB` for large response caches

Cache entries store: the cached value, a `fetched_at` timestamp, a `ttl_seconds` value, and the ETag from the response (for conditional GET requests).

---

## 22.12 State Debugging

The AwoERP developer tooling (Chapter 43) includes a state inspector that shows the live state of a surface at any moment:

- **State variables panel:** All declared state variables with current values; each shows the last action that mutated it and the timestamp
- **Form state panel:** Field-by-field form state (value, dirty, touched, error)
- **Data source panel:** Status, last fetch time, row count, and raw response data for each data source
- **Action log:** Chronological list of executed actions with step-by-step results and timings
- **Binding evaluator:** Interactive tool for evaluating arbitrary binding expressions against the current state

On mobile, the state inspector is accessible via the developer shake gesture or USB debugging connection. On web, it is a browser DevTools extension panel.

---

*End of Chapter 22*

**Previous:** [Chapter 21 — Event System](../vol-04-rendering/21-event-system.md)
**Next:** [Chapter 23 — Validation Framework](./23-validation-framework.md)
