> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Local Development Setup"
id: ops-007
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors, framework-authors, contributors]
since: "1.0"
normative-level: informative
related:
  - "[Quick Start](../01-introduction/quick-start.md)"
  - "[Migrations](migrations.md)"
  - "[Environment Variables Reference](../12-configuration/environment-variables.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Local Development Setup

**OPS-007 | Status: Accepted | Stability: Stable**

Complete setup guide for local Awo development.

---

## 1. Prerequisites

| Tool | Version | Install |
|---|---|---|
| Go | 1.22+ | https://go.dev/dl/ |
| Docker | 24+ | https://docs.docker.com/get-docker/ |
| Docker Compose | V2 | Included with Docker Desktop |
| `migrate` CLI | latest | `go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest` |
| `wire` CLI | latest | `go install github.com/google/wire/cmd/wire@latest` |

---

## 2. Start Infrastructure

```bash
cd /path/to/erp
docker compose up -d
```

Services started:

| Service | Port | Credentials |
|---|---|---|
| PostgreSQL 16 | 5432 | `postgres:postgres` |
| PgBouncer | 5433 | `app:app_password` |
| Redis 7 | 6379 | No auth in development |
| Temporal server | 7233 | — |
| Temporal Web UI | 8080 | http://localhost:8080 |

---

## 3. Environment Variables

Copy the development env file:

```bash
cp .env.example .env
```

`.env` contents for local development:

```bash
DATABASE_URL=postgres://app:app_password@localhost:5433/awo?pool_max_conns=10
REDIS_URL=redis://localhost:6379/0
TEMPORAL_HOST=localhost:7233
TEMPORAL_NAMESPACE=default
JWT_SECRET=dev-jwt-secret-not-for-production-64-chars-minimum-length-here
SESSION_ENCRYPTION_KEY=dev-session-key-32-bytes-minimum
PORT=8081
LOG_LEVEL=debug
LOG_FORMAT=text
```

**Never use these values outside of local development.**

---

## 4. Run Migrations

```bash
# Apply all pending migrations
DATABASE_URL=postgres://app:app_password@localhost:5433/awo migrate -path db/migration up

# Or use the migrate command (reads DATABASE_URL from env)
source .env && go run ./cmd/migrate up
```

---

## 5. Start the Server

```bash
source .env && go run ./cmd/server
```

Server starts at `http://localhost:8081`.

---

## 6. Create a Development Tenant

```bash
curl -X POST http://localhost:8081/api/v1/platform/tenants \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Acme Corp",
    "subdomain": "acme",
    "plan": "standard",
    "admin_email": "admin@acme.com",
    "admin_name": "Admin User"
  }'
```

The provisioning workflow creates the admin user and seeds all roles. The admin's temporary password is logged at DEBUG level.

---

## 7. Hot Reload

Awo does not have built-in hot reload. Use `air` for file-watching restarts:

```bash
go install github.com/cosmtrek/air@latest
air
```

`air` config is in `.air.toml` at the repo root. It watches `internal/`, `framework/`, and `cmd/` for Go file changes.

---

## 8. Running Tests

```bash
# Unit tests (no infrastructure required)
go test ./internal/core/...
go test ./framework/...

# Integration tests (requires running PostgreSQL and Redis)
go test -tags integration ./internal/core/...

# Specific package
go test ./internal/core/finance/...
```

Integration tests use a separate database (`awo_test`) and clean up after themselves via `TestMain`.

---

## 9. Working with Migrations

```bash
# Create a new migration pair
TIMESTAMP=$(date +%Y%m%d%H%M%S)
touch db/migration/${TIMESTAMP}_add_my_field.up.sql
touch db/migration/${TIMESTAMP}_add_my_field.down.sql

# Apply latest migration
source .env && go run ./cmd/migrate up

# Roll back one migration
source .env && go run ./cmd/migrate down 1

# Check current version
source .env && go run ./cmd/migrate version

# Force to a specific version (after fixing a bad migration)
source .env && go run ./cmd/migrate force 20241215143022
```

---

## 10. Temporal Development

Open the Temporal Web UI at http://localhost:8080 to:
- View running and completed workflows
- Send signals to paused approval workflows
- Terminate stuck workflows
- Replay workflow history

```bash
# Install Temporal CLI for command-line workflow management
go install go.temporal.io/sdk/temporal@latest
# (or use the Temporal CLI: https://docs.temporal.io/cli)

# List recent workflows
temporal workflow list --namespace default

# Describe a workflow
temporal workflow describe --workflow-id "{tenant-uuid}.finance_invoice.{id}.on_submit" --namespace default
```

---

## 11. Common Development Issues

### "connection refused" on PostgreSQL

PgBouncer is not ready yet. Wait 5-10 seconds after `docker compose up`, then retry.

### "set_tenant_context: tenant not found"

The test tenant doesn't exist yet. Run the create tenant curl command from step 6.

### "schema_migrations: dirty"

A migration partially applied. Fix the migration file then:
```bash
go run ./cmd/migrate force {VERSION}  # version = the dirty migration timestamp
go run ./cmd/migrate up
```

### Wire compilation error after adding a new provider

Run `wire gen ./cmd/server/` to regenerate `wire_gen.go`.

### Temporal worker not connecting

Check that `TEMPORAL_HOST=localhost:7233` is set and Temporal is running:
```bash
docker compose ps temporal
```

---

## Related Documents

- [Quick Start](../01-introduction/quick-start.md) — create your first entity
- [Migrations](migrations.md) — full migration workflow
- [Testing Patterns](../16-module-dev-guide/15-testing-patterns.md) — writing integration tests
- [Environment Variables Reference](../12-configuration/environment-variables.md) — all env var options
