package service_test

// Phase 2 Step 5 — Bulk import through the canonical mutation pipeline.
//
// These tests exercise EntityService.CreateBatch (the persistence dependency
// ioport.Import now uses exclusively, via the ioport.EntityBatchMutator
// interface) against REAL PostgreSQL — a real contrib/pgx.Repository, a real
// audit.PostgresWriter, and a real events/outbox.OutboxWriter — proving the
// same per-flush mutation+audit+outbox atomicity invariant ADR-025 §14
// requires for bulk-imported rows, not merely single-row Create (already
// covered by entity_pg_atomicity_test.go). Reuses that file's harness
// helpers (setupPGService, pgActivateTenant, readOutboxRow, auditRowCount,
// entityRowCount, pgEntityName/pgAdminEntityName, platformAuditLogTestDDL,
// readOutboxMigrationSQL) — same package, same fixtures, no duplication.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
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

// ── Test A — Successful batch: mutation + audit + outbox all commit, one per row ──

func TestEntityService_PG_CreateBatch_SuccessfulBatch_AllRowsCommitWithOwnAuditAndOutbox(t *testing.T) {
	h := setupPGService(t, pgEntityName, audit.CategoryData)
	tenantID, ctx := pgActivateTenant(t, h.pool)
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	rows := make([]map[string]any, 0, 25)
	for i := 0; i < 25; i++ {
		rows = append(rows, map[string]any{"status": "imported"})
	}

	res, err := h.svc.CreateBatch(ctx, rows, actor, false)
	require.NoError(t, err)
	require.Len(t, res.Created, 25)
	assert.Empty(t, res.Skipped)

	for _, rec := range res.Created {
		assert.Equal(t, 1, entityRowCount(t, h.pool, pgEntityName, rec.ID), "each row must be persisted")
		assert.Equal(t, 1, auditRowCount(t, h.pool, rec.ID), "each row must have exactly one audit record, not a batch summary")

		ob := readOutboxRow(t, h.pool, rec.ID)
		require.True(t, ob.found, "each row must have its own outbox event")
		assert.Equal(t, string(events.EventCreated), ob.eventType)
		assert.Equal(t, tenantID, ob.tenantID)
		assert.Equal(t, actor.UserID, ob.actorID)
	}

	var outboxCount int
	require.NoError(t, testdb.QueryRowSQL(t, h.pool, `SELECT COUNT(*) FROM events_outbox WHERE tenant_id = $1`, tenantID).Scan(&outboxCount))
	assert.Equal(t, 25, outboxCount, "a 25-row batch must produce exactly 25 outbox rows, not one summary event")
}

// ── Test B — Validation failure ──────────────────────────────────────────────

func TestEntityService_PG_CreateBatch_ValidationFailure_SkipInvalidFalse_AbortsWithNoRowsPersisted(t *testing.T) {
	const entityName = "svc_pg_batch_required_entity"
	pool := testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, pool)
	testdb.ApplySQL(t, pool, pgEntityDDL(entityName))
	testdb.ApplySQL(t, pool, platformAuditLogTestDDL)
	testdb.ApplySQL(t, pool, readOutboxMigrationSQL(t))

	d := &def.SystemDefinition{
		Name:   entityName,
		Module: "svc",
		Label:  "PGBatchRequired",
		Fields: []def.FieldDef{{Name: "status", Type: def.FieldTypeData, Required: true}},
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
	svc := service.NewEntityService(es, repo, pipeline, nil).WithPublisher(outbox.NewWriter(pool))

	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantID})
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	rows := []map[string]any{
		{"status": "ok-1"},
		{}, // missing required "status" — fails RunBeforeCreate
		{"status": "ok-2"},
	}

	res, err := svc.CreateBatch(ctx, rows, actor, false)
	require.Error(t, err, "a validation failure with skipInvalid=false must abort the whole call")
	require.Nil(t, res, "no partial result may be returned when the call aborts before any transaction opens")

	var entityCount, auditCount, outboxCount int
	require.NoError(t, testdb.QueryRowSQL(t, pool, `SELECT COUNT(*) FROM `+entityName).Scan(&entityCount))
	require.NoError(t, testdb.QueryRowSQL(t, pool, `SELECT COUNT(*) FROM platform_audit_log`).Scan(&auditCount))
	require.NoError(t, testdb.QueryRowSQL(t, pool, `SELECT COUNT(*) FROM events_outbox`).Scan(&outboxCount))
	assert.Equal(t, 0, entityCount, "no row may be persisted — not even the two valid ones — when skipInvalid is false")
	assert.Equal(t, 0, auditCount)
	assert.Equal(t, 0, outboxCount)
}

func TestEntityService_PG_CreateBatch_ValidationFailure_SkipInvalidTrue_ValidRowsStillCommit(t *testing.T) {
	const entityName = "svc_pg_batch_required_skip_entity"
	pool := testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, pool)
	testdb.ApplySQL(t, pool, pgEntityDDL(entityName))
	testdb.ApplySQL(t, pool, platformAuditLogTestDDL)
	testdb.ApplySQL(t, pool, readOutboxMigrationSQL(t))

	d := &def.SystemDefinition{
		Name:   entityName,
		Module: "svc",
		Label:  "PGBatchRequiredSkip",
		Fields: []def.FieldDef{{Name: "status", Type: def.FieldTypeData, Required: true}},
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
	svc := service.NewEntityService(es, repo, pipeline, nil).WithPublisher(outbox.NewWriter(pool))

	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantID})
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	rows := []map[string]any{
		{"status": "ok-1"},
		{}, // invalid — missing required field
		{"status": "ok-2"},
	}

	res, err := svc.CreateBatch(ctx, rows, actor, true)
	require.NoError(t, err, "with skipInvalid=true the call must not fail just because one row was invalid")
	require.Len(t, res.Created, 2, "the two valid rows must still commit")
	require.Len(t, res.Skipped, 1)
	assert.Equal(t, 1, res.Skipped[0].Index, "the skipped row must be attributed to its 0-based input index")

	var entityCount int
	require.NoError(t, testdb.QueryRowSQL(t, pool, `SELECT COUNT(*) FROM `+entityName).Scan(&entityCount))
	assert.Equal(t, 2, entityCount, "exactly the 2 valid rows must be persisted — the invalid row never reached the database")
	for _, rec := range res.Created {
		assert.Equal(t, 1, auditRowCount(t, pool, rec.ID))
		ob := readOutboxRow(t, pool, rec.ID)
		assert.True(t, ob.found)
	}
}

// ── Test C — Hook failure rolls back the whole flush ─────────────────────────

func TestEntityService_PG_CreateBatch_AfterCreateHookFailure_RollsBackWholeFlush(t *testing.T) {
	const entityName = "svc_pg_batch_hookfail_entity"
	pool := testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, pool)
	testdb.ApplySQL(t, pool, pgEntityDDL(entityName))
	testdb.ApplySQL(t, pool, platformAuditLogTestDDL)
	testdb.ApplySQL(t, pool, readOutboxMigrationSQL(t))

	failingHook := &failingAfterCreateHook{}
	d := &def.SystemDefinition{
		Name:   entityName,
		Module: "svc",
		Label:  "PGBatchHookFail",
		Fields: []def.FieldDef{{Name: "status", Type: def.FieldTypeData}},
		Hooks:  def.HookSet{AfterCreate: []def.AfterCreateHook{failingHook}},
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
	svc := service.NewEntityService(es, repo, pipeline, nil).WithPublisher(outbox.NewWriter(pool))

	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantID})
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	rows := []map[string]any{{"status": "a"}, {"status": "b"}, {"status": "c"}}
	res, err := svc.CreateBatch(ctx, rows, actor, false)
	require.Error(t, err, "a failing AfterCreate hook anywhere in the flush must fail CreateBatch")
	assert.Nil(t, res.Created)

	var entityCount, auditCount, outboxCount int
	require.NoError(t, testdb.QueryRowSQL(t, pool, `SELECT COUNT(*) FROM `+entityName).Scan(&entityCount))
	require.NoError(t, testdb.QueryRowSQL(t, pool, `SELECT COUNT(*) FROM platform_audit_log`).Scan(&auditCount))
	require.NoError(t, testdb.QueryRowSQL(t, pool, `SELECT COUNT(*) FROM events_outbox`).Scan(&outboxCount))
	assert.Equal(t, 0, entityCount, "none of the 3 rows may survive — the whole flush transaction rolled back")
	assert.Equal(t, 0, auditCount)
	assert.Equal(t, 0, outboxCount)
}

// ── Test D — Audit failure (ADMIN category) rolls back the whole flush ───────

func TestEntityService_PG_CreateBatch_AdminAuditFailure_RollsBackWholeFlush(t *testing.T) {
	h := setupPGService(t, pgAdminEntityName, audit.CategoryAdmin)
	testdb.ApplySQL(t, h.pool, `REVOKE INSERT ON platform_audit_log FROM awo_app;`)

	tenantID, ctx := pgActivateTenant(t, h.pool)
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	rows := []map[string]any{{"status": "a"}, {"status": "b"}}
	res, err := h.svc.CreateBatch(ctx, rows, actor, false)
	require.Error(t, err, "CategoryAdmin audit failure must propagate (ADR-017), failing the whole flush")
	assert.Nil(t, res.Created)

	var entityCount, outboxCount int
	require.NoError(t, testdb.QueryRowSQL(t, h.pool, `SELECT COUNT(*) FROM `+pgAdminEntityName).Scan(&entityCount))
	require.NoError(t, testdb.QueryRowSQL(t, h.pool, `SELECT COUNT(*) FROM events_outbox`).Scan(&outboxCount))
	assert.Equal(t, 0, entityCount)
	assert.Equal(t, 0, outboxCount)
}

// ── Test E — Outbox failure rolls back the whole flush (unconditional) ───────

func TestEntityService_PG_CreateBatch_OutboxFailure_RollsBackWholeFlush(t *testing.T) {
	h := setupPGService(t, pgEntityName, audit.CategoryData)
	testdb.ApplySQL(t, h.pool, `REVOKE INSERT ON events_outbox FROM awo_app;`)

	tenantID, ctx := pgActivateTenant(t, h.pool)
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	rows := []map[string]any{{"status": "a"}, {"status": "b"}}
	res, err := h.svc.CreateBatch(ctx, rows, actor, false)
	require.Error(t, err, "an outbox insert failure must always propagate (ADR-025 §6), failing the whole flush")
	assert.Nil(t, res.Created)

	var entityCount, auditCount int
	require.NoError(t, testdb.QueryRowSQL(t, h.pool, `SELECT COUNT(*) FROM `+pgEntityName).Scan(&entityCount))
	require.NoError(t, testdb.QueryRowSQL(t, h.pool, `SELECT COUNT(*) FROM platform_audit_log`).Scan(&auditCount))
	assert.Equal(t, 0, entityCount)
	assert.Equal(t, 0, auditCount)
}

// ── Test F — Real database failure (unique violation) reports no false success ─

func TestEntityService_PG_CreateBatch_UniqueViolation_NoFalseSuccessReporting(t *testing.T) {
	const entityName = "svc_pg_batch_unique_entity"
	pool := testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, pool)
	testdb.ApplySQL(t, pool, `
CREATE TABLE `+entityName+` (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     uuid NOT NULL,
    status        text,
    code          text UNIQUE,
    custom_fields jsonb,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE `+entityName+` ENABLE ROW LEVEL SECURITY;
ALTER TABLE `+entityName+` FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON `+entityName+` USING (tenant_id = current_tenant_id());
GRANT SELECT, INSERT, UPDATE, DELETE ON `+entityName+` TO awo_app;
`)
	testdb.ApplySQL(t, pool, platformAuditLogTestDDL)
	testdb.ApplySQL(t, pool, readOutboxMigrationSQL(t))

	d := &def.SystemDefinition{
		Name:   entityName,
		Module: "svc",
		Label:  "PGBatchUnique",
		Fields: []def.FieldDef{{Name: "status", Type: def.FieldTypeData}, {Name: "code", Type: def.FieldTypeData}},
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
	svc := service.NewEntityService(es, repo, pipeline, nil).WithPublisher(outbox.NewWriter(pool))

	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantID})
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	rows := []map[string]any{
		{"status": "a", "code": "DUP"},
		{"status": "b", "code": "DUP"}, // real unique-constraint violation
	}
	res, err := svc.CreateBatch(ctx, rows, actor, false)
	require.Error(t, err, "a real unique-constraint violation must surface as an error, not a false success")
	assert.Nil(t, res.Created)

	var entityCount int
	require.NoError(t, testdb.QueryRowSQL(t, pool, `SELECT COUNT(*) FROM `+entityName).Scan(&entityCount))
	assert.Equal(t, 0, entityCount, "the first (otherwise-valid) row must not remain committed when its sibling in the same flush violates a constraint")
}

// ── Test G — Batch boundary: a later flush's failure does not corrupt an earlier one ──

func TestEntityService_PG_CreateBatch_LaterFlushFailure_DoesNotCorruptEarlierFlush(t *testing.T) {
	h := setupPGService(t, pgEntityName, audit.CategoryData)
	tenantID, ctx := pgActivateTenant(t, h.pool)
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	// Flush 1 succeeds.
	res1, err := h.svc.CreateBatch(ctx, []map[string]any{{"status": "batch1-a"}, {"status": "batch1-b"}}, actor, false)
	require.NoError(t, err)
	require.Len(t, res1.Created, 2)

	// Flush 2 fails (revoke outbox insert to force a real failure).
	// ApplySQL resets the session to the superuser role to run the REVOKE
	// DDL (required — a non-superuser role cannot revoke its own grant) and
	// does not switch back; re-activate the tenant/AppRole context
	// afterward so the second CreateBatch call still runs as the
	// (now-revoked) awo_app role instead of silently bypassing the revoke
	// as superuser.
	testdb.ApplySQL(t, h.pool, `REVOKE INSERT ON events_outbox FROM awo_app;`)
	testdb.ActivateTenant(t, h.pool, tenantID)
	_, err = h.svc.CreateBatch(ctx, []map[string]any{{"status": "batch2-a"}}, actor, false)
	require.Error(t, err)

	// Flush 1's rows must remain committed and untouched.
	for _, rec := range res1.Created {
		assert.Equal(t, 1, entityRowCount(t, h.pool, pgEntityName, rec.ID), "flush 1's rows must survive flush 2's failure")
		assert.Equal(t, 1, auditRowCount(t, h.pool, rec.ID))
		ob := readOutboxRow(t, h.pool, rec.ID)
		assert.True(t, ob.found)
	}

	// Flush 2 produced nothing.
	var batch2Count int
	require.NoError(t, testdb.QueryRowSQL(t, h.pool, `SELECT COUNT(*) FROM `+pgEntityName+` WHERE status = 'batch2-a'`).Scan(&batch2Count))
	assert.Equal(t, 0, batch2Count)
}

// ── Tenant isolation (two real tenants) ───────────────────────────────────────

func TestEntityService_PG_CreateBatch_TenantIsolation_TwoTenants(t *testing.T) {
	h := setupPGService(t, pgEntityName, audit.CategoryData)

	tenantA := testdb.CreateTenant(t, h.pool, "ACTIVE")
	tenantB := testdb.CreateTenant(t, h.pool, "ACTIVE")

	testdb.ActivateTenant(t, h.pool, tenantA)
	ctxA := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantA})
	actorA := &def.Actor{UserID: uuid.New(), TenantID: tenantA}
	resA, err := h.svc.CreateBatch(ctxA, []map[string]any{{"status": "A-1"}, {"status": "A-2"}}, actorA, false)
	require.NoError(t, err)
	require.Len(t, resA.Created, 2)

	// Every row in A's batch must be attributed to tenant A, never B.
	for _, rec := range resA.Created {
		ob := readOutboxRow(t, h.pool, rec.ID)
		require.True(t, ob.found)
		assert.Equal(t, tenantA, ob.tenantID)
		assert.NotEqual(t, tenantB, ob.tenantID)
	}

	// Tenant B must not see tenant A's rows via RLS.
	testdb.ActivateTenant(t, h.pool, tenantB)
	ctxB := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantB})
	seenByB, _, err := h.svc.Query(ctxB, nil)
	require.NoError(t, err)
	for _, rec := range seenByB {
		for _, a := range resA.Created {
			assert.NotEqual(t, a.ID, rec.ID, "tenant B must never see tenant A's batch-imported rows via RLS")
		}
	}
}

func TestEntityService_PG_CreateBatch_RowDataCannotOverrideTenantID(t *testing.T) {
	h := setupPGService(t, pgEntityName, audit.CategoryData)

	tenantA := testdb.CreateTenant(t, h.pool, "ACTIVE")
	tenantB := testdb.CreateTenant(t, h.pool, "ACTIVE")
	testdb.ActivateTenant(t, h.pool, tenantA)

	ctxA := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantA})
	actorA := &def.Actor{UserID: uuid.New(), TenantID: tenantA}

	// A row attempts to smuggle a foreign tenant_id via its own data map.
	// "tenant_id" is not a declared field on this entity, so csv/json-level
	// mapping already can't reach it — this proves the same at the
	// EntityService.CreateBatch/driver.CreateInput boundary directly: the
	// persisted tenant_id must come only from ctx, never from row data.
	rows := []map[string]any{{"status": "attempt", "tenant_id": tenantB.String()}}
	res, err := h.svc.CreateBatch(ctxA, rows, actorA, false)
	require.NoError(t, err)
	require.Len(t, res.Created, 1)

	ob := readOutboxRow(t, h.pool, res.Created[0].ID)
	require.True(t, ob.found)
	assert.Equal(t, tenantA, ob.tenantID, "the row must be attributed to the context tenant (A), never a tenant_id smuggled through row data")
}

// ── Sensitive field redaction, per row ────────────────────────────────────────

func TestEntityService_PG_CreateBatch_SensitiveField_RedactedInEveryRowsOutboxPayload(t *testing.T) {
	h := setupPGService(t, pgEntityName, audit.CategoryData)
	tenantID, ctx := pgActivateTenant(t, h.pool)
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	const secret1, secret2 = "111-11-1111", "222-22-2222"
	rows := []map[string]any{
		{"status": "a", "ssn": secret1},
		{"status": "b", "ssn": secret2},
	}
	res, err := h.svc.CreateBatch(ctx, rows, actor, false)
	require.NoError(t, err)
	require.Len(t, res.Created, 2)

	for i, rec := range res.Created {
		ob := readOutboxRow(t, h.pool, rec.ID)
		require.True(t, ob.found)
		var decoded map[string]any
		require.NoError(t, json.Unmarshal(ob.payload, &decoded))
		assert.Equal(t, "[REDACTED]", decoded["ssn"], "row %d: the Sensitive field must be redacted", i)
		assert.NotContains(t, string(ob.payload), secret1)
		assert.NotContains(t, string(ob.payload), secret2)
	}
}

// ── Correlation ID preserved per row in a batch ───────────────────────────────

func TestEntityService_PG_CreateBatch_CorrelationID_PreservedForEveryRow(t *testing.T) {
	h := setupPGService(t, pgEntityName, audit.CategoryData)
	tenantID, ctx := pgActivateTenant(t, h.pool)
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	const requestID = "import-job-req-42"
	ctx = audit.WithRequestContext(ctx, audit.RequestContext{RequestID: requestID})

	res, err := h.svc.CreateBatch(ctx, []map[string]any{{"status": "a"}, {"status": "b"}}, actor, false)
	require.NoError(t, err)
	require.Len(t, res.Created, 2)

	for _, rec := range res.Created {
		ob := readOutboxRow(t, h.pool, rec.ID)
		require.True(t, ob.found)
		require.NotNil(t, ob.correlationID)
		assert.Equal(t, requestID, *ob.correlationID)
		assert.Equal(t, actor.UserID, ob.actorID)
	}
}

// ── Empty batch is a safe no-op ───────────────────────────────────────────────

func TestEntityService_PG_CreateBatch_EmptyRows_NoOp(t *testing.T) {
	h := setupPGService(t, pgEntityName, audit.CategoryData)
	tenantID, ctx := pgActivateTenant(t, h.pool)
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	res, err := h.svc.CreateBatch(ctx, nil, actor, false)
	require.NoError(t, err)
	assert.Empty(t, res.Created)
	assert.Empty(t, res.Skipped)
}
