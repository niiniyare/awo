> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Action Test Patterns

**Classification:** Reference — Tier 2
**Owner:** `16-testing/ACTION_TEST_PATTERNS.md`
**Status:** Living document

---

## Purpose

Canonical patterns for unit testing action handlers using a mock `ActionRuntime`.

---

## 1. Mock ActionRuntime

```go
// mockRuntime satisfies def.ActionRuntime for testing action handlers.
type mockRuntime struct {
    tenantID        uuid.UUID
    actor           *def.Actor
    publishedEvents []def.ActionEvent
    startedWorkflows []def.ActionWorkflowSpec
    repos           map[string]*mockRepo
    clock           time.Time
}

func newMockRuntime(tenantID uuid.UUID, actor *def.Actor) *mockRuntime {
    return &mockRuntime{
        tenantID: tenantID,
        actor:    actor,
        repos:    make(map[string]*mockRepo),
        clock:    time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC),
    }
}

func (m *mockRuntime) Repo(entityName string) def.ActionEntityRepo {
    if r, ok := m.repos[entityName]; ok {
        return r
    }
    return &mockRepo{}
}

func (m *mockRuntime) SetRepo(entityName string, repo *mockRepo) {
    m.repos[entityName] = repo
}

func (m *mockRuntime) Tx(ctx context.Context, fn func(context.Context) error) error {
    return fn(ctx)  // no real TX in tests — executes synchronously
}

func (m *mockRuntime) Publish(_ context.Context, event def.ActionEvent) error {
    m.publishedEvents = append(m.publishedEvents, event)
    return nil
}

func (m *mockRuntime) StartWorkflow(_ context.Context, spec def.ActionWorkflowSpec) (string, error) {
    m.startedWorkflows = append(m.startedWorkflows, spec)
    if spec.WorkflowID != "" {
        return spec.WorkflowID, nil
    }
    return "test-workflow-id", nil
}

func (m *mockRuntime) Notify(_ context.Context, _ def.ActionNotification) error { return nil }
func (m *mockRuntime) InvalidateCache(_ context.Context, _ string) error         { return nil }
func (m *mockRuntime) Cache() def.ActionCache                                     { return &noopCache{} }
func (m *mockRuntime) Clock() time.Time                                           { return m.clock }
func (m *mockRuntime) Logger() *slog.Logger                                       { return slog.Default() }
func (m *mockRuntime) TenantID() uuid.UUID                                        { return m.tenantID }
func (m *mockRuntime) Actor() *def.Actor                                          { return m.actor }
```

---

## 2. Action Handler Test Pattern

```go
func TestSubmitInvoiceAction(t *testing.T) {
    tenantID := uuid.MustParse("11000000-0000-0000-0000-000000000001")
    actorID  := uuid.MustParse("22000000-0000-0000-0000-000000000002")
    invoiceID := uuid.New()

    tests := []struct {
        name       string
        record     *def.EntityRecord
        wantErr    bool
        wantStatus int
        wantEvents int
    }{
        {
            name:       "Draft invoice — submits successfully",
            record:     invoiceRecord(invoiceID, "Draft"),
            wantErr:    false,
            wantEvents: 1,
        },
        {
            name:       "Submitted invoice — rejected",
            record:     invoiceRecord(invoiceID, "Submitted"),
            wantErr:    true,
            wantEvents: 0,
        },
        {
            name:       "Paid invoice — rejected",
            record:     invoiceRecord(invoiceID, "Paid"),
            wantErr:    true,
            wantEvents: 0,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            rt := newMockRuntime(tenantID, &def.Actor{
                UserID:   actorID,
                TenantID: tenantID,
                Roles:    []string{"role:finance.accounts_payable"},
            })
            rt.SetRepo("finance_invoice", &mockRepo{
                getFunc: func(_ context.Context, id uuid.UUID) (*def.EntityRecord, error) {
                    return tt.record, nil
                },
                updateFunc: func(_ context.Context, _ uuid.UUID, _ map[string]any) (*def.EntityRecord, error) {
                    return tt.record, nil
                },
            })

            result, err := SubmitInvoiceAction(context.Background(), def.ActionContext{
                Runtime:  rt,
                RecordID: invoiceID,
                Body:     map[string]any{},
            })

            if (err != nil) != tt.wantErr {
                t.Errorf("SubmitInvoiceAction() error = %v, wantErr %v", err, tt.wantErr)
            }
            if !tt.wantErr {
                if result == nil {
                    t.Error("expected non-nil result")
                }
                if len(rt.publishedEvents) != tt.wantEvents {
                    t.Errorf("published %d events, want %d", len(rt.publishedEvents), tt.wantEvents)
                }
                if len(rt.publishedEvents) > 0 {
                    if rt.publishedEvents[0].Topic != "finance.invoice.submitted" {
                        t.Errorf("wrong topic: %s", rt.publishedEvents[0].Topic)
                    }
                }
            }
        })
    }
}
```

---

## 3. Asserting Published Events

```go
func assertEvent(t *testing.T, rt *mockRuntime, topic string) {
    t.Helper()
    for _, e := range rt.publishedEvents {
        if e.Topic == topic {
            return
        }
    }
    t.Errorf("expected event with topic %q, got events: %v", topic, rt.publishedEvents)
}

func assertNoEvents(t *testing.T, rt *mockRuntime) {
    t.Helper()
    if len(rt.publishedEvents) != 0 {
        t.Errorf("expected no events, got %d: %v", len(rt.publishedEvents), rt.publishedEvents)
    }
}
```

---

## 4. Asserting Workflow Starts

```go
func assertWorkflowStarted(t *testing.T, rt *mockRuntime, workflowFn string) {
    t.Helper()
    for _, w := range rt.startedWorkflows {
        if w.WorkflowFn == workflowFn {
            return
        }
    }
    t.Errorf("expected workflow %q to be started, got: %v", workflowFn, rt.startedWorkflows)
}
```

---

## 5. Testing Error Cases

```go
func TestCancelInvoiceAction_MissingReason(t *testing.T) {
    rt := newMockRuntime(uuid.New(), &def.Actor{Roles: []string{"role:tenant.admin"}})

    _, err := CancelInvoiceAction(context.Background(), def.ActionContext{
        Runtime:  rt,
        RecordID: uuid.New(),
        Body:     map[string]any{}, // missing "reason"
    })

    if err == nil {
        t.Fatal("expected error, got nil")
    }
    var ve *def.ValidationError
    if !errors.As(err, &ve) {
        t.Fatalf("expected ValidationError, got %T: %v", err, err)
    }
    if ve.Fields["reason"] == "" {
        t.Error("expected field error for 'reason'")
    }
}
```

---

## References

- [`16-testing/TEST_STRATEGY.md`](TEST_STRATEGY.md) — Test strategy overview
- [`13-actions/ACTION_HANDLER_GUIDE.md`](../13-actions/ACTION_HANDLER_GUIDE.md) — Handler authoring guide
- [`13-actions/ACTION_RUNTIME_REFERENCE.md`](../13-actions/ACTION_RUNTIME_REFERENCE.md) — ActionRuntime interface
