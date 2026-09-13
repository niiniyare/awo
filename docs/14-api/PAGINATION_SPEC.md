# Pagination Specification

**Classification:** Specification — Tier 1
**Owner:** `14-api/PAGINATION_SPEC.md`
**Status:** Frozen at v1.0

---

## Purpose

This document specifies the pagination model for list endpoints: query parameters, `PageInfo` response structure, and the Filter DSL mapping.

---

## 1. Pagination Model

Awo uses **offset-based pagination**. Cursor-based pagination is not supported in v1.

---

## 2. Query Parameters

| Parameter | Type | Default | Maximum | Description |
|-----------|------|---------|---------|-------------|
| `limit` | integer | 20 | 500 | Maximum records to return |
| `offset` | integer | 0 | — | Number of records to skip |
| `order` | string | `created_at DESC` | — | Sort expression (field + direction) |

### Order Expression Format

```
order={field_name}+{direction}
```

Examples:
```
GET /api/v1/entities/finance-invoice?order=created_at+DESC
GET /api/v1/entities/finance-invoice?order=number+ASC
GET /api/v1/entities/finance-invoice?order=total+DESC
```

Only fields that are sortable (have a B-tree index) are valid sort keys. `LongText`, `JSON`, and `Sensitive` fields MUST NOT be used as sort keys. Invalid sort keys return HTTP 400.

---

## 3. PageInfo Response

List responses include a `meta` object:

```json
{
  "data": [ ... ],
  "meta": {
    "total":       47,
    "limit":       20,
    "offset":       0,
    "has_next":    true,
    "has_previous": false
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `total` | integer | Total records matching the filter (before limit/offset) |
| `limit` | integer | The limit applied to this response |
| `offset` | integer | The offset applied to this response |
| `has_next` | bool | `offset + limit < total` |
| `has_previous` | bool | `offset > 0` |

---

## 4. Go PageInfo Struct

```go
// Package: awo.so/awo/def

type PageInfo struct {
    Total       int64 `json:"total"`
    Limit       int   `json:"limit"`
    Offset      int   `json:"offset"`
    HasNext     bool  `json:"has_next"`
    HasPrevious bool  `json:"has_previous"`
}
```

Computed from `total`, `limit`, `offset`:
```go
HasNext     = offset + limit < total
HasPrevious = offset > 0
```

---

## 5. Filter DSL Integration

Pagination parameters are applied via the Filter DSL at the repository level:

```go
f := filter.And(
    filter.Eq("status", "Draft"),
)
records, pageInfo, err := repo.Query(ctx, f,
    def.WithLimit(20),
    def.WithOffset(40),
    def.WithOrder("created_at DESC"),
)
```

The Filter DSL owns the WHERE clause; pagination and ordering are separate from filter predicates.

---

## 6. Performance Constraints

- `COUNT(*)` is always executed to compute `total`. For very large tables (>10M rows), this may be slow. Consider caching `total` for expensive list views.
- `LIMIT` and `OFFSET` are applied at the database level — never in-memory pagination.
- Sort by indexed fields only. Sorting by unindexed fields on large tables MUST be rejected.

---

## 7. Normative Requirements

- Maximum `limit` is 500. Requests exceeding 500 MUST return HTTP 400.
- Negative `limit` or `offset` values MUST return HTTP 400.
- `total` MUST reflect the count after all filter predicates (including privacy policy filters) are applied.
- Sorting by `Sensitive: true`, `LongText`, or `JSON` fields MUST return HTTP 400.

---

## References

- [`14-api/API_CONVENTIONS.md`](API_CONVENTIONS.md) — List query parameters
- [`06-filter/FILTER_DSL_REFERENCE.md`](../06-filter/FILTER_DSL_REFERENCE.md) — Filter DSL
