> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Bulk Operations"
id: api-003
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[API Conventions](conventions.md)"
  - "[EntityRepository](../05-persistence/entity-repository.md)"
  - "[Error Handling](error-handling.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Bulk Operations

**API-003 | Status: Accepted | Stability: Stable**

This document specifies bulk create, bulk update, and bulk delete operations in the Awo API: request format, response format, atomicity guarantees, and size limits.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Bulk Create

```
POST /api/v1/entities/{entity-type}/bulk
Content-Type: application/json

{
  "items": [
    {"field1": "value1", "field2": "value2"},
    {"field1": "value3", "field2": "value4"}
  ]
}
```

**Response: HTTP 207 Multi-Status**

```json
{
  "data": {
    "results": [
      {"status": 201, "id": "018e1b2c-..."},
      {"status": 422, "error": {"code": "validation_error", "fields": {"field1": "Required."}}}
    ],
    "succeeded": 1,
    "failed": 1
  }
}
```

Bulk create is **not atomic** — each item is processed independently. Items that succeed are committed; items that fail return errors. This allows partial success when importing large datasets.

If all-or-nothing semantics are required, use the standard `POST /` endpoint in a loop within a client-side transaction, or use a custom action with `WithTx`.

### Size Limit

Bulk create requests MUST NOT exceed 500 items per request. Requests exceeding this limit return HTTP 400:

```json
{"error": {"code": "bulk_limit_exceeded", "message": "Maximum 500 items per bulk request."}}
```

---

## 2. Bulk Update

```
PATCH /api/v1/entities/{entity-type}/bulk
Content-Type: application/json

{
  "filter": {"status": "Draft", "created_at_lte": "2024-01-01"},
  "patch": {"status": "Archived"}
}
```

**Response: HTTP 200**

```json
{
  "data": {
    "updated": 47
  }
}
```

Bulk update applies a patch to all records matching the filter. It is **atomic** — all matched records are updated in a single SQL statement:

```sql
UPDATE finance_invoice
SET status = 'Archived', updated_at = NOW()
WHERE tenant_id = current_tenant_id()
  AND status = 'Draft'
  AND created_at <= '2024-01-01'
```

### Bulk Update Constraints

- The patch MUST NOT include `NamingSeries` fields (immutable after assignment)
- The patch MUST NOT include `Immutable` fields
- Fields in the patch are validated against the entity's field definitions
- `before_validate` and `before_save` hooks are **not called** for bulk updates — use only when hooks can be safely skipped
- `after_save` hook is **not called** for bulk updates
- Audit log records one bulk-update entry (not per-row entries)

### Permission Check

The actor MUST have `write` permission on the entity type. Row-level permissions (PolicyFunc) are enforced by RLS on the underlying SQL statement — rows the actor cannot see are not updated.

---

## 3. Bulk Delete

```
DELETE /api/v1/entities/{entity-type}/bulk
Content-Type: application/json

{
  "ids": [
    "018e1b2c-3d4e-7f89-abcd-ef0123456789",
    "018e1b2c-3d4e-7f89-abcd-ef0123456790"
  ]
}
```

**Response: HTTP 200**

```json
{
  "data": {
    "deleted": 2,
    "not_found": 0,
    "unauthorized": 0
  }
}
```

Bulk delete is **atomic** — all deletions occur in a single transaction. If any deletion fails due to a FK constraint, the entire operation is rolled back and returns HTTP 409.

### Bulk Delete Constraints

- Maximum 100 IDs per request (stricter than bulk create due to FK constraint checking)
- IDs not belonging to the current tenant silently return as `not_found` (not an error — prevents cross-tenant probing)
- IDs the actor lacks `delete` permission for return as `unauthorized` without revealing whether the record exists

---

## 4. Bulk Import (File Upload)

For importing entities from CSV or XLSX:

```
POST /api/v1/entities/{entity-type}/import
Content-Type: multipart/form-data

file: <CSV or XLSX file>
options: {"on_duplicate": "skip"}   // or "update" or "error"
```

**Response: HTTP 202 Accepted**

```json
{
  "data": {
    "import_id": "018e1b2c-...",
    "workflow_id": "tenant-uuid.finance_invoice.import.018e1b2c"
  }
}
```

Import is always asynchronous — the file is processed by a Temporal workflow. The response includes a `workflow_id` that can be polled for status:

```
GET /api/v1/entities/{entity-type}/import/{import_id}/status
```

```json
{
  "data": {
    "state":     "processing",
    "processed": 234,
    "total":     500,
    "succeeded": 230,
    "failed":    4,
    "errors": [
      {"row": 12, "error": "Customer 'UNKNOWN' not found."},
      {"row": 47, "error": "total_kes must be greater than 0."}
    ]
  }
}
```

Import workflows apply the same hooks, validation, and audit logging as single-record creates.

---

## 5. EntityRepository Bulk Methods

Module authors call bulk operations through the repository interface:

```go
// BulkCreate — not atomic, returns per-item results
results, err := repo.BulkCreate(ctx, []entity.CreateInput{
    {Fields: map[string]any{"name": "Alice", "email": "alice@example.com"}},
    {Fields: map[string]any{"name": "Bob",   "email": "bob@example.com"}},
})
// results[i].Error != nil if item i failed

// BulkUpdate — atomic, returns count
count, err := repo.BulkUpdate(ctx,
    filter.And(filter.Eq("status", "Draft"), filter.Lt("created_at", cutoffDate)),
    entity.Patch{"status": "Archived"},
)

// BulkDelete via filter (use with extreme care)
// Not exposed on EntityRepository interface — use Delete in loop within WithTx
// or use a custom SQL migration for one-time bulk cleanup
```

---

## 6. Atomicity Summary

| Operation | Atomic? | Hooks Called? | Audit Log |
|---|---|---|---|
| Bulk Create | No (per-item) | Yes (per-item) | Per-item entry |
| Bulk Update | Yes | No | One bulk entry |
| Bulk Delete | Yes | No | One bulk entry |
| Bulk Import | No (per-row, in workflow) | Yes (per-row) | Per-row entry |

---

## 7. Error Response Format

Bulk endpoints that support partial success return HTTP 207 with per-item results. Bulk endpoints with atomic semantics return a single error on failure:

```json
{
  "error": {
    "code": "bulk_fk_constraint",
    "message": "One or more records could not be deleted due to related records.",
    "detail": "Cascade delete is not enabled on this entity."
  }
}
```

---

## Related Documents

- [API Conventions](conventions.md) — standard CRUD endpoints, response envelope
- [EntityRepository](../05-persistence/entity-repository.md) — `BulkCreate`, `BulkUpdate` interface specification
- [Error Handling](error-handling.md) — error envelope format
- [Hooks](../04-domain/hooks.md) — which hooks run during bulk operations
- [Glossary](../GLOSSARY.md) — EntityRepository, BulkCreate, BulkUpdate
