package service_test

// Phase 2 Step 7a — EntityService.Update/Delete's initial standalone read
// (s.repo.Get(ctx, id), called before either method opens its own mutation
// transaction) must succeed under the REAL, transaction-scoped production
// set_tenant_context() — not just testdb.SetupTestDB's own simpler,
// session-persistent pass-through — and must not depend on a single-
// connection pool happening to mask the underlying contrib/pgx.Repository
// fix (see contrib/pgx/standalone_read_tenant_context_test.go for the
// repository-level tests and full root-cause explanation).
//
// This does not change EntityService.Update/Delete themselves (no code in
// api/service was touched for Step 7a) — it proves the contrib/pgx fix is
// sufficient for the one production call pattern (an initial Get before a
// mutation transaction) that originally surfaced this defect.

import (
	"context"
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
	"awo.so/awo/events/outbox"
	"awo.so/awo/registry"
	"awo.so/awo/runtime"
	"awo.so/awo/runtime/tenant"
	testdb "awo.so/awo/testutil/db"
)

// setupMultiConnPGService mirrors setupPGService (same DDL, same pipeline
// wiring) but returns an EntityService built on a genuine multi-connection
// pool (testdb.OpenConcurrentPool) instead of SetupTestDB's own
// pool_max_conns=1 pool — so Update/Delete's initial Get has no single-
// connection affinity to accidentally rely on. setupPool (the original,
// pool_max_conns=1 pool) is also returned — CreateTenant/ActivateTenant use
// it, matching every other test's convention.
func setupMultiConnPGService(t *testing.T, entityName string) (svc *service.EntityService, setupPool *pgxpool.Pool) {
	t.Helper()

	setupPool = testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, setupPool)
	testdb.ApplySQL(t, setupPool, pgEntityDDL(entityName))
	testdb.ApplySQL(t, setupPool, platformAuditLogTestDDL)
	testdb.ApplySQL(t, setupPool, readOutboxMigrationSQL(t))
	concPool := testdb.OpenConcurrentPool(t, setupPool)

	d := pgEntityDef(entityName)
	reg, err := registry.BuildFrom([]def.EntityDefinition{d})
	require.NoError(t, err)
	schema, err := compiler.Compile(reg)
	require.NoError(t, err)
	es := schema.ByName[entityName]
	require.NotNil(t, es)

	if def.Lookup(entityName) == nil {
		def.Register(d)
	}
	audit.Register(audit.EntityAuditConfig{EntityName: entityName, Enabled: true, Category: audit.CategoryData})

	auditWriter := audit.NewPostgresWriter(contribpgx.NewPoolQuerier(concPool))
	pipeline := runtime.NewPipeline(schema, auditWriter)
	repo := contribpgx.NewRepository(concPool, es)

	svc = service.NewEntityService(es, repo, pipeline).WithPublisher(outbox.NewWriter(concPool))
	return svc, setupPool
}

func TestEntityService_PG_Update_MultiConnectionPool_RealLifecycleFunction_InitialGetSucceeds(t *testing.T) {
	svc, setupPool := setupMultiConnPGService(t, "svc_pg_step7a_update_entity")

	tenantID := testdb.CreateTenant(t, setupPool, "ACTIVE")
	testdb.ActivateTenant(t, setupPool, tenantID)
	ctx := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantID})
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	created, err := svc.Create(ctx, map[string]any{"status": "draft"}, actor)
	require.NoError(t, err)

	// The bug: Update's opening s.repo.Get(ctx, id), standalone, under the
	// real (transaction-scoped) set_tenant_context and a multi-connection
	// pool, previously returned NotFoundError here for a record that
	// unquestionably exists and belongs to the caller's own tenant.
	updated, err := svc.Update(ctx, created.ID, map[string]any{"status": "final"}, actor)
	require.NoError(t, err, "Update's initial standalone Get must find the caller's own record")
	assert.Equal(t, "final", updated.Data["status"])
}

func TestEntityService_PG_Delete_MultiConnectionPool_RealLifecycleFunction_InitialGetSucceeds(t *testing.T) {
	svc, setupPool := setupMultiConnPGService(t, "svc_pg_step7a_delete_entity")

	tenantID := testdb.CreateTenant(t, setupPool, "ACTIVE")
	testdb.ActivateTenant(t, setupPool, tenantID)
	ctx := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantID})
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	created, err := svc.Create(ctx, map[string]any{"status": "draft"}, actor)
	require.NoError(t, err)

	err = svc.Delete(ctx, created.ID, actor)
	require.NoError(t, err, "Delete's initial standalone Get must find the caller's own record")

	_, err = svc.Get(ctx, created.ID)
	assert.True(t, runtime.IsNotFound(err), "the record must genuinely be gone after Delete")
}

func TestEntityService_PG_Update_MultiConnectionPool_TenantBCannotUpdateTenantARecord(t *testing.T) {
	svc, setupPool := setupMultiConnPGService(t, "svc_pg_step7a_update_isolation_entity")

	tenantA := testdb.CreateTenant(t, setupPool, "ACTIVE")
	tenantB := testdb.CreateTenant(t, setupPool, "ACTIVE")

	testdb.ActivateTenant(t, setupPool, tenantA)
	ctxA := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantA})
	actorA := &def.Actor{UserID: uuid.New(), TenantID: tenantA}
	created, err := svc.Create(ctxA, map[string]any{"status": "draft"}, actorA)
	require.NoError(t, err)

	testdb.ActivateTenant(t, setupPool, tenantB)
	ctxB := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantB})
	actorB := &def.Actor{UserID: uuid.New(), TenantID: tenantB}

	_, err = svc.Update(ctxB, created.ID, map[string]any{"status": "hijacked"}, actorB)
	assert.True(t, runtime.IsNotFound(err), "tenant B must never be able to update tenant A's record, expected NotFoundError, got: %v", err)
}
