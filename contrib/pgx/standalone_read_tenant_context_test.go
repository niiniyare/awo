package pgx_test

// Phase 2 Step 7a — Standalone repository reads must establish tenant
// context themselves.
//
// contrib/pgx.Repository.Get/Query/Exists/Count/Aggregate, when called
// outside an active transaction (the common case: EntityService.Get/Query/
// Count/Exists, the HTTP GET/list handlers, and EntityService.Update/
// Delete's own opening Get), previously ran directly against a bare pool
// connection with no tenant context established — the production
// set_tenant_context() (migration/bootstrap/002_utilities.up.sql) sets its
// GUC with set_config(..., true), PostgreSQL's SET LOCAL semantics, scoped
// to whichever single transaction set it. A standalone call has no
// transaction of its own to carry that GUC into, so RLS reads
// current_tenant_id() = NULL and returns zero rows — even for the caller's
// own, correctly-scoped records.
//
// Every test in this file deliberately uses:
//   - testdb.InstallTenantLifecycle, which installs the REAL production
//     set_tenant_context()/current_tenant_id() (is_local=true) — not
//     SetupTestDB's own simpler, session-persistent (is_local=false)
//     pass-through, which would not have exposed this bug at all.
//   - testdb.OpenConcurrentPool, a genuine multi-connection pool sharing the
//     same isolated schema — not the single-connection pool SetupTestDB
//     itself returns, which would mask the bug by accident (the same
//     physical connection happening to be reused for every call).
//
// These tests fail against the pre-fix Repository.Get/Query (reverting
// withReadTx to a bare connFromContext(ctx, r.pool) call reproduces the
// failure deterministically — verified during implementation, not assumed).

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contribpgx "awo.so/awo/contrib/pgx"
	"awo.so/awo/def"
	"awo.so/awo/driver"
	"awo.so/awo/runtime"
	testdb "awo.so/awo/testutil/db"
)

// createRecord wraps repo.Create in repo.WithTx — Repository.Create never
// opens a transaction of its own (unlike Get/Query after this fix); it
// requires the caller to already be inside one, which is where
// setTenantContext establishes the tenant context an INSERT's own RLS
// WITH CHECK needs. This is pre-existing, correct Repository behavior
// (mirrored by every other test in this package that calls Create), not
// something Step 7a changes.
func createRecord(t *testing.T, repo *contribpgx.Repository, ctx context.Context, data map[string]any) *def.EntityRecord {
	t.Helper()
	var rec *def.EntityRecord
	err := repo.WithTx(ctx, func(txCtx context.Context) error {
		var err error
		rec, err = repo.Create(txCtx, driver.CreateInput{Data: data})
		return err
	})
	require.NoError(t, err)
	return rec
}

// setupStandaloneReadTest installs the REAL production tenant-lifecycle
// function (testdb.InstallTenantLifecycle) and returns both the original
// (setup) pool and a genuinely separate, multi-connection pool
// (testdb.OpenConcurrentPool) sharing the same isolated schema — so a Get/
// Query call in these tests has no special single-connection affinity to
// rely on, matching a real production pgxpool.
func setupStandaloneReadTest(t *testing.T) (setupPool, concurrentPool *pgxpool.Pool) {
	t.Helper()
	setupPool = testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, setupPool)
	testdb.ApplySQL(t, setupPool, testEntityDDL)
	concurrentPool = testdb.OpenConcurrentPool(t, setupPool)
	return setupPool, concurrentPool
}

func activeTenant(t *testing.T, setupPool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	return testdb.CreateTenant(t, setupPool, "ACTIVE")
}

// ── Repository.Get ───────────────────────────────────────────────────────

func TestRepository_Get_StandaloneRead_RealLifecycleFunction_OwnerCanRead(t *testing.T) {
	setupPool, concPool := setupStandaloneReadTest(t)
	tenantA := activeTenant(t, setupPool)

	// Create the record via a mutation (WithTx), which already worked
	// correctly before this fix — establishes tenant context fresh, inside
	// its own transaction.
	repo := contribpgx.NewRepository(concPool, entitySchema())
	ctxA := testdb.WithTenant(context.Background(), tenantA)
	created := createRecord(t, repo, ctxA, map[string]any{"name": "A-record"})

	// The bug: a standalone Get on a multi-connection pool, under the real
	// (transaction-scoped) set_tenant_context, previously returned
	// NotFoundError here even though the record genuinely belongs to
	// tenantA and ctxA is the correct, authenticated tenant context.
	got, err := repo.Get(ctxA, created.ID)
	require.NoError(t, err, "the record's own tenant must be able to read it via a standalone Get")
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, "A-record", got.Data["name"])
}

func TestRepository_Get_StandaloneRead_TenantBCannotReadTenantA(t *testing.T) {
	setupPool, concPool := setupStandaloneReadTest(t)
	tenantA := activeTenant(t, setupPool)
	tenantB := activeTenant(t, setupPool)

	repo := contribpgx.NewRepository(concPool, entitySchema())
	ctxA := testdb.WithTenant(context.Background(), tenantA)
	created := createRecord(t, repo, ctxA, map[string]any{"name": "A-secret"})

	ctxB := testdb.WithTenant(context.Background(), tenantB)
	_, err := repo.Get(ctxB, created.ID)
	assert.True(t, runtime.IsNotFound(err), "tenant B must never read tenant A's record, expected NotFoundError, got: %v", err)
}

func TestRepository_Get_StandaloneRead_NoTenantContext_FailsSafelyNotTenantWide(t *testing.T) {
	setupPool, concPool := setupStandaloneReadTest(t)
	tenantA := activeTenant(t, setupPool)

	repo := contribpgx.NewRepository(concPool, entitySchema())
	ctxA := testdb.WithTenant(context.Background(), tenantA)
	created := createRecord(t, repo, ctxA, map[string]any{"name": "A-record"})

	// No TenantContext at all — must fail closed (not found / no rows),
	// never silently see the record because "no tenant" was mistaken for
	// "every tenant" or "whichever tenant a previous call happened to set."
	_, err := repo.Get(context.Background(), created.ID)
	assert.True(t, runtime.IsNotFound(err), "no tenant context must fail closed, got: %v", err)
}

func TestRepository_Get_StandaloneRead_NonexistentRecord_NotFoundUnchanged(t *testing.T) {
	setupPool, concPool := setupStandaloneReadTest(t)
	tenantA := activeTenant(t, setupPool)

	repo := contribpgx.NewRepository(concPool, entitySchema())
	ctxA := testdb.WithTenant(context.Background(), tenantA)

	_, err := repo.Get(ctxA, uuid.New())
	assert.True(t, runtime.IsNotFound(err), "ordinary not-found semantics must be unchanged, got: %v", err)
}

// ── Repository.Query ─────────────────────────────────────────────────────

func TestRepository_Query_StandaloneRead_RealLifecycleFunction_OwnerSeesOwnRecords(t *testing.T) {
	setupPool, concPool := setupStandaloneReadTest(t)
	tenantA := activeTenant(t, setupPool)

	repo := contribpgx.NewRepository(concPool, entitySchema())
	ctxA := testdb.WithTenant(context.Background(), tenantA)
	createRecord(t, repo, ctxA, map[string]any{"name": "A-1"})
	createRecord(t, repo, ctxA, map[string]any{"name": "A-2"})

	records, info, err := repo.Query(ctxA, nil)
	require.NoError(t, err)
	assert.Len(t, records, 2, "the tenant's own standalone Query must see its own records")
	assert.EqualValues(t, 2, info.Total, "Count-in-same-transaction (info.Total) must also see them")
}

func TestRepository_Query_StandaloneRead_TenantIsolation(t *testing.T) {
	setupPool, concPool := setupStandaloneReadTest(t)
	tenantA := activeTenant(t, setupPool)
	tenantB := activeTenant(t, setupPool)

	repo := contribpgx.NewRepository(concPool, entitySchema())
	ctxA := testdb.WithTenant(context.Background(), tenantA)
	createRecord(t, repo, ctxA, map[string]any{"name": "A-only"})

	ctxB := testdb.WithTenant(context.Background(), tenantB)
	records, _, err := repo.Query(ctxB, nil)
	require.NoError(t, err)
	assert.Empty(t, records, "tenant B's standalone Query must never see tenant A's records")
}

func TestRepository_Query_StandaloneRead_NoTenantContext_SeesNothing(t *testing.T) {
	setupPool, concPool := setupStandaloneReadTest(t)
	tenantA := activeTenant(t, setupPool)

	repo := contribpgx.NewRepository(concPool, entitySchema())
	ctxA := testdb.WithTenant(context.Background(), tenantA)
	createRecord(t, repo, ctxA, map[string]any{"name": "A-record"})

	records, _, err := repo.Query(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, records, "no tenant context must see nothing, not every tenant's records")
}

// ── Concurrency: mandatory multi-connection isolation proof ──────────────

// TestRepository_ConcurrentTenants_StandaloneReads_NoCrossContamination is
// the required concurrent isolation test (Step 7a §10): many goroutines,
// two tenants, a shared multi-connection pool, real transaction-scoped
// set_tenant_context. It fails deterministically against the pre-fix
// implementation (standalone Get/Query with no transaction of their own)
// with spurious NotFoundError/empty-result failures — not with a security
// leak, since RLS itself never leaks data across tenants even when
// mis-scoped; the pre-fix bug is availability/correctness (a legitimate
// owner is told their own record does not exist), which is exactly what
// this test's "no unexpected NotFound for the caller's own records"
// assertion catches.
func TestRepository_ConcurrentTenants_StandaloneReads_NoCrossContamination(t *testing.T) {
	setupPool, concPool := setupStandaloneReadTest(t)
	tenantA := activeTenant(t, setupPool)
	tenantB := activeTenant(t, setupPool)

	repo := contribpgx.NewRepository(concPool, entitySchema())
	ctxA := testdb.WithTenant(context.Background(), tenantA)
	ctxB := testdb.WithTenant(context.Background(), tenantB)

	recA := createRecord(t, repo, ctxA, map[string]any{"name": "A-owned"})
	recB := createRecord(t, repo, ctxB, map[string]any{"name": "B-owned"})

	const goroutinesPerTenant = 8
	const iterationsPerGoroutine = 20

	var wg sync.WaitGroup
	errCh := make(chan error, goroutinesPerTenant*2*iterationsPerGoroutine)

	worker := func(ctx context.Context, own, foreign *struct {
		id   uuid.UUID
		name string
	}) {
		defer wg.Done()
		for i := 0; i < iterationsPerGoroutine; i++ {
			got, err := repo.Get(ctx, own.id)
			if err != nil {
				errCh <- err
				continue
			}
			if got.Data["name"] != own.name {
				errCh <- fmt.Errorf("expected own record %q, got %q", own.name, got.Data["name"])
			}

			_, err = repo.Get(ctx, foreign.id)
			if !runtime.IsNotFound(err) {
				errCh <- fmt.Errorf("expected NotFound reading foreign tenant's record, got: %v", err)
			}

			records, _, err := repo.Query(ctx, nil)
			if err != nil {
				errCh <- err
				continue
			}
			for _, rec := range records {
				if rec.ID == foreign.id {
					errCh <- fmt.Errorf("Query leaked foreign tenant's record %s", foreign.id)
				}
			}
		}
	}

	a := &struct {
		id   uuid.UUID
		name string
	}{recA.ID, "A-owned"}
	b := &struct {
		id   uuid.UUID
		name string
	}{recB.ID, "B-owned"}

	for i := 0; i < goroutinesPerTenant; i++ {
		wg.Add(2)
		go worker(ctxA, a, b)
		go worker(ctxB, b, a)
	}
	wg.Wait()
	close(errCh)

	var failures []error
	for e := range errCh {
		failures = append(failures, e)
	}
	assert.Empty(t, failures, "concurrent standalone reads across two tenants on a shared multi-connection pool must never cross-contaminate or spuriously miss the caller's own records: %v", failures)
}

// ── Connection safety ─────────────────────────────────────────────────────

// TestRepository_Get_StandaloneRead_RepeatedCalls_DoNotLeakConnections proves
// withReadTx's transaction (opened for every standalone Get/Query call that
// has a tenant context) is always committed or rolled back, releasing its
// connection back to the pool — using the same pool_max_conns=1 discipline
// withtx_panic_safety_test.go uses for WithTx: if any call ever failed to
// release its connection, the very next call on this pool would hang until
// the test's own timeout, since there is no second connection for it to use.
func TestRepository_Get_StandaloneRead_RepeatedCalls_DoNotLeakConnections(t *testing.T) {
	pool := testdb.SetupTestDB(t) // pool_max_conns=1
	testdb.InstallTenantLifecycle(t, pool)
	testdb.ApplySQL(t, pool, testEntityDDL)

	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := testdb.WithTenant(context.Background(), tenantID)

	repo := contribpgx.NewRepository(pool, entitySchema())
	created := createRecord(t, repo, ctx, map[string]any{"name": "leak-check"})

	for i := 0; i < 25; i++ {
		_, err := repo.Get(ctx, created.ID)
		require.NoError(t, err, "call %d: a leaked connection from an earlier call would starve this one", i)
		_, _, err = repo.Query(ctx, nil)
		require.NoError(t, err, "call %d", i)
		_, err = repo.Exists(ctx, nil)
		require.NoError(t, err, "call %d", i)
		_, err = repo.Count(ctx, nil)
		require.NoError(t, err, "call %d", i)
	}
}
