# Awo Implementation Roadmap

**Status:** Authoritative implementation tracker. Supersedes all prior status claims in this file's history.
**Source:** `AUDIT_REPORT.md` (2026-09-13/18 full architecture/implementation/competitive audit — read it for evidence behind every item below).
**Rule:** Never check an item complete without evidence (a passing test, a verified `git diff`, or an explicit command output). "Code exists" ≠ "phase complete."
**Baseline at time of writing:** repo does not build as committed (see Phase 0). Once repaired, `go build`/`go vet`/`go test ./...` all pass clean (~100 packages), coverage 51.5%. `-race` untested (host limitation, must run in CI).
**Phase 0 status (this pass):** 0.1, 0.2, 0.3 DONE — see checkboxes and evidence below, and `PHASE0_REPORT.md` for the full completion report. 0.4 (full documentation reconciliation) intentionally deferred beyond a minimal, accurate root `README.md` — it is large enough to be its own tracked effort and several of its sub-items are explicitly gated on Phase 1.1 landing first.

Every item includes GOAL / WHY / FILES / DEPENDENCIES / IMPLEMENTATION / TESTS / ACCEPTANCE CRITERIA. Phases are ordered by actual dependency, not just severity — documentation-first per the framework's own Principle 5, then core contracts, then tests, then implementation, then infrastructure, then platform entities, then CLI/tooling, then advanced features.

**CURRENT PHASE & IMMEDIATE OBJECTIVE (2026-10-13):** Phases 0-2 and Phase 3 Step 0/1 (organization RLS + path computation) are closed. The immediate objective has shifted from pure security/correctness hardening to **proving Awo can support a real business process end-to-end** — the first showcase, `examples/expense` (Business Expense), is implemented and verified (see **SHOWCASE** section immediately below). This does not supersede or deprioritize the still-open Phase 1/2/3 security and correctness items below it — it runs alongside them. See `docs/00-overview/TECHNICAL_FINDINGS_AND_FUTURE_IMPROVEMENTS.md` for the durable record of everything discovered while building it (several new, previously-unknown findings — R.1/R.2/R.3, 3.1a, 3.1b below — came directly out of this showcase work).

---

## SHOWCASE — Business Expense (`examples/expense`)

- [x] **STATUS: P0 vertical slice implemented and verified this pass.**
- **GOAL:** Prove, with the smallest entity that exercises the most framework capability, that Awo supports a real business process: tenant-isolated persistence, a generated CRUD API, default permission enforcement, mandatory audit, and an explicit, tested status lifecycle — with zero hand-written SQL beyond the entity's own migration and zero hand-written API/routing code.
- **WHY THIS ENTITY (full selection reasoning: `docs/00-overview/TECHNICAL_FINDINGS_AND_FUTURE_IMPROVEMENTS.md` §D):** familiar to any non-technical stakeholder; small, closed-ended lifecycle (`draft -> submitted -> approved`/`rejected -> draft`); fits `SystemDefinition` exactly (no accounting integration, no approval-chain engine, no notification infrastructure needed to demonstrate it credibly); a `CustomDefinition` was considered and rejected — **confirmed non-functional against any real database today** (see findings register §A.3/R.2) — making `SystemDefinition` the only viable choice regardless of which would otherwise have been faster.
- **FILES:** `examples/expense/definition.go`, `examples/expense/hooks.go` (`StatusTransitionGuard`), `examples/expense/migrations/001_create_business_expense.up.sql`/`.down.sql`, `examples/expense/migrations/migrations.go` (registered with `migration.Register`, Priority 40 — "custom application modules" per `migration.Source`'s own documented convention), `examples/expense/expense_pg_test.go`. Wired into `cmd/server/main.go`, `cmd/awo/cmds_schema.go` (entity registration) and `cmd/migrate/main.go` (migration registration — the step most other platform modules are missing; see R.1) as the one explicit, documented exception to the "business modules don't live in this repo" rule.
- **ACCEPTANCE CRITERIA (mapped to the illustrative 9-point scenario — status per point):**

  | # | Scenario requirement | Status |
  |---|---|---|
  | 1 | User creates an expense | **Implemented and verified** — `TestExpense_Create_DefaultsToDraft_TenantScoped` |
  | 2 | Required fields / business rules validated | **Implemented and verified** — field-level `Required`/`Select` enum validation (generic framework mechanism, exercised by every Create call in the test suite) |
  | 3 | Stored with correct tenant context | **Implemented and verified** — `TestExpense_TenantIsolation_OtherTenantCannotSeeIt`, real RLS, non-superuser `awo_app` role |
  | 4 | Retrievable via existing API/UI | API: **implemented and verified** (generic `router.go` CRUD route, same mechanism as every other entity). UI (SDUI): **implemented but insufficiently verified** — the generic generator is tested; this entity's own generated page was not inspected this pass |
  | 5 | Unauthorized operations rejected | **Implemented, not independently re-verified for this entity** — same generic `authz.RequirePermission` mechanism as every other entity; not given a dedicated test this pass (would duplicate coverage that already exists generically) |
  | 6 | Changes produce expected audit history | **Implemented, not independently re-verified for this entity** — generic audit mechanism proven in Phase 2 Step 7b; this entity's own audit rows were not queried this pass (see findings register §A.8) |
  | 7 | Permitted status transition moves the lifecycle forward | **Implemented and verified** — `TestExpense_StatusTransitionGuard_ValidTransition_Succeeds`, `..._InvalidTransition_Rejected`, `..._UnrelatedFieldUpdate_Unaffected` (the last one specifically proving this module did not repeat `platform/organization.MoveGuard`'s parameter-order bug — see 3.1a) |
  | 8 | UI generated from existing capabilities | Same as #4 — **implemented but insufficiently verified** |
  | 9 | Automated tests demonstrate the behavior | **Implemented and verified** — 5 tests, real PostgreSQL, real role, real migration file, all passing |

- **TESTS:** `examples/expense/expense_pg_test.go` — 5 tests, all passing against real PostgreSQL (`go test ./examples/... -v`).
- **VERIFICATION RUN THIS PASS:** `gofmt -l .` clean. `go vet ./...` clean. `go build ./...` clean. `git diff --check` clean. `go test ./examples/... -v`: 5/5 pass. `go test ./... -count=1`: **68 packages ok, 0 FAIL** (up from 67 before this entity existed). `-race`: unsupported on this host (android/arm64), CI's `linux/amd64` job remains authoritative.
- **NOT DONE (explicitly deferred, not silently skipped):** SDUI page inspection for this entity (#4/#8 above); a dedicated authorization-rejection test for this entity (#5); a dedicated audit-row-content test for this entity (#6) — all three rely on already-proven generic mechanisms rather than being unverified claims.

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
- [x] **GOAL:** `set_tenant_context()` itself refuses non-ACTIVE tenants, exactly as `RLS_SPEC.md`/`SECURITY_MODEL.md`/`TENANT_LIFECYCLE.md` all specify, so every caller — HTTP or internal — inherits the guarantee.
- **WHY:** Only one Fiber middleware (`TenantResolver`) checked tenant status, and `POST /api/v1/auth/login` is mounted outside that middleware's group entirely — a suspended/archived tenant's users could still authenticate and receive a live session.
- **FILES:** `migration/bootstrap/002_utilities.up.sql` (the real, embedded `set_tenant_context`/`current_tenant_id` — this is the version that actually ships, not `generator/generator.go`'s template alone), `testutil/db/db.go` (`InstallTenantLifecycle`, `CreateTenant`, `ActivateTenant`/`TryActivateTenant` — installs and drives the real function in tests), `platform/iam/service.go` (Login path).
- **IMPLEMENTATION:** `set_tenant_context(p_tenant_id uuid)` looks up `platform_tenant.status`, raises `P0001` (not found) / `P0002` (not ACTIVE) before calling `set_config`. `AuthService.Login` wraps the `set_tenant_context` call with `dberr.Parse`, rolls back, and writes a failed-login audit record on rejection.
- **TESTS:** `platform/iam/service_integration_test.go`: `TestAuthService_Login_SuspendedTenant_Rejected`, `TestAuthService_Login_ArchivedTenant_Rejected`, `TestAuthService_Login_PendingTenant_Rejected`, `TestAuthService_Login_NonexistentTenant_Rejected`, `TestAuthService_Login_ActiveTenant_ValidCredentials_Succeeds` (also caught and fixed a real `Login` panic — see 1.2 below). All against real PostgreSQL, real `set_tenant_context`.
- **ACCEPTANCE CRITERIA:** [x] Login rejection proven for SUSPENDED/ARCHIVED/PENDING/nonexistent tenants. [x] `testutil/db` drives the real production function, not a simplified stand-in. [x] Legitimate ACTIVE-tenant login still succeeds (regression-tested).

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
- [x] **GOAL:** A logged-out token cannot be revived by the Redis-miss PG-recovery path.
- **WHY:** Redis delete was authoritative and immediate; the PG `revoked_at` write was best-effort; `recoverSessionFromDB` trusted PG unconditionally on a Redis miss — a logout followed by a well-timed request could resurrect the session from the PG audit copy.
- **FILES:** `auth/session_store.go` (new `SessionStore.IsRevoked` method), `contrib/redis/session_store.go` (tombstone keys: `Delete`/`DeleteAll` now write a `session:revoked:{token}` key with a 24h TTL before/after the real delete), `platform/iam/service.go` (`ValidateToken` checks `IsRevoked` before falling back to `recoverSessionFromDB`).
- **IMPLEMENTATION:** Redis tombstone, not a synchronous-PG-write approach — a short-TTL sentinel key that `ValidateToken` checks before ever trusting a PG-recovered session row. `IsRevoked` failing (Redis unavailable) fails closed: 503, not a silent fall-through to PG.
- **TESTS:** `TestAuthService_ValidateToken_RevocationTombstone_BlocksResurrection` (`platform/iam/service_integration_test.go`) — verified red (reverting the `IsRevoked` check reproduces resurrection) then green.
- **ACCEPTANCE CRITERIA:** [x] Resurrection is proven closed by a real regression test. [x] Redis-unavailable path fails closed (503), not open.

### 1.3 Bulk import/export must not bypass the pipeline
- [ ] **STATUS: confirmed still open, deliberately not touched this pass** — tracked separately from the rest of Phase 1's security-hardening work per explicit scope control (this is a pipeline/audit-integrity gap, not an RLS/tenant/session boundary issue). Re-verified `ioport`/`BulkCreate` are unchanged.
- **GOAL:** Every row created via `ioport.Import` runs the same validation/hooks/audit as `Create`.
- **WHY:** `BulkCreate` is a raw `pgx.Batch` INSERT with zero pipeline integration; every CSV/JSON import is invisible to the audit trail and exempt from required/immutable-field rules.
- **FILES:** `ioport/importer.go`, `contrib/pgx/repo.go` (`BulkCreate`), `driver/repository.go`.
- **DEPENDENCIES:** none.
- **IMPLEMENTATION:** Either (a) route `ioport.Import` through per-record `Create` (accept the performance cost, or batch-with-pipeline in chunks), or (b) keep `BulkCreate` as a distinct, explicitly-documented "trusted, pipeline-exempt" path used only where the caller has already validated/audited by other means, and make `ioport.Import` use path (a) exclusively. Do not leave the current silent default.
- **TESTS:** import a CSV missing a required field → expect a rejection, not a silently-persisted invalid row. Import a batch → expect one `platform_audit_log` row per created record.
- **ACCEPTANCE CRITERIA:** [ ] Required/immutable-field violations in an import are rejected. [ ] Every imported record has a corresponding audit record. [ ] Performance regression (if any) measured and accepted or mitigated with chunked pipelined batches.

### 1.4 Workflow-trigger durability (implement the documented outbox)
- [x] **STATUS: closed — Phase 2 Steps 3B/4/6.** Re-verified against the actual current source (not assumed from this entry's own prior text, which described a design ADR-025 later superseded — see below).
- **GOAL:** `WorkflowTrigger` starts survive a crash or transient Temporal outage between commit and dispatch. Met.
- **WHY (historical):** `EntityService.startWorkflows` used to call Temporal directly and synchronously; failure was a log line and a `// TODO`. Fixed.
- **ARCHITECTURE NOTE — supersedes this item's original text:** ADR-025 (`docs/adr/ADR-025-transactional-events-and-workflow-durability.md`) deliberately rejected a second, workflow-specific `workflow_outbox` table (the design this item originally called for). The actual implementation reuses the single `events_outbox` table (Phase 2 Step 2) for both lifecycle events and workflow-start intents (`events.EventWorkflowTriggerFired`, carrying a `def.ActionWorkflowSpec` payload), dispatched by the same relay via a dedicated `events/outbox.WorkflowTriggerSubscriber` (Step 4) that calls `workflow.WorkflowExecutor.Start`. There is exactly one durable outbox, not two.
- **RESOLVED:** `events/outbox/relay.go`'s tenant-context restoration (this item's own "NEW FINDING") was fixed in Step 3B — `processOne`/`restoreTenantContext` now restore `tenant.TenantContext` from each event's own `TenantID` before dispatch, never the relay loop's ambient context. `EntityService.Create/Update/Delete/CreateBatch` (Step 6) now publish `EventWorkflowTriggerFired` transactionally instead of calling Temporal directly; `runtime.ActionContext.StartWorkflow` (Step 4) does the same for custom actions. `router.RegisterOptions.Temporal` (the old direct-client field) is removed; the relay's `WorkflowTriggerSubscriber` is wired directly against a `workflow.WorkflowExecutor` at process bootstrap (`cmd/server/main.go`).
- **FILES:** `api/service/entity.go`, `runtime/runtime_action_context.go`, `events/outbox/workflow_trigger_subscriber.go`, `events/outbox/relay.go`, `cmd/server/main.go`, `api/router/router.go`.
- **TESTS:** real-PostgreSQL tests proving workflow intent survives mutation rollback/commit, relay dispatch after commit, Temporal-error retryability, and tenant-context restoration per event — see `api/service/entity_workflow_intent_pg_test.go`, `api/service/action_context_pg_test.go`. WorkflowID-based duplicate-dispatch safety is Temporal's own responsibility (documented, not re-implemented) — the relay/outbox layer remains at-least-once, never exactly-once, by design (ADR-025 §11).
- **ACCEPTANCE CRITERIA:** [x] Durable outbox row exists per workflow-trigger event, populated transactionally with the entity mutation (in `events_outbox`, not a separate table — see architecture note). [x] Relay dispatches pending rows via `WorkflowTriggerSubscriber`. [x] `EntityService.*` no longer calls Temporal directly (verified by source-text regression test). [x] `events/outbox/relay.go` restores tenant context before invoking a subscriber. [ ] `docs/08-workflow/OUTBOX_SPEC.md`/`TEMPORAL_INTEGRATION.md` fully rewritten to match reality — partially corrected this pass (see Step 6 report), a full pass over both documents remains a documentation follow-up, not a code gap.

### 1.5 Casbin policy reload on permission change
- [ ] **STATUS: confirmed still open** (re-checked this pass — `command grep -rn "\.Reload("` across all non-test `.go` files still returns zero production callers). Not addressed this pass — out of scope for the RLS/tenant/session boundary work.
- **GOAL:** Revoking a role's permission takes effect without a process restart.
- **WHY:** `CasbinEvaluator.Reload()` exists, works, and is never called anywhere outside its own package.
- **FILES:** `auth/casbin.go`, wherever `iam_role_permissions` mutations happen (likely a `platform/iam` admin handler/service method).
- **DEPENDENCIES:** none.
- **IMPLEMENTATION:** Call `Reload()` (or an incremental policy update, if Casbin supports it more cheaply) from the `iam_role_permissions` mutation path. Consider a hook (`AfterCreate`/`AfterDelete` on that entity) mirroring the existing `UserRoleChangeHook` pattern.
- **TESTS:** revoke a permission from a role with a live session holding it; call the now-forbidden endpoint immediately (no restart) → expect 403.
- **ACCEPTANCE CRITERIA:** [ ] Test above passes. [ ] Reload latency documented (should be near-immediate, not batched/delayed in a way that reintroduces a meaningful window).

### 1.6 Fix `/auth/logout` and `/auth/me` (currently always 401)
- [x] **GOAL:** Self-service logout actually works.
- **WHY:** Both routes read `c.Locals("session")`, populated only by `RequireAuth`, which wasn't mounted ahead of them.
- **FILES:** `platform/iam/handler.go` (`registerRoutes` takes a `requireAuth fiber.Handler`, applied to `/logout` and `/me`), `platform/iam/module.go` (`WithAuthMiddleware` builder — platform/iam deliberately does not import `awo/api/middleware` itself, to avoid inverting framework layering; the caller injects the constructed middleware), `cmd/server/main.go` / `cmd/awo/serve_impl.go` (wire `iamModule.WithAuthMiddleware(middleware.RequireAuth(iamModule.Auth, auditSigningSecret))` before `RegisterRoutes`).
- **TESTS:** `platform/iam/handler_route_test.go` (new, this pass) — real `fiber.App` + real `iam.Module.RegisterRoutes` wiring, not a hand-built stand-in: `TestLogoutRoute_ValidToken_Returns200_AndActuallyRevokesSession` (logs in via the real `/login` route, calls `/logout` with the token, then proves `ValidateToken` rejects it afterward), `TestMeRoute_ValidToken_ReturnsIdentity`, `TestMeRoute_NoToken_Returns401`. This closes the acceptance-criteria gap the original audit called out explicitly — prior tests only exercised `AuthService` directly and would not have caught the routing bug.
- **ACCEPTANCE CRITERIA:** [x] Route-level integration test passes (not just `AuthService`-level). [x] `GET /api/v1/auth/me` returns the caller's identity with a valid token. [x] `/me` correctly still returns 401 with no token (verified not accidentally wide-open now that real middleware is mounted).

### 1.7 Connection-pool tenant-context isolation (adversarial verification)
- [x] **GOAL:** Prove — with real PostgreSQL, a real pooled connection, and the real (transaction-local) `set_tenant_context` — that connection reuse across tenants cannot leak tenant context or data, including on rollback, on a failed tenant-context setup, on nested transactions, and under genuine concurrency.
- **WHY:** `set_tenant_context`'s GUC is transaction-local (`set_config(..., true)`); this property is exactly what makes pooled-connection reuse across tenants safe, and it had no direct test.
- **FILES:** `contrib/pgx/connection_pool_test.go` (new).
- **IMPLEMENTATION (test-only — no production code changed):** `TestConnectionPool_CommitDoesNotLeakTenantContext`, `TestConnectionPool_RollbackDoesNotLeakTenantContextOrData`, `TestConnectionPool_FailedTenantContextSetup_DoesNotPoisonConnection`, `TestConnectionPool_NestedWithTx_ReusesSameTransaction_NoSecondAcquire`, `TestConnectionPool_ConcurrentTenants_TrulyIsolated` (8 goroutines, real multi-connection pool via `testdb.OpenConcurrentPool`).
- **PITFALL CAUGHT WHILE WRITING THESE TESTS:** the first version produced an apparent cross-tenant leak that was actually a test bug — `testdb.CreateTenant` internally calls `ResetRole` (needs superuser to insert into `platform_tenant`), which silently undid an earlier one-time `SET ROLE awo_app` in the test, so both "tenant" transactions ran as PostgreSQL superuser (which unconditionally bypasses RLS). Fixed by adding a `switchToAppRole` helper that asserts `current_user` actually changed, called after all `CreateTenant` calls and immediately before exercising `WithTx`; documented as a standing pitfall in `testutil/db/db.go`'s `CreateTenant` doc comment.
- **ACCEPTANCE CRITERIA:** [x] All 5 tests pass against real PostgreSQL. [x] Concurrency test uses genuine separate physical connections, not sequential reuse.

### 1.8 Filter/query adversarial security tests, and a real SQL-injection fix
- [x] **GOAL:** Prove adversarial filter/sort inputs cannot bypass RLS or reach raw SQL unescaped.
- **WHY:** Auditing `contrib/pgx/repo.go`'s `Query` method found a genuine, remotely-exploitable SQL injection: the `ORDER BY` clause interpolated `qo.SortField` (traceable to `api/handler/crud.go`'s unvalidated `?orderBy=` query parameter) directly into the SQL string with no allowlist and no identifier quoting.
- **FILES:** `contrib/pgx/repo.go` (`Query`'s `ORDER BY` construction — now allowlist-checked via `sqlbuild.NewAllowlist(r.schema).Check(qo.SortField)` and quoted via the newly-exported `sqlbuild.QuoteIdent`), `contrib/pgx/sqlbuild/sqlbuild.go` (`QuoteIdent`, exported), `contrib/pgx/sqlbuild/allowlist.go` (`NewAllowlist` fixed to read both `es.Fields` and `es.FieldsByName` — a pre-existing test fixture only populated the latter, which would have made the allowlist reject legitimate fields), `contrib/pgx/filter_security_test.go` (new), `contrib/pgx/rls_defense_test.go` (pre-existing, unmodified, still passing — covers WHERE-clause adversarial cases; this new file covers `ORDER BY`/sort-field cases specifically).
- **TESTS:** `TestQuery_SortField_RejectsSQLInjectionAttempt` uses the payload `` name" -- `` — verified via raw `psql` to succeed *silently* pre-fix (the trailing `--` swallows the syntax break, producing valid-but-manipulated SQL with no error at all, which is what makes this class of bug dangerous rather than just noisy) — plus `TestQuery_SortField_UnknownColumn_Rejected`, `TestQuery_SortField_LegitimateField_StillWorks`, `TestFilterSecurity_NotOperator_StillRLSScoped`, `TestFilterSecurity_NullComparison_RLSScoped`, `TestFilterSecurity_BulkUpdate_CannotCrossTenant`, `TestFilterSecurity_Delete_CannotCrossTenant`.
- **KNOWN GAP LOGGED, NOT FIXED (separate from the SortField injection):** `sqlbuild.Allowlist`/`BuildWithAllowlist` — a proper, unit-tested field-authorization mechanism — has **zero production callers**. `Repository.Query`'s WHERE-clause building uses the plain `sqlbuild.Build`, which is not exploitable for injection (its `quoteIdent` already escapes WHERE-clause field names), but it is an authorization/exposure gap: nothing stops a filter from referencing an undeclared column name that happens to exist on the table. Lower severity than the fixed SortField issue; deferred — see new item 1.13 below.
- **ACCEPTANCE CRITERIA:** [x] SQL injection via `orderBy` closed and regression-tested (red confirmed pre-fix, green after). [x] Legitimate sort fields still work. [x] WHERE-clause adversarial cases (pre-existing `rls_defense_test.go`) still pass.

### 1.9 Organisation-scope RLS security boundary (verification only — `OrganizationService` remains unimplemented)
- [x] **GOAL:** Prove the `ScopeOrganization` RLS policy (`org_id = current_org_id()`) actually isolates sibling organisations within one tenant, and fails closed with no org context set — without building out `platform/organization.OrganizationService` (confirmed still 100% stubbed: every method returns `"not implemented"` — out of scope per explicit instruction to test the boundary, not implement the service).
- **FILES:** `generator/generator.go` (extracted exported `OrgContextSQL()`, mirroring the existing `TenantContextSQL()` pattern, so tests install the real `set_org_context`/`current_org_id` functions instead of a hand-copied restatement), `contrib/pgx/organization_security_test.go` (new).
- **TESTS:** `TestOrganizationRLS_SiblingOrgsIsolated`, `TestOrganizationRLS_NoOrgContext_SeesNothing` — raw SQL mirroring exactly what `generator.go` emits for `ScopeOrganization`, run inside explicit transactions (`set_org_context`'s GUC is transaction-local, same as `set_tenant_context`'s — a bare auto-commit statement would not see its own context on the next statement).
- **ACCEPTANCE CRITERIA:** [x] Sibling-org isolation proven. [x] Fail-closed (zero rows) with no org context proven. [x] `OrganizationService`'s stub status re-confirmed via source read, not assumed from the prior audit.

### 1.10 Platform-admin re-verification (no RLS bypass exists; two misleading comments fixed)
- [x] **GOAL:** Re-audit the current `platform_admin` implementation from scratch — do not assume a bypass exists (or doesn't) just because a prior audit said so.
- **FINDING:** No RLS/tenant-isolation bypass exists anywhere for platform admins. The only privilege elevation is a Casbin (permission) bypass in `api/authz/authz.go`'s `RequirePermission` (`viewer.IsPlatformAdmin()` skips the `CanPerform` check) — it is audit-logged (`slog.InfoContext` on every bypass) and the viewer's real `tenant_id` still flows through normal RLS; every `CREATE POLICY` in the codebase uses plain `tenant_id`/`org_id` equality with no admin-flag branch.
- **BUGS FOUND AND FIXED (doc-only, no behavior change):** `runtime/tenant.SystemContext()`'s doc comment falsely claimed repository operations "bypass RLS via ... a special platform admin token" — no such mechanism exists anywhere in `set_tenant_context()`, and `SystemContext` has zero production callers. `migration/bootstrap/002_utilities.up.sql`'s `current_tenant_id()` comments similarly implied a NULL tenant context was an intentional "platform-admin bypass" — corrected: `tenant_id = NULL` is never true in SQL, so a NULL tenant context fails closed (zero rows) on every tenant-scoped table; the only tables genuinely reachable with no tenant context are ones with no tenant-scoping RLS policy at all (e.g. `platform_tenant` itself).
- **FILES:** `runtime/tenant/tenant.go`, `migration/bootstrap/002_utilities.up.sql` (comments only), `contrib/pgx/platform_admin_security_test.go` (new).
- **TESTS:** `TestSystemContext_NilTenantID_RejectedNotBypassed` — proves `set_tenant_context(uuid.Nil)` (what `SystemContext`'s sentinel `TenantID` would produce if ever wired into `Repository.WithTx`) is rejected with `tenant_not_found` (P0001), not silently accepted as a platform-wide context.
- **ACCEPTANCE CRITERIA:** [x] No RLS bypass found (confirmed by direct source review of every `CREATE POLICY`, not inference). [x] Misleading comments corrected. [x] Regression test proves `SystemContext`'s sentinel value is rejected, not silently trusted.

### 1.11 Background/Temporal tenant-context audit (verification only — no async subsystem is live yet)
- [x] **GOAL:** Establish how tenant identity would propagate through the outbox, Temporal workflows/activities, the scheduler, and other async workers — without implementing the workflow-outbox durability redesign (that is item 1.4, separately tracked).
- **FINDING:** Every candidate subsystem is either pure scaffolding or fully unwired: **Temporal** — no real workflow/activity functions exist anywhere, only a codegen template (`cmd/awo/scaffold.go`); **scheduler** (`scheduler/scheduler.go`) — a generic `robfig/cron` wrapper with zero production callers, runs jobs with a bare `context.Background()` and no per-tenant iteration primitive; **mail/notification** (singular) — entity definitions only, explicitly documented as future work; **`platform/notifications`** (plural) — has a real driver/hook/service but dispatch is synchronous and in-transaction (inherits the caller's already-tenant-scoped context correctly), and nothing calls `RegisterDriver`/`NewService` outside tests; **search indexing / import-export / report generation** — no async implementation exists at all. The one real finding is logged under 1.4 above (the outbox relay's `deliver` not restoring tenant context — currently unreachable, no subscribers exist).
- **VERIFIED (not just inferred) the two-layer defense a background job gets today if it ever runs with no tenant context at all:** Go-level write path — `Repository.Create`/`BulkCreate` call `tenant.FromContext` (not `TryFromContext`) and panic immediately, fail-fast, rather than writing an unscoped/wrong-tenant row. SQL-level read path — `Repository.WithTx` uses `tenant.TryFromContext` and silently skips `set_tenant_context` if absent (fail-open at the Go level, in isolation), but `current_tenant_id()` then reads NULL and every tenant-scoped RLS policy's `tenant_id = current_tenant_id()` is never true for NULL, so reads return zero rows, not every tenant's data (fail-closed in net effect).
- **NEW RELIABILITY FINDING, LOGGED NOT FIXED:** `Repository.WithTx` has no panic recovery — a panic inside its `fn` callback (e.g. `Create`'s intentional panic on missing tenant context) abandons the open transaction without releasing its connection back to the pool. Discovered while writing this item's own regression test (the first draft triggered exactly this hang against a `pool_max_conns=1` test pool). Not fixed this pass — a connection/transaction lifecycle change, out of scope for the RLS/tenant-context security boundary this phase targets; needs its own dedicated pass (likely a `defer recover()` in `WithTx` that still rolls back and re-panics).
- **FILES:** `contrib/pgx/missing_tenant_context_test.go` (new). No production code changed for this item specifically (the outbox finding is logged under 1.4; the `WithTx` panic-safety finding is logged here for a future pass).
- **TESTS:** `TestRepository_Create_NoTenantContext_PanicsRatherThanWritingUnscoped` (calls `Create` directly, not through `WithTx`, specifically to avoid the panic-leaks-the-connection hazard above), `TestRepository_Query_NoTenantContext_SeesNothing_NotEveryTenant`.
- **ACCEPTANCE CRITERIA:** [x] Confirmed, per subsystem, implemented vs. stubbed vs. dead code — not padded with doc references for subsystems that don't exist in code. [x] Fail-closed behavior for a tenant-context-less background job proven by real tests, not asserted from reading the code. [x] New findings (outbox tenant-context restoration, `WithTx` panic safety) logged where a future implementer will find them, not lost.

### 1.12 Dependabot vulnerability triage
- [ ] **STATUS:** triaged, none fixed this pass (none affect the Phase 1 security boundary; see below).
- **CURRENT STATE (re-pulled from `gh api repos/niiniyare/awo/dependabot/alerts` this pass — the count and contents differ from whatever was reported in the original audit, which is expected: Dependabot alerts continuously appear and resolve):** 14 open alerts.
  - **#62 — `google.golang.org/grpc` v1.83.1, HIGH, transitive, runtime scope** (GHSA-2v4p-qf9q-27wj, DoS panic in the xDS routing interceptor of servers built with `xds.NewGRPCServer`). **Exposure: none** — confirmed via source search that this codebase never imports `google.golang.org/grpc` directly anywhere; it is pulled in transitively (almost certainly via the Temporal SDK's gRPC client transport) and no xDS-based gRPC *server* is constructed anywhere. Not exploitable as currently used. Safe to bump when convenient (compatible transitive version bump); not fixed this pass since it doesn't touch the security boundary this phase targets.
  - **13 alerts, all `web/pnpm-lock.yaml`, all `scope: development`** — `vite` (direct devDependency, 5 alerts, MEDIUM–HIGH), `postcss` (transitive via vite, 4 alerts, MEDIUM–HIGH), `nanoid` (transitive, 1 open + 3 auto-dismissed superseded alerts, HIGH), `esbuild` (transitive, 1, LOW), `picomatch` (transitive, 1 open + 1 auto-dismissed, MEDIUM), `rollup` (transitive, 1, HIGH). All are the frontend build toolchain's *devDependencies* — they run only during `pnpm build`/`pnpm dev` on a developer or CI machine, never in the deployed server or in the bundle shipped to browsers. Exposure is bounded to the build environment/supply chain, not the running application. `web.old/` (a second, unreferenced directory — confirmed via `cmd/server/main.go`/`cmd/awo/serve_impl.go` that only `web/` is ever served) has its own `package.json` but generated zero alerts against it in this pull; if it is genuinely dead, deleting it removes its scan surface entirely regardless of vulnerabilities.
- **RECOMMENDATION:** Bump `web/`'s vite/postcss/esbuild/rollup/nanoid/picomatch versions in a normal dependency-maintenance pass (low risk, no runtime exposure, but still worth clearing since Dependabot will keep reporting them); bump `google.golang.org/grpc` to ≥1.83.2 opportunistically. Neither blocks Phase 1. Confirm whether `web.old/` should simply be deleted (appears to be superseded by `web/`, referenced nowhere in Go code).
- **ACCEPTANCE CRITERIA:** [x] Every open alert classified with package, scope, direct/transitive, and evidence-based exploitability (not guessed). [x] None fixed blindly — the one runtime/Go alert was confirmed unreachable before deciding not to fix it immediately. [ ] Actual version bumps — deferred to a dependency-maintenance pass, not part of Phase 1.

### 1.13 `sqlbuild.Allowlist` has zero production callers (authorization gap, lower severity than 1.8's injection fix)
- [ ] **GOAL:** Either wire `sqlbuild.BuildWithAllowlist`/`Allowlist` into `Repository.Query`/`BulkUpdate`/`Update`'s WHERE-clause construction, or explicitly document why the plain `sqlbuild.Build` (no field authorization) is an accepted risk.
- **WHY:** Found while fixing 1.8's `ORDER BY` SQL injection. `Allowlist` is a real, unit-tested, already-correct mechanism (`contrib/pgx/sqlbuild/allowlist.go`) — but nothing in production calls it for WHERE-clause building. This is NOT a SQL-injection risk (`sqlbuild.Build`'s `quoteIdent` already escapes field names correctly) — it is an authorization/exposure gap: a filter can reference any column that exists on the underlying table, not just fields the entity schema declares, since there is no allowlist check on the WHERE-clause path.
- **FILES:** `contrib/pgx/repo.go` (`Query`, `BulkUpdate`, `Update` — wherever `sqlbuild.Build` is called instead of `sqlbuild.BuildWithAllowlist`).
- **DEPENDENCIES:** none.
- **ACCEPTANCE CRITERIA:** [ ] Decision made and implemented (wire it in, or document the accepted risk explicitly in code and in `RLS_SPEC.md`/equivalent).

## Phase 1 Security Closure — P0-A / P0-B (this pass)

Triggered by 1.8's discovery of a real ORDER BY SQL injection: a dedicated
security-closure pass to (a) prove that fix holds against a much wider
adversarial payload catalogue, (b) audit every other dynamic-SQL surface in
the codebase for the same class of bug, and (c) resolve the `Repository.WithTx`
panic-safety gap 1.11 had logged but not fixed. Full narrative, evidence, and
the PASS/FAIL acceptance matrix are in `PHASE1_SECURITY_CLOSURE_REPORT.md`;
this section is the roadmap-tracking summary.

### 1.14 P0-A: `updateSystem`/`BulkUpdate` write-path SQL injection and column-authorization bypass
- [x] **GOAL:** No client-controlled map key can reach `Repository.Update`/`BulkUpdate`'s `SET` clause without being both allowlist-checked and correctly identifier-quoted.
- **WHY — genuinely live, more severe than the ORDER BY bug it was found while auditing:** `updateSystem`'s `for field, val := range input.Data { sets = append(sets, fmt.Sprintf('"%s" = $%d', field, ...)) }` interpolated the *map's own keys* with no escaping and no allowlist. `input.Data` is exactly the raw, unrestricted `map[string]any` from `PATCH /api/v1/entities/:entity/:id`'s JSON body (`api/handler/crud.go` → `EntityService.Update` → `runtime/pipeline.go`'s `RunBeforeUpdate`/`validateFields`, which check `ImmutableFields`/`RequiredFields`/`FieldValidators` but never reject an unrecognized key) — reachable end-to-end with no upstream filtering, on any `IsSystem` entity (the framework's own tables: `iam_users`, `iam_sessions`, etc., not just business entities). Beyond the quote-breakout injection itself, the same unchecked-key pattern let a caller include `"tenant_id"` or `"id"` as an ordinary patch field, which — independent of any injection — would let a caller reassign a record's tenant or primary key through a normal field update.
- **FILES:** `contrib/pgx/repo.go` (`updateSystem`, `BulkUpdate`'s system branch, new `checkWritableField` helper), `contrib/pgx/write_injection_test.go` (new).
- **IMPLEMENTATION:** Added `Repository.checkWritableField(field)`, checked against `r.schema.FieldsByName`/`Fields` — deliberately narrower than `sqlbuild.NewAllowlist` (used for read contexts), since it must exclude the standard framework columns (`id`, `tenant_id`, `created_at`, `updated_at`, `deleted_at`, `custom_fields`) that a write context must never let through, regardless of whether RLS's own `WITH CHECK` would also reject the result. Identifiers are quoted via `sqlbuild.QuoteIdent` instead of naive `fmt.Sprintf("%s")`.
- **TESTS (real PostgreSQL):** `TestUpdate_MaliciousFieldName_RejectedNotInjected`, `TestUpdate_TenantIDInPatchBody_Rejected`, `TestUpdate_IDInPatchBody_Rejected`, `TestUpdate_CustomFieldsColumnDirectOverwrite_Rejected`, `TestUpdate_AnyUndeclaredColumnName_Rejected`, `TestUpdate_LegitimateField_StillWorks`, `TestBulkUpdate_MaliciousFieldName_RejectedNotInjected`, `TestBulkUpdate_LegitimateField_StillWorks`. Verified red against the pre-fix code (temporarily reverted, confirmed the malicious-field-name and undeclared-column tests fail — i.e. the pre-fix code accepts them — while, interestingly, the `tenant_id` reassignment attempt was independently blocked even pre-fix by RLS's implicit `WITH CHECK` clause, confirming genuine defense-in-depth rather than a single point of failure), then restored and confirmed green.
- **ACCEPTANCE CRITERIA:** [x] Malicious field names rejected, not executed. [x] Framework-managed columns (`tenant_id`, `id`, `custom_fields`) rejected even though syntactically harmless. [x] Legitimate declared fields still patchable. [x] Verified red/green against real PostgreSQL.

### 1.15 `Repository.Aggregate` equivalent injection (found via the Step-5 dynamic-SQL audit; zero production callers today)
- [x] **GOAL:** Close the same vulnerable shape in `Aggregate` before anything comes to depend on it.
- **WHY:** `fn.Field` and `spec.GroupBy` used the identical naive `fmt.Sprintf('"%s"', ...)` pattern the ORDER BY bug had; `fn.Fn` (the aggregate function name) was interpolated completely unquoted with no validation against the 5 known functions at all. Confirmed via exhaustive caller search (`driver.AggregateSpec`/`Repository.Aggregate`) that this method has **zero production callers** — `testing/fakestore` is the only other implementation, used in tests only — so this is a landmine fix, not a live incident.
- **FILES:** `contrib/pgx/repo.go` (`Aggregate`, new `isValidAggregateFn` helper), `contrib/pgx/write_injection_test.go`.
- **IMPLEMENTATION:** `fn.Field`/`spec.GroupBy` now go through `sqlbuild.NewAllowlist(r.schema).Check` + `sqlbuild.QuoteIdent` (read context — standard columns legitimately groupable/aggregatable). `fn.Fn` is checked against an exact-match set of the 5 declared `driver.AggregateFn` constants — a function name can't be identifier-quoted the way a column can (quoting would turn it into a column reference), so an allowlist is the only defense available for that position.
- **TESTS (real PostgreSQL):** `TestAggregate_MaliciousFieldName_Rejected`, `TestAggregate_MaliciousGroupBy_Rejected`, `TestAggregate_UnknownFunction_Rejected`, `TestAggregate_LegitimateUsage_StillWorks`.
- **NEW NON-SECURITY BUG FOUND, LOGGED NOT FIXED:** `Aggregate`'s row-scanning only ever calls `rows.Next()` once and never loops — `driver.AggregateResult.Groups` (documented as populated "when `AggregateSpec.GroupBy` is set") is never actually populated anywhere in the method; a GroupBy producing more than one group would silently return only the first group's values. Out of scope for a security pass (a correctness bug, and the method has zero production callers regardless) — needs its own fix when `Aggregate` gains a real caller.
- **ACCEPTANCE CRITERIA:** [x] All 4 new tests pass. [x] Zero-caller status confirmed by source search, not assumed.

### 1.16 `report.GenerateSQL` — GroupBy/OrderBy/Aggregate field & function validation (found via the Step-5 audit; zero production callers today)
- [x] **GOAL:** Close the same class of gap in the standalone report-SQL generator.
- **WHY:** `report.GenerateSQL`'s `GroupBy`, `OrderBy[].Field`, and `Aggregates[].Field` were passed through `quote()` (which strips embedded `"` characters before wrapping — this actually already prevented identifier-breakout injection, verified by re-reading the function, not assumed) but were never checked against the entity schema at all, unlike `Fields[].Name` and `Joins[].EdgeName` in the same function. `Aggregates[].Func` (the aggregate function name) was interpolated completely unquoted and unvalidated — the same severity class as 1.15's `fn.Fn` finding. Confirmed via exhaustive caller search that `report.GenerateSQL` has **zero production callers** anywhere in the codebase.
- **FILES:** `report/report.go`, `report/report_test.go`.
- **IMPLEMENTATION:** Added `isValidAggregateFunc` (exact-match against the 5 declared `AggregateFunc` constants). Added a `knownOutputName` check (real entity field OR an alias already projected in the SELECT clause — `GroupBy`/`OrderBy` are documented to accept either) for `GroupBy` and `OrderBy[].Field`; added a schema-membership check for `Aggregates[].Field`. Deliberately left `ReportField.Expr` untouched — it is documented as an intentional raw-SQL escape hatch for Go-code-authored report definitions (the same trust level as hand-writing SQL) and touching it would be redesigning intended functionality, not fixing a gap; its doc comment now states explicitly that it must never be populated from end-user/API input.
- **TESTS:** `TestGenerateSQL_UnknownGroupByField_Error`, `TestGenerateSQL_UnknownOrderByField_Error`, `TestGenerateSQL_OrderByAlias_Succeeds`, `TestGenerateSQL_UnknownAggregateFunc_Error`, `TestGenerateSQL_UnknownAggregateField_Error` — all pass, all 11 pre-existing tests in the file continue to pass unmodified.
- **ACCEPTANCE CRITERIA:** [x] All 5 new tests pass. [x] No regression in the 11 pre-existing tests. [x] `Expr`'s intentional raw-SQL status documented, not silently left ambiguous.

### 1.17 P0-B: `Repository.WithTx` panic safety
- [x] **GOAL:** A panic inside a `WithTx` callback must not leave the transaction's connection permanently checked out of the pool, and must still propagate as a normal Go panic (not be silently downgraded to an error).
- **WHY:** `BeginTx` checks out a physical connection that only `Commit`/`Rollback` returns to the pool (pgxpool's `Tx` wraps `Release` internally). Before the fix, neither ran if `fn` panicked — verified directly: a bounded-timeout diagnostic against a `pool_max_conns=1` pool showed a second, unrelated tenant's subsequent `WithTx` call fail with `"begin transaction: context deadline exceeded"` after a prior call's callback panicked, proving the connection was genuinely never released, not merely inferred from reading the code.
- **FILES:** `contrib/pgx/repo.go` (`WithTx`), `contrib/pgx/withtx_panic_safety_test.go` (new).
- **IMPLEMENTATION:** A `committed bool` flag plus `defer func() { if !committed { _ = pgxTx.Rollback(ctx) } }()`, set to `true` only after `Commit` succeeds. The defer does **not** call `recover()` — it runs during panic unwinding regardless (guaranteeing the rollback/connection-release always happens), and because it never recovers, the original panic continues propagating normally afterward. This was a deliberate choice over a recover-and-return-error pattern: a panic here means a real bug (e.g. a caller invoking `Create` with no tenant context), and downgrading it to a normal error return would hide exactly the class of bug `tenant.FromContext`'s intentional panic exists to surface loudly.
- **TESTS (real PostgreSQL, `pool_max_conns=1`):** `TestWithTx_PanicBeforeAnyMutation_RollsBackAndReleasesConnection`, `TestWithTx_PanicAfterMutation_RollsBackTheMutationToo`, `TestWithTx_PanicInNestedCall_CaughtByOutermostRollback` (proves the outer transaction's defer catches a panic from inside a nested `WithTx` call, which does no transaction management of its own via the `existing.InTx()` short-circuit), `TestWithTx_NoPanic_StillCommitsNormally` (regression guard on ordinary commit behavior). Verified red first via a throwaway, bounded-timeout diagnostic against the reverted code (confirmed genuine connection exhaustion, not inferred), then green after restoring the fix.
- **ACCEPTANCE CRITERIA:** [x] Panic still propagates to the caller (not swallowed). [x] Connection is provably reusable by a different tenant immediately afterward. [x] A mutation performed before the panic is rolled back, not partially committed. [x] A panic inside a nested `WithTx` call is still caught by the outermost transaction's cleanup. [x] Ordinary (non-panicking) commit behavior unchanged.

### 1.18 CRITICAL: `ScopeOrganization`/`ScopeOrganizationTree` RLS provides no actual isolation (found during final forensic verification)
- [x] **STATUS: closed — Phase 2 Step 8.1.**
- **GOAL:** Make the organisation-scoping RLS policy actually isolate organisations, not merely coexist syntactically with the tenant-isolation policy.
- **WHY:** `generator.go` emitted TWO separate `CREATE POLICY` statements for every `ScopeOrganization`/`ScopeOrganizationTree` entity — the org-scope policy AND the `tenant_isolation` policy every non-System-scope entity also gets. Both were PERMISSIVE (the `CREATE POLICY` default). PostgreSQL combines multiple permissive policies with OR, not AND: a row passes if it satisfies AT LEAST ONE applicable permissive policy. Since `tenant_isolation` alone is satisfied by every row in the tenant regardless of `org_id`, the org-scope policy was a complete no-op — for SELECT, INSERT, and UPDATE alike. Reproduced directly against real PostgreSQL (non-superuser `awo_app` role) both historically and again as this fix's RED verification: a caller in Org A's context saw Org B's rows too; INSERT with a foreign `org_id` (correct `tenant_id`) succeeded; UPDATE reassigning an existing row's `org_id` to a foreign organisation succeeded.
- **EXPLOITABILITY BEFORE THE FIX:** None — confirmed via exhaustive source search that zero entities in this codebase declare `ScopeOrganization`/`ScopeOrganizationTree` (consistent with `platform/organization.OrganizationService` being 100% stubbed, tracked separately under Phase 3). This was a landmine in an unused capability, not a live incident.
- **INTENDED SECURITY MODEL (verified against `def/scope.go` and the existing generator implementation before choosing a fix, per this task's own "do not guess" requirement):** `ScopeOrganization` is flat — a record belongs to exactly one organisation, and only a viewer whose active organisation context equals that `org_id` may access it (no hierarchy). `ScopeOrganizationTree` is hierarchical via `platform_organization`'s materialized `path` column ("/root-id/.../self-id/"): a viewer's active organisation may access its own records and every descendant's, never an ancestor's or a sibling's — this was already the correct direction encoded in `current_org_path()`'s `path LIKE current_org_path() || '%'` prefix match (viewer path is a prefix of the record org's path ⇒ record org is viewer's org or a descendant); the generator-emitted expression itself did not need to change, only how it combines with `tenant_isolation`. Tenant isolation remains the outer, non-negotiable boundary in all cases — confirmed `current_org_path()`'s own lookup against `platform_organization` is itself tenant-RLS'd, so an `org_id` belonging to a different tenant resolves to a NULL path and fails closed rather than resolving a foreign tenant's hierarchy.
- **FIX CHOSEN:** Declare the org-scope policy (`{entity}_org_isolation` / `{entity}_org_tree_isolation`) `AS RESTRICTIVE` instead of leaving it PERMISSIVE (the `CREATE POLICY` default). PostgreSQL ANDs RESTRICTIVE policies against the OR-combined PERMISSIVE result; since `tenant_isolation` is an entity's only PERMISSIVE policy, the effective check becomes exactly `tenant_isolation AND org_isolation` — a one-keyword-per-policy change, zero impact on tenant-only/system-scoped entities (which never get an org policy at all), and automatically covers `WITH CHECK` for INSERT/UPDATE (a policy without an explicit `WITH CHECK` uses its `USING` expression for both). Rejected: combining both conditions into one policy expression (functionally equivalent but a larger, less deterministic-to-diff generator change than adding one keyword); making `org_id` a generator-managed standard column (a separate, broader change unrelated to the OR/AND defect itself, left for a future task if `checkWritableField`'s org_id write-path coverage is ever needed).
- **FILES:** `generator/generator.go` (`generateEntitySQL`'s org-policy emission for `ScopeOrganization`/`ScopeOrganizationTree` now emits `AS RESTRICTIVE`; new exported `OrgPathSQL()` mirroring the existing `TenantContextSQL()`/`OrgContextSQL()` pattern, so `current_org_path()` has one canonical source instead of being hand-copied into `sharedInfraSQL` and test fixtures separately), `contrib/pgx/organization_security_test.go` (rewritten: the three "KnownGap" tests asserting the broken behavior are replaced with tests asserting the fixed, secure behavior, plus new hierarchy, cross-tenant-adversarial, write-side, and connection-pool coverage).
- **TESTS (real PostgreSQL, non-superuser `awo_app` role, real generator-exported context functions):** `TestOrganizationRLS_SiblingOrgsIsolated`, `TestOrganizationRLS_NoOrgContext_SeesNothing`, `TestOrganizationRLS_TenantIsolation_SameOrgID_DifferentTenants` (tenant boundary holds even when `org_id` values collide across tenants), `TestOrganizationRLS_WriteSide_OrgReassignment_Blocked` (INSERT and UPDATE into a foreign org now rejected; row-still-owned-by-original-org sanity check), `TestOrganizationRLS_Tree_Hierarchy` (5-node company/region/branch tree: parent sees all descendants, a mid-level node sees its own subtree only, siblings and cross-region are isolated, children cannot see ancestors), `TestOrganizationRLS_Tree_CrossTenantOrgIDCollision_Denied`, `TestOrganizationRLS_ConcurrentPool_AlternatingTenantsAndOrgs_NoLeakage` (genuine multi-connection pool via `testdb.OpenConcurrentPool`, 6 tenant/org fixtures read concurrently — Step 7a's connection-safety invariant re-verified, not regressed). All 7 tests pass. Generator-level regression: existing `TestGenerate_ScopeOrganization_RLSPolicy`/`TestGenerate_ScopeOrganizationTree_RLSPolicy`/index/infra tests in `generator/generator_test.go` pass unchanged (they assert substrings the fix preserves).
- **RED → GREEN:** Verified by temporarily reverting only the `AS RESTRICTIVE` keyword in both `generator/generator.go` and the test file's hand-mirrored DDL constants (leaving the new `OrgPathSQL` export in place so the package still compiles), confirming 5 of the 7 tests failed with exactly the predicted symptom (cross-org visibility, unrejected cross-org INSERT/UPDATE, full-tree visibility from a leaf node), then restoring both files to the fixed state and confirming all 7 pass again.
- **REGRESSION:** Full suite `go test ./... -count=1` green (all packages). Step 7a's standalone-read tenant-context tests and Step 7b's audit-actor-attribution tests re-run explicitly and pass unchanged. `gofmt -l .` clean, `go vet ./...` clean, `git diff --check` clean. `-race` unsupported on this host (android/arm64) — CI's dedicated `linux/amd64` race job remains the authoritative source, consistent with Steps 7a/7b.
- **ACCEPTANCE CRITERIA:** [x] A fix approach chosen and documented. [x] Sibling-organisation SELECT isolation restored and passing. [x] Cross-organisation INSERT/UPDATE rejected and passing. [x] Hierarchy semantics (parent→descendant allow, child→ancestor deny, sibling deny) verified against a real multi-node tree. [x] Cross-tenant adversarial case (colliding `org_id` across tenants) verified denied. [x] Connection-pool safety re-verified, Step 7a invariant not regressed. [x] Fix verified against real PostgreSQL with a non-superuser role. [x] RED→GREEN verified.

### 1.19 Standalone repository reads didn't establish RLS tenant context (found during Phase 2 Step 6 closure audit)
- [x] **STATUS: closed — Phase 2 Step 7a.**
- **GOAL:** `contrib/pgx.Repository.Get`/`Query`/`Exists`/`Count`/`Aggregate`, called outside an active transaction, correctly establish the caller's tenant context before the SQL that depends on it runs — without weakening RLS.
- **WHY:** The production `set_tenant_context()` (`migration/bootstrap/002_utilities.up.sql`) sets its RLS GUC with `set_config(..., true)` — PostgreSQL's `SET LOCAL` semantics: the value is scoped to whichever single transaction set it, and is gone the instant that transaction ends. `Repository.WithTx` already gets this right for mutations (calls `setTenantContext` fresh, inside its own transaction, before running its callback). `Get`/`Query`/`Exists`/`Count`/`Aggregate` never called `setTenantContext` themselves and, when called standalone (no surrounding `WithTx`) — the common case for `EntityService.Get`/`Query`/`Count`/`Exists`, the HTTP `GET`/list handlers, and `EntityService.Update`/`Delete`'s own opening `Get` before their mutation transaction — ran directly against a bare, auto-commit pool connection with no transaction of their own to carry the GUC into. `current_tenant_id()` then read NULL, and RLS's `tenant_id = current_tenant_id()` was never true for any row — failing closed (never a cross-tenant leak) but also hiding the caller's own, correctly-scoped records. Empirically reproduced (not just reasoned about) against the real, transaction-scoped production function with a multi-connection pool before the fix.
- **AFFECTED PATHS (all fixed by the same mechanism):** `Repository.Get`, `.Query`, `.Exists`, `.Count`, `.Aggregate` (the last three added to the fix beyond the two originally scoped, since they share the identical defect and the identical call pattern — `EntityService.Count`/`.Exists` call them standalone too). `EntityService.Update`/`.Delete`'s initial read and the HTTP `GET`/list handlers required no code changes of their own — they inherit the fix transparently through `Repository.Get`/`Query`.
- **FILES:** `contrib/pgx/repo.go` (new `withReadTx` helper; `Get`/`Query`/`Exists`/`Count`/`Aggregate` rewritten to use it; `Count`'s body extracted into an internal `countLocked` so `Query`'s own same-transaction `Total` count doesn't try to open a second read transaction), `contrib/pgx/standalone_read_tenant_context_test.go` (new), `api/service/entity_standalone_read_tenant_context_pg_test.go` (new).
- **IMPLEMENTATION:** `withReadTx(ctx, fn)`: if `ctx` already carries an active transaction, run `fn` directly on it (no redundant transaction). Otherwise, if `ctx` carries a `tenant.TenantContext`, open a short-lived transaction, call `setTenantContext` inside it (mirroring `WithTx`'s own panic-safe commit/rollback-via-defer pattern so a panic cannot leak the checked-out connection), run `fn`, then commit. Otherwise (no tenant context at all), run `fn` directly against the bare pool, unchanged from prior behavior — RLS still fails closed. `WithTx` itself was not modified.
- **TESTS (real PostgreSQL, `testdb.InstallTenantLifecycle`'s real production function, `testdb.OpenConcurrentPool`'s genuine multi-connection pool — not the single-connection/session-persistent setup that would have masked this bug):** `TestRepository_Get_StandaloneRead_RealLifecycleFunction_OwnerCanRead`, `..._TenantBCannotReadTenantA`, `..._NoTenantContext_FailsSafelyNotTenantWide`, `..._NonexistentRecord_NotFoundUnchanged`, `TestRepository_Query_StandaloneRead_RealLifecycleFunction_OwnerSeesOwnRecords`, `..._TenantIsolation`, `..._NoTenantContext_SeesNothing`, `TestRepository_ConcurrentTenants_StandaloneReads_NoCrossContamination` (16 goroutines × 20 iterations, two tenants, shared multi-connection pool), `TestRepository_Get_StandaloneRead_RepeatedCalls_DoNotLeakConnections` (`pool_max_conns=1`, 25×4 sequential calls), `TestEntityService_PG_Update_MultiConnectionPool_RealLifecycleFunction_InitialGetSucceeds`, `TestEntityService_PG_Delete_MultiConnectionPool_RealLifecycleFunction_InitialGetSucceeds`, `TestEntityService_PG_Update_MultiConnectionPool_TenantBCannotUpdateTenantARecord`. All verified RED against the pre-fix code (temporarily reverted via `git stash`, confirmed failing, restored) before being confirmed GREEN.
- **ACCEPTANCE CRITERIA:** [x] Tenant A can read tenant A's records via standalone Get/Query on a multi-connection pool under the real lifecycle function. [x] Tenant B cannot read tenant A's records. [x] No tenant context fails safely (empty/not-found), never tenant-wide. [x] Ordinary not-found semantics unchanged. [x] `EntityService.Update`/`Delete` work correctly under the real lifecycle function on a multi-connection pool. [x] Concurrent multi-goroutine, two-tenant test passes with no cross-contamination and no spurious not-found for the caller's own records. [x] No connection leak (repeated calls on a `pool_max_conns=1` pool never hang). [x] Full regression suite (`go test ./...`) remains green — 67+ packages, 0 failures. [x] `-race` unsupported on this host (android/arm64); rely on CI for race verification, not faked locally.

### 1.20 `record.Meta.Actor` never populated — real audit rows had NULL actor attribution (found during Phase 2 final gap audit)
- [x] **STATUS: closed — Phase 2 Step 7b.**
- **GOAL:** A real (non-test-stub) `EntityService` mutation's `platform_audit_log` row accurately identifies the actor who performed it — `actor_id` for a human user, `service_account_id` for a service account — never `NULL` for an authenticated, actor-driven mutation.
- **WHY:** `runtime.Pipeline.RunAuditRecord` (`runtime/pipeline.go`) reads `record.Meta.Actor` to attribute the audit row it writes. `contrib/pgx.Repository.Create`/`.Update` return a freshly-scanned `*def.EntityRecord` with no `Meta` field populated at all (confirmed by direct inspection of their row-construction code — `createSystem`/`createCustom`/`updateSystem`/`updateCustom`/`getWithDB` never touch `Meta`), even though the actor was already available to them via `driver.CreateInput.Actor`/`UpdateInput.Actor`. `Repository.Get` (what `EntityService.Delete`'s pre-deletion snapshot comes from) never populates `Meta` either, and `Repository.Delete` itself takes no actor parameter and returns no record. Net effect: every real Create/Update/Delete/CreateBatch mutation correctly authenticated the actor, correctly wrote the mutation, and correctly attributed the actor on the durable outbox lifecycle/workflow-intent events (Steps 3–6, which take `actor` as an explicit parameter and never read `Meta.Actor`) — but the audit row itself had `actor_id`/`service_account_id` = `NULL`. This went undetected because `api/service/entity_audit_test.go`'s `stubRepo` test double *does* correctly set `Meta: def.RecordMeta{Actor: input.Actor}` on Create/Update — masking the gap in every unit test; no test exercised the real `contrib/pgx.Repository` and then inspected the persisted `platform_audit_log` row's `actor_id` column.
- **AFFECTED PATHS (all fixed by the same one-line pattern):** `EntityService.Create`, `.Update`, `.Delete`, `.CreateBatch` (Step 5) — the one place `actor` is unconditionally in scope for every mutation type, and the exact boundary between "repository returns a record" and "the audit pipeline consumes it." Custom actions (`AsActionRepo`/`ActionContext`, Step 4) and cross-entity actions require no separate change — they delegate directly to `EntityService.Create`/`Update`/`Delete` and inherit the fix transparently.
- **FILES:** `api/service/entity.go` (`Create`/`Update`/`Delete`/`CreateBatch`: set `record.Meta.Actor = actor` on the repository-returned/pre-deletion record, immediately before the existing `RunAuditRecord` call; `publishLifecycleEvent`'s doc comment updated — it deliberately continues using its own explicit `actor` parameter, not `Meta.Actor`, so outbox attribution has one single always-correct source, unaffected by this fix), `api/service/entity_audit_actor_attribution_pg_test.go` (new).
- **IMPLEMENTATION:** No repository, audit-pipeline, or outbox code changed — `runtime.Pipeline.RunAuditRecord`'s signature and `contrib/pgx.Repository`'s behavior are both unchanged. The fix is entirely at the `EntityService` boundary: the same `actor *def.Actor` already passed into `driver.CreateInput`/`UpdateInput` (Create/Update) or already held from the method's own parameter (Delete/CreateBatch) is additionally assigned to the record's `Meta.Actor` field right before that record is handed to `RunAuditRecord`. `def.Actor`'s existing `UserID`/`ServiceAccountID` mutual-exclusivity (ADR-003/ADR-015) is unchanged — `audit.PostgresWriter.Write` already branches on which field is set; no second actor model was introduced.
- **TESTS (real PostgreSQL, assertions against the actual persisted `platform_audit_log` row, not merely `record.Meta.Actor != nil`):** `TestEntityService_PG_Create_AuditRow_ActorAttributionCorrect`, `TestEntityService_PG_Update_AuditRow_ActorAttributionCorrect` (also asserts before/after data unaffected), `TestEntityService_PG_Delete_AuditRow_ActorAttributionCorrect`, `TestEntityService_PG_Create_AuditRow_ServiceAccountAttributionCorrect` (exercises `def.Actor.ServiceAccountID` — the existing, canonical service-account representation; no separate service-account mutation path exists in the codebase to test instead), `TestEntityService_PG_Create_AuditRow_ActorAttributionDoesNotCrossTenants` (two real tenants), `TestEntityService_PG_CreateBatch_AuditRows_ActorAttributionCorrect`, `TestEntityService_PG_Create_OutboxActorAttribution_UnaffectedByAuditFix` (regression guard: outbox attribution must remain independent of this fix). All six actor-attribution tests were verified RED against the pre-fix `entity.go` (temporarily reverted via `git stash`, confirmed failing with exactly `actor_id IS NULL`, then restored) before being confirmed GREEN; the outbox regression guard correctly passed both before and after, proving it is testing something the fix must not disturb, not something the fix causes to pass.
- **ACCEPTANCE CRITERIA:** [x] `platform_audit_log.actor_id` correctly identifies the actor for Create/Update/Delete/CreateBatch. [x] `service_account_id` (not `actor_id`) is populated for a service-account actor, and never both. [x] Update's audit row attributes the actor who performed the update, not the original creator (and vice versa for Delete). [x] Tenant isolation holds for actor attribution across two real tenants. [x] Outbox lifecycle/workflow-intent actor attribution (Steps 3–6) is unchanged. [x] RED→GREEN verified, not merely asserted. [x] Full regression suite (`go test ./...`) remains green — 67 packages, 0 failures. [x] `-race` unsupported on this host (android/arm64); reported honestly, not faked.

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

## Findings-Register Follow-ups (R.x) — discovered building the showcase

Full evidence for each: `docs/00-overview/TECHNICAL_FINDINGS_AND_FUTURE_IMPROVEMENTS.md`.

### R.1 Most platform modules' migrations are not wired into `cmd/migrate`
- [ ] **GOAL:** Every platform module whose entity is meant to be real in production has its migration reachable by the actual migration runner, not just by test harnesses that read the `.sql` file directly.
- **WHY:** `cmd/migrate/main.go` blank-imports exactly 4 migration packages (`events/outbox`, `migration/bootstrap`, `audit`, `iam`, `tenant`). `organization`, `settings`, `flags`, `metadata`, `registry`, `notifications` (plural) all have real `.sql` files under `migrations/` with **no** `migrations.go` wrapper calling `migration.Register` — `cmd/migrate up` against a fresh database will never create their tables. `attachment`, `mail`, `notification` (singular) have no migration at all. Findings register §A.2.
- **FILES:** one new `migrations.go` per affected module (mirroring `platform/audit/migrations/migrations.go`'s exact pattern — `//go:embed *.sql` + `migration.Register`), plus the corresponding blank import added to `cmd/migrate/main.go`.
- **DEPENDENCIES:** blocks Phase 3's `OrganizationService` work from meaning anything against a real database — do this for `organization` first if nothing else.
- **TESTS:** apply the full migration set to a clean database via the real `cmd/migrate up` (or an equivalent test using `migration.BuildFS()`) and confirm each previously-orphaned table now exists.
- **ACCEPTANCE CRITERIA:** [ ] `organization` wired (priority: blocks Phase 3). [ ] `settings`, `flags`, `metadata`, `registry`, `notifications` (plural) each wired or explicitly deleted if genuinely dead. [ ] `attachment`/`mail`/`notification` (singular) get a real migration if they are meant to be functional, or are documented as scaffolding-only if not.

### R.2 Custom entities (`def.CustomDefinition`) have no backing table anywhere
- [ ] **GOAL:** `custom_entity_records` exists, with tenant RLS, wherever a `CustomDefinition` entity is actually used — or `CustomDefinition` is documented as not-yet-functional until this lands, so no module author is told to use it under false assumptions.
- **WHY:** `compiler/schema.go` routes every `CustomDefinition` to a shared `custom_entity_records` table (filtered by `entity_type`) that is never created by any migration or Go code anywhere in the repository. Findings register §A.3.
- **FILES:** new migration (likely `migration/bootstrap/` or its own small module) creating `custom_entity_records(id, tenant_id, entity_type, data jsonb, created_at, updated_at, custom_fields)` with the standard `tenant_isolation` RLS policy.
- **DEPENDENCIES:** none.
- **TESTS:** create/get/query a `CustomDefinition` entity against real PostgreSQL (none of the existing test suite does this against a real database today — confirm by grep before writing the fix, since this task's own research did not find one).
- **ACCEPTANCE CRITERIA:** [ ] Table exists, RLS enforced. [ ] A real-Postgres test creates and reads back a `CustomDefinition` entity. [ ] `docs/01-entity/ENTITY_DEFINITION_SPEC.md`'s description of `CustomDefinition` corrected if it currently implies this already works.

### R.3 Verify whether `iam_user`'s generic CRUD path is broken by its missing `custom_fields` column
- [ ] **GOAL:** Confirm or refute (currently a hypothesis, not a confirmed defect) whether `Get`/`Create` against `iam_user` via the generic `EntityService`/`contrib/pgx.Repository` path fails with `column iam_user.custom_fields does not exist`.
- **WHY:** `platform/iam/migrations/001_iam_user.up.sql` creates `iam_user` (the table the real `SystemDefinition.QualifiedName` resolves to) without a `custom_fields` column; `contrib/pgx.Repository.systemColumnsAndScanner` unconditionally selects one for every system entity. A second, differently-named migration (`20260720000001_create_iam_users.up.sql`, table `iam_users` — plural, with `custom_fields`) exists alongside it and is also embedded, but is not the table the entity definition actually targets. Findings register §A.6.
- **FILES:** none yet — this is a verification task first.
- **DEPENDENCIES:** none.
- **TESTS:** one focused real-Postgres test: apply `platform/iam`'s real migrations, call `EntityService.Create`/`.Get` against the `iam_user` entity (not `AuthService`'s own hand-written queries), observe the actual result.
- **ACCEPTANCE CRITERIA:** [ ] Hypothesis confirmed or refuted with a real test, not reasoning alone. [ ] If confirmed: add `custom_fields` to `iam_user`'s real migration (not the orphaned `iam_users` one) and delete or clearly mark the dead duplicate. [ ] If refuted: document why (e.g. `AuthService` never actually uses the generic path) so this doesn't get re-flagged without cause.

---

## Phase 3 — Organisation Hierarchy (make it real)

**UNBLOCKED as of Phase 2 Step 8.1 + Step 0 (path-representation reconciliation):**
item 1.18's `ScopeOrganization`/`ScopeOrganizationTree` RLS defect is closed and verified
(see 1.18 above — the org policy is now `AS RESTRICTIVE`, giving the intended
`tenant_isolation AND org_isolation` semantics, confirmed against real PostgreSQL with
adversarial sibling/hierarchy/cross-tenant tests). The companion path-representation
question this phase's own 3.1 acceptance criteria raised (`ltree` vs. VARCHAR+LIKE) is
also now resolved: repository inspection (Phase 3 Step 0) confirmed no `CREATE EXTENSION
ltree` exists anywhere in the migration set and no active ADR, generator code, or RLS
policy actually uses the `<@` ltree operator — the shipped `VARCHAR(4096)` materialized
path + `LIKE 'prefix%'` representation (already what `generator.go`'s
`current_org_path()`/`org_tree_isolation` policy and Step 8.1's fix implement) is retained
as authoritative. `ADR-012` (`docs/00-overview/DECISION_REGISTER.md`) and
`def/scope.go`'s `ScopeOrganizationTree` doc comment have been corrected to match. Phase 3
implementation (3.1) may now proceed.

### 3.1 Implement `OrganizationService`
- [ ] **GOAL:** Create/Move/ResolveScope actually work.
- **WHY:** AUDIT_REPORT.md §4, §9 S10 — every method returns `"not implemented"` today. **`PathComputeHook`'s own defect is now CLOSED — see 3.1 (status below) and Phase 3 Step 1's evidence; do not re-describe it as open.** Two new, smaller defects were found while verifying that fix and remain open: see **3.1a** and **3.1b** below. Also newly confirmed: `platform/organization`'s migration is not wired into the real `cmd/migrate` runner at all (see `docs/00-overview/TECHNICAL_FINDINGS_AND_FUTURE_IMPROVEMENTS.md` §A.2/R.1) — `OrganizationService` would be meaningless against a database that was never actually migrated to have `platform_organization`; fix R.1 before or alongside this item. `OrgAssignmentDefinition` also still declares an explicit `tenant_id` `FieldDef`, the same bug Phase 3 Step 1 fixed on `organization.Definition` itself — fix it the same way (remove the field) before implementing any `OrganizationService` method that creates an assignment record.
- **STATUS (Phase 3 Step 1, closed):** `PathComputeHook` now correctly computes path/depth from the real persisted parent via a real repository lookup, running `AfterCreate` (not `BeforeCreate` — the record's real ID does not exist until after persistence; see `platform/organization/hooks.go`'s doc comment for the full why). Verified against real PostgreSQL, non-superuser `awo_app` role, RED→GREEN. `organization.Definition`'s own `tenant_id` `FieldDef` was also removed (conflicted with `contrib/pgx.Repository.createSystem`'s unconditional column injection — a separate bug found while building the real-Postgres tests this fix needed).
- **FILES:** `platform/organization/service.go` (still stubbed — this item's actual remaining scope), `platform/organization/hooks.go` (`PathComputeHook` — done), `platform/organization/migrations/migrations.go` (does not exist yet — needed for R.1).
- **DEPENDENCIES:** R.1 (migration wiring) should land first or alongside — see above. 3.1a and 3.1b are independent, smaller, and can land separately.
- **IMPLEMENTATION:** Implement each `OrganizationService` method against `EntityRepository`.
- **TESTS:** create a 3-level org hierarchy (Tenant → Holding → Company A); confirm `path`/`depth` are correct at each level (this half is now proven — see Phase 3 Step 1's `platform/organization/path_compute_hook_pg_test.go`); confirm a Company-A-scoped user cannot see Holding-level records; confirm a manager sees their subtree.
- **ACCEPTANCE CRITERIA:** [x] `path` field decision made explicitly — VARCHAR(4096)+LIKE retained as authoritative (Phase 3 Step 0; no genuine `ltree` requirement found anywhere in the repository); `ADR-012` corrected to match (ADR-024 was unrelated — SessionValidator — and required no change). [x] `PathComputeHook` computes real path/depth from the real parent on the real call path (Phase 3 Step 1). [ ] `platform_organization`'s migration wired into `cmd/migrate` (R.1). [ ] All `OrganizationService` methods implemented, tested against real PG. [ ] Hierarchy creation/query integration tests pass (these were explicitly unchecked/pending in every prior status doc).

### 3.1a `platform/organization.MoveGuard` rejects every Update, not just path/depth writes (found during Phase 3 Step 1's verification)
- [ ] **GOAL:** `MoveGuard` only rejects an Update that actually tries to set `path`/`depth` directly; unrelated field updates succeed.
- **WHY:** `runtime/pipeline.go`'s actual call site passes hooks `(proposed, current)` — proposed/patched record first, prior/unpatched record second (verified from source, not assumed). `MoveGuard.BeforeUpdate(_, current, incoming *def.EntityRecord)` names its own parameters the other way around; Go binds positionally regardless of name, so `MoveGuard`'s `incoming` variable actually receives the **prior, unpatched** record — which, for any real persisted organization row, always has a non-null `path` key. The guard's check is therefore true for every update, not just attempts to set path/depth. Full writeup: `docs/00-overview/TECHNICAL_FINDINGS_AND_FUTURE_IMPROVEMENTS.md` §B.1.
- **FILES:** `platform/organization/hooks.go` (`MoveGuard.BeforeUpdate`).
- **DEPENDENCIES:** none. Should land before 3.1's `OrganizationService.Move`/`Disable`/`Enable` (all Update-based, all would be unconditionally blocked by this bug otherwise).
- **IMPLEMENTATION:** Swap the parameter names (or, more robustly, rename them to match what they actually are — `proposed, current` — so the mismatch can't recur) and check `proposed.Data["path"]`/`proposed.Data["depth"]` (the value the caller is trying to set), not the prior record's.
- **TESTS:** update an existing organization's unrelated field (e.g. `name`) → must succeed (currently fails). Attempt to set `path`/`depth` directly via Update → must still be rejected (regression guard).
- **ACCEPTANCE CRITERIA:** [ ] Unrelated-field update succeeds. [ ] Direct path/depth write still rejected. [ ] Verified against real PostgreSQL.

### 3.1b Organization-creation audit snapshot does not reflect `PathComputeHook`'s computed path/depth (found during Phase 3 Step 1's verification)
- [ ] **GOAL:** The `platform_audit_log` row for a `platform_organization` creation event records the real, computed `path`/`depth`, not the placeholder default.
- **WHY:** `EntityService.Create` (`api/service/entity.go`) calls `RunAuditRecord` (line 144) **before** `RunAfterCreate` (line 147) — generically, for every entity. `PathComputeHook` (an `AfterCreate` hook) mutates `record.Data["path"]`/`["depth"]` in place at line 147, after the audit row has already been marshaled and inserted. Full writeup, including the options considered and explicitly NOT decided: `docs/00-overview/TECHNICAL_FINDINGS_AND_FUTURE_IMPROVEMENTS.md` §B.2.
- **FILES:** `api/service/entity.go` (if the generic ordering changes — affects every entity, not just organization) or `platform/organization/hooks.go` (if a narrower, organization-specific fix is chosen instead).
- **DEPENDENCIES:** none, but **do not casually reorder `EntityService.Create`'s generic `RunAuditRecord`/`RunAfterCreate` sequence** without full-suite regression proof it doesn't change audit behavior for every other entity's `AfterCreate` hooks.
- **IMPLEMENTATION:** Not decided. Options from the findings register: (a) move `RunAuditRecord` after `RunAfterCreate` generically; (b) re-marshal the audit row's `after_data` if any `AfterCreate` hook mutated the record; (c) document as an accepted, narrow limitation and leave `EntityService.Create` unchanged. Get sign-off on which before implementing — this is exactly the kind of consequential change the engineering-discipline skill requires alternatives + trade-offs for, not a unilateral pick.
- **TESTS:** create an organization with a parent; query the actual persisted `platform_audit_log` row's `after_data`; assert it matches the real computed path/depth, not the placeholder.
- **ACCEPTANCE CRITERIA:** [ ] Decision made and documented (not silently implemented). [ ] Audit row matches persisted/returned path+depth for organization creation. [ ] No regression in any other entity's audit-row contents (full suite).

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
