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

### A.6 IAM has two divergent, differently-named user tables, only one of which the framework actually reads/writes
`platform/iam/migrations/` contains both `001_iam_user.up.sql` (table `iam_user`, singular, **no** `custom_fields` column) and `20260720000001_create_iam_users.up.sql` (table `iam_users`, plural, **has** `custom_fields`). `platform/iam/migrations.go`'s `//go:embed *.sql` embeds both — both tables would be created by a real `cmd/migrate up`. `platform/iam/definition.go`'s `SystemDefinition{Module:"iam", Name:"user"}` compiles to `QualifiedName = "iam_user"` (singular) — the table the generic `contrib/pgx.Repository`'s scanner actually targets, and that table **lacks `custom_fields`**, which `contrib/pgx.Repository.systemColumnsAndScanner` unconditionally selects for every system entity (confirmed: this exact column-missing error is what examples/expense's own migration hit before `custom_fields` was added to it — see A.1's evidence). **This means a `Get`/`Create` against `iam_user` via the generic `EntityService`/`contrib/pgx.Repository` path would very likely fail with the identical `column iam_user.custom_fields does not exist` error — `(hypothesis, not yet executed against a real database in this session)`.** `platform/iam/service.go`'s `AuthService` appears to use its own repository/query methods for login, which may or may not go through the same generic scanner — not traced in this pass. **Needs a dedicated, short verification pass before being treated as confirmed.** See `tasks.md` task **R.3**.

### A.7 Generated CRUD + permission enforcement is real and already demonstrated, not just documented
`api/router/router.go:143-168`: every compiled entity gets `GET /`, `POST /`, `GET /:id`, `PATCH /:id`, `DELETE /:id`, with `authz.RequirePermission(opts.Authz, perm, <verb>)` wrapping every route when an evaluator is configured. This is the same generic mechanism `examples/expense` is served through — no entity-specific routing code was written for the showcase.

### A.8 Audit is mandatory-by-default and already verified for the generic mutation path
`ADR-023`/`def/entity.go`: `AllowAudit()` defaults to `true`; a module author must explicitly set `DisableAudit: true` to opt out (e.g. `platform/notification` does, for volume reasons — see its own doc comment). Actor attribution on the persisted `platform_audit_log` row was proven correct against real Postgres in the prior session's Phase 2 Step 7b (`api/service/entity_audit_actor_attribution_pg_test.go`). This session's showcase work deliberately did not re-prove this generic mechanism per-entity (see §D's acceptance-criteria table) — it is already "implemented and verified" at the framework level.

### A.9 SDUI page generation is schema-driven, not hand-coded per entity — not independently re-verified this pass
`ARCH_OVERVIEW.md` describes `WidgetTree -> amis JSON` generation from the compiled schema. `sdui/generator`'s existing test suite (part of the 68-package green run — see §F) covers this at the unit level. This pass did not add an SDUI-specific test for `examples/expense` — classify SDUI support for the showcase entity as **implemented but insufficiently verified for this specific entity** (the generic mechanism is tested; this entity's own generated page was not inspected).

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

- `go test ./examples/... -v`: 5/5 new tests pass (real PostgreSQL, non-superuser `awo_app` role, real `examples/expense` migration file, real `EntityService`/`contrib/pgx.Repository` pipeline).
- `go test ./... -count=1`: 68 packages `ok`, 0 `FAIL` (up from 67 before this pass's addition of `examples/expense`).
- `gofmt -l .`: clean. `go vet ./...`: clean. `go build ./...`: clean. `git diff --check`: clean.
- `-race`: unsupported on this host (android/arm64); CI's dedicated `linux/amd64` race job remains authoritative, consistent with every prior pass in this project's history.
