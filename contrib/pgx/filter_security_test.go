package pgx_test

// Adversarial coverage for the query/filter layer: no application-level
// filter, sort field, or pagination value may ever reach raw SQL
// unescaped or unchecked, and RLS (not the application-level WHERE clause)
// remains the actual security boundary for tenant isolation even when the
// application layer's own defenses are attacked directly.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/driver"
	"awo.so/awo/filter"
	testdb "awo.so/awo/testutil/db"
)

// TestQuery_SortField_RejectsSQLInjectionAttempt is the regression test for
// a real SQL injection vector: Query built its ORDER BY clause as
// fmt.Sprintf(`ORDER BY "%s" %s`, qo.SortField, dir) — unlike every WHERE-
// clause field reference (which goes through quoteIdent, correctly doubling
// embedded quote characters), SortField was interpolated with no escaping
// at all. A value containing a literal `"` breaks out of the quoted
// identifier and injects arbitrary SQL into the executed statement.
// api/handler/crud.go passes the raw `?orderBy=` HTTP query parameter
// straight through to this code path with no validation, making this
// remotely exploitable by any caller who can hit the standard List endpoint.
func TestQuery_SortField_RejectsSQLInjectionAttempt(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, testEntityDDL)

	tenantID := testdb.RawTenantID()
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := testdb.WithTenant(context.Background(), tenantID)
	repo := newRepo(pool)

	_, err := repo.Create(ctx, driver.CreateInput{Data: map[string]any{"name": "row1"}})
	require.NoError(t, err)

	// This exact payload is the sharpest proof of the vulnerability: against
	// the unfixed code (fmt.Sprintf(`ORDER BY "%s" %s`, SortField, dir), no
	// escaping), it produces `ORDER BY "name" --" ASC` — a SQL line comment
	// that silently swallows the direction clause and the trailing quote,
	// leaving a perfectly valid, successfully-executing query. No syntax
	// error, no crash — just a query that silently did something other than
	// what the field-name validation should have allowed, which is exactly
	// what makes this class of bug dangerous: it doesn't fail loudly, it
	// just quietly accepts attacker-controlled SQL fragments. A correct
	// fix rejects this string outright as "not a real column" before it
	// ever reaches SQL.
	malicious := `name" --`
	_, _, err = repo.Query(ctx, nil, driver.WithSort(malicious, true))
	require.Error(t, err, "a sort field containing raw SQL metacharacters must be rejected "+
		"before reaching the database — the unfixed code accepts this payload silently "+
		"(no error) because the comment swallows the syntax break")
}

// TestQuery_SortField_AdversarialPayloads_AllRejected enumerates the wider
// attack-shape catalogue for the ORDER BY allowlist fix — function calls,
// subqueries, expression injection, comment injection, quoted-identifier
// breakout, case/whitespace variants, and a stacked-statement attempt. Every
// one of these is expected to be rejected the same way: sqlbuild.Allowlist.Check
// is an exact map-membership test against the entity's declared field names,
// so any string that isn't byte-for-byte "name" or "code" fails identically
// — but each shape is exercised individually against a real Query() call
// rather than inferred, since the goal is to prove the *behavior*, not the
// mechanism.
func TestQuery_SortField_AdversarialPayloads_AllRejected(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, testEntityDDL)

	tenantID := testdb.RawTenantID()
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := testdb.WithTenant(context.Background(), tenantID)
	repo := newRepo(pool)

	_, err := repo.Create(ctx, driver.CreateInput{Data: map[string]any{"name": "row1", "code": "c1"}})
	require.NoError(t, err)

	payloads := map[string]string{
		"function call":                 `pg_sleep(5)`,
		"function call wrapping column": `lower(name)`,
		"subquery":                      `(SELECT password_hash FROM iam_users LIMIT 1)`,
		"expression injection":          `name || code`,
		"comment injection":             `name --`,
		"comment injection block":       `name /*`,
		"quoted identifier breakout":    `name" = 'x'; --`,
		"stacked statement attempt":     `name; DROP TABLE test_entity; --`,
		"case variant":                  `NAME`,
		"leading whitespace":            ` name`,
		"trailing whitespace":           `name `,
		"comma-separated multi-field":   `name, code`,
		"embedded direction syntax":     `name ASC`,
		"embedded direction lowercase":  `name desc`,
		"leading dash (rails-style)":    `-name`,
		"unicode homoglyph":             "namе", // Cyrillic 'е' instead of Latin 'e'
		"null byte":                     "name\x00",
		"nested quotes":                 `na""me`,
		"empty after trim look-alike":   `  `,
	}
	for label, payload := range payloads {
		t.Run(label, func(t *testing.T) {
			_, _, err := repo.Query(ctx, nil, driver.WithSort(payload, true))
			assert.Error(t, err, "sort field payload (%s) %q must be rejected, not reach SQL", label, payload)
		})
	}
}

// TestQuery_SortField_UnknownColumn_Rejected proves the fix is a real
// allowlist against the entity's actual columns, not merely quote-escaping:
// a syntactically-clean but nonexistent/undeclared column name must also be
// rejected, rather than reaching PostgreSQL as a raw "column does not
// exist" error (which would still mean an attacker could probe for the
// existence of arbitrary table columns, including ones never declared as
// entity fields).
func TestQuery_SortField_UnknownColumn_Rejected(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, testEntityDDL)

	tenantID := testdb.RawTenantID()
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := testdb.WithTenant(context.Background(), tenantID)
	repo := newRepo(pool)

	_, _, err := repo.Query(ctx, nil, driver.WithSort("not_a_real_column", true))
	require.Error(t, err, "sorting by a column not declared on the entity must be rejected")
}

// TestQuery_SortField_LegitimateField_StillWorks is the corresponding
// positive case — the fix must not break normal sorting.
func TestQuery_SortField_LegitimateField_StillWorks(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, testEntityDDL)

	tenantID := testdb.RawTenantID()
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := testdb.WithTenant(context.Background(), tenantID)
	repo := newRepo(pool)

	_, err := repo.Create(ctx, driver.CreateInput{Data: map[string]any{"name": "b"}})
	require.NoError(t, err)
	_, err = repo.Create(ctx, driver.CreateInput{Data: map[string]any{"name": "a"}})
	require.NoError(t, err)

	records, _, err := repo.Query(ctx, nil, driver.WithSort("name", true))
	require.NoError(t, err)
	require.Len(t, records, 2)
	assert.Equal(t, "a", records[0].Data["name"])
	assert.Equal(t, "b", records[1].Data["name"])
}

// TestFilterSecurity_NotOperator_StillRLSScoped exercises filter.Not
// combined with RLS: a NOT predicate designed to invert normal filtering
// logic must still never surface another tenant's rows.
func TestFilterSecurity_NotOperator_StillRLSScoped(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, testEntityDDL)

	tenantA := testdb.RawTenantID()
	tenantB := testdb.RawTenantID()
	repo := newRepo(pool)

	testdb.ActivateTenant(t, pool, tenantA)
	ctxA := testdb.WithTenant(context.Background(), tenantA)
	_, err := repo.Create(ctxA, driver.CreateInput{Data: map[string]any{"name": "A-row", "code": "keep"}})
	require.NoError(t, err)

	testdb.ActivateTenant(t, pool, tenantB)
	ctxB := testdb.WithTenant(context.Background(), tenantB)
	_, err = repo.Create(ctxB, driver.CreateInput{Data: map[string]any{"name": "B-row", "code": "other"}})
	require.NoError(t, err)

	// "NOT code = 'other'" as Tenant B should match nothing of Tenant B's
	// own zero-or-more non-'other' rows, and must never surface Tenant A's
	// row regardless of how the NOT inverts the WHERE clause.
	f := filter.Not(filter.Eq("code", "other"))
	records, _, err := repo.Query(ctxB, f, driver.WithSkipCount())
	require.NoError(t, err)
	for _, r := range records {
		assert.Equal(t, tenantB, r.TenantID, "NOT predicate must not surface another tenant's row")
	}
}

// TestFilterSecurity_NullComparison_RLSScoped proves IsNull/IsNotNull
// predicates — which generate different SQL shapes than equality — are
// equally bound by RLS.
func TestFilterSecurity_NullComparison_RLSScoped(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, testEntityDDL)

	tenantA := testdb.RawTenantID()
	tenantB := testdb.RawTenantID()
	repo := newRepo(pool)

	testdb.ActivateTenant(t, pool, tenantA)
	ctxA := testdb.WithTenant(context.Background(), tenantA)
	_, err := repo.Create(ctxA, driver.CreateInput{Data: map[string]any{"name": "A-row"}}) // code left NULL
	require.NoError(t, err)

	testdb.ActivateTenant(t, pool, tenantB)
	ctxB := testdb.WithTenant(context.Background(), tenantB)

	records, _, err := repo.Query(ctxB, filter.IsNull("code"), driver.WithSkipCount())
	require.NoError(t, err)
	assert.Empty(t, records, "IsNull predicate must still be RLS-scoped to Tenant B, "+
		"even though Tenant A's row is the only one matching code IS NULL")
}

// TestFilterSecurity_BulkUpdate_CannotCrossTenant proves a filter-driven
// bulk mutation cannot be crafted to affect another tenant's rows, even
// with a maximally permissive filter (nil — matches everything the caller
// can see).
func TestFilterSecurity_BulkUpdate_CannotCrossTenant(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, testEntityDDL)

	tenantA := testdb.RawTenantID()
	tenantB := testdb.RawTenantID()
	repo := newRepo(pool)

	testdb.ActivateTenant(t, pool, tenantA)
	ctxA := testdb.WithTenant(context.Background(), tenantA)
	_, err := repo.Create(ctxA, driver.CreateInput{Data: map[string]any{"name": "A-original"}})
	require.NoError(t, err)

	testdb.ActivateTenant(t, pool, tenantB)
	ctxB := testdb.WithTenant(context.Background(), tenantB)
	_, err = repo.Create(ctxB, driver.CreateInput{Data: map[string]any{"name": "B-original"}})
	require.NoError(t, err)

	// Tenant B issues the broadest possible BulkUpdate (nil filter — "update
	// everything you can see"). This must only ever touch Tenant B's own row.
	affected, err := repo.BulkUpdate(ctxB, nil, driver.Patch{Set: map[string]any{"name": "HACKED"}})
	require.NoError(t, err)
	assert.Equal(t, int64(1), affected, "BulkUpdate with the broadest filter must only affect the caller's own tenant's rows")

	// Verify as superuser (bypassing RLS) that Tenant A's row is untouched.
	rows, derr := testdb.QueryRowsSQL(t, pool, "SELECT name FROM test_entity WHERE tenant_id = $1", tenantA)
	require.NoError(t, derr)
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		require.NoError(t, rows.Scan(&n))
		names = append(names, n)
	}
	assert.Equal(t, []string{"A-original"}, names, "Tenant A's row must be unmodified by Tenant B's BulkUpdate")
}

// TestFilterSecurity_Delete_CannotCrossTenant proves Delete-by-ID cannot
// remove another tenant's row even when the exact ID is known.
func TestFilterSecurity_Delete_CannotCrossTenant(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, testEntityDDL)

	tenantA := testdb.RawTenantID()
	tenantB := testdb.RawTenantID()
	repo := newRepo(pool)

	testdb.ActivateTenant(t, pool, tenantA)
	ctxA := testdb.WithTenant(context.Background(), tenantA)
	rec, err := repo.Create(ctxA, driver.CreateInput{Data: map[string]any{"name": "A-row"}})
	require.NoError(t, err)

	testdb.ActivateTenant(t, pool, tenantB)
	ctxB := testdb.WithTenant(context.Background(), tenantB)
	delErr := repo.Delete(ctxB, rec.ID)
	require.Error(t, delErr, "deleting another tenant's record by ID must fail, not silently succeed")

	// Confirm as superuser that the row still exists.
	row := testdb.QueryRowSQL(t, pool, "SELECT count(*) FROM test_entity WHERE id = $1", rec.ID)
	var count int
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, 1, count, "Tenant A's row must still exist after Tenant B's failed cross-tenant delete")
}
