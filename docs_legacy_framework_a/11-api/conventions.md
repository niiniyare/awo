> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "API Conventions"
id: api-001
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Error Handling](error-handling.md)"
  - "[EntityDefinition](../03-kernel/entity-def.md)"
  - "[RBAC](../07-iam/rbac.md)"
  - "[Sessions](../07-iam/sessions.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# API Conventions

**API-001 | Status: Accepted | Stability: Stable**

This document specifies URL structure, request/response formats, pagination, field serialization, and middleware pipeline integration for the Awo API.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. URL Structure

### Auto-Generated CRUD Routes

Every registered `EntityDefinition` produces the following routes automatically:

```
GET    /api/v1/entities/{entity-name}           — list (paginated)
GET    /api/v1/entities/{entity-name}/{id}       — get single record
POST   /api/v1/entities/{entity-name}            — create
PATCH  /api/v1/entities/{entity-name}/{id}       — update (partial)
DELETE /api/v1/entities/{entity-name}/{id}       — delete
```

Custom actions declared in `EntityDefinition.Actions`:

```
POST   /api/v1/entities/{entity-name}/{id}/{action-name}
```

### Schema Routes (SDUI)

```
GET    /api/v1/schemas/{entity-name}/list
GET    /api/v1/schemas/{entity-name}/create
GET    /api/v1/schemas/{entity-name}/edit
GET    /api/v1/schemas/{entity-name}/detail
```

### Platform Routes

```
POST   /api/v1/auth/login
POST   /api/v1/auth/logout
POST   /api/v1/auth/refresh
GET    /api/v1/auth/me

GET    /api/v1/tenants              — platform admin only
POST   /api/v1/tenants
GET    /api/v1/tenants/{id}
PATCH  /api/v1/tenants/{id}

GET    /health/live
GET    /health/ready
GET    /metrics                     — Prometheus scrape endpoint
```

---

## 2. Request Format

### Content Type

All request bodies MUST use `Content-Type: application/json`. Multipart form data is used only for file uploads via dedicated upload endpoints.

### Required Headers

| Header | Required | Description |
|---|---|---|
| `X-Tenant-ID` | Yes (for tenant-scoped requests) | Tenant UUID |
| `Authorization` | Yes (for authenticated requests) | `Bearer {session-token}` |
| `X-Request-ID` | Recommended | Client-generated trace ID; returned in response |
| `Content-Type` | Yes (POST/PATCH) | `application/json` |

### List Request — Query Parameters

```
GET /api/v1/entities/finance_invoice
    ?page=1              — page number (1-indexed), default 1
    &page_size=20        — records per page, default 20, max 100
    &order_by=created_at — field to sort by
    &order_dir=desc      — asc or desc
    &search=acme         — full-text search (entity must declare Searchable fields)
    &filter={...}        — Filter DSL JSON (URL-encoded)
```

---

## 3. Response Envelope

All API responses MUST use the standard envelope format.

### Success (single record)

```json
{
  "data": {
    "id": "018e1b2c-...",
    "tenant_id": "018d9f3a-...",
    "number": "INV-2024-00001",
    "customer": "018c7b1d-...",
    "status": "Draft",
    "total_kes": "45000.0000",
    "created_at": "2024-03-15T09:30:00+03:00",
    "updated_at": "2024-03-15T09:30:00+03:00"
  },
  "meta": {
    "request_id": "req-abc123"
  }
}
```

### Success (list)

```json
{
  "data": [...],
  "meta": {
    "request_id": "req-abc123",
    "page": 1,
    "page_size": 20,
    "total_count": 143,
    "has_next_page": true,
    "next_cursor": "eyJpZCI6Ii4uLiJ9"
  }
}
```

### Workflow-Triggered Response (HTTP 202)

When a create or action triggers a Temporal workflow:

```json
{
  "data": {
    "id": "018e1b2c-..."
  },
  "meta": {
    "request_id": "req-abc123",
    "workflow_id": "018d9f3a.finance_invoice.018e1b2c.on_create.InvoiceCreationWorkflow",
    "accepted": true
  }
}
```

HTTP 202 signals the operation is durable but asynchronous. The workflow ID can be used to query status.

---

## 4. Field Serialization

### UUIDs

All UUIDs serialized as lowercase hyphenated strings: `"018e1b2c-3d4e-7f89-abcd-ef0123456789"`

### DateTime

DateTime fields are stored as UTC in PostgreSQL and serialized with EAT offset (+03:00) for Kenyan tenants:

```json
"created_at": "2024-03-15T09:30:00+03:00"
```

The offset is determined by the tenant's `timezone` setting. Non-Kenyan tenants receive their configured timezone offset.

### Currency

Currency fields are serialized as:
- **Raw value** (for data manipulation): `"45000.0000"` — 4 decimal places, string type
- **Display value** (for SDUI): `"KES 45,000.00"` — locale-formatted, in separate `{field}_display` key

```json
{
  "total_kes": "45000.0000",
  "total_kes_display": "KES 45,000.00"
}
```

The display key is included when the request includes `Accept: application/vnd.awo.display+json`. Standard requests receive only the raw value.

### Boolean

`true` / `false` — never `1` / `0`.

### Date

`"2024-03-15"` — ISO 8601 date only.

### Time

`"14:30:00"` — 24-hour HH:MM:SS.

### Select / MultiSelect

Select fields serialized as their stored string value: `"Draft"`, `"Submitted"`.

MultiSelect fields serialized as JSON arrays: `["tag1", "tag2"]`.

### NamingSeries

Read-only; serialized as string: `"INV-2024-00001"`. Not accepted in create/update bodies.

### Link

Serialized as UUID string of the referenced record. The linked record's display label is included as `{field}_label`:

```json
{
  "customer": "018c7b1d-...",
  "customer_label": "Acme Corp"
}
```

---

## 5. Pagination

List responses support two pagination modes:

### Page-Number Pagination (default)

```json
{
  "meta": {
    "page": 1,
    "page_size": 20,
    "total_count": 143,
    "has_next_page": true
  }
}
```

Suitable for admin UIs with page navigation.

### Cursor-Based Pagination

Request with `cursor` parameter to skip page-number calculation (no `COUNT(*)` query):

```
GET /api/v1/entities/finance_invoice?cursor=eyJpZCI6Ii4uLiJ9&page_size=20
```

Response:

```json
{
  "meta": {
    "next_cursor": "eyJpZCI6InguLi4ifQ==",
    "has_next_page": true
  }
}
```

Cursor is a base64-encoded position. Clients MUST NOT decode or construct cursors — treat as opaque.

Use cursor pagination for: infinite scroll, high-volume data exports, keyset-based iteration.

---

## 6. Request Context

Every request handler receives a pre-resolved context with:

```go
// Available via context extraction:
requestID := c.Locals("request_id").(string)
tenantID  := c.Locals("tenant_id").(uuid.UUID)
actor     := c.Locals("actor").(session.Actor)
```

These values are set by middleware before any route handler executes. Route handlers MUST NOT re-resolve tenant or actor — the middleware pipeline guarantees they are present for authenticated routes.

---

## 7. Custom Handler Constraints

Custom HTTP handlers (written when auto-generated CRUD is insufficient) MUST follow these rules:

- Maximum 50 lines per handler function
- No business logic in handler — delegate to service layer or action handler
- No direct database calls — use `EntityRepository` only
- Always return the standard response envelope
- Always log with `request_id`, `tenant_id`, `user_id`

```go
// Correct: thin handler
func GetInvoiceSummaryHandler(c *fiber.Ctx) error {
    tenantID := c.Locals("tenant_id").(uuid.UUID)
    actor    := c.Locals("actor").(session.Actor)
    invoiceID, err := uuid.Parse(c.Params("id"))
    if err != nil {
        return fiber.NewError(fiber.StatusBadRequest, "invalid invoice ID")
    }

    summary, err := invoiceService.GetSummary(c.Context(), tenantID, actor, invoiceID)
    if err != nil {
        return mapEntityError(err)
    }

    return c.JSON(Response{Data: summary})
}
```

---

## 8. Versioning

The API version is embedded in the URL: `/api/v1/`. Breaking changes require a new version prefix (`/api/v2/`). The framework supports running multiple API versions simultaneously during migration periods.

Non-breaking additions (new optional fields, new endpoints) do not require a version bump. Breaking changes include: removing fields, changing field types, changing URL structure, changing required headers.

---

## Related Documents

- [Error Handling](error-handling.md) — error envelope format and HTTP status codes
- [EntityDefinition](../03-kernel/entity-def.md) — what drives auto-generated routes
- [RBAC](../07-iam/rbac.md) — permission checking in route handlers
- [Sessions](../07-iam/sessions.md) — session validation and actor resolution
- [Filter DSL](../05-persistence/filter-dsl.md) — list endpoint filtering
- [Glossary](../GLOSSARY.md) — API Layer, Response Envelope, Cursor Pagination
