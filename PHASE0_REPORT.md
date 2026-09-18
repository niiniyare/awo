# Phase 0 Completion Report — Repository Bootstrap / Build Integrity

**Scope:** `tasks.md` Phase 0 only. No later-phase work was performed.
**Executed:** this session, against the repository at commit `d8191d0` ("audited"), producing changes now present at commit `934a867` ("audited") — see §17 on commit timing, which was outside this agent's control.

---

## 1. Repository state before Phase 0

- No committed `go.mod`/`go.sum` — the repository could not build from a clean checkout.
- `cmd/awo/cmds_schema.go` and `cmd/server/main.go` both blank-imported `awo.so/modules/finance`, a package absent from this repository, making both binaries fail to compile even once a `go.mod` existed.
- 19 `.go` files were not `gofmt`-formatted.
- No CI configuration existed anywhere in the repository.
- No root `README.md` existed.
- Three non-project artifacts were present and already committed from the prior audit session: `dump.rdb` (a Redis RDB snapshot), `docs/links_check.txt` (an audit-tooling scratch file), and `index.html` (a wholly unrelated "Printable Price Tags" barcode/XLSX tool with no connection to Awo, SDUI, or ERP).
- `tasks.md` Phase 0 items were unchecked, written as an audit-time plan, not yet executed.

## 2. Canonical module decision

**`module awo.so/awo`** — confirmed, not assumed.

Evidence: every one of the ~365 current `.go` files in this repository imports itself and its siblings exclusively as `awo.so/awo/<path>` (e.g. `awo.so/awo/def`, `awo.so/awo/platform/iam`). A full-repository search for the module paths named in the task brief (`github.com/niiniyare/erp`, `github.com/niiniyare/ruun`) returned zero matches anywhere in current source *or* git history. The only historical module path this repository's git history ever contained was `module github.com/niiniyare/awo`, last seen in the commit immediately before a "deleted" wipe — and that code was an unrelated, much older gRPC/goa-based aircraft-booking demo (`aircraft.sql`, `airport`, protobuf/grpc-gateway dependencies), not this ERP framework. There is no live conflict to resolve: `awo.so/awo` is the sole convention in use.

## 3. Finance module decision

**Option (a): removed** `_ "awo.so/modules/finance"` from both `cmd/awo/cmds_schema.go` and `cmd/server/main.go`. Finance was not brought back into this repository.

Reasoning, evidence-based (full detail in `tasks.md` Phase 0.2 and `README.md` "Finance module status"):
- `docs/00-overview/ARCH_OVERVIEW.md` §10 "Platform Modules" lists exactly seven framework-native modules (Tenant, IAM, Feature Flags, Settings, Audit, Metadata, Module Registry). Finance is not among them.
- `docs/99-modules/FINANCE_MODULE_SPEC.md` self-describes Finance as "the canonical **reference** module" — an example for module authors, not a kernel component.
- `docs/99-modules/MODULE_AUTHOR_GUIDE.md` documents business modules living under `internal/core/{module}/`, a Go `internal/` package — importable only from within this same module tree by Go's own visibility rules. This independently confirms the intended design was never "Finance lives in a separate repository that this framework repository somehow still needs at compile time" — the correct shape is the reverse: an *application* repository depends on `awo.so/awo` and supplies its own business modules.
- This file's own stated original scope line (preserved in git history) was "Transform AWO into a clean, reusable, production-grade Go framework **extractable from the ERP repository**" — i.e. the separation this repository now embodies was the goal, not an accident to be reversed.
- No framework core package (`def`, `compiler`, `runtime`, etc.) references Finance anywhere; only the two `cmd/` blank-imports did, purely for entity-registration side effects.

This was not a mechanical "whatever makes the compiler happy" choice — option (b) (bringing Finance's source back into this repo) was considered and rejected on the documentary evidence above.

## 4. Files changed

- `cmd/awo/cmds_schema.go`, `cmd/server/main.go` — Finance import removed, replaced with an explanatory comment.
- `tasks.md` — Phase 0 checkboxes marked with evidence; new item **1.1a** added (see §16).
- `.gitignore` — hardened (see §9).
- 19 files reformatted by `gofmt -w .` (whitespace/formatting only, zero semantic change, verified by re-running `go build`/`go vet`/`go test` after): `api/meta/handler.go`, `auth/auth_test.go`, `auth/viewer_test.go`, `cmd/awo/cmds_schema.go`, `cmd/awo/main.go`, `contrib/pgx/bulk_test.go`, `contrib/pgx/repository_test.go`, `contrib/pgx/sqlbuild/allowlist.go`, `contrib/redis/session_store_test.go`, `generator/openapi/openapi.go`, `ioport/ioport_test.go`, `platform/audit/audit_integration_test.go`, `platform/mail/definition.go`, `platform/notification/definition.go`, `runtime/action_context.go`, `runtime/action_context_test.go`, `runtime/pipeline_test.go`, `sdui/adapt/adapt_test.go`, `sdui/amis/renderer.go`.

## 5. Files removed

- `dump.rdb` — Redis runtime snapshot, not project content.
- `docs/links_check.txt` — leftover audit-tooling scratch output.
- `index.html` — unrelated third-party HTML tool (barcode/price-tag generator), no connection to this project.

## 6. Files added

- `go.mod`, `go.sum` — carried forward from the prior audit session's diagnostic build (content re-verified this session; `go mod tidy` after the Finance-import removal produced a **byte-identical** `go.mod`, confirming it was already correct).
- `README.md` — accurate bootstrap/build/test/prerequisites/Finance-status documentation, verified against actual source behavior (not copied from `docs/`, which was independently found to contain stale env-var/package-path claims — see §9).
- `.github/workflows/ci.yml` — CI pipeline (see §8).
- `PHASE0_REPORT.md` — this report.

## 7. Dependency changes

**None.** `go mod tidy`, run again after removing the Finance import, produced a byte-for-byte identical `go.mod` to the one already committed — the Finance package's own (never-resolvable) dependencies never fed into this module's graph in the first place.

Full dependency graph reviewed by hand against the task brief's specific callouts:
- **Temporal** (`go.temporal.io/sdk`) — one direct dependency, confined as expected.
- **Redis** (`github.com/go-redis/redis/v8`, plus `github.com/alicebob/miniredis/v2` for tests) — as expected.
- **PostgreSQL/pgx** (`github.com/jackc/pgx/v5`) — as expected, **except**: `github.com/jackc/pgconn` v1.14.3 (the *legacy*, pxg-v4-era standalone package) is also a **direct** dependency, pulled in solely by `internal/dberr/dberr.go`. This is a genuinely surprising/likely-accidental dependency — see §16.
- **Fiber** (`github.com/gofiber/fiber/v2`, `github.com/valyala/fasthttp`) — as expected.
- **Casbin** (`github.com/casbin/casbin/v2`) — as expected, confined.
- **UI/AMIS** — no direct Go dependency (AMIS is a frontend asset, pinned separately per `docs/00-overview/ARCH_OVERVIEW.md`, not a Go module concern).
- **Testing** (`github.com/stretchr/testify`, `github.com/alicebob/miniredis/v2`) — as expected.
- **Code-generation/CLI** (`github.com/spf13/viper`, `github.com/golang-migrate/migrate/v4`, `github.com/fsnotify/fsnotify`) — as expected.
- **Observability** — seven separate `go.opentelemetry.io/otel/*` direct requires, including three simultaneous exporter backends (OTLP-gRPC, OTLP-HTTP, and stdout). Plausibly intentional (deployer picks an exporter at runtime), but flagged as worth a deliberate confirmation later — not changed in Phase 0.
- `github.com/robfig/cron` is pinned at **v1**, not v3 — already independently identified in the prior audit as the root cause of the `Scheduler.Cancel()` bug tracked in `tasks.md` Phase 4.3. Not touched here (would be a dependency refactor, out of Phase 0 scope).

**No dependency was added, removed, or upgraded in this pass**, and nothing was introduced solely for audit-tooling convenience — the entire graph is fully determined by genuine framework source code.

## 8. CI changes

Added `.github/workflows/ci.yml` with three jobs:
1. `fmt-vet-build` — `gofmt -l .` (must be empty), `go vet ./...`, `go build ./...`, and a `go mod tidy` idempotency check (fails the build if tidying would change `go.mod`/`go.sum`).
2. `test` — `go test ./... -count=1 -coverprofile=coverage.out` against real PostgreSQL 16 and Redis 7 **service containers** (not mocks), with coverage uploaded as a build artifact and printed (not gated — current 51.5% is far below the 90% target tracked in `tasks.md` Phase 9; gating now would only produce noise).
3. `race` — `go test -race ./... -count=1` on `ubuntu-latest` (linux/amd64), the same Postgres/Redis service containers, since `-race` cannot run on this development sandbox's android/arm64 host.

**This workflow has not yet been exercised by an actual GitHub Actions run** — no push was made to a remote in this session. Verify it on first push; do not assume it is correct purely because it parses and reads sensibly.

## 9. Documentation changes

Added `README.md` only. Verified its claims directly against source rather than trusting existing docs — e.g. `docs/20-devops/ENVIRONMENT_VARIABLES.md` claims `JWT_SECRET`/`SESSION_SECRET` are required-at-startup and references a nonexistent `internal/config/config.go`; the real `bootstrap.Run()`/`config/manager.go` only hard-require `DATABASE_URL` and `REDIS_URL` at the bootstrap layer, and env-var binding is `DATABASE_URL`, `REDIS_URL`, `PORT`, `APP_NAME` (via viper), plus directly-read `TEMPORAL_HOST`, `AWO_MIGRATION_DIR`, `LOG_LEVEL`. `README.md` reflects the verified reality, not the stale doc.

Did **not** perform the full documentation reconciliation tracked as `tasks.md` item **0.4** (merging the two colliding ADR sequences, updating `ARCH_OVERVIEW.md`/`PACKAGE_DEPENDENCY_MAP.md` to list the current ~40-package tree, fixing dead links, resolving the missing `ARCH_FREEZE_REVIEW.md`/`CLAUDE.md` references, deduplicating `ACTOR_MODEL.md`/`ACTOR_SPEC.md` and `SESSION_MODEL.md`/`SESSION_SPEC.md`). That item remains open by design — it's explicitly out of Phase 0's scope per the task brief ("Phase 0 is not the full documentation rewrite"), and several of its sub-items are themselves gated on Phase 1.1 landing first.

**New, previously-unrecorded finding surfaced while reading `docs/99-modules/ONBOARDING_CHECKLIST.md`** (inside `docs/`) during this investigation: its closing line says *"Do not infer from Framework A docs (`docs/` directory) — they describe a different framework"* — directly contradicting `docs_legacy_framework_a/README.md`'s own banner, which says `docs_legacy_framework_a/` is deprecated "Framework A" and `docs/` is the current, correct set. This is a genuine, self-referential contradiction about which directory is authoritative, most likely a copy-paste leftover from an earlier doc-generation pass that was never fully adapted. Logged here rather than silently fixed (Phase 0 is not a documentation-rewrite phase); worth folding into the 0.4 cleanup.

## 10. Exact commands executed

```
go mod init awo.so/awo          # (prior session; re-verified, not re-run)
go mod tidy                     # re-run after Finance-import removal — zero diff
gofmt -l .                      # found 19 files
gofmt -w .                      # fixed them
go vet ./...
go build ./...
go test ./... -count=1          # full suite, real PostgreSQL 18 (local) + Redis 7 (local)
go test -race ./def/...         # confirm race-detector platform support
git rm --cached dump.rdb docs/links_check.txt index.html && rm -f <same>
git diff --check
git status --short
```

## 11. Exact test/build/vet results

```
gofmt -l .        → (no output — clean)
go vet ./...      → (no output — clean, exit 0)
go build ./...    → (no output — clean, exit 0)
go test ./... -count=1
  → EXIT:0
  → 66 packages report "ok", 0 "FAIL", remainder "? … [no test files]"
  → 101 total lines of output (stable across two independent full runs this session)
```

## 12. Race-test result

```
$ go test -race ./def/...
-race is not supported on android/arm64
(exit code 2)
```
Confirmed, not assumed — this is the Go toolchain's own message, re-verified independently this session (matches the prior audit's finding exactly). `-race` is unsupported on this platform because the Go distribution does not ship a TSAN runtime for android/arm64. CI's `race` job runs on `ubuntu-latest` (linux/amd64), which does have race-detector support, and must be the source of truth for race results going forward — this sandbox never can be.

## 13. PostgreSQL integration result

**PASS.** Against a real local PostgreSQL 18 instance with a non-superuser `awo_app` role and `FORCE ROW LEVEL SECURITY` (created by the test suite itself, per `testutil/db/db.go`):
```
ok  awo.so/awo/contrib/pgx            4.517s
ok  awo.so/awo/contrib/pgx/sqlbuild   0.035s
ok  awo.so/awo/platform/audit         1.843s
ok  awo.so/awo/platform/iam           4.991s
ok  awo.so/awo/testutil/db            0.417s
```
No test in this run was skipped for lack of `TEST_DATABASE_URL` — it was set throughout.

## 14. Redis integration result

**PASS.**
```
ok  awo.so/awo/contrib/redis   0.102s
```
Plus `platform/iam`'s session-store integration tests (Redis + PostgreSQL together), also green (see §13).

## 15. Remaining warnings

None from `go vet`, `go build`, or `gofmt`. Test suite is fully green. Statement coverage was not re-measured in this pass (not a Phase 0 acceptance criterion); the prior audit's figure of 51.5% stands until `tasks.md` Phase 9 addresses it.

## 16. Risks discovered

1. **`internal/dberr/dberr.go` likely dead code.** It imports `github.com/jackc/pgconn` (pgx v4-era) instead of `github.com/jackc/pgx/v5/pgconn`. Since these are distinct types from distinct modules, `errors.As(err, &pgconn.PgError{})` can never match a real pgx/v5 error, meaning `dberr.Parse`/`dberr.IsTransient` — which already contain ready-made translation logic for `P0001`/`P0002` (tenant-not-found/tenant-not-active), directly relevant to `tasks.md` Phase 1.1 — currently never fire. Not fixed here (Phase 0 must not refactor dependencies); tracked as new item **`tasks.md` 1.1a**.
2. **This development sandbox is itself flaky in a way unrelated to the codebase.** During this exact Phase 0 execution, three separate full-suite test runs were corrupted or interrupted mid-flight by the sandbox's own session/process lifecycle (PostgreSQL being killed between tool calls; `go test`'s parallel linker temp-directories being torn down mid-link, producing spurious `cannot open output file: No such file or directory` errors for `tx`, `version`, `workflow`, and others). The final, trustworthy run (§11) was obtained by running synchronously and in the background to avoid this. **This is an artifact of the interactive sandbox, not of the repository** — a real CI runner (§8) will not exhibit this pattern, since it isn't subject to the same interactive session churn. Documented here so a future reader doesn't mistake earlier corrupted local output (visible in shell history) for a real regression.
3. **`docs/99-modules/ONBOARDING_CHECKLIST.md` self-contradicts `docs_legacy_framework_a/README.md` about which directory is Framework A** — see §9. Low risk today (doesn't affect running code) but actively confusing for a new contributor following the onboarding path.

## 17. Items intentionally deferred to later phases

- `tasks.md` **0.4** (full documentation reconciliation) — remainder beyond the new `README.md`.
- `tasks.md` **1.1a** (the `dberr`/`pgconn` import fix) — newly discovered this session, explicitly not fixed per the "don't refactor dependencies in Phase 0" instruction.
- CI's first real GitHub Actions run — the workflow is authored and committed but unexercised; verify on first push.
- Coverage re-measurement, and everything else in `tasks.md` Phase 1 onward.
- **Note on process, not scope:** this agent did not create any git commit, per the explicit instruction not to. However, the repository owner committed the working-tree state produced during this session directly (commit `934a867`, "audited") partway through this Phase 0 execution, outside this agent's control. All Phase 0 acceptance-criteria verification in this report was re-run *after* that commit, against the committed state, so the results above describe what is actually on disk and in git history now — not a pre-commit snapshot.

## 18. Phase 0 acceptance criteria — PASS/FAIL

| # | Criterion | Result |
|---|---|---|
| 1 | A canonical Go module path has been established from repository evidence | **PASS** |
| 2 | A real committed-ready `go.mod` exists | **PASS** |
| 3 | `go.sum` is correct and reproducible | **PASS** (`go mod tidy` → zero diff) |
| 4 | No temporary `go.mod` workaround remains | **PASS** |
| 5 | All historical/conflicting module paths have been investigated | **PASS** |
| 6 | The `modules/finance` dependency has a documented architectural resolution | **PASS** |
| 7 | The repository builds from a clean checkout without audit-only modifications | **PASS** |
| 8 | `go build ./...` passes | **PASS** |
| 9 | `go vet ./...` passes | **PASS** |
| 10 | `go test ./...` passes | **PASS** (66/66 packages, 0 failures) |
| 11 | Real PostgreSQL integration tests pass where required | **PASS** |
| 12 | Real Redis integration tests pass where required | **PASS** |
| 13 | Race testing has either passed or been proven impossible, and is covered by CI | **PASS** (proven impossible locally; CI targets a supported architecture) |
| 14 | CI verifies build/test/vet/integration/race appropriately | **PARTIAL** — workflow authored and committed, but not yet exercised by a real run; do not treat as fully proven until the first push confirms it |
| 15 | No investigation artifacts remain without justification | **PASS** |
| 16 | `.gitignore` appropriately protects generated/local artifacts | **PASS** |
| 17 | README contains accurate bootstrap/build/test instructions | **PASS** |
| 18 | No later-phase functionality has been implemented accidentally | **PASS** |
| 19 | `git diff --check` passes | **PASS** |
| 20 | `tasks.md` accurately records Phase 0 completion status | **PASS** |

**18 of 19 line items fully PASS; 1 (CI) is PARTIAL pending a real workflow run.**

## 19. Final recommendation

**The repository is safe to proceed to Phase 1.** The build-integrity foundation Phase 0 exists to establish is genuinely in place: a real, evidence-based module identity; a documented, non-arbitrary resolution of the Finance-import blocker; a clean `go build`/`go vet`/`gofmt`/`go test` baseline verified twice independently against real PostgreSQL and Redis; and a CI pipeline that will keep this baseline honest going forward once its first run is confirmed.

The one open item (CI's first real run) is low-risk and fast to close — push to a branch and watch it run before treating "CI is green" as an established fact rather than a well-founded expectation. It does not need to block starting Phase 1 work in parallel.
