> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Hardening Guide"
id: sec-003
status: accepted
category: GUIDE
stability: STABLE
audience: [operators]
since: "1.0"
normative-level: normative
related:
  - "[Security Model](security-model.md)"
  - "[Deployment](../14-operations/deployment.md)"
  - "[Configuration](../12-configuration/configuration.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Hardening Guide

**SEC-003 | Status: Accepted | Stability: Stable**

Production hardening checklist for Awo deployments. Requirements marked MUST are security-critical. Items marked SHOULD are strongly recommended.

---

## 1. Network

| Item | Requirement |
|---|---|
| All external traffic over HTTPS (TLS 1.2+) | MUST |
| Internal service communication over private network | MUST |
| PostgreSQL port not exposed to internet | MUST |
| Redis port not exposed to internet | MUST |
| Temporal gRPC port not exposed to internet | MUST |
| WAF in front of public API endpoints | SHOULD |
| Rate limiting enabled (`iam.rate_limit_per_minute`) | MUST |
| `tenant_id` query param disabled in production | SHOULD |

To disable `tenant_id` query param:

```
Setting: platform.tenant_id_query_param_enabled = false
```

This forces tenant identification via `X-Tenant-ID` header or subdomain — both are harder to forge than a URL query parameter.

---

## 2. Secrets Management

| Item | Requirement |
|---|---|
| No secrets in environment variables in production | MUST (use Vault or equivalent) |
| JWT/session signing key rotated annually | SHOULD |
| Database password rotated after any personnel change | MUST |
| Redis AUTH password set | MUST |
| Webhook secrets are write-only (never returned in GET) | MUST (enforced by framework) |

HashiCorp Vault integration pattern:

```go
// cmd/server/main.go — fetch secrets at startup
vaultClient := vault.NewClient(cfg.VaultAddr)
secrets, err := vaultClient.Logical().Read("secret/data/awo/prod")
if err != nil {
    log.Fatal("failed to fetch secrets from Vault", "err", err)
}
cfg.DatabaseURL = secrets.Data["database_url"].(string)
cfg.JWTSecret   = secrets.Data["jwt_secret"].(string)
```

Never commit secrets to source control. Never log secrets.

---

## 3. Database

| Item | Requirement |
|---|---|
| RLS enabled and FORCE on all tenant-scoped tables | MUST |
| App role has no superuser or CREATEROLE privileges | MUST |
| Migration role separate from app role | SHOULD |
| `pg_audit` extension for query auditing in regulated environments | SHOULD |
| Connection from app via PgBouncer (transaction mode) | MUST |
| SSL required on PostgreSQL connections | MUST |

Verify RLS coverage:

```sql
SELECT relname, relrowsecurity, relforcerowsecurity
FROM pg_class
WHERE relkind = 'r'
  AND relname NOT IN ('tenants', 'iam_audit_log', 'schema_migrations')
  AND relrowsecurity = false;
-- Result MUST be empty
```

---

## 4. Redis

| Item | Requirement |
|---|---|
| AUTH password set | MUST |
| TLS enabled | SHOULD |
| `maxmemory-policy volatile-lru` | MUST (prevents session eviction under memory pressure when using volatile-lru) |
| Bind to private interface only | MUST |
| `rename-command FLUSHALL ""` | SHOULD (prevent accidental flush) |
| `rename-command CONFIG ""` | SHOULD (prevent runtime reconfiguration) |

---

## 5. HTTP Headers

Configure Fiber to set security headers on all responses:

```go
app.Use(helmet.New(helmet.Config{
    XSSProtection:         "1; mode=block",
    ContentTypeNosniff:    "nosniff",
    XFrameOptions:         "SAMEORIGIN",
    HSTSMaxAge:            31536000,
    HSTSExcludeSubdomains: false,
    ContentSecurityPolicy: "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'",
    ReferrerPolicy:        "strict-origin-when-cross-origin",
}))
```

`unsafe-inline` for scripts and styles is required by amis's inline schema evaluation. Scope with nonce if stricter CSP is required.

---

## 6. Session Security

| Item | Requirement |
|---|---|
| Session TTL ≤ 8 hours for standard users | SHOULD |
| Session TTL ≤ 1 hour for platform admin | MUST |
| All sessions invalidated on password change | MUST (enforced by framework) |
| Concurrent session limit configurable | SHOULD |
| `Secure` and `HttpOnly` cookie flags | MUST (if using cookie transport) |

Configure session TTL via Settings:

```
Setting: iam.session_ttl_seconds = 28800   (8 hours)
Setting: iam.platform_admin_session_ttl_seconds = 3600   (1 hour)
```

---

## 7. Audit Log

| Item | Requirement |
|---|---|
| Audit log enabled for all entity mutations | MUST (enforced by framework) |
| Audit log entries are immutable | MUST (enforced by framework — no UPDATE/DELETE on iam_audit_log table) |
| Audit log retained for ≥ 7 years (Kenya DPA 2019) | MUST for regulated data |
| Audit log exported to append-only external store | SHOULD for tamper evidence |

The `iam_audit_log` table has no `UPDATE` or `DELETE` grants for the app role. Verify:

```sql
SELECT grantee, privilege_type
FROM information_schema.role_table_grants
WHERE table_name = 'iam_audit_log'
  AND grantee = 'awo_app';
-- Only INSERT and SELECT should appear
```

---

## 8. Kubernetes Pod Security

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop: ["ALL"]
```

Network policy — restrict egress to only required services:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: awo-server-egress
spec:
  podSelector:
    matchLabels:
      app: awo-server
  policyTypes: [Egress]
  egress:
  - to: [{namespaceSelector: {matchLabels: {name: postgres}}}]
    ports: [{port: 5432}]
  - to: [{namespaceSelector: {matchLabels: {name: redis}}}]
    ports: [{port: 6379}]
  - to: [{namespaceSelector: {matchLabels: {name: temporal}}}]
    ports: [{port: 7233}]
```

---

## 9. Dependency Supply Chain

| Item | Requirement |
|---|---|
| `go.sum` committed and verified in CI | MUST |
| Dependency updates reviewed before merging | MUST |
| amis SDK pinned version — no auto-update | MUST (see ADR-003) |
| Container base image scanned for CVEs in CI | SHOULD |
| Dependabot or equivalent for Go module alerts | SHOULD |

Never `go get -u` in production without reviewing the diff. Breaking changes in indirect dependencies have caused production incidents in Awo's dependency chain.

---

## Related Documents

- [Security Model](security-model.md) — 5-layer defense model, threat entries
- [Input Validation](input-validation.md) — injection prevention, field validation
- [Configuration](../12-configuration/configuration.md) — all configurable security settings
- [Deployment](../14-operations/deployment.md) — Kubernetes deployment manifest
