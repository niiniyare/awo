package pgx_test

// R.8/§B.9: contrib/pgx.Repository.systemColumnsAndScanner previously scanned
// every custom field through a generic `var v any` destination. For a
// FieldTypeLink column (a real Postgres `uuid` column), pgx/v5 decodes that
// into [16]byte with no more specific destination type, and encoding/json
// marshals a fixed-size byte array as 16 plain integers, not a UUID string —
// confirmed live against the real Expense demo (findings register §B.9),
// where `submitted_by` rendered as e.g. [22,233,217,...] instead of a UUID.
//
// def/record.go's own documented contract says FieldTypeLink -> uuid.UUID in
// EntityRecord.Data. This test proves, against real PostgreSQL with the real
// non-superuser awo_app role and a real FieldTypeLink field targeting the
// real iam_user entity, that Create+Get through the generic repository path
// now returns a genuine uuid.UUID (not a byte array) and that it marshals to
// a correct UUID string end-to-end — the same observable behavior a real API
// response and the real Expense UI depend on.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/compiler"
	contribpgx "awo.so/awo/contrib/pgx"
	"awo.so/awo/def"
	"awo.so/awo/driver"
	"awo.so/awo/platform/iam"
	"awo.so/awo/platform/tenant"
	"awo.so/awo/registry"
	testdb "awo.so/awo/testutil/db"
)

// linkProbeDefinition is a minimal entity with a single FieldTypeLink field
// targeting the real iam_user entity — isolated from examples/expense on
// purpose, so this test proves the generic contrib/pgx fix, not anything
// Expense-specific.
var linkProbeDefinition = def.SystemDefinition{
	Name:           "link_probe",
	Module:         "test",
	Label:          "Link Probe",
	NoUpdatedAt:    true,
	NoCustomFields: true,
	Fields: []def.FieldDef{
		{
			Name:       "owner",
			Type:       def.FieldTypeLink,
			Label:      "Owner",
			LinkTarget: "iam_user",
			Required:   true,
		},
	},
}

func TestFieldTypeLink_ScansAsRealUUID_NotByteArray(t *testing.T) {
	reg, err := registry.BuildFrom([]def.EntityDefinition{
		&tenant.Definition,
		&iam.UserDefinition,
		&linkProbeDefinition,
	})
	require.NoError(t, err)
	schema, err := compiler.Compile(reg)
	require.NoError(t, err)

	es := schema.ByName["test_link_probe"]
	require.NotNil(t, es, "test_link_probe missing from compiled schema")

	pool := testdb.SetupTestDB(t)

	tenantSQL, err := os.ReadFile(filepath.Join("..", "..", "platform", "tenant", "migrations", "001_platform_tenant.up.sql"))
	require.NoError(t, err)
	testdb.ApplySQL(t, pool, string(tenantSQL))
	testdb.ApplySQL(t, pool, `GRANT SELECT, INSERT, UPDATE ON platform_tenant TO `+testdb.AppRole+`;`)

	userSQL, err := os.ReadFile(filepath.Join("..", "..", "platform", "iam", "migrations", "001_iam_user.up.sql"))
	require.NoError(t, err)
	testdb.ApplySQL(t, pool, string(userSQL))
	testdb.ApplySQL(t, pool, `GRANT SELECT, INSERT, UPDATE ON iam_user TO `+testdb.AppRole+`;`)

	testdb.ApplySQL(t, pool, `
CREATE TABLE IF NOT EXISTS test_link_probe (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  uuid        NOT NULL REFERENCES platform_tenant(id),
    created_at timestamptz NOT NULL DEFAULT NOW(),
    owner      uuid        NOT NULL REFERENCES iam_user(id)
);
ALTER TABLE test_link_probe ENABLE ROW LEVEL SECURITY;
ALTER TABLE test_link_probe FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON test_link_probe
    AS PERMISSIVE FOR ALL TO PUBLIC
    USING (tenant_id = current_tenant_id());
GRANT SELECT, INSERT, UPDATE ON test_link_probe TO `+testdb.AppRole+`;`)

	ctx := context.Background()
	tenantID := uuid.New()
	_, err = pool.Exec(ctx,
		`INSERT INTO platform_tenant (id, name, slug, status, contact_email) VALUES ($1, $2, $3, 'ACTIVE', $4)`,
		tenantID, "Probe Co", "probe-"+tenantID.String()[:8], "ops@probe.test")
	require.NoError(t, err)
	userID := uuid.New()
	testdb.ApplySQL(t, pool, `INSERT INTO iam_user (id, tenant_id, email, full_name) VALUES ('`+userID.String()+`', '`+tenantID.String()+`', 'probe@test.local', 'Probe User');`)

	ctx = testdb.WithTenant(ctx, tenantID)
	repo := contribpgx.NewRepository(pool, es)

	created, err := repo.Create(ctx, driver.CreateInput{
		Data: map[string]any{"owner": userID},
	})
	require.NoError(t, err)

	got, err := repo.Get(ctx, created.ID)
	require.NoError(t, err)

	gotOwner, ok := got.Data["owner"].(uuid.UUID)
	require.True(t, ok, "Data[\"owner\"] must be a real uuid.UUID, got %T: %#v", got.Data["owner"], got.Data["owner"])
	assert.Equal(t, userID, gotOwner)

	// End-to-end proof: this is what api/response actually sends to the
	// browser. A [16]byte would marshal as 16 integers; uuid.UUID marshals
	// as the correct hyphenated string via its MarshalText.
	raw, err := json.Marshal(got.Data)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))
	ownerJSON, ok := decoded["owner"].(string)
	require.True(t, ok, "owner must serialize as a JSON string, got: %s", raw)
	assert.Equal(t, userID.String(), ownerJSON)
}
