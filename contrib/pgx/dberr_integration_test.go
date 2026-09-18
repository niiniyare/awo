package pgx_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/compiler"
	contribpgx "awo.so/awo/contrib/pgx"
	"awo.so/awo/def"
	"awo.so/awo/driver"
	"awo.so/awo/runtime"
	testdb "awo.so/awo/testutil/db"
)

// testEntityUniqueCodeDDL adds a UNIQUE constraint on "code" so a real
// PostgreSQL unique_violation (SQLSTATE 23505) can be triggered end-to-end
// through the live repository path, not synthesized.
const testEntityUniqueCodeDDL = `
CREATE TABLE test_entity_unique (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     uuid NOT NULL,
    name          text NOT NULL,
    code          text NOT NULL,
    custom_fields jsonb,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT test_entity_unique_code_uniq UNIQUE (code)
);

ALTER TABLE test_entity_unique ENABLE ROW LEVEL SECURITY;
ALTER TABLE test_entity_unique FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON test_entity_unique
    USING (tenant_id = current_tenant_id());

GRANT SELECT, INSERT, UPDATE, DELETE ON test_entity_unique TO awo_app;
`

// TestRepository_Create_RealUniqueViolationTranslatedCorrectly proves the
// full, live error path end-to-end: repo.Create -> pgx/v5 Exec -> a genuine
// PostgreSQL unique_violation -> dberr.Parse -> *runtime.BusinessError.
//
// This is the integration-level companion to internal/dberr's unit tests: it
// does not construct a pgconn.PgError by hand, it triggers a real one from
// real PostgreSQL through the exact code path a production request takes
// (contrib/pgx.Repository.Create, which calls dberr.Parse on every error
// return — see contrib/pgx/repo.go). Before the Phase 1 fix (dberr.go
// importing the legacy github.com/jackc/pgconn instead of
// github.com/jackc/pgx/v5/pgconn), this test failed: errors.As could not
// match pgx/v5's real error type, and the caller received a generic wrapped
// error instead of a typed, HTTP-mappable *runtime.BusinessError.
func TestRepository_Create_RealUniqueViolationTranslatedCorrectly(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, testEntityUniqueCodeDDL)

	tenantID := testdb.RawTenantID()
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := testdb.WithTenant(context.Background(), tenantID)

	repo := contribpgx.NewRepository(pool, &compiler.EntitySchema{
		QualifiedName: "test_entity_unique",
		TableName:     "test_entity_unique",
		IsSystem:      true,
		FieldsByName: map[string]def.FieldDef{
			"name": {Name: "name", Type: def.FieldTypeData},
			"code": {Name: "code", Type: def.FieldTypeData},
		},
	})

	_, err := repo.Create(ctx, driver.CreateInput{
		Data: map[string]any{"name": "First", "code": "DUP-1"},
	})
	require.NoError(t, err, "first insert must succeed")

	_, err = repo.Create(ctx, driver.CreateInput{
		Data: map[string]any{"name": "Second", "code": "DUP-1"},
	})
	require.Error(t, err, "second insert with duplicate code must fail")

	var be *runtime.BusinessError
	if !errors.As(err, &be) {
		t.Fatalf("Create() error = %v (%T), want *runtime.BusinessError — "+
			"dberr.Parse failed to recognize the real pgx/v5 unique_violation error", err, err)
	}
	assert.Equal(t, "duplicate", be.Code)
	assert.Equal(t, 409, be.Status)
}
