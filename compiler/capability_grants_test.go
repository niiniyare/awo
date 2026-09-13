package compiler

import (
	"testing"

	"awo.so/awo/def"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// invoiceDef returns a minimal finance_invoice definition with a full
// PermissionSet using permission identifiers (ADR-011).
func invoiceDef() def.EntityDefinition {
	return &def.SystemDefinition{
		Name:        "invoice",
		Module:      "finance",
		Label:       "Invoice",
		LabelPlural: "Invoices",
		Permissions: def.PermissionSet{
			Create: []string{"finance.invoice.create"},
			Read:   []string{"finance.invoice.read"},
			Write:  []string{"finance.invoice.update"},
			Delete: []string{"finance.invoice.delete"},
			Actions: map[string][]string{
				"submit":  {"finance.invoice.submit"},
				"approve": {"finance.invoice.approve"},
			},
		},
	}
}

// TestCapabilityGrants_EmittedFromPermissionSet verifies that Compile produces
// CapabilityGrant records (not CasbinPolicy) and that each grant contains a
// permission identifier — never a role name.
func TestCapabilityGrants_EmittedFromPermissionSet(t *testing.T) {
	schema := mustCompile([]def.EntityDefinition{invoiceDef()})

	grants := schema.CapabilityGrants
	require.NotEmpty(t, grants)

	// Build a lookup map: (permission, entity, action) → true
	type key struct{ perm, entity, action string }
	grantSet := make(map[key]bool, len(grants))
	for _, g := range grants {
		grantSet[key{g.Permission, g.Entity, g.Action}] = true

		// INVARIANT: Permission MUST NOT look like a role name.
		assert.NotContains(t, g.Permission, "role:",
			"CapabilityGrant.Permission must be a permission identifier, not a role name (got %q)", g.Permission)

		// INVARIANT: Entity must be the qualified name.
		assert.Equal(t, "finance_invoice", g.Entity,
			"CapabilityGrant.Entity must be the qualified entity name")
	}

	// Verify every declared permission produced a CapabilityGrant.
	expected := []key{
		{"finance.invoice.create", "finance_invoice", "create"},
		{"finance.invoice.read", "finance_invoice", "read"},
		{"finance.invoice.update", "finance_invoice", "write"},
		{"finance.invoice.delete", "finance_invoice", "delete"},
		{"finance.invoice.submit", "finance_invoice", "submit"},
		{"finance.invoice.approve", "finance_invoice", "approve"},
	}
	for _, e := range expected {
		assert.True(t, grantSet[e],
			"expected CapabilityGrant{Permission:%q, Entity:%q, Action:%q}", e.perm, e.entity, e.action)
	}
}

// TestCapabilityGrants_EmptyPermissionSet produces no grants for that entity.
func TestCapabilityGrants_EmptyPermissionSet(t *testing.T) {
	schema := mustCompile([]def.EntityDefinition{simpleDef("item", "inventory")})
	assert.Empty(t, schema.CapabilityGrants,
		"entity with empty PermissionSet must produce no CapabilityGrants")
}

// TestCapabilityGrants_FieldNames verifies CapabilityGrant has Permission/Entity/Action
// fields (not Subject/Object/Action from the retired CasbinPolicy type — ADR-011).
func TestCapabilityGrants_FieldNames(t *testing.T) {
	schema := mustCompile([]def.EntityDefinition{invoiceDef()})
	require.NotEmpty(t, schema.CapabilityGrants)
	g := schema.CapabilityGrants[0]
	// These compile-time field accesses fail if the rename hasn't happened.
	_ = g.Permission
	_ = g.Entity
	_ = g.Action
}

// TestCompiledSchema_CapabilityGrants_NotCasbinPolicies verifies CompiledSchema
// exposes CapabilityGrants. If CasbinPolicies still existed this test would
// also compile — but the missing field would fail at build time.
func TestCompiledSchema_CapabilityGrants_NotCasbinPolicies(t *testing.T) {
	schema := mustCompile([]def.EntityDefinition{invoiceDef()})
	grants := schema.CapabilityGrants
	assert.NotNil(t, grants)
}
