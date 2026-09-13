> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Module Development Checklist"
id: mdg-016
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: normative
related:
  - "[Module System](../10-modules/module-system.md)"
  - "[Common Mistakes](14-common-mistakes.md)"
  - "[Testing Patterns](15-testing-patterns.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Module Development Checklist

**MDG-016 | Status: Accepted | Stability: Stable**

A gate-by-gate checklist for every new module before it is merged to main.

---

## Gate 1 — Entity Definition

- [ ] All entity names follow `{module}_{noun}` format (e.g. `finance_invoice`)
- [ ] Entity `Name` is final — no rename after first migration (ADR-008)
- [ ] Financial amounts use `FieldCurrency` (not `FieldFloat` or `FieldInt`)
- [ ] System entity chosen for: financial data, inventory accounting, IAM data, high-frequency writes
- [ ] Custom entity chosen for: tenant-specific, frequently evolving, no financial/IAM participation
- [ ] `role:tenant.admin` included in all `Create`, `Write`, `Delete` permission lists
- [ ] Custom role names follow `role:{module}.{function}` format
- [ ] `Immutable: true` on fields that must not change after creation
- [ ] `Sensitive: true` on fields that must not appear in logs or responses (passwords, tokens)
- [ ] `Searchable: true` on fields intended for full-text search (generates GIN index)

---

## Gate 2 — Hooks and Validators

- [ ] No raw SQL in hooks — all data access via `EntityRepository` and Filter DSL
- [ ] No `time.Now()` in hooks — pass `time.Time` via context or input (for testability)
- [ ] Hook errors use `ValidationError` (field-level, 422) or `BusinessError` (domain rule, 4xx)
- [ ] Error chains use `fmt.Errorf("op.Method: %w", err)` — no bare `return err`
- [ ] `after_save` hooks designed for rollback safety — must be idempotent or compensatable
- [ ] No cross-module `EntityRepository` imports in hooks — use service interfaces
- [ ] `FieldValidator` implementations are pure (no I/O, no external calls)

---

## Gate 3 — Migrations

- [ ] One `.up.sql` + one `.down.sql` per schema change (no auto-migrate)
- [ ] File naming: 14-digit Unix timestamp + description slug
- [ ] Every new tenant-scoped table has RLS enabled:
  ```sql
  ALTER TABLE my_entity ENABLE ROW LEVEL SECURITY;
  ALTER TABLE my_entity FORCE ROW LEVEL SECURITY;
  CREATE POLICY tenant_isolation ON my_entity USING (tenant_id = current_tenant_id());
  ```
- [ ] New indexes use `CREATE INDEX CONCURRENTLY` (no table locks in production)
- [ ] `down.sql` actually reverses the `up.sql` (tested locally)
- [ ] No destructive operations in `up.sql` (no DROP COLUMN, no TRUNCATE) in first migration — follow zero-downtime patterns

---

## Gate 4 — RBAC and Policies

- [ ] `PolicyFunc` declared for all entities where not all actors should see all rows
- [ ] PolicyFunc uses Filter DSL — no raw SQL predicates
- [ ] PolicyFunc does not import repositories directly — reads actor from context
- [ ] No `WHERE tenant_id = ?` in application code — RLS enforces this automatically
- [ ] SDUI permission gates use `psc.IfPermitted(...)` — not `VisibleWhen` for authorization

---

## Gate 5 — Workflow Integration

- [ ] No `time.Now()` or `time.Sleep()` in workflow functions — use `workflow.Now()` / `workflow.Sleep()`
- [ ] No direct I/O in workflow functions — all I/O in activities
- [ ] No goroutines in workflow functions — use `workflow.Go`
- [ ] Activities are idempotent — check for existing work before creating/updating
- [ ] `SetTenantContext` called at the start of every activity that touches the database
- [ ] Workflow IDs follow `{tenant-uuid}.{entity-type}.{record-id}.{event}` convention
- [ ] Saga compensations registered in declaration order (LIFO execution on failure)

---

## Gate 6 — SDUI

- [ ] No custom React/JS for standard CRUD views — amis JSON schemas only
- [ ] amis SDK version not updated (pinned in `web/sdk/`)
- [ ] SDUI permission gates use `psc.IfPermitted(...)` (absent from schema, not just hidden)
- [ ] Form field names match `FieldDef.Name` exactly (for validation error routing)
- [ ] Currency fields have `type: "number"` with `precision: 4` in amis schema
- [ ] Dark mode: CSS custom property tokens, not `theme("dark")`

---

## Gate 7 — Settings and Feature Flags

- [ ] Configurable thresholds declared as `def.Setting` (not hard-coded)
- [ ] Settings read once before loops — not inside loops
- [ ] Boolean capability switches use Feature Flags, not Settings
- [ ] Setting keys follow `{module}.{parameter_name}` format
- [ ] Flag keys follow `{module}.{feature_name}` format

---

## Gate 8 — Observability

- [ ] All log entries carry: `request_id`, `tenant_id`, `user_id` where available
- [ ] Sensitive fields never logged (`password_hash`, `session_token`, API secrets)
- [ ] Custom Prometheus metrics registered for domain-specific business events
- [ ] Health check path works after module registration (no nil panics in init())

---

## Gate 9 — Tests

- [ ] Unit tests for all hooks (table-driven, mock `EntityRepository`)
- [ ] Unit tests for all `FieldValidator` implementations
- [ ] Workflow tests use `testsuite.WorkflowTestSuite` (not real Temporal)
- [ ] Integration tests use real PostgreSQL (not mocked DB)
- [ ] Test covers the PolicyFunc isolation (actor A cannot see actor B's records)
- [ ] Test file alongside source: `entity_foo_test.go` next to `entity_foo.go`

---

## Gate 10 — Security

- [ ] No secrets in code (API keys, passwords, signing keys) — environment variables only
- [ ] Input validation at all entry points (API handler validates before passing to service)
- [ ] Error responses never include stack traces or internal details
- [ ] No raw user input in log messages (prevent log injection)
- [ ] File upload handlers (if any) validate MIME type, extension, and size limit

---

## Pre-Merge Final Check

```bash
# Run by the author before creating the PR
go build ./...
go vet ./...
go test ./internal/core/{module}/...
go test -tags integration ./internal/core/{module}/...
```

(Never run these in CI automatically — tell the user to run them per CLAUDE.md policy.)

Verify the migration round-trips cleanly:

```bash
migrate -path db/migration -database $DATABASE_URL up
migrate -path db/migration -database $DATABASE_URL down 1
migrate -path db/migration -database $DATABASE_URL up
```

---

## Related Documents

- [Common Mistakes](14-common-mistakes.md) — what each checklist item prevents
- [Testing Patterns](15-testing-patterns.md) — test templates for Gate 9
- [Module System](../10-modules/module-system.md) — module manifest and registration
- [Hardening Guide](../15-security/hardening-guide.md) — Gate 10 detail
