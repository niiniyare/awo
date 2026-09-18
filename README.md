# Awo Framework

Awo is a Go-native framework for building multi-tenant ERP systems. This repository is the **standalone framework** — extracted from the original monorepo — and contains only the reusable kernel (`def`, `compiler`, `runtime`, `filter`, `driver`, the `platform/*` framework-native modules, SDUI, API layer, generators, CLI). It does **not** contain any business/application module (Finance, Inventory, etc.) — see [Finance module status](#finance-module-status) below.

For the full architecture, read [`docs/README.md`](docs/README.md) (the canonical, "frozen at v1.0" specification set) and [`AUDIT_REPORT.md`](AUDIT_REPORT.md) (an evidence-based audit of what in that specification is actually implemented today, including known gaps). [`tasks.md`](tasks.md) is the live, dependency-ordered implementation roadmap.

---

## Module path

```
module awo.so/awo
```

Every package in this repository imports itself as `awo.so/awo/<path>` (e.g. `awo.so/awo/def`, `awo.so/awo/platform/iam`). This is the sole, exclusive convention used throughout the source tree — there is no other active module path in this repository.

## Prerequisites

| Requirement | Notes |
|---|---|
| Go | Version pinned in [`go.mod`](go.mod) — use `go version` to check, or run `go build` and let the toolchain manage it |
| PostgreSQL | 14+ recommended. Required at runtime and for integration tests. RLS (`FORCE ROW LEVEL SECURITY`) is a core part of the tenancy security model — the test suite exercises real RLS policies against a real database, never a mock. |
| Redis | 7+ recommended. Required at runtime (sessions, cache, rate limiting) and for integration tests. |
| Temporal | Optional. The server runs in degraded mode (CRUD works, workflow starts are unavailable) if `TEMPORAL_HOST` is unset or unreachable. |

## Build

```bash
go build ./...
go vet ./...
gofmt -l .   # must print nothing
```

## Test

Unit tests need no external services. Integration tests (RLS isolation, session storage, audit atomicity, etc.) need a real PostgreSQL database and a real Redis instance — **this framework's test suite deliberately never mocks the database or Redis for anything security-relevant** (see [`docs/16-testing/TEST_STRATEGY.md`](docs/16-testing/TEST_STRATEGY.md)).

```bash
# Point at a real PostgreSQL + Redis instance. A dedicated test database is
# recommended — integration tests create and drop isolated schemas within it.
export DATABASE_URL="postgres://admin:admin@localhost:5432/awo?sslmode=disable"
export TEST_DATABASE_URL="$DATABASE_URL"
export REDIS_URL="redis://localhost:6379"

go test ./... -count=1
```

Integration tests that need `TEST_DATABASE_URL` skip themselves automatically if it's unset — you will still get a green run, just with reduced coverage of the security-critical paths. Set it to actually exercise RLS/tenant-isolation/session tests.

`go test -race ./...` is part of CI but is **not runnable on an android/arm64 host** (the Go toolchain does not ship a race-detector runtime for that platform — `-race is not supported on android/arm64`). Run it on any standard linux/amd64, linux/arm64 (non-Android), or macOS machine, or rely on CI.

## CI

[`.github/workflows/ci.yml`](.github/workflows/ci.yml) runs, on every push/PR: `gofmt` check, `go vet`, `go build`, a `go mod tidy` dependency-integrity check, the full test suite against real PostgreSQL + Redis service containers (with coverage reported), and `go test -race` on a supported runner.

## Finance module status

Both CLI entrypoints (`cmd/awo`, `cmd/server`) previously blank-imported `awo.so/modules/finance` — a package that was never carried over from the original monorepo into this standalone repository, which made the repository fail to build. This has been resolved: **Finance is not a framework-native module** (it is not among the seven platform modules listed in [`docs/00-overview/ARCH_OVERVIEW.md`](docs/00-overview/ARCH_OVERVIEW.md) §10 — Tenant, IAM, Feature Flags, Settings, Audit, Metadata, Module Registry), and [`docs/99-modules/FINANCE_MODULE_SPEC.md`](docs/99-modules/FINANCE_MODULE_SPEC.md) itself describes Finance as "the canonical **reference** module" — an example of how to author a business module, not part of the kernel.

The two `cmd/` binaries now build and run with only `platform/*` framework-native modules. An application that wants Finance (or any other business module) should depend on `awo.so/awo` as a library and provide its own `cmd/` entrypoint that blank-imports its business modules alongside the platform ones — exactly the pattern `cmd/server/main.go` and `cmd/awo/cmds_schema.go` demonstrate for platform modules today.

See `AUDIT_REPORT.md` §Investigation Method and `tasks.md` Phase 0.2 for the full reasoning behind this decision.

## Repository layout at a glance

```
def/, compiler/, registry/, runtime/, filter/, driver/   — framework core
contrib/pgx/, contrib/redis/                              — infrastructure adapters
platform/*                                                 — framework-native modules (tenant, iam, audit, flags, settings, metadata, organization, notification, attachment, mail)
api/*, sdui/*, generator/*, docgen/                        — API, UI-generation, and codegen layers
cmd/awo/, cmd/server/, cmd/migrate/                        — CLI, HTTP+Temporal server, migration runner
docs/                                                       — canonical specification (see docs/README.md)
```

## Documentation status

`docs/` is the canonical, "frozen at v1.0" specification set, but it was written at an earlier architectural snapshot and has not been fully updated as the framework grew — see `AUDIT_REPORT.md` §1 for the specific, evidence-based list of drift between `docs/` and the current source tree. Treat `docs/` as authoritative for the concepts it covers, and treat the current source tree as authoritative for anything `docs/` doesn't mention (roughly a quarter of the packages that exist today). `docs_legacy_framework_a/` is explicitly deprecated — do not use it.
