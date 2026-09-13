> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Field Validators"
id: dom-007
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: normative
related:
  - "[Fields](fields.md)"
  - "[Hooks](hooks.md)"
  - "[API Conventions](../11-api/conventions.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Field Validators

**DOM-007 | Status: Accepted | Stability: Stable**

This document specifies the `FieldValidator` interface, built-in validators, composition rules, and the boundary between field validators and hook validation.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. FieldValidator Interface

```go
// framework/def/validator.go

type FieldValidator interface {
    Validate(value any) error
}
```

A `FieldValidator` receives the raw field value (after type coercion) and returns nil (valid) or a `*ValidationError` (invalid).

Validators MUST be:
- **Pure**: no side effects, no external calls
- **Stateless**: no mutable fields; safe for concurrent use
- **Fast**: synchronous, no I/O; called on every request

Validators MUST NOT:
- Access the database or Redis
- Call external APIs
- Depend on other field values (use `before_validate` hook for cross-field validation)
- Modify the value (validators are read-only)

---

## 2. Built-in Validators

### EmailValidator

```go
// Validates RFC 5322 email format
// Usage: def.ValidateEmail
entity.FieldDef{
    Name:       "email",
    Type:       def.FieldData,
    Validators: []entity.FieldValidator{def.ValidateEmail},
}
```

### PhoneValidator (E.164)

```go
// Validates international phone number format: +254712345678
entity.FieldDef{
    Name:       "phone",
    Type:       def.FieldData,
    Validators: []entity.FieldValidator{def.ValidatePhone},
}
```

### URLValidator

```go
// Validates HTTPS or HTTP URL format
entity.FieldDef{
    Name:       "website",
    Type:       def.FieldData,
    Validators: []entity.FieldValidator{def.ValidateURL},
}
```

### KRAKRAValidatePIN

```go
// Validates Kenya Revenue Authority Personal Identification Number format
// Format: P followed by 9 digits, or A followed by 9 digits
entity.FieldDef{
    Name:       "kra_pin",
    Type:       def.FieldData,
    Validators: []entity.FieldValidator{def.ValidateKRAPIN},
}
```

### RegexValidator

```go
// Validate against a custom pattern
entity.FieldDef{
    Name: "product_code",
    Type: def.FieldData,
    Validators: []entity.FieldValidator{
        def.RegexValidator{
            Pattern: `^[A-Z]{2}-\d{4}$`,
            Message: "Product code must be two uppercase letters, a dash, and four digits (e.g., AB-1234).",
        },
    },
}
```

### RangeValidator

```go
// For numeric fields beyond Min/Max (which enforce hard limits)
// Use when the message or logic needs more nuance
entity.FieldDef{
    Name: "discount_percent",
    Type: def.FieldFloat,
    Validators: []entity.FieldValidator{
        def.RangeValidator{Min: 0, Max: 100, Message: "Discount must be between 0% and 100%."},
    },
}
```

---

## 3. Custom Validator Implementation

```go
// internal/core/crm/validators.go

type KenyaNationalIDValidator struct{}

func (v KenyaNationalIDValidator) Validate(value any) error {
    id, ok := value.(string)
    if !ok || id == "" {
        return nil  // Required constraint handles empty — do not double-validate
    }

    // Kenya National ID: 7-8 digits
    if !kenyanIDRegexp.MatchString(id) {
        return &errors.ValidationError{
            Fields: map[string]string{
                "national_id": "Kenya National ID must be 7 or 8 digits.",
            },
        }
    }
    return nil
}

var kenyanIDRegexp = regexp.MustCompile(`^\d{7,8}$`)
```

Declare on the field:

```go
{
    Name:       "national_id",
    Type:       def.FieldData,
    Validators: []entity.FieldValidator{KenyaNationalIDValidator{}},
}
```

The field name in the `ValidationError.Fields` map MUST match the field's `Name` in the `FieldDef`. This is how the amis form knows which field to display the error under.

---

## 4. Validator Composition

Multiple validators on one field run in declaration order. All validators run (not short-circuit on first failure):

```go
{
    Name: "email",
    Type: def.FieldData,
    Validators: []entity.FieldValidator{
        def.ValidateEmail,            // runs first
        OrganizationEmailValidator{},        // runs second (rejects gmail/yahoo)
        DomainBlocklistValidator{},          // runs third (rejects known spam domains)
    },
}
```

All errors are collected and returned together in the `ValidationError.Fields` map. If multiple validators fail for the same field, the last error message wins. For independent messages, use separate fields or a composite message.

---

## 5. Validator vs Hook: When to Use Each

| Scenario | Mechanism |
|---|---|
| Format check (email, phone, regex) | `FieldValidator` on `FieldDef` |
| Range/length check | `Min`/`Max`/`MaxLen` on `FieldDef`, or `RangeValidator` |
| Cross-field check (start < end date) | `before_validate` hook |
| Uniqueness check (DB query) | `before_validate` hook + UNIQUE constraint |
| Business rule (credit limit) | `before_save` hook |
| External validation (tax ID lookup) | `before_save` hook or activity |

`FieldValidator` runs synchronously before the hook pipeline. Hooks run after all field validators pass.

---

## 6. Error Format

Field validators MUST return `*errors.ValidationError` with the field name as the key:

```go
return &errors.ValidationError{
    Fields: map[string]string{
        "email": "Invalid email address format.",
    },
}
```

The error message MUST be:
- User-facing (suitable for display in the UI)
- In plain English (localization handled separately)
- Specific about what is wrong and how to fix it

Do not return generic messages like "Validation failed" — the amis form displays the message directly next to the field.

---

## 7. Testing Validators

Validators are plain Go structs — test them directly:

```go
func TestKenyaNationalIDValidator(t *testing.T) {
    v := KenyaNationalIDValidator{}

    tests := []struct {
        input   string
        wantErr bool
    }{
        {"12345678", false},   // valid: 8 digits
        {"1234567",  false},   // valid: 7 digits
        {"123456",   true},    // invalid: too short
        {"123456789", true},   // invalid: too long
        {"ABCDEFGH", true},    // invalid: letters
        {"",         false},   // empty: not validated (Required handles this)
    }

    for _, tt := range tests {
        t.Run(tt.input, func(t *testing.T) {
            err := v.Validate(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
            }
        })
    }
}
```

No mocking required — validators have no dependencies.

---

## Related Documents

- [Fields](fields.md) — `FieldDef` complete reference including `Validators` field
- [Hooks](hooks.md) — `before_validate` hook for cross-field and DB-dependent validation
- [API Conventions](../11-api/conventions.md) — HTTP 422 validation error response format
- [Module Dev Guide §3](../16-module-dev-guide/03-entity-fields.md) — declaring validators on fields
