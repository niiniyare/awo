# Phase 1 Security Closure Report

Triggered by the discovery, during Phase 1's filter/query security work, of a real
SQL injection in `Repository.Query`'s `ORDER BY` construction. This report covers
the full closure pass: reproducing and fixing that injection against a much wider
adversarial payload set, auditing every other dynamic-SQL construction site in the
codebase for the same class of bug (which surfaced two more real findings), and
resolving the `Repository.WithTx` panic-safety gap flagged but not fixed earlier
in Phase 1. Nothing in the existing Phase 1 work (session revocation, tenant
lifecycle enforcement, organisation RLS, platform-admin verification, `/auth/logout`
and `/auth/me`) was reverted, redone, or reworked — this pass builds on top of it.

## 1. SQL injection discovery

The injection itself (`Repository.Query`'s `ORDER BY` clause interpolating the
unvalidated `?orderBy=` HTTP query parameter) was found and fixed earlier in this
Phase 1 effort. This closure pass's job was to verify that fix holds against a
far wider attack-shape catalogue than the original single proof-of-concept
payload, and to determine whether the same root cause exists anywhere else.

## 2. Exact vulnerable path (as it existed before the original fix)

```
HTTP GET /api/v1/entities/{entity}?orderBy=<attacker string>
  → api/handler/crud.go: orderBy := c.Query("orderBy")   (no validation)
  → driver.WithSort(orderBy, ...)                         (raw pass-through)
  → contrib/pgx/repo.go Query(): qo.SortField
  → fmt.Sprintf(`ORDER BY "%s" %s`, qo.SortField, dir)     (naive interpolation)
  → executed against PostgreSQL
```

A value containing a literal `"` breaks out of the quoted identifier. The
sharpest proof payload, `` name" -- ``, produces `ORDER BY "name" --" ASC` —
PostgreSQL's `--` line comment silently swallows the trailing direction and
quote with **no syntax error at all**, which is what makes this class of bug
dangerous: it doesn't fail loudly, it quietly does something other than what
field-name validation should have allowed.

## 3. Reproduction (this pass)

Re-verified against the current, fixed code with a table-driven adversarial
suite (`TestQuery_SortField_AdversarialPayloads_AllRejected`,
`contrib/pgx/filter_security_test.go`) covering 18 distinct attack shapes
against a real PostgreSQL database:

| Shape | Payload example | Result |
|---|---|---|
| Function call | `pg_sleep(5)` | rejected |
| Function wrapping a real column | `lower(name)` | rejected |
| Subquery | `(SELECT password_hash FROM iam_users LIMIT 1)` | rejected |
| Expression injection | `name \|\| code` | rejected |
| Comment injection (line) | `name --` | rejected |
| Comment injection (block) | `name /*` | rejected |
| Quoted-identifier breakout | `name" = 'x'; --` | rejected |
| Stacked statement attempt | `name; DROP TABLE test_entity; --` | rejected |
| Case variant | `NAME` | rejected |
| Leading/trailing whitespace | `` ` name`/`name ` `` | rejected |
| Comma-separated multi-field | `name, code` | rejected |
| Embedded direction syntax | `name ASC` / `name desc` | rejected |
| Leading dash (Rails-style) | `-name` | rejected |
| Unicode homoglyph | `namе` (Cyrillic е) | rejected |
| Null byte | `name\x00` | rejected |
| Nested quotes | `` na""me `` | rejected |
| Whitespace-only | `"  "` | rejected |
| Unknown but syntactically clean column | `not_a_real_column` | rejected |

Plus positive controls: legitimate ascending/descending sort by a real
declared field continues to work
(`TestQuery_SortField_LegitimateField_StillWorks`).

## 4. Root cause

`sqlbuild.NewAllowlist(r.schema).Check(qo.SortField)` performs an **exact
map-membership test** against the entity's declared field names (plus the
standard framework columns). Every one of the 18 shapes above fails that
check identically — none of them is byte-for-byte equal to a real field
name — so the mechanism doesn't need per-shape logic to defend against
per-shape attacks; it structurally rejects anything that isn't a real,
declared column, before the value is ever quoted (`sqlbuild.QuoteIdent`) and
written into SQL.

## 5. Exploitability (as it existed pre-fix)

Remote, unauthenticated-for-the-endpoint's-own-auth-model, reachable via the
standard `GET .../entities/{entity}?orderBy=` list endpoint available for
every registered entity. Practical impact ranged from silently defeating
pagination/ordering guarantees (the comment-injection PoC) up to, with a
more elaborate balanced-quote payload, arbitrary WHERE/ORDER-BY-position SQL
expression injection — bounded only by PostgreSQL's simple/extended query
protocol not permitting stacked statements in a single parameterized call.

## 6. Fix architecture

```
untrusted ?orderBy= value
        ↓
sqlbuild.NewAllowlist(r.schema).Check(field)   — is this a real, declared column?
        ↓ (only if it passed)
sqlbuild.QuoteIdent(field)                     — can this string break out of its identifier position?
        ↓
ORDER BY <quoted identifier> ASC|DESC
```

Both steps are necessary and serve different purposes: the allowlist is the
**authorization** boundary (only real, intended columns — sorting/grouping by
`tenant_id`/`created_at` is legitimate, so the standard framework columns are
included here), and `QuoteIdent` is the **escaping** boundary (doubles any
embedded `"`). Neither alone would be sufficient: quoting alone (the
pre-fix state minus even that) still permits sorting by any real column that
happens to exist on the table, including ones never declared as entity
fields; an allowlist alone, if some future caller ever needs a
looser-than-exact-match check, would still need correct escaping.

`sqlbuild.Allowlist` (`contrib/pgx/sqlbuild/allowlist.go`) was investigated
and judged **exactly the right existing mechanism for read contexts** — it
already existed, was already unit-tested, and its `NewAllowlist` constructor
already needed one bug fixed (reading both `es.Fields` and `es.FieldsByName`,
done in this Phase 1 effort's earlier work) to be safely adoptable. It is now
used for `Query`'s `ORDER BY` and `Aggregate`'s `Field`/`GroupBy`. It remains
**unsuitable, by design, for write contexts** (`Update`/`BulkUpdate`'s `SET`
clause) — see §9 — so a second, narrower check
(`Repository.checkWritableField`) was added there rather than forcing one
abstraction to serve two different authorization questions. This is a
disposition, not a punt: two allowlist-shaped functions exist because they
answer two different questions ("can this be read/sorted/grouped by" vs.
"can this be written to"), not because of duplicated, competing logic — the
underlying identifier-quoting (`sqlbuild.QuoteIdent`) is shared by both.

## 7. Regression tests (ORDER BY specifically)

`contrib/pgx/filter_security_test.go`: `TestQuery_SortField_RejectsSQLInjectionAttempt`
(the original sharp proof-of-concept), `TestQuery_SortField_UnknownColumn_Rejected`,
`TestQuery_SortField_LegitimateField_StillWorks`, and the 18-shape
`TestQuery_SortField_AdversarialPayloads_AllRejected` table above — all against
real PostgreSQL, all passing.

## 8. Dynamic-SQL audit (Step 5) — findings

Every `fmt.Sprintf`/`fmt.Fprintf` constructing SQL, plus every WHERE/ORDER
BY/GROUP BY/HAVING/JOIN/LIMIT/OFFSET/IN-clause/identifier-interpolation site
across `contrib/pgx`, `report`, `events/outbox`, `generator`, and
`testutil/db` was reviewed. Two more real instances of the same root cause
were found, both confirmed to have **zero production callers today**
(verified by exhaustive caller search, not assumed):

1. **`Repository.Aggregate`** (`contrib/pgx/repo.go`) — `fn.Field`/`spec.GroupBy`
   used the identical naive-quoting pattern the ORDER BY bug had; `fn.Fn`
   (the aggregate function name) was interpolated completely unquoted with
   no validation at all. Fixed (§9, item 1.15 in `tasks.md`).
2. **`report.GenerateSQL`** (`report/report.go`) — `GroupBy`/`OrderBy[].Field`/
   `Aggregates[].Field` lacked schema validation (though not breakout-vulnerable —
   its `quote()` strips embedded quote characters, independently verified by
   re-reading the function rather than assumed); `Aggregates[].Func` was
   unvalidated and unquoted, the same severity class as `Repository.Aggregate`'s
   `fn.Fn`. Fixed (§9, item 1.16 in `tasks.md`). `ReportField.Expr`'s
   documented raw-SQL escape hatch was deliberately left untouched — see §9.

**One additional finding that IS live** (not dead code, found by tracing
`report.GenerateSQL`'s sibling in `contrib/pgx/repo.go` rather than the
report package): `updateSystem`/`BulkUpdate`'s write-path `SET` clause. This
is the most severe finding of the whole audit — see §9.

**Everything else reviewed was confirmed safe by construction**:
`r.schema.TableName` interpolations throughout `contrib/pgx/repo.go` are
compile-time, developer-authored values (never client input);
`createSystem`/`bulkCreateSystem` iterate the schema's own known field list
and look up values by trusted keys (never the map's own keys — the key
structural difference from the vulnerable `updateSystem`/`BulkUpdate`);
`events/outbox/relay.go`'s `outboxTable` is a hardcoded constant;
`generator/generator.go`'s `fmt.Fprintf` calls build migration/DDL text at
compile/migration time from developer-authored entity definitions, never
from runtime request data; WHERE-clause building via `sqlbuild.Build`
already quotes identifiers correctly (a separate, lower-severity,
already-logged authorization gap — `tasks.md` 1.13 — not an injection).

## 9. `updateSystem`/`BulkUpdate` — the most severe finding

`Repository.Update`'s system-entity branch and `BulkUpdate`'s system branch
built their `SET` clause by iterating the **keys of a client-controlled
map** — `driver.UpdateInput.Data` / `driver.Patch.Set` — with the same naive
`fmt.Sprintf('"%s" = $%d', field, ...)` pattern:

```
PATCH /api/v1/entities/{entity}/{id}   body: arbitrary JSON object
  → api/handler/crud.go: json.Unmarshal(c.Body(), &body)   (no key restriction)
  → EntityService.Update(ctx, id, body, actor)
  → runtime/pipeline.go RunBeforeUpdate → validateFields    (checks RequiredFields/
                                                               FieldValidators only —
                                                               never rejects an
                                                               unrecognized key)
  → driver.UpdateInput{Data: patched.Data}
  → contrib/pgx/repo.go updateSystem(): for field := range input.Data { ... }
  → fmt.Sprintf(`"%s" = $%d`, field, ...)                   (naive interpolation)
```

This is worse than the ORDER BY bug in every dimension that matters: it is
on the **write** path (can corrupt/modify data, not just alter read
semantics), it applies to any `IsSystem` entity — meaning the framework's
own tables (`iam_users`, `iam_sessions`, etc.), not just business
entities — and beyond the pure injection (a key containing `"` breaking out
of its identifier), the unchecked-key pattern independently allowed a
**privilege-escalation** primitive: a patch body containing `"tenant_id"` or
`"id"` as an ordinary key would reach the SET clause with no error, letting
a caller attempt to reassign a record's tenant or primary key through a
normal field update.

**Fix:** `Repository.checkWritableField(field)` — checked against
`r.schema.FieldsByName`/`Fields` (the entity's own declared business fields
*only*), applied before both the injection-relevant quoting and before the
value is used at all. This is deliberately **narrower** than
`sqlbuild.NewAllowlist` (§6): the standard framework columns
(`id`, `tenant_id`, `created_at`, `updated_at`, `deleted_at`, `custom_fields`)
are legitimate to sort/group by (read context) but must never be
client-writable via a generic patch (write context) — they simply are not
members of `FieldsByName`, so excluding them required no special-casing,
just using the right source set for the question being asked.

**Verified red/green against real PostgreSQL**: the fix was temporarily
reverted, and `TestUpdate_MaliciousFieldName_RejectedNotInjected` and
`TestUpdate_AnyUndeclaredColumnName_Rejected` both failed (i.e., the pre-fix
code accepted the malicious/invalid field name with no error) — confirming
the vulnerability was real, not merely theoretical. Interestingly,
`TestUpdate_TenantIDInPatchBody_Rejected` **passed even against the
reverted code** — PostgreSQL's own RLS `WITH CHECK` clause (implicit,
matching the policy's `USING` expression, since none of `test_entity`'s
policies declare a separate one) independently rejected the resulting
cross-tenant row with "new row violates row-level security policy." This is
good news confirming genuine defense-in-depth, not a gap in the test — but
it does not extend to `"id"` (a primary-key column with no tenant-scoping
RLS involvement at all) or `"custom_fields"` (a JSONB column RLS has no
opinion about), both of which the reverted code would have accepted with no
resistance whatsoever. The fix was then restored and all 8
`write_injection_test.go` Update/BulkUpdate tests confirmed green.

## 10. sqlbuild.Allowlist decision

**Adopted (option A: adopt and integrate), for read contexts only.** See §6
for the mechanism and §9 for why a second, narrower check
(`checkWritableField`) exists for write contexts rather than stretching
`Allowlist` to cover both — they answer genuinely different questions
("visible/sortable" vs. "writable"), and conflating them would have meant
either weakening the write check (by including framework columns) or adding
a mode flag to `Allowlist` for a distinction that's really about which field
*set* to check against, not about the allowlist mechanism itself. No second
*competing* mechanism was introduced — both call sites end by quoting
through the same `sqlbuild.QuoteIdent`.

## 11. `Repository.WithTx` panic-safety analysis

Transaction lifecycle: `BeginTx` (acquires a physical connection from the
pool) → optional `set_tenant_context` → `fn(txCtx)` → `Commit` or `Rollback`.
Only the last step releases the connection back to the pool (pgxpool's `Tx`
wraps `Release` internally). Traced every failure point:

- **Panic before any DB call inside `fn`**: pre-fix, neither Commit nor
  Rollback ever ran — connection leaked. Fixed and tested
  (`TestWithTx_PanicBeforeAnyMutation_RollsBackAndReleasesConnection`).
- **Panic after a mutation inside `fn`**: same leak, plus the mutation was
  never explicitly rolled back (though an uncommitted transaction that's
  simply abandoned would eventually be rolled back by PostgreSQL when the
  connection itself is eventually closed/reset — the real bug is the
  connection never being returned to the Go-level pool in the meantime).
  Fixed and tested, confirming the mutation itself is genuinely gone, not
  just that a later call happens to work
  (`TestWithTx_PanicAfterMutation_RollsBackTheMutationToo`).
- **Panic inside a nested `WithTx` call** (the `existing.InTx()`
  short-circuit path, which does no transaction management of its own):
  confirmed the *outermost* `WithTx`'s defer still catches it, since Go
  panic unwinding passes through intermediate stack frames regardless of
  nesting depth (`TestWithTx_PanicInNestedCall_CaughtByOutermostRollback`).
- **Connection reuse after a panic**: proven directly, not inferred — every
  panic test above finishes by successfully running a completely unrelated
  tenant's `WithTx` call against the *same* `pool_max_conns=1` pool
  connection.
- **Tenant-context leak after a panic**: none of the post-panic
  verification queries in any test ever see the panicked transaction's
  tenant's data, confirming no residual `set_tenant_context` state survives
  (consistent with `set_config`'s transaction-local scope, which a Rollback
  correctly discards).

## 12. Panic-safety implementation

```go
committed := false
defer func() {
    if !committed {
        _ = pgxTx.Rollback(ctx)
    }
}()
...
if err := pgxTx.Commit(ctx); err != nil {
    return err
}
committed = true
return nil
```

The defer runs during panic unwinding regardless of whether it recovers
anything. Because it does **not** call `recover()`, the original panic
continues propagating to the caller normally after the defer completes —
this was a deliberate choice, not an oversight: `tenant.FromContext`'s panic
on a missing `TenantContext` exists specifically to surface a real caller
bug loudly and immediately, and silently downgrading that to a normal error
return inside `WithTx` would defeat the purpose of that design. Preserving
normal Go panic semantics while still guaranteeing cleanup is exactly what a
non-recovering defer provides for free — no explicit recover-and-re-panic
dance was needed.

## 13. Connection-pool evidence

Verified red *before* writing the permanent test suite: a bounded
`context.WithTimeout(3s)` diagnostic (not left in the permanent suite — a
genuine leak under `context.Background()` would hang the test process
indefinitely, exactly what happened once during this investigation and was
then corrected) against the reverted (pre-fix) code showed a second
tenant's `WithTx` call fail with `"begin transaction: context deadline
exceeded"` — direct proof the single pooled connection was never released,
not an inference from reading the code. Restored the fix and confirmed all
4 panic-safety tests plus the full `contrib/pgx` suite green afterward.

## 14. Tenant-security regression verification (Step 8 — reviewing all Phase 1 fixes together)

Reviewed the full set of Phase 1 production changes together for unintended
interactions, specifically re-confirming:

- **Tenant lifecycle enforcement does not depend solely on middleware**:
  `set_tenant_context()` itself (the database function, called from every
  `WithTx`, from `Login`, and from any direct repository caller) validates
  ACTIVE status — confirmed still true, untouched by this pass.
- **Session revocation cannot be defeated by stale Redis state**: the
  tombstone mechanism (`IsRevoked` checked before trusting a PG-recovered
  session) is unrelated to and unaffected by this pass's changes — re-ran
  `TestAuthService_ValidateToken_RevocationTombstone_BlocksResurrection` to
  confirm.
- **`checkWritableField`'s interaction with IAM's own `Update`/`BulkUpdate`
  callers**: `platform/iam/service.go` updates `last_login_at` (a real
  declared field on `iamUsersSchema()`) and revokes sessions via
  hardcoded `"revoked_at"` keys — both pass the new check unmodified. Ran
  the full `platform/iam`, `platform/notifications`, `platform/registry`,
  `platform/settings`, `platform/metadata`, and `api/*` suites after the
  write-path fix; all green, no interaction found.
- **`Repository.Aggregate`'s fix and `report.GenerateSQL`'s fix are
  independent** — different packages, different types, no shared code path
  other than both now validating against a compiled entity schema.

## 15. Full test matrix (this closure pass)

| File | New tests | Result |
|---|---|---|
| `contrib/pgx/filter_security_test.go` (+1 to existing) | `TestQuery_SortField_AdversarialPayloads_AllRejected` (18 subtests) | PASS |
| `contrib/pgx/write_injection_test.go` (new) | 12 | PASS |
| `contrib/pgx/withtx_panic_safety_test.go` (new) | 4 | PASS |
| `report/report_test.go` (+5 to existing 11) | 5 | PASS |

Plus the complete pre-existing suite (every package in the repository, `go
test ./...`) re-run clean after every change in this pass, and specifically
re-run for `contrib/pgx`, `contrib/redis`, `report`, `platform/iam`,
`platform/notifications`, `platform/registry`, `platform/settings`,
`platform/metadata`, `api/*` after the write-path fix to check for
interactions (§14).

## 16. Coverage

No coverage percentage was gamed or targeted — every new test asserts one of
two concrete behavioral properties: "malicious input → rejected, and the
underlying data is provably unmodified" or "panic → transaction rolled back,
connection provably reusable, no tenant-context leak." Two red/green cycles
were performed against real, temporarily-reverted code (§9, §13) rather than
trusting that a passing test against the fixed code alone proves the fix
did anything.

## 17. Race results

`go test -race` is **not supported on this sandbox** (android/arm64) — this
is a platform limitation, not something this pass could resolve; consistent
with the Phase 0 finding, race detection is enforced by CI's dedicated
`race` job (ubuntu-linux-amd64). No changes in this pass are believed to
introduce a data race — `WithTx`'s new `committed` flag is a plain local
`bool`, mutated only by the same goroutine that reads it (never shared
across goroutines), and every other change is either pure validation logic
or SQL-string construction with no new shared mutable state.

## 18. Files changed (this pass)

Modified: `contrib/pgx/repo.go` (`updateSystem`, `BulkUpdate`, `Aggregate`,
`WithTx`, new `checkWritableField`/`isValidAggregateFn` helpers),
`contrib/pgx/filter_security_test.go`, `report/report.go`, `report/report_test.go`,
`docs/04-multitenancy/RLS_SPEC.md`, `docs/14-api/API_CONVENTIONS.md`,
`docs/19-operations/RUNBOOK_DATABASE.md`, `tasks.md`.

Added: `contrib/pgx/write_injection_test.go`, `contrib/pgx/withtx_panic_safety_test.go`,
`PHASE1_SECURITY_CLOSURE_REPORT.md` (this file).

An untracked `Makefile` was noticed in the repository root during this
pass's `git status` review — it references paths (`internal/core`,
`internal/shared`, `framework/db/migrations`, an `awoctl` tool) that do not
match this repository's actual structure. It was not created by this pass,
was not modified, and is not included in any of the changes described here
or in the recommended commit below; flagging it for the user's own
attention since its origin is unclear.

## 19. Remaining vulnerabilities / known gaps (not fixed this pass, all logged in `tasks.md`)

1. `sqlbuild.Allowlist` still has no adoption for `Repository.Query`'s
   WHERE-clause building (`tasks.md` 1.13) — an authorization/exposure gap,
   not an injection (already correctly quoted); pre-existing, unrelated to
   this pass's fixes, deliberately not expanded into scope here.
2. `Repository.Aggregate`'s `GroupBy` multi-row scanning bug (§8/§9 above,
   `tasks.md` 1.15) — a correctness bug, not security; zero production
   callers.
3. `events/outbox/relay.go`'s missing tenant-context restoration
   (`tasks.md` 1.4) — unreachable today (no subscribers exist), logged for
   whoever implements the outbox for real.

## 20. Original P0 status (explicitly re-confirmed, not assumed)

- **Bulk import bypasses audit and validation** (`tasks.md` 1.3): confirmed
  still open, untouched by this pass. `ioport.Import` still routes through
  `BulkCreate`'s raw `pgx.Batch` path with zero pipeline/audit integration.
- **Workflow-outbox durability** (`tasks.md` 1.4): confirmed still open,
  untouched by this pass. `EntityService.startWorkflows` still calls
  Temporal directly and synchronously; no `workflow_outbox` table exists.

Neither was implemented, marked complete, or removed from `tasks.md` — both
remain mandatory roadmap items exactly as before this pass.

## 21. Acceptance matrix

| # | Criterion | Status |
|---|---|---|
| 1 | `Repository.Query` ORDER BY injection reproduced | PASS |
| 2 | Root cause documented | PASS |
| 3 | Arbitrary SQL syntax can no longer enter ORDER BY | PASS |
| 4 | Valid ordering behavior remains functional | PASS |
| 5 | Unknown fields are rejected | PASS |
| 6 | SQL expressions are rejected | PASS |
| 7 | Function calls are rejected | PASS |
| 8 | Subqueries are rejected | PASS |
| 9 | SQL comments are rejected | PASS |
| 10 | Malformed ordering syntax is rejected | PASS |
| 11 | Tenant isolation remains intact while ordering is processed | PASS |
| 12 | All other major dynamic-SQL surfaces were audited | PASS |
| 13 | No equivalent SQL injection remains | PASS — two equivalent-shape bugs found and fixed (`Repository.Aggregate`, `report.GenerateSQL`, both zero-caller landmines) plus one live, more severe finding found and fixed (`updateSystem`/`BulkUpdate` write-path) |
| 14 | `sqlbuild.Allowlist` has an explicit architectural disposition | PASS — adopted for read contexts; a narrower, separate check used for write contexts, both documented in §6/§9/§10 and in `RLS_SPEC.md` §10 |
| 15 | `Repository.WithTx` panic behavior is understood | PASS |
| 16 | Transaction cleanup after panic is safe | PASS |
| 17 | Connection reuse after panic is safe | PASS |
| 18 | Tenant context cannot leak after panic | PASS |
| 19 | PostgreSQL integration tests prove the security properties | PASS — every claim in this report is backed by a named, currently-passing test against real PostgreSQL, plus two explicit red/green revert cycles |
| 20 | Redis integration tests remain green | PASS |
| 21 | Full test suite passes | PASS — `go test ./...`, every package |
| 22 | `go vet` passes | PASS |
| 23 | `go build` passes | PASS |
| 24 | Race tests pass in supported CI | N/A locally (android/arm64 unsupported), deferred to CI's dedicated race job as established in Phase 0 |
| 25 | Documentation is accurate | PASS — `RLS_SPEC.md` §10/§11, `API_CONVENTIONS.md` §9, `RUNBOOK_DATABASE.md` updated to match verified, current behavior |
| 26 | `tasks.md` contains evidence | PASS — items 1.14–1.17 added with full evidence, cross-referenced to this report |
| 27 | Original bulk-import P0 remains tracked if unresolved | PASS — `tasks.md` 1.3 unchanged, confirmed still open |
| 28 | Original workflow-outbox P0 remains tracked if unresolved | PASS — `tasks.md` 1.4 unchanged, confirmed still open, plus a new sub-finding logged within it |

## 22. Security closure decision

**SECURITY CLOSED — see the ADDENDUM below for a required qualification found
during a subsequent forensic pass (§A10 has the final, complete decision;
read it before treating this section alone as the full picture).**

Both P0s (the live `ORDER BY` injection and the live, more severe
`updateSystem`/`BulkUpdate` write-path injection/privilege-escalation bug
found during this pass's own audit) are fixed and regression-tested against
real PostgreSQL, with explicit red/green verification against the
temporarily-reverted vulnerable code for both. The broader dynamic-SQL audit
found two further equivalent-shape bugs, both fixed as defense-in-depth
despite having zero production callers today, and confirmed no other
construction site in the codebase shares the same root cause. The
`Repository.WithTx` panic-safety gap is closed and proven under real
PostgreSQL with a real connection-leak reproduction before the fix. The two
pre-existing P0s explicitly out of this pass's scope (bulk-import audit
bypass, workflow-outbox durability) remain correctly tracked, unresolved,
and undisturbed in `tasks.md`. No new unresolved SQL-injection-class
vulnerability is known to exist in the codebase as of this report.

---

# ADDENDUM — Final Forensic Verification Pass

A second, deliberately adversarial pass over everything above, specifically
looking for anything the first pass missed, false-positive tests, and
untracked artifacts. This pass found one new, genuinely important
CRITICAL-severity finding (§A2 below) and corrected two tests that had been
silently passing against an inaccurate fixture. Nothing above this line was
reverted or redone — this addendum extends it.

## A1. Untracked `Makefile` investigation

An untracked `Makefile` sits at the repository root. Investigated via `git
log --all --oneline -- Makefile`, which revealed this repository's git
history contains two unrelated projects: 172 commits (commit `562a7db`
through `e5af9c5`) belonging to an entirely different, earlier personal
project (a flight/airline-booking system built with sqlc, gRPC, buf, goa —
commit messages like "ailrne company impilmented", "moved from
Database/sql to pgx", "monte carlo and inspirational booking schema"), then
commit `e5af9c5` ("deleted") removed that project's tracked files, and the
very next commit, `6ee6557` ("moved awo framework here"), is where Awo
itself begins.

The untracked `Makefile` is **not byte-identical** to that old project's own
tracked Makefile (compared directly via `git show e5af9c5^:Makefile`) — it
has been partially adapted: rebranded "AWO ERP" in its header, given
`DB_NAME ?= awo` (matching this project's actual conventions, not the old
project's `flight` database), but still references paths that do not exist
in this repository at all (`internal/core`, `internal/shared`,
`internal/adapters`, `framework/db/migrations`) and a `REQUIRED_TOOLS` list
including `awoctl` (plausibly this project's own CLI, `cmd/awo/`) alongside
tools from the old project (`sqlc`, `mockgen`). It defines only 3 targets —
`help`, `check-tools`, `status` — and stops there; it does not yet implement
`infra-up`, `migrate-up`, `run`, or `db-test-setup`, all of which
`docs/20-devops/LOCAL_DEVELOPMENT.md` already documents as the expected
local-development workflow (`docs/20-devops/LOCAL_DEVELOPMENT.md:100-141`).
No credentials or dangerous commands were found — `DB_USER ?= admin` /
`DB_PSSWD ?= admin` are the same well-known local-dev placeholder values
used throughout this session's own test fixtures, not a real secret.

**Conclusion: legitimate, intended, but unfinished Awo infrastructure (not
an unrelated artifact)** — an incomplete draft of the Makefile the project's
own documentation already assumes exists, built starting from the old
project's Makefile as a structural template. Not deleted, not modified, not
staged. The user should decide whether to finish it, discard it, or leave it
as a local scratch file — that decision was not made here.

## A2. CRITICAL, newly-discovered finding: `ScopeOrganization` RLS provides no actual isolation

While re-verifying the organisation-RLS tests from item 1.9 with the
*complete*, accurate DDL shape `generator.go` actually emits (the original
test fixture only created the `org_isolation` policy — `org_id =
current_org_id()` — and omitted the `tenant_isolation` policy every
non-System-scope entity, `ScopeOrganization` included, also always
receives), both previously-"passing" isolation tests failed. Root cause,
confirmed directly against real PostgreSQL with a minimal reproduction
outside any test framework:

```sql
CREATE POLICY t_tenant_isolation ON t USING (tenant_id = current_tenant_id());
CREATE POLICY t_org_isolation    ON t USING (org_id = current_org_id());
```

Both `CREATE POLICY` statements above are **PERMISSIVE** (the default —
neither is declared `AS RESTRICTIVE`). PostgreSQL combines multiple
permissive policies applicable to the same command with **OR**, not AND: a
row is visible/writable if it satisfies **at least one** applicable
permissive policy, not all of them. Reproduced directly via `psql` (non-superuser
`awo_app` role, real RLS-forced table):

- **INSERT**: a row with the caller's own `tenant_id` but an `org_id`
  belonging to an organisation the caller has no context for **succeeds**
  (`INSERT 0 1`) — `tenant_isolation`'s check alone is sufficient.
- **UPDATE**: reassigning an existing row's `org_id` to a foreign
  organisation, while leaving `tenant_id` correct, **succeeds** (`UPDATE
  1`) — same reason.
- **SELECT**: with both policies present, a caller in Org A's context sees
  **both** Org A's and Org B's rows within the same tenant — `org_isolation`
  is not merely bypassable for writes, it provides **zero** read isolation
  either, because `tenant_isolation`'s `USING` clause alone already
  satisfies the OR-combined check for every row in the tenant.

In short: **for any entity using `ScopeOrganization`/`ScopeOrganizationTree`,
the org-scoping RLS policy is currently a complete no-op** the moment the
(always-present) tenant-isolation policy is also in effect — which is
always, for every real entity. This is not a narrow edge case; it is the
normal, only-possible configuration for this scope type as currently
generated.

**Not closed by `Repository.checkWritableField`** (the fix for
`updateSystem`/`BulkUpdate`'s injection, §9 above): that check excludes
fields absent from the entity's `FieldsByName`/`Fields`, but `org_id` is
**not** an implicit standard column the way `tenant_id` is —
`generator.go`'s `generateEntitySQL` hardcodes `id`, `tenant_id`,
`created_at`, `updated_at`, `deleted_at` unconditionally, but has no
equivalent unconditional `org_id UUID` column for `ScopeOrganization`/
`ScopeOrganizationTree` entities at all. An entity author would have to
declare `org_id` as an ordinary field themselves for the column to exist —
meaning `checkWritableField` would treat it exactly like any other
business field and allow it through a generic patch, compounding the RLS
gap with a write-path gap too, unless the entity author separately marks it
`Immutable: true` (which nothing currently enforces or even suggests).

**Exploitability: zero today.** Confirmed via exhaustive source search that
**no entity in this codebase declares `ScopeOrganization` or
`ScopeOrganizationTree`** — the only references are the framework's own
generator/RLS-helper code, never a real `def.EntityDefinition`. Consistent
with `platform/organization.OrganizationService` being 100% stubbed. This is
a landmine, not a live incident, in the same category as the
`Repository.Aggregate`/`report.GenerateSQL` findings (§8) — but far more
severe in *kind*, since it defeats an entire RLS scope's isolation
guarantee outright rather than lacking input validation on top of a sound
boundary.

**Recorded, not fixed, in this pass** — deliberately: the correct fix is a
genuine RLS-architecture decision (candidates: declare `org_isolation` `AS
RESTRICTIVE` so it ANDs with the permissive `tenant_isolation` policy;
combine both conditions into a single policy expression, `USING (tenant_id
= current_tenant_id() AND org_id = current_org_id())`; or make `org_id` a
generator-managed standard column excluded from `FieldsByName` the same way
`tenant_id` is) and deserves its own dedicated review, not a rushed change
inside an already-large verification pass. Logged as a new `tasks.md` item
(1.18) with CRITICAL severity and an explicit block on any real
`ScopeOrganization` usage or Phase 3 (organisation hierarchy) work until
resolved.

**Regression tests** (`contrib/pgx/organization_security_test.go`):
`TestOrganizationRLS_SiblingOrgsIsolated` and
`TestOrganizationRLS_NoOrgContext_SeesNothing` were corrected to assert the
real, current (broken) behavior against the accurate two-policy DDL, with
extensive comments explaining the correction and stating that a future fix
must flip these assertions back, not delete them. A new test,
`TestOrganizationRLS_KnownGap_MultiplePermissivePoliciesAllowOrgReassignment`,
directly proves the INSERT and UPDATE cross-organisation writes described
above. All three pass (i.e., correctly document current reality).

## A3. Full write-path trace (Part 2 of this pass)

Traced `PATCH /api/v1/entities/:entity/:id` end-to-end: routes are
registered once per **compiled** entity (`for _, es := range
schema.Entities` in `api/router/router.go`, each bound to its own
`*compiler.EntitySchema` and `contrib.NewRepository` instance at startup) —
there is no runtime table-name resolution from a client-supplied string, so
an attacker cannot target an unregistered table via the URL. Confirmed both
production entrypoints (`cmd/server/main.go`, `cmd/awo/serve_impl.go`) wire
real, non-nil `IAM`/`Tenants`/`Authz` into `RegisterOptions`, so
`TenantResolver` → `RequireAuth` → per-method RBAC are all genuinely active
in front of every entity route (they are each individually conditional on
these options being non-nil in `router.Register`'s implementation — worth
knowing if a future caller ever constructs `RegisterOptions` incompletely,
but not a defect today). `Repository.checkWritableField` lives at the
lowest common layer (inside `Repository.Update`/`BulkUpdate` themselves),
so every caller — the generated CRUD handler, any future custom `ActionDef`
handler that happens to call `Repository.Update`, any internal service —
automatically inherits the protection; there is no parallel code path that
writes to a system entity's typed columns while bypassing the repository.
The one custom `ActionDef.HandlerFunc` that exists in the codebase today
(`platform/notification`'s `mark_read`) is a no-op stub that never touches
the request body or the repository, so this is a described trust boundary
for future module authors, not a currently-realized gap. The audit-write
path (`RunAuditRecord` → `audit.pg_writer`/`transactional_writer`) stores
`BeforeData`/`AfterData` as JSON-marshaled blobs, never as dynamic SQL
identifiers — confirmed no injection surface there either.

## A4. Protected/system field taxonomy (Part 3 of this pass)

Determined from actual generator/compiler source, not assumed:

| Field | Mechanism | Client-writable via generic PATCH? |
|---|---|---|
| `id`, `tenant_id`, `created_at`, `updated_at` | Hardcoded standard columns in `generateEntitySQL`; never added to `es.Fields`/`FieldsByName` | No — excluded by `checkWritableField` structurally (proven by test) |
| `deleted_at` | Same as above — emitted unconditionally, but no repository method currently implements soft-delete against it | No — same exclusion (proven by test), though currently inert either way |
| `custom_fields` | System-managed JSONB column, merged only via `UpdateInput.CustomFields`'s dedicated path | No — excluded (proven by test); a `custom_fields` key inside `Data` is rejected, not silently accepted |
| `org_id` (`ScopeOrganization`/`ScopeOrganizationTree`) | **Not** an implicit standard column — must be declared as an ordinary field by the entity author | **Yes**, unless the author separately marks it `Immutable: true` — nothing currently prompts or enforces that. Moot today (§A2 — no entity uses this scope), but a real gap the moment one does |
| `created_by`/`updated_by`/`deleted_by`/`version`/`revision` | Do not exist as framework concepts anywhere (`def`/`compiler`/`generator`) | N/A — would need to be declared as ordinary fields, subject to the same `Immutable: true` responsibility as any other business field |
| `status` / lifecycle fields | Ordinary `FieldTypeSelect` fields with a DB `CHECK` constraint on declared `Options`; no built-in transition/state-machine validation | Yes, by design — transition rules are the entity author's responsibility via `BeforeUpdate`/`BeforeSave` hooks, the framework's existing and correct extension point |
| Ordinary declared business fields | `es.Fields`/`FieldsByName`, optionally `Immutable: true` | Yes, unless `Immutable: true` (checked by `runtime/pipeline.go`'s `RunBeforeUpdate`, a layer above and independent of `checkWritableField`) |

New regression tests added: `TestUpdate_UpdatedAtInPatchBody_Rejected`,
`TestUpdate_DeletedAtInPatchBody_Rejected` (explicit, named coverage for
two more fields from the requested checklist, beyond the
`tenant_id`/`id`/`custom_fields`/generic-undeclared-column tests already in
place).

## A5. Tenant-boundary regression (Part 4 of this pass)

Two new tests added, both against real PostgreSQL with the non-superuser
`awo_app` role: `TestUpdate_CreateInputCannotSmuggleTenantID` (proves
`Create` is structurally immune to the class of bug `Update` had —
`createSystem` looks up values from its own known field list rather than
iterating the client map's keys, so a `"tenant_id"` key in `CreateInput.Data`
is silently ignored, never written) and
`TestUpdate_CrossTenantByID_RejectedAsNotFound` (Tenant B cannot `Update` a
record it can name the exact ID of but that belongs to Tenant A — RLS
filters the `UPDATE ... WHERE "id" = $N` to zero affected rows, surfaced as
`*runtime.NotFoundError`; confirmed via a direct superuser query afterward
that Tenant A's row is byte-for-byte unmodified). Combined with the
already-existing `TestFilterSecurity_BulkUpdate_CannotCrossTenant` and
`TestFilterSecurity_Delete_CannotCrossTenant`, and the corrected §A2
findings, tenant-level RLS remains fully sound and RLS remains the final
enforcement boundary for **tenant** isolation specifically — the boundary
proven broken in this addendum is the separate, currently-unused
**organisation** dimension.

## A6. WithTx rollback-error safety (Part 7 of this pass) — confirmed from pgx's own source, not a live simulation

The one remaining unverified item from the original panic-safety analysis —
"what if `Rollback` itself fails?" — was answered definitively by reading
`pgxpool`'s own source rather than attempting a fragile live simulation:

```go
// github.com/jackc/pgx/v5/pgxpool/tx.go
func (tx *Tx) Rollback(ctx context.Context) error {
	err := tx.t.Rollback(ctx)
	if tx.c != nil {
		tx.c.Release()   // unconditional — runs even if the line above returned an error
		tx.c = nil
	}
	return err
}
```

And `pgxpool.Conn.Release()` itself: if the connection is closed, busy, or
not in the idle transaction state (`TxStatus() != 'I'` — exactly the state
a failed Rollback would leave it in), `Release()` calls `res.Destroy()` and
triggers a pool health check, rather than returning a potentially-corrupted
connection for reuse. This gives an unconditional, library-level guarantee
covering the scenario a live test could only ever probabilistically
approximate: whether `Rollback`'s own wire operation succeeds or fails, the
connection is either safely returned to the pool or safely destroyed and
replaced — never left in permanent limbo, and never handed to another
tenant in an unknown state.

## A7. Re-verification summary (Parts 8–11 of this pass)

Full targeted re-run of every previously-fixed Phase 1 security area
(pgx/v5 error compatibility, all five tenant-lifecycle statuses,
`set_tenant_context`, RLS isolation, connection-pool tenant isolation,
session-revocation tombstones, `/auth/logout`, `/auth/me`, platform-admin
policy tests) plus the full `go test ./...` (67 packages, zero failures)
after every change in this addendum — all green, no regressions introduced
by any fix in the original report or this addendum. `gofmt`/`go vet`/`go
build` clean. `go test -race` remains unsupported on this sandbox
(android/arm64) — not falsely claimed as run. Dependabot re-checked: same 14
open alerts as the original triage, none newly relevant to the security
boundary, PostgreSQL/pgx, auth/session handling, HTTP parsing, or SQL
generation — no action taken, consistent with the original triage.

## A8. Original P0s — re-confirmed via fresh source evidence

- **Bulk import bypasses audit/validation**: `ioport/importer.go` still
  calls `repo.BulkCreate` directly at 3 call sites, zero pipeline
  integration. Confirmed unresolved.
- **Workflow-outbox durability**: no `workflow_outbox` file exists anywhere
  in the repository (`find . -iname '*workflow_outbox*'` → empty);
  `EntityService.startWorkflows` (`api/service/entity.go`) is unchanged.
  Confirmed unresolved.

Neither was touched, implemented, or marked complete in this addendum.

## A9. Updated acceptance — what changed since the original report

Every criterion in the original acceptance matrix (§21) still holds exactly
as stated **for the ORDER BY / write-path-injection / WithTx scope this
report's main body targeted**. This addendum does not overturn any of those
28 PASS lines. It adds one finding **outside that original scope** (RLS
policy composition for an entirely unused scope type, not an untrusted-
identifier injection) that must be tracked and fixed before `ScopeOrganization`
is ever used in production, and corrects two tests whose original fixture
did not match the DDL `generator.go` actually emits.

## A10. Final security decision (re-affirmed with the new finding disclosed)

**SECURITY CLOSED** for the scope this report and its addendum actually
targeted: the `ORDER BY` injection, the `updateSystem`/`BulkUpdate`
write-path injection and tenant/id-reassignment bug, the `Aggregate`/
`report.GenerateSQL` equivalent-shape landmines, and the `WithTx`
panic-safety gap are all fixed, regression-tested against real PostgreSQL
(with explicit red/green verification for both P0s), and re-verified
without regression across every previously-fixed Phase 1 security area and
the full test suite.

This is **explicitly qualified**, not unconditional: the organisation-scope
RLS composition bug found in this addendum (§A2) is a real, CRITICAL-severity
gap with zero production exploitability today (no entity uses
`ScopeOrganization` anywhere in the codebase) that must be resolved —
tracked as `tasks.md` 1.18 — before Phase 3 (organisation hierarchy) begins
or before any entity ever declares `ScopeOrganization`/`ScopeOrganizationTree`
in production. It does not reopen this report's own closure decision because
it sits outside the scope that decision covers (an unused capability's RLS
design, not an untrusted-identifier injection reachable today), but it must
not be lost track of, and Phase 3 must not begin without it being resolved
first.

The two pre-existing P0s (bulk-import audit bypass, workflow-outbox
durability) remain tracked, confirmed unresolved via fresh source evidence,
and untouched by this addendum, exactly as required.
