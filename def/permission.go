package def

// PermissionSet declares the authorization gates for each operation on an entity.
// Each field is a list of permission identifiers — stable, engine-agnostic
// capability names that identify what a subject must be granted to perform the
// operation. An empty slice means "deny all" (no one can perform the operation).
//
// Permission identifiers use the format "{module}.{entity}.{operation}".
// Examples: "finance.invoice.create", "iam.user.read", "inventory.stock_item.delete".
//
// Permission identifiers are NEVER role names. Role-to-permission mapping is
// managed by the IAM module and loaded separately by the PolicyEvaluator at
// startup. Mixing roles into PermissionSet would couple EntityDefinition to a
// specific authorization backend — a violation of ADR-001 and ADR-011.
//
// The platform-admin role bypasses all authorization checks unconditionally and
// is never referenced here — the PolicyEvaluator short-circuits it before
// consulting the compiled CapabilityGrants.
//
// The compiler transforms PermissionSet values into [CapabilityGrant] records.
// The PolicyEvaluator loads CapabilityGrants and a separate role-to-permission
// mapping to resolve authorization decisions at request time.
type PermissionSet struct {
	// Create lists permission identifiers required to create new records.
	// e.g. []string{"finance.invoice.create"}
	Create []string

	// Read lists permission identifiers required to read records.
	// e.g. []string{"finance.invoice.read"}
	Read []string

	// Write lists permission identifiers required to update existing records.
	// e.g. []string{"finance.invoice.update"}
	Write []string

	// Delete lists permission identifiers required to delete records.
	// e.g. []string{"finance.invoice.delete"}
	Delete []string

	// Actions maps action names to the permission identifiers required to invoke
	// them. The framework verifies the caller holds the permission declared on
	// [ActionDef.Permission] before the action handler is invoked.
	// e.g. map[string][]string{"submit": {"finance.invoice.submit"}}
	Actions map[string][]string

	// Policy is the row-level filter applied to all read operations for this
	// entity. Nil means no additional row restriction beyond RLS. Policy runs
	// after the PolicyEvaluator's operation-level check and may further narrow
	// the result set based on the authenticated viewer's attributes.
	Policy PolicyFunc
}
