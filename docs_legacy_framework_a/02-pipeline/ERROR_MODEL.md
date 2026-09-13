> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Error Model

**Classification:** Specification — Tier 0
**Owner:** `02-pipeline/ERROR_MODEL.md`
**Status:** Frozen at v1.0
**Package:** `awo.so/awo/def` and `awo.so/awo/shared/errors`

---

## Purpose

This document specifies all error types, their semantics, HTTP status mappings, and the error propagation protocol. All framework components and module authors MUST use these types.

## Dependencies

None. This is a Tier 0 specification.

## Related Specifications

- [`14-api/ERROR_RESPONSE_FORMAT.md`](../14-api/ERROR_RESPONSE_FORMAT.md) — Client-facing error envelope

---

## 1. Error Types

### ValidationError

```go
// Package: awo.so/awo/def

type ValidationError struct {
    Fields map[string]string  // field name → user-facing message
}

func (e *ValidationError) Error() string {
    // returns a joined string of all field errors
}
```

**When to use:** Field-level input validation failures. Use when the error is attributable to a specific field in the request.

**HTTP status:** 422 Unprocessable Entity.

**Who creates it:** Hook implementations (`BeforeCreateHook`, etc.), field validators (`FieldValidator` functions). The framework runtime creates it during the VALIDATE stage for built-in constraint violations.

**Multi-field:** A single `ValidationError` MAY contain errors for multiple fields. All field errors are collected before returning.

**Example:**
```go
return &def.ValidationError{Fields: map[string]string{
    "email":       "Email address is not valid",
    "customer_id": "Customer is required",
}}
```

---

### BusinessError

```go
// Package: awo.so/awo/def

type BusinessError struct {
    Code    string  // machine-readable: "invoice.already_submitted"
    Message string  // user-facing message
    Status  int     // HTTP status code
}

func (e *BusinessError) Error() string {
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}
```

**When to use:** Domain rule violations. Use when the operation is structurally valid (fields are correct) but violates a business constraint.

**HTTP status:** Specified by `Status` field. Common values:
- 400 Bad Request — malformed business operation
- 409 Conflict — state conflict (record already in target state, duplicate)
- 422 Unprocessable Entity — semantic validation failure
- 403 Forbidden — specific to action-level business rules

**Code format:** `{entity}.{violation}` — e.g., `invoice.already_submitted`, `customer.credit_limit_exceeded`.

**Example:**
```go
return &def.BusinessError{
    Code:    "invoice.already_submitted",
    Message: "This invoice has already been submitted and cannot be modified",
    Status:  409,
}
```

---

### NotFoundError

```go
// Package: awo.so/awo/def (or runtime)

type NotFoundError struct {
    EntityName string
    ID         uuid.UUID
}

func (e *NotFoundError) Error() string {
    return fmt.Sprintf("%s %s not found", e.EntityName, e.ID)
}
```

**When to use:** A record with the requested ID does not exist within the current tenant's RLS context.

**HTTP status:** 404 Not Found.

**Note:** A record that exists in another tenant produces a 404 (not a 403) because the RLS filter makes it invisible. This is correct behavior — revealing the existence of another tenant's data would be a security violation.

---

### PermissionError

```go
// Package: awo.so/awo/def (or runtime)

type PermissionError struct {
    Action string
    Object string
}

func (e *PermissionError) Error() string {
    return fmt.Sprintf("permission denied: %s on %s", e.Action, e.Object)
}
```

**When to use:** The PolicyEvaluator denied the operation. Created by the framework runtime at the AUTHORIZE stage. Module authors do not create this type.

**HTTP status:** 403 Forbidden.

---

## 2. Error Wrapping Protocol

All errors MUST be wrapped using `fmt.Errorf("operation: %w", err)` to preserve the error chain. This is required for `errors.As` unwrapping to work correctly.

```go
// Correct:
return fmt.Errorf("InvoiceValidator.BeforeCreate: validate customer: %w", err)

// Wrong:
return fmt.Errorf("validation failed: %v", err)  // breaks unwrapping
return err                                         // no context
```

**Never use type switches to inspect errors.** Always use `errors.As`:

```go
// Correct:
var ve *def.ValidationError
if errors.As(err, &ve) {
    // handle validation error
}

// Wrong:
switch e := err.(type) {
case *def.ValidationError:  // does not work if the error is wrapped
```

---

## 3. Database Error Mapping

The repository layer MUST map PostgreSQL errors to domain errors before returning them to callers:

```go
// awo/shared/db/errors.go

func parseDBError(err error, op string) error {
    var pgErr *pgconn.PgError
    if errors.As(err, &pgErr) {
        switch pgErr.Code {
        case "23505": // unique_violation
            return &def.BusinessError{
                Code:    "duplicate_record",
                Message: "A record with these values already exists",
                Status:  409,
            }
        case "23514": // check_violation
            return &def.BusinessError{
                Code:    "constraint_violation",
                Message: pgErr.Message,
                Status:  400,
            }
        case "23503": // foreign_key_violation
            return &def.BusinessError{
                Code:    "reference_not_found",
                Message: "Referenced record does not exist",
                Status:  400,
            }
        case "23502": // not_null_violation
            return &def.BusinessError{
                Code:    "required_field_missing",
                Message: pgErr.ColumnName + " is required",
                Status:  400,
            }
        }
    }
    return fmt.Errorf("%s: %w", op, err)
}
```

---

## 4. Handler Error Mapping

HTTP route handlers MUST map domain errors to HTTP responses using `errors.As`:

```go
// awo/shared/errors/http.go

func ToHTTPError(err error) (int, ErrorResponse) {
    var ve *def.ValidationError
    if errors.As(err, &ve) {
        return 422, ErrorResponse{
            Code:    "validation_error",
            Message: "Request validation failed",
            Fields:  ve.Fields,
        }
    }

    var be *def.BusinessError
    if errors.As(err, &be) {
        return be.Status, ErrorResponse{
            Code:    be.Code,
            Message: be.Message,
        }
    }

    var nfe *def.NotFoundError
    if errors.As(err, &nfe) {
        return 404, ErrorResponse{
            Code:    "not_found",
            Message: fmt.Sprintf("%s not found", nfe.EntityName),
        }
    }

    var pe *def.PermissionError
    if errors.As(err, &pe) {
        return 403, ErrorResponse{
            Code:    "forbidden",
            Message: "You do not have permission to perform this action",
        }
    }

    // Unknown error: log with full context, return opaque response
    slog.Error("unhandled error",
        "err", err,
        // ... request context fields
    )
    return 500, ErrorResponse{
        Code:    "internal_error",
        Message: "An unexpected error occurred",
    }
}
```

**The internal error message MUST NOT expose `err.Error()` to the client.** Stack traces, PostgreSQL errors, and internal state MUST remain server-side.

---

## 5. Error Response Envelope

```json
{
  "error": {
    "code": "validation_error",
    "message": "Request validation failed",
    "fields": {
      "email": "Email address is not valid",
      "customer_id": "Customer is required"
    }
  }
}
```

For non-validation errors, `"fields"` is omitted:
```json
{
  "error": {
    "code": "invoice.already_submitted",
    "message": "This invoice has already been submitted and cannot be modified"
  }
}
```

See [`14-api/ERROR_RESPONSE_FORMAT.md`](../14-api/ERROR_RESPONSE_FORMAT.md) for the full client-facing format specification.

---

## 6. Logging Requirements

Every error that reaches the handler MUST be logged with structured context before the response is sent:

```go
slog.Error("request failed",
    "request_id", requestID,
    "tenant_id",  tenantID.String(),
    "user_id",    userID.String(),
    "method",     c.Method(),
    "path",       c.Path(),
    "err",        err,
)
```

**Sensitive data MUST NOT appear in log output.** The `slog` handler MUST be configured to exclude sensitive fields.

---

## 7. Normative Requirements

- Module authors MUST use `*def.ValidationError` for field-level validation failures.
- Module authors MUST use `*def.BusinessError` for domain rule violations.
- Module authors MUST wrap errors using `fmt.Errorf("context: %w", err)`.
- Module authors MUST NOT use type switches on errors — use `errors.As`.
- HTTP handlers MUST NOT expose raw `err.Error()` output to clients.
- HTTP handlers MUST log unhandled errors with full structured context.
- The client-facing error envelope MUST conform to the format in [`14-api/ERROR_RESPONSE_FORMAT.md`](../14-api/ERROR_RESPONSE_FORMAT.md).

---

## References

- `awo/def/errors.go` — ValidationError, BusinessError, NotFoundError, PermissionError
- `awo/shared/errors/http.go` — ToHTTPError implementation
- [`14-api/ERROR_RESPONSE_FORMAT.md`](../14-api/ERROR_RESPONSE_FORMAT.md) — Client envelope format
- [`02-pipeline/LIFECYCLE_SPEC.md`](LIFECYCLE_SPEC.md) — Where errors abort the pipeline
