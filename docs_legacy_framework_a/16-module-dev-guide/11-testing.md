> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Testing"
id: mdg-11
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Add Migrations](10-add-migrations.md)"
  - "[Module Checklist](12-checklist.md)"
  - "[Hooks](../04-domain/hooks.md)"
  - "[Activities](../09-workflow/activities.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Testing

**MDG-11 | Module Developer Guide**

This document covers the testing strategy for the `crm` module: unit tests for hooks and policies, integration tests with real PostgreSQL, and workflow tests with Temporal's test suite.

---

## 1. Testing Philosophy

| Layer | Test Type | Dependencies |
|---|---|---|
| Hooks | Unit test | Mock `EntityRepository` |
| Policies | Unit test | No infrastructure (pure function) |
| Service logic | Unit test | Mock `EntityRepository` |
| Activities | Unit test | Mock external clients (email, HTTP) |
| Workflows | Unit test | Temporal `WorkflowTestSuite` with activity mocks |
| API routes | Integration | Real PostgreSQL + Redis (not mocked) |

**Never mock the database in integration tests.** Mock/production divergence has caused production incidents. Use real PostgreSQL in a test database for integration tests.

All test commands are run manually by the developer — never auto-executed by Claude Code.

---

## 2. Mock EntityRepository

```go
// internal/core/crm/crm_test.go
package crm_test

// mockContactRepo implements entity.EntityRepository for testing
type mockContactRepo struct {
    existsResult bool
    existsErr    error
    lastUpdate   *entity.UpdateInput
}

func (m *mockContactRepo) Exists(ctx context.Context, f entity.Filter) (bool, error) {
    return m.existsResult, m.existsErr
}

func (m *mockContactRepo) Update(ctx context.Context, id uuid.UUID, input entity.UpdateInput) (*entity.EntityRecord, error) {
    m.lastUpdate = &input
    return &entity.EntityRecord{ID: id, Fields: input.Fields}, nil
}

// ... implement other interface methods as no-ops or panics ...
func (m *mockContactRepo) Get(_ context.Context, _ uuid.UUID) (*entity.EntityRecord, error) {
    panic("not implemented in this mock")
}
// etc.
```

---

## 3. Hook Unit Tests

```go
// Test: email uniqueness guard
func TestContactEmailUniqueGuard_DuplicateEmail(t *testing.T) {
    repo := &mockContactRepo{existsResult: true}
    hook := &crm.ContactEmailUniqueGuard{Repo: repo}

    err := hook.BeforeCreate(context.Background(), &entity.EntityRecord{
        Fields: map[string]any{"email": "existing@example.com"},
    }, entity.HookContext{})

    var ve *errors.ValidationError
    if !errors.As(err, &ve) {
        t.Fatalf("want ValidationError, got %T: %v", err, err)
    }
    if _, ok := ve.Fields["email"]; !ok {
        t.Error("want error on 'email' field")
    }
}

func TestContactEmailUniqueGuard_UniqueEmail(t *testing.T) {
    repo := &mockContactRepo{existsResult: false}
    hook := &crm.ContactEmailUniqueGuard{Repo: repo}

    err := hook.BeforeCreate(context.Background(), &entity.EntityRecord{
        Fields: map[string]any{"email": "new@example.com"},
    }, entity.HookContext{})

    if err != nil {
        t.Fatalf("want nil error, got: %v", err)
    }
}

func TestContactEmailUniqueGuard_RepositoryError(t *testing.T) {
    dbErr := fmt.Errorf("connection refused")
    repo := &mockContactRepo{existsErr: dbErr}
    hook := &crm.ContactEmailUniqueGuard{Repo: repo}

    err := hook.BeforeCreate(context.Background(), &entity.EntityRecord{
        Fields: map[string]any{"email": "test@example.com"},
    }, entity.HookContext{})

    if err == nil {
        t.Fatal("want error when repo fails, got nil")
    }
    // Should not be a ValidationError — this is a system error
    var ve *errors.ValidationError
    if errors.As(err, &ve) {
        t.Error("repository error should not be wrapped as ValidationError")
    }
}
```

---

## 4. Policy Unit Tests

```go
func TestContactOwnerPolicy_SalesRep_SeesOwnContacts(t *testing.T) {
    repID := uuid.New()
    ctx := session.WithActor(context.Background(), session.Actor{
        UserID: repID,
        Roles:  []string{"role:crm.sales_rep"},
    })

    f := crm.ContactOwnerPolicy(ctx)

    expected := filter.Eq("assigned_to", repID)
    if !f.Equals(expected) {
        t.Errorf("want filter{assigned_to=%s}, got %v", repID, f)
    }
}

func TestContactOwnerPolicy_Manager_SeesAll(t *testing.T) {
    ctx := session.WithActor(context.Background(), session.Actor{
        UserID: uuid.New(),
        Roles:  []string{"role:crm.manager"},
    })

    f := crm.ContactOwnerPolicy(ctx)

    if !f.IsAll() {
        t.Errorf("manager should see all contacts, got filter: %v", f)
    }
}
```

---

## 5. Activity Unit Tests

```go
func TestSendWelcomeEmailActivity_Success(t *testing.T) {
    emailClient := &mockEmailClient{}
    repo := &mockContactRepo{existsResult: false}  // email not yet sent

    acts := &workflows.CRMActivities{
        EmailClient: emailClient,
        Repo:        repo,
    }

    err := acts.SendWelcomeEmailActivity(context.Background(), workflows.SendWelcomeEmailInput{
        TenantID:  uuid.New(),
        ContactID: uuid.New(),
        Email:     "welcome@example.com",
        FullName:  "Test User",
    })

    if err != nil {
        t.Fatalf("want nil error, got: %v", err)
    }
    if !emailClient.sent {
        t.Error("want email to be sent")
    }
}

func TestSendWelcomeEmailActivity_Idempotent_AlreadySent(t *testing.T) {
    emailClient := &mockEmailClient{}
    repo := &mockContactRepo{existsResult: true}  // email already sent

    acts := &workflows.CRMActivities{
        EmailClient: emailClient,
        Repo:        repo,
    }

    err := acts.SendWelcomeEmailActivity(context.Background(), workflows.SendWelcomeEmailInput{
        Email: "already@example.com",
    })

    if err != nil {
        t.Fatalf("want nil error on idempotent call, got: %v", err)
    }
    if emailClient.sent {
        t.Error("want email NOT to be sent when already sent")
    }
}
```

---

## 6. Workflow Tests with TestSuite

```go
func TestContactWelcomeWorkflow_Success(t *testing.T) {
    var suite testsuite.WorkflowTestSuite
    env := suite.NewTestWorkflowEnvironment()

    var acts *workflows.CRMActivities
    env.OnActivity(acts.SendWelcomeEmailActivity, mock.Anything,
        workflows.SendWelcomeEmailInput{
            Email:    "test@example.com",
            FullName: "Test Contact",
        },
    ).Return(nil)

    env.ExecuteWorkflow(workflows.ContactWelcomeWorkflow, workflows.ContactWelcomeInput{
        TenantID:  uuid.New(),
        ContactID: uuid.New(),
        Email:     "test@example.com",
        FullName:  "Test Contact",
    })

    suite.NoError(env.GetWorkflowResult(nil))
}

func TestContactWelcomeWorkflow_EmailFailure_Retries(t *testing.T) {
    var suite testsuite.WorkflowTestSuite
    env := suite.NewTestWorkflowEnvironment()

    callCount := 0
    var acts *workflows.CRMActivities
    env.OnActivity(acts.SendWelcomeEmailActivity, mock.Anything, mock.Anything).
        Return(func(_ context.Context, _ workflows.SendWelcomeEmailInput) error {
            callCount++
            if callCount < 3 {
                return fmt.Errorf("SMTP unavailable")  // retryable
            }
            return nil
        })

    env.ExecuteWorkflow(workflows.ContactWelcomeWorkflow, workflows.ContactWelcomeInput{
        Email: "test@example.com",
    })

    suite.NoError(env.GetWorkflowResult(nil))
    if callCount != 3 {
        t.Errorf("want 3 activity calls, got %d", callCount)
    }
}
```

---

## 7. Table-Driven Tests

For validators and hook logic with many cases:

```go
func TestEmailValidator_Validate(t *testing.T) {
    v := &crm.EmailValidator{}

    tests := []struct {
        name    string
        value   any
        wantErr bool
    }{
        {"valid email",      "user@example.com",  false},
        {"valid with plus",  "user+tag@example.com", false},
        {"no at sign",       "notanemail",         true},
        {"no dot",           "user@example",       true},
        {"empty string",     "",                   false},  // Required handles empty
        {"nil",              nil,                  false},  // Required handles nil
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := v.Validate(context.Background(), tt.value)
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate(%q) error = %v, wantErr = %v", tt.value, err, tt.wantErr)
            }
        })
    }
}
```

---

## Next: [Module Checklist →](12-checklist.md)
