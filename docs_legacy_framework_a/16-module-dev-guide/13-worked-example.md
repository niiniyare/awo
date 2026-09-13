> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Worked Example: Complete CRM Contact Module"
id: mdg-13
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Module Checklist](12-checklist.md)"
  - "[Getting Started](01-getting-started.md)"
  - "[Module System](../10-modules/module-system.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Worked Example: Complete CRM Contact Module

**MDG-13 | Module Developer Guide**

This document shows the complete, final state of all files for the `crm` module built through MDG-01 to MDG-12. Use this as a reference for the complete picture when the individual steps feel disjointed.

---

## File Tree

```
internal/core/crm/
    crm.go
    manifest.go
    def.go
    hooks.go
    policy.go
    workflows/
        workflows.go
        activities.go
        register.go
    migrations/
        20241215143022_crm_contact_indexes.up.sql
        20241215143022_crm_contact_indexes.down.sql
    crm_test.go
```

---

## crm.go — Registration Entry Point

```go
package crm

import "awo.so/awo/def"

func init() {
    def.RegisterManifest(&Manifest)
    def.Register(&ContactDefinition)
    def.Register(&InteractionDefinition)
}

func RegisterActivities(w worker.Worker, deps Dependencies) {
    workflows.Register(w, deps)
}
```

---

## manifest.go — Module Manifest

```go
package crm

import "awo.so/awo/def"

var Manifest = def.ModuleManifest{
    Name:     "crm",
    Label:    "CRM",
    Version:  "1.0.0",
    Provides: []string{"crm.contacts", "crm.interactions"},
    Requires: []string{"platform.tenancy", "platform.iam"},
    Owner:    "crm-team",
}
```

---

## def.go — Entity Definitions

```go
package crm

import "awo.so/awo/def"

var ContactDefinition = def.EntityDefinition{
    Name:         "crm_contact",
    Module:       "crm",
    Label:        "Contact",
    LabelPlural:  "Contacts",
    StorageModel: def.StorageCustom,

    Fields: []def.FieldDef{
        {Name: "full_name",          Type: def.FieldData,        Label: "Full Name",        Required: true,  MaxLen: 128, Searchable: true},
        {Name: "email",              Type: def.FieldData,        Label: "Email",             Required: true,  MaxLen: 256, Unique: true, Searchable: true, Validators: []def.FieldValidator{&EmailValidator{}}},
        {Name: "phone",              Type: def.FieldData,        Label: "Phone",             MaxLen: 32},
        {Name: "status",             Type: def.FieldSelect,      Label: "Status",            Options: []string{"Lead","Prospect","Active","Inactive","Lost"}, Default: "Lead"},
        {Name: "assigned_to",        Type: def.FieldLink,        Label: "Assigned To",       LinkTarget: "user", Required: true},
        {Name: "notes",              Type: def.FieldLongText,    Label: "Notes"},
        {Name: "source",             Type: def.FieldSelect,      Label: "Source",            Options: []string{"Referral","Website","Event","Cold Outreach","Social Media","Other"}, Default: "Other"},
        {Name: "tags",               Type: def.FieldMultiSelect, Label: "Tags",              Options: []string{"VIP","Decision Maker","Technical","Finance","Operations"}},
        {Name: "first_contact_date", Type: def.FieldDate,        Label: "First Contact Date"},
    },

    Edges: []def.EdgeDef{
        {Name: "interactions", Target: "crm_interaction", Type: def.EdgeOneToMany, CascadeDelete: true},
    },

    Hooks: def.HookSet{
        BeforeCreate: []def.BeforeCreateHook{&ContactEmailUniqueGuard{}},
        AfterCreate:  []def.AfterCreateHook{&ContactFirstContactDateSetter{}},
    },

    Permissions: def.PermissionSet{
        Create: []string{"role:crm.sales_rep", "role:crm.manager", "role:tenant.admin"},
        Read:   []string{"role:crm.sales_rep", "role:crm.manager", "role:crm.viewer", "role:tenant.admin"},
        Write:  []string{"role:crm.sales_rep", "role:crm.manager", "role:tenant.admin"},
        Delete: []string{"role:crm.manager", "role:tenant.admin"},
    },

    Policy: def.PolicyFunc(ContactOwnerPolicy),

    Actions: []def.ActionDef{
        {Name: "qualify",  Method: def.ActionMethodPost, Label: "Qualify Contact", Permission: "role:crm.sales_rep", HandlerFunc: QualifyContactAction, ConfirmationRequired: true},
        {Name: "reassign", Method: def.ActionMethodPost, Label: "Reassign",         Permission: "role:crm.manager",   HandlerFunc: ReassignContactAction},
    },

    WorkflowTriggers: []def.WorkflowTrigger{
        {
            On:         def.EventOnCreate,
            WorkflowFn: "ContactWelcomeWorkflow",
            TaskQueue:  "crm.contact.create",
            InputBuilder: func(rec *def.EntityRecord, tc def.TriggerContext) (any, error) {
                return workflows.ContactWelcomeInput{
                    TenantID:  rec.TenantID,
                    ContactID: rec.ID,
                    Email:     rec.Fields["email"].(string),
                    FullName:  rec.Fields["full_name"].(string),
                }, nil
            },
        },
    },

    PageBuilders: def.PageBuilderSet{
        Detail: BuildContactDetailPage,
    },
}

var InteractionDefinition = def.EntityDefinition{
    Name:         "crm_interaction",
    Module:       "crm",
    Label:        "Interaction",
    LabelPlural:  "Interactions",
    StorageModel: def.StorageCustom,
    Fields: []def.FieldDef{
        {Name: "contact",       Type: def.FieldLink,     Label: "Contact",       LinkTarget: "crm_contact", Required: true, Immutable: true},
        {Name: "type",          Type: def.FieldSelect,   Label: "Type",          Options: []string{"Call","Email","Meeting","Note"}, Required: true},
        {Name: "date",          Type: def.FieldDateTime, Label: "Date",          Required: true},
        {Name: "summary",       Type: def.FieldSmallText,Label: "Summary",       Required: true},
        {Name: "notes",         Type: def.FieldLongText, Label: "Notes"},
        {Name: "conducted_by",  Type: def.FieldLink,     Label: "Conducted By",  LinkTarget: "user", Required: true},
        {Name: "outcome",       Type: def.FieldSelect,   Label: "Outcome",       Options: []string{"Positive","Neutral","Negative","Follow-up Required"}},
    },
}
```

---

## API Surface Generated

```
GET    /api/v1/entities/crm_contact
GET    /api/v1/entities/crm_contact/{id}
POST   /api/v1/entities/crm_contact
PATCH  /api/v1/entities/crm_contact/{id}
DELETE /api/v1/entities/crm_contact/{id}
POST   /api/v1/entities/crm_contact/{id}/qualify
POST   /api/v1/entities/crm_contact/{id}/reassign

GET    /api/v1/entities/crm_interaction
GET    /api/v1/entities/crm_interaction/{id}
POST   /api/v1/entities/crm_interaction
PATCH  /api/v1/entities/crm_interaction/{id}
DELETE /api/v1/entities/crm_interaction/{id}

GET    /api/v1/schemas/crm_contact/list
GET    /api/v1/schemas/crm_contact/create
GET    /api/v1/schemas/crm_contact/edit
GET    /api/v1/schemas/crm_contact/detail   ← custom PageBuilderFunc
```

---

## Temporal Workflow Started On

```
POST /api/v1/entities/crm_contact
→ contact record committed
→ outbox entry committed (same TX)
→ relay dispatches: ContactWelcomeWorkflow
   workflow ID: {tenant}.crm_contact.{contact-id}.on_create.ContactWelcomeWorkflow
   → sends welcome email
```

---

## What Each File Covers

| File | What MDG step | What it does |
|---|---|---|
| `manifest.go` | MDG-01 | Module identity and dependencies |
| `crm.go` | MDG-01 | init() registration |
| `def.go` | MDG-02 to MDG-09 | All entity definitions |
| `hooks.go` | MDG-05 | Lifecycle hook implementations |
| `policy.go` | MDG-06 | Row-level filter functions |
| `handler.go` | MDG-07, MDG-09 | Action handlers + page builders |
| `workflows/workflows.go` | MDG-08 | Workflow functions |
| `workflows/activities.go` | MDG-08 | Activity struct + methods |
| `workflows/register.go` | MDG-08 | Activity registration |
| `migrations/*.sql` | MDG-10 | Index SQL |
| `crm_test.go` | MDG-11 | Hook, policy, activity tests |

---

## Checklist Status

All items in [Module Checklist](12-checklist.md) are satisfied by this module. Reference the checklist before submitting the module for review.
