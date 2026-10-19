# Business Expense — Showcase Demonstration Guide

Status: **fully verified by live execution** (real login, real authenticated
CRUD + lifecycle, real tenant isolation under real RLS) **and** by the
real-Postgres test suite, at the **API level**. The generated **visual** UI
is a separate, currently-broken story — disclosed precisely in §5/§6 below,
with two independent, now-identified causes (not merely "no browser was
available"). See
`docs/00-overview/TECHNICAL_FINDINGS_AND_FUTURE_IMPROVEMENTS.md` for the full
findings behind every item below, including **R.6** (the generic-repository
defect that previously blocked the authenticated flow entirely — now fixed),
**R.7** (a real, separate operational finding about database credential
choice, uncovered and resolved while running this demonstration), and
**R.10**/**R.11** (the two causes of the UI gap, found this pass).

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

**KNOWN GAP (R.10, see findings register §B.10):** launched exactly this
way — from the repository root, as shown above — `GET /ui`, `GET /ui/*`, and
`GET /showcase*` all return `404`. The server's static file serving
(`cmd/awo/serve_impl.go`) hard-codes paths relative to its *working
directory* (`./awo/web/dist`, `./awo/web/showcase/index.html`), not to the
repository root, and no `awo/` subdirectory exists anywhere in this repo. The
only way confirmed this pass to make `/ui`/`/showcase` actually serve real
HTML is to launch the compiled binary with its working directory set to the
**parent** of this checkout (so `./awo/web/dist` happens to resolve to this
repo's real `web/dist`, since the checkout itself is named `awo`):

```sh
go build -o /tmp/awo-bin ./cmd/awo   # from inside the repo
cd ..                                 # one level above the checkout
/tmp/awo-bin serve --db "$DATABASE_URL" --redis "$REDIS_URL" --port 8080
```

All API routes (`/api/v1/...`, §3 onward below) work correctly either way —
only the static UI/showcase file serving is affected. This is a real,
undecided framework gap, not something to work around permanently; see R.10.

**ANOTHER KNOWN GAP (R.12, see findings register §B.12) — don't trust the
server's own console log for this specific check:** if you run the broken
(repo-root) form above and look at the server's own structured log instead
of checking the HTTP response directly, a bare `GET /` (and in fact any
request matching no registered route at all) is misreported there as
`status=200`, even though the real response sent to the client is a genuine
`404`. Verify with `curl -D - http://localhost:8080/ui` (or any such path)
and read the actual `HTTP/1.1 ...` status line yourself, rather than trusting
the log line for this specific class of request.

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

## 5. UI — SDUI backend schema verified; the checked-in visual frontend does NOT show this entity today (two independent, confirmed causes)

### 5a. The backend's own SDUI generation is correct (verified live, this pass)

```sh
curl -s http://localhost:8080/api/v1/ui/nav -H "Authorization: Bearer <token>" -H "X-Tenant-ID: <acme-tenant-uuid>"
curl -s http://localhost:8080/api/v1/ui/business/expenses -H "Authorization: Bearer <token>" -H "X-Tenant-ID: <acme-tenant-uuid>"
curl -s http://localhost:8080/api/v1/ui/business/expenses/<id> -H "Authorization: Bearer <token>" -H "X-Tenant-ID: <acme-tenant-uuid>"
```

All three returned `200` with well-formed amis JSON schema: `GET /api/v1/ui/nav`
correctly includes `business → Expenses → business_expense →
/ui/business/expenses` among the other real entities; the list-view schema
has the expected columns and an API binding to `/api/v1/business/expenses`;
the detail-view schema has readable field labels, validations, and a
real created record's values. Re-verified this pass with a freshly-created,
isolated login session — not just re-asserted from a prior run.

### 5b. The checked-in `web/dist` frontend cannot reach any of the above, for two separate, now-identified reasons

**Cause 1 (R.10):** the documented startup command in §2 above 404s on every
`/ui`/`/showcase` route, because the server's static file paths are relative
to its working directory and assume a directory named `awo` exists as a
*child* of the CWD — true only if you launch from one level above this
checkout (see §2's workaround). This alone was never diagnosed in any prior
pass of this demo.

**Cause 2 (R.11), confirmed even after working around Cause 1:** the actual
HTML/JS shipped in `web/dist/index.html` is a static, hand-authored demo
page, not a build artifact wired to this backend:

- Its sidebar navigation is a **hard-coded JavaScript array** of fictional
  modules (Finance, Inventory, HR, CRM, Forecourt, plus a thin Platform
  group) — it does not include "Expenses" and does not match any of this
  project's real registered entities.
- Its router always fetches a **static file** at `/schemas/pages/<page>.json`
  — never the real `/api/v1/ui/{module}/{entity}` endpoint. Confirmed live:
  `GET /schemas/pages/business/expenses.json` → `404` (no such file exists;
  there is no "business" folder under `web/dist/schemas/pages/`).
- Its login form calls `fetch('/auth/login', ...)` — missing the real
  backend's `/api/v1` prefix. Confirmed live: `POST /auth/login` → `404`.
  The real route, used throughout §2-§4 above, is `POST /api/v1/auth/login`.
- Its "Create workspace" tab calls `/auth/register` — a route that does not
  exist anywhere in this backend at all (only `/login`, `/logout`, `/me` are
  registered).
- Its API fetcher sends an `X-Awo-Tenant` header; every real request in this
  document uses the backend's actual header, `X-Tenant-ID`.

**Net effect:** even with a working browser, opening `/ui` today would show
a sidebar with no Expense entry, and manually navigating to it (by any
means) would not retrieve real data for Expense or any other entity — the
static shell simply never talks to this backend correctly. This is **not**
an SDUI-engine defect (§5a proves the engine itself is correct) — it is a
disconnected frontend artifact. See findings register §B.10/§B.11 and
`tasks.md` **R.10**/**R.11** for the full evidence and the (undecided,
out-of-scope-for-this-pass) remediation options.

**What was NOT done:** no browser-capable tool exists in this environment
(confirmed: no Chromium/Firefox, no Playwright/Puppeteer, no compatible
Linux userland to run one under Termux/Android) — so visual rendering itself
remains unverified regardless of the above. **This is disclosed as an
explicit, open gap, not claimed as done** — tracked as `tasks.md` **R.9**.
Given 5b's findings, a browser would not currently show anything different
from what the HTTP-level checks above already show.

## 6. What this does NOT prove yet

- Visual browser rendering of the generated UI (R.9 — no browser-capable tool
  available in this environment).
- That the checked-in `web/dist` frontend shell actually displays the
  Expense entity at all — confirmed this pass that, as currently wired, it
  cannot (R.10/R.11); only the backend's SDUI JSON generation is verified
  (§5a).
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
