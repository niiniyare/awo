package service_test

// Part 6 of the "coherent, production-grade ERP" mission: Expense's
// "Submitted By" column showed a raw UUID instead of a meaningful name. The
// root cause traced to two places that treated a FieldTypeLink value as
// nothing more than an id:
//
//   - sdui/generator built list/detail/summary-card widgets bound directly to
//     the raw link field, with no mechanism to show anything else.
//   - EntityService.Query/Get never resolved what a linked id actually names.
//
// This test proves the fix at the service layer: Query and Get populate
// "<field>_label" for every FieldTypeLink field (EntityService.resolveLinkLabels),
// using driver.LinkLabelResolver — here, the real contrib/pgx.LinkResolver
// against real PostgreSQL — to look up the linked iam_user's own full_name.
// It is deliberately NOT built on examples/expense: proving this against a
// throwaway "owner -> iam_user" entity shows the fix is generic (any
// FieldTypeLink field on any entity benefits), not an Expense-specific patch.
//
// Both halves the mission asked for are covered:
//   - the raw id is still present and correct (Data["owner"]) — compatibility
//     with every existing consumer of the raw link value is preserved;
//   - the user-facing label is now present and correct (Data["owner_label"]),
//     and falls back gracefully to the raw id string when no resolver is
//     wired at all (e.g. tests that construct EntityService directly).

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/api/service"
	"awo.so/awo/audit"
	"awo.so/awo/compiler"
	contribpgx "awo.so/awo/contrib/pgx"
	"awo.so/awo/def"
	"awo.so/awo/platform/iam"
	"awo.so/awo/platform/tenant"
	"awo.so/awo/registry"
	"awo.so/awo/runtime"
	testdb "awo.so/awo/testutil/db"
)

// linkLabelProbeDefinition mirrors contrib/pgx/link_field_uuid_test.go's
// linkProbeDefinition (a minimal entity with one FieldTypeLink field
// targeting the real iam_user entity), kept as a separate type here so this
// package's test does not need to import contrib/pgx_test's unexported type.
var linkLabelProbeDefinition = def.SystemDefinition{
	Name:           "link_label_probe",
	Module:         "test",
	Label:          "Link Label Probe",
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

func TestEntityService_PG_QueryGet_ResolveLinkLabels(t *testing.T) {
	reg, err := registry.BuildFrom([]def.EntityDefinition{
		&tenant.Definition,
		&iam.UserDefinition,
		&linkLabelProbeDefinition,
	})
	require.NoError(t, err)
	schema, err := compiler.Compile(reg)
	require.NoError(t, err)

	es := schema.ByName["test_link_label_probe"]
	require.NotNil(t, es, "test_link_label_probe missing from compiled schema")
	lookup := es.FieldLookups["owner"]
	require.NotNil(t, lookup, "owner must have a compiled lookup (FieldTypeLink)")
	require.Equal(t, "full_name", lookup.LabelField, "iam_user's label field must resolve to full_name")

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
CREATE TABLE IF NOT EXISTS test_link_label_probe (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  uuid        NOT NULL REFERENCES platform_tenant(id),
    created_at timestamptz NOT NULL DEFAULT NOW(),
    owner      uuid        NOT NULL REFERENCES iam_user(id)
);
ALTER TABLE test_link_label_probe ENABLE ROW LEVEL SECURITY;
ALTER TABLE test_link_label_probe FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON test_link_label_probe
    AS PERMISSIVE FOR ALL TO PUBLIC
    USING (tenant_id = current_tenant_id());
GRANT SELECT, INSERT, UPDATE ON test_link_label_probe TO `+testdb.AppRole+`;`)

	ctx := context.Background()
	tenantID := uuid.New()
	_, err = pool.Exec(ctx,
		`INSERT INTO platform_tenant (id, name, slug, status, contact_email) VALUES ($1, $2, $3, 'ACTIVE', $4)`,
		tenantID, "Label Probe Co", "label-probe-"+tenantID.String()[:8], "ops@probe.test")
	require.NoError(t, err)
	userID := uuid.New()
	testdb.ApplySQL(t, pool, `INSERT INTO iam_user (id, tenant_id, email, full_name) VALUES ('`+userID.String()+`', '`+tenantID.String()+`', 'probe@test.local', 'Probe User');`)

	ctx = testdb.WithTenant(ctx, tenantID)
	repo := contribpgx.NewRepository(pool, es)
	pipeline := runtime.NewPipeline(schema, audit.NoopAuditWriter{})
	actor := &def.Actor{UserID: userID, TenantID: tenantID}

	created, err := service.NewEntityService(es, repo, pipeline).
		WithLinkResolver(contribpgx.NewLinkResolver(pool)).
		Create(ctx, map[string]any{"owner": userID}, actor)
	require.NoError(t, err)

	t.Run("resolver wired: Get upgrades the label to the linked user's full_name", func(t *testing.T) {
		svc := service.NewEntityService(es, repo, pipeline).WithLinkResolver(contribpgx.NewLinkResolver(pool))

		got, err := svc.Get(ctx, created.ID)
		require.NoError(t, err)

		gotOwner, ok := got.Data["owner"].(uuid.UUID)
		require.True(t, ok, "Data[\"owner\"] must remain a real uuid.UUID — the raw value must still be correct")
		assert.Equal(t, userID, gotOwner)

		label, ok := got.Data["owner_label"].(string)
		require.True(t, ok, "Data[\"owner_label\"] must be present and a string")
		assert.Equal(t, "Probe User", label)
	})

	t.Run("resolver wired: Query upgrades the label for every row in the page", func(t *testing.T) {
		svc := service.NewEntityService(es, repo, pipeline).WithLinkResolver(contribpgx.NewLinkResolver(pool))

		records, _, err := svc.Query(ctx, nil)
		require.NoError(t, err)
		require.Len(t, records, 1)
		assert.Equal(t, "Probe User", records[0].Data["owner_label"])
	})

	t.Run("no resolver wired: label falls back to the raw id string, never blank or missing", func(t *testing.T) {
		svc := service.NewEntityService(es, repo, pipeline) // no WithLinkResolver call

		got, err := svc.Get(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, userID.String(), got.Data["owner_label"],
			"without a resolver, owner_label must still be set (to the raw id) so UI layers can bind to it unconditionally")
	})
}

// linkLabelCollisionProbeDefinition declares BOTH a FieldTypeLink field
// ("owner") AND its own, independent, author-declared "owner_label" field —
// the exact name resolveLinkLabels would otherwise generate. Part 7 of the
// "shared SDUI record-management" mission explicitly asks that the generated
// display field "does not conflict with user-defined fields" — this proves
// resolveLinkLabels detects the collision (via s.schema.FieldsByName) and
// leaves the real field alone rather than silently overwriting it.
var linkLabelCollisionProbeDefinition = def.SystemDefinition{
	Name:           "link_label_collision_probe",
	Module:         "test",
	Label:          "Link Label Collision Probe",
	NoUpdatedAt:    true,
	NoCustomFields: true,
	Fields: []def.FieldDef{
		{Name: "owner", Type: def.FieldTypeLink, Label: "Owner", LinkTarget: "iam_user", Required: true},
		{Name: "owner_label", Type: def.FieldTypeData, Label: "Owner Label (author-declared)"},
	},
}

func TestEntityService_PG_ResolveLinkLabels_DoesNotShadowUserDefinedField(t *testing.T) {
	reg, err := registry.BuildFrom([]def.EntityDefinition{
		&tenant.Definition,
		&iam.UserDefinition,
		&linkLabelCollisionProbeDefinition,
	})
	require.NoError(t, err)
	schema, err := compiler.Compile(reg)
	require.NoError(t, err)

	es := schema.ByName["test_link_label_collision_probe"]
	require.NotNil(t, es)

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
CREATE TABLE IF NOT EXISTS test_link_label_collision_probe (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid        NOT NULL REFERENCES platform_tenant(id),
    created_at  timestamptz NOT NULL DEFAULT NOW(),
    owner       uuid        NOT NULL REFERENCES iam_user(id),
    owner_label text
);
ALTER TABLE test_link_label_collision_probe ENABLE ROW LEVEL SECURITY;
ALTER TABLE test_link_label_collision_probe FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON test_link_label_collision_probe
    AS PERMISSIVE FOR ALL TO PUBLIC
    USING (tenant_id = current_tenant_id());
GRANT SELECT, INSERT, UPDATE ON test_link_label_collision_probe TO `+testdb.AppRole+`;`)

	ctx := context.Background()
	tenantID := uuid.New()
	_, err = pool.Exec(ctx,
		`INSERT INTO platform_tenant (id, name, slug, status, contact_email) VALUES ($1, $2, $3, 'ACTIVE', $4)`,
		tenantID, "Collision Probe Co", "collision-probe-"+tenantID.String()[:8], "ops@probe.test")
	require.NoError(t, err)
	userID := uuid.New()
	testdb.ApplySQL(t, pool, `INSERT INTO iam_user (id, tenant_id, email, full_name) VALUES ('`+userID.String()+`', '`+tenantID.String()+`', 'probe@test.local', 'Probe User');`)

	ctx = testdb.WithTenant(ctx, tenantID)
	repo := contribpgx.NewRepository(pool, es)
	pipeline := runtime.NewPipeline(schema, audit.NoopAuditWriter{})
	actor := &def.Actor{UserID: userID, TenantID: tenantID}

	svc := service.NewEntityService(es, repo, pipeline).WithLinkResolver(contribpgx.NewLinkResolver(pool))
	created, err := svc.Create(ctx, map[string]any{
		"owner":       userID,
		"owner_label": "This is the author's own field, not a generated one",
	}, actor)
	require.NoError(t, err)

	got, err := svc.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "This is the author's own field, not a generated one", got.Data["owner_label"],
		"resolveLinkLabels must never overwrite a field the entity already declares under that exact name")
}
