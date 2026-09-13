# API Conventions

**Classification:** Specification — Tier 0
**Owner:** `14-api/API_CONVENTIONS.md`
**Status:** Frozen at v1.0

---

## Purpose

This document specifies the URL structure, HTTP methods, response envelope, status codes, and serialisation conventions for all Awo Framework API endpoints. Every API client and every server route handler MUST conform to this specification.

---

## 1. Base URL Structure

```
/api/v1/entities/{entity-type}
```

- `entity-type` is the **qualified entity name** in kebab-case: e.g. `finance-invoice`, `inventory-stock-move`.
- The framework auto-generates all routes from `EntityDefinition`. Do not register entity routes manually.

---

## 2. Standard CRUD Routes

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/entities/{type}` | List records (paginated) |
| `GET` | `/api/v1/entities/{type}/{id}` | Get single record |
| `POST` | `/api/v1/entities/{type}` | Create record |
| `PATCH` | `/api/v1/entities/{type}/{id}` | Update record (partial) |
| `DELETE` | `/api/v1/entities/{type}/{id}` | Delete record |

---

## 3. Custom Action Routes

```
POST /api/v1/entities/{type}/{id}/{action-name}
GET  /api/v1/entities/{type}/{id}/{action-name}   (read-only actions)
```

Examples:
```
POST /api/v1/entities/finance-invoice/550e8400-.../submit
POST /api/v1/entities/finance-invoice/550e8400-.../approve
POST /api/v1/entities/finance-invoice/550e8400-.../cancel
GET  /api/v1/entities/finance-invoice/550e8400-.../preview-pdf
```

`action-name` is the `ActionDef.Name` field in kebab-case.

---

## 4. SDUI Schema Routes

```
GET /api/v1/ui/{entity-type}/list
GET /api/v1/ui/{entity-type}/create
GET /api/v1/ui/{entity-type}/edit
GET /api/v1/ui/{entity-type}/detail
```

Returns amis JSON schema. Cached with 5-minute TTL. Requires authentication and tenant context.

---

## 5. Success Response Envelope

All successful responses use:

```json
{
  "data": { ... },
  "meta": {
    "total":  150,
    "limit":  20,
    "offset": 0
  }
}
```

- `data`: the response payload. Object for single-record responses; array for list responses.
- `meta`: pagination metadata. Present only for list responses. Omitted for single-record and action responses.

### Single Record Response (GET /{id}, POST, PATCH)

```json
{
  "data": {
    "id":     "550e8400-e29b-41d4-a716-446655440000",
    "number": "INV-2026-00001",
    "status": "Draft",
    "total":  "12500.0000"
  }
}
```

### List Response (GET /)

```json
{
  "data": [
    { "id": "550e8400-...", "number": "INV-2026-00001", "status": "Draft" },
    { "id": "660e9500-...", "number": "INV-2026-00002", "status": "Paid" }
  ],
  "meta": {
    "total":  47,
    "limit":  20,
    "offset": 0
  }
}
```

### Action Response (POST /{id}/{action})

```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000"
  },
  "meta": {
    "message": "Invoice submitted for approval."
  }
}
```

When the action starts a workflow, HTTP 202 Accepted is returned:

```json
{
  "data": {
    "id":          "550e8400-e29b-41d4-a716-446655440000",
    "workflow_id": "abc123.finance_invoice.550e8400-....on_submit"
  },
  "meta": {
    "message": "Invoice submitted for approval."
  }
}
```

---

## 6. Error Response Envelope

See [`14-api/ERROR_RESPONSE_FORMAT.md`](ERROR_RESPONSE_FORMAT.md) for the full error specification.

```json
{
  "error": {
    "code":    "invoice.not_draft",
    "message": "Only Draft invoices can be submitted.",
    "fields":  {}
  }
}
```

---

## 7. HTTP Status Codes

| Status | When Used |
|--------|-----------|
| `200 OK` | Successful GET, PATCH, DELETE, and non-workflow actions |
| `201 Created` | Successful POST (record creation) |
| `202 Accepted` | Action started a Temporal workflow |
| `400 Bad Request` | Malformed request body (not JSON, invalid field types) |
| `401 Unauthorized` | Missing or invalid session token |
| `402 Payment Required` | Tenant status is SUSPENDED |
| `403 Forbidden` | Authenticated but lacks permission for the operation |
| `404 Not Found` | Entity record not found by ID |
| `409 Conflict` | Business rule violation (wrong state, duplicate, constraint) |
| `410 Gone` | Tenant status is ARCHIVED |
| `422 Unprocessable Entity` | Validation error (field-level errors returned) |
| `429 Too Many Requests` | Rate limit exceeded |
| `503 Service Unavailable` | Tenant status is PENDING, or dependency failure |

---

## 8. Request Headers

| Header | Required | Description |
|--------|----------|-------------|
| `X-Tenant-ID` | Preferred | Tenant UUID. Takes priority over subdomain and query param. |
| `Authorization` | Required | `Bearer {session-token}` |
| `X-Request-ID` | Recommended | Client-provided request ID for tracing. Generated if absent. |
| `X-Idempotency-Key` | Optional | Idempotency key for POST/PATCH operations. See `IDEMPOTENCY_SPEC.md`. |
| `Content-Type` | Required for POST/PATCH | `application/json` |

---

## 9. List Query Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `limit` | integer | Maximum records to return (default: 20, max: 500) |
| `offset` | integer | Number of records to skip (default: 0) |
| `order` | string | Sort expression: `field_name ASC` or `field_name DESC` |
| `q` | string | Full-text search (applies to `Searchable` fields) |
| `{field_name}` | varies | Field filter: `status=Draft` or `status[]=Draft&status[]=Submitted` |

Filter parameters are passed to the Filter DSL. Only `Searchable: true` fields support `q`. Non-`Searchable` fields support equality and set filters.

---

## 10. Serialisation Rules

| Type | Wire Format |
|------|------------|
| `uuid.UUID` | Standard UUID string with dashes: `"550e8400-e29b-41d4-a716-446655440000"` |
| `decimal.Decimal` (Currency) | String with 4 decimal places: `"12500.0000"` |
| `float64` (Float) | JSON number: `3.14` |
| `int64` (Int) | JSON number: `42` |
| `time.Time` (DateTime) | ISO 8601 with EAT offset for Kenyan tenants: `"2026-07-20T14:30:00+03:00"` |
| `time.Time` (Date) | `"2026-07-20"` |
| `bool` | JSON boolean: `true` / `false` |
| `null` | JSON `null` — used for optional fields |

**Currency serialisation:** `numeric(20,4)` stored value, serialised as 4-decimal string. Do NOT use floats for monetary values at any layer.

**DateTime timezone:** stored UTC in PostgreSQL; serialised with EAT (+03:00) offset for Kenyan tenants. The offset is tenant-configurable via Settings module.

---

## 11. Tenant Identification (Priority Order)

1. `X-Tenant-ID` header (UUID string)
2. Subdomain parsing: `bo.{tenant-slug}.app.example.com` → slug lookup
3. `tenant_id` query parameter (webhooks/legacy only; disabled in production)

---

## References

- [`14-api/IDEMPOTENCY_SPEC.md`](IDEMPOTENCY_SPEC.md) — Idempotency key protocol
- [`14-api/PAGINATION_SPEC.md`](PAGINATION_SPEC.md) — Pagination details
- [`14-api/ERROR_RESPONSE_FORMAT.md`](ERROR_RESPONSE_FORMAT.md) — Error envelope
- [`04-multitenancy/TENANT_IDENTIFICATION.md`](../04-multitenancy/TENANT_IDENTIFICATION.md) — Tenant resolution
