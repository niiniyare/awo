package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"awo.so/awo/def"
	"awo.so/awo/events"
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
func (r *noopRepo) Count(_ context.Context, _ def.ActionFilter) (int64, error) { return 0, nil }
func (r *noopRepo) Exists(_ context.Context, _ def.ActionFilter) (bool, error) { return false, nil }
func (r *noopRepo) Create(_ context.Context, _ map[string]any) (*def.EntityRecord, error) {
	return nil, errors.New("noopRepo.Create: not implemented")
}
func (r *noopRepo) Update(_ context.Context, _ uuid.UUID, _ map[string]any) (*def.EntityRecord, error) {
	return nil, errors.New("noopRepo.Update: not implemented")
}
func (r *noopRepo) Delete(_ context.Context, _ uuid.UUID) error {
	return errors.New("noopRepo.Delete: not implemented")
}

// capturePublisher records every published DomainEvent.
type capturePublisher struct {
	received []events.DomainEvent
}

func (p *capturePublisher) Publish(_ context.Context, e events.DomainEvent) error {
	p.received = append(p.received, e)
	return nil
}

func (p *capturePublisher) last() *events.DomainEvent {
	if len(p.received) == 0 {
		return nil
	}
	return &p.received[len(p.received)-1]
}

// failingPublisher always returns err — used to prove Publish/StartWorkflow
// propagate the underlying events.Publisher's error rather than swallowing
// it (e.g. the "no active transaction" case events/outbox.OutboxWriter.Publish
// returns in production).
type failingPublisher struct{ err error }

func (p *failingPublisher) Publish(_ context.Context, _ events.DomainEvent) error { return p.err }

func noopTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

func makeTestActionContext(pub events.Publisher) *ActionContext {
	tenantID := uuid.New()
	actor := &def.Actor{TenantID: tenantID}
	return NewActionContext(ActionContextConfig{
		Ctx:        context.Background(),
		TenantID:   tenantID,
		Actor:      actor,
		EntityName: "finance_invoice",
		RecordID:   uuid.New(),
		Publish:    pub,
		TxFn:       noopTx,
		RepoFn:     func(name string) def.ActionEntityRepo { return &noopRepo{name: name} },
	})
}

// --- Tests ---

func TestActionContext_ViewerAndTenant(t *testing.T) {
	tenantID := uuid.New()
	actor := &def.Actor{TenantID: tenantID, Roles: []string{"role:tenant.admin"}}
	ac := NewActionContext(ActionContextConfig{
		Ctx:        context.Background(),
		TenantID:   tenantID,
		Actor:      actor,
		EntityName: "finance_invoice",
		RecordID:   uuid.New(),
		Publish:    events.NoopPublisher{},
		TxFn:       noopTx,
		RepoFn:     func(name string) def.ActionEntityRepo { return &noopRepo{name: name} },
	})

	if ac.TenantID() != tenantID {
		t.Errorf("TenantID: got %v, want %v", ac.TenantID(), tenantID)
	}
	if ac.Actor() != actor {
		t.Error("Actor: returned wrong actor")
	}
}

func TestActionContext_Repo(t *testing.T) {
	ac := makeTestActionContext(events.NoopPublisher{})
	repo := ac.Repo("finance_invoice")
	if repo == nil {
		t.Fatal("Repo: returned nil")
	}
	if repo.EntityName() != "finance_invoice" {
		t.Errorf("Repo.EntityName: got %q, want %q", repo.EntityName(), "finance_invoice")
	}
}

func TestActionContext_Tx(t *testing.T) {
	ac := makeTestActionContext(events.NoopPublisher{})
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
	ac := makeTestActionContext(cap)

	err := ac.Publish(context.Background(), def.ActionEvent{
		Topic:   "finance.invoice.submitted",
		Payload: map[string]any{"status": "submitted"},
	})
	if err != nil {
		t.Fatalf("Publish: unexpected error: %v", err)
	}
	last := cap.last()
	if last == nil {
		t.Fatal("Publish: no event received by publisher")
	}
	if last.ActionName != "finance.invoice.submitted" {
		t.Errorf("Publish: ActionName got %q, want %q", last.ActionName, "finance.invoice.submitted")
	}
	// TenantID must be auto-populated from the context tenant.
	if last.TenantID == uuid.Nil {
		t.Error("Publish: TenantID must be auto-populated when left zero in ActionEvent")
	}
	// EntityName/RecordID must be populated from the ActionContext's own
	// binding — a real, previously-undiscovered bug (Step 4): the outbox
	// schema's entity_name/record_id columns are NOT NULL, so leaving them
	// zero would make every real Publish call fail at the database layer.
	if last.EntityName != "finance_invoice" {
		t.Errorf("Publish: EntityName got %q, want %q", last.EntityName, "finance_invoice")
	}
	if last.RecordID == uuid.Nil {
		t.Error("Publish: RecordID must be populated from the ActionContext's own binding, not left zero")
	}
	// Payload must be genuine JSON (Step 4 fix: json.Marshal, not
	// fmt.Sprintf("%v", ...)) — round-trip it and confirm the values survive.
	var decoded map[string]any
	if err := json.Unmarshal(last.Payload, &decoded); err != nil {
		t.Fatalf("Publish: payload is not valid JSON: %v (payload: %q)", err, last.Payload)
	}
	if decoded["status"] != "submitted" {
		t.Errorf("Publish: payload round-trip: got %v, want status=submitted", decoded)
	}
}

func TestActionContext_Publish_PropagatesPublisherError(t *testing.T) {
	sentinel := errors.New("no active transaction in context")
	ac := makeTestActionContext(&failingPublisher{err: sentinel})

	err := ac.Publish(context.Background(), def.ActionEvent{Topic: "x", Payload: map[string]any{}})
	if !errors.Is(err, sentinel) {
		t.Errorf("Publish: expected underlying publisher error to propagate, got %v", err)
	}
}

// TestActionContext_StartWorkflow_PublishesDurableIntent_NotDirectTemporalCall
// is the central Step 4 regression test at the unit level: StartWorkflow
// must publish an EventWorkflowTriggerFired domain event through the same
// events.Publisher Publish uses — it must never call a workflow executor
// directly. There is no workflow.WorkflowExecutor dependency on ActionContext
// at all anymore (see ActionContextConfig — Executor was removed); if
// StartWorkflow tried to call one, this test file would not compile.
func TestActionContext_StartWorkflow_PublishesDurableIntent_NotDirectTemporalCall(t *testing.T) {
	cap := &capturePublisher{}
	ac := makeTestActionContext(cap)

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

	last := cap.last()
	if last == nil {
		t.Fatal("StartWorkflow: no event published — the durable intent must be an outbox event, not a direct executor call")
	}
	if last.Type != events.EventWorkflowTriggerFired {
		t.Errorf("StartWorkflow: event Type got %q, want %q", last.Type, events.EventWorkflowTriggerFired)
	}
	if last.EntityName != "finance_invoice" || last.RecordID == uuid.Nil {
		t.Errorf("StartWorkflow: EntityName/RecordID must be populated (NOT NULL outbox columns): got EntityName=%q RecordID=%v",
			last.EntityName, last.RecordID)
	}

	var spec def.ActionWorkflowSpec
	if err := json.Unmarshal(last.Payload, &spec); err != nil {
		t.Fatalf("StartWorkflow: payload is not valid JSON: %v", err)
	}
	if spec.WorkflowFn != "SubmitInvoiceWorkflow" {
		t.Errorf("StartWorkflow: payload WorkflowFn got %q, want %q", spec.WorkflowFn, "SubmitInvoiceWorkflow")
	}
	if spec.TaskQueue != "finance.invoice.submit" {
		t.Errorf("StartWorkflow: payload TaskQueue got %q, want %q", spec.TaskQueue, "finance.invoice.submit")
	}
	if spec.WorkflowID != wid {
		t.Errorf("StartWorkflow: payload WorkflowID (%q) must match the returned workflowID (%q) — "+
			"the relay's later dispatch must use the exact same resolved ID", spec.WorkflowID, wid)
	}
}

func TestActionContext_StartWorkflow_AutoGeneratesID(t *testing.T) {
	cap := &capturePublisher{}
	ac := makeTestActionContext(cap)

	// Leave WorkflowID empty — ActionContext must auto-generate one.
	wid, err := ac.StartWorkflow(context.Background(), def.ActionWorkflowSpec{
		WorkflowFn: "SomeWorkflow",
		TaskQueue:  "some.queue",
	})
	if err != nil {
		t.Fatalf("StartWorkflow: unexpected error: %v", err)
	}
	if wid == "" {
		t.Error("StartWorkflow: auto-generated WorkflowID must not be empty")
	}

	var spec def.ActionWorkflowSpec
	if err := json.Unmarshal(cap.last().Payload, &spec); err != nil {
		t.Fatalf("payload not valid JSON: %v", err)
	}
	if spec.WorkflowID != wid {
		t.Errorf("auto-generated WorkflowID must be reflected in the published payload: got %q, want %q", spec.WorkflowID, wid)
	}
}

func TestActionContext_StartWorkflow_PropagatesPublisherError(t *testing.T) {
	sentinel := errors.New("no active transaction in context")
	ac := makeTestActionContext(&failingPublisher{err: sentinel})

	_, err := ac.StartWorkflow(context.Background(), def.ActionWorkflowSpec{WorkflowFn: "X", TaskQueue: "q"})
	if !errors.Is(err, sentinel) {
		t.Errorf("StartWorkflow: expected underlying publisher error to propagate, got %v", err)
	}
}

func TestActionContext_Clock_ReturnsTime(t *testing.T) {
	ac := makeTestActionContext(events.NoopPublisher{})
	now := ac.Clock()
	if now.IsZero() {
		t.Error("Clock: returned zero time")
	}
}

func TestActionContext_Logger_NotNil(t *testing.T) {
	ac := makeTestActionContext(events.NoopPublisher{})
	if ac.Logger() == nil {
		t.Error("Logger: returned nil")
	}
}

func TestActionContext_Cache_NotNil(t *testing.T) {
	ac := makeTestActionContext(events.NoopPublisher{})
	if ac.Cache() == nil {
		t.Error("Cache: returned nil")
	}
}

func TestActionContext_Notify_NoopWhenNilFn(t *testing.T) {
	// notifyFn is not set in makeTestActionContext — Notify must be a no-op.
	ac := makeTestActionContext(events.NoopPublisher{})
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
	ac := makeTestActionContext(events.NoopPublisher{})
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
		TxFn:     noopTx,
		RepoFn:   func(string) def.ActionEntityRepo { return &noopRepo{} },
	})
}

func TestNewActionContext_PanicsOnNilTxFn(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when TxFn is nil")
		}
	}()
	NewActionContext(ActionContextConfig{
		Ctx:      context.Background(),
		TenantID: uuid.New(),
		Publish:  events.NoopPublisher{},
		TxFn:     nil, // intentionally nil
		RepoFn:   func(string) def.ActionEntityRepo { return &noopRepo{} },
	})
}

func TestNewActionContext_PanicsOnNilRepoFn(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when RepoFn is nil")
		}
	}()
	NewActionContext(ActionContextConfig{
		Ctx:      context.Background(),
		TenantID: uuid.New(),
		Publish:  events.NoopPublisher{},
		TxFn:     noopTx,
		RepoFn:   nil, // intentionally nil
	})
}
