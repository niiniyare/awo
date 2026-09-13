> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Test Strategy

**Classification:** Guide — Tier 2
**Owner:** `16-testing/TEST_STRATEGY.md`
**Status:** Living document

---

## Purpose

This document specifies the testing strategy for Awo Framework code — the distinction between unit and integration tests, mandatory real-PostgreSQL policy for integration tests, test organisation, and the framework-specific testing APIs.

---

## 1. Test Layers

### Unit Tests

**Scope:** Functions and types with no external I/O — hook implementations, validators, PolicyFuncs, Filter builders, workflow determinism helpers.

**Dependencies:** Mock `EntityRepository` implementations. Mock `ActionRuntime`. In-process data only.

**Location:** Same package as the code under test, file named `{subject}_test.go`.

**When to write:** For every hook, validator, and policy function. For every action handler business rule.

### Integration Tests

**Scope:** Repository interactions, pipeline execution, RLS enforcement, migration effects.

**Dependencies:** Real PostgreSQL (see §3). Real Redis (for cache and session tests). No mocks at the database layer.

**Location:** `{module}/integration_test.go` or `{module}/integration/` subdirectory.

**When to write:** For every new entity's CRUD operations. For every migration. For every RLS policy.

### Workflow Tests

**Scope:** Temporal workflow and activity logic.

**Dependencies:** `testsuite.WorkflowTestSuite` (Temporal's in-process test runner). Mock activities.

**Location:** `workflows/{workflow_name}_test.go`.

**When to write:** For every workflow. Test all signal handlers and compensation paths.

---

## 2. Test File Location

```
internal/
  core/
    finance/
      hooks.go
      hooks_test.go          ← unit tests alongside source
      actions.go
      actions_test.go
      integration_test.go    ← integration tests (tag: //go:build integration)
```

Build tag for integration tests:

```go
//go:build integration

package finance_test
```

Run unit tests: `go test ./...`
Run integration tests: `go test -tags=integration ./...`

---

## 3. Real PostgreSQL Mandate

**Integration tests MUST use a real PostgreSQL database. Database mocking is prohibited.**

Rationale: In a prior incident, mocked repository tests passed while a production migration broke a unique constraint because the mock did not enforce constraint semantics. Real PostgreSQL catches:
- Constraint violations (unique, check, FK)
- RLS policy gaps
- Migration errors
- Query plan differences on real data volumes

### Test Database Setup

Each integration test suite uses `TestMain` to:
1. Run all pending migrations against a test database.
2. Seed baseline data (platform tenant, system roles).
3. Run tests.
4. Truncate tenant-scoped tables after each test (NOT drop — too slow).

```go
func TestMain(m *testing.M) {
    db := testdb.MustConnect()
    testdb.MustMigrate(db)
    testdb.MustSeed(db)
    os.Exit(m.Run())
}
```

`testdb` is the framework's test helper package at `awo.so/awo/testdb`.

---

## 4. Table-Driven Tests

Prefer table-driven tests for validators, hook logic, and field mapping:

```go
func TestInvoiceValidator_BeforeCreate(t *testing.T) {
    tests := []struct {
        name    string
        record  *def.EntityRecord
        wantErr bool
        errCode string
    }{
        {
            name:    "valid invoice",
            record:  validInvoiceRecord(),
            wantErr: false,
        },
        {
            name:    "missing customer",
            record:  recordWithout("customer_id"),
            wantErr: true,
            errCode: "invoice.customer_required",
        },
        {
            name:    "negative total",
            record:  recordWith("total", "-100.00"),
            wantErr: true,
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            hook := &InvoiceValidator{}
            err := hook.BeforeCreate(context.Background(), tt.record)
            if (err != nil) != tt.wantErr {
                t.Fatalf("got err=%v, wantErr=%v", err, tt.wantErr)
            }
            if tt.errCode != "" {
                var be *def.BusinessError
                if !errors.As(err, &be) || be.Code != tt.errCode {
                    t.Errorf("want code=%q, got %v", tt.errCode, err)
                }
            }
        })
    }
}
```

---

## 5. Never Run Tests Automatically

Per CLAUDE.md critical rules:
- **Never run `go test` or `go vet`** — provide test code, tell user to run them.
- **Never auto-migrate** in tests without explicit `testdb.MustMigrate()` call.

---

## References

- [`16-testing/HOOK_TEST_PATTERNS.md`](HOOK_TEST_PATTERNS.md) — Hook mock patterns
- [`16-testing/ACTION_TEST_PATTERNS.md`](ACTION_TEST_PATTERNS.md) — ActionRuntime mock patterns
- [`16-testing/REGISTRY_TEST_PATTERNS.md`](REGISTRY_TEST_PATTERNS.md) — Isolated registry patterns
