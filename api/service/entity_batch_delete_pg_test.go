package service_test

// Bulk-delete safety tests (Awo SDUI — Complete Record Selection and
// Contextual Actions).
//
// api/handler.EntityHandler.DeleteBatch is deliberately NOT a new
// authorization/tenant/lifecycle/transaction mechanism — it composes the
// exact same EntityService.Delete call used by the single-record DELETE
// /:id route, once per ID, inside one outer WithTx. These tests exercise
// that composition directly against real PostgreSQL (not the HTTP layer,
// not mocks) to prove the guarantee the handler relies on: any one ID
// failing for any reason (not found, cross-tenant, lifecycle-restricted)
// rolls back every row in the batch, including ones already processed
// earlier in the same call — true all-or-nothing, not partial deletion.

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/api/service"
	"awo.so/awo/audit"
	"awo.so/awo/compiler"
	contribpgx "awo.so/awo/contrib/pgx"
	"awo.so/awo/def"
	"awo.so/awo/events/outbox"
	"awo.so/awo/registry"
	"awo.so/awo/runtime"
	"awo.so/awo/runtime/tenant"
	testdb "awo.so/awo/testutil/db"
)

// deleteBatch mirrors api/handler.EntityHandler.DeleteBatch's own
// composition exactly: one outer WithTx, one EntityService.Delete call per
// ID, abort (and so roll back) on the first error. Kept here rather than
// importing api/handler to avoid this test package depending on Fiber.
func deleteBatch(ctx context.Context, svc *service.EntityService, ids []uuid.UUID, actor *def.Actor) error {
	return svc.WithTx(ctx, func(txCtx context.Context) error {
		for _, id := range ids {
			if err := svc.Delete(txCtx, id, actor); err != nil {
				return err
			}
		}
		return nil
	})
}

func TestEntityService_PG_DeleteBatch_AllAuthorizedSameTenant_AllDeleted(t *testing.T) {
	h := setupPGService(t, "svc_pg_batchdel_happy", audit.CategoryData)
	tenantID, ctx := pgActivateTenant(t, h.pool)
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	r1, err := h.svc.Create(ctx, map[string]any{"status": "a"}, actor)
	require.NoError(t, err)
	r2, err := h.svc.Create(ctx, map[string]any{"status": "b"}, actor)
	require.NoError(t, err)
	r3, err := h.svc.Create(ctx, map[string]any{"status": "c"}, actor)
	require.NoError(t, err)

	err = deleteBatch(ctx, h.svc, []uuid.UUID{r1.ID, r2.ID, r3.ID}, actor)
	require.NoError(t, err, "a batch of valid, same-tenant, authorized ids must fully succeed")

	for _, id := range []uuid.UUID{r1.ID, r2.ID, r3.ID} {
		assert.Equal(t, 0, entityRowCount(t, h.pool, "svc_pg_batchdel_happy", id), "id %s must be deleted", id)
	}
}

func TestEntityService_PG_DeleteBatch_MissingID_WholeBatchRolledBack(t *testing.T) {
	h := setupPGService(t, "svc_pg_batchdel_missing", audit.CategoryData)
	tenantID, ctx := pgActivateTenant(t, h.pool)
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	r1, err := h.svc.Create(ctx, map[string]any{"status": "a"}, actor)
	require.NoError(t, err)
	r2, err := h.svc.Create(ctx, map[string]any{"status": "b"}, actor)
	require.NoError(t, err)
	missingID := uuid.New() // never created — a real "not found" case.

	err = deleteBatch(ctx, h.svc, []uuid.UUID{r1.ID, missingID, r2.ID}, actor)
	require.Error(t, err, "a batch containing a missing id must fail")

	// r1 was processed (deleted) BEFORE the missing id was hit in the same
	// transaction — it must still exist afterward. This is the actual
	// atomicity proof, not just "the call returned an error".
	assert.Equal(t, 1, entityRowCount(t, h.pool, "svc_pg_batchdel_missing", r1.ID),
		"r1 must still exist — it was deleted earlier in the same rolled-back transaction")
	assert.Equal(t, 1, entityRowCount(t, h.pool, "svc_pg_batchdel_missing", r2.ID),
		"r2 (never reached) must still exist")
}

func TestEntityService_PG_DeleteBatch_CrossTenantID_WholeBatchRolledBack(t *testing.T) {
	h := setupPGService(t, "svc_pg_batchdel_xtenant", audit.CategoryData)

	tenantA := testdb.CreateTenant(t, h.pool, "ACTIVE")
	tenantB := testdb.CreateTenant(t, h.pool, "ACTIVE")
	testdb.ActivateTenant(t, h.pool, tenantA)
	testdb.ActivateTenant(t, h.pool, tenantB)

	ctxA := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantA})
	actorA := &def.Actor{UserID: uuid.New(), TenantID: tenantA}
	ctxB := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantB})
	actorB := &def.Actor{UserID: uuid.New(), TenantID: tenantB}

	ownA, err := h.svc.Create(ctxA, map[string]any{"status": "a"}, actorA)
	require.NoError(t, err)
	ownA2, err := h.svc.Create(ctxA, map[string]any{"status": "a2"}, actorA)
	require.NoError(t, err)
	foreignB, err := h.svc.Create(ctxB, map[string]any{"status": "b"}, actorB)
	require.NoError(t, err)

	// Acting as tenant A, attempt to delete two of tenant A's own records
	// plus tenant B's record. RLS makes Get(foreignB.ID) resolve to "not
	// found" under tenant A's context — the browser never gets a "forbidden
	// because it's someone else's" signal distinguishing it from a typo'd
	// id, which is the correct, safe behavior (no tenant-existence leak).
	err = deleteBatch(ctxA, h.svc, []uuid.UUID{ownA.ID, foreignB.ID, ownA2.ID}, actorA)
	require.Error(t, err, "a batch reaching into another tenant's record must fail")

	assert.Equal(t, 1, entityRowCount(t, h.pool, "svc_pg_batchdel_xtenant", ownA.ID),
		"tenant A's own record (processed before the cross-tenant id) must survive the rollback")
	assert.Equal(t, 1, entityRowCount(t, h.pool, "svc_pg_batchdel_xtenant", ownA2.ID),
		"tenant A's second record (never reached) must survive the rollback")
	assert.Equal(t, 1, entityRowCount(t, h.pool, "svc_pg_batchdel_xtenant", foreignB.ID),
		"tenant B's record must never be deletable from tenant A's batch")
}

// lockedRecordDeleteGuard is a BeforeDeleteHook rejecting deletion of any
// record whose "status" field is "locked" — a stand-in for a real
// lifecycle-restriction hook (e.g. "cannot delete an approved expense"),
// proving DeleteBatch's composition respects whatever BeforeDeleteHook an
// entity declares, generically, without DeleteBatch knowing anything about
// this specific rule.
type lockedRecordDeleteGuard struct{}

func (lockedRecordDeleteGuard) BeforeDelete(_ context.Context, record *def.EntityRecord) error {
	if s, _ := record.Data["status"].(string); s == "locked" {
		return assert.AnError
	}
	return nil
}

func TestEntityService_PG_DeleteBatch_LifecycleRestrictedRecord_WholeBatchRolledBack(t *testing.T) {
	const entityName = "svc_pg_batchdel_lifecycle"
	pool := testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, pool)
	testdb.ApplySQL(t, pool, pgEntityDDL(entityName))
	testdb.ApplySQL(t, pool, platformAuditLogTestDDL)
	testdb.ApplySQL(t, pool, readOutboxMigrationSQL(t))

	d := &def.SystemDefinition{
		Name:   entityName,
		Module: "svc",
		Label:  "PGBatchDelLifecycle",
		Fields: []def.FieldDef{{Name: "status", Type: def.FieldTypeData}},
		Hooks:  def.HookSet{BeforeDelete: []def.BeforeDeleteHook{lockedRecordDeleteGuard{}}},
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

	deletable, err := svc.Create(ctx, map[string]any{"status": "open"}, actor)
	require.NoError(t, err)
	locked, err := svc.Create(ctx, map[string]any{"status": "locked"}, actor)
	require.NoError(t, err)

	err = deleteBatch(ctx, svc, []uuid.UUID{deletable.ID, locked.ID}, actor)
	require.Error(t, err, "a batch containing a lifecycle-restricted record must fail")

	assert.Equal(t, 1, entityRowCount(t, pool, entityName, deletable.ID),
		"the deletable record (processed before the locked one) must survive the rollback")
	assert.Equal(t, 1, entityRowCount(t, pool, entityName, locked.ID),
		"the lifecycle-restricted record must never be deleted")
}
