# New Module Author — Onboarding Checklist

**Classification:** Reference — Tier 2
**Owner:** `99-modules/ONBOARDING_CHECKLIST.md`
**Status:** Living document

---

## Purpose

Reading order and checklist for engineers building a new Awo Framework module for the first time.

---

## Phase 1 — Constitutional Documents (Read First, Non-Negotiable)

Read in this exact order. Do not write any code before completing this phase.

- [ ] `CLAUDE.md` — Critical rules and quick reference (at repo root)
- [ ] [`00-overview/DECISION_REGISTER.md`](../00-overview/DECISION_REGISTER.md) — All 12 ADRs. Understand every architectural decision.
- [ ] [`00-overview/ARCH_OVERVIEW.md`](../00-overview/ARCH_OVERVIEW.md) — 5-layer architecture, startup sequence, package DAG
- [ ] [`00-overview/PRINCIPLES.md`](../00-overview/PRINCIPLES.md) — Design principles that govern all decisions
- [ ] [`00-overview/GLOSSARY.md`](../00-overview/GLOSSARY.md) — Every term defined; read when you encounter an unfamiliar concept

---

## Phase 2 — Entity Primitives (Before Any Entity Definition)

- [ ] [`01-entity/ENTITY_DEFINITION_SPEC.md`](../01-entity/ENTITY_DEFINITION_SPEC.md) — The central primitive; understand all 14 methods
- [ ] [`01-entity/FIELD_TYPES_REFERENCE.md`](../01-entity/FIELD_TYPES_REFERENCE.md) — Every field type, PostgreSQL mapping, Go type
- [ ] [`01-entity/EDGE_TYPES_REFERENCE.md`](../01-entity/EDGE_TYPES_REFERENCE.md) — EdgeOneToMany / EdgeManyToOne semantics
- [ ] [`01-entity/NAMING_CONVENTIONS.md`](../01-entity/NAMING_CONVENTIONS.md) — Entity naming, field naming, action naming

---

## Phase 3 — Lifecycle and Auth (Before Writing Hooks)

- [ ] [`02-pipeline/LIFECYCLE_SPEC.md`](../02-pipeline/LIFECYCLE_SPEC.md) — All 9 pipeline stages, TX boundaries
- [ ] [`02-pipeline/HOOK_CONTRACT.md`](../02-pipeline/HOOK_CONTRACT.md) — Hook interfaces, execution order, recursion policy
- [ ] [`02-pipeline/ERROR_MODEL.md`](../02-pipeline/ERROR_MODEL.md) — ValidationError, BusinessError, wrapping protocol
- [ ] [`03-auth/AUTHORIZATION_SPEC.md`](../03-auth/AUTHORIZATION_SPEC.md) — PermissionSet, PolicyEvaluator
- [ ] [`03-auth/RBAC_ROLES_REFERENCE.md`](../03-auth/RBAC_ROLES_REFERENCE.md) — Role naming for your module

---

## Phase 4 — Infrastructure (Before Migrations)

- [ ] [`04-multitenancy/RLS_SPEC.md`](../04-multitenancy/RLS_SPEC.md) — RLS setup is mandatory on every tenant-scoped table
- [ ] [`15-migrations/MIGRATION_GUIDE.md`](../15-migrations/MIGRATION_GUIDE.md) — File naming, up/down pairs, zero-downtime patterns
- [ ] [`15-migrations/RLS_TABLE_TEMPLATE.md`](../15-migrations/RLS_TABLE_TEMPLATE.md) — Copy this template for every new table
- [ ] [`15-migrations/MIGRATION_CHECKLIST.md`](../15-migrations/MIGRATION_CHECKLIST.md) — Run through this before every migration PR

---

## Phase 5 — Feature-Specific Docs (Read When Needed)

| Feature | Documents |
|---------|-----------|
| Custom actions | [`13-actions/ACTION_HANDLER_GUIDE.md`](../13-actions/ACTION_HANDLER_GUIDE.md), [`13-actions/ACTION_RUNTIME_REFERENCE.md`](../13-actions/ACTION_RUNTIME_REFERENCE.md) |
| Filter DSL | [`06-filter/FILTER_DSL_REFERENCE.md`](../06-filter/FILTER_DSL_REFERENCE.md) |
| Domain events | [`09-events/EVENT_OUTBOX_SPEC.md`](../09-events/EVENT_OUTBOX_SPEC.md), [`09-events/DOMAIN_EVENTS_REFERENCE.md`](../09-events/DOMAIN_EVENTS_REFERENCE.md) |
| Temporal workflows | [`08-workflow/TEMPORAL_INTEGRATION.md`](../08-workflow/TEMPORAL_INTEGRATION.md), [`08-workflow/OUTBOX_SPEC.md`](../08-workflow/OUTBOX_SPEC.md) |
| Naming series | [`07-naming/NAMING_SERIES_SPEC.md`](../07-naming/NAMING_SERIES_SPEC.md) |
| Custom SDUI | [`10-sdui/WIDGET_TREE_SPEC.md`](../10-sdui/WIDGET_TREE_SPEC.md), [`10-sdui/PAGE_BUILDER_GUIDE.md`](../10-sdui/PAGE_BUILDER_GUIDE.md) |
| Audit log | [`12-audit/AUDIT_SPEC.md`](../12-audit/AUDIT_SPEC.md) |
| Cache | [`11-cache/CACHE_SPEC.md`](../11-cache/CACHE_SPEC.md) |
| Testing | [`16-testing/TEST_STRATEGY.md`](../16-testing/TEST_STRATEGY.md), [`16-testing/HOOK_TEST_PATTERNS.md`](../16-testing/HOOK_TEST_PATTERNS.md) |

---

## Phase 6 — Module Development

- [ ] Read [`99-modules/MODULE_AUTHOR_GUIDE.md`](MODULE_AUTHOR_GUIDE.md) — Directory structure, registration, checklist
- [ ] Read [`99-modules/FINANCE_MODULE_SPEC.md`](FINANCE_MODULE_SPEC.md) — Study the canonical example before writing your own

---

## Module PR Readiness Checklist

Before opening a PR for a new module:

- [ ] All entities follow `{module}_{noun}` naming
- [ ] All `SystemDefinition` entities have `.up.sql` + `.down.sql` migration pairs
- [ ] All migrations use RLS template (`ENABLE RLS`, `FORCE RLS`, `CREATE POLICY`)
- [ ] `def.Register()` called only in `init()`
- [ ] Module blank-imported in `cmd/server/main.go`
- [ ] Currency fields use `numeric(20,4)` in SQL
- [ ] `AuditEnabled: true` on all financial/IAM entities
- [ ] Unit tests: every hook, every policy function
- [ ] Integration tests: entity CRUD against real PostgreSQL
- [ ] `PermissionSet` on every entity uses permission identifiers (`{module}.{entity}.{action}`) — no `role:` strings
- [ ] Role-to-permission mappings for module added to [`03-auth/RBAC_ROLES_REFERENCE.md`](../03-auth/RBAC_ROLES_REFERENCE.md)
- [ ] Domain events documented in [`09-events/DOMAIN_EVENTS_REFERENCE.md`](../09-events/DOMAIN_EVENTS_REFERENCE.md)
- [ ] Migration checklist signed off: [`15-migrations/MIGRATION_CHECKLIST.md`](../15-migrations/MIGRATION_CHECKLIST.md)

---

## Anti-Patterns to Avoid

| Anti-Pattern | Correct Pattern |
|-------------|----------------|
| `role:finance.viewer` in `PermissionSet` | Use permission identifier: `"finance.invoice.read"` |
| `WHERE tenant_id = ?` in app code | Trust RLS — never add tenant filter manually |
| `time.Now()` in workflow functions | `workflow.Now(ctx)` |
| `time.Sleep()` in workflow functions | `workflow.Sleep(ctx, d)` |
| Raw SQL in business logic | `ActionEntityRepo` methods |
| Storing money as float | `FieldTypeCurrency` → `decimal.Decimal` |
| `registry.RegisterCustomForTenant` from handler | Only from `init()` |
| Lazy loading edges | Explicit `QueryOption`s |
| Hard-coded secret in code | Environment variable |
| `go test` auto-run by Claude | Tell user to run it |

---

## Questions?

If a specification is ambiguous or missing, check:
1. [`00-overview/DECISION_REGISTER.md`](../00-overview/DECISION_REGISTER.md) — Is there an ADR that covers this?
2. `awo/` source code — What does the existing implementation tell you?
3. `CLAUDE.md` — Is there a CRITICAL RULE that applies?

Do not infer from Framework A docs (`docs/` directory) — they describe a different framework.
