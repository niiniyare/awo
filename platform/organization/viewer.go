package organization

import (
	"context"

	"github.com/google/uuid"
)

// ViewerContext carries the identity and organization membership of the
// authenticated user through a request. It is the application's view of who
// is acting — richer than TenantContext (which only carries tenant + locale).
//
// The framework populates ViewerContext in auth middleware after session
// validation. Application services read it to evaluate organization scope,
// apply permission policies, and build audit records.
//
// ViewerContext is orthogonal to TenantContext:
//   - TenantContext: framework — which tenant does this request belong to?
//   - ViewerContext: application — who within that tenant is acting, and in
//     which organizational context?
//
// Never store secrets, tokens, or raw credentials in ViewerContext.
type ViewerContext struct {
	// TenantID is the tenant the viewer belongs to. Matches TenantContext.
	TenantID uuid.UUID

	// UserID is the authenticated user's UUID.
	UserID uuid.UUID

	// ActiveOrganizationID is the organization the user is currently operating
	// within. A user with multiple assignments can switch their active org during
	// a session. uuid.Nil means no active org selected (uses primary org).
	ActiveOrganizationID uuid.UUID

	// PrimaryOrganizationID is the user's home org from platform_org_assignment
	// where is_primary = true. Used as default when ActiveOrganizationID is nil.
	PrimaryOrganizationID uuid.UUID

	// OrganizationAssignments is the full list of org memberships for this user
	// in the current tenant. Loaded once at session validation time.
	OrganizationAssignments []OrgMembership

	// VisibilityMode controls which organizations are visible to this viewer.
	// Set by middleware based on the viewer's roles and active organization.
	VisibilityMode VisibilityMode

	// ExplicitOrganizationIDs is the caller-supplied fixed set of org IDs.
	// Populated only when VisibilityMode == VisibilityExplicit.
	ExplicitOrganizationIDs []uuid.UUID

	// ScopeResolver is a custom resolver invoked when VisibilityMode == VisibilityCustom.
	ScopeResolver ScopeResolver

	// Roles is the flat list of IAM role names assigned to this user.
	// Used by RBAC evaluation; not used for org scope resolution.
	Roles []string

	// IsPlatformAdmin is true when the viewer holds role:platform-admin.
	// Platform admins bypass org scope — they see all tenants and all orgs.
	IsPlatformAdmin bool

	// IsTenantAdmin is true when the viewer holds role:tenant.admin.
	// Tenant admins default to VisibilityEntireTenant within their tenant.
	IsTenantAdmin bool
}

// OrgMembership describes a user's membership in one organization node.
type OrgMembership struct {
	OrganizationID uuid.UUID
	Role           string // org-level role: manager, member, viewer, …
	IsPrimary      bool
}

// EffectiveOrganizationID returns the organization the viewer is currently
// operating within. Falls back to PrimaryOrganizationID if ActiveOrganizationID
// is not set.
func (v ViewerContext) EffectiveOrganizationID() uuid.UUID {
	if v.ActiveOrganizationID != uuid.Nil {
		return v.ActiveOrganizationID
	}
	return v.PrimaryOrganizationID
}

// HasOrgMembership reports whether the viewer is a member of the given org.
func (v ViewerContext) HasOrgMembership(orgID uuid.UUID) bool {
	for _, m := range v.OrganizationAssignments {
		if m.OrganizationID == orgID {
			return true
		}
	}
	return false
}

// ScopeResolver is implemented by application code that needs custom org
// visibility logic. Invoked by OrganizationService.ResolveScope when
// ViewerContext.VisibilityMode == VisibilityCustom.
type ScopeResolver interface {
	// Resolve returns the org IDs visible to the viewer.
	// Returning nil means no org filter (entire tenant visible).
	Resolve(ctx context.Context, viewer ViewerContext) ([]uuid.UUID, error)
}

type viewerKey struct{}

// WithViewer attaches a ViewerContext to ctx. Called by auth middleware after
// session validation and org assignment loading.
func WithViewer(ctx context.Context, v ViewerContext) context.Context {
	return context.WithValue(ctx, viewerKey{}, v)
}

// ViewerFromContext retrieves the ViewerContext from ctx.
// Returns a zero ViewerContext (no user) when absent — callers must check
// UserID != uuid.Nil before trusting the result.
func ViewerFromContext(ctx context.Context) (ViewerContext, bool) {
	v, ok := ctx.Value(viewerKey{}).(ViewerContext)
	return v, ok
}

// MustViewerFromContext retrieves the ViewerContext from ctx and panics if
// absent. Use only in handler/service code after the auth middleware has run.
func MustViewerFromContext(ctx context.Context) ViewerContext {
	v, ok := ViewerFromContext(ctx)
	if !ok {
		panic("organization: ViewerContext missing from context — auth middleware must run before service calls")
	}
	return v
}
