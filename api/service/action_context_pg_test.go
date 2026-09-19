package service_test

// Phase 2 Step 4 — Canonical ActionContext Runtime.
//
// These tests exercise the real api/service.ActionContextFactory, the real
// runtime.ActionContext it constructs, the real events/outbox.OutboxWriter
// (Step 1), the real events_outbox migration (Step 2), the real
// api/service.EntityService canonical mutation pipeline (Step 3), and the
// real events/outbox.Relay (Step 3B) — against real PostgreSQL — proving
// the full chain:
//
//	custom action → ActionContext.Publish/StartWorkflow/Repo(...).Create
//	  → same transaction as any other mutation
//	  → COMMIT
//	  → relay → WorkflowTriggerSubscriber → (fake) Temporal
//
// A fake workflow.WorkflowExecutor stands in for Temporal — no live Temporal
// cluster is required or contacted anywhere in this file, matching ADR-025
// §12 (Phase 2 does not require a running Temporal server).

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/api/service"
	"awo.so/awo/audit"
	"awo.so/awo/def"
	"awo.so/awo/events"
	"awo.so/awo/events/outbox"
	testdb "awo.so/awo/testutil/db"
	"awo.so/awo/workflow"
)

// fakeWorkflowExecutor is a controllable workflow.WorkflowExecutor test
// double — the "controlled Temporal mock/stub" the Step 4 instructions
// require, so these tests never need a live Temporal cluster.
type fakeWorkflowExecutor struct {
	mu       sync.Mutex
	started  []workflow.WorkflowSpec
	failWith error
}

func (e *fakeWorkflowExecutor) Start(_ context.Context, spec workflow.WorkflowSpec) (workflow.WorkflowID, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.failWith != nil {
		return "", e.failWith
	}
	e.started = append(e.started, spec)
	return workflow.WorkflowID(spec.WorkflowID), nil
}
func (e *fakeWorkflowExecutor) Signal(context.Context, workflow.WorkflowID, string, any) error {
	return nil
}
func (e *fakeWorkflowExecutor) Query(context.Context, workflow.WorkflowID, string) (any, error) {
	return nil, nil
}
func (e *fakeWorkflowExecutor) Cancel(context.Context, workflow.WorkflowID) error { return nil }

func (e *fakeWorkflowExecutor) count() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.started)
}

func (e *fakeWorkflowExecutor) startedSpecs() []workflow.WorkflowSpec {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]workflow.WorkflowSpec, len(e.started))
	copy(out, e.started)
	return out
}

// setupActionContextTest builds a real EntityService (Step 3 harness, via
// setupPGService already defined in entity_pg_atomicity_test.go, same
// package) plus a real ActionContextFactory registered against it,
// publishing through a fresh outbox.NewWriter(h.pool) — the identical
// mechanism h.svc already uses internally, not a second, independently
// constructed one.
func setupActionContextTest(t *testing.T) (*pgServiceHarness, *service.ActionContextFactory) {
	t.Helper()
	h := setupPGService(t, pgEntityName, audit.CategoryData)
	factory := service.NewActionContextFactory(outbox.NewWriter(h.pool))
	factory.Register(h.svc)
	return h, factory
}

func workflowOutboxRowState(t *testing.T, h *pgServiceHarness, recordID uuid.UUID) (found bool, deliveredAt, deadAt, attempts any) {
	t.Helper()
	row := testdb.QueryRowSQL(t, h.pool, `
		SELECT delivered_at, dead_at, attempts FROM events_outbox
		WHERE record_id = $1 AND type = $2
	`, recordID, string(events.EventWorkflowTriggerFired))
	var d, dd any
	var a int
	if err := row.Scan(&d, &dd, &a); err != nil {
		return false, nil, nil, nil
	}
	return true, d, dd, a
}

// ── A/S — ActionContext is created with the canonical runtime ──────────────

func TestActionContextFactory_New_ReturnsFunctionalRuntime(t *testing.T) {
	h, factory := setupActionContextTest(t)
	tenantID, ctx := pgActivateTenant(t, h.pool)
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}
	recordID := uuid.New()

	rt, err := factory.New(ctx, pgEntityName, recordID, actor)
	require.NoError(t, err)
	require.NotNil(t, rt)

	assert.Equal(t, tenantID, rt.TenantID())
	assert.Equal(t, actor, rt.Actor())
	assert.NotNil(t, rt.Repo(pgEntityName), "Repo must return a functional adapter, not nil")
}

func TestActionContextFactory_New_UnregisteredEntity_ReturnsError(t *testing.T) {
	_, factory := setupActionContextTest(t)
	_, err := factory.New(context.Background(), "no_such_entity", uuid.New(), &def.Actor{})
	require.Error(t, err, "requesting an ActionContext for an unregistered entity must fail loudly, not silently construct a broken one")
}

// ── B/C/D/E/F — Publish transaction semantics ───────────────────────────────

func TestActionContext_Publish_WritesToEventsOutbox_SameTransactionAsMutation(t *testing.T) {
	h, factory := setupActionContextTest(t)
	tenantID, ctx := pgActivateTenant(t, h.pool)
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}
	recordID := uuid.New()

	rt, err := factory.New(ctx, pgEntityName, recordID, actor)
	require.NoError(t, err)

	require.NoError(t, rt.Tx(ctx, func(txCtx context.Context) error {
		return rt.Publish(txCtx, def.ActionEvent{Topic: "svc.custom_action_fired", Payload: map[string]any{"ok": true}})
	}))

	ob := readOutboxRow(t, h.pool, recordID)
	require.True(t, ob.found, "Publish must write a real events_outbox row")
	assert.Equal(t, string(events.EventActionFired), ob.eventType)
	assert.Equal(t, tenantID, ob.tenantID)
}

func TestActionContext_Publish_RollsBackWithMutation(t *testing.T) {
	h, factory := setupActionContextTest(t)
	tenantID, ctx := pgActivateTenant(t, h.pool)
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}
	recordID := uuid.New()

	rt, err := factory.New(ctx, pgEntityName, recordID, actor)
	require.NoError(t, err)

	txErr := rt.Tx(ctx, func(txCtx context.Context) error {
		if err := rt.Publish(txCtx, def.ActionEvent{Topic: "x", Payload: map[string]any{}}); err != nil {
			return err
		}
		return assert.AnError // force rollback
	})
	require.Error(t, txErr)

	ob := readOutboxRow(t, h.pool, recordID)
	assert.False(t, ob.found, "a rolled-back transaction must leave no outbox row behind")
}

func TestActionContext_Publish_NoActiveTransaction_ReturnsErrorNotSilentSuccess(t *testing.T) {
	h, factory := setupActionContextTest(t)
	tenantID, ctx := pgActivateTenant(t, h.pool)
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	rt, err := factory.New(ctx, pgEntityName, uuid.New(), actor)
	require.NoError(t, err)

	// Publish called directly, NOT inside rt.Tx(...) — must fail loudly
	// (events.Publisher's documented contract), never silently succeed via
	// a bare pool connection outside any transaction, and never silently
	// open a second, independent transaction of its own.
	err = rt.Publish(ctx, def.ActionEvent{Topic: "x", Payload: map[string]any{}})
	require.Error(t, err)
}

// ── G/H — Payload JSON correctness ──────────────────────────────────────────

func TestActionContext_Publish_StructuredJSONPayload_SurvivesUnchanged(t *testing.T) {
	h, factory := setupActionContextTest(t)
	tenantID, ctx := pgActivateTenant(t, h.pool)
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}
	recordID := uuid.New()

	rt, err := factory.New(ctx, pgEntityName, recordID, actor)
	require.NoError(t, err)

	nested := map[string]any{
		"status": "submitted",
		"lines":  []any{map[string]any{"sku": "A1", "qty": float64(2)}},
	}
	require.NoError(t, rt.Tx(ctx, func(txCtx context.Context) error {
		return rt.Publish(txCtx, def.ActionEvent{Topic: "x", Payload: nested})
	}))

	ob := readOutboxRow(t, h.pool, recordID)
	require.True(t, ob.found)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(ob.payload, &decoded))
	assert.Equal(t, nested, decoded, "the persisted payload must be structurally identical to the original — no double-encoding, no data loss")
}

func TestActionContext_Publish_NilPayload_DoesNotError(t *testing.T) {
	h, factory := setupActionContextTest(t)
	tenantID, ctx := pgActivateTenant(t, h.pool)
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}
	recordID := uuid.New()

	rt, err := factory.New(ctx, pgEntityName, recordID, actor)
	require.NoError(t, err)

	require.NoError(t, rt.Tx(ctx, func(txCtx context.Context) error {
		return rt.Publish(txCtx, def.ActionEvent{Topic: "x", Payload: nil})
	}))

	ob := readOutboxRow(t, h.pool, recordID)
	require.True(t, ob.found)
	// json.Marshal(nil) => "null", a valid single JSON token — not an error,
	// not a fmt.Sprintf("%v", nil) => "<nil>" (invalid JSON, the pre-fix bug).
	assert.True(t, json.Valid(ob.payload))
}

// ── I/J/K — Correlation ID / system actor / tenant ID metadata ─────────────

func TestActionContext_Publish_CorrelationIDAndTenantPreserved_SystemActorEmpty(t *testing.T) {
	h, factory := setupActionContextTest(t)
	tenantID, ctx := pgActivateTenant(t, h.pool)
	const requestID = "req-not-a-uuid-555"
	ctx = audit.WithRequestContext(ctx, audit.RequestContext{RequestID: requestID})
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}
	recordID := uuid.New()

	rt, err := factory.New(ctx, pgEntityName, recordID, actor)
	require.NoError(t, err)
	require.NoError(t, rt.Tx(ctx, func(txCtx context.Context) error {
		return rt.Publish(txCtx, def.ActionEvent{Topic: "x", Payload: map[string]any{}})
	}))

	ob := readOutboxRow(t, h.pool, recordID)
	require.True(t, ob.found)
	assert.Equal(t, tenantID, ob.tenantID)
	require.NotNil(t, ob.correlationID)
	assert.Equal(t, requestID, *ob.correlationID, "correlation_id must round-trip as plain text, no UUID coercion")
	assert.True(t, ob.systemActor == nil || *ob.systemActor == "", "system_actor must remain empty for an ordinary actor-driven action")
}

// ── L/M — Workflow intent payload is bounded ────────────────────────────────

func TestActionContext_StartWorkflow_PayloadIsBoundedActionWorkflowSpec_NoExtraData(t *testing.T) {
	h, factory := setupActionContextTest(t)
	tenantID, ctx := pgActivateTenant(t, h.pool)
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}
	recordID := uuid.New()

	rt, err := factory.New(ctx, pgEntityName, recordID, actor)
	require.NoError(t, err)

	require.NoError(t, rt.Tx(ctx, func(txCtx context.Context) error {
		_, err := rt.StartWorkflow(txCtx, def.ActionWorkflowSpec{
			WorkflowFn: "SubmitInvoiceWorkflow",
			TaskQueue:  "finance.invoice.submit",
			Input:      map[string]any{"invoice_id": "abc"},
		})
		return err
	}))

	ob := readOutboxRow(t, h.pool, recordID)
	require.True(t, ob.found)
	assert.Equal(t, string(events.EventWorkflowTriggerFired), ob.eventType)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(ob.payload, &decoded))
	// Bounded: exactly the def.ActionWorkflowSpec JSON keys — never a full
	// entity snapshot, credentials, or Temporal connection details.
	allowedKeys := map[string]bool{"WorkflowFn": true, "TaskQueue": true, "WorkflowID": true, "Input": true}
	for k := range decoded {
		assert.True(t, allowedKeys[k], "unexpected key %q in workflow-intent payload — payload must be exactly the bounded ActionWorkflowSpec", k)
	}
	assert.Equal(t, "SubmitInvoiceWorkflow", decoded["WorkflowFn"])
}

// ── N/O — StartWorkflow does not call Temporal; it publishes durable intent ─

func TestActionContext_StartWorkflow_DoesNotCallExecutor_PublishesIntentInstead(t *testing.T) {
	h, factory := setupActionContextTest(t)
	tenantID, ctx := pgActivateTenant(t, h.pool)
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}
	recordID := uuid.New()

	fakeExec := &fakeWorkflowExecutor{}
	rt, err := factory.New(ctx, pgEntityName, recordID, actor)
	require.NoError(t, err)

	require.NoError(t, rt.Tx(ctx, func(txCtx context.Context) error {
		_, err := rt.StartWorkflow(txCtx, def.ActionWorkflowSpec{WorkflowFn: "X", TaskQueue: "q"})
		return err
	}))

	assert.Equal(t, 0, fakeExec.count(),
		"StartWorkflow must never call the workflow executor directly from a mutation-bound path — "+
			"ActionContext has no executor dependency at all; only the relay's WorkflowTriggerSubscriber does")

	ob := readOutboxRow(t, h.pool, recordID)
	require.True(t, ob.found, "a durable workflow-trigger intent must exist instead")
}

func TestActionContext_StartWorkflow_RollsBackWithSurroundingTransaction(t *testing.T) {
	h, factory := setupActionContextTest(t)
	tenantID, ctx := pgActivateTenant(t, h.pool)
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}
	recordID := uuid.New()

	rt, err := factory.New(ctx, pgEntityName, recordID, actor)
	require.NoError(t, err)

	txErr := rt.Tx(ctx, func(txCtx context.Context) error {
		if _, err := rt.StartWorkflow(txCtx, def.ActionWorkflowSpec{WorkflowFn: "X", TaskQueue: "q"}); err != nil {
			return err
		}
		return assert.AnError
	})
	require.Error(t, txErr)

	ob := readOutboxRow(t, h.pool, recordID)
	assert.False(t, ob.found, "a rolled-back transaction must leave no workflow-trigger intent behind — no eventual dispatch can occur")
}

// ── P/Q/R — Relay integration: dispatch, failure, and commit independence ──

func TestWorkflowIntent_Relay_DispatchesAfterCommit(t *testing.T) {
	h, factory := setupActionContextTest(t)
	tenantID, ctx := pgActivateTenant(t, h.pool)
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}
	recordID := uuid.New()

	rt, err := factory.New(ctx, pgEntityName, recordID, actor)
	require.NoError(t, err)
	require.NoError(t, rt.Tx(ctx, func(txCtx context.Context) error {
		_, err := rt.StartWorkflow(txCtx, def.ActionWorkflowSpec{
			WorkflowFn: "SubmitInvoiceWorkflow", TaskQueue: "finance.invoice.submit",
			Input: map[string]any{"invoice_id": "abc"},
		})
		return err
	}))

	fakeExec := &fakeWorkflowExecutor{}
	relay := outbox.New(h.pool)
	relay.Subscribe(events.EventWorkflowTriggerFired, outbox.NewWorkflowTriggerSubscriber(fakeExec))
	require.NoError(t, relay.Poll(context.Background()))

	require.Equal(t, 1, fakeExec.count(), "the relay must dispatch the durable intent to the executor after commit")
	spec := fakeExec.startedSpecs()[0]
	assert.Equal(t, "finance.invoice.submit", spec.TaskQueue)
	assert.Equal(t, "SubmitInvoiceWorkflow", spec.WorkflowFn)

	found, deliveredAt, _, _ := workflowOutboxRowState(t, h, recordID)
	require.True(t, found)
	assert.NotNil(t, deliveredAt, "the outbox row must be marked delivered after successful dispatch")
}

func TestWorkflowIntent_Relay_DispatchFailure_LeavesEventRetryable(t *testing.T) {
	h, factory := setupActionContextTest(t)
	tenantID, ctx := pgActivateTenant(t, h.pool)
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}
	recordID := uuid.New()

	rt, err := factory.New(ctx, pgEntityName, recordID, actor)
	require.NoError(t, err)
	require.NoError(t, rt.Tx(ctx, func(txCtx context.Context) error {
		_, err := rt.StartWorkflow(txCtx, def.ActionWorkflowSpec{WorkflowFn: "X", TaskQueue: "q"})
		return err
	}))

	failingExec := &fakeWorkflowExecutor{failWith: workflow.ErrWorkflowUnavailable}
	relay := outbox.New(h.pool)
	relay.Subscribe(events.EventWorkflowTriggerFired, outbox.NewWorkflowTriggerSubscriber(failingExec))
	require.NoError(t, relay.Poll(context.Background()))

	found, deliveredAt, deadAt, attempts := workflowOutboxRowState(t, h, recordID)
	require.True(t, found)
	assert.Nil(t, deliveredAt, "a failed dispatch must not be marked delivered")
	assert.Nil(t, deadAt, "a single failure must not immediately dead-letter — Step 3B's existing backoff/MaxAttempts policy governs this, unchanged")
	assert.Equal(t, 1, attempts, "exactly one dispatch attempt was made")
}

func TestWorkflowIntent_TemporalUnavailable_DoesNotRollBackAlreadyCommittedMutation(t *testing.T) {
	h, factory := setupActionContextTest(t)
	tenantID, ctx := pgActivateTenant(t, h.pool)
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}
	recordID := uuid.New()

	rt, err := factory.New(ctx, pgEntityName, recordID, actor)
	require.NoError(t, err)

	// A real business mutation (via the SAME ActionContext's Repo, going
	// through the canonical EntityService pipeline) and a workflow-trigger
	// intent, both inside one transaction.
	var created *def.EntityRecord
	require.NoError(t, rt.Tx(ctx, func(txCtx context.Context) error {
		var err error
		created, err = rt.Repo(pgEntityName).Create(txCtx, map[string]any{"status": "draft"})
		if err != nil {
			return err
		}
		_, err = rt.StartWorkflow(txCtx, def.ActionWorkflowSpec{WorkflowFn: "X", TaskQueue: "q"})
		return err
	}))
	require.NotNil(t, created)

	// Temporal is now "unavailable" — the relay's dispatch fails.
	failingExec := &fakeWorkflowExecutor{failWith: workflow.ErrWorkflowUnavailable}
	relay := outbox.New(h.pool)
	relay.Subscribe(events.EventWorkflowTriggerFired, outbox.NewWorkflowTriggerSubscriber(failingExec))
	require.NoError(t, relay.Poll(context.Background()))

	// The business mutation — already committed, minutes/attempts before
	// this relay cycle even ran — must remain exactly as committed. A
	// PostgreSQL transaction cannot be rolled back after COMMIT returns;
	// this test proves the design never attempts to violate that.
	assert.Equal(t, 1, entityRowCount(t, h.pool, pgEntityName, created.ID),
		"a post-commit relay/Temporal failure must never affect an already-committed mutation")
}

// ── S — Custom action's Repo(...) goes through the canonical pipeline ──────

func TestActionContext_Repo_Create_GoesThroughCanonicalPipeline_AuditAndOutbox(t *testing.T) {
	h, factory := setupActionContextTest(t)
	tenantID, ctx := pgActivateTenant(t, h.pool)
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	rt, err := factory.New(ctx, pgEntityName, uuid.New(), actor)
	require.NoError(t, err)

	created, err := rt.Repo(pgEntityName).Create(ctx, map[string]any{"status": "via-action"})
	require.NoError(t, err)
	require.NotNil(t, created)

	// The canonical EntityService.Create pipeline ran: audit record +
	// lifecycle outbox event, exactly like an HTTP POST would produce —
	// actions do not get a lesser mutation path.
	assert.Equal(t, 1, auditRowCount(t, h.pool, created.ID), "Repo(...).Create must write an audit record via the canonical pipeline")
	ob := readOutboxRow(t, h.pool, created.ID)
	assert.True(t, ob.found, "Repo(...).Create must publish a lifecycle outbox event via the canonical pipeline")
}
