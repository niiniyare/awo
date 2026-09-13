> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Sessions"
id: iam-002
status: accepted
category: SPEC
stability: FROZEN
audience: [framework-authors, operators]
since: "1.0"
normative-level: normative
related:
  - "[RBAC](rbac.md)"
  - "[Authentication](authentication.md)"
  - "[Architecture Invariants](../02-architecture/invariants.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Sessions

**IAM-002 | Status: Accepted | Stability: Frozen**

This document specifies session lifecycle, Redis storage format, token generation, expiry semantics, and session validation behavior.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Session Model

A session is the server-side record of an authenticated actor's current login. Sessions are stored in Redis — not in PostgreSQL. Redis is the authoritative session store. See [INV-007](../02-architecture/invariants.md#inv-007-session-validation-always-requires-a-live-redis-connection).

Sessions are not JWT tokens. Awo does not use stateless authentication. Every request that requires authentication performs a Redis lookup to validate the session token. This design choice:
- Enables immediate session revocation (delete from Redis → session is invalid immediately)
- Requires Redis availability for all authenticated requests (correct security behavior)
- Avoids the expiry window problem inherent in JWT-based auth

---

## 2. Token Format

Session tokens are 256-bit cryptographically random values, URL-safe base64-encoded:

```go
tokenBytes := make([]byte, 32)
rand.Read(tokenBytes)
token := base64.URLEncoding.EncodeToString(tokenBytes)
// Example: "dGhpcyBpcyBhIHNlY3JldCBzZXNzaW9uIHRva2Vu-Ab3c"
```

Tokens are:
- 44 characters in URL-safe base64
- Unpredictable (256-bit entropy)
- Not self-describing (no embedded claims — all data is in Redis)

---

## 3. Redis Storage

```
Key:   session:{token}
Value: JSON-encoded SessionData
TTL:   Session expiry duration (configurable; default 8 hours)
```

### SessionData Format

```json
{
  "user_id":   "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "tenant_id": "b2c3d4e5-f6a7-8901-bcde-fa2345678901",
  "roles":     ["role:finance.accounts_payable", "role:tenant.user"],
  "branch":    "branch-nairobi-001",
  "created_at": "2024-12-15T08:00:00Z",
  "expires_at": "2024-12-15T16:00:00Z",
  "ip":        "197.232.1.42",
  "user_agent": "Mozilla/5.0 ..."
}
```

SessionData MUST NOT include: passwords, password hashes, API keys, or any sensitive credential material.

---

## 4. Session Validation

Session validation is step 7 of the [Middleware Pipeline](../GLOSSARY.md#middleware-pipeline). It executes after tenant resolution.

Validation steps:
1. Extract token from `Authorization: Bearer {token}` header (or `session_token` cookie for browser clients)
2. Execute `GET session:{token}` on Redis
3. If key does not exist: return HTTP 401 `{"error":{"code":"session.invalid"}}`
4. If key exists: deserialize SessionData
5. Validate `expires_at` > now(): if expired, DELETE the key and return HTTP 401 `{"error":{"code":"session.expired"}}`
6. Validate `tenant_id` matches the resolved TenantContext: if mismatch, return HTTP 403 `{"error":{"code":"session.tenant_mismatch"}}`
7. Attach Actor to request context from SessionData

### Redis Unavailability

If the Redis GET operation fails (connection refused, timeout, etc.), the middleware MUST return HTTP 503. There is no fallback. See [INV-007](../02-architecture/invariants.md#inv-007-session-validation-always-requires-a-live-redis-connection).

---

## 5. Session Creation

Created by the authentication endpoint on successful credential verification. See [Authentication](authentication.md).

```go
func createSession(ctx context.Context, rdb redis.Client, user User, tenantID uuid.UUID) (string, error) {
    token := generateToken()
    expiry := time.Now().Add(8 * time.Hour)

    data := SessionData{
        UserID:   user.ID,
        TenantID: tenantID,
        Roles:    user.Roles,
        // ...
        ExpiresAt: expiry,
    }

    jsonBytes, _ := json.Marshal(data)
    err := rdb.Set(ctx,
        "session:"+token,
        jsonBytes,
        time.Until(expiry),
    ).Err()

    return token, err
}
```

---

## 6. Session Expiry and Renewal

Sessions expire at `expires_at`. Expired sessions are not renewed automatically.

Optional sliding expiry: if configured (`SessionSlidingExpiry: true`), each successful request extends the session TTL by the configured window. Implemented by updating the Redis key TTL on each validated request:

```go
rdb.Expire(ctx, "session:"+token, sessionDuration)
```

Sliding expiry is disabled by default. When enabled, a session remains alive as long as the user is active, expiring only after the configured inactivity period.

---

## 7. Session Revocation

```go
// Logout — revoke immediately
rdb.Del(ctx, "session:"+token)

// Revoke all sessions for a user (e.g., on password change)
// Sessions are indexed by user for efficient revocation:
// Key: user_sessions:{user_id} — SET of session tokens
for token := range rdb.SMembers(ctx, "user_sessions:"+userID) {
    rdb.Del(ctx, "session:"+token)
}
rdb.Del(ctx, "user_sessions:"+userID)
```

Revocation is immediate. There is no propagation delay (unlike JWT-based auth with expiry windows).

---

## 8. Token Security

Tokens MUST be transmitted over TLS only. HTTP transport of session tokens is prohibited in any environment other than local development.

Tokens MUST NOT be logged. The session token is a `Sensitive` field in the `iam_session` entity. See [LAW-013](../02-architecture/laws.md#law-013-sensitive-fields-are-never-logged).

Browser clients receive the token in a `Set-Cookie` header with `HttpOnly`, `Secure`, and `SameSite=Strict` attributes:

```
Set-Cookie: session_token={token}; HttpOnly; Secure; SameSite=Strict; Path=/; Max-Age=28800
```

API clients receive the token in the response body:

```json
{"data": {"token": "{token}", "expires_at": "2024-12-15T16:00:00+03:00"}}
```

---

## Related Documents

- [Authentication](authentication.md) — session creation flow
- [RBAC](rbac.md) — roles stored in session, used for permission evaluation
- [Architecture Invariants](../02-architecture/invariants.md) — INV-007
- [Glossary](../GLOSSARY.md) — Session, Actor, SessionStore
