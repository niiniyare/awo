> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Multi-Tenancy Patterns"
id: ten-004
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: informative
related:
  - "[Tenant Model](tenant-model.md)"
  - "[RLS](rls.md)"
  - "[Tenant Lifecycle](tenant-lifecycle.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Multi-Tenancy Patterns

**TEN-004 | Status: Accepted | Stability: Stable**

This document describes patterns that commonly arise in multi-tenant module development: cross-tenant operations, platform admin context, tenant-scoped background jobs, and tenant-specific customization.

---

## 1. Standard Tenant-Scoped Operations

The normal pattern: middleware resolves the tenant from the request, sets the context, and all repository operations are automatically scoped.

```go
// In a route handler — tenant is already set by middleware
func ListInvoicesHandler(c *fiber.Ctx) error {
    // repo is tenant-scoped automatically
    invoices, _, err := invoiceRepo.Query(c.Context(), filter.All())
    // ...
}
```

Module authors MUST NOT construct `context.WithValue(ctx, "tenant_id", ...)` manually. The tenant context is established by the middleware pipeline before any handler executes.

---

## 2. Platform Admin Context (Cross-Tenant)

Platform administrators sometimes need to query across all tenants. The `platform-admin` role bypasses RBAC and receives a special context that bypasses RLS.

```go
// Platform admin handler — reads global table directly
// Global tables (tenants, iam_audit_log, etc.) have no RLS
func ListAllTenantsHandler(c *fiber.Ctx) error {
    // tenantRepo reads from the global `tenants` table (no RLS)
    tenants, _, err := tenantRepo.QueryGlobal(c.Context(), filter.All())
    // ...
}
```

`QueryGlobal` is available only on the Tenant entity repository — it is not part of the `EntityRepository[T]` interface. Business modules MUST NOT use `QueryGlobal`.

---

## 3. Tenant-Scoped Background Jobs

Scheduled background jobs (e.g., monthly report generation, subscription renewal) must operate on behalf of a specific tenant. Use the `TenantJob` wrapper:

```go
// Temporal activity — must set tenant context explicitly
func (a *ReportActivities) GenerateMonthlyReportActivity(ctx context.Context, input ReportInput) error {
    // Activities do not have HTTP request context — must set tenant context explicitly
    tenantCtx, err := a.TenantStore.SetTenantContext(ctx, input.TenantID)
    if err != nil {
        return fmt.Errorf("GenerateMonthlyReportActivity: set tenant: %w", err)
    }

    // All repo operations on tenantCtx are tenant-scoped
    invoices, _, err := a.InvoiceRepo.Query(tenantCtx, filter.Eq("status", "Submitted"))
    // ...
}
```

Background jobs MUST always call `SetTenantContext` before any repository operation. There is no automatic tenant context in background goroutines or Temporal activities.

---

## 4. Tenant Context in Goroutines

When spawning goroutines for parallel processing within a request handler, the context MUST be passed explicitly:

```go
// Correct: pass context to goroutine
go func(ctx context.Context) {
    // ctx carries tenant context
    repo.Query(ctx, filter.All())
}(c.Context())  // pass a copy of the context

// Incorrect: capture context by closure (may be mutated by the outer goroutine)
go func() {
    ctx := c.Context()  // dangerous: ctx may be cancelled when request ends
    repo.Query(ctx, filter.All())
}()
```

For long-running goroutines, use `context.WithoutCancel(c.Context())` to detach the goroutine's context from the request lifecycle while preserving tenant context:

```go
ctx := context.WithoutCancel(c.Context())
go func(ctx context.Context) {
    // Long-running work; ctx has tenant context but won't be cancelled with request
}(ctx)
```

---

## 5. Tenant-Specific Customization

Tenants can customize entity behavior through three mechanisms, in increasing complexity:

### Level 1: Custom Fields (Runtime, No Code)

Tenants add fields to any entity via the admin UI. No migration, no redeploy.

```
Admin UI: CRM Contacts → Custom Fields → + Add Field
→ Field: "industry_sector", Type: Select, Options: [Agriculture, Finance, Tech, ...]
```

### Level 2: Settings Overrides (Runtime, No Code)

Module behaviors configurable via the Settings module:

```
Setting: finance.invoice_approval_threshold = 50000.00
Tenants above 50,000 KES must go through approval; below can self-approve.
```

### Level 3: NamingSeries Override (Runtime, No Code)

Tenants customize document numbering prefix:

```
Setting: finance_invoice.number.series_prefix = "ACME-INV"
Result: "ACME-INV-2024-00001" instead of "INV-2024-00001"
```

All three levels require no code change or redeploy — they are runtime configuration via existing framework mechanisms.

---

## 6. Anti-Patterns

### Direct Tenant ID Comparison in Business Logic

```go
// WRONG: application-level tenant filtering
invoices, err := repo.Query(ctx, filter.Eq("tenant_id", tenantID))
// The repo already applies tenant filter via RLS — this double-applies it
// and also bypasses PolicyFunc row-level security

// CORRECT: no tenant filter in business logic
invoices, err := repo.Query(ctx, filter.All())
// RLS handles tenant isolation; PolicyFunc handles row-level security
```

### Constructing Tenant Context in Business Logic

```go
// WRONG: business logic sets tenant context manually
ctx = store.SetTenantContextFromID(ctx, tenantID)
invoices, err := repo.Query(ctx, ...)

// CORRECT: tenant context is set by middleware (for request handlers)
// or by the activity's explicit SetTenantContext call (for background jobs)
```

### Sharing Contexts Across Requests

```go
// WRONG: storing context in struct field (context not designed for storage)
type Service struct {
    ctx context.Context  // BAD
    repo EntityRepository
}

// CORRECT: context passed as first parameter to every method
func (s *Service) GetInvoice(ctx context.Context, id uuid.UUID) (*Invoice, error) {
    return s.repo.Get(ctx, id)
}
```

---

## Related Documents

- [Tenant Model](tenant-model.md) — tenant identification and context mechanics
- [RLS](rls.md) — database-level enforcement
- [Tenant Lifecycle](tenant-lifecycle.md) — tenant status machine
- [Architecture Laws](../02-architecture/laws.md) — LAW-005 (no store op without tenant context)
- [Glossary](../GLOSSARY.md) — Tenant Context, Platform Admin, Multi-Tenancy
