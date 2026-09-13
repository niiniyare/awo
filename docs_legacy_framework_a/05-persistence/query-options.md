> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Query Options"
id: pers-011
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Entity Repository](entity-repository.md)"
  - "[Filter DSL](filter-dsl.md)"
  - "[Cursor-Based Pagination](cursor-pagination.md)"
  - "[Edges](../04-domain/edges.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Query Options

**PERS-011 | Status: Accepted | Stability: Stable**

Complete reference for `QueryOption` values accepted by `repo.Query`, `repo.Get`, and related methods.

---

## 1. Overview

`QueryOption` functions modify the query behavior beyond the base filter predicate. They are passed as variadic arguments:

```go
results, pageInfo, err := repo.Query(ctx,
    filter.Eq("status", "Active"),
    def.WithSort("created_at", "desc"),
    def.WithLimit(25),
    def.WithEdge("customer"),
    def.WithEdge("lines"),
)
```

Options are additive — multiple options of the same type may have defined behavior (e.g., multiple `WithEdge` calls add to the list of edges to load).

---

## 2. Pagination Options

### WithLimit

```go
def.WithLimit(n int) QueryOption
```

Sets the maximum number of records to return. Range: 1–100. Default: 25. Values outside this range are clamped.

### WithCursor

```go
def.WithCursor(cursor string) QueryOption
```

Sets the pagination cursor from a previous `PageInfo.NextCursor`. An empty string or omitting `WithCursor` returns the first page.

### WithOffset

```go
def.WithOffset(offset int) QueryOption
```

Offset-based pagination (for internal use only — not exposed via API). Use `WithCursor` for API-driven pagination.

---

## 3. Sorting Options

### WithSort

```go
def.WithSort(field string, direction string) QueryOption
// direction: "asc" or "desc"
```

Sets the primary sort field and direction. The sort field must have a B-tree index for efficient pagination.

Multiple `WithSort` calls add secondary sort keys:

```go
def.WithSort("status", "asc"),
def.WithSort("created_at", "desc"),
// SQL: ORDER BY status ASC, created_at DESC, id DESC
```

`id` is always appended as the final tiebreaker (keyset pagination requirement).

---

## 4. Edge Loading Options

### WithEdge

```go
def.WithEdge(edgeName string) QueryOption
```

Loads the named edge alongside the primary record(s). Multiple calls load multiple edges.

```go
// Load one-to-many "lines" and many-to-one "customer"
def.WithEdge("lines"),
def.WithEdge("customer"),
```

Edge loading issues one additional query per edge type (not per record). For `WithEdge("lines")` on 25 invoices, the framework issues ONE query: `SELECT * FROM invoice_line WHERE invoice_id = ANY($1)` where `$1` is the array of invoice IDs.

### WithEdgeFilter

```go
def.WithEdgeFilter(edgeName string, f filter.Filter) QueryOption
```

Loads a filtered subset of a one-to-many edge:

```go
// Load only active invoice lines
def.WithEdgeFilter("lines", filter.Eq("active", true))
```

### WithEdgeSort

```go
def.WithEdgeSort(edgeName string, field string, direction string) QueryOption
```

Sorts the loaded edge records:

```go
def.WithEdge("lines"),
def.WithEdgeSort("lines", "line_number", "asc"),
```

---

## 5. Field Selection Options

### WithFields

```go
def.WithFields(fieldNames ...string) QueryOption
```

Limits the fields returned in each record. Useful for list views that don't need all fields:

```go
// Only return id, name, status — reduces data transfer
def.WithFields("id", "name", "status", "created_at")
```

Always include `id` — omitting it breaks pagination and edge loading.

### WithSensitiveFields

```go
def.WithSensitiveFields() QueryOption
```

Includes fields marked `Sensitive: true` in the response. Requires the actor to have a specific platform-level permission. Used only in platform admin contexts.

---

## 6. Count Options

### WithCount

```go
def.WithCount() QueryOption
```

Includes `TotalCount` in the returned `PageInfo`. Adds a COUNT query overhead. Only use when the total count is needed (e.g., for "Showing 1-25 of 142 records" UI).

```go
results, pageInfo, err := repo.Query(ctx, filter.All(), def.WithCount())
// pageInfo.TotalCount = 142
```

---

## 7. Lock Options

### WithForUpdate

```go
def.WithForUpdate() QueryOption
```

Adds `SELECT ... FOR UPDATE` to the query. Prevents concurrent modification of the returned rows within the same transaction. Use for optimistic locking patterns:

```go
err = repo.WithTx(ctx, func(ctx context.Context, txRepo EntityRepository[Invoice]) error {
    invoice, _ := txRepo.Get(ctx, invoiceID, def.WithForUpdate())
    // invoice is locked — concurrent requests wait
    if invoice.Fields["status"] != "Draft" {
        return &def.BusinessError{Code: "invoice.not_draft", Status: 409}
    }
    txRepo.Update(ctx, invoiceID, def.UpdateInput{Fields: map[string]any{"status": "Submitted"}})
    return nil
})
```

---

## 8. Soft Delete Options

### WithDeleted

```go
def.WithDeleted() QueryOption
```

Includes soft-deleted records (where `deleted_at IS NOT NULL`) in the query results. By default, soft-deleted records are invisible.

### WithDeletedOnly

```go
def.WithDeletedOnly() QueryOption
```

Returns only soft-deleted records. Useful for trash/restore views.

---

## 9. Get vs Query

`repo.Get(ctx, id, opts...)` accepts most QueryOption values:

```go
invoice, err := repo.Get(ctx, invoiceID,
    def.WithEdge("lines"),
    def.WithEdge("customer"),
    def.WithFields("id", "number", "status", "total_kes"),
)
```

Options not applicable to `Get` (pagination, sorting): silently ignored.

---

## Related Documents

- [Entity Repository](entity-repository.md) — method signatures that accept QueryOptions
- [Filter DSL](filter-dsl.md) — predicates used alongside QueryOptions
- [Cursor-Based Pagination](cursor-pagination.md) — `WithCursor` and `WithLimit` detail
- [Edges](../04-domain/edges.md) — `WithEdge` edge loading vs lazy loading
