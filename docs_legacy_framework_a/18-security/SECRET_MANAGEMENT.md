> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Secret Management

**Classification:** Specification — Tier 1
**Owner:** `18-security/SECRET_MANAGEMENT.md`
**Status:** Frozen at v1.0

---

## Purpose

This document specifies how secrets are managed in the Awo Framework — how they are loaded, where they MUST NOT appear, and the production secret management approach.

---

## 1. Rule: No Hard-Coded Secrets

Secrets MUST NEVER appear in source code, including:
- Go source files
- `docker-compose.yml`
- Kubernetes manifests (unless using sealed secrets or external secret operator)
- Migration files
- Test fixtures checked into git

Violation: immediate security review required. Assume the secret is compromised if it was ever committed.

---

## 2. Config Struct

All secrets are loaded from environment variables into the typed `Config` struct at startup:

```go
// internal/config/config.go
type Config struct {
    // Server
    Port int `env:"PORT,required"`

    // Database
    DatabaseURL string `env:"DATABASE_URL,required"`

    // Redis
    RedisURL string `env:"REDIS_URL,required"`

    // Temporal
    TemporalHost      string `env:"TEMPORAL_HOST,required"`
    TemporalNamespace string `env:"TEMPORAL_NAMESPACE" envDefault:"default"`

    // Authentication — SENSITIVE
    JWTSecret     string `env:"JWT_SECRET,required"`
    SessionSecret string `env:"SESSION_SECRET,required"`

    // Logging
    LogLevel slog.Level `env:"LOG_LEVEL" envDefault:"info"`
}
```

Startup validates all `required` fields — missing secrets cause fatal startup failure. Silent defaults to insecure values are prohibited.

---

## 3. Environment Variables (Development)

Development uses `.env` file loaded by the local runner:

```bash
# .env (NEVER commit this file)
DATABASE_URL=postgres://awo:devpassword@localhost:5432/awo_dev?sslmode=disable
REDIS_URL=redis://localhost:6379
TEMPORAL_HOST=localhost:7233
JWT_SECRET=dev-jwt-secret-change-in-production
SESSION_SECRET=dev-session-secret-change-in-production
PORT=8080
LOG_LEVEL=debug
```

`.env` MUST be in `.gitignore`. The repository includes `.env.example` with placeholder values and documentation comments.

---

## 4. Production Secret Management

Production uses HashiCorp Vault (or an equivalent: AWS Secrets Manager, GCP Secret Manager):

```
Vault path: secret/awo/{environment}/app
Keys: DATABASE_URL, REDIS_URL, JWT_SECRET, SESSION_SECRET, ...
```

Secrets are injected as environment variables at pod start via the Vault Agent sidecar or the External Secrets Operator:

```yaml
# Kubernetes: ExternalSecret
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: awo-app-secrets
spec:
  secretStoreRef:
    name: vault-backend
    kind: ClusterSecretStore
  target:
    name: awo-app-secrets
  data:
    - secretKey: DATABASE_URL
      remoteRef:
        key: secret/awo/production/app
        property: database_url
```

The pod spec mounts the Kubernetes `Secret` as environment variables:

```yaml
envFrom:
  - secretRef:
      name: awo-app-secrets
```

---

## 5. Secret Rotation

| Secret | Rotation Frequency | Process |
|--------|-------------------|---------|
| `JWT_SECRET` | 90 days | Rolling rotation (accept old+new during overlap period) |
| `SESSION_SECRET` | 90 days | Forces all active sessions to re-authenticate |
| `DATABASE_URL` password | 180 days | PgBouncer reload required |
| `REDIS_URL` password | 180 days | Rolling restart required |

Session token invalidation on `SESSION_SECRET` rotation: all `session:*` keys in Redis are deleted. All users are logged out. Schedule during low-traffic window.

---

## 6. What MUST NEVER Appear in Logs

```go
// These fields MUST be excluded from all log entries:
config.JWTSecret       // JWT signing key
config.SessionSecret   // Session signing key
config.DatabaseURL     // Contains password
config.RedisURL        // Contains password (if set)
```

The `Config` struct MUST implement a custom `String()` / `LogValue()` method that masks sensitive fields:

```go
func (c Config) LogValue() slog.Value {
    return slog.GroupValue(
        slog.Int("port", c.Port),
        slog.String("log_level", c.LogLevel.String()),
        slog.String("database_url", maskURL(c.DatabaseURL)),
        slog.String("redis_url", maskURL(c.RedisURL)),
        // JWTSecret and SessionSecret: NOT included
    )
}
```

`maskURL` replaces the password component with `***`: `postgres://user:***@host/db`.

---

## 7. Test Secrets

Test secrets MUST use clearly fake values:
- `JWT_SECRET=test-jwt-secret-not-for-production`
- Short, recognisable, obviously fake

Test database URLs point to ephemeral test databases, never production.

---

## Normative Requirements

- Secrets MUST be loaded from environment variables. Hard-coded secrets in code are prohibited.
- Missing required secrets MUST cause fatal startup failure.
- Secrets MUST NOT appear in log entries.
- `.env` files MUST be in `.gitignore`.
- Production secrets MUST be managed by a dedicated secret store (Vault or equivalent).

---

## References

- [`18-security/SECURITY_MODEL.md`](SECURITY_MODEL.md) — Overall security model
- [`17-observability/LOGGING_SPEC.md`](../17-observability/LOGGING_SPEC.md) — Log field exclusions
