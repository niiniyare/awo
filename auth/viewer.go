package auth

import (
	"context"

	"awo.so/awo/def"
	"github.com/google/uuid"
)

// ViewerContext is the authorization surface for every operation within a
// request context. The middleware pipeline constructs it from the validated
// Session and injects it into context.Context before any handler, hook, or
// repository operation executes.
//
// All methods MUST be goroutine-safe. The implementation MUST be immutable
// after construction (ADR-002).
type ViewerContext interface {
	// TenantID returns the UUID of the tenant this viewer operates within.
	// Never uuid.Nil for authenticated requests.
	TenantID() uuid.UUID

	// UserID returns the UUID of the authenticated user.
	// Returns uuid.Nil for service account sessions.
	UserID() uuid.UUID

	// ServiceAccountID returns the UUID of the service account.
	// Returns uuid.Nil for human user sessions.
	ServiceAccountID() uuid.UUID

	// Roles returns the complete set of role names assigned to this viewer.
	// Role names use the format "role:{domain}.{name}".
	// Example: []string{"role:tenant.admin", "role:finance.accounts_payable"}
	Roles() []string

	// HasRole returns true if the viewer holds the named role.
	// Comparison is case-sensitive. Implementations MUST be O(1).
	HasRole(role string) bool

	// IsPlatformAdmin returns true if the viewer holds "role:platform-admin".
	// Platform admins bypass all PolicyEvaluator checks unconditionally.
	IsPlatformAdmin() bool

	// Actor constructs a [def.Actor] from this ViewerContext for use in hooks
	// and action handlers. The returned Actor contains a defensive copy of Roles.
	// Implementations MUST NOT return nil for authenticated requests (ADR-002).
	Actor() *def.Actor
}

// viewerKey is an unexported context key type for ViewerContext, preventing
// collisions with keys from other packages.
type viewerKey struct{}

// WithViewer embeds v into ctx and returns the derived context. Called by the
// session middleware after validating the request session. All downstream
// handlers, hooks, and repository operations receive this context.
func WithViewer(ctx context.Context, v ViewerContext) context.Context {
	return context.WithValue(ctx, viewerKey{}, v)
}

// ViewerFromContext extracts the ViewerContext from ctx.
//
// PANICS if ViewerContext is absent. This is intentional (ADR-002): absence
// means the middleware guarantee was violated. A panic at development time is
// preferable to a silent authorization bypass at production time.
//
// ViewerContext is absent only when:
//   - A handler bypasses the middleware pipeline (programming error).
//   - A background goroutine uses context.Background() instead of the request
//     context (programming error — use [WithViewer] with [NewSystemViewer]).
//   - A test omits ViewerContext injection (tests must call [WithViewer]).
func ViewerFromContext(ctx context.Context) ViewerContext {
	v, ok := ctx.Value(viewerKey{}).(ViewerContext)
	if !ok {
		panic("auth: ViewerContext missing from context — middleware not applied or wrong context passed")
	}
	return v
}

// DefaultViewer is the concrete ViewerContext constructed from a Session.
// It is constructed once per request by the session middleware and is
// immutable for the lifetime of the request.
//
// Module code must not construct DefaultViewer directly; that is the
// framework middleware's responsibility.
type DefaultViewer struct {
	tenantID         uuid.UUID
	userID           uuid.UUID
	serviceAccountID uuid.UUID
	roles            []string
	roleSet          map[string]bool // O(1) HasRole lookup
}

// NewViewer constructs a DefaultViewer from the validated Session. The roles
// slice is copied defensively; the roleSet is pre-built for O(1) HasRole.
func NewViewer(s *Session) *DefaultViewer {
	roleSet := make(map[string]bool, len(s.Roles))
	for _, r := range s.Roles {
		roleSet[r] = true
	}
	return &DefaultViewer{
		tenantID:         s.TenantID,
		userID:           s.UserID,
		serviceAccountID: s.ServiceAccountID,
		roles:            append([]string(nil), s.Roles...),
		roleSet:          roleSet,
	}
}

// Ensure DefaultViewer implements ViewerContext at compile time.
var _ ViewerContext = (*DefaultViewer)(nil)

func (v *DefaultViewer) TenantID() uuid.UUID         { return v.tenantID }
func (v *DefaultViewer) UserID() uuid.UUID           { return v.userID }
func (v *DefaultViewer) ServiceAccountID() uuid.UUID { return v.serviceAccountID }
func (v *DefaultViewer) Roles() []string             { return v.roles }
func (v *DefaultViewer) HasRole(role string) bool    { return v.roleSet[role] }
func (v *DefaultViewer) IsPlatformAdmin() bool       { return v.roleSet["role:platform-admin"] }
func (v *DefaultViewer) Actor() *def.Actor {
	return &def.Actor{
		UserID:           v.userID,
		ServiceAccountID: v.serviceAccountID,
		TenantID:         v.tenantID,
		Roles:            append([]string(nil), v.roles...),
	}
}

// SystemViewer represents a platform-level background process (Temporal
// activity, scheduled job, migration script). It carries "role:platform-admin"
// which causes it to bypass all PolicyEvaluator checks unconditionally.
//
// # Security warning
//
// SystemViewer MUST NEVER be used in request-handling code paths. It exists
// solely for background goroutines and Temporal activities that operate outside
// any HTTP request context (e.g. scheduled jobs, data migrations, provisioning
// workflows).
//
// Using SystemViewer in a request handler to bypass authorization is a
// CRITICAL security violation. It silently grants platform-admin access to
// every entity and action, including cross-tenant reads and writes.
//
// Legitimate use:
//
//	ctx = auth.WithViewer(ctx, auth.NewSystemViewer(tenantID)) // in Temporal activity
//
// NEVER:
//
//	ctx = auth.WithViewer(c.UserContext(), auth.NewSystemViewer(tenantID)) // in HTTP handler
type SystemViewer struct {
	tenantID uuid.UUID
}

// NewSystemViewer constructs a SystemViewer for background operations within
// tenantID. Pass uuid.Nil for cross-tenant platform operations.
func NewSystemViewer(tenantID uuid.UUID) *SystemViewer {
	return &SystemViewer{tenantID: tenantID}
}

// Ensure SystemViewer implements ViewerContext at compile time.
var _ ViewerContext = (*SystemViewer)(nil)

func (v *SystemViewer) TenantID() uuid.UUID         { return v.tenantID }
func (v *SystemViewer) UserID() uuid.UUID           { return uuid.Nil }
func (v *SystemViewer) ServiceAccountID() uuid.UUID { return uuid.Nil }
func (v *SystemViewer) Roles() []string             { return []string{"role:platform-admin"} }
func (v *SystemViewer) HasRole(role string) bool    { return role == "role:platform-admin" }
func (v *SystemViewer) IsPlatformAdmin() bool       { return true }
func (v *SystemViewer) Actor() *def.Actor {
	return &def.Actor{
		UserID:           uuid.Nil,
		ServiceAccountID: uuid.Nil,
		TenantID:         v.tenantID,
		Roles:            []string{"role:platform-admin"},
	}
}
