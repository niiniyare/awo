> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Awo Authorization Architecture Review
## Pre-v1.0 Final Design Assessment

**Date:** 2026-07-20
**Status:** Design Review — No Code Changes
**Scope:** Complete framework authorization architecture, all packages

---

## Table of Contents

1. [Current Authorization Architecture](#1-current-authorization-architecture)
2. [Strengths](#2-strengths)
3. [Weaknesses](#3-weaknesses)
4. [Architectural Risks](#4-architectural-risks)
5. [Future Scalability Analysis](#5-future-scalability-analysis)
6. [Comparative Analysis](#6-comparative-analysis)
7. [Proposed Canonical Authorization Architecture](#7-proposed-canonical-authorization-architecture)
8. [Permission Declarations After Redesign](#8-permission-declarations-after-redesign)
9. [Tenant Admin Configuration](#9-tenant-admin-configuration)
10. [Request Authorization Flow](#10-request-authorization-flow)
11. [Required Platform Additions](#11-required-platform-additions)
12. [Phased Migration Strategy](#12-phased-migration-strategy)

---

## 1. Current Authorization Architecture

### Overview

Authorization in Awo today is distributed across five distinct mechanisms, each addressing a different layer of the permission model. These mechanisms were designed independently and are connected primarily through Casbin as the shared evaluation engine.

### 1.1 Compile-Time Permission Declaration (`def.PermissionSet`)

Every `EntityDefinition` carries a `PermissionSet` that declares CRUD permission subjects:

```
PermissionSet {
    Create  []string               // ["role:finance.accounts_payable", "role:tenant.admin"]
    Read    []string
    Write   []string
    Delete  []string
    Actions map[string][]string    // {"submit": ["role:finance.accounts_payable"]}
    Policy  PolicyFunc             // row-level filter
}
```

Subject format is a freeform string with two conventions:
- `"role:{name}"` — role-based grant
- `"user:{uuid}"` — per-user grant

These subjects are **hardcoded in Go source** inside the business module (e.g. `internal/core/finance/`). The compiler transforms them into flat Casbin policy tuples: `(subject, object, action)` where object = `QualifiedName`.

### 1.2 Action-Level Permission (`def.ActionDef.Permission`)

Each custom action carries a single permission string:

```go
ActionDef {
    Name:       "submit",
    Permission: "role:finance.accounts_payable",
    ...
}
```

If empty, the action inherits the entity's `Write` permission. There is no mechanism for:
- Multiple permitted subjects on a single action
- Deny overrides
- Context-sensitive permission (e.g. "only when record.status == Draft")
- Organization-scoped permission
- Approval-gated permission

### 1.3 Row-Level Filtering (`def.PolicyFunc`)

`PolicyFunc` is a Go function embedded in the `PermissionSet` that returns a `Filter` predicate injected into every query:

```go
Policy: def.PolicyFunc(func(ctx context.Context) def.Filter {
    actor := session.ActorFromContext(ctx)
    return filter.Eq("assigned_to", actor.UserID)
})
```

Characteristics:
- Runs at request time, not compile time
- Can inspect actor, feature flags, tenant settings
- Multiple policies are AND-composed
- Cannot be stored in database, configured by tenants, or versioned
- Not exported in any introspection API

### 1.4 Database-Level RLS (Row Level Security)

PostgreSQL enforces `tenant_id = current_tenant_id()` via RLS policies on every tenant-scoped table. This is the only truly tamper-proof authorization layer — it cannot be bypassed by application code.

This layer handles **tenant isolation** but not **user-level** or **role-level** row filtering.

### 1.5 Platform Admin Bypass

The `Actor.IsPlatformAdmin` flag bypasses Casbin entirely. This is checked in the IAM middleware before any policy evaluation. No audit trail of what a platform admin accessed is currently enforced at the authorization layer.

### 1.6 Casbin Integration

The compiler emits `[]CasbinPolicy` tuples from `PermissionSet` declarations. At startup, these are loaded into a Casbin enforcer. The IAM middleware checks `enforcer.Enforce(subject, domain, object, action)` before each route handler runs.

Role hierarchy is expressed via Casbin `g` (grouping) assertions, allowing `role:tenant.admin` to inherit permissions from `role:tenant.user`.

### 1.7 Where Authorization Currently Touches the Framework

| Location | Mechanism | What it controls |
|---|---|---|
| `def.PermissionSet` | RBAC subjects | Who can CRUD a resource |
| `def.ActionDef.Permission` | Single role string | Who can invoke a custom action |
| `def.PolicyFunc` | Go closure | Which rows are visible per request |
| `compiler/schema.go:emitPolicies()` | Casbin tuple generation | Compile-time policy materialization |
| `runtime/pipeline.go` | Hook pipeline | No auth here — hooks assume permission already checked |
| `runtime/registry.go` | Namespace lookup | Used by IAM middleware to resolve entities from routes |
| `sdui/generator.go` | Schema generation | Does NOT filter by permission today — noted as "caller's responsibility" |
| `introspect/introspect.go` | Schema exposure | No auth gate on introspection endpoint |
| Middleware pipeline (step 6) | Session validation | Populates Actor from Redis session |
| Middleware pipeline (step 7) | Rate limiting | Per-tenant + per-user, not per-permission |

### 1.8 What the Current Model Completely Lacks

- **ViewerContext** — no compiled struct carrying resolved permissions for the current request
- **Organization-scoped permissions** — no mechanism to say "user X can approve invoices in branch Y only"
- **Field-level permissions** — Sensitive flag hides fields but cannot grant read-to-some, hide-from-others
- **Temporal workflow authorization** — workflow trigger fires if entity action succeeds; no independent auth on workflow operations
- **Permission metadata** — no introspectable record of what permissions exist in the system
- **Deny rules** — no mechanism to explicitly deny a role that would otherwise inherit permission
- **Delegation** — no mechanism for a user to grant a subset of their permissions to another user
- **Time-bound grants** — no mechanism for temporary permission elevation
- **Approval-gated elevation** — no mechanism for "user can request permission X, approved by role Y"
- **Policy simulation** — no way to ask "would actor A be permitted to do B on record C?"
- **Audit integration** — permission decisions are not recorded in the audit log

---

## 2. Strengths

### 2.1 Compile-Time Validation of Permission Structure

The compiler validates that:
- Every link target exists
- Every action has a handler
- Every entity has valid field types

While it does not validate permission subject names (role strings are not validated against registered roles), the compilation phase creates a natural checkpoint. This is the correct foundation to extend.

### 2.2 Single Declaration Site

`PermissionSet` on `EntityDefinition` is the single source of truth for CRUD permissions. There is no duplication between route registration, middleware, and handler. The compiler derives Casbin policies from one declaration. This is architecturally sound.

### 2.3 PolicyFunc Composability

Row-level policies are AND-composed by the runtime, not OR-composed. This is the correct default for security: if you declare multiple policies, all must pass. This prevents accidental permission expansion through policy composition.

### 2.4 RLS as Uncrossable Tenant Boundary

Database-level RLS on every tenant-scoped table means tenant isolation cannot be bypassed by application bugs. Even a completely broken permission evaluation layer cannot expose one tenant's data to another. This is a strong foundational guarantee.

### 2.5 Namespace Architecture

The compiler derives a complete set of namespaces from `QualifiedName`: permission, event, workflow, cache, metric, table. This means all systems speak the same identity language. A Casbin policy object, a Redis cache key, a Temporal workflow ID, and a Prometheus metric all reference the same canonical name.

### 2.6 Zero Boilerplate for Standard CRUD Authorization

Business modules declare permissions once in `PermissionSet`; the framework generates routes, Casbin policies, and (eventually) SDUI permission gates from that declaration. No manual route protection needed.

---

## 3. Weaknesses

### 3.1 Role Names Are Magic Strings in Business Code

The most serious structural weakness: permission subjects like `"role:finance.accounts_payable"` are hardcoded in Go source files inside business modules. This means:

- Role names cannot be validated at compile time (the compiler does not know what roles exist)
- Renaming a role requires a code change and redeploy
- Tenants cannot create their own custom roles that map to framework permissions
- No single source of truth for what roles exist — they emerge from scanning all `PermissionSet` declarations across all modules
- Business modules are coupled to IAM module internals through string literals

### 3.2 ActionDef.Permission Is Structurally Insufficient

A single string field cannot express:
- "role A OR role B can invoke this action" — no OR composition
- "role A can invoke, but only if record.status == 'Draft'" — no contextual guard
- "role A can invoke, but only within their assigned organization" — no scope restriction
- "role A can invoke, but requires approval from role B first" — no gating
- "deny role A even though they inherit this permission" — no deny override

This field will need to be redesigned regardless. Doing it now before v1.0 is the right choice.

### 3.3 PolicyFunc Cannot Be Tenant-Configured

`PolicyFunc` is a Go closure compiled into the binary. Tenant admins cannot:
- Add additional row restrictions
- Modify existing row restrictions
- Configure which users see which records via an admin UI

This is a fundamental ceiling on the configurability of the authorization system for multi-tenant ERP use cases, where row-level visibility rules vary significantly between organizations.

### 3.4 No Permission Metadata in CompiledSchema

`EntitySchema` has no compiled representation of what permissions exist for documentation, introspection, or validation tooling. The `PermissionSet` is compiled into flat Casbin tuples and the original structure is lost. There is no way to ask the framework at runtime: "what roles are required to invoke the submit action on an invoice?"

### 3.5 Casbin Coupling

The framework is structurally coupled to Casbin:
- `CasbinPolicy` struct in the compiler output — a named Casbin concept
- IAM middleware directly uses Casbin enforcer interface
- The policy model `(subject, domain, object, action)` is Casbin's model

Replacing Casbin with OPA, Cedar, Zanzibar-style tuples, or a custom engine would require changes to the compiler output format, the IAM middleware, and all callsites. This is a v1.0 risk.

### 3.6 SDUI Permission Filtering Is Incomplete

`sdui/generator.go` notes that "permission-gated elements are absent from schema" but the current implementation does not implement this. The comment in the code says "Permission check runs at schema-serve time" but the `generate()`, `generateList()`, `generateForm()`, and `generateDetail()` functions do not accept an actor and do not filter by permission. This is a placeholder, not an implementation.

### 3.7 No Authorization in Hook Pipeline

Hooks receive `EntityRecord` and `context.Context` but no authorization context. A `BeforeCreate` hook that needs to check "does the actor have permission to link to this customer?" has no framework-provided mechanism to do so. It must either:
- Access session context directly (coupling to session package)
- Duplicate the actor extraction logic
- Ignore the authorization requirement

### 3.8 Workflow Triggers Are Unconditionally Authorized

A `WorkflowTrigger` fires whenever the matched action succeeds. There is no mechanism to require additional permission to start a workflow, restrict which workflow a given actor can trigger, or audit workflow authorization separately from action authorization.

### 3.9 No Organization Scope

The permission model is: `(actor, resource_type, operation)`. There is no `scope` dimension. A user who can approve invoices for Branch A also implicitly approves invoices for Branch B, unless the business module implements organization filtering manually via `PolicyFunc`. This is duplicated, inconsistent, and not visible to the authorization system.

---

## 4. Architectural Risks

### 4.1 Role Names Baked Into Binary — Cannot Evolve

**Risk level: HIGH**

If a Kenyan ERP deployment needs to rename "accounts_payable" to "ap_clerk" for their HR system, they cannot. If a module needs to add a new granular role, the existing role strings in all existing `PermissionSet` declarations do not automatically include it. The only fix is a code change.

This creates a class of permission-related bugs that are indistinguishable from code bugs.

### 4.2 Casbin Is an Implementation Detail Masquerading as an Architecture

**Risk level: HIGH**

The choice to name `CasbinPolicy` in the compiler output and couple the IAM module to Casbin's enforcer interface makes Casbin a permanent architectural commitment, not a replaceable implementation detail. The fact that the compiler output has a type named after a specific third-party library is an abstraction boundary violation.

Google Zanzibar, AWS Cedar, and OPA have different policy models, tuple formats, and evaluation semantics. Adopting any of them later would require changing the compiler output format and all consumers.

### 4.3 PolicyFunc Is Untestable in Isolation

**Risk level: MEDIUM**

`PolicyFunc` is a Go closure. It can only be tested by:
- Running the full pipeline with a real or mocked context
- Manually constructing the exact context it expects

There is no framework-level mechanism to: serialize a policy, inspect what predicates it generates for a given actor, or audit which policies are active for a given entity. Policy logic is a black box.

### 4.4 No Permission Versioning

**Risk level: MEDIUM**

When a business module changes its `PermissionSet`, existing Casbin policies in the database may conflict with new code declarations. There is currently no mechanism to:
- Version permission declarations
- Detect drift between code-declared permissions and database-loaded permissions
- Run permission migrations in a controlled way

### 4.5 Simultaneous Auth for API and SDUI

**Risk level: MEDIUM**

Today, API authorization and SDUI authorization are separate concerns that are not architecturally unified. A user who lacks "write" permission to an entity should not see the edit form in SDUI. But since `sdui/generator.go` does not accept permission context, this enforcement does not exist. A determined user could request the schema directly.

### 4.6 Platform Admin Has No Audit Trail

**Risk level: MEDIUM**

The `IsPlatformAdmin` bypass is a single boolean that circumvents all Casbin checks. Platform admin access is not recorded in the audit log at the authorization layer. For regulatory compliance (SOC2, ISO 27001, GDPR), all privileged access must be audited.

---

## 5. Future Scalability Analysis

### 5.1 Zero Trust

Zero Trust requires: never trust, always verify; explicit verification for every request; least-privilege access; assume breach. The current model fails Zero Trust on:

- Static role assignments with no continuous verification
- No device/context factors in authorization decisions
- No per-request revalidation of role grants
- Platform admin bypass is the antithesis of Zero Trust

Zero Trust requires the authorization model to move from `(subject, object, action)` to `(subject, object, action, context)` where context includes: time, device, location, risk score, session age, and MFA status.

### 5.2 Enterprise RBAC

Enterprise RBAC typically requires:
- Hierarchical roles with inheritance and override capability
- Role constraints (separation of duty: "cannot hold role A and role B simultaneously")
- Role cardinality limits ("at most 2 users can hold this role at once")
- Temporal roles (active 09:00–17:00 only)
- Organizational roles (role active within department subtree only)

The current model supports simple inheritance via Casbin `g` assertions but none of the others.

### 5.3 ABAC (Attribute-Based Access Control)

ABAC decisions are based on attributes of: the subject, the resource, the action, and the environment. Example: "accounts_payable users can approve invoices where invoice.total < 100,000 KES and invoice.vendor is in their approved vendor list."

The current model cannot express this. `PolicyFunc` can filter rows by actor attributes, but it is:
- Not composable declaratively
- Not configurable by tenants
- Not introspectable
- Not auditable per-decision

### 5.4 Multi-Organization Hierarchy

As Awo grows to serve enterprise clients with complex org structures (group companies, subsidiaries, cost centers, branches, departments), authorization must understand the organizational context of both the actor and the resource. The current flat permission model with no scope dimension cannot model:

- "CFO of parent company can read invoices from all subsidiaries"
- "Branch manager can approve expenses for their branch only"
- "Regional manager can override branch limits for their region"

### 5.5 External Identity Providers

When Awo integrates with Okta, Azure AD, Google Workspace, or Auth0, the IDP provides claims about the user (groups, roles, attributes). These must be mapped to Awo's internal role model. The current hardcoded role string `"role:finance.accounts_payable"` has no stable mapping mechanism to external IDP group names.

### 5.6 Service Accounts and Machine Identities

The current `Actor` struct has a `UserID` field but no concept of service account, API token scope, or machine identity. API tokens issued to third-party integrations should carry limited scopes, not inherit full user permissions.

---

## 6. Comparative Analysis

### 6.1 ERPNext / Frappe

**Permission model:** Role-based with permission levels (0=No access, 1=Read, 2=Write, 3=Create, 4=Delete, 5=Submit, 6=Cancel, 7=Amend), per-DocType, with page and field-level restrictions.

**What they do well:**
- Permission levels as integers — clean, orderable, UI-friendly
- Field-level permissions per role per DocType — can hide/read-only specific fields
- Document-level user restrictions (link user to specific branches/cost centers)
- Permission manager UI built into the framework — non-developer configuration
- Report-level and page-level permissions separate from entity permissions

**What we should adopt:**
- Separate permission levels (read, write, create, delete, submit, cancel, amend) rather than collapsing write=create=update
- Field-level permission declarations as first-class citizens in `FieldDef`
- The concept of a **Permission Manager** as a platform entity — permissions configured in database, not binary

**What we should deliberately avoid:**
- Permission integers — opaque, ordering implies hierarchy that doesn't always hold
- DocType-level permission model — too coarse for multi-tenant ERP where different tenants need different granularities
- Tight coupling of permission model to Frappe's specific architecture (hook into `frappe.has_permission` everywhere)
- Lack of compile-time validation — Frappe permissions are pure runtime database state with no compile-time safety net

### 6.2 Odoo

**Permission model:** Access rights per model per group (read/write/create/unlink), record rules (domain-based row filters), IR rules applied per group.

**What they do well:**
- Record rules as stored domain expressions — configurable without code
- Group hierarchy via XML inheritance — declarative composition
- Multi-company record rules — organization scope built into the model
- Separation of access rights (CRUD gates) from record rules (row filters)

**What we should adopt:**
- Record rules as stored, database-persisted domain expressions — this is the right way to make `PolicyFunc` tenant-configurable
- The clear separation of "access rights" (can you perform this operation at all) from "record rules" (which records can you see)
- Multi-company scoping as a first-class dimension

**What we should deliberately avoid:**
- XML-first permission declarations — no compile-time validation
- IR rules as raw domain strings — too fragile, no type safety
- Permission inheritance via XML override files — creates migration nightmares
- The global `sudo()` pattern — equivalent to our platform admin bypass, no audit trail

### 6.3 Keycloak Authorization Services

**Permission model:** UMA 2.0 (User-Managed Access), resource servers, resources, scopes, policies, permissions. Supports: role-based, attribute-based, JavaScript policy evaluation, time-based, aggregated policies.

**What they do well:**
- **Resources** are first-class objects with owners, scopes, and URIs
- **Scopes** are fine-grained operations attached to resources (not just CRUD)
- **Policies** are composable evaluation units: role policy, user policy, JS policy, time policy, aggregate policy
- **Permissions** bind a resource + scope to a set of policies
- Supports delegation: resource owners can grant access to other users
- Complete policy evaluation can be tested via API (permission evaluation endpoint)
- Full audit trail via event listeners

**What we should adopt:**
- **Resources + Scopes** model — richer than `(object, action)`; a single entity can have multiple scopes beyond CRUD (e.g., `invoice:read`, `invoice:submit`, `invoice:cancel`, `invoice:amend`)
- **Policy composition** — the concept that a permission is satisfied when a set of composed policies all evaluate to grant
- **Policy evaluation endpoint** — ability to ask "can actor A do X on resource Y?" without actually performing the operation
- **UMA delegation** — resource owners can grant access to other users for a specific resource

**What we should deliberately avoid:**
- The full UMA protocol complexity at v1.0 — it solves external delegation at internet scale, overkill for ERP
- JavaScript policy evaluation — runtime code injection in permission system is a security risk
- Full Keycloak dependency — we need our own authorization layer with optional Keycloak integration

### 6.4 AWS IAM

**Permission model:** Principals, identities, resources, actions, conditions. Policies expressed in JSON with explicit Allow/Deny, wildcards, condition operators, resource ARNs. Evaluation: explicit deny wins over explicit allow.

**What they do well:**
- **Explicit deny always wins** — deny-override semantics are the correct default for enterprise
- **Condition operators** — contextual predicates on the authorization decision (`aws:RequestTime`, `aws:SourceIp`, `s3:prefix`)
- **Resource patterns** — wildcard matching on resource identifiers (ARN-like)
- **Policy simulation** — IAM Policy Simulator can evaluate what a given policy allows without performing the action
- **Service control policies** — organization-level maximum permissions (cannot grant more than the SCP allows)
- **Permission boundaries** — limits on what an IAM entity can be granted
- **Managed vs inline policies** — reusable policy documents vs per-identity policies

**What we should adopt:**
- **Explicit deny > explicit allow** evaluation semantics — critical for enterprise security
- **Conditions** on permission grants — expressed as metadata predicates, not Go closures
- **Permission boundaries** concept — a maximum set of permissions that can be delegated
- The concept of **managed policies** that can be reused across roles (our current `PermissionSet` is per-entity, not reusable)
- **Policy simulation** as a first-class framework feature

**What we should deliberately avoid:**
- JSON policy documents as user-facing configuration — too complex for ERP admins
- The full ARN complexity — our entity namespace model (`module:entity`) is already clean
- Mixing authentication and authorization concepts (IAM does both; we should keep them separate)

### 6.5 Google Zanzibar

**Permission model:** Relation-based access control (ReBAC). Relationships stored as tuples: `(object, relation, user)`. Permissions evaluated by traversing the relationship graph. Supports: direct membership, computed userset (inherit permissions from another relation), and tuple-to-userset (cross-object permission).

**What they do well:**
- **Relationship tuples as first-class data** — stored in database, not in code
- **Permission evaluation by graph traversal** — naturally handles complex hierarchies without explicit enumeration
- **Consistent snapshots** — zookies (timestamps) ensure reads are evaluated against a consistent state
- **Universal applicability** — same model works for: user→resource, org→resource, folder→file, group→group
- **Expressive hierarchical permissions** — "can access X if member of group that has access to parent of X" without writing code

**What we should adopt:**
- **Relationship tuples as database-persisted entities** — this is the right model for tenant-configurable permissions
- The concept that **group membership IS a relationship** — not a special case in the permission model
- **Organization hierarchy via relationships** — a user's org unit membership determines their resource access through traversal, not through hardcoded policy functions
- **Snapshot-based evaluation** for consistent reads in multi-tenant context (aligns with our RLS model)
- The `(object#relation@user)` triple as the atomic unit of permission grant — more expressive than our current `(subject, object, action)` Casbin tuple

**What we should deliberately avoid:**
- Zanzibar's scale complexity — designed for billions of tuples; we need the model, not the Google-scale infrastructure
- Full consistency guarantees at Zanzibar level — eventual consistency with explicit staleness handling is sufficient for ERP
- The fully generic namespace approach — we have module-local entity names already; use them

### 6.6 Azure RBAC

**Permission model:** Role definitions (list of allowed operations + data actions), role assignments (principal + role + scope), deny assignments (explicit deny that overrides role assignments). Scope hierarchy: management group → subscription → resource group → resource.

**What they do well:**
- **Scope hierarchy** — permissions granted at parent scope apply to all children; permissions can be restricted at child scope
- **Data actions separate from management actions** — can grant read on resource metadata without granting read on resource data
- **Deny assignments** — can deny specific permissions even when inherited; useful for "everyone in org has read, except contractor accounts"
- **Built-in vs custom role definitions** — platform provides standard roles, tenants create custom ones
- **Role assignment audit log** — all changes to role assignments are recorded

**What we should adopt:**
- **Scope hierarchy** — permissions granted at organization level apply to all children unless overridden; this maps to our organization module
- **Data action vs management action** distinction — in Awo: read entity data vs read entity metadata vs manage entity definition
- **Deny assignments** as a first-class concept
- **Custom role definitions** that tenants create in the admin UI

**What we should deliberately avoid:**
- Azure's management group → subscription → resource group hierarchy — too many layers; our org hierarchy is sufficient
- The `*/read`, `*/write` wildcard action pattern — too broad, we want explicit action declarations
- Separate RBAC and ABAC systems — we want a unified evaluation pipeline

### 6.7 Kubernetes RBAC

**Permission model:** Role (within namespace) / ClusterRole (cluster-wide), RoleBinding / ClusterRoleBinding, Subjects (User, Group, ServiceAccount). Verbs: get, list, watch, create, update, patch, delete.

**What they do well:**
- **Namespace-scoped vs cluster-scoped** — directly analogous to our tenant-scoped vs platform-scoped
- **ServiceAccount as first-class principal** — machine identities are not second-class citizens
- **Aggregated ClusterRoles** — roles that automatically include permissions from roles matching a label selector; dynamic composition
- **Impersonation** — one principal can act as another (with audit); controlled delegation
- **Non-resource URLs** — permissions on API endpoints that don't map to resources (e.g. `/healthz`, `/metrics`)

**What we should adopt:**
- **Service accounts as first-class identities** — API tokens for integrations should be service account principals with their own permission grants
- **Impersonation with audit** — platform admins and support staff need controlled ability to act as tenant admins with full audit trail
- **Non-resource permissions** — permissions on framework operations (trigger migration, clear cache, run backfill) that don't map to entity CRUD
- **Binding separation** — roles are defined independently from role-to-subject bindings; bindings are data, not code

**What we should deliberately avoid:**
- The verb proliferation (get vs list vs watch are all "read" in ERP context)
- ClusterRole aggregation via label selectors — too clever, creates implicit dependencies
- Lack of deny rules — Kubernetes RBAC has no deny; all permissions are additive; enterprise ERP requires deny

---

## 7. Proposed Canonical Authorization Architecture

### 7.1 Core Principles

1. **Permissions are metadata, not code.** Role names, grants, and resource definitions live in the database. Code declares what permission slots exist; data fills them.
2. **Compile-time validation of permission slots.** The compiler validates that permission slots referenced in `EntityDefinition` exist in the registered permission vocabulary. Role names are validated against a platform-maintained registry.
3. **Engine-agnostic policy evaluation.** The framework defines a `PolicyEvaluator` interface. Casbin, OPA, Cedar, or a custom engine can be plugged in without changing the compiler output or business module code.
4. **Explicit deny overrides explicit allow.** Deny rules always win. This is non-negotiable for enterprise ERP where separation of duty is a compliance requirement.
5. **All permission decisions are auditable.** Every authorization decision (grant or deny) is recorded with: principal, resource, action, scope, policy applied, timestamp, request ID.
6. **Permission model is self-describing.** The compiled schema exports a complete permission metadata document that can drive admin UI, documentation, and policy simulation.
7. **Organization scope is a first-class dimension.** Every permission grant can be scoped to an organization node. Evaluation traverses the org hierarchy.
8. **PolicyFunc evolves into stored record rules.** Row-level filtering is expressed as a composable, database-persisted predicate that is tenant-configurable within framework-defined bounds.

### 7.2 Identity Model

```
Principal
├── UserPrincipal
│   ├── id: uuid
│   ├── tenant_id: uuid
│   ├── email: string
│   ├── external_idp_subject: string     ← IDP sub claim
│   ├── mfa_verified: bool
│   └── last_authenticated_at: time
│
├── ServiceAccountPrincipal
│   ├── id: uuid
│   ├── tenant_id: uuid
│   ├── name: string                     ← "erp-integration", "payroll-sync"
│   └── token_scopes: []Scope           ← explicit capability list
│
└── PlatformAdminPrincipal
    ├── id: uuid
    └── impersonating: *Principal        ← delegation with audit trail
```

### 7.3 Subject Model

A Subject is the authorization identity derived from a Principal at request time:

```
Subject
├── PrincipalID: uuid
├── PrincipalType: User | ServiceAccount | PlatformAdmin
├── TenantID: uuid
├── OrgUnitPath: []uuid               ← chain from root to actor's org unit
├── DirectRoles: []RoleID
├── EffectiveRoles: []RoleID           ← direct + inherited
├── TokenScopes: []Scope               ← for service accounts
├── SessionContext:
│   ├── MFAVerified: bool
│   ├── AuthMethod: password|sso|token|api_key
│   ├── SessionAge: duration
│   └── RiskScore: float               ← for Zero Trust
└── RequestContext:
    ├── RequestID: string
    ├── IPAddress: string
    └── Timestamp: time
```

Subject is constructed by the session middleware from the authenticated session and is immutable for the lifetime of the request. It replaces the current `def.Actor`.

### 7.4 Resource Model

```
Resource
├── EntityQualifiedName: string        ← "finance_invoice"
├── RecordID: *uuid                    ← nil for collection operations
├── OrgUnitID: *uuid                   ← which org unit owns this record
└── Attributes: map[string]any         ← record field values for ABAC conditions
```

Resource is constructed by the route handler before policy evaluation. For collection operations (list, create) `RecordID` is nil and `Attributes` are empty.

### 7.5 Permission Vocabulary

The `PermissionSlot` replaces the current role string in `PermissionSet`. Slots are declared in framework code and compiled into metadata:

```
PermissionSlot
├── ID: string                         ← "finance.invoice.submit"
├── Module: string                     ← "finance"
├── Resource: string                   ← "finance_invoice"
├── Action: string                     ← "submit"
├── Label: string                      ← "Submit Invoice"
├── Description: string
├── RiskLevel: Low | Medium | High     ← for audit and approval gating
└── SoDConflicts: []SlotID             ← separation of duty conflicts
```

Slots are registered by the compiler from `EntityDefinition` and `ActionDef` declarations. The IAM module exposes a `GET /api/v1/platform/permission-slots` endpoint that lists all slots — this drives the permission manager UI.

### 7.6 Role Model

```
RoleDefinition                         ← platform entity (IAM module)
├── ID: uuid
├── TenantID: uuid                     ← nil for platform-defined roles
├── Name: string                       ← stable identifier: "finance.accounts_payable"
├── Label: string                      ← "Accounts Payable Clerk" (tenant-translatable)
├── Module: string                     ← "finance" (nil for cross-module roles)
├── IsSystemRole: bool                 ← platform-defined, cannot be deleted
├── Grants: []PermissionGrant
└── InheritsFrom: []RoleID

PermissionGrant
├── SlotID: string                     ← "finance.invoice.submit"
├── OrgScope: OrgScope                 ← Unrestricted | OwnUnit | UnitAndChildren | Specific([]uuid)
└── Conditions: []GrantCondition       ← ABAC predicates

GrantCondition
├── Field: string                      ← resource field name
├── Operator: eq|lt|gt|in|contains|regex
└── Value: any                         ← literal or subject attribute reference
```

**Key properties:**
- Roles are database entities, not binary strings
- Platform-defined system roles are seeded at tenant bootstrap (cannot be modified by tenants)
- Tenant admins can create custom roles that grant subsets of system role permissions
- Role hierarchy via `InheritsFrom` — inheritance accumulates via traversal, not Casbin `g` assertions
- `SoDConflicts` on `PermissionSlot` are enforced at role-assignment time, not policy-evaluation time

### 7.7 Permission Grant Pipeline

The grant system replaces the current flat `PermissionSet.Create/Read/Write/Delete` string lists:

```
Grant                                  ← platform entity (IAM module)
├── ID: uuid
├── TenantID: uuid
├── GranteeType: Role | User | ServiceAccount
├── GranteeID: uuid
├── SlotID: string                     ← references PermissionSlot
├── OrgScope: OrgScope
├── Conditions: []GrantCondition
├── ValidFrom: *time.Time              ← nil = immediate
├── ValidUntil: *time.Time             ← nil = permanent
├── GrantedBy: uuid                    ← user who created this grant
├── ApprovedBy: *uuid                  ← if grant required approval
└── Reason: string                     ← audit trail

DenyRule                               ← explicit deny, overrides all grants
├── ID: uuid
├── TenantID: uuid
├── DeniedType: Role | User | ServiceAccount
├── DeniedID: uuid
├── SlotID: string
├── OrgScope: OrgScope
├── ValidUntil: *time.Time
└── Reason: string
```

### 7.8 Evaluation Pipeline

```
Request arrives
      │
      ▼
┌─────────────────────────────────────────────────────────────┐
│  1. AUTHENTICATE                                             │
│     Session token → Principal                               │
│     API key → ServiceAccountPrincipal                       │
│     IDP token → UserPrincipal (via IDP mapping)             │
└─────────────────────────────────────────────────────────────┘
      │
      ▼
┌─────────────────────────────────────────────────────────────┐
│  2. BUILD SUBJECT                                            │
│     Principal + Session + Org memberships → Subject         │
│     EffectiveRoles = DirectRoles ∪ InheritedRoles           │
│     OrgUnitPath = full ancestry chain                       │
│     Cached in Redis for session lifetime                    │
└─────────────────────────────────────────────────────────────┘
      │
      ▼
┌─────────────────────────────────────────────────────────────┐
│  3. RESOLVE RESOURCE                                         │
│     Route → EntityQualifiedName + Operation + Action        │
│     Record lookup → OrgUnitID + Attributes (for ABAC)       │
└─────────────────────────────────────────────────────────────┘
      │
      ▼
┌─────────────────────────────────────────────────────────────┐
│  4. RESOLVE PERMISSION SLOT                                  │
│     (EntityQualifiedName, Operation) → SlotID               │
│     From compiled schema — O(1) lookup                      │
└─────────────────────────────────────────────────────────────┘
      │
      ▼
┌─────────────────────────────────────────────────────────────┐
│  5. EVALUATE DENY RULES                                      │
│     Any matching DenyRule → DENY immediately                │
│     Explicit deny ALWAYS wins                               │
└─────────────────────────────────────────────────────────────┘
      │
      ▼
┌─────────────────────────────────────────────────────────────┐
│  6. EVALUATE GRANTS                                          │
│     For each effective role:                                │
│       Load grants for SlotID                               │
│       Check OrgScope: actor.OrgPath ∩ resource.OrgUnitID   │
│       Evaluate GrantConditions against resource attributes  │
│       Check temporal validity (ValidFrom, ValidUntil)       │
│     Any grant satisfied → proceed to row filter             │
│     No grants satisfied → DENY                             │
└─────────────────────────────────────────────────────────────┘
      │
      ▼
┌─────────────────────────────────────────────────────────────┐
│  7. APPLY ROW FILTER (formerly PolicyFunc)                   │
│     Load stored RecordRules for entity + actor roles        │
│     Compose all matching rules with AND                     │
│     Inject into repository query                            │
└─────────────────────────────────────────────────────────────┘
      │
      ▼
┌─────────────────────────────────────────────────────────────┐
│  8. APPLY FIELD FILTER                                       │
│     For read operations: remove fields actor cannot read    │
│     For write operations: reject writes to fields actor     │
│     cannot write                                            │
└─────────────────────────────────────────────────────────────┘
      │
      ▼
┌─────────────────────────────────────────────────────────────┐
│  9. EMIT AUDIT EVENT                                         │
│     Record: Principal, SlotID, Resource, Outcome,           │
│     GrantsEvaluated, DenyRulesEvaluated, Timestamp          │
└─────────────────────────────────────────────────────────────┘
```

### 7.9 Policy Evaluator Interface

The framework defines a `PolicyEvaluator` interface. All authorization decisions flow through it. The default implementation uses the grant/deny model above. A Casbin adapter, OPA adapter, or Cedar adapter can be substituted:

```
PolicyEvaluator interface {
    // Evaluate returns the authorization decision for a single permission slot.
    Evaluate(ctx, subject Subject, resource Resource, slotID string) Decision

    // EvaluateWithExplanation returns the decision plus the reasoning trace.
    // Used by the policy simulation endpoint.
    EvaluateWithExplanation(ctx, subject Subject, resource Resource, slotID string) (Decision, Explanation)
}

Decision {
    Outcome: Allow | Deny
    Reason:  string
    MatchedGrant: *GrantID
    MatchedDeny:  *DenyRuleID
}

Explanation {
    DenyRulesChecked:  []DenyRuleEvaluation
    GrantsChecked:     []GrantEvaluation
    OrgScopeReason:   string
    ConditionResults:  []ConditionResult
}
```

### 7.10 Viewer Context

Replace `def.Actor` with `ViewerContext`, constructed by the session middleware and passed through all framework layers:

```
ViewerContext {
    Subject:          Subject           ← full identity + session context
    EffectiveGrants:  []GrantSummary   ← pre-resolved for performance
    OrgMemberships:   []OrgMembership  ← org units + roles within each
    Evaluator:        PolicyEvaluator  ← bound evaluator instance
    RequestID:        string
    TenantID:         uuid

    // Convenience methods
    CanDo(slotID string, resource Resource) bool
    CanDoInOrg(slotID string, orgUnitID uuid) bool
    MustCanDo(slotID string, resource Resource) error   // returns PermissionError or nil
    VisibleFields(entityName string) []string            // field-level filter
}
```

`ViewerContext` is injected via `context.Context`. All hooks, action handlers, and policy functions receive it. Hooks no longer need to extract the actor from session — they call `viewer.CanDo(...)` directly.

### 7.11 Compiled Permission Metadata

The compiler emits, in addition to current outputs, a `PermissionManifest`:

```
PermissionManifest {
    Slots: []PermissionSlot        ← all registered slots, keyed by ID
    DefaultGrants: []DefaultGrant  ← platform-defined initial grants for system roles
    SoDRules: []SoDRule            ← separation of duty conflicts between slots
    FieldPermissions: map[string][]FieldPermissionSlot
}
```

`DefaultGrants` are the compile-time declarations from `EntityDefinition` — they express what a fresh tenant installation should grant by default. These are applied at tenant bootstrap, not at every request. After bootstrap, grants are database records that can be modified by the tenant admin.

This means **business module code no longer hardcodes role names**. It declares `DefaultGrants` using symbolic role names (`platform.tenant_admin`, `finance.accounts_payable`) that are resolved against the registered `RoleRegistry` at compile time.

### 7.12 Organization Scope Evaluation

```
OrgScope (enum)
├── Global              ← ignores org hierarchy; any matching resource
├── OwnUnitOnly         ← resource.OrgUnitID must be actor's primary org unit
├── UnitAndDescendants  ← resource.OrgUnitID must be in actor's subtree
└── ExplicitUnits       ← resource.OrgUnitID must be in granted unit list
```

The organization hierarchy is a tree stored in `platform_org_unit`. Scope evaluation walks the tree. The evaluated subtree is cached per actor per session in Redis.

### 7.13 Record Rules (Replacing PolicyFunc)

`PolicyFunc` is deprecated in favor of `RecordRule` — a database-persisted, tenant-configurable predicate:

```
RecordRule                             ← platform entity (IAM module)
├── ID: uuid
├── TenantID: uuid
├── EntityQualifiedName: string        ← "finance_invoice"
├── SlotID: string                     ← applies when actor has this slot
├── PredicateType: OwnerOnly | OrgScoped | AttributeMatch | Custom
├── PredicateConfig: map[string]any    ← type-specific configuration
│   ── OwnerOnly: {field: "assigned_to"}
│   ── OrgScoped: {field: "org_unit_id"}
│   ── AttributeMatch: {field: "status", operator: "in", values: ["Draft"]}
├── LogicOp: And | Or (with sibling rules)
└── IsSystemRule: bool                 ← true = framework-defined, cannot be deleted
```

The runtime compiles `RecordRule` entries into `Filter` predicates at request time. System-defined rules (the current `PolicyFunc` equivalents) are expressed as `IsSystemRule: true` entries seeded at bootstrap.

Tenant admins can add `RecordRule` entries in the admin UI to further restrict row visibility without code changes.

---

## 8. Permission Declarations After Redesign

### 8.1 EntityDefinition — Before vs After

**Before (current):**
```go
var InvoiceDefinition = def.SystemDefinition{
    Name:   "invoice",
    Module: "finance",
    Permissions: def.PermissionSet{
        Create: []string{"role:finance.accounts_payable", "role:tenant.admin"},
        Read:   []string{"role:finance.viewer", "role:tenant.admin"},
        Write:  []string{"role:finance.accounts_payable", "role:tenant.admin"},
        Delete: []string{"role:tenant.admin"},
        Actions: map[string][]string{
            "submit": {"role:finance.accounts_payable"},
        },
        Policy: def.PolicyFunc(func(ctx context.Context) def.Filter {
            actor := session.ActorFromContext(ctx)
            return filter.Eq("assigned_to", actor.UserID)
        }),
    },
}
```

**After (proposed):**
```go
var InvoiceDefinition = def.SystemDefinition{
    Name:   "invoice",
    Module: "finance",
    Permissions: def.PermissionDeclaration{
        // Slots are declared symbolically. The compiler validates these names
        // against the platform RoleRegistry. No magic strings.
        DefaultGrants: []def.DefaultGrant{
            {Slot: "finance.invoice.create", ToRoles: []string{"finance.accounts_payable", "platform.tenant_admin"}},
            {Slot: "finance.invoice.read",   ToRoles: []string{"finance.viewer", "platform.tenant_admin"}},
            {Slot: "finance.invoice.write",  ToRoles: []string{"finance.accounts_payable", "platform.tenant_admin"}},
            {Slot: "finance.invoice.delete", ToRoles: []string{"platform.tenant_admin"}},
        },
        // SystemRecordRules replace PolicyFunc — stored at tenant bootstrap,
        // not evaluated as closures.
        SystemRecordRules: []def.SystemRecordRule{
            {
                SlotID:         "finance.invoice.read",
                PredicateType:  def.OrgScopedPredicate,
                PredicateConfig: map[string]any{"field": "org_unit_id"},
            },
        },
        // SoD constraints declared here, enforced at role-assignment time.
        SoDConflicts: []def.SoDConstraint{
            {SlotA: "finance.invoice.create", SlotB: "finance.invoice.approve",
             Message: "Cannot create and approve same invoices (separation of duty)"},
        },
    },
}
```

### 8.2 ActionDef — Before vs After

**Before (current):**
```go
ActionDef{
    Name:       "submit",
    Permission: "role:finance.accounts_payable",  // single string, cannot express OR, context, org scope
    HandlerFunc: SubmitInvoiceAction,
}
```

**After (proposed):**
```go
ActionDef{
    Name:       "submit",
    SlotID:     "finance.invoice.submit",          // references a registered PermissionSlot
    // No permission subjects here — grants are in the database, configured by tenant admin
    // The slot carries: label, description, risk level, SoD conflicts
    HandlerFunc: SubmitInvoiceAction,
    // Optional compile-time guard: record must satisfy this condition for action to be available
    // This is a metadata predicate, not a Go closure
    AvailableWhen: &def.RecordCondition{
        Field: "status", Operator: def.OperatorIn, Values: []any{"Draft", "Rejected"},
    },
}
```

### 8.3 FieldDef — Permission Additions

```go
FieldDef{
    Name: "internal_notes",
    Type: def.FieldTypeLongText,
    // Field-level permission slots — compiler registers these as sub-slots
    ReadSlot:  "finance.invoice.read_internal",   // separate from invoice.read
    WriteSlot: "finance.invoice.write_internal",
    // Falls back to entity-level slot if nil — no field slot = same as entity slot
}
```

### 8.4 ActionHandlerFunc — ViewerContext Usage

```go
func SubmitInvoiceAction(ctx *def.ActionContext) (*def.ActionResult, error) {
    viewer := ctx.Viewer                           // ViewerContext, not Actor

    // Check additional permission before executing critical step
    if !viewer.CanDo("finance.journal.create", def.Resource{}) {
        return nil, def.PermissionError("finance.journal.create")
    }

    // Repository access through ActionRuntime
    repo := ctx.Runtime.Repository("finance_invoice")
    // ...
}
```

---

## 9. Tenant Admin Configuration

### 9.1 Permission Manager Workflow

**Step 1: View Available Permission Slots**

Tenant admin opens `Settings → Permissions → Permission Slots`. The SDUI page is driven by `GET /api/v1/platform/permission-slots?module=finance` which returns all compiled `PermissionSlot` metadata. No code needed — the slots are compiler-generated.

**Step 2: View Registered Roles**

`Settings → Permissions → Roles` lists all roles for the tenant: system roles (platform-defined, cannot be deleted) and custom roles (created by the tenant admin).

**Step 3: Create Custom Role**

```
Role Name:     "senior_ap_clerk"
Label:         "Senior AP Clerk"
Inherits From: "finance.accounts_payable"
```

The admin grants additional permission slots to this role:
```
+ finance.invoice.approve  (OrgScope: OwnUnitOnly)
```

And adds a condition:
```
GrantCondition: invoice.total_kes < 500,000
```

This role is now: "can approve invoices within own org unit where invoice total is under 500,000 KES."

**Step 4: Assign Role to User**

```
User:          alice@company.com
Role:          senior_ap_clerk
OrgUnit:       "Nairobi Branch"
ValidUntil:    2026-12-31                    (temporary grant)
Reason:        "Covering for Wanjiru on maternity leave"
```

**Step 5: Add Record Rule (optional)**

If the tenant wants to further restrict what invoices Alice can see:

```
Entity:        finance_invoice
Applies When:  actor has slot finance.invoice.read
Predicate:     invoice.org_unit_id = actor.primary_org_unit_id
```

This is stored as a `RecordRule` in the database. The framework applies it at query time.

**Step 6: Simulate Permission**

`Settings → Permissions → Policy Simulator`

Input:
```
User:    alice@company.com
Action:  submit
Record:  INV-2026-00047 (status=Draft, total=450,000 KES, org_unit=Nairobi Branch)
```

Output:
```
Decision: ALLOW
Matched Grant: senior_ap_clerk → finance.invoice.submit
Org Scope: OwnUnitOnly — record org_unit (Nairobi Branch) matches actor org_unit (Nairobi Branch)
Condition: invoice.total_kes (450,000) < 500,000 — PASS
Row Rule: invoice.org_unit_id = actor.primary_org_unit — PASS
```

**Step 7: Temporary Elevation / Break-Glass**

Platform admin or tenant admin can create a time-bounded grant with approval requirement:

```
Grant:         alice@company.com → finance.invoice.delete
ValidUntil:    +4 hours
Requires Approval From: role:platform.tenant_admin
Reason:        "Bulk cleanup of test invoices before go-live"
```

The grant activates only after approval. All actions taken under this grant are flagged in the audit log as "elevated access."

---

## 10. Request Authorization Flow

### 10.1 Complete Flow: `POST /api/v1/finance/invoices/:id/submit`

```
1. REQUEST ARRIVES at Fiber middleware pipeline

2. REQUEST ID assigned (X-Request-ID or generated UUID)

3. STRUCTURED LOGGING context initialized

4. PANIC RECOVERY installed

5. CORS validated (per-tenant subdomain origin)

6. TENANT RESOLVED from X-Tenant-ID / subdomain / query param
   → set_tenant_context($tenant_id) called on PostgreSQL connection
   → RLS activated for this connection

7. SESSION VALIDATED
   Session token → Redis lookup
   → Principal constructed (UserPrincipal or ServiceAccountPrincipal)
   → If expired: 401 Unauthorized
   → If not found: 401 Unauthorized
   → If tenant mismatch: 403 Forbidden

8. SUBJECT BUILT
   Principal + OrgMemberships + DirectRoles → Subject
   EffectiveRoles = DirectRoles ∪ InheritedRoles (org hierarchy traversal)
   OrgUnitPath = full ancestry chain from root to actor's org unit
   Cached: Redis key "subject:{session_token}" TTL = session expiry

9. RATE LIMIT CHECKED (per-tenant + per-user sliding window)

10. ROUTE MATCHED
    Path: "/api/v1/finance/invoices/:id/submit"
    → RouteDescriptor: {EntityQualifiedName: "finance_invoice", Operation: "action", ActionName: "submit"}
    → PermissionSlot: "finance.invoice.submit"

11. DENY RULES EVALUATED
    Load DenyRules for subject's roles + direct user denies
    Filter by SlotID = "finance.invoice.submit"
    Filter by TenantID + temporal validity
    Any match → 403 Forbidden (audit event written)

12. RECORD LOADED (for action routes, record must exist)
    SELECT * FROM finance_invoice WHERE id = :id
    (RLS ensures tenant isolation — cannot see other tenant's records)
    → Resource constructed: {EntityQualifiedName, RecordID, OrgUnitID, Attributes{status, total_kes, ...}}
    → 404 if not found

13. GRANTS EVALUATED
    For each EffectiveRole in subject.EffectiveRoles:
      Load grants WHERE slot_id = "finance.invoice.submit" AND grantee = role AND tenant = :tenant
      Evaluate OrgScope:
        OwnUnitOnly → resource.OrgUnitID == actor.PrimaryOrgUnitID?
        UnitAndDescendants → resource.OrgUnitID ∈ actor.OrgSubtree?
      Evaluate GrantConditions:
        invoice.total_kes < 500000?
        invoice.status ∈ ["Draft", "Rejected"]?
      Evaluate temporal: ValidFrom <= now <= ValidUntil?
    Any grant fully satisfied → proceed
    No grants → 403 Forbidden (audit event written)

14. AvailableWhen CONDITION CHECKED (from ActionDef metadata)
    record.status ∈ ["Draft", "Rejected"] — if not met → 409 Conflict
    This is a business rule guard, not a permission check

15. ACTION HANDLER INVOKED
    ctx.Viewer = ViewerContext{Subject, EffectiveGrants, Evaluator}
    ctx.Runtime = ActionRuntime{Repo, Tx, EventBus, WorkflowRuntime, ...}
    SubmitInvoiceAction(ctx) called

16. WITHIN HANDLER:
    a. Any additional viewer.CanDo() checks are evaluated by PolicyEvaluator
    b. Repository operations are wrapped with ViewerContext
    c. Row-level RecordRules applied at repository query time
    d. Field-level permissions applied at response serialization time

17. RESPONSE SERIALIZED
    Sensitive fields excluded
    Fields actor cannot read excluded
    200 OK or 202 Accepted (if workflow triggered)

18. AUDIT EVENT WRITTEN (async, non-blocking)
    Principal, SlotID, Resource, Outcome, GrantID, DenyRuleID,
    RequestID, TenantID, Timestamp, Duration
```

### 10.2 Login Flow

```
POST /api/v1/auth/login {email, password, tenant_id}
  ↓
Verify credentials against iam_user record
  ↓
Verify tenant status = ACTIVE
  ↓
MFA challenge if required (based on tenant policy)
  ↓
Build Principal
  ↓
Load DirectRoles from iam_role_assignment
  ↓
Traverse role inheritance graph → EffectiveRoles
  ↓
Load OrgMemberships → OrgUnitPath
  ↓
Construct Subject
  ↓
Pre-resolve EffectiveGrants for common slots (warm cache)
  ↓
Write session to Redis: key="session:{token}", TTL=tenant-configured
  ↓
Return {token, expires_at, subject_summary}
```

---

## 11. Required Platform Additions

### 11.1 New Platform Entities (IAM Module Extensions)

| Entity | Purpose |
|---|---|
| `iam_permission_slot` | Compiler-seeded registry of all permission slots |
| `iam_role_definition` | Tenant-configurable role records (extends system roles) |
| `iam_role_grant` | Binds a role definition to a set of permission slots with scope + conditions |
| `iam_principal_grant` | Direct user/service-account grants (bypass role assignment) |
| `iam_deny_rule` | Explicit deny overrides |
| `iam_record_rule` | Database-persisted row-level predicates (replaces PolicyFunc) |
| `iam_role_assignment` | Principal → Role + OrgUnit + temporal bounds |
| `iam_delegation` | User A → delegates subset of permissions to User B |
| `iam_service_account` | Machine identity with token scopes |
| `iam_api_token` | Service account token with scope + expiry |
| `platform_org_unit` | Organizational hierarchy node |
| `platform_org_membership` | User membership in an org unit |

### 11.2 New Compiler Phases

**Phase 5: Permission Slot Emission**

After current Phase 4 (Casbin policy emission), the compiler adds:

- Scan all `EntityDefinition.Permissions.DefaultGrants` declarations
- Validate slot IDs against registered permission vocabulary
- Validate role names against registered `RoleRegistry`
- Emit `PermissionManifest` containing all slots, default grants, SoD rules, field permissions
- The `PermissionManifest` replaces `[]CasbinPolicy` in `CompiledSchema`
- `CasbinPolicy` type is removed from compiler output (engine-agnostic)

**Phase 6: Record Rule Registration**

- Scan `SystemRecordRules` from all `EntityDefinition.Permissions` declarations
- Validate predicate types and field names against entity schemas
- Emit `SystemRecordRuleManifest` — applied at tenant bootstrap

### 11.3 New Runtime Services

| Service | Responsibility |
|---|---|
| `SubjectResolver` | Builds `Subject` from Principal + session + org memberships |
| `PolicyEvaluator` | Evaluates deny rules + grants for a slot + resource |
| `GrantStore` | Loads and caches grants/deny-rules by tenant + slot |
| `RecordRuleEngine` | Compiles `RecordRule` predicates into `Filter` DSL |
| `FieldPermissionFilter` | Strips unreadable fields from response, rejects writes to unwritable fields |
| `AuditEmitter` | Records permission decisions to audit log (async) |
| `PolicySimulator` | Evaluates hypothetical decisions without side effects |
| `OrgHierarchyCache` | Caches org unit ancestry per tenant |

### 11.4 Updated `ActionContext`

The already-modified `ActionContext` gains `Viewer ViewerContext` (replacing current `Runtime ActionRuntime`). The `ActionRuntime` remains as the service-access interface within `ViewerContext`.

### 11.5 New Documentation Sections

| Section | Content |
|---|---|
| Authorization Model Overview | Identities, subjects, roles, grants, slots, scopes, evaluation pipeline |
| Permission Slot Reference | All slots for all modules, auto-generated from `PermissionManifest` |
| Role Configuration Guide | How tenant admins create and assign roles |
| Record Rule Guide | How to configure row-level visibility rules |
| Policy Simulation Guide | How to test authorization decisions before deploying |
| SoD Conflict Reference | All separation of duty rules declared in framework |
| Break-Glass Procedures | Emergency elevation, audit requirements |
| IDP Integration Guide | Mapping external groups to Awo roles |
| Service Account Guide | Creating and scoping API tokens |

---

## 12. Phased Migration Strategy

### Phase 0: Stabilize Current Model (v0.x — Now, No Breaking Changes)

**Goal:** Make the current model internally consistent without any breaking changes.

Actions:
1. Add `PermissionManifest` to `CompiledSchema` alongside `CasbinPolicy` — additive only
2. Make SDUI `generate()` accept and use `ViewerContext` for field filtering — the existing `Actor` temporarily adapts to a stub `ViewerContext`
3. Add `PermissionSlot` registration infrastructure — slots can be declared in code even before they're database-persisted
4. Write `PolicySimulator` stub that evaluates current Casbin policies — same interface as the eventual full evaluator
5. Add introspection of `PermissionSet` declarations (what roles are declared for each entity) to the introspect API
6. Add audit trail for permission decisions (Casbin evaluation results) to the audit log

**Breaking changes:** None. Current `PermissionSet` still works. Current role strings still work.

### Phase 1: Database-Persisted Role Model (v1.1 — After v1.0 Freeze)

**Goal:** Move role grants from binary to database. Business module code still declares role names as strings but these now validate against a registered `RoleRegistry`.

Actions:
1. Deploy `iam_permission_slot`, `iam_role_definition`, `iam_role_grant` entities
2. Compiler validates that role names in `PermissionSet` exist in `RoleRegistry`
3. Tenant bootstrap seeds `DefaultGrant` declarations as database records
4. IAM middleware switches from Casbin to `PolicyEvaluator` backed by database grants
5. Casbin is now an optional, pluggable adapter — not the default
6. `iam_role_assignment` entity deployed; tenant admins can assign roles to users
7. Permission Manager SDUI page deployed

**Breaking changes:** Role string format changes from `"role:finance.accounts_payable"` to `"finance.accounts_payable"` (the `role:` prefix was an artifact of Casbin subject format, not a semantic requirement). A migration helper converts existing strings.

### Phase 2: Slot-Based Action Permissions (v1.2)

**Goal:** Replace `ActionDef.Permission string` with `ActionDef.SlotID string`.

Actions:
1. Deploy `PermissionSlot` as a compiler-registered concept
2. Add `SlotID` field to `ActionDef` — `Permission` field deprecated but still supported
3. Compiler emits slot metadata for all actions (auto-generates slots from entity + action name)
4. `AvailableWhen` condition added to `ActionDef`
5. SDUI uses slot metadata to show/hide action buttons based on `ViewerContext`

**Breaking changes:** `ActionDef.Permission` deprecated (still works, emits a compiler warning). Modules should migrate to `SlotID` over the v1.2 release cycle.

### Phase 3: Organization Scope (v1.3)

**Goal:** Add `OrgScope` to grants and evaluation pipeline.

Actions:
1. Deploy `platform_org_unit`, `platform_org_membership`
2. `Subject` construction includes `OrgUnitPath`
3. `PolicyEvaluator` evaluates `OrgScope` on grants
4. Record rules gain `OrgScoped` predicate type
5. Finance module invoices and payments gain `org_unit_id` field
6. Existing grants get `OrgScope: Global` migration (preserves current behavior)

**Breaking changes:** None for existing code. Optional adoption of org-scoped grants.

### Phase 4: Record Rules (v1.4)

**Goal:** Replace `PolicyFunc` with stored `RecordRule`.

Actions:
1. Deploy `iam_record_rule` entity
2. `RecordRuleEngine` compiles rules into `Filter` predicates
3. `SystemRecordRules` on `PermissionDeclaration` seeded at bootstrap
4. `PolicyFunc` deprecated — emits compiler warning, still evaluates as a fallback
5. Migration tooling: analyze existing `PolicyFunc` closures and generate equivalent `SystemRecordRule` declarations
6. Tenant admins can add custom record rules via Permission Manager

**Breaking changes:** `PolicyFunc` deprecated (not removed). Framework evaluates both `PolicyFunc` (legacy) and `RecordRule` (new) with AND semantics during transition.

### Phase 5: Field-Level Permissions (v1.5)

**Goal:** Field-level read and write slots.

Actions:
1. `FieldDef` gains optional `ReadSlot` and `WriteSlot`
2. Compiler emits `FieldPermissionSlot` entries in `PermissionManifest`
3. `FieldPermissionFilter` applied at response serialization and input validation
4. SDUI generator uses `ViewerContext.VisibleFields()` to exclude unreadable fields from schema
5. The existing `Sensitive: true` flag becomes a shorthand for "ReadSlot requires sensitive_data_reader role"

**Breaking changes:** None. Fields without explicit slots inherit entity-level slot (current behavior preserved).

### Phase 6: Explicit Deny + SoD (v1.6)

**Goal:** Explicit deny rules and separation of duty enforcement.

Actions:
1. Deploy `iam_deny_rule` entity
2. Evaluation pipeline adds deny rule check before grant evaluation
3. `SoDConflicts` on `PermissionSlot` enforced at `iam_role_assignment` creation time
4. Platform admin UI for managing deny rules

**Breaking changes:** None. Additive.

### Phase 7: Delegated Permissions + Break-Glass (v2.0)

**Goal:** Time-bounded grants, delegation, break-glass access.

Actions:
1. Deploy `iam_delegation` entity
2. Temporal validity fields on `iam_role_assignment` activated
3. Approval workflow for break-glass elevation (Temporal workflow)
4. Platform admin impersonation with full audit trail
5. `PolicySimulator` complete implementation

**Breaking changes:** None. All additive.

---

## v1.0 Freeze Decision

### What MUST Change Before v1.0

The following items constitute v1.0 blockers from an authorization architecture perspective:

1. **`CasbinPolicy` must be renamed** to `PolicyTuple` or `PermissionTuple` in compiler output. The compiler output type must not name a specific third-party library. This is a pure rename — no semantic change. Low risk.

2. **`ActionDef.Permission` must accept a slice, not a single string.** The current single-string field cannot express OR semantics. Before v1.0 freeze, change to `Permissions []string` with the single-string form as a convenience wrapper. This prevents a breaking change at v1.1.

3. **`PermissionManifest` must be part of `CompiledSchema`.** The compiler must emit permission metadata in a form that the introspect API can serve. This enables the Permission Manager UI at v1.1 without requiring a compiler change.

4. **Introspect endpoint must be permission-gated.** Currently unprotected. Must require `role:platform.admin` or `role:tenant.admin` before v1.0.

5. **SDUI generator must accept `ViewerContext`.** Currently a placeholder. Must be wired before v1.0 so permission-gated elements are actually absent from schema.

### What Can Wait Until v1.1

- Database-persisted roles (Phase 1)
- Organization scope (Phase 3)
- Record rules replacing PolicyFunc (Phase 4)
- Field-level permissions (Phase 5)
- Explicit deny rules (Phase 6)
- Delegation and break-glass (Phase 7)

### Assessment

The current authorization architecture is **sound as a foundation but incomplete as an implementation**. The compiler-first, metadata-driven approach is correct. The namespace architecture is correct. The dual-layer model (operation gate + row filter) is correct. The absence of a `ViewerContext`, the coupling to Casbin naming, the single-string `ActionDef.Permission`, and the unimplemented SDUI permission gates are the only structural issues that must be resolved before v1.0.

The proposed architecture described in this document is directly evolvable from the current state through the phased migration strategy. No architectural restart is required.

---

*End of Authorization Architecture Review*
