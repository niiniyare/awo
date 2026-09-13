> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Define an Entity"
id: mdg-02
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Getting Started](01-getting-started.md)"
  - "[Add Fields](03-add-fields.md)"
  - "[EntityDefinition](../03-kernel/entity-def.md)"
  - "[System Entities](../05-persistence/system-entities.md)"
  - "[Custom Entities](../05-persistence/custom-entities.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Define an Entity

**MDG-02 | Module Developer Guide**

This document covers the initial `EntityDefinition` declaration, choosing between system and custom storage, and registering the entity with the framework.

---

## 1. Choose the Storage Model

Before writing a single line, answer: **system entity or custom entity?**

| Question | System Entity | Custom Entity |
|---|---|---|
| Financial integrity required? | **Yes** → system | No |
| Referenced by FK from other system entities? | **Yes** → system | No |
| Write rate > hundreds per second? | **Yes** → system | No |
| Fields need SQL constraints (CHECK, UNIQUE)? | **Yes** → system | Rarely |
| Schema changes expected at runtime? | No | **Yes** → custom |
| Fields evolve frequently per tenant? | No | **Yes** → custom |

For `crm_contact`: contacts are NOT financial, NOT referenced by FKs from system entities, and the schema may evolve (tenants add custom fields). **Custom entity** is correct.

If in doubt: start with custom entity. You can escalate to system entity later via a migration — the reverse (system → custom) is destructive.

---

## 2. Declare the EntityDefinition

```go
// internal/core/crm/def.go
package crm

import "awo.so/awo/def"

var ContactDefinition = def.EntityDefinition{
    // Stable identifier — used in URLs, Redis keys, Temporal IDs, Casbin policies
    // Never rename after the first migration (LAW-011)
    Name:   "crm_contact",
    Module: "crm",

    // Human-readable labels for SDUI
    Label:       "Contact",
    LabelPlural: "Contacts",

    // Custom entity: data stored in JSONB
    StorageModel: def.StorageCustom,

    // Fields, Edges, Hooks, Permissions, Actions, WorkflowTriggers, PageBuilders
    // — added in subsequent steps
}

var InteractionDefinition = def.EntityDefinition{
    Name:         "crm_interaction",
    Module:       "crm",
    Label:        "Interaction",
    LabelPlural:  "Interactions",
    StorageModel: def.StorageCustom,
}
```

---

## 3. Naming Rules

Entity names follow `{module}_{noun}` format:
- `crm_contact` — correct
- `crm_contacts` — wrong (plural)
- `contact` — wrong (no module prefix)
- `CrmContact` — wrong (not snake_case)

**Never rename entity names** after the first migration that creates their table. The name appears in:
- PostgreSQL table names
- Redis cache keys (`page:crm_contact:*`)
- Temporal workflow IDs (`{tenant}.crm_contact.{id}.on_create.ContactWorkflow`)
- Casbin policy objects
- API URLs (`/api/v1/entities/crm_contact`)

Renaming requires coordinated changes across all of these — treat the entity name as an immutable identifier.

---

## 4. What the Framework Generates

After `def.Register(&ContactDefinition)` and `Registry.Compile()`:

| Subsystem | What Is Generated |
|---|---|
| PostgreSQL | Row in `custom_entity_records` (custom entity table) |
| API | `GET/POST /api/v1/entities/crm_contact`, `GET/PATCH/DELETE /api/v1/entities/crm_contact/{id}` |
| SDUI | List, create, edit, detail schemas cached in Redis |
| Casbin | Policy object `"crm_contact"` ready for permission declarations |
| Temporal | WorkflowTrigger dispatch points (when triggers are declared) |

At this point the entity has no fields, no permissions, and all routes return 403 (no one has access). Add fields and permissions in the next steps.

---

## 5. System Entity Alternative

If you determined the entity should be a system entity:

```go
var ContactDefinition = def.EntityDefinition{
    Name:         "crm_contact",
    Module:       "crm",
    Label:        "Contact",
    LabelPlural:  "Contacts",
    StorageModel: def.StorageSystem,  // SQL columns, not JSONB
    // Requires a migration: CREATE TABLE crm_contact (...)
    // See MDG-10 for migration details
}
```

System entities require a migration file before the server will start — the framework validates that the table exists at startup.

---

## Next: [Add Fields →](03-add-fields.md)
