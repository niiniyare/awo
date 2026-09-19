package service_test

// Phase 2 Step 6 — Complete the transactional workflow handoff.
//
// EntityService.Create/Update/Delete/CreateBatch previously fired a matching
// WorkflowTrigger by calling s.temporal.ExecuteWorkflow directly, outside
// the mutation transaction, after commit — a non-durable side effect: if the
// process crashed between commit and that call, or Temporal was briefly
// unreachable, the workflow silently never started, with no record of the
// missed intent anywhere.
//
// These tests prove the replacement: a matching WorkflowTrigger now causes
// EntityService to publish a durable events.EventWorkflowTriggerFired event
// through the SAME transaction-bound publisher, and in the SAME transaction,
// as the mutation and its audit/lifecycle event — never a direct Temporal
// call. Actual dispatch to Temporal happens later, independently, via the
// existing (Step 3B) outbox relay and (Step 4) WorkflowTriggerSubscriber —
// reused unchanged here, not reimplemented.
//
// Real PostgreSQL throughout; a fake workflow.WorkflowExecutor (reusing
// fakeWorkflowExecutor from action_context_pg_test.go, same package) stands
// in for Temporal — no live Temporal cluster required (ADR-025 §12).

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/api/service"
	"awo.so/awo/audit"
	"awo.so/awo/compiler"
	contribpgx "awo.so/awo/contrib/pgx"
	"awo.so/awo/def"
	"awo.so/awo/events"
	"awo.so/awo/events/outbox"
	"awo.so/awo/registry"
	"awo.so/awo/runtime"
	"awo.so/awo/runtime/tenant"
	testdb "awo.so/awo/testutil/db"
)

const wfEntityName = "svc_pg_workflow_trigger_entity"

// selectiveFailPublisher wraps a real events.Publisher and fails ONLY for a
// chosen EventType, delegating everything else through unchanged. Used to
// prove that a workflow-intent-specific publish failure — as opposed to a
// blanket outbox outage — rolls back the whole transaction, including an
// already-attempted lifecycle-event publish earlier in the same transaction
// (which never actually persists, since nothing in the transaction commits
// until WithTx's own COMMIT, which never runs once the callback returns an
// error).
type selectiveFailPublisher struct {
	inner  events.Publisher
	failOn events.EventType
	err    error
}

func (p *selectiveFailPublisher) Publish(ctx context.Context, e events.DomainEvent) error {
	if e.Type == p.failOn {
		return p.err
	}
	return p.inner.Publish(ctx, e)
}

// setupWorkflowTriggerService builds a real EntityService for a freshly
// created entity declaring exactly one WorkflowTrigger on the given event —
// pgEntityDef (used by every other file in this package) deliberately
// declares none. pub, when non-nil, overrides the default
// outbox.NewWriter(pool) publisher.
//
// Deliberately does NOT call testdb.InstallTenantLifecycle: this file's
// Update/Delete tests call EntityService.Get standalone (outside any
// transaction) as EntityService.Update/Delete themselves do internally, and
// InstallTenantLifecycle's production set_tenant_context sets
// "awo.tenant_id" with is_local=true (PostgreSQL's SET LOCAL semantics) —
// scoped to the single implicit transaction of whichever one statement set
// it, so it is already gone by the time a SEPARATE, later autocommit
// statement (like a standalone Get) runs. contrib/pgx.Repository.Get/Query
// never call set_tenant_context themselves (only Repository.WithTx does,
// fresh, inside its own transaction) — a pre-existing gap this file's tests
// must route around, not fix (out of Step 6's scope: this is an RLS/tenant-
// context durability issue, not a Temporal/workflow one — see the Step 6
// report's Remaining Findings). Applying just InstallTenantLifecycle's own
// minimal platform_tenant table (below), without swapping in its is_local
// =true function, keeps SetupTestDB's own default set_tenant_context —
// session-scoped (is_local=false) — in effect, which does not have this
// problem, letting these tests exercise workflow-intent durability without
// tripping over the unrelated bug.
func setupWorkflowTriggerService(t *testing.T, on def.EventType, pub events.Publisher) (svc *service.EntityService, pool *pgxpool.Pool) {
	t.Helper()

	pool = testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, `
CREATE TABLE IF NOT EXISTS platform_tenant (
    id     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    status text NOT NULL DEFAULT 'ACTIVE'
);
GRANT SELECT ON platform_tenant TO awo_app;
`)
	testdb.ApplySQL(t, pool, pgEntityDDL(wfEntityName))
	testdb.ApplySQL(t, pool, platformAuditLogTestDDL)
	testdb.ApplySQL(t, pool, readOutboxMigrationSQL(t))

	inputBuilder := func(record *def.EntityRecord, tc def.TriggerContext) (any, error) {
		return map[string]any{"status": record.GetString("status")}, nil
	}

	d := &def.SystemDefinition{
		Name:   wfEntityName,
		Module: "svc",
		Label:  "PGWorkflowTrigger",
		Fields: []def.FieldDef{
			{Name: "status", Type: def.FieldTypeData},
			{Name: "ssn", Type: def.FieldTypeData, Sensitive: true},
		},
		WorkflowTriggers: []def.WorkflowTrigger{
			{On: on, WorkflowFn: "test_workflow", TaskQueue: "test.queue", InputBuilder: inputBuilder},
		},
	}
	reg, err := registry.BuildFrom([]def.EntityDefinition{d})
	require.NoError(t, err)
	schema, err := compiler.Compile(reg)
	require.NoError(t, err)
	es := schema.ByName[wfEntityName]
	require.NotNil(t, es)

	if def.Lookup(wfEntityName) == nil {
		def.Register(d)
	}
	audit.Register(audit.EntityAuditConfig{EntityName: wfEntityName, Enabled: true, Category: audit.CategoryData})

	auditWriter := audit.NewPostgresWriter(contribpgx.NewPoolQuerier(pool))
	pipeline := runtime.NewPipeline(schema, auditWriter)
	repo := contribpgx.NewRepository(pool, es)

	if pub == nil {
		pub = outbox.NewWriter(pool)
	}
	svc = service.NewEntityService(es, repo, pipeline).WithPublisher(pub)
	return svc, pool
}

// ── 1. Create ─────────────────────────────────────────────────────────────

func TestEntityService_PG_Create_WorkflowTriggered_PersistsIntentTransactionally(t *testing.T) {
	svc, pool := setupWorkflowTriggerService(t, def.EventOnCreate, nil)
	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantID})
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}
	ctx = audit.WithRequestContext(ctx, audit.RequestContext{RequestID: "req-create-wf-1"})

	created, err := svc.Create(ctx, map[string]any{"status": "draft"}, actor)
	require.NoError(t, err)

	// Lifecycle event still exists (unaffected by adding workflow intents).
	lifecycle := readOutboxRow(t, pool, created.ID)
	require.True(t, lifecycle.found)
	assert.Equal(t, string(events.EventCreated), lifecycle.eventType)

	// Workflow intent exists, same record, same transaction's outbox write.
	var count int
	require.NoError(t, testdb.QueryRowSQL(t, pool,
		`SELECT COUNT(*) FROM events_outbox WHERE record_id = $1 AND type = $2`,
		created.ID, string(events.EventWorkflowTriggerFired)).Scan(&count))
	require.Equal(t, 1, count, "exactly one workflow-intent event must exist for the created record")

	row := testdb.QueryRowSQL(t, pool,
		`SELECT tenant_id, actor_id, correlation_id, payload FROM events_outbox WHERE record_id = $1 AND type = $2`,
		created.ID, string(events.EventWorkflowTriggerFired))
	var gotTenant uuid.UUID
	var gotActor uuid.UUID
	var gotCorrelation *string
	var payload []byte
	require.NoError(t, row.Scan(&gotTenant, &gotActor, &gotCorrelation, &payload))

	assert.Equal(t, tenantID, gotTenant, "workflow intent must carry the mutation's own tenant")
	assert.Equal(t, actor.UserID, gotActor, "workflow intent must carry the mutation's own actor")
	require.NotNil(t, gotCorrelation)
	assert.Equal(t, "req-create-wf-1", *gotCorrelation, "correlation ID must propagate to the workflow intent, same as the lifecycle event")

	var spec def.ActionWorkflowSpec
	require.NoError(t, json.Unmarshal(payload, &spec))
	assert.Equal(t, "test_workflow", spec.WorkflowFn)
	assert.Equal(t, "test.queue", spec.TaskQueue)
	assert.NotEmpty(t, spec.WorkflowID)
}

// ── 2. Update ─────────────────────────────────────────────────────────────

func TestEntityService_PG_Update_WorkflowTriggered_PersistsIntentTransactionally(t *testing.T) {
	svc, pool := setupWorkflowTriggerService(t, def.EventOnUpdate, nil)
	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantID})
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	created, err := svc.Create(ctx, map[string]any{"status": "draft"}, actor)
	require.NoError(t, err)

	// Create itself must NOT have fired an on_update trigger.
	var createCount int
	require.NoError(t, testdb.QueryRowSQL(t, pool,
		`SELECT COUNT(*) FROM events_outbox WHERE record_id = $1 AND type = $2`,
		created.ID, string(events.EventWorkflowTriggerFired)).Scan(&createCount))
	assert.Equal(t, 0, createCount, "an on_update trigger must not fire on Create")

	updated, err := svc.Update(ctx, created.ID, map[string]any{"status": "final"}, actor)
	require.NoError(t, err)

	var count int
	require.NoError(t, testdb.QueryRowSQL(t, pool,
		`SELECT COUNT(*) FROM events_outbox WHERE record_id = $1 AND type = $2`,
		updated.ID, string(events.EventWorkflowTriggerFired)).Scan(&count))
	assert.Equal(t, 1, count, "Update must fire the matching on_update workflow trigger")
}

// ── 3. Delete ─────────────────────────────────────────────────────────────

func TestEntityService_PG_Delete_WorkflowTriggered_PersistsIntentTransactionally(t *testing.T) {
	svc, pool := setupWorkflowTriggerService(t, def.EventOnDelete, nil)
	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantID})
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	created, err := svc.Create(ctx, map[string]any{"status": "draft"}, actor)
	require.NoError(t, err)

	require.NoError(t, svc.Delete(ctx, created.ID, actor))

	var count int
	require.NoError(t, testdb.QueryRowSQL(t, pool,
		`SELECT COUNT(*) FROM events_outbox WHERE record_id = $1 AND type = $2`,
		created.ID, string(events.EventWorkflowTriggerFired)).Scan(&count))
	assert.Equal(t, 1, count, "Delete must fire the matching on_delete workflow trigger")
}

// ── 4. Mutation failure produces no workflow intent ──────────────────────

func TestEntityService_PG_Create_ValidationFailure_NoWorkflowIntent(t *testing.T) {
	const entityName = "svc_pg_wf_required_entity"
	pool := testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, pool)
	testdb.ApplySQL(t, pool, pgEntityDDL(entityName))
	testdb.ApplySQL(t, pool, platformAuditLogTestDDL)
	testdb.ApplySQL(t, pool, readOutboxMigrationSQL(t))

	d := &def.SystemDefinition{
		Name:   entityName,
		Module: "svc",
		Label:  "PGWFRequired",
		Fields: []def.FieldDef{{Name: "status", Type: def.FieldTypeData, Required: true}},
		WorkflowTriggers: []def.WorkflowTrigger{
			{On: def.EventOnCreate, WorkflowFn: "test_workflow", TaskQueue: "test.queue",
				InputBuilder: func(*def.EntityRecord, def.TriggerContext) (any, error) { return nil, nil }},
		},
	}
	reg, err := registry.BuildFrom([]def.EntityDefinition{d})
	require.NoError(t, err)
	schema, err := compiler.Compile(reg)
	require.NoError(t, err)
	es := schema.ByName[entityName]
	require.NotNil(t, es)

	audit.Register(audit.EntityAuditConfig{EntityName: entityName, Enabled: true, Category: audit.CategoryData})
	auditWriter := audit.NewPostgresWriter(contribpgx.NewPoolQuerier(pool))
	pipeline := runtime.NewPipeline(schema, auditWriter)
	repo := contribpgx.NewRepository(pool, es)
	svc := service.NewEntityService(es, repo, pipeline).WithPublisher(outbox.NewWriter(pool))

	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantID})
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	_, err = svc.Create(ctx, map[string]any{}, actor) // missing required "status"
	require.Error(t, err)

	var entityCount, outboxCount int
	require.NoError(t, testdb.QueryRowSQL(t, pool, `SELECT COUNT(*) FROM `+entityName).Scan(&entityCount))
	require.NoError(t, testdb.QueryRowSQL(t, pool, `SELECT COUNT(*) FROM events_outbox`).Scan(&outboxCount))
	assert.Equal(t, 0, entityCount)
	assert.Equal(t, 0, outboxCount, "no workflow intent (or anything else) may exist when the mutation never reached a transaction")
}

// ── 5. Workflow-intent-specific publish failure rolls back everything ────

func TestEntityService_PG_Create_WorkflowIntentPublishFailure_RollsBackMutationAndLifecycleEvent(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, pool)
	testdb.ApplySQL(t, pool, pgEntityDDL(wfEntityName))
	testdb.ApplySQL(t, pool, platformAuditLogTestDDL)
	testdb.ApplySQL(t, pool, readOutboxMigrationSQL(t))

	d := &def.SystemDefinition{
		Name:   wfEntityName,
		Module: "svc",
		Label:  "PGWorkflowTrigger",
		Fields: []def.FieldDef{{Name: "status", Type: def.FieldTypeData}},
		WorkflowTriggers: []def.WorkflowTrigger{
			{On: def.EventOnCreate, WorkflowFn: "test_workflow", TaskQueue: "test.queue",
				InputBuilder: func(*def.EntityRecord, def.TriggerContext) (any, error) { return nil, nil }},
		},
	}
	reg, err := registry.BuildFrom([]def.EntityDefinition{d})
	require.NoError(t, err)
	schema, err := compiler.Compile(reg)
	require.NoError(t, err)
	es := schema.ByName[wfEntityName]
	require.NotNil(t, es)

	if def.Lookup(wfEntityName) == nil {
		def.Register(d)
	}
	audit.Register(audit.EntityAuditConfig{EntityName: wfEntityName, Enabled: true, Category: audit.CategoryData})
	auditWriter := audit.NewPostgresWriter(contribpgx.NewPoolQuerier(pool))
	pipeline := runtime.NewPipeline(schema, auditWriter)
	repo := contribpgx.NewRepository(pool, es)

	realPub := outbox.NewWriter(pool)
	failingPub := &selectiveFailPublisher{
		inner:  realPub,
		failOn: events.EventWorkflowTriggerFired,
		err:    assert.AnError,
	}
	svc := service.NewEntityService(es, repo, pipeline).WithPublisher(failingPub)

	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantID})
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	_, err = svc.Create(ctx, map[string]any{"status": "draft"}, actor)
	require.Error(t, err, "a workflow-intent-specific publish failure must fail the whole Create call")

	var entityCount, auditCount, outboxCount int
	require.NoError(t, testdb.QueryRowSQL(t, pool, `SELECT COUNT(*) FROM `+wfEntityName).Scan(&entityCount))
	require.NoError(t, testdb.QueryRowSQL(t, pool, `SELECT COUNT(*) FROM platform_audit_log`).Scan(&auditCount))
	require.NoError(t, testdb.QueryRowSQL(t, pool, `SELECT COUNT(*) FROM events_outbox`).Scan(&outboxCount))
	assert.Equal(t, 0, entityCount, "the mutation must roll back even though the lifecycle event's own publish would have succeeded")
	assert.Equal(t, 0, auditCount)
	assert.Equal(t, 0, outboxCount, "the lifecycle event, having been published earlier in the SAME transaction, must roll back too — it was never committed")
}

// ── 6/7/8. CreateBatch ─────────────────────────────────────────────────────

func TestEntityService_PG_CreateBatch_WorkflowIntents_PersistedInSameFlushTransaction(t *testing.T) {
	svc, pool := setupWorkflowTriggerService(t, def.EventOnCreate, nil)
	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantID})
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	rows := make([]map[string]any, 10)
	for i := range rows {
		rows[i] = map[string]any{"status": "imported"}
	}
	res, err := svc.CreateBatch(ctx, rows, actor, false)
	require.NoError(t, err)
	require.Len(t, res.Created, 10)

	var lifecycleCount, workflowCount int
	require.NoError(t, testdb.QueryRowSQL(t, pool,
		`SELECT COUNT(*) FROM events_outbox WHERE type = $1`, string(events.EventCreated)).Scan(&lifecycleCount))
	require.NoError(t, testdb.QueryRowSQL(t, pool,
		`SELECT COUNT(*) FROM events_outbox WHERE type = $1`, string(events.EventWorkflowTriggerFired)).Scan(&workflowCount))
	assert.Equal(t, 10, lifecycleCount)
	assert.Equal(t, 10, workflowCount, "each of the 10 imported rows must produce its own workflow intent — not one summary event")
}

func TestEntityService_PG_CreateBatch_Rollback_RemovesWorkflowIntents(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, pool)
	testdb.ApplySQL(t, pool, pgEntityDDL(wfEntityName))
	testdb.ApplySQL(t, pool, platformAuditLogTestDDL)
	testdb.ApplySQL(t, pool, readOutboxMigrationSQL(t))

	d := &def.SystemDefinition{
		Name:   wfEntityName,
		Module: "svc",
		Label:  "PGWorkflowTrigger",
		Fields: []def.FieldDef{{Name: "status", Type: def.FieldTypeData}},
		WorkflowTriggers: []def.WorkflowTrigger{
			{On: def.EventOnCreate, WorkflowFn: "test_workflow", TaskQueue: "test.queue",
				InputBuilder: func(*def.EntityRecord, def.TriggerContext) (any, error) { return nil, nil }},
		},
	}
	reg, err := registry.BuildFrom([]def.EntityDefinition{d})
	require.NoError(t, err)
	schema, err := compiler.Compile(reg)
	require.NoError(t, err)
	es := schema.ByName[wfEntityName]
	require.NotNil(t, es)

	if def.Lookup(wfEntityName) == nil {
		def.Register(d)
	}
	audit.Register(audit.EntityAuditConfig{EntityName: wfEntityName, Enabled: true, Category: audit.CategoryData})
	auditWriter := audit.NewPostgresWriter(contribpgx.NewPoolQuerier(pool))
	pipeline := runtime.NewPipeline(schema, auditWriter)
	repo := contribpgx.NewRepository(pool, es)
	svc := service.NewEntityService(es, repo, pipeline).WithPublisher(outbox.NewWriter(pool))

	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantID})
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	// Force the flush transaction to fail after BulkCreate succeeds, by
	// revoking the audit-table privilege the RunAuditRecord stage needs.
	testdb.ApplySQL(t, pool, `REVOKE INSERT ON platform_audit_log FROM awo_app;`)
	testdb.ActivateTenant(t, pool, tenantID) // ApplySQL reset role — see Step 5's own note on this pitfall

	rows := []map[string]any{{"status": "a"}, {"status": "b"}}
	res, err := svc.CreateBatch(ctx, rows, actor, false)
	require.Error(t, err)
	assert.Nil(t, res.Created)

	var entityCount, workflowCount int
	require.NoError(t, testdb.QueryRowSQL(t, pool, `SELECT COUNT(*) FROM `+wfEntityName).Scan(&entityCount))
	require.NoError(t, testdb.QueryRowSQL(t, pool,
		`SELECT COUNT(*) FROM events_outbox WHERE type = $1`, string(events.EventWorkflowTriggerFired)).Scan(&workflowCount))
	assert.Equal(t, 0, entityCount)
	assert.Equal(t, 0, workflowCount, "no workflow intent may survive a rolled-back flush")
}

func TestEntityService_PG_CreateBatch_LaterFlushFailure_DoesNotCorruptEarlierFlushWorkflowIntents(t *testing.T) {
	svc, pool := setupWorkflowTriggerService(t, def.EventOnCreate, nil)
	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantID})
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	res1, err := svc.CreateBatch(ctx, []map[string]any{{"status": "a"}, {"status": "b"}}, actor, false)
	require.NoError(t, err)
	require.Len(t, res1.Created, 2)

	testdb.ApplySQL(t, pool, `REVOKE INSERT ON events_outbox FROM awo_app;`)
	testdb.ActivateTenant(t, pool, tenantID)
	_, err = svc.CreateBatch(ctx, []map[string]any{{"status": "c"}}, actor, false)
	require.Error(t, err)

	var workflowCount int
	require.NoError(t, testdb.QueryRowSQL(t, pool,
		`SELECT COUNT(*) FROM events_outbox WHERE type = $1`, string(events.EventWorkflowTriggerFired)).Scan(&workflowCount))
	assert.Equal(t, 2, workflowCount, "flush 1's 2 workflow intents must survive flush 2's failure")
}

// ── 9. End-to-end: EntityService → commit → relay → subscriber → Temporal ──

func TestEntityService_PG_Create_WorkflowIntent_DispatchedByRelayAfterCommit(t *testing.T) {
	svc, pool := setupWorkflowTriggerService(t, def.EventOnCreate, nil)
	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantID})
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	created, err := svc.Create(ctx, map[string]any{"status": "draft"}, actor)
	require.NoError(t, err)

	fakeExec := &fakeWorkflowExecutor{}
	relay := outbox.New(pool)
	relay.Subscribe(events.EventWorkflowTriggerFired, outbox.NewWorkflowTriggerSubscriber(fakeExec))
	require.NoError(t, relay.Poll(context.Background()))

	assert.Equal(t, 1, fakeExec.count(), "the relay must dispatch the durable workflow intent to the executor after commit")
	specs := fakeExec.startedSpecs()
	require.Len(t, specs, 1)
	assert.Equal(t, "test_workflow", specs[0].WorkflowFn)
	assert.Equal(t, "test.queue", specs[0].TaskQueue)

	found, deliveredAt, _, _ := workflowOutboxRowState(t, &pgServiceHarness{pool: pool}, created.ID)
	require.True(t, found)
	assert.NotNil(t, deliveredAt, "the workflow-intent event must be marked delivered after a successful dispatch")
}

func TestEntityService_PG_Create_WorkflowIntent_TemporalErrorLeavesEventRetryable(t *testing.T) {
	svc, pool := setupWorkflowTriggerService(t, def.EventOnCreate, nil)
	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantID})
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	created, err := svc.Create(ctx, map[string]any{"status": "draft"}, actor)
	require.NoError(t, err)

	failingExec := &fakeWorkflowExecutor{failWith: assert.AnError}
	relay := outbox.New(pool)
	relay.Subscribe(events.EventWorkflowTriggerFired, outbox.NewWorkflowTriggerSubscriber(failingExec))
	require.NoError(t, relay.Poll(context.Background()))

	found, deliveredAt, deadAt, attempts := workflowOutboxRowState(t, &pgServiceHarness{pool: pool}, created.ID)
	require.True(t, found)
	assert.Nil(t, deliveredAt, "a Temporal dispatch failure must never mark the event delivered")
	assert.Nil(t, deadAt, "a single failure must not immediately dead-letter (MaxAttempts not yet reached)")
	assert.Equal(t, 1, attempts)
}

func TestEntityService_PG_Create_WorkflowIntent_TenantContextRestoredFromEvent_NotAmbient(t *testing.T) {
	svc, pool := setupWorkflowTriggerService(t, def.EventOnCreate, nil)
	tenantA := testdb.CreateTenant(t, pool, "ACTIVE")
	tenantB := testdb.CreateTenant(t, pool, "ACTIVE")

	testdb.ActivateTenant(t, pool, tenantA)
	ctxA := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantA})
	actorA := &def.Actor{UserID: uuid.New(), TenantID: tenantA}
	createdA, err := svc.Create(ctxA, map[string]any{"status": "A"}, actorA)
	require.NoError(t, err)

	testdb.ActivateTenant(t, pool, tenantB)
	ctxB := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantB})
	actorB := &def.Actor{UserID: uuid.New(), TenantID: tenantB}
	createdB, err := svc.Create(ctxB, map[string]any{"status": "B"}, actorB)
	require.NoError(t, err)

	// Ambient tenant context is left as B (the last activation) — the relay
	// must restore each event's OWN tenant from the row, never reuse
	// whatever happens to be ambient across iterations of its delivery loop.
	fakeExec := &fakeWorkflowExecutor{}
	relay := outbox.New(pool)
	relay.Subscribe(events.EventWorkflowTriggerFired, outbox.NewWorkflowTriggerSubscriber(fakeExec))
	require.NoError(t, relay.Poll(context.Background()))

	assert.Equal(t, 2, fakeExec.count())

	row := testdb.QueryRowSQL(t, pool,
		`SELECT tenant_id FROM events_outbox WHERE record_id = $1 AND type = $2`,
		createdA.ID, string(events.EventWorkflowTriggerFired))
	var gotA uuid.UUID
	require.NoError(t, row.Scan(&gotA))
	row = testdb.QueryRowSQL(t, pool,
		`SELECT tenant_id FROM events_outbox WHERE record_id = $1 AND type = $2`,
		createdB.ID, string(events.EventWorkflowTriggerFired))
	var gotB uuid.UUID
	require.NoError(t, row.Scan(&gotB))

	assert.Equal(t, tenantA, gotA)
	assert.Equal(t, tenantB, gotB)
	assert.NotEqual(t, gotA, gotB)
}

// ── Static verification: no direct Temporal call in the mutation path ─────

// assertSourceFree reads filename (relative to this package's own directory
// — `go test` always runs with the package directory as its working
// directory, so no path resolution beyond the bare filename is needed) and
// fails the test if any forbidden substring appears in it. A source-text
// check, not a reflection/AST one, deliberately: it fails loudly and
// specifically if a direct Temporal import or call is ever reintroduced into
// one of EntityService's own files, which is exactly the regression this
// step exists to prevent — independent of whether any single test happens
// to exercise that code path at runtime.
func assertSourceFree(t *testing.T, filename string, forbidden []string) {
	t.Helper()
	src, err := os.ReadFile(filename)
	require.NoErrorf(t, err, "read %s", filename)
	text := string(src)
	for _, needle := range forbidden {
		assert.NotContainsf(t, text, needle,
			"%s must never contain %q — EntityService and its Action* helpers must not call Temporal directly (Phase 2 Step 6)",
			filename, needle)
	}
}

func TestEntityService_SourceNeverCallsTemporalDirectly(t *testing.T) {
	forbidden := []string{"ExecuteWorkflow", "temporalclient", "go.temporal.io/sdk/client"}
	assertSourceFree(t, "entity.go", forbidden)
	assertSourceFree(t, "action_context_factory.go", forbidden)
	assertSourceFree(t, "action_repo.go", forbidden)
}
