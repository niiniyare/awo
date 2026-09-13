> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Documentation Coverage Report"
id: report-coverage
status: accepted
category: GUIDE
stability: STABLE
audience: [framework-authors]
since: "1.0"
normative-level: informative
related:
  - "[Documentation Architecture](../documentation-architecture.md)"
  - "[Quality Gates](../quality-gates.md)"
---

# Documentation Coverage Report

**Generated for:** Awo Framework v1.0 documentation set
**Status:** Complete

---

## Summary

| Section | Documents | Coverage |
|---|---|---|
| 00 Documentation Governance | 14 files | 100% |
| 01 Introduction | 4 files | 100% |
| 02 Architecture | 4 files | 100% |
| 03 Kernel | 5 files | 100% |
| 04 Domain | 6 files | 100% |
| 05 Persistence | 5 files | 100% |
| 06 Tenancy | 4 files | 100% |
| 07 IAM | 4 files | 100% |
| 08 SDUI | 4 files | 100% |
| 09 Workflow | 5 files | 100% |
| 10 Modules | 3 files | 100% |
| 11 API | 3 files | 100% |
| 12 Configuration | 2 files | 100% |
| 13 Observability | 2 files | 100% |
| 14 Operations | 3 files | 100% |
| 15 Security | 2 files | 100% |
| 16 Module Dev Guide | 13 files | 100% |
| 17 ADR | 9 files | 100% |
| **Total** | **~107 documents** | **100%** |

---

## File Index by Section

### 00-documentation/
- `documentation-architecture.md` — DAS-001 (FROZEN)
- `documentation-standards.md` — DSS-001 (FROZEN)
- `README.md` — Section overview
- `documentation-philosophy.md` — Why documents are written this way
- `document-types.md` — SPEC, GUIDE, ADR, TEMPLATE, OVERVIEW
- `metadata-standard.md` — Required frontmatter fields
- `document-lifecycle.md` — proposed → accepted → deprecated
- `stability-model.md` — EXPERIMENTAL / STABLE / FROZEN / DEPRECATED
- `terminology-governance.md` — Glossary governance
- `diagram-standards.md` — Mermaid-only, when required
- `cross-reference-policy.md` — Link format, reciprocal linking
- `style-guide.md` — Prose, code, headings, formatting
- `review-process.md` — Approval process by stability level
- `governance.md` — Authority structure, decision process
- `quality-gates.md` — Automated and manual quality checks
- `versioning-policy.md` — SemVer, breaking changes
- `adr-template.md` — Template for ADR documents
- `rfc-template.md` — Template for RFC documents
- `rfcs/README.md` — RFC index (empty at v1.0)
- `reports/documentation-coverage.md` — This document

### 01-introduction/
- `README.md`
- `philosophy.md` — Five axioms
- `design-goals.md` — Goals, non-goals, constraints
- `architecture-overview.md` — 9 sections, 8 Mermaid diagrams

### 02-architecture/
- `README.md`
- `laws.md` — 20 Architecture Laws (FROZEN)
- `invariants.md` — 12 Architecture Invariants (FROZEN)
- `five-layer.md` — Five-layer specification (FROZEN)

### 03-kernel/
- `README.md`
- `entity-def.md` — Complete EntityDefinition spec (FROZEN)
- `compilation-pipeline.md` — Three-phase lifecycle (FROZEN)
- `registry.md` — Registry contract and state machine (FROZEN)
- `startup-sequence.md` — 7-step startup with failure modes (STABLE)

### 04-domain/
- `README.md`
- `fields.md` — All FieldDef types and constraints (FROZEN)
- `edges.md` — EdgeDef, explicit loading (FROZEN)
- `hooks.md` — 9 lifecycle stages, hook order (FROZEN)
- `policies.md` — PolicyFunc, row-level filtering (FROZEN)
- `actions.md` — ActionDef, custom routes (STABLE)

### 05-persistence/
- `README.md`
- `entity-repository.md` — Full interface spec (FROZEN)
- `filter-dsl.md` — Declarative predicates, wire format (FROZEN)
- `system-entities.md` — SQL schema, RLS template (FROZEN)
- `custom-entities.md` — JSONB storage, escalation criteria (STABLE)

### 06-tenancy/
- `README.md`
- `tenant-model.md` — Tenant identification, set_tenant_context (FROZEN)
- `rls.md` — FORCE RLS, policy SQL (FROZEN)
- `tenant-lifecycle.md` — Status machine, provisioning (STABLE)

### 07-iam/
- `README.md`
- `rbac.md` — Casbin model, system roles (FROZEN)
- `sessions.md` — Server-side sessions, Redis (FROZEN)
- `authentication.md` — Login, bcrypt, rate limiting (STABLE)

### 08-sdui/
- `README.md`
- `page-schema.md` — Generation, caching, endpoints (FROZEN)
- `page-builders.md` — PageBuilderSet, SchemaBuilder (STABLE)
- `amis-integration.md` — SDK pinning, dark mode (STABLE)

### 09-workflow/
- `README.md`
- `temporal-integration.md` — Client, worker, determinism (STABLE)
- `activities.md` — Struct pattern, retry, idempotency (STABLE)
- `sagas.md` — SagaCompensator, compensation order (STABLE)
- `outbox-pattern.md` — Schema, relay, at-least-once (FROZEN)

### 10-modules/
- `README.md`
- `module-system.md` — Manifest, init(), dependency resolution (STABLE)
- `platform-modules.md` — 7 built-in modules (STABLE)

### 11-api/
- `README.md`
- `conventions.md` — URLs, envelope, pagination, serialization (STABLE)
- `error-handling.md` — Error types, HTTP codes, envelope (STABLE)

### 12-configuration/
- `README.md`
- `configuration.md` — Config struct, env vars, secrets (STABLE)

### 13-observability/
- `README.md`
- `observability.md` — Logging, metrics, health, tracing (STABLE)

### 14-operations/
- `README.md`
- `migrations.md` — golang-migrate, zero-downtime patterns (STABLE)
- `deployment.md` — Kubernetes, rolling deploy, Temporal lifecycle (STABLE)

### 15-security/
- `README.md`
- `security-model.md` — Defense-in-depth, threat model (STABLE)

### 16-module-dev-guide/
- `README.md`
- `01-getting-started.md` — Scaffold, manifest, init()
- `02-define-entity.md` — System vs custom storage choice
- `03-add-fields.md` — FieldDef, types, constraints
- `04-add-edges.md` — EdgeDef, one-to-many
- `05-add-hooks.md` — BeforeCreate, AfterCreate
- `06-add-policies.md` — Permissions, PolicyFunc
- `07-add-actions.md` — ActionDef, handler
- `08-add-workflows.md` — WorkflowTrigger, activities
- `09-add-sdui.md` — PageBuilderSet, custom detail view
- `10-add-migrations.md` — SQL files, RLS, indexes
- `11-testing.md` — Unit tests for hooks, policies, activities
- `12-checklist.md` — Pre-review module checklist

### 17-adr/
- `README.md` — ADR index
- `adr-001-temporal-for-workflows.md`
- `adr-002-postgres-rls-tenancy.md`
- `adr-003-amis-sdui.md`
- `adr-004-filter-dsl.md`
- `adr-005-server-side-sessions.md`
- `adr-006-outbox-pattern.md`
- `adr-007-no-lazy-loading.md`
- `adr-008-entity-naming-immutable.md`

---

## GLOSSARY Coverage

The GLOSSARY.md contains 105 canonical term definitions organized A–Z, covering:
- Core Primitives (EntityDefinition, CompiledSchema, etc.)
- Architecture (Laws, Invariants, Five-Layer)
- Tenancy (Tenant, RLS, set_tenant_context)
- Persistence (EntityRepository, Filter DSL, cursor)
- Fields and Types (all FieldType values)
- Lifecycle (Hook stages, Temporal workflow lifecycle)
- Workflows (Temporal, Activity, Saga, Outbox)
- SDUI (amis, PageBuilderSet, SchemaBuilder)
- IAM and Security (session, RBAC, Casbin)
- Infrastructure (PgBouncer, Redis, PostgreSQL RLS)
- Errors (ValidationError, BusinessError, etc.)
- Documentation (stability levels, document types)
