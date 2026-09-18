# Awo Framework — Architecture, Implementation & Competitive-Readiness Audit

**Date:** 2026-09-13/18
**Scope:** Entire repository at `.` (standalone Awo repo, post-extraction from the former monorepo)
**Method:** Direct source/doc reading + running build/vet/test against a real PostgreSQL + Redis instance, cross-validated by independent parallel investigations per subsystem. Every finding below is sourced from source code, SQL, or documentation text actually read during this audit — not from trusting `tasks.md`'s own status claims.
**Baseline test result:** see §0 and §26.

---

## 0. Executive Summary

Awo's core thesis — a Go-native, metadata-driven ERP kernel where one `EntityDefinition` compiles into persistence, API, OpenAPI, SDUI, docs, and authorization — is **real, not decorative**. The compiler → registry → runtime pipeline, the Filter/SQL layer, the SDUI widget-tree/renderer pipeline, the report builder, and the RLS tenant-isolation mechanism are all genuine, tested engineering, not scaffolding. Once the repository is made to build (see below), `go test ./...` passes **100% clean** across ~100 packages, including real-PostgreSQL RLS integration tests.

However, three classes of problem are serious enough to block a "production-grade" claim today:

1. **The repository does not build as committed.** No `go.mod` was carried over in the move out of the monorepo, and both CLI entry points (`cmd/awo`, `cmd/server`) blank-import `awo.so/modules/finance`, a package that does not exist in this repo. This is fixed for this audit's diagnostic purposes (temporary `go.mod`, `module awo.so/awo`, matching all existing `awo.so/awo/...` import paths exactly) but is **not committed** — see §29 Phase 0.
2. **Several "frozen, constitutional" documented security guarantees do not hold in the running code**, most seriously: tenant-ACTIVE-status is not enforced by the single documented enforcement point (`set_tenant_context()`), and is concretely bypassable via the login endpoint; session revocation has a real race that can resurrect a logged-out session; Casbin permission revocation has no effect until process restart; workflow-trigger durability (the documented outbox pattern) doesn't exist; bulk import bypasses audit and validation entirely.
3. **Documentation has drifted badly from implementation in a way that actively misleads.** The "Frozen at v1.0" architecture documents describe roughly a quarter of the packages that exist in the repository today. Two independent, colliding ADR-numbering sequences exist. Several files reference a constitutional source document (`ARCH_FREEZE_REVIEW.md`) and a `CLAUDE.md` that do not exist in the repo.

None of this is a fundamental architecture indictment — the underlying design is sound and, where implemented faithfully, competitive with or better than Odoo/Frappe's equivalent mechanisms (see §26-28). The gap is between what was *designed and documented* and what was *actually wired end-to-end into the running server*. That gap is fixable with focused, dependency-ordered work, tracked in the accompanying `tasks.md`.

**Baseline test result (this audit, after temporary build repair):**
```
go build ./...   → PASS (0 errors), after: (a) go.mod created with `module awo.so/awo`,
                    (b) `_ "awo.so/modules/finance"` temporarily commented out in
                    cmd/awo/cmds_schema.go and cmd/server/main.go (both since reverted —
                    see tasks.md Phase 0)
go vet ./...     → PASS (clean, no findings)
go test ./...    → PASS — 100% of packages, ~100 packages exercised, including real
                    PostgreSQL RLS integration tests (contrib/pgx, testutil/db,
                    platform/iam, platform/audit) against a local PostgreSQL 18 +
                    Redis 7 instance (TEST_DATABASE_URL / DATABASE_URL / REDIS_URL set)
go test -race    → NOT RUNNABLE in this environment (android/arm64 Termux does not
                    support -race); must run in CI on linux/amd64
Coverage         → 51.5% of statements (go tool cover -func, partial run — see §26)
```

---

## 1. Documentation Assessment

Read directly, in full: `docs/README.md` and 24 of the ~70 documents it indexes (00-overview/*, 03-auth/AUTHORIZATION_SPEC + SESSION_SPEC, 04-multitenancy/RLS_SPEC, 12-audit/AUDIT_ARCH-adjacent review docs, 16-testing/TEST_STRATEGY, 18-security/SECURITY_MODEL), plus the four root-level non-`docs/` design documents (`IMPLEMENTATION.md` — actually SDUI-specific despite its generic name, `phaseA.md`, `sdui_architecture.md`, `sdui_implementation.md`) and the explicitly-deprecated `docs_legacy_framework_a/` tree (correctly labeled "MUST NOT be used," superseded by `docs/`).

**Verdict: `docs/` is the intended architectural contract, but it is a snapshot of the framework as it stood at the v1.0 kernel freeze (dated 2026-07-20 per `V1_RELEASE_SNAPSHOT.md`) — roughly 12 packages (`def`, `filter`, `cache`, `outbox`, `audit`, `auth`, `naming`, `registry`, `compiler`, `sdui/widget`, `sdui/amis`, `sdui`, `runtime`). The repository today has 40+ top-level packages** (`platform/*`, `api/*`, `contrib/*`, `driver`, `generator`, `docgen`, `workflow`, `scheduler`, `report`, `ioport`, `module`, `observability`, `config`, `crypto`, `secrets`, `lock`, `tx`, `version`, `sdk`, `cmd/*`, `testing/*`, `testutil`, `tests`, `migration`, `migrations`, `db`, `bootstrap`, `introspect`, `internal`, `perf`, `public`). None of this substantial post-freeze expansion is reflected in `ARCH_OVERVIEW.md`'s package table or `PACKAGE_DEPENDENCY_MAP.md`'s dependency graph — both still marked "Status: Frozen at v1.0" as if current.

The gap is filled, unevenly, by `tasks.md` (dated 2026-08-31) and the SDUI-specific `IMPLEMENTATION.md`/`phaseA.md`/`sdui_architecture.md`/`sdui_implementation.md` — none of which were reconciled back into `docs/`, and which **independently invented a second, colliding ADR-numbering sequence** (see finding P1-1 below).

**Ironic self-violation:** `docs/00-overview/PRINCIPLES.md` Principle 5 states: *"Every architectural concept has exactly one canonical document that defines it... Duplicated documentation is worse than no documentation... The 50-file limit on `awo/docs/` enforces this discipline at the filesystem level."* The repository currently has, alongside the disciplined `docs/` tree: `tasks.md`, `IMPLEMENTATION.md`, `phaseA.md`, `sdui_architecture.md`, `sdui_implementation.md`, and an entire second `docs_legacy_framework_a/` documentation tree — a direct, self-referential violation of the framework's own constitutional principle.

**Positive finding:** the project has already, at least once, practiced exactly the rigor this audit demands. `docs/12-audit/DOC_REVIEW_REPORT.md` and `docs/12-audit/IMPL_READINESS_REPORT.md` (both dated 2026-07-27) are a genuine "verify every claim against source before writing code" pass on the audit subsystem specifically — they found and corrected a real documentation error (ADR-013 wrongly claimed the driver owns transactions; it's actually the service layer) before implementation began, exactly per this audit's Phase 28 "documentation-first rule." The elaborate design that report specified (partitioned `platform_audit_log`, pg_cron maintenance, risk scoring, HMAC session IDs, category-driven failure policy) was **subsequently actually built** (confirmed: `audit/scorer.go`, `db/migrations/000452_platform_audit_log.up.sql`, `platform/audit/migrations/001_platform_audit_log.up.sql`, both containing `PARTITION BY RANGE` and `pg_cron`). This precedent should be generalized as a standing practice (see `tasks.md` Phase 0).

### Documentation vs Implementation — top findings (full list in §29 tasks.md and §9 Security)

| # | Doc claim | Reality | Status | Sev |
|---|---|---|---|---|
| D1 | Two ADR registers exist: `docs/00-overview/DECISION_REGISTER.md` (ADR-001..024, audit-focused from ADR-013) and `tasks.md` (independently invented ADR-020..029, Wire/filter/session-focused) — **same numbers, different decisions** | Both are real, checked-in, "binding" documents | CONTRADICTORY | P1 |
| D2 | `ARCH_OVERVIEW.md`/`PACKAGE_DEPENDENCY_MAP.md` "Frozen at v1.0" package graph (~12 packages) | 40+ top-level packages exist | CONTRADICTORY (stale) | P1 |
| D3 | `docs/README.md` "Constitutional Source: `ARCH_FREEZE_REVIEW.md`... Do not modify" | File does not exist anywhere in the repo | MISSING | P2 |
| D4 | `ARCH_OVERVIEW.md`, `PRINCIPLES.md`, `TEST_STRATEGY.md` all reference `../../CLAUDE.md` as mandatory first-read | File does not exist in the repo | MISSING | P2 |
| D5 | `RLS_SPEC.md`/`SECURITY_MODEL.md`/`GLOBAL_TABLES.md` refer to a `tenants` table and GUC `app.current_tenant_id` | Real table is `platform_tenant`; real GUC is `awo.tenant_id` | CONTRADICTORY | P0 (see §4) |
| D6 | `docs/README.md` §05 links `05-registry/REGISTRY_SPEC.md` | File doesn't exist; a whole unindexed `docs/05-compiler/` (3 files) exists instead | CONTRADICTORY | P3 |
| D7 | `docs/README.md` §04 links `TENANT_RESOLUTION.md` | Actual file is `TENANT_IDENTIFICATION.md` | MISSING (broken link) | P3 |
| D8 | `ENTITY_DEFINITION_SPEC.md` shows 16 methods (per its own ADR-022 note) and `docs/README.md` says 14 | `def/entity.go` interface has 18 methods | CONTRADICTORY (3-way) | P3 |
| D9 | `docs/10-sdui/WIDGET_TREE_SPEC.md` (ADR-006, "Frozen at v1.0") lists 17 `NodeKind` constants | `sdui/widget/node.go` has 39, all genuinely wired end-to-end (no dead metadata) | CONTRADICTORY (doc stale; code is fine) | P2 |
| D10 | `ACTOR_MODEL.md` vs `ACTOR_SPEC.md`, `SESSION_MODEL.md` vs `SESSION_SPEC.md` — near-duplicate pairs at different "Tier" classifications, cross-referencing each other, neither marked deprecated | Both pairs genuinely overlap in scope | UNDOCUMENTED (stale duplicate) | P3 |
| D11 | `docs/16-testing/TEST_STRATEGY.md` documents a `//go:build integration` convention, an `internal/core/{module}/` layout, and a `testdb` package; also states "Per CLAUDE.md: never run `go test`" | Actual repo uses `testutil/db` with env-var skip (`TEST_DATABASE_URL`), flat top-level packages, no build tags; `CLAUDE.md` doesn't exist | CONTRADICTORY (stale) | P3 |

---

## 2. Current Architecture — Verified Lifecycle

```
EntityDefinition (def.SystemDefinition/CustomDefinition, 18-method interface)
   ↓ def.Register() in module init()
Registry (registry.Build/Seal — validates, panics on malformed schema — fail-fast, real)
   ↓
Compiler (compiler.Compile → CompiledSchema: Entities, Routes, CapabilityGrants,
          DependencyOrder via depgraph.go — cycle detection + topo sort, REAL & tested)
   ↓                                    ↓                        ↓
Migration generator                 Runtime Pipeline          SDUI Generator → amis
(generator/generator.go →           (runtime/pipeline.go:      (sdui/generator →
 SQL: tables, RLS policies,          BeforeValidate→Validate→   sdui/widget IR →
 triggers, GIN/trigram idx —         [AUTHORIZE is middleware,  sdui/amis renderer;
 REAL, no diff/rename safety)        not pipeline — doc says    39 NodeKinds, all wired)
   ↓                                 otherwise] → before_save →
db/migrations/*.sql                  PERSIST → RunAuditRecord
   ↓ golang-migrate                  (inside same TX) →
PostgreSQL (RLS via                  after_save hooks →
 FORCE ROW LEVEL SECURITY,           [TX commits] →
 non-superuser awo_app role,         startWorkflows (OUTSIDE TX,
 REAL, tested)                        direct Temporal call — NOT
                                       via the documented outbox,
                                       which doesn't exist — P0)
   ↓
OpenAPI (TWO independent generators: api/openapi/ [live] and
 generator/openapi/ [CLI] — duplicated, drift risk)
   ↓
Docgen (docgen.Generate → Markdown per entity — REAL, tested, missing
 defaults/index sections)
```

**Per-subsystem status** (IMPLEMENTED / TESTED / INTEGRATED / STUBBED / MISSING):

| Subsystem | Status | Evidence |
|---|---|---|
| EntityDefinition → Registry → Compiler → CompiledSchema | IMPLEMENTED, TESTED, INTEGRATED | `compiler/depgraph_test.go` passes; every field (Fields, Edges, Hooks, Permissions, Actions, WorkflowTrigger, Scope, AllowAudit) has a traced consumer |
| Runtime pipeline (validate/authorize/persist/audit/hooks) | IMPLEMENTED, TESTED, INTEGRATED — but AUTHORIZE actually lives in `api/authz` middleware, not the pipeline itself, contrary to `LIFECYCLE_SPEC.md` §1 | `docs/12-audit/IMPL_READINESS_REPORT.md` "What the documentation missed" #4 |
| Filter → SQL | IMPLEMENTED, TESTED, injection-safe (parameterized, allowlisted) | `contrib/pgx/sqlbuild/sqlbuild.go` |
| Migration generation | IMPLEMENTED, TESTED for first-boot schema; NO diff/rename/data-loss-safety for schema evolution | `generator/generator.go` |
| RLS tenant isolation (the mechanism itself) | IMPLEMENTED, TESTED against real PG with non-superuser role | `testutil/db`, `contrib/pgx/rls_defense_test.go` |
| RLS tenant **status** enforcement | STUBBED — documented as DB-layer, actually only a bypassable HTTP middleware | §4, §9 |
| Audit (mutation → transactional audit record) | IMPLEMENTED, TESTED, INTEGRATED for CRUD; **MISSING for BulkCreate/import and custom Actions** | §7 |
| Audit history API (`/:id/history`) | IMPLEMENTED but **not wired in production** (`AuditQueryer` defaults to Noop) | §7 |
| Feature flags / Settings | IMPLEMENTED (platform-native, per ADR-028); precedence chain and SDUI wiring not independently re-verified in this pass — flag as needs-confirmation | tasks.md, not fully re-derived from source in this audit |
| Report builder | IMPLEMENTED — genuine join/aggregate/group/having DSL, not a raw-SQL wrapper | `report/report.go` (461 lines: Joins, GroupBy, Having, 5 aggregate funcs, parameterized filter-to-SQL) |
| Search | PARTIAL/MISSING — `Searchable` fields generate GIN/trigram *indexes* only; no query-side search API/engine exists anywhere in the repo | direct grep, no `search/` package, no ranking/multi-field query surface |
| Workflow trigger dispatch | IMPLEMENTED DIFFERENTLY FROM DOCS — direct synchronous Temporal call, no outbox, contradicts ADR-007 explicitly | §7 |
| Scheduler | IMPLEMENTED but `Cancel()` is decorative (job keeps firing) | §7 |
| Import/Export | IMPLEMENTED, genuinely metadata-driven, but bypasses audit/validation via BulkCreate; missing dry-run/XLSX/link-resolution | §7 |
| SDUI (widget IR → amis) | IMPLEMENTED, TESTED, complete NodeKind coverage (39/39) | `sdui/amis/renderer.go` |
| OpenAPI | IMPLEMENTED, TESTED, but **duplicated** (two generators) | §7 |
| Docgen | IMPLEMENTED, TESTED, missing defaults/index sections | §7 |
| CLI | IMPLEMENTED for schema/generate/docgen/serve/doctor/new; **STUBBED** for migrate/module-list/validate; **ABSENT** entirely for tenant/org/user/role/permission/feature/settings/job/schedule/search/report/import/export/audit ops | §7 |
| Module/entity auto-registration | DEFINED ONLY, unintegrated dead code (`module/` package has zero external callers); real mechanism is 100% hand-edited blank-import lists | §7 |

---

## 3. Entity System Assessment

`EntityDefinition` is a genuine single source of truth for the subsystems it actually feeds (persistence, routes, permissions declaration, migration SQL, OpenAPI, docgen, SDUI). No "decorative metadata" was found in this core — every field checked by the audit (Fields, Edges, Hooks, Actions, WorkflowTrigger, Scope, AllowAudit, Searchable, Sensitive) has a traceable, tested consumer.

Gaps: no dependency-tracked computed+stored field engine (unlike Odoo's `@api.depends`); no entity inheritance mechanism (acceptable — Go idiomatically uses composition via `SystemDefinition`/`CustomDefinition` embedding, not a gap); no schema-diff/rename-safety in the migration generator (P2, real production risk once the team starts evolving schemas at scale — a field rename is indistinguishable from drop+add with no warning).

---

## 4. Multi-Tenancy Assessment — the most serious finding in this audit

**The documented guarantee — "tenant isolation MUST be enforced by the database, not by application code" (Principle 3), with `set_tenant_context()` as "the single RLS enforcement point" that validates tenant existence and `ACTIVE` status — does not hold as implemented.**

The real `set_tenant_context()` (`generator/generator.go:130-134`) is a bare `set_config` call:
```sql
CREATE OR REPLACE FUNCTION set_tenant_context(p_tenant_id uuid) RETURNS void AS $$
BEGIN
    PERFORM set_config('awo.tenant_id', p_tenant_id::text, true);
END;
```
No existence check. No status check. No exception. `testutil/db/db.go`'s test helper reimplements the identical trivial version — meaning **the test suite validates against the weakened behavior, not the documented contract**, so nothing in CI (once CI exists) would catch a regression here even today.

Tenant-ACTIVE enforcement instead lives entirely in one piece of Go middleware, `api/middleware/tenant.go`'s `TenantResolver`, mounted only on the authenticated `/api/v1` route group. **`POST /api/v1/auth/login` is registered directly on the bare Fiber app, outside that group** (`cmd/server/main.go`, `platform/iam/handler.go`). It takes `tenant_id` raw from the request body and calls `AuthService.Login`, which calls `set_tenant_context()` with zero status validation and issues a fully valid, Redis-backed session token.

**Concrete, reproducible bypass:** suspend a tenant (`platform_tenant.status = 'SUSPENDED'`), `POST /api/v1/auth/login` with valid credentials for a user of that tenant → expect a block (402/403 per `TENANT_LIFECYCLE.md`'s own HTTP table), actually returns `200` with a live token. Subsequent business-data calls *are* blocked by `TenantResolver` (bounding the blast radius to "credentials confirmed + live token issued" rather than full data access), but this is a genuine violation of the documented tenant-lifecycle contract, and it proves the database layer provides **zero** defense-in-depth — any other current or future code path that calls `set_tenant_context` for a non-ACTIVE tenant (a background job, a Temporal activity, a CLI command, a new internal RPC) inherits no protection at all.

**Severity: P0.** See `tasks.md` Phase 1 for the fix (move the ACTIVE-status check into `set_tenant_context()` itself, matching the frozen spec exactly, so every caller gets the guarantee for free).

**Organisation hierarchy:** schema and RLS-policy generation for `ScopeOrganization`/`ScopeOrganizationTree` are real (`generator/generator.go`), but `OrganizationService` — the application-layer service meant to provide Create/Move/ResolveScope — is **100% stubbed** (every method returns `"not implemented"`), and `PathComputeHook` silently mis-records every non-root organisation node as a root when exercised via any real (non-test) call path, because it only computes a correct materialized path when a test-only internal field is pre-populated. The org hierarchy feature is non-functional beyond its database scaffolding. Also, `path` is `VARCHAR(4096)` with `LIKE`-prefix matching, not the documented `ltree` type (ADR-012/024) — functionally workable for framework-generated UUID paths (no injection risk found), but without the ltree GiST-index performance/integrity guarantees the ADR promises, and without a declared `CREATE EXTENSION ltree` dependency either way.

**Platform-admin bypass model:** correctly scoped and audit-logged — `IsPlatformAdmin()` bypasses Casbin/RBAC only, never RLS, matching ADR-023 exactly (`api/authz/authz.go:48-58`). The one dangling piece is `runtime/tenant.SystemContext` — dead code (zero production callers) with a misleading doc comment implying a "special platform admin token" RLS-bypass mechanism that doesn't exist; it fails safe (returns zero rows) today, but is a latent risk if a future author "completes" it based on the comment rather than deleting it.

---

## 5. IAM / RBAC / Session Assessment

**Real and correct:** `ViewerContext`/`Actor`/`Session` structs match their ADRs field-for-field; the `PermissionSet`→`CapabilityGrant`→Casbin pipeline cleanly separates declaration from enforcement (ADR-001/011, genuinely engine-agnostic — neither Odoo nor Frappe achieves this separation); session revocation on role-*assignment* change is wired via `UserRoleChangeHook`; the service-account Redis session-indexing bug (BUG-001) is genuinely fixed.

**Two P0/P1-severity gaps found:**

1. **Session revocation race (P0).** Logout deletes the Redis key first (authoritative path), then updates `iam_sessions.revoked_at` in a **separate, best-effort transaction whose failure is logged and silently swallowed**. `ValidateToken`, on a Redis miss, falls back to `recoverSessionFromDB`, trusts PostgreSQL, and **resurrects the session back into Redis**. In the window between the Redis delete and the (possibly-failed) PG commit, a replayed "logged out" token is treated as valid again. Reproduction: force the PG update to fail (or race a concurrent request against Logout) and replay the token — it validates.
2. **Casbin policy reload is never called in production (P1).** `CasbinEvaluator.Reload()` exists and works but has zero call sites outside its own package/tests. Permissions are loaded into the in-memory enforcer once at bootstrap. Revoking a role's permission in `iam_role_permissions` has **no effect until the process restarts** — an unbounded stale-authorization window, not merely a cache-TTL delay.
3. **`/api/v1/auth/logout` and `/api/v1/auth/me` always return 401 (P1).** Both routes are registered without `middleware.RequireAuth` ahead of them, but their handlers read `c.Locals("session")`, which only that middleware populates. Self-service logout is **permanently non-functional** via the documented API; no test exercises the real route wiring (only `AuthService` is unit-tested), so this has never been caught.

---

## 6. Audit Architecture Assessment

The audit subsystem is, ironically, the **most rigorously engineered part of the framework** — and also the source of two of the most important P0 findings, because its scope (mutations) is exactly where the framework's "audit by default" promise is tested hardest.

**What's real:** `AllowAudit()` defaults true (compiled from `DisableAudit bool`, ADR-023); the write is genuinely transactional with the mutation (`EntityService`'s `repo.WithTx` calls `pipeline.RunAuditRecord` between `repo.Create/Update/Delete` and `after_save` hooks — confirmed by reading the actual call sequence, not assumed); category-driven failure policy (ADMIN/SECURITY propagate and roll back the mutation, everything else suppresses+logs+meters) is implemented and tested; sensitive-field stripping happens before diffing, not after; the elaborate storage design (monthly `PARTITION BY RANGE`, pg_cron maintenance function, append-only grants for `awo_app`, HMAC-derived session IDs, `RiskScorer`) was **actually built**, not just planned — confirmed via `audit/scorer.go`, `db/migrations/000452_platform_audit_log.up.sql`, real-PostgreSQL `testify.Suite` integration tests for atomicity and rollback.

**What's broken:**
- **BulkCreate (and therefore all CSV/JSON import) completely bypasses the pipeline — no validation, no hooks, no audit record (P0).** `ioport.Import` → `EntityRepository.BulkCreate` → a raw `pgx.Batch` of INSERTs that never touches `RunAuditRecord` or any hook. Every bulk-imported row is invisible to the audit trail and exempt from required/immutable-field enforcement — a direct violation of the "audit by default" principle for an entire, officially-supported data path.
- **Custom Actions have zero audit coverage from the framework** (confirmed gap, `docs/12-audit/IMPL_READINESS_REPORT.md` OQ-9 — action authors must call `AuditWriter.Write()` explicitly; nothing enforces they do).
- **The `/:id/history` audit-timeline endpoint silently no-ops in production.** `cmd/server/main.go`'s `router.Register()` call never sets `RegisterOptions.AuditQueryer`, so it defaults to `audit.NoopQueryer{}` even though `audit.PoolQueryer` is fully implemented and tested. Every history request against the deployed server returns an empty array, not an error — a silent, undetectable data-availability gap.
- **Two independent migration files both create `platform_audit_log`** — `db/migrations/000452_platform_audit_log.up.sql` (the generator's configured default output directory, per `config/manager.go`'s `migration.dir` default) and `platform/audit/migrations/001_platform_audit_log.up.sql` (module-embedded, its own `migrations.go` loader). Both are real, non-trivial, independently-authored SQL files with the same table definition — if both are ever applied to the same database, the second `CREATE TABLE ... PARTITION BY RANGE` fails outright. Which one is actually live in a real deployment was not conclusively determined in this pass; this must be resolved before any production deployment (P1).

Dependency-graph note: `awo/audit` imports `pgxpool` directly (`audit/queryer.go`), contradicting both `driver/doc.go`'s explicit "framework never imports pgx outside `driver/pgx`" rule and `PACKAGE_DEPENDENCY_MAP.md`'s Level-1 "audit imports only `def`" claim — a real, compiled, load-bearing dependency violation of the "frozen" package graph (P1).

---

## 7. SDUI / OpenAPI / Docgen / CLI / Module Registration Assessment

**SDUI:** genuinely strong. All 39 `NodeKind` constants have real generator paths and real renderer cases — no dead metadata, no silently-dropped UI capability. Permission/sensitive-gated fields are genuinely *absent* from generated schemas (not merely `Hidden: true`), matching the documented contract. Dark-mode via CSS custom-property tokens, not `.cxd-*` overrides, confirmed. The only doc problem is `WIDGET_TREE_SPEC.md` itself being stale (17 vs 39 kinds) despite being marked "Frozen."

**OpenAPI:** real generation exists, but as **two independent implementations** — `api/openapi/openapi.go` (live, serves `/api/openapi.json`) and `generator/openapi/openapi.go` (CLI-only, `awo generate openapi`) — that currently agree but must be hand-kept in sync; a future field-type addition updated in only one will silently desync the live spec from the CLI output (P2).

**Docgen:** real, tested, produces fields/edges/permissions/actions/workflow-triggers/audit-behavior per entity; missing a default-values column and an index/uniqueness-constraint section (P3).

**CLI:** `serve`, `new`, `schema {compile,validate,graph,inspect,fingerprint}`, `entity {list,inspect}`, `generate {migrations,docs,openapi}`, `docgen`, `doctor`, `version` are all real. `migrate {up,down,version,status}` and `validate <file>` are confirmed-still-open stubs (tasks.md BUG-016/017). `module list` is an **undocumented** stub (prints a curl hint, doesn't query anything — not tracked as a known bug anywhere). Nothing exists yet for tenant/org/user/role/permission/feature/settings/job/schedule/search/report/import/export/audit operations — today `awo` is a schema/codegen tool plus a server launcher, not the "serious framework control plane" (kubectl/terraform/helm-grade) target architecture.

**Module/entity registration:** `module/` (Manifest, `ModuleRegistry`, Kahn's-algorithm dependency resolution) is fully built but **entirely unintegrated dead code** — zero callers anywhere outside its own package. The real mechanism, confirmed by `bootstrap.go`'s own doc comment, is 100% hand-edited blank-import lists in `cmd/server/main.go` and `cmd/awo/cmds_schema.go`. This is the direct, concrete answer to the audit's Phase 20 question: **no configuration-driven registration exists today**, despite the framework having already built (and never wired up) the infrastructure that would provide it.

**Workflow/Scheduler:** `workflow.WorkflowExecutor` is a real, clean interface used correctly by custom Actions — but the automatic `WorkflowTrigger` lifecycle path in `EntityService` bypasses it entirely and calls the Temporal SDK directly, and (more seriously) **the documented `workflow_outbox` durability pattern (ADR-007) does not exist anywhere in the codebase** — no table, no type, nothing; a direct, synchronous `ExecuteWorkflow` call with a `// TODO: write to outbox table` comment is the entire failure-handling story (P0 — a crash or Temporal blip between commit and dispatch silently drops the trigger). `Scheduler.Cancel()` is decorative — the underlying `robfig/cron` v1 closure never checks the active/inactive flag, so a "cancelled" job keeps firing forever; the one test for this never waits a tick, so the bug has never been caught (P1).

**Import/Export:** genuinely metadata-driven (no per-entity glue code needed) for CSV/JSON — a real positive versus a hand-rolled-per-entity approach. Missing dry-run, XLSX, link-field label→UUID resolution, and duplicate detection (P2). Shares the BulkCreate audit-bypass problem from §6.

---

## 8. Feature Flags / Settings Assessment

Not independently re-verified to the same depth as other subsystems in this pass (the parallel investigation assigned to this area encountered a session interruption before delivering a full report). What is confirmed: the subsystem exists as `platform/flags` and `platform/settings`, both registered as platform entities; ADR-028 (`tasks.md`) claims a Redis→memory evaluation chain with PostgreSQL as source of truth. **This should be the first item re-verified before relying on the roadmap's Phase 2 work in this area** — see `tasks.md` Phase 2, item marked "verify before building on."

---

## 9. Security Assessment (consolidated)

| # | Location | Scenario | Impact | Severity | Fix |
|---|---|---|---|---|---|
| S1 | `generator/generator.go:120-134`, `platform/iam/handler.go`, `cmd/server/main.go` | `set_tenant_context()` never validates tenant existence/ACTIVE status; `/api/v1/auth/login` is mounted outside the `TenantResolver`-gated group | Suspended/archived-tenant users can still authenticate and obtain a live session token | **P0** | Move ACTIVE-status validation into `set_tenant_context()` itself (matching the documented spec); apply an ACTIVE check ahead of login regardless |
| S2 | `platform/iam/service.go` (Logout, `recoverSessionFromDB`) | Redis delete (authoritative) precedes a best-effort, silently-failable PG revocation write; Redis-miss fallback trusts PG and resurrects the session | A "logged out" token can be replayed successfully during/after a failed revocation write | **P0** | Make PG revocation synchronous and checked before Logout returns success, or add a Redis tombstone with matching TTL that `recoverSessionFromDB` must check first |
| S3 | `ioport.Import` → `EntityRepository.BulkCreate` | Bulk import never runs validation, hooks, or the audit pipeline | Imported data bypasses required/immutable-field rules and is invisible to compliance audit | **P0** | Route `BulkCreate` through the per-record pipeline, or explicitly document and gate the exemption |
| S4 | `EntityService.startWorkflows` | Direct synchronous Temporal call, no outbox, failure only logged | Workflow triggers silently lost on a crash/Temporal blip between commit and dispatch | **P0** | Implement the documented `workflow_outbox` (reuse the existing `events/outbox` machinery) |
| S5 | `auth/casbin.go` `Reload()` | Never called outside its own package | Revoking a role's permission has no effect until process restart | **P1** | Wire an admin mutation hook on `iam_role_permissions` to call `Reload` |
| S6 | `platform/iam/handler.go` (logout/me routes) | Missing `RequireAuth` middleware ahead of handlers reading `c.Locals("session")` | Self-service logout always 401 — users cannot revoke their own sessions via the API | P1 | Apply `RequireAuth` ahead of these two routes |
| S7 | `cmd/server/main.go` `router.Register()` | `AuditQueryer` never set, defaults to Noop | `/:id/history` always returns empty in production despite a working, tested `PoolQueryer` | P1 | Pass `AuditQueryer: audit.NewPoolQueryer(pool)` |
| S8 | `compiler/schema.go`, `audit/queryer.go` | `compiler` imports `auth`; `audit` imports `pgxpool` directly | Both are explicit violations of the documented, "frozen" prohibited-dependency table | P1 | Move `CapabilityGrant` to a leaf package; move `PoolQueryer` into `contrib/pgx` |
| S9 | `runtime/tenant.SystemContext` | Dead code, misleading doc comment implying an RLS-bypass mechanism that doesn't exist | Fails safe today (zero rows); latent risk if "completed" naively by a future author | P2 | Delete, or implement explicitly and audit-gated, and correct the comment |
| S10 | `platform/organization/service.go`, `hooks.go` | `OrganizationService` 100% stubbed; `PathComputeHook` silently mis-records every non-root org as root on the real call path | Org hierarchy — the entire multi-org isolation model — is non-functional beyond schema/RLS scaffolding | P1 | Implement `OrganizationService` for real before documenting org hierarchy as usable |

**Confirmed clean** (checked adversarially, no issue found): Filter→SQL injection surface (fully parameterized + allowlisted); platform-admin Casbin-only bypass (correctly scoped, audit-logged); RLS tenant isolation mechanism itself, once a valid ACTIVE tenant UUID is in play (real `FORCE ROW LEVEL SECURITY`, non-superuser role, tested); session-token entropy/generation and `ConstantTimeCompare` usage; `PermissionSet`→`CapabilityGrant`→Casbin separation (no role names leaking into `def`).

---

## 10. Testing Assessment

**Methodology is sound where it matters most:** every test claiming to verify RLS, tenant isolation, or permission denial uses a real PostgreSQL connection with a non-superuser `awo_app` role — never a mock at that boundary, exactly per `TEST_STRATEGY.md`'s "Real PostgreSQL Mandate."

**Gaps:**
- **Zero CI/CD configuration exists** (no `.github/workflows`, no equivalent) — every "100% pass" claim, including this audit's own baseline, comes from an ad hoc local run with no durability guarantee against future regressions.
- A **global, mutable `def` registry** (`def.Register`/`def.Lookup`) causes test-order-dependence severe enough that some tests defensively `t.Skip` to avoid double-registration panics — masking real registration-bug coverage under repeated or parallel runs.
- `workflow.Saga` and the Temporal no-server test harness (`workflow/testing.go`) are built but **entirely unused** — `TemporalExecutor`'s real dispatch path (mapping `WorkflowSpec` → Temporal's `StartWorkflowOptions`) is untested.
- `sdui/renderer` and `sdui/sduictx` have real, non-trivial untested branch logic (0.0% coverage in the profile) — duplicate-registration errors, locale-fallback chains, permission-fingerprint cache-key validation.
- One flaky-by-construction test: a fixed 20ms sleep for goroutine synchronization in `sdui/cache/cache_test.go`.
- `-race` cannot be validated in this environment (android/arm64) — must run in CI on a supported host.
- Overall statement coverage: **51.5%**, unevenly distributed (`tx`, `secrets`, `sdui/widget` at 100%; `workflow` at 9.6%; `sdui/renderer`/`sdui/sduictx` at 0%) — well short of the 90% target `tasks.md` sets for itself.

**Honest answer to "how much should a new engineer trust this suite":** trust it fully for anything touching RLS, tenant isolation, session validation, or Casbin enforcement — those paths are exercised against real infrastructure and would catch a real regression. Do **not** trust it for workflow/Temporal dispatch correctness, scheduler cancellation behavior, or SDUI rendering context edge cases — those paths have real logic with no coverage at all, and (per §4/§9) the RLS test helper itself validates against a *weakened* `set_tenant_context()` that doesn't match the documented — or even the generator's own comment-stated — security contract, so "the RLS tests pass" does not mean "tenant-status enforcement works."

---

## 11. Performance Assessment (reasoning-based, not benchmarked)

No benchmarks exist in the repository (`grep -rl "func Benchmark"` found none outside a `tests/bench` package that currently has "no tests to run"). Likely bottlenecks, each grounded in a specific file:

1. **BulkCreate is genuinely fixed** (a real positive) — `pgx.Batch`-based, 2 round trips regardless of row count (previously sequential, BUG-008).
2. **`pgxpool` has no explicit connection-pool tuning** in `bootstrap/bootstrap.go` — relies on driver defaults (`max(4, NumCPU)`), a plausible throughput ceiling under real concurrent multi-tenant load, unverified without a load test.
3. **SDUI schema regeneration cost per request** is mitigated by a real cache-key fingerprinting scheme (`SchemaFP`, `PermFP`, `FeatureFlagFP`) — a genuine strength, not a gap.
4. **Migration-generated indexing coverage for `FieldTypeLink`/FK columns** was not conclusively confirmed in this pass — worth a direct read of `generator/generator.go`'s field-loop before relying on it at scale.
5. **`set_tenant_context()` runs once per transaction**, not per query — no obvious per-request overhead multiplication found.

---

## 12. Odoo Comparison

| Dimension | Odoo | Awo | Advantage | Learn | Don't copy |
|---|---|---|---|---|---|
| ORM/metadata | Python `models.Model`, metaclass registry, dynamic | Go struct implementing `EntityDefinition`, compiled once, fail-fast at boot | Awo: compile-time safety, refactor safety | Odoo's `@api.depends` dependency-tracked computed+stored fields is genuinely excellent — Awo has nothing comparable yet | Odoo's deep `_inherit`/`_inherits` multi-level inheritance chains — a documented maintainability liability even inside Odoo's own ecosystem |
| Multi-company/tenancy | Company_id + record rules, app-layer, one DB; separate Odoo.sh = DB-per-tenant | Single-schema, DB-enforced RLS | Awo (once §4's gaps are fixed): DB-enforced isolation Odoo's record rules cannot structurally guarantee | — | — |
| Permissions | `ir.model.access` + `ir.rule` (domain-based record rules) | `PermissionSet` (declaration) + `PolicyEvaluator`/Casbin (enforcement) + `PolicyFunc` (row filter) | Awo: engine-agnostic declaration/enforcement split — Odoo has no equivalent separation | Odoo admins can reconfigure `ir.rule` without a code change; Awo's `PolicyFunc` is Go-only | — |
| Workflow/automation | `base_automation` (no-code) + `ir.cron` | `WorkflowTrigger` → Temporal (durable, code-first) | Awo: Temporal's durability/replay >> synchronous automation for long-running/financial processes | Odoo's no-code automation authoring is a real adoption advantage Awo entirely lacks today | — |
| Reporting/search | Pivot/graph on any model ad hoc; QWeb print; search is basic ILIKE | `report/report.go` real join/aggregate DSL; search = index-generation only | Odoo: ad hoc pivot without any report definition; Awo has no query-side search at all | Odoo's zero-config pivot/groupby ergonomics are excellent | — |
| API | XML-RPC/JSON-RPC, no first-class OpenAPI | Auto-generated CRUD + real OpenAPI (once de-duplicated) | Awo, clearly | — | — |
| Audit/versioning | `mail.thread` chatter — activity feed, not compliance-grade | `platform_audit_log` — partitioned, categorized, risk-scored, tamper-evidenced (once §6/§9's gaps are fixed) | Awo's *design* is more rigorous | — | — |
| Ecosystem | Tens of thousands of community addons | None | Odoo, overwhelmingly — not an architecture problem, an adoption-time reality | — | — |

## 13. Frappe/ERPNext Comparison

| Dimension | Frappe | Awo | Advantage | Learn | Don't copy |
|---|---|---|---|---|---|
| Schema model | DocType — JSON metadata stored *in the database*, live-editable via UI | Compiled Go struct, no runtime schema mutation | Awo: no "someone edited a DocType on prod" class of bug | Frappe's live Customize Form / Custom Field is a massive practical advantage for last-mile tailoring Awo cannot offer today | Frappe's fully-live, DB-stored metadata for *core* entities — trades away exactly the safety Awo is built to provide |
| Multi-tenancy | Site-per-tenant = separate DB (bench) | Single-schema RLS | Awo: far better shared-schema economics; Frappe: stronger blast-radius isolation (whole separate DB) but heavy per-tenant ops cost | — | Frappe's DB-per-site — no shared-schema story, heavy at scale |
| Permissions | Role Permission Manager + User Permissions (admin-configurable, no code) | `PermissionSet`/Casbin/PolicyFunc (code-first) | Frappe: admin reconfigurability | Consider an optional, admin-configurable row-rule layer on top of `PolicyFunc` for simple cases | — |
| Workflow | Workflow doctype (no-code state machine UI) + Server Scripts + cron | Temporal-backed `WorkflowTrigger` | Awo: durability/reliability; Frappe: no-code authoring | Same as Odoo — a no-code layer over Temporal would close a real gap | — |
| Reporting | Query Report (raw SQL/Python escape hatch) + Report Builder (no-code) + Dashboards | `report/report.go` (structured only) | Frappe explicitly separates the "safe no-code" and "raw SQL escape hatch" paths — a good pattern | Offer both: a safe `ReportDefinition` AND a clearly-labeled raw-SQL developer escape hatch, rather than one mechanism pretending to do both | Frappe's Query Report *devolving into* "wrap arbitrary SQL" — exactly what this audit told Awo to avoid |
| API | Automatic `/api/resource/{doctype}` REST, no OpenAPI in core | Auto-generated CRUD + real OpenAPI | Awo, clearly (neither incumbent ships OpenAPI generation from its metadata layer) | — | — |
| Audit/versioning | `Version` doctype — automatic diff-on-save, core, comprehensive | `platform_audit_log` — more rigorous design (categories/risk/compliance flags) once wired correctly | Frappe: simpler and unconditionally reliable *today*; Awo: more rigorous *on paper*, currently undermined by the BulkCreate/Action/history-API gaps in §6 | Frappe's Version system is unconditionally automatic — Awo's audit-by-default promise currently has real holes | — |
| Developer ergonomics | `bench new-app`, fast prototyping, dynamic Python | Go compile-time safety, but 100% manual module registration today | Frappe: genuinely faster to a working CRUD+permissions+report app | Frappe's app/DocType auto-discovery vs Awo's hand-edited blank-import lists is a direct, current gap (§7) | — |

## 14. Competitive Differentiation

**Where Awo can be genuinely better than both**, once the P0/P1 findings in this report are closed:
1. DB-enforced, structural multi-tenancy (RLS) — stronger than Odoo's app-layer record rules, cheaper at scale than Frappe's DB-per-site.
2. Compile-time-checked metadata generating persistence + API + OpenAPI + docs + UI from one Go struct — refactor safety neither competitor has.
3. Durable workflow engine (Temporal) as a first-class citizen — once the outbox gap (§7/§9 S4) is closed, meaningfully more reliable than cron/synchronous automation for financial/long-running processes.
4. Audit-by-default with a genuinely compliance-grade design (categories, risk score, tamper evidence, partitioning) — once the BulkCreate/Action/history-API gaps (§6/§9) are closed.
5. Engine-agnostic authorization (`PermissionSet`/`PolicyEvaluator` split) — neither competitor cleanly separates declaration from enforcement engine.

**Where Awo is currently weaker and must close the gap before any v1.0 parity claim:**
1. No live/no-code customization story for business admins (Frappe's Customize Form, Odoo Studio) — every change requires a Go code change + redeploy.
2. No no-code workflow/automation authoring — Temporal's power is developer-only today.
3. No query-side search engine/API — index generation only.
4. Module/entity discovery is 100% manual despite the framework's own "declaration over implementation" principle, and despite having already built (but never wired) a real discovery mechanism (`module/`).
5. Zero ecosystem — not an architecture problem, but a real adoption-time fact.
6. Multiple P0 security-contract gaps (§4, §5, §9) currently make the "structural, not additive" security claim — Awo's core differentiator — not yet true in the running system.

---

## 15. Target Architecture & Dependency Graph

Confirmed layering (with violations marked):

```
CORE:          def, filter, cache, driver (interface only)
INFRASTRUCTURE: contrib/pgx, contrib/redis   (adapters — expected to import pgx/redis)
ADAPTER:       generator, docgen, sdui/amis
PLATFORM:      registry, compiler [VIOLATION: imports auth — should not],
               runtime, audit [VIOLATION: imports pgxpool directly — should not],
               platform/*, api/*, workflow, scheduler, report, ioport
APPLICATION:   cmd/*, bootstrap
```
Two concrete, load-bearing violations of the documented "Prohibited Dependencies" table exist today (`compiler`→`auth`, `audit`→`pgxpool`) — see §9 S8 for the fix.

PostgreSQL cannot realistically be abstracted away without losing the RLS-based security model that is Awo's core differentiator; the `driver.EntityRepository[T]` interface already does the practical work of keeping SQL generation swappable at the query layer. Redis, Temporal, and Casbin are each confined to their own adapter package and genuinely swappable behind an interface, **except** where `audit` reaches into `pgxpool` directly (a fixable, isolated violation, not a structural one).

---

## 16. P0-P4 Findings Summary

See `tasks.md` for the full dependency-ordered roadmap. Counts: **P0: 5, P1: 12, P2: 12, P3: 9, P4: 1** (full list with file:line evidence in §1, §4-9 above and enumerated as roadmap items in `tasks.md`).

---

## 17. Definition of "Awo v1.0 Ready"

Awo can honestly claim v1.0 production-readiness only when, in addition to today's genuine strengths (real metadata pipeline, real RLS mechanism, real SDUI/OpenAPI/docgen, real audit design):

1. The repository builds and tests from a clean checkout with no manual intervention (Phase 0).
2. Every P0 in §9 is closed and covered by a regression test that would fail if the bug were reintroduced.
3. `docs/` is reconciled with the actual 40+-package codebase, and the two colliding ADR sequences are merged into one.
4. CI runs `go build`, `go vet`, `go test ./... -race` (on a supported host) on every change.
5. The `module/` registration mechanism is actually wired in, or removed — no more built-but-unwired subsystems presented as capabilities.
6. Coverage on security-critical paths (RLS, auth, audit) is ≥95%, not just "tested by some cases."

---

## Appendix: Investigation Method

This audit combined direct primary-source reading (documentation, Go source, SQL migrations, `go build`/`go vet`/`go test` against a real local PostgreSQL 18 + Redis 7 instance) with five parallel, independently-instructed investigations covering: (A) entity/compiler/registry/runtime/filter/migration core, (B) tenancy/org/IAM/auth/security, (C) platform entities/audit/flags/settings/report/search, (D) workflow/scheduler/import-export/SDUI/OpenAPI/docgen/CLI/module registration, (E) dependency architecture/testing/performance. Several of these independently re-derived and confirmed the same findings from different angles (notably the tenant-status-validation gap and the CLI/module-registration gap), which is reflected in the confidence level of those findings above. Two of the parallel investigations (E and part of C) were interrupted by a platform rate limit before producing a complete final report; their partial findings are incorporated above and marked as needing re-confirmation in `tasks.md` where applicable.
