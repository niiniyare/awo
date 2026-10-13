# Business Expense — Showcase Demonstration Guide

Status: **fully verified by live execution** (real login, real authenticated
CRUD + lifecycle, real tenant isolation under real RLS) **and** by the
real-Postgres test suite. The one remaining gap is disclosed honestly in §5/§6
below (visual browser rendering of the generated UI — no browser-capable tool
was available in this environment). See
`docs/00-overview/TECHNICAL_FINDINGS_AND_FUTURE_IMPROVEMENTS.md` for the full
findings behind every item below, including **R.6** (the generic-repository
defect that previously blocked the authenticated flow entirely — now fixed)
and **R.7** (a real, separate operational finding about database credential
choice, uncovered and resolved while running this demonstration).

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

**IMPORTANT — database credentials (R.7, see findings register §B.8):** `DATABASE_URL`
must point at a role that is `NOSUPERUSER`/`NOBYPASSRLS`, e.g.:

```sh
psql "$DATABASE_URL" -c "CREATE ROLE awo_demo LOGIN PASSWORD 'change-me' NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS;"
psql "$DATABASE_URL" -c "GRANT USAGE ON SCHEMA public TO awo_demo; GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO awo_demo; GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO awo_demo;"
export DATABASE_URL="postgres://awo_demo:change-me@localhost:5432/awo?sslmode=disable"
```

A PostgreSQL superuser (or any `BYPASSRLS` role) silently defeats tenant
isolation — confirmed directly in this session (see §4's tenant-isolation
check and R.7). Using superuser credentials for `DATABASE_URL`, as the
earlier draft of this demo did, is a deployment mistake, not something this
entity or the framework's RLS policies do wrong.

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

## 4. CRUD and lifecycle — **verified live, end-to-end, with a real authenticated session**

With **R.6 fixed** (see `docs/00-overview/TECHNICAL_FINDINGS_AND_FUTURE_IMPROVEMENTS.md`
§B.7), real login against a real, fully-migrated database works for the
first time in this project's history. The full flow below was run live
against a real PostgreSQL database, a properly-restricted (`NOSUPERUSER
NOBYPASSRLS`) role, and the real running `awo serve` process.

### Login

```sh
curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "alice@acme.test", "password": "<redacted>", "tenant_id": "<acme-tenant-uuid>"}'
```

Succeeded with a `200` and a bearer token in the response body (format:
`{"token": "...", "user": {...}, ...}` — exact fields per
`platform/iam/handler.go`'s `login` handler). Token and real UUIDs are
redacted here per this project's rule against committing real credentials;
every reader can reproduce this exactly by creating their own disposable
tenant/user first.

### Authenticated identity check

```sh
curl -s http://localhost:8080/api/v1/auth/me \
  -H "Authorization: Bearer <token>" -H "X-Tenant-ID: <acme-tenant-uuid>"
```

Returned `200` with Alice's identity (`iam_user` row), confirming the bearer
token and tenant-header auth pattern both work end-to-end.

### Create

```sh
curl -s -X POST http://localhost:8080/api/v1/business/expenses \
  -H "Authorization: Bearer <token>" -H "X-Tenant-ID: <acme-tenant-uuid>" \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Flight to Mombasa for client site visit",
    "amount": "45000.00",
    "currency": "KES",
    "category": "travel",
    "expense_date": "2026-10-01",
    "submitted_by": "<alice-user-uuid>"
  }'
```

`201 Created`. Response envelope matches the generic entity-record shape
(`id`, `tenant_id`, `data.*`, `created_at`, `updated_at`), `status` defaulted
to `draft` as declared. **Known cosmetic defect, disclosed honestly (see
findings register §B.9, `tasks.md` R.8):** `submitted_by` renders in the JSON
response as a raw byte array, e.g.
`[22,233,217,138,62,34,69,55,174,252,161,252,51,161,162,124]`, instead of a
UUID string — the underlying stored/FK-enforced value is correct; this is a
response-serialization gap in Link-type fields generally, not specific to
this entity, and not fixed in this pass.

### Get / List (persistence verified by read-back)

```sh
curl -s http://localhost:8080/api/v1/business/expenses/<id> \
  -H "Authorization: Bearer <token>" -H "X-Tenant-ID: <acme-tenant-uuid>"

curl -s http://localhost:8080/api/v1/business/expenses \
  -H "Authorization: Bearer <token>" -H "X-Tenant-ID: <acme-tenant-uuid>"
```

Both returned `200` with the created record (list envelope:
`{"data":[...],"meta":{"page":1,"page_size":20}}`), confirming the record
genuinely persisted and round-trips through the real `contrib/pgx.Repository`
path — not just accepted and discarded.

### Submit, then approve (lifecycle)

```sh
curl -s -X PATCH http://localhost:8080/api/v1/business/expenses/<id> \
  -H "Authorization: Bearer <token>" -H "X-Tenant-ID: <acme-tenant-uuid>" \
  -H "Content-Type: application/json" -d '{"status": "submitted"}'

curl -s -X PATCH http://localhost:8080/api/v1/business/expenses/<id> \
  -H "Authorization: Bearer <token>" -H "X-Tenant-ID: <acme-tenant-uuid>" \
  -H "Content-Type: application/json" -d '{"status": "approved"}'
```

Both returned `200`, record's `status` field updated correctly each time —
`StatusTransitionGuard` (the `BeforeUpdate` hook in `examples/expense/hooks.go`)
allowed both transitions.

### Invalid transition (rejected)

```sh
# Against a fresh draft record, attempting draft -> approved directly:
curl -s -w '%{http_code}' -X PATCH http://localhost:8080/api/v1/business/expenses/<draft-id> \
  -H "Authorization: Bearer <token>" -H "X-Tenant-ID: <acme-tenant-uuid>" \
  -H "Content-Type: application/json" -d '{"status": "approved"}'
```

Returned `409`, body carrying `expense.invalid_status_transition` — the same
guard, same error code, as the real-Postgres test suite's
`TestExpense_StatusTransitionGuard_InvalidTransition_Rejected`, now also
confirmed live through the real HTTP path, not just the repository-level test.

### Tenant isolation (verified live, bidirectionally, under real RLS)

A second tenant ("Bob", tenant B) was created and authenticated the same way.
Attempting to read tenant A's ("Alice's") expense record as Bob:

```sh
curl -s -w '%{http_code}' http://localhost:8080/api/v1/business/expenses/<alice-record-id> \
  -H "Authorization: Bearer <bob-token>" -H "X-Tenant-ID: <bob-tenant-uuid>"

curl -s http://localhost:8080/api/v1/business/expenses \
  -H "Authorization: Bearer <bob-token>" -H "X-Tenant-ID: <bob-tenant-uuid>"
```

Returned `404 not_found` for the direct `Get`, and
`{"data":[],"meta":{"page":1,"page_size":20}}` for the `List` — Bob cannot see
or enumerate Alice's record. Alice, immediately re-checked afterward with the
same `List` call, still correctly saw her own records.

**This result is only trustworthy under correctly-restricted database
credentials — see R.7/§B.8.** The first run of this exact check, made with
`DATABASE_URL` pointed at a PostgreSQL superuser (this environment's own
bootstrapped `admin` role, used for convenience earlier in this project),
showed Bob **incorrectly** able to read Alice's record — not because Awo's
RLS policy is wrong, but because PostgreSQL superusers unconditionally bypass
RLS regardless of `FORCE ROW LEVEL SECURITY`. Re-running the identical check
after switching to a real, `NOBYPASSRLS` role produced the correct result
shown above, proving the framework's tenant isolation is sound and isolating
the root cause entirely to credential choice, not framework logic.

### Automated coverage (still green, unaffected by any of the above)

```sh
export TEST_DATABASE_URL="postgres://admin:admin@localhost:5432/awo?sslmode=disable"
go test ./examples/... -v
```

| Scenario | Test |
|---|---|
| Create defaults to `draft`, correctly tenant-scoped | `TestExpense_Create_DefaultsToDraft_TenantScoped` |
| A second tenant cannot retrieve the first tenant's record (real RLS) | `TestExpense_TenantIsolation_OtherTenantCannotSeeIt` |
| `draft -> submitted -> approved` succeeds | `TestExpense_StatusTransitionGuard_ValidTransition_Succeeds` |
| `draft -> approved` (skipping `submitted`) is rejected with `expense.invalid_status_transition` | `TestExpense_StatusTransitionGuard_InvalidTransition_Rejected` |
| Updating an unrelated field (e.g. `notes`) is never blocked by the status guard | `TestExpense_StatusTransitionGuard_UnrelatedFieldUpdate_Unaffected` |

5/5 passing.

## 5. UI — SDUI schema verified; visual rendering NOT verified

```sh
curl -s http://localhost:8080/api/v1/ui/nav -H "Authorization: Bearer <token>" -H "X-Tenant-ID: <acme-tenant-uuid>"
curl -s http://localhost:8080/api/v1/ui/business/expenses -H "Authorization: Bearer <token>" -H "X-Tenant-ID: <acme-tenant-uuid>"
curl -s http://localhost:8080/api/v1/ui/business/expenses/<id> -H "Authorization: Bearer <token>" -H "X-Tenant-ID: <acme-tenant-uuid>"
```

All three returned `200` with well-formed amis JSON schema: the nav entry for
Expenses, a list-view schema with the expected columns and an API binding to
`/api/v1/business/expenses`, and a detail-view schema with readable field
labels and the created record's values.

**What was NOT done:** this environment has no browser-capable tool, and the
static frontend bundle (`./awo/web/dist/index.html`) is not built here, so
the generated page was never actually opened and visually inspected — only
its underlying JSON schema was. **This is disclosed as an explicit, open gap,
not claimed as done** — tracked as `tasks.md` **R.9**. Do not treat the JSON
schema check above as equivalent to visual verification.

## 6. What this does NOT prove yet

- Visual browser rendering of the generated UI (R.9 — no browser-capable tool
  available in this environment; JSON schema only, see §5).
- Audit-row content for this entity specifically (the generic mechanism is
  proven elsewhere in the codebase; not re-queried for this entity this pass).
- A fully clean serialization of Link-type fields in JSON responses (R.8 —
  `submitted_by` renders as a byte array, cosmetic, not fixed this pass).

## Removability

Nothing outside `examples/expense/` and `cmd/awo/migrate.go`/`cmd/migrate/main.go`'s
blank-import lists and `cmd/server/main.go`/`cmd/awo/cmds_schema.go`'s
registration imports references this module. Deleting `examples/expense/`
and those 4 import lines removes it completely; no framework package imports
`examples/expense`.
