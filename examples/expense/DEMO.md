# Business Expense — Showcase Demonstration Guide

Status: **partially verified by live execution, partially verified by the
real-Postgres test suite only**. This document says exactly which is which —
see `docs/00-overview/TECHNICAL_FINDINGS_AND_FUTURE_IMPROVEMENTS.md` for the
full findings behind every caveat below.

## What this proves

A complete business entity — persistence, tenant isolation, generated CRUD
API, permission enforcement, mandatory audit, and an explicit status
lifecycle — built with zero hand-written SQL beyond this entity's own
migration, and zero hand-written API/routing code.

## 1. Apply the migration (verified live, via the real CLI)

```sh
export DATABASE_URL="postgres://admin:admin@localhost:5432/awo?sslmode=disable"
go run ./cmd/awo migrate status
go run ./cmd/awo migrate up
```

Actually run this pass. Output:

```
status: current version 25002, dirty: false
...
migrate: up complete
```

Confirmed via `psql -c '\d business_expense'`: real table, `tenant_isolation`
RLS policy (`FORCE ROW LEVEL SECURITY`), FK constraints to `platform_tenant`
and `iam_user`, `trg_set_updated_at` trigger — all exactly as declared in
`examples/expense/migrations/001_create_business_expense.up.sql`, nothing
hand-patched into the database separately.

## 2. Start the server (verified live)

```sh
export REDIS_URL="redis://localhost:6379"
go run ./cmd/awo serve --db "$DATABASE_URL" --redis "$REDIS_URL" --port 8080
```

```
Server ready  (21 entities, 106 routes)
   API      http://localhost:8080/api/v1
   UI       http://localhost:8080/ui
   Showcase http://localhost:8080/showcase
```

## 3. Discover the route (verified live)

```sh
curl -s http://localhost:8080/api/v1/showcase/entities | python3 -m json.tool
```

Real response (filtered to the expense entry):

```json
{
  "qualifiedName": "business_expense",
  "module": "business",
  "label": "Expense",
  "labelPlural": "Expenses",
  "fieldCount": 6,
  "routePrefix": "/api/v1/business/expenses"
}
```

## 4. CRUD and lifecycle — **verified by the real-Postgres test suite, not by a completed live curl session**

The live, fully-authenticated curl flow (login → bearer token → CRUD →
cross-tenant isolation check) is currently **blocked** by a severe,
newly-discovered framework defect unrelated to this entity: `TenantResolver`
and `AuthService.Login`'s role-loading both fail against certain real,
core platform tables (`platform_tenant`, `iam_user_role`) because
`contrib/pgx.Repository`'s generic scanner assumes columns those tables
don't have. See `docs/00-overview/TECHNICAL_FINDINGS_AND_FUTURE_IMPROVEMENTS.md`
§B.7 and `tasks.md` **R.6** for the full writeup — it is a framework-level
issue, not something wrong with this entity's own definition or migration,
and not something to patch casually.

**What IS verified, against real PostgreSQL, with the real non-superuser
`awo_app` role, through the real `EntityService`/`contrib/pgx.Repository`
pipeline** (`go test ./examples/... -v`, 5/5 passing):

| Scenario | Test |
|---|---|
| Create defaults to `draft`, correctly tenant-scoped | `TestExpense_Create_DefaultsToDraft_TenantScoped` |
| A second tenant cannot retrieve the first tenant's record (real RLS) | `TestExpense_TenantIsolation_OtherTenantCannotSeeIt` |
| `draft -> submitted -> approved` succeeds | `TestExpense_StatusTransitionGuard_ValidTransition_Succeeds` |
| `draft -> approved` (skipping `submitted`) is rejected with `expense.invalid_status_transition` | `TestExpense_StatusTransitionGuard_InvalidTransition_Rejected` |
| Updating an unrelated field (e.g. `notes`) is never blocked by the status guard | `TestExpense_StatusTransitionGuard_UnrelatedFieldUpdate_Unaffected` |

Run them yourself:

```sh
export TEST_DATABASE_URL="postgres://admin:admin@localhost:5432/awo?sslmode=disable"
go test ./examples/... -v
```

### Illustrative request shapes (fields and routes verified; full response bodies not captured live due to the blocker above)

```sh
# Create (route and required fields verified against the real compiled schema)
curl -s -X POST http://localhost:8080/api/v1/business/expenses \
  -H "X-Tenant-ID: <tenant-uuid>" -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Flight to client site",
    "amount": "450.00",
    "category": "travel",
    "submitted_by": "<iam_user-uuid>"
  }'

# Submit for approval
curl -s -X PATCH http://localhost:8080/api/v1/business/expenses/<id> \
  -H "X-Tenant-ID: <tenant-uuid>" -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"status": "submitted"}'

# Approve
curl -s -X PATCH http://localhost:8080/api/v1/business/expenses/<id> \
  -H "X-Tenant-ID: <tenant-uuid>" -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"status": "approved"}'

# Invalid transition (draft -> approved directly) — expect 409 expense.invalid_status_transition
```

`<token>` requires a working login, which requires R.6 to be resolved first
(or a dev database where `platform_tenant`/`iam_user_role` happen to satisfy
the generic scanner's assumptions — not the case on a fresh `cmd/migrate`'d
database as of this pass).

## 5. UI

**Not demonstrated this pass.** The live server was stopped once R.6 blocked
further progress down the authenticated path, before `/ui` could be opened
and inspected. This is disclosed as an open gap, not claimed as done.

## 6. What this does NOT prove yet

- Live, authenticated, cross-tenant curl isolation check (blocked by R.6; RLS
  isolation itself IS proven, by the test suite, in §4 above).
- SDUI UI rendering for this entity (not reached).
- Audit-row content for this entity specifically (the generic mechanism is
  proven elsewhere in the codebase; not re-queried for this entity this pass).

## Removability

Nothing outside `examples/expense/` and `cmd/awo/migrate.go`/`cmd/migrate/main.go`'s
blank-import lists and `cmd/server/main.go`/`cmd/awo/cmds_schema.go`'s
registration imports references this module. Deleting `examples/expense/`
and those 4 import lines removes it completely; no framework package imports
`examples/expense`.
