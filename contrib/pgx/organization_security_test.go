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
-- Mirrors generator.go's ScopeOrganization policy exactly:
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

// TestOrganizationRLS_SiblingOrgsIsolated proves that two organisations
// within the SAME tenant cannot see each other's ScopeOrganization rows —
// the "sibling isolation" property — using the real, generated RLS policy
// shape, independent of the (currently non-functional) OrganizationService.
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

	assert.Equal(t, []string{"A-confidential"}, namesVisibleTo(orgA),
		"Org A must see only its own row, never sibling Org B's, within the same tenant")
	assert.Equal(t, []string{"B-confidential"}, namesVisibleTo(orgB),
		"Org B must see only its own row, never sibling Org A's, within the same tenant")
}

// TestOrganizationRLS_NoOrgContext_SeesNothing proves the fail-safe default:
// a tenant-scoped connection with no organisation context established sees
// zero ScopeOrganization rows, not all of them.
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
	assert.False(t, rows.Next(), "no organisation context set must fail closed (zero rows), not open (all rows) — "+
		"the row inserted under Org A's context above must not be visible once that context has reset")
}
