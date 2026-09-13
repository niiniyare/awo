> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Migration Runner"
id: ops-011
status: accepted
category: GUIDE
stability: STABLE
audience: [operators, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Migrations](migrations.md)"
  - "[Deployment](deployment.md)"
  - "[Local Development Setup](local-development.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Migration Runner

**OPS-011 | Status: Accepted | Stability: Stable**

How the `cmd/migrate` process works, how to run it in CI/CD, and how to manage the migration lifecycle safely.

---

## 1. Why a Separate Process

Migrations run as a **separate binary** (`cmd/migrate/`), not inside the API server. This is intentional:

- **Safety**: API server cannot auto-migrate on startup. Silent schema changes in production are forbidden (see LAW-012).
- **CI gate**: Migrations can be reviewed, tested in staging, and approved before production deployment.
- **Rollback isolation**: If a migration fails, the API server keeps running on the last known-good schema.
- **Privilege separation**: The migration role can have DDL privileges; the `awo_app` role used by the API server does not.

---

## 2. Migration File Format

Location: `db/migration/`

Files are named with a 14-digit Unix timestamp + description slug:

```
db/migration/
    20241201120000_initial_schema.up.sql
    20241201120000_initial_schema.down.sql
    20241215143022_create_contact.up.sql
    20241215143022_create_contact.down.sql
    20250103091500_add_invoice_status_index.up.sql
    20250103091500_add_invoice_status_index.down.sql
```

**Rules**:
- Every `.up.sql` must have a corresponding `.down.sql`
- `.down.sql` must exactly reverse the `.up.sql` (not approximately — exactly)
- Timestamps must be monotonically increasing — use `date +%Y%m%d%H%M%S` to generate
- Never edit a migration that has already been applied to any environment

---

## 3. Running Migrations

### Local Development

```bash
# Apply all pending migrations
go run ./cmd/migrate up

# Apply exactly N migrations
go run ./cmd/migrate up -n 1

# Roll back last N migrations
go run ./cmd/migrate down -n 1

# Show current migration version
go run ./cmd/migrate version

# Show migration status (applied / pending)
go run ./cmd/migrate status
```

The migrate command reads `DATABASE_URL` from the environment:

```bash
export DATABASE_URL="postgresql://awo_migrate:password@localhost:5433/awo?sslmode=disable"
go run ./cmd/migrate up
```

Note: connect directly to PostgreSQL (`5433` in the docker compose setup), **not** through PgBouncer. The migration role needs DDL privileges and long-running transactions that conflict with PgBouncer's transaction mode.

### Docker

```bash
docker run --rm \
  -e DATABASE_URL="postgresql://awo_migrate:password@postgres:5432/awo?sslmode=require" \
  awo-migrate:latest up
```

### Kubernetes Job (production)

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: awo-migrate-v1-2-3
  namespace: awo
spec:
  backoffLimit: 0        # fail immediately — never retry silently
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: migrate
          image: your-registry/awo-migrate:v1.2.3
          command: ["./migrate", "up"]
          env:
            - name: DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: awo-db-secrets
                  key: migration-url    # migration role, not app role
          resources:
            requests:
              memory: 64Mi
              cpu: 100m
```

---

## 4. CI/CD Pipeline Integration

Migrations run **before** the API server is deployed:

```
┌─────────────────────────────────────────────────┐
│  Build Stage                                    │
│    - go build → awo-server binary               │
│    - go build → awo-migrate binary              │
│    - Docker images tagged with git SHA          │
└───────────────────────┬─────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────┐
│  Staging Deploy                                 │
│    1. Run migration Job (must complete ✓)       │
│    2. Run smoke test on staging DB schema       │
│    3. Deploy API server (rolling update)        │
└───────────────────────┬─────────────────────────┘
                        │ (manual approval gate)
┌───────────────────────▼─────────────────────────┐
│  Production Deploy                              │
│    1. Run migration Job (must complete ✓)       │
│    2. Wait for Job to succeed                   │
│    3. Deploy API server (rolling update)        │
└─────────────────────────────────────────────────┘
```

If the migration Job fails → deployment pipeline stops. API server keeps running on old schema.

Example GitHub Actions step:

```yaml
- name: Run migrations
  run: |
    kubectl apply -f k8s/migrate-job.yaml
    kubectl wait --for=condition=complete job/awo-migrate-${{ github.sha }} \
      --timeout=300s \
      --namespace=awo
```

---

## 5. Migration Role vs Application Role

Two separate database roles:

| Role | Privileges | Used by |
|---|---|---|
| `awo_migrate` | DDL (CREATE TABLE, ALTER TABLE, DROP INDEX, etc.) | `cmd/migrate` only |
| `awo_app` | DML only (SELECT, INSERT, UPDATE, DELETE) | API server (via PgBouncer) |

Never give `awo_app` DDL privileges. If an API bug executes malformed SQL, it cannot alter the schema.

```sql
-- Create migration role
CREATE ROLE awo_migrate LOGIN PASSWORD '...' NOSUPERUSER NOCREATEDB NOCREATEROLE;
GRANT CONNECT ON DATABASE awo TO awo_migrate;
GRANT ALL ON SCHEMA public TO awo_migrate;

-- Allow migration role to manage RLS policies
ALTER ROLE awo_migrate CREATEROLE;  -- only if needed for GRANT statements in migrations
```

---

## 6. Migration Tracking Table

`golang-migrate` creates a `schema_migrations` table to track applied versions:

```sql
CREATE TABLE schema_migrations (
    version    bigint PRIMARY KEY,
    dirty      boolean NOT NULL
);
```

- `version`: the timestamp from the filename (e.g. `20241215143022`)
- `dirty`: `true` if the last migration failed mid-execution — requires manual intervention

### Dirty State Recovery

If a migration fails and leaves `dirty = true`:

```bash
# Check current state
go run ./cmd/migrate version
# Output: 20241215143022 (dirty)

# Option 1: Fix the SQL and force the version back
go run ./cmd/migrate force 20241201120000  # resets to last known-good version
# Then: fix the migration file, re-run

# Option 2: Roll back manually in psql, then force version
psql $DATABASE_URL
-- Manually undo what the failed migration did
-- Then:
go run ./cmd/migrate force 20241201120000
```

Never set `dirty = false` directly in `schema_migrations` without actually rolling back the partial changes.

---

## 7. Zero-Downtime Migration Checklist

Before writing a migration, verify:

| Change | Zero-downtime approach |
|---|---|
| Add column | `ADD COLUMN ... DEFAULT NULL` — safe. Add NOT NULL constraint later after backfill |
| Add index | `CREATE INDEX CONCURRENTLY` — no table lock |
| Drop column | Add to "ignored" list in app first, deploy, then drop column in next release |
| Rename column | Add new column + dual-write + backfill + switch reads + drop old |
| Add FK constraint | Add column first (nullable), backfill, then add FK |
| Add CHECK constraint | `ADD CONSTRAINT ... NOT VALID` first, then `VALIDATE CONSTRAINT` (no full-table lock) |
| Change column type | Requires column rename pattern — never single-step in production |

---

## 8. RLS Setup in New Table Migrations

Every new tenant-scoped table **must** include RLS setup in its `.up.sql`:

```sql
-- db/migration/20250315090000_create_finance_invoice.up.sql

CREATE TABLE finance_invoice (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid NOT NULL REFERENCES tenants(id),
    number      varchar(50) NOT NULL,
    customer    uuid NOT NULL REFERENCES finance_customer(id),
    status      varchar(20) NOT NULL DEFAULT 'Draft'
                CHECK (status IN ('Draft', 'Submitted', 'Paid', 'Cancelled')),
    total_kes   numeric(20,4) NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT NOW(),
    updated_at  timestamptz NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX finance_invoice_tenant_id_idx ON finance_invoice (tenant_id);
CREATE INDEX finance_invoice_customer_idx  ON finance_invoice (customer);
CREATE INDEX finance_invoice_status_idx    ON finance_invoice (status);

-- RLS (mandatory for all tenant-scoped tables)
ALTER TABLE finance_invoice ENABLE ROW LEVEL SECURITY;
ALTER TABLE finance_invoice FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON finance_invoice
    USING (tenant_id = current_tenant_id());

-- Grant to application role
GRANT SELECT, INSERT, UPDATE, DELETE ON finance_invoice TO awo_app;
```

```sql
-- db/migration/20250315090000_create_finance_invoice.down.sql

DROP TABLE IF EXISTS finance_invoice;
```

---

## Related Documents

- [Migrations](migrations.md) — migration patterns, zero-downtime techniques
- [Deployment](deployment.md) — how migration Job fits into Kubernetes deploy
- [PgBouncer Setup](pgbouncer-setup.md) — why migrations connect directly, not through PgBouncer
- [Local Development Setup](local-development.md) — docker compose, local migration flow
