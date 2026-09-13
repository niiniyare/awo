# Error Response Format

**Classification:** Specification — Tier 1
**Owner:** `14-api/ERROR_RESPONSE_FORMAT.md`
**Status:** Frozen at v1.0

---

## Purpose

This document specifies the client-facing error response envelope for all Awo Framework API endpoints. Internal error types (`BusinessError`, `ValidationError`) are specified in [`02-pipeline/ERROR_MODEL.md`](../02-pipeline/ERROR_MODEL.md). This document specifies what clients see.

---

## 1. Error Envelope

All error responses use:

```json
{
  "error": {
    "code":    "machine_readable_code",
    "message": "Human-readable description.",
    "fields":  {}
  }
}
```

| Field | Type | Always Present | Description |
|-------|------|---------------|-------------|
| `code` | string | Yes | Machine-readable error code. Stable across versions. |
| `message` | string | Yes | Human-readable message. NOT suitable for parsing — may change. |
| `fields` | object | Yes | Field-level error map. Empty object `{}` when no field errors. |

The `error` key is always present on error responses. The `data` and `meta` keys are absent on error responses.

---

## 2. Error Code Conventions

| Code Pattern | Meaning |
|-------------|---------|
| `{entity}.{rule}` | Domain business rule violation. e.g. `invoice.not_draft` |
| `validation_error` | Field-level validation failure (422 only) |
| `not_found` | Record not found |
| `permission_denied` | Authenticated but lacks permission |
| `unauthorized` | No valid session |
| `rate_limited` | Request rate limit exceeded |
| `idempotency.in_flight` | Duplicate in-flight request |
| `internal_error` | Unexpected server error (never expose internals) |
| `tenant.suspended` | Tenant is suspended |
| `tenant.archived` | Tenant is archived |

Codes are lowercase, dot-separated, never spaces. Codes MUST be stable — changing a code is a breaking change for API clients.

---

## 3. Validation Error (HTTP 422)

Field-level errors from `ValidationError` populate the `fields` object:

```json
{
  "error": {
    "code":    "validation_error",
    "message": "One or more fields are invalid.",
    "fields": {
      "customer_id": "Customer is required.",
      "total":       "Total must be greater than zero.",
      "due_date":    "Due date must be after invoice date."
    }
  }
}
```

`fields` keys are field names as they appear in the request body. `fields` values are user-facing messages.

The amis SDUI renderer reads `fields` and displays errors inline below the corresponding form inputs.

---

## 4. Business Error (HTTP 400, 409, etc.)

Domain rule violations from `BusinessError`:

```json
{
  "error": {
    "code":    "invoice.already_submitted",
    "message": "Only Draft invoices can be submitted.",
    "fields":  {}
  }
}
```

`fields` is always an empty object for `BusinessError` — field-level detail is not applicable to business rule violations.

---

## 5. Not Found (HTTP 404)

```json
{
  "error": {
    "code":    "not_found",
    "message": "Invoice not found.",
    "fields":  {}
  }
}
```

---

## 6. Permission Denied (HTTP 403)

```json
{
  "error": {
    "code":    "permission_denied",
    "message": "You do not have permission to perform this action.",
    "fields":  {}
  }
}
```

**MUST NOT** reveal which permission is missing or which roles would grant access — that information aids attackers in privilege escalation attempts.

---

## 7. Unauthorized (HTTP 401)

```json
{
  "error": {
    "code":    "unauthorized",
    "message": "Authentication required.",
    "fields":  {}
  }
}
```

---

## 8. Rate Limited (HTTP 429)

```json
{
  "error": {
    "code":    "rate_limited",
    "message": "Too many requests. Please retry after 60 seconds.",
    "fields":  {}
  }
}
```

Include the `Retry-After` HTTP header with the number of seconds until the window resets.

---

## 9. Internal Error (HTTP 500)

```json
{
  "error": {
    "code":    "internal_error",
    "message": "An unexpected error occurred.",
    "fields":  {}
  }
}
```

**MUST NOT** include stack traces, internal error messages, SQL errors, or any implementation details. The full error is logged internally with `request_id` for correlation. Return the `request_id` in the `X-Request-ID` response header so clients can provide it when reporting issues.

---

## 10. Tenant Status Errors

| Tenant Status | HTTP Status | Code |
|--------------|------------|------|
| `PENDING` | 503 + `Retry-After: 60` | `tenant.pending` |
| `SUSPENDED` | 402 | `tenant.suspended` |
| `ARCHIVED` | 410 | `tenant.archived` |

---

## 11. Rules for Error Responses

- Internal error messages MUST NOT be exposed to clients.
- Stack traces MUST NEVER appear in error responses.
- `fields` MUST always be present (empty object `{}` if no field errors).
- `code` values MUST be stable — they are part of the API contract.
- The `Retry-After` header MUST be set on 429 and 503 responses.

---

## References

- [`02-pipeline/ERROR_MODEL.md`](../02-pipeline/ERROR_MODEL.md) — Internal error types and wrapping
- [`14-api/API_CONVENTIONS.md`](API_CONVENTIONS.md) — HTTP status codes
