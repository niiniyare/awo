> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "API Clients (Machine-to-Machine Auth)"
id: iam-006
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, operators]
since: "1.0"
normative-level: normative
related:
  - "[Authentication](authentication.md)"
  - "[RBAC](rbac.md)"
  - "[Sessions](sessions.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# API Clients (Machine-to-Machine Auth)

**IAM-006 | Status: Accepted | Stability: Stable**

This document specifies API client authentication for machine-to-machine integrations: client credential creation, token issuance, scope restriction, and rotation.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. API Client vs User Session

| Property | User Session | API Client Token |
|---|---|---|
| Authentication | Email + password (+ MFA) | Client ID + Client Secret |
| Issued to | Human user | Machine/service |
| TTL | 8 hours (configurable) | 90 days (configurable) |
| Refresh | Re-authenticate | Rotate secret |
| MFA required | Per tenant policy | Never (machine auth) |
| Rate limit | Per-user limit | Separate per-client limit |

---

## 2. Creating an API Client

```
POST /api/v1/admin/api-clients
Authorization: Bearer {admin_session_token}
Body: {
  "name":   "ERP Integration Service",
  "scopes": ["finance_invoice:read", "finance_invoice:create"],
  "rate_limit_per_minute": 300
}
```

Response:

```json
{
  "data": {
    "client_id":     "awo_client_018e1b2c...",
    "client_secret": "awo_secret_...",
    "name":          "ERP Integration Service",
    "scopes":        ["finance_invoice:read", "finance_invoice:create"],
    "created_at":    "2024-03-15T09:30:00Z"
  }
}
```

The `client_secret` is returned once only — it is stored hashed (bcrypt cost 10). The caller MUST store it securely. If lost, rotate (§5).

---

## 3. Token Issuance

API clients authenticate to obtain a session token:

```
POST /api/v1/auth/token
Content-Type: application/json
Body: {
  "grant_type":    "client_credentials",
  "client_id":     "awo_client_018e1b2c...",
  "client_secret": "awo_secret_..."
}
```

Response:

```json
{
  "data": {
    "access_token": "awo_...",
    "token_type":   "Bearer",
    "expires_in":   7776000,
    "scopes":       ["finance_invoice:read", "finance_invoice:create"]
  }
}
```

The `access_token` is a standard Awo session token stored in Redis under `session:{token}`. The Redis entry includes the client's scope list instead of a user role set.

### Token Storage

```
Redis key: session:{token}
Value:
{
  "type":       "api_client",
  "client_id":  "awo_client_018e1b2c...",
  "tenant_id":  "...",
  "scopes":     ["finance_invoice:read", "finance_invoice:create"],
  "issued_at":  "...",
  "expires_at": "..."
}
```

---

## 4. Scope-Based Authorization

Scopes restrict what an API client can do within its assigned RBAC role (`role:api-client`):

### Scope Format

`{entity-type}:{action}` or `{entity-type}:*` (all actions)

| Scope | Permitted |
|---|---|
| `finance_invoice:read` | GET list and detail |
| `finance_invoice:create` | POST create |
| `finance_invoice:write` | PATCH update |
| `finance_invoice:delete` | DELETE |
| `finance_invoice:*` | All CRUD + actions |
| `finance_invoice:submit` | Custom action `submit` |

### Scope Enforcement

The middleware checks scopes for `type: "api_client"` sessions:

```go
// middleware/scope_check.go
func ScopeCheck(entityType, action string) fiber.Handler {
    return func(c *fiber.Ctx) error {
        actor := session.ActorFromContext(c.Context())
        if actor.SessionType != "api_client" {
            return c.Next()  // user sessions use RBAC, not scopes
        }

        required := fmt.Sprintf("%s:%s", entityType, action)
        wildcard := fmt.Sprintf("%s:*", entityType)

        for _, scope := range actor.Scopes {
            if scope == required || scope == wildcard {
                return c.Next()
            }
        }

        return c.Status(403).JSON(ErrorEnvelope{Error: ErrorBody{
            Code:    "insufficient_scope",
            Message: fmt.Sprintf("This API client does not have the '%s' scope.", required),
        }})
    }
}
```

An API client CANNOT access resources its scopes do not cover, even if the `role:api-client` Casbin role would otherwise permit it. Scopes are an additional restriction, not an expansion.

---

## 5. Secret Rotation

```
POST /api/v1/admin/api-clients/{client_id}/rotate
Authorization: Bearer {admin_session_token}
```

Response:

```json
{
  "data": {
    "client_id":     "awo_client_018e1b2c...",
    "client_secret": "awo_secret_NEW...",
    "rotated_at":    "2024-03-15T09:30:00Z"
  }
}
```

Rotation:
1. New secret generated and stored (hashed)
2. Old secret immediately invalidated
3. All existing tokens for this client invalidated from Redis
4. Rotation event recorded in audit log

The calling service must update its stored secret and re-authenticate immediately after rotation.

---

## 6. IP Allowlist (Optional)

API clients can be restricted to specific IP ranges:

```
PATCH /api/v1/admin/api-clients/{client_id}
Body: {
  "ip_allowlist": ["203.0.113.0/24", "198.51.100.42/32"]
}
```

When set, token issuance and API requests from outside the allowlist return HTTP 403 with `{"code": "ip_not_allowed"}`.

Use CIDR notation. IPv4 and IPv6 both supported.

---

## 7. Listing and Auditing

```
GET /api/v1/admin/api-clients
```

Returns all API clients for the tenant. Never returns `client_secret`.

```
GET /api/v1/admin/api-clients/{client_id}/activity
```

Returns last 100 token issuance events and API calls made by this client.

---

## 8. Security Requirements

| Requirement | Level |
|---|---|
| Client secrets transmitted over HTTPS only | MUST |
| Client secrets stored hashed (bcrypt) | MUST |
| Client secrets never returned after initial creation | MUST |
| Scope list is minimum required | MUST (principle of least privilege) |
| Secrets rotated after any personnel change at integration partner | SHOULD |
| Tokens expire and require re-authentication | MUST |
| IP allowlist configured for production integrations | SHOULD |

---

## Related Documents

- [Authentication](authentication.md) — user authentication flow
- [RBAC](rbac.md) — `role:api-client` system role and Casbin policies
- [Sessions](sessions.md) — session token storage in Redis
- [Hardening Guide](../15-security/hardening-guide.md) — secret management
