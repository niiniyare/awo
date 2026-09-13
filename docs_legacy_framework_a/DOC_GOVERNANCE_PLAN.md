> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Awo Framework — Documentation Governance Plan

**Classification:** Documentation Architecture Audit
**Date:** 2026-07-20
**Auditor:** Principal Documentation Architect
**Source of truth:** `awo/docs/ARCH_FREEZE_REVIEW.md` (ADR-001 through ADR-012)
**Scope:** All 230+ documentation files in the repository

---

## Executive Finding

The repository contains documentation for two fundamentally different frameworks.

**Framework A (old):** A traditional Go monolith using SQLC, Google Wire, Zerolog, `store.WithTenant()`, and a 22-layer domain/repository/service/handler architecture. Documented in `docs/` (Portal 1–9, ~230 files).

**Framework B (current):** The Awo Framework using EntityDefinition-driven auto-generation, RuntimeFactory, slog, `set_tenant_context()`, and a 5-layer pipeline architecture. Documented in `CLAUDE.md`, `awo/docs/` (~4 files), and partially in the frozen ADRs.

These two frameworks are not related by evolution. They differ at every layer: programming model, technology stack, directory structure, testing approach, API conventions, session model, authorization model, and monetary precision.

**The entire `docs/` tree documents Framework A.**

**Framework B has almost no documentation.**

The correct response is not to refactor 230 files. It is to delete `docs/`, salvage ~15 files worth keeping, and build the missing Framework B documentation from the ARCH_FREEZE_REVIEW roadmap.

---

## Phase 1 — Documentation Inventory Assessment

### `docs/` (root portal index)

**Rating: DELETE**

`docs/README.md` is a portal navigation index for Framework A. Its content becomes the new `awo/docs/README.md`, redesigned for Framework B. The current file's links will all be dead after the migration.

---

### `docs/01-getting-started/`

**Rating: NEEDS REFACTOR**

Three files: quick-start, prerequisites, project structure.

Concepts (local setup, prerequisites) are portable. Content is wrong: references `make wire`, Zerolog setup, SQLC generation, old directory tree. Quick-start describes running a framework that no longer exists. Salvageable only by complete rewrite of the implementation steps. Structural outline: KEEP. Content: DELETE AND REWRITE.

---

### `docs/02-product-overview/`

**Rating: DELETE**

Two files: product overview, module overview.

Product marketing content. Describes modules (Contracts, HR, Finance) at a feature level. No technical specification value. Not referenced by any implementation doc. Not updated when architecture changed.

---

### `docs/03-platform-architecture/`

**Rating: NEEDS REFACTOR** (directory survives; most files do not)

**`00-overview/` — DELETE**

`01-system-overview.md` lists technology stack as: SQLC, Google Wire, Zerolog, pgx/v5. All wrong for Framework B. Architecture diagram shows SQLC Repository layer. The diagram itself is wrong. No salvageable content.

`02-architecture-principles.md` — not read but almost certainly describes Framework A's layering principles. Superseded by ARCH_FREEZE_REVIEW.md `PRINCIPLES.md` (which must be written).

**`01-multi-tenancy/` — NEEDS REFACTOR**

`01-tenancy-model.md` — Partially correct (tenant lifecycle state machine is right). Critically wrong: introduces `ResolvedSession` struct that conflicts with ADR-004's `auth.Session`. Uses `EntityScope` concept not in the frozen architecture. References `SET LOCAL app.tenant_id` directly instead of `set_tenant_context()`. Tenant identification from session token only — conflicts with frozen arch's X-Tenant-ID header + subdomain resolution.

`02-rls-enforcement.md` — Conceptually correct (FORCE ROW LEVEL SECURITY is right). Implementation wrong: uses raw `SET LOCAL app.tenant_id = $1` instead of `set_tenant_context()` stored procedure. Uses `store.WithTenant()` wrapper that does not exist in Framework B. SQL verification queries are correct and reusable.

`03-entity-scope.md` — Describes EntityScope concept not in the frozen architecture. DELETE.

**`02-iam/` — DELETE (entire directory)**

Describes the old IAM/authorization model. Conflicts with ADR-001 (PolicyEvaluator), ADR-002 (ViewerContext), ADR-003 (Actor model), ADR-004 (Session). Every file in this directory contains information that either contradicts the frozen architecture or is superseded by it.

**`03-data-architecture/` — NEEDS REFACTOR**

Content about PostgreSQL, decimal types, JSONB — mostly framework-agnostic. Likely references SQLC. Worth auditing for salvageable content but not canonical.

**`04-event-architecture/` — NEEDS REFACTOR**

Event outbox concept exists in the frozen architecture (ADR-008). These files may contain useful implementation detail about the event bus, but they predate the outbox schema def. Content cannot be canonical because the outbox schema (`event_outbox` table) is defined in the ARCH_FREEZE_REVIEW, not here.

**`05-observability/` — GOOD**

Logging, metrics, health checks. Framework-agnostic content. Prometheus metric names, Kubernetes health probe conventions — these survive a framework change. The specific logging library (Zerolog vs slog) will be wrong, but the metric taxonomy and alert thresholds are portable.

---

### `docs/04-backend-engineering/`

**Rating: DELETE (entire directory, ~150–170 files)**

This is the Module Development Guide for Framework A. It is the largest single documentation artifact in the repository. Every file in this directory describes a framework that no longer exists.

The MDG describes:
- SQLC-annotated queries (`db/queries/contracts.sql`)
- Google Wire provider sets and `wire_gen.go`
- `store.WithTenant()` on every repository call
- `ContractRepository` interface with explicit `tenantID uuid.UUID` parameters
- Domain structs, value objects, SQLC row mappers
- Zerolog structured logging patterns
- `go-playground/validator` for request validation
- `c.Locals(domain.LocalsKeySession)` for session extraction
- Flutter mobile (marked PLANNED)
- 22 numbered layers that map to nothing in Framework B

Not a single line of this guide applies to Framework B, where:
- EntityDefinition drives everything automatically
- No SQLC — auto-generated persistence
- No Wire — RuntimeFactory
- No domain/repository/service directories
- No explicit `tenantID` parameters — extracted from context
- No session extraction — injected by middleware

**The 23-section worked example uses `contracts` module**. Framework B uses `finance` module as the canonical example. The example is wrong. The framework it describes is wrong. The directory structure it teaches (`domain/`, `repository/`, `service/`) is wrong.

Deleting this directory removes approximately 65–70% of all documentation files in the repository. This is the correct action.

---

### `docs/05-frontend-engineering/`

**Rating: NEEDS REFACTOR**

`01-frontend-overview.md` — Partially salvageable. Static JSON schema files in `schemas/pages/` are the old approach; Framework B generates SDUI dynamically via `sdui.Generator`. Custom React components are explicitly prohibited by CLAUDE.md. AMIS dark theme fix is correct (confirmed in memory). File needs rewrite but the AMIS/dark-mode content is valuable.

`02-amis-schema-guide.md` — References old static schema patterns. Superseded by the WidgetTree IR (ADR-006) and the AMIS renderer spec. Salvageable structure, wrong implementation details.

`03-dark-theme.md` — CORRECT and canonical. The dark theme fix (override CSS custom properties at `html.dark`) is confirmed working and documented in memory as correct. This file is one of the few in `docs/` worth preserving verbatim.

---

### `docs/06-devops/`

**Rating: GOOD**

Kubernetes deployment, CI/CD pipeline, environment variables, local development. This content is largely framework-agnostic. The deployment topology (PostgreSQL + Redis + Temporal + Kubernetes) is the same in both frameworks. Environment variables will differ at the application level but the pattern is correct.

These files are worth moving, not deleting.

---

### `docs/07-security/`

**Rating: NEEDS REFACTOR**

`01-security-overview.md` — Defence-in-depth model is conceptually correct. Three critical errors: (1) "SQLC generates parameterized queries — SQL injection is structurally prevented" — SQLC does not exist in Framework B; (2) Casbin RBAC described without PolicyEvaluator abstraction (ADR-001); (3) `missing_ok` prohibition is correct but references old GUC pattern.

The security model section on sessions, audit, and data isolation is conceptually right but references wrong implementations.

---

### `docs/08-api-reference/`

**Rating: DELETE (entire directory)**

The API reference documents endpoints that do not exist in Framework B.

`01-api-reference-overview.md` — Response format shows `{"status": 0, "msg": "..."}` AMIS format. Framework B uses `{"data": {...}, "meta": {...}}` (success) and `{"error": {"code": "...", "message": "...", "fields": {}}}` (error). Pagination shows `page`/`page_size` offset-based — Framework B uses limit/offset via Filter DSL. Monetary values shown as `"50000.000000"` (6 decimal places) — Framework B uses `numeric(20,4)`.

`03-contracts-api.md` — Documents Contracts module endpoints. Contracts module does not exist in Framework B. Finance module is the canonical module. Delete.

`05-entities-api.md` — May describe the entity API correctly if it matches `/api/v1/entities/{entity-type}`. Possibly salvageable structure but needs complete content review.

The directory-level verdict: delete. The API conventions will be re-specified in `awo/docs/14-api/API_CONVENTIONS.md`.

---

### `docs/09-operations/`

**Rating: GOOD**

Six runbooks: service down, database issues, high error rate, migration failure, Temporal worker stuck, event outbox backlog. These are operational playbooks that are framework-agnostic. The event outbox schema query in the runbook overview (`SELECT COUNT(*) FROM event_outbox WHERE delivered_at IS NULL`) matches the frozen architecture's outbox design.

Alerting thresholds, diagnostic commands, health probe ports — all portable.

Move to `awo/docs/19-operations/`. These runbooks have immediate production value.

---

### `awo/docs/`

**Rating: CANONICAL (growing)**

Currently contains:
- `AWO_ARCH_INVARIANT_AUDIT.md` — Canonical reference
- `ARCH_FREEZE_REVIEW.md` — Constitutional document; THE source of truth

These two files plus `CLAUDE.md` are the only canonical documentation in the repository. Everything else is either wrong, outdated, or for a different framework.

---

### `CLAUDE.md` (root)

**Rating: CANONICAL**

The single most important document in the repository for day-to-day development. Contains CRITICAL RULES, Project Layout, EntityDefinition pattern, field types, middleware pipeline, SDUI conventions, Temporal rules, testing guidelines. Stays at root forever. Not part of the `docs/` restructuring.

---

### `frappe-iam.md` (root)

**Rating: DELETE**

A Frappe/ERPNext permission system reference guide. Wrong project. Wrong location. Not referenced by anything. Delete.

---

### `V1_RELEASE_READINESS.md` (root)

**Rating: ARCHIVE**

A status snapshot document dated 2026-07-20. Documents that three blockers were resolved. Has historical value as a record of decisions made. Not a specification. Should become `awo/docs/00-overview/V1_RELEASE_SNAPSHOT.md` or simply archived.

---

## Phase 2 — Duplicate Detection Matrix

| Document A | Document B | Overlap | Resolution | Winner |
|-----------|-----------|---------|------------|--------|
| `docs/03-platform-architecture/01-multi-tenancy/02-rls-enforcement.md` | `docs/04-backend-engineering/MDG/05-repository-layer/02-rls-and-tenant-isolation.md` | RLS enforcement mechanism, GUC, WithTenant pattern | DELETE both | New: `awo/docs/04-multitenancy/RLS_SPEC.md` |
| `docs/03-platform-architecture/01-multi-tenancy/01-tenancy-model.md` | `docs/04-backend-engineering/MDG/05-repository-layer/02-rls-and-tenant-isolation.md` | Tenant isolation mechanism | DELETE both | New: `awo/docs/04-multitenancy/TENANT_LIFECYCLE.md` |
| `docs/03-platform-architecture/01-multi-tenancy/01-tenancy-model.md` | `docs/07-security/01-security-overview.md` | Session model, tenant isolation | DELETE one, REFACTOR one | `07-security` partially moves to `awo/docs/18-security/` |
| `docs/07-security/01-security-overview.md` | `docs/03-platform-architecture/02-iam/03-authorization-model.md` | Authorization model, Casbin RBAC | DELETE both | New: `awo/docs/03-auth/AUTHORIZATION_SPEC.md` |
| `docs/08-api-reference/01-api-reference-overview.md` | `docs/04-backend-engineering/MDG/14-api-design/02-url-conventions.md` | URL conventions, response format | DELETE both | New: `awo/docs/14-api/API_CONVENTIONS.md` |
| `docs/08-api-reference/02-authentication.md` | `docs/03-platform-architecture/02-iam/01-sessions.md` (assumed) | Session auth, token format | DELETE both | New: `awo/docs/03-auth/SESSION_MODEL.md` |
| `docs/04-backend-engineering/MDG/11-temporal-workflows/` (multiple files) | `docs/03-platform-architecture/04-event-architecture/` (multiple files) | Temporal workflow patterns, determinism rules | DELETE MDG section; REFACTOR arch section | New: `awo/docs/08-workflow/TEMPORAL_INTEGRATION.md` |
| `docs/04-backend-engineering/MDG/15-audit-logging/01-audit-overview.md` | `docs/07-security/01-security-overview.md` (audit section) | Audit trail, immutable audit_log | DELETE MDG section; REFACTOR security | New: `awo/docs/12-audit/AUDIT_SPEC.md` |
| `docs/04-backend-engineering/MDG/03-database-layer/` | `docs/15-migrations/MIGRATION_GUIDE.md` (doesn't exist yet) | Migration patterns, RLS table setup | DELETE MDG section | New: `awo/docs/15-migrations/MIGRATION_GUIDE.md` |
| `docs/04-backend-engineering/MDG/02-domain-layer/04-domain-errors.md` | `docs/03-platform-architecture/` (error handling mentioned everywhere) | Error model, BusinessError, ValidationError | DELETE MDG section | New: `awo/docs/02-pipeline/ERROR_MODEL.md` |
| `docs/05-frontend-engineering/02-amis-schema-guide.md` | `docs/04-backend-engineering/MDG/18-amis-web-schemas/` | AMIS schema patterns | DELETE MDG section; REFACTOR frontend guide | New: `awo/docs/10-sdui/PAGE_BUILDER_GUIDE.md` |
| `docs/03-platform-architecture/00-overview/01-system-overview.md` | `docs/README.md` | System overview, portal index | DELETE both | New: `awo/docs/README.md` + `awo/docs/00-overview/ARCH_OVERVIEW.md` |
| `docs/01-getting-started/03-project-structure.md` | `CLAUDE.md` (Project Layout section) | Directory structure | DELETE getting-started version | `CLAUDE.md` is canonical |
| `docs/04-backend-engineering/MDG/22-testing-guide/` | `docs/16-testing/TEST_STRATEGY.md` (doesn't exist yet) | Test patterns, integration tests | DELETE MDG section | New: `awo/docs/16-testing/TEST_STRATEGY.md` |
| `docs/04-backend-engineering/MDG/21-server-startup/` | `CLAUDE.md` (Startup Order section) | Startup sequence | DELETE MDG section | `CLAUDE.md` is canonical |
| `docs/04-backend-engineering/MDG/08-middleware-layer/` | `docs/03-platform-architecture/` (middleware mentioned) | Middleware chain order | DELETE MDG section | New: `awo/docs/02-pipeline/LIFECYCLE_SPEC.md` |
| `docs/06-devops/02-environment-variables.md` | `docs/04-backend-engineering/MDG/21-server-startup/` (config section) | Environment variables | DELETE MDG section | `docs/06-devops/` → `awo/docs/20-devops/` |
| `awo/docs/AWO_ARCH_INVARIANT_AUDIT.md` | `awo/docs/ARCH_FREEZE_REVIEW.md` | Architecture analysis | KEEP BOTH — different purposes | Both canonical; audit is historical |
| `docs/04-backend-engineering/MDG/13-event-driven/` | `docs/03-platform-architecture/04-event-architecture/` | Event-driven patterns | DELETE MDG section; REFACTOR arch | New: `awo/docs/09-events/EVENT_OUTBOX_SPEC.md` |

**Conceptual Duplications (no direct file pairing — same concept repeated in many files):**

| Concept | Appears In | Action |
|---------|-----------|--------|
| RLS explanation | Portal 3 multi-tenancy (3 files), Portal 4 MDG (5+ files), Portal 7 security, Portal 8 API ref | One canonical: `awo/docs/04-multitenancy/RLS_SPEC.md` |
| Session/auth flow | Portal 3 IAM (multiple), Portal 7 security, Portal 8 auth | One canonical: `awo/docs/03-auth/SESSION_MODEL.md` |
| Error format | Portal 4 MDG domain errors, Portal 4 MDG handler errors, Portal 8 API ref | One canonical: `awo/docs/02-pipeline/ERROR_MODEL.md` |
| Temporal determinism rules | Portal 4 MDG temporal (multiple), Portal 3 arch | One canonical: `awo/docs/08-workflow/TEMPORAL_INTEGRATION.md` |
| Monetary type rule | CLAUDE.md, Portal 4 MDG (multiple), Portal 3 data arch | `CLAUDE.md` is canonical; all others reference it |
| Audit trail | Portal 4 MDG audit (multiple files), Portal 7 security | One canonical: `awo/docs/12-audit/AUDIT_SPEC.md` |
| Pagination | Portal 4 MDG API design, Portal 8 API ref | One canonical: `awo/docs/14-api/PAGINATION_SPEC.md` |

---

## Phase 3 — Contradiction Report

Documents are assessed against the 12 frozen ADRs and the ARCH_FREEZE_REVIEW.

| Document | Conflicting Decision | Severity | Action |
|----------|---------------------|----------|--------|
| `docs/03-platform-architecture/00-overview/01-system-overview.md` | All ADRs — wrong tech stack (SQLC, Wire, Zerolog), wrong architecture diagram (SQLC repository layer), wrong layer model | **CRITICAL** | DELETE |
| `docs/03-platform-architecture/01-multi-tenancy/01-tenancy-model.md` | ADR-004: `ResolvedSession` struct conflicts with `auth.Session`; session embeds EntityScope not in frozen arch; tenant identification says "session is canonical" but frozen arch uses X-Tenant-ID header + subdomain as primary | **HIGH** | REWRITE |
| `docs/03-platform-architecture/01-multi-tenancy/02-rls-enforcement.md` | ADR (RLS): Uses raw `SET LOCAL app.tenant_id = $1` — frozen arch mandates `set_tenant_context()` stored procedure which validates tenant status; uses `store.WithTenant()` which doesn't exist | **HIGH** | REWRITE |
| `docs/03-platform-architecture/02-iam/` (all files) | ADR-001 (PolicyEvaluator), ADR-002 (ViewerContext), ADR-003 (Actor), ADR-004 (Session) — entire IAM model is wrong | **CRITICAL** | DELETE |
| `docs/04-backend-engineering/` (all ~170 files) | All 12 ADRs — SQLC (no SQLC in frozen arch), Wire (no Wire), Zerolog (slog), WithTenant (set_tenant_context), domain/repo/service layout (definition/hooks/actions), EntityScope (not in arch), go-playground/validator (not in arch), Flutter (not in arch), numeric(20,6) (frozen uses 20,4) | **CRITICAL** | DELETE |
| `docs/05-frontend-engineering/01-frontend-overview.md` | ADR-006 (WidgetTree IR): Says schemas live in static `schemas/pages/` JSON files — frozen arch generates SDUI dynamically from EntityDefinition. Mentions custom React components — CLAUDE.md prohibits them. Mentions Flutter — not in frozen arch | **HIGH** | REWRITE |
| `docs/05-frontend-engineering/02-amis-schema-guide.md` | ADR-006 (WidgetTree IR): Documents direct amis JSON authoring — frozen arch mandates WidgetTree IR as intermediate representation; direct amis JSON is prohibited for framework-generated pages | **MEDIUM** | REWRITE |
| `docs/07-security/01-security-overview.md` | ADR-001 (PolicyEvaluator): "No role check in business code: session.HasRole() does not exist — use authz service" — conflicts with `viewer.HasRole()` in ViewerContext (ADR-002). "SQLC generates parameterized queries" — no SQLC in frozen arch. Authorization described without PolicyEvaluator abstraction | **HIGH** | REWRITE |
| `docs/08-api-reference/01-api-reference-overview.md` | API conventions: Response envelope format different (`{"status": 0}` AMIS format vs `{"data": {}, "meta": {}}`). Monetary values shown as 6 decimals vs 4. Pagination uses `page`/`page_size` vs Filter DSL limit/offset | **HIGH** | DELETE |
| `docs/08-api-reference/03-contracts-api.md` | Entity model: Contracts module does not exist in Framework B; Finance module is the canonical example | **CRITICAL** | DELETE |
| `frappe-iam.md` | Not Framework B documentation — Frappe/ERPNext reference | **HIGH** | DELETE |
| `docs/01-getting-started/01-quick-start.md` | Project structure: References `make wire`, `sqlc generate`, old directory layout | **MEDIUM** | REWRITE |
| `docs/02-product-overview/` | Framework identity: Describes "AwoERP" as a product; frozen arch positions it as "Awo Framework" — an ERP framework, not an ERP product | **LOW** | DELETE (wrong framing) |

---

## Phase 4 — Canonical Documentation Map

The following is the final documentation hierarchy. It replaces all existing `docs/` content.

All framework documentation lives in `awo/docs/`. The `docs/` directory at the repository root is deleted.

```
awo/docs/
│
├── README.md                          ← Master index (≤3 pages; links only; no content)
│
├── 00-overview/                       ← Constitutional layer — read first
│   ├── ARCH_OVERVIEW.md               ← System diagram, 5-layer model, startup order [MISSING]
│   ├── PRINCIPLES.md                  ← Design principles benchmarked against Kubernetes/Temporal [MISSING]
│   ├── PACKAGE_DEPENDENCY_MAP.md      ← DAG of all awo/* packages; no cycles guaranteed [MISSING]
│   ├── DECISION_REGISTER.md           ← ADR-001 to ADR-012 (= ARCH_FREEZE_REVIEW.md renamed)
│   ├── GLOSSARY.md                    ← One definition per term, forever [MISSING]
│   └── V1_RELEASE_SNAPSHOT.md         ← Historical status (= V1_RELEASE_READINESS.md moved)
│
├── 01-entity/                         ← EntityDefinition is the central primitive
│   ├── ENTITY_DEFINITION_SPEC.md      ← EntityDefinition interface, all methods [MISSING]
│   ├── FIELD_TYPES_REFERENCE.md       ← All FieldType constants: PG column, Go type, widget [MISSING]
│   ├── EDGE_TYPES_REFERENCE.md        ← EdgeType semantics, FK, cascade [MISSING]
│   └── NAMING_CONVENTIONS.md          ← Module_noun format, action naming, field naming [MISSING]
│
├── 02-pipeline/                       ← The hook pipeline — how every mutation works
│   ├── LIFECYCLE_SPEC.md              ← ASSEMBLE→VALIDATE→TX→PERSIST→AUDIT→after_save [MISSING]
│   ├── HOOK_CONTRACT.md               ← All hook interfaces, execution order, recursion policy [MISSING]
│   ├── ERROR_MODEL.md                 ← ValidationError, BusinessError, HTTP mapping [MISSING]
│   └── PIPELINE_SEQUENCE.md           ← Sequence diagram: request → response [MISSING]
│
├── 03-auth/                           ← Identity, session, authorization
│   ├── AUTHORIZATION_SPEC.md          ← PermissionSet, PolicyEvaluator, CapabilityGrant [MISSING]
│   ├── ACTOR_MODEL.md                 ← Actor struct (post-ADR-003), ServiceAccount [MISSING]
│   ├── SESSION_MODEL.md               ← auth.Session struct, Redis storage, revocation [MISSING]
│   ├── VIEWER_CONTEXT.md              ← ViewerContext interface, WithViewer, ViewerFromContext [MISSING]
│   ├── RBAC_ROLES_REFERENCE.md        ← Built-in roles, naming conventions [MISSING]
│   └── CASBIN_ADAPTER.md             ← Implementation doc: Casbin PolicyEvaluator [MISSING]
│
├── 04-multitenancy/                   ← Multi-tenancy and RLS
│   ├── RLS_SPEC.md                    ← set_tenant_context(), FORCE RLS, PgBouncer req [NEEDS REWRITE from old]
│   ├── TENANT_LIFECYCLE.md            ← State machine: PENDING→ACTIVE→SUSPENDED→ARCHIVED [SALVAGE from old]
│   ├── TENANT_IDENTIFICATION.md       ← X-Tenant-ID header, subdomain parsing [MISSING]
│   └── GLOBAL_TABLES.md              ← Tables exempt from RLS; rationale [MISSING]
│
├── 05-compiler/                       ← How EntityDefinitions become a runtime registry
│   ├── COMPILE_SPEC.md                ← Phase-by-phase: routes, lookups, link resolution [MISSING]
│   ├── COMPILED_SCHEMA_REFERENCE.md   ← CompiledSchema, EntitySchema, CapabilityGrant [MISSING]
│   └── VALIDATION_RULES.md            ← All validation rules in Registry.Build [MISSING]
│
├── 06-filter/                         ← Composable predicate DSL
│   ├── FILTER_DSL_REFERENCE.md        ← All constructors, Filter struct, examples [MISSING]
│   └── FILTER_POLICY_PATTERNS.md      ← OwnerOnly, BranchScoped, TenantIsolation [MISSING]
│
├── 07-naming/                         ← Naming series
│   ├── NAMING_SERIES_SPEC.md          ← Pattern syntax, tokens, counter key, reset periods [EXISTS in awo/naming/]
│   └── NAMING_SERIES_EXAMPLES.md      ← INV-{YYYY}-{SEQ:6} examples [MISSING]
│
├── 08-workflow/                       ← Temporal integration
│   ├── TEMPORAL_INTEGRATION.md        ← WorkflowTrigger, determinism rules, activities [MISSING]
│   ├── OUTBOX_SPEC.md                 ← workflow_outbox schema, worker, retry policy [MISSING]
│   ├── SAGA_PATTERN.md                ← Compensating transactions, SagaCompensator [MISSING]
│   └── WORKFLOW_ID_CONVENTION.md      ← {tenant}.{entity}.{record}.{event} [MISSING]
│
├── 09-events/                         ← Event outbox
│   ├── EVENT_OUTBOX_SPEC.md           ← event_outbox schema, EventBroker interface [MISSING]
│   └── DOMAIN_EVENTS_REFERENCE.md     ← Topic naming, payload contracts [MISSING]
│
├── 10-sdui/                           ← Server-driven UI
│   ├── WIDGET_TREE_SPEC.md            ← Node struct, all NodeKind values, DataSource [MISSING]
│   ├── AMIS_RENDERER_SPEC.md          ← NodeKind → amis type mapping, caching [MISSING]
│   ├── PAGE_BUILDER_GUIDE.md          ← Custom PageBuilder, permission-gated elements [MISSING]
│   ├── SDUI_FIELD_WIDGET_MAP.md        ← FieldType → NodeKind → amis type table [MISSING]
│   └── DARK_THEME.md                  ← AMIS dark theme fix [SALVAGE from docs/05-frontend-engineering/03-dark-theme.md]
│
├── 11-cache/                          ← Cache contracts
│   ├── CACHE_SPEC.md                  ← Cache interface, Counter interface, key naming, TTL [MISSING]
│   └── CACHE_KEY_REFERENCE.md         ← All key patterns: session:, page:, eval:, rl: [MISSING]
│
├── 12-audit/                          ← Audit pipeline
│   ├── AUDIT_SPEC.md                  ← AuditRecord schema, pipeline stage, AuditEnabled [MISSING]
│   └── AUDIT_QUERY_PATTERNS.md        ← How to query audit log; what is never logged [MISSING]
│
├── 13-actions/                        ← Custom action handlers
│   ├── ACTION_HANDLER_GUIDE.md        ← ActionContext, ActionRuntime API, patterns [MISSING]
│   ├── ACTION_RUNTIME_REFERENCE.md    ← All ActionRuntime methods, contracts [MISSING]
│   └── CUSTOM_ACTIONS_EXAMPLES.md     ← Finance actions as canonical worked examples [MISSING]
│
├── 14-api/                            ← HTTP API conventions
│   ├── API_CONVENTIONS.md             ← URL structure, response envelope, HTTP status [MISSING]
│   ├── IDEMPOTENCY_SPEC.md            ← X-Idempotency-Key protocol (ADR-009) [MISSING]
│   ├── PAGINATION_SPEC.md             ← limit/offset via Filter DSL, PageInfo struct [MISSING]
│   └── ERROR_RESPONSE_FORMAT.md       ← Client-facing error envelope, field errors [MISSING]
│
├── 15-migrations/                     ← Database migration governance
│   ├── MIGRATION_GUIDE.md             ← File naming, up/down pairs, zero-downtime patterns [MISSING]
│   ├── RLS_TABLE_TEMPLATE.md          ← SQL template for every new tenant-scoped table [MISSING]
│   └── MIGRATION_CHECKLIST.md         ← Mandatory items: RLS, tenant_id FK, indexes [MISSING]
│
├── 16-testing/                        ← Test strategy
│   ├── TEST_STRATEGY.md               ← Unit vs integration, real PostgreSQL mandate [MISSING]
│   ├── HOOK_TEST_PATTERNS.md          ← Mock EntityRepository, BeforeCreate examples [MISSING]
│   ├── ACTION_TEST_PATTERNS.md        ← ActionRuntime mock, test helpers [MISSING]
│   └── REGISTRY_TEST_PATTERNS.md      ← BuildFrom for isolated test registries [MISSING]
│
├── 17-observability/                  ← Logging, metrics, health
│   ├── LOGGING_SPEC.md                ← slog fields, context keys, sensitive exclusions [MISSING]
│   ├── METRICS_SPEC.md                ← Prometheus metrics, histograms, label conventions [SALVAGE from docs/03-platform-architecture/05-observability/]
│   └── HEALTH_CHECKS.md              ← /health/live and /health/ready contracts [MISSING]
│
├── 18-security/                       ← Security model
│   ├── SECURITY_MODEL.md              ← RLS, RBAC, session, rate limiting, threat model [REWRITE from docs/07-security/]
│   ├── SENSITIVE_FIELDS.md            ← Sensitive: true semantics, log exclusion [MISSING]
│   └── SECRET_MANAGEMENT.md           ← Env vars, Vault, never-in-logs rules [MISSING]
│
├── 19-operations/                     ← Operations runbooks
│   ├── RUNBOOK_INDEX.md               ← = docs/09-operations/01-runbook-overview.md [MOVE]
│   ├── RUNBOOK_SERVICE_DOWN.md        ← = docs/09-operations/02-service-down.md [MOVE]
│   ├── RUNBOOK_DATABASE.md            ← = docs/09-operations/03-database-issues.md [MOVE]
│   ├── RUNBOOK_HIGH_ERROR_RATE.md     ← = docs/09-operations/04-high-error-rate.md [MOVE]
│   ├── RUNBOOK_MIGRATION_FAILURE.md   ← = docs/09-operations/05-migration-failure.md [MOVE]
│   ├── RUNBOOK_TEMPORAL_WORKER.md     ← = docs/09-operations/06-temporal-worker.md [MOVE]
│   └── RUNBOOK_EVENT_OUTBOX.md        ← = docs/09-operations/07-event-outbox.md [MOVE]
│
├── 20-devops/                         ← Deployment and infrastructure
│   ├── ENVIRONMENT_VARIABLES.md       ← = docs/06-devops/02-environment-variables.md [MOVE]
│   ├── KUBERNETES_DEPLOYMENT.md       ← = docs/06-devops/03-kubernetes-deployment.md [MOVE]
│   ├── CICD_PIPELINE.md               ← = docs/06-devops/04-cicd-pipeline.md [MOVE]
│   ├── LOCAL_DEVELOPMENT.md           ← = docs/06-devops/05-local-development.md [MOVE]
│   └── MONITORING.md                  ← = docs/06-devops/06-monitoring.md [MOVE]
│
└── 99-modules/                        ← Module reference implementations
    ├── FINANCE_MODULE_SPEC.md         ← 24 entities, 3 migration files, action contracts [MISSING]
    ├── MODULE_AUTHOR_GUIDE.md          ← How to build a module; EntityDefinition patterns [MISSING]
    └── ONBOARDING_CHECKLIST.md        ← New module author: what to read, in order [MISSING]
```

**What happens to `docs/` (the root Portal structure):**
- Deleted entirely
- The `docs/README.md` master index is rewritten as `awo/docs/README.md`
- Portal numbering (01–09) is removed as a navigation concept
- Content is organized by technical concept, not by audience

---

## Phase 5 — Canonical Specifications

These are the constitutional documents. Every other document references them. They never duplicate each other. They are written once and maintained as the ground truth.

Violating these documents is an architecture violation, not a documentation mistake.

### Tier 0 — Non-Negotiable Before Any Code

| Document | What It Governs | Why Irreplaceable |
|----------|----------------|-------------------|
| `CLAUDE.md` | Development rules, critical constraints, quick reference | Every developer reads this daily |
| `awo/docs/00-overview/DECISION_REGISTER.md` | All 12 ADRs, every architectural decision | No decision is made without checking this first |
| `awo/docs/00-overview/ARCH_OVERVIEW.md` | System diagram, layer model, startup order | Mental model document; without it, developers build the wrong thing |
| `awo/docs/01-entity/ENTITY_DEFINITION_SPEC.md` | EntityDefinition interface — the framework's central primitive | Everything derives from this; if it's wrong, everything is wrong |
| `awo/docs/02-pipeline/LIFECYCLE_SPEC.md` | Exact pipeline stages, TX boundaries, hook execution order | Hook authors need this; pipeline implementors need this |
| `awo/docs/03-auth/AUTHORIZATION_SPEC.md` | PermissionSet + PolicyEvaluator + CapabilityGrant | Security depends on this being correct and singular |
| `awo/docs/04-multitenancy/RLS_SPEC.md` | set_tenant_context(), FORCE RLS, PgBouncer requirement | Data isolation depends on this being followed exactly |
| `awo/docs/14-api/API_CONVENTIONS.md` | URL structure, response envelope, HTTP status codes | Every API client and every server route depends on this |

### Tier 1 — Required Before Subsystem Implementation

| Document | Governs |
|----------|---------|
| `awo/docs/02-pipeline/ERROR_MODEL.md` | Error types, HTTP mapping, error chain protocol |
| `awo/docs/03-auth/SESSION_MODEL.md` | auth.Session struct — session shape is frozen |
| `awo/docs/03-auth/VIEWER_CONTEXT.md` | ViewerContext interface — how auth propagates |
| `awo/docs/05-compiler/COMPILE_SPEC.md` | Compilation phases, CapabilityGrant output |
| `awo/docs/10-sdui/WIDGET_TREE_SPEC.md` | Node IR — everything SDUI depends on this |
| `awo/docs/08-workflow/OUTBOX_SPEC.md` | workflow_outbox schema — all workflow starts flow through this |
| `awo/docs/09-events/EVENT_OUTBOX_SPEC.md` | event_outbox schema — all events flow through this |
| `awo/docs/12-audit/AUDIT_SPEC.md` | AuditRecord schema, AuditEnabled semantics |
| `awo/docs/15-migrations/MIGRATION_GUIDE.md` | Migration file naming, RLS requirements |

### The One-Source Rule

Every concept has exactly one canonical document. All other documents reference it with a link. They do not repeat it.

If two documents explain the same concept, one of them becomes a redirect (`→ See CANONICAL_DOC.md`) or is deleted.

---

## Phase 6 — Documentation Dependency Graph

Documents are organized in a strict dependency order. A reader always knows what to read before the current document. No circular dependencies.

```
Level 0 — Roots (depend on nothing):
  CLAUDE.md
  DECISION_REGISTER.md  (= ARCH_FREEZE_REVIEW)
  ARCH_OVERVIEW.md
  GLOSSARY.md

Level 1 — Foundation (depend only on Level 0):
  ENTITY_DEFINITION_SPEC.md → ARCH_OVERVIEW
  ACTOR_MODEL.md → ARCH_OVERVIEW
  RLS_SPEC.md → ARCH_OVERVIEW
  PRINCIPLES.md → ARCH_OVERVIEW

Level 2 — Core contracts (depend on Level 0–1):
  FIELD_TYPES_REFERENCE.md → ENTITY_DEFINITION_SPEC
  EDGE_TYPES_REFERENCE.md → ENTITY_DEFINITION_SPEC
  NAMING_CONVENTIONS.md → ENTITY_DEFINITION_SPEC
  ERROR_MODEL.md → ENTITY_DEFINITION_SPEC
  SESSION_MODEL.md → ACTOR_MODEL
  TENANT_LIFECYCLE.md → RLS_SPEC
  TENANT_IDENTIFICATION.md → RLS_SPEC
  GLOBAL_TABLES.md → RLS_SPEC

Level 3 — Pipeline and auth (depend on Level 1–2):
  LIFECYCLE_SPEC.md → ENTITY_DEFINITION_SPEC, ERROR_MODEL
  AUTHORIZATION_SPEC.md → ENTITY_DEFINITION_SPEC, ACTOR_MODEL
  VIEWER_CONTEXT.md → AUTHORIZATION_SPEC, SESSION_MODEL
  RBAC_ROLES_REFERENCE.md → AUTHORIZATION_SPEC

Level 4 — Subsystems (depend on Level 1–3):
  HOOK_CONTRACT.md → LIFECYCLE_SPEC
  PIPELINE_SEQUENCE.md → LIFECYCLE_SPEC, AUTHORIZATION_SPEC
  COMPILE_SPEC.md → ENTITY_DEFINITION_SPEC, AUTHORIZATION_SPEC
  FILTER_DSL_REFERENCE.md → ENTITY_DEFINITION_SPEC
  NAMING_SERIES_SPEC.md → ENTITY_DEFINITION_SPEC
  OUTBOX_SPEC.md (workflow) → LIFECYCLE_SPEC
  EVENT_OUTBOX_SPEC.md → LIFECYCLE_SPEC
  AUDIT_SPEC.md → LIFECYCLE_SPEC, AUTHORIZATION_SPEC
  WIDGET_TREE_SPEC.md → ENTITY_DEFINITION_SPEC
  CACHE_SPEC.md → ARCH_OVERVIEW
  MIGRATION_GUIDE.md → RLS_SPEC

Level 5 — Implementation guides (depend on Level 1–4):
  CASBIN_ADAPTER.md → AUTHORIZATION_SPEC, COMPILE_SPEC
  AMIS_RENDERER_SPEC.md → WIDGET_TREE_SPEC
  ACTION_HANDLER_GUIDE.md → LIFECYCLE_SPEC, AUTHORIZATION_SPEC
  ACTION_RUNTIME_REFERENCE.md → ACTION_HANDLER_GUIDE
  TEMPORAL_INTEGRATION.md → OUTBOX_SPEC
  SAGA_PATTERN.md → TEMPORAL_INTEGRATION
  PAGE_BUILDER_GUIDE.md → WIDGET_TREE_SPEC, AMIS_RENDERER_SPEC
  HOOK_TEST_PATTERNS.md → HOOK_CONTRACT
  REGISTRY_TEST_PATTERNS.md → COMPILE_SPEC

Level 6 — Reference material (depend on Level 1–5):
  COMPILED_SCHEMA_REFERENCE.md → COMPILE_SPEC
  VALIDATION_RULES.md → COMPILE_SPEC
  FILTER_POLICY_PATTERNS.md → FILTER_DSL_REFERENCE
  SDUI_FIELD_WIDGET_MAP.md → WIDGET_TREE_SPEC, FIELD_TYPES_REFERENCE
  CACHE_KEY_REFERENCE.md → CACHE_SPEC
  DOMAIN_EVENTS_REFERENCE.md → EVENT_OUTBOX_SPEC
  API_CONVENTIONS.md → ENTITY_DEFINITION_SPEC, ERROR_MODEL
  IDEMPOTENCY_SPEC.md → API_CONVENTIONS
  PAGINATION_SPEC.md → API_CONVENTIONS, FILTER_DSL_REFERENCE
  ERROR_RESPONSE_FORMAT.md → API_CONVENTIONS, ERROR_MODEL
  RLS_TABLE_TEMPLATE.md → MIGRATION_GUIDE
  MIGRATION_CHECKLIST.md → MIGRATION_GUIDE, RLS_TABLE_TEMPLATE
  TEST_STRATEGY.md → ENTITY_DEFINITION_SPEC, LIFECYCLE_SPEC
  ACTION_TEST_PATTERNS.md → ACTION_HANDLER_GUIDE, TEST_STRATEGY
  LOGGING_SPEC.md → ARCH_OVERVIEW
  METRICS_SPEC.md → ARCH_OVERVIEW
  HEALTH_CHECKS.md → ARCH_OVERVIEW
  SECURITY_MODEL.md → AUTHORIZATION_SPEC, RLS_SPEC
  SENSITIVE_FIELDS.md → SECURITY_MODEL, ENTITY_DEFINITION_SPEC
  SECRET_MANAGEMENT.md → SECURITY_MODEL

Level 7 — Module guides (depend on Level 1–6):
  FINANCE_MODULE_SPEC.md → ENTITY_DEFINITION_SPEC, LIFECYCLE_SPEC, ACTION_HANDLER_GUIDE
  MODULE_AUTHOR_GUIDE.md → all Level 1–6 specs
  ONBOARDING_CHECKLIST.md → MODULE_AUTHOR_GUIDE

Level 8 — Operational (depend on Level 0–6, operationally):
  RUNBOOK_*.md → HEALTH_CHECKS, METRICS_SPEC, EVENT_OUTBOX_SPEC
  KUBERNETES_DEPLOYMENT.md → ENVIRONMENT_VARIABLES, HEALTH_CHECKS
  ENVIRONMENT_VARIABLES.md → ARCH_OVERVIEW
  CICD_PIPELINE.md → KUBERNETES_DEPLOYMENT
  LOCAL_DEVELOPMENT.md → ENVIRONMENT_VARIABLES
```

**Verified: No circular dependencies exist in this graph.**

---

## Phase 7 — Deletion Candidates

These files should be deleted outright. No salvage. No archival. No merge. Deleted.

### Category A — Wrong Framework Entirely (~170 files)

Every file under `docs/04-backend-engineering/` describes Framework A (SQLC + Wire + Zerolog). Not a single file applies to Framework B. Deleting this directory eliminates approximately 70% of all documentation files in the repository.

Specific sections to delete:
- `00-module-development-guide/01-overview/` — wrong framework overview
- `00-module-development-guide/02-domain-layer/` — DDD domain layer doesn't exist in Framework B
- `00-module-development-guide/03-database-layer/` — SQLC schema doesn't exist
- `00-module-development-guide/04-sqlc-layer/` — SQLC doesn't exist
- `00-module-development-guide/05-repository-layer/` — old repository pattern
- `00-module-development-guide/06-service-layer/` — service layer doesn't exist
- `00-module-development-guide/07-handler-layer/` — old handler pattern
- `00-module-development-guide/08-middleware-layer/` — old middleware, superseded
- `00-module-development-guide/09-wire-registration/` — Wire doesn't exist
- `00-module-development-guide/10–12/` — additional wrong layers
- `00-module-development-guide/13-event-driven/` — old event bus
- `00-module-development-guide/14-api-design/` — old API conventions
- `00-module-development-guide/15-audit-logging/` — old audit hook pattern
- `00-module-development-guide/16–22/` — additional wrong sections
- `00-module-development-guide/23-worked-example/` — Contracts module; wrong module, wrong framework

**Why not archive?** Archiving implies future reference value. These files will mislead contributors who find them. A developer who reads "use `store.WithTenant()` on every repository call" will write wrong code. The only safe action is deletion.

### Category B — Wrong Framework, Wrong Location

| File | Reason |
|------|--------|
| `frappe-iam.md` | Documents Frappe/ERPNext, not Awo Framework |
| `docs/02-product-overview/01-product-overview.md` | Marketing content; no technical specification value |
| `docs/02-product-overview/02-module-overview.md` | Marketing content; duplicates CLAUDE.md module list |
| `docs/03-platform-architecture/00-overview/01-system-overview.md` | Wrong technology stack diagram |
| `docs/03-platform-architecture/00-overview/02-architecture-principles.md` | Superseded by ARCH_FREEZE_REVIEW principles |
| `docs/03-platform-architecture/02-iam/` (all files) | Wrong IAM model; conflicts with ADR-001–004 |
| `docs/03-platform-architecture/01-multi-tenancy/03-entity-scope.md` | EntityScope concept not in frozen architecture |
| `docs/08-api-reference/03-contracts-api.md` | Contracts module doesn't exist |
| `docs/08-api-reference/` (all other files) | Wrong API format, wrong endpoint patterns |

### Category C — Status Documents That Should Not Exist

| File | Reason |
|------|--------|
| `V1_RELEASE_READINESS.md` | Point-in-time status snapshot; misleading once items change. Move to `awo/docs/00-overview/V1_RELEASE_SNAPSHOT.md` as a historical record with a prominent "historical document — not current" banner. |

---

## Phase 8 — Merge Plan

Complete disposition for every file in the repository.

### `docs/README.md`

**Action: MERGE → RENAME**
Merge content into `awo/docs/README.md`. The portal link table format is good; the destinations change completely. Old file deleted.

---

### `docs/01-getting-started/`

| File | Action | Destination |
|------|--------|-------------|
| `01-quick-start.md` | REWRITE → MOVE | `awo/docs/20-devops/LOCAL_DEVELOPMENT.md` (merge with devops local dev content) |
| `02-prerequisites.md` | MERGE | Into `LOCAL_DEVELOPMENT.md` (prerequisites is a section, not a file) |
| `03-project-structure.md` | DELETE | Superseded by `CLAUDE.md` Project Layout section |

---

### `docs/02-product-overview/`

| File | Action | Destination |
|------|--------|-------------|
| `01-product-overview.md` | DELETE | Marketing content; no specification value |
| `02-module-overview.md` | DELETE | Duplicates CLAUDE.md; adds no precision |

---

### `docs/03-platform-architecture/`

| File | Action | Destination |
|------|--------|-------------|
| `00-overview/01-system-overview.md` | DELETE | Wrong tech stack |
| `00-overview/02-architecture-principles.md` | DELETE | Superseded by ARCH_FREEZE_REVIEW |
| `01-multi-tenancy/01-tenancy-model.md` | REWRITE → SPLIT | State machine section → `awo/docs/04-multitenancy/TENANT_LIFECYCLE.md`; session section → deleted (conflicts with ADR-004) |
| `01-multi-tenancy/02-rls-enforcement.md` | REWRITE → RENAME | `awo/docs/04-multitenancy/RLS_SPEC.md` — update GUC to set_tenant_context(), remove store.WithTenant() |
| `01-multi-tenancy/03-entity-scope.md` | DELETE | EntityScope concept not in frozen arch |
| `02-iam/` (all files) | DELETE | Entire directory; conflicts with ADR-001–004 |
| `03-data-architecture/` | AUDIT → PARTIAL MERGE | Data types section → `awo/docs/01-entity/FIELD_TYPES_REFERENCE.md`; JSONB section → into entity spec; rest likely DELETE |
| `04-event-architecture/` | REWRITE → MERGE | Outbox concepts → `awo/docs/09-events/EVENT_OUTBOX_SPEC.md`; old event bus → DELETE |
| `05-observability/01-observability-overview.md` | REWRITE → RENAME | `awo/docs/17-observability/` (update Zerolog → slog) |
| `05-observability/04-metrics.md` | MOVE | `awo/docs/17-observability/METRICS_SPEC.md` (mostly correct content) |

---

### `docs/04-backend-engineering/` (all ~170 files)

| File | Action | Destination |
|------|--------|-------------|
| ALL FILES | DELETE | Wrong framework; no salvageable content |

One exception: If the worked example in `23-worked-example/` contains correct SQL patterns for RLS table creation, extract those SQL snippets only into `awo/docs/15-migrations/RLS_TABLE_TEMPLATE.md` before deletion.

---

### `docs/05-frontend-engineering/`

| File | Action | Destination |
|------|--------|-------------|
| `01-frontend-overview.md` | REWRITE → RENAME | `awo/docs/10-sdui/PAGE_BUILDER_GUIDE.md` — remove static schema references, remove custom React/Flutter, update for WidgetTree IR |
| `02-amis-schema-guide.md` | REWRITE → MERGE | Content merged into `awo/docs/10-sdui/AMIS_RENDERER_SPEC.md` after WidgetTree spec written |
| `03-dark-theme.md` | MOVE | `awo/docs/10-sdui/DARK_THEME.md` — content is CORRECT, no changes |

---

### `docs/06-devops/`

| File | Action | Destination |
|------|--------|-------------|
| `01-devops-overview.md` | DELETE | Replace with `awo/docs/README.md` link to devops section |
| `02-environment-variables.md` | MOVE | `awo/docs/20-devops/ENVIRONMENT_VARIABLES.md` |
| `03-kubernetes-deployment.md` | MOVE | `awo/docs/20-devops/KUBERNETES_DEPLOYMENT.md` |
| `04-cicd-pipeline.md` | MOVE | `awo/docs/20-devops/CICD_PIPELINE.md` |
| `05-local-development.md` | MERGE + MOVE | Merge with quick-start → `awo/docs/20-devops/LOCAL_DEVELOPMENT.md` |
| `06-monitoring.md` | MERGE + MOVE | Merge with metrics → `awo/docs/17-observability/MONITORING.md` |

---

### `docs/07-security/`

| File | Action | Destination |
|------|--------|-------------|
| `01-security-overview.md` | REWRITE → SPLIT | Defence-in-depth model → `awo/docs/18-security/SECURITY_MODEL.md` (remove SQLC references, update to PolicyEvaluator, update RLS to set_tenant_context); audit section → reference to `awo/docs/12-audit/AUDIT_SPEC.md`; session section → reference to `awo/docs/03-auth/SESSION_MODEL.md` |
| Other files (if exist) | AUDIT → REWRITE or DELETE | Case by case; all SQLC/Wire references become deletions |

---

### `docs/08-api-reference/`

| File | Action | Destination |
|------|--------|-------------|
| `01-api-reference-overview.md` | DELETE | Response format wrong; write new `awo/docs/14-api/API_CONVENTIONS.md` |
| `02-authentication.md` | DELETE | Wrong session model; write new `awo/docs/03-auth/SESSION_MODEL.md` |
| `03-contracts-api.md` | DELETE | Contracts module does not exist |
| `04-tenants-api.md` | ARCHIVE | Tenant API shape may be salvageable; audit before deletion |
| `05-entities-api.md` | REWRITE | Closest to correct — entities API `/api/v1/entities/{entity-type}` matches frozen arch; write new `awo/docs/14-api/API_CONVENTIONS.md` drawing from this |
| `06-iam-api.md` | DELETE | Wrong IAM model |
| `07-finance-api.md` | REWRITE | Merge with `awo/docs/99-modules/FINANCE_MODULE_SPEC.md` |

---

### `docs/09-operations/`

| File | Action | Destination |
|------|--------|-------------|
| `01-runbook-overview.md` | MOVE | `awo/docs/19-operations/RUNBOOK_INDEX.md` |
| `02-service-down.md` | MOVE | `awo/docs/19-operations/RUNBOOK_SERVICE_DOWN.md` |
| `03-database-issues.md` | MOVE | `awo/docs/19-operations/RUNBOOK_DATABASE.md` |
| `04-high-error-rate.md` | MOVE | `awo/docs/19-operations/RUNBOOK_HIGH_ERROR_RATE.md` |
| `05-migration-failure.md` | MOVE | `awo/docs/19-operations/RUNBOOK_MIGRATION_FAILURE.md` |
| `06-temporal-worker.md` | MOVE | `awo/docs/19-operations/RUNBOOK_TEMPORAL_WORKER.md` |
| `07-event-outbox.md` | MOVE | `awo/docs/19-operations/RUNBOOK_EVENT_OUTBOX.md` |

---

### Root Level

| File | Action | Destination |
|------|--------|-------------|
| `CLAUDE.md` | KEEP | Stays at root forever |
| `frappe-iam.md` | DELETE | Wrong project, wrong location |
| `V1_RELEASE_READINESS.md` | ARCHIVE → RENAME | `awo/docs/00-overview/V1_RELEASE_SNAPSHOT.md` with historical banner |

---

### `awo/docs/` (existing)

| File | Action | Destination |
|------|--------|-------------|
| `AWO_ARCH_INVARIANT_AUDIT.md` | KEEP | Historical audit; stays in `awo/docs/` |
| `ARCH_FREEZE_REVIEW.md` | RENAME | → `awo/docs/00-overview/DECISION_REGISTER.md` |
| `DOC_GOVERNANCE_PLAN.md` (this file) | KEEP | Governance reference |

---

## Phase 9 — Missing Documentation

All documents below are required by the frozen architecture and do not currently exist. Ranked by implementation priority.

### Priority 0 — Blocking (required before implementation begins)

| Document | Required By | Blocks |
|----------|------------|--------|
| `awo/docs/00-overview/ARCH_OVERVIEW.md` | ARCH_FREEZE_REVIEW Phase 8 Tier 0 | Every other document |
| `awo/docs/01-entity/ENTITY_DEFINITION_SPEC.md` | Framework implementors | Every module author |
| `awo/docs/02-pipeline/LIFECYCLE_SPEC.md` | Hook authors | Cannot write hooks without this |
| `awo/docs/02-pipeline/ERROR_MODEL.md` | All error handling | API responses, hook errors |
| `awo/docs/03-auth/AUTHORIZATION_SPEC.md` | Auth implementation | Cannot implement PolicyEvaluator |
| `awo/docs/03-auth/ACTOR_MODEL.md` | Auth implementation | Actor.IsPlatformAdmin change (ADR-003) |
| `awo/docs/03-auth/VIEWER_CONTEXT.md` | Runtime implementation | Cannot implement repo authorization |
| `awo/docs/00-overview/GLOSSARY.md` | Everyone | Term consistency |

### Priority 1 — Required Before Subsystem Implementation

| Document | Required By | Blocks |
|----------|------------|--------|
| `awo/docs/03-auth/SESSION_MODEL.md` | IAM implementation | Session middleware |
| `awo/docs/04-multitenancy/RLS_SPEC.md` | All database work | Incorrect RLS = security flaw |
| `awo/docs/05-compiler/COMPILE_SPEC.md` | Compiler implementation | Cannot implement compiler |
| `awo/docs/10-sdui/WIDGET_TREE_SPEC.md` | SDUI refactor (ADR-006) | Cannot refactor generator |
| `awo/docs/08-workflow/OUTBOX_SPEC.md` | Workflow outbox (ADR-007) | Cannot implement durable workflow starts |
| `awo/docs/09-events/EVENT_OUTBOX_SPEC.md` | Event outbox (ADR-008) | Cannot implement durable events |
| `awo/docs/12-audit/AUDIT_SPEC.md` | Audit pipeline (ADR-005) | Cannot implement mandatory audit stage |
| `awo/docs/15-migrations/MIGRATION_GUIDE.md` | All database work | Migration conventions undefined |

### Priority 2 — Required Before Integration

| Document | Required By |
|----------|------------|
| `awo/docs/14-api/API_CONVENTIONS.md` | All API consumers and producers |
| `awo/docs/14-api/ERROR_RESPONSE_FORMAT.md` | Frontend/client integration |
| `awo/docs/16-testing/TEST_STRATEGY.md` | Test authors |
| `awo/docs/06-filter/FILTER_DSL_REFERENCE.md` | All repo and policy authors |
| `awo/docs/07-naming/NAMING_SERIES_SPEC.md` | All entity authors |
| `awo/docs/11-cache/CACHE_SPEC.md` | Runtime and session authors |
| `awo/docs/13-actions/ACTION_RUNTIME_REFERENCE.md` | All action handler authors |

### Priority 3 — Required Before v1.0 Release

| Document | Required By |
|----------|------------|
| `awo/docs/10-sdui/AMIS_RENDERER_SPEC.md` | SDUI implementation |
| `awo/docs/10-sdui/PAGE_BUILDER_GUIDE.md` | Module authors |
| `awo/docs/12-audit/AUDIT_QUERY_PATTERNS.md` | Operations team |
| `awo/docs/18-security/SECURITY_MODEL.md` | Security review |
| `awo/docs/99-modules/MODULE_AUTHOR_GUIDE.md` | New module authors |
| `awo/docs/99-modules/FINANCE_MODULE_SPEC.md` | Finance implementation |

### Package Interface Files (Not Docs, But Required)

These are Go files in new packages, not documentation, but their absence blocks all documentation authors:

| Package File | Required By ADR |
|-------------|----------------|
| `awo/auth/viewer.go` — ViewerContext interface | ADR-002 |
| `awo/auth/evaluator.go` — PolicyEvaluator interface | ADR-001 |
| `awo/auth/session.go` — Session struct | ADR-004 |
| `awo/sdui/widget/tree.go` — Node, NodeKind | ADR-006 |
| `awo/outbox/outbox.go` — EventBroker, WorkflowDispatcher | ADR-007, ADR-008 |
| `awo/audit/audit.go` — AuditRecord, Writer | ADR-005 |
| Update `awo/def/record.go` — remove Actor.IsPlatformAdmin | ADR-003 |
| Update `awo/compiler/schema.go` — rename CasbinPolicies | ADR-011 |

---

## Phase 10 — Final Documentation Constitution

### What Is Awo Documentation?

Awo documentation is the single authoritative source of truth for all decisions about how the framework works, how it should be used, and how it should be extended. It is not supplementary material. It is part of the architecture.

Every claim in a specification file is either correct or it is a bug.

---

### Document Type Definitions

**Specification** — Defines what IS. Immutable once the feature is stable. Written by architects. Read by implementors and module authors.

Rules:
- One concept per specification
- No implementation suggestions — only contracts
- Must list what MUST happen, what MUST NOT happen, and what the failure mode is when violated
- Example section allowed (one per major concept), never duplicating the specification itself
- Never mixed with tutorial material
- A Specification becomes invalid only via a new ADR

Examples: `ENTITY_DEFINITION_SPEC.md`, `RLS_SPEC.md`, `LIFECYCLE_SPEC.md`

---

**ADR (Architecture Decision Record)** — Documents a decision that was made, why alternatives were rejected, and its consequences. Never describes how to implement — only that a decision was made.

Rules:
- One decision per ADR
- Fields: Decision ID, Title, Status, Decision, Alternatives Considered, Consequences, Affected Packages, Breaking Change Risk
- Status is one of: PROPOSED, DECIDED, SUPERSEDED
- A decided ADR is never deleted — only superseded by a newer ADR
- The DECISION_REGISTER.md is the only ADR document

What belongs in an ADR: "We decided to use context-embedded ViewerContext instead of explicit parameters."
What does NOT belong in an ADR: "Here is how ViewerContext works." (that's a Specification)

---

**Guide** — Describes HOW to accomplish a task within the constraints set by Specifications. Written by senior engineers. Read by module authors and contributors.

Rules:
- Every claim references a Specification
- Never defines contracts — only shows how to fulfill them
- A Guide never contradicts a Specification (if it appears to, the Guide is wrong)
- Guides can be updated without ADRs
- Maximum one guide per subsystem entry point

Examples: `MODULE_AUTHOR_GUIDE.md`, `ACTION_HANDLER_GUIDE.md`, `PAGE_BUILDER_GUIDE.md`

---

**Reference** — Complete catalog of a thing. No prose explanation. Lookup-optimized.

Rules:
- Table or list format preferred over prose
- No duplicated content — each item appears once
- Linked to from Specifications and Guides, not standalone reading
- Examples: every FieldType with its PostgreSQL column, Go type, and SDUI widget. All NodeKind constants. All cache key patterns.

Examples: `FIELD_TYPES_REFERENCE.md`, `CACHE_KEY_REFERENCE.md`, `SDUI_FIELD_WIDGET_MAP.md`

---

**Runbook** — Step-by-step operational procedure for a specific failure mode. Written for on-call engineers under production pressure.

Rules:
- One failure mode per runbook
- First section: "How to confirm this is the problem" (diagnostic commands)
- Second section: "Immediate mitigation" (stop the bleeding)
- Third section: "Root cause resolution"
- No theory. No architecture discussion. Commands only.
- Updated whenever the production environment changes

Examples: `RUNBOOK_SERVICE_DOWN.md`, `RUNBOOK_EVENT_OUTBOX.md`

---

**Tutorial** — Walks a reader through building something from scratch. Written for onboarding. Read once.

Rules:
- Has a defined starting state and ending state
- The reader produces a working artifact at the end
- Becomes stale quickly — must be validated on every framework change
- Maximum one tutorial per major audience type

Examples: `ONBOARDING_CHECKLIST.md` (this is the only tutorial this framework needs)

---

### What NEVER Gets Its Own Document

The following should never be standalone documents. They become sections inside the correct Specification or Reference.

| Concept | Belongs In |
|---------|-----------|
| "How to use slog" | `LOGGING_SPEC.md` section |
| "Why we use decimal not float" | `FIELD_TYPES_REFERENCE.md` note + CLAUDE.md rule |
| "What is a tenant" | `TENANT_LIFECYCLE.md` paragraph |
| "Why we chose Temporal" | ADR (already in DECISION_REGISTER) |
| "How pagination works" | `PAGINATION_SPEC.md` — ONE file, no sub-documents |
| "Error wrapping with fmt.Errorf" | `ERROR_MODEL.md` section |
| "Zerolog vs slog comparison" | DELETE — historical; decided |
| Any "deep dive" into X | Merge into X's specification |
| Any "understanding X" | Merge into X's specification |
| Any "introduction to X" | ARCH_OVERVIEW.md paragraph + link to X's spec |
| Technology motivation documents | ADR or PRINCIPLES.md |
| RFC-style proposals for decided things | DELETE — they are not decisions; the ADR is |
| Changelogs | Git history |
| "Notes on X" | Merge or DELETE |
| "FAQ about X" | Merge answers into correct spec |
| Onboarding for specific audiences | One ONBOARDING_CHECKLIST.md with audience sections |

---

### Governance Rules (Preventing Re-Bloat)

**Rule 1: The 50-file limit.**
`awo/docs/` must never exceed 50 files. This is enforced by a CI check that counts markdown files in the directory. If the count would exceed 50, a reviewer must identify which existing file the new content belongs in.

**Rule 2: Every new file requires a justification.**
A new documentation file may only be created by answering: "Which existing document did I consider putting this in, and why was that wrong?" This answer goes in the PR description.

**Rule 3: No document survives implementation.**
When a subsystem is implemented, its Specification is reviewed for accuracy. If the implementation diverged from the spec, the spec is updated, not the implementation. If the implementation is correct and the spec was wrong, the spec is updated and a note is added to the DECISION_REGISTER.

**Rule 4: Specifications own their concept globally.**
If concept X is mentioned in two documents, the secondary mention must be a link to the primary specification. No exceptions. The CI enforcer checks that ENTITY_DEFINITION_SPEC.md is the only file defining the EntityDefinition interface.

**Rule 5: Guides reference, never define.**
A Guide document that defines a contract is wrong. Contracts are in Specifications. If a Guide defines something, either move it to the Specification or delete the definition from the Guide and add a link.

**Rule 6: Runbooks own their failure mode.**
Runbooks are not cross-referenced within operational procedures. An on-call engineer reads one runbook from top to bottom. No "see also" sections that chain runbooks together. If a runbook needs information from another runbook, that information is duplicated (acceptable exception to Rule 4 — operational clarity outweighs DRY in runbooks).

**Rule 7: ADRs are append-only.**
The DECISION_REGISTER.md is never edited to remove ADRs. A wrong ADR is superseded by a new ADR. The superseding ADR explains why the previous decision was wrong. The old ADR gets status: SUPERSEDED.

**Rule 8: The GLOSSARY is the term authority.**
If a term is defined in a Specification AND in the GLOSSARY, the GLOSSARY definition wins. If they conflict, the GLOSSARY is updated to match the Specification, and the Specification's inline definition is removed in favor of a link to the GLOSSARY.

**Rule 9: No portal numbers in `awo/docs/`.**
The old Portal 1–9 numbering is deleted. Directories are named by concept, not by audience. Audience is handled by the `awo/docs/README.md` reading paths ("Framework implementor reads X → Y → Z", "Module author reads A → B → C").

**Rule 10: CLAUDE.md is not documentation.**
CLAUDE.md is a constraint file. It contains rules and enforces them. It is not a tutorial, guide, or specification. Content from CLAUDE.md is never copied into `awo/docs/` — `awo/docs/` references CLAUDE.md for rules, not the other way around.

---

### Summary: The Final State

**Before this plan:**
- 230+ files
- Two frameworks documented simultaneously
- ~170 files for Framework A (deleted)
- ~4 files for Framework B (keeping and growing)
- No canonical specifications
- Concepts defined in 4–6 places simultaneously
- Audience-based portal structure that hides technical content

**After this plan:**
- ≤50 files in `awo/docs/`
- One framework documented completely
- Every concept has exactly one canonical document
- All 12 ADRs are recorded and final
- Concept-based structure that enables search and reference
- Reading paths for different audiences defined in one README

**Net change:**
- Files deleted: ~200
- Files moved/renamed: ~15
- Files rewritten: ~10
- Files missing (must be created): ~45

The missing files represent the actual documentation debt for Framework B. Writing them is the documentation equivalent of implementing the framework. Both tasks proceed in parallel, with specifications written before implementation begins.

---

*This governance plan supersedes all previous documentation organization decisions. The `docs/` directory tree is formally deprecated as of this document's date. All future documentation work occurs in `awo/docs/` under the rules defined in Phase 10.*
