> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Add SDUI"
id: mdg-09
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Add Workflows](08-add-workflows.md)"
  - "[Add Migrations](10-add-migrations.md)"
  - "[Page Builders](../08-sdui/page-builders.md)"
  - "[amis Integration](../08-sdui/amis-integration.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Add SDUI

**MDG-09 | Module Developer Guide**

This document adds a custom detail view to `crm_contact` using `PageBuilderSet`. The custom view shows a two-column layout and an interaction history tab — not achievable with the auto-generated defaults.

---

## 1. When to Override Defaults

The generated default schemas handle ~90% of cases correctly. Override only when:
- Multi-column layout needed (default is single-column)
- Tabbed layout with embedded sub-views
- Custom action confirmation dialogs with rich content
- Complex field ordering that cannot be achieved by field order in the definition

For `crm_contact`, the detail view needs a tabbed layout (Contact Info | Interactions | Notes). The default single-column layout does not support tabs.

---

## 2. Declare the PageBuilderSet

```go
// internal/core/crm/def.go
var ContactDefinition = def.EntityDefinition{
    // ... all previous fields ...

    PageBuilders: def.PageBuilderSet{
        // Only override the Detail view — List, Create, Edit use defaults
        Detail: BuildContactDetailPage,
    },
}
```

---

## 3. Implement the PageBuilderFunc

```go
// internal/core/crm/handler.go (or a separate page_builders.go)
package crm

import (
    "context"
    "awo.so/awo/entity"
)

// BuildContactDetailPage builds the detail view for crm_contact.
// Shows a tabbed layout: Contact Info | Interactions | Notes.
func BuildContactDetailPage(ctx context.Context, psc entity.PageSchemaContext) (entity.PageSchema, error) {
    b := psc.Builder

    return b.Page(
        // Toolbar with actions — only shown if actor has permission
        b.Toolbar(
            psc.IfPermitted("role:crm.sales_rep",
                b.ActionButton("Qualify", "qualify",
                    b.ConfirmDialog("Mark this contact as a qualified lead?"),
                ),
            ),
            psc.IfPermitted("role:crm.manager",
                b.ActionButton("Reassign", "reassign", nil),
            ),
        ),

        b.Tabs(
            // Tab 1: Contact Information
            b.Tab("Contact Info",
                b.TwoColumnForm(
                    b.FieldDisplay("full_name", "Full Name"),
                    b.FieldDisplay("email", "Email"),
                    b.FieldDisplay("phone", "Phone"),
                    b.FieldDisplay("status", "Status"),
                    b.FieldDisplay("source", "Source"),
                    b.FieldDisplay("assigned_to", "Assigned To"),
                    b.FieldDisplay("first_contact_date", "First Contact"),
                    b.FieldDisplay("tags", "Tags"),
                ),
            ),

            // Tab 2: Interaction History
            b.Tab("Interactions",
                b.EmbeddedList("interactions",
                    b.Column("date", "Date"),
                    b.Column("type", "Type"),
                    b.Column("summary", "Summary"),
                    b.Column("conducted_by", "By"),
                    b.Column("outcome", "Outcome"),
                ),
            ),

            // Tab 3: Notes
            b.Tab("Notes",
                b.FieldDisplay("notes", "Notes"),
            ),
        ),
    ), nil
}
```

---

## 4. Permission-Gated Elements

`psc.IfPermitted` returns `nil` if the actor lacks the permission — amis does not render nil elements:

```go
// Only sales reps see the Qualify button
// Only managers see the Reassign button
// Viewers see no action buttons (not hidden/disabled — absent from schema)
psc.IfPermitted("role:crm.sales_rep",
    b.ActionButton("Qualify", "qualify", ...),
),
```

This is more security-correct than disabling buttons — a disabled button reveals what operations exist. An absent element does not.

---

## 5. Feature-Flag-Gated Elements

Use `psc.Flags.IsEnabled()` to conditionally include schema elements based on feature flags:

```go
// Only show the AI insights panel if the feature flag is enabled for this tenant
var aiInsightsPanel entity.PageElement
if psc.Flags.IsEnabled("crm.ai_contact_insights") {
    aiInsightsPanel = b.Card("AI Insights",
        b.StatCard("Engagement Score", "ai_engagement_score"),
        b.StatCard("Churn Risk", "ai_churn_risk"),
    )
}

return b.Page(
    // ...
    aiInsightsPanel,  // nil if flag off — amis ignores nil elements
), nil
```

---

## 6. Raw amis JSON Escape Hatch

When the `SchemaBuilder` does not have the needed primitive:

```go
// Custom amis timeline component — no SchemaBuilder wrapper exists yet
timelineComponent := map[string]any{
    "type": "timeline",
    "items": "${interactions | map(i => ({time: i.date, title: i.type, detail: i.summary}))}",
}

// TODO: add SchemaBuilder primitive for timeline component
return b.Page(
    b.Tab("Timeline", timelineComponent),
), nil
```

Add the `TODO:` comment to track missing primitives. Avoid using raw amis JSON for elements that could be wrapped — it bypasses type safety.

---

## 7. Caching Behavior

The custom page schema is cached in Redis:

```
page:crm_contact:{schema_version}:{tenant_id}:{actor_roles_hash}
TTL: 5 minutes
```

Different role combinations produce different schemas (permission-gated elements differ). The cache key includes a hash of the actor's role set, not the individual user ID — actors with the same role combination share a cached schema.

Cache is invalidated when:
- Tenant permissions change (role grant/revoke)
- Feature flags change
- New binary deployed (different schema_version)

---

## 8. Avoid Unnecessary Overrides

Do NOT override the list view just to reorder columns. Reorder fields in `EntityDefinition.Fields` instead — the generated list view respects field declaration order.

Do NOT override the create/edit form for minor layout preferences. The generated form is consistent with all other entities in the application. Overrides create maintenance burden.

---

## Next: [Add Migrations →](10-add-migrations.md)
