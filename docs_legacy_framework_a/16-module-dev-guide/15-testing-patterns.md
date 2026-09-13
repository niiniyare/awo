> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Testing Patterns"
id: mdg-015
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Module Testing](11-testing.md)"
  - "[Hooks](../04-domain/hooks.md)"
  - "[Temporal Integration](../09-workflow/temporal-integration.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Testing Patterns

**MDG-015 | Status: Accepted | Stability: Stable**

Additional testing patterns beyond the basics in MDG-011: integration test setup, hook chain testing, policy function testing, and workflow testing patterns.

---

## 1. Integration Test Database Setup

Integration tests use a real PostgreSQL instance — not mocks (per project feedback: mocks masked a broken migration in production).

```go
// internal/core/crm/integration_test.go

var testDB *pgxpool.Pool

func TestMain(m *testing.M) {
    // Requires TEST_DATABASE_URL environment variable
    dbURL := os.Getenv("TEST_DATABASE_URL")
    if dbURL == "" {
        fmt.Println("TEST_DATABASE_URL not set — skipping integration tests")
        os.Exit(0)
    }

    var err error
    testDB, err = pgxpool.New(context.Background(), dbURL)
    if err != nil {
        log.Fatal("failed to connect to test DB", err)
    }
    defer testDB.Close()

    // Run migrations on test DB
    if err := runMigrations(dbURL); err != nil {
        log.Fatal("migration failed", err)
    }

    os.Exit(m.Run())
}

func runMigrations(dbURL string) error {
    m, err := migrate.New("file://../../db/migration", dbURL)
    if err != nil { return err }
    err = m.Up()
    if err != nil && !errors.Is(err, migrate.ErrNoChange) {
        return err
    }
    return nil
}
```

---

## 2. Test Tenant Setup

Each test needs an isolated tenant to avoid data leakage between tests:

```go
func setupTestTenant(t *testing.T, ctx context.Context) (context.Context, uuid.UUID) {
    t.Helper()

    tenantID := uuid.New()
    _, err := testDB.Exec(ctx,
        `INSERT INTO tenants (id, name, status) VALUES ($1, $2, 'ACTIVE')`,
        tenantID, "test-tenant-"+tenantID.String()[:8],
    )
    require.NoError(t, err)

    // Set tenant context for this test
    _, err = testDB.Exec(ctx, `SELECT set_tenant_context($1)`, tenantID)
    require.NoError(t, err)

    // Cleanup after test
    t.Cleanup(func() {
        // Delete tenant and cascade to all tenant data
        testDB.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, tenantID)
    })

    return ctx, tenantID
}
```

---

## 3. Hook Chain Testing

Test the full hook chain, not individual hooks in isolation, to verify ordering and interaction:

```go
func TestInvoiceCreationHookChain(t *testing.T) {
    ctx, tenantID := setupTestTenant(t, context.Background())
    repo := buildTestRepo(testDB, tenantID)

    // Create with a valid customer
    customerID := createTestCustomer(t, ctx, repo)

    invoice, err := repo.Create(ctx, entity.CreateInput{
        Fields: map[string]any{
            "customer":  customerID,
            "total_kes": decimal.NewFromFloat(50000),
            "status":    "Draft",
        },
    })
    require.NoError(t, err)

    // Verify NamingSeries was assigned by AfterCreate hook
    assert.NotEmpty(t, invoice.Fields["number"])
    assert.Regexp(t, `^INV-\d{4}-\d{5}$`, invoice.Fields["number"])

    // Verify audit log entry was created
    var count int
    testDB.QueryRow(ctx,
        `SELECT COUNT(*) FROM iam_audit_log WHERE entity_type = 'finance_invoice' AND entity_id = $1`,
        invoice.ID,
    ).Scan(&count)
    assert.Equal(t, 1, count)
}
```

---

## 4. Policy Function Testing

```go
func TestContactOwnerPolicy(t *testing.T) {
    ctx, tenantID := setupTestTenant(t, context.Background())

    // Create two users
    userA := createTestUser(t, ctx, tenantID, "alice@example.com")
    userB := createTestUser(t, ctx, tenantID, "bob@example.com")

    // Create contacts assigned to userA
    ctxA := session.WithActor(ctx, session.Actor{UserID: userA.ID, TenantID: tenantID})
    contact, _ := contactRepo.Create(ctxA, entity.CreateInput{
        Fields: map[string]any{"name": "Test Contact", "assigned_to": userA.ID},
    })

    // UserA should see the contact
    results, _, err := contactRepo.Query(ctxA, filter.All())
    require.NoError(t, err)
    assert.Len(t, results, 1)
    assert.Equal(t, contact.ID, results[0].ID)

    // UserB should NOT see the contact (OwnerOnly policy)
    ctxB := session.WithActor(ctx, session.Actor{UserID: userB.ID, TenantID: tenantID})
    results, _, err = contactRepo.Query(ctxB, filter.All())
    require.NoError(t, err)
    assert.Len(t, results, 0)  // policy filters it out
}
```

---

## 5. Workflow Testing

Use Temporal's test suite — do not call Temporal production server in unit tests:

```go
func TestInvoiceApprovalWorkflow(t *testing.T) {
    suite.Run(t, new(InvoiceApprovalWorkflowTestSuite))
}

type InvoiceApprovalWorkflowTestSuite struct {
    suite.Suite
    testsuite.WorkflowTestSuite
    env *testsuite.TestWorkflowEnvironment
}

func (s *InvoiceApprovalWorkflowTestSuite) SetupTest() {
    s.env = s.NewTestWorkflowEnvironment()
    s.env.RegisterWorkflow(InvoiceApprovalWorkflow)
    s.env.RegisterActivity(&InvoiceActivities{})
}

func (s *InvoiceApprovalWorkflowTestSuite) TestApprovalGranted() {
    s.env.OnActivity((*InvoiceActivities).NotifyApproverActivity, mock.Anything, mock.Anything).
        Return(nil)
    s.env.OnActivity((*InvoiceActivities).ApproveInvoiceActivity, mock.Anything, mock.Anything).
        Return(nil)

    // Send approve signal after workflow starts
    s.env.RegisterDelayedCallback(func() {
        s.env.SignalWorkflow(SignalApprove, ApprovalDecision{
            ApproverID: uuid.New(),
            Comment:    "Looks good.",
        })
    }, time.Second)

    s.env.ExecuteWorkflow(InvoiceApprovalWorkflow, ApprovalInput{
        TenantID:  uuid.New(),
        InvoiceID: uuid.New(),
        Amount:    decimal.NewFromFloat(50000),
    })

    s.True(s.env.IsWorkflowCompleted())
    s.NoError(s.env.GetWorkflowError())
}

func (s *InvoiceApprovalWorkflowTestSuite) TestApprovalTimeout() {
    s.env.OnActivity((*InvoiceActivities).NotifyApproverActivity, mock.Anything, mock.Anything).
        Return(nil)
    s.env.OnActivity((*InvoiceActivities).RejectInvoiceActivity, mock.Anything, mock.Anything).
        Return(nil)

    // No signal sent — workflow should timeout and auto-reject
    // Temporal test env advances time automatically
    s.env.ExecuteWorkflow(InvoiceApprovalWorkflow, ApprovalInput{
        TenantID:  uuid.New(),
        InvoiceID: uuid.New(),
        Amount:    decimal.NewFromFloat(50000),
    })

    s.True(s.env.IsWorkflowCompleted())
    s.NoError(s.env.GetWorkflowError())

    // Verify RejectInvoiceActivity was called (not ApproveInvoiceActivity)
    s.env.AssertExpectations(s.T())
}
```

---

## 6. Table-Driven Validator Tests

```go
func TestEmailValidator(t *testing.T) {
    v := def.ValidateEmail

    tests := []struct {
        name    string
        input   any
        wantErr bool
        errMsg  string
    }{
        {"valid email", "alice@example.com", false, ""},
        {"valid email with plus", "alice+test@example.com", false, ""},
        {"missing @", "notanemail", true, "Invalid email"},
        {"missing domain", "alice@", true, "Invalid email"},
        {"empty string", "", false, ""},  // Required constraint handles empty
        {"nil input", nil, false, ""},    // nil: not an email — no validation
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := v.Validate(tt.input)
            if tt.wantErr {
                require.Error(t, err)
                var ve *errors.ValidationError
                require.ErrorAs(t, err, &ve)
                assert.Contains(t, ve.Fields["email"], tt.errMsg)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

---

## 7. HTTP Handler Testing

Use Fiber's test utility — do not spin up a real server:

```go
func TestCreateInvoiceHandler(t *testing.T) {
    app := setupTestApp(t)

    body := `{"customer": "018e1b2c-...", "total_kes": "50000.0000", "status": "Draft"}`
    req := httptest.NewRequest("POST", "/api/v1/entities/finance_invoice",
        strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+testSessionToken)
    req.Header.Set("X-Tenant-ID", testTenantID.String())

    resp, err := app.Test(req)
    require.NoError(t, err)
    assert.Equal(t, 201, resp.StatusCode)

    var result SuccessEnvelope
    json.NewDecoder(resp.Body).Decode(&result)
    assert.NotEmpty(t, result.Data["id"])
}

func TestCreateInvoiceHandler_MissingCustomer(t *testing.T) {
    app := setupTestApp(t)

    body := `{"total_kes": "50000.0000"}`  // missing required customer
    req := httptest.NewRequest("POST", "/api/v1/entities/finance_invoice",
        strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+testSessionToken)
    req.Header.Set("X-Tenant-ID", testTenantID.String())

    resp, err := app.Test(req)
    require.NoError(t, err)
    assert.Equal(t, 422, resp.StatusCode)

    var errResp ErrorEnvelope
    json.NewDecoder(resp.Body).Decode(&errResp)
    assert.Equal(t, "validation_error", errResp.Error.Code)
    assert.Contains(t, errResp.Error.Fields, "customer")
}
```

---

## Related Documents

- [Module Testing](11-testing.md) — basic test patterns (mock repos, workflow test suite)
- [Hooks](../04-domain/hooks.md) — hook execution order
- [Policy Functions](../04-domain/policies.md) — PolicyFunc specification
- [Architecture Laws](../02-architecture/laws.md) — LAW testing notes
