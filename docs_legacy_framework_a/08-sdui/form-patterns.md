> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Form Patterns"
id: sdui-005
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Page Builders](page-builders.md)"
  - "[Page Schema](page-schema.md)"
  - "[Fields](../04-domain/fields.md)"
  - "[Hooks](../04-domain/hooks.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Form Patterns

**SDUI-005 | Status: Accepted | Stability: Stable**

This document describes common patterns for forms in Awo's SDUI layer: conditional field visibility, field dependencies, inline validation display, multi-step forms, and embedded child entity forms.

---

## 1. Default Form Generation

Every `EntityDefinition` auto-generates create and edit forms from its `Fields` declaration. No `PageBuilderSet` override is needed for standard CRUD forms.

Auto-generated create form rules:
- `NamingSeries` fields: absent (server assigns on create)
- `Immutable` fields: absent (cannot be set on create and cannot be changed on edit)
- `Sensitive` fields: excluded from read responses; present as masked input on forms
- `Required` fields: rendered with red asterisk and client-side required validation
- `Select` fields: rendered as dropdown with options from `FieldDef.Options`
- `Bool` fields: rendered as toggle/switch
- `Currency` fields: rendered with currency prefix label (KES)
- `Date` / `DateTime` fields: rendered with date picker
- `Link` fields: rendered as typeahead search (calls entity list API)

Override only when auto-generation is insufficient for the use case.

---

## 2. Conditional Field Visibility

Show or hide fields based on another field's value:

```go
func BuildInvoiceCreateForm(ctx context.Context, actor session.Actor) ([]byte, error) {
    psc := sdui.NewPageSchemaContext(ctx, actor)

    form := amis.Form(amis.FormProps{
        API:    "POST /api/v1/entities/finance_invoice",
        Fields: []amis.FormField{
            amis.SelectField("type", "Invoice Type").
                Options([]string{"Standard", "Recurring", "ProForma"}),

            amis.LinkField("customer", "Customer").
                Required(true),

            amis.CurrencyField("total_kes", "Total (KES)").
                Required(true),

            // Only shown when type = "Recurring"
            amis.SelectField("recurrence_interval", "Recurrence").
                Options([]string{"Monthly", "Quarterly", "Annually"}).
                VisibleWhen("${type === 'Recurring'}"),

            // Only shown when type = "Recurring"
            amis.DateField("recurrence_end_date", "Recurrence End Date").
                VisibleWhen("${type === 'Recurring'}"),

            amis.LongTextField("notes", "Notes"),
        },
    })

    return psc.Marshal(form)
}
```

`VisibleWhen` uses amis expression syntax (`${...}`) evaluated client-side against the form's data context. It only affects visibility, not validation — a hidden required field does not block submission.

---

## 3. Field Dependencies (Cascading Dropdowns)

When one field's options depend on another field's value, use a dependent API call:

```go
amis.SelectField("country", "Country").
    Options(amis.APIOptions("/api/v1/entities/country?page_size=250")),

amis.SelectField("county", "County").
    Options(amis.APIOptions("/api/v1/entities/county?country=${country}")).
    Depends("country").  // clears value when "country" changes
    VisibleWhen("${!!country}"),
```

The `Depends` call ensures amis re-fetches county options and clears the selected county whenever the country selection changes.

---

## 4. Inline Validation Display

Server-side validation errors (HTTP 422) from the `before_validate` hook are automatically mapped to field-level error messages by the amis form if the response follows the amis error format:

```json
{
  "status": 422,
  "msg": "Validation failed",
  "errors": {
    "total_kes": "Invoice total must be greater than zero.",
    "customer": "Customer is required for submitted invoices."
  }
}
```

The framework maps `ValidationError.Fields` to this format automatically. No additional page builder code needed.

Custom field validators in FieldDef also produce this format:

```go
{
    Name: "email",
    Type: def.FieldData,
    Validators: []entity.FieldValidator{EmailValidator{}},
}
// EmailValidator.Validate returns: &ValidationError{Fields: {"email": "Invalid email format."}}
```

---

## 5. Multi-Step Forms

For complex entity creation that benefits from progressive disclosure:

```go
func BuildOnboardingForm(ctx context.Context, actor session.Actor) ([]byte, error) {
    psc := sdui.NewPageSchemaContext(ctx, actor)

    wizard := amis.Wizard(amis.WizardProps{
        Mode: "horizontal",
        Steps: []amis.WizardStep{
            {
                Title: "Basic Info",
                Fields: []amis.FormField{
                    amis.DataField("name", "Contact Name").Required(true),
                    amis.DataField("email", "Email").Required(true),
                    amis.SelectField("type", "Contact Type").
                        Options([]string{"Lead", "Customer", "Partner"}),
                },
            },
            {
                Title: "Company",
                Fields: []amis.FormField{
                    amis.DataField("company_name", "Company Name"),
                    amis.SelectField("industry", "Industry").
                        Options(amis.APIOptions("/api/v1/entities/industry")),
                    amis.DataField("website", "Website"),
                },
            },
            {
                Title: "Assignment",
                Fields: []amis.FormField{
                    amis.LinkField("assigned_to", "Assigned Sales Rep").
                        Required(true),
                    amis.SelectField("priority", "Priority").
                        Options([]string{"Low", "Medium", "High"}),
                    amis.LongTextField("notes", "Initial Notes"),
                },
                // Submit on this final step
                SubmitAPI: "POST /api/v1/entities/crm_contact",
            },
        },
    })

    return psc.Marshal(wizard)
}
```

The wizard submits once — on the final step — with the entire collected data. It does not create a partial entity at each step.

---

## 6. Inline Child Entity Form (One-to-Many)

For parent+children creation in a single form (e.g., Invoice + Invoice Lines):

```go
amis.Form(amis.FormProps{
    API: "POST /api/v1/entities/finance_invoice",
    Fields: []amis.FormField{
        amis.LinkField("customer", "Customer").Required(true),
        amis.SelectField("status", "Status").Options([]string{"Draft"}),

        // Inline table for invoice lines
        amis.SubFormTable("lines", "Invoice Lines").
            Columns([]amis.Column{
                {Name: "description", Label: "Description", Type: "input-text", Required: true},
                {Name: "quantity",    Label: "Qty",         Type: "input-number", Required: true},
                {Name: "unit_price",  Label: "Unit Price",  Type: "input-number", Required: true},
                {Name: "amount",      Label: "Amount",      Type: "tpl",
                 Template: "${quantity * unit_price | number:2}"},
            }).
            AddButtonLabel("+ Add Line").
            MinRows(1).
            MaxRows(50),

        amis.CurrencyField("total_kes", "Total").ReadOnly(true).
            Value("${SUM(lines, 'amount')}"),

        amis.LongTextField("notes", "Notes"),
    },
})
```

The server receives the `lines` array as part of the parent entity's create input. The parent hook's `BeforeCreate` or the entity's custom create handler is responsible for persisting child records within the same transaction.

---

## 7. Edit Form — Pre-population

Auto-generated edit forms pre-populate from the entity's GET response. Custom edit forms follow the same pattern:

```go
amis.Form(amis.FormProps{
    // GET fetches current values; PATCH submits changes
    API: amis.CRUDFormAPI{
        FetchAPI:  "GET /api/v1/entities/crm_contact/${id}",
        SubmitAPI: "PATCH /api/v1/entities/crm_contact/${id}",
    },
    Fields: []amis.FormField{
        amis.DataField("name", "Contact Name").Required(true),
        amis.DataField("email", "Email").Required(true),
        // NamingSeries field: read-only display, no edit
        amis.StaticField("code", "Contact Code"),
        // ...
    },
})
```

`StaticField` renders a non-editable display of the value. Use it for `Immutable` or `NamingSeries` fields in edit forms.

---

## 8. Confirmation Dialogs for Destructive Actions

For actions that are hard to reverse, wrap the submit in a confirmation dialog:

```go
amis.Button(amis.ButtonProps{
    Label:   "Cancel Invoice",
    Type:    "danger",
    Confirm: &amis.ConfirmProps{
        Title:   "Cancel Invoice?",
        Content: "This cannot be undone. The invoice will be permanently cancelled.",
    },
    OnClick: amis.AjaxAction{
        API:    "POST /api/v1/entities/finance_invoice/${id}/cancel",
        Reload: "window",
    },
})
```

Always use `Confirm` for destructive entity actions (`cancel`, `archive`, `delete`). The amis confirmation dialog is built-in — no custom modal needed.

---

## 9. Form Anti-Patterns

### Business Logic in VisibleWhen

```go
// WRONG: business rule in VisibleWhen expression
amis.Field("override_price").VisibleWhen("${actor.role === 'manager'}")
// VisibleWhen is client-side and can be bypassed.
// Use server-side permission gating: psc.IfPermitted()

// CORRECT: permission-gated in page builder
if psc.IfPermitted("finance_invoice", "price_override") {
    fields = append(fields, amis.CurrencyField("override_price", "Override Price"))
}
```

### Required Validation in VisibleWhen

```go
// WRONG: hidden field with Required(true)
amis.CurrencyField("tax_amount", "Tax").
    Required(true).
    VisibleWhen("${apply_tax === true}")
// When hidden, amis still validates required — submission blocked

// CORRECT: use server-side validation; client Required is a UX hint only
// Or: set Required(false) and validate conditionally in BeforeCreate hook
```

### Submitting to Bespoke Endpoints

```go
// WRONG: custom dashboard endpoint that bypasses EntityDefinition lifecycle
amis.Form{API: "POST /api/v1/custom/create-invoice-and-notify"}
// Bypasses: hooks, audit log, permission evaluation, workflow triggers

// CORRECT: always POST to the entity CRUD endpoint
amis.Form{API: "POST /api/v1/entities/finance_invoice"}
// Hooks, audit log, and workflow triggers run automatically
```

---

## Related Documents

- [Page Builders](page-builders.md) — `PageBuilderSet`, `PageSchemaContext`, `psc.IfPermitted`
- [Fields](../04-domain/fields.md) — `FieldDef`, field types, `Immutable`, `Sensitive`, `Required`
- [Hooks](../04-domain/hooks.md) — `before_validate` hook for server-side validation
- [API Conventions](../11-api/conventions.md) — validation error format (HTTP 422)
- [Glossary](../GLOSSARY.md) — SDUI, amis, Page Builder, Form
