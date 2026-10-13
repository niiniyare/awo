# Chapter 43 — IDE Tooling

> **Volume:** IX — Testing, CI/CD & Governance
> **Phase:** 4
> **Audience:** All Engineers, Platform Engineers
> **Prerequisites:** Chapters 05–12, 17–21

---

## Table of Contents

- [43.1 Developer Experience Goals](#431-developer-experience-goals)
- [43.2 Go Development Setup](#432-go-development-setup)
- [43.3 SDUI Surface Authoring](#433-sdui-surface-authoring)
- [43.4 TypeScript / React Development](#434-typescript--react-development)
- [43.5 Database Tooling](#435-database-tooling)
- [43.6 CI/CD Pipeline](#436-cicd-pipeline)
- [43.7 Local Development Environment](#437-local-development-environment)

---

## 43.1 Developer Experience Goals

1. **First `docker compose up` to a running local environment: under 5 minutes.**
2. **End-to-end test cycle (edit → see result): under 30 seconds for backend; under 3 seconds for frontend.**
3. **Surface definition changes hot-reload in the local environment without restarting any service.**
4. **All CI checks runnable locally with a single command.**

---

## 43.2 Go Development Setup

### 43.2.1 Required Tools

```bash
# Install all Go tooling with one script
make dev-tools

# What it installs:
# - go 1.23+
# - goa (code generation from DSL)
# - mockery (mock generation)
# - sqlc (type-safe SQL)
# - golangci-lint (linting)
# - govulncheck (vulnerability scanning)
# - dlv (Delve debugger)
```

### 43.2.2 golangci-lint Configuration

```yaml
# .golangci.yml
linters:
  enable:
    - errcheck        # Check all error returns are handled
    - govet           # go vet
    - staticcheck     # Staticcheck
    - unused          # Unused code
    - goimports       # Import formatting
    - exhaustive      # Exhaustive switch statements on enums
    - noctx           # Detect HTTP calls without context
    - sqlclosecheck   # Ensure sql.Rows.Close() is called
    - wrapcheck       # Ensure errors from external packages are wrapped

linters-settings:
  errcheck:
    check-type-assertions: true
  exhaustive:
    default-signifies-exhaustive: false  # Explicit default is fine
```

### 43.2.3 VS Code Settings (Go)

```json
// .vscode/settings.json
{
  "go.lintTool": "golangci-lint",
  "go.lintFlags": ["--fast"],
  "go.formatTool": "goimports",
  "go.testFlags": ["-v", "-race"],
  "go.coverageDecorator": { "type": "highlight" },
  "[go]": {
    "editor.formatOnSave": true,
    "editor.codeActionsOnSave": {
      "source.organizeImports": "explicit"
    }
  }
}
```

### 43.2.4 Code Generation Workflow

When changing the Goa DSL, run generation before any other work:

```bash
# Regenerate all: OpenAPI spec, HTTP handlers, gRPC stubs
make gen

# What runs:
# goa gen github.com/awoerp/api/design      → gen/http/, gen/grpc/
# sqlc generate                              → internal/db/sqlc/
# mockery --all                              → internal/mocks/
```

Generated files are committed to the repository. Never edit generated files by hand; edit the DSL or SQL query and regenerate.

---

## 43.3 SDUI Surface Authoring

### 43.3.1 Surface Definition LSP

A custom Language Server Protocol (LSP) implementation provides IDE support for surface definition JSON files:

**Features:**
- Schema-based autocomplete for component types and properties
- Validation of `label_key` references against translation files
- Validation of `action_id` references against declared actions
- Binding expression (`$row.`, `$state.`, `$session.`) completions
- Hover documentation for every component type and property
- Go-to-definition for action IDs

```bash
# Install the SDUI LSP
go install github.com/awoerp/sdui-lsp@latest

# Add to VS Code settings.json
{
  "awoerp.sdui.lsp.enabled": true,
  "awoerp.sdui.translationsPath": "./translations",
  "awoerp.sdui.surfacesPath": "./surfaces"
}
```

### 43.3.2 Surface Preview

The surface preview tool runs the compiler locally and opens a browser preview of the compiled surface:

```bash
# Preview a surface as a specific role
awoerp-preview --surface invoice_list --role approver --locale sw-KE

# Opens browser at http://localhost:3001
# Hot-reloads on surface definition changes
```

The preview tool connects to the local development backend (for data) but uses the local surface definition (not the published one). This lets engineers iterate on surface definitions without publishing them.

### 43.3.3 Surface Linter

```bash
# Run the surface linter
awoerp-lint surfaces/

# Checks:
# - All label_key values exist in en-KE translations
# - All action_id references point to declared actions
# - No components lack required accessibility labels
# - No infinite binding loops
# - All required_data_sources are declared
```

The surface linter runs in CI and blocks publication of invalid surface definitions.

---

## 43.4 TypeScript / React Development

### 43.4.1 Type Generation from OpenAPI

TypeScript types for API responses are generated from the OpenAPI spec:

```bash
# Generate TypeScript types from OpenAPI spec
npx openapi-typescript gen/http/openapi3.json --output src/types/api.ts
```

API calls use the generated types:

```typescript
import type { Invoice, ListInvoicesResponse } from '@/types/api';

async function listInvoices(): Promise<ListInvoicesResponse> {
    const res = await fetch('/api/v1/invoices');
    return res.json() as Promise<ListInvoicesResponse>;
}
```

### 43.4.2 SDUI Component Type Generation

Component prop types for the rendering engine are generated from the SDUI JSON schema:

```bash
# Generate TypeScript types from SDUI schema
npx json-schema-to-typescript sdui-schema.json --out src/types/sdui.ts
```

### 43.4.3 Frontend Linting

```json
// .eslintrc.json
{
  "extends": [
    "eslint:recommended",
    "plugin:@typescript-eslint/strict-type-checked",
    "plugin:react-hooks/recommended",
    "plugin:jsx-a11y/strict"
  ],
  "rules": {
    "no-console": "error",
    "@typescript-eslint/no-explicit-any": "error",
    "@typescript-eslint/no-non-null-assertion": "error"
  }
}
```

---

## 43.5 Database Tooling

### 43.5.1 sqlc

All SQL queries are defined in `.sql` files and compiled to type-safe Go functions by `sqlc`:

```sql
-- internal/db/queries/invoices.sql

-- name: GetInvoice :one
SELECT * FROM invoices
WHERE id = $1 AND tenant_id = $2
LIMIT 1;

-- name: ListInvoicesByTenant :many
SELECT * FROM invoices
WHERE tenant_id = $1
  AND status = COALESCE(sqlc.narg('status'), status)
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
```

`sqlc generate` produces:

```go
// internal/db/sqlc/invoices.sql.go (generated)
func (q *Queries) GetInvoice(ctx context.Context, arg GetInvoiceParams) (Invoice, error) { ... }
func (q *Queries) ListInvoicesByTenant(ctx context.Context, arg ListInvoicesByTenantParams) ([]Invoice, error) { ... }
```

### 43.5.2 Migration Tool

Migrations use `golang-migrate`:

```bash
# Create a new migration
make migration name=add_invoice_custom_fields
# → creates: migrations/20240315123456_add_invoice_custom_fields.up.sql
# →          migrations/20240315123456_add_invoice_custom_fields.down.sql

# Run migrations
make migrate-up

# Roll back one step
make migrate-down
```

Each migration file is reviewed in the PR like any code change. Down migrations must exist and be tested.

### 43.5.3 Local DB Access

```bash
# Connect to local PostgreSQL as the app role (respects RLS)
make db-connect-app

# Connect as superuser (bypasses RLS, for debugging)
make db-connect-super

# Open pgAdmin
make pgadmin
# → http://localhost:5050
```

---

## 43.6 CI/CD Pipeline

### 43.6.1 Pipeline Overview

```
PR opened / pushed
    │
    ├─ [lint] golangci-lint, eslint, surface-lint
    ├─ [test:unit] go test ./..., pnpm test
    ├─ [test:integration] go test -tags=integration ./...
    ├─ [test:contract] Pact consumer tests, OpenAPI validation
    ├─ [test:build] go build, pnpm build
    └─ [test:security] gosec, govulncheck, npm audit
    │
    All pass → Deploy to staging
    │
    ├─ [test:e2e] Playwright, Detox
    ├─ [test:visual] Visual regression
    └─ [test:a11y] axe-core
    │
    All pass → PR can be merged to main
    │
    main
    ├─ [deploy:staging] Automatic
    └─ [deploy:production] Manual approval required
```

### 43.6.2 GitHub Actions Workflow (excerpt)

```yaml
# .github/workflows/ci.yml
name: CI

on:
  pull_request:
    branches: [main]

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.23' }
      - run: go test -race -count=1 ./...

  integration-tests:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16
        env:
          POSTGRES_PASSWORD: test
          POSTGRES_DB: awoerp_test
      redis:
        image: redis:7
    steps:
      - uses: actions/checkout@v4
      - run: go test -tags=integration -race -count=1 ./...

  e2e-tests:
    needs: [unit-tests, integration-tests]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Deploy to ephemeral staging
        run: make deploy-ephemeral
      - name: Run Playwright
        run: pnpm test:e2e
      - name: Teardown ephemeral staging
        if: always()
        run: make teardown-ephemeral
```

### 43.6.3 Deployment Stages

| Stage | Trigger | Approval |
|-------|---------|---------|
| Ephemeral (per-PR) | PR opened | Automatic |
| Staging | Merge to main | Automatic |
| Production | Git tag (vX.Y.Z) | Manual (two engineers) |

---

## 43.7 Local Development Environment

### 43.7.1 docker compose Setup

```yaml
# docker-compose.yml
services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_PASSWORD: dev
      POSTGRES_DB: awoerp_dev
    volumes:
      - postgres_data:/var/lib/postgresql/data
    ports:
      - "5432:5432"

  redis:
    image: redis:7
    ports:
      - "6379:6379"

  temporal:
    image: temporalio/auto-setup:1.24
    depends_on: [postgres]
    environment:
      DB: postgresql
      DB_PORT: 5432
      POSTGRES_USER: postgres
      POSTGRES_PWD: dev

  openfga:
    image: openfga/openfga:latest
    command: run
    ports:
      - "8080:8080"

volumes:
  postgres_data:
```

### 43.7.2 First-Time Setup

```bash
# Clone and set up the development environment
git clone https://github.com/awoerp/awoerp
cd awoerp

# Start all dependencies
docker compose up -d

# Install Go tools
make dev-tools

# Install Node dependencies
pnpm install

# Run database migrations
make migrate-up

# Seed development data
make seed-dev

# Start the API server (hot-reload via air)
make dev-api

# Start the web app (hot-reload via Vite)
make dev-web

# Open the local app
open http://localhost:3000
```

### 43.7.3 Hot Reload

- **Go backend:** `air` watches for file changes and recompiles the binary in < 3 seconds for most changes.
- **Surface definitions:** The SDUI compiler watches the `./surfaces/` directory. Changes to a surface definition invalidate the cache and recompile on the next request.
- **Web frontend:** Vite HMR (Hot Module Replacement) applies React changes without full page reload.

---

*End of Chapter 43*

**Previous:** [Chapter 42 — API Contract Testing](./42-api-contract-testing.md)
**Next:** [Chapter 44 — Migration](./44-migration.md)
