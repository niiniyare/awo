> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Authorization Model

## Canonical Two-Stage Isolation Model

Awo enforces two independent isolation boundaries on every request. They are orthogonal — each fires independently, neither depends on the other.

```
┌─────────────────────────────────────────────────────────────┐
│  Stage 1 — Tenant Isolation                                 │
│  Layer: Database (PostgreSQL RLS)                           │
│  Enforced by: Framework                                     │
│  Guarantee: cross-tenant data never leaks                   │
├─────────────────────────────────────────────────────────────┤
│  Stage 2 — Organization Scope                               │
│  Layer: Application (Go)                                    │
│  Enforced by: OrganizationService.ResolveScope()            │
│  Guarantee: user sees only their authorized org nodes       │
└─────────────────────────────────────────────────────────────┘
```

**Critical rule:** RLS is NEVER responsible for organization visibility. Organization scope is NEVER implemented through RLS predicates.

---

## Stage 1 — Tenant Isolation (Database)

**Mechanism:** PostgreSQL Row-Level Security
**Who enforces it:** Framework (`set_tenant_context()` stored procedure + RLS policies)
**When it fires:** On every single database query, automatically

```sql
-- Every tenant-scoped table has this policy:
CREATE POLICY tenant_isolation ON invoice
    USING (tenant_id = current_tenant_id());

-- current_tenant_id() reads app.current_tenant_id — a transaction-local variable
-- set by set_tenant_context() at the start of each connection checkout.
```

This is a hard kernel-level guarantee. No application bug can bypass it. A misconfigured repository that forgets to pass tenant_id still gets correctly filtered by RLS.

**What it does NOT do:** RLS does not know about organizational hierarchy. It does not enforce which organizations a user may access. That is entirely Stage 2.

---

## Stage 2 — Organization Scope (Application)

**Mechanism:** `OrganizationService.ResolveScope(viewer)` → `[]uuid.UUID`
**Who enforces it:** Application services (calling code)
**When it fires:** Explicitly, before every org-scoped repository query

```
ViewerContext
    ↓
OrganizationService.ResolveScope(viewer)
    ↓
[]uuid.UUID  (allowed org IDs)  ← nil means VisibilityEntireTenant
    ↓
Application service appends:
    filter.In("org_id", orgIDs...)
    ↓
Repository.Query(ctx, filter)
    ↓
Tenant RLS fires automatically (Stage 1)
    ↓
Database
```

**Repositories are organization-agnostic.** They receive typed filter predicates. They never inspect the user or compute visibility.

**Wrong pattern** (repositories must not do this):
```go
// WRONG — repository computing org visibility
func (r *invoiceRepo) List(ctx context.Context) ([]Invoice, error) {
    viewer := organization.MustViewerFromContext(ctx) // ← WRONG
    orgIDs, _ := r.orgSvc.ResolveScope(ctx, viewer)  // ← WRONG
    // ...
}
```

**Correct pattern** (application service computes scope, passes to repo):
```go
// CORRECT — application service resolves scope, repo receives filter
func (s *InvoiceService) List(ctx context.Context, opts ListOptions) ([]*Invoice, error) {
    viewer := organization.MustViewerFromContext(ctx)
    orgIDs, err := s.orgSvc.ResolveScope(ctx, viewer)
    if err != nil {
        return nil, err
    }
    f := buildBaseFilter(opts)
    if len(orgIDs) > 0 {  // nil = EntireTenant, omit org filter
        f = filter.And(f, filter.In("org_id", toAny(orgIDs)...))
    }
    records, _, err := s.repo.Query(ctx, f)
    return records, err
}
```

---

## Full Request Pipeline

```
┌──────────────────────────────────────────────────────────┐
│  HTTP Request                                            │
│  GET /api/v1/entities/invoice                            │
└──────────────────────┬───────────────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────────┐
│  Authentication Middleware                               │
│  1. Resolve tenant (X-Tenant-ID header / subdomain)      │
│  2. set_tenant_context(tenantID) → sets RLS variable     │
│  3. Validate session token (Redis)                       │
│  4. OrganizationService.LoadViewer(tenantID, userID)     │
│     → ViewerContext{UserID, Roles, Assignments, Mode, …} │
└──────────────────────┬───────────────────────────────────┘
                       │  ViewerContext in context.Context
                       ▼
┌──────────────────────────────────────────────────────────┐
│  Application Service (e.g. InvoiceService.List)          │
│  1. MustViewerFromContext(ctx)                           │
│  2. OrganizationService.ResolveScope(ctx, viewer)        │
│     → []uuid.UUID  (Stage 2 — org scope)                 │
│  3. Build filter: base filter + org_id IN (…)            │
└──────────────────────┬───────────────────────────────────┘
                       │  filter.Filter (typed predicates)
                       ▼
┌──────────────────────────────────────────────────────────┐
│  Repository.Query(ctx, filter)                           │
│  Generates: SELECT … WHERE … AND org_id = ANY($n)        │
└──────────────────────┬───────────────────────────────────┘
                       │  SQL
                       ▼
┌──────────────────────────────────────────────────────────┐
│  PostgreSQL                                              │
│  RLS fires: AND tenant_id = current_tenant_id()  ← Stage 1│
│  Final query executes with both predicates active        │
└──────────────────────────────────────────────────────────┘
```

Both stages fire. Stage 1 (tenant RLS) is automatic and unconditional. Stage 2 (org scope) is the application service's responsibility.

---

## Visibility Modes

`VisibilityMode` is evaluated by `ResolveScope` to determine which org IDs are returned.

| Mode | Resolved IDs | Typical User |
|---|---|---|
| `VisibilitySelf` | `[viewer.EffectiveOrgID]` | Branch cashier, data-entry clerk |
| `VisibilityChildren` | Direct children of effective org | Parent org viewing sub-units |
| `VisibilitySubtree` | Effective org + all descendants | Regional manager |
| `VisibilityParent` | Effective org + all ancestors | Breadcrumb navigation; escalation |
| `VisibilityAssigned` | All IDs in `OrganizationAssignments` | HR BP spanning multiple depts |
| `VisibilityExplicit` | `viewer.ExplicitOrganizationIDs` | Internal auditor (non-contiguous) |
| `VisibilityEntireTenant` | **nil** (no org filter) | tenant.admin, HQ Finance Manager |
| `VisibilityCustom` | `viewer.ScopeResolver.Resolve(…)` | Matrix orgs, project teams |

`nil` return → caller omits the org filter entirely. Tenant RLS still fires — data is still tenant-scoped.

### HQ / Cross-Org Users

Users who must operate across multiple or all organizations:

```
CEO / HQ Finance Manager
  VisibilityMode = VisibilityEntireTenant
  ResolveScope() → nil
  Query: WHERE tenant_id = $1  (RLS only, no org filter)
  Result: all invoices in tenant

Kenya Regional Manager
  VisibilityMode = VisibilitySubtree
  ActiveOrganizationID = Kenya node
  ResolveScope() → [Kenya, Nairobi, Westlands, Mombasa, …]
  Query: WHERE tenant_id = $1 AND org_id = ANY($2)

Internal Auditor
  VisibilityMode = VisibilityExplicit
  ExplicitOrganizationIDs = [OrgA, OrgC, OrgF]
  ResolveScope() → [OrgA, OrgC, OrgF]
  Query: WHERE tenant_id = $1 AND org_id = ANY($2)
```

---

## Post-Scope Authorization

After org scope is applied, two additional authorization checks run:

### RBAC (Casbin)

```
enforcer.Enforce(userID, tenantID, entityType, action) → bool
```

Operation-level gate: can this user `read` the `invoice` entity type? Independent of org scope — it checks capabilities, not visibility.

### Business Policies (EntityDefinition.Policy)

Row-level predicate injected into every query for the entity:

```go
Policy: def.PolicyFunc(func(ctx context.Context) def.Filter {
    viewer, _ := organization.ViewerFromContext(ctx)
    return filter.Eq("assigned_to", viewer.UserID.String())
})
```

Declared once on the entity definition, enforced everywhere. Runs after tenant isolation and org scope — operates on the already-filtered dataset.

---

## Summary: What Each Layer Is Responsible For

| Question | Answer | Enforcement |
|---|---|---|
| Is this request for the correct tenant? | `tenant_id = current_tenant_id()` | PostgreSQL RLS (Stage 1) |
| Which org nodes can this user access? | `ResolveScope()` → org ID set | OrganizationService (Stage 2) |
| Can this user perform this action on this entity type? | Casbin RBAC | Framework + application |
| Can this user see this specific row? | `EntityDefinition.Policy` predicate | Application |
