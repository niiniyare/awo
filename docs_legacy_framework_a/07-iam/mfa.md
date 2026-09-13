> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Multi-Factor Authentication"
id: iam-005
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, operators]
since: "1.0"
normative-level: normative
related:
  - "[Authentication](authentication.md)"
  - "[Sessions](sessions.md)"
  - "[Password Policy](password-policy.md)"
  - "[Security Model](../15-security/security-model.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Multi-Factor Authentication

**IAM-005 | Status: Accepted | Stability: Stable**

This document specifies the MFA system in Awo: TOTP enrollment, verification flow, backup codes, and enforcement policies.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Supported MFA Methods

| Method | Status | Notes |
|---|---|---|
| TOTP (RFC 6238) | MUST support | Google Authenticator, Authy, 1Password compatible |
| Backup codes | MUST support | 10 single-use codes generated at TOTP enrollment |
| Email OTP | MAY support | 6-digit code, 10-minute TTL |
| SMS OTP | NOT supported | Too easily SIM-swapped; not recommended for business ERP |
| Hardware key (WebAuthn) | PLANNED | Not in v1.0 |

---

## 2. TOTP Enrollment Flow

```
POST /api/v1/auth/mfa/totp/setup
Authorization: Bearer {session_token}
```

Response:
```json
{
  "data": {
    "secret":       "JBSWY3DPEHPK3PXP",
    "qr_code_url":  "otpauth://totp/Awo:alice@example.com?secret=JBSWY3DPEHPK3PXP&issuer=Awo",
    "backup_codes": [
      "ABCD-1234", "EFGH-5678", "IJKL-9012",
      "MNOP-3456", "QRST-7890", "UVWX-2345",
      "YZAB-6789", "CDEF-0123", "GHIJ-4567",
      "KLMN-8901"
    ]
  }
}
```

The secret is stored encrypted (AES-256-GCM) in the user record. The QR code URL is generated server-side — the client renders it as a QR image. Backup codes are hashed with bcrypt (cost 10) before storage.

### Confirm Enrollment

The setup is not active until the user confirms with a valid TOTP code:

```
POST /api/v1/auth/mfa/totp/confirm
Authorization: Bearer {session_token}
Body: {"code": "123456"}
```

On success: `mfa_enrolled = true` set in user record. On failure: secret discarded; user must restart setup.

---

## 3. Login Flow with MFA

### Step 1: Credential Verification

```
POST /api/v1/auth/login
Body: {"email": "...", "password": "..."}
```

If credentials are valid and user has MFA enrolled:

```json
{
  "data": {
    "mfa_required":   true,
    "mfa_challenge_token": "mfachallenge_...",
    "mfa_methods":    ["totp", "backup_code"]
  }
}
```

The `mfa_challenge_token` is a short-lived Redis token (10 minutes TTL) representing a verified-password-but-not-yet-mfa state. It is not a session token — it grants no API access.

### Step 2: MFA Verification

```
POST /api/v1/auth/mfa/verify
Body: {
  "challenge_token": "mfachallenge_...",
  "method":          "totp",
  "code":            "123456"
}
```

On success: challenge token deleted from Redis; full session token issued:

```json
{
  "data": {
    "session_token": "awo_...",
    "expires_at":    "2024-03-15T17:30:00Z"
  }
}
```

On failure: challenge token remains valid for up to 5 attempts, then invalidated.

---

## 4. TOTP Verification

```go
func VerifyTOTP(secret, code string) bool {
    // Allow ±1 time step (30 seconds) for clock skew
    t := time.Now().Unix()
    for _, offset := range []int64{-1, 0, 1} {
        counter := (t + offset*30) / 30
        expected := computeHOTP(secret, counter)
        if hmac.Equal([]byte(expected), []byte(code)) {
            return true
        }
    }
    return false
}
```

Used codes MUST be stored to prevent replay attacks:

```go
// Store used TOTP codes in Redis with TTL slightly longer than the window
redisKey := fmt.Sprintf("totp_used:%s:%s", userID, code)
set, err := redis.SetNX(ctx, redisKey, "1", 90*time.Second)
if !set {
    return false, errors.New("TOTP code already used")
}
```

---

## 5. Backup Code Verification

```
POST /api/v1/auth/mfa/verify
Body: {
  "challenge_token": "mfachallenge_...",
  "method":          "backup_code",
  "code":            "ABCD-1234"
}
```

Backup codes are single-use:
1. Hash submitted code with bcrypt
2. Compare against stored hashed backup codes
3. If match found: delete the matched code from the user's backup code list
4. Issue session token

After using a backup code, the user is shown a warning: "You used a backup code. Please re-enroll TOTP immediately."

---

## 6. MFA Enforcement Policy

Tenants can require MFA for all users:

```
Setting: iam.mfa_required = true
```

When enabled:
- Users without MFA enrolled are allowed to log in but are redirected to the TOTP setup page
- All endpoints return HTTP 403 with `{"code": "mfa_enrollment_required"}` until setup is complete
- Exception: the MFA enrollment endpoints themselves

Platform administrators always have MFA enforced regardless of tenant setting.

---

## 7. MFA Reset (Admin)

If a user loses their TOTP device and all backup codes:

```
POST /api/v1/admin/users/{user_id}/mfa/reset
Authorization: Bearer {admin_session_token}
```

This:
1. Clears `mfa_enrolled = false`
2. Deletes stored TOTP secret and backup codes
3. Invalidates all existing sessions for the user
4. Sends email to user notifying them of the reset

The admin action is recorded in the audit log with the admin's user ID.

---

## 8. Recovery Email OTP (Optional)

When configured, users can receive a 6-digit OTP via email as a fallback:

```
POST /api/v1/auth/mfa/email/send
Body: {"challenge_token": "mfachallenge_..."}
```

Sends a 6-digit code to the user's registered email. Code stored as:
```
Redis key: email_otp:{sha256(code)}:{user_id}
TTL: 10 minutes
```

Verification follows the same `/api/v1/auth/mfa/verify` endpoint with `method: "email_otp"`.

Email OTP is a weaker factor than TOTP. Tenants that require strict MFA (financial institutions, regulated environments) SHOULD disable email OTP:

```
Setting: iam.mfa_email_otp_enabled = false
```

---

## Related Documents

- [Authentication](authentication.md) — login flow, credential verification
- [Sessions](sessions.md) — session lifecycle after MFA verification
- [Password Policy](password-policy.md) — bcrypt cost, token storage
- [Security Model](../15-security/security-model.md) — MFA as defense layer
