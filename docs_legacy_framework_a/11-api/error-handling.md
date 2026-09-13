> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Error Handling"
id: api-002
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[API Conventions](conventions.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
  - "[Invariants](../02-architecture/invariants.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Error Handling

**API-002 | Status: Accepted | Stability: Stable**

This document specifies the error type hierarchy, HTTP status mappings, the error response envelope format, field-level validation errors, and the no-stack-trace-to-client invariant.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Error Type Hierarchy

The framework defines four named error types. All errors in the system MUST be one of these types by the time they reach the handler layer:

```go
// ValidationError — field-level input validation failure
// HTTP 422 Unprocessable Entity
type ValidationError struct {
    Fields map[string]string  // field name → user-facing message
}
func (e *ValidationError) Error() string { return "validation error" }

// BusinessError — domain rule violation
// HTTP status varies (400, 409, 410, etc.) — set on each instance
type BusinessError struct {
    Code    string  // machine-readable: "{module}.{rule}" e.g. "finance.invoice_already_paid"
    Message string  // user-facing English message
    Status  int     // HTTP status code
}
func (e *BusinessError) Error() string { return e.Message }

// NotFoundError — record does not exist or is not visible to actor
// HTTP 404 Not Found
type NotFoundError struct {
    EntityType string
    ID         uuid.UUID
}
func (e *NotFoundError) Error() string {
    return fmt.Sprintf("%s %s not found", e.EntityType, e.ID)
}

// PermissionError — actor lacks required permission
// HTTP 403 Forbidden
type PermissionError struct {
    Required string  // e.g. "role:finance.accounts_payable"
    Action   string  // e.g. "write"
    Entity   string  // e.g. "finance_invoice"
}
func (e *PermissionError) Error() string {
    return fmt.Sprintf("permission denied: %s on %s", e.Action, e.Entity)
}
```

---

## 2. HTTP Status Mapping

| Error Type | HTTP Status | Notes |
|---|---|---|
| `ValidationError` | 422 | Field-level failures; always includes `fields` map |
| `BusinessError` | `error.Status` | 400 for bad input, 409 for conflict, 410 for gone, etc. |
| `NotFoundError` | 404 | Returns same 404 whether record absent or access denied (prevents enumeration) |
| `PermissionError` | 403 | Only returned when record exists but actor lacks permission |
| Unhandled / internal | 500 | No error details in response; logged internally |
| Tenant PENDING | 503 + `Retry-After: 60` | Tenant not yet active |
| Tenant SUSPENDED | 402 | Payment required |
| Tenant ARCHIVED | 410 | Gone |

Note: `NotFoundError` and `PermissionError` both return their record exists / access denied distinction only at the 403 vs 404 boundary when the framework is certain. For general entity lookups, always return 404 when the record is not visible — do not reveal existence of inaccessible records.

---

## 3. Error Response Envelope

All error responses MUST use this envelope:

### Validation Error (HTTP 422)

```json
{
  "error": {
    "code": "validation_error",
    "message": "One or more fields failed validation.",
    "fields": {
      "customer": "Customer is required.",
      "total_kes": "Amount must be greater than zero.",
      "due_date": "Due date cannot be in the past."
    }
  },
  "meta": {
    "request_id": "req-abc123"
  }
}
```

The `fields` map keys MUST match the field names in the `EntityDefinition`. Values MUST be user-facing messages (not technical descriptions).

### Business Error (HTTP 400/409/etc.)

```json
{
  "error": {
    "code": "finance.invoice_already_submitted",
    "message": "This invoice has already been submitted for approval."
  },
  "meta": {
    "request_id": "req-abc123"
  }
}
```

The `code` field is machine-readable. Frontend code may switch on it for localization or conditional UI behavior. The `message` field is a default English fallback.

### Not Found (HTTP 404)

```json
{
  "error": {
    "code": "not_found",
    "message": "The requested record was not found."
  },
  "meta": {
    "request_id": "req-abc123"
  }
}
```

Do NOT include entity type or ID in the 404 message — this prevents confirming the existence of records the actor cannot access.

### Internal Error (HTTP 500)

```json
{
  "error": {
    "code": "internal_error",
    "message": "An unexpected error occurred. Please contact support."
  },
  "meta": {
    "request_id": "req-abc123"
  }
}
```

Internal errors MUST NOT include stack traces, SQL queries, internal error messages, or any other implementation detail ([INV-010](../02-architecture/invariants.md#inv-010)).

---

## 4. Error Mapping in Handlers

Handlers use `errors.As` (never type switch) to unwrap error chains:

```go
func mapEntityError(err error) error {
    var ve *errors.ValidationError
    if errors.As(err, &ve) {
        return c.Status(422).JSON(ErrorEnvelope{
            Error: ErrorBody{
                Code:    "validation_error",
                Message: "One or more fields failed validation.",
                Fields:  ve.Fields,
            },
            Meta: MetaBody{RequestID: requestID},
        })
    }

    var be *errors.BusinessError
    if errors.As(err, &be) {
        return c.Status(be.Status).JSON(ErrorEnvelope{
            Error: ErrorBody{Code: be.Code, Message: be.Message},
            Meta:  MetaBody{RequestID: requestID},
        })
    }

    var nfe *errors.NotFoundError
    if errors.As(err, &nfe) {
        return c.Status(404).JSON(ErrorEnvelope{
            Error: ErrorBody{Code: "not_found", Message: "The requested record was not found."},
            Meta:  MetaBody{RequestID: requestID},
        })
    }

    var pe *errors.PermissionError
    if errors.As(err, &pe) {
        return c.Status(403).JSON(ErrorEnvelope{
            Error: ErrorBody{Code: "forbidden", Message: "You do not have permission to perform this action."},
            Meta:  MetaBody{RequestID: requestID},
        })
    }

    // Unhandled — log internally, return generic 500
    slog.Error("unhandled error",
        "request_id", requestID,
        "tenant_id",  tenantID,
        "user_id",    userID,
        "err",        err,
    )
    return c.Status(500).JSON(ErrorEnvelope{
        Error: ErrorBody{Code: "internal_error", Message: "An unexpected error occurred."},
        Meta:  MetaBody{RequestID: requestID},
    })
}
```

`errors.As` is mandatory — not `errors.Is`, not type switch. Error chains must be unwrapped correctly because hooks and services wrap errors with `fmt.Errorf("op: %w", err)`.

---

## 5. Error Construction in Business Logic

### Repository Layer

The repository layer converts pgx/PostgreSQL errors to domain errors:

```go
func parseDBError(err error, op string) error {
    var pgErr *pgconn.PgError
    if errors.As(err, &pgErr) {
        switch pgErr.Code {
        case "23505": // unique_violation
            return &errors.BusinessError{
                Code:    "duplicate_record",
                Message: "A record with these values already exists.",
                Status:  409,
            }
        case "23514": // check_violation
            return &errors.BusinessError{
                Code:    "constraint_violation",
                Message: pgErr.Message,
                Status:  400,
            }
        case "23503": // foreign_key_violation
            return &errors.BusinessError{
                Code:    "invalid_reference",
                Message: "The referenced record does not exist.",
                Status:  400,
            }
        }
    }
    return fmt.Errorf("%s: %w", op, err)
}
```

### Hook Layer (Validation)

Before-validate hooks return `*ValidationError`:

```go
func (h *InvoiceValidator) BeforeCreate(ctx context.Context, rec *entity.EntityRecord) error {
    errs := map[string]string{}

    customer := rec.Fields["customer"]
    if customer == "" || customer == nil {
        errs["customer"] = "Customer is required."
    }

    amount, ok := rec.Fields["total_kes"].(decimal.Decimal)
    if !ok || amount.LessThanOrEqual(decimal.Zero) {
        errs["total_kes"] = "Amount must be greater than zero."
    }

    if len(errs) > 0 {
        return &errors.ValidationError{Fields: errs}
    }
    return nil
}
```

### Service / Domain Layer

Business rule violations return `*BusinessError`:

```go
func (s *InvoiceService) Submit(ctx context.Context, id uuid.UUID) error {
    invoice, err := s.repo.Get(ctx, id)
    if err != nil {
        return fmt.Errorf("InvoiceService.Submit: %w", err)
    }

    if invoice.Status != "Draft" {
        return &errors.BusinessError{
            Code:    "finance.invoice_not_draft",
            Message: fmt.Sprintf("Invoice cannot be submitted from status %q.", invoice.Status),
            Status:  409,
        }
    }
    // ...
}
```

---

## 6. Error Logging

Every unhandled error (HTTP 500) MUST be logged with structured context:

```go
slog.Error("request error",
    "request_id", requestID,
    "tenant_id",  tenantID.String(),
    "user_id",    actor.UserID.String(),
    "method",     c.Method(),
    "path",       c.Path(),
    "err",        err,
)
```

Handled errors (4xx) SHOULD NOT be logged at Error level — they are expected client behavior. Log at Debug level if useful for tracing.

Exception: `PermissionError` logs at Warn level to aid security monitoring.

---

## 7. amis Error Format

amis (the SDUI renderer) expects validation errors in a specific format. The framework's error handler automatically translates `ValidationError` into the amis format when the request originates from amis (detected via `X-amis-Request` header or Accept header):

```json
{
  "status": 422,
  "msg": "Validation failed",
  "errors": {
    "customer": "Customer is required.",
    "total_kes": "Amount must be greater than zero."
  }
}
```

This translation is handled by the framework's error middleware — module authors do not need to handle it explicitly.

---

## Related Documents

- [API Conventions](conventions.md) — success response format and URL structure
- [Architecture Invariants](../02-architecture/invariants.md) — INV-010 (no stack traces to client)
- [Architecture Laws](../02-architecture/laws.md) — LAW-013 (sensitive fields)
- [Glossary](../GLOSSARY.md) — ValidationError, BusinessError, Response Envelope
