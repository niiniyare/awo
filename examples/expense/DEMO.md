# Business Expense — Showcase Demonstration Guide

Status: **fully verified by live execution** (real login, real authenticated
CRUD + lifecycle, real tenant isolation under real RLS, persistence across a
server restart) **and** by the real-Postgres test suite, at both the **API**
and the **UI's own HTTP-contract** level — the checked-in frontend
(`web/dist/index.html`) now genuinely calls the real backend (R.10/R.11,
fixed 2026-10-19). The only remaining gap is **visual** browser rendering
(R.9) — disclosed precisely in §5/§6 below; see findings register §B.13 for
exactly why, checked this pass rather than re-asserted. See
`docs/00-overview/TECHNICAL_FINDINGS_AND_FUTURE_IMPROVEMENTS.md` for the full
findings behind every item below, including **R.6** (the generic-repository
defect that previously blocked the authenticated flow entirely — now fixed,
re-verified again this pass), **R.7** (a real, separate operational finding
about database credential choice, uncovered and resolved while running this
demonstration), **R.10**/**R.11** (the CLI asset-serving and frontend-
integration gaps, both now fixed), and **R.12** (a server logging defect
found and fixed on 2026-10-19 — see the note in §2 below).

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

**(R.10, fixed 2026-10-19 — no longer a caveat):** an earlier pass of this
document warned that launching exactly this way — from the repository root,
as shown above — made `GET /ui`/`GET /showcase*` 404, because the server's
static file serving read a literal, working-directory-relative path. The
showcase frontend is now embedded into the `cmd/awo` binary at compile time
(see findings register §B.10 and `tasks.md` R.10); `/`, `/ui`, and
`/showcase` all now return real `200`s from exactly the command above, with
no workaround, both via `go run` and a separately built executable.

**(R.12, fixed 2026-10-19 — no longer a caveat):** an earlier pass of this
document warned that the server's own structured log misreported some `404`
responses as `status=200` (for a fully-unmatched route, and for a couple of
handlers that returned a `*fiber.Error` directly). That was a real,
confirmed defect in `api/middleware/logging.go`, fixed and regression-tested
this pass (see findings register §B.12 and `tasks.md` R.12) — the server's
own console log can now be trusted for this check too; a fresh re-run after
the fix showed `WARN ... status=404` logged correctly for both previously-
affected cases.

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

## 5. UI — now integrated with the real backend (R.11, fixed 2026-10-19); visual rendering still unverified (R.9)

**Opening it in a browser:** `http://localhost:8080/ui` (or `--port`'s value).
The login overlay's real fields, confirmed from the served HTML, are labeled
exactly **"Tenant ID"** (a UUID — there is no self-service tenant lookup;
ask your administrator or see your own seeded demo tenant), **"Email"**, and
**"Password"**, with a **"Sign in"** button. After signing in, the sidebar
is built live from the real `/api/v1/ui/nav` response. The Expense entity's
direct URL (confirmed from that response's own `listUrl`, not guessed) is
`http://localhost:8080/ui/business/expenses` — deep-linking to it while
logged out shows the login screen first and lands on the list immediately
after a successful sign-in; the create form is at
`.../ui/business/expenses/create`.

### 5a. The backend's own SDUI generation is correct (re-verified live, this pass)

```sh
curl -s http://localhost:8080/api/v1/ui/nav -H "Authorization: Bearer <token>" -H "X-Tenant-ID: <acme-tenant-uuid>"
curl -s http://localhost:8080/api/v1/ui/business/expenses -H "Authorization: Bearer <token>" -H "X-Tenant-ID: <acme-tenant-uuid>"
curl -s http://localhost:8080/api/v1/ui/business/expenses/<id> -H "Authorization: Bearer <token>" -H "X-Tenant-ID: <acme-tenant-uuid>"
```

All three return `200` with well-formed amis JSON schema: `GET /api/v1/ui/nav`
correctly includes `business → Expenses → business_expense →
/ui/business/expenses` among the other 20 real entities; the list-view schema
has the expected columns, filters, a "New Expense" create action, and an API
binding to `/api/v1/business/expenses`; the detail/edit-view schemas have
readable field labels, validations, and a real created record's values.

### 5b. The checked-in `web/dist` frontend is now wired to call all of the above for real

Previously (two separate, now-fixed causes — see findings register §B.10/§B.11
for the full history): the documented startup command 404'd on `/ui`/`/showcase`
entirely (R.10, fixed — see §2), and even once that was worked around, the
shipped frontend was a disconnected static demo shell with a fictional
hardcoded nav, a login form pointed at a nonexistent URL, and a schema loader
that only ever read static mock files, never the real backend (R.11).

`web/dist/index.html` has been rewritten to use the real contract throughout:
real login (`POST /api/v1/auth/login`), a real session check (`GET
/api/v1/auth/me`), a sidebar built live from `GET /api/v1/ui/nav`, real
schema loading from `GET /api/v1/ui/{module}/{entity}[/{id}][/create|/edit]`,
real path-based routing matching the backend's own generated links, and a
generic adapter translating the real `{data}`/`{error}` response envelopes
into what amis expects. Verified this pass by replaying, via curl, exactly
the sequence of requests the rewritten JS makes:

```sh
# 1. Login (the frontend's login form)
curl -s -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" \
  -d '{"email":"<email>","password":"<password>","tenant_id":"<tenant-uuid>"}'

# 2. Session check on page load
curl -s http://localhost:8080/api/v1/auth/me -H "Authorization: Bearer <token>" -H "X-Tenant-ID: <tenant-uuid>"

# 3. Sidebar nav
curl -s http://localhost:8080/api/v1/ui/nav -H "Authorization: Bearer <token>" -H "X-Tenant-ID: <tenant-uuid>"

# 4. List data (amis's own crud component call, page_size — not perPage — reaches the backend)
curl -s "http://localhost:8080/api/v1/business/expenses?page=1&page_size=20" -H "Authorization: Bearer <token>" -H "X-Tenant-ID: <tenant-uuid>"

# 5. Create (the "New Expense" form's submit — submitted_by filled by the user via the
#    now-enabled remote-select, which the generated create form previously left disabled
#    with no default)
curl -s -X POST http://localhost:8080/api/v1/business/expenses -H "Authorization: Bearer <token>" -H "X-Tenant-ID: <tenant-uuid>" \
  -H "Content-Type: application/json" \
  -d '{"description":"...","amount":"777.00","category":"other","expense_date":"2026-10-19","submitted_by":"<user-uuid>"}'

# 6. The new record appears in a subsequent list fetch (same as step 4, re-run)
```

All six steps returned the expected real results this pass — login `200`,
`/auth/me` `200`, nav `200` (21 entries), list `200` (existing + new records),
create `201`, and the new record present in the very next list response
(confirmed by `meta.total` incrementing and the new description appearing).
The full lifecycle (submit/approve/invalid-transition/tenant-isolation) was
then re-run through the same `PATCH .../{id}` contract the generated Edit
form's "Save" button uses — identical results to §4 above, with a second
tenant correctly blocked. The created record was confirmed directly in
PostgreSQL via `psql`, and again after a full server restart.

**Known-fixed UX issue (R.14, see findings register §B.15):** a real user
testing this exact flow on Android Chrome found that the create form did
**not** navigate away or clear after a successful Save — inviting repeated
submissions and multiple real duplicate "Test expense" records. A first fix
(adding a generic post-submit `redirect`) was shipped but **the user's own
retest showed it still didn't work** — correctly treated as authoritative
over the unit tests that had claimed success. The real causes turned out to
be two separate bugs in the frontend's fetcher/data-binding, found this
second pass by actually downloading and executing the real amis SDK: (1) the
fetcher's response shape collided with how amis's own success/failure
detection works, specifically because Expense's own `status` field
(draft/submitted/approved) was mistaken for amis's application-level status
code, making every submission look like a failure to amis regardless of the
real HTTP result; (2) edit forms never received the record's own id into
amis's data scope, so `${id}` template references never resolved and the
edit form rendered blank. Both are now fixed and verified by actually
executing the real amis SDK against the real shipped code (not just unit
tests) — see R.14/§B.15 for the full evidence. Every duplicate record this
bug produced, across both rounds, was deliberately left in the database —
removing them is a separate decision, not a side effect of this fix. This
fix has **not** been confirmed in an actual browser yet — the server is left
running live for that retest.

**What was NOT done — visual rendering (R.9):** this is HTTP-contract-level
verification, replaying the exact requests the real JavaScript makes — it is
**not** a substitute for opening the page in a real browser and looking at
it. Re-checked this pass (not just re-asserted): real Chrome/Firefox apps
are installed on this device and can genuinely be launched pointed at the
running server (`am start -a android.intent.action.VIEW -d ...`), but this
sandboxed shell has no screenshot command, no accessible console/network
log, and no reachable remote-debugging port — there is no way to observe
what actually renders. See findings register §B.13 for the full list of
what was checked and ruled out. **This remains an explicit, open gap** —
tracked as `tasks.md` **R.9** — not claimed as done.

## 6. What this does NOT prove yet

- Visual browser rendering of the generated UI (R.9 — real browsers exist on
  this device and can be launched, but no inspection channel — screenshot,
  console, remote debugging — is reachable from this sandboxed shell; see
  §B.13).
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
