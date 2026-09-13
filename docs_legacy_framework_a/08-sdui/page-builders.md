> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Page Builders"
id: sdui-002
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: normative
related:
  - "[Page Schema](page-schema.md)"
  - "[amis Integration](amis-integration.md)"
  - "[EntityDefinition](../03-kernel/entity-def.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Page Builders

**SDUI-002 | Status: Accepted | Stability: Stable**

This document specifies the PageBuilderSet, the PageBuilderFunc signature, the PageSchemaContext, and patterns for building custom page schemas.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. When to Use Page Builders

Default-generated schemas handle standard ERP views adequately. Page builders are for genuine exceptions:
- Multi-column layouts not achievable with default field ordering
- Custom action confirmation dialogs with rich content
- Detail views with tabs, charts, or embedded sub-views
- List views with complex filters or custom column formatting
- Approval workflow status displays with timeline visualization

Do not override a default view simply for cosmetic preferences. The generated defaults are consistent across the application. Overrides create maintenance burden.

---

## 2. PageBuilderSet

```go
type PageBuilderSet struct {
    List   PageBuilderFunc // override list page
    Create PageBuilderFunc // override create form
    Edit   PageBuilderFunc // override edit form
    Detail PageBuilderFunc // override detail view
}

type PageBuilderFunc func(ctx context.Context, psc PageSchemaContext) (PageSchema, error)
```

Only assign the fields you need to override. Nil fields use the framework default.

---

## 3. PageSchemaContext

```go
type PageSchemaContext struct {
    // The compiled entity definition (read-only)
    Entity *CompiledEntity

    // The requesting actor (for permission-gated elements)
    Actor Actor

    // Tenant context (for tenant-specific customization)
    Tenant TenantContext

    // Feature flags evaluated for this tenant and actor
    Flags FeatureFlagSet

    // Helper for building permission-gated elements:
    // returns nil if actor lacks the permission, otherwise returns the element
    IfPermitted func(permission string, element PageElement) PageElement

    // Helper for building amis schema elements
    Builder *SchemaBuilder
}
```

---

## 4. PageSchema and SchemaBuilder

`PageSchema` is an `any` that serializes to the amis JSON format. The `SchemaBuilder` provides Go-typed helpers for constructing common amis elements:

```go
func BuildInvoiceDetailPage(ctx context.Context, psc PageSchemaContext) (entity.PageSchema, error) {
    b := psc.Builder

    return b.Page(
        b.Toolbar(
            // Submit button — only if actor has accounts_payable role
            psc.IfPermitted("role:finance.accounts_payable",
                b.ActionButton("Submit", "submit", b.ConfirmDialog("Submit this invoice for approval?")),
            ),
            // Cancel button
            psc.IfPermitted("role:finance.accounts_payable",
                b.ActionButton("Cancel", "cancel", b.ConfirmDialog("Cancel this invoice?")),
            ),
        ),
        b.Tabs(
            b.Tab("Details",
                b.TwoColumnForm(
                    b.FieldDisplay("number", "Invoice #"),
                    b.FieldDisplay("customer", "Customer"),
                    b.FieldDisplay("status", "Status"),
                    b.FieldDisplay("due_date", "Due Date"),
                    b.FieldDisplay("total_kes", "Total"),
                ),
            ),
            b.Tab("Lines",
                b.EmbeddedList("lines",
                    b.Column("description", "Description"),
                    b.Column("quantity", "Qty"),
                    b.Column("unit_price", "Unit Price"),
                    b.Column("total", "Total"),
                ),
            ),
            b.Tab("Notes",
                b.FieldDisplay("notes", "Notes"),
            ),
        ),
    ), nil
}
```

---

## 5. Available SchemaBuilder Primitives

The SchemaBuilder provides typed wrappers for amis elements. All elements serialize to valid amis JSON:

**Layout:**
- `b.Page(...)` — top-level page container
- `b.Tabs(...tab)` — tabbed panel
- `b.Tab(label, ...body)` — single tab
- `b.TwoColumnForm(...)` — two-column grid layout
- `b.Card(title, ...body)` — card panel
- `b.Divider()` — horizontal rule

**Forms and Fields:**
- `b.FieldDisplay(name, label)` — read-only field display
- `b.TextField(name, label)` — text input (edit forms)
- `b.SelectField(name, label, options)` — dropdown (edit forms)
- `b.DateField(name, label)` — date picker
- `b.CurrencyField(name, label)` — currency input

**Lists:**
- `b.EmbeddedList(edgeName, ...columns)` — embedded sub-table for a OneToMany edge
- `b.Column(field, label)` — list column definition
- `b.LinkColumn(field, label, entityType)` — clickable FK column

**Actions:**
- `b.ActionButton(label, actionName)` — action trigger button
- `b.ConfirmDialog(message)` — confirmation dialog wrapping an action
- `b.Toolbar(...elements)` — action toolbar

**Data:**
- `b.Chart(type, field)` — amis chart (bar, line, pie)
- `b.StatCard(label, field)` — KPI summary card

---

## 6. Raw amis JSON Escape Hatch

When the SchemaBuilder does not provide the needed element, raw amis JSON may be embedded:

```go
rawElement := map[string]any{
    "type":   "tpl",
    "tpl":    "${status | toDate | date: 'YYYY-MM-DD'}",
    "label":  "Formatted Status",
}
return b.Page(rawElement), nil
```

Raw amis JSON escapes the type safety of the SchemaBuilder. Use it only when no typed primitive is available. When used, add a `// TODO: add SchemaBuilder primitive for {element_type}` comment to track the missing primitive.

---

## Related Documents

- [Page Schema](page-schema.md) — how schemas are cached and served
- [amis Integration](amis-integration.md) — available amis features and limitations
- [EntityDefinition §10](../03-kernel/entity-def.md#10-page-builders) — PageBuilderSet declaration
- [Glossary](../GLOSSARY.md) — Page Builder, PageBuilderSet, Page Schema, SDUI
