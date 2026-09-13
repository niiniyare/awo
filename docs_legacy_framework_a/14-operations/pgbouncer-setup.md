> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "PgBouncer Setup"
id: ops-010
status: accepted
category: GUIDE
stability: STABLE
audience: [operators]
since: "1.0"
normative-level: normative
related:
  - "[Deployment](deployment.md)"
  - "[Tenant Model](../06-tenancy/tenant-model.md)"
  - "[Architecture Invariants](../02-architecture/invariants.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# PgBouncer Setup

**OPS-010 | Status: Accepted | Stability: Stable**

Concrete PgBouncer configuration required for Awo's RLS enforcement to function correctly.

---

## 1. Why PgBouncer is Mandatory

Awo uses PostgreSQL's `SET LOCAL` to write a transaction-local config variable:

```sql
SELECT set_tenant_context('018e1234-5678-7abc-def0-123456789abc');
-- internally: SET LOCAL app.current_tenant_id = '018e1234-...';
```

`SET LOCAL` resets when the transaction ends (`COMMIT` or `ROLLBACK`). This is how the RLS policy `USING (tenant_id = current_tenant_id())` is automatically cleared between requests — no manual cleanup needed.

**The problem with session-mode pooling**: In session mode, a connection is held by one client for the entire session. PostgreSQL session-level `SET` persists across transactions. If a pool recycles the connection but session state leaks, the next tenant gets the wrong `current_tenant_id` — a catastrophic RLS bypass.

**PgBouncer in transaction mode** assigns a backend connection only for the duration of one transaction. When the transaction commits, the connection returns to the pool with all `SET LOCAL` values cleared. This guarantees clean state per transaction.

**This is not optional.** INV-007 in the Architecture Invariants states: RLS enforcement requires PgBouncer in transaction mode.

---

## 2. Connection Pool Architecture

```
Awo API Servers (N instances)
        │
        │  TCP :5432 (application port)
        ▼
   PgBouncer (transaction mode)
        │
        │  TCP :5432 (real PostgreSQL port — internal only)
        ▼
   PostgreSQL Primary
```

Awo API connects to PgBouncer. PgBouncer connects to PostgreSQL. PostgreSQL is not exposed to the application network directly.

---

## 3. PgBouncer Configuration

### `pgbouncer.ini`

```ini
[databases]
; Map the "awo" database name to the real PostgreSQL host
awo = host=postgres-primary port=5432 dbname=awo

[pgbouncer]
; Bind address
listen_addr = 0.0.0.0
listen_port = 5432

; CRITICAL: transaction mode for RLS
pool_mode = transaction

; Auth method — scram-sha-256 matches PostgreSQL default
auth_type = scram-sha-256
auth_file = /etc/pgbouncer/userlist.txt

; Pool sizing
; Formula: max_client_conn should be ≥ (api_instances × max_open_conns_per_instance)
max_client_conn = 1000
default_pool_size = 25          ; backend connections per (db, user) pair
min_pool_size = 5               ; keep warm connections
reserve_pool_size = 5           ; burst headroom
reserve_pool_timeout = 3        ; seconds before promoting reserve

; Connection limits
max_db_connections = 100        ; hard cap to PostgreSQL
max_user_connections = 0        ; 0 = unlimited (controlled by pool_size)

; Timeouts (seconds)
server_connect_timeout = 15
server_login_timeout = 15
query_timeout = 0               ; 0 = no timeout (Awo enforces statement_timeout)
client_idle_timeout = 600       ; disconnect idle clients after 10 min
server_idle_timeout = 600       ; recycle idle backend connections

; TLS to PostgreSQL backend
server_tls_sslmode = require
server_tls_ca_file = /etc/pgbouncer/root.crt

; TLS from application clients
client_tls_sslmode = require
client_tls_cert_file = /etc/pgbouncer/server.crt
client_tls_key_file = /etc/pgbouncer/server.key

; Logging
log_connections = 0             ; set to 1 for debugging; too verbose in prod
log_disconnections = 0
log_pooler_errors = 1
stats_period = 60

; Admin interface (restricted to localhost)
admin_users = pgbouncer_admin
stats_users = pgbouncer_monitor

; PID file
pidfile = /var/run/pgbouncer/pgbouncer.pid
```

### `userlist.txt`

```
"awo_app"    "SCRAM-SHA-256$..."   ; hashed password — generate with psql \password
"pgbouncer_admin"  "SCRAM-SHA-256$..."
"pgbouncer_monitor" "SCRAM-SHA-256$..."
```

Generate password hash:

```bash
# Connect to PostgreSQL, set password, copy the stored hash
psql -c "SELECT rolpassword FROM pg_authid WHERE rolname='awo_app';"
```

---

## 4. PostgreSQL Role Configuration

The application role must not be a superuser and must work within RLS:

```sql
-- Application role (run as postgres superuser)
CREATE ROLE awo_app LOGIN PASSWORD '...' NOSUPERUSER NOCREATEDB NOCREATEROLE;

-- Grant access to the database
GRANT CONNECT ON DATABASE awo TO awo_app;
GRANT USAGE ON SCHEMA public TO awo_app;

-- Table-level grants (use a migration to automate for new tables)
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO awo_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO awo_app;

-- Default privileges for future tables
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO awo_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO awo_app;

-- The role must NOT bypass RLS (it is not a superuser, so this is default)
-- Verify:
SELECT rolbypassrls FROM pg_roles WHERE rolname = 'awo_app';
-- Must return: f
```

---

## 5. Awo Application Configuration

Set `DATABASE_URL` to point at PgBouncer, not PostgreSQL directly:

```env
# Points to PgBouncer (transaction mode)
DATABASE_URL=postgresql://awo_app:password@pgbouncer:5432/awo?sslmode=require

# pgx pool settings — align with PgBouncer pool_size
DB_MAX_OPEN_CONNS=20
DB_MAX_IDLE_CONNS=5
DB_CONN_MAX_LIFETIME=300s   # 5 min — PgBouncer server_idle_timeout is 600s
```

**Do not use `sslmode=disable`** in any environment except local development with a local PgBouncer instance.

---

## 6. Transaction Mode Restrictions

PgBouncer transaction mode has one significant restriction: **prepared statements are not supported per-connection**.

pgx (Awo's PostgreSQL driver) must be configured to use the **simple query protocol** (no prepared statements) when going through PgBouncer:

```go
// In internal/config/db.go — pgx pool config
config, err := pgxpool.ParseConfig(cfg.DatabaseURL)
if err != nil {
    return nil, fmt.Errorf("parse db config: %w", err)
}

// Disable prepared statements for PgBouncer transaction mode compatibility
config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
```

Alternatively, enable `server_reset_query_always` in PgBouncer — but the pgx approach is cleaner and avoids query text being sent twice.

---

## 7. Verifying RLS Works Through PgBouncer

After deploying, verify tenant isolation holds:

```sql
-- Connect via PgBouncer as awo_app
-- Attempt 1: no tenant context set — should return 0 rows (RLS blocks all)
SELECT COUNT(*) FROM invoice;
-- Expected: 0

-- Attempt 2: set tenant context and verify rows are scoped
SELECT set_tenant_context('018e1234-5678-7abc-def0-123456789abc');
SELECT COUNT(*) FROM invoice;
-- Expected: rows for that tenant only

-- Attempt 3: new connection (PgBouncer recycles) — no tenant context
-- Must return 0 again, confirming SET LOCAL reset correctly
SELECT COUNT(*) FROM invoice;
-- Expected: 0
```

---

## 8. Monitoring

PgBouncer exposes stats via the admin interface:

```bash
# Connect to PgBouncer admin
psql -h pgbouncer -p 5432 -U pgbouncer_admin pgbouncer

-- Current pool state
SHOW POOLS;

-- Client connections
SHOW CLIENTS;

-- Server (backend) connections
SHOW SERVERS;

-- Aggregate stats
SHOW STATS;
```

Key metrics to watch:
- `cl_waiting`: clients waiting for a connection. Spikes → increase `default_pool_size` or add PgBouncer instances
- `sv_idle`: idle backend connections. Too high → reduce `min_pool_size`
- `avg_wait_time`: average client wait. Should be <1ms in normal operation

Prometheus exporter: `pgbouncer_exporter` — scrape `/metrics` and alert on `pgbouncer_client_waiting > 10` for sustained periods.

---

## 9. Docker Compose (Local Development)

```yaml
services:
  pgbouncer:
    image: bitnami/pgbouncer:latest
    environment:
      POSTGRESQL_HOST: postgres
      POSTGRESQL_PORT: 5432
      POSTGRESQL_DATABASE: awo
      POSTGRESQL_USERNAME: awo_app
      POSTGRESQL_PASSWORD: localpassword
      PGBOUNCER_POOL_MODE: transaction          # CRITICAL
      PGBOUNCER_MAX_CLIENT_CONN: 100
      PGBOUNCER_DEFAULT_POOL_SIZE: 10
      PGBOUNCER_SERVER_TLS_SSLMODE: disable     # local only
      PGBOUNCER_CLIENT_TLS_SSLMODE: disable     # local only
    ports:
      - "5432:5432"
    depends_on:
      postgres:
        condition: service_healthy

  postgres:
    image: postgres:16
    environment:
      POSTGRES_DB: awo
      POSTGRES_USER: awo_app
      POSTGRES_PASSWORD: localpassword
    ports:
      - "5433:5432"   # expose direct port for migrations only
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U awo_app -d awo"]
      interval: 5s
      timeout: 5s
      retries: 5
```

Migrations connect directly to PostgreSQL (`5433`) — they need DDL privileges that `awo_app` may not have. Use a separate migration role or connect directly.

---

## Related Documents

- [Deployment](deployment.md) — Kubernetes manifests, rolling deploy
- [Tenant Model](../06-tenancy/tenant-model.md) — how `set_tenant_context()` is called
- [Architecture Invariants](../02-architecture/invariants.md) — INV-007
- [Local Development Setup](local-development.md) — full docker compose stack
