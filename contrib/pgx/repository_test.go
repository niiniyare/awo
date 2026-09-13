package pgx_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contribpgx "awo.so/awo/contrib/pgx"
	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"awo.so/awo/driver"
	"awo.so/awo/filter"
	"awo.so/awo/runtime"
	testdb "awo.so/awo/testutil/db"
)

// testEntityDDL is a minimal tenant-scoped system entity table.
// Mirrors what the migration generator produces for ScopeTenant+SystemDefinition.
const testEntityDDL = `
CREATE TABLE test_entity (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     uuid NOT NULL,
    name          text NOT NULL,
    code          text,
    custom_fields jsonb,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE test_entity ENABLE ROW LEVEL SECURITY;
ALTER TABLE test_entity FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON test_entity
    USING (tenant_id = current_tenant_id());

GRANT SELECT, INSERT, UPDATE, DELETE ON test_entity TO awo_app;
`

// entitySchema returns a compiler.EntitySchema matching testEntityDDL.
func entitySchema() *compiler.EntitySchema {
	return &compiler.EntitySchema{
		QualifiedName: "test_entity",
		TableName:     "test_entity",
		IsSystem:      true,
		FieldsByName: map[string]def.FieldDef{
			"name": {Name: "name", Type: def.FieldTypeData},
			"code": {Name: "code", Type: def.FieldTypeData},
		},
	}
}

// newRepo builds a *Repository for the test entity using the given pool.
func newRepo(pool *pgxpool.Pool) *contribpgx.Repository {
	return contribpgx.NewRepository(pool, entitySchema())
}

// setupRepoTest creates an isolated DB schema, applies DDL, returns pool +
// a context wired with tenantA already activated (AppRole switched).
func setupRepoTest(t *testing.T) (*pgxpool.Pool, uuid.UUID, context.Context) {
	t.Helper()
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, testEntityDDL)

	tenantID := testdb.RawTenantID()
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := testdb.WithTenant(context.Background(), tenantID)
	return pool, tenantID, ctx
}

// ── Create ────────────────────────────────────────────────────────────────────

func TestRepository_Create_PersistsRecord(t *testing.T) {
	pool, _, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	rec, err := repo.Create(ctx, driver.CreateInput{
		Data: map[string]any{"name": "Alpha", "code": "A1"},
	})
	require.NoError(t, err)
	require.NotNil(t, rec)

	assert.NotEqual(t, uuid.Nil, rec.ID, "ID must be set")
	assert.Equal(t, "Alpha", rec.Data["name"])
	assert.Equal(t, "A1", rec.Data["code"])
	assert.False(t, rec.CreatedAt.IsZero(), "created_at must be set")
	assert.False(t, rec.UpdatedAt.IsZero(), "updated_at must be set")
}

func TestRepository_Create_TenantIDSet(t *testing.T) {
	pool, tenantID, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	rec, err := repo.Create(ctx, driver.CreateInput{
		Data: map[string]any{"name": "Beta"},
	})
	require.NoError(t, err)
	assert.Equal(t, tenantID, rec.TenantID, "tenant_id must match context tenantID")
}

// ── Get ───────────────────────────────────────────────────────────────────────

func TestRepository_Get_ReturnsRecord(t *testing.T) {
	pool, _, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	created, err := repo.Create(ctx, driver.CreateInput{
		Data: map[string]any{"name": "Gamma"},
	})
	require.NoError(t, err)

	got, err := repo.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, "Gamma", got.Data["name"])
}

func TestRepository_Get_NotFound(t *testing.T) {
	pool, _, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	_, err := repo.Get(ctx, uuid.New())
	require.Error(t, err)
	assert.True(t, runtime.IsNotFound(err), "expected NotFoundError, got: %v", err)
}

func TestRepository_Get_TenantIsolation(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, testEntityDDL)

	tenantA := testdb.RawTenantID()
	tenantB := testdb.RawTenantID()

	// Create a record as Tenant A.
	testdb.ActivateTenant(t, pool, tenantA)
	ctxA := testdb.WithTenant(context.Background(), tenantA)
	repo := newRepo(pool)
	recA, err := repo.Create(ctxA, driver.CreateInput{
		Data: map[string]any{"name": "TenantA-Record"},
	})
	require.NoError(t, err)

	// As Tenant B: Get by Tenant A's ID must return not-found.
	testdb.ActivateTenant(t, pool, tenantB)
	ctxB := testdb.WithTenant(context.Background(), tenantB)
	_, err = repo.Get(ctxB, recA.ID)
	assert.True(t, runtime.IsNotFound(err),
		"Tenant B must not be able to read Tenant A's record via RLS")
}

// ── Query ─────────────────────────────────────────────────────────────────────

func TestRepository_Query_ReturnsAll(t *testing.T) {
	pool, _, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	for _, name := range []string{"One", "Two", "Three"} {
		_, err := repo.Create(ctx, driver.CreateInput{
			Data: map[string]any{"name": name},
		})
		require.NoError(t, err)
	}

	records, _, err := repo.Query(ctx, nil, driver.WithSkipCount())
	require.NoError(t, err)
	assert.Len(t, records, 3)
}

func TestRepository_Query_WithEqFilter(t *testing.T) {
	pool, _, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	for _, name := range []string{"Alpha", "Beta", "Gamma"} {
		_, err := repo.Create(ctx, driver.CreateInput{
			Data: map[string]any{"name": name},
		})
		require.NoError(t, err)
	}

	records, _, err := repo.Query(ctx, filter.Eq("name", "Beta"), driver.WithSkipCount())
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "Beta", records[0].Data["name"])
}

func TestRepository_Query_EmptyResult(t *testing.T) {
	pool, _, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	records, info, err := repo.Query(ctx, filter.Eq("name", "nonexistent"))
	require.NoError(t, err)
	assert.Empty(t, records)
	assert.Equal(t, int64(0), info.Total)
}

func TestRepository_Query_TenantIsolation(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, testEntityDDL)

	tenantA := testdb.RawTenantID()
	tenantB := testdb.RawTenantID()
	repo := newRepo(pool)

	// Tenant A creates 2 records.
	testdb.ActivateTenant(t, pool, tenantA)
	ctxA := testdb.WithTenant(context.Background(), tenantA)
	for _, name := range []string{"A1", "A2"} {
		_, err := repo.Create(ctxA, driver.CreateInput{Data: map[string]any{"name": name}})
		require.NoError(t, err)
	}

	// Tenant B creates 1 record.
	testdb.ActivateTenant(t, pool, tenantB)
	ctxB := testdb.WithTenant(context.Background(), tenantB)
	_, err := repo.Create(ctxB, driver.CreateInput{Data: map[string]any{"name": "B1"}})
	require.NoError(t, err)

	// Tenant A queries — must see exactly 2 rows.
	testdb.ActivateTenant(t, pool, tenantA)
	recs, _, err := repo.Query(ctxA, nil, driver.WithSkipCount())
	require.NoError(t, err)
	assert.Len(t, recs, 2, "Tenant A must see only its 2 rows via RLS")

	// Tenant B queries — must see exactly 1 row.
	testdb.ActivateTenant(t, pool, tenantB)
	recs, _, err = repo.Query(ctxB, nil, driver.WithSkipCount())
	require.NoError(t, err)
	assert.Len(t, recs, 1, "Tenant B must see only its 1 row via RLS")
}

func TestRepository_Query_Pagination(t *testing.T) {
	pool, _, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	for i := range 5 {
		_, err := repo.Create(ctx, driver.CreateInput{
			Data: map[string]any{"name": "item", "code": string(rune('A' + i))},
		})
		require.NoError(t, err)
	}

	// Page 1: 3 records.
	page1, info1, err := repo.Query(ctx, nil, driver.WithPage(1, 3))
	require.NoError(t, err)
	assert.Len(t, page1, 3)
	assert.True(t, info1.HasNextPage)
	assert.False(t, info1.HasPrevPage)

	// Page 2: remaining 2 records.
	page2, info2, err := repo.Query(ctx, nil, driver.WithPage(2, 3))
	require.NoError(t, err)
	assert.Len(t, page2, 2)
	assert.False(t, info2.HasNextPage)
	assert.True(t, info2.HasPrevPage)
}

// ── Count / Exists ────────────────────────────────────────────────────────────

func TestRepository_Count(t *testing.T) {
	pool, _, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	for _, name := range []string{"X", "Y", "Z"} {
		_, err := repo.Create(ctx, driver.CreateInput{Data: map[string]any{"name": name}})
		require.NoError(t, err)
	}

	n, err := repo.Count(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(3), n)
}

func TestRepository_Exists(t *testing.T) {
	pool, _, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	ok, err := repo.Exists(ctx, filter.Eq("name", "ghost"))
	require.NoError(t, err)
	assert.False(t, ok)

	_, err = repo.Create(ctx, driver.CreateInput{Data: map[string]any{"name": "ghost"}})
	require.NoError(t, err)

	ok, err = repo.Exists(ctx, filter.Eq("name", "ghost"))
	require.NoError(t, err)
	assert.True(t, ok)
}

// ── Update ────────────────────────────────────────────────────────────────────

func TestRepository_Update_ModifiesFields(t *testing.T) {
	pool, _, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	rec, err := repo.Create(ctx, driver.CreateInput{Data: map[string]any{"name": "Original"}})
	require.NoError(t, err)

	updated, err := repo.Update(ctx, rec.ID, driver.UpdateInput{
		Data: map[string]any{"name": "Modified"},
	})
	require.NoError(t, err)
	assert.Equal(t, "Modified", updated.Data["name"])
	assert.Equal(t, rec.ID, updated.ID)
}

func TestRepository_Update_NotFound(t *testing.T) {
	pool, _, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	_, err := repo.Update(ctx, uuid.New(), driver.UpdateInput{
		Data: map[string]any{"name": "ghost"},
	})
	require.Error(t, err)
	assert.True(t, runtime.IsNotFound(err))
}

// ── Delete ────────────────────────────────────────────────────────────────────

func TestRepository_Delete_RemovesRecord(t *testing.T) {
	pool, _, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	rec, err := repo.Create(ctx, driver.CreateInput{Data: map[string]any{"name": "ToDelete"}})
	require.NoError(t, err)

	require.NoError(t, repo.Delete(ctx, rec.ID))

	_, err = repo.Get(ctx, rec.ID)
	assert.True(t, runtime.IsNotFound(err), "deleted record must not be found")
}

func TestRepository_Delete_NotFound(t *testing.T) {
	pool, _, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	err := repo.Delete(ctx, uuid.New())
	require.Error(t, err)
	assert.True(t, runtime.IsNotFound(err))
}

// ── BulkCreate ────────────────────────────────────────────────────────────────

func TestRepository_BulkCreate_InsertsAll(t *testing.T) {
	pool, _, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	inputs := []driver.CreateInput{
		{Data: map[string]any{"name": "Bulk1"}},
		{Data: map[string]any{"name": "Bulk2"}},
		{Data: map[string]any{"name": "Bulk3"}},
	}
	records, err := repo.BulkCreate(ctx, inputs)
	require.NoError(t, err)
	require.Len(t, records, 3)

	// Verify each has a unique ID and correct name.
	seen := map[uuid.UUID]bool{}
	for _, rec := range records {
		assert.NotEqual(t, uuid.Nil, rec.ID)
		assert.False(t, seen[rec.ID], "IDs must be unique")
		seen[rec.ID] = true
	}

	// Verify all rows are actually in the DB.
	n, err := repo.Count(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(3), n)
}

func TestRepository_BulkCreate_TenantIsolation(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, testEntityDDL)

	tenantA := testdb.RawTenantID()
	tenantB := testdb.RawTenantID()
	repo := newRepo(pool)

	// Tenant A bulk-creates 2 records.
	testdb.ActivateTenant(t, pool, tenantA)
	ctxA := testdb.WithTenant(context.Background(), tenantA)
	_, err := repo.BulkCreate(ctxA, []driver.CreateInput{
		{Data: map[string]any{"name": "A1"}},
		{Data: map[string]any{"name": "A2"}},
	})
	require.NoError(t, err)

	// Tenant B sees 0 rows via RLS.
	testdb.ActivateTenant(t, pool, tenantB)
	ctxB := testdb.WithTenant(context.Background(), tenantB)
	n, err := repo.Count(ctxB, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(0), n, "Tenant B must not see Tenant A's bulk-created rows")
}

// ── WithTx rollback ───────────────────────────────────────────────────────────

func TestRepository_WithTx_RollbackRevertsMutation(t *testing.T) {
	pool, _, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	var createdID uuid.UUID
	err := repo.WithTx(ctx, func(txCtx context.Context) error {
		rec, err := repo.Create(txCtx, driver.CreateInput{Data: map[string]any{"name": "Transient"}})
		if err != nil {
			return err
		}
		createdID = rec.ID
		return errRollback // trigger rollback
	})
	require.ErrorIs(t, err, errRollback)

	// Record must not exist after rollback.
	_, err = repo.Get(ctx, createdID)
	assert.True(t, runtime.IsNotFound(err), "rolled-back record must not persist")
}

// errRollback is a sentinel used to force transaction rollback in tests.
var errRollback = errors.New("intentional rollback")
