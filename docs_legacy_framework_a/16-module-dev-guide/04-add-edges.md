> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Add Edges"
id: mdg-04
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Add Fields](03-add-fields.md)"
  - "[Add Hooks](05-add-hooks.md)"
  - "[Edges](../04-domain/edges.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Add Edges

**MDG-04 | Module Developer Guide**

This document adds `EdgeDef` declarations to `crm_contact`, establishing the one-to-many relationship with `crm_interaction`.

---

## 1. Add the Edge to ContactDefinition

```go
// internal/core/crm/def.go
var ContactDefinition = def.EntityDefinition{
    // ... Name, Module, Label, Fields from previous step ...

    Edges: []def.EdgeDef{
        {
            // Edge name — used in API responses and SDUI
            Name: "interactions",

            // Target entity type
            Target: "crm_interaction",

            // Edge cardinality
            Type: def.EdgeOneToMany,

            // Delete all interactions when the contact is deleted
            CascadeDelete: true,

            // Optional: how this edge appears in SDUI detail view
            // (default: embedded sub-table)
        },
    },
}
```

---

## 2. Define the InteractionDefinition

The `crm_interaction` entity referenced by the edge:

```go
var InteractionDefinition = def.EntityDefinition{
    Name:         "crm_interaction",
    Module:       "crm",
    Label:        "Interaction",
    LabelPlural:  "Interactions",
    StorageModel: def.StorageCustom,

    Fields: []def.FieldDef{
        {
            Name:     "contact",
            Type:     def.FieldLink,
            Label:    "Contact",
            LinkTarget: "crm_contact",
            Required: true,
            Immutable: true,  // cannot change which contact an interaction belongs to
        },
        {
            Name:    "type",
            Type:    def.FieldSelect,
            Label:   "Type",
            Options: []string{"Call", "Email", "Meeting", "Note"},
            Required: true,
        },
        {
            Name:  "date",
            Type:  def.FieldDateTime,
            Label: "Date",
            Required: true,
        },
        {
            Name:  "summary",
            Type:  def.FieldSmallText,
            Label: "Summary",
            Required: true,
        },
        {
            Name:  "notes",
            Type:  def.FieldLongText,
            Label: "Notes",
        },
        {
            Name:     "conducted_by",
            Type:     def.FieldLink,
            Label:    "Conducted By",
            LinkTarget: "user",
            Required: true,
        },
        {
            Name:    "outcome",
            Type:    def.FieldSelect,
            Label:   "Outcome",
            Options: []string{"Positive", "Neutral", "Negative", "Follow-up Required"},
        },
    },
}
```

---

## 3. Loading Edges Explicitly

Edges are never lazy-loaded. To retrieve a contact with its interactions, request the edge explicitly:

```go
contact, _, err := repo.Query(ctx, filter.Eq("id", contactID),
    entity.WithEdge("interactions"),
    entity.OrderBy("interactions.date", entity.Desc),
    entity.Limit(10),
)
```

The framework executes a second query (`SELECT ... WHERE contact = $1 ORDER BY date DESC LIMIT 10`) and populates `contact.Edges["interactions"]`.

**Never** attempt to access edges without explicitly requesting them — the value will be nil.

---

## 4. CascadeDelete Behavior

When `CascadeDelete: true` and a contact is deleted:
1. Framework first deletes all `crm_interaction` records where `contact = deletedContactID`
2. Each deletion is logged in the Audit Log (individual entries per deleted interaction)
3. The contact itself is then deleted
4. All deletions occur within the same PostgreSQL transaction

The Audit Log captures the cascade — individual interaction deletions appear as separate audit entries with a note indicating the cascade trigger.

---

## 5. ManyToMany Edges

For a many-to-many relationship (e.g., a contact associated with multiple companies):

```go
{
    Name:   "companies",
    Target: "crm_company",
    Type:   def.EdgeManyToMany,
    // Framework creates a join table: crm_contact_crm_company
    // No CascadeDelete — removing the join row, not the company
},
```

The join table is managed by the framework. Module authors do not write migrations for join tables when using `EdgeManyToMany`.

---

## 6. What SDUI Generates for Edges

The detail view for `crm_contact` automatically includes an embedded sub-table showing `crm_interaction` records. The sub-table shows columns derived from `InteractionDefinition.Fields`.

The list view and forms do NOT show edge data by default — only the detail view.

To override the edge representation in SDUI, use a custom `PageBuilderFunc` (see [MDG-09: Add SDUI](09-add-sdui.md)).

---

## Next: [Add Hooks →](05-add-hooks.md)
