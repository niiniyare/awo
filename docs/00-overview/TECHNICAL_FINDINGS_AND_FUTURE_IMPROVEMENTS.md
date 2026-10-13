# Technical Findings & Future Improvements

**Classification:** Living register — not a specification, not an ADR.
**Owner:** `00-overview/TECHNICAL_FINDINGS_AND_FUTURE_IMPROVEMENTS.md`
**Status:** Active. Update this file whenever delivery-focused work surfaces a
finding that would otherwise be lost. Do not duplicate `tasks.md` — link to
the relevant task ID instead of repeating its content here.

## Purpose

`tasks.md` is the execution roadmap: what to do next, in what order. This
file is the long-term memory: what we learned about how the framework
*actually* behaves, confirmed defects, non-blocking technical debt, product
ideas, and the conditions under which a deferred item should be revisited.
Keep these two documents connected (via task IDs) but separate in purpose.

Every claim below is sourced from direct repository inspection (file/line,
test output, or a command actually run) — see each entry's evidence. Entries
written from reasoning alone are labeled `(hypothesis, unverified)`.

## 2026-10-13 pass: CLI developer journey + live end-to-end execution

This pass's objective was to prove the Business Expense showcase through
Awo's own CLI and a real running server, not just through the test suite.
Doing so required actually running `awo migrate up`, `awo serve`, and real
`curl` calls against a real, freshly-`cmd/migrate`'d PostgreSQL database —
and that live execution surfaced several severe, previously-unknown defects
that no amount of source reading or isolated-schema testing had exposed,
because `testdb`'s per-test isolated schema (used by every `*_pg_test.go` in
this repo, including `examples/expense`'s own) installs its RLS/tenant
functions directly rather than through the real migration path, and the
existing `platform/iam` test suite's own fixture (see B.6 below) had quietly
drifted into testing a fictional schema that happened to be self-consistent
with the bug it should have caught. **Nothing here was found by reading code
harder — all of it required actually running the system.**

New findings this pass: B.4, B.5, B.6 (fixed), B.7 (confirmed, not fixed,
severe), and A.10. R.3 (previously a hypothesis) is now **confirmed**.

---

## A. Confirmed architectural and implementation findings

These are facts about current behavior, each independently verified this
pass (2026-10-13) by reading source and/or running real commands — not
inherited from a prior report without re-checking.

### A.1 One `def.Register` call drives five subsystems — genuinely true for System entities with a wired migration
Verified by building `examples/expense` end-to-end: a single `SystemDefinition` + `def.Register` in `init()` produced, with zero hand-written API/routing code, a working tenant-isolated CRUD API (`api/router/router.go`'s generic per-entity loop, `router.go:143-168`) and a generated CRUD surface, once (a) the entity's own migration was written and (b) that migration was registered with `migration.Register` and blank-imported into `cmd/migrate/main.go`. See `examples/expense/` and the passing `examples/expense/expense_pg_test.go`.

**Important qualifier, see A.2 and B.1 below: step (b) is the part that is routinely missing for existing platform entities.** "The Go definition exists and compiles" is not the same claim as "the table exists in a database migrated by the real tooling."

### A.2 Most platform modules' migrations are not wired into the real migration runner (`cmd/migrate`)
`cmd/migrate/main.go` blank-imports exactly 4 migration packages: `events/outbox/migrations`, `migration/bootstrap`, `platform/audit/migrations`, `platform/iam/migrations`, `platform/tenant/migrations`. Checked every `platform/*` directory for a `migrations/migrations.go` (the file that calls `migration.Register`, making a module's SQL reachable by `cmd/migrate`):

| Module | Has `migrations/*.sql`? | Has `migrations.go` (wired)? |
|---|---|---|
| `audit` | yes | **yes** |
| `iam` | yes | **yes** |
| `tenant` | yes | **yes** |
| `organization` | yes | **no — orphaned** |
| `settings` | yes | **no — orphaned** |
| `flags` | yes | **no — orphaned** |
| `metadata` | yes | **no — orphaned** |
| `registry` | yes | **no — orphaned** |
| `notifications` (plural) | yes | **no — orphaned** |
| `attachment` | no migration at all | — |
| `mail` | no migration at all | — |
| `notification` (singular) | no migration at all | — |

"Orphaned" means: the `.sql` files exist on disk, but `cmd/migrate up` against a fresh database will never apply them — they are reachable only by a test harness that reads the file directly (e.g. `testutil/db.ApplySQL` + `os.ReadFile`, the pattern this session's own Phase 3 Step 1 and Step 0 tests used) or by manual `psql`. This means **`platform_organization` itself — the table Phase 2 Step 8.1's RLS fix and Phase 3 Step 1's `PathComputeHook` fix both depend on — cannot currently be created by the actual migration tooling against a fresh database.** Every test proving those fixes correct used a hand-applied copy of the migration file, which is legitimate for unit/integration testing but does not establish that a real deployment would have the table at all. See `tasks.md` task **R.1**.

### A.3 Custom entities (`def.CustomDefinition`) are non-functional against a real database today
`compiler/schema.go:455-459`: every `CustomDefinition` entity's `TableName` is set to the literal string `"custom_entity_records"` — a single shared table, filtered by `entity_type`, for all custom entities tenant-wide. Confirmed via `grep -rln "custom_entity_records" --include="*.sql" .` (zero results) and `grep -rn "CREATE TABLE.*custom_entity_records"` across all `.go` files (zero results): **this table is never created anywhere** — not in a migration, not in a Go-code `CREATE TABLE` string. A `Create`/`Get`/`Query` call against any `CustomDefinition` entity will fail with `relation "custom_entity_records" does not exist` the first time it touches a real (non-mocked) database. This directly shaped the showcase-entity decision (§D) — a `CustomDefinition`, despite looking like the faster path architecturally, was not a viable choice until this is fixed. See `tasks.md` task **R.2**.

### A.4 `platform/organization.MoveGuard` unconditionally rejects every Update on `platform_organization` (confirmed defect, see B.1)
Full detail in §B.1. Summary: a parameter-naming mistake (not a type error — the code compiles and the interface is satisfied) means the hook inspects the pre-patch record instead of the proposed one, so its "reject only if path/depth is being set directly" check is always true for any update to any organization row.

### A.5 `PathComputeHook`'s audit snapshot does not reflect the computed path/depth (confirmed defect, carried over from the prior session's own verification)
Full detail in §B.2 — this is the item the task prompt explicitly asked to be evaluated here, not silently marked fixed. It is unresolved.

### A.6 IAM had two divergent, differently-named user tables — RESOLVED this pass (was R.3's hypothesis, now confirmed and fixed)
**Originally reported as a hypothesis; now confirmed by live execution and fixed.** `platform/iam/migrations/` contained both `001_iam_user.up.sql` (table `iam_user`, singular — the one `platform/iam/definition.go`'s real `SystemDefinition` actually targets) and 10 timestamp-prefixed files including `20260720000001_create_iam_users.up.sql` (table `iam_users`, plural). The 10 timestamp-prefixed files were **never actually applied by any real migration run** — see B.4 (the root cause: a silent filename-parsing limit in the migration engine). Confirmed empirically against a real, fully-`cmd/migrate`'d database: `\dt iam_users` → no such table; `\d iam_user` → real table, confirmed **lacking `custom_fields`** (part of the broader B.7 finding). Four of `platform/iam/queries.go`'s hand-written SQL statements (login credential lookup, API-token/service-account join, session recovery) referenced the plural names (`iam_users`, `iam_api_tokens`, `iam_service_accounts`, `iam_sessions`) that were never real — confirmed via a real login attempt against a real server returning `relation "iam_users" does not exist`-class failures. **Fixed**: `platform/iam/queries.go`'s 4 queries now reference the real singular table names. The 4 genuinely-missing, non-duplicate tables the real system needs (`iam_roles`, `iam_permissions`, `iam_role_permissions`, the system-role seed) were promoted from the dead timestamp-prefixed files into correctly-numbered `008`-`011` files continuing `platform/iam/migrations/`'s real `001`-`007` sequence; the 10 dead files (6 of which were confirmed duplicates of already-real tables) were deleted. See B.6 for the full fix writeup and B.5 for an operational gotcha hit while applying it.

### A.7 Generated CRUD + permission enforcement is real and already demonstrated, not just documented
`api/router/router.go:143-168`: every compiled entity gets `GET /`, `POST /`, `GET /:id`, `PATCH /:id`, `DELETE /:id`, with `authz.RequirePermission(opts.Authz, perm, <verb>)` wrapping every route when an evaluator is configured. This is the same generic mechanism `examples/expense` is served through — no entity-specific routing code was written for the showcase.

### A.8 Audit is mandatory-by-default and already verified for the generic mutation path
`ADR-023`/`def/entity.go`: `AllowAudit()` defaults to `true`; a module author must explicitly set `DisableAudit: true` to opt out (e.g. `platform/notification` does, for volume reasons — see its own doc comment). Actor attribution on the persisted `platform_audit_log` row was proven correct against real Postgres in the prior session's Phase 2 Step 7b (`api/service/entity_audit_actor_attribution_pg_test.go`). This session's showcase work deliberately did not re-prove this generic mechanism per-entity (see §D's acceptance-criteria table) — it is already "implemented and verified" at the framework level.

### A.9 SDUI page generation is schema-driven, not hand-coded per entity — not independently re-verified this pass
`ARCH_OVERVIEW.md` describes `WidgetTree -> amis JSON` generation from the compiled schema. `sdui/generator`'s existing test suite (part of the 68-package green run — see §F) covers this at the unit level. This pass did not add an SDUI-specific test for `examples/expense` — classify SDUI support for the showcase entity as **implemented but insufficiently verified for this specific entity** (the generic mechanism is tested; this entity's own generated page was not inspected). The live server (A.10) could not be kept running long enough, after the auth/RBAC fixes, to open `/ui` and visually inspect the generated Expense view — **UI visual inspection remains a genuine, disclosed gap**, not a completed verification.

### A.10 `awo migrate` is now a real, native CLI command — not a pointer to a second binary
Before this pass, `cmd/awo`'s `migrate up/down/version/status` subcommand (`cmd/awo/main.go`'s old `runMigrate`) did nothing but print `"run the dedicated migration binary: go run ./cmd/migrate <sub>"` — a real command the help text advertised but that performed no actual migration. Fixed in `cmd/awo/migrate.go`: it now drives the same `migration.Runner`/`migration.FS()` (`migration/engine.go`, `migration/migration.go`) the standalone `cmd/migrate` binary already used, reusing the existing public extension point rather than inventing a new one. **Verified by actually running it**: `DATABASE_URL=... go run ./cmd/awo migrate status` → `status: current version 25002, dirty: false`; `go run ./cmd/awo migrate up` → applied `examples/expense`'s migration for real (`migrate: up complete`, version jumped to 40001), confirmed via `psql \d business_expense` showing the real table, RLS policy, FK constraints, and trigger. This closes the single biggest gap in the CLI developer journey (§8 of the showcase milestone prompt: "Apply migrations to a development PostgreSQL database" via the CLI, not by leaving it for a separate command).

Caveat found while doing this: `cmd/awo/migrate.go`'s blank-import list of migration-registering packages **must be kept in sync by hand** with `cmd/migrate/main.go`'s own list — there is no single source of truth for "which modules' migrations does this binary know about." Both currently list the same 6 packages; a future module added to one and not the other would silently diverge. Not fixed this pass (would need a registry-of-registries or build-tag mechanism — likely overkill for the current number of modules); noted as a maintenance risk, not a defect.

---

## B. Known defects and correctness gaps

### B.1 `platform/organization.MoveGuard` blocks every Update, not just path/depth writes — **CONFIRMED, not fixed**

- **Observable behavior:** any call to `EntityService.Update` on a `platform_organization` record — including one that touches only, say, the `name` or `active` field — would be rejected with `"organization.MoveGuard: path is managed by OrganizationService.Move — do not set directly"`, regardless of whether `path`/`depth` are actually part of the patch.
- **Root cause (verified from source, not inferred):** `runtime/pipeline.go:253`, the actual call site, is `h.BeforeUpdate(pctx.Ctx, proposed, current)` — first argument is always the **proposed** (patched) record, second is always the **prior**, unpatched one (confirmed independently by reading `EntityService.Update`, `api/service/entity.go:167-180`: `current, _ := s.repo.Get(...)` then `s.pipeline.RunBeforeUpdate(pctx, current)`). `platform/organization/hooks.go:195`, `MoveGuard.BeforeUpdate(_, current, incoming *def.EntityRecord)`, names its *own* first parameter `current` and second `incoming` — but Go binds positionally, not by name: `MoveGuard`'s `current` variable actually receives the **proposed/patched** record, and its `incoming` variable actually receives the **prior/unpatched** one. The check `if _, ok := incoming.Data["path"]; ok` therefore inspects the **unpatched** record — which, for any real, already-persisted `platform_organization` row, always has a non-null `path` key — so the condition is true for every update, not just ones that try to set `path` directly.
- **Business/security/reliability impact:** Not a security issue (it fails closed, over-restrictively, never under-restrictively). It is a correctness/availability gap: once `OrganizationService.Move`/`Disable`/`Enable` are implemented (any of them, since all are Update-based per `platform/organization/service.go`'s documented interface), they would be unconditionally blocked by this bug, independent of their own correctness. It also blocks any legitimate, unrelated field edit to an organization record today.
- **Relevant source:** `platform/organization/hooks.go:195-203`; `runtime/pipeline.go:251-257`; `api/service/entity.go:167-183`.
- **Blocks the first showcase?** No — `examples/expense` does not use `platform/organization` or `MoveGuard`. It was discovered as a side effect of getting `StatusTransitionGuard`'s own parameter order correct (see `examples/expense/hooks.go`'s doc comment, and the dedicated regression test `TestExpense_StatusTransitionGuard_UnrelatedFieldUpdate_Unaffected`, which exists specifically to prove this showcase module did not repeat the mistake).
- **Recommended next action:** fix `MoveGuard`'s parameter naming/binding before implementing `OrganizationService.Move`/`Disable`/`Enable` (Phase 3 Step 2+) — tracked as `tasks.md` task **3.1a**. No regression test exists for `MoveGuard` today; one should be added alongside the fix (update an org record's unrelated field, before the fix: expect the bug's false rejection; after: expect success).

### B.2 Organization-creation audit snapshot does not reflect `PathComputeHook`'s computed path/depth — **CONFIRMED (carried over), NOT FIXED**

- **Observable behavior:** the `platform_audit_log` row for a `platform_organization` creation event records `path: "/"`, `depth: 0` (the field's placeholder default) in `after_data` — never the real, computed path/depth — even though the organization row itself, and the value `EntityService.Create` returns to its caller, are both correct.
- **Root cause (verified from source):** `api/service/entity.go`'s `Create` calls `s.pipeline.RunAuditRecord(txCtx, created, nil, created.Data)` **before** `s.pipeline.RunAfterCreate(txCtx, created)` (line 144 vs. 147). `RunAuditRecord` → `audit.PostgresWriter.Write` synchronously `json.Marshal`s `AfterData` and executes the `INSERT INTO platform_audit_log` before returning — before `PathComputeHook.AfterCreate` (which runs at line 147, inside the same transaction) has mutated `created.Data["path"]`/`["depth"]` in place. Since `created.Data` is a Go map (reference type), the hook's later mutation cannot retroactively change what was already marshaled and persisted.
- **Business/security/reliability impact:** an audit-fidelity gap. The record of *what happened* (the audit trail) does not match *what is actually true* for this one entity's creation event. This matters if auditing is ever relied on for compliance/forensics on organization-hierarchy changes. It does not affect the organization row's own correctness, RLS, or tenant isolation.
- **Relevant source/tests:** `api/service/entity.go:144` vs. `:147`; `audit/pg_writer.go`'s synchronous marshal-then-insert; `platform/organization/hooks.go`'s `PathComputeHook.AfterCreate`. No test asserts the actual persisted audit row's `path`/`depth` fields for this entity — the gap was identified by source trace during the prior session's own independent verification pass, not by a failing test.
- **Blocks the first showcase?** No — `examples/expense`'s own mutations do not involve a self-referential, hook-mutated field the way `PathComputeHook` does; its audit rows are expected to be accurate (not independently re-verified this pass — see A.8).
- **Recommended next action — do not fix casually:** per the generic `EntityService.Create` method's shared use by every entity, any fix must not regress audit accuracy for entities whose `AfterCreate` hooks do *not* mutate `record.Data` (the common case). Options, **not decided or implemented**: (a) move `RunAuditRecord` after `RunAfterCreate` for every entity (broad, needs full-suite regression proof it changes nothing else); (b) re-marshal/re-write the audit row's `after_data` if any `AfterCreate` hook mutated the record (narrower, more invasive to `RunAuditRecord`'s contract); (c) document this as an accepted, narrow limitation specific to self-referential path-computation hooks and leave `EntityService.Create` unchanged. Tracked as `tasks.md` task **3.1b**.

### B.3 `Repository.Aggregate`'s `GroupBy` never produces more than one group
Carried over from the pre-existing `tasks.md` 1.15 finding (re-confirmed present in source, not re-tested this pass): `Aggregate`'s row-scanning calls `rows.Next()` once and never loops. Zero production callers today. Not a showcase blocker.

### B.4 Migration engine silently drops any `.sql` file whose numeric prefix exceeds 999 — **CONFIRMED, fixed for `platform/iam`'s own files, engine itself not fixed**

- **Observable behavior:** a `.sql` migration file named with a timestamp-style prefix (e.g. `20260720000001_create_iam_users.up.sql`) is never applied by `cmd/migrate` or `awo migrate`, with **zero error, zero warning, zero log line** — it is simply as if the file did not exist.
- **Root cause (verified from source):** `migration/migration.go:324`, `parseSQLFilename`: `if err != nil || n < 1 || n > 999 { return }` — any filename whose leading numeric prefix parses to a number outside `[1, 999]` causes `parseSQLFilename` to return `ok=false`; its only caller, `readSteps` (`migration/migration.go:268-271`), does `if !ok { continue }` — silently skipping the file with no diagnostic of any kind.
- **How this was found:** not by reading code — by actually running `awo serve` against a real, fully-`cmd/migrate`'d database and observing `auth+RBAC disabled — IAM tables not found: relation "iam_role_permissions" does not exist`, then tracing why a table defined in a real `.sql` file on disk didn't exist in a real, successfully-migrated database.
- **Impact:** this is a generic migration-engine defect, not specific to `iam`. Any module (including a future one) whose migration author does not follow the exact `{001-999}_{description}.{up,down}.sql` convention loses that migration **silently** — the single worst failure mode for a schema-migration tool, since nothing signals the gap until a downstream feature (here: the entire RBAC subsystem) mysteriously doesn't work.
- **Fixed this pass (narrowly, for `platform/iam` only):** the 4 genuinely-needed, non-duplicate tables from the dropped files (`iam_roles`, `iam_permissions`, `iam_role_permissions`, system-role seed) were promoted into correctly-numbered `008`-`011` files; the 10 non-conforming files (6 of them confirmed stale duplicates of already-real tables — see A.6) were deleted.
- **NOT fixed:** the engine itself still silently drops any future non-conforming filename for any module. **Recommended fix** (not implemented — a deliberate, scoped engine change, not something to rush): make `readSteps` return an error (or at minimum `slog.Warn`) for any `.sql`-suffixed file it cannot parse, instead of silently `continue`-ing. Tracked as `tasks.md` **R.4**.

### B.5 Golang-migrate's monotonic version model cannot retroactively apply a migration numbered below the already-applied watermark

- **Observable behavior:** after promoting `platform/iam`'s 4 missing tables into versions 20008-20011 (see B.4) on a database that had **already** applied a later module's migration (business_expense, version 40001), running `awo migrate up` reported `migrations applied version=40001` — i.e. **did nothing** — because `golang-migrate`'s `Up()` only applies versions greater than the current one, and 20008-20011 < 40001.
- **Root cause:** `migration.BuildFSFrom`'s `globalVer := src.Priority*1000 + step.localNum` scheme assigns each module a fixed, disjoint 1000-version "slot" ordered by `Priority`. This works cleanly for a brand-new database (every module's slot is applied in Priority order the first time), but has no mechanism for inserting a new migration into an **already-passed** slot on a database that has since progressed past it — `golang-migrate`'s single monotonic version counter has no concept of "apply this specific missing step out of order."
- **Impact:** this is a real operational limitation of the current migration-engine design, not a one-off bug. It matters whenever a module's migration set is extended after other modules (with a higher `Priority`) have already been migrated on a long-lived database — exactly what happened here.
- **Workaround used this pass:** applied the 4 new `.sql` files' content directly via `psql -f`, bypassing `golang-migrate`'s version tracking for this one dev database. This is **not** a general solution and is not something the showcase or any documentation should present as a supported workflow — on a **fresh** database (the normal case this task's own instructions anticipate — "a configured development database"), `awo migrate up` applies every module in the correct order the first time with no such issue, which is why `examples/expense`'s own migration applied cleanly via the real CLI with zero workaround (see A.10).
- **Not fixed:** this is an architectural limitation of the Priority-slot + golang-migrate combination, not a quick patch. Tracked as `tasks.md` **R.5** for future consideration (e.g. a `migrate repair`/`migrate apply-missing` command, or moving to a per-migration-id tracking table instead of a single version counter).

### B.6 `platform/iam/queries.go` and its own test fixture both referenced table names that never existed in the real schema — **CONFIRMED and FIXED**

- **Observable behavior:** `POST /api/v1/auth/login` against a real, fully-migrated database (after B.4's RBAC tables were restored) failed with `relation "iam_users" does not exist`-class errors; once worked around by table-name correction, role-loading then failed with a different missing-table-class error until the full chain (queries.go + migrations) was consistent.
- **Root cause:** `platform/iam/queries.go` (the single file documented as centralizing "every raw SQL statement used by AuthService") had 4 queries written against the plural table names (`iam_users`, `iam_api_tokens`, `iam_service_accounts`, `iam_sessions`) that matched the **dead, never-applied** timestamp-prefixed migrations (B.4), not the real singular tables (`iam_user`, `iam_api_token`, `iam_service_account`, `iam_session`) that `platform/iam`'s real, conforming `001`-`007` migrations actually create. **More surprisingly**, `platform/iam/service_integration_test.go`'s own hand-rolled test fixture (a synthetic `compiler.EntitySchema` with `QualifiedName`/`TableName` literally set to `"iam_users"`/`"iam_sessions"`, plus matching `CREATE TABLE`/`INSERT`/`SELECT` statements) was **also** written against the same wrong, plural names — meaning the existing test suite was internally self-consistent with the bug and could never have caught it. Confirmed directly: reverting only the `queries.go` fix made 2 previously-failing-under-the-fix tests pass again (because the test's own fixture still used the old, wrong names) — proving the test fixture, not just the production code, had drifted from reality.
- **Fixed:** `platform/iam/queries.go`'s 4 SQL statements (and their doc comments) corrected to the real singular table names. `platform/iam/service_integration_test.go` and `platform/iam/handler_route_test.go`'s fixtures correspondingly corrected (`iam_users`→`iam_user`, `iam_sessions`→`iam_session`, mechanical word-boundary rename, no logic changes).
- **Verified:** `go test ./platform/iam/... -count=1 -v`: all tests pass (previously 6 failed: 2 in `handler_route_test.go`, 4 in `service_integration_test.go`, all now green). Full suite: `go test ./... -count=1`: 68 packages ok, 0 failed — no other package affected.
- **Business/security impact:** this was a **complete, unconditional failure of real-schema login** — not a corner case. Every login attempt against a correctly-migrated database would have failed, masked by generic "invalid credentials" error handling that doesn't distinguish "wrong password" from "the query referenced a table that doesn't exist." This is now fixed and tested.

### B.7 `contrib/pgx.Repository`'s generic scanner hardcoded `custom_fields` and `updated_at` as always-present — **CONFIRMED and FIXED, 2026-10-13 R.6 pass**

- **Observable behavior (before the fix):** `TenantResolver`'s own lookup of `platform_tenant` via the generic `EntityRepository` failed; `AuthService.Login`'s role-loading query against `iam_user_role` failed with `column iam_user_role.updated_at does not exist`. Both reproduced live against a real, fully-migrated database, and both reproduced as RED in a new regression test before the fix (see below).
- **Root cause (verified from source):** `contrib/pgx.Repository.systemColumnsAndScanner`/`createSystem`/`updateSystem` unconditionally referenced `tenant_id`, `updated_at`, and `custom_fields` in generated SQL for every system entity, with no check against what the entity's real table actually has. Three real, independent column-presence gaps existed:
  - **No `tenant_id`:** `platform_tenant` only — it IS the tenant; ScopeSystem by its own long-standing doc comment, but the `Scope` field was never actually set to `def.ScopeSystem` on the real definition (defaulted to `ScopeTenant`), and `contrib/pgx.Repository` never checked `Scope` at all — a second, compounding gap (zero entities anywhere declared `ScopeSystem` before this fix; the repository code path for it had never been exercised, ever).
  - **No `custom_fields`:** `platform_tenant` and **all 6** `platform/iam` system entities (`iam_user`, `iam_user_role`, `iam_service_account`, `iam_api_token`, `iam_session`, `iam_login_audit`) — none of their real, hand-written migrations have it; it is a convention that postdates them (every `awo/generator`-generated table, and `platform_organization`, `examples/expense`, have it).
  - **No `updated_at`:** `iam_user_role`, `iam_api_token`, `iam_session`, `iam_login_audit` — by explicit design (each migration's own comment: "immutable after creation" / "append-only").
- **Fix (smallest generic solution, chosen from the alternatives this register previously listed):** `def.EntityDefinition` gained two new interface methods, `HasUpdatedAt() bool` / `HasCustomFields() bool` (default `true` on `SystemDefinition`, overridable via new `NoUpdatedAt`/`NoCustomFields` struct fields; fixed `true`/`false` respectively on `CustomDefinition`, which has its own, different physical shape). `compiler.EntitySchema` gained matching `HasUpdatedAt`/`HasCustomFields` fields, propagated at compile time. `contrib/pgx.Repository`'s `systemColumnsAndScanner`/`createSystem`/`updateSystem` now consult `r.schema.HasUpdatedAt`, `r.schema.HasCustomFields`, and `r.schema.Scope != def.ScopeSystem` (for `tenant_id`) before referencing each column, instead of assuming all three unconditionally. A `CustomFields` payload supplied for a `NoCustomFields` entity is explicitly rejected with an error (not silently dropped — "do not conceal missing required fields... or silently discard data"). `platform/tenant.Definition` now declares `Scope: def.ScopeSystem, NoCustomFields: true`; all 6 `platform/iam` definitions declare the correct `NoUpdatedAt`/`NoCustomFields` combination for their real schema.
- **Why this is generic, not a special case:** every system entity that already has all three columns (the common case — `platform_organization`, `examples/expense`, every `awo/generator`-generated table) is completely unaffected — the new fields default to the old, unconditional behavior. No entity-specific branching exists anywhere in `contrib/pgx`; the 7 affected entities each make one explicit, documented declaration using a mechanism any future entity can use the same way.
- **Tests (real PostgreSQL, non-superuser `awo_app` role, real migrations read from disk):** `contrib/pgx/system_column_contract_test.go` — `TestSystemColumnContract_PlatformTenant_GetWorks_NoTenantIDColumn`, `TestSystemColumnContract_IAMUserRole_CreateAndGetWork_NoUpdatedAtOrCustomFields`, `TestSystemColumnContract_CreateWithCustomFields_RejectedExplicitly_NotSilentlyDropped`. **RED→GREEN verified**: `git stash`'d only `contrib/pgx/repo.go` (keeping the new schema/definition fields in place), reran the 3 new tests — 2 failed with exactly the predicted real SQL errors (`column platform_tenant.tenant_id does not exist`, `column iam_user.custom_fields does not exist`); restored, all 3 passed again.
- **Regression:** `go test ./contrib/pgx/... ./platform/iam/... ./platform/tenant/... ./platform/organization/... ./examples/...` and the full suite all pass — see §F.
- **Live-execution confirmation:** with this fix in place, `POST /api/v1/auth/login` against a real, fully-migrated database **succeeded for the first time this project's history** (see §A.11 and the showcase's own DEMO.md for the full, real curl transcript).

### B.8 Demo environment used Postgres superuser credentials — RLS was silently bypassed; NOT a framework defect, but a confirmed, severe operational gap with zero framework safeguard

- **Observable behavior:** running the live demo with `DATABASE_URL` pointed at the `admin` role (this Termux environment's own bootstrapped Postgres superuser — the same credentials `psql` had been using all session), tenant B could read and list tenant A's `business_expense` records in full, over the real HTTP API — an apparent, severe cross-tenant leak.
- **Root cause (verified via `pg_roles`):** `admin` has `rolsuper = true`. **PostgreSQL superusers bypass RLS unconditionally, even with `FORCE ROW LEVEL SECURITY`** — this is documented Postgres behavior, not an Awo bug. Re-ran the identical test after creating a real, `LOGIN`-capable, non-superuser role (`awo_demo`, `NOSUPERUSER NOBYPASSRLS`, granted only ordinary table privileges — no different from what any real production deployment's database credentials should be) and pointing the server at it instead: **tenant isolation then worked correctly** — tenant B got a real 404 for tenant A's record and an empty list; tenant A continued to see exactly her own 2 records. This proves Awo's RLS/tenant-isolation mechanism itself is correct; the leak was 100% caused by this session's own choice of database credentials for the live demo, not by framework code.
- **The real, confirmed gap:** `grep`'d every server entrypoint (`cmd/server`, `cmd/awo/serve_impl.go`, `bootstrap/`) for any check of the connecting role's privileges, or any `SET ROLE`/role-switching mechanism — **zero occurrences**. The application connects with whatever `DATABASE_URL` credentials it is given and trusts them completely, with **no safeguard, warning, or check** against an operator supplying superuser (or table-owner) credentials, which would silently and completely defeat RLS for the entire application with no indication anything is wrong. `testutil/db.SetupTestDB`'s own extensive doc comments already identify exactly this risk for the *test* harness ("PostgreSQL superusers bypass RLS even with FORCE ROW LEVEL SECURITY") and solve it there (dedicated restricted role + explicit `SET ROLE`) — but the equivalent safeguard does not exist anywhere in the real server startup path.
- **Classification:** not a showcase blocker once the demo's own credentials were corrected (environment-only change, zero framework code touched for the fix itself). A genuine, confirmed, severe **operational-safety gap**: nothing stops a real deployment from making the identical mistake, silently, in production. Tracked as `tasks.md` **R.7**.

### B.9 `submitted_by` (and likely every `FieldTypeLink` value) serializes as a raw byte array in JSON API responses, not a UUID string

- **Observable behavior:** every `business_expense` API response (Create/Get/List) renders `"submitted_by":[22,233,217,138,62,34,69,55,174,252,161,252,51,161,162,124]` instead of a proper UUID string.
- **Root cause (not fully traced this pass — flagged, not investigated in depth):** almost certainly the same root cause as B.7/A.6/A.2's recurring pattern this project keeps finding: `contrib/pgx`'s generic `any`-destination scan decodes a `uuid` column into Go's `[16]byte` representation (confirmed empirically for `parent_id` during the prior session's Phase 3 Step 1 work), and nothing in the JSON-response path converts that back into a canonical UUID string for Link-type fields specifically.
- **Impact:** cosmetic/usability, not a security or data-correctness issue — the underlying stored value is correct (confirmed: the FK constraint enforced correctly, and the same value round-trips consistently). But it is a real, visible defect in the generated API's response shape for any entity with a `FieldTypeLink` field, likely affecting every system entity in the framework, not just this showcase.
- **Not fixed this pass** — out of scope for R.6, and the task's own scope boundary (§7, §9) explicitly says not to add business-specific workarounds to the framework and not to expand into unrelated fixes. Tracked as `tasks.md` **R.8**.

### B.10 `cmd/awo serve`'s static UI/showcase file paths only resolve if the server's working directory happens to be the parent of a directory literally named `awo` — **CONFIRMED, not fixed**

- **Observable behavior:** running the server via the exact, previously-documented command (`go run ./cmd/awo serve ...` from the repository root — i.e. inside this checkout, whose own directory happens to be named `awo`) returns `404 sendfile: file ./awo/web/dist/index.html not found` for `GET /ui`, `GET /ui/*`, and `GET /showcase*`, and `Cannot GET /` (404) for `GET /`. Confirmed live, this pass, with a fresh `go run ./cmd/awo serve` from the repo root.
- **Root cause (verified from source):** `cmd/awo/serve_impl.go` hard-codes `app.Static("/", "./awo/web/dist", ...)`, `c.SendFile("./awo/web/dist/index.html")`, and `c.SendFile("./awo/web/showcase/index.html")` — paths relative to the **process's current working directory**, not to the repository root, the binary's location, or any configurable flag. `cmd/server/main.go` has the identical shape (`./awo/web/static`). No `awo/` subdirectory exists anywhere inside this repository — the real frontend lives at `web/` (repo-root-relative). These paths only resolve if the process is launched with its **working directory set to the parent of a directory literally named `awo`** — which happens to be exactly this repository's own checkout directory name, by coincidence, not by any documented convention. Confirmed experimentally: building the binary and launching it with `cwd` = the parent of this checkout (so `./awo/web/dist` resolves to this repo's real `web/dist`) makes `/`, `/ui`, `/ui/*`, and `/showcase*` all return real `200`s with real HTML.
- **Impact:** the documented startup procedure (`DEMO.md`'s own prior instructions, and this project's established pattern of running `go run ./cmd/awo serve` from inside the repo) **cannot ever serve the UI**, regardless of browser availability — a 404 occurs before a browser's JS engine would even get involved. This is a real, load-bearing defect in the CLI's own documented developer workflow, independent of and in addition to the no-browser-tooling finding (B.11).
- **Not fixed this pass** — fixing this (e.g. resolving the static directory relative to the executable's own path, a `--web-dir` flag, or embedding the built assets via `go:embed`) is a real design decision with trade-offs (packaging/distribution model for the compiled binary), and the task scope for this pass was verification, not an architectural change to the CLI's asset-serving model. Tracked as `tasks.md` **R.10**.

### B.11 The checked-in `web/dist` frontend is a static, hand-authored demo shell — it is not wired to Awo's real backend or its real SDUI endpoints for any entity, including Expense — **CONFIRMED, not fixed**

- **Observable behavior:** Even after working around B.10's path issue and getting `/ui` to serve real HTML, the rendered shell's sidebar navigation is a hard-coded JavaScript array listing entirely fictional modules and entities (`Finance`/`Chart of Accounts`, `Inventory`/`Items`, `HR`/`Employees`, `CRM`/`Customers`, `Forecourt`/`Sites`, plus a `Platform` group with `Users`/`Roles`/`Tenants`) — **none of which is `business_expense`**, and none of which matches this repository's real, live-registered 21 entities (confirmed via the real `/api/v1/ui/nav` endpoint, which correctly lists `business → Expenses → business_expense → /ui/business/expenses`, among the other real entities — the backend's SDUI generation is genuinely correct; see §B.7/§A.* and this pass's own live verification below).
- **Root cause (verified from the served HTML/JS source, `web/dist/index.html`):**
  - The page's router (`navigate()`) always fetches `/schemas/pages/<hash>.json` — a **static file path** under `web/dist/schemas/pages/`, never the real `/api/v1/ui/{module}/{entity}` endpoint, for any hash value. There is no code path, including manual hash navigation (e.g. `#business/expenses`), that reaches the live SDUI backend. Confirmed live: `GET /schemas/pages/business/expenses.json` → `404` (no such static file exists; `web/dist/schemas/pages/` only has hand-authored `finance/`, `inventory/`, `hr/`, `crm/`, `forecourt/`, `platform/` subfolders for the fictional modules above).
  - The login form's JS calls `fetch('/auth/login', ...)` — missing the real backend's actual `/api/v1` prefix. Confirmed live: `POST /auth/login` → `404`; the real route is `POST /api/v1/auth/login` (confirmed working, `200`, this pass). The "Create workspace" registration tab similarly calls `fetch('/auth/register', ...)`, a route that **does not exist anywhere in the real backend** (`platform/iam/handler.go` registers only `/login`, `/logout`, `/me` — no registration endpoint at all).
  - The AMIS data-fetcher sends a `X-Awo-Tenant` header; the real backend's middleware expects `X-Tenant-ID` (confirmed from `platform/iam/handler.go` and live use throughout this pass and the prior one).
- **Impact — more fundamental than "no browser was available":** even granting full browser access, the shell as currently checked in would show a sidebar with no Expense entry, could not actually log in against the real backend (wrong URL), and could not render any entity's real data (wrong schema source) — for Expense or any other entity. The backend's own SDUI JSON generation capability (nav, list, detail) is independently confirmed correct and working this pass (see §A.11/Runtime demonstration below) — the gap is entirely in this static frontend artifact's wiring, not in Awo's SDUI engine.
- **Not fixed this pass** — rewriting or rewiring `web/dist`'s JS to call the real backend is a real frontend-engineering task with design decisions of its own (keep the static-CDN/amis-embed approach vs. adopt the `vite build` pipeline the repo's own `vite.config.js`/`package.json` imply but which was never actually run — `web/dist/index.html` is byte-identical to `web/index.html`, confirming it was hand-copied, not built), and is well beyond this pass's verification scope. Tracked as `tasks.md` **R.11**.

---

## C. Architectural improvements and technical debt (verified status, not hypotheses, unless marked)

| Item | Status this pass | Evidence | Tracked as |
|---|---|---|---|
| Orphaned platform migrations (A.2) | **Confirmed** | `cmd/migrate/main.go`'s import list vs. `platform/*/migrations/` directory contents | R.1 |
| Custom entities non-functional (A.3) | **Confirmed** | grep for `custom_entity_records` CREATE TABLE, zero results | R.2 |
| IAM dual user-table (A.6) | **Confirmed the duplication; the "generic path breaks" consequence is a hypothesis** | file diff of the two migrations | R.3 |
| `OrgAssignmentDefinition` tenant-field compatibility | **Confirmed still present**, same shape as the bug Phase 3 Step 1 fixed on `organization.Definition` itself (declaring `tenant_id` as an explicit `FieldDef` conflicts with `contrib/pgx.Repository.createSystem`'s unconditional injection) — re-read `platform/organization/definition.go`'s `OrgAssignmentDefinition.Fields`, confirmed the same `tenant_id` field still present | tasks.md 3.1 (note added) |
| Audit snapshot consistency for self-mutating `AfterCreate` hooks (B.2) | **Confirmed, unresolved** | see B.2 | 3.1b |
| `MoveGuard` parameter-order bug (B.1) | **Confirmed, unresolved, newly discovered** | see B.1 | 3.1a |
| Dependency-injection / module registration (`module/` package unwired) | **Not re-verified this pass** — carried over from `tasks.md` 2.5, which already documents it with evidence | — | 2.5 (existing) |
| Finance module registration | **Confirmed absent by design**, not a gap — `docs/00-overview/ARCH_OVERVIEW.md` §10 lists exactly 7 framework-native modules, Finance is not one; `tasks.md` 0.2 already documents the evidence-based decision to keep it out of this repo | — | 0.2 (existing, closed) |
| Permission administration / Casbin policy reload | **Not re-verified this pass** — carried over from `tasks.md` 1.5 | — | 1.5 (existing) |
| `sqlbuild.Allowlist` zero production callers | **Not re-verified this pass** — carried over from `tasks.md` 1.13 | — | 1.13 (existing) |
| Audit querying/observability (`GET /:id/history` wiring) | **Not re-verified this pass** — carried over from `tasks.md` 2.2 | — | 2.2 (existing) |
| OpenAPI generator duplication | **Not re-verified this pass** — carried over from `tasks.md` 4.4 | — | 4.4 (existing) |
| Module registration convention (dual `db/migrations/` + per-module `migrations/`) | **Confirmed still coexisting**, and now more clearly understood: the per-module convention is the one that's broken/unwired for most modules (A.2), not merely "two conventions, both fine" as `tasks.md` 5.2 previously characterized it | tasks.md 5.2 (needs a note correcting its framing — not done this pass, flagged only) |
| Test infrastructure / integration coverage | Not expanded this pass beyond `examples/expense`'s own 5 tests | — | 9.1-9.5 (existing) |
| Documentation drift / stale task references | `tasks.md` 3.1's own "WHY" text still described `PathComputeHook`'s pre-fix behavior as current — corrected in this pass's `tasks.md` edit (see §E of that file's changelog) | — | — |

---

## D. Product opportunities and future capabilities

**Brainstorming below — none of this is approved architecture. Distinguish
from the Decision Register, which is binding.**

- Reusable "status lifecycle" helper: `examples/expense/hooks.go`'s `StatusTransitionGuard` is hand-written per entity. If a second business entity needs the same shape (a small, explicit state machine over a `select` field), consider a small `sdk` helper (`sdk.StatusGuard(transitions map[string][]string)`) that returns a ready `BeforeUpdateHook` — but only once a second real use case exists (per `awo-engineering-discipline`'s anti-overengineering discipline: don't abstract from one example).
- Reusable "approval chain" entity pattern (submitted-by / approved-by / approved-at) — `examples/expense` demonstrates the smallest version of this (a `submitted_by` link + a status field). A second module needing multi-step approval would be the trigger to extract a shared pattern, not before.
- Declarative validation beyond required/immutable/select-enum (e.g. cross-field rules, "amount must be positive") — not needed by the current showcase (Postgres `numeric` + required covers it); would matter for a second, more complex entity.
- SDUI layout customization for business entities (the showcase currently relies on the default, zero-config generated layout) — worth revisiting once a second entity needs a deliberately different view.
- Reporting on business entities (`report/` package exists, zero production callers per `tasks.md` 1.16) — natural next capability once more than one business entity exists to report across.
- Naming-series support for business entities (e.g. human-readable expense numbers like `EXP-2026-0001`) — `def.FieldTypeNamingSeries` already exists and is used elsewhere; not applied to `examples/expense` to keep this showcase minimal, worth adding if a demo audience expects it.

---

## E. Decisions and revisit conditions

| Deferred item | Revisit when |
|---|---|
| Orphaned platform migrations (A.2 / R.1) | Before any of `organization`/`settings`/`flags`/`metadata`/`registry`/`notifications`(plural) is exposed through a production API, or before Phase 3 Step 2 (`OrganizationService`) is implemented — `OrganizationService` would be meaningless against a database that was never actually migrated to have `platform_organization`. |
| Custom entities non-functional (A.3 / R.2) | Before any module author is told "use a `CustomDefinition` for a quick JSONB-backed entity" — that guidance is currently false. Must be fixed before it is ever recommended again, in documentation or in practice. |
| `MoveGuard` bug (B.1 / 3.1a) | Before implementing any part of `OrganizationService` that calls `Update` on a `platform_organization` record (`Move`, `Disable`, `Enable` — per `platform/organization/service.go`'s documented interface, all three would be silently broken otherwise). |
| Audit snapshot consistency (B.2 / 3.1b) | Before relying on `platform_organization`'s audit history for compliance/forensics, or before implementing a second entity with a self-referential, `AfterCreate`-mutating hook (which would hit the identical gap). |
| IAM dual-table hypothesis (A.6 / R.3) | Before trusting `iam_user`'s generic `EntityService`/`contrib/pgx.Repository` CRUD path in any new work — confirm or refute with one focused test before building on it. |
| Reusable status-guard / approval-chain helpers (§D) | When a **second** business entity needs the same shape — not before (avoid premature abstraction from a single example, per the engineering-discipline skill). |
| `module/` package wiring (`tasks.md` 2.5) | Before a third platform/business module registration pattern is added by hand — if the manual blank-import list keeps growing, the cost of wiring `module/` properly starts to pay for itself. |

---

## F. This pass's own verification evidence

### 2026-09 pass (showcase entity creation)
- `go test ./examples/... -v`: 5/5 new tests pass (real PostgreSQL, non-superuser `awo_app` role, real `examples/expense` migration file, real `EntityService`/`contrib/pgx.Repository` pipeline).
- `go test ./... -count=1`: 68 packages `ok`, 0 `FAIL` (up from 67 before this pass's addition of `examples/expense`).
- `gofmt -l .`: clean. `go vet ./...`: clean. `go build ./...`: clean. `git diff --check`: clean.
- `-race`: unsupported on this host (android/arm64); CI's dedicated `linux/amd64` race job remains authoritative, consistent with every prior pass in this project's history.

### 2026-10-13 pass (CLI developer journey + live execution)
- `go run ./cmd/awo migrate status` / `migrate up` against a real PostgreSQL database: **actually run**, actually applied `examples/expense`'s migration (version 25002 → 40001), confirmed via `psql \d business_expense` (real table, RLS policy, FK constraints, trigger — full output captured in-session).
- `go run ./cmd/awo serve --db ... --redis ...`: **actually started**, served real traffic; `/health/ready`, `/api/v1/showcase/info`, `/api/v1/showcase/entities` all hit via real `curl` with real responses.
- Live `curl` CRUD attempt against `/api/v1/business/expenses` surfaced B.4/B.7 (real panics/errors, captured verbatim from server logs) rather than completing the full authenticated flow — **the live authenticated end-to-end curl demonstration (login → bearer token → CRUD) remains blocked by B.7**, disclosed honestly rather than faked or skipped silently.
- `go test ./platform/iam/... -count=1 -v`: all tests pass after the B.6 fix (previously 6 failed under the production fix alone, due to the test fixture's own drift — now both are corrected and consistent).
- `go test ./... -count=1` (after all this pass's changes): **68 packages `ok`, 0 `FAIL`** — identical package count to the prior pass; no new package added, none broken.
- `gofmt -l .`: clean. `go vet ./...`: clean. `go build ./...`: clean. `git diff --check`: clean.
- `-race`: unsupported on this host (android/arm64); not run.
