# Local Development Setup

**Classification:** Guide — Tier 2
**Owner:** `20-devops/LOCAL_DEVELOPMENT.md`
**Status:** Living document

---

## Docker Compose Services

`docker-compose.yml` starts all infrastructure dependencies:

```yaml
version: "3.9"
services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_USER: awo
      POSTGRES_PASSWORD: awo_dev_password
      POSTGRES_DB: awo_dev
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD", "pg_isready", "-U", "awo"]
      interval: 5s
      retries: 10

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      retries: 10

  temporal:
    image: temporalio/auto-setup:1.24
    environment:
      DB: sqlite
    ports:
      - "7233:7233"
    depends_on:
      - postgres

  temporal-ui:
    image: temporalio/ui:2.26
    environment:
      TEMPORAL_ADDRESS: temporal:7233
    ports:
      - "8088:8080"
    depends_on:
      - temporal

volumes:
  postgres_data:
```

```bash
make infra-up      # start infrastructure
make infra-down    # stop infrastructure
make infra-reset   # delete all data and restart
```

---

## Environment File

```bash
cp .env.example .env
# Edit .env with local values
```

`.env.example`:

```bash
DATABASE_URL=postgres://awo:awo_dev_password@localhost:5432/awo_dev?sslmode=disable
REDIS_URL=redis://localhost:6379/0
TEMPORAL_HOST=localhost:7233
TEMPORAL_NAMESPACE=default
JWT_SECRET=dev-jwt-secret-not-for-production-at-least-64-chars
SESSION_SECRET=dev-session-secret-not-for-production-at-least-64-chars
ENV=development
LOG_LEVEL=debug
LOG_FORMAT=text
PORT=8080
```

`.env` is gitignored — never commit it.

---

## First-Time Setup

```bash
# 1. Start infrastructure
make infra-up

# 2. Run migrations
make migrate-up

# 3. Start the server
make run
```

Do NOT run `go build` or `go run` directly — use `make run` which loads `.env` first.

---

## Live Reload

Install `air` for hot-reload during development:

```bash
go install github.com/air-verse/air@latest
make dev   # uses air for live reload
```

Air watches `*.go` files and rebuilds on save.

---

## Test Database

Tests use a separate database. Configure in `.env.test`:

```bash
DATABASE_URL=postgres://awo:awo_dev_password@localhost:5432/awo_test?sslmode=disable
REDIS_URL=redis://localhost:6379/1   # DB index 1 — separate from dev
```

Create and migrate:

```bash
createdb -h localhost -U awo awo_test
DATABASE_URL=... make migrate-up
# or
make db-test-setup
```

---

## Seed Data

The seed script creates two tenants for local development:

```bash
make db-seed
```

Creates:
- Tenant: `Acme Corp` (ID: `aaaaaaaa-0000-0000-0000-000000000001`)
  - Admin: `admin@acme.com` / `Password123!`
- Tenant: `Beta Inc` (ID: `bbbbbbbb-0000-0000-0000-000000000002`)
  - Admin: `admin@beta.com` / `Password123!`

Use these for testing cross-tenant isolation.

---

## Frontend Development

```bash
# Serve static files
cd web && python3 -m http.server 3000
```

Point browser to `http://localhost:3000`. The amis frontend calls `http://localhost:8080/api/v1/`.

---

## Ports Reference

| Service | Port | URL |
|---------|------|-----|
| API server | 8080 | `http://localhost:8080/api/v1` |
| Health checks | 8080 | `http://localhost:8080/health/live` |
| Prometheus metrics | 9090 | `http://localhost:9090/metrics` |
| PostgreSQL | 5432 | `postgres://localhost:5432/awo_dev` |
| Redis | 6379 | `redis://localhost:6379` |
| Temporal gRPC | 7233 | — |
| Temporal UI | 8088 | `http://localhost:8088` |
| Frontend | 3000 | `http://localhost:3000` |

---

## References

- [`20-devops/ENVIRONMENT_VARIABLES.md`](ENVIRONMENT_VARIABLES.md) — All environment variables
- [`18-security/SECRET_MANAGEMENT.md`](../18-security/SECRET_MANAGEMENT.md) — Secret handling rules
