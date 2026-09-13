> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Add Actions"
id: mdg-07
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Add Policies](06-add-policies.md)"
  - "[Add Workflows](08-add-workflows.md)"
  - "[Actions](../04-domain/actions.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Add Actions

**MDG-07 | Module Developer Guide**

This document adds a `qualify` action to `crm_contact` — a custom operation beyond standard CRUD that promotes a contact from Lead to Active status.

---

## 1. Declare the Action

```go
// internal/core/crm/def.go
var ContactDefinition = def.EntityDefinition{
    // ... Name, Module, Label, StorageModel, Fields, Edges, Hooks, Permissions, Policy ...

    Actions: []def.ActionDef{
        {
            Name:                 "qualify",
            Method:               def.ActionMethodPost,
            Label:                "Qualify Contact",
            Permission:           "role:crm.sales_rep",
            HandlerFunc:          QualifyContactAction,
            ConfirmationRequired: true,  // SDUI shows a confirm dialog before sending
        },
        {
            Name:        "reassign",
            Method:      def.ActionMethodPost,
            Label:       "Reassign",
            Permission:  "role:crm.manager",
            HandlerFunc: ReassignContactAction,
        },
    },
}
```

The framework generates: `POST /api/v1/entities/crm_contact/{id}/qualify`

---

## 2. Implement the Action Handler

```go
// internal/core/crm/handler.go  (or hooks.go — wherever logical)
package crm

import (
    "context"
    "fmt"
    "awo.so/awo/def"
    "awo.so/awo/errors"
)

// QualifyContactAction promotes a Lead contact to Active status.
func QualifyContactAction(ctx context.Context, action def.ActionContext) (*def.ActionResult, error) {
    // action.Record contains the pre-loaded contact record
    contact := action.Record

    status, _ := contact.Fields["status"].(string)
    if status != "Lead" && status != "Prospect" {
        return nil, &errors.BusinessError{
            Code:    "crm.contact_not_qualifiable",
            Message: fmt.Sprintf("Contact cannot be qualified from status %q. Only Lead or Prospect contacts can be qualified.", status),
            Status:  409,
        }
    }

    // Update the contact status
    _, err := action.Repo.Update(ctx, action.RecordID, def.UpdateInput{
        Fields: map[string]any{
            "status": "Active",
        },
    })
    if err != nil {
        return nil, fmt.Errorf("QualifyContactAction: update status: %w", err)
    }

    // Optionally trigger a workflow (send qualification email, notify manager, etc.)
    // action.TriggerWorkflow("ContactQualifiedWorkflow", ContactQualifiedInput{...})

    return &def.ActionResult{
        Message: "Contact successfully qualified.",
        Data: map[string]any{
            "previous_status": status,
            "new_status":      "Active",
        },
    }, nil
}
```

---

## 3. Action Handler Rules

Action handlers MUST follow these constraints:

- Maximum ~50 lines per handler function
- No direct database calls — use `action.Repo` (scoped EntityRepository)
- Return `*ActionResult` on success; return a typed error on failure
- The handler receives a pre-loaded, permission-checked record — do not re-fetch unless stale data is a concern
- `action.Repo` is already scoped to the correct tenant and actor — do not re-apply tenant filters

---

## 4. Reassign Action (With Input)

Actions can accept a request body:

```go
type ReassignInput struct {
    NewAssignee string `json:"new_assignee_id"` // UUID of the new sales rep
    Reason      string `json:"reason"`
}

func ReassignContactAction(ctx context.Context, action def.ActionContext) (*def.ActionResult, error) {
    var input ReassignInput
    if err := action.BindInput(&input); err != nil {
        return nil, &errors.ValidationError{
            Fields: map[string]string{"new_assignee_id": "Invalid assignee ID."},
        }
    }

    newAssigneeID, err := uuid.Parse(input.NewAssignee)
    if err != nil {
        return nil, &errors.ValidationError{
            Fields: map[string]string{"new_assignee_id": "Must be a valid UUID."},
        }
    }

    _, err = action.Repo.Update(ctx, action.RecordID, def.UpdateInput{
        Fields: map[string]any{
            "assigned_to": newAssigneeID,
        },
    })
    if err != nil {
        return nil, fmt.Errorf("ReassignContactAction: %w", err)
    }

    return &def.ActionResult{Message: "Contact reassigned."}, nil
}
```

---

## 5. Action Response

Successful action response (HTTP 200 by default):

```json
{
  "data": {
    "previous_status": "Lead",
    "new_status": "Active"
  },
  "meta": {
    "request_id": "req-abc123",
    "message": "Contact successfully qualified."
  }
}
```

Workflow-triggering actions return HTTP 202:

```json
{
  "data": {"id": "018e1b2c-..."},
  "meta": {
    "request_id": "req-abc123",
    "workflow_id": "018d9f3a.crm_contact.018e1b2c.qualify.ContactQualifiedWorkflow",
    "accepted": true
  }
}
```

---

## 6. SDUI Integration

With `ConfirmationRequired: true`, the SDUI generates:
- An action button labeled "Qualify Contact" in the detail view toolbar
- A confirmation dialog: "Are you sure you want to qualify this contact?"
- The button is absent if the actor lacks `role:crm.sales_rep` (permission-gated, not disabled)

The button also appears in the list view's row actions if the entity's list schema includes row-level action buttons (default behavior for entities with actions).

---

## Next: [Add Workflows →](08-add-workflows.md)
