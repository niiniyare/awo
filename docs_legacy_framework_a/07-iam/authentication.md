> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Authentication"
id: iam-003
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors, operators]
since: "1.0"
normative-level: normative
related:
  - "[Sessions](sessions.md)"
  - "[RBAC](rbac.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Authentication

**IAM-003 | Status: Accepted | Stability: Stable**

This document specifies the authentication flow: credential verification, session creation, logout, password management, and rate limiting on auth endpoints.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Authentication Endpoints

Authentication endpoints are exempt from session validation middleware (they are the mechanism by which sessions are created). They are NOT exempt from:
- Request ID injection
- Structured logging
- Panic recovery
- CORS
- Tenant resolution (login must be tenant-scoped)
- Rate limiting (auth endpoints have stricter rate limits)

### POST /api/v1/auth/login

```json
Request:
{
  "email":    "user@example.com",
  "password": "..."
}

Response 200:
{
  "data": {
    "token":      "dGhpcyBpcyBhIHNlc...",
    "expires_at": "2024-12-15T16:00:00+03:00",
    "user": {
      "id":    "a1b2c3d4-...",
      "name":  "Amara Osei",
      "email": "user@example.com",
      "roles": ["role:finance.accounts_payable"]
    }
  }
}

Response 401:
{
  "error": {"code": "auth.invalid_credentials", "message": "Invalid email or password"}
}
```

The error message is identical for invalid email and invalid password. Distinguishing the two would allow email enumeration.

### POST /api/v1/auth/logout

Requires valid session token. Deletes the session from Redis.

```json
Response 200:
{"data": {"message": "Logged out successfully"}}
```

### POST /api/v1/auth/refresh

Extends the session expiry. Only available when `SessionSlidingExpiry: true`.

---

## 2. Credential Verification

```go
func verifyCredentials(ctx context.Context, email, password string, tenantID uuid.UUID) (*User, error) {
    // 1. Look up user by email within tenant
    users, _, err := userRepo.Query(ctx, filter.And(
        filter.Eq("email", email),
        filter.Eq("status", "active"),
    ))
    if err != nil || len(users) == 0 {
        // Perform dummy comparison to prevent timing attacks
        bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
        return nil, ErrInvalidCredentials
    }

    user := users[0]

    // 2. Compare password hash (bcrypt, cost >= 12)
    if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
        return nil, ErrInvalidCredentials
    }

    // 3. Check account status
    if user.Status != "active" {
        return nil, ErrAccountLocked
    }

    return &user, nil
}
```

### Password Hashing

Passwords MUST be hashed with bcrypt at cost ≥ 12. Plain-text passwords MUST NEVER be stored.

Password hashes are stored in the `iam_user` system entity with `Sensitive: true`. They MUST NOT appear in logs, API responses, or audit log entries.

### Timing Attack Prevention

The credential verification MUST always call `bcrypt.CompareHashAndPassword` — even when the email is not found — to prevent timing attacks that could be used to enumerate valid email addresses.

---

## 3. Auth Rate Limiting

Authentication endpoints have stricter rate limits than data API endpoints:

| Endpoint | Limit | Window | Key |
|---|---|---|---|
| `POST /auth/login` | 10 attempts | 15 minutes | `rl:auth:{tenant}:{ip}` |
| `POST /auth/login` | 5 failed attempts | 15 minutes | `rl:auth:fail:{tenant}:{email}` |

On exceeding the limit:

```json
HTTP/1.1 429 Too Many Requests
Retry-After: 900

{
  "error": {
    "code": "auth.rate_limited",
    "message": "Too many login attempts. Please try again later."
  }
}
```

After 5 consecutive failed attempts for the same email, the account is temporarily locked for 15 minutes. This lock is stored in Redis (not as a status change in PostgreSQL, to avoid writes on failed auth).

---

## 4. Password Reset

Password reset uses a time-limited, single-use token:

1. `POST /api/v1/auth/forgot-password` — sends reset email if email exists (always returns 200 regardless)
2. Reset email contains a token: `https://app.awo.so/reset-password?token={token}`
3. Token stored in Redis: `Key: pwd_reset:{token}`, `Value: user_id`, `TTL: 1 hour`
4. `POST /api/v1/auth/reset-password` — validates token, updates password hash, deletes token
5. All active sessions for the user are revoked after password reset

---

## 5. First Login and Forced Password Change

Provisioned users (created by admin) have `must_change_password: true`. On first login:
- Session is created normally
- Response includes `"must_change_password": true`
- The API client SHOULD redirect to the password change UI
- Subsequent requests on this session MUST be allowed normally (the enforcement is UI-level, not API-level)

---

## Related Documents

- [Sessions](sessions.md) — session lifecycle after authentication
- [RBAC](rbac.md) — roles loaded from session for permission evaluation
- [Glossary](../GLOSSARY.md) — Actor, Session, Authentication
