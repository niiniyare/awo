> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Input Validation and Sanitization"
id: sec-002
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Security Model](security-model.md)"
  - "[Fields](../04-domain/fields.md)"
  - "[Hooks](../04-domain/hooks.md)"
  - "[API Conventions](../11-api/conventions.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Input Validation and Sanitization

**SEC-002 | Status: Accepted | Stability: Stable**

This document specifies where and how input validation is enforced in Awo, covering field-level constraints, hook validation, injection prevention, and output encoding.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Validation Layers

Input validation in Awo occurs at three layers:

| Layer | Mechanism | When |
|---|---|---|
| Field constraints | `FieldDef` (`Required`, `MaxLen`, `Min`, `Max`, `Validators`) | Before `before_validate` hook |
| Hook validation | `before_validate` hook returning `ValidationError` | Before persistence |
| Database constraints | `CHECK`, `NOT NULL`, FK constraints in SQL | At INSERT/UPDATE time |

All three layers are required. Database constraints are the last line of defense — they should never be the first or only check.

---

## 2. Field-Level Validation

### Built-in Constraints

```go
{
    Name:     "email",
    Type:     def.FieldData,
    Required: true,
    MaxLen:   254,        // RFC 5321 max email length
    Validators: []entity.FieldValidator{EmailValidator{}},
}

{
    Name: "quantity",
    Type: def.FieldInt,
    Min:  1,
    Max:  10000,
}

{
    Name: "code",
    Type: def.FieldData,
    Unique: true,
    MaxLen: 20,
}
```

The framework applies these constraints before any hook runs. Constraint violations return HTTP 422 with `ValidationError`.

### Custom Field Validators

```go
type EmailValidator struct{}

func (v EmailValidator) Validate(value any) error {
    s, ok := value.(string)
    if !ok || s == "" {
        return nil  // Required constraint handles empty
    }
    if !emailRegexp.MatchString(s) {
        return &errors.ValidationError{
            Fields: map[string]string{"email": "Invalid email address format."},
        }
    }
    return nil
}

var emailRegexp = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
```

Validators MUST be pure functions — no database access, no external calls. Database-dependent validation (e.g., uniqueness across records) belongs in the `before_validate` hook.

---

## 3. SQL Injection Prevention

Awo's `EntityRepository` interface and `Filter` DSL prevent SQL injection by design:

- All query predicates are expressed as `Filter` objects — never raw SQL strings
- The store layer translates `Filter` to parameterized queries using pgx's `$N` placeholders
- Module authors MUST NOT construct raw SQL in business logic

```go
// CORRECT: parameterized via Filter DSL
results, _, err := repo.Query(ctx, filter.Eq("email", userInput))
// Generates: WHERE email = $1  (parameterized)

// WRONG: string interpolation (not possible via EntityRepository — demonstrates what to avoid)
// query := fmt.Sprintf("WHERE email = '%s'", userInput)  // SQL injection risk
```

If raw SQL is ever required (e.g., in a migration or a one-off admin script), always use parameterized queries:

```sql
-- CORRECT
SELECT * FROM crm_contact WHERE email = $1;

-- WRONG
SELECT * FROM crm_contact WHERE email = '${userInput}';
```

---

## 4. XSS Prevention

Awo does not render HTML from user-supplied content. All UI is generated as amis JSON schemas — user data appears in amis component values, not as raw HTML.

Where user content appears in amis `tpl` (template) components, it MUST be passed as a bound value, not interpolated into the template string:

```go
// CORRECT: value bound, not interpolated
amis.Tpl("${customer_name}")  // amis HTML-escapes the value

// WRONG: interpolated into Go string (no escaping)
amis.Tpl(fmt.Sprintf("Customer: %s", customerName))  // XSS if customerName contains HTML
```

For any endpoint that returns HTML (e.g., invoice PDF generation), user-supplied values MUST be HTML-escaped before inclusion:

```go
html.EscapeString(userValue)
```

---

## 5. Path Traversal Prevention

File upload endpoints (e.g., bulk import) MUST validate filenames before using them in file paths:

```go
func sanitizeFilename(name string) (string, error) {
    // Reject any path components
    if strings.Contains(name, "/") || strings.Contains(name, "..") || strings.Contains(name, "\\") {
        return "", errors.New("invalid filename")
    }
    // Allow only alphanumeric, dash, underscore, dot
    if !filenameRegexp.MatchString(name) {
        return "", errors.New("invalid filename characters")
    }
    return name, nil
}
```

Store uploaded files in a configured directory (not derived from user input), using a UUID-based internal filename:

```go
internalName := uuid.New().String() + filepath.Ext(sanitizedOriginalName)
```

---

## 6. Request Size Limits

```go
app := fiber.New(fiber.Config{
    BodyLimit: 10 * 1024 * 1024,  // 10 MB default body limit
})
```

Bulk import endpoints may need a higher limit for large files:

```go
// Override per-route
app.Post("/api/v1/entities/:type/import",
    middleware.BodyLimitOverride(50*1024*1024),  // 50 MB for imports
    handler.BulkImportHandler,
)
```

JSON body parsing errors (malformed JSON, size exceeded) return HTTP 400.

---

## 7. Sensitive Field Handling

Fields declared `Sensitive: true` are:

- Excluded from all list and detail API responses
- Excluded from webhook payloads
- Excluded from audit log entries
- Never logged at any level
- Transmitted only to authenticated users with explicit permission

```go
{
    Name:      "tax_pin",
    Type:      def.FieldData,
    Sensitive: true,
    // Only returned via a dedicated, permission-gated endpoint
}
```

Module authors MUST NOT log field values without checking the field's `Sensitive` flag. Use the structured logger's field omission via the framework's log helper:

```go
// Framework logger omits Sensitive fields automatically
logger.LogEntityEvent(ctx, "contact.created", record)
// tax_pin, password_hash, etc. are omitted
```

---

## 8. Validation in Hooks vs Field Definitions

| Use case | Where |
|---|---|
| Format check (email, phone, URL) | `FieldValidator` on `FieldDef` |
| Range check (min/max value) | `Min`/`Max` on `FieldDef` |
| Required check | `Required: true` on `FieldDef` |
| Cross-field consistency (start < end) | `before_validate` hook |
| Database uniqueness (not just field-level) | `before_validate` hook + DB UNIQUE constraint |
| Business rule (credit limit check) | `before_save` hook |
| External API validation (tax ID verify) | `before_save` hook or activity |

Do not perform cross-field or DB-dependent validation in `FieldValidator` — they run without context or DB access.

---

## Related Documents

- [Security Model](security-model.md) — 5-layer defense in depth, threat model
- [Fields](../04-domain/fields.md) — `FieldDef` constraint reference
- [Hooks](../04-domain/hooks.md) — `before_validate`, `before_save` hooks
- [Error Handling](../11-api/error-handling.md) — HTTP 422 validation error format
