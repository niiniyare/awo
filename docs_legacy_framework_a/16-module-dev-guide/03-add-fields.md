> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Add Fields"
id: mdg-03
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Define an Entity](02-define-entity.md)"
  - "[Add Edges](04-add-edges.md)"
  - "[Fields](../04-domain/fields.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Add Fields

**MDG-03 | Module Developer Guide**

This document adds `FieldDef` declarations to the `crm_contact` entity, covering common field types, constraints, and naming rules.

---

## 1. Field Naming Rules

- Snake case: `first_name`, `email_address`, `assigned_to`
- No module prefix — field names are scoped to the entity
- Stable after the first data is written — renaming requires a migration for system entities and a data migration for custom entities

---

## 2. Add Fields to ContactDefinition

```go
// internal/core/crm/def.go
var ContactDefinition = def.EntityDefinition{
    Name:         "crm_contact",
    Module:       "crm",
    Label:        "Contact",
    LabelPlural:  "Contacts",
    StorageModel: def.StorageCustom,

    Fields: []def.FieldDef{
        // Required text field — short string, GIN-indexed for search
        {
            Name:       "full_name",
            Type:       def.FieldData,
            Label:      "Full Name",
            Required:   true,
            MaxLen:     128,
            Searchable: true,  // enables GIN trigram index → full-text search
        },

        // Email — unique per tenant, searchable
        {
            Name:       "email",
            Type:       def.FieldData,
            Label:      "Email",
            Required:   true,
            MaxLen:     256,
            Unique:     true,
            Searchable: true,
            Validators: []def.FieldValidator{&EmailValidator{}},
        },

        // Phone — optional
        {
            Name:   "phone",
            Type:   def.FieldData,
            Label:  "Phone",
            MaxLen: 32,
        },

        // Status — controlled vocabulary
        {
            Name:    "status",
            Type:    def.FieldSelect,
            Label:   "Status",
            Options: []string{"Lead", "Prospect", "Active", "Inactive", "Lost"},
            Default: "Lead",
        },

        // Assigned sales rep — Link to the user entity (IAM module)
        {
            Name:       "assigned_to",
            Type:       def.FieldLink,
            Label:      "Assigned To",
            LinkTarget: "user",
            Required:   true,
        },

        // Long-form notes — not indexed, not searchable
        {
            Name:  "notes",
            Type:  def.FieldLongText,
            Label: "Notes",
        },

        // Source — how the contact was acquired
        {
            Name:    "source",
            Type:    def.FieldSelect,
            Label:   "Source",
            Options: []string{"Referral", "Website", "Event", "Cold Outreach", "Social Media", "Other"},
            Default: "Other",
        },

        // Tags — multi-select for flexible categorization
        {
            Name:    "tags",
            Type:    def.FieldMultiSelect,
            Label:   "Tags",
            Options: []string{"VIP", "Decision Maker", "Technical", "Finance", "Operations"},
        },

        // Contact date — when first engaged
        {
            Name:  "first_contact_date",
            Type:  def.FieldDate,
            Label: "First Contact Date",
        },
    },
}
```

---

## 3. Custom FieldValidator

The `EmailValidator` referenced in the `email` field:

```go
// internal/core/crm/hooks.go (or def.go)
type EmailValidator struct{}

func (v *EmailValidator) Validate(ctx context.Context, value any) error {
    email, ok := value.(string)
    if !ok || email == "" {
        return nil  // Required check handles empty; this handles format
    }
    if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
        return fmt.Errorf("must be a valid email address")
    }
    return nil
}
```

The validator error message becomes the field-level validation message in the HTTP 422 response.

---

## 4. System Fields (Automatic)

These fields are added automatically by the framework to every entity — do NOT declare them:

| Field | Type | Notes |
|---|---|---|
| `id` | UUID v7 | Primary key, time-ordered |
| `tenant_id` | UUID | Tenant isolation |
| `created_at` | timestamptz | Set on create |
| `updated_at` | timestamptz | Updated on every write |
| `created_by` | UUID | Actor user ID |
| `updated_by` | UUID | Actor user ID |

Declaring any of these in `Fields` causes a compilation error.

---

## 5. Adding a NamingSeries Field

If `crm_contact` needs an auto-generated reference number:

```go
{
    Name:   "ref_number",
    Type:   def.FieldNamingSeries,
    Label:  "Reference #",
    Series: "CRM-{YYYY}-{SEQ:5}",
    // Result: CRM-2024-00001, CRM-2024-00002, ...
    // TenantOverridable: true  — allows tenants to customize the prefix
},
```

NamingSeries values are assigned atomically during create — concurrent creates never produce the same number.

---

## 6. Sensitive Fields

For fields containing PII that should be excluded from logs and standard API responses:

```go
{
    Name:      "national_id",
    Type:      def.FieldData,
    Label:     "National ID",
    MaxLen:    20,
    Sensitive: true,  // excluded from logs, error messages, standard API responses
},
```

Sensitive fields appear in API responses only when the actor has explicit `sensitive-read` permission on the entity.

---

## 7. What SDUI Generates

With these fields declared, the framework generates:
- **List view**: columns for `full_name`, `email`, `status`, `assigned_to`, `first_contact_date`
- **Create form**: all non-system fields, with appropriate controls (text input, select dropdown, date picker)
- **Edit form**: same as create form
- **Detail view**: all fields with read-only display

The generated defaults are correct for most cases. Override only when the layout genuinely requires it (see [MDG-09: Add SDUI](09-add-sdui.md)).

---

## Next: [Add Edges →](04-add-edges.md)
