> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Session Management"
id: iam-009
status: accepted
category: SPEC
stability: STABLE
audience: [framework-authors, module-authors]
since: "1.0"
normative-level: normative
related:
  - "[Authentication](authentication.md)"
  - "[MFA](mfa.md)"
  - "[Password Policy](password-policy.md)"
  - "[ADR-016: Server-Side Sessions](../17-adr/adr-016-server-side-sessions.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Session Management

**IAM-009 | Status: Accepted | Stability: Stable**

Complete session lifecycle: creation, validation, sliding window TTL, revocation, and listing active sessions.

---

## 1. Session Token Format

Session tokens are 256-bit (32 bytes) cryptographically random values, hex-encoded:

```go
func generateSessionToken() (string, error) {
    b := make([]byte, 32)
    if _, err := rand.Read(b); err != nil {
        return "", fmt.Errorf("generateSessionToken: %w", err)
    }
    return hex.EncodeToString(b), nil
}
```

Format: 64 lowercase hex characters. Example:
```
a3f7b2c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5f6a7b8c9d0e1f2
```

---

## 2. Session Record Structure

Stored in Redis as a hash:

```go
type SessionRecord struct {
    UserID       string    // UUID string
    TenantID     string    // UUID string
    Roles        []string  // ["role:finance.accounts_payable", "role:tenant.user"]
    ActorType    string    // "user", "api_client"
    IPAddress    string
    UserAgent    string
    CreatedAt    time.Time
    LastSeenAt   time.Time
    ExpiresAt    time.Time
    MFAVerified  bool      // true after TOTP verification
    MFARequired  bool      // true if tenant enforces MFA and user has MFA enrolled
    Scopes       []string  // for api_client sessions: ["finance.read", "hr.read"]
    RequiresPasswordChange bool
}
```

Redis key: `session:{token}` (hex token)

---

## 3. Session Creation

```go
func (s *SessionService) CreateSession(ctx context.Context, input CreateSessionInput) (string, error) {
    token, err := generateSessionToken()
    if err != nil {
        return "", fmt.Errorf("CreateSession: generate token: %w", err)
    }

    ttl := s.resolveSessionTTL(ctx, input.TenantID)
    expiresAt := time.Now().Add(ttl)

    record := SessionRecord{
        UserID:    input.UserID.String(),
        TenantID:  input.TenantID.String(),
        Roles:     input.Roles,
        ActorType: "user",
        IPAddress: input.IPAddress,
        UserAgent: input.UserAgent,
        CreatedAt: time.Now(),
        LastSeenAt: time.Now(),
        ExpiresAt: expiresAt,
        MFAVerified: !input.MFARequired,
        MFARequired: input.MFARequired,
    }

    // Store session in Redis
    key := "session:" + token
    if err := s.Redis.HSet(ctx, key, sessionToMap(record)); err != nil {
        return "", fmt.Errorf("CreateSession: store session: %w", err)
    }
    s.Redis.ExpireAt(ctx, key, expiresAt)

    // Track active sessions per user (for listing + single-session enforcement)
    userKey := fmt.Sprintf("user_sessions:%s:%s", input.TenantID, input.UserID)
    s.Redis.SAdd(ctx, userKey, token)
    s.Redis.Expire(ctx, userKey, 30*24*time.Hour)  // cleanup: 30 days

    slog.Info("session created",
        "user_id",   input.UserID,
        "tenant_id", input.TenantID,
        "ip",        input.IPAddress,
        "token_hint", token[:8],  // Never log full token
    )

    return token, nil
}
```

---

## 4. Session Validation (Every Request)

```go
func (s *SessionService) ValidateSession(ctx context.Context, token string) (*SessionRecord, error) {
    key := "session:" + token
    vals, err := s.Redis.HGetAll(ctx, key).Result()
    if err != nil {
        return nil, fmt.Errorf("ValidateSession: redis error: %w", err)
    }
    if len(vals) == 0 {
        return nil, ErrSessionNotFound
    }

    record := mapToSession(vals)

    // Check expiry (belt + suspenders — Redis TTL handles this, but double-check)
    if time.Now().After(record.ExpiresAt) {
        s.Redis.Del(ctx, key)
        return nil, ErrSessionExpired
    }

    // Sliding window: reset TTL on each request
    ttl := s.resolveSessionTTL(ctx, uuid.MustParse(record.TenantID))
    newExpiry := time.Now().Add(ttl)
    s.Redis.HSet(ctx, key, "last_seen_at", time.Now().Unix(), "expires_at", newExpiry.Unix())
    s.Redis.ExpireAt(ctx, key, newExpiry)

    return &record, nil
}
```

---

## 5. Session Revocation

### Revoke Single Session

```go
func (s *SessionService) RevokeSession(ctx context.Context, token string) error {
    record, err := s.ValidateSession(ctx, token)
    if err != nil {
        return nil  // Already expired/gone — idempotent
    }

    s.Redis.Del(ctx, "session:"+token)

    // Remove from user session index
    userKey := fmt.Sprintf("user_sessions:%s:%s", record.TenantID, record.UserID)
    s.Redis.SRem(ctx, userKey, token)

    slog.Info("session revoked", "token_hint", token[:8])
    return nil
}
```

### Revoke All Sessions for a User

Used when:
- User changes password
- Admin forces logout
- User account is suspended

```go
func (s *SessionService) RevokeAllUserSessions(ctx context.Context, tenantID, userID uuid.UUID) error {
    userKey := fmt.Sprintf("user_sessions:%s:%s", tenantID, userID)
    tokens, err := s.Redis.SMembers(ctx, userKey).Result()
    if err != nil {
        return fmt.Errorf("RevokeAllUserSessions: list sessions: %w", err)
    }

    for _, token := range tokens {
        s.Redis.Del(ctx, "session:"+token)
    }
    s.Redis.Del(ctx, userKey)

    slog.Info("all user sessions revoked",
        "user_id",   userID,
        "tenant_id", tenantID,
        "count",     len(tokens),
    )
    return nil
}
```

---

## 6. Session Listing API

Tenant admins can list active sessions for their users:

```
GET /api/v1/platform/sessions?user_id={user_uuid}
```

Response:

```json
{
  "data": [
    {
      "token_hint": "a3f7b2c4",
      "ip_address": "196.201.214.100",
      "user_agent": "Mozilla/5.0...",
      "created_at": "2024-03-15T08:00:00+03:00",
      "last_seen_at": "2024-03-15T10:30:00+03:00",
      "expires_at": "2024-03-15T18:00:00+03:00",
      "mfa_verified": true
    }
  ]
}
```

The full token is NEVER returned. Only `token_hint` (first 8 chars) is shown.

---

## 7. Single Active Session Enforcement

For tenants with strict security requirements (banking, government):

```go
// Tenant setting: iam.single_session = true
func (s *SessionService) enforceSingleSession(ctx context.Context, tenantID, userID uuid.UUID, newToken string) error {
    singleSession, _ := s.Settings.GetBool(ctx, "iam.single_session")
    if !singleSession {
        return nil
    }

    // Revoke all existing sessions before completing new login
    if err := s.RevokeAllUserSessions(ctx, tenantID, userID); err != nil {
        return fmt.Errorf("enforceSingleSession: revoke existing: %w", err)
    }
    return nil
}
```

---

## 8. Session Security Headers

Every response includes:

```
Set-Cookie: awo_session=<token>; HttpOnly; Secure; SameSite=Strict; Max-Age=28800
```

For API clients, the token is returned in the response body (not cookie) and sent in `Authorization: Bearer <token>` headers.

---

## Related Documents

- [Authentication](authentication.md) — login flow that calls CreateSession
- [MFA](mfa.md) — MFARequired flag and TOTP verification flow
- [Password Policy](password-policy.md) — password change revokes all sessions
- [ADR-016: Server-Side Sessions](../17-adr/adr-016-server-side-sessions.md) — design rationale
