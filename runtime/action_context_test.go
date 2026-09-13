package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"awo.so/awo/def"
	"awo.so/awo/events"
	"awo.so/awo/workflow"
)

// --- compile-time interface check ---

// TestActionContext_ImplementsInterface verifies that *ActionContext satisfies
// the def.ActionRuntime interface at compile time. No runtime assertion is
// needed — the var _ assignment below is a compile-time guard.
//
// This test exists so the file is picked up by go test; the real assertion is
// the package-level var in runtime_action_context.go.
func TestActionContext_ImplementsInterface(t *testing.T) {
	// var _ def.ActionRuntime = (*ActionContext)(nil) is declared in
	// runtime_action_context.go. If ActionContext is incomplete, the package
	// will not compile and this test will fail at build time.
	t.Log("ActionContext implements def.ActionRuntime (compile-time verified)")
}

// --- helpers ---

// noopRepo implements def.ActionEntityRepo with all methods returning zero
// values. Used when repo behaviour is not under test.
type noopRepo struct{ name string }

func (r *noopRepo) EntityName() string { return r.name }
func (r *noopRepo) Get(_ context.Context, _ uuid.UUID) (*def.EntityRecord, error) {
	return nil, errors.New("noopRepo.Get: not implemented")
}
func (r *noopRepo) Query(_ context.Context, _ def.ActionFilter, _ ...def.ActionQueryOpt) ([]*def.EntityRecord, error) {
	return nil, nil
}
func (r *noopRepo) Count(_ context.Context, _ def.ActionFilter) (int64, error)  { return 0, nil }
func (r *noopRepo) Exists(_ context.Context, _ def.ActionFilter) (bool, error)  { return false, nil }
func (r *noopRepo) Create(_ context.Context, _ map[string]any) (*def.EntityRecord, error) {
	return nil, errors.New("noopRepo.Create: not implemented")
}
func (r *noopRepo) Update(_ context.Context, _ uuid.UUID, _ map[string]any) (*def.EntityRecord, error) {
	return nil, errors.New("noopRepo.Update: not implemented")
}
func (r *noopRepo) Delete(_ context.Context, _ uuid.UUID) error {
	return errors.New("noopRepo.Delete: not implemented")
}

// capturePublisher records the last published DomainEvent.
type capturePublisher struct {
	last *events.DomainEvent
}

func (p *capturePublisher) Publish(_ context.Context, e events.DomainEvent) error {
	p.last = &e
	return nil
}

// captureExecutor records the last WorkflowSpec passed to Start.
type captureExecutor struct {
	last *workflow.WorkflowSpec
}

func (e *captureExecutor) Start(_ context.Context, spec workflow.WorkflowSpec) (workflow.WorkflowID, error) {
	e.last = &spec
	return workflow.WorkflowID("test-workflow-id"), nil
}
func (e *captureExecutor) Signal(_ context.Context, _ workflow.WorkflowID, _ string, _ any) error {
	return nil
}
func (e *captureExecutor) Query(_ context.Context, _ workflow.WorkflowID, _ string) (any, error) {
	return nil, nil
}
func (e *captureExecutor) Cancel(_ context.Context, _ workflow.WorkflowID) error { return nil }

func noopTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

func makeTestActionContext(pub events.Publisher, exec workflow.WorkflowExecutor) *ActionContext {
	tenantID := uuid.New()
	actor := &def.Actor{TenantID: tenantID}
	return NewActionContext(ActionContextConfig{
		Ctx:      context.Background(),
		TenantID: tenantID,
		Actor:    actor,
		Publish:  pub,
		Executor: exec,
		TxFn:     noopTx,
		RepoFn:   func(name string) def.ActionEntityRepo { return &noopRepo{name: name} },
	})
}

// --- Tests ---

func TestActionContext_ViewerAndTenant(t *testing.T) {
	tenantID := uuid.New()
	actor := &def.Actor{TenantID: tenantID, Roles: []string{"role:tenant.admin"}}
	ac := NewActionContext(ActionContextConfig{
		Ctx:      context.Background(),
		TenantID: tenantID,
		Actor:    actor,
		Publish:  events.NoopPublisher{},
		Executor: workflow.NoopExecutor{},
		TxFn:     noopTx,
		RepoFn:   func(name string) def.ActionEntityRepo { return &noopRepo{name: name} },
	})

	if ac.TenantID() != tenantID {
		t.Errorf("TenantID: got %v, want %v", ac.TenantID(), tenantID)
	}
	if ac.Actor() != actor {
		t.Error("Actor: returned wrong actor")
	}
}

func TestActionContext_Repo(t *testing.T) {
	ac := makeTestActionContext(events.NoopPublisher{}, workflow.NoopExecutor{})
	repo := ac.Repo("finance_invoice")
	if repo == nil {
		t.Fatal("Repo: returned nil")
	}
	if repo.EntityName() != "finance_invoice" {
		t.Errorf("Repo.EntityName: got %q, want %q", repo.EntityName(), "finance_invoice")
	}
}

func TestActionContext_Tx(t *testing.T) {
	ac := makeTestActionContext(events.NoopPublisher{}, workflow.NoopExecutor{})
	called := false
	err := ac.Tx(context.Background(), func(_ context.Context) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("Tx: unexpected error: %v", err)
	}
	if !called {
		t.Error("Tx: inner function was not called")
	}
}

func TestActionContext_Publish(t *testing.T) {
	cap := &capturePublisher{}
	ac := makeTestActionContext(cap, workflow.NoopExecutor{})

	err := ac.Publish(context.Background(), def.ActionEvent{
		Topic:   "finance.invoice.submitted",
		Payload: map[string]any{"status": "submitted"},
	})
	if err != nil {
		t.Fatalf("Publish: unexpected error: %v", err)
	}
	if cap.last == nil {
		t.Fatal("Publish: no event received by publisher")
	}
	if cap.last.ActionName != "finance.invoice.submitted" {
		t.Errorf("Publish: ActionName got %q, want %q", cap.last.ActionName, "finance.invoice.submitted")
	}
	// TenantID must be auto-populated from the context tenant.
	if cap.last.TenantID == uuid.Nil {
		t.Error("Publish: TenantID must be auto-populated when left zero in ActionEvent")
	}
}

func TestActionContext_StartWorkflow(t *testing.T) {
	cap := &captureExecutor{}
	ac := makeTestActionContext(events.NoopPublisher{}, cap)

	wid, err := ac.StartWorkflow(context.Background(), def.ActionWorkflowSpec{
		WorkflowFn: "SubmitInvoiceWorkflow",
		TaskQueue:  "finance.invoice.submit",
		Input:      map[string]any{"invoice_id": "abc"},
	})
	if err != nil {
		t.Fatalf("StartWorkflow: unexpected error: %v", err)
	}
	if wid == "" {
		t.Error("StartWorkflow: returned empty workflowID")
	}
	if cap.last == nil {
		t.Fatal("StartWorkflow: executor.Start not called")
	}
	if cap.last.TaskQueue != "finance.invoice.submit" {
		t.Errorf("StartWorkflow: TaskQueue got %q, want %q", cap.last.TaskQueue, "finance.invoice.submit")
	}
}

func TestActionContext_StartWorkflow_AutoGeneratesID(t *testing.T) {
	cap := &captureExecutor{}
	ac := makeTestActionContext(events.NoopPublisher{}, cap)

	// Leave WorkflowID empty — ActionContext must auto-generate one.
	_, err := ac.StartWorkflow(context.Background(), def.ActionWorkflowSpec{
		WorkflowFn: "SomeWorkflow",
		TaskQueue:  "some.queue",
	})
	if err != nil {
		t.Fatalf("StartWorkflow: unexpected error: %v", err)
	}
	if cap.last.WorkflowID == "" {
		t.Error("StartWorkflow: auto-generated WorkflowID must not be empty")
	}
}

func TestActionContext_Clock_ReturnsTime(t *testing.T) {
	ac := makeTestActionContext(events.NoopPublisher{}, workflow.NoopExecutor{})
	now := ac.Clock()
	if now.IsZero() {
		t.Error("Clock: returned zero time")
	}
}

func TestActionContext_Logger_NotNil(t *testing.T) {
	ac := makeTestActionContext(events.NoopPublisher{}, workflow.NoopExecutor{})
	if ac.Logger() == nil {
		t.Error("Logger: returned nil")
	}
}

func TestActionContext_Cache_NotNil(t *testing.T) {
	ac := makeTestActionContext(events.NoopPublisher{}, workflow.NoopExecutor{})
	if ac.Cache() == nil {
		t.Error("Cache: returned nil")
	}
}

func TestActionContext_Notify_NoopWhenNilFn(t *testing.T) {
	// notifyFn is not set in makeTestActionContext — Notify must be a no-op.
	ac := makeTestActionContext(events.NoopPublisher{}, workflow.NoopExecutor{})
	err := ac.Notify(context.Background(), def.ActionNotification{
		UserIDs: []uuid.UUID{uuid.New()},
		Subject: "Test notification",
		Body:    "Hello",
	})
	if err != nil {
		t.Errorf("Notify: expected nil error when notifyFn is nil; got %v", err)
	}
}

func TestActionContext_InvalidateCache_NoopWhenNilFn(t *testing.T) {
	ac := makeTestActionContext(events.NoopPublisher{}, workflow.NoopExecutor{})
	err := ac.InvalidateCache(context.Background(), "finance_invoice")
	if err != nil {
		t.Errorf("InvalidateCache: expected nil error when invalidateFn is nil; got %v", err)
	}
}

func TestNewActionContext_PanicsOnNilPublish(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when Publish is nil")
		}
	}()
	NewActionContext(ActionContextConfig{
		Ctx:      context.Background(),
		TenantID: uuid.New(),
		Publish:  nil, // intentionally nil
		Executor: workflow.NoopExecutor{},
		TxFn:     noopTx,
		RepoFn:   func(string) def.ActionEntityRepo { return &noopRepo{} },
	})
}

func TestNewActionContext_PanicsOnNilExecutor(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when Executor is nil")
		}
	}()
	NewActionContext(ActionContextConfig{
		Ctx:      context.Background(),
		TenantID: uuid.New(),
		Publish:  events.NoopPublisher{},
		Executor: nil, // intentionally nil
		TxFn:     noopTx,
		RepoFn:   func(string) def.ActionEntityRepo { return &noopRepo{} },
	})
}
