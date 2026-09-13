> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Cursors and Pagination"
id: pers-005
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[EntityRepository](entity-repository.md)"
  - "[Filter DSL](filter-dsl.md)"
  - "[API Conventions](../11-api/conventions.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Cursors and Pagination

**PERS-005 | Status: Accepted | Stability: Stable**

This document specifies the two pagination modes — page-number and cursor-based — their implementation, trade-offs, and correct usage.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Two Pagination Modes

### Page-Number Pagination

```go
results, pageInfo, err := repo.Query(ctx, filter,
    entity.OrderBy("created_at", entity.Desc),
    entity.Page(3),
    entity.PageSize(20),
)
// pageInfo.TotalCount = 143
// pageInfo.HasNextPage = true
// pageInfo.Page = 3
```

SQL equivalent:
```sql
SELECT *, COUNT(*) OVER() AS total_count
FROM finance_invoice
WHERE tenant_id = current_tenant_id()
ORDER BY created_at DESC
LIMIT 20 OFFSET 40
```

Pros: Total count available; supports "page N of M" UI.

Cons: `COUNT(*) OVER()` is expensive on large tables. `OFFSET` performance degrades on deep pages (OFFSET 10000 requires scanning 10000 rows to discard).

### Cursor-Based Pagination

```go
results, pageInfo, err := repo.Query(ctx, filter,
    entity.OrderBy("created_at", entity.Desc),
    entity.WithCursor("eyJpZCI6Ii4uLiJ9"),  // opaque cursor
    entity.PageSize(20),
)
// pageInfo.NextCursor = "eyJpZCI6InguLi4ifQ=="
// pageInfo.HasNextPage = true
// pageInfo.TotalCount = -1  // not computed in cursor mode
```

SQL equivalent (keyset pagination):
```sql
SELECT *
FROM finance_invoice
WHERE tenant_id = current_tenant_id()
  AND (created_at, id) < ($cursor_created_at, $cursor_id)  -- keyset condition
ORDER BY created_at DESC, id DESC
LIMIT 20
```

Pros: Constant performance regardless of page depth; no `COUNT(*)` overhead.

Cons: No total count; no random page access (must traverse sequentially).

---

## 2. Cursor Structure

Cursors are opaque base64-encoded JSON. Clients MUST NOT decode, construct, or manipulate cursors. The cursor structure is an implementation detail.

Internal structure:
```json
{
  "order_field": "created_at",
  "order_dir": "desc",
  "values": {
    "created_at": "2024-03-15T09:30:00Z",
    "id": "018e1b2c-3d4e-7f89-abcd-ef0123456789"
  }
}
```

The cursor encodes the sort field values and the primary key of the last record on the current page. The next page starts at the keyset position immediately after these values.

`id` is always included as a tiebreaker. This ensures correct pagination when multiple records share the same value for the primary sort field.

---

## 3. Cursor Stability

Cursors are stable for the lifetime of the dataset being paginated:
- If new records are inserted before the cursor position, they appear on earlier pages (not disrupting cursor navigation)
- If records are deleted before the cursor position, subsequent pages are unaffected (keyset condition skips the gap)
- If the sort order changes between cursor calls, the cursor MUST be discarded and pagination restarted

Cursors have a 1-hour validity window encoded in the cursor. Expired cursors return HTTP 400.

---

## 4. When to Use Each Mode

| Use Case | Mode |
|---|---|
| Admin list view with "Page 1 of 8" navigation | Page-number |
| Infinite scroll on a mobile view | Cursor |
| Data export (iterate all records) | Cursor |
| Dashboard with total counts | Page-number |
| Large dataset (>10k records per page set) | Cursor |

Default: page-number pagination unless the use case clearly requires cursor pagination.

---

## 5. PageInfo Response

Both modes return `PageInfo`:

```go
type PageInfo struct {
    // Populated in both modes
    HasNextPage bool
    HasPrevPage bool

    // Page-number mode only (-1 in cursor mode)
    Page       int
    PageSize   int
    TotalCount int

    // Cursor mode only (empty string in page-number mode)
    NextCursor string
    PrevCursor string
}
```

---

## 6. API Wire Format

Page-number request:
```
GET /api/v1/entities/finance_invoice?page=3&page_size=20&order_by=created_at&order_dir=desc
```

Cursor request:
```
GET /api/v1/entities/finance_invoice?cursor=eyJpZCI6Ii4uLiJ9&page_size=20
```

Specifying both `page` and `cursor` is an error — returns HTTP 400.

---

## Related Documents

- [EntityRepository](entity-repository.md) — `WithCursor`, `Page`, `PageSize` QueryOptions
- [API Conventions](../11-api/conventions.md) — pagination in the API response envelope
- [Filter DSL](filter-dsl.md) — filters combined with pagination
- [Glossary](../GLOSSARY.md) — Cursor Pagination, Page-Number Pagination, PageInfo
