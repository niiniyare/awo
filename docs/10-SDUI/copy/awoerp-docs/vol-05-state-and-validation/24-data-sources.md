# Chapter 24 — Data Sources

> **Volume:** V — State, Validation & Data
> **Phase:** 3
> **Audience:** Platform Engineers, Backend Engineers, Web Engineers, Mobile Engineers
> **Prerequisites:** Chapters 08, 22

---

## Table of Contents

- [24.1 Data Source Philosophy](#241-data-source-philosophy)
- [24.2 Data Source Declaration](#242-data-source-declaration)
- [24.3 Data Source Types](#243-data-source-types)
- [24.4 Data Source Lifecycle](#244-data-source-lifecycle)
- [24.5 Parameterized Data Sources](#245-parameterized-data-sources)
- [24.6 Dependent Data Sources](#246-dependent-data-sources)
- [24.7 Pagination](#247-pagination)
- [24.8 Data Source Caching](#248-data-source-caching)
- [24.9 Data Transformation](#249-data-transformation)
- [24.10 Aggregation and Summary Data](#2410-aggregation-and-summary-data)
- [24.11 Shared Data Sources](#2411-shared-data-sources)
- [24.12 Data Source Security](#2412-data-source-security)
- [24.13 Data Source Telemetry](#2413-data-source-telemetry)

---

## 24.1 Data Source Philosophy

A **data source** is the bridge between a surface's UI components and the backend data they display. It is declared in the surface definition alongside the components that use it, fetched by the rendering engine when the surface loads, and refreshed reactively when its parameters change.

Data sources embody the SDUI data principle: **the server controls what data is fetched; the client controls when it is displayed**. The surface definition declares `endpoint`, `method`, and `params` — the client executes the fetch and delivers the results as `$data.*` bindings. The client cannot invent or transform the data it fetches (beyond the `transform` declarations covered in Section 24.9).

**Data sources are not queries.** They do not expose a query language or allow the client to filter, join, or aggregate server data. Every data source maps to a backend endpoint that returns exactly the data needed for that surface in exactly the shape the surface expects. The backend is responsible for the right data in the right shape; the client is responsible for displaying it.

---

## 24.2 Data Source Declaration

Data sources are declared in the surface definition's `data_sources` array. They are referenced in components via their `id`.

```json
{
  "surface": {
    "data_sources": [
      {
        "id": "invoice",
        "type": "single",
        "endpoint": "/api/invoices/$route.params.invoice_id",
        "method": "GET"
      },
      {
        "id": "invoice_line_items",
        "type": "list",
        "endpoint": "/api/invoices/$route.params.invoice_id/line-items",
        "method": "GET",
        "params": {
          "page": "$state.line_items_page",
          "page_size": 50
        },
        "refresh_on": ["invoice_saved", "line_item_added", "line_item_deleted"]
      },
      {
        "id": "gl_accounts",
        "type": "list",
        "endpoint": "/api/gl-accounts",
        "method": "GET",
        "params": {
          "active": true
        },
        "cache_ttl_seconds": 300,
        "load_strategy": "eager"
      }
    ]
  }
}
```

### 24.2.1 DataSource Schema

```typescript
interface DataSourceDeclaration {
  id: string;                        // Unique within the surface
  type: 'single' | 'list';          // Single object or list of objects
  endpoint: string;                  // API endpoint (may include bindings)
  method: 'GET' | 'POST';           // HTTP method (POST for complex query bodies)
  params?: Record<string, unknown>; // URL query parameters (may include bindings)
  body?: Record<string, unknown>;   // Request body (POST only, may include bindings)
  headers?: Record<string, string>; // Additional headers
  load_strategy?: 'eager' | 'lazy' | 'manual';  // When to load (default: "eager")
  refresh_on?: string[];            // Surface bus events that trigger a refresh
  cache_ttl_seconds?: number;       // Override default cache TTL
  transform?: TransformSpec;        // Client-side data transformation
  max_rows?: number;                // Override default 2000 row limit
  polling_interval_seconds?: number; // Auto-refresh interval (if needed; prefer SSE)
}
```

---

## 24.3 Data Source Types

### 24.3.1 single

A `single` data source fetches one object (or a wrapper around one object). Its fields are accessed as `$data.source_id.field_name`.

```json
{ "id": "invoice", "type": "single", "endpoint": "/api/invoices/:id" }
```

Bindings: `$data.invoice.invoice_number`, `$data.invoice.status`, `$data.invoice.amount`

### 24.3.2 list

A `list` data source fetches an array of objects. The response must conform to the platform's list response envelope:

```json
{
  "data": [ {...}, {...} ],
  "meta": {
    "total": 1247,
    "page": 1,
    "page_size": 25,
    "cursor": "eyJpZCI6MTAwfQ=="
  }
}
```

The array is accessed as `$data.source_id.rows`. Individual row fields are accessed via `$row.field_name` in the context of list-iterating components (data tables, repeat layouts).

Summary bindings: `$data.source_id.$rowCount`, `$data.source_id.meta.total`

---

## 24.4 Data Source Lifecycle

### 24.4.1 Load Strategies

| Strategy | When loaded | Use case |
|----------|-------------|----------|
| `"eager"` (default) | Immediately on surface mount | Primary surface data (invoice, customer) |
| `"lazy"` | When a component that uses it scrolls into view | Heavy data not needed for initial paint (audit log, history) |
| `"manual"` | Only when explicitly triggered by `action.invalidate_data_source` or `action.load_data_source` | Data loaded conditionally (e.g., only after user clicks "Load History") |

### 24.4.2 Refresh Triggers

A data source can be refreshed (re-fetched) by:

1. **`action.invalidate_data_source`** — explicit invalidation from an action chain after a mutation
2. **`refresh_on` events** — surface bus events that trigger a refresh when emitted
3. **Parameter changes** — when any binding in `params` or `endpoint` changes value, the data source re-fetches automatically
4. **SSE `data_refresh` event** — the server pushes a refresh signal for specific data source IDs (Chapter 21)
5. **`polling_interval_seconds`** — periodic auto-refresh (use sparingly; prefer SSE push)
6. **Manual user action** — a pull-to-refresh gesture (mobile) or a refresh button bound to `action.invalidate_data_source`

### 24.4.3 Loading State

While a data source is loading, `$data.source_id.$loading` is `true`. Components that display data from a loading data source can use this to show skeleton loaders or loading indicators.

```json
{
  "type": "ui.data_table",
  "props": {
    "data_source_id": "invoices_list",
    "loading": "$data.invoices_list.$loading",
    "empty_state": {
      "title": "No invoices found",
      "subtitle": "Create your first invoice to get started",
      "action_id": "create_invoice"
    }
  }
}
```

---

## 24.5 Parameterized Data Sources

Data source parameters are reactive — when any parameter binding's value changes, the data source automatically re-fetches with the new parameter values.

### 24.5.1 State-Driven Parameters

```json
{
  "id": "invoices_list",
  "type": "list",
  "endpoint": "/api/invoices",
  "params": {
    "status": "$state.status_filter",
    "vendor_id": "$state.selected_vendor_id",
    "date_from": "$state.date_range.start",
    "date_to": "$state.date_range.end",
    "search": "$state.search_query",
    "sort_by": "$state.sort_column",
    "sort_dir": "$state.sort_direction",
    "page": "$state.current_page",
    "page_size": "$state.page_size"
  }
}
```

When `status_filter` changes (e.g., the user selects "Overdue" from a filter dropdown), the data source refetches with `status=overdue`. This is how server-side filtering and sorting are implemented.

### 24.5.2 Debouncing Parameter Changes

Parameters that change frequently (e.g., a search field) should be debounced to avoid a fetch on every keystroke. The `params_debounce_ms` field debounces all parameter changes:

```json
{
  "id": "vendor_search",
  "type": "list",
  "endpoint": "/api/vendors",
  "params": {
    "q": "$state.vendor_search_query"
  },
  "params_debounce_ms": 300
}
```

### 24.5.3 Endpoint Bindings

The endpoint URL itself may include bindings for path parameters:

```json
{
  "id": "vendor_contacts",
  "type": "list",
  "endpoint": "/api/vendors/$state.selected_vendor_id/contacts",
  "load_strategy": "manual"
}
```

When `selected_vendor_id` is `null`, the endpoint would resolve to `/api/vendors/null/contacts`, which the server would reject. Use `load_strategy: "manual"` or a `load_condition` (below) to prevent this.

### 24.5.4 Load Condition

A data source with a `load_condition` only fetches when the condition evaluates to truthy:

```json
{
  "id": "vendor_contacts",
  "type": "list",
  "endpoint": "/api/vendors/$state.selected_vendor_id/contacts",
  "load_condition": "$expr{ $state.selected_vendor_id !== null }"
}
```

---

## 24.6 Dependent Data Sources

Dependent data sources form a chain: the output of one data source feeds into the parameters of another.

```json
{
  "data_sources": [
    {
      "id": "purchase_order",
      "type": "single",
      "endpoint": "/api/purchase-orders/$route.params.po_id"
    },
    {
      "id": "vendor",
      "type": "single",
      "endpoint": "/api/vendors/$data.purchase_order.vendor_id",
      "load_condition": "$expr{ !!$data.purchase_order.vendor_id }",
      "depends_on": ["purchase_order"]
    },
    {
      "id": "vendor_open_pos",
      "type": "list",
      "endpoint": "/api/purchase-orders",
      "params": {
        "vendor_id": "$data.purchase_order.vendor_id",
        "status": "open"
      },
      "load_condition": "$expr{ !!$data.purchase_order.vendor_id }",
      "depends_on": ["purchase_order"]
    }
  ]
}
```

The `depends_on` array declares that the data source should not fetch until all listed data sources have successfully loaded. This prevents a cascade of failed requests when the dependency's data is not yet available.

---

## 24.7 Pagination

### 24.7.1 Offset Pagination

The default pagination strategy. Uses `page` (1-based) and `page_size` parameters.

```json
{
  "params": {
    "page": "$state.current_page",
    "page_size": "$state.page_size"
  }
}
```

The response `meta.total` is used by `ui.data_table` to calculate the total page count for the pagination control.

### 24.7.2 Cursor Pagination

For large, frequently-updated datasets where offset pagination produces inconsistent results. Uses a cursor token from the previous response.

```json
{
  "id": "activity_feed",
  "type": "list",
  "endpoint": "/api/activity",
  "pagination": {
    "strategy": "cursor",
    "cursor_param": "after",
    "cursor_source": "$data.activity_feed.meta.cursor"
  }
}
```

The rendering engine appends `?after=<cursor>` to the next page request. The cursor value comes from the previous response's `meta.cursor` field.

### 24.7.3 Infinite Scroll Pagination

For data sources used with infinite-scroll components (activity feeds, notification lists). New pages append to the existing data rather than replacing it.

```json
{
  "id": "notifications",
  "type": "list",
  "pagination": {
    "strategy": "infinite",
    "trigger": "scroll_end",    // Fetch next page when user scrolls to bottom
    "page_size": 20
  }
}
```

---

## 24.8 Data Source Caching

### 24.8.1 Default Cache Behaviour

By default, data source responses are cached for **30 seconds** (the stale time). After 30 seconds, the next access triggers a background re-fetch. The cache entry is evicted after **5 minutes** if not accessed.

All cache entries use the full request (endpoint + params + tenant + user role set + locale) as the cache key. Two users with different roles fetching the same endpoint receive different cache entries, because the server may return different data based on their permissions.

### 24.8.2 Cache TTL Configuration

Override the default TTL with `cache_ttl_seconds`:

| TTL | Use case |
|-----|----------|
| `0` | Never cache (always fetch fresh — use for real-time data) |
| `30` (default) | Standard surface data |
| `300` | Slow-changing reference data (GL accounts, tax codes, units of measure) |
| `3600` | Static reference data (countries, currencies, time zones) |

```json
{
  "id": "countries",
  "type": "list",
  "endpoint": "/api/reference/countries",
  "cache_ttl_seconds": 3600
}
```

### 24.8.3 Cache Invalidation

`action.invalidate_data_source` evicts the cache for the specified data source IDs and triggers a background re-fetch:

```json
{
  "type": "action.invalidate_data_source",
  "props": {
    "source_ids": ["invoices_list", "invoice_summary"],
    "invalidate_related": true   // Also invalidate data sources that depend on these
  }
}
```

### 24.8.4 Persistent Cache (Mobile)

On mobile, data source responses are additionally persisted to encrypted disk storage (see Chapter 17, Section 17.10). The persistent cache is used when the device is offline — it serves the last-known data with a staleness indicator in the UI.

---

## 24.9 Data Transformation

Client-side data transformations allow the surface definition to reshape the API response without requiring a new backend endpoint. Transformations are applied after the response is received, before the data is made available in `$data.*` bindings.

**Principle:** Transformations are for presentation shaping only — sorting a list, computing a display label, combining two fields. They are not for business logic. If the transformation would replicate server-side business rules, the backend should provide a purpose-built endpoint.

### 24.9.1 Available Transformations

```json
{
  "id": "chart_data",
  "type": "list",
  "endpoint": "/api/reports/monthly-revenue",
  "transform": {
    "sort": { "field": "month", "direction": "asc" },
    "map": {
      "label": "$row.month_label",
      "value": "$row.net_revenue",
      "color": "$expr{ $row.net_revenue >= 0 ? 'green' : 'red' }"
    },
    "filter": "$expr{ $row.month >= $state.date_range.start }"
  }
}
```

| Transform operation | Description |
|--------------------|-------------|
| `sort` | Sort list by a field (asc/desc) |
| `filter` | Client-side filter by an expression (applied after `max_rows` enforcement) |
| `map` | Reshape each row using field expressions |
| `limit` | Take the first N rows |
| `group_by` | Group rows by a field value, producing `{key, rows[]}` objects |

---

## 24.10 Aggregation and Summary Data

Summary values (totals, counts, averages) are computed server-side and returned as separate fields in the data source response, not computed on the client from the list data. The `ui.data_table`'s aggregation row binds to these server-computed values.

```json
{
  "id": "invoice_line_items",
  "type": "list",
  "endpoint": "/api/invoices/:id/line-items"
}
```

Response:
```json
{
  "data": [ ... ],
  "meta": {
    "total": 12,
    "page": 1,
    "page_size": 50
  },
  "summary": {
    "subtotal": 145000.00,
    "tax_amount": 23200.00,
    "total": 168200.00,
    "currency": "KES"
  }
}
```

Summary fields are accessed as `$data.invoice_line_items.summary.total`.

---

## 24.11 Shared Data Sources

The `data.shared_source` node type (Chapter 13, Section 13.5.1) declares a data source that is shared by multiple widgets on a dashboard. Instead of each widget fetching its data independently, they all subscribe to the same data source instance and receive the same data.

This pattern reduces API calls for dashboards where multiple widgets display different views of the same dataset (e.g., different chart types over the same monthly revenue data).

```json
{
  "type": "data.shared_source",
  "props": {
    "id": "monthly_revenue",
    "endpoint": "/api/reports/monthly-revenue",
    "params": {
      "year": "$state.selected_year"
    }
  }
}
```

Widgets reference the shared source:
```json
{
  "type": "chart.line",
  "props": {
    "data_source_id": "monthly_revenue",
    "x_field": "month_label",
    "y_field": "net_revenue"
  }
}
```

When `selected_year` changes, all widgets subscribing to `monthly_revenue` update simultaneously from a single re-fetch.

---

## 24.12 Data Source Security

### 24.12.1 Authentication

Every data source request is an authenticated HTTP request. The rendering engine includes the session authentication token as a Bearer token in every fetch. Data sources cannot be fetched without a valid session.

### 24.12.2 Server-Side Row-Level Security

The backend applies PostgreSQL Row-Level Security (RLS) policies to every data source query. A user can only receive rows they are authorized to see, regardless of what the surface definition's endpoint declares. There is no client-side concept of "row filtering for security" — that is entirely the database's responsibility.

### 24.12.3 Column-Level Security

Backend endpoints return only the columns the requesting user is authorized to see. If a column is permission-protected (e.g., `salary`, `cost_price`), the backend omits it from the response for users without the required permission. A surface definition that references `$data.employee.salary` for a user who cannot see that field receives `undefined` — the field is simply absent from the response.

The rendering engine handles `undefined` bindings gracefully — they render as empty/blank, not as an error.

### 24.12.4 Tenant Data Isolation

The `X-Tenant-ID` header is included in every data source request. The backend enforces that the requested data belongs to the declared tenant. This is an additional defense-in-depth measure on top of database-level tenant filtering.

---

## 24.13 Data Source Telemetry

| Event | Properties | Purpose |
|-------|-----------|---------|
| `data_source_load_start` | source_id, endpoint, surface_id | Request initiated |
| `data_source_load_success` | source_id, duration_ms, row_count, bytes, cache_hit | Performance and cache effectiveness |
| `data_source_load_error` | source_id, error_code, http_status, duration_ms | Error analysis |
| `data_source_cache_hit` | source_id, age_seconds | Cache hit rate |
| `data_source_cache_evicted` | source_id | Cache eviction |
| `data_source_invalidated` | source_id, trigger | Invalidation reasons |
| `data_source_refresh` | source_id, trigger, duration_ms | Background refresh performance |
| `data_source_transform_applied` | source_id, operations[] | Transform usage |

Data source load duration (`duration_ms`) is measured from request initiation to data availability in the `$data.*` bindings, including deserialization and transformation time.

---

*End of Chapter 24*

**Previous:** [Chapter 23 — Validation Framework](./23-validation-framework.md)
**Next:** [Chapter 25 — API Contracts](../vol-06-platform-apis/25-api-contracts.md)
