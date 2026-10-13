# Chapter 46 — Best Practices

> **Volume:** X — Reference
> **Phase:** 4
> **Audience:** All Engineers
> **Prerequisites:** All previous chapters

---

## Table of Contents

- [46.1 Multi-Tenancy Best Practices](#461-multi-tenancy-best-practices)
- [46.2 SDUI Authoring Best Practices](#462-sdui-authoring-best-practices)
- [46.3 API Design Best Practices](#463-api-design-best-practices)
- [46.4 Database Best Practices](#464-database-best-practices)
- [46.5 Security Best Practices](#465-security-best-practices)
- [46.6 Observability Best Practices](#466-observability-best-practices)
- [46.7 Testing Best Practices](#467-testing-best-practices)
- [46.8 Performance Best Practices](#468-performance-best-practices)

---

## 46.1 Multi-Tenancy Best Practices

### Set tenant context before every database operation

Every database operation in the application tier sets `app.tenant_id` before executing queries. This is not optional — it is the enforcement point for RLS. Use the `WithTenantContext` wrapper (Chapter 28) everywhere, not just in handlers.

```go
// Good
db.WithTenantContext(ctx, tenantID, func(ctx context.Context, q *Queries) error {
    return q.ListInvoices(ctx, ListInvoicesParams{...})
})

// Bad — bypasses RLS
q.ListInvoices(ctx, ListInvoicesParams{TenantID: tenantID}) // relies on manual filter
```

### Never trust tenant ID from the request body

The authenticated tenant comes from the JWT session, not from a `tenant_id` field in the request body. A request body field with a tenant ID is ignored. An engineer who reads `tenantID := req.TenantID` instead of `tenantID := auth.TenantFromContext(ctx)` creates a privilege escalation vulnerability.

### Test tenant isolation, always

Every integration test that touches data creates two tenants and verifies that tenant A cannot read tenant B's data. This is the minimum standard. It is not optional. Tests that create only one tenant are not testing isolation.

---

## 46.2 SDUI Authoring Best Practices

### One surface, one purpose

A surface definition covers one functional screen. It does not embed another surface's logic. Shared components (a header, a footer) are layout components included by multiple surfaces, not shared surfaces.

### Label keys, not hardcoded strings

Every human-readable string in a surface definition is a `label_key`, never a raw string value. A raw string in a surface definition cannot be translated, cannot be overridden by a tenant, and cannot be validated by the linter.

```json
// Good
{ "type": "action.button", "label_key": "invoice.create.submit" }

// Bad
{ "type": "action.button", "label": "Create Invoice" }
```

### Keep binding expressions simple

Binding expressions (`$row.amount`, `$session.locale`) are intended for data binding, not business logic. A binding expression that does arithmetic, string manipulation, or conditional logic beyond a simple null coalesce belongs in the surface's `computed_fields`, not inline in a binding.

### Compile locally before committing

Run `awoerp-lint surfaces/` before every commit that touches a surface definition. CI catches the same errors, but catching them locally is faster and produces better error messages.

---

## 46.3 API Design Best Practices

### Return stable IDs, not positional references

Every resource in the API has a UUID identifier. Lists return arrays of objects with IDs, not indexed arrays where position carries meaning. A client that stores `result[0]` will break when the sort order changes.

### Paginate every list endpoint

No list endpoint returns an unbounded result set. Every list endpoint has `page` and `page_size` parameters (or cursor-based equivalents) and returns `meta.total`, `meta.page`, `meta.page_size`. A list endpoint that returns up to 10,000 rows is an eventual performance outage.

### Use consistent error shapes

Every error response from the API has the same shape (Chapter 25). A client that handles errors once handles all errors. A client that must special-case each endpoint for error handling is a maintenance problem.

```json
{
  "error": {
    "code": "validation_failed",
    "message": "Invoice amount must be positive",
    "details": [
      { "field": "amount", "code": "must_be_positive" }
    ]
  }
}
```

### Mutations are idempotent where possible

POST endpoints that create resources accept a client-generated `idempotency_key`. If the same key is submitted twice, the second call returns the result of the first call without creating a duplicate resource. This makes retries safe.

---

## 46.4 Database Best Practices

### Use sqlc for all queries

All SQL queries are written in `.sql` files, compiled by `sqlc`, and called through the generated Go functions. No raw SQL strings in application code. No `fmt.Sprintf` to build SQL. No string concatenation anywhere near a query.

### Index foreign keys and filter columns

Every foreign key column has an index. Every column that appears in a `WHERE` clause in a production query has an index. Missing indexes on large tables produce linear scans that are invisible in development (small data) and catastrophic in production (large data).

### Prefer narrow selects over `SELECT *`

Queries select the columns they need, not `SELECT *`. This reduces data transfer, avoids surprises when columns are added, and makes the generated Go types explicit about what each query returns.

### Benchmark before adding composite indexes

A composite index on `(tenant_id, status, created_at)` is faster for the query it was built for, but it adds write overhead and disk usage. Benchmark the query with `EXPLAIN ANALYZE` before adding composite indexes. The index must pay for itself.

---

## 46.5 Security Best Practices

### Check permissions before reading, not just before writing

Authorization checks gate both reads and writes. A viewer who cannot see an invoice's amount also cannot call `GET /invoices/{id}` and read `amount` from the response. Field-level authorization is enforced in the surface compiler (Chapter 14) and in API response serialization.

### Rotate secrets regularly

Database credentials, JWT signing keys, and API keys are rotated quarterly. Rotation is automated. The rotation procedure is documented and tested — not left as a manual step that someone remembers (or forgets) to do.

### Validate all external input at the boundary

Input validation happens at the API boundary, before the input reaches any business logic. The Goa DSL declares types and constraints for every request field. The OpenAPI validation middleware enforces them. Business logic never validates raw strings.

### Log security decisions, not just outcomes

When a permission check fails, the log line records what was checked, for whom, against what resource, and what the outcome was. A log that says `"authorization failed"` is useless for debugging. A log that says `"FGA check: user:alice relation:approve object:invoice:inv-001 result:false"` is useful.

---

## 46.6 Observability Best Practices

### Every request has a request ID

The `request_id` (a ULID) is generated at the edge and carried through every log line, every trace span, and every outbound call for the duration of the request. It is the primary key for debugging a specific request. Without it, correlating logs across services is guesswork.

### Log at the point of decision

Log lines are written when a decision is made — an authorization check, a cache hit/miss, a workflow enqueue. They are not written at function entry/exit. An application that logs "entering GetInvoice" and "leaving GetInvoice" has 2× the log volume and half the useful information of one that logs "cache miss: invoice inv-001, fetching from DB".

### Alerts should be actionable

Every alert has a runbook. A runbook describes: what the symptom looks like, the first three things to check, the steps to mitigate, and when to escalate. An alert without a runbook is noise that engineers learn to ignore.

### Use error budget burn rate alerts, not point-in-time thresholds

A threshold alert on "error rate > 1%" fires and resolves and fires again. A burn rate alert on "consuming error budget 14× faster than sustainable" fires once and stays firing until the underlying problem is fixed. Burn rate alerts have fewer false positives and better capture the severity of an outage.

---

## 46.7 Testing Best Practices

### Test behavior, not implementation

Tests assert that a function produces the correct output for a given input. They do not assert that the function called a specific internal method in a specific order. Tests that verify call sequences break when the implementation is refactored, even if the behavior is unchanged.

### Use table-driven tests for edge cases

Go's table-driven test pattern (Chapter 41.2) makes it easy to add new edge cases without writing new test functions. A test that covers the happy path plus three edge cases in one table is better than four separate test functions.

### Integration tests own their state

Every integration test creates the data it needs and cleans it up with `t.Cleanup`. Tests never depend on data created by a previous test or left over from a previous run. Tests that share state are fragile and produce unpredictable failures.

### The test pyramid is a guide, not a rule

The suggested counts (3,000 unit / 800 integration / 100 E2E) reflect the typical distribution for AwoERP. If a particular feature is better covered by integration tests than unit tests, write integration tests. The pyramid is a heuristic about cost and speed, not a quota.

---

## 46.8 Performance Best Practices

### Cache compiled surface definitions

The surface compiler is the most expensive operation in the hot path. Compiled surface definitions are cached in Redis with a TTL tied to the surface definition version. A cache hit is ~1ms; a compile is ~50ms. At scale, the difference is significant.

### Avoid N+1 queries in list endpoints

A list endpoint that fetches 25 invoices and then fetches each invoice's vendor in a separate query is 26 database round trips. Use joins, `sqlc`'s `many` query type, or batching. Every list endpoint is reviewed for N+1 patterns before shipping.

### Use read replicas for reporting queries

Queries that aggregate large datasets (reports, exports) run against a read replica. They do not compete with transactional queries on the primary. A report query that takes 10 seconds is a 10-second table lock on the primary if it runs there.

### Profile before optimizing

A performance problem has a root cause. The root cause is identified with profiling (pprof for Go, Xcode Instruments for iOS, Android Studio Profiler for Android), not by intuition. Optimization without measurement frequently optimizes the wrong thing.

---

*End of Chapter 46*

**Previous:** [Chapter 45 — Governance](../vol-09-operations/45-governance.md)
**Next:** [Chapter 47 — Anti-Patterns](./47-anti-patterns.md)
