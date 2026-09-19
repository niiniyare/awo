package pgx_test

// Write-path SQL injection / column-authorization regression tests.
//
// contrib/pgx.Repository.Update (system-entity branch, "updateSystem") and
// BulkUpdate build their SET clause directly from the KEYS of a client-
// controlled map (driver.UpdateInput.Data / driver.Patch.Set). For Update,
// that map is exactly the raw, unrestricted JSON body of
// PATCH /api/v1/entities/:entity/:id (api/handler/crud.go unmarshals it into
// a bare map[string]any with no key allowlist anywhere upstream — see
// runtime/pipeline.go's RunBeforeUpdate/validateFields, which only checks
// ImmutableFields/RequiredFields/FieldValidators, never rejects an
// unrecognized key). Before the fix, a map key was interpolated straight
// into `"%s" = $N` with no escaping and no check that it was even a real
// column — both a SQL-injection surface (a key containing `"` breaks out of
// the identifier) and a privilege-escalation surface (a key of "tenant_id"
// or "id" would happily reach the SET clause, letting a caller reassign a
// record's tenant or primary key through an ordinary field update). The fix
// (Repository.checkWritableField) rejects any key that is not one of the
// entity's own declared business fields — which structurally excludes
// tenant_id/id/created_at/custom_fields, since those are framework-managed
// columns handled by dedicated code paths, never entity FieldsByName
// entries — before the key ever reaches the SQL string.
import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/driver"
	"awo.so/awo/filter"
	"awo.so/awo/runtime"
	"awo.so/awo/runtime/tenant"
	testdb "awo.so/awo/testutil/db"
)

func TestUpdate_MaliciousFieldName_RejectedNotInjected(t *testing.T) {
	pool, _, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	rec, err := repo.Create(ctx, driver.CreateInput{Data: map[string]any{"name": "Alpha", "code": "A1"}})
	require.NoError(t, err)

	_, err = repo.Update(ctx, rec.ID, driver.UpdateInput{
		Data: map[string]any{`name" = 'HACKED', "code`: "whatever"},
	})
	require.Error(t, err, "a field name that would break out of its identifier quotes must be rejected, not executed")

	after, err := repo.Get(ctx, rec.ID)
	require.NoError(t, err)
	assert.Equal(t, "Alpha", after.Data["name"], "the row must be completely unmodified after a rejected update")
	assert.Equal(t, "A1", after.Data["code"])
}

func TestUpdate_TenantIDInPatchBody_Rejected(t *testing.T) {
	pool, _, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	rec, err := repo.Create(ctx, driver.CreateInput{Data: map[string]any{"name": "Alpha", "code": "A1"}})
	require.NoError(t, err)

	otherTenant := testdb.RawTenantID()
	_, err = repo.Update(ctx, rec.ID, driver.UpdateInput{
		Data: map[string]any{"tenant_id": otherTenant.String()},
	})
	require.Error(t, err, `"tenant_id" is a framework-managed column, not a declared entity field — `+
		"a patch must never be able to reassign a record to a different tenant through it")
}

func TestUpdate_IDInPatchBody_Rejected(t *testing.T) {
	pool, _, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	rec, err := repo.Create(ctx, driver.CreateInput{Data: map[string]any{"name": "Alpha", "code": "A1"}})
	require.NoError(t, err)

	_, err = repo.Update(ctx, rec.ID, driver.UpdateInput{
		Data: map[string]any{"id": testdb.RawTenantID().String()},
	})
	require.Error(t, err, `"id" is the primary key, not a declared entity field — a patch must never be able to change it`)
}

func TestUpdate_CustomFieldsColumnDirectOverwrite_Rejected(t *testing.T) {
	pool, _, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	rec, err := repo.Create(ctx, driver.CreateInput{Data: map[string]any{"name": "Alpha", "code": "A1"}})
	require.NoError(t, err)

	_, err = repo.Update(ctx, rec.ID, driver.UpdateInput{
		Data: map[string]any{"custom_fields": "not-json-and-not-the-merge-path"},
	})
	require.Error(t, err, `"custom_fields" must only be reachable via UpdateInput.CustomFields' `+
		"dedicated jsonb-merge path, not as an ordinary Data key")
}

func TestUpdate_LegitimateField_StillWorks(t *testing.T) {
	pool, _, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	rec, err := repo.Create(ctx, driver.CreateInput{Data: map[string]any{"name": "Alpha", "code": "A1"}})
	require.NoError(t, err)

	updated, err := repo.Update(ctx, rec.ID, driver.UpdateInput{
		Data: map[string]any{"name": "Beta"},
	})
	require.NoError(t, err, "a legitimate declared field must still be patchable after the fix")
	assert.Equal(t, "Beta", updated.Data["name"])
	assert.Equal(t, "A1", updated.Data["code"], "fields absent from the patch must be left untouched")
}

// TestUpdate_CreateInputCannotSmuggleTenantID proves Create is structurally
// immune to the class of bug Update had: createSystem builds its column
// list from r.sortedFieldNames() (the schema's own declared fields) and
// looks up values FROM that list, rather than iterating input.Data's own
// keys — so a "tenant_id" key in the Data map is never even looked at, let
// alone written. TenantID always comes from tenant.FromContext(ctx), never
// from client-supplied data, on the Create path.
func TestUpdate_CreateInputCannotSmuggleTenantID(t *testing.T) {
	pool, tenantID, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	otherTenant := testdb.RawTenantID()
	rec, err := repo.Create(ctx, driver.CreateInput{
		Data: map[string]any{"name": "Alpha", "code": "A1", "tenant_id": otherTenant.String()},
	})
	require.NoError(t, err, "Create must not error just because the client-supplied map contains an "+
		"extraneous 'tenant_id' key — it must simply be ignored")
	assert.Equal(t, tenantID, rec.TenantID, "the record's real tenant_id must come from ctx, "+
		"never from the attempted 'tenant_id' key in the Data map")
}

// TestUpdate_CrossTenantByID_RejectedAsNotFound proves that Tenant A cannot
// use Update to modify a record it can name the ID of but that actually
// belongs to Tenant B — RLS filters Repository.Update's
// UPDATE ... WHERE "id" = $N down to zero affected rows for a foreign
// tenant's record, and Repository.Update surfaces that as NotFoundError,
// not as a silent no-op success or, worse, a cross-tenant write.
func TestUpdate_CrossTenantByID_RejectedAsNotFound(t *testing.T) {
	pool := setupPoolTest(t)
	repo := newRepo(pool)

	tenantA := testdb.CreateTenant(t, pool, "ACTIVE")
	tenantB := testdb.CreateTenant(t, pool, "ACTIVE")
	switchToAppRole(t, pool)

	ctxA := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantA})
	var victimID uuid.UUID
	require.NoError(t, repo.WithTx(ctxA, func(txCtx context.Context) error {
		rec, err := repo.Create(txCtx, driver.CreateInput{Data: map[string]any{"name": "A-owned"}})
		victimID = rec.ID
		return err
	}))

	ctxB := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantB})
	err := repo.WithTx(ctxB, func(txCtx context.Context) error {
		_, err := repo.Update(txCtx, victimID, driver.UpdateInput{Data: map[string]any{"name": "HACKED-BY-B"}})
		return err
	})
	require.Error(t, err, "Tenant B must not be able to update a record belonging to Tenant A, even "+
		"knowing its exact ID")
	var nf *runtime.NotFoundError
	assert.ErrorAs(t, err, &nf, "the failure must be surfaced as NotFoundError (RLS made the row "+
		"invisible to the UPDATE's WHERE clause), not some other error shape")

	// Confirm as superuser that Tenant A's row is genuinely unmodified.
	// test_entity (see entitySchema/testEntityDDL) is a "system" entity with
	// real typed columns, not a jsonb blob — "name" is queried directly.
	row := testdb.QueryRowSQL(t, pool, "SELECT name FROM test_entity WHERE id = $1", victimID)
	var name string
	require.NoError(t, row.Scan(&name))
	assert.Equal(t, "A-owned", name, "Tenant A's record must be completely unmodified by Tenant B's rejected attempt")
}

func TestBulkUpdate_MaliciousFieldName_RejectedNotInjected(t *testing.T) {
	pool, _, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	rec, err := repo.Create(ctx, driver.CreateInput{Data: map[string]any{"name": "Alpha", "code": "A1"}})
	require.NoError(t, err)

	_, err = repo.BulkUpdate(ctx, filter.Eq("id", rec.ID.String()), driver.Patch{
		Set: map[string]any{`name" = 'HACKED', "code`: "whatever"},
	})
	require.Error(t, err, "BulkUpdate must reject the same class of malicious field name Update rejects")

	after, err := repo.Get(ctx, rec.ID)
	require.NoError(t, err)
	assert.Equal(t, "Alpha", after.Data["name"])
}

func TestBulkUpdate_LegitimateField_StillWorks(t *testing.T) {
	pool, _, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	_, err := repo.Create(ctx, driver.CreateInput{Data: map[string]any{"name": "Alpha", "code": "A1"}})
	require.NoError(t, err)

	n, err := repo.BulkUpdate(ctx, filter.Eq("code", "A1"), driver.Patch{
		Set: map[string]any{"name": "Gamma"},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
}

// TestUpdate_UpdatedAtInPatchBody_Rejected proves updated_at specifically —
// not just tenant_id/id/custom_fields — is rejected. updated_at is always
// set by Repository.Update itself (the hardcoded `"updated_at" = $1` in
// every SET clause); a client-supplied value for it must never reach the
// SQL at all, let alone override the server-computed timestamp.
func TestUpdate_UpdatedAtInPatchBody_Rejected(t *testing.T) {
	pool, _, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	rec, err := repo.Create(ctx, driver.CreateInput{Data: map[string]any{"name": "Alpha", "code": "A1"}})
	require.NoError(t, err)

	_, err = repo.Update(ctx, rec.ID, driver.UpdateInput{
		Data: map[string]any{"updated_at": "2099-01-01T00:00:00Z"},
	})
	require.Error(t, err, `"updated_at" is a framework-managed column, not a declared entity field, and `+
		"must be rejected — Repository.Update already sets it itself on every call")
}

// TestUpdate_DeletedAtInPatchBody_Rejected proves deleted_at — a standard
// column generator.go emits on every non-System table (see
// generateEntitySQL) even though no repository method currently implements
// soft-delete against it — cannot be set via a generic patch either. It is
// not a declared entity field (same reasoning as tenant_id/id/created_at:
// it is part of the hardcoded standard-columns block, never added to
// es.Fields/FieldsByName), so checkWritableField excludes it the same way.
func TestUpdate_DeletedAtInPatchBody_Rejected(t *testing.T) {
	pool, _, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	rec, err := repo.Create(ctx, driver.CreateInput{Data: map[string]any{"name": "Alpha", "code": "A1"}})
	require.NoError(t, err)

	_, err = repo.Update(ctx, rec.ID, driver.UpdateInput{
		Data: map[string]any{"deleted_at": "2020-01-01T00:00:00Z"},
	})
	require.Error(t, err, `"deleted_at" is a framework-managed standard column, not a declared entity `+
		"field, and must be rejected even though no repository method currently acts on it")
}

// ── Aggregate ─────────────────────────────────────────────────────────────────
//
// Repository.Aggregate has zero production callers today (confirmed by
// source review), but shares the exact same vulnerable shape updateSystem
// had: fn.Field and spec.GroupBy were interpolated into the SQL with naive
// `"%s"` quoting and no allowlist check, and fn.Fn (the aggregate function
// name itself) was interpolated completely unquoted with no validation at
// all against driver's fixed set of five functions. Fixed as defense in
// depth / landmine removal before anything comes to depend on it.

func TestAggregate_MaliciousFieldName_Rejected(t *testing.T) {
	pool, _, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	_, err := repo.Create(ctx, driver.CreateInput{Data: map[string]any{"name": "Alpha", "code": "A1"}})
	require.NoError(t, err)

	_, err = repo.Aggregate(ctx, nil, driver.AggregateSpec{
		Functions: []driver.AggregateFunc{
			{Fn: driver.AggregateFnMax, Field: `name" ; DROP TABLE test_entity; --`, Alias: "x"},
		},
	})
	require.Error(t, err, "an aggregate field that would break out of its identifier quotes must be rejected")
}

func TestAggregate_MaliciousGroupBy_Rejected(t *testing.T) {
	pool, _, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	_, err := repo.Create(ctx, driver.CreateInput{Data: map[string]any{"name": "Alpha", "code": "A1"}})
	require.NoError(t, err)

	_, err = repo.Aggregate(ctx, nil, driver.AggregateSpec{
		Functions: []driver.AggregateFunc{{Fn: driver.AggregateFnCount, Alias: "cnt"}},
		GroupBy:   `code" ; DROP TABLE test_entity; --`,
	})
	require.Error(t, err, "a GROUP BY field that would break out of its identifier quotes must be rejected")
}

func TestAggregate_UnknownFunction_Rejected(t *testing.T) {
	pool, _, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	_, err := repo.Create(ctx, driver.CreateInput{Data: map[string]any{"name": "Alpha", "code": "A1"}})
	require.NoError(t, err)

	_, err = repo.Aggregate(ctx, nil, driver.AggregateSpec{
		Functions: []driver.AggregateFunc{
			{Fn: driver.AggregateFn("count(1); DROP TABLE test_entity; --"), Field: "name", Alias: "x"},
		},
	})
	require.Error(t, err, "an aggregate function name outside the fixed count/sum/avg/min/max set must be "+
		"rejected — it is interpolated as a bare SQL token with no quoting possible")
}

func TestAggregate_LegitimateUsage_StillWorks(t *testing.T) {
	pool, _, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	_, err := repo.Create(ctx, driver.CreateInput{Data: map[string]any{"name": "Alpha", "code": "A1"}})
	require.NoError(t, err)
	_, err = repo.Create(ctx, driver.CreateInput{Data: map[string]any{"name": "Beta", "code": "A1"}})
	require.NoError(t, err)

	result, err := repo.Aggregate(ctx, nil, driver.AggregateSpec{
		Functions: []driver.AggregateFunc{{Fn: driver.AggregateFnCount, Alias: "cnt"}},
		GroupBy:   "code",
	})
	require.NoError(t, err, "legitimate aggregate usage must still work after the fix")
	assert.Equal(t, int64(2), result.Values["cnt"])
}

// TestUpdate_AnyUndeclaredColumnName_Rejected proves the check is a real
// allowlist (only the entity's own declared fields), not a denylist of a few
// known-dangerous names — an arbitrary column name that happens to be
// syntactically harmless (no quote characters to break out with) but simply
// isn't a declared field must still be rejected.
func TestUpdate_AnyUndeclaredColumnName_Rejected(t *testing.T) {
	pool, _, ctx := setupRepoTest(t)
	repo := newRepo(pool)

	rec, err := repo.Create(ctx, driver.CreateInput{Data: map[string]any{"name": "Alpha", "code": "A1"}})
	require.NoError(t, err)

	_, err = repo.Update(ctx, rec.ID, driver.UpdateInput{
		Data: map[string]any{"created_at": "2020-01-01T00:00:00Z"},
	})
	require.Error(t, err, `"created_at" is a framework-managed column, not a declared entity field, `+
		"and must be rejected even though the value itself contains no injection payload")
}
