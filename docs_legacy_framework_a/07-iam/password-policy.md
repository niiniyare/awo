> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Password Policy"
id: iam-004
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, operators]
since: "1.0"
normative-level: normative
related:
  - "[Authentication](authentication.md)"
  - "[Sessions](sessions.md)"
  - "[Security Model](../15-security/security-model.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Password Policy

**IAM-004 | Status: Accepted | Stability: Stable**

This document specifies password complexity requirements, hashing algorithm, storage rules, rotation policy, and the forced-change flow for new accounts.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Password Requirements

All user passwords MUST meet:

| Requirement | Minimum |
|---|---|
| Length | 8 characters |
| Character classes | At least 2 of: uppercase, lowercase, digit, special character |
| Not a common password | Checked against top-10,000 common passwords list |
| Not the username | Password MUST NOT equal the user's email address |

These requirements are enforced by the IAM module's password validator:

```go
// Returns a ValidationError with field "password" if requirements not met
func ValidatePasswordStrength(password, email string) error {
    if len(password) < 8 {
        return &errors.ValidationError{
            Fields: map[string]string{
                "password": "Password must be at least 8 characters.",
            },
        }
    }
    // ... additional checks
}
```

---

## 2. Password Hashing

Passwords MUST be hashed using bcrypt with cost ≥ 12:

```go
hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
```

The cost parameter SHOULD increase as hardware capabilities improve. The current minimum (12) provides approximately 250ms computation time on 2024 hardware — slow enough to deter brute force, fast enough for acceptable login UX.

Passwords MUST NEVER be:
- Stored in plaintext
- Logged at any level
- Included in audit log entries
- Transmitted over HTTP (HTTPS required for all auth endpoints)

---

## 3. Temporary Passwords

When an account is provisioned (new user created by admin or module provisioning), a temporary password is generated:

```go
// Cryptographically random 16-character temporary password
tempPassword := generateTempPassword(16)
// Sent to user via secure channel (email, Slack, etc.)
// Stored hashed (same bcrypt cost)
// Flagged: requires_password_change = true
```

The `requires_password_change` flag is stored in the user's session data. On every request, the middleware checks this flag. If set, all endpoints return HTTP 403 with `{"code": "password_change_required"}` except the password change endpoint itself.

```go
// Forced password change check in middleware
if actor.RequiresPasswordChange && c.Path() != "/api/v1/auth/change-password" {
    return c.Status(403).JSON(ErrorEnvelope{
        Error: ErrorBody{
            Code:    "password_change_required",
            Message: "You must change your password before continuing.",
        },
    })
}
```

---

## 4. Password Rotation Policy

Awo does not enforce mandatory periodic password rotation by default — research shows forced rotation reduces security (users choose predictable patterns). Password rotation IS triggered by:

- Suspected compromise (admin forces reset)
- New account creation (one-time forced change)
- User-initiated change
- Account recovery (password reset flow)

Organizations that require periodic rotation can configure it via the Settings module:

```
Setting: iam.password_rotation_days = 90   (0 = no forced rotation, default)
```

When rotation is enabled, sessions created more than N days before the password last changed are invalidated.

---

## 5. Password Change Flow

```
POST /api/v1/auth/change-password
Body: {
    "current_password": "...",
    "new_password": "...",
    "new_password_confirm": "..."
}
```

1. Validate session (required — unauthenticated users cannot change password this way)
2. Verify `current_password` against stored hash (prevents session hijacking)
3. Validate `new_password` meets requirements
4. Verify `new_password == new_password_confirm`
5. Hash new password with bcrypt cost ≥ 12
6. Update user record: `password_hash = new_hash, requires_password_change = false`
7. Invalidate all other sessions for this user (force re-login on other devices)
8. Return HTTP 200

Step 7 is intentional: a compromised session that forces a password change should not remain active after the change.

---

## 6. Password Reset (Forgot Password)

```
POST /api/v1/auth/forgot-password
Body: {"email": "user@example.com"}
```

Always returns HTTP 200 — even if the email does not exist (prevents user enumeration).

If the email exists:
1. Generate a cryptographically random 32-byte reset token
2. Store: `reset_tokens:{sha256(token)}` → `{user_id, expires_at}` in Redis, TTL 1 hour
3. Send reset email with link: `https://app.example.com/reset-password?token={raw_token}`

```
POST /api/v1/auth/reset-password
Body: {
    "token": "...",
    "new_password": "...",
    "new_password_confirm": "..."
}
```

1. Lookup `reset_tokens:{sha256(token)}` in Redis
2. If not found or expired: HTTP 400 `{"code": "invalid_reset_token"}`
3. Validate new password requirements
4. Hash and store new password
5. Delete reset token from Redis (single-use)
6. Invalidate all existing sessions for the user
7. Return HTTP 200

---

## Related Documents

- [Authentication](authentication.md) — login, logout, session creation
- [Sessions](sessions.md) — session management and revocation
- [Security Model](../15-security/security-model.md) — T3 (session hijacking), T5 (sensitive data)
- [Glossary](../GLOSSARY.md) — bcrypt, Session Token, Password Reset
