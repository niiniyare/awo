> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Hook Test Patterns

**Classification:** Reference — Tier 2
**Owner:** `16-testing/HOOK_TEST_PATTERNS.md`
**Status:** Living document

---

## Purpose

Canonical patterns for unit testing hook implementations using mock `EntityRepository`.

---

## 1. Mock EntityRepository

Hooks that need repository access (e.g., uniqueness checks, cross-entity validation) receive a mock that satisfies `def.ActionEntityRepo`:

```go
// mockRepo satisfies def.ActionEntityRepo for testing.
type mockRepo struct {
    getFunc    func(ctx context.Context, id uuid.UUID) (*def.EntityRecord, error)
    existsFunc func(ctx context.Context, f def.ActionFilter) (bool, error)
    createFunc func(ctx context.Context, data map[string]any) (*def.EntityRecord, error)
    updateFunc func(ctx context.Context, id uuid.UUID, patch map[string]any) (*def.EntityRecord, error)
}

func (m *mockRepo) EntityName() string { return "mock" }
func (m *mockRepo) Get(ctx context.Context, id uuid.UUID) (*def.EntityRecord, error) {
    if m.getFunc != nil {
        return m.getFunc(ctx, id)
    }
    return nil, &def.NotFoundError{EntityType: "mock", ID: id}
}
func (m *mockRepo) Exists(ctx context.Context, f def.ActionFilter) (bool, error) {
    if m.existsFunc != nil {
        return m.existsFunc(ctx, f)
    }
    return false, nil
}
func (m *mockRepo) Query(_ context.Context, _ def.ActionFilter, _ ...def.ActionQueryOpt) ([]*def.EntityRecord, error) {
    return nil, nil
}
func (m *mockRepo) Count(_ context.Context, _ def.ActionFilter) (int64, error) { return 0, nil }
func (m *mockRepo) Create(ctx context.Context, data map[string]any) (*def.EntityRecord, error) {
    if m.createFunc != nil {
        return m.createFunc(ctx, data)
    }
    rec := def.NewEntityRecord("mock")
    for k, v := range data {
        rec.Set(k, v)
    }
    return rec, nil
}
func (m *mockRepo) Update(ctx context.Context, id uuid.UUID, patch map[string]any) (*def.EntityRecord, error) {
    if m.updateFunc != nil {
        return m.updateFunc(ctx, id, patch)
    }
    return def.NewEntityRecord("mock"), nil
}
func (m *mockRepo) Delete(_ context.Context, _ uuid.UUID) error { return nil }
```

---

## 2. BeforeCreate Hook Test Pattern

```go
func TestInvoiceNumberValidator_BeforeCreate(t *testing.T) {
    tests := []struct {
        name        string
        record      *def.EntityRecord
        existsResult bool
        wantErr     bool
    }{
        {
            name:        "unique number — passes",
            record:      invoiceRecordWith("number", "INV-2026-00001"),
            existsResult: false,
            wantErr:     false,
        },
        {
            name:        "duplicate number — fails",
            record:      invoiceRecordWith("number", "INV-2026-00001"),
            existsResult: true,
            wantErr:     true,
        },
        {
            name:        "empty number — fails",
            record:      invoiceRecordWith("number", ""),
            existsResult: false,
            wantErr:     true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            repo := &mockRepo{
                existsFunc: func(_ context.Context, _ def.ActionFilter) (bool, error) {
                    return tt.existsResult, nil
                },
            }
            hook := &InvoiceNumberValidator{repo: repo}
            err := hook.BeforeCreate(context.Background(), tt.record)
            if (err != nil) != tt.wantErr {
                t.Errorf("BeforeCreate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

---

## 3. AfterCreate Hook Test Pattern

```go
func TestInvoiceCreatedNotifier_AfterCreate(t *testing.T) {
    var notifiedUserIDs []uuid.UUID
    mockNotify := func(userIDs []uuid.UUID) {
        notifiedUserIDs = userIDs
    }

    rec := invoiceRecordWith("customer_id", uuid.MustParse("aa000000-0000-0000-0000-000000000001"))
    hook := &InvoiceCreatedNotifier{notify: mockNotify}

    err := hook.AfterCreate(context.Background(), rec)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(notifiedUserIDs) == 0 {
        t.Error("expected notification to be sent, got none")
    }
}
```

---

## 4. BeforeUpdate Hook Test Pattern

```go
func TestInvoiceImmutableValidator_BeforeUpdate(t *testing.T) {
    tests := []struct {
        name    string
        before  *def.EntityRecord
        patch   map[string]any
        wantErr bool
    }{
        {
            name:    "status change allowed on Draft",
            before:  invoiceRecordWith("status", "Draft"),
            patch:   map[string]any{"status": "Submitted"},
            wantErr: false,
        },
        {
            name:    "number change rejected — immutable",
            before:  invoiceRecordWith("number", "INV-2026-00001"),
            patch:   map[string]any{"number": "INV-2026-99999"},
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            hook := &InvoiceImmutableValidator{}
            err := hook.BeforeUpdate(context.Background(), tt.before, tt.patch)
            if (err != nil) != tt.wantErr {
                t.Errorf("BeforeUpdate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

---

## 5. Test Record Helpers

```go
func invoiceRecordWith(field string, value any) *def.EntityRecord {
    rec := def.NewEntityRecord("finance_invoice")
    rec.ID = uuid.New()
    rec.TenantID = uuid.MustParse("11000000-0000-0000-0000-000000000001")
    rec.Set("status", "Draft")
    rec.Set("total", "0.0000")
    rec.Set(field, value)
    return rec
}

func validInvoiceRecord() *def.EntityRecord {
    rec := def.NewEntityRecord("finance_invoice")
    rec.ID = uuid.New()
    rec.TenantID = uuid.MustParse("11000000-0000-0000-0000-000000000001")
    rec.Set("customer_id", uuid.New().String())
    rec.Set("status", "Draft")
    rec.Set("total", "1000.0000")
    rec.Set("number", "INV-2026-00001")
    return rec
}
```

---

## References

- [`16-testing/TEST_STRATEGY.md`](TEST_STRATEGY.md) — When to use unit vs integration tests
- [`02-pipeline/HOOK_CONTRACT.md`](../02-pipeline/HOOK_CONTRACT.md) — Hook interfaces
