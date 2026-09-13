package def

// Scope declares the data isolation boundary for an entity.
//
// The scope controls which rows a viewer can access by default, and what
// tenant/organization context the framework injects into repository queries.
//
// Scope is declared on [EntityDefinition] via [SystemDefinition.Scope] and
// [CustomDefinition.Scope]. The default (zero value) is [ScopeTenant].
//
// Note: Scope declares intent. Enforcement is layered:
//  1. PolicyFunc — application-layer pre-filter (optional optimization)
//  2. PostgreSQL RLS — authoritative row-level enforcement
//
// Never rely on Scope alone as a security boundary.
type Scope string

const (
	// ScopeSystem indicates rows are global (not tenant-scoped). Used for
	// platform-level tables such as platform_tenants and global IAM configuration.
	// System entities have no tenant_id column and no RLS policy.
	// Example: platform_tenant, iam_roles, iam_role_permissions.
	ScopeSystem Scope = "system"

	// ScopeTenant is the default. Rows are scoped to a single tenant via
	// tenant_id + PostgreSQL RLS. All ordinary business entities use this scope.
	// Example: iam_user, finance_invoice, finance_payment.
	ScopeTenant Scope = "tenant"

	// ScopeOrganization rows are further restricted to a specific organization
	// within a tenant. The entity must have an organization_id column.
	// The framework injects an additional filter for the viewer's organization.
	// Example: finance_budget (per-branch), inventory_location (per-warehouse).
	ScopeOrganization Scope = "organization"

	// ScopeOrganizationTree rows are visible to the viewer's organization and
	// all ancestor organizations up the hierarchy. Used for entities where
	// parent organizations need visibility into children's data.
	// Requires ltree-enabled organization hierarchy (platform_organizations).
	// Example: approval records visible to management chain.
	ScopeOrganizationTree Scope = "organization_tree"

	// ScopeUser rows are private to the creating user. No other user (including
	// tenant admins) can read them without explicit sharing. Used for
	// personal drafts, private notes, and user-specific configuration.
	// Example: user_draft, user_preference.
	ScopeUser Scope = "user"
)
