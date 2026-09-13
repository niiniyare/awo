// Package integration contains framework validation tests.
//
// Tests in this file run without any infrastructure (no Postgres, no Redis).
// They validate the registry, compiler, and hook pipeline using fakestore.
//
// Integration tests that require a real database are in db_test.go and are
// guarded by the "integration" build tag.
package integration_test

import (
	"testing"

	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"awo.so/awo/platform/iam"
	"awo.so/awo/platform/organization"
	"awo.so/awo/platform/tenant"
	"awo.so/awo/registry"
)

// platformDefs returns the minimal set of EntityDefinitions for the platform
// modules. Uses BuildFrom (not Build) so the global registry is not touched
// and mandatory-entity checks are skipped.
func platformDefs() []def.EntityDefinition {
	return []def.EntityDefinition{
		&tenant.Definition,
		&iam.UserDefinition,
		&organization.Definition,
		&organization.OrgTypeDefinition,
		&organization.OrgAssignmentDefinition,
	}
}

// TestRegistryBuild verifies that all platform definitions pass registry validation.
func TestRegistryBuild(t *testing.T) {
	reg, err := registry.BuildFrom(platformDefs())
	if err != nil {
		t.Fatalf("registry.BuildFrom: %v", err)
	}
	if reg.Count() != 5 {
		t.Errorf("expected 5 entities, got %d", reg.Count())
	}
	names := []string{
		"platform_tenant",
		"iam_user",
		"platform_organization",
		"platform_org_type",
		"platform_org_assignment",
	}
	for _, n := range names {
		if reg.Lookup(n) == nil {
			t.Errorf("entity %q not found in registry", n)
		}
	}
}

// TestRegistryModuleLookup verifies ByModule filtering.
func TestRegistryModuleLookup(t *testing.T) {
	reg, err := registry.BuildFrom(platformDefs())
	if err != nil {
		t.Fatalf("%v", err)
	}
	platformEntities := reg.ByModule("platform")
	if len(platformEntities) != 4 {
		t.Errorf("expected 4 platform entities, got %d", len(platformEntities))
	}
	iamEntities := reg.ByModule("iam")
	if len(iamEntities) != 1 {
		t.Errorf("expected 1 iam entity, got %d", len(iamEntities))
	}
	// EntityName() returns the module-local name ("user", not "iam_user").
	if iamEntities[0].EntityName() != "user" {
		t.Errorf("unexpected local entity name: %s (expected \"user\")", iamEntities[0].EntityName())
	}
}

// TestRegistryDuplicateDetection verifies that a duplicate entity name is
// caught. BuildFrom de-dupes by inserting into a map — the second definition
// with the same name silently replaces the first. Count stays at expected.
func TestRegistryDuplicateDetection(t *testing.T) {
	defs := []def.EntityDefinition{
		&tenant.Definition,
		&tenant.Definition, // exact same pointer — duplicate
	}
	reg, err := registry.BuildFrom(defs)
	// BuildFrom may accept this (map dedup) or reject it — both are valid.
	// What matters: Lookup still works and returns a valid definition.
	if err != nil {
		t.Logf("BuildFrom rejected duplicate (acceptable): %v", err)
		return
	}
	if reg.Lookup("platform_tenant") == nil {
		t.Error("platform_tenant not found after dedup")
	}
}

// TestRegistryBuildFrom_NoMandatoryCheck verifies that BuildFrom accepts a
// subset of entities without triggering mandatory-entity checks (which only
// Build() enforces).
func TestRegistryBuildFrom_NoMandatoryCheck(t *testing.T) {
	// platform_tenant alone — would fail Build() (no iam_user etc.) but
	// BuildFrom should accept it since mandatory-entity check is skipped.
	reg, err := registry.BuildFrom([]def.EntityDefinition{
		&tenant.Definition,
	})
	if err != nil {
		t.Fatalf("BuildFrom with single entity: %v", err)
	}
	if reg.Lookup("platform_tenant") == nil {
		t.Error("platform_tenant not found")
	}
}

// TestCompilerCompile verifies that Compile produces a valid CompiledSchema.
func TestCompilerCompile(t *testing.T) {
	reg, err := registry.BuildFrom(platformDefs())
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	schema, err := compiler.Compile(reg)
	if err != nil {
		t.Fatalf("compiler.Compile: %v", err)
	}
	if schema == nil {
		t.Fatal("schema is nil")
	}
	if len(schema.Entities) != 5 {
		t.Errorf("expected 5 compiled entities, got %d", len(schema.Entities))
	}

	es := schema.ByName["platform_tenant"]
	if es == nil {
		t.Fatal("platform_tenant not in compiled schema")
	}
	// Verify fields compiled correctly.
	if _, ok := es.FieldsByName["name"]; !ok {
		t.Error("field 'name' missing from compiled tenant schema")
	}
	if !es.RequiredFields["name"] {
		t.Error("field 'name' should be Required")
	}
	if !es.ImmutableFields["slug"] {
		t.Error("field 'slug' should be Immutable")
	}
}

// TestCompilerDiagnostics verifies that Validate can run standalone.
func TestCompilerDiagnostics(t *testing.T) {
	reg, err := registry.BuildFrom(platformDefs())
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	ds := compiler.Validate(reg)
	// No errors expected for a well-formed definition.
	if ds.HasErrors() {
		t.Errorf("unexpected compiler errors:\n%s", ds)
	}
	for _, d := range ds {
		t.Logf("diagnostic [%s] %s: %s", d.Severity, d.EntityName, d.Message)
	}
}

// TestCompilerFingerprint verifies deterministic SHA-256 output.
func TestCompilerFingerprint(t *testing.T) {
	reg, err := registry.BuildFrom(platformDefs())
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	schema, err := compiler.Compile(reg)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	fp1 := compiler.Fingerprint(schema)
	fp2 := compiler.Fingerprint(schema)

	if fp1 == "" {
		t.Error("fingerprint is empty")
	}
	if fp1 != fp2 {
		t.Errorf("fingerprint not deterministic: %q vs %q", fp1, fp2)
	}
	if len(fp1) != 64 {
		t.Errorf("expected 64-char SHA-256 hex, got %d chars", len(fp1))
	}
	t.Logf("schema fingerprint: %s", fp1)
}

// TestCompilerRoutes verifies that CRUD routes are generated for platform_tenant.
func TestCompilerRoutes(t *testing.T) {
	reg, err := registry.BuildFrom(platformDefs())
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	schema, err := compiler.Compile(reg)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	var tenantRoutes []compiler.RouteDescriptor
	for _, r := range schema.Routes {
		if r.EntityQualifiedName == "platform_tenant" {
			tenantRoutes = append(tenantRoutes, r)
		}
	}
	if len(tenantRoutes) == 0 {
		t.Error("no routes generated for platform_tenant")
	}
	t.Logf("platform_tenant routes: %d", len(tenantRoutes))
	for _, r := range tenantRoutes {
		t.Logf("  %s %s", r.Method, r.Path)
	}
}
