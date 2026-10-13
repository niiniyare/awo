# Chapter 30 — Customization

> **Volume:** VI — Platform APIs & Configuration
> **Phase:** 3
> **Audience:** Platform Engineers, Product Engineers, Tenant Administrators
> **Prerequisites:** Chapters 05–08, 22, 25, 28, 29

---

## Table of Contents

- [30.1 Customization Philosophy](#301-customization-philosophy)
- [30.2 Tenant Theme Customization](#302-tenant-theme-customization)
- [30.3 Custom Fields (Field Extensions)](#303-custom-fields-field-extensions)
- [30.4 Custom Validation Rules](#304-custom-validation-rules)
- [30.5 Surface Overrides](#305-surface-overrides)
- [30.6 Workflow Customization](#306-workflow-customization)
- [30.7 Label and Terminology Overrides](#307-label-and-terminology-overrides)
- [30.8 Customization in the Compilation Pipeline](#308-customization-in-the-compilation-pipeline)

---

## 30.1 Customization Philosophy

AwoERP customization operates at two levels:

**Configuration-level customization** (covered here) — tenant administrators configure the platform to match their business needs using the admin UI. No code is required. This includes themes, custom fields, label overrides, workflow steps, and surface section visibility.

**Extension-level customization** (covered in Chapter 31) — developers write plugins that add new component types, new API endpoints, or new workflow activities. This requires code and goes through the plugin distribution process.

**What cannot be customized** — The SDUI JSON schema, the binding syntax, the action system protocol, and the security model are not configurable. These are platform contracts that all clients depend on.

---

## 30.2 Tenant Theme Customization

### 30.2.1 Theme Tokens

Tenant admins can override the platform's design tokens via the `tenant_settings` → `Appearance` surface. The override set is stored as a token map:

```json
{
  "color_primary": "#1A5276",
  "color_primary_hover": "#154360",
  "color_primary_text": "#FFFFFF",
  "color_secondary": "#D4AC0D",
  "color_success": "#1E8449",
  "color_error": "#C0392B",
  "color_warning": "#D68910",
  "font_family_base": "Inter, sans-serif",
  "font_size_base": "14px",
  "border_radius_base": "4px",
  "border_radius_large": "8px",
  "logo_url": "https://cdn.awoerp.com/tenants/ten-456/logo.png",
  "favicon_url": "https://cdn.awoerp.com/tenants/ten-456/favicon.ico"
}
```

### 30.2.2 Token Injection

During surface compilation, the tenant's theme token overrides are merged with the platform defaults and included in the surface definition:

```json
{
  "surface_id": "invoice_list",
  "design_tokens": {
    "light": {
      "color_primary": "#1A5276",
      "color_primary_hover": "#154360",
      ...
    },
    "dark": {
      "color_primary": "#2E86C1",
      ...
    }
  },
  "root": { ... }
}
```

The web engine injects these as CSS custom properties on `:root`. Mobile engines apply them as platform design system tokens.

### 30.2.3 Logo and Branding

The tenant logo appears in the navigation shell. It is:
- Uploaded via the Appearance settings surface (max 2MB, PNG or SVG)
- Stored in tenant-scoped blob storage
- Served via CDN with appropriate cache headers

---

## 30.3 Custom Fields (Field Extensions)

### 30.3.1 Custom Field Declaration

Tenant admins can add custom fields to standard entities (Invoice, Vendor, Employee, etc.) through the `admin_fields` surface. Each custom field is:
- Stored in the `tenant_field_extensions` table
- Rendered in a "Custom Fields" section on the relevant surfaces
- Included in API responses as a `custom_fields` object
- Searchable and filterable (for indexed field types)

```sql
CREATE TABLE tenant_field_extensions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    entity_type     TEXT NOT NULL,   -- 'invoice' | 'vendor' | 'employee' | ...
    field_key       TEXT NOT NULL,   -- snake_case key, unique per entity+tenant
    label           TEXT NOT NULL,
    field_type      TEXT NOT NULL,   -- text | number | date | boolean | select | multiselect
    required        BOOLEAN NOT NULL DEFAULT false,
    options         JSONB,           -- For select/multiselect
    validation      JSONB,           -- Validation rule declarations
    display_order   INT NOT NULL DEFAULT 0,
    active          BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, entity_type, field_key)
);
```

### 30.3.2 Custom Field Values Storage

Custom field values are stored as JSONB in a companion table alongside the entity:

```sql
CREATE TABLE invoice_custom_fields (
    invoice_id  UUID NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    tenant_id   UUID NOT NULL,
    values      JSONB NOT NULL DEFAULT '{}',
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (invoice_id, tenant_id)
);
```

The JSONB schema is: `{ "field_key": value, ... }`. The `values` column is updated as a whole on form submission.

### 30.3.3 Custom Field API Response

Standard entity API responses include custom field values:

```json
{
  "data": {
    "id": "inv-001",
    "invoice_number": "INV-2024-001",
    "amount": 145000,
    "status": "approved",
    "custom_fields": {
      "po_reference": "PO-2024-789",
      "department": "engineering",
      "project_code": "PRJ-42"
    }
  }
}
```

### 30.3.4 Custom Field Compilation

When the SDUI compiler builds a form surface for an entity that has custom fields, it retrieves the tenant's `tenant_field_extensions` for that entity type and appends a "Custom Fields" section to the form:

```go
func (c *Compiler) appendCustomFields(ctx context.Context,
    section *FormSection, entityType string, tenantID string) error {

    extensions, err := c.store.GetFieldExtensions(ctx, tenantID, entityType)
    if err != nil || len(extensions) == 0 {
        return err
    }

    customSection := &FormSection{
        Title: "Custom Fields",
        Fields: make([]FieldNode, 0, len(extensions)),
    }

    for _, ext := range extensions {
        if !ext.Active {
            continue
        }
        field := fieldNodeFromExtension(ext)
        // Bind to custom_fields.{key} instead of top-level field
        field.ID = fmt.Sprintf("custom_fields.%s", ext.FieldKey)
        customSection.Fields = append(customSection.Fields, field)
    }

    section.Subsections = append(section.Subsections, customSection)
    return nil
}
```

---

## 30.4 Custom Validation Rules

Tenant admins can add validation rules to custom fields through the field configuration UI. The supported rule set is the same as the platform validation framework (Chapter 23):

```json
{
  "field_key": "po_reference",
  "validation": {
    "required": true,
    "pattern": "^PO-\\d{4}-\\d+$",
    "pattern_message": "Must be in format PO-YYYY-number",
    "max_length": 20
  }
}
```

Custom validation rules are compiled into the surface definition's validation layer exactly as platform-defined rules are. They participate in all five validation layers (Chapter 23).

---

## 30.5 Surface Overrides

### 30.5.1 Section Visibility Overrides

Tenant admins can hide standard sections of built-in surfaces that are not relevant to their business:

```sql
CREATE TABLE tenant_surface_overrides (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    surface_id      TEXT NOT NULL,
    overrides       JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, surface_id)
);
```

Override schema:
```json
{
  "sections": {
    "section_tax_details": { "visible": false },
    "section_attachments": { "visible": true }
  },
  "fields": {
    "field_currency": { "required": true, "default_value": "KES" }
  }
}
```

### 30.5.2 Field Default Values

Tenant admins can set default values for standard fields to reduce data entry for their context. The most common use case is pre-setting the currency field for all invoices in a single-currency operation:

```json
{
  "fields": {
    "field_currency": { "default_value": "KES" },
    "field_department": { "default_value": "engineering" }
  }
}
```

These defaults are injected into the surface definition's state initialization.

### 30.5.3 Limitations

Surface overrides cannot:
- Add new required fields that don't exist in the entity schema
- Change the data type of existing fields
- Override security-related fields (permissions, audit fields)
- Remove fields that are required by the API contract

---

## 30.6 Workflow Customization

### 30.6.1 Approval Workflow Configuration

Built-in approval workflows (invoice approval, purchase order approval) can be configured without code. Tenant admins can set:
- Approval tiers (single-level, two-level, amount-based tiers)
- Approver roles per tier
- Escalation rules (e.g., auto-escalate after 48 hours)
- Notification recipients

```json
{
  "workflow_key": "invoice_approval",
  "tiers": [
    {
      "tier": 1,
      "condition": "$amount <= 500000",
      "approver_roles": ["invoice_approver"],
      "escalate_after_hours": 48,
      "escalate_to_roles": ["finance_manager"]
    },
    {
      "tier": 2,
      "condition": "$amount > 500000",
      "approver_roles": ["finance_manager"],
      "escalate_after_hours": 24,
      "escalate_to_roles": ["tenant_admin"]
    }
  ],
  "notify_on_submit": ["finance_manager"],
  "notify_on_approve": ["invoice_creator"],
  "notify_on_reject": ["invoice_creator", "finance_manager"]
}
```

This configuration is stored in `tenant_workflow_configs` and is loaded by the Temporal workflow activities that drive the approval process.

### 30.6.2 Email Notification Templates

Notification email templates can be customized per tenant:

```sql
CREATE TABLE tenant_notification_templates (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL,
    event_key   TEXT NOT NULL,   -- e.g. 'invoice.submitted', 'invoice.approved'
    subject     TEXT NOT NULL,
    body_html   TEXT NOT NULL,
    body_text   TEXT NOT NULL,
    UNIQUE(tenant_id, event_key)
);
```

Templates use Handlebars syntax with a context object that includes the relevant entity fields.

---

## 30.7 Label and Terminology Overrides

Different industries and regions use different terminology for the same concepts. AwoERP supports terminology customization at the tenant level:

```sql
CREATE TABLE tenant_label_overrides (
    tenant_id   UUID NOT NULL,
    label_key   TEXT NOT NULL,
    value       TEXT NOT NULL,
    locale      TEXT NOT NULL DEFAULT 'en',
    PRIMARY KEY (tenant_id, label_key, locale)
);
```

Example overrides:
| Label Key | Platform Default | Tenant Override |
|-----------|-----------------|----------------|
| `entity.invoice` | Invoice | Tax Invoice |
| `entity.vendor` | Vendor | Supplier |
| `action.approve` | Approve | Authorize |
| `status.draft` | Draft | In Progress |

Label overrides are compiled into the surface definition's locale layer. The rendering engine uses compiled labels; it never looks up overrides at runtime.

---

## 30.8 Customization in the Compilation Pipeline

### 30.8.1 Customization Resolution Order

When compiling a surface definition for a tenant, the compiler applies customizations in this order:

```
1. Load base surface definition (platform default)
2. Apply tenant field extensions (add custom fields)
3. Apply tenant surface overrides (hide sections, set defaults)
4. Apply tenant label overrides (replace label strings)
5. Apply tenant theme tokens (inject into design_tokens)
6. Apply workflow config overrides (configure workflow surfaces)
7. Filter by user permissions (remove unauthorized elements)
8. Validate and serialize
```

### 30.8.2 Cache Invalidation on Customization Change

Customization changes invalidate the relevant surface definition cache entries. Because the tenant ID is part of the cache key, only the affected tenant's cached definitions are invalidated:

```go
func (s *CustomizationService) UpdateFieldExtension(ctx context.Context,
    tenantID string, entityType string, extension FieldExtension) error {

    if err := s.store.UpsertFieldExtension(ctx, tenantID, extension); err != nil {
        return err
    }

    // Invalidate compiled surfaces for this entity type
    affectedSurfaces := s.surfaceIndex.SurfacesForEntityType(entityType)
    for _, surfaceID := range affectedSurfaces {
        s.cache.InvalidateSurface(ctx, surfaceID, tenantID)
    }

    return nil
}
```

---

*End of Chapter 30*

**Previous:** [Chapter 29 — Feature Flags](./29-feature-flags.md)
**Next:** [Chapter 31 — Plugins](./31-plugins.md)
