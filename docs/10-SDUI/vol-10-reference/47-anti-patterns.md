# Chapter 47 — Anti-Patterns

> **Volume:** X — Reference
> **Phase:** 4
> **Audience:** All Engineers
> **Prerequisites:** All previous chapters

---

## Table of Contents

- [47.1 Multi-Tenancy Anti-Patterns](#471-multi-tenancy-anti-patterns)
- [47.2 SDUI Anti-Patterns](#472-sdui-anti-patterns)
- [47.3 API Anti-Patterns](#473-api-anti-patterns)
- [47.4 Database Anti-Patterns](#474-database-anti-patterns)
- [47.5 Authorization Anti-Patterns](#475-authorization-anti-patterns)
- [47.6 Observability Anti-Patterns](#476-observability-anti-patterns)
- [47.7 Testing Anti-Patterns](#477-testing-anti-patterns)
- [47.8 Workflow Anti-Patterns](#478-workflow-anti-patterns)

---

## 47.1 Multi-Tenancy Anti-Patterns

### Anti-Pattern: Manual WHERE clauses for tenant isolation

```go
// Bad — relies on the developer remembering to add tenant_id
rows, err := db.QueryContext(ctx,
    "SELECT * FROM invoices WHERE tenant_id = $1 AND status = $2",
    tenantID, status,
)
```

The danger is not this query — it is the next query written by someone who is in a hurry and forgets the `AND tenant_id = $1`. RLS is the enforcement mechanism precisely because it cannot be forgotten. Use `WithTenantContext` (Chapter 28) and let the database enforce isolation.

---

### Anti-Pattern: Superuser connection in the application

The application tier connects to PostgreSQL as a role that bypasses RLS (`BYPASSRLS`). This removes the isolation guarantee entirely. The application role must respect RLS. Superuser access is for migrations only.

---

### Anti-Pattern: Caching cross-tenant

```go
// Bad — the cache key does not include tenant_id
cacheKey := fmt.Sprintf("invoice:%s", invoiceID)
```

A cache miss on tenant A's invoice fetches it from the database. The result is stored under `invoice:inv-001`. Tenant B requests `inv-001`. Cache hit. Tenant B reads tenant A's data.

```go
// Good
cacheKey := fmt.Sprintf("invoice:%s:%s", tenantID, invoiceID)
```

Every cache key includes the tenant ID. No exceptions.

---

## 47.2 SDUI Anti-Patterns

### Anti-Pattern: Business logic in binding expressions

```json
// Bad — arithmetic and conditional logic in a binding
{
  "type": "display.text",
  "value_binding": "$row.amount * (1 + $row.tax_rate) > 100000 ? 'large' : 'normal'"
}
```

Binding expressions are for data binding. Business logic belongs in `computed_fields`, in the surface's data source definition, or in the backend. Binding expressions with logic are untestable, unreadable, and not validated by the schema.

---

### Anti-Pattern: Hardcoded locale strings

```json
// Bad
{ "type": "display.text", "value": "Invoice" }

// Good
{ "type": "display.text", "label_key": "invoice.noun" }
```

A hardcoded string in a surface definition cannot be translated, cannot be overridden by a tenant, and is invisible to the i18n linter. Every user-visible string is a `label_key`.

---

### Anti-Pattern: Surfaces that encode roles in their ID

```
// Bad surface IDs
surfaces/invoice-list-admin.json
surfaces/invoice-list-viewer.json
surfaces/invoice-list-approver.json

// Good — one surface, compiled per role
surfaces/invoice-list.json
```

The surface compiler compiles the same surface definition for each role, applying field-level and action-level visibility rules. A surface definition that is duplicated per role means that every change must be made in N places. Roles are a compilation concern, not an authoring concern.

---

## 47.3 API Anti-Patterns

### Anti-Pattern: Returning 200 for errors

```json
// Bad — HTTP 200 with an error payload
HTTP/1.1 200 OK
{ "success": false, "error": "Invoice not found" }
```

HTTP status codes carry meaning. A 404 is a not-found. A 403 is a forbidden. A 422 is a validation error. Clients (and monitoring systems) use status codes to route responses. A 200 with `success: false` means every client must parse the body before knowing if the request succeeded. Use the correct status code.

---

### Anti-Pattern: Unbounded list endpoints

```go
// Bad — no pagination; returns all invoices in the database for this tenant
func ListInvoices(ctx context.Context) ([]Invoice, error) {
    return q.ListAllInvoices(ctx, tenantID)
}
```

A tenant with 500,000 invoices makes this endpoint return 500,000 rows. Every call. Paginate all list endpoints. Default `page_size` of 25; maximum of 100.

---

### Anti-Pattern: Synchronous long-running operations

```go
// Bad — blocks until the export is complete; may timeout
func ExportInvoices(w http.ResponseWriter, r *http.Request) {
    data := generateExport(r.Context(), tenantID) // may take 30+ seconds
    writeCSV(w, data)
}
```

Operations that may take more than a second are queued as Temporal workflows and return a job ID immediately. The client polls for completion or receives an SSE notification. A synchronous 30-second HTTP request will be killed by the load balancer's timeout before it completes.

---

## 47.4 Database Anti-Patterns

### Anti-Pattern: Migrations that lock tables

```sql
-- Bad — acquires ACCESS EXCLUSIVE lock; blocks all reads and writes
ALTER TABLE invoices ADD COLUMN tax_rate DECIMAL(5,2) NOT NULL DEFAULT 0;
```

On a table with millions of rows, `ADD COLUMN NOT NULL DEFAULT` rewrites every row and holds an exclusive lock while doing so. Every query against that table blocks. Follow the zero-downtime migration patterns in Chapter 44.

---

### Anti-Pattern: Raw SQL strings

```go
// Bad — SQL injection waiting to happen; not type-safe; not reviewable
query := fmt.Sprintf(
    "SELECT * FROM invoices WHERE status = '%s'",
    req.Status,
)
rows, err := db.QueryContext(ctx, query)
```

All SQL queries live in `.sql` files and are compiled by `sqlc`. No raw SQL strings. No string formatting of queries. No exceptions.

---

### Anti-Pattern: SELECT * in production queries

```go
// Bad — fetches all columns including large JSONB fields
rows, err := q.GetAllInvoiceColumns(ctx, invoiceID)
```

`SELECT *` returns all columns including large ones (JSONB custom fields, attachment metadata). The query transfers unnecessary data and breaks silently when columns are added or removed. Queries select the columns they need.

---

### Anti-Pattern: Transactions wrapping non-transactional operations

```go
// Bad — HTTP call inside a database transaction
tx, _ := db.BeginTx(ctx, nil)
invoice, _ := q.WithTx(tx).CreateInvoice(ctx, params)
notifyExternalService(invoice.ID) // HTTP call — holds the transaction open
tx.Commit()
```

Transactions should be as short as possible. An HTTP call inside a transaction holds database locks while waiting for the network. If the external service is slow, the transaction holds locks for seconds. External calls happen after the transaction commits.

---

## 47.5 Authorization Anti-Patterns

### Anti-Pattern: Authorization checked only on write

```go
// Bad — reads are ungated; only writes check permissions
func GetInvoice(ctx context.Context, id string) (*Invoice, error) {
    // No permission check — anyone can read any invoice
    return q.GetInvoice(ctx, id)
}

func ApproveInvoice(ctx context.Context, id string) error {
    if !authz.Can(ctx, "approve", "invoice", id) {
        return ErrForbidden
    }
    return q.ApproveInvoice(ctx, id)
}
```

Authorization gates both reads and writes. A viewer who cannot approve an invoice also cannot see the `amount` field on the invoice detail page. Authorization is not only about what actions can be taken; it is also about what data can be seen.

---

### Anti-Pattern: Hard-coded role checks

```go
// Bad — role-based logic hard-coded in Go
if user.Role == "admin" || user.Role == "approver" {
    // allow
}
```

Role-to-permission mapping lives in the OpenFGA authorization model (Chapter 13). Application code calls `authz.Can(ctx, relation, objectType, objectID)`. It does not inspect the user's role directly. Hard-coded role checks bypass the authorization model and drift out of sync with it.

---

## 47.6 Observability Anti-Patterns

### Anti-Pattern: Logging PII

```go
// Bad — logs personal information
log.Info("user logged in", "email", user.Email, "ip", r.RemoteAddr)
```

Logs are stored for 90 days and accessible to all engineers. PII in logs creates a data retention and access control problem. Log identifiers (user_id, tenant_id) rather than personal attributes (email, name, IP address).

---

### Anti-Pattern: Using ERROR for expected conditions

```go
// Bad — 404s are not server errors
if err == sql.ErrNoRows {
    log.Error("invoice not found", "invoice_id", id)
    return ErrNotFound
}
```

`ERROR` level is reserved for 5xx conditions — unexpected failures that indicate a bug or infrastructure problem. A 404 (resource not found) and a 403 (forbidden) are expected conditions that are logged at `INFO`. An alert on ERROR rate is useless if ERROR fires for every 404.

---

### Anti-Pattern: Traces without context propagation

```go
// Bad — creates a root span; not connected to the incoming request's trace
ctx, span := tracer.Start(context.Background(), "process-invoice")
```

All spans are created from the incoming request's context, which carries the trace ID. A span created from `context.Background()` creates an unconnected root span that cannot be correlated with the request that triggered it.

---

## 47.7 Testing Anti-Patterns

### Anti-Pattern: Tests that depend on each other

```go
// Bad — TestApproveInvoice assumes TestCreateInvoice ran first
func TestApproveInvoice(t *testing.T) {
    // relies on invoice "inv-001" existing from a previous test
    err := approveInvoice(ctx, "inv-001")
    assert.NoError(t, err)
}
```

Each test creates its own data and cleans it up. Tests run in parallel and in any order. A test that depends on another test's side effects is fragile and produces unpredictable failures.

---

### Anti-Pattern: Mocking the database

```go
// Bad — tests the mock, not the SQL
mockDB.On("GetInvoice", invoiceID).Return(fakeInvoice, nil)
```

Database queries are tested against a real PostgreSQL instance (in a Docker container via `dockertest`). A mock database test verifies that the Go code passes arguments to the mock correctly — it does not verify that the SQL query is correct, that indexes are used, or that RLS policies apply. Use the real database.

---

### Anti-Pattern: Tests with production data

No production data in tests. Test data is synthetic and created specifically for the test. Using production data in tests creates privacy problems, makes tests fragile (production data changes), and creates a pathway for production credentials to appear in test configurations.

---

## 47.8 Workflow Anti-Patterns

### Anti-Pattern: Non-deterministic workflow code

```go
// Bad — time.Now() in workflow code is non-deterministic
func MyWorkflow(ctx workflow.Context) error {
    deadline := time.Now().Add(24 * time.Hour) // different on replay
    ...
}
```

Temporal workflows must be deterministic. `time.Now()`, `rand`, goroutines started outside of workflow primitives, and network calls outside of activities all produce non-deterministic behavior that breaks workflow replay. Use `workflow.Now(ctx)` for time, `workflow.Go` for goroutines, and activities for all I/O.

---

### Anti-Pattern: Large payloads in workflow signals and results

```go
// Bad — sends entire invoice document as workflow input
workflow.ExecuteWorkflow(ctx, ApproveInvoice, invoice) // invoice is 50KB
```

Workflow inputs and results are stored in Temporal's persistence. Large payloads increase storage cost, increase serialization time, and make the workflow history difficult to inspect. Pass IDs; fetch data inside activities.

```go
// Good
workflow.ExecuteWorkflow(ctx, ApproveInvoice, invoiceID) // just the ID
```

---

*End of Chapter 47*

**Previous:** [Chapter 46 — Best Practices](./46-best-practices.md)
**Next:** [Chapter 48 — Reference Architecture](./48-reference-architecture.md)
