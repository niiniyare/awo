> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Configuration"
id: cfg-001
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, operators]
since: "1.0"
normative-level: normative
related:
  - "[Startup Sequence](../03-kernel/startup-sequence.md)"
  - "[Observability](../13-observability/observability.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Configuration

**CFG-001 | Status: Accepted | Stability: Stable**

This document specifies the typed configuration struct, all environment variables, validation rules, secrets management, and fail-fast startup behavior.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Configuration Principles

- All configuration via environment variables — no configuration files in production
- Config loaded into a typed struct at startup — not passed around as raw strings
- All required fields validated at startup — process exits on invalid config (fail-fast)
- Sensitive fields never logged — enforced by omitting them from log output
- No insecure defaults — missing required secrets cause immediate startup failure

---

## 2. Config Struct

```go
// internal/config/config.go
package config

type Config struct {
    // === Server ===
    Port        int    `env:"PORT,required"`
    Environment string `env:"ENVIRONMENT,required"` // "development", "staging", "production"

    // === Database ===
    DatabaseURL string `env:"DATABASE_URL,required"`
    // PgBouncer MUST be in transaction mode — validated at startup via session-level set_config check

    // === Redis ===
    RedisURL string `env:"REDIS_URL,required"`

    // === Temporal ===
    TemporalHost      string `env:"TEMPORAL_HOST,required"`
    TemporalNamespace string `env:"TEMPORAL_NAMESPACE,required"`

    // === Security (sensitive — never log these fields) ===
    JWTSecret        string `env:"JWT_SECRET,required"`
    EncryptionKey    string `env:"ENCRYPTION_KEY,required"` // 32-byte hex for AES-256

    // === Session ===
    SessionTTLMinutes    int  `env:"SESSION_TTL_MINUTES" envDefault:"480"`  // 8 hours
    SessionSlidingExpiry bool `env:"SESSION_SLIDING_EXPIRY" envDefault:"true"`

    // === Tenant ===
    DefaultTimezone string `env:"DEFAULT_TIMEZONE" envDefault:"Africa/Nairobi"`
    DefaultLocale   string `env:"DEFAULT_LOCALE" envDefault:"en-KE"`

    // === Rate Limiting ===
    RateLimitEnabled    bool `env:"RATE_LIMIT_ENABLED" envDefault:"true"`
    RateLimitPerMinute  int  `env:"RATE_LIMIT_PER_MINUTE" envDefault:"60"`

    // === Observability ===
    LogLevel        string `env:"LOG_LEVEL" envDefault:"info"` // debug, info, warn, error
    MetricsEnabled  bool   `env:"METRICS_ENABLED" envDefault:"true"`
    TracingEnabled  bool   `env:"TRACING_ENABLED" envDefault:"false"`
    OTELEndpoint    string `env:"OTEL_ENDPOINT"`  // required if TracingEnabled

    // === CORS ===
    AllowedOrigins []string `env:"ALLOWED_ORIGINS" envSeparator:","`
    // Format: "https://*.example.com,https://admin.example.com"

    // === Feature Flags ===
    FeatureFlagCacheTTLSeconds int `env:"FLAG_CACHE_TTL_SECONDS" envDefault:"300"`

    // === File Storage (optional) ===
    StorageProvider string `env:"STORAGE_PROVIDER"` // "local", "s3", "gcs"
    StorageBucket   string `env:"STORAGE_BUCKET"`
    StorageRegion   string `env:"STORAGE_REGION"`
}
```

---

## 3. Environment Variables Reference

### Required (no defaults)

| Variable | Description |
|---|---|
| `PORT` | HTTP listen port (e.g., `8080`) |
| `ENVIRONMENT` | Deployment environment: `development`, `staging`, `production` |
| `DATABASE_URL` | PostgreSQL connection string via PgBouncer |
| `REDIS_URL` | Redis connection string |
| `TEMPORAL_HOST` | Temporal server address (e.g., `temporal:7233`) |
| `TEMPORAL_NAMESPACE` | Temporal namespace (e.g., `awo-production`) |
| `JWT_SECRET` | Minimum 32 bytes, cryptographically random |
| `ENCRYPTION_KEY` | 64-character hex (32 bytes decoded); used for field-level encryption |

### Optional (with defaults)

| Variable | Default | Description |
|---|---|---|
| `SESSION_TTL_MINUTES` | `480` | Session expiry in minutes (8 hours) |
| `SESSION_SLIDING_EXPIRY` | `true` | Extend TTL on each request |
| `DEFAULT_TIMEZONE` | `Africa/Nairobi` | Tenant default timezone |
| `DEFAULT_LOCALE` | `en-KE` | Tenant default locale |
| `RATE_LIMIT_ENABLED` | `true` | Enable rate limiting |
| `RATE_LIMIT_PER_MINUTE` | `60` | Requests per minute per user |
| `LOG_LEVEL` | `info` | Minimum log level |
| `METRICS_ENABLED` | `true` | Expose `/metrics` endpoint |
| `TRACING_ENABLED` | `false` | Enable OTEL tracing |
| `FLAG_CACHE_TTL_SECONDS` | `300` | Feature flag Redis cache TTL |

---

## 4. Config Loading

```go
// internal/config/loader.go
func Load() (*Config, error) {
    cfg := &Config{}
    if err := env.Parse(cfg); err != nil {
        return nil, fmt.Errorf("config: parse env: %w", err)
    }
    if err := validate(cfg); err != nil {
        return nil, fmt.Errorf("config: %w", err)
    }
    return cfg, nil
}

func validate(cfg *Config) error {
    var errs []string

    // Environment validation
    validEnvs := map[string]bool{"development": true, "staging": true, "production": true}
    if !validEnvs[cfg.Environment] {
        errs = append(errs, fmt.Sprintf("ENVIRONMENT must be one of: development, staging, production; got %q", cfg.Environment))
    }

    // Port range
    if cfg.Port < 1 || cfg.Port > 65535 {
        errs = append(errs, fmt.Sprintf("PORT must be 1-65535; got %d", cfg.Port))
    }

    // Secret length (minimum security requirements)
    if len(cfg.JWTSecret) < 32 {
        errs = append(errs, "JWT_SECRET must be at least 32 characters")
    }

    // Encryption key: must be 64 hex chars (32 bytes)
    if len(cfg.EncryptionKey) != 64 {
        errs = append(errs, "ENCRYPTION_KEY must be exactly 64 hex characters (32 bytes)")
    }

    // OTEL endpoint required when tracing enabled
    if cfg.TracingEnabled && cfg.OTELEndpoint == "" {
        errs = append(errs, "OTEL_ENDPOINT required when TRACING_ENABLED=true")
    }

    if len(errs) > 0 {
        return fmt.Errorf("invalid configuration:\n  - %s", strings.Join(errs, "\n  - "))
    }
    return nil
}
```

On validation failure, the startup sequence logs all errors and exits with code 1:

```
FATAL config: invalid configuration:
  - JWT_SECRET must be at least 32 characters
  - OTEL_ENDPOINT required when TRACING_ENABLED=true
```

Multiple errors are reported together — never fail on the first and hide the rest.

---

## 5. Secrets Management

### Development

Use a `.env` file loaded by the shell or a tool like `direnv`. The `.env` file MUST NOT be committed to version control.

```bash
# .env (local development only — gitignored)
DATABASE_URL=postgres://awo:awo@localhost:5432/awo_dev?sslmode=disable
REDIS_URL=redis://localhost:6379/0
JWT_SECRET=dev-only-secret-do-not-use-in-production-XXXXX
ENCRYPTION_KEY=0000000000000000000000000000000000000000000000000000000000000000
```

### Staging and Production

Secrets MUST be managed via HashiCorp Vault or equivalent. Inject secrets as environment variables at process start — never pass them through CI/CD pipeline artifacts or container image build args.

Kubernetes deployment pattern:

```yaml
# In Kubernetes: reference secrets, not literal values
env:
  - name: JWT_SECRET
    valueFrom:
      secretKeyRef:
        name: awo-secrets
        key: jwt-secret
  - name: DATABASE_URL
    valueFrom:
      secretKeyRef:
        name: awo-secrets
        key: database-url
```

---

## 6. Logging Config (Safe Subset)

At startup, the Config is logged for debugging — sensitive fields MUST be omitted:

```go
func (c *Config) LogSafe() map[string]any {
    return map[string]any{
        "port":            c.Port,
        "environment":     c.Environment,
        "temporal_host":   c.TemporalHost,
        "temporal_ns":     c.TemporalNamespace,
        "log_level":       c.LogLevel,
        "rate_limit":      c.RateLimitEnabled,
        "session_ttl_min": c.SessionTTLMinutes,
        "tracing":         c.TracingEnabled,
        // JWTSecret, EncryptionKey, DatabaseURL, RedisURL — intentionally absent
    }
}
```

`DatabaseURL` and `RedisURL` are omitted from logs even though they are not secrets by themselves — they may contain embedded credentials (in DSN format) and MUST be treated as sensitive.

---

## 7. PgBouncer Transaction Mode Verification

The startup sequence verifies PgBouncer is in transaction mode — session mode breaks RLS:

```go
// Executed during PostgreSQL initialization (Step 2 of startup sequence)
func verifyPgBouncerMode(ctx context.Context, pool *pgxpool.Pool) error {
    var result string
    err := pool.QueryRow(ctx, "SELECT current_setting('pgbouncer.pool_mode', true)").Scan(&result)
    if err != nil || result != "transaction" {
        // Also check by attempting a session-level set_config and verifying it resets
        // (session mode would persist the config; transaction mode would reset it)
        return fmt.Errorf("PgBouncer must be in transaction mode; got %q", result)
    }
    return nil
}
```

This verification runs before the EntityRegistry is initialized. Startup fails if transaction mode is not confirmed.

---

## Related Documents

- [Startup Sequence](../03-kernel/startup-sequence.md) — when config is loaded (Step 1)
- [Observability](../13-observability/observability.md) — log level and metrics configuration
- [Tenant Model](../06-tenancy/tenant-model.md) — PgBouncer transaction mode requirement
- [Glossary](../GLOSSARY.md) — Config Struct, Secrets Management
