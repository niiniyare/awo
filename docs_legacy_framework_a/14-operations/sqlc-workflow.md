> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "SQLC Workflow"
id: ops-012
status: accepted
category: GUIDE
stability: STABLE
audience: [framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[SQLC Integration](../05-persistence/sqlc-integration.md)"
  - "[Migrations](migrations.md)"
  - "[Local Development Setup](local-development.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# SQLC Workflow

**OPS-012 | Status: Accepted | Stability: Stable**

Step-by-step guide for adding, modifying, and validating SQLC queries in the Awo framework.

---

## 1. When to Touch SQLC

SQLC queries are **framework-internal**. You only need to touch them when:

- Adding a new system entity (new table → new query file)
- Adding a specialized query used internally by the repository implementation
- Fixing a bug in an existing generated query

Module authors writing business logic never touch SQLC. If a module needs a custom data shape, compose `EntityRepository` calls in a service function.

---

## 2. Prerequisites

Install `sqlc`:

```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

# Verify
sqlc version
# sqlc v1.27.0
```

The project pins the SQLC version in `tools.go`:

```go
//go:build tools

package tools

import (
    _ "github.com/sqlc-dev/sqlc/cmd/sqlc"
)
```

Install via module:

```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@$(go list -m -f '{{.Version}}' github.com/sqlc-dev/sqlc)
```

---

## 3. Adding Queries for a New Entity

### Step 1: Create the migration first

SQLC validates queries against the live schema. Migrations must exist before generating:

```bash
# Create migration files
touch db/migration/$(date +%Y%m%d%H%M%S)_create_finance_invoice.up.sql
touch db/migration/$(date +%Y%m%d%H%M%S)_create_finance_invoice.down.sql
```

Write the `CREATE TABLE` in `.up.sql`, run it:

```bash
go run ./cmd/migrate up
```

### Step 2: Create the query file

```bash
touch db/queries/finance_invoice.sql
```

Write annotated queries:

```sql
-- db/queries/finance_invoice.sql

-- name: GetFinanceInvoice :one
SELECT * FROM finance_invoice
WHERE id = @id
  AND tenant_id = current_tenant_id();

-- name: ListFinanceInvoices :many
SELECT * FROM finance_invoice
WHERE tenant_id = current_tenant_id()
ORDER BY created_at DESC
LIMIT @limit_val OFFSET @offset_val;

-- name: CreateFinanceInvoice :one
INSERT INTO finance_invoice (
    id, tenant_id, number, customer, status, total_kes, created_at, updated_at
) VALUES (
    @id, current_tenant_id(), @number, @customer, @status, @total_kes, NOW(), NOW()
)
RETURNING *;

-- name: UpdateFinanceInvoice :one
UPDATE finance_invoice
SET status     = COALESCE(@status, status),
    total_kes  = COALESCE(@total_kes, total_kes),
    updated_at = NOW()
WHERE id = @id
  AND tenant_id = current_tenant_id()
RETURNING *;

-- name: DeleteFinanceInvoice :exec
DELETE FROM finance_invoice
WHERE id = @id
  AND tenant_id = current_tenant_id();

-- name: CountFinanceInvoices :one
SELECT COUNT(*) FROM finance_invoice
WHERE tenant_id = current_tenant_id();
```

### Step 3: Generate

```bash
sqlc generate
```

SQLC reads `sqlc.yaml`, validates queries against the schema (via `db/migration/` files), and writes output to `db/sqlc/`.

### Step 4: Verify generated output

```bash
# New files should appear
ls db/sqlc/
# models.go, querier.go, db.go, finance_invoice.sql.go, ...
```

Check the generated struct matches expectations:

```go
// db/sqlc/models.go (generated)
type FinanceInvoice struct {
    ID        uuid.UUID        `db:"id"        json:"id"`
    TenantID  uuid.UUID        `db:"tenant_id" json:"tenant_id"`
    Number    string           `db:"number"    json:"number"`
    Customer  uuid.UUID        `db:"customer"  json:"customer"`
    Status    string           `db:"status"    json:"status"`
    TotalKes  decimal.Decimal  `db:"total_kes" json:"total_kes"`
    CreatedAt time.Time        `db:"created_at" json:"created_at"`
    UpdatedAt time.Time        `db:"updated_at" json:"updated_at"`
}
```

### Step 5: Wire the repository

Register the new Querier methods in the repository provider — see [Wire Provider Sets](../03-kernel/wire-provider-sets.md).

---

## 4. Query Annotation Reference

| Annotation | Return type | Use for |
|---|---|---|
| `:one` | `(T, error)` | Get by ID, insert RETURNING, update RETURNING |
| `:many` | `([]T, error)` | List queries |
| `:exec` | `error` | Delete, update with no return |
| `:execrows` | `(int64, error)` | Bulk update — returns rows affected |
| `:execresult` | `(sql.Result, error)` | Rare — avoid |

---

## 5. Parameter Conventions

Use named parameters (`@param`) not positional (`$1`) in query files — SQLC handles numbering:

```sql
-- Good: named parameters
SELECT * FROM finance_invoice WHERE id = @id AND status = @status;

-- Avoid: positional (harder to read, reorder-sensitive)
SELECT * FROM finance_invoice WHERE id = $1 AND status = $2;
```

For nullable/optional filter parameters, use the `COALESCE` pattern:

```sql
-- Optional status filter: pass NULL to skip
WHERE tenant_id = current_tenant_id()
  AND (@status::text IS NULL OR status = @status)
```

SQLC generates a params struct:

```go
type ListFinanceInvoicesParams struct {
    Status    sql.NullString
    LimitVal  int32
    OffsetVal int32
}
```

---

## 6. Modifying an Existing Query

1. Edit the `.sql` file in `db/queries/`
2. Run `sqlc generate`
3. Check `db/sqlc/` for changed files
4. Update any callers in the framework's repository implementation
5. Run integration tests

If the query signature changes (different params struct), the compiler catches all callers that need updating.

---

## 7. Validating Without Generating

SQLC can validate queries against the schema without writing output:

```bash
sqlc vet
```

`sqlc vet` catches:
- SQL syntax errors
- References to non-existent tables/columns
- Type mismatches between SQL and Go overrides
- Missing `RETURNING` on `:one` INSERT/UPDATE

Run `sqlc vet` in CI before `sqlc generate` to fail fast on bad SQL.

---

## 8. CI Integration

```yaml
# .github/workflows/ci.yml (excerpt)

- name: Validate SQLC queries
  run: sqlc vet

- name: Check generated code is up to date
  run: |
    sqlc generate
    git diff --exit-code db/sqlc/
    # Fails if generated files differ from committed — means someone edited sqlc output manually
    # or forgot to regenerate after changing a query file
```

The second check enforces that `db/sqlc/` is always in sync with `db/queries/` — no manual edits to generated files.

---

## 9. Common Errors

### `unknown table "finance_invoice"`

SQLC validates queries against migration files. The table doesn't exist yet:
1. Ensure the `.up.sql` migration file exists in `db/migration/`
2. Verify the filename timestamp is correct (14 digits)
3. Re-run `sqlc generate`

### `cannot use NULL with non-nullable column`

Column is `NOT NULL` but query inserts `NULL`. Check migration vs query:
- Either make the column nullable in the migration (and add a new migration if already applied)
- Or provide a non-null default in the query

### Generated struct missing field

SQLC only generates fields for columns that exist in the schema. If a new column was added in a migration, `sqlc generate` must be re-run after applying the migration.

### `overrides` type mismatch

The `sqlc.yaml` `overrides` section maps `db_type` to `go_type`. If a new column uses a type not in the overrides list, SQLC uses a generic `any`. Add the override:

```yaml
overrides:
  - db_type: "my_custom_type"
    go_type: "mypackage.MyType"
```

---

## Related Documents

- [SQLC Integration](../05-persistence/sqlc-integration.md) — architecture overview, layering
- [Migrations](migrations.md) — creating the schema before adding queries
- [Migration Runner](migration-runner.md) — applying migrations locally
- [Wire Provider Sets](../03-kernel/wire-provider-sets.md) — wiring the Querier into repositories
