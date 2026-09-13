# Environment Variables Reference

**Classification:** Reference — Tier 2
**Owner:** `20-devops/ENVIRONMENT_VARIABLES.md`
**Status:** Living document

---

## Overview

All configuration is via environment variables. No config files are read at runtime. The `Config` struct in `internal/config/config.go` is the typed authority for all variables. Missing required variables cause fatal startup failure.

---

## Required Variables

The server will not start if these are missing.

| Variable | Example | Description |
|----------|---------|-------------|
| `DATABASE_URL` | `postgres://user:pass@host:5432/awo?sslmode=require` | PostgreSQL connection string (pgxpool DSN) |
| `REDIS_URL` | `redis://:{password}@host:6379/0` | Redis connection string |
| `JWT_SECRET` | 64-char random hex | JWT signing key |
| `SESSION_SECRET` | 64-char random hex | Session token signing key |
| `TEMPORAL_HOST` | `temporal-frontend:7233` | Temporal gRPC endpoint |
| `TEMPORAL_NAMESPACE` | `awo-production` | Temporal namespace |

---

## Optional / Defaulted

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | API server listen port |
| `HEALTH_PORT` | `8081` | Health check server port |
| `METRICS_PORT` | `9090` | Prometheus metrics port |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | `json` | `json` (production), `text` (development) |
| `DATABASE_MAX_CONNS` | `20` | pgxpool max open connections per pod |
| `DATABASE_MIN_CONNS` | `5` | pgxpool min idle connections |
| `DATABASE_CONN_LIFETIME` | `1h` | Max connection lifetime |
| `OUTBOX_RELAY_BATCH_SIZE` | `100` | Events per outbox relay cycle |
| `OUTBOX_RELAY_INTERVAL` | `1s` | Poll interval for outbox relay goroutine |
| `SESSION_TTL` | `28800` | Session TTL in seconds (8h) |
| `RATE_LIMIT_RPM` | `1000` | Requests per minute per user (rate limit) |
| `ENV` | `production` | `development` / `staging` / `production` |

---

## Temporal Worker Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `TEMPORAL_MAX_CONCURRENT_WORKFLOW_TASKS` | `10` | Concurrent workflow task pollers per worker |
| `TEMPORAL_MAX_CONCURRENT_ACTIVITIES` | `50` | Concurrent activity executions per worker |
| `TEMPORAL_TASK_QUEUE_PREFIX` | `awo` | Task queue name prefix (e.g., `awo.finance.invoice.approval`) |

---

## Observability Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | — | OTLP gRPC endpoint for traces (`http://otel-collector:4317`) |
| `OTEL_SERVICE_NAME` | `awo-erp` | Service name in traces |
| `OTEL_TRACES_SAMPLER` | `parentbased_traceidratio` | Sampling strategy |
| `OTEL_TRACES_SAMPLER_ARG` | `0.1` | Sampling ratio (10% in production) |

---

## Secrets Management

In production, secrets are injected as environment variables from Kubernetes Secrets populated by External Secrets Operator (Vault or AWS Secrets Manager):

```yaml
# ExternalSecret syncs from Vault
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: awo-erp-secrets
spec:
  secretStoreRef:
    name: vault-backend
    kind: ClusterSecretStore
  target:
    name: awo-erp-secrets
  data:
    - secretKey: DATABASE_URL
      remoteRef:
        key: secret/awo/production/app
        property: database_url
    - secretKey: JWT_SECRET
      remoteRef:
        key: secret/awo/production/app
        property: jwt_secret
```

```yaml
# Pod spec
spec:
  containers:
  - name: awo-erp-server
    envFrom:
    - secretRef:
        name: awo-erp-secrets
    - configMapRef:
        name: awo-erp-config
```

Never put secrets in:
- Container images
- Kubernetes ConfigMaps (not encrypted at rest)
- Helm values files committed to git

---

## Local Development (.env)

```bash
# .env (must be in .gitignore)
DATABASE_URL=postgres://awo:dev@localhost:5432/awo_dev?sslmode=disable
REDIS_URL=redis://localhost:6379/0
TEMPORAL_HOST=localhost:7233
TEMPORAL_NAMESPACE=default
JWT_SECRET=dev-jwt-secret-not-for-production
SESSION_SECRET=dev-session-secret-not-for-production
ENV=development
LOG_FORMAT=text
LOG_LEVEL=debug
```

---

## References

- [`18-security/SECRET_MANAGEMENT.md`](../18-security/SECRET_MANAGEMENT.md) — Secret management policy
- `internal/config/config.go` — Typed Config struct
