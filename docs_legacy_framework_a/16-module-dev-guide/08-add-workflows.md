> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Add Workflows"
id: mdg-08
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Add Actions](07-add-actions.md)"
  - "[Add SDUI](09-add-sdui.md)"
  - "[Temporal Integration](../09-workflow/temporal-integration.md)"
  - "[Activities](../09-workflow/activities.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Add Workflows

**MDG-08 | Module Developer Guide**

This document adds a `WorkflowTrigger` and activity to `crm_contact` — a welcome email workflow triggered automatically when a contact is created.

---

## 1. Declare the WorkflowTrigger

```go
// internal/core/crm/def.go
var ContactDefinition = def.EntityDefinition{
    // ... all previous fields ...

    WorkflowTriggers: []def.WorkflowTrigger{
        {
            // Trigger on record creation
            On: def.EventOnCreate,

            // Name of the registered workflow function
            WorkflowFn: "ContactWelcomeWorkflow",

            // Temporal task queue
            TaskQueue: "crm.contact.create",

            // Builds the workflow input from the created record
            InputBuilder: func(rec *def.EntityRecord, tc def.TriggerContext) (any, error) {
                email, _ := rec.Fields["email"].(string)
                name, _  := rec.Fields["full_name"].(string)
                if email == "" {
                    return nil, fmt.Errorf("InputBuilder: email field missing")
                }
                return ContactWelcomeInput{
                    TenantID:  rec.TenantID,
                    ContactID: rec.ID,
                    Email:     email,
                    FullName:  name,
                }, nil
            },
        },
    },
}
```

---

## 2. Declare the Workflow Input

```go
// internal/core/crm/workflows/workflows.go
package workflows

type ContactWelcomeInput struct {
    TenantID  uuid.UUID `json:"tenant_id"`
    ContactID uuid.UUID `json:"contact_id"`
    Email     string    `json:"email"`
    FullName  string    `json:"full_name"`
}
```

---

## 3. Implement the Workflow Function

```go
// internal/core/crm/workflows/workflows.go (continued)

// ContactWelcomeWorkflow sends a welcome email after contact creation.
// Stability: STABLE
func ContactWelcomeWorkflow(ctx workflow.Context, input ContactWelcomeInput) error {
    logger := workflow.GetLogger(ctx)
    logger.Info("ContactWelcomeWorkflow started", "contact_id", input.ContactID)

    ao := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
        StartToCloseTimeout: 30 * time.Second,
        RetryPolicy: &temporal.RetryPolicy{
            MaximumAttempts:        3,
            InitialInterval:        time.Second,
            BackoffCoefficient:     2.0,
            NonRetryableErrorTypes: []string{"crm.contact_not_found"},
        },
    })

    var acts *CRMActivities  // nil — Temporal uses the registered struct

    err := workflow.ExecuteActivity(ao, acts.SendWelcomeEmailActivity, SendWelcomeEmailInput{
        TenantID:  input.TenantID,
        ContactID: input.ContactID,
        Email:     input.Email,
        FullName:  input.FullName,
    }).Get(ao, nil)
    if err != nil {
        logger.Error("SendWelcomeEmailActivity failed", "error", err)
        return fmt.Errorf("ContactWelcomeWorkflow: send email: %w", err)
    }

    return nil
}
```

**Determinism reminder:** Workflow code MUST NOT:
- Call `time.Now()` → use `workflow.Now(ctx)`
- Call `time.Sleep()` → use `workflow.Sleep(ctx, d)`
- Make HTTP/DB calls → use activity functions

---

## 4. Implement the Activity

```go
// internal/core/crm/workflows/activities.go
package workflows

type CRMActivities struct {
    EmailClient notifications.EmailClient
    Repo        entity.EntityRepository
}

type SendWelcomeEmailInput struct {
    TenantID  uuid.UUID `json:"tenant_id"`
    ContactID uuid.UUID `json:"contact_id"`
    Email     string    `json:"email"`
    FullName  string    `json:"full_name"`
}

// SendWelcomeEmailActivity sends a welcome email to a newly created contact.
// Idempotent: checks audit log before sending to prevent duplicate emails on retry.
func (a *CRMActivities) SendWelcomeEmailActivity(ctx context.Context, input SendWelcomeEmailInput) error {
    // Idempotency check: was this email already sent for this contact?
    alreadySent, err := a.Repo.Exists(ctx, filter.And(
        filter.Eq("entity_type", "crm_contact"),
        filter.Eq("entity_id",   input.ContactID.String()),
        filter.Eq("event",       "welcome_email_sent"),
    ))
    if err != nil {
        return fmt.Errorf("SendWelcomeEmailActivity: check sent: %w", err)
    }
    if alreadySent {
        return nil  // Already sent — idempotent return
    }

    // Send the email
    err = a.EmailClient.Send(ctx, notifications.Email{
        To:      input.Email,
        Subject: "Welcome to our CRM",
        Template: "crm_welcome",
        Data: map[string]any{
            "name": input.FullName,
        },
    })
    if err != nil {
        return fmt.Errorf("SendWelcomeEmailActivity: send: %w", err)
    }

    // Record that the email was sent (for idempotency on retry)
    // TODO: record in iam_audit_log or a dedicated email_log entity
    return nil
}
```

---

## 5. Register the Workflow and Activities

```go
// internal/core/crm/workflows/register.go
package workflows

type Dependencies struct {
    EmailClient notifications.EmailClient
    Repo        entity.EntityRepository
}

func Register(w worker.Worker, deps Dependencies) {
    // Register workflow function
    w.RegisterWorkflow(ContactWelcomeWorkflow)
    w.RegisterWorkflow(ProvisionCRMModuleWorkflow)

    // Register activities struct
    acts := &CRMActivities{
        EmailClient: deps.EmailClient,
        Repo:        deps.Repo,
    }
    w.RegisterActivity(acts)
}
```

```go
// internal/core/crm/crm.go — expose the register function
package crm

func RegisterActivities(w worker.Worker, deps Dependencies) {
    workflows.Register(w, deps)
}
```

---

## 6. Workflow ID Generated

The framework generates the canonical workflow ID for the `ContactWelcomeWorkflow` trigger:

```
{tenant-uuid}.crm_contact.{contact-uuid}.on_create.ContactWelcomeWorkflow
```

This ID is stored on the entity record before the workflow is dispatched. If the workflow fails to start (Temporal unavailable), the outbox relay retries until success.

---

## 7. Testing the Workflow

Use Temporal's `testsuite.WorkflowTestSuite` to test workflow logic without a running Temporal server:

```go
func TestContactWelcomeWorkflow_Success(t *testing.T) {
    suite := &testsuite.WorkflowTestSuite{}
    env := suite.NewTestWorkflowEnvironment()

    // Mock the activity
    env.OnActivity((*CRMActivities).SendWelcomeEmailActivity, mock.Anything,
        SendWelcomeEmailInput{
            Email:    "test@example.com",
            FullName: "Test Contact",
        },
    ).Return(nil)

    env.ExecuteWorkflow(ContactWelcomeWorkflow, ContactWelcomeInput{
        TenantID:  uuid.New(),
        ContactID: uuid.New(),
        Email:     "test@example.com",
        FullName:  "Test Contact",
    })

    suite.NoError(env.GetWorkflowResult(nil))
}
```

---

## Next: [Add SDUI →](09-add-sdui.md)
