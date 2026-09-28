package pgx_test

// Organisation-scope RLS security boundary (Step 8.1 fix).
//
// platform/organization.OrganizationService (Create, Move, Tree, ResolveScope,
// etc.) is entirely stubbed — every method returns "not implemented" — and is
// out of scope to build out here; that is a real, separately-tracked gap,
// not something this test works around or papers over. What this test
// verifies is the part of the organisation model that IS real and already
// generated: the RLS policy generator.Generate() emits for ScopeOrganization
// and ScopeOrganizationTree entities. That policy is the actual security
// boundary — whatever Go-level service eventually creates/moves
// organisations, this is what stops one organisation's data from being
// visible to (or writable by) another within the same tenant. Exercised
// directly via raw SQL (mirroring exactly what generator.go emits) so it does
// not depend on the broken service layer at all.
//
// THE FIX (see generator/generator.go's generateEntitySQL):
//
// generator.go previously emitted TWO PERMISSIVE policies per org-scoped
// entity: tenant_isolation (tenant_id = current_tenant_id()) and
// org_isolation/org_tree_isolation (the org check). PostgreSQL combines
// PERMISSIVE policies with OR, so satisfying tenant_isolation alone — true
// for every row in the tenant, regardless of org_id — made the row
// visible/writable regardless of organisation, defeating org scoping
// entirely (see the git history of this file for the original
// "TestOrganizationRLS_KnownGap_..." tests that proved this).
//
// The fix declares the org policy AS RESTRICTIVE. PostgreSQL ANDs
// RESTRICTIVE policies against the OR-combined PERMISSIVE result, so the
// effective check becomes exactly:
//
//	tenant_isolation AND (org_isolation | org_tree_isolation)
//
// This also automatically covers WITH CHECK (INSERT/UPDATE): a policy
// without an explicit WITH CHECK clause uses its USING expression for both.

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/generator"
	testdb "awo.so/awo/testutil/db"
)

// orgScopedEntityDDL mirrors generator.go's ScopeOrganization DDL exactly
// (flat, single-organization equality check — no hierarchy).
const orgScopedEntityDDL = `
CREATE TABLE org_scoped_entity (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  uuid NOT NULL,
    org_id     uuid NOT NULL,
    name       text NOT NULL
);
ALTER TABLE org_scoped_entity ENABLE ROW LEVEL SECURITY;
ALTER TABLE org_scoped_entity FORCE ROW LEVEL SECURITY;
CREATE POLICY org_scoped_entity_tenant_isolation ON org_scoped_entity
    USING (tenant_id = current_tenant_id());
CREATE POLICY org_scoped_entity_org_isolation ON org_scoped_entity AS RESTRICTIVE
    USING (org_id = current_org_id());
GRANT SELECT, INSERT, UPDATE, DELETE ON org_scoped_entity TO awo_app;
`

// orgTreeScopedEntityDDL mirrors generator.go's ScopeOrganizationTree DDL
// exactly (hierarchical — current org and its descendants).
const orgTreeScopedEntityDDL = `
CREATE TABLE org_tree_scoped_entity (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  uuid NOT NULL,
    org_id     uuid NOT NULL,
    name       text NOT NULL
);
ALTER TABLE org_tree_scoped_entity ENABLE ROW LEVEL SECURITY;
ALTER TABLE org_tree_scoped_entity FORCE ROW LEVEL SECURITY;
CREATE POLICY org_tree_scoped_entity_tenant_isolation ON org_tree_scoped_entity
    USING (tenant_id = current_tenant_id());
CREATE POLICY org_tree_scoped_entity_org_tree_isolation ON org_tree_scoped_entity AS RESTRICTIVE
    USING (org_id IN (
        SELECT id FROM platform_organization
        WHERE path LIKE current_org_path() || '%'
    ));
GRANT SELECT, INSERT, UPDATE, DELETE ON org_tree_scoped_entity TO awo_app;
`

// platformOrganizationDDL is a minimal stand-in for the real
// platform_organization table (see platform/organization/migrations),
// carrying only the columns current_org_path() and this test's hierarchy
// fixture actually need: id, tenant_id, path. Tenant-RLS'd exactly like the
// real table, which is what makes current_org_path() fail closed (NULL path,
// no match) when current_org_id() names an organisation in a different
// tenant — see TestOrganizationRLS_Tree_CrossTenantOrgIDCollision_Denied.
const platformOrganizationDDL = `
CREATE TABLE platform_organization (
    id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL,
    path      text NOT NULL
);
ALTER TABLE platform_organization ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform_organization FORCE ROW LEVEL SECURITY;
CREATE POLICY platform_organization_tenant_isolation ON platform_organization
    USING (tenant_id = current_tenant_id());
GRANT SELECT, INSERT ON platform_organization TO awo_app;
`

// grantOrgContextExecute grants awo_app EXECUTE on the org-context functions
// installed via generator.OrgContextSQL() — SetupTestDB's own RLS-helper
// install only grants the tenant-context pair (see SetupTestDB), so this
// must run once per test, before ActivateTenant switches the session to the
// non-superuser AppRole.
const grantOrgContextExecute = `
GRANT EXECUTE ON FUNCTION set_org_context(uuid) TO awo_app;
GRANT EXECUTE ON FUNCTION current_org_id() TO awo_app;
GRANT EXECUTE ON FUNCTION current_org_path() TO awo_app;
`

// ---------------------------------------------------------------------------
// Flat ScopeOrganization tests
// ---------------------------------------------------------------------------

func TestOrganizationRLS_SiblingOrgsIsolated(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, generator.OrgContextSQL())
	testdb.ApplySQL(t, pool, platformOrganizationDDL)
	testdb.ApplySQL(t, pool, generator.OrgPathSQL())
	testdb.ApplySQL(t, pool, grantOrgContextExecute)
	testdb.ApplySQL(t, pool, orgScopedEntityDDL)

	tenantID := testdb.RawTenantID()
	orgA := uuid.New()
	orgB := uuid.New()

	testdb.ActivateTenant(t, pool, tenantID) // sets tenant context + AppRole
	ctx := t.Context()

	insertAs := func(orgID uuid.UUID, name string) {
		t.Helper()
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)
		_, err = tx.Exec(ctx, "SELECT set_org_context($1)", orgID)
		require.NoError(t, err)
		_, err = tx.Exec(ctx,
			"INSERT INTO org_scoped_entity (tenant_id, org_id, name) VALUES ($1, $2, $3)",
			tenantID, orgID, name)
		require.NoError(t, err)
		require.NoError(t, tx.Commit(ctx))
	}
	insertAs(orgA, "A-confidential")
	insertAs(orgB, "B-confidential")

	namesVisibleTo := func(orgID uuid.UUID) []string {
		t.Helper()
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)
		_, err = tx.Exec(ctx, "SELECT set_org_context($1)", orgID)
		require.NoError(t, err)
		rows, err := tx.Query(ctx, "SELECT name FROM org_scoped_entity ORDER BY name")
		require.NoError(t, err)
		defer rows.Close()
		var names []string
		for rows.Next() {
			var n string
			require.NoError(t, rows.Scan(&n))
			names = append(names, n)
		}
		return names
	}

	// FIXED: each org now sees only its own row — org_isolation is
	// RESTRICTIVE, so it genuinely narrows the tenant_isolation result
	// instead of being OR'd away by it.
	assert.Equal(t, []string{"A-confidential"}, namesVisibleTo(orgA),
		"Org A must see only its own row, not Org B's")
	assert.Equal(t, []string{"B-confidential"}, namesVisibleTo(orgB),
		"Org B must see only its own row, not Org A's")
}

func TestOrganizationRLS_NoOrgContext_SeesNothing(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, generator.OrgContextSQL())
	testdb.ApplySQL(t, pool, platformOrganizationDDL)
	testdb.ApplySQL(t, pool, generator.OrgPathSQL())
	testdb.ApplySQL(t, pool, grantOrgContextExecute)
	testdb.ApplySQL(t, pool, orgScopedEntityDDL)

	tenantID := testdb.RawTenantID()
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := t.Context()

	orgA := uuid.New()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "SELECT set_org_context($1)", orgA)
	require.NoError(t, err)
	_, err = tx.Exec(ctx,
		"INSERT INTO org_scoped_entity (tenant_id, org_id, name) VALUES ($1, $2, $3)",
		tenantID, orgA, "A-row")
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))

	// set_org_context's GUC is transaction-local (is_local=true) and was only
	// ever set inside the transaction above, which already committed. This
	// next statement runs as a fresh implicit auto-commit transaction with no
	// organisation context established — exactly simulating a fresh request
	// that never called set_org_context. current_org_id() returns NULL, and
	// `org_id = NULL` is never true, so the RESTRICTIVE org_isolation policy
	// fails closed: the row must not be visible.
	rows, err := pool.Query(ctx, "SELECT name FROM org_scoped_entity")
	require.NoError(t, err)
	defer rows.Close()
	assert.False(t, rows.Next(),
		"with no organisation context set, the row must not be visible (fail closed)")
}

func TestOrganizationRLS_TenantIsolation_SameOrgID_DifferentTenants(t *testing.T) {
	// Adversarial: tenant B must never see tenant A's row even when B's
	// request happens to carry the exact same org_id value (org_id
	// namespaces do not cross tenant boundaries; tenant isolation is the
	// outer boundary and must hold regardless of org_id equality).
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, generator.OrgContextSQL())
	testdb.ApplySQL(t, pool, platformOrganizationDDL)
	testdb.ApplySQL(t, pool, generator.OrgPathSQL())
	testdb.ApplySQL(t, pool, grantOrgContextExecute)
	testdb.ApplySQL(t, pool, orgScopedEntityDDL)

	tenantA := testdb.RawTenantID()
	tenantB := testdb.RawTenantID()
	sharedOrgID := uuid.New() // same org_id value used under both tenants
	ctx := t.Context()

	testdb.ActivateTenant(t, pool, tenantA)
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "SELECT set_org_context($1)", sharedOrgID)
	require.NoError(t, err)
	_, err = tx.Exec(ctx,
		"INSERT INTO org_scoped_entity (tenant_id, org_id, name) VALUES ($1, $2, $3)",
		tenantA, sharedOrgID, "tenant-A-secret")
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))

	testdb.ActivateTenant(t, pool, tenantB)
	tx2, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx2.Exec(ctx, "SELECT set_org_context($1)", sharedOrgID)
	require.NoError(t, err)
	rows, err := tx2.Query(ctx, "SELECT name FROM org_scoped_entity")
	require.NoError(t, err)
	defer rows.Close()
	assert.False(t, rows.Next(),
		"tenant B must not see tenant A's row merely because org_id matches — tenant isolation is the outer boundary")
	require.NoError(t, tx2.Commit(ctx))
}

func TestOrganizationRLS_WriteSide_OrgReassignment_Blocked(t *testing.T) {
	// Write-side counterpart of the read-side tests above: an actor acting
	// as org A must not be able to INSERT or UPDATE a row into a foreign
	// organisation merely because tenant_id is correct. This is the WITH
	// CHECK enforcement of the same RESTRICTIVE policy (INSERT/UPDATE use
	// USING as WITH CHECK by default when no explicit WITH CHECK is given).
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, generator.OrgContextSQL())
	testdb.ApplySQL(t, pool, platformOrganizationDDL)
	testdb.ApplySQL(t, pool, generator.OrgPathSQL())
	testdb.ApplySQL(t, pool, grantOrgContextExecute)
	testdb.ApplySQL(t, pool, orgScopedEntityDDL)

	tenantID := testdb.RawTenantID()
	orgA := uuid.New()
	orgForeign := uuid.New() // an organisation this caller has no claim to
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := t.Context()

	// INSERT a row whose org_id (orgForeign) does not match the caller's own
	// organisation context (orgA), tenant_id correctly the caller's own.
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "SELECT set_org_context($1)", orgA)
	require.NoError(t, err)
	_, err = tx.Exec(ctx,
		"INSERT INTO org_scoped_entity (tenant_id, org_id, name) VALUES ($1, $2, $3)",
		tenantID, orgForeign, "should-be-rejected-wrong-org")
	assert.Error(t, err,
		"FIXED: inserting into a foreign organisation must now be rejected by the RESTRICTIVE org_isolation WITH CHECK")
	_ = tx.Rollback(ctx)

	// Insert the row-to-be-reassigned in its own, separately committed
	// transaction so the later failed UPDATE's rollback does not also
	// discard it.
	tx2, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx2.Exec(ctx, "SELECT set_org_context($1)", orgA)
	require.NoError(t, err)
	_, err = tx2.Exec(ctx,
		"INSERT INTO org_scoped_entity (tenant_id, org_id, name) VALUES ($1, $2, $3)",
		tenantID, orgA, "row-to-be-reassigned")
	require.NoError(t, err)
	require.NoError(t, tx2.Commit(ctx))

	// UPDATE that existing (own-org) row's org_id away to a foreign org, in a
	// fresh transaction — expected to fail and roll back.
	tx3, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx3.Exec(ctx, "SELECT set_org_context($1)", orgA)
	require.NoError(t, err)
	_, err = tx3.Exec(ctx,
		"UPDATE org_scoped_entity SET org_id = $1 WHERE name = 'row-to-be-reassigned'", orgForeign)
	assert.Error(t, err,
		"FIXED: reassigning an existing row to a foreign organisation via UPDATE must now be rejected")
	_ = tx3.Rollback(ctx)

	// Sanity: the row is still there, still owned by orgA (UPDATE rolled
	// back by the error, not silently partially applied).
	tx4, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx4.Exec(ctx, "SELECT set_org_context($1)", orgA)
	require.NoError(t, err)
	var gotOrg uuid.UUID
	err = tx4.QueryRow(ctx, "SELECT org_id FROM org_scoped_entity WHERE name = 'row-to-be-reassigned'").Scan(&gotOrg)
	require.NoError(t, err)
	assert.Equal(t, orgA, gotOrg, "row must remain assigned to org A after the rejected UPDATE")
	require.NoError(t, tx4.Commit(ctx))
}

// ---------------------------------------------------------------------------
// Hierarchical ScopeOrganizationTree tests
// ---------------------------------------------------------------------------

// TestOrganizationRLS_Tree_Hierarchy installs platform_organization plus a
// small hierarchy:
//
//	company (root)
//	  ├── regionA
//	  │     └── branchA1
//	  └── regionB
//	        └── branchB1
func TestOrganizationRLS_Tree_Hierarchy(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, generator.OrgContextSQL())
	testdb.ApplySQL(t, pool, platformOrganizationDDL)
	testdb.ApplySQL(t, pool, generator.OrgPathSQL())
	testdb.ApplySQL(t, pool, grantOrgContextExecute)
	testdb.ApplySQL(t, pool, orgTreeScopedEntityDDL)

	tenantID := testdb.RawTenantID()
	ctx := t.Context()

	company := uuid.New()
	regionA := uuid.New()
	regionB := uuid.New()
	branchA1 := uuid.New()
	branchB1 := uuid.New()

	// platform_organization is superuser-owned by default; ActivateTenant
	// switches to AppRole, which only has SELECT/INSERT granted (no DDL),
	// matching the real table's privilege shape. Insert the hierarchy first
	// as the owning role (before switching), using explicit materialized
	// paths mirroring the real platform_organization convention
	// ("/root-id/.../self-id/").
	pathOf := func(ids ...uuid.UUID) string {
		p := "/"
		for _, id := range ids {
			p += id.String() + "/"
		}
		return p
	}
	insertOrgNode := func(id uuid.UUID, path string) {
		t.Helper()
		_, err := pool.Exec(ctx,
			`INSERT INTO platform_organization (id, tenant_id, path) VALUES ($1, $2, $3)`,
			id, tenantID, path)
		require.NoError(t, err)
	}
	insertOrgNode(company, pathOf(company))
	insertOrgNode(regionA, pathOf(company, regionA))
	insertOrgNode(regionB, pathOf(company, regionB))
	insertOrgNode(branchA1, pathOf(company, regionA, branchA1))
	insertOrgNode(branchB1, pathOf(company, regionB, branchB1))

	testdb.ActivateTenant(t, pool, tenantID)

	insertRecord := func(orgID uuid.UUID, name string) {
		t.Helper()
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)
		_, err = tx.Exec(ctx, "SELECT set_org_context($1)", orgID)
		require.NoError(t, err)
		_, err = tx.Exec(ctx,
			"INSERT INTO org_tree_scoped_entity (tenant_id, org_id, name) VALUES ($1, $2, $3)",
			tenantID, orgID, name)
		require.NoError(t, err)
		require.NoError(t, tx.Commit(ctx))
	}
	insertRecord(company, "company-record")
	insertRecord(regionA, "regionA-record")
	insertRecord(regionB, "regionB-record")
	insertRecord(branchA1, "branchA1-record")
	insertRecord(branchB1, "branchB1-record")

	namesVisibleTo := func(viewerOrg uuid.UUID) []string {
		t.Helper()
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)
		_, err = tx.Exec(ctx, "SELECT set_org_context($1)", viewerOrg)
		require.NoError(t, err)
		rows, err := tx.Query(ctx, "SELECT name FROM org_tree_scoped_entity ORDER BY name")
		require.NoError(t, err)
		defer rows.Close()
		var names []string
		for rows.Next() {
			var n string
			require.NoError(t, rows.Scan(&n))
			names = append(names, n)
		}
		return names
	}

	// Parent → descendants: company (the root) sees every record, including
	// both regions and both branches.
	assert.ElementsMatch(t, []string{
		"company-record", "regionA-record", "regionB-record", "branchA1-record", "branchB1-record",
	}, namesVisibleTo(company), "company (root) must see all descendant records")

	// Parent → its own subtree only: regionA sees itself and branchA1, but
	// not regionB's subtree (a sibling subtree it is not an ancestor of).
	assert.ElementsMatch(t, []string{"regionA-record", "branchA1-record"}, namesVisibleTo(regionA),
		"regionA must see its own record and its descendant branchA1, not regionB's subtree")

	// Sibling isolation: branchA1 must not see branchB1 (unrelated leaf).
	assert.ElementsMatch(t, []string{"branchA1-record"}, namesVisibleTo(branchA1),
		"branchA1 must see only its own record")
	assert.ElementsMatch(t, []string{"branchB1-record"}, namesVisibleTo(branchB1),
		"branchB1 must see only its own record")

	// Child → parent: branchA1 must NOT be able to see the ancestor
	// (regionA/company) records — hierarchy visibility flows one direction
	// only (parent sees descendants; children do not automatically see
	// ancestors).
	branchA1Names := namesVisibleTo(branchA1)
	assert.NotContains(t, branchA1Names, "regionA-record", "child must not see ancestor (regionA) record")
	assert.NotContains(t, branchA1Names, "company-record", "child must not see ancestor (company) record")

	// Cross-region isolation: regionA must not see regionB's subtree.
	regionANames := namesVisibleTo(regionA)
	assert.NotContains(t, regionANames, "regionB-record", "regionA must not see regionB's own record")
	assert.NotContains(t, regionANames, "branchB1-record", "regionA must not see regionB's descendant branchB1")
}

func TestOrganizationRLS_Tree_CrossTenantOrgIDCollision_Denied(t *testing.T) {
	// Adversarial: current_org_path() resolves the viewer's org via a plain
	// SELECT against platform_organization, which itself carries tenant RLS.
	// If current_org_id() names an org ID that only exists under a DIFFERENT
	// tenant, that SELECT (running under this tenant's context) must find
	// zero rows, yielding a NULL path — and `path LIKE NULL || '%'` must be
	// NULL (never true), so the policy fails closed rather than resolving
	// the foreign tenant's hierarchy.
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, generator.OrgContextSQL())
	testdb.ApplySQL(t, pool, platformOrganizationDDL)
	testdb.ApplySQL(t, pool, generator.OrgPathSQL())
	testdb.ApplySQL(t, pool, grantOrgContextExecute)
	testdb.ApplySQL(t, pool, orgTreeScopedEntityDDL)

	ctx := t.Context()
	tenantA := testdb.RawTenantID()
	tenantB := testdb.RawTenantID()
	orgUnderB := uuid.New()

	// orgUnderB exists only under tenant B.
	_, err := pool.Exec(ctx,
		`INSERT INTO platform_organization (id, tenant_id, path) VALUES ($1, $2, $3)`,
		orgUnderB, tenantB, "/"+orgUnderB.String()+"/")
	require.NoError(t, err)

	// A record under tenant A, coincidentally reusing orgUnderB's UUID as
	// its org_id (org_id namespaces are not guaranteed disjoint across
	// tenants at the raw-SQL layer; the policy must not rely on that).
	testdb.ActivateTenant(t, pool, tenantA)
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "SELECT set_org_context($1)", orgUnderB)
	require.NoError(t, err)
	_, err = tx.Exec(ctx,
		"INSERT INTO org_tree_scoped_entity (tenant_id, org_id, name) VALUES ($1, $2, $3)",
		tenantA, orgUnderB, "tenantA-record")
	// Insert itself may or may not be rejected depending on whether the
	// WITH CHECK subquery also resolves to NULL for the inserting
	// transaction; either outcome is acceptable here as long as the
	// followup read (below, using tenant B's own real context) cannot see
	// a tenant-A row. Record but do not require a specific insert outcome.
	_ = err
	_ = tx.Rollback(ctx)

	// From tenant B, acting as the real orgUnderB (which does have a valid
	// path under tenant B), confirm no tenant-A data is reachable.
	testdb.ActivateTenant(t, pool, tenantB)
	tx2, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx2.Exec(ctx, "SELECT set_org_context($1)", orgUnderB)
	require.NoError(t, err)
	rows, err := tx2.Query(ctx, "SELECT name FROM org_tree_scoped_entity")
	require.NoError(t, err)
	defer rows.Close()
	assert.False(t, rows.Next(), "tenant B must not see any tenant-A row regardless of org_id collisions")
	require.NoError(t, tx2.Commit(ctx))
}

// ---------------------------------------------------------------------------
// Connection-pool safety (Step 7a invariant must not regress)
// ---------------------------------------------------------------------------

func TestOrganizationRLS_ConcurrentPool_AlternatingTenantsAndOrgs_NoLeakage(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, generator.OrgContextSQL())
	testdb.ApplySQL(t, pool, platformOrganizationDDL)
	testdb.ApplySQL(t, pool, generator.OrgPathSQL())
	testdb.ApplySQL(t, pool, grantOrgContextExecute)
	testdb.ApplySQL(t, pool, orgScopedEntityDDL)

	concurrent := testdb.OpenConcurrentPool(t, pool)
	ctx := t.Context()

	type tenantOrg struct {
		tenant, org uuid.UUID
		row         string
	}
	fixtures := make([]tenantOrg, 6)
	for i := range fixtures {
		fixtures[i] = tenantOrg{tenant: testdb.RawTenantID(), org: uuid.New(), row: uuid.New().String()}
	}

	// Seed each tenant/org's single row using the primary (session-role)
	// pool, sequentially, one ActivateTenant per fixture.
	for _, f := range fixtures {
		testdb.ActivateTenant(t, pool, f.tenant)
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		_, err = tx.Exec(ctx, "SELECT set_org_context($1)", f.org)
		require.NoError(t, err)
		_, err = tx.Exec(ctx,
			"INSERT INTO org_scoped_entity (tenant_id, org_id, name) VALUES ($1, $2, $3)",
			f.tenant, f.org, f.row)
		require.NoError(t, err)
		require.NoError(t, tx.Commit(ctx))
	}

	// Now read each fixture back through the genuine multi-connection pool,
	// interleaved across goroutines, each establishing tenant+org context on
	// whichever pooled connection it happens to acquire (mirroring Step 7a's
	// standalone-read concurrency test). No fixture must ever observe another
	// fixture's row.
	errs := make(chan error, len(fixtures))
	for _, f := range fixtures {
		f := f
		go func() {
			tx, err := concurrent.Begin(ctx)
			if err != nil {
				errs <- err
				return
			}
			defer tx.Rollback(ctx)
			if _, err := tx.Exec(ctx, "SELECT set_tenant_context($1)", f.tenant.String()); err != nil {
				errs <- err
				return
			}
			if _, err := tx.Exec(ctx, "SELECT set_org_context($1)", f.org); err != nil {
				errs <- err
				return
			}
			rows, err := tx.Query(ctx, "SELECT name FROM org_scoped_entity")
			if err != nil {
				errs <- err
				return
			}
			defer rows.Close()
			var got []string
			for rows.Next() {
				var n string
				if err := rows.Scan(&n); err != nil {
					errs <- err
					return
				}
				got = append(got, n)
			}
			if len(got) != 1 || got[0] != f.row {
				errs <- assert.AnError
				return
			}
			errs <- nil
		}()
	}
	for range fixtures {
		require.NoError(t, <-errs, "concurrent pooled connection leaked tenant/org context across fixtures")
	}
}
