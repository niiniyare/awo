# Chapter 25 — API Contracts

> **Volume:** VI — Platform APIs & Configuration
> **Phase:** 3
> **Audience:** Backend Engineers, Platform Engineers, Mobile Engineers, Web Engineers
> **Prerequisites:** Chapters 03–08, 22–24

---

## Table of Contents

- [25.1 API Design Principles](#251-api-design-principles)
- [25.2 Surface Definition API](#252-surface-definition-api)
- [25.3 Data Source API Conventions](#253-data-source-api-conventions)
- [25.4 Mutation API Conventions](#254-mutation-api-conventions)
- [25.5 Authentication and Session APIs](#255-authentication-and-session-apis)
- [25.6 Streaming APIs](#256-streaming-apis)
- [25.7 Versioning and Compatibility](#257-versioning-and-compatibility)
- [25.8 Error Response Conventions](#258-error-response-conventions)
- [25.9 API Documentation and Schema](#259-api-documentation-and-schema)
- [25.10 gRPC API Contract](#2510-grpc-api-contract)

---

## 25.1 API Design Principles

**REST for resources, gRPC for streaming.** The AwoERP platform uses REST (over HTTPS) for surface definition fetches, data source queries, and mutations. It uses gRPC server streaming for real-time event subscriptions on mobile clients. WebSocket is used for bidirectional real-time flows (collaborative editing, terminal).

**Stable URLs.** Surface URLs and data source endpoint URLs are stable identifiers. Once published, a URL does not change meaning. Backward-incompatible changes introduce a new URL. This stability is required because URLs are stored in navigation definitions, deep links, bookmarks, and audit logs.

**Consistent envelope.** All API responses use the same envelope shape so that the rendering engine's network layer can handle errors, pagination metadata, and headers uniformly.

**Tenant-first.** Every API endpoint operates in the context of a tenant, declared via the `X-Tenant-ID` header. There are no tenant-neutral endpoints (except authentication endpoints that precede tenant context establishment).

---

## 25.2 Surface Definition API

### 25.2.1 Fetch Surface Definition

```
GET /api/sdui/surfaces/{surface_id}
Authorization: Bearer {token}
X-Tenant-ID: {tenant_id}
Accept-Language: {locale}
X-Platform: web | mobile_ios | mobile_android
X-Client-Version: {semver}
X-Capability-Hash: {hash}
If-None-Match: {etag}     ← conditional request; omit on first fetch
```

**Success response (200):**
```json
{
  "surface_id": "invoice_list",
  "version": "1.47.0",
  "schema_version": "2.3",
  "compiled_at": "2024-03-15T10:32:00Z",
  "etag": "\"sha256:abc123\"",
  "cache_ttl_seconds": 30,
  "root": { ... },           ← root ComponentNode
  "data_sources": [ ... ],
  "state": [ ... ],
  "actions": { ... }
}
```

**Not modified (304):** When `If-None-Match` matches the current ETag, the server returns `304 Not Modified` with no body. The rendering engine serves the cached definition.

**404:** Surface ID does not exist or is not provisioned for this tenant.

**403:** User does not have permission to access this surface.

### 25.2.2 List Available Surfaces

```
GET /api/sdui/surfaces?category={category}
Authorization: Bearer {token}
X-Tenant-ID: {tenant_id}
```

Returns the list of surface IDs and metadata available to the requesting user, used for navigation definition building and permission-gated navigation item generation.

### 25.2.3 Cache Headers

The surface definition response includes cache control headers that guide client caching:

```
ETag: "sha256:abc123def456"
Cache-Control: private, max-age=30
Vary: Accept-Language, X-Platform, X-Client-Version, X-Capability-Hash
```

The `Vary` header is essential — it ensures that caches differentiate between definitions compiled for different platforms, locales, and client versions.

---

## 25.3 Data Source API Conventions

### 25.3.1 List Endpoint Convention

All list data source endpoints return the standard list envelope:

```
GET /api/{resource}?{params}
Authorization: Bearer {token}
X-Tenant-ID: {tenant_id}
Accept-Language: {locale}
```

**Response (200):**
```json
{
  "data": [
    { "id": "inv-001", "invoice_number": "INV-2024-001", "amount": 145000, "status": "approved" },
    { "id": "inv-002", "invoice_number": "INV-2024-002", "amount": 87500, "status": "pending" }
  ],
  "meta": {
    "total": 1247,
    "page": 1,
    "page_size": 25,
    "total_pages": 50,
    "cursor": null
  },
  "summary": {
    "total_amount": 14526000,
    "count_by_status": { "approved": 823, "pending": 312, "rejected": 112 }
  }
}
```

**Standard query parameters for list endpoints:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `page` | integer | Page number (1-based, offset pagination) |
| `page_size` | integer | Items per page (max: 100 for API consumers; max: 2000 for surface data sources) |
| `after` | string | Cursor for cursor pagination |
| `sort_by` | string | Field to sort by |
| `sort_dir` | `asc` \| `desc` | Sort direction |
| `search` | string | Full-text search query |
| `{field}` | any | Filter by field value (endpoint-specific) |

### 25.3.2 Single Resource Endpoint Convention

```
GET /api/{resource}/{id}
Authorization: Bearer {token}
X-Tenant-ID: {tenant_id}
```

**Response (200):**
```json
{
  "data": {
    "id": "inv-001",
    "invoice_number": "INV-2024-001",
    ...
  }
}
```

The `data` wrapper is present even for single resources, for consistency with list responses.

### 25.3.3 Go Handler Pattern

```go
// Standard list handler using Goa-generated types
func (s *InvoiceService) ListInvoices(ctx context.Context,
    payload *invoice.ListInvoicesPayload) (*invoice.InvoiceListResult, error) {

    tenantID := middleware.TenantID(ctx)
    userID := middleware.UserID(ctx)

    // RLS is enforced by the database layer — no manual filtering needed
    rows, total, err := s.store.ListInvoices(ctx, store.ListInvoicesParams{
        TenantID:  tenantID,
        Status:    payload.Status,
        VendorID:  payload.VendorID,
        DateFrom:  payload.DateFrom,
        DateTo:    payload.DateTo,
        Page:      payload.Page,
        PageSize:  payload.PageSize,
        SortBy:    payload.SortBy,
        SortDir:   payload.SortDir,
    })
    if err != nil {
        return nil, apperr.Internal(err)
    }

    summary, err := s.store.InvoiceSummary(ctx, tenantID, payload)
    if err != nil {
        return nil, apperr.Internal(err)
    }

    return &invoice.InvoiceListResult{
        Data:    mapInvoiceRows(rows),
        Meta:    paginationMeta(payload.Page, payload.PageSize, total),
        Summary: mapSummary(summary),
    }, nil
}
```

---

## 25.4 Mutation API Conventions

### 25.4.1 Create

```
POST /api/{resource}
Content-Type: application/json
Authorization: Bearer {token}
X-Tenant-ID: {tenant_id}
X-Idempotency-Key: {uuid}   ← client-generated; retries with same key are safe

Body: { field: value, ... }
```

**Success response (201):**
```json
{
  "data": { "id": "inv-123", ... }
}
```

The `X-Idempotency-Key` header makes create operations idempotent. If a request with the same key is received within 24 hours, the server returns the original response without re-executing the handler.

### 25.4.2 Update (Full)

```
PUT /api/{resource}/{id}
Content-Type: application/json
```

Replaces the entire resource. Used when the form submits the complete object.

### 25.4.3 Update (Partial)

```
PATCH /api/{resource}/{id}
Content-Type: application/json

Body: { field_to_update: new_value }
```

Updates only the specified fields. Used for inline edits and field-level saves.

### 25.4.4 Delete

```
DELETE /api/{resource}/{id}
```

**Success response (200):**
```json
{ "deleted": true, "id": "inv-123" }
```

### 25.4.5 Bulk Operations

```
POST /api/{resource}/bulk/{operation}
Content-Type: application/json

Body: {
  "ids": ["inv-001", "inv-002", "inv-003"],
  "params": { "comment": "Approved in batch" }
}
```

**Response (200):**
```json
{
  "succeeded": ["inv-001", "inv-002"],
  "failed": [
    { "id": "inv-003", "error": "Invoice is already approved" }
  ],
  "summary": { "succeeded_count": 2, "failed_count": 1 }
}
```

Bulk operations are partial — some items may succeed while others fail. The response always lists both.

---

## 25.5 Authentication and Session APIs

Authentication follows the OAuth 2.0 Authorization Code flow with PKCE, implemented by the backend's auth service. The rendering engine's shell handles the auth flow transparently.

### 25.5.1 Token Endpoint

```
POST /auth/token
Content-Type: application/x-www-form-urlencoded

grant_type=authorization_code&code={code}&code_verifier={verifier}&client_id={client_id}
```

**Response:**
```json
{
  "access_token": "eyJ...",
  "token_type": "Bearer",
  "expires_in": 3600,
  "refresh_token": "eyJ...",
  "refresh_token_expires_in": 604800
}
```

### 25.5.2 Session Context Endpoint

After authentication, the shell fetches the session context needed to initialize the rendering engine:

```
GET /api/session
Authorization: Bearer {token}
X-Tenant-ID: {tenant_id}
```

**Response:**
```json
{
  "user": {
    "id": "usr-123",
    "display_name": "Aisha Mohamed",
    "email": "aisha@acme.co.ke",
    "roles": ["finance_manager", "invoice_approver"]
  },
  "tenant": {
    "id": "ten-456",
    "name": "Acme Ltd",
    "logo_url": "https://cdn.awoerp.com/tenants/ten-456/logo.png",
    "currency": "KES",
    "locale": "en-KE",
    "timezone": "Africa/Nairobi",
    "theme": { ... }
  },
  "feature_flags": {
    "advanced_reporting": true,
    "ai_suggestions": false,
    "beta_dashboard": true
  },
  "permissions": {
    "summary": "invoice:read invoice:create invoice:approve ..."
  }
}
```

---

## 25.6 Streaming APIs

Streaming APIs are covered in full in Chapter 21. This section documents the contract for use in API design consistency.

### 25.6.1 SSE Endpoint

```
GET /api/stream/surfaces/{surface_id}
Authorization: Bearer {token}
X-Tenant-ID: {tenant_id}
Last-Event-ID: {last_event_id}   ← omit on first connect
Accept: text/event-stream
```

### 25.6.2 Workflow Signal Endpoint

```
POST /api/workflows/{workflow_id}/signal
Content-Type: application/json
Authorization: Bearer {token}
X-Tenant-ID: {tenant_id}

{
  "signal_name": "approve",
  "payload": { "approved_by": "usr-123", "comment": "Looks good" }
}
```

**Response (200):**
```json
{ "delivered": true, "workflow_id": "wf-789", "signal_name": "approve" }
```

---

## 25.7 Versioning and Compatibility

### 25.7.1 API Versioning

The platform API uses URL path versioning for breaking changes:

```
/api/v1/{resource}      ← current stable version
/api/v2/{resource}      ← new version (when needed)
```

Non-breaking changes (new optional fields, new endpoints) are made to the current version without a version bump. Breaking changes (removed fields, changed field types, changed semantics) introduce a new version.

Surface definitions reference specific versioned API paths. When an API version is deprecated, the backend compilation pipeline fails with a warning for any surface definition that references the deprecated path.

### 25.7.2 Schema Version Compatibility

The surface definition schema is versioned separately from the API. The `schema_version` in the surface definition header declares the schema version used. Rendering engines declare the schema versions they support in the `X-Client-Version` header.

The compilation pipeline produces surface definitions for the schema version range the requesting client supports. Clients that are significantly behind may receive a simplified definition with components substituted for fallbacks.

### 25.7.3 Deprecation Timeline

API endpoints are deprecated on a minimum 180-day notice:
- **Day 0:** Deprecated endpoint returns an `X-Deprecated: true` header and a `Sunset` header with the removal date
- **Day 90:** Warning logged to the platform's observability pipeline for every call to the deprecated endpoint
- **Day 180:** Endpoint returns `410 Gone`

---

## 25.8 Error Response Conventions

All API error responses use the following envelope:

```json
{
  "error": "snake_case_error_code",
  "message": "Human-readable description of the error",
  "detail": "Additional technical detail (optional)",
  "request_id": "req-abc123",
  "field_errors": { ... },           ← present on 422
  "business_rule_errors": [ ... ]    ← present on certain 422s
}
```

### 25.8.1 Standard Error Codes

| HTTP Status | Error Code | Description |
|-------------|-----------|-------------|
| 400 | `bad_request` | Malformed request body or query parameters |
| 401 | `unauthenticated` | Missing or invalid authentication token |
| 403 | `forbidden` | Authenticated but not authorized for this resource |
| 404 | `not_found` | Resource does not exist or is not visible to this user |
| 409 | `conflict` | Optimistic locking conflict; client should reload and retry |
| 410 | `gone` | Resource permanently deleted or endpoint removed |
| 422 | `validation_failed` | Business validation errors |
| 429 | `rate_limited` | Too many requests; `Retry-After` header included |
| 500 | `internal_error` | Server error; `request_id` included for support |
| 503 | `service_unavailable` | Planned maintenance or temporary overload |

### 25.8.2 Request IDs

Every response includes a `request_id` header (`X-Request-ID`). The same ID is in the error response body. This ID correlates the client-side error event with the server-side log entry, enabling support to trace any error report.

---

## 25.9 API Documentation and Schema

### 25.9.1 OpenAPI Specification

All REST endpoints are documented in an OpenAPI 3.1 specification, generated from the Goa service definitions. The spec is served at:

```
GET /api/openapi.json       ← machine-readable spec
GET /api/docs               ← Swagger UI (development environments only)
```

The Goa framework generates both the Go service interfaces and the OpenAPI spec from the same DSL, ensuring the spec is always in sync with the implementation.

### 25.9.2 JSON Schema for Surface Definitions

The surface definition JSON Schema is versioned and served at:

```
GET /api/sdui/schema/{version}.json
```

This schema is the authoritative source for:
- Backend compiler validation (Chapter 8)
- Client rendering engine validation (Chapters 17, 18)
- API contract tests (Chapter 42)
- IDE tooling (Chapter 43)

---

## 25.10 gRPC API Contract

The gRPC service definitions are in the `proto/` directory and are the authoritative API contract for mobile clients.

```protobuf
syntax = "proto3";
package awoerp.v1;

service SurfaceService {
  rpc GetSurface(GetSurfaceRequest) returns (SurfaceDefinition);
  rpc SubscribeSurface(SubscribeSurfaceRequest) returns (stream SurfaceEvent);
}

message GetSurfaceRequest {
  string surface_id = 1;
  string tenant_id = 2;
  string locale = 3;
  string platform = 4;
  string client_version = 5;
  string capability_hash = 6;
  string etag = 7;            // For conditional fetch
}

message SurfaceDefinition {
  string surface_id = 1;
  string version = 2;
  string schema_version = 3;
  string etag = 4;
  int32 cache_ttl_seconds = 5;
  bytes definition_json = 6;  // Compressed JSON definition
}

service DataService {
  rpc Query(QueryRequest) returns (QueryResponse);
  rpc Mutate(MutateRequest) returns (MutateResponse);
  rpc BulkMutate(BulkMutateRequest) returns (BulkMutateResponse);
}
```

The gRPC transport uses mTLS for mobile-to-backend communication. Authentication tokens are passed as gRPC metadata.

---

*End of Chapter 25*

**Previous:** [Chapter 24 — Data Sources](../vol-05-state-and-validation/24-data-sources.md)
**Next:** [Chapter 26 — Security Architecture](./26-security.md)
