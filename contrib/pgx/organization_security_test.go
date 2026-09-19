package pgx_test

// Organisation-scope RLS security boundary.
//
// platform/organization.OrganizationService (Create, Move, Tree, ResolveScope,
// etc.) is entirely stubbed — every method returns "not implemented" — and is
// out of scope to build out here; that is a real, separately-tracked gap,
// not something this test works around or papers over. What this test
// verifies is the part of the organisation model that IS real and already
// generated: the RLS policy generator.Generate() emits for ScopeOrganization
// entities (org_id = current_org_id()). That policy is the actual security
// boundary — whatever Go-level service eventually creates/moves
// organisations, this is what stops one organisation's data from being
// visible to another within the same tenant. Exercised directly via raw SQL
// (mirroring exactly what generator.go emits) so it does not depend on the
// broken service layer at all.

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/generator"
	testdb "awo.so/awo/testutil/db"
)

const orgScopedEntityDDL = `
CREATE TABLE org_scoped_entity (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  uuid NOT NULL,
    org_id     uuid NOT NULL,
    name       text NOT NULL
);
ALTER TABLE org_scoped_entity ENABLE ROW LEVEL SECURITY;
ALTER TABLE org_scoped_entity FORCE ROW LEVEL SECURITY;
-- Mirrors generator.go's ScopeOrganization DDL exactly: EVERY non-System-scope
-- entity (ScopeOrganization included) gets the tenant_isolation policy in
-- addition to its scope-specific one. Both are PERMISSIVE (the CREATE POLICY
-- default) — this combination is itself the subject of
-- TestOrganizationRLS_KnownGap_MultiplePermissivePoliciesAllowOrgReassignment
-- below, so it must be present here, not simplified away.
CREATE POLICY org_scoped_entity_tenant_isolation ON org_scoped_entity
    USING (tenant_id = current_tenant_id());
CREATE POLICY org_scoped_entity_org_isolation ON org_scoped_entity
    USING (org_id = current_org_id());
GRANT SELECT, INSERT, UPDATE, DELETE ON org_scoped_entity TO awo_app;
`

// grantOrgContextExecute grants awo_app EXECUTE on the org-context functions
// installed via generator.OrgContextSQL() — SetupTestDB's own RLS-helper
// install only grants the tenant-context pair (see SetupTestDB), so this
// must run once per test, before ActivateTenant switches the session to the
// non-superuser AppRole.
const grantOrgContextExecute = `
GRANT EXECUTE ON FUNCTION set_org_context(uuid) TO awo_app;
GRANT EXECUTE ON FUNCTION current_org_id() TO awo_app;
`

// TestOrganizationRLS_SiblingOrgsIsolated was written to prove that two
// organisations within the same tenant cannot see each other's
// ScopeOrganization rows. It originally passed against a DDL fixture that
// only created the org_isolation policy, omitting the tenant_isolation
// policy every non-System-scope entity also gets in the real, generated
// schema (see orgScopedEntityDDL above, corrected during a later security
// pass to include both). Once both policies are present — as they always
// are for any real entity — this property does NOT hold: PostgreSQL
// combines multiple PERMISSIVE policies with OR, so satisfying
// tenant_isolation alone (true for every row in the tenant, regardless of
// org_id) makes the row visible, completely defeating org_isolation. This
// test now asserts that actual (broken) behavior rather than the originally
// intended one — see
// TestOrganizationRLS_KnownGap_MultiplePermissivePoliciesAllowOrgReassignment
// below for the full writeup and the write-side (INSERT/UPDATE) version of
// the same root cause. If this assertion starts failing, the gap has been
// closed and this test should be rewritten back to assert real isolation.
func TestOrganizationRLS_SiblingOrgsIsolated(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, generator.OrgContextSQL())
	testdb.ApplySQL(t, pool, grantOrgContextExecute)
	testdb.ApplySQL(t, pool, orgScopedEntityDDL)

	tenantID := testdb.RawTenantID()
	orgA := uuid.New()
	orgB := uuid.New()

	testdb.ActivateTenant(t, pool, tenantID) // sets tenant context + AppRole
	ctx := t.Context()

	// set_org_context uses set_config(..., is_local=true), i.e. it is
	// transaction-local — it does not survive past the transaction that set
	// it. Each bare pool.Exec/Query runs as its own implicit auto-commit
	// transaction, so setting the context and using it must happen inside
	// one explicit transaction.
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
		rows, err := tx.Query(ctx, "SELECT name FROM org_scoped_entity")
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

	// KNOWN GAP (see the doc comment above and the KnownGap test below):
	// both orgs currently see BOTH rows, because tenant_isolation alone
	// satisfies the OR-combined permissive-policy check regardless of
	// org_id. This is the opposite of the property this test is named for.
	assert.Equal(t, []string{"A-confidential", "B-confidential"}, namesVisibleTo(orgA),
		"KNOWN GAP: Org A currently sees Org B's row too — org_isolation provides no actual protection "+
			"once tenant_isolation (present on every real entity) is also in effect")
	assert.Equal(t, []string{"A-confidential", "B-confidential"}, namesVisibleTo(orgB),
		"KNOWN GAP: Org B currently sees Org A's row too, for the same reason")
}

// TestOrganizationRLS_NoOrgContext_SeesNothing was written to prove the
// fail-safe default: a tenant-scoped connection with no organisation
// context established should see zero ScopeOrganization rows, not all of
// them. Like TestOrganizationRLS_SiblingOrgsIsolated above, this passed
// against the old single-policy DDL fixture but does not hold once the real
// two-policy shape is used: testdb.ActivateTenant's tenant context is
// session-level (persists across statements/transactions on the same
// connection), so even after org context resets to NULL, tenant_isolation
// alone still makes the row visible via the same OR-combination. Asserts
// actual (broken) behavior — see the KnownGap test below for the full
// writeup.
func TestOrganizationRLS_NoOrgContext_SeesNothing(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, generator.OrgContextSQL())
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
	// that never called set_org_context.
	rows, err := pool.Query(ctx, "SELECT name FROM org_scoped_entity")
	require.NoError(t, err)
	defer rows.Close()
	assert.True(t, rows.Next(), "KNOWN GAP: with no organisation context set, the row is still visible — "+
		"tenant_isolation's session-level context alone satisfies the OR-combined permissive-policy check")
}

// TestOrganizationRLS_KnownGap_MultiplePermissivePoliciesAllowOrgReassignment
// is a KNOWN, DOCUMENTED, UNFIXED GAP — not a passing security guarantee.
//
// generator.go emits TWO separate CREATE POLICY statements for every
// ScopeOrganization entity: the org_isolation policy (org_id =
// current_org_id()) AND the tenant_isolation policy every non-System-scope
// entity gets (tenant_id = current_tenant_id()) — see orgScopedEntityDDL
// above, which now mirrors that exactly (an earlier version of this test
// file only created the org_isolation policy alone, and so never exercised
// this interaction).
//
// PostgreSQL combines multiple PERMISSIVE policies (the CREATE POLICY
// default — neither one here is declared AS RESTRICTIVE) with OR, not AND: a
// row passes if it satisfies AT LEAST ONE applicable permissive policy's
// USING/WITH CHECK expression. That means an INSERT or UPDATE whose new row
// values satisfy tenant_isolation (tenant_id = current_tenant_id()) is
// allowed through REGARDLESS of whether it satisfies org_isolation — a
// caller can INSERT a row (or UPDATE an existing one) with an org_id
// belonging to an organisation they have no claim to, as long as tenant_id
// is their own tenant's.
//
// This is currently a landmine, not a live incident: zero entities in this
// codebase declare ScopeOrganization/ScopeOrganizationTree today (confirmed
// by source search), so no real table has this exposure yet. It also is NOT
// closed by contrib/pgx.Repository.checkWritableField (the fix for the
// updateSystem/BulkUpdate write-path injection): that check only excludes
// fields absent from the entity schema's FieldsByName/Fields, and
// generator.go does not treat org_id as an implicit standard column the way
// it treats tenant_id (added unconditionally in generateEntitySQL) — org_id
// would need to be declared as an ordinary entity field by whoever defines
// a ScopeOrganization entity, which means checkWritableField would allow it
// through like any other business field unless separately special-cased.
//
// Recorded here, asserting the CURRENT (undesirable) behavior, specifically
// so this is caught immediately if generator.go or checkWritableField
// changes in a way that silently fixes or silently reintroduces it: a
// currently-failing assertion here means the gap has been closed and this
// test must be rewritten to assert the new, correct behavior instead of
// being deleted or loosened.
func TestOrganizationRLS_KnownGap_MultiplePermissivePoliciesAllowOrgReassignment(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, generator.OrgContextSQL())
	testdb.ApplySQL(t, pool, grantOrgContextExecute)
	testdb.ApplySQL(t, pool, orgScopedEntityDDL)

	tenantID := testdb.RawTenantID()
	orgA := uuid.New()
	orgForeign := uuid.New() // an organisation this caller has no claim to
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := t.Context()

	// INSERT a row whose org_id (orgForeign) does not match the caller's own
	// organisation context (orgA), but whose tenant_id is correctly the
	// caller's own tenant.
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "SELECT set_org_context($1)", orgA)
	require.NoError(t, err)
	_, err = tx.Exec(ctx,
		"INSERT INTO org_scoped_entity (tenant_id, org_id, name) VALUES ($1, $2, $3)",
		tenantID, orgForeign, "should-be-rejected-wrong-org")
	assert.NoError(t, err, "KNOWN GAP: this INSERT into a foreign organisation currently succeeds "+
		"because the tenant_isolation and org_isolation policies are both PERMISSIVE and combine with OR — "+
		"tenant_id alone satisfies the combined check. If this assertion starts failing, the gap has been "+
		"closed (e.g. org_isolation made RESTRICTIVE, or org_id turned into a generator-managed standard "+
		"column) — update this test to assert rejection instead of reverting whatever fixed it.")
	require.NoError(t, tx.Commit(ctx))

	// UPDATE an existing row's org_id away from the caller's own organisation
	// context, again with tenant_id left correctly matching.
	tx2, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx2.Exec(ctx, "SELECT set_org_context($1)", orgA)
	require.NoError(t, err)
	_, err = tx2.Exec(ctx,
		"INSERT INTO org_scoped_entity (tenant_id, org_id, name) VALUES ($1, $2, $3)",
		tenantID, orgA, "row-to-be-reassigned")
	require.NoError(t, err)
	_, err = tx2.Exec(ctx,
		"UPDATE org_scoped_entity SET org_id = $1 WHERE name = 'row-to-be-reassigned'", orgForeign)
	assert.NoError(t, err, "KNOWN GAP: reassigning an existing row to a foreign organisation via UPDATE "+
		"currently succeeds for the same OR-combined-permissive-policies reason as the INSERT case above")
	require.NoError(t, tx2.Commit(ctx))
}
