> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "API Pagination Guide"
id: api-007
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors, api-consumers]
since: "1.0"
normative-level: informative
related:
  - "[Entity API Reference](entity-api-reference.md)"
  - "[Cursor-Based Pagination](../05-persistence/cursor-pagination.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# API Pagination Guide

**API-007 | Status: Accepted | Stability: Stable**

How to paginate through entity lists using the Awo API cursor.

---

## 1. Default Behavior

Without pagination parameters, the API returns the first 25 records sorted by `created_at` descending:

```
GET /api/v1/entities/finance_invoice
```

```json
{
  "data": [ /* 25 records */ ],
  "meta": {
    "cursor": "eyJzZiI6ImNyZWF0ZWRfYXQi...",
    "has_more": true
  }
}
```

---

## 2. Fetching Pages

Use the cursor from `meta.cursor` to fetch subsequent pages:

```
GET /api/v1/entities/finance_invoice?cursor=eyJzZiI6ImNyZWF0ZWRfYXQi...
```

When `has_more: false`, there are no more records.

---

## 3. Page Size

Control page size with `limit` (1–100):

```
GET /api/v1/entities/finance_invoice?limit=50
```

---

## 4. Sorting

Sort by any indexed field:

```
GET /api/v1/entities/finance_invoice?sort=total_kes:desc
GET /api/v1/entities/finance_invoice?sort=created_at:asc
```

The cursor is tied to the sort field — use the same sort when fetching subsequent pages:

```
GET /api/v1/entities/finance_invoice?sort=total_kes:desc&cursor=eyJ...
```

Changing sort invalidates the cursor (it will return incorrect results).

---

## 5. Total Count

The API does not return `total_count` by default (avoids COUNT query overhead). Request it explicitly:

```
GET /api/v1/entities/finance_invoice?count=true
```

```json
{
  "data": [ /* 25 records */ ],
  "meta": {
    "cursor": "eyJ...",
    "has_more": true,
    "total_count": 142
  }
}
```

Only request `count=true` when you need to display "Showing 1-25 of 142 records". Don't include it in every poll or background fetch.

---

## 6. Combining with Filters

Filters apply to every page. Pass the same filter with the cursor:

```
# First page
GET /api/v1/entities/finance_invoice?filter={"op":"eq","field":"status","value":"Draft"}

# Next page — same filter + cursor
GET /api/v1/entities/finance_invoice?filter={"op":"eq","field":"status","value":"Draft"}&cursor=eyJ...
```

---

## 7. amis CRUD Pagination

amis CRUD components handle pagination automatically. The framework translates amis `page` parameters to cursor parameters:

```go
amis.CRUD(amis.CRUDProps{
    API:      "GET /api/v1/entities/finance_invoice",
    PageSize: 25,
    // amis sends ?page=2; framework middleware translates to cursor
})
```

No cursor management needed in amis page builders.

---

## 8. Pagination in Go (Repository Layer)

```go
// First page
results, pageInfo, err := repo.Query(ctx,
    filter.Eq("status", "Active"),
    def.WithSort("created_at", "desc"),
    def.WithLimit(25),
)

// Subsequent pages
for pageInfo.HasMore {
    results, pageInfo, err = repo.Query(ctx,
        filter.Eq("status", "Active"),
        def.WithSort("created_at", "desc"),
        def.WithLimit(25),
        def.WithCursor(pageInfo.NextCursor),
    )
    // process results...
}
```

For bulk processing (export, report generation), use this pattern in a Temporal activity — not in a request handler.

---

## 9. Cursor Validity

Cursors are valid for the duration of the session. Cursors may become invalid if:
- The underlying data changes significantly (records deleted, sort field values changed)
- The cursor format changes between API versions (breaking change — announced with major version bump)

On invalid cursor: the API returns HTTP 400 with `{"error": {"code": "invalid_cursor"}}`. Start pagination from the beginning.

---

## Related Documents

- [Entity API Reference](entity-api-reference.md) — `sort`, `limit`, `cursor` parameter spec
- [Cursor-Based Pagination](../05-persistence/cursor-pagination.md) — implementation detail
- [Bulk Operations](bulk-operations.md) — async bulk export instead of exhaustive pagination
