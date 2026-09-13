> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Awo Canonical Authorization Architecture
## Pre-v1.0 Lead Architect Review

**Date:** 2026-07-20
**Status:** Final architectural recommendation — no code generated
**Supersedes:** AWO_AUTH_ARCHITECTURE_REVIEW.md (that document identified the problems; this one resolves them)

---

## Governing Principle

> **Framework defines capabilities. IAM defines who holds them. Business modules know only what exists.**

Every design decision in this document is evaluated against that single principle. Any concept that causes a business module to reference a role name, a user, or a grant is architecturally incorrect regardless of how convenient it appears.

---

## Part 1 — Critical Review of the Previous Proposal

### 1.1 DefaultGrants — Incorrect Placement

**Verdict: Wrong. Must be removed from business modules entirely.**

The proposal has `EntityDefinition.Permissions.DefaultGrants` mapping capabilities to role names inside the Finance module:

```
DefaultGrants: []def.DefaultGrant{
    {Slot: "finance.invoice.create", ToRoles: []string{"finance.accounts_payable", ...}},
}
```

This violates the governing principle completely. Finance now knows the name `finance.accounts_payable`. If a deployment renames that role, the binary is wrong. If a tenant wants a different default configuration, there is no override path. If a green-field deployment uses a flat single-role model, the declared defaults create noise in the bootstrap.

DefaultGrants belong in the IAM module's bootstrap configuration — a file that is authoritatively IAM's responsibility and can be replaced per deployment without touching Finance, CRM, or any other business module.

Business modules declare capabilities. Who receives them is not their concern.

### 1.2 PermissionSlot.RiskLevel — Wrong Layer

**Verdict: Remove from capability declaration.**

`RiskLevel: Low | Medium | High` on a `PermissionSlot` is a risk assessment. Risk assessment is a compliance posture, not a capability property. Different organizations classify the same action at different risk levels. Finance approval in a 5-person startup is Low. Finance approval in a regulated bank is High.

Risk classification belongs in IAM role configuration or tenant policy, not compiled into the binary by the module developer.

The correct approach: `RiskLevel` is a property of a `Grant` or `Role`, not a `Capability`. The tenant decides how risky they consider a given grant to be.

### 1.3 PermissionSlot.SoDConflicts — Premature and Wrong Layer

**Verdict: Remove entirely from v1.0. Wrong location regardless of version.**

Separation-of-duty constraints say "a user cannot hold both capability A and capability B simultaneously." But whether this constraint should exist depends entirely on the tenant's compliance requirements. A small business running Awo as a basic ERP has no need for SoD enforcement. A bank does.

Additionally, SoD enforcement at the capability level prevents legitimate use cases. Some users legitimately need both create and approve access in emergency situations. The correct model is role-level SoD configuration managed entirely by the IAM module — not hardcoded into capability metadata.

SoD is deferred to post-v1.0. When introduced, it belongs on `RoleDefinition` or `Grant`, not on `Capability`.

### 1.4 PlatformAdminPrincipal as a Separate Type — Wrong

**Verdict: Remove. Platform admin is a grant, not a principal type.**

The proposal has three principal types: UserPrincipal, ServiceAccountPrincipal, PlatformAdminPrincipal. This is wrong because it models an authorization decision (what this person can do) as an identity property (what kind of person they are).

Platform admin access is a grant of the `platform.admin` capability — held by a UserPrincipal. The evaluator treats it like any other grant but with cross-tenant scope. The current `Actor.IsPlatformAdmin bool` bypass that circumvents Casbin entirely must be removed before v1.0. Replace it with a grant-based evaluation that is auditable and revocable.

The "bypass everything" behavior remains functionally correct. The grant for `platform.admin` simply satisfies every capability evaluation immediately. The change is that it is now:
- Auditable (the grant check is logged)
- Revocable (remove the grant, not redeploy the binary)
- Observable (the introspect endpoint can show who holds it)

### 1.5 Subject.OrgUnitPath and Subject.RiskScore — Premature

**Verdict: RiskScore out. OrgUnitPath is additive but should not be in Subject.**

`RiskScore: float` on Subject is Zero Trust infrastructure. Correct concept, wrong version. Zero Trust evaluation requires device posture, continuous authentication signals, anomaly detection — none of which Awo provides at v1.0. Including a stub field creates false confidence.

`OrgUnitPath: []uuid` is useful but belongs in the resolved `AuthorizationContext` built during grant evaluation, not in Subject. Subject is identity. Org membership is authorization context that is resolved at evaluation time from the grants database.

### 1.6 ViewerContext — God Object Problem

**Verdict: Correct concept, wrong design. Simplify the public interface.**

The proposed ViewerContext exposes:
- Subject (identity)
- EffectiveGrants (cached pre-resolved grants)
- OrgMemberships (organizational position)
- Evaluator (the engine)

This is too much. Business module code that receives ViewerContext should not be able to inspect raw grants or memberships — that would couple business logic to IAM internals. The ViewerContext's public API surface should be minimal and stable:

```
ViewerContext {
    // These are the only methods business module code should call:
    CanDo(capability string) bool
    CanDoOnResource(capability string, resource Resource) bool
    RecordFilter(entityName string) Filter
    VisibleFields(entityName string) []string   // v2, all fields at v1.0

    // Identity facts — stable and safe to read:
    PrincipalID() uuid
    TenantID() uuid
    RequestID() string
}
```

The evaluator, grants cache, org memberships are internal implementation details. They must not be accessible to business module code.

### 1.7 RecordRule.PredicateType as Enum — Wrong

**Verdict: The enum constrains future predicates. Use a declarative DSL instead.**

```
PredicateType: OwnerOnly | OrgScoped | AttributeMatch | Custom
```

This enumerates all permitted predicate shapes at design time. Adding a new predicate pattern requires a framework change. Instead, the framework should define a Filter DSL and RecordRules express predicates in that DSL. The DSL can evolve (additive new operators) without breaking existing rules.

The correct model: `SystemRecordFilter` declares a standard DSL predicate compiled into the manifest. Tenant-configured `RecordRule` entries also express DSL predicates, constrained to fields and operators the entity schema permits.

### 1.8 PolicyFunc — Should Become Declarative

**Verdict: Deprecated, not removed. Must be converted to metadata form.**

`PolicyFunc` is a Go closure — uncheckable, unintrospectable, untestable in isolation, and unconfigurable by tenants. It cannot be turned off by a tenant admin. It cannot be combined with tenant-added rules in a controlled way.

The correct model is a `SystemRecordFilter` declared in the `CapabilitySet` as a pure data structure expressing what predicates apply. The framework's RecordRuleEngine evaluates it alongside tenant-configured filters. `PolicyFunc` is retained as a compatibility shim but marked deprecated; modules should migrate to declarative form.

### 1.9 CasbinPolicy in CompiledSchema — Must Go

**Verdict: Remove before v1.0. Framework output must not name external libraries.**

`CasbinPolicy` in the compiler output is an architecture boundary violation. The compiler is a framework component; Casbin is an implementation detail of one possible policy engine. Naming Casbin types in the compiler output creates a permanent coupling that makes engine replacement require a compiler change.

Replace with `PolicyTuple` — same data, engine-neutral name. This is a rename-only change.

### 1.10 PermissionSet.Create/Read/Write/Delete []string — Must Go

**Verdict: Remove before v1.0. Role names in business module source code are wrong.**

The current `PermissionSet` with string slices like `["role:finance.accounts_payable"]` inside `internal/core/finance/` is the primary architectural problem. This is what the entire redesign is solving. Every role name embedded in a business module source file is a maintenance liability and a violation of the governing principle.

Replacement: `CapabilitySet` with no role references.

### 1.11 Grant.GrantConditions for ABAC — Defer

**Verdict: Correct concept, wrong version. Design the data model to accommodate it, do not implement it.**

ABAC conditions (grant me capability X only if invoice.total < 500000) require:
- A predicate language on grants
- Attribute resolution from the resource at evaluation time
- Evaluation logic in the PolicyEvaluator

This adds significant complexity to the evaluation pipeline at v1.0. The correct approach: design the `Grant` entity with a nullable `Conditions json` column that is ignored by the v1.0 evaluator. When ABAC is implemented in v2, the column is already there and the Grant API does not change.

### 1.12 Field Permissions via FieldDef.ReadSlot / WriteSlot — Defer

**Verdict: Correct direction, premature for v1.0. Sensitive flag is sufficient.**

`FieldDef.Sensitive: true` already handles the most common field-level access control requirement: "exclude from normal reads, log, and API responses." Field-level RBAC (different roles see different fields) is a v2 capability. The `Sensitive` flag is the v1.0 answer. The design must not break in v2 when field slots are introduced — leaving room for `FieldDef.ReadCapability` and `FieldDef.WriteCapability` optional fields does not cost anything.

### 1.13 Break-Glass, Delegation, SoD, Policy Simulation — All v2

**Verdict: Correctly deferred. Do not let v2 requirements contaminate v1.0 model shape.**

These are all valuable. None are needed to ship a functional ERP authorization system. The v1.0 model must be designed so they can be added without breaking the core. Each one is additive if the foundation is correct.

### 1.14 AvailableWhen on ActionDef — Belongs Elsewhere

**Verdict: Wrong field. Business rule guard ≠ authorization guard.**

```
AvailableWhen: &def.RecordCondition{Field: "status", Operator: eq, Values: ["Draft"]}
```

Whether an action is available based on record state is a business rule, not an authorization check. An actor may be authorized to submit an invoice but the invoice is in the wrong state. The correct location for this is:

- SDUI: `ActionDef.AvailableCondition` — drives the UI button visibility (already supported by amis conditional rendering)
- Handler: The handler returns a `BusinessError` if the state guard fails
- Not authorization: The authorization system does not know about record state; it only knows about principal + capability + scope

This concept should remain on `ActionDef` but must be clearly labeled as a UI/business-rule hint, not an authorization predicate. Remove it from any authorization evaluation path.

---

## Part 2 — Separation Matrix

The governing principle requires complete separation: business modules know what exists (capabilities), IAM knows who has them. Here is where every concept must live and why.

### 2.1 Concepts Owned by the Compiler

| Concept | Location | Why |
|---|---|---|
| `Capability` | Compiler output (`CapabilityManifest`) | Derived from EntityDefinition + ActionDef; exists before any database |
| `CapabilityManifest` | Compiler output | Static catalog of all capabilities in the system |
| `RouteCapabilityMap` | Compiler output | Maps every route descriptor to its required capability |
| `SystemRecordFilters` | Compiler output | Declarative row-visibility rules compiled from CapabilitySet |
| `FieldSensitivityMap` | Compiler output | Which fields are Sensitive per entity |
| `PolicyTuple` | Compiler output (replaces CasbinPolicy) | Engine-neutral: (subject, object, action) |

The compiler knows nothing about users, roles, or grants. It knows about capabilities, routes, and metadata.

### 2.2 Concepts Owned by Business Modules

| Concept | Location | Why |
|---|---|---|
| `CapabilitySet` | Business module (EntityDefinition field) | Module declares what operations exist on its entities |
| `ActionDef.Capability` | Business module (ActionDef field) | Module declares what capability this action requires |
| `SystemRecordFilter declarations` | Business module (CapabilitySet field) | Module declares what row visibility rules apply structurally |
| Hook logic (BeforeCreate, etc.) | Business module | Business rules about data, not about people |
| Handler logic (ActionHandlerFunc) | Business module | Business operations, calls `ViewerContext.CanDo()` for secondary checks |

Business modules know only: what capabilities exist on their entities, and what the default row-visibility predicates are. They do not know what roles exist, who holds grants, or how grants are evaluated.

### 2.3 Concepts Owned by the IAM Module

| Concept | Location | Why |
|---|---|---|
| `iam_role` | IAM module (platform entity) | Named bundle of capabilities — an IAM, not business, concept |
| `iam_role_capability` | IAM module (platform entity) | Which capabilities a role includes — pure IAM |
| `iam_grant` | IAM module (platform entity) | Principal → Capability binding — pure IAM |
| `iam_deny` | IAM module (platform entity) | Explicit deny override — pure IAM |
| `iam_principal_role` | IAM module (platform entity) | User assigned to role — pure IAM |
| `iam_record_rule` | IAM module (platform entity) | Tenant-configurable row-visibility predicates — IAM config |
| `iam_service_account` | IAM module (platform entity) | Machine identity — IAM identity |
| `iam_api_token` | IAM module (platform entity) | Credential bound to service account — IAM credential |
| `iam_identity_provider` | IAM module (platform entity) | External IDP config — IAM identity federation |
| `iam_idp_claim_mapping` | IAM module (platform entity) | IDP claim → internal role mapping — IAM concern |
| `iam_session` (Redis) | IAM module | Ephemeral auth state — IAM auth concern |
| `PolicyEvaluator` (runtime) | IAM module | Evaluation engine — IAM decides how to evaluate |
| `SubjectResolver` (runtime) | IAM module | Builds request identity from session — IAM auth concern |
| `BootstrapConfig` | IAM module | Default role-capability assignments for new tenants — IAM bootstrap |

The IAM module owns everything about who has what capability. Business modules own nothing in this list.

### 2.4 Concepts Owned by the Platform Organization Module

| Concept | Location | Why |
|---|---|---|
| `platform_org_unit` | Platform module (organization entity) | Org tree structure — organizational, not IAM |
| `iam_principal_org_unit` | IAM module | Principal's membership in an org unit — IAM uses org data |
| `OrgScope` on grants | IAM module | Org-scoped grant restriction — IAM concern |

Org hierarchy is a platform-level concept. IAM references it but does not own it. Business entities reference `org_unit_id` as a Link field but do not know what org units mean for authorization.

### 2.5 Concepts Owned by the Framework Runtime

| Concept | Location | Why |
|---|---|---|
| `ViewerContext` (interface) | Framework runtime | Stable API surface — what business modules receive |
| `Principal` (struct) | Framework runtime | Base identity type used across framework |
| `Resource` (struct) | Framework runtime | What is being operated on — route layer constructs |
| `AuthzRequest` (struct) | Framework runtime | Input to PolicyEvaluator — framework constructs |
| `AuthzDecision` (struct) | Framework runtime | Output of PolicyEvaluator — framework consumes |
| `Filter` (interface) | Framework runtime | Predicate DSL used by RecordRuleEngine and PolicyFunc |
| `RecordRuleEngine` | Framework runtime | Compiles RecordRules + SystemRecordFilters into Filter predicates |
| `FieldVisibilityFilter` | Framework runtime | Strips Sensitive fields — framework responsibility |
| `AuditEmitter` | Framework runtime | Records every authorization decision |

The framework runtime owns the contracts between layers. The IAM module plugs in via those contracts.

### 2.6 Concepts Owned by the Bootstrap Process

| Concept | Location | Why |
|---|---|---|
| Default role definitions | IAM module `bootstrap.go` | What roles exist at tenant creation — IAM configuration |
| Default role-capability grants | IAM module `bootstrap.go` | What each role can do by default — IAM configuration |
| Default org unit seeds | Platform module `bootstrap.go` | Every tenant starts with a root org unit |

**This is the critical correction from the previous proposal.** DefaultGrants are NOT declared in Finance or CRM. They live in `internal/platform/iam/bootstrap.go`. The IAM module reads the `CapabilityManifest` at startup and seeds grants for system roles. When a new module is added, the IAM module's bootstrap config is updated — not the module's own EntityDefinition.

### 2.7 Concepts Owned by Administrator Configuration

| Concept | Location | Why |
|---|---|---|
| Custom role definitions | Tenant database (runtime) | Tenant admins create roles — no code change |
| Custom role-capability grants | Tenant database (runtime) | Tenant admins assign capabilities to roles — no code change |
| Tenant-specific record rules | Tenant database (runtime) | Tenant admins add row visibility constraints — no code change |
| User-to-role assignments | Tenant database (runtime) | IAM operation — no code change |
| Org unit structure | Tenant database (runtime) | Organizational configuration — no code change |

### 2.8 The Impossible Matrix Row

The following should be **impossible** in the final architecture:

> A business module Go source file containing a role name, a user ID, a grant ID, or any reference to the IAM module's database entities.

If the Finance module contains `"role:finance.accounts_payable"` anywhere in its source files after the migration, the architecture has not been implemented correctly.

---

## Part 3 — The Canonical Permission Model

Starting from first principles: what is the minimum set of concepts needed to answer "is principal P allowed to do operation O on resource R?"

### 3.1 The Five Required Primitives

**1. Capability** — What operations exist in the system.
Compiled from EntityDefinition and ActionDef. Static. Not in the database. Cannot be changed at runtime.

```
Capability {
    ID:          string     // "finance.invoice.submit"
    Module:      string     // "finance"
    Entity:      string     // "finance_invoice"
    Operation:   string     // "submit"
    Label:       string     // "Submit Invoice"
    Description: string
}
```

The ID is always `{module}.{entity}.{operation}`. Standard CRUD operations are auto-generated (`{module}.{entity}.read`, `.write`, `.create`, `.delete`). Custom actions declare their capability via `ActionDef.Capability`. The compiler registers all capabilities into the `CapabilityManifest`.

**2. Principal** — Who is performing the operation.
Constructed by the authentication layer. Two concrete types: User and ServiceAccount. Platform admin is NOT a third type — it is a User principal holding a specific grant.

```
Principal {
    ID:           uuid
    Type:         User | ServiceAccount
    TenantID:     uuid
    SessionID:    string      // for audit trail
}
```

**3. Grant** — Principal (or Role) holds a Capability.
The core authorization fact. Stored in the IAM database. Evaluated at request time.

```
Grant {
    ID:              uuid
    TenantID:        uuid
    GranteeType:     Role | Principal
    GranteeID:       uuid
    CapabilityID:    string            // "finance.invoice.submit"
    OrgConstraint:   []uuid            // empty = global within tenant
    OrgInclusive:    bool              // true = subtree, false = exact units only
    ValidFrom:       *time.Time
    ValidUntil:      *time.Time
    Conditions:      json              // null at v1.0; ABAC extension point
    GrantedBy:       uuid
    CreatedAt:       time.Time
}
```

**4. Deny** — Explicit override that wins over all grants.
Same shape as Grant. Evaluated before grants. Any matching Deny → Denied immediately.

```
Deny {
    ID:              uuid
    TenantID:        uuid
    DeniedType:      Role | Principal
    DeniedID:        uuid
    CapabilityID:    string
    OrgConstraint:   []uuid
    ValidUntil:      *time.Time
    Reason:          string
    CreatedBy:       uuid
}
```

**5. RecordFilter** — Which records a principal can see.
Two kinds: SystemRecordFilters (compiled from CapabilitySet, immutable) and TenantRecordRules (tenant-configured, stored in IAM database). Both expressed in the same Filter DSL. Combined with AND semantics.

```
SystemRecordFilter {
    EntityName:     string          // "finance_invoice"
    CapabilityID:   string          // "finance.invoice.read"
    Predicate:      FilterDSL       // e.g. OrgScopedFilter("org_unit_id")
    // Compiled into the CapabilityManifest. Cannot be changed at runtime.
}

TenantRecordRule {
    ID:              uuid
    TenantID:        uuid
    EntityName:      string
    CapabilityID:    string
    Predicate:       FilterDSL
    ForRoles:        []uuid         // empty = applies to all principals
    IsSystemSeeded:  bool           // true if seeded by IAM bootstrap
    CreatedBy:       uuid
}
```

### 3.2 Concepts That Can Be Removed

**`PermissionSet`** — Remove. Replace with `CapabilitySet` that contains no role names.

**`CasbinPolicy`** — Rename to `PolicyTuple`. Remove the Casbin name from the framework.

**`PermissionNamespace`** — This is already `QualifiedName`. One of them is redundant. `PermissionNamespace` is removed; `QualifiedName` is the canonical identity.

**`PermissionDeclaration`** — Name is misleading (sounds like the policy, not the metadata). Replace with `CapabilitySet` declared in EntityDefinition.

**`RoleRegistry`** — Roles are IAM database entities. There is no compile-time role registry. The compiler validates capability names but not role names (roles do not exist at compile time).

**`PermissionManifest`** — Renamed to `CapabilityManifest`. Same concept, cleaner name.

**`DefaultGrants` on EntityDefinition** — Removed entirely. Lives in IAM bootstrap.

**`Actor` struct** — Renamed to `Principal`. The term "Actor" is domain-ambiguous (actors in Temporal, actors in theater). `Principal` is the established security term.

**`Actor.IsPlatformAdmin bool`** — Removed. Platform admin is a grant, not a flag.

### 3.3 Concepts That Are Merged

**`PolicyTuple` (renamed from `CasbinPolicy`) + the eventual PolicyEvaluator input** — These should not be separate concepts. The compiler emits `CapabilityManifest` (not tuples). The evaluator reads from the manifest and the IAM database. No intermediate `PolicyTuple` type needs to exist in the framework.

**`PermissionSet.Policy PolicyFunc` + `SystemRecordFilter`** — PolicyFunc is the legacy form; SystemRecordFilter is the canonical form. They express the same concept. At v1.0, both can coexist: if a `PolicyFunc` is declared, it is wrapped by the framework into an equivalent filter predicate and evaluated alongside SystemRecordFilters. The deprecation path is clear.

### 3.4 The Final Model Shape

```
Framework layer:        Capability (static, compiled)
                             │
                    CapabilityManifest
                    SystemRecordFilters

IAM layer:          Role ──── Grant ──→ Capability
                    │         │
                Principal    Deny ──→ Capability
                    │
                TenantRecordRule ──→ Filter DSL

Runtime layer:      PolicyEvaluator (interface, replaceable)
                    RecordRuleEngine (compiles filters)
                    ViewerContext (stable public API)
```

Five primitives. Three layers. No business module references anything below the `Capability` line.

---

## Part 4 — Canonical IAM Domain Model

### 4.1 Identity Entities

**`iam_user`** (System Entity — mandatory)
- Responsibility: Human principal within a tenant
- Ownership: Framework-managed (schema fixed), tenant-populated (data)
- Lifecycle: Created on invite → Active → Suspended → Deleted (soft)
- Relationships: Has many principal_roles; belongs to many org_units

**`iam_service_account`** (System Entity)
- Responsibility: Machine identity for API integrations
- Ownership: Tenant-managed
- Lifecycle: Created by admin → Active → Deactivated → Deleted
- Relationships: Has many api_tokens; has grants directly (or via role)
- Key constraint: Service account grants are scoped by explicit capability list, cannot exceed what a human could grant it

**`iam_api_token`** (System Entity)
- Responsibility: Credential bound to a service account
- Ownership: Tenant-managed
- Lifecycle: Created → Used → Expired/Revoked
- Relationships: Belongs to service account; carries scope list (subset of service account grants)
- Key constraint: Token scope cannot exceed service account grants; rotation creates new token before revoking old

**`iam_identity_provider`** (System Entity)
- Responsibility: External IDP configuration (OIDC, SAML)
- Ownership: Tenant-managed (within platform-defined bounds)
- Lifecycle: Created by admin → Active → Disabled
- Relationships: Has many idp_claim_mappings

**`iam_idp_claim_mapping`** (System Entity)
- Responsibility: Maps an IDP claim value to an internal role
- Ownership: Tenant-managed
- Lifecycle: Created by admin → Active → Deleted
- Key constraint: Can only map to roles that exist; claim_value is a string match against IDP token claims

### 4.2 Role Management Entities

**`iam_role`** (System Entity)
- Responsibility: Named bundle of capability grants
- Ownership: Platform-seeded system roles (cannot be deleted); tenant-created custom roles
- Lifecycle: Seeded at bootstrap (system roles) or created by admin (custom roles) → Active → (Archived — custom roles only)
- Relationships: Has many role_capabilities; can inherit from parent roles
- Key fields: `name` (stable identifier), `is_system` (seeded by platform, protects from deletion), `inherits_from` (parent role IDs for hierarchy)
- Key constraint: Custom role capabilities cannot exceed the granting admin's own grants (privilege escalation prevention)

**`iam_role_capability`** (System Entity)
- Responsibility: Grants a capability to a role (optionally with org scope)
- Ownership: Platform-seeded (system roles); tenant-managed (custom roles)
- Lifecycle: Created with role → Active → Revoked
- Relationships: Belongs to role; references a capability ID from the CapabilityManifest
- Key fields: `role_id`, `capability_id`, `org_constraint` (json), `conditions` (json, null at v1.0)
- Key constraint: `capability_id` must exist in the compiled CapabilityManifest (validated at assignment time, not compile time)

**`iam_principal_role`** (System Entity)
- Responsibility: Assigns a principal to a role
- Ownership: Tenant-managed
- Lifecycle: Created by admin → Active → Revoked
- Relationships: Principal (user or service account) + Role + optional OrgUnit scope + temporal bounds
- Key fields: `principal_id`, `principal_type`, `role_id`, `scoped_to_org_unit_id`, `valid_from`, `valid_until`

### 4.3 Grant Entities

**`iam_grant`** (System Entity)
- Responsibility: Direct principal-to-capability binding, bypassing role assignment
- Ownership: Tenant-managed (used for exceptional one-off grants)
- Lifecycle: Created → Active → Expired/Revoked
- Relationships: Principal + Capability + OrgConstraint
- Note: Roles are the preferred mechanism. Direct grants exist for one-time exceptional permissions.
- Key fields: same shape as `iam_role_capability` but bound to a principal directly

**`iam_deny`** (System Entity)
- Responsibility: Explicit deny override — prevents a principal or role from using a capability even if granted
- Ownership: Tenant-managed; platform-managed (for platform-wide blocks)
- Lifecycle: Created → Active → Expired/Deleted
- Key constraint: A deny for role X denies all users in role X. A deny for user Y denies only Y.

### 4.4 Record Visibility Entities

**`iam_record_rule`** (System Entity)
- Responsibility: Tenant-configurable row-visibility predicate
- Ownership: Platform-seeded (system rules from CapabilitySet declarations); tenant-managed (additional rules)
- Lifecycle: Seeded at bootstrap → Active → (Tenant rules can be deleted)
- Key fields: `entity_name`, `capability_id`, `predicate` (Filter DSL, stored as json), `applies_to_role_ids` (empty = all principals), `is_system_seeded`
- Key constraint: System-seeded rules cannot be deleted (they are framework invariants). Tenant rules are additive (AND-composed with system rules).

### 4.5 Organizational Entities

**`platform_org_unit`** (System Entity — platform module, not IAM)
- Responsibility: A node in the organizational hierarchy tree
- Ownership: Tenant-managed
- Lifecycle: Created by admin → Active → (Archived — cannot delete if children or members)
- Relationships: Has parent (nullable — null = root); has many children; has many principal memberships
- Key fields: `name`, `parent_id` (self-referencing FK), `path` (materialized path for fast subtree queries: e.g. `/root/nairobi/westlands/`)

**`iam_principal_org_membership`** (System Entity — IAM module)
- Responsibility: Principal belongs to an org unit
- Ownership: Tenant-managed
- Lifecycle: Created by admin → Active → Removed
- Key fields: `principal_id`, `org_unit_id`, `is_primary` (one primary per principal)
- Note: Principals can belong to multiple org units; primary determines the default scope

### 4.6 Session and Audit Entities

**Sessions** — Not a PostgreSQL entity. Stored in Redis as `session:{token}` with TTL. Contains: PrincipalID, TenantID, authenticated-at, MFA-verified bool, session metadata. The database is not the source of truth for sessions; Redis is.

**`platform_audit_event`** (System Entity — platform module)
- Responsibility: Immutable record of every authorization decision and data mutation
- Ownership: Framework-written, never modified
- Lifecycle: Append-only; archived after retention period (configurable per tenant)
- Key fields: `request_id`, `principal_id`, `tenant_id`, `event_type`, `capability_id` (for authz events), `entity_name`, `record_id`, `decision` (allow/deny), `reason`, `matched_grant_id`, `matched_deny_id`, `timestamp`
- Key constraint: No UPDATE or DELETE permitted on this table; RLS allows only INSERT and SELECT

### 4.7 What Is Intentionally Absent

**`iam_delegation`** — Deferred to v2. Modeled as: a Temporal workflow that validates the delegator's grants, creates time-bounded direct grants for the delegatee, and records the delegation in audit.

**SoD enforcement entities** — Deferred to v2. When introduced, expressed as `iam_role_constraint` with type=SoD and conflicting role pairs.

**Break-glass entities** — Deferred to v2. Modeled as: a Temporal workflow (elevated-access-request) that creates a time-bounded Grant after approval, tagged in audit as break-glass.

**`iam_permission_group`** — Not needed. Roles serve this purpose. Permission groups are a Frappe-specific concept that duplicates what roles already provide.

---

## Part 5 — Architectural Boundaries

Each boundary is a contract. Nothing crosses it without going through the contract.

### 5.1 The Boundaries

```
┌─────────────────────────────────────────────────────┐
│  BUSINESS MODULES                                    │
│  Finance, CRM, Inventory, HR                         │
│                                                      │
│  Knows: Capability names (own entities only)         │
│  Knows: SystemRecordFilter shapes                    │
│  Knows: ViewerContext public API                     │
│  Does not know: Roles, grants, users, org units     │
└──────────────────────┬──────────────────────────────┘
                       │ ViewerContext (CanDo, Filter)
                       │ ActionRuntime (Repo, Tx, Events)
┌──────────────────────▼──────────────────────────────┐
│  FRAMEWORK RUNTIME                                   │
│                                                      │
│  Constructs ViewerContext from authentication result │
│  Routes requests to handlers                         │
│  Injects RecordFilters into Repository calls         │
│  Emits audit events                                  │
│  Invokes PolicyEvaluator (via interface)             │
└──────────┬──────────────────────┬───────────────────┘
           │ PolicyEvaluator ifc  │ Repository ifc
┌──────────▼──────────┐ ┌────────▼───────────────────┐
│  IAM MODULE          │ │  REPOSITORY LAYER           │
│                      │ │                             │
│  Implements          │ │  Accepts Filter predicates  │
│  PolicyEvaluator     │ │  Executes SQL via pgx       │
│  Loads grants/denies │ │  Never knows about authz    │
│  Manages sessions    │ │  RLS enforces tenant bounds │
│  Manages roles       │ │                             │
└──────────────────────┘ └─────────────────────────────┘

┌─────────────────────────────────────────────────────┐
│  COMPILER                                            │
│                                                      │
│  Input: EntityDefinition + ActionDef from all mods  │
│  Output: CapabilityManifest, Routes, SystemFilters  │
│  Output: Fingerprint (for cache invalidation)        │
│  Does not know: users, roles, grants, sessions       │
└─────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────┐
│  SDUI (amis schemas)                                 │
│                                                      │
│  Consumes CapabilityManifest (which buttons to show)│
│  Consumes ViewerContext (which elements are visible)│
│  Does not perform authorization                      │
│  Does not know about grants or roles                 │
└─────────────────────────────────────────────────────┘
```

### 5.2 Authentication vs Authorization vs Identity

**Authentication:** "Who are you?" — validates credentials, returns Principal. Zero authorization knowledge.

**Identity:** "What do we know about you?" — enriches Principal with session facts. No capability evaluation.

**Authorization:** "Are you allowed?" — evaluates grants against the requested capability. Receives Principal + Capability + Resource. No credential knowledge.

These are three separate middleware stages. None of them is aware of the other's internals. They communicate through data structures, not method calls between packages.

### 5.3 Where Role Management Lives

Role management (create role, assign capability, assign user to role) is the IAM module's domain. It has no presence in:
- The compiler (roles do not exist at compile time)
- Business modules (Finance does not manage roles)
- The repository layer (no role tables visible to generic repository)
- The SDUI layer (SDUI generates forms from EntityDefinition; the role management UI is generated from IAM's own EntityDefinitions)

### 5.4 Organization vs IAM

The organization module (org units, org tree) is a platform module, not an IAM module. IAM references org units (principal memberships, grant scopes) but does not own the organizational hierarchy. This separation matters because:

- Org hierarchy can exist without IAM (for reporting, cost centers, document routing)
- IAM uses org hierarchy as a scope dimension but does not define what an org unit means
- Business modules reference `org_unit_id` as a Link field; they do not know this field has authorization significance

### 5.5 Duplication That Must Be Removed

| Duplicated Concern | Appears In | Correct Location |
|---|---|---|
| Role names | Business module PermissionSet | Nowhere in business modules |
| Casbin types | Compiler output | Compiler output only (as engine-neutral PolicyTuple) |
| Permission evaluation | Casbin + route middleware + hook pipeline | PolicyEvaluator only, invoked by middleware |
| "Platform admin" identity | Actor.IsPlatformAdmin bool | Grant of platform.admin capability |
| Row filtering | PolicyFunc (inline) + SystemRecordFilter (proposed) | SystemRecordFilter only (PolicyFunc deprecated) |

---

## Part 6 — Authorization Evaluation Pipeline

### 6.1 Stage-by-Stage Design

**Stage 0: Route Resolution (pre-request)**

- Input: Compiled routes table (from CapabilityManifest)
- Output: RouteDescriptor (entity, operation, required capability)
- Ownership: Framework, compiled at startup
- Cacheability: Permanent (until restart)
- This stage is complete before the request arrives; the compiler pre-computes it

---

**Stage 1: Authentication**

- Input: Credentials (session token, API key, IDP token)
- Output: `Principal` (ID, Type, TenantID, SessionID) or 401
- Ownership: IAM module (authentication service)
- Cacheability: Session → Principal mapping cached in Redis as the session record itself
- Extensibility: New auth methods are new session creation flows; Principal shape is stable
- Contract: Returns Principal or error. No capability knowledge.

---

**Stage 2: Tenant Validation**

- Input: Principal.TenantID + RouteDescriptor.TenantID (from X-Tenant-ID header)
- Output: Validated tenant context, or 402/503/410 per tenant status
- Ownership: Platform middleware
- Cacheability: Tenant status cached in Redis (short TTL, invalidated on status change)
- Contract: Tenant must be ACTIVE. Calls set_tenant_context() on DB connection → activates RLS.

---

**Stage 3: Identity Resolution**

- Input: Principal
- Output: `ResolvedIdentity` (direct role IDs, direct grant IDs, org unit memberships)
- Ownership: IAM module (identity resolution service)
- Cacheability: Per-session, cached in Redis with TTL matching session expiry. Invalidated when role assignments change.
- Query: Load from `iam_principal_role`, `iam_grant`, `iam_principal_org_membership` filtered by principal
- Contract: Returns structured identity facts. No capability evaluation here.

---

**Stage 4: ViewerContext Construction**

- Input: Principal + ResolvedIdentity + RouteDescriptor
- Output: `ViewerContext` (opaque to business modules; carries evaluator binding)
- Ownership: Framework runtime
- Cacheability: Per-request only — constructed from cached identity, not cached itself
- Design: ViewerContext is constructed once per request and passed via context.Context
- Contract: Exposes `CanDo()`, `CanDoOnResource()`, `RecordFilter()`, `VisibleFields()`. Nothing else.

---

**Stage 5: Operation Permission Check**

- Input: ViewerContext + required Capability (from RouteDescriptor) + Resource (or nil for list/create)
- Output: Allow or Deny
- Ownership: PolicyEvaluator (IAM module implements, framework invokes via interface)
- Cacheability: Per-capability per-principal evaluations can be cached in Redis (key: tenant+principal+capability) with very short TTL (30s–60s) — covers burst requests but reflects grant changes quickly
- Evaluation order: Deny rules → Grants (direct + role-inherited) → OrgScope → Temporal validity → first-match Allow or No-match Deny
- Contract: `PolicyEvaluator.Evaluate(ctx, AuthzRequest) AuthzDecision`. Engine-replaceable.

---

**Stage 6: Record Visibility Filter Injection**

- Input: ViewerContext + EntityName + (optional) RecordID
- Output: `Filter` predicate (injected into repository query)
- Ownership: RecordRuleEngine (framework runtime)
- Evaluation: Compose SystemRecordFilters (from CapabilityManifest) + matching TenantRecordRules (from IAM database, keyed by entity+capability+principal roles)
- Cacheability: TenantRecordRules cached in Redis per tenant+entity, invalidated when rules change
- Contract: Returns a `Filter` value that is AND-composed with the repository's base filter. Repository knows nothing about why these predicates exist.

---

**Stage 7: Repository Execution**

- Input: Query parameters + Filter from Stage 6
- Output: Records (bounded by both RLS and RecordFilters)
- Ownership: Repository layer (blind to authorization)
- The repository receives predicates and executes them. It does not know they came from an authorization system.
- RLS provides the tenant isolation floor. RecordFilter provides the role/org visibility ceiling.

---

**Stage 8: Hook Pipeline**

- Input: EntityRecord + ViewerContext (injected via context.Context)
- Output: Modified EntityRecord or error
- Ownership: Business module hook implementations
- Hooks may call `viewer.CanDo()` for secondary checks within business logic (e.g. "does this actor have the journal.create capability before posting?")
- The authorization system is accessible via ViewerContext; hooks do not call PolicyEvaluator directly.

---

**Stage 9: Field Visibility**

- Input: EntityRecord fields + ViewerContext + EntitySchema.SensitiveFields
- Output: Sanitized response (Sensitive fields stripped if not authorized)
- Ownership: Framework runtime (response serializer)
- At v1.0: `Sensitive: true` fields are excluded from all responses unless the principal holds the specific read_sensitive capability for this entity
- At v2.0: Full field-level capability checks per FieldDef.ReadCapability
- Contract: Repository records are never returned directly to API clients; they always pass through field visibility filter.

---

**Stage 10: Audit Emission**

- Input: AuthzDecision + Principal + Capability + Resource + RequestID
- Output: `platform_audit_event` record (async, non-blocking)
- Ownership: Framework runtime (AuditEmitter)
- Every Deny is audited immediately. Every Allow is audited asynchronously.
- Audit records are immutable; no UPDATE/DELETE permitted at DB level via constraint.

---

### 6.2 The Contracts Between Stages

The stages communicate through four data structures:

```
Principal:         ID, Type, TenantID, SessionID
ResolvedIdentity:  DirectRoles, DirectGrants, OrgMemberships
AuthzRequest:      Principal, CapabilityID, Resource (optional)
AuthzDecision:     Allowed, Reason, MatchedGrantID, MatchedDenyID
```

These four are the seams between authentication, identity, and authorization. The PolicyEvaluator receives only `AuthzRequest` and returns only `AuthzDecision`. It knows nothing about sessions, HTTP requests, or business logic.

---

## Part 7 — Future Evolution

### 7.1 Enterprise RBAC

**Requirement:** Role inheritance, role cardinality, temporal roles, org-scoped roles.

**Impact:** Additive. Role inheritance (`iam_role.inherits_from`) is already in the IAM domain model. Cardinality and temporal constraints are new fields on `iam_role` and `iam_principal_role`. No framework changes.

### 7.2 ABAC (Attribute-Based Access Control)

**Requirement:** Grants conditioned on resource attribute values (`invoice.total < 500000`).

**Impact:** Additive. The `Grant.conditions json` column is already nullable in the proposed model. The PolicyEvaluator needs to evaluate this column against the resolved resource attributes. No framework contract changes. Only the evaluator implementation grows.

**Design note:** The resource attributes are provided via `AuthzRequest.Resource` which already carries a field-value map. The evaluator evaluates conditions against this map. The business module does not change.

### 7.3 ReBAC (Zanzibar-style Relationship-Based)

**Requirement:** "User can edit document if they are the owner, OR if they are a member of the document's project team."

**Impact:** Engine replacement, not breaking. The `PolicyEvaluator` interface is replaced with a Zanzibar-compatible engine. Grants become relationship tuples. The `AuthzRequest` and `AuthzDecision` contracts do not change. Business modules do not change. ViewerContext API does not change.

**Design note:** This is why the PolicyEvaluator interface must be designed from day one as a replaceable component with a stable external contract.

### 7.4 Zero Trust

**Requirement:** Authorization factors in device posture, session freshness, risk signals, continuous verification.

**Impact:** Additive to `AuthzRequest`. A `SessionContext` field (MFA-verified, auth-method, session-age, risk-score) is added to `AuthzRequest`. The evaluator can use these factors in grant conditions. Existing grants without conditions are unaffected. Business modules do not change.

**Design note:** The `ResolvedIdentity` → `AuthzRequest` construction (Stage 5) picks up session context from the session Redis record and passes it through. No business module code changes.

### 7.5 Just-in-Time Access / Temporary Elevation

**Requirement:** User requests a capability, an approver grants it for N hours, it auto-expires.

**Impact:** Additive. JIT access is a Temporal workflow that:
1. Creates a time-bounded `iam_grant` record after approval
2. Invalidates the grants cache for that principal
3. Records the lifecycle in `platform_audit_event`

No framework changes. The grant model already supports `valid_until`. The JIT workflow is new IAM module code.

### 7.6 Delegation

**Requirement:** User A grants a subset of their capabilities to User B for a period.

**Impact:** Additive. A Temporal workflow validates: does A hold the grants they are delegating? If yes, creates time-bounded grants for B, records the delegation in audit. The grant model does not change.

**Design note:** The constraint "cannot delegate more than you have" requires the delegation workflow to call PolicyEvaluator for each delegated capability. This is IAM workflow code.

### 7.7 Policy Simulation

**Requirement:** "Would actor A be allowed to do X on resource Y, and why?"

**Impact:** Additive. `PolicyEvaluator.EvaluateWithExplanation(AuthzRequest) (AuthzDecision, Explanation)` is a second interface method. Existing callers use `Evaluate()`. The simulation endpoint calls `EvaluateWithExplanation()`. No existing code changes.

### 7.8 Multi-Company Hierarchy

**Requirement:** Group company with subsidiaries — CFO of parent can see all subsidiary invoices.

**Impact:** Additive to org module. `platform_org_unit` already supports a tree structure. A "group company" is a root org unit whose subtree contains subsidiary org units. Cross-company grants are `iam_role_capability` entries with `org_constraint` spanning multiple root nodes. No framework changes.

### 7.9 Cross-Tenant Platform Administration

**Requirement:** Platform admin can operate across tenant boundaries.

**Impact:** Additive. Platform admin principal holds grants for `platform.admin` capability. The evaluator recognizes this capability as cross-tenant (hardcoded in the evaluator). No business module changes.

**Design note:** Platform admin scope is always logged as a cross-tenant access event in `platform_audit_event`.

### 7.10 External Identity Providers

**Requirement:** User logs in via Okta; their Okta groups map to Awo roles.

**Impact:** Additive to authentication stage only. The IDP token is verified, the IDP claim values are extracted, `iam_idp_claim_mapping` maps them to role IDs, and a normal `ResolvedIdentity` is constructed. The authorization pipeline from Stage 4 onward does not know or care whether the identity came from password login or IDP.

### 7.11 Machine Identities and Service Accounts

**Requirement:** External billing system calls the invoice API with an API token.

**Impact:** Already in the model. ServiceAccountPrincipal + API token scope. The token scope constrains which capabilities are visible to the evaluator. No framework changes.

### 7.12 Offline Authorization

**Requirement:** Edge deployment without live connection to the authorization database.

**Impact:** Additive. The `CapabilityManifest` is already a static compiled artifact. A "manifest export" command can generate a distributable grant snapshot for a given tenant at a point in time. An offline evaluator reads this snapshot instead of the database. No framework contract changes; only a new evaluator implementation.

### 7.13 Distributed Authorization

**Requirement:** Authorization evaluated by a separate service (OPA server, Cedar agent, external policy engine).

**Impact:** Additive. A new `PolicyEvaluator` implementation that makes HTTP/gRPC calls to an external engine. The `AuthzRequest` is serialized and sent; `AuthzDecision` is deserialized from the response. No business module or framework changes.

---

## Part 8 — Final Canonical Architecture

### 8.1 The Canonical Authorization Architecture

Authorization in Awo is organized around one invariant:

> **Capabilities are framework facts. Grants are runtime data. The evaluator is a replaceable bridge.**

**Framework facts** (produced by the compiler, static):
- What capabilities exist (`CapabilityManifest`)
- What row-visibility rules apply structurally (`SystemRecordFilters`)
- What routes require what capabilities (`RouteCapabilityMap`)

**Runtime data** (in the database, dynamic):
- What roles exist and what capabilities they include (`iam_role`, `iam_role_capability`)
- What principals have what roles (`iam_principal_role`)
- What principals have direct grants (`iam_grant`)
- What denies are active (`iam_deny`)
- What tenant record rules apply (`iam_record_rule`)
- What org units exist and who belongs to them (`platform_org_unit`, `iam_principal_org_membership`)

**The evaluator bridge** (IAM module implementation, replaceable):
- Receives `AuthzRequest`: Principal + Capability + optional Resource
- Loads grants from runtime data (cached)
- Evaluates deny rules, grant scope, temporal validity
- Returns `AuthzDecision`: Allow/Deny + reason + matched rule IDs

This three-part structure ensures:
- Business modules only touch framework facts (capabilities)
- The evaluation engine is replaceable without touching business modules or the compiler
- All authorization decisions are auditable because they all flow through the evaluator

### 8.2 The Canonical Permission Model

Five entities. No more. No less.

```
1. Capability       — what operation exists (compiler-generated)
2. Principal        — who is acting (authentication-generated)
3. Grant            — principal/role holds capability with optional scope (IAM database)
4. Deny             — explicit override, always wins (IAM database)
5. RecordFilter     — row visibility predicate (compiler + IAM database)
```

Roles are bundles of Grants. Org units are scope dimensions on Grants. Sessions are ephemeral principal credentials. Everything else is implementation.

### 8.3 The Canonical IAM Domain Model

**Identity Entities:**
- `iam_user` — human principal
- `iam_service_account` — machine principal
- `iam_api_token` — scoped credential for service accounts

**Role Entities:**
- `iam_role` — named capability bundle (system or custom)
- `iam_role_capability` — capability entry in a role
- `iam_principal_role` — principal assigned to role

**Grant Entities:**
- `iam_grant` — direct principal-to-capability binding
- `iam_deny` — explicit deny override

**Visibility Entities:**
- `iam_record_rule` — tenant-configurable row predicate

**Organization Entities:**
- `platform_org_unit` — tree node (platform module, not IAM)
- `iam_principal_org_membership` — principal in org unit

**IDP Entities:**
- `iam_identity_provider` — OIDC/SAML config
- `iam_idp_claim_mapping` — IDP claim → role mapping

**Audit:**
- `platform_audit_event` — immutable decision log

**Bootstrap:**
- IAM module `bootstrap.go` — default roles and default role-capability grants for new tenants

Intentionally absent at v1.0: delegation, SoD constraints, break-glass workflow entities, field-level permission entities.

### 8.4 The Canonical Runtime Evaluation Pipeline

```
Stage 1: AUTHENTICATE
         Credentials → Principal | 401
         Ownership: IAM auth service
         Cache: session → principal in Redis

Stage 2: VALIDATE TENANT
         Principal.TenantID → ACTIVE tenant | 402/503/410
         Ownership: Platform middleware
         Side effect: set_tenant_context() activates RLS

Stage 3: RESOLVE IDENTITY
         Principal → ResolvedIdentity (roles, direct grants, org memberships)
         Ownership: IAM identity service
         Cache: per session in Redis

Stage 4: CONSTRUCT VIEWER
         Principal + ResolvedIdentity → ViewerContext (opaque)
         Ownership: Framework runtime

Stage 5: CHECK OPERATION PERMISSION
         ViewerContext + Capability → Allow | Deny
         Ownership: PolicyEvaluator (IAM implements, framework invokes)
         Cache: per principal+capability pair, 30–60s TTL

Stage 6: INJECT RECORD FILTER
         ViewerContext + EntityName → Filter predicate
         Ownership: RecordRuleEngine (framework)
         Applied: at repository query construction

Stage 7: EXECUTE REPOSITORY
         Query + Filter → Records
         Ownership: Repository layer
         RLS provides tenant floor; RecordFilter provides role ceiling

Stage 8: EXECUTE HOOKS
         EntityRecord + ViewerContext → EntityRecord | error
         Ownership: Business module (hook implementations)
         Secondary CanDo() calls go through ViewerContext

Stage 9: FILTER FIELDS
         EntityRecord + SensitiveFields → sanitized record
         Ownership: Framework runtime (response serializer)

Stage 10: EMIT AUDIT
          AuthzDecision + context → platform_audit_event
          Ownership: Framework runtime (async, non-blocking)
```

### 8.5 Concepts That Must Be Deleted Before v1.0

| Concept | Action | Replacement |
|---|---|---|
| `def.PermissionSet` (string slices) | Remove | `def.CapabilitySet` (no role names) |
| `def.Actor` | Rename | `def.Principal` |
| `Actor.IsPlatformAdmin bool` | Remove | Grant of `platform.admin` capability |
| `ActionDef.Permission string` | Replace | `ActionDef.Capability string` |
| `compiler.CasbinPolicy` type | Rename | `compiler.PolicyTuple` |
| Role name strings in `internal/core/finance/` | Remove | No role names in business modules |
| `PermissionSet.Policy PolicyFunc` | Deprecate (keep as shim) | `CapabilitySet.RecordFilters []SystemRecordFilter` |
| `DefaultGrants` in EntityDefinition | Remove | IAM module `bootstrap.go` |
| Introspect endpoint without auth gate | Add gate | Require `platform.introspect.read` capability |
| SDUI generator ignoring permissions | Fix | Pass ViewerContext; filter by CanDo() |

### 8.6 Concepts Intentionally Deferred Until Post-v1.0

| Concept | Version | Notes |
|---|---|---|
| ABAC grant conditions | v1.2 | `Grant.conditions json` null at v1.0; evaluated when non-null |
| Field-level capabilities | v1.3 | `FieldDef.ReadCapability`, `FieldDef.WriteCapability` optional |
| SoD constraint enforcement | v1.4 | `iam_role_constraint` entity with SoD type |
| Policy simulation endpoint | v1.5 | Requires `EvaluateWithExplanation` on PolicyEvaluator |
| JIT access workflow | v1.5 | Temporal workflow creating time-bounded grants |
| Delegation | v2.0 | Temporal workflow; requires privilege escalation guard |
| Break-glass elevation | v2.0 | Temporal workflow with approval requirement |
| External IdP integration | v1.2 | Authentication-layer only; no core model changes |
| Offline authorization | v2.0 | Manifest export format + offline evaluator implementation |
| ReBAC graph evaluation | v2.0 | Engine replacement; no contract changes if evaluator interface is stable |
| Zero Trust session factors | v1.3 | Add `SessionContext` to `AuthzRequest`; existing grants unaffected |

### 8.7 Final Architectural Scores

Scores applied to the **canonical architecture proposed in this document**, not the previous review's proposal.

| Dimension | Score | Reasoning |
|---|---|---|
| **Simplicity** | 8/10 | Five core primitives. Single evaluation path. Clear ownership. One point deducted for org hierarchy complexity; one for the legacy PolicyFunc shim period. |
| **Extensibility** | 9/10 | Every future requirement is additive-only. PolicyEvaluator is engine-neutral. AuthzRequest is extensible without breaking callers. Grant.conditions accommodates ABAC. One point deducted because RecordRuleEngine DSL evolution must be managed carefully to avoid breaking existing rules. |
| **Correctness** | 9/10 | Governing principle fully satisfied: business modules reference only capabilities. Deny rules are explicit and win. Org scope is first-class. Audit is non-optional. One point deducted: the SessionContext (Zero Trust) path is not yet designed in detail; its integration into `AuthzRequest` needs a separate design when introduced. |
| **Long-term Maintainability** | 9/10 | The separation matrix is clean and verifiable. The boundary between framework and IAM is an interface, not a package import. Business modules are provably clean when the compiler rejects role name strings. One point deducted: maintaining IAM bootstrap.go (the default capability-to-role mapping) as the system grows in capabilities requires organizational discipline; it will need its own governance process. |

**Overall:** The architecture satisfies the governing principle without compromise. The five primitives form a complete model. The evaluation pipeline has no ambiguous ownership. Every future requirement has a clear additive path. The design is ready for v1.0 freeze.

---

## Summary Reference

**The governing principle:**
> Framework defines capabilities. IAM defines who holds them. Business modules know only what exists.

**The five primitives:**
1. Capability — what can be done (compiler-generated)
2. Principal — who is doing it (authentication-generated)
3. Grant — binding of principal/role to capability (IAM database)
4. Deny — explicit override (IAM database)
5. RecordFilter — row visibility predicate (compiler + IAM database)

**The evaluation pipeline:** Authenticate → Validate → Resolve → Construct → Check → Filter → Execute → Hook → Sanitize → Audit

**What must change before v1.0:** Remove role name strings from business modules, rename CasbinPolicy to PolicyTuple, rename Actor to Principal, remove IsPlatformAdmin boolean bypass, replace ActionDef.Permission string with ActionDef.Capability string, gate the introspect endpoint, implement SDUI permission filtering.

**What can safely wait:** ABAC conditions, field permissions, SoD enforcement, policy simulation, JIT access, delegation, break-glass, IDP integration, Zero Trust session factors, ReBAC graph evaluation, offline authorization.

**The evaluator is replaceable.** That is not a nice-to-have. It is the load-bearing beam of the entire architecture.

---

*End of Canonical Authorization Architecture Review*
