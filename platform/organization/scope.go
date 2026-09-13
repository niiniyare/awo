package organization

// VisibilityMode controls which organization nodes are included when
// OrganizationService.ResolveScope resolves the set visible to a viewer.
//
// # Two-stage isolation model
//
// VisibilityMode operates entirely in the application layer (Stage 2).
// Tenant isolation (Stage 1) is handled by PostgreSQL RLS and is never
// affected by VisibilityMode. The two stages are orthogonal:
//
//	Stage 1: RLS fires on every DB query — tenant_id = current_tenant_id().
//	         This prevents cross-tenant leaks. VisibilityMode plays no role.
//
//	Stage 2: ResolveScope(viewer) → []uuid.UUID of visible org IDs.
//	         Application service appends org_id IN (…) to the filter.
//	         Repositories receive only typed filter predicates.
//
// Repositories are organization-agnostic. They never compute visibility.
// They receive the resolved filter and execute queries inside the tenant
// RLS context. Both isolation stages fire independently.
//
// # Canonical request pipeline
//
//	Request
//	  → Authentication (session validation)
//	  → ViewerContext (load org assignments, roles)
//	  → OrganizationService.ResolveScope()   ← Stage 2
//	  → Append org filter to query predicates
//	  → Repository.Query()
//	  → Tenant RLS fires                     ← Stage 1
//	  → Database
type VisibilityMode int

const (
	// VisibilitySelf: viewer's active organization only.
	// Typical use: branch cashier, data-entry clerk for one branch.
	VisibilitySelf VisibilityMode = iota

	// VisibilityChildren: direct children of the viewer's active org.
	// Does not include the viewer's own node or deeper descendants.
	// Typical use: a parent org viewing immediate sub-units.
	VisibilityChildren

	// VisibilitySubtree: viewer's active org and all descendants recursively.
	// Uses materialized path prefix for O(1) query — no recursive CTE.
	// Typical use: regional manager viewing all branches under their region.
	VisibilitySubtree

	// VisibilityParent: viewer's active org and all ancestors up to the root.
	// Typical use: breadcrumb navigation; escalation path display.
	VisibilityParent

	// VisibilityAssigned: all organizations the viewer is explicitly assigned
	// to via platform_org_assignment, regardless of tree position.
	// Typical use: HR business partner assigned to multiple unrelated departments.
	VisibilityAssigned

	// VisibilityExplicit: a caller-supplied fixed set of org IDs.
	// Requires ViewerContext.ExplicitOrganizationIDs to be populated.
	// Typical use: internal auditor assigned to a non-contiguous org set.
	VisibilityExplicit

	// VisibilityEntireTenant: all org nodes in the tenant — no org filter.
	// Returns nil from ResolveScope; application service omits org predicate.
	// Typical use: tenant.admin, HQ Finance Manager, platform.admin.
	VisibilityEntireTenant

	// VisibilityCustom: delegates to ViewerContext.ScopeResolver.
	// Typical use: matrix organizations; project-team cross-org access.
	VisibilityCustom
)

// String returns a stable, human-readable name for the mode constant.
func (m VisibilityMode) String() string {
	switch m {
	case VisibilitySelf:
		return "self"
	case VisibilityChildren:
		return "children"
	case VisibilitySubtree:
		return "subtree"
	case VisibilityParent:
		return "parent"
	case VisibilityAssigned:
		return "assigned"
	case VisibilityExplicit:
		return "explicit"
	case VisibilityEntireTenant:
		return "entire_tenant"
	case VisibilityCustom:
		return "custom"
	default:
		return "unknown"
	}
}
