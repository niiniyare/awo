> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Module Checklist"
id: mdg-12
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Testing](11-testing.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
  - "[Invariants](../02-architecture/invariants.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Module Checklist

**MDG-12 | Module Developer Guide**

This checklist must be completed before a new module is submitted for review. Each item maps to an Architecture Law or Invariant.

---

## Module Structure

- [ ] Module directory follows the standard layout (`def.go`, `hooks.go`, `policy.go`, `manifest.go`, `{module}.go`)
- [ ] `ModuleManifest` declares `Name`, `Label`, `Version`, `Provides`, `Requires`
- [ ] All entity definitions registered in `init()` via `def.Register()`
- [ ] Module manifest registered via `def.RegisterManifest()`
- [ ] Blank import added to `cmd/server/main.go`
- [ ] `RegisterActivities()` called in worker setup if module has workflows

---

## Entity Naming

- [ ] Entity names follow `{module}_{noun}` format (singular noun)
- [ ] Entity names are snake_case
- [ ] No entity name conflicts with existing entities (checked against registry)
- [ ] Entity name is stable — not changing post-migration

---

## Fields and Types

- [ ] No monetary value stored as `Float` — use `Currency` (`numeric(20,4)`)
- [ ] No `time.Now()` in workflow functions — use `workflow.Now(ctx)`
- [ ] Sensitive fields (PII, credentials) declared `Sensitive: true`
- [ ] `NamingSeries` fields have format string declared (`INV-{YYYY}-{SEQ:5}`)
- [ ] `Link` fields reference existing entity types (validated at compile time)
- [ ] No system column names duplicated (`id`, `tenant_id`, `created_at`, `updated_at`, `created_by`, `updated_by`)

---

## Permissions

- [ ] `PermissionSet` declared with at least `Create`, `Read`, `Write`, `Delete`
- [ ] All roles referenced in permissions exist (seeded by provisioning workflow or existing)
- [ ] Module roles follow `role:{module}.{role_name}` format
- [ ] Provisioning workflow seeds module roles on tenant installation

---

## Policies

- [ ] `PolicyFunc` declared if entity data should be row-level filtered
- [ ] Policy returns `filter.All()` for admin roles (not just restricts everyone)
- [ ] Policy unit tests cover: restricted role, unrestricted role, edge cases

---

## Hooks

- [ ] Hook structs named `{Entity}{Purpose}Hook` (e.g., `ContactEmailUniqueGuard`)
- [ ] `BeforeCreate` hooks return `*ValidationError` for field-level errors
- [ ] `AfterCreate`/`AfterUpdate` hooks return wrapped errors with `fmt.Errorf("Op: %w", err)`
- [ ] Hooks that require `EntityRepository` use dependency injection (not global state)
- [ ] Hook unit tests cover: success, validation failure, system error

---

## Actions

- [ ] Action handler functions are ≤50 lines
- [ ] Actions use `action.Repo` (pre-scoped) — no direct DB access
- [ ] `ConfirmationRequired: true` for destructive actions
- [ ] Action names are lowercase, hyphen-separated (e.g., `qualify`, `reassign`)

---

## Workflows

- [ ] Workflow functions contain NO `time.Now()`, `time.Sleep()`, direct I/O, goroutines
- [ ] Workflow functions are deterministic
- [ ] All activities are idempotent (checked with explicit idempotency guard or `ON CONFLICT DO NOTHING`)
- [ ] Activity struct uses dependency injection (not global state)
- [ ] Activity names follow `{Verb}{Noun}Activity` convention
- [ ] Activity unit tests cover: success, non-retryable error, idempotent re-execution
- [ ] Workflow tests use `testsuite.WorkflowTestSuite` with activity mocks
- [ ] `RegisterActivities()` function exists in `workflows/register.go`

---

## SDUI

- [ ] Default-generated schemas used where sufficient — custom `PageBuilderFunc` only for genuine exceptions
- [ ] `PageBuilderFunc` uses `psc.IfPermitted()` for permission-gated elements (not inline permission checks)
- [ ] Raw amis JSON escape hatch includes `// TODO: add SchemaBuilder primitive for {type}` comment
- [ ] amis SDK NOT updated as part of this module work

---

## Migrations

- [ ] Migration file timestamp is unique (14-digit Unix timestamp)
- [ ] Both `.up.sql` and `.down.sql` present and correct
- [ ] All new tenant-scoped tables include: `ENABLE ROW LEVEL SECURITY`, `FORCE ROW LEVEL SECURITY`, `CREATE POLICY tenant_isolation`
- [ ] All production indexes use `CREATE INDEX CONCURRENTLY`
- [ ] No `ALTER TABLE ... RENAME` on columns with existing data
- [ ] Migration tested locally (apply up, verify schema, apply down, verify clean)
- [ ] Migration does NOT run automatically at server startup

---

## Error Handling

- [ ] All errors from external calls wrapped with `fmt.Errorf("Op: %w", err)`
- [ ] No raw `errors.New()` for typed errors — use `*ValidationError`, `*BusinessError`
- [ ] No stack traces or internal error details leaked to API responses
- [ ] Handler functions use `errors.As` (not type switch) for error unwrapping

---

## Observability

- [ ] Business-significant operations logged with `request_id`, `tenant_id`, `user_id`
- [ ] No sensitive fields in log messages
- [ ] Custom Prometheus metrics (if any) prefixed with module name: `{module}_{metric}`

---

## Security

- [ ] No hardcoded secrets, connection strings, or API keys
- [ ] No raw SQL in business logic — uses `EntityRepository`
- [ ] No `WHERE tenant_id = ?` in application code — RLS enforces this at DB level
- [ ] No `BYPASSRLS` or `SUPERUSER` role granted to application role

---

## Laws Compliance Summary

| Law | Check |
|---|---|
| LAW-001 | `definition/` package has no upward imports |
| LAW-005 | No store operation without tenant context |
| LAW-006 | Workflow started via WorkflowTrigger (outbox), not direct Temporal call |
| LAW-007 | Hook registration order is intentional (declared order = execution order) |
| LAW-011 | Entity names unique, stable, immutable |
| LAW-012 | Migrations append-only (no edits after apply) |
| LAW-013 | Sensitive fields not in logs or responses |
| LAW-015 | All tenant-scoped tables have FORCE RLS |
| LAW-016 | Workflow IDs constructed by framework, not module code |
| LAW-017 | Cross-module hooks declare `HookPolicy.Open` |
| LAW-020 | All public APIs have stability annotations |

---

## Review Submission

Before raising a PR:
1. All checklist items above are checked
2. Unit tests pass (run manually: `go test ./internal/core/crm/...`)
3. Migration has been applied and verified in local environment
4. `go vet ./internal/core/crm/...` reports no issues
