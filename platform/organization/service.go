package organization

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// OrganizationService is the domain service for organization lifecycle, tree
// operations, path computation, and scope resolution.
//
// # Canonical pipeline
//
//	Request → Authentication → ViewerContext → ResolveScope() →
//	append org filter → Repository.Query() → Tenant RLS → Database
//
// Repositories are organization-agnostic. They receive typed filter predicates
// and execute queries inside the active tenant RLS context. Both isolation
// stages (tenant RLS + org scope) fire independently.
//
// # ResolveScope
//
// ResolveScope is the single entry point for org visibility resolution.
// It converts a ViewerContext into a []uuid.UUID of allowed org IDs.
// Application services append those IDs as an IN predicate before calling
// any repository method. nil return means VisibilityEntireTenant — caller
// omits the org predicate entirely (tenant RLS still fires).
type OrganizationService interface {
	// ── Tree operations ──────────────────────────────────────────────────────

	// Create adds a new org node. input.ParentID == uuid.Nil → root node.
	// Calls ComputePath internally to set path and depth.
	Create(ctx context.Context, input CreateInput) (*OrganizationDTO, error)

	// GetByID returns an org node by UUID.
	GetByID(ctx context.Context, id uuid.UUID) (*OrganizationDTO, error)

	// GetByCode returns an org node by its immutable code within a tenant.
	GetByCode(ctx context.Context, tenantID uuid.UUID, code string) (*OrganizationDTO, error)

	// Move re-parents id under newParentID, recomputing path+depth for the
	// entire subtree atomically. newParentID == uuid.Nil promotes id to root.
	// Calls ValidateMove before executing — returns error on cycle.
	Move(ctx context.Context, id, newParentID uuid.UUID) error

	// ValidateMove checks whether moving id under newParentID would create
	// a cycle or violate any tree invariant. Does not modify any data.
	// Call before Move if you need early validation without executing.
	ValidateMove(ctx context.Context, id, newParentID uuid.UUID) error

	// ComputePath derives the materialized path and depth for a node given
	// its parent. If parentID == uuid.Nil, returns ("/<selfID>/", 0).
	// Used internally by Create and Move; exposed for callers that build
	// org records outside the standard Create flow (e.g. bulk import).
	ComputePath(ctx context.Context, selfID, parentID uuid.UUID) (path string, depth int, err error)

	// Disable marks id and all its descendants as active=false.
	Disable(ctx context.Context, id uuid.UUID) error

	// Enable re-activates id. Does not re-enable descendants.
	Enable(ctx context.Context, id uuid.UUID) error

	// Tree returns all org nodes for a tenant in depth-first order.
	Tree(ctx context.Context, tenantID uuid.UUID) ([]*OrganizationDTO, error)

	// Ancestors returns the path from root to id (root first, target last).
	// No recursive CTE — reads ancestor IDs from the materialized path string.
	Ancestors(ctx context.Context, id uuid.UUID) ([]*OrganizationDTO, error)

	// Descendants returns all nodes below id. maxDepth=0 means unlimited.
	// Uses materialized path prefix query — O(1) regardless of tree depth.
	Descendants(ctx context.Context, id uuid.UUID, maxDepth int) ([]*OrganizationDTO, error)

	// ── Scope resolution ─────────────────────────────────────────────────────

	// ResolveScope converts a ViewerContext into the set of org IDs visible
	// to that viewer according to their VisibilityMode.
	//
	// Returns nil when VisibilityEntireTenant is in effect — caller omits
	// the org filter entirely. Tenant RLS still fires normally.
	//
	// Mode → resolved IDs:
	//   Self           → [viewer.EffectiveOrganizationID()]
	//   Children       → direct children of effective org
	//   Subtree        → effective org + all descendants (path prefix)
	//   Parent         → effective org + all ancestors (path parse)
	//   Assigned       → all IDs in viewer.OrganizationAssignments
	//   Explicit       → viewer.ExplicitOrganizationIDs
	//   EntireTenant   → nil (no org filter)
	//   Custom         → viewer.ScopeResolver.Resolve(ctx, viewer)
	ResolveScope(ctx context.Context, viewer ViewerContext) ([]uuid.UUID, error)

	// ── Type registry ────────────────────────────────────────────────────────

	// RegisterType adds a new org type to the tenant's type registry.
	RegisterType(ctx context.Context, tenantID uuid.UUID, input OrgTypeInput) (*OrgTypeDTO, error)

	// ListTypes returns active org types for a tenant, ordered by sort_order.
	ListTypes(ctx context.Context, tenantID uuid.UUID) ([]*OrgTypeDTO, error)

	// DeactivateType marks a type as inactive. Does not modify org nodes that
	// carry the type — type is stored as a denormalized string on org nodes.
	DeactivateType(ctx context.Context, id uuid.UUID) error

	// ── Assignments ──────────────────────────────────────────────────────────

	// Assign adds userID to an org with the given role. If input.SetPrimary
	// is true, any existing primary assignment for the user is cleared first.
	Assign(ctx context.Context, input AssignInput) (*AssignmentDTO, error)

	// Unassign removes a user from an org. If the removed assignment was the
	// primary, no new primary is auto-selected — caller must call SetPrimary.
	Unassign(ctx context.Context, tenantID, userID, orgID uuid.UUID) error

	// SetPrimary marks orgID as the user's primary org within the tenant.
	// Clears any existing primary for that user first.
	SetPrimary(ctx context.Context, tenantID, userID, orgID uuid.UUID) error

	// ListAssignments returns all org memberships for a user within a tenant.
	ListAssignments(ctx context.Context, tenantID, userID uuid.UUID) ([]*AssignmentDTO, error)

	// LoadViewer loads the full ViewerContext for a user. Called once per
	// session by auth middleware after session validation. Loads org
	// assignments, resolves roles, sets IsTenantAdmin and IsPlatformAdmin.
	// The caller is responsible for setting ActiveOrganizationID from session
	// state (if the user has an active org selection persisted in session).
	LoadViewer(ctx context.Context, tenantID, userID uuid.UUID) (ViewerContext, error)
}

// ── Input / DTO types ─────────────────────────────────────────────────────────

// CreateInput holds the fields needed to create a new org node.
type CreateInput struct {
	TenantID    uuid.UUID
	Name        string
	Code        string
	Type        string // references platform_org_type.name; not validated by framework
	Description string
	ParentID    uuid.UUID // uuid.Nil = root node
	Active      bool
}

// OrganizationDTO is the read model returned by tree operation methods.
type OrganizationDTO struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	Name        string
	Code        string
	Type        string
	Path        string
	Depth       int
	ParentID    uuid.UUID // uuid.Nil for root nodes
	Active      bool
	Description string
}

// OrgTypeInput holds the fields needed to register a new org type.
type OrgTypeInput struct {
	Name        string
	Label       string
	Description string
	SortOrder   int
}

// OrgTypeDTO is the read model for a registered org type.
type OrgTypeDTO struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	Name        string
	Label       string
	Description string
	SortOrder   int
	Active      bool
}

// AssignInput holds the fields needed to add a user to an org.
type AssignInput struct {
	TenantID       uuid.UUID
	UserID         uuid.UUID
	OrganizationID uuid.UUID
	Role           string // org-level role: manager, member, viewer, …
	SetPrimary     bool
}

// AssignmentDTO is the read model for a single org membership record.
type AssignmentDTO struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	UserID         uuid.UUID
	OrganizationID uuid.UUID
	Role           string
	IsPrimary      bool
}

// ── Stub implementation ───────────────────────────────────────────────────────

type stubOrganizationService struct{}

// NewOrganizationService returns a stub OrganizationService.
// TODO: inject EntityRepository[platform_organization] and related repos.
func NewOrganizationService() OrganizationService { return &stubOrganizationService{} }

func (s *stubOrganizationService) Create(_ context.Context, _ CreateInput) (*OrganizationDTO, error) {
	return nil, fmt.Errorf("not implemented: OrganizationService.Create")
}

func (s *stubOrganizationService) GetByID(_ context.Context, _ uuid.UUID) (*OrganizationDTO, error) {
	return nil, fmt.Errorf("not implemented: OrganizationService.GetByID")
}

func (s *stubOrganizationService) GetByCode(_ context.Context, _ uuid.UUID, _ string) (*OrganizationDTO, error) {
	return nil, fmt.Errorf("not implemented: OrganizationService.GetByCode")
}

func (s *stubOrganizationService) Move(_ context.Context, _, _ uuid.UUID) error {
	return fmt.Errorf("not implemented: OrganizationService.Move")
}

func (s *stubOrganizationService) ValidateMove(_ context.Context, _, _ uuid.UUID) error {
	return fmt.Errorf("not implemented: OrganizationService.ValidateMove")
}

func (s *stubOrganizationService) ComputePath(_ context.Context, _, _ uuid.UUID) (string, int, error) {
	return "", 0, fmt.Errorf("not implemented: OrganizationService.ComputePath")
}

func (s *stubOrganizationService) Disable(_ context.Context, _ uuid.UUID) error {
	return fmt.Errorf("not implemented: OrganizationService.Disable")
}

func (s *stubOrganizationService) Enable(_ context.Context, _ uuid.UUID) error {
	return fmt.Errorf("not implemented: OrganizationService.Enable")
}

func (s *stubOrganizationService) Tree(_ context.Context, _ uuid.UUID) ([]*OrganizationDTO, error) {
	return nil, fmt.Errorf("not implemented: OrganizationService.Tree")
}

func (s *stubOrganizationService) Ancestors(_ context.Context, _ uuid.UUID) ([]*OrganizationDTO, error) {
	return nil, fmt.Errorf("not implemented: OrganizationService.Ancestors")
}

func (s *stubOrganizationService) Descendants(_ context.Context, _ uuid.UUID, _ int) ([]*OrganizationDTO, error) {
	return nil, fmt.Errorf("not implemented: OrganizationService.Descendants")
}

func (s *stubOrganizationService) ResolveScope(_ context.Context, _ ViewerContext) ([]uuid.UUID, error) {
	return nil, fmt.Errorf("not implemented: OrganizationService.ResolveScope")
}

func (s *stubOrganizationService) RegisterType(_ context.Context, _ uuid.UUID, _ OrgTypeInput) (*OrgTypeDTO, error) {
	return nil, fmt.Errorf("not implemented: OrganizationService.RegisterType")
}

func (s *stubOrganizationService) ListTypes(_ context.Context, _ uuid.UUID) ([]*OrgTypeDTO, error) {
	return nil, fmt.Errorf("not implemented: OrganizationService.ListTypes")
}

func (s *stubOrganizationService) DeactivateType(_ context.Context, _ uuid.UUID) error {
	return fmt.Errorf("not implemented: OrganizationService.DeactivateType")
}

func (s *stubOrganizationService) Assign(_ context.Context, _ AssignInput) (*AssignmentDTO, error) {
	return nil, fmt.Errorf("not implemented: OrganizationService.Assign")
}

func (s *stubOrganizationService) Unassign(_ context.Context, _, _, _ uuid.UUID) error {
	return fmt.Errorf("not implemented: OrganizationService.Unassign")
}

func (s *stubOrganizationService) SetPrimary(_ context.Context, _, _, _ uuid.UUID) error {
	return fmt.Errorf("not implemented: OrganizationService.SetPrimary")
}

func (s *stubOrganizationService) ListAssignments(_ context.Context, _, _ uuid.UUID) ([]*AssignmentDTO, error) {
	return nil, fmt.Errorf("not implemented: OrganizationService.ListAssignments")
}

func (s *stubOrganizationService) LoadViewer(_ context.Context, _, _ uuid.UUID) (ViewerContext, error) {
	return ViewerContext{}, fmt.Errorf("not implemented: OrganizationService.LoadViewer")
}
