> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Cursor-Based Pagination"
id: pers-009
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Entity Repository](entity-repository.md)"
  - "[Filter DSL](filter-dsl.md)"
  - "[Entity API Reference](../11-api/entity-api-reference.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Cursor-Based Pagination

**PERS-009 | Status: Accepted | Stability: Stable**

Awo uses keyset (cursor-based) pagination for all list queries. This document covers the implementation, cursor format, and performance characteristics.

---

## 1. Why Cursor Pagination

Offset pagination (`LIMIT n OFFSET m`) requires the database to scan and discard `m` rows before returning results. At page 100 with page size 25, that is 2,500 rows scanned. Performance degrades linearly with page depth.

Keyset pagination uses a WHERE clause that filters based on the last seen value:

```sql
-- Instead of: LIMIT 25 OFFSET 500
-- Use: WHERE (created_at, id) < ($last_created_at, $last_id) LIMIT 25
```

This always performs a constant-time B-tree seek regardless of page depth.

---

## 2. Cursor Format

Cursors are opaque base64-encoded JSON tokens:

```go
type paginationCursor struct {
    SortField string `json:"sf"`  // sort field name
    SortValue any    `json:"sv"`  // sort field value at the last record
    ID        string `json:"id"`  // UUID of the last record (tiebreaker)
}
```

Example cursor (decoded):
```json
{"sf": "created_at", "sv": "2024-03-15T08:30:00Z", "id": "01933b2c-..."}
```

The cursor is opaque to clients — they pass it back as-is. Its format may change between API versions.

---

## 3. Query Generation

The repository generates a keyset WHERE clause from the cursor:

```go
func buildKeysetClause(cursor *paginationCursor, direction string) (string, []any) {
    if cursor == nil {
        return "", nil
    }

    op := "<"  // descending (newest first, default)
    if direction == "asc" {
        op = ">"
    }

    // (sort_field, id) < ($sort_value, $last_id)
    // Handles ties via ID comparison
    clause := fmt.Sprintf(
        "(%s, id) %s ($1, $2)",
        pgx.Identifier{cursor.SortField}.Sanitize(),
        op,
    )
    return clause, []any{cursor.SortValue, cursor.ID}
}
```

The sort field is always part of the keyset. `id` (UUID v7, time-ordered) is the secondary sort key that breaks ties within the same timestamp.

---

## 4. QueryOption Usage

```go
// Fetch first page
invoices, pageInfo, err := repo.Query(ctx,
    filter.Eq("status", "Draft"),
    def.WithSort("created_at", "desc"),
    def.WithLimit(25),
)

// Fetch next page using cursor from response
invoices, pageInfo, err := repo.Query(ctx,
    filter.Eq("status", "Draft"),
    def.WithSort("created_at", "desc"),
    def.WithLimit(25),
    def.WithCursor(pageInfo.NextCursor),
)
```

### PageInfo Response

```go
type PageInfo struct {
    NextCursor  string  // empty string if no more pages
    HasMore     bool
    Count       int     // records in this page (not total)
}
```

---

## 5. API Wire Format

```
GET /api/v1/entities/finance_invoice?sort=created_at:desc&limit=25
```

Response:

```json
{
  "data": [ ... ],
  "meta": {
    "cursor": "eyJzZiI6ImNyZWF0ZWRfYXQiLCJzdii6IjIwMjQ...",
    "has_more": true
  }
}
```

Next page:

```
GET /api/v1/entities/finance_invoice?sort=created_at:desc&limit=25&cursor=eyJzZiI6ImNyZWF0ZWRfYXQi...
```

---

## 6. Sort Field Index Requirements

The sort field must have a B-tree index for keyset pagination to be efficient. The framework enforces this:

- `created_at` and `updated_at` are indexed on all system entity tables (auto)
- Custom sort fields must have explicit indexes declared in the migration

```sql
-- For sorting by total_kes
CREATE INDEX CONCURRENTLY ON finance_invoice(total_kes DESC, id DESC);
```

The secondary `id` column in the index prevents page gaps when the sort field has repeated values (many invoices with the same `total_kes`).

---

## 7. Cursor Stability

Cursors are stable within a consistent snapshot — new records inserted during pagination do not appear in the middle of a page sequence. They appear at the beginning of the sequence on the next fresh request (starting from page 1).

This is a fundamental property of keyset pagination: it does not provide snapshot isolation across pages. For audit/export scenarios requiring exactly-once record delivery, use a timestamp-bounded query with explicit `WHERE created_at BETWEEN $start AND $end` instead.

---

## 8. SDUI Integration

amis CRUD components use the cursor automatically — the framework translates amis pagination requests to keyset cursor parameters:

```go
amis.CRUD(amis.CRUDProps{
    API:      "GET /api/v1/entities/finance_invoice",
    PageSize: 25,
    // amis sends "page" parameter; framework middleware translates to cursor
})
```

The translation is handled by the Fiber middleware — module authors do not write cursor translation code.

---

## Related Documents

- [Entity Repository](entity-repository.md) — `WithCursor`, `WithLimit`, `WithSort` QueryOptions
- [Filter DSL](filter-dsl.md) — filters composed with cursor queries
- [Entity API Reference](../11-api/entity-api-reference.md) — cursor in API response meta
- [Performance Tuning](../14-operations/performance-tuning.md) — index requirements for sort fields
