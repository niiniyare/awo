# Chapter 23 — Validation Framework

> **Volume:** V — State, Validation & Data
> **Phase:** 3
> **Audience:** Platform Engineers, Backend Engineers, Web Engineers, Mobile Engineers
> **Prerequisites:** Chapters 08, 11, 22

---

## Table of Contents

- [23.1 Validation Philosophy](#231-validation-philosophy)
- [23.2 Validation Layers](#232-validation-layers)
- [23.3 Field-Level Validation Rules](#233-field-level-validation-rules)
- [23.4 Cross-Field Validation](#234-cross-field-validation)
- [23.5 Remote Validation](#235-remote-validation)
- [23.6 Server-Side Validation](#236-server-side-validation)
- [23.7 Validation Error Display](#237-validation-error-display)
- [23.8 Validation in Multi-Step Forms](#238-validation-in-multi-step-forms)
- [23.9 Business Rule Validation](#239-business-rule-validation)
- [23.10 Validation at the AST Compiler](#2310-validation-at-the-ast-compiler)

---

## 23.1 Validation Philosophy

**The server always validates.** No matter how thorough the client-side validation, the server re-validates every mutation request. Client-side validation is a UX convenience — it gives users immediate feedback without a round-trip. It is not a security or correctness boundary. Any logic that must be enforced should be in the server validation layer.

**Validation rules travel with the field definition.** Validation rules are declared alongside the field in the surface definition, compiled by the backend, and delivered to the client in the JSON definition. There is no separately maintained client-side validation configuration file. When the backend changes a validation rule (e.g., a new minimum length requirement), it is reflected in the next surface definition fetch without a client app update.

**Validation messages are localizable.** All validation message strings support the localization system (Chapter 37). Messages are either declared as localization keys or as literal strings with interpolation placeholders.

---

## 23.2 Validation Layers

```
User interaction
      │
      ▼
┌─────────────────────────────────────────────────┐
│  Layer 1: Real-time field validation            │
│  When: on_blur (default) or on_change           │
│  Rules: format, required, min/max, pattern      │
│  Implementation: client-side (instant feedback) │
└─────────────────────────────┬───────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────┐
│  Layer 2: Cross-field validation                │
│  When: on_blur of dependent field, on submit    │
│  Rules: field_a must be < field_b               │
│  Implementation: client-side expression eval    │
└─────────────────────────────┬───────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────┐
│  Layer 3: Remote validation                     │
│  When: on_blur (debounced 500ms)                │
│  Rules: remote_unique, remote_exists            │
│  Implementation: async API call, cached 30s     │
└─────────────────────────────┬───────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────┐
│  Layer 4: Form-level validation on submit       │
│  When: user clicks submit                       │
│  Rules: all Layer 1–3 rules re-run together     │
│  Blocks submission if any field has errors      │
└─────────────────────────────┬───────────────────┘
                              │ (if all pass)
                              ▼
┌─────────────────────────────────────────────────┐
│  Layer 5: Server-side validation                │
│  When: every mutation request                   │
│  Rules: all business rules, constraints, auth   │
│  Response: 422 with field_errors map            │
│  Client: distributes errors to form fields      │
└─────────────────────────────────────────────────┘
```

---

## 23.3 Field-Level Validation Rules

Validation rules are declared in the field's `validation` array. They are evaluated in declaration order; the first failing rule produces the error message displayed to the user.

### 23.3.1 Rule Reference

| Rule | Props | Description |
|------|-------|-------------|
| `required` | `message?` | Field must have a non-empty value |
| `min_length` | `value`, `message?` | String must have at least `value` characters |
| `max_length` | `value`, `message?` | String must have at most `value` characters |
| `exact_length` | `value`, `message?` | String must have exactly `value` characters |
| `pattern` | `pattern` (regex), `message?` | String must match the regex pattern |
| `email` | `message?` | Must be a valid email address |
| `url` | `message?` | Must be a valid URL |
| `phone` | `locale?`, `message?` | Must be a valid phone number for the given locale |
| `min_value` | `value`, `message?` | Number must be ≥ `value` |
| `max_value` | `value`, `message?` | Number must be ≤ `value` |
| `min_date` | `value \| Binding`, `message?` | Date must be on or after `value` |
| `max_date` | `value \| Binding`, `message?` | Date must be on or before `value` |
| `integer` | `message?` | Number must be a whole integer |
| `positive` | `message?` | Number must be > 0 |
| `non_negative` | `message?` | Number must be ≥ 0 |
| `decimal_places` | `max`, `message?` | Number must have at most `max` decimal places |
| `one_of` | `values[]`, `message?` | Value must be in the given list |
| `not_one_of` | `values[]`, `message?` | Value must not be in the given list |
| `file_max_size` | `bytes`, `message?` | File must be smaller than `bytes` |
| `file_types` | `mime_types[]`, `message?` | File must be one of the listed MIME types |
| `remote_unique` | `endpoint`, `field`, `message?` | Value must be unique per the remote check |
| `remote_exists` | `endpoint`, `field`, `message?` | Value must exist per the remote check |
| `expression` | `expr`, `message` | Custom expression that must evaluate to `true` |

### 23.3.2 Declaration Example

```json
{
  "type": "field.text",
  "props": {
    "id": "vendor_email",
    "label": "Vendor Email",
    "placeholder": "vendor@example.com",
    "validation": [
      { "rule": "required", "message": "Email is required" },
      { "rule": "email", "message": "Must be a valid email address" },
      { "rule": "max_length", "value": 254, "message": "Email must be 254 characters or less" },
      {
        "rule": "remote_unique",
        "endpoint": "/api/vendors/check-email",
        "field": "email",
        "message": "A vendor with this email already exists"
      }
    ]
  }
}
```

### 23.3.3 Dynamic Validation Rules

Validation rules can reference binding expressions to make rules dynamic:

```json
{
  "rule": "min_date",
  "value": "$state.contract_start_date",
  "message": "End date must be on or after the contract start date"
}
```

```json
{
  "rule": "max_value",
  "value": "$data.po_header.approved_budget",
  "message": "Line item total cannot exceed the approved budget"
}
```

### 23.3.4 Validation Trigger

The default trigger is `"on_blur"` — validation runs when the field loses focus. Fields can specify an alternative trigger:

```json
{ "rule": "required", "trigger": "on_change" }
```

The `on_change` trigger runs validation on every keystroke. It is appropriate for format rules (`email`, `pattern`) but not for expensive rules (`remote_unique`), which should always use `on_blur` with debouncing.

---

## 23.4 Cross-Field Validation

Cross-field validation rules span multiple fields. They are declared at the `form.section` or `form.container` level, not on individual fields.

```json
{
  "type": "form.section",
  "props": {
    "id": "date_section",
    "cross_field_validations": [
      {
        "id": "end_after_start",
        "expr": "$expr{ !$form.start_date || !$form.end_date || $form.end_date >= $form.start_date }",
        "message": "End date must be on or after start date",
        "target_field_id": "end_date"
      },
      {
        "id": "budget_not_exceeded",
        "expr": "$expr{ $form.line_items.reduce((sum, item) => sum + item.total, 0) <= $form.approved_budget }",
        "message": "Total line items ($expr{ formatCurrency($form.line_items.reduce((s,i) => s+i.total, 0), $tenant.currency) }) exceeds approved budget",
        "target_field_id": "approved_budget"
      }
    ]
  }
}
```

Cross-field validations run:
- When any field referenced in the expression changes (triggered by `on_blur` on any of the referenced fields)
- On form submit (all cross-field validations are run regardless of which field was last touched)

The `target_field_id` specifies which field displays the error message. If omitted, the message appears in a form-level error summary.

---

## 23.5 Remote Validation

Remote validation (`remote_unique`, `remote_exists`) calls a backend endpoint to check the validity of a field value. To avoid excess API calls, remote validation is:

- **Debounced:** The call is made 500ms after the user stops typing
- **Cached:** Results are cached for 30 seconds per value. If the same value is entered again within 30 seconds, the cached result is used immediately
- **Skipped on unchanged values:** If the value has not changed since the last successful remote check, the rule is considered passing

### 23.5.1 Remote Validation Endpoint Contract

```
POST /api/validate/unique
Content-Type: application/json
Authorization: Bearer {token}
X-Tenant-ID: {tenant_id}

{
  "field": "email",
  "value": "vendor@example.com",
  "context": {
    "exclude_id": "$route.params.vendor_id"   // Exclude the current record from uniqueness check
  }
}

Response 200:
{ "valid": true }

Response 200 (invalid):
{ "valid": false, "message": "A vendor with this email already exists" }
```

The `context` object is populated from the field's `remote_unique.context` prop, which can include bindings:

```json
{
  "rule": "remote_unique",
  "endpoint": "/api/validate/unique",
  "field": "email",
  "context": {
    "exclude_id": "$route.params.vendor_id"
  }
}
```

---

## 23.6 Server-Side Validation

Every mutation endpoint validates its request body. Client-side validation passing is not a precondition for server-side validation — the server always validates independently.

### 23.6.1 Validation Middleware

Go validation middleware runs before the handler function. It validates the request body against the declared input schema (defined with Go struct tags, validated using the `ozzo-validation` library or a custom validation framework).

```go
// Example handler with validation
func (h *InvoiceHandler) Create(ctx context.Context, payload *CreateInvoicePayload) (*InvoiceResponse, error) {
    if err := payload.Validate(ctx, h.store); err != nil {
        return nil, apperr.ValidationFailed(err)
    }
    // ... handler logic
}

func (p *CreateInvoicePayload) Validate(ctx context.Context, store Store) error {
    return validation.ValidateStructWithContext(ctx, p,
        validation.Field(&p.VendorID, validation.Required, validation.By(store.VendorExists)),
        validation.Field(&p.Amount, validation.Required, validation.Min(0.01)),
        validation.Field(&p.DueDate, validation.Required, validation.Min(time.Now())),
        validation.Field(&p.LineItems, validation.Required, validation.Length(1, 500)),
    )
}
```

### 23.6.2 422 Response Format

When validation fails, the server returns `422 Unprocessable Entity` with a structured error body:

```json
{
  "error": "validation_failed",
  "message": "The request contains validation errors",
  "field_errors": {
    "amount": "Amount must be greater than zero",
    "due_date": "Due date cannot be in the past",
    "vendor_id": "Vendor not found",
    "line_items[2].quantity": "Quantity is required",
    "line_items[2].unit_price": "Unit price must be greater than zero"
  }
}
```

**Field error key conventions:**
- Top-level fields: `"field_name"`
- Nested object fields: `"object.field_name"`
- Array item fields: `"array_name[index].field_name"`
- Non-field errors: `"_form"` (displayed in the form-level error summary)

### 23.6.3 Client Error Distribution

The rendering engine's form layer receives the `422` response and distributes `field_errors` to the corresponding form fields. Each field that has an entry in `field_errors` is marked as invalid with the server's error message, replacing any client-side validation message. Fields not mentioned in `field_errors` are unaffected.

The distribution logic:
1. Receives `field_errors` map from the 422 response
2. For each key in `field_errors`:
   - Finds the form field with matching `id`
   - Sets the field's error message to the value
   - Marks the field as `touched` (to ensure the error is visible)
3. Scrolls to the first field with an error
4. Focuses the first field with an error

---

## 23.7 Validation Error Display

### 23.7.1 Inline Field Errors

Field errors appear inline below the field, in a `role="alert"` element linked to the field's `aria-errormessage` attribute. Error messages appear only after the user has touched the field (on_blur) or after a submit attempt.

```
┌─────────────────────────────────────────┐
│  Due Date *                             │
│  ┌───────────────────────────────────┐  │
│  │  15/03/2024                       │  │  ← field in error state (red border)
│  └───────────────────────────────────┘  │
│  ⚠ Due date cannot be in the past      │  ← inline error message
└─────────────────────────────────────────┘
```

### 23.7.2 Form-Level Validation Summary

When a form has multiple errors at submit time, a validation summary appears above the action bar (or at the top of the form for long forms). The summary lists all fields with errors, each as a clickable link that scrolls to and focuses the offending field.

```json
{
  "type": "form.container",
  "props": {
    "validation_summary": {
      "position": "top",          // "top" | "bottom" | "action_bar"
      "show_on": "submit_attempt" // "always" | "submit_attempt"
    }
  }
}
```

### 23.7.3 Section-Level Errors

For tabbed or wizard forms, errors in non-visible sections are indicated by an error badge on the tab or step indicator. This ensures users know validation errors exist even in sections they are not currently viewing.

---

## 23.8 Validation in Multi-Step Forms

Wizard/stepper forms (Chapter 11, Section 11.7.3) validate one step at a time. When the user attempts to advance to the next step:

1. All fields in the current step are validated (Layers 1–3)
2. All cross-field validations for the current section are run
3. If any field has errors, the advance is blocked and errors are displayed
4. Remote validations for the current step are awaited before advancing

This allows the user to progress through the form one section at a time without encountering all errors at once. The server performs full validation on the final submit.

```json
{
  "type": "form.container",
  "props": {
    "layout": "wizard",
    "steps": [
      {
        "id": "vendor_details",
        "label": "Vendor Details",
        "validate_on_next": true,    // Default: true for wizard forms
        "fields": ["name", "email", "phone", "tax_id"]
      },
      {
        "id": "bank_details",
        "label": "Bank Details",
        "validate_on_next": true,
        "fields": ["account_name", "account_number", "bank_code"]
      },
      {
        "id": "review",
        "label": "Review & Submit",
        "validate_on_next": false    // Final step — validate on submit
      }
    ]
  }
}
```

---

## 23.9 Business Rule Validation

Business rules are complex validations that go beyond field format checks. They involve cross-table lookups, state machine constraints, financial calculations, and approval logic. Business rule validation is always server-side.

### 23.9.1 Examples of Business Rules

- **Budget constraint:** A purchase order line item cannot be added if it would cause the PO total to exceed the approved budget (requires looking up the budget from the budget table)
- **Approval state machine:** An invoice cannot be submitted for payment if it is already in "paid" status
- **Credit limit check:** A sales order cannot exceed the customer's available credit limit
- **Period close:** A journal entry cannot be posted to a closed accounting period
- **Quantity available:** A stock transfer cannot exceed the available quantity at the source location

### 23.9.2 Business Rule Error Responses

Business rule violations return 422 with a `code` field that identifies the specific rule:

```json
{
  "error": "validation_failed",
  "message": "Cannot submit this purchase order",
  "field_errors": {},
  "business_rule_errors": [
    {
      "code": "budget.exceeded",
      "message": "PO total (KES 245,000) exceeds approved budget (KES 200,000)",
      "detail": "The remaining budget is KES −45,000. Reduce line items or request a budget increase.",
      "action": {
        "label": "Request Budget Increase",
        "action_id": "open_budget_increase_request"
      }
    }
  ]
}
```

Business rule errors are displayed in the form-level error summary, separate from field errors. When the error includes an `action`, the summary renders the action as a button that executes the specified action ID.

---

## 23.10 Validation at the AST Compiler

The AST compilation pipeline (Chapter 8) performs structural validation of surface definitions before they are served to clients. This catches authoring errors at definition time rather than at runtime.

### 23.10.1 Structural Validation Checks

The compiler validates:
- All component types are registered in the platform's component registry
- All `action_id` references in event handlers resolve to declared actions in the `actions` map
- All `data_source_id` references resolve to declared data sources
- All `state.var_name` references resolve to declared state variables
- All slot assignments match the component's declared slot definitions
- Payload size budget (≤ 200 KB web, ≤ 100 KB mobile) is not exceeded
- Data source `max_rows` does not exceed 2,000

### 23.10.2 Validation Error Format (Compiler)

When the compiler finds validation errors, it returns them as a structured list with the path to the offending node:

```json
{
  "valid": false,
  "errors": [
    {
      "path": "root.children[2].events.on_click",
      "code": "unresolved_action_id",
      "message": "Action 'approve_invoice' is referenced by node 'approve_button' but not declared in the 'actions' map"
    },
    {
      "path": "root.children[5].props.data_source_id",
      "code": "unresolved_data_source",
      "message": "Data source 'invoice_totals' is referenced but not declared"
    }
  ]
}
```

In development mode, the compiler returns the full error list and a partial definition (for surfaces where some parts are valid). In production, a definition with fatal errors is not served — the endpoint returns `400 Bad Request` with the error list.

---

*End of Chapter 23*

**Previous:** [Chapter 22 — Client State Management](./22-client-state-management.md)
**Next:** [Chapter 24 — Data Sources](./24-data-sources.md)
