> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Organization Model

## Isolation Model (Canonical)

```
Stage 1 — Tenant Isolation
  Layer:    Database (PostgreSQL RLS)
  Enforces: tenant_id = current_tenant_id() on every query
  Scope:    Cross-tenant data never leaks

Stage 2 — Organization Scope
  Layer:    Application (Go — OrganizationService)
  Enforces: user sees only their authorized org nodes
  Scope:    Within a single tenant's org tree

These stages are orthogonal. RLS never enforces org visibility.
Org scope is never implemented through RLS predicates.
```

Organization tables (`platform_organization`, `platform_org_type`, `platform_org_assignment`) carry standard tenant RLS — `tenant_id = current_tenant_id()` — identical to all other business entity tables. They carry **no** org-visibility predicates. Org visibility is a Stage 2 concern.

---

## Tenant vs Organization

| | Tenant | Organization |
|---|---|---|
| **Represents** | Customer account | Business hierarchy node |
| **Created by** | Awo provisioning | Tenant admin |
| **Multiplicity** | One per account | Many per tenant |
| **Isolation** | PostgreSQL RLS (Stage 1) | Application scope (Stage 2) |
| **RLS** | Yes — all business tables | Yes — tenant_id only; no org predicates |
| **Immutable field** | `slug` | `code` |

---

## Organization Hierarchy

Organizations form an arbitrary-depth tree within a tenant. The framework imposes no fixed shape and no predefined types.

```
Tenant
└── Holding Company        (type=holding,    depth=0)
    ├── Kenya              (type=country,    depth=1)
    │   ├── Nairobi HQ     (type=branch,     depth=2)
    │   │   ├── Retail     (type=team,       depth=3)
    │   │   └── Credit     (type=team,       depth=3)
    │   ├── Westlands      (type=branch,     depth=2)
    │   └── Mombasa        (type=branch,     depth=2)
    ├── Tanzania           (type=country,    depth=1)
    └── Uganda             (type=country,    depth=1)
```

### Node Types

Types are **metadata-driven**. Tenant admins register valid types via `platform_org_type`. The framework never validates against a predefined list and never hardcodes any type name.

Examples of types tenant admins may register:
`company`, `subsidiary`, `holding`, `region`, `country`, `territory`, `division`, `department`, `branch`, `warehouse`, `cost_centre`, `store`, `team`.

The `type` field on `platform_organization` is a denormalized varchar — no FK constraint to `platform_org_type`. This allows type names to be deactivated without cascading effects on existing nodes.

### Materialized Path

`path` stores the full ancestor chain: `/root-id/parent-id/self-id/`

Enables O(1) queries without recursive CTEs:

```sql
-- Descendants of Nairobi HQ:
SELECT * FROM platform_organization
WHERE path LIKE '/holding-id/kenya-id/nairobi-id/%'
  AND tenant_id = current_tenant_id();  -- tenant RLS + explicit param

-- Ancestors from path string (no CTE):
SELECT id FROM platform_organization
WHERE id = ANY(
    string_to_array(trim(both '/' from path_value), '/')::uuid[]
)
AND tenant_id = current_tenant_id();
```

`ComputePath` and `ValidateMove` in `OrganizationService` maintain path integrity and prevent cycles.

---

## Viewer Context

`ViewerContext` is the single source of truth about the acting user's organizational context. Auth middleware populates it by calling `OrganizationService.LoadViewer()` after session validation.

```go
type ViewerContext struct {
    TenantID                uuid.UUID
    UserID                  uuid.UUID
    ActiveOrganizationID    uuid.UUID    // current session org
    PrimaryOrganizationID   uuid.UUID    // home org (is_primary=true)
    OrganizationAssignments []OrgMembership
    VisibilityMode          VisibilityMode
    ExplicitOrganizationIDs []uuid.UUID  // VisibilityExplicit only
    ScopeResolver           ScopeResolver // VisibilityCustom only
    Roles                   []string
    IsTenantAdmin           bool
    IsPlatformAdmin         bool
}
```

### Active Organization

Users with multiple org assignments can switch their **active organization** within a session. `EffectiveOrganizationID()` returns `ActiveOrganizationID` if set, falling back to `PrimaryOrganizationID`. Middleware validates that the requested org is in `OrganizationAssignments` before allowing the switch.

---

## User Organization Assignments

Users may belong to multiple organizations simultaneously.

```
User: Jane (Regional Finance Manager)
├── Assignment: Kenya            role=manager   is_primary=true
├── Assignment: Nairobi HQ       role=member
└── Assignment: Mombasa          role=viewer
```

Exactly one assignment per user per tenant may have `is_primary=true` (enforced by a partial unique index). `VisibilityAssigned` resolves to all IDs in the user's `OrganizationAssignments` list.

### IAM Roles vs Organization Roles

| Type | Table | Examples | Purpose |
|---|---|---|---|
| IAM role | `iam_user_role` | `tenant.admin`, `finance.viewer` | Platform-wide capabilities |
| Org role | `platform_org_assignment.role` | `manager`, `member`, `viewer` | Position within an org node |

Independent. A user may be `tenant.user` (IAM) and `manager` in their primary org simultaneously.

---

## Visibility Modes

`OrganizationService.ResolveScope(ctx, viewer)` evaluates `viewer.VisibilityMode` and returns the allowed org ID set.

| Mode | What ResolveScope Returns | Typical User |
|---|---|---|
| `VisibilitySelf` | `[viewer.EffectiveOrgID]` | Branch cashier |
| `VisibilityChildren` | Direct children of effective org | Parent org reviewing sub-units |
| `VisibilitySubtree` | Effective org + all descendants | Regional manager |
| `VisibilityParent` | Effective org + all ancestors | Breadcrumb / escalation |
| `VisibilityAssigned` | All IDs in `OrganizationAssignments` | HR BP across departments |
| `VisibilityExplicit` | `viewer.ExplicitOrganizationIDs` | Internal auditor |
| `VisibilityEntireTenant` | `nil` (no org filter) | tenant.admin, HQ Finance |
| `VisibilityCustom` | `viewer.ScopeResolver.Resolve(…)` | Matrix orgs, project teams |

`nil` → caller omits org filter. Tenant RLS still fires — data stays tenant-scoped.

---

## Repository Contract

Repositories are **organization-agnostic**. They receive typed filter predicates. They never compute visibility.

```
Viewer
  → OrganizationService.ResolveScope()    (Stage 2)
  → []uuid.UUID
  → Application service builds filter
  → Repository.Query(ctx, filter)
  → Tenant RLS fires                      (Stage 1)
  → Database
```

Repositories must never:
- Call `OrganizationService.ResolveScope`
- Read `ViewerContext` from the context
- Inspect user roles or assignments
- Compute which organizations are accessible

---

## HQ / Cross-Org Access

Users who must access multiple or all organizations:

```
CEO / HQ Finance Manager
  IsTenantAdmin = true
  VisibilityMode = VisibilityEntireTenant
  ResolveScope() → nil
  Repository gets: WHERE tenant_id = $1  (RLS only)

Kenya Regional Manager
  ActiveOrg = Kenya
  VisibilityMode = VisibilitySubtree
  ResolveScope() → [Kenya, NairobiHQ, Westlands, Mombasa, …]
  Repository gets: WHERE tenant_id = $1 AND org_id = ANY($2)

Internal Auditor
  VisibilityMode = VisibilityExplicit
  ExplicitOrganizationIDs = [OrgA, OrgC]
  ResolveScope() → [OrgA, OrgC]
  Repository gets: WHERE tenant_id = $1 AND org_id = ANY($2)

HR Business Partner (matrix)
  VisibilityMode = VisibilityAssigned
  Assignments: [Dept_Engineering, Dept_Finance, Dept_Sales]
  ResolveScope() → [Dept_Engineering, Dept_Finance, Dept_Sales]
```

No special framework paths. All cases resolved by `VisibilityMode` + `ResolveScope`.

---

## OrganizationService API

```go
// Tree operations
Create(ctx, CreateInput) (*OrganizationDTO, error)
GetByID(ctx, id) (*OrganizationDTO, error)
GetByCode(ctx, tenantID, code) (*OrganizationDTO, error)
Move(ctx, id, newParentID) error
ValidateMove(ctx, id, newParentID) error   // dry-run cycle check
ComputePath(ctx, selfID, parentID) (path, depth, error)
Disable(ctx, id) error
Enable(ctx, id) error
Tree(ctx, tenantID) ([]*OrganizationDTO, error)
Ancestors(ctx, id) ([]*OrganizationDTO, error)
Descendants(ctx, id, maxDepth) ([]*OrganizationDTO, error)

// Scope resolution
ResolveScope(ctx, ViewerContext) ([]uuid.UUID, error)

// Type registry
RegisterType(ctx, tenantID, OrgTypeInput) (*OrgTypeDTO, error)
ListTypes(ctx, tenantID) ([]*OrgTypeDTO, error)
DeactivateType(ctx, id) error

// Assignments
Assign(ctx, AssignInput) (*AssignmentDTO, error)
Unassign(ctx, tenantID, userID, orgID) error
SetPrimary(ctx, tenantID, userID, orgID) error
ListAssignments(ctx, tenantID, userID) ([]*AssignmentDTO, error)
LoadViewer(ctx, tenantID, userID) (ViewerContext, error)
```

---

## ERP Module Implementation Pattern

Every ERP module (Finance, CRM, Inventory, HR, POS, Manufacturing, etc.) that stores org-scoped data follows this pattern:

```go
// 1. Entity definition declares org_id field.
{Name: "org_id", Type: def.FieldTypeLink, LinkTarget: "platform_organization"}

// 2. Service resolves scope before every list/query operation.
func (s *InvoiceService) List(ctx context.Context, opts ListOptions) ([]*Invoice, error) {
    viewer := organization.MustViewerFromContext(ctx)
    orgIDs, err := s.orgSvc.ResolveScope(ctx, viewer)
    if err != nil {
        return nil, fmt.Errorf("invoice.List: resolve org scope: %w", err)
    }
    f := buildFilter(opts)
    if len(orgIDs) > 0 {
        f = filter.And(f, filter.In("org_id", toAny(orgIDs)...))
    }
    records, _, err := s.repo.Query(ctx, f)
    return records, err
}

// 3. Repository.Query receives the filter — knows nothing about org scope.
```

This pattern must be applied consistently across all ERP modules. The framework provides the primitives; the module is responsible for calling them.
