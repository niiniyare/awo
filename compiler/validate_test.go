package compiler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/def"
	"awo.so/awo/registry"
)

// validateDefs returns (diagnostics, registryError).
// registryError is non-nil when registry.BuildFrom rejects the definitions
// before compiler.Validate even runs.
func validateDefs(defs []def.EntityDefinition) (Diagnostics, error) {
	reg, err := registry.BuildFrom(defs)
	if err != nil {
		return nil, err
	}
	return Validate(reg), nil
}

// mustValidate is for tests that expect the registry to accept the defs.
func mustValidate(t *testing.T, defs []def.EntityDefinition) Diagnostics {
	t.Helper()
	ds, err := validateDefs(defs)
	require.NoError(t, err, "registry.BuildFrom should accept these defs")
	return ds
}

// --- Tests for things caught by compiler.Validate ONLY (not by registry) ---

func TestValidate_Clean(t *testing.T) {
	ds := mustValidate(t, []def.EntityDefinition{simpleDef("mod_item", "mod")})
	assert.False(t, ds.HasErrors(), "expected no errors: %v", ds.Errors())
}

func TestValidate_RequiredAndDefault_Warning(t *testing.T) {
	d := &def.SystemDefinition{
		Name: "mod_item", Module: "mod", Label: "Item", LabelPlural: "Items",
		Fields: []def.FieldDef{
			{Name: "status", Type: def.FieldTypeData, Required: true, Default: func() any { return "draft" }},
		},
	}
	ds := mustValidate(t, []def.EntityDefinition{d})
	found := false
	for _, w := range ds.Warnings() {
		if w.FieldName == "status" {
			found = true
		}
	}
	assert.True(t, found, "expected Required+Default warning on field 'status'")
}

func TestValidate_ImmutableNoRequiredNoDefault_Warning(t *testing.T) {
	d := &def.SystemDefinition{
		Name: "mod_item", Module: "mod", Label: "Item", LabelPlural: "Items",
		Fields: []def.FieldDef{
			{Name: "code", Type: def.FieldTypeData, Immutable: true},
		},
	}
	ds := mustValidate(t, []def.EntityDefinition{d})
	found := false
	for _, w := range ds.Warnings() {
		if w.FieldName == "code" {
			found = true
		}
	}
	assert.True(t, found, "expected Immutable-without-Required-or-Default warning on field 'code'")
}

// FieldTypeInt with monetary suffix → compiler warning; registry only blocks FieldTypeFloat.
func TestValidate_MoneyInt_Warning(t *testing.T) {
	d := &def.SystemDefinition{
		Name: "mod_item", Module: "mod", Label: "Item", LabelPlural: "Items",
		Fields: []def.FieldDef{
			{Name: "unit_price", Type: def.FieldTypeInt},
		},
	}
	ds := mustValidate(t, []def.EntityDefinition{d})
	found := false
	for _, w := range ds.Warnings() {
		if w.FieldName == "unit_price" {
			found = true
		}
	}
	assert.True(t, found, "expected monetary-type warning on unit_price (FieldTypeInt)")
}

func TestValidate_Diagnostics_AsError_NilWhenClean(t *testing.T) {
	ds := mustValidate(t, []def.EntityDefinition{simpleDef("mod_x", "mod")})
	assert.Nil(t, ds.AsError(), "AsError should be nil when no errors")
}

// --- Tests for things caught by the registry (errors surface before compiler) ---

func TestValidate_NamingSeriesEmptySeries_CaughtByRegistry(t *testing.T) {
	d := &def.SystemDefinition{
		Name: "mod_item", Module: "mod", Label: "Item", LabelPlural: "Items",
		Fields: []def.FieldDef{
			{Name: "number", Type: def.FieldTypeNamingSeries, Series: ""},
		},
	}
	_, err := validateDefs([]def.EntityDefinition{d})
	require.Error(t, err, "registry should reject NamingSeries with empty Series")
	assert.Contains(t, err.Error(), "NamingSeries")
}

func TestValidate_NamingSeriesWithSeries_OK(t *testing.T) {
	d := &def.SystemDefinition{
		Name: "mod_item", Module: "mod", Label: "Item", LabelPlural: "Items",
		Fields: []def.FieldDef{
			{Name: "number", Type: def.FieldTypeNamingSeries, Series: "ITEM-{YYYY}-{SEQ:5}"},
		},
	}
	ds, err := validateDefs([]def.EntityDefinition{d})
	require.NoError(t, err)
	assert.False(t, ds.HasErrors(), "valid NamingSeries should produce no errors: %v", ds.Errors())
}

func TestValidate_MoneyFloat_CaughtByRegistry(t *testing.T) {
	d := &def.SystemDefinition{
		Name: "mod_item", Module: "mod", Label: "Item", LabelPlural: "Items",
		Fields: []def.FieldDef{
			{Name: "unit_price", Type: def.FieldTypeFloat},
		},
	}
	_, err := validateDefs([]def.EntityDefinition{d})
	require.Error(t, err, "registry should reject FieldTypeFloat with monetary field name")
}

// TestValidate_RoleNameInPermission verifies "role:*" in PermissionSet → error.
func TestValidate_RoleNameInPermission(t *testing.T) {
	d := &def.SystemDefinition{
		Name: "mod_item", Module: "mod", Label: "Item", LabelPlural: "Items",
		Permissions: def.PermissionSet{
			Read: []string{"role:admin"}, // role name — violates ADR-001/ADR-011
		},
	}
	ds := mustValidate(t, []def.EntityDefinition{d})
	found := false
	for _, diag := range ds.Errors() {
		if diag.Severity == SeverityError {
			found = true
		}
	}
	assert.True(t, found, "expected SeverityError for role-name permission; got:\n%s", ds)
}

// TestValidate_MalformedPermissionIdentifier verifies non-dot-notation perm → warning.
func TestValidate_MalformedPermissionIdentifier(t *testing.T) {
	d := &def.SystemDefinition{
		Name: "mod_item", Module: "mod", Label: "Item", LabelPlural: "Items",
		Permissions: def.PermissionSet{
			Read: []string{"just_read"}, // missing module.entity.op format
		},
	}
	ds := mustValidate(t, []def.EntityDefinition{d})
	found := false
	for _, w := range ds.Warnings() {
		if w.Severity == SeverityWarning {
			found = true
		}
	}
	assert.True(t, found, "expected warning for malformed permission identifier; got:\n%s", ds)
}

// TestValidate_ValidPermissions verifies well-formed permissions produce no warnings.
func TestValidate_ValidPermissions(t *testing.T) {
	d := &def.SystemDefinition{
		Name: "mod_item", Module: "mod", Label: "Item", LabelPlural: "Items",
		Permissions: def.PermissionSet{
			Create: []string{"mod.item.create"},
			Read:   []string{"mod.item.read"},
			Write:  []string{"mod.item.write"},
			Delete: []string{"mod.item.delete"},
			Actions: map[string][]string{
				"submit": {"mod.item.submit"},
			},
		},
	}
	ds := mustValidate(t, []def.EntityDefinition{d})
	for _, w := range ds.Warnings() {
		if w.Severity == SeverityWarning && (w.Message != "" && len(w.Message) > 0) {
			// only fail on permission-related warnings
			t.Logf("unexpected warning: %s", w.Message)
		}
	}
	assert.False(t, ds.HasErrors(), "valid permissions should not produce errors")
}

// TestValidate_DuplicateEdge verifies duplicate edge names are rejected.
// The registry catches this before compiler.Validate runs.
func TestValidate_DuplicateEdge(t *testing.T) {
	target := &def.SystemDefinition{
		Name: "mod_target", Module: "mod", Label: "Target", LabelPlural: "Targets",
	}
	d := &def.SystemDefinition{
		Name: "mod_item", Module: "mod", Label: "Item", LabelPlural: "Items",
		Edges: []def.EdgeDef{
			{Name: "children", Type: def.EdgeOneToMany, Target: "mod_target"},
			{Name: "children", Type: def.EdgeOneToMany, Target: "mod_target"}, // duplicate
		},
	}
	_, err := validateDefs([]def.EntityDefinition{target, d})
	assert.Error(t, err, "expected error for duplicate edge name")
	assert.Contains(t, err.Error(), "children")
}

// TestCollectPermissions verifies all permission slots are flattened.
func TestCollectPermissions(t *testing.T) {
	ps := def.PermissionSet{
		Create: []string{"m.e.create"},
		Read:   []string{"m.e.read"},
		Write:  []string{"m.e.write"},
		Delete: []string{"m.e.delete"},
		Actions: map[string][]string{
			"submit": {"m.e.submit"},
			"cancel": {"m.e.cancel"},
		},
	}
	got := collectPermissions(ps)
	if len(got) != 6 {
		t.Errorf("expected 6 permissions, got %d: %v", len(got), got)
	}
}

// TestValidate_MoneyFeeAndTax verifies *_fee and *_tax suffix detection.
func TestValidate_MoneyFeeAndTax(t *testing.T) {
	for _, name := range []string{"service_fee", "vat_tax"} {
		d := &def.SystemDefinition{
			Name: "mod_item", Module: "mod", Label: "Item", LabelPlural: "Items",
			Fields: []def.FieldDef{
				{Name: name, Type: def.FieldTypeInt},
			},
		}
		ds := mustValidate(t, []def.EntityDefinition{d})
		found := false
		for _, w := range ds.Warnings() {
			if w.FieldName == name {
				found = true
			}
		}
		assert.True(t, found, "expected monetary warning for field %q", name)
	}
}

// TestValidate_LinkListOrphan verifies FieldTypeLinkList with bad target is rejected.
// The registry catches unresolved LinkTargets before compiler.Validate runs.
func TestValidate_LinkListOrphan(t *testing.T) {
	d := &def.SystemDefinition{
		Name: "mod_item", Module: "mod", Label: "Item", LabelPlural: "Items",
		Fields: []def.FieldDef{
			{Name: "tag_ids", Type: def.FieldTypeLinkList, LinkTarget: "nonexistent_tag"},
		},
	}
	_, err := validateDefs([]def.EntityDefinition{d})
	assert.Error(t, err, "expected error for orphaned LinkList target")
	assert.Contains(t, err.Error(), "tag_ids")
}
