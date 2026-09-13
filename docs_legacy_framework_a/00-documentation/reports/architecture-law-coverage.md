> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Architecture Law Coverage Report"
id: report-law-coverage
status: accepted
category: GUIDE
stability: STABLE
audience: [framework-authors]
since: "1.0"
normative-level: informative
related:
  - "[Architecture Laws](../../02-architecture/laws.md)"
  - "[Architecture Invariants](../../02-architecture/invariants.md)"
  - "[Documentation Coverage](documentation-coverage.md)"
---

# Architecture Law Coverage Report

**Generated for:** Awo Framework v1.0 documentation set

This report verifies that every Architecture Law and Invariant is enforced in at least one normative document beyond its primary def.

---

## Architecture Laws

| Law | Title | Primary Definition | Enforced In |
|---|---|---|---|
| LAW-001 | def/ has no upward imports | laws.md | five-layer.md, module-system.md |
| LAW-002 | Runtime consumes only CompiledSchema | laws.md | compilation-pipeline.md, registry.md |
| LAW-003 | Registry sealed after compilation | laws.md | registry.md, startup-sequence.md |
| LAW-004 | CompiledSchema immutable | laws.md | compilation-pipeline.md |
| LAW-005 | No store op without tenant context | laws.md | tenant-model.md, rls.md |
| LAW-006 | Outbox + entity commit atomically | laws.md | outbox-pattern.md, entity-def.md |
| LAW-007 | Hook order deterministic | laws.md | hooks.md |
| LAW-008 | Layer imports strictly downward | laws.md | five-layer.md |
| LAW-009 | Driver interfaces stable after v1.0 | laws.md | registry.md |
| LAW-010 | Content hash is schema identity | laws.md | compilation-pipeline.md, page-schema.md |
| LAW-011 | Entity names globally unique, immutable | laws.md | entity-def.md, adr-008 |
| LAW-012 | Migrations append-only | laws.md | migrations.md |
| LAW-013 | Sensitive fields never logged | laws.md | fields.md, security-model.md |
| LAW-014 | Filter wire format versioned | laws.md | filter-dsl.md |
| LAW-015 | Database enforces tenant isolation (FORCE RLS) | laws.md | rls.md, adr-002 |
| LAW-016 | Workflow ID format canonical | laws.md | temporal-integration.md |
| LAW-017 | Cross-module hooks need HookPolicy.Open | laws.md | hooks.md |
| LAW-018 | Outbox table framework-private | laws.md | outbox-pattern.md, adr-006 |
| LAW-019 | Schema identical across instances | laws.md | deployment.md |
| LAW-020 | All public APIs carry stability annotations | laws.md | stability-model.md |

**Coverage: 20/20 laws enforced in at least one non-primary document.**

---

## Architecture Invariants

| Invariant | Title | Primary Definition | Enforced In |
|---|---|---|---|
| INV-001 | All tenant-scoped queries execute under RLS | invariants.md | rls.md, tenant-model.md |
| INV-002 | CompiledSchema fixed before first request | invariants.md | startup-sequence.md, compilation-pipeline.md |
| INV-003 | Hook execution order consistent | invariants.md | hooks.md |
| INV-004 | Every committed mutation has audit log entry | invariants.md | platform-modules.md |
| INV-005 | Every committed mutation has outbox entry (when triggers declared) | invariants.md | outbox-pattern.md |
| INV-006 | Sensitive fields absent from logs/errors/responses | invariants.md | fields.md, security-model.md |
| INV-007 | Session validation always requires live Redis | invariants.md | sessions.md, adr-005 |
| INV-008 | All monetary arithmetic uses exact decimal | invariants.md | fields.md, system-entities.md |
| INV-009 | Content hash identical across all instances | invariants.md | compilation-pipeline.md, deployment.md |
| INV-010 | No stack trace in client-facing error response | invariants.md | error-handling.md, security-model.md |
| INV-011 | Middleware pipeline executes in full | invariants.md | api/conventions.md |
| INV-012 | Entity names stable after first migration | invariants.md | entity-def.md, adr-008 |

**Coverage: 12/12 invariants enforced in at least one non-primary document.**

---

## ADR Coverage

| ADR | Decision | Documents Affected |
|---|---|---|
| ADR-001 | Temporal for workflows | temporal-integration.md, activities.md, sagas.md |
| ADR-002 | PostgreSQL RLS for tenancy | rls.md, tenant-model.md |
| ADR-003 | amis for SDUI | page-schema.md, page-builders.md, amis-integration.md |
| ADR-004 | Declarative Filter DSL | filter-dsl.md |
| ADR-005 | Server-side sessions over JWT | sessions.md, authentication.md |
| ADR-006 | Transactional outbox | outbox-pattern.md |
| ADR-007 | No lazy loading | edges.md, entity-repository.md |
| ADR-008 | Entity names immutable | entity-def.md |

**Coverage: All 8 ADRs reference their corresponding specification documents.**
