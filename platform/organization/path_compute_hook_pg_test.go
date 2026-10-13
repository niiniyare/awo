package organization_test

// Phase 3 Step 1 — PathComputeHook correctness.
//
// Exercises the REAL, registered platform_organization hook pipeline (the
// actual Definition.Hooks.AfterCreate entry, wired via
// organization.WireRepository exactly as cmd/server/main.go wires it at
// bootstrap) through EntityService.Create against real PostgreSQL with the
// real production platform_organization migration and the real,
// transaction-scoped set_tenant_context() (testdb.InstallTenantLifecycle) —
// not a test-only shortcut field.
//
// Root cause this proves fixed: PathComputeHook previously ran BeforeCreate,
// before the repository generates the record's real ID, and fell back to
// silently recording every node (root or not) as a root with a fixed,
// non-unique self-segment whenever the test-only "_parent_path" field
// wasn't pre-populated — which it never is on the real API/EntityService
// call path. The fix moves computation to AfterCreate (inside the same
// transaction as creation, with the real persisted ID) and does a real
// parent lookup via the real repository.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/api/service"
	"awo.so/awo/audit"
	contribpgx "awo.so/awo/contrib/pgx"
	"awo.so/awo/def"
	"awo.so/awo/platform/organization"
	"awo.so/awo/runtime"
	"awo.so/awo/runtime/tenant"
	testdb "awo.so/awo/testutil/db"
)

// setupOrgPGService compiles the REAL organization.Definition (reusing
// definition_test.go's buildOrgSchema, same package), applies the REAL
// platform_organization migration file verbatim (read from disk, not
// hand-copied, so this test cannot silently drift from production DDL) plus
// the awo_app grants testdb's non-superuser role model requires, installs
// the real transaction-scoped tenant-lifecycle function, wires the real
// PathComputeHook singleton via organization.WireRepository exactly as
// production bootstrap does, and returns a ready EntityService.
func setupOrgPGService(t *testing.T) (svc *service.EntityService, pool *pgxpool.Pool) {
	t.Helper()

	schema := buildOrgSchema(t)
	es := schema.ByName["platform_organization"]
	require.NotNil(t, es, "platform_organization missing from compiled schema")

	pool = testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, pool)

	migrationPath := filepath.Join("migrations", "20260101000080_create_platform_organization.up.sql")
	migrationSQL, err := os.ReadFile(migrationPath)
	require.NoError(t, err, "read real platform_organization migration")
	testdb.ApplySQL(t, pool, string(migrationSQL))
	testdb.ApplySQL(t, pool, `
GRANT SELECT, INSERT, UPDATE, DELETE ON platform_organization TO `+testdb.AppRole+`;
`)

	repo := contribpgx.NewRepository(pool, es)
	organization.WireRepository(repo)

	pipeline := runtime.NewPipeline(schema, audit.NoopAuditWriter{})
	svc = service.NewEntityService(es, repo, pipeline)
	return svc, pool
}

func newCtx(tenantID uuid.UUID) context.Context {
	return tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantID})
}

func newActor(tenantID uuid.UUID) *def.Actor {
	return &def.Actor{UserID: uuid.New(), TenantID: tenantID}
}

// orgData merges tenant_id (a required FieldDef on organization.Definition,
// mirroring OrganizationService.CreateInput.TenantID — not an implicit
// driver-injected column the way generator-driven entities get it) into the
// given fields.
func orgData(tenantID uuid.UUID, fields map[string]any) map[string]any {
	data := map[string]any{"tenant_id": tenantID.String()}
	for k, v := range fields {
		data[k] = v
	}
	return data
}

// --- Case 1: root organization ---------------------------------------------

func TestPathComputeHook_RootOrganization_CorrectPathAndDepth(t *testing.T) {
	svc, pool := setupOrgPGService(t)
	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := newCtx(tenantID)
	actor := newActor(tenantID)

	created, err := svc.Create(ctx, orgData(tenantID, map[string]any{"name": "Acme Corp", "code": "ACME"}), actor)
	require.NoError(t, err)

	wantPath := "/" + created.ID.String() + "/"
	assert.Equal(t, wantPath, created.Data["path"], "root path must be self-ID-derived, not a placeholder")
	assert.Equal(t, int64(0), created.Data["depth"])

	// Re-read to prove it was actually persisted, not just corrected in memory.
	fetched, err := svc.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, wantPath, fetched.Data["path"])
	assert.Equal(t, int32(0), fetched.Data["depth"])
}

// --- Case 2: Company -> Region ----------------------------------------------

func TestPathComputeHook_CompanyToRegion_CorrectChildPathAndDepth(t *testing.T) {
	svc, pool := setupOrgPGService(t)
	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := newCtx(tenantID)
	actor := newActor(tenantID)

	company, err := svc.Create(ctx, orgData(tenantID, map[string]any{"name": "Company", "code": "CO1"}), actor)
	require.NoError(t, err)
	companyPath := company.Data["path"].(string)

	region, err := svc.Create(ctx, orgData(tenantID, map[string]any{
		"name": "Region", "code": "RG1", "parent_id": company.ID.String(),
	}), actor)
	require.NoError(t, err)

	wantPath := companyPath + region.ID.String() + "/"
	assert.Equal(t, wantPath, region.Data["path"])
	assert.Equal(t, int64(1), region.Data["depth"])
}

// --- Case 3: Company -> Region -> Branch, all three levels ------------------

func TestPathComputeHook_ThreeLevelHierarchy_CorrectAtEveryLevel(t *testing.T) {
	svc, pool := setupOrgPGService(t)
	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := newCtx(tenantID)
	actor := newActor(tenantID)

	company, err := svc.Create(ctx, orgData(tenantID, map[string]any{"name": "Company", "code": "CO2"}), actor)
	require.NoError(t, err)
	region, err := svc.Create(ctx, orgData(tenantID, map[string]any{
		"name": "Region", "code": "RG2", "parent_id": company.ID.String(),
	}), actor)
	require.NoError(t, err)
	branch, err := svc.Create(ctx, orgData(tenantID, map[string]any{
		"name": "Branch", "code": "BR2", "parent_id": region.ID.String(),
	}), actor)
	require.NoError(t, err)

	companyPath := "/" + company.ID.String() + "/"
	regionPath := companyPath + region.ID.String() + "/"
	branchPath := regionPath + branch.ID.String() + "/"

	assert.Equal(t, companyPath, company.Data["path"])
	assert.Equal(t, int64(0), company.Data["depth"])
	assert.Equal(t, regionPath, region.Data["path"])
	assert.Equal(t, int64(1), region.Data["depth"])
	assert.Equal(t, branchPath, branch.Data["path"])
	assert.Equal(t, int64(2), branch.Data["depth"])
}

// --- Case 4: missing parent --------------------------------------------------

func TestPathComputeHook_MissingParent_RejectedSafely(t *testing.T) {
	svc, pool := setupOrgPGService(t)
	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := newCtx(tenantID)
	actor := newActor(tenantID)

	nonexistentParent := uuid.New()
	before, err := svc.Count(ctx, nil)
	require.NoError(t, err)

	_, err = svc.Create(ctx, orgData(tenantID, map[string]any{
		"name": "Orphan", "code": "ORPH", "parent_id": nonexistentParent.String(),
	}), actor)
	require.Error(t, err, "create with a nonexistent parent_id must fail, not silently become a root")

	after, countErr := svc.Count(ctx, nil)
	require.NoError(t, countErr)
	assert.Equal(t, before, after, "the failed create must not leave a row behind (whole transaction rolled back)")
}

// --- Case 5: parent belonging to another tenant ------------------------------

func TestPathComputeHook_ParentInAnotherTenant_RejectedWithoutLeakingData(t *testing.T) {
	svc, pool := setupOrgPGService(t)
	tenantA := testdb.CreateTenant(t, pool, "ACTIVE")
	tenantB := testdb.CreateTenant(t, pool, "ACTIVE")

	testdb.ActivateTenant(t, pool, tenantB)
	ctxB := newCtx(tenantB)
	actorB := newActor(tenantB)
	foreignParent, err := svc.Create(ctxB, orgData(tenantB, map[string]any{"name": "Tenant B Root", "code": "TBROOT"}), actorB)
	require.NoError(t, err)

	testdb.ActivateTenant(t, pool, tenantA)
	ctxA := newCtx(tenantA)
	actorA := newActor(tenantA)

	before, err := svc.Count(ctxA, nil)
	require.NoError(t, err)

	_, err = svc.Create(ctxA, orgData(tenantA, map[string]any{
		"name": "Cross Tenant Child", "code": "XTC", "parent_id": foreignParent.ID.String(),
	}), actorA)
	require.Error(t, err, "a parent belonging to another tenant must be rejected, exactly like a missing parent")

	after, countErr := svc.Count(ctxA, nil)
	require.NoError(t, countErr)
	assert.Equal(t, before, after, "the rejected create must not leave a row behind in tenant A")
}

// --- Case 6: malformed parent_id (parent lookup can't even be attempted) ----

func TestPathComputeHook_MalformedParentID_RejectedSafely(t *testing.T) {
	svc, pool := setupOrgPGService(t)
	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := newCtx(tenantID)
	actor := newActor(tenantID)

	before, err := svc.Count(ctx, nil)
	require.NoError(t, err)

	_, err = svc.Create(ctx, orgData(tenantID, map[string]any{
		"name": "Malformed", "code": "MALF", "parent_id": "not-a-uuid",
	}), actor)
	require.Error(t, err, "a malformed parent_id must be rejected, not silently treated as absent (root)")

	after, countErr := svc.Count(ctx, nil)
	require.NoError(t, countErr)
	assert.Equal(t, before, after)
}

// --- Case 7: caller-supplied path/depth cannot override computed values ----

func TestPathComputeHook_CallerSuppliedPathAndDepth_Overridden(t *testing.T) {
	svc, pool := setupOrgPGService(t)
	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := newCtx(tenantID)
	actor := newActor(tenantID)

	created, err := svc.Create(ctx, orgData(tenantID, map[string]any{
		"name": "Attacker", "code": "ATK",
		"path": "/attacker-controlled/", "depth": 999,
	}), actor)
	require.NoError(t, err)

	wantPath := "/" + created.ID.String() + "/"
	assert.Equal(t, wantPath, created.Data["path"], "caller-supplied path must be overwritten by the computed value")
	assert.Equal(t, int64(0), created.Data["depth"], "caller-supplied depth must be overwritten by the computed value")

	fetched, err := svc.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, wantPath, fetched.Data["path"], "the persisted row must carry the computed path, not the caller-supplied one")
	assert.Equal(t, int32(0), fetched.Data["depth"])
}
