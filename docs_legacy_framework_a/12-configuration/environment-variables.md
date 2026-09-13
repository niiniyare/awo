> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Environment Variables Reference"
id: cfg-003
status: accepted
category: REFERENCE
stability: STABLE
audience: [operators]
since: "1.0"
normative-level: normative
related:
  - "[Configuration](configuration.md)"
  - "[Hardening Guide](../15-security/hardening-guide.md)"
  - "[Deployment](../14-operations/deployment.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Environment Variables Reference

**CFG-003 | Status: Accepted | Stability: Stable**

Complete reference for all environment variables consumed by the Awo server process.

---

## Required Variables

These variables MUST be set. The process exits on startup if any are missing.

| Variable | Type | Example | Description |
|---|---|---|---|
| `DATABASE_URL` | string | `postgres://app:pass@pgbouncer:5432/awo?pool_max_conns=30` | PostgreSQL connection string via PgBouncer |
| `REDIS_URL` | string | `redis://:pass@redis:6379/0` | Redis connection string |
| `TEMPORAL_HOST` | string | `temporal:7233` | Temporal server address |
| `TEMPORAL_NAMESPACE` | string | `awo-production` | Temporal namespace |
| `JWT_SECRET` | string | `<256-bit hex>` | Signing key for internal JWTs (platform admin bootstrap) |
| `SESSION_ENCRYPTION_KEY` | string | `<32-byte hex>` | Key for encrypting sensitive session fields in Redis |
| `PORT` | int | `8080` | HTTP server port |

---

## Optional Variables

| Variable | Type | Default | Description |
|---|---|---|---|
| `LOG_LEVEL` | string | `info` | Logging level: `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | string | `json` | Log format: `json` (production) or `text` (development) |
| `METRICS_PORT` | int | `9090` | Prometheus metrics endpoint port |
| `TEMPORAL_TASK_QUEUE_PREFIX` | string | `` | Prefix for all Temporal task queue names (useful for staging isolation) |
| `MAX_REQUEST_BODY_SIZE` | string | `4MB` | Maximum HTTP request body size |
| `REQUEST_TIMEOUT` | string | `30s` | HTTP request timeout (excludes streaming endpoints) |
| `CORS_ALLOWED_ORIGINS` | string | `*` | Comma-separated allowed CORS origins; use explicit domains in production |
| `SESSION_TTL` | string | `8h` | Default session TTL; can be overridden per tenant via settings |
| `SESSION_SLIDING_WINDOW` | bool | `true` | Reset session TTL on each request |
| `RATE_LIMIT_TENANT_RPM` | int | `6000` | Tenant-wide requests per minute limit |
| `RATE_LIMIT_USER_RPM` | int | `600` | Per-user requests per minute limit |
| `RATE_LIMIT_AUTH_RPM` | int | `20` | Authentication endpoint requests per minute (per IP) |
| `OUTBOX_POLL_INTERVAL` | string | `500ms` | How often the outbox relay checks for pending events |
| `OUTBOX_BATCH_SIZE` | int | `50` | Maximum events processed per outbox poll cycle |
| `FEATURE_FLAG_CACHE_TTL` | string | `5m` | Feature flag Redis cache TTL |
| `PAGE_SCHEMA_CACHE_TTL` | string | `5m` | amis page schema Redis cache TTL |
| `PGBOUNCER_ADMIN_URL` | string | `` | PgBouncer admin console URL for pool stats (observability only) |

---

## Vault-Injected Variables (Production)

In production, these are injected by HashiCorp Vault's agent sidecar. They MUST NOT be hard-coded in deployment manifests.

| Variable | Vault path |
|---|---|
| `DATABASE_URL` | `secret/awo/production/database_url` |
| `REDIS_URL` | `secret/awo/production/redis_url` |
| `JWT_SECRET` | `secret/awo/production/jwt_secret` |
| `SESSION_ENCRYPTION_KEY` | `secret/awo/production/session_key` |
| `TEMPORAL_AUTH_TOKEN` | `secret/awo/production/temporal_token` |
| Email provider credentials | `secret/awo/production/email_*` |
| Payment gateway keys | `secret/awo/production/payment_*` |
| KRA eTIMS API key | `secret/awo/production/etims_api_key` |

---

## Optional External Integrations

| Variable | Description |
|---|---|
| `EMAIL_PROVIDER` | `sendgrid`, `ses`, `smtp` |
| `SENDGRID_API_KEY` | SendGrid API key (when `EMAIL_PROVIDER=sendgrid`) |
| `SMTP_HOST` | SMTP hostname (when `EMAIL_PROVIDER=smtp`) |
| `SMTP_PORT` | SMTP port |
| `SMTP_USERNAME` | SMTP username |
| `SMTP_PASSWORD` | SMTP password |
| `SMTP_FROM` | From address for outgoing email |
| `STORAGE_PROVIDER` | `s3`, `gcs`, `local` |
| `AWS_ACCESS_KEY_ID` | AWS credentials (when `STORAGE_PROVIDER=s3`) |
| `AWS_SECRET_ACCESS_KEY` | AWS secret |
| `AWS_S3_BUCKET` | S3 bucket name |
| `AWS_REGION` | AWS region |
| `ETIMS_API_URL` | KRA eTIMS API base URL |
| `ETIMS_API_KEY` | KRA eTIMS API key |
| `SENTRY_DSN` | Sentry error tracking DSN |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OpenTelemetry collector endpoint |

---

## Kubernetes ConfigMap / Secret Split

```yaml
# configmap.yaml — non-sensitive, version-controlled
apiVersion: v1
kind: ConfigMap
metadata:
  name: awo-config
data:
  PORT: "8080"
  LOG_LEVEL: "info"
  LOG_FORMAT: "json"
  TEMPORAL_HOST: "temporal.temporal.svc.cluster.local:7233"
  TEMPORAL_NAMESPACE: "awo-production"
  RATE_LIMIT_TENANT_RPM: "6000"
  OUTBOX_POLL_INTERVAL: "500ms"

---
# secret.yaml — sensitive, never commit values, use Vault or sealed-secrets
apiVersion: v1
kind: Secret
metadata:
  name: awo-secrets
type: Opaque
stringData:
  DATABASE_URL: "{{ from Vault }}"
  REDIS_URL: "{{ from Vault }}"
  JWT_SECRET: "{{ from Vault }}"
  SESSION_ENCRYPTION_KEY: "{{ from Vault }}"
```

---

## Validation at Startup

The process validates all required variables and type-checks optional ones at startup. Validation failure produces a clear error message and exits immediately — no silent bad defaults.

```
FATAL: missing required environment variable: DATABASE_URL
FATAL: invalid SESSION_TTL "8hours": expected duration format (e.g. "8h", "30m")
```

Sensitive variables (`JWT_SECRET`, `SESSION_ENCRYPTION_KEY`) are validated for minimum length (32 bytes / 64 hex chars) but never logged.

---

## Related Documents

- [Configuration](configuration.md) — typed config struct that wraps these variables
- [Hardening Guide](../15-security/hardening-guide.md) — production secret management
- [Deployment](../14-operations/deployment.md) — Kubernetes manifest with ConfigMap/Secret reference
