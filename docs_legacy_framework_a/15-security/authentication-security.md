> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Authentication Security"
id: sec-005
status: accepted
category: SPEC
stability: STABLE
audience: [framework-authors, operators]
since: "1.0"
normative-level: normative
related:
  - "[Threat Model](threat-model.md)"
  - "[Authentication](../07-iam/authentication.md)"
  - "[Session Management](../07-iam/session-management.md)"
  - "[Password Policy](../07-iam/password-policy.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Authentication Security

**SEC-005 | Status: Accepted | Stability: Stable**

Security controls for Awo's authentication layer: token security, brute-force prevention, credential storage, timing attack prevention, and MFA enforcement.

---

## 1. Token Security

### Token Format

Session tokens are 32 bytes of cryptographically random data, hex-encoded to 64 characters:

```go
func generateToken() (string, error) {
    buf := make([]byte, 32)
    if _, err := rand.Read(buf); err != nil {
        return "", fmt.Errorf("generateToken: %w", err)
    }
    return hex.EncodeToString(buf), nil
}
```

- **Entropy**: 256 bits — brute-force infeasible (2^256 possibilities)
- **No structure**: opaque string; cannot be decoded to reveal user/tenant info
- **No JWT**: server-side lookup required; token cannot be forged if Redis is compromised alone

### Token Storage

Tokens are stored in Redis as `session:{token}` hash. They are **never**:
- Stored in the database (lost on Redis flush, but no persistent exposure)
- Logged in access logs
- Included in error responses
- Sent in URL query parameters

### Token Transmission

Tokens must be transmitted via `Authorization: Bearer {token}` header only. HTTPS enforced in production via `Strict-Transport-Security` header (see [API Authentication](../11-api/authentication.md)).

---

## 2. Credential Storage

### Password Hashing

Passwords are hashed with bcrypt, cost factor 12:

```go
hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
```

Cost 12 takes ~300ms on modern hardware — sufficient work factor to slow offline attacks.

**Never store**:
- Plaintext passwords
- MD5 or SHA-family hashes (without bcrypt wrapping)
- Reversible encryption of passwords

### API Client Secrets

API client secrets are stored as bcrypt hashes (same cost 12). The plaintext secret is shown once at creation time and cannot be retrieved. If lost, the secret must be rotated (new secret generated, old hash replaced).

---

## 3. Timing Attack Prevention

All credential comparisons use constant-time comparison:

```go
// Password verification — constant time
err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(providedPassword))

// WRONG — timing leak via early return:
if storedHash == providedPassword { ... }
```

### Dummy Hash for Non-Existent Users

When a user is not found, compute a bcrypt comparison against a dummy hash to prevent timing attacks that reveal whether an email is registered:

```go
func (a *AuthActivities) VerifyCredentials(ctx context.Context, email, password string) (*User, error) {
    user, err := a.userRepo.FindByEmail(ctx, email)
    if err != nil || user == nil {
        // Constant-time comparison against dummy hash even when user not found
        // This takes the same time as a real bcrypt comparison
        _ = bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(password))
        return nil, errors.ErrInvalidCredentials  // same error as wrong password
    }

    if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
        return nil, errors.ErrInvalidCredentials
    }
    return user, nil
}

// dummyHash is a pre-computed bcrypt hash of a random string
var dummyHash = "$2a$12$aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
```

---

## 4. Brute-Force Prevention

### Login Rate Limiting

Login attempts are rate-limited per IP and per tenant:

```go
// Redis sliding window: max 10 attempts per minute per (IP, tenant)
func checkLoginRateLimit(ctx context.Context, redis *redis.Client, ip, tenantID string) error {
    key := fmt.Sprintf("rl:login:%s:%s", ip, tenantID)
    count, err := redis.Incr(ctx, key).Result()
    if err != nil {
        return fmt.Errorf("checkLoginRateLimit: %w", err)
    }
    if count == 1 {
        redis.Expire(ctx, key, time.Minute)
    }
    if count > 10 {
        return &errors.BusinessError{
            Code: "auth.rate_limited", Message: "Too many login attempts", Status: 429}
    }
    return nil
}
```

### Account Lockout

After 10 consecutive failed attempts within 15 minutes, the account is locked for 30 minutes:

```go
func (a *AuthActivities) RecordFailedAttempt(ctx context.Context, userID uuid.UUID) error {
    key := fmt.Sprintf("login:failures:%s", userID)
    count, _ := a.Redis.Incr(ctx, key).Result()
    if count == 1 {
        a.Redis.Expire(ctx, key, 15*time.Minute)
    }
    if count >= 10 {
        // Lock account
        _, err := a.userRepo.Update(ctx, userID, def.UpdateInput{
            Fields: map[string]any{
                "locked_until": time.Now().UTC().Add(30 * time.Minute),
            },
        })
        return err
    }
    return nil
}
```

Account lock is stored in the database (not Redis) so it survives Redis flush. Unlock is automatic (time-based) or manual by tenant admin.

---

## 5. MFA Security

### TOTP Implementation

TOTP uses RFC 6238 with:
- 30-second window
- HMAC-SHA1 (standard for TOTP)
- 6-digit codes
- 1-window tolerance (accept codes from ±30 seconds for clock skew)

TOTP secrets are stored encrypted in the database. The encryption key is injected from Vault — never hard-coded.

### MFA Session Isolation

The MFA intermediate session token (`mfa_session_token`) has a 5-minute TTL and grants access only to `POST /auth/mfa/verify`. It cannot be used for any other API endpoint.

### Backup Codes

- 10 backup codes generated at MFA enrollment
- Each is a one-time use 8-character alphanumeric code
- Stored as bcrypt hashes (cost 12)
- Consumed (marked used) atomically in a Redis transaction to prevent race conditions

---

## 6. Session Security Controls

### HTTPS-Only Cookies (if cookie transport is added in future)

Currently tokens are `Authorization: Bearer` only. If cookie transport is ever added, cookies must be:
- `Secure` (HTTPS only)
- `HttpOnly` (no JavaScript access)
- `SameSite=Strict` (no cross-site requests)

### Session Invalidation

Sessions are immediately invalidated on:
- Explicit logout (`POST /auth/logout`)
- Password change
- MFA secret rotation
- Account lock
- Tenant suspension or archival

Redis `DEL session:{token}` is the invalidation mechanism. No waiting for expiry.

### Concurrent Session Policy

Default: multiple concurrent sessions allowed (different devices). Tenant can enforce single-session policy via settings:

```go
// If single_session setting is true, all other sessions are revoked on new login
if settings.GetBool(ctx, "iam.single_session_policy") {
    revokeAllUserSessions(ctx, userID)
}
```

---

## 7. Security Headers

All responses include:

```
Strict-Transport-Security: max-age=63072000; includeSubDomains; preload
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
X-XSS-Protection: 0                 (disabled — CSP is preferred)
Content-Security-Policy: default-src 'self'; script-src 'self' 'unsafe-inline'
Referrer-Policy: strict-origin-when-cross-origin
Cache-Control: no-store             (on all auth responses)
```

---

## 8. Audit Trail

All authentication events are recorded in the audit log:

| Event | Logged Fields |
|---|---|
| Successful login | user_id, tenant_id, ip, user_agent, session_id |
| Failed login | email (hashed), tenant_id, ip, failure_reason |
| Logout | user_id, session_id |
| Account locked | user_id, consecutive_failures |
| Password changed | user_id, changed_by (self or admin) |
| MFA enrolled | user_id |
| MFA disabled | user_id, disabled_by |
| API client created | client_id, created_by |

Failed logins log the **hashed** email (SHA-256) — not plaintext — to allow pattern analysis without storing personal data in audit logs.

---

## Related Documents

- [Threat Model](threat-model.md) — attack vectors and mitigations
- [Authentication](../07-iam/authentication.md) — login flow implementation
- [Session Management](../07-iam/session-management.md) — token creation, sliding TTL, revocation
- [Password Policy](../07-iam/password-policy.md) — complexity requirements, reset flow
- [API Authentication](../11-api/authentication.md) — request authentication protocol
- [Alerting](../13-observability/alerting.md) — auth failure rate alerts
