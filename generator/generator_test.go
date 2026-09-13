package generator_test

import (
	"strings"
	"testing"

	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"awo.so/awo/generator"
	"awo.so/awo/registry"
)

// buildTestSchema compiles a minimal set of entity definitions into a CompiledSchema.
func buildTestSchema(t *testing.T, defs ...def.EntityDefinition) *compiler.CompiledSchema {
	t.Helper()
	reg, err := registry.BuildFrom(defsSlice(defs))
	if err != nil {
		t.Fatalf("registry.BuildFrom: %v", err)
	}
	schema, err := compiler.Compile(reg)
	if err != nil {
		t.Fatalf("compiler.Compile: %v", err)
	}
	return schema
}

func defsSlice(defs []def.EntityDefinition) []def.EntityDefinition { return defs }

// minimalEntity returns a SystemDefinition with defaults set.
func minimalEntity(name, module string, fields ...def.FieldDef) *def.SystemDefinition {
	return &def.SystemDefinition{
		Name:   name,
		Module: module,
		Fields: fields,
	}
}

func TestGenerate_ReturnsInfraFile(t *testing.T) {
	schema := buildTestSchema(t, minimalEntity("widget", "test"))
	plan, err := generator.Generate(schema, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Files) < 1 {
		t.Fatal("expected at least one file")
	}
	infra := plan.Files[0]
	if !strings.Contains(infra.Name, "infrastructure") {
		t.Errorf("first file should be infrastructure, got %q", infra.Name)
	}
	if !strings.Contains(infra.SQL, "current_tenant_id") {
		t.Error("infrastructure file should define current_tenant_id()")
	}
	if !strings.Contains(infra.SQL, "set_tenant_context") {
		t.Error("infrastructure file should define set_tenant_context()")
	}
	if !strings.Contains(infra.SQL, "awo_set_updated_at") {
		t.Error("infrastructure file should define awo_set_updated_at()")
	}
	if !strings.Contains(infra.SQL, "pg_trgm") {
		t.Error("infrastructure file should create pg_trgm extension")
	}
}

func TestGenerate_EntityFile_StandardColumns(t *testing.T) {
	schema := buildTestSchema(t, minimalEntity("widget", "test"))
	plan, err := generator.Generate(schema, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var entityFile *generator.MigrationFile
	for i := range plan.Files {
		if strings.Contains(plan.Files[i].Name, "test_widget") {
			entityFile = &plan.Files[i]
			break
		}
	}
	if entityFile == nil {
		t.Fatal("expected entity file for test_widget")
	}
	sql := entityFile.SQL
	for _, col := range []string{"id", "tenant_id", "created_at", "updated_at", "deleted_at"} {
		if !strings.Contains(sql, col) {
			t.Errorf("standard column %q missing from generated SQL", col)
		}
	}
}

func TestGenerate_EntityFile_PrimaryKey(t *testing.T) {
	schema := buildTestSchema(t, minimalEntity("widget", "test"))
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "test_widget")
	if !strings.Contains(sql, "PRIMARY KEY") {
		t.Error("generated SQL should have PRIMARY KEY")
	}
	if !strings.Contains(sql, "gen_random_uuid()") {
		t.Error("id should default to gen_random_uuid()")
	}
}

func TestGenerate_EntityFile_RLS(t *testing.T) {
	schema := buildTestSchema(t, minimalEntity("widget", "test"))
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "test_widget")
	if !strings.Contains(sql, "ENABLE ROW LEVEL SECURITY") {
		t.Error("RLS not enabled")
	}
	if !strings.Contains(sql, "current_tenant_id()") {
		t.Error("RLS policy should use current_tenant_id()")
	}
}

func TestGenerate_EntityFile_UpdatedAtTrigger(t *testing.T) {
	schema := buildTestSchema(t, minimalEntity("widget", "test"))
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "test_widget")
	if !strings.Contains(sql, "awo_set_updated_at") {
		t.Error("updated_at trigger missing")
	}
	if !strings.Contains(sql, "BEFORE UPDATE") {
		t.Error("trigger should fire BEFORE UPDATE")
	}
}

func TestGenerate_FieldType_Data(t *testing.T) {
	schema := buildTestSchema(t, minimalEntity("widget", "test",
		def.FieldDef{Name: "code", Type: def.FieldTypeData},
	))
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "test_widget")
	if !strings.Contains(sql, "VARCHAR(255)") {
		t.Errorf("FieldTypeData default should be VARCHAR(255), got:\n%s", sql)
	}
}

func TestGenerate_FieldType_Data_MaxLen(t *testing.T) {
	schema := buildTestSchema(t, minimalEntity("widget", "test",
		def.FieldDef{Name: "code", Type: def.FieldTypeData, MaxLen: 50},
	))
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "test_widget")
	if !strings.Contains(sql, "VARCHAR(50)") {
		t.Errorf("MaxLen=50 should produce VARCHAR(50)")
	}
}

func TestGenerate_FieldType_SmallText(t *testing.T) {
	schema := buildTestSchema(t, minimalEntity("widget", "test",
		def.FieldDef{Name: "note", Type: def.FieldTypeSmallText},
	))
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "test_widget")
	if !strings.Contains(sql, "VARCHAR(1024)") {
		t.Error("FieldTypeSmallText should produce VARCHAR(1024)")
	}
}

func TestGenerate_FieldType_LongText(t *testing.T) {
	schema := buildTestSchema(t, minimalEntity("widget", "test",
		def.FieldDef{Name: "body", Type: def.FieldTypeLongText},
	))
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "test_widget")
	if !strings.Contains(sql, "TEXT") {
		t.Error("FieldTypeLongText should produce TEXT")
	}
}

func TestGenerate_FieldType_Int(t *testing.T) {
	schema := buildTestSchema(t, minimalEntity("widget", "test",
		def.FieldDef{Name: "qty", Type: def.FieldTypeInt},
	))
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "test_widget")
	if !strings.Contains(sql, "BIGINT") {
		t.Error("FieldTypeInt should produce BIGINT")
	}
}

func TestGenerate_FieldType_Float(t *testing.T) {
	schema := buildTestSchema(t, minimalEntity("widget", "test",
		def.FieldDef{Name: "rate", Type: def.FieldTypeFloat},
	))
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "test_widget")
	if !strings.Contains(sql, "DOUBLE PRECISION") {
		t.Error("FieldTypeFloat should produce DOUBLE PRECISION")
	}
}

func TestGenerate_FieldType_Currency(t *testing.T) {
	schema := buildTestSchema(t, minimalEntity("widget", "test",
		def.FieldDef{Name: "value", Type: def.FieldTypeCurrency},
	))
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "test_widget")
	if !strings.Contains(sql, "NUMERIC(20,4)") {
		t.Error("FieldTypeCurrency should produce NUMERIC(20,4)")
	}
}

func TestGenerate_FieldType_Bool(t *testing.T) {
	schema := buildTestSchema(t, minimalEntity("widget", "test",
		def.FieldDef{Name: "active", Type: def.FieldTypeBool},
	))
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "test_widget")
	if !strings.Contains(sql, "BOOLEAN NOT NULL DEFAULT FALSE") {
		t.Error("FieldTypeBool should produce BOOLEAN NOT NULL DEFAULT FALSE")
	}
}

func TestGenerate_FieldType_Date(t *testing.T) {
	schema := buildTestSchema(t, minimalEntity("widget", "test",
		def.FieldDef{Name: "due_date", Type: def.FieldTypeDate},
	))
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "test_widget")
	if !strings.Contains(sql, "DATE") {
		t.Error("FieldTypeDate should produce DATE")
	}
}

func TestGenerate_FieldType_DateTime(t *testing.T) {
	schema := buildTestSchema(t, minimalEntity("widget", "test",
		def.FieldDef{Name: "posted_at", Type: def.FieldTypeDateTime},
	))
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "test_widget")
	if !strings.Contains(sql, "TIMESTAMPTZ") {
		t.Error("FieldTypeDateTime should produce TIMESTAMPTZ")
	}
}

func TestGenerate_FieldType_Select_WithCheck(t *testing.T) {
	schema := buildTestSchema(t, minimalEntity("widget", "test",
		def.FieldDef{Name: "status", Type: def.FieldTypeSelect, Options: []string{"draft", "active", "archived"}},
	))
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "test_widget")
	if !strings.Contains(sql, "VARCHAR(100)") {
		t.Error("FieldTypeSelect should produce VARCHAR(100)")
	}
	if !strings.Contains(sql, "CHECK") {
		t.Error("FieldTypeSelect with options should produce CHECK constraint")
	}
	if !strings.Contains(sql, "'draft'") || !strings.Contains(sql, "'active'") {
		t.Error("CHECK constraint should include options")
	}
}

func TestGenerate_FieldType_MultiSelect(t *testing.T) {
	schema := buildTestSchema(t, minimalEntity("widget", "test",
		def.FieldDef{Name: "tags", Type: def.FieldTypeMultiSelect},
	))
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "test_widget")
	if !strings.Contains(sql, "TEXT[]") {
		t.Error("FieldTypeMultiSelect should produce TEXT[]")
	}
}

func TestGenerate_FieldType_NamingSeries(t *testing.T) {
	schema := buildTestSchema(t, minimalEntity("widget", "test",
		def.FieldDef{Name: "doc_no", Type: def.FieldTypeNamingSeries, Series: "WID-{YYYY}-{SEQ:5}"},
	))
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "test_widget")
	if !strings.Contains(sql, "VARCHAR(100)") {
		t.Error("FieldTypeNamingSeries should produce VARCHAR(100)")
	}
}

func TestGenerate_FieldType_JSON(t *testing.T) {
	schema := buildTestSchema(t, minimalEntity("widget", "test",
		def.FieldDef{Name: "meta", Type: def.FieldTypeJSON},
	))
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "test_widget")
	if !strings.Contains(sql, "JSONB") {
		t.Error("FieldTypeJSON should produce JSONB")
	}
}

func TestGenerate_FieldType_Link(t *testing.T) {
	target := minimalEntity("category", "test")
	source := &def.SystemDefinition{
		Name:   "item",
		Module: "test",
		Fields: []def.FieldDef{
			{Name: "category_id", Type: def.FieldTypeLink, LinkTarget: "test_category"},
		},
	}
	schema := buildTestSchema(t, target, source)
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "test_item")
	if !strings.Contains(sql, "UUID") {
		t.Error("FieldTypeLink should produce UUID column")
	}
	if !strings.Contains(sql, "REFERENCES") {
		t.Error("FieldTypeLink should produce REFERENCES FK")
	}
	if !strings.Contains(sql, "test_category") {
		t.Error("FK should reference test_category")
	}
}

func TestGenerate_FieldType_LinkList(t *testing.T) {
	target := minimalEntity("tag", "test")
	source := &def.SystemDefinition{
		Name:   "widget",
		Module: "test",
		Fields: []def.FieldDef{
			{Name: "tag_ids", Type: def.FieldTypeLinkList, LinkTarget: "test_tag"},
		},
	}
	schema := buildTestSchema(t, target, source)
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "test_widget")
	if !strings.Contains(sql, "UUID[]") {
		t.Error("FieldTypeLinkList should produce UUID[]")
	}
}

func TestGenerate_Required_NotNull(t *testing.T) {
	schema := buildTestSchema(t, minimalEntity("widget", "test",
		def.FieldDef{Name: "name", Type: def.FieldTypeData, Required: true},
	))
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "test_widget")
	if !strings.Contains(sql, "NOT NULL") {
		t.Error("Required field should produce NOT NULL")
	}
}

func TestGenerate_Unique_Constraint(t *testing.T) {
	schema := buildTestSchema(t, minimalEntity("widget", "test",
		def.FieldDef{Name: "code", Type: def.FieldTypeData, Unique: true},
	))
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "test_widget")
	if !strings.Contains(sql, "UNIQUE") {
		t.Error("Unique field should produce UNIQUE constraint")
	}
}

func TestGenerate_Searchable_GINIndex(t *testing.T) {
	schema := buildTestSchema(t, minimalEntity("widget", "test",
		def.FieldDef{Name: "name", Type: def.FieldTypeData, Searchable: true},
	))
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "test_widget")
	if !strings.Contains(sql, "GIN") {
		t.Error("Searchable field should produce GIN index")
	}
	if !strings.Contains(sql, "gin_trgm_ops") {
		t.Error("GIN index should use gin_trgm_ops")
	}
}

func TestGenerate_Link_FKIndex(t *testing.T) {
	target := minimalEntity("category", "test")
	source := &def.SystemDefinition{
		Name:   "item",
		Module: "test",
		Fields: []def.FieldDef{
			{Name: "category_id", Type: def.FieldTypeLink, LinkTarget: "test_category"},
		},
	}
	schema := buildTestSchema(t, target, source)
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "test_item")
	if !strings.Contains(sql, "CREATE INDEX") {
		t.Error("Link field should produce an index")
	}
	if !strings.Contains(sql, "category_id") {
		t.Error("FK index should reference category_id")
	}
}

func TestGenerate_AllowAudit_True(t *testing.T) {
	// DisableAudit defaults to false → AllowAudit() returns true.
	d := minimalEntity("widget", "test")
	schema := buildTestSchema(t, d)
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "test_widget")
	if !strings.Contains(sql, "awo_audit_log") {
		t.Error("AllowAudit=true should produce audit trigger referencing awo_audit_log")
	}
}

func TestGenerate_AllowAudit_False(t *testing.T) {
	d := &def.SystemDefinition{
		Name:         "log",
		Module:       "test",
		DisableAudit: true,
	}
	schema := buildTestSchema(t, d)
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "test_log")
	if strings.Contains(sql, "awo_audit_log") {
		t.Error("DisableAudit=true should NOT produce audit trigger")
	}
}

func TestGenerate_SequenceNumbering(t *testing.T) {
	schema := buildTestSchema(t, minimalEntity("widget", "test"))
	plan, _ := generator.Generate(schema, generator.Options{StartSeq: 5})
	if !strings.HasPrefix(plan.Files[0].Name, "000005_") {
		t.Errorf("first file should start at seq 5, got %q", plan.Files[0].Name)
	}
	if !strings.HasPrefix(plan.Files[1].Name, "000006_") {
		t.Errorf("second file should be seq 6, got %q", plan.Files[1].Name)
	}
}

func TestGenerate_CustomDefinition_Skipped(t *testing.T) {
	custom := &def.CustomDefinition{
		Name:   "widget",
		Module: "test",
	}
	schema := buildTestSchema(t, custom)
	plan, err := generator.Generate(schema, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	// Only the infra file — no entity file for custom definitions.
	for _, f := range plan.Files {
		if strings.Contains(f.Name, "test_widget") {
			t.Error("CustomDefinition should not produce a migration file")
		}
	}
}

func TestGenerate_TableNameInSQL(t *testing.T) {
	schema := buildTestSchema(t, minimalEntity("invoice", "finance"))
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "finance_invoice")
	if !strings.Contains(sql, "finance_invoice") {
		t.Error("generated SQL should reference the entity's qualified name as table name")
	}
}

func TestGenerate_FieldDescription_Comment(t *testing.T) {
	schema := buildTestSchema(t, minimalEntity("widget", "test",
		def.FieldDef{Name: "note", Type: def.FieldTypeData, Description: "A test note field"},
	))
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "test_widget")
	if !strings.Contains(sql, "A test note field") {
		t.Error("field Description should appear as a SQL comment")
	}
}

func TestGenerate_MultipleEntities_FileCount(t *testing.T) {
	schema := buildTestSchema(t,
		minimalEntity("alpha", "test"),
		minimalEntity("beta", "test"),
		minimalEntity("gamma", "test"),
	)
	plan, err := generator.Generate(schema, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	// 1 infra + 3 entity files = 4
	if len(plan.Files) != 4 {
		t.Errorf("expected 4 files, got %d", len(plan.Files))
	}
}

// systemEntity returns a SystemDefinition with ScopeSystem set.
func systemEntity(name, module string, fields ...def.FieldDef) *def.SystemDefinition {
	return &def.SystemDefinition{
		Name:   name,
		Module: module,
		Scope:  def.ScopeSystem,
		Fields: fields,
	}
}

func TestGenerate_ScopeSystem_NoTenantID(t *testing.T) {
	schema := buildTestSchema(t, systemEntity("currency", "finance"))
	plan, err := generator.Generate(schema, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	sql := findEntitySQL(plan, "finance_currency")
	if sql == "" {
		t.Fatal("expected migration file for finance_currency")
	}
	if strings.Contains(sql, "tenant_id") {
		t.Error("ScopeSystem entity must NOT have tenant_id column")
	}
	if strings.Contains(sql, "platform_tenant") {
		t.Error("ScopeSystem entity must NOT reference platform_tenant")
	}
}

func TestGenerate_ScopeSystem_NoRLS(t *testing.T) {
	schema := buildTestSchema(t, systemEntity("currency", "finance"))
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "finance_currency")
	if strings.Contains(sql, "ROW LEVEL SECURITY") {
		t.Error("ScopeSystem entity must NOT have RLS enabled")
	}
	if strings.Contains(sql, "CREATE POLICY") {
		t.Error("ScopeSystem entity must NOT have tenant isolation policy")
	}
	if strings.Contains(sql, "current_tenant_id()") {
		t.Error("ScopeSystem entity must NOT reference current_tenant_id() in policy")
	}
}

func TestGenerate_ScopeSystem_NoTenantIndex(t *testing.T) {
	schema := buildTestSchema(t, systemEntity("currency", "finance"))
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "finance_currency")
	if strings.Contains(sql, "tenant_id_idx") {
		t.Error("ScopeSystem entity must NOT have tenant_id index")
	}
}

func TestGenerate_ScopeSystem_HasUpdatedAtTrigger(t *testing.T) {
	schema := buildTestSchema(t, systemEntity("currency", "finance"))
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "finance_currency")
	if !strings.Contains(sql, "awo_set_updated_at") {
		t.Error("ScopeSystem entity still needs updated_at trigger")
	}
}

func TestGenerate_ScopeTenant_HasTenantIDAndRLS(t *testing.T) {
	schema := buildTestSchema(t, minimalEntity("invoice", "finance"))
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "finance_invoice")
	if !strings.Contains(sql, "tenant_id") {
		t.Error("ScopeTenant entity must have tenant_id column")
	}
	if !strings.Contains(sql, "ROW LEVEL SECURITY") {
		t.Error("ScopeTenant entity must have RLS enabled")
	}
	if !strings.Contains(sql, "CREATE POLICY") {
		t.Error("ScopeTenant entity must have tenant isolation policy")
	}
}

func TestGenerate_InfraFile_HasAuditLogStub(t *testing.T) {
	schema := buildTestSchema(t, minimalEntity("widget", "test"))
	plan, err := generator.Generate(schema, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	infra := plan.Files[0]
	if !strings.Contains(infra.SQL, "awo_audit_log") {
		t.Error("infrastructure file should define awo_audit_log() stub trigger function")
	}
}

// findEntitySQL searches the plan for a file containing the entity name and returns its SQL.
func findEntitySQL(plan *generator.Plan, entityName string) string {
	for _, f := range plan.Files {
		if strings.Contains(f.Name, entityName) {
			return f.SQL
		}
	}
	return ""
}

// orgScopedEntity returns a SystemDefinition with ScopeOrganization set.
func orgScopedEntity(name, module string, scope def.Scope, fields ...def.FieldDef) *def.SystemDefinition {
	return &def.SystemDefinition{
		Name:   name,
		Module: module,
		Scope:  scope,
		Fields: fields,
	}
}

func TestGenerate_ScopeOrganization_RLSPolicy(t *testing.T) {
	schema := buildTestSchema(t, orgScopedEntity("budget", "finance", def.ScopeOrganization))
	plan, err := generator.Generate(schema, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	sql := findEntitySQL(plan, "finance_budget")
	if !strings.Contains(sql, "org_isolation") {
		t.Error("ScopeOrganization entity must have org isolation policy")
	}
	if !strings.Contains(sql, "current_org_id()") {
		t.Error("ScopeOrganization RLS policy must use current_org_id()")
	}
}

func TestGenerate_ScopeOrganization_OrgIDIndex(t *testing.T) {
	schema := buildTestSchema(t, orgScopedEntity("budget", "finance", def.ScopeOrganization))
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "finance_budget")
	if !strings.Contains(sql, "tenant_id, org_id") {
		t.Error("ScopeOrganization entity must have composite (tenant_id, org_id) index")
	}
}

func TestGenerate_ScopeOrganizationTree_RLSPolicy(t *testing.T) {
	schema := buildTestSchema(t, orgScopedEntity("approval", "platform", def.ScopeOrganizationTree))
	plan, err := generator.Generate(schema, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	sql := findEntitySQL(plan, "platform_approval")
	if !strings.Contains(sql, "org_tree_isolation") {
		t.Error("ScopeOrganizationTree entity must have org tree isolation policy")
	}
	if !strings.Contains(sql, "current_org_path()") {
		t.Error("ScopeOrganizationTree RLS policy must use current_org_path()")
	}
}

func TestGenerate_ScopeOrganizationTree_OrgIDIndex(t *testing.T) {
	schema := buildTestSchema(t, orgScopedEntity("approval", "platform", def.ScopeOrganizationTree))
	plan, _ := generator.Generate(schema, generator.Options{})
	sql := findEntitySQL(plan, "platform_approval")
	if !strings.Contains(sql, "tenant_id, org_id") {
		t.Error("ScopeOrganizationTree entity must have composite (tenant_id, org_id) index")
	}
}

func TestGenerate_InfraFile_HasSetOrgContext(t *testing.T) {
	schema := buildTestSchema(t, minimalEntity("widget", "test"))
	plan, err := generator.Generate(schema, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	infra := plan.Files[0]
	if !strings.Contains(infra.SQL, "set_org_context") {
		t.Error("infrastructure file should define set_org_context()")
	}
	if !strings.Contains(infra.SQL, "current_org_id") {
		t.Error("infrastructure file should define current_org_id()")
	}
	if !strings.Contains(infra.SQL, "current_org_path") {
		t.Error("infrastructure file should define current_org_path()")
	}
}
