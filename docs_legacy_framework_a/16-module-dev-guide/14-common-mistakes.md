> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Common Mistakes and How to Avoid Them"
id: mdg-014
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Pre-Review Checklist](12-pre-review-checklist.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
  - "[Module Boundary Rules](../02-architecture/module-boundaries.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Common Mistakes and How to Avoid Them

**MDG-014 | Status: Accepted | Stability: Stable**

A catalog of the most common mistakes made when building Awo modules, organized by category, with the symptom, root cause, and correct pattern.

---

## 1. Persistence Mistakes

### 1.1 Float for Money

**Symptom**: Rounding errors in financial reports; totals off by pennies.

**Root cause**: Using `FieldFloat` or `float64` for monetary amounts.

```go
// WRONG
{Name: "amount", Type: def.FieldFloat}

// CORRECT
{Name: "amount", Type: def.FieldCurrency}
// → numeric(20,4) in PostgreSQL, decimal.Decimal in Go
```

**Rule**: LAW-003: Never store money as float.

---

### 1.2 WHERE tenant_id in Business Logic

**Symptom**: Double-filtering, or bypassing PolicyFunc row-level security.

**Root cause**: Manually adding tenant filter to repository queries.

```go
// WRONG
results, _, err := repo.Query(ctx, filter.Eq("tenant_id", tenantID))

// CORRECT
results, _, err := repo.Query(ctx, filter.All())
// RLS handles tenant isolation automatically
```

**Rule**: LAW-005, ARCH-003.

---

### 1.3 N+1 Queries

**Symptom**: Hundreds of database queries per page load; slow list views.

**Root cause**: Loading edges inside a loop instead of using `WithEdge`.

```go
// WRONG: N+1 — one query per contact to load interactions
contacts, _, _ := contactRepo.Query(ctx, filter.All())
for _, c := range contacts {
    c.Interactions, _ = interactionRepo.Query(ctx, filter.Eq("contact_id", c.ID))
}

// CORRECT: single query with edge loading
contacts, _, _ := contactRepo.Query(ctx, filter.All(),
    entity.WithEdge("interactions"))
```

**Rule**: LAW-009: Never lazy-load edges.

---

### 1.4 Raw SQL in Business Logic

**Symptom**: Migration-time breakage when table schema changes; bypassed RLS.

**Root cause**: Constructing SQL strings in module code.

```go
// WRONG
rows, _ := db.Query("SELECT * FROM crm_contact WHERE status = $1", status)

// CORRECT
results, _, _ := repo.Query(ctx, filter.Eq("status", status))
```

**Rule**: LAW-002: Never raw SQL in business logic.

---

## 2. Tenancy Mistakes

### 2.1 Missing Tenant Context in Background Jobs

**Symptom**: RLS rejects all queries in Temporal activities; `no tenant context` error.

**Root cause**: Forgetting to call `SetTenantContext` in activities.

```go
// WRONG: no tenant context
func (a *Activities) ProcessActivity(ctx context.Context, input Input) error {
    results, _, err := a.Repo.Query(ctx, filter.All())  // fails: no tenant context
    return err
}

// CORRECT
func (a *Activities) ProcessActivity(ctx context.Context, input Input) error {
    tenantCtx, err := a.TenantStore.SetTenantContext(ctx, input.TenantID)
    if err != nil { return err }
    results, _, err := a.Repo.Query(tenantCtx, filter.All())
    return err
}
```

**Rule**: LAW-005, TEN-004 §3.

---

### 2.2 Storing Context in Struct Fields

**Symptom**: Wrong tenant data served; race conditions in concurrent requests.

**Root cause**: Storing `context.Context` as a struct field.

```go
// WRONG
type Service struct {
    ctx  context.Context  // carries tenant ID — shared across concurrent requests!
    repo EntityRepository
}

// CORRECT
func (s *Service) GetData(ctx context.Context) ([]Data, error) {
    return s.repo.Query(ctx, filter.All())
}
```

**Rule**: Go context best practices; TEN-004 §4.

---

## 3. Workflow Mistakes

### 3.1 time.Now() in Workflow Code

**Symptom**: Non-determinism errors on Temporal workflow replay; `nondeterministic error`.

**Root cause**: Using `time.Now()` instead of `workflow.Now(ctx)`.

```go
// WRONG
deadline := time.Now().Add(7 * 24 * time.Hour)

// CORRECT
deadline := workflow.Now(ctx).Add(7 * 24 * time.Hour)
```

**Rule**: LAW-010: Never use time.Now() in workflow code.

---

### 3.2 I/O in Workflow Functions

**Symptom**: Network calls blocking workflow replay; non-determinism.

**Root cause**: Making HTTP calls or DB queries in workflow functions directly.

```go
// WRONG: HTTP call in workflow
func MyWorkflow(ctx workflow.Context, input Input) error {
    resp, _ := http.Get("https://api.example.com/...")  // I/O in workflow!
    // ...
}

// CORRECT: I/O in activity
func MyWorkflow(ctx workflow.Context, input Input) error {
    var activities *MyActivities
    return workflow.ExecuteActivity(ctx, activities.FetchDataActivity, input).Get(ctx, nil)
}
```

**Rule**: LAW-011: Never write I/O inside workflow functions.

---

### 3.3 Non-Idempotent Activities

**Symptom**: Duplicate emails sent; duplicate records created on retry.

**Root cause**: Activities not checking if work was already done.

```go
// WRONG: sends email every time activity runs
func (a *Activities) SendEmailActivity(ctx context.Context, input Input) error {
    return a.Email.Send(ctx, buildEmail(input))
}

// CORRECT: idempotent
func (a *Activities) SendEmailActivity(ctx context.Context, input Input) error {
    already, _ := a.SentLog.Exists(ctx, input.EntityID, "welcome")
    if already { return nil }
    if err := a.Email.Send(ctx, buildEmail(input)); err != nil { return err }
    return a.SentLog.Record(ctx, input.EntityID, "welcome")
}
```

---

## 4. Security Mistakes

### 4.1 Stack Traces in API Responses

**Symptom**: Internal Go stack trace visible in API error response.

**Root cause**: Passing errors directly to the JSON response.

```go
// WRONG
return c.Status(500).JSON(fiber.Map{"error": err.Error()})
// err.Error() may contain: "pgconn: connect failed: connection refused at 10.0.0.5:5432"

// CORRECT
slog.Error("operation failed", "err", err)
return c.Status(500).JSON(ErrorEnvelope{Error: ErrorBody{
    Code:    "internal_error",
    Message: "An unexpected error occurred.",
}})
```

**Rule**: LAW-013: Never expose internal stack traces to clients.

---

### 4.2 Logging Sensitive Fields

**Symptom**: Passwords or session tokens appear in application logs.

**Root cause**: Logging entity records without filtering sensitive fields.

```go
// WRONG
slog.Info("user created", "user", user)  // user struct includes password_hash

// CORRECT
slog.Info("user created", "user_id", user.ID, "email", user.Email)
// Never log: password_hash, session_token, reset_token, client_secret
```

**Rule**: LAW-013.

---

### 4.3 Authorization in VisibleWhen (SDUI)

**Symptom**: Users see buttons they should not; actions succeed despite UI appearing disabled.

**Root cause**: Using amis `VisibleWhen` for authorization instead of server-side gating.

```go
// WRONG: VisibleWhen is client-side and bypassable
amis.Button{VisibleWhen: "${actor.role === 'admin'}", OnClick: deleteAction}

// CORRECT: exclude from schema if no permission
if psc.IfPermitted("finance_invoice", "delete") {
    buttons = append(buttons, deleteButton)
}
```

**Rule**: SEC-002 §4.

---

## 5. Module Structure Mistakes

### 5.1 Custom CRUD Handlers

**Symptom**: Missing audit log; missing hook execution; missing permission checks.

**Root cause**: Writing custom `POST /` handlers instead of using auto-generated CRUD.

```go
// WRONG: custom create handler — bypasses entire EntityDefinition lifecycle
app.Post("/api/v1/invoices", func(c *fiber.Ctx) error {
    var invoice Invoice
    c.BodyParser(&invoice)
    db.Create(&invoice)
    return c.JSON(invoice)
})

// CORRECT: no handler needed for standard CRUD
// def.Register(&InvoiceDefinition) auto-generates all CRUD routes
```

**Rule**: LAW-001: Never write custom CRUD route handlers.

---

### 5.2 Hard-Coded Secrets

**Symptom**: Secrets committed to git; exposed in Dockerfile or env files.

```go
// WRONG
const JWTSecret = "super-secret-key-1234"

// CORRECT
jwtSecret := os.Getenv("JWT_SECRET")
if jwtSecret == "" {
    log.Fatal("JWT_SECRET environment variable is required")
}
```

**Rule**: LAW-020: Never hard-code secrets.

---

### 5.3 RegisterCustomForTenant in Request Handler

**Symptom**: Data race; random crashes under concurrent load; intermittent wrong-tenant schema served.

**Root cause**: Calling `registry.RegisterCustomForTenant` from an HTTP handler goroutine.

```go
// WRONG: registry write races with concurrent reads
func CreateSchemaHandler(c *fiber.Ctx) error {
    registry.RegisterCustomForTenant(tenantID, schema)
    return c.SendStatus(200)
}

// CORRECT: queue the schema change; apply with write lock
func CreateSchemaHandler(c *fiber.Ctx) error {
    err := schemaMutationQueue.Enqueue(c.Context(), tenantID, schema)
    return c.Status(202).JSON(...)
}
```

**Rule**: LAW-012, KERN-005 §3.

---

## 6. amis / SDUI Mistakes

### 6.1 Custom React/JS for Standard Views

**Symptom**: SDK update breaks custom components; increased maintenance burden.

**Root cause**: Writing React components for functionality available in amis.

```
WRONG: custom React table component for entity list
CORRECT: amis CRUD component with column configuration
```

**Rule**: LAW-019: Never write custom React/JS for standard ERP views.

---

### 6.2 Updating amis SDK Without Audit

**Symptom**: All forms break after `npm update`; amis component API changed.

**Root cause**: Updating `web/sdk/` without reviewing changelog and testing all pages.

```
WRONG: git pull; npm update; deploy
CORRECT: Pin SDK. Update only after full compatibility audit. See ADR-003.
```

**Rule**: LAW-019.

---

## Related Documents

- [Pre-Review Checklist](12-pre-review-checklist.md) — checklist form of these rules
- [Architecture Laws](../02-architecture/laws.md) — full law text
- [Module Boundary Rules](../02-architecture/module-boundaries.md) — cross-module import rules
- [Security Model](../15-security/security-model.md) — security constraint rationale
