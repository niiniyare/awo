# Awo Implementation Roadmap

**Status:** Authoritative implementation tracker. Supersedes all prior status claims in this file's history.
**Source:** `AUDIT_REPORT.md` (2026-09-13/18 full architecture/implementation/competitive audit — read it for evidence behind every item below).
**Rule:** Never check an item complete without evidence (a passing test, a verified `git diff`, or an explicit command output). "Code exists" ≠ "phase complete."
**Baseline at time of writing:** repo does not build as committed (see Phase 0). Once repaired, `go build`/`go vet`/`go test ./...` all pass clean (~100 packages), coverage 51.5%. `-race` untested (host limitation, must run in CI).
**Phase 0 status (this pass):** 0.1, 0.2, 0.3 DONE — see checkboxes and evidence below, and `PHASE0_REPORT.md` for the full completion report. 0.4 (full documentation reconciliation) intentionally deferred beyond a minimal, accurate root `README.md` — it is large enough to be its own tracked effort and several of its sub-items are explicitly gated on Phase 1.1 landing first.

Every item includes GOAL / WHY / FILES / DEPENDENCIES / IMPLEMENTATION / TESTS / ACCEPTANCE CRITERIA. Phases are ordered by actual dependency, not just severity — documentation-first per the framework's own Principle 5, then core contracts, then tests, then implementation, then infrastructure, then platform entities, then CLI/tooling, then advanced features.

---

## Phase 0 — Restore Buildability (BLOCKS EVERYTHING)

### 0.1 Commit a real `go.mod`
- [x] **GOAL:** Repository builds from a clean checkout with no manual steps.
- **WHY:** No `go.mod` was carried over when the framework was extracted from the monorepo into this standalone repo. Every package's own import statements already use `awo.so/awo/...` — the correct fix is `module awo.so/awo` at the repo root (not `module awo.so`), since that makes `./def` resolve to import path `awo.so/awo/def`, matching every existing source file exactly.
- **FILES:** `go.mod`, `go.sum` (repo root).
- **DEPENDENCIES:** none.
- **IMPLEMENTATION:** `go mod init awo.so/awo && go mod tidy`. Module-path decision independently re-verified from git history (not just "it made the build work"): the only historical module path ever committed here was `github.com/niiniyare/awo` (a pre-rebrand GitHub path, last seen in an unrelated, now-deleted aircraft/booking-demo codebase — not this framework), and the *entire current source tree* (~365 Go files) uses `awo.so/awo/...` exclusively, with zero occurrences of `github.com/niiniyare/erp`, `github.com/niiniyare/ruun`, or any other candidate path. `awo.so/awo` is canonical beyond reasonable doubt.
- **TESTS:** `go build ./...` → PASS. `go vet ./...` → PASS. `gofmt -l .` → clean (19 pre-existing unformatted files fixed as part of this pass — pure whitespace, no semantic change).
- **ACCEPTANCE CRITERIA:** [x] `go.mod` committed with `module awo.so/awo`. [x] `go.sum` committed. [x] `go vet ./...` runs clean.

### 0.2 Resolve the `awo.so/modules/finance` import
- [x] **GOAL:** `cmd/awo` and `cmd/server` compile.
- **WHY:** Both binaries blank-imported `_ "awo.so/modules/finance"` — a package that does not exist anywhere in this repository.
- **FILES:** `cmd/awo/cmds_schema.go`, `cmd/server/main.go`.
- **DECISION (evidence-based, not mechanical):** Option (a) — removed the import from both framework reference binaries. Evidence: `docs/00-overview/ARCH_OVERVIEW.md` §10 "Platform Modules" lists exactly seven framework-native modules (Tenant, IAM, Feature Flags, Settings, Audit, Metadata, Module Registry) — Finance is not among them. `docs/99-modules/FINANCE_MODULE_SPEC.md` itself calls Finance "the canonical **reference** module," i.e. a worked example for module authors, not a kernel component. `docs/99-modules/MODULE_AUTHOR_GUIDE.md` documents business modules living under `internal/core/{module}/` (a Go `internal/` package — importable only from within this same module tree, which itself rules out "Finance lives in a wholly separate repo" as the intended design); this repo's own extraction effort (this file's own stated scope: "Transform AWO into a clean, reusable... framework EXTRACTABLE from the ERP repository") makes clear the *intent* was to separate the reusable kernel from the application/business layer, which — given `internal/`'s visibility rules — necessarily means business modules stay behind in an application repository that imports `awo.so/awo` as a dependency, not inside this framework repo at all. Nothing in `def/`, `compiler/`, `runtime/`, or any other core package references Finance; only the two `cmd/` blank-imports did, purely for entity-registration side effects.
- **IMPLEMENTATION:** Removed `_ "awo.so/modules/finance"` from `cmd/awo/cmds_schema.go` and `cmd/server/main.go`; replaced with an explanatory comment pointing to this decision and `AUDIT_REPORT.md`.
- **TESTS:** `go build ./...` → PASS (exit 0). `go test ./...` → PASS, 66/66 tested packages, 0 failures (see Phase 0 completion report).
- **ACCEPTANCE CRITERIA:** [x] `go build ./...` clean. [x] `go vet ./...` clean. [x] Decision documented (above, and in `PHASE0_REPORT.md`).
- **Secondary finding surfaced while investigating this item (not fixed — belongs later):** `internal/dberr/dberr.go` imports the legacy standalone `github.com/jackc/pgconn` (pgx v4-era) instead of `github.com/jackc/pgx/v5/pgconn`, which the rest of the framework uses. Since these are distinct Go types from different modules, `errors.As(err, &pgErr)` against `github.com/jackc/pgconn.PgError` can never match an error actually produced by pgx/v5 — `dberr.Parse`/`dberr.IsTransient` are very likely dead code today, silently falling through to the generic-wrap branch for every real PostgreSQL error. Notably, `dberr.go` already defines `CodeTenantNotFound = "P0001"` / `CodeTenantNotActive = "P0002"` with translation logic ready to go — direct evidence that the `set_tenant_context()` RAISE EXCEPTION behavior in Phase 1.1 was previously intended and partially plumbed on the Go side; only the SQL function itself (and now this import) need fixing. Tracked as a new item: **1.1a** below.

### 0.3 Stand up CI
- [x] **GOAL:** Every push/PR runs `go build ./...`, `go vet ./...`, `go test ./...`, and `go test ./... -race` (on a linux/amd64 runner — `-race` is unsupported on the android/arm64 host this audit ran on).
- **WHY:** Zero CI/CD existed before this pass. Every "100% pass" claim, including this audit's own, was from an ad hoc local run with no durability against regressions.
- **FILES:** `.github/workflows/ci.yml` (new — three jobs: `fmt-vet-build`, `test` with real Postgres+Redis service containers, `race` on linux/amd64).
- **DEPENDENCIES:** 0.1, 0.2.
- **TESTS:** workflow authored and present in the working tree; **not yet exercised by an actual GitHub Actions run** in this pass (no push/PR was made — see `PHASE0_REPORT.md` for why, and confirm on first push).
- **ACCEPTANCE CRITERIA:** [x] CI file committed to the working tree (not yet pushed). [ ] A real PR/push shows the pipeline running and passing — **verify on first push, do not assume**. [x] `-race` job present, targets a supported architecture (`ubuntu-latest`, linux/amd64).

### 0.4 Reconcile documentation drift (do this before touching any P0/P1 code fix below)
- [ ] **GOAL:** `docs/` stops actively misleading engineers.
- **WHY:** Per the audit's Phase 28 rule and the framework's own Principle 5 ("one concept, one owner"), fixing code before fixing the doc that will immediately contradict the fix is backwards.
- **FILES:** `docs/00-overview/DECISION_REGISTER.md`, `docs/00-overview/ARCH_OVERVIEW.md`, `docs/00-overview/PACKAGE_DEPENDENCY_MAP.md`, `docs/04-multitenancy/RLS_SPEC.md`, `docs/04-multitenancy/GLOBAL_TABLES.md`, `docs/README.md`, `tasks.md` (this file, ADR table below).
- **IMPLEMENTATION:**
  1. Merge the two colliding ADR sequences (`docs/00-overview/DECISION_REGISTER.md` ADR-001..024 vs. this file's old ADR-020..029 table) into **one** numbered sequence. Renumber the old tasks.md-only decisions (Wire removal, filter-as-query-abstraction, session PG/Redis model, platform-admin-bypass model, org hierarchy, migration-gen, workflow.Executor interface, feature-flags-native, unified-audit) as ADR-025 through ADR-033 in `docs/00-overview/DECISION_REGISTER.md`, since ADR-024 is the current end of that register.
  2. Update `ARCH_OVERVIEW.md` §4 and `PACKAGE_DEPENDENCY_MAP.md`'s dependency graph to list the actual 40+ top-level packages (`platform/*`, `api/*`, `contrib/*`, `driver`, `generator`, `docgen`, `workflow`, `scheduler`, `report`, `ioport`, `module`, `observability`, `config`, `crypto`, `secrets`, `lock`, `tx`, `version`, `sdk`, `cmd/*`, `testing/*`, `testutil`, `tests`, `migration`, `migrations`, `db`, `bootstrap`), not just the 2026-07-20 kernel snapshot.
  3. Fix `RLS_SPEC.md`/`GLOBAL_TABLES.md`/`SECURITY_MODEL.md` to say `platform_tenant` (not `tenants`) and GUC `awo.tenant_id` (not `app.current_tenant_id`) — **only after** Phase 1.1 below decides which side (doc or code) is authoritative; do not just rubber-stamp the current code as "correct" without the tenant-ACTIVE-status fix, since the doc's stricter version is the one that's actually secure.
  4. Fix broken `docs/README.md` links: `05-registry/REGISTRY_SPEC.md` (doesn't exist — either write it or repoint to `05-compiler/COMPILE_SPEC.md` + index the previously-unlisted `05-compiler/` directory), `04-multitenancy/TENANT_RESOLUTION.md` → `TENANT_IDENTIFICATION.md`.
  5. Either write the missing `docs/ARCH_FREEZE_REVIEW.md` (cited repeatedly as the constitutional rationale source) and `CLAUDE.md` (cited as mandatory first-read in 3+ docs), or remove the references.
  6. Resolve `ACTOR_MODEL.md` vs `ACTOR_SPEC.md` and `SESSION_MODEL.md` vs `SESSION_SPEC.md` — mark one of each pair deprecated/superseded, don't leave both live.
  7. Consider relocating `IMPLEMENTATION.md`, `phaseA.md`, `sdui_architecture.md`, `sdui_implementation.md` out of the repo root (they're historical design blueprints, not living docs) — e.g. into a `docs/adr-history/` or archive folder — so the root doesn't keep accumulating parallel tracker documents alongside this file.
- **TESTS:** a `docs-lint` script (new, simple) that checks every relative link in `docs/README.md` resolves to a real file — run in CI (0.3).
- **ACCEPTANCE CRITERIA:** [ ] One ADR sequence. [ ] Architecture docs list real packages. [ ] No dead links in `docs/README.md`. [ ] No two "frozen"/"constitutional" documents describe the same concept differently.

---

## Phase 1 — Close the P0 Security/Correctness Gaps

Each item here needs: a failing regression test written first (red), the fix (green), then the doc corrected to match.

### 1.1 Tenant ACTIVE-status enforcement at the RLS layer
- [ ] **GOAL:** `set_tenant_context()` itself refuses non-ACTIVE tenants, exactly as `RLS_SPEC.md`/`SECURITY_MODEL.md`/`TENANT_LIFECYCLE.md` all specify, so every caller — HTTP or internal — inherits the guarantee.
- **WHY:** Currently only one Fiber middleware (`TenantResolver`) checks tenant status, and `POST /api/v1/auth/login` is mounted outside that middleware's group entirely — a suspended/archived tenant's users can still authenticate and receive a live session (AUDIT_REPORT.md §4, §9 S1).
- **FILES:** `generator/generator.go` (the `set_tenant_context` SQL template), `testutil/db/db.go` (test helper reimplementation — must match), `platform/iam/handler.go` / `platform/iam/service.go` (Login path), `db/migrations/*` (any already-applied version needs a follow-up migration).
- **DEPENDENCIES:** 0.4 (doc reconciliation for GUC naming).
- **IMPLEMENTATION:** Rewrite the generated `set_tenant_context(p_tenant_id uuid)` function to `SELECT status FROM platform_tenant WHERE id = p_tenant_id`, raise on not-found / not-ACTIVE with distinct SQLSTATEs, then `set_config`. Update `testutil/db/db.go`'s reimplementation to match exactly (currently it's a simplified stand-in that would hide a regression). Either apply the ACTIVE check ahead of the login handler too (defense in depth) or rely on the DB function alone — prefer both.
- **TESTS:** (integration, real PG) create a SUSPENDED tenant; call `set_tenant_context` directly → expect an error. Call `POST /api/v1/auth/login` for a user of a SUSPENDED tenant → expect 402/403, not 200. Call `EntityRepository.WithTx` directly (bypassing HTTP) with a SUSPENDED tenant's ID → expect rejection, not silent zero-row success.
- **ACCEPTANCE CRITERIA:** [ ] New test proves the login bypass is closed. [ ] New test proves non-HTTP callers (simulating a background job) are also rejected. [ ] `testutil/db` helper updated to match production behavior exactly. [ ] `RLS_SPEC.md` and code agree on GUC name and table name.

### 1.1a Fix `internal/dberr` importing the wrong pgconn package
- [x] **GOAL:** `dberr.Parse`/`dberr.IsTransient` actually match real pgx/v5 errors.
- **WHY:** Discovered during Phase 0.2's dependency investigation. `internal/dberr/dberr.go` imported `github.com/jackc/pgconn` (the pgx-v4-era standalone package) and type-asserted `errors.As(err, &pgconn.PgError{})`. Every other framework package uses `github.com/jackc/pgx/v5`, whose errors are `*github.com/jackc/pgx/v5/pgconn.PgError` — a distinct Go type. `errors.As` cannot match across the two.
- **NOT DEAD CODE — CORRECTION TO THE PHASE 0 SPECULATION:** grep confirms `internal/dberr` has exactly **one** caller, `contrib/pgx/repo.go`, but that caller invokes `dberr.Parse` on the error return of **every** repository operation (Get, Query, Exists, Count, Aggregate, Create, Update, Delete, BulkCreate, BulkUpdate — 18 call sites). This bug was live on the hot path of the entire repository layer, not dormant.
- **FILES:** `internal/dberr/dberr.go` (import fixed), `internal/dberr/dberr_test.go` (new), `contrib/pgx/dberr_integration_test.go` (new), `go.mod`/`go.sum` (tidied — the legacy `github.com/jackc/pgconn` requirement is gone).
- **IMPLEMENTATION:** changed the import to `"github.com/jackc/pgx/v5/pgconn"`. No other code changes needed — `PgError`'s field set is a superset in v5 (adds `SeverityUnlocalized`), fully compatible.
- **TESTS (both red before the fix, green after — verified twice, including a temporary revert-and-rerun to confirm the red state at both levels):**
  - `internal/dberr/dberr_test.go` — unit-level, 8 SQLSTATE cases (`TestParse_MatchesRealPgxV5Error`) + `TestIsTransient_MatchesRealPgxV5Error`, constructing real `*pgx/v5/pgconn.PgError` values and wrapping them the way pgx/v5 itself wraps driver errors.
  - `contrib/pgx/dberr_integration_test.go` — integration-level, real PostgreSQL: `TestRepository_Create_RealUniqueViolationTranslatedCorrectly` triggers a genuine `unique_violation` (SQLSTATE 23505) through the live `Repository.Create` path and asserts the caller receives `*runtime.BusinessError{Code:"duplicate", Status:409}`, not a generic wrapped error.
- **EVIDENCE:** `go test ./internal/dberr/... ./contrib/pgx/... -v` → all PASS (unit + integration, real Postgres). Pre-fix run captured both failure modes verbatim: unit test output `Parse(unique_violation) = ... (*fmt.wrapError), want *runtime.BusinessError`; integration test output `Create() error = ...duplicate key value violates unique constraint... (*fmt.wrapError), want *runtime.BusinessError`.
- **ACCEPTANCE CRITERIA:** [x] New tests fail against pre-fix code (verified twice), pass after the import fix. [x] `go.mod`/`go.sum` no longer reference `github.com/jackc/pgconn` (confirmed via `go mod tidy` + grep). [x] Confirmed `dberr.Parse`/`IsTransient` have exactly one caller (`contrib/pgx/repo.go`), invoked on every repository operation's error path — not unused.

### 1.2 Session revocation race
- [ ] **GOAL:** A logged-out token cannot be revived by the Redis-miss PG-recovery path.
- **WHY:** AUDIT_REPORT.md §5, §9 S2 — Redis delete is authoritative and immediate; the PG `revoked_at` write is best-effort and its failure is silently swallowed; `recoverSessionFromDB` trusts PG unconditionally on a Redis miss.
- **FILES:** `platform/iam/service.go` (Logout, `recoverSessionFromDB`).
- **DEPENDENCIES:** none.
- **IMPLEMENTATION:** Make the PG revocation write synchronous and checked before Logout returns success (fail closed — return an error rather than a silent partial revoke), **or** write a short-TTL Redis tombstone key on logout that `recoverSessionFromDB` checks before trusting a PG row.
- **TESTS:** (integration) force the PG update to fail (e.g. inject a connection error) and confirm the token is still rejected afterward, not resurrected. Concurrent Logout + validate-token race test.
- **ACCEPTANCE CRITERIA:** [ ] Forced-PG-failure test passes (token stays revoked). [ ] Concurrency test passes. [ ] `SESSION_SPEC.md` documents the actual chosen mechanism.

### 1.3 Bulk import/export must not bypass the pipeline
- [ ] **GOAL:** Every row created via `ioport.Import` runs the same validation/hooks/audit as `Create`.
- **WHY:** AUDIT_REPORT.md §6, §7, §9 S3 — `BulkCreate` is a raw `pgx.Batch` INSERT with zero pipeline integration; every CSV/JSON import is invisible to the audit trail and exempt from required/immutable-field rules.
- **FILES:** `ioport/importer.go`, `contrib/pgx/repo.go` (`BulkCreate`), `driver/repository.go`.
- **DEPENDENCIES:** none.
- **IMPLEMENTATION:** Either (a) route `ioport.Import` through per-record `Create` (accept the performance cost, or batch-with-pipeline in chunks), or (b) keep `BulkCreate` as a distinct, explicitly-documented "trusted, pipeline-exempt" path used only where the caller has already validated/audited by other means, and make `ioport.Import` use path (a) exclusively. Do not leave the current silent default.
- **TESTS:** import a CSV missing a required field → expect a rejection, not a silently-persisted invalid row. Import a batch → expect one `platform_audit_log` row per created record.
- **ACCEPTANCE CRITERIA:** [ ] Required/immutable-field violations in an import are rejected. [ ] Every imported record has a corresponding audit record. [ ] Performance regression (if any) measured and accepted or mitigated with chunked pipelined batches.

### 1.4 Workflow-trigger durability (implement the documented outbox)
- [ ] **GOAL:** `WorkflowTrigger` starts survive a crash or transient Temporal outage between commit and dispatch, as ADR-007 requires.
- **WHY:** AUDIT_REPORT.md §7, §9 S4 — `EntityService.startWorkflows` calls Temporal directly and synchronously; failure is a log line and a `// TODO`; no `workflow_outbox` table exists.
- **FILES:** `api/service/entity.go` (`startWorkflows`), new: a `workflow_outbox` table/migration, a relay worker (can likely reuse `events/outbox`'s existing relay machinery/pattern — it already solves this exact problem for domain events).
- **DEPENDENCIES:** none, but should be designed alongside 1.7 below (unify the two workflow-dispatch paths).
- **IMPLEMENTATION:** Write the workflow start to a durable `workflow_outbox` row in the same TX as the entity mutation; a background relay (mirroring `events/outbox`'s relay) dispatches pending rows to Temporal via `workflow.WorkflowExecutor`, with WorkflowID-based dedup so retries are safe.
- **TESTS:** kill the process (or mock a Temporal-unavailable error) between commit and dispatch → confirm the workflow eventually starts once the relay runs. Confirm no duplicate workflow starts on relay retry (WorkflowID dedup).
- **ACCEPTANCE CRITERIA:** [ ] `workflow_outbox` table exists and is populated transactionally with the entity mutation. [ ] Relay worker dispatches pending rows. [ ] Crash-recovery test passes. [ ] `EntityService.startWorkflows` no longer calls Temporal directly. [ ] `docs/08-workflow/OUTBOX_SPEC.md`/`TEMPORAL_INTEGRATION.md` match reality.

### 1.5 Casbin policy reload on permission change
- [ ] **GOAL:** Revoking a role's permission takes effect without a process restart.
- **WHY:** AUDIT_REPORT.md §5, §9 S5 — `CasbinEvaluator.Reload()` exists, works, and is never called anywhere outside its own package.
- **FILES:** `auth/casbin.go`, wherever `iam_role_permissions` mutations happen (likely a `platform/iam` admin handler/service method).
- **DEPENDENCIES:** none.
- **IMPLEMENTATION:** Call `Reload()` (or an incremental policy update, if Casbin supports it more cheaply) from the `iam_role_permissions` mutation path. Consider a hook (`AfterCreate`/`AfterDelete` on that entity) mirroring the existing `UserRoleChangeHook` pattern.
- **TESTS:** revoke a permission from a role with a live session holding it; call the now-forbidden endpoint immediately (no restart) → expect 403.
- **ACCEPTANCE CRITERIA:** [ ] Test above passes. [ ] Reload latency documented (should be near-immediate, not batched/delayed in a way that reintroduces a meaningful window).

### 1.6 Fix `/auth/logout` and `/auth/me` (currently always 401)
- [ ] **GOAL:** Self-service logout actually works.
- **WHY:** AUDIT_REPORT.md §5, §9 S6 — both routes read `c.Locals("session")`, populated only by `RequireAuth`, which isn't mounted ahead of them.
- **FILES:** `platform/iam/handler.go`, `platform/iam/module.go` (route registration), `api/router/router.go`.
- **DEPENDENCIES:** none.
- **IMPLEMENTATION:** Mount `middleware.RequireAuth` (without `TenantResolver`, since these are tenant-header-driven, not `/api/v1`-group routes) ahead of `logout`/`me` registration.
- **TESTS:** integration test hitting the real Fiber route (not just `AuthService` directly) — `POST /api/v1/auth/logout` with a valid bearer token → expect 200, and confirm the session is actually gone afterward.
- **ACCEPTANCE CRITERIA:** [ ] Route-level integration test passes (this class of bug specifically evaded unit tests before — must be a real HTTP-route test). [ ] `GET /api/v1/auth/me` returns the caller's identity with a valid token.

---

## Phase 2 — Close the P1 Architecture/Dependency Gaps

### 2.1 Dependency-graph violations
- [ ] **GOAL:** `compiler` and `audit` stop violating the documented "Prohibited Dependencies" table.
- **WHY:** AUDIT_REPORT.md §9 S8 — `compiler/schema.go` imports `auth` for `CapabilityGrant`; `audit/queryer.go` imports `pgxpool` directly, contradicting `driver/doc.go`'s stated pgx-confinement rule.
- **FILES:** `compiler/schema.go`, `def/` (or a new leaf package for `CapabilityGrant`), `audit/queryer.go`, `contrib/pgx/`.
- **DEPENDENCIES:** none.
- **IMPLEMENTATION:** Move `CapabilityGrant`'s type definition down to `def` (or a new dependency-free package both `compiler` and `auth` can import) so `compiler` doesn't reach upward into `auth`. Move `audit.PoolQueryer`'s pgx-specific implementation into `contrib/pgx` (mirroring the existing `contrib/pgx.NewPoolQuerier` pattern for the writer side), leaving `audit` itself pgx-free behind its `Queryer` interface.
- **TESTS:** a dependency-graph lint (can be a simple `go list -deps` check in CI) asserting `compiler` and `audit` never import `auth`/`pgxpool` respectively going forward.
- **ACCEPTANCE CRITERIA:** [ ] Both violations fixed. [ ] CI lint added so they can't silently reappear. [ ] `PACKAGE_DEPENDENCY_MAP.md` updated to reflect the corrected, enforced graph.

### 2.2 Wire the audit history endpoint into production
- [ ] **GOAL:** `GET /:id/history` returns real data in the deployed server, not an empty array.
- **WHY:** AUDIT_REPORT.md §6, §7, §9 S7 — `cmd/server/main.go`'s `router.Register()` call never sets `RegisterOptions.AuditQueryer`, silently defaulting to `NoopQueryer`.
- **FILES:** `cmd/server/main.go`.
- **DEPENDENCIES:** none (one-line fix, but needs a real end-to-end test since this exact class of "wired in tests, not wired in prod" bug already happened once).
- **IMPLEMENTATION:** `router.Register(app, schema, router.RegisterOptions{..., AuditQueryer: audit.NewPoolQueryer(result.Pool), ...})`.
- **TESTS:** an end-to-end test that starts the real server binary (or as close to it as practical) and hits `/:id/history` against a record with audit history — not just a handler-level unit test with a manually-constructed `RegisterOptions`.
- **ACCEPTANCE CRITERIA:** [ ] Fix applied. [ ] End-to-end test added that would have caught this class of bug. [ ] Audit a `RegisterOptions`-construction checklist/test so other options fields can't silently default-to-noop in production the same way.

### 2.3 Reconcile the duplicate `platform_audit_log` migrations
- [ ] **GOAL:** Exactly one migration creates `platform_audit_log`.
- **WHY:** AUDIT_REPORT.md §6 — `db/migrations/000452_platform_audit_log.up.sql` (top-level, the generator's configured default output dir) and `platform/audit/migrations/001_platform_audit_log.up.sql` (module-embedded) both independently create the same partitioned table. If both are ever applied to one database, the second fails.
- **FILES:** `db/migrations/000452_*.sql`, `platform/audit/migrations/001_*.sql`, `platform/audit/migrations/migrations.go`, whatever loads `db/migrations/` (locate via `config.manager.go`'s `migration.dir`).
- **DEPENDENCIES:** none, but do this before any production deployment.
- **IMPLEMENTATION:** First determine, by tracing the actual bootstrap/deploy path, which one is genuinely live. Delete or clearly mark-as-superseded the other. If both are somehow independently used in different deployment modes, document that explicitly and make it impossible to run both against the same database (e.g. one checks for the other's table name first).
- **TESTS:** apply the full migration set to a clean database end-to-end (both directories, in whatever order a real deploy would use) and confirm no conflict — or confirm the dead one has been removed and a clean apply still works.
- **ACCEPTANCE CRITERIA:** [ ] Exactly one authoritative `platform_audit_log` migration remains (or the dual-path design is explicit and documented). [ ] Clean-database migration test passes.

### 2.4 Unify workflow dispatch paths
- [ ] **GOAL:** Both custom Actions and automatic `WorkflowTrigger`s go through `workflow.WorkflowExecutor`.
- **WHY:** AUDIT_REPORT.md §7 — currently only the Action path uses the interface; the automatic trigger path hits the Temporal SDK directly, breaking the "Temporal must be replaceable" goal and creating inconsistent retry/error semantics.
- **FILES:** `api/service/entity.go`, `workflow/executor.go`.
- **DEPENDENCIES:** should land together with 1.4 (the outbox work) since both touch the same call site.
- **IMPLEMENTATION:** Make `EntityService` take a `workflow.WorkflowExecutor` and route the automatic-trigger path through it exclusively, same as the Action path already does.
- **TESTS:** unit test with a fake `WorkflowExecutor` confirming the automatic-trigger path calls it, not the Temporal SDK directly.
- **ACCEPTANCE CRITERIA:** [ ] One dispatch path. [ ] Test passes. [ ] `NoopExecutor`/degraded-mode behavior (server runs, workflows queue) verified for the trigger path too, not just Actions.

### 2.5 Wire or remove `module/` (entity/module registration)
- [ ] **GOAL:** Answer, concretely, whether Awo has configuration-driven module registration.
- **WHY:** AUDIT_REPORT.md §7 — `module/` (Manifest, `ModuleRegistry`, Kahn's-algorithm dependency resolution) is fully built, has its own tests, and has **zero callers anywhere outside itself**. The real mechanism is 100% hand-edited blank-import lists in two `cmd/` files.
- **FILES:** `module/manifest.go`, `module/registry.go`, `bootstrap/bootstrap.go`, `cmd/server/main.go`, `cmd/awo/cmds_schema.go`, every `platform/*` package's `init()`.
- **DEPENDENCIES:** 0.2 (decide the fate of the finance-module import first, since this phase changes the same registration mechanism).
- **IMPLEMENTATION:** Wire `module.ModuleRegistry` into `bootstrap.Run`: each `platform/*` package (and any future business module) registers a `Manifest` in `init()`; `bootstrap` calls `Resolve()` to order registration deterministically and validate version compatibility, replacing (or augmenting, if compile-time safety needs to be preserved via blank imports for `go build` reachability) the hand-edited import lists. If, after review, the team decides `module/` isn't worth wiring in, delete it — don't leave built-but-unwired infrastructure presented as a capability.
- **TESTS:** a test that registers two modules with a declared dependency out of import order and confirms `Resolve()` orders them correctly; a bootstrap test confirming all platform modules are discovered without a hand-edited list (if implemented), or confirming `module/` is fully removed (if not).
- **ACCEPTANCE CRITERIA:** [ ] Either `module/` is load-bearing in `bootstrap.Run` with a passing dependency-ordering test, or it's deleted. [ ] `docs/99-modules/MODULE_AUTHOR_GUIDE.md` matches whichever choice is made.

---

## Phase 3 — Organisation Hierarchy (make it real)

### 3.1 Implement `OrganizationService`
- [ ] **GOAL:** Create/Move/ResolveScope actually work.
- **WHY:** AUDIT_REPORT.md §4, §9 S10 — every method returns `"not implemented"` today; `PathComputeHook` silently mis-records every non-root org as a root on the real (non-test) call path, since it only computes a correct path when an internal test-only field is pre-populated.
- **FILES:** `platform/organization/service.go`, `platform/organization/hooks.go`.
- **DEPENDENCIES:** none, but should land before any business module claims to use org scoping.
- **IMPLEMENTATION:** Implement each `OrganizationService` method against `EntityRepository`, and fix `PathComputeHook` to always compute the materialized path from the real parent record (via a repository lookup), not from a pre-populated test-only field.
- **TESTS:** create a 3-level org hierarchy (Tenant → Holding → Company A); confirm `path`/`depth` are correct at each level; confirm a Company-A-scoped user cannot see Holding-level records; confirm a manager sees their subtree.
- **ACCEPTANCE CRITERIA:** [ ] All `OrganizationService` methods implemented, tested against real PG. [ ] Hierarchy creation/query integration tests pass (these were explicitly unchecked/pending in every prior status doc). [ ] `path` field decision made explicitly (real `ltree` extension vs. formally-documented VARCHAR+LIKE) and ADR-012/024 corrected to match.

---

## Phase 4 — Platform Entity & Subsystem Cleanup (dead code, duplication)

### 4.1 Resolve `platform/notification` vs `platform/notifications`
- [ ] **GOAL:** One notification module, registered in production.
- **WHY:** AUDIT_REPORT.md — `platform/notification` (singular, 130-line stub) is the one blank-imported by both `cmd/server/main.go` and `cmd/awo/cmds_schema.go`. `platform/notifications` (plural, full module: driver/hooks/service/migrations) is blank-imported only from `tests/integration/db_test.go` — never reaches production.
- **FILES:** `platform/notification/`, `platform/notifications/`, `cmd/server/main.go`, `cmd/awo/cmds_schema.go`.
- **DEPENDENCIES:** none.
- **IMPLEMENTATION:** Decide which is intended to be real. If `notifications` (plural) is the actual, more complete implementation, switch both `cmd/` binaries to import it instead and delete `notification` (singular). If `notification` (singular) is intentionally the current minimal version, delete `notifications` (plural) and its now-orphaned test import.
- **TESTS:** the surviving module's existing tests must pass; add a bootstrap-level test confirming the notification entity registered in a running server matches the intended module.
- **ACCEPTANCE CRITERIA:** [ ] Exactly one `platform/notification*` package remains. [ ] Production `cmd/` binaries import it. [ ] No orphaned test-only import of a module absent from production.

### 4.2 Delete dead/misleading code
- [ ] **GOAL:** Remove code whose existence actively misleads.
- **FILES:** `runtime/tenant.SystemContext` (misleading "special admin token" doc comment, zero production callers — see 1.1/S9), `api/handler/sdui.go` (4-line dead stub superseded by `api/sdui/handler.go`).
- **DEPENDENCIES:** 1.1 (SystemContext's fate may be decided alongside the tenant-status fix — either delete it or implement it explicitly and audit-gated, never leave the misleading comment).
- **ACCEPTANCE CRITERIA:** [ ] `SystemContext` either removed or reimplemented with an accurate comment and an explicit audit-gated mechanism. [ ] Dead `sdui.go` stub removed.

### 4.3 Fix `Scheduler.Cancel()`
- [ ] **GOAL:** A cancelled job actually stops firing.
- **WHY:** AUDIT_REPORT.md §7 — the `robfig/cron` v1 closure never checks the active/inactive flag `Cancel()` sets; the existing test never waits a tick, so this has never been caught.
- **FILES:** `scheduler/scheduler.go`, `scheduler/scheduler_test.go`.
- **IMPLEMENTATION:** Migrate to `robfig/cron/v3` (has `cron.Remove(id)`), or gate the closure body on the active flag.
- **TESTS:** schedule a job with a sub-second interval, cancel it, wait past the next scheduled fire time, confirm it did not fire.
- **ACCEPTANCE CRITERIA:** [ ] Test above passes (currently would fail if written honestly).

### 4.4 Deduplicate OpenAPI generators
- [ ] **GOAL:** One OpenAPI generation implementation.
- **WHY:** AUDIT_REPORT.md §7 — `api/openapi/openapi.go` (live) and `generator/openapi/openapi.go` (CLI) independently reimplement the same entity→schema logic; a future field-type addition updated in only one silently desyncs the live spec from the CLI output.
- **FILES:** `api/openapi/openapi.go`, `generator/openapi/openapi.go`.
- **IMPLEMENTATION:** Have `api/openapi` call into `generator/openapi.Generate` and adapt its richer `Spec` type to the live route's response shape, eliminating the duplicate implementation.
- **TESTS:** existing tests for both packages should still pass after consolidation; add a test proving both entry points produce identical output for the same schema.
- **ACCEPTANCE CRITERIA:** [ ] One implementation. [ ] Both entry points verified identical.

---

## Phase 5 — Migration Engine Hardening

### 5.1 Schema-diff / rename safety
- [ ] **GOAL:** The generator cannot silently produce a data-losing migration for a field rename.
- **WHY:** AUDIT_REPORT.md §3 — currently a field rename in `def.FieldDef` is indistinguishable from drop+add; no warning, no plan/diff/approve step exists.
- **FILES:** `generator/generator.go`, `def/field.go` (may need a `RenamedFrom string` field or equivalent).
- **DEPENDENCIES:** Phase 0-2 complete first (don't build new migration-engine features on top of an unbuildable repo or unfixed security gaps).
- **IMPLEMENTATION:** At minimum: (a) detect a column drop + a column add of a compatible type in the same generation pass and refuse to emit silently — require an explicit `RenamedFrom` annotation to emit a real `ALTER TABLE ... RENAME COLUMN`; (b) implement the `plan → diff → explain → approve → generate → apply → verify` workflow the audit's Phase 10 describes, at least as a CLI flag (`awo generate migrations --explain`) before full automation.
- **TESTS:** rename a field without `RenamedFrom` → generation refuses/warns loudly. Rename with `RenamedFrom` set → real `RENAME COLUMN`, no data loss, verified against a seeded table.
- **ACCEPTANCE CRITERIA:** [ ] No silent drop+add possible for a documented rename. [ ] `MIGRATION_GUIDE.md` documents the workflow.

### 5.2 Migration directory consolidation
- [ ] **GOAL:** One clear, documented migration-file location/convention.
- **WHY:** Currently `db/migrations/` (generator default), top-level `migrations/` (framework bootstrap), and per-module `platform/*/migrations/` all coexist with different naming conventions (numeric vs. timestamp).
- **DEPENDENCIES:** 2.3 (the `platform_audit_log` duplicate is one symptom of this).
- **IMPLEMENTATION:** Document (in `MIGRATION_GUIDE.md`) which directories are canonical for what (e.g. "framework bootstrap = top-level `migrations/`; per-platform-module = `platform/<module>/migrations/`, embedded via `migrations.go`; generator output for business-module schemas = `db/migrations/`") and verify the actual migration-runner code applies them in the intended order without conflicts.
- **ACCEPTANCE CRITERIA:** [ ] Convention documented. [ ] A clean-database bootstrap test applies all of them without conflict.

---

## Phase 6 — Search (currently missing)

- [ ] **GOAL:** Decide and, if in scope for v1, build a real query-side search capability.
- **WHY:** AUDIT_REPORT.md §2 — `Searchable` fields today only drive GIN/trigram *index generation*; there is no search API, no ranking, no multi-field/global search anywhere in the repo. This is a real, notable competitive gap versus both Odoo and Frappe.
- **FILES:** new — likely a `search/` package sitting on top of `filter/`, consuming the trigram/tsvector indexes the generator already creates.
- **DEPENDENCIES:** Phase 0-2. Not urgent relative to the P0/P1 security items, but should not be deferred indefinitely if "serious ERP framework" is the goal.
- **IMPLEMENTATION:** Design first (per Phase 28's documentation-first rule) — a `search.Query` type building on PostgreSQL full-text (`tsvector`/`to_tsquery`) and trigram similarity, tenant/org-scoped, permission-aware, exposed via `filter.Filter` integration and a CLI/API surface. Explicitly design for replaceability (PostgreSQL first, not architecturally locked in) per the audit's Phase 11 guidance.
- **ACCEPTANCE CRITERIA:** [ ] Design doc written and reviewed before code. [ ] Basic global search works end-to-end with tenant/permission scoping. [ ] Indexed-field search is measurably faster than a naive `ILIKE` scan on a representative dataset.

---

## Phase 7 — Feature Flags / Settings Re-Verification

- [ ] **GOAL:** Confirm (or fix) the precedence chain, hot-path caching, and cross-subsystem wiring claimed by ADR-028/tasks.md's old table.
- **WHY:** AUDIT_REPORT.md §8 — this area's dedicated parallel investigation was interrupted before delivering a full report; existing claims are unverified by this audit and should not be trusted at face value.
- **FILES:** `platform/flags/`, `platform/settings/`.
- **DEPENDENCIES:** none — can run in parallel with Phase 1-2.
- **IMPLEMENTATION:** Re-derive, from source, the actual precedence order (system→tenant→org→role→user, or whatever the code implements), the actual cache-invalidation strategy, and whether `WithSettingsProvider`/`WithFlagsProvider` SDUI wiring is genuinely consumed end-to-end or decorative.
- **ACCEPTANCE CRITERIA:** [ ] A short verification report (evidence-based, file:line) replaces this phase's placeholder status. [ ] Any gaps found get their own follow-up item here.

---

## Phase 8 — CLI: Build the Control Plane

- [ ] **GOAL:** `awo` becomes a real operational control plane, not just schema/codegen + server launcher.
- **WHY:** AUDIT_REPORT.md §7 — no CLI surface exists today for tenant/org/user/role/permission/feature/settings/job/schedule/search/report/import/export/audit operations; `migrate`, `module list`, and `validate` are confirmed stubs.
- **FILES:** `cmd/awo/*.go` (new subcommand files per area).
- **DEPENDENCIES:** the corresponding library-level work in earlier phases (e.g. CLI `tenant` commands depend on `platform/tenant` being solid, which it already is; CLI `search` commands depend on Phase 6).
- **IMPLEMENTATION:** Design the minimal-but-competitive command tree first (per the audit's Phase 19 guidance — do not blindly adopt every command the audit brainstormed; decide what's actually needed). At minimum, finish the three already-tracked stubs (`migrate up/down/version/status`, `module list`, `validate <file>`) before adding new surface area. Interactive mode, if built at all, must be opt-in, never default.
- **TESTS:** each new subcommand gets a CLI-level test (not just the underlying library call).
- **ACCEPTANCE CRITERIA:** [ ] `migrate`/`module list`/`validate` are real, not stubs. [ ] A documented, deliberately-scoped command tree exists for at least tenant/user/role/audit operations. [ ] No command is silently non-functional (every stub either works or clearly errors "not implemented" rather than printing a misleading hint).

---

## Phase 9 — Test Suite Hardening

### 9.1 Fix registry test-order-dependence
- [ ] **GOAL:** Tests don't `t.Skip` to avoid a shared-global-registry double-registration panic.
- **FILES:** `def/` (registry), `registry/registry_test.go`.
- **IMPLEMENTATION:** Give the registry a `Reset()`/test-scoped-instance path, or use unique per-test-run entity names.
- **ACCEPTANCE CRITERIA:** [ ] `go test ./... -count=2` and `go test ./... -shuffle=on` both pass with no skips caused by registration collisions.

### 9.2 Exercise `workflow.Saga` and `TemporalExecutor`'s real dispatch path
- [ ] **GOAL:** The built-but-unused Temporal no-server test harness (`workflow/testing.go`) actually gets used.
- **FILES:** `workflow/saga.go`, `workflow/testing.go`, `workflow/executor.go`.
- **ACCEPTANCE CRITERIA:** [ ] `Saga` has unit tests (no Temporal needed). [ ] At least one test wires `NewTestEnv`/`MockActivityResult` against `TemporalExecutor`.

### 9.3 Cover `sdui/renderer` and `sdui/sduictx`
- [ ] **GOAL:** Real branch logic currently at 0% coverage gets tested.
- **FILES:** `sdui/renderer/renderer.go`, `sdui/renderer/locale.go`, `sdui/sduictx/context.go`.
- **ACCEPTANCE CRITERIA:** [ ] `Validate()`, locale-fallback tiers, and `Registry.Register`/`MustLookup` have table-driven tests.

### 9.4 Remove sleep-based test synchronization
- [ ] **GOAL:** No flaky fixed-sleep tests.
- **FILES:** `sdui/cache/cache_test.go:163`.
- **IMPLEMENTATION:** Replace the 20ms sleep with a barrier (counted channel/WaitGroup) confirming goroutines have actually entered the function before proceeding.
- **ACCEPTANCE CRITERIA:** [ ] Test passes reliably under `-count=20` with artificial scheduler delay/load.

### 9.5 Coverage push toward 90% on security-critical paths
- [ ] **GOAL:** RLS, auth, audit packages reach ≥95% coverage (not just "framework overall ≥90%").
- **DEPENDENCIES:** 9.1-9.4, plus the P0/P1 fixes above (which each come with their own new tests).
- **ACCEPTANCE CRITERIA:** [ ] `go test ./... -coverprofile=coverage.out && go tool cover -func=coverage.out` shows ≥95% for `auth`, `platform/iam`, `contrib/pgx`, `audit`, `platform/audit`, `platform/organization`. [ ] Overall framework coverage tracked and trending toward 90%, reported honestly (not rounded up).

---

## Phase 10 — Performance Baseline

- [ ] **GOAL:** Turn the reasoning-based performance assessment (AUDIT_REPORT.md §11) into measured data.
- **FILES:** `tests/bench/` (currently empty — "no tests to run").
- **IMPLEMENTATION:** Write real `func Benchmark*` cases for: Filter→SQL generation, a representative `EntityRepository.Create`/`Query` round trip, SDUI schema generation with and without cache hit, `set_tenant_context` overhead per transaction. Add explicit `pgxpool` tuning (`MaxConns`, idle/lifetime) to `bootstrap/bootstrap.go` based on the results, with pool-exhaustion metrics/alerting.
- **ACCEPTANCE CRITERIA:** [ ] Benchmarks exist and run in CI (tracked, not gated, to start). [ ] `bootstrap.go` has explicit, justified pool sizing. [ ] Migration generator's FK/Link-field indexing coverage confirmed (or fixed) — was flagged as unverified in this audit.

---

## Immediate Next Actions (ordered)

1. **Phase 0** — without a committed `go.mod` and a working `cmd/` build, nothing else can be verified by CI or by any future contributor. This is the literal first thing to do.
2. **Phase 0.4** — reconcile documentation before touching security-critical code, so the fix and the spec don't diverge again immediately.
3. **Phase 1** — close the five P0 findings, each with a regression test written first.
4. **Phase 2** — close the P1 dependency/wiring gaps (many are near-one-line fixes with outsized impact: the audit-history no-op, the dependency violations).
5. **Phase 3** — make organisation hierarchy real before any business module is built assuming it works.
6. **Phases 4-10** — cleanup, migration hardening, search, CLI, test/perf hardening, in roughly that dependency order; several can run in parallel once Phase 0-2 are closed.

**Do not mark any checkbox above complete without the evidence its own item specifies.** This file's predecessor was found, during this audit, to contain checkboxes marked complete on the basis of code in a module (`modules/finance`) that does not exist in this repository — the failure mode this rule exists to prevent.
