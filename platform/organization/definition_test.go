package organization_test

import (
	"testing"

	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"awo.so/awo/platform/organization"
	"awo.so/awo/registry"
)

// buildOrgSchema registers the organization entities and compiles them.
// Minimal stubs for FK targets (platform_tenant, iam_user) are included so
// the compiler's LinkTarget check passes without importing the full IAM module.
func buildOrgSchema(t *testing.T) *compiler.CompiledSchema {
	t.Helper()
	tenantDef := &def.SystemDefinition{
		Name:   "tenant",
		Module: "platform",
		Scope:  def.ScopeSystem,
	}
	iamUserDef := &def.SystemDefinition{
		Name:   "user",
		Module: "iam",
	}
	reg, err := registry.BuildFrom([]def.EntityDefinition{
		tenantDef,
		iamUserDef,
		&organization.Definition,
		&organization.OrgTypeDefinition,
		&organization.OrgAssignmentDefinition,
	})
	if err != nil {
		t.Fatalf("BuildFrom: %v", err)
	}
	schema, err := compiler.Compile(reg)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	return schema
}

// TestOrganizationEntity_Registers verifies the entity definition compiles
// without error.
func TestOrganizationEntity_Registers(t *testing.T) {
	schema := buildOrgSchema(t)
	if schema.ByName["platform_organization"] == nil {
		t.Error("platform_organization not found in compiled schema")
	}
}

// TestOrganizationEntity_HasParentID verifies the parent_id field is a
// FieldTypeLink pointing to platform_organization (self-referential).
func TestOrganizationEntity_HasParentID(t *testing.T) {
	var parentField *def.FieldDef
	for i := range organization.Definition.Fields {
		f := &organization.Definition.Fields[i]
		if f.Name == "parent_id" {
			parentField = f
			break
		}
	}
	if parentField == nil {
		t.Fatal("parent_id field missing from platform_organization")
	}
	if parentField.Type != def.FieldTypeLink {
		t.Errorf("parent_id type: got %q, want FieldTypeLink", parentField.Type)
	}
	if parentField.LinkTarget != "platform_organization" {
		t.Errorf("parent_id LinkTarget: got %q, want %q", parentField.LinkTarget, "platform_organization")
	}
}

// TestOrganizationEntity_SelfRef verifies the compiler accepts the self-
// referential link without a cycle error.
func TestOrganizationEntity_SelfRef(t *testing.T) {
	schema := buildOrgSchema(t)
	// If compilation reached here without error, self-ref is accepted.
	if len(schema.Entities) == 0 {
		t.Error("expected entities in schema")
	}
}

// TestOrganizationEntity_Scope verifies the entity scope is ScopeTenant
// (organizations belong to a tenant, not themselves org-scoped).
func TestOrganizationEntity_Scope(t *testing.T) {
	got := organization.Definition.EntityScope()
	if got != def.ScopeTenant {
		t.Errorf("platform_organization scope: got %q, want ScopeTenant", got)
	}
}

// TestOrganizationEntity_AllowAudit verifies audit is enabled (DisableAudit
// not set — all organization mutations should be audited).
func TestOrganizationEntity_AllowAudit(t *testing.T) {
	if !organization.Definition.AllowAudit() {
		t.Error("platform_organization: AllowAudit() must return true")
	}
}

// TestOrganizationEntity_HasPathField verifies a path field exists for the
// materialized path hierarchy.
func TestOrganizationEntity_HasPathField(t *testing.T) {
	for _, f := range organization.Definition.Fields {
		if f.Name == "path" {
			return
		}
	}
	t.Error("platform_organization: path field missing")
}

// TestOrganizationEntity_HasDepthField verifies a depth field exists.
func TestOrganizationEntity_HasDepthField(t *testing.T) {
	for _, f := range organization.Definition.Fields {
		if f.Name == "depth" {
			if f.Type != def.FieldTypeInt {
				t.Errorf("depth field type: got %q, want FieldTypeInt", f.Type)
			}
			return
		}
	}
	t.Error("platform_organization: depth field missing")
}

// TestOrganizationEntity_PermissionIdentifiers verifies that permission
// identifiers follow the platform.organization.{operation} convention and
// contain no role names or RBAC constructs.
func TestOrganizationEntity_PermissionIdentifiers(t *testing.T) {
	perms := organization.Definition.EntityPermissions()
	for _, p := range perms.Create {
		if p != "platform.organization.create" {
			t.Errorf("unexpected create permission %q", p)
		}
	}
	for _, p := range perms.Read {
		if p != "platform.organization.read" {
			t.Errorf("unexpected read permission %q", p)
		}
	}
}
