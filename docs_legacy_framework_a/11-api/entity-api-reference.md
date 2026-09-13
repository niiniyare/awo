> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Entity API Reference"
id: api-006
status: accepted
category: REFERENCE
stability: STABLE
audience: [module-authors, api-consumers]
since: "1.0"
normative-level: normative
related:
  - "[API Overview](README.md)"
  - "[Authentication](authentication.md)"
  - "[Bulk Operations](bulk-operations.md)"
  - "[Filter DSL](../05-persistence/filter-dsl.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Entity API Reference

**API-006 | Status: Accepted | Stability: Stable**

Complete reference for the auto-generated Entity CRUD API. All entity types share the same URL pattern and request/response structure.

---

## Base URL

```
/api/v1/entities/{entity-type}
```

`entity-type` is the entity's `Name` field from `EntityDefinition` (snake_case, e.g. `finance_invoice`, `crm_customer`).

---

## Standard CRUD Endpoints

### List

```
GET /api/v1/entities/{entity-type}
```

**Query parameters**:

| Parameter | Type | Description |
|---|---|---|
| `filter` | JSON string | Filter DSL predicate tree (URL-encoded JSON) |
| `sort` | `field:asc\|desc` | Sort field and direction (e.g. `created_at:desc`) |
| `cursor` | string | Pagination cursor from previous response |
| `limit` | int | Page size (1–100, default: 25) |
| `fields` | comma-separated | Limit returned fields (e.g. `id,name,status`) |
| `with` | comma-separated | Load edges explicitly (e.g. `with=lines,customer`) |
| `search` | string | Full-text search on `Searchable` fields |

**Response**:

```json
{
  "data": [
    {
      "id": "01933b2c-...",
      "number": "INV-2024-00001",
      "status": "Draft",
      "total_kes": "15000.0000",
      "created_at": "2024-03-15T08:30:00+03:00",
      "updated_at": "2024-03-15T08:30:00+03:00"
    }
  ],
  "meta": {
    "cursor": "eyJpZCI6IjAx...",
    "has_more": true,
    "total_count": 142
  }
}
```

`total_count` is included only when `count=true` query parameter is provided (adds COUNT query overhead).

---

### Get Single

```
GET /api/v1/entities/{entity-type}/{id}
```

**Query parameters**: `fields`, `with` (same as List).

**Response**:

```json
{
  "data": {
    "id": "01933b2c-...",
    "number": "INV-2024-00001",
    "customer": "01922a1b-...",
    "status": "Draft",
    "total_kes": "15000.0000",
    "lines": [
      {
        "id": "01933b3d-...",
        "description": "Consulting services",
        "quantity": 10,
        "unit_price": "1500.0000",
        "total": "15000.0000"
      }
    ],
    "created_at": "2024-03-15T08:30:00+03:00",
    "updated_at": "2024-03-15T08:30:00+03:00"
  }
}
```

Lines included because `with=lines` was specified (or are eagerly loaded for this entity).

**Error responses**:
- `404` — entity not found or not accessible to caller (PolicyFunc filtered it out)
- `403` — actor lacks `Read` permission on this entity type

---

### Create

```
POST /api/v1/entities/{entity-type}
```

**Request body**:

```json
{
  "customer": "01922a1b-...",
  "status": "Draft",
  "total_kes": "15000.0000",
  "notes": "Q1 consulting services"
}
```

Required fields from `FieldDef.Required: true` must be provided. `Immutable` fields can be set on create but not updated later.

**Response**: `201 Created`

```json
{
  "data": {
    "id": "01933b2c-...",
    "number": "INV-2024-00001",
    "customer": "01922a1b-...",
    "status": "Draft",
    "total_kes": "15000.0000",
    "created_at": "2024-03-15T08:30:00+03:00",
    "updated_at": "2024-03-15T08:30:00+03:00"
  }
}
```

If a `WorkflowTrigger` is declared for `EventOnCreate`:

```json
{
  "data": { "id": "...", ... },
  "workflow_id": "tenant-uuid.finance_invoice.01933b2c.on_create"
}
```

**Error responses**:
- `422` — validation error (field-level messages)
- `400` — business rule violation
- `403` — actor lacks `Create` permission
- `409` — unique constraint violation

---

### Update

```
PATCH /api/v1/entities/{entity-type}/{id}
```

Partial update — only fields present in the request body are modified.

**Request body**:

```json
{
  "status": "Submitted",
  "notes": "Updated payment terms"
}
```

`Immutable` fields in the request body are rejected with `422`.

**Response**: `200 OK` — full updated entity record.

---

### Delete

```
DELETE /api/v1/entities/{entity-type}/{id}
```

**Response**: `204 No Content`

Soft delete behavior: if the entity has a `deleted_at` field, it is set instead of physically deleting the row. Hard delete otherwise.

**Error responses**:
- `404` — not found
- `403` — actor lacks `Delete` permission
- `409` — referential integrity violation (child records exist with no cascade delete)

---

## Custom Actions

```
POST /api/v1/entities/{entity-type}/{id}/{action-name}
```

Declared via `ActionDef` on the entity def.

**Request body**: action-specific payload (declared by `HandlerFunc`).

**Response**: `200 OK` (synchronous) or `202 Accepted` (workflow-triggered)

```json
{
  "data": {
    "message": "Invoice submitted for approval",
    "id": "01933b2c-..."
  },
  "workflow_id": "tenant-uuid.finance_invoice.01933b2c.on_submit"
}
```

---

## Filter DSL via API

Filters are sent as URL-encoded JSON:

```
GET /api/v1/entities/finance_invoice?filter={"op":"and","conditions":[{"field":"status","op":"eq","value":"Draft"},{"field":"total_kes","op":"gte","value":"10000"}]}
```

Filter wire format:

```json
{
  "op": "and",
  "conditions": [
    { "field": "status", "op": "eq", "value": "Draft" },
    { "field": "total_kes", "op": "gte", "value": "10000" },
    {
      "op": "or",
      "conditions": [
        { "field": "assigned_to", "op": "eq", "value": "user-uuid" },
        { "field": "department", "op": "in", "value": ["dept-1", "dept-2"] }
      ]
    }
  ]
}
```

Supported `op` values: `eq`, `neq`, `lt`, `lte`, `gt`, `gte`, `in`, `nin`, `null`, `notnull`, `like`, `and`, `or`, `not`.

PolicyFunc is always applied in addition to the client's filter — clients cannot bypass row-level security via filter manipulation.

---

## Response Envelope

All successful responses:

```json
{
  "data": { ... },
  "meta": { ... }
}
```

All error responses:

```json
{
  "error": {
    "code": "validation_error",
    "message": "One or more fields are invalid",
    "fields": {
      "total_kes": "Must be greater than zero",
      "customer": "Required"
    }
  }
}
```

Error codes:

| Code | HTTP status | Meaning |
|---|---|---|
| `validation_error` | 422 | Field-level validation failures |
| `not_found` | 404 | Entity not found |
| `forbidden` | 403 | Permission denied |
| `conflict` | 409 | Unique constraint or state machine violation |
| `bad_request` | 400 | Business rule violation |
| `internal_error` | 500 | Unexpected error (details in server logs only) |

---

## Headers

### Request

| Header | Required | Description |
|---|---|---|
| `X-Tenant-ID` | Yes (for API clients) | Tenant UUID |
| `Authorization` | Yes | `Bearer {session_token}` |
| `Content-Type` | Required for POST/PATCH | `application/json` |
| `X-Request-ID` | Optional | Caller-provided trace ID |

### Response

| Header | Description |
|---|---|
| `X-Request-ID` | Echo of request ID (or generated UUID) |
| `X-RateLimit-Limit` | Requests allowed per window |
| `X-RateLimit-Remaining` | Requests remaining in current window |
| `X-RateLimit-Reset` | Unix timestamp when window resets |

---

## DateTime Format

All `DateTime` and `Date` fields are serialized in ISO 8601 format:

- `DateTime`: `"2024-03-15T08:30:00+03:00"` (EAT offset for Kenyan tenants)
- `Date`: `"2024-03-15"`

Input parsing accepts both UTC (`Z` suffix) and offset formats.

---

## Currency Serialization

`Currency` fields serialize as numeric strings with 4 decimal places:

```json
"total_kes": "15000.0000"
```

The `_display` variant includes locale formatting (read-only, not accepted on input):

```json
"total_kes": "15000.0000",
"total_kes_display": "KES 15,000.00"
```

---

## Related Documents

- [Authentication](authentication.md) — token acquisition and session management
- [Bulk Operations](bulk-operations.md) — bulk create, update, delete
- [Webhooks](webhooks.md) — event subscriptions for entity mutations
- [Rate Limiting](rate-limiting.md) — limits and retry-after behavior
- [Filter DSL](../05-persistence/filter-dsl.md) — full predicate reference
