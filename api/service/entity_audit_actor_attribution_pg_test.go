package service_test

// Phase 2 Step 7b — audit-record actor attribution.
//
// runtime.Pipeline.RunAuditRecord reads record.Meta.Actor to attribute the
// persisted platform_audit_log row. contrib/pgx.Repository.Create/Update
// return a freshly-scanned *def.EntityRecord with no Meta populated at all,
// and Repository.Get (what EntityService.Delete's pre-deletion snapshot
// comes from) never populates it either — so, before this fix,
// platform_audit_log.actor_id/service_account_id were NULL for every real
// (non-test-stub) mutation, even though the correct actor was known
// throughout: authenticated by HTTP middleware, passed explicitly into
// EntityService.Create/Update/Delete/CreateBatch, and already correctly
// used by the durable outbox lifecycle/workflow-intent events (Steps 3–6),
// which take actor as an explicit parameter rather than reading
// record.Meta.Actor. Only the audit pipeline was affected.
//
// These tests assert directly against the actual persisted
// platform_audit_log row's actor_id/service_account_id columns — a record
// with an empty (but non-panicking) Meta.Actor read would still pass a test
// that merely checks "no error", which is why every test here reads the
// database, not the in-memory return value.

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/audit"
	"awo.so/awo/def"
	testdb "awo.so/awo/testutil/db"
)

// ── Test 1 — Create actor attribution ────────────────────────────────────

func TestEntityService_PG_Create_AuditRow_ActorAttributionCorrect(t *testing.T) {
	h := setupPGService(t, pgEntityName, audit.CategoryData)
	tenantID, ctx := pgActivateTenant(t, h.pool)
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	created, err := h.svc.Create(ctx, map[string]any{"status": "draft"}, actor)
	require.NoError(t, err)

	var gotTenant, gotRecord uuid.UUID
	var gotEntity, gotOperation string
	var gotActorID *uuid.UUID
	var gotServiceAccountID *uuid.UUID
	row := testdb.QueryRowSQL(t, h.pool, `
		SELECT tenant_id, entity_name, record_id, operation, actor_id, service_account_id
		FROM platform_audit_log WHERE record_id = $1
	`, created.ID)
	require.NoError(t, row.Scan(&gotTenant, &gotEntity, &gotRecord, &gotOperation, &gotActorID, &gotServiceAccountID))

	require.NotNil(t, gotActorID, "platform_audit_log.actor_id must not be NULL for a real, actor-driven Create — this is the exact defect Step 7b fixes")
	assert.Equal(t, actor.UserID, *gotActorID)
	assert.Nil(t, gotServiceAccountID, "a human-user actor must never also populate service_account_id")
	assert.Equal(t, tenantID, gotTenant)
	assert.Equal(t, pgEntityName, gotEntity)
	assert.Equal(t, created.ID, gotRecord)
	assert.Equal(t, "create", gotOperation)
}

// ── Test 2 — Update actor attribution ────────────────────────────────────

func TestEntityService_PG_Update_AuditRow_ActorAttributionCorrect(t *testing.T) {
	h := setupPGService(t, pgEntityName, audit.CategoryData)
	tenantID, ctx := pgActivateTenant(t, h.pool)
	creator := &def.Actor{UserID: uuid.New(), TenantID: tenantID}
	updater := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	created, err := h.svc.Create(ctx, map[string]any{"status": "draft"}, creator)
	require.NoError(t, err)

	updated, err := h.svc.Update(ctx, created.ID, map[string]any{"status": "final"}, updater)
	require.NoError(t, err)
	assert.Equal(t, "final", updated.Data["status"])

	var gotTenant, gotRecord uuid.UUID
	var gotEntity, gotOperation string
	var gotActorID *uuid.UUID
	var beforeJSON, afterJSON []byte
	row := testdb.QueryRowSQL(t, h.pool, `
		SELECT tenant_id, entity_name, record_id, operation, actor_id, before_data, after_data
		FROM platform_audit_log WHERE record_id = $1 AND operation = 'update'
	`, created.ID)
	require.NoError(t, row.Scan(&gotTenant, &gotEntity, &gotRecord, &gotOperation, &gotActorID, &beforeJSON, &afterJSON))

	require.NotNil(t, gotActorID, "the UPDATE audit row's actor_id must not be NULL")
	assert.Equal(t, updater.UserID, *gotActorID, "the UPDATE audit row must attribute the actor who performed the UPDATE, not the original Create actor")
	assert.Equal(t, tenantID, gotTenant)
	assert.Equal(t, pgEntityName, gotEntity)
	assert.Equal(t, created.ID, gotRecord)
	assert.Contains(t, string(beforeJSON), "draft", "before_data must still reflect the pre-update state")
	assert.Contains(t, string(afterJSON), "final", "after_data must still reflect the post-update state")
}

// ── Test 3 — Delete actor attribution ────────────────────────────────────

func TestEntityService_PG_Delete_AuditRow_ActorAttributionCorrect(t *testing.T) {
	h := setupPGService(t, pgEntityName, audit.CategoryData)
	tenantID, ctx := pgActivateTenant(t, h.pool)
	creator := &def.Actor{UserID: uuid.New(), TenantID: tenantID}
	deleter := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	created, err := h.svc.Create(ctx, map[string]any{"status": "draft"}, creator)
	require.NoError(t, err)

	require.NoError(t, h.svc.Delete(ctx, created.ID, deleter))

	var gotTenant, gotRecord uuid.UUID
	var gotEntity, gotOperation string
	var gotActorID *uuid.UUID
	var beforeJSON []byte
	var afterJSON []byte
	row := testdb.QueryRowSQL(t, h.pool, `
		SELECT tenant_id, entity_name, record_id, operation, actor_id, before_data, after_data
		FROM platform_audit_log WHERE record_id = $1 AND operation = 'delete'
	`, created.ID)
	require.NoError(t, row.Scan(&gotTenant, &gotEntity, &gotRecord, &gotOperation, &gotActorID, &beforeJSON, &afterJSON))

	require.NotNil(t, gotActorID, "the DELETE audit row's actor_id must not be NULL")
	assert.Equal(t, deleter.UserID, *gotActorID, "the DELETE audit row must attribute the actor who performed the delete, not the original creator")
	assert.Equal(t, tenantID, gotTenant)
	assert.Equal(t, pgEntityName, gotEntity)
	assert.Equal(t, created.ID, gotRecord)
	assert.Contains(t, string(beforeJSON), "draft", "before_data must still reflect the pre-deletion state")
	assert.Empty(t, afterJSON, "after_data must remain empty for a delete — unchanged semantics")
}

// ── Test 4 — Service-account attribution ─────────────────────────────────
//
// def.Actor already models a service-account principal via ServiceAccountID
// (mutually exclusive with UserID — ADR-003/ADR-015; audit.PostgresWriter.Write
// already branches on which field is set, populating actor_id XOR
// service_account_id). There is no separate service-account *mutation* path
// in the codebase (EntityService.Create/Update/Delete take a *def.Actor
// regardless of whether it represents a human user or a service account) —
// so this test exercises that same, existing, canonical Actor
// representation with ServiceAccountID set instead of UserID, rather than
// inventing a new path.

func TestEntityService_PG_Create_AuditRow_ServiceAccountAttributionCorrect(t *testing.T) {
	h := setupPGService(t, pgEntityName, audit.CategoryData)
	tenantID, ctx := pgActivateTenant(t, h.pool)
	svcActor := &def.Actor{ServiceAccountID: uuid.New(), TenantID: tenantID}

	created, err := h.svc.Create(ctx, map[string]any{"status": "draft"}, svcActor)
	require.NoError(t, err)

	var gotActorID, gotServiceAccountID *uuid.UUID
	row := testdb.QueryRowSQL(t, h.pool, `
		SELECT actor_id, service_account_id FROM platform_audit_log WHERE record_id = $1
	`, created.ID)
	require.NoError(t, row.Scan(&gotActorID, &gotServiceAccountID))

	require.NotNil(t, gotServiceAccountID, "platform_audit_log.service_account_id must be populated for a service-account actor")
	assert.Equal(t, svcActor.ServiceAccountID, *gotServiceAccountID)
	assert.Nil(t, gotActorID, "a service-account actor must never also populate actor_id")
}

// ── Test 5 — Tenant isolation of actor attribution ───────────────────────

func TestEntityService_PG_Create_AuditRow_ActorAttributionDoesNotCrossTenants(t *testing.T) {
	h := setupPGService(t, pgEntityName, audit.CategoryData)

	tenantA := testdb.CreateTenant(t, h.pool, "ACTIVE")
	tenantB := testdb.CreateTenant(t, h.pool, "ACTIVE")

	testdb.ActivateTenant(t, h.pool, tenantA)
	ctxA := testdb.WithTenant(context.Background(), tenantA)
	actorA := &def.Actor{UserID: uuid.New(), TenantID: tenantA}
	createdA, err := h.svc.Create(ctxA, map[string]any{"status": "A"}, actorA)
	require.NoError(t, err)

	testdb.ActivateTenant(t, h.pool, tenantB)
	ctxB := testdb.WithTenant(context.Background(), tenantB)
	actorB := &def.Actor{UserID: uuid.New(), TenantID: tenantB}
	createdB, err := h.svc.Create(ctxB, map[string]any{"status": "B"}, actorB)
	require.NoError(t, err)

	var tA uuid.UUID
	var aA *uuid.UUID
	rowA := testdb.QueryRowSQL(t, h.pool, `SELECT tenant_id, actor_id FROM platform_audit_log WHERE record_id = $1`, createdA.ID)
	require.NoError(t, rowA.Scan(&tA, &aA))
	require.NotNil(t, aA)
	assert.Equal(t, tenantA, tA)
	assert.Equal(t, actorA.UserID, *aA)
	assert.NotEqual(t, actorB.UserID, *aA, "tenant A's audit row must never be attributed to tenant B's actor")

	var tB uuid.UUID
	var aB *uuid.UUID
	rowB := testdb.QueryRowSQL(t, h.pool, `SELECT tenant_id, actor_id FROM platform_audit_log WHERE record_id = $1`, createdB.ID)
	require.NoError(t, rowB.Scan(&tB, &aB))
	require.NotNil(t, aB)
	assert.Equal(t, tenantB, tB)
	assert.Equal(t, actorB.UserID, *aB)
	assert.NotEqual(t, actorA.UserID, *aB, "tenant B's audit row must never be attributed to tenant A's actor")
}

// ── CreateBatch — same defect, same fix, verified directly ──────────────
//
// EntityService.CreateBatch (Phase 2 Step 5) has the identical Meta.Actor
// gap as Create: contrib/pgx.Repository.BulkCreate's returned records have
// no Meta populated either. Fixed by the same one-line assignment in
// CreateBatch's per-row loop — verified here directly rather than left
// merely inferred from Create's own coverage.

func TestEntityService_PG_CreateBatch_AuditRows_ActorAttributionCorrect(t *testing.T) {
	h := setupPGService(t, pgEntityName, audit.CategoryData)
	tenantID, ctx := pgActivateTenant(t, h.pool)
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	res, err := h.svc.CreateBatch(ctx, []map[string]any{
		{"status": "batch-a"},
		{"status": "batch-b"},
	}, actor, false)
	require.NoError(t, err)
	require.Len(t, res.Created, 2)

	for _, rec := range res.Created {
		var gotActorID *uuid.UUID
		row := testdb.QueryRowSQL(t, h.pool, `SELECT actor_id FROM platform_audit_log WHERE record_id = $1`, rec.ID)
		require.NoError(t, row.Scan(&gotActorID))
		require.NotNil(t, gotActorID, "each CreateBatch row's audit record must attribute the batch's actor")
		assert.Equal(t, actor.UserID, *gotActorID)
	}
}

// ── Test 6 — Outbox actor attribution remains unchanged ──────────────────
//
// The Step 3–6 durable outbox lifecycle/workflow-intent events already
// attributed the correct actor by taking it as an explicit parameter
// (never reading record.Meta.Actor) — this Step 7b fix must not alter or
// replace that mechanism. This test exists to catch an accidental
// regression from an incorrect attempt to unify the two.

func TestEntityService_PG_Create_OutboxActorAttribution_UnaffectedByAuditFix(t *testing.T) {
	h := setupPGService(t, pgEntityName, audit.CategoryData)
	tenantID, ctx := pgActivateTenant(t, h.pool)
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	created, err := h.svc.Create(ctx, map[string]any{"status": "draft"}, actor)
	require.NoError(t, err)

	ob := readOutboxRow(t, h.pool, created.ID)
	require.True(t, ob.found)
	assert.Equal(t, actor.UserID, ob.actorID, "the outbox lifecycle event's actor attribution (Step 3's own explicit-actor-parameter mechanism) must be unchanged by the Step 7b audit fix")
}
