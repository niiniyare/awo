# Session Specification

**Classification:** Specification — Tier 0
**Owner:** `03-auth/SESSION_SPEC.md`
**Status:** Active — Metadata field added (ADR-021)
**Package:** `awo.so/awo/auth`

---

## Purpose

This document specifies `auth.Session` — the frozen authenticated identity record. A Session is the server-side source of truth for an active authentication. It is stored in Redis and loaded on every request. It is the origin of both `ViewerContext` and `Actor`.

## Scope

- `auth.Session` struct definition and all fields (ADR-004)
- All methods: `IsExpired`, `ToActor`, `ToViewer`, `RedisKey`, `TTL`
- `GenerateToken` — token generation
- Redis storage schema
- Session lifecycle: creation, validation, revocation, expiry
- Token security properties
- Service account session behaviour (API tokens)

## Dependencies

- [`00-overview/DECISION_REGISTER.md`](../00-overview/DECISION_REGISTER.md) — ADR-004
- [`03-auth/VIEWER_CONTEXT.md`](VIEWER_CONTEXT.md) — Session.ToViewer() output
- [`03-auth/ACTOR_SPEC.md`](ACTOR_SPEC.md) — Session.ToActor() output
- [`11-cache/CACHE_KEY_REFERENCE.md`](../11-cache/CACHE_KEY_REFERENCE.md) — Redis key patterns

---

## 1. Struct Definition (ADR-004)

The `Session` struct is frozen except for the `Metadata` field added by ADR-021. No fields may be added, removed, or renamed without a new ADR and a breaking change notice.

```go
// Package: awo.so/awo/auth

// Session is the server-side authenticated identity record.
// It is stored in Redis under the key "session:{token}" and loaded on every
// authenticated request by the session validation middleware.
//
// The struct is frozen at v1.0 (ADR-004). All fields tagged `json:"-"` are
// runtime-only and MUST NOT be stored in Redis.
type Session struct {
    // Token is the opaque session identifier.
    // 256-bit cryptographically random value, base64url-encoded without padding.
    // Redis key: "session:{token}"
    // NEVER logged. NEVER included in audit records. Sensitive field.
    Token string `json:"token"`

    // UserID is the UUID of the authenticated human user.
    // uuid.Nil for service account sessions.
    UserID uuid.UUID `json:"user_id"`

    // ServiceAccountID is the UUID of the authenticated service account.
    // uuid.Nil for human user sessions.
    // Exactly one of UserID and ServiceAccountID is non-nil.
    ServiceAccountID uuid.UUID `json:"service_account_id"`

    // TenantID is the tenant this session is scoped to.
    // Sessions are tenant-scoped. Operating in a different tenant requires
    // a new session.
    TenantID uuid.UUID `json:"tenant_id"`

    // Roles is the complete set of role names at authentication time.
    // Loaded from iam_user_roles (or iam_service_account_roles) once at login,
    // including all inherited roles from the role hierarchy.
    // Stale after role changes — the session must be re-issued to reflect updates.
    Roles []string `json:"roles"`

    // ExpiresAt is the absolute expiry time.
    // After this point the session is invalid regardless of last activity.
    // Default: 24 hours for human users. Configurable per service account.
    ExpiresAt time.Time `json:"expires_at"`

    // IssuedAt is when this session was created.
    IssuedAt time.Time `json:"issued_at"`

    // DeviceID is an opaque client-provided device identifier.
    // Used for concurrent session tracking and forced-logout by device.
    // Optional; empty string if the client does not provide one.
    DeviceID string `json:"device_id,omitempty"`

    // IPAddress is the client IP address at authentication time.
    // Informational only. Stored for audit and anomaly detection.
    IPAddress string `json:"ip_address,omitempty"`

    // RequestID carries the X-Request-ID header value for the current request.
    // NEVER stored in Redis — set by middleware on each request.
    RequestID string `json:"-"`

    // Metadata holds framework-reserved key-value context (ADR-021).
    // All keys MUST be prefixed "awo:". Module code MUST NOT write to this map.
    // Stored in Redis (JSON). Values MUST be JSON-serializable.
    // Example keys: "awo:device_fingerprint", "awo:mfa_method".
    // MUST NOT contain sensitive values (tokens, passwords, secrets).
    Metadata map[string]any `json:"metadata,omitempty"`
}
```

---

## 2. Methods

```go
// IsExpired reports whether this session has passed its expiry time.
// now is typically time.Now(); pass explicitly for testability.
func (s *Session) IsExpired(now time.Time) bool {
    return now.After(s.ExpiresAt)
}

// ToActor constructs a def.Actor from this Session.
// Called by the session middleware to populate RecordMeta.Actor and ActionContext.Actor.
// The returned Actor contains a defensive copy of Roles.
func (s *Session) ToActor() *def.Actor

// ToViewer constructs a ViewerContext from this Session.
// Called by the session middleware to populate the request context via WithViewer.
// The returned DefaultViewer pre-builds a roleSet map for O(1) HasRole.
func (s *Session) ToViewer() ViewerContext

// RedisKey returns the Redis key for storing this session.
// Format: "session:{token}"
func (s *Session) RedisKey() string {
    return "session:" + s.Token
}

// TTL returns the remaining time-to-live for this session.
// Returns zero if the session is already expired.
// Used as the Redis SET EX value.
func (s *Session) TTL(now time.Time) time.Duration {
    if s.IsExpired(now) {
        return 0
    }
    return s.ExpiresAt.Sub(now)
}
```

---

## 3. Token Generation

```go
// GenerateToken returns a cryptographically secure session token.
// 256-bit entropy from crypto/rand, base64url-encoded without padding.
// Returns an error only if the OS random source is unavailable (critical failure).
func GenerateToken() (string, error) {
    b := make([]byte, 32)  // 256 bits
    if _, err := rand.Read(b); err != nil {
        return "", fmt.Errorf("auth: generate token: %w", err)
    }
    return base64.RawURLEncoding.EncodeToString(b), nil
}
```

Token properties:
- 256-bit entropy — brute force infeasible at any foreseeable scale
- base64url without padding — URL-safe, header-safe, no `=` characters
- Opaque to clients — no self-verifiable structure (not a JWT)
- Never logged, never included in error responses

---

## 4. Redis Storage

```
Key:   session:{token}
Value: JSON-encoded Session (RequestID field excluded — json:"-")
TTL:   Session.TTL(now)  →  ExpiresAt − now
```

The session middleware performs a single `GET session:{token}` per request. No pipeline, no transactions. The GET atomically returns both the session data and implicitly validates TTL (Redis returns nil on expired keys).

Storage is write-once: sessions are created at login and deleted at logout. They are never updated in Redis. Role changes take effect by revoking the current session and requiring re-login.

---

## 5. Session Lifecycle

```
LOGIN
  1. Verify credentials (password hash or API token hash)
  2. Load user's roles from iam_user_roles + resolve inheritance
  3. GenerateToken()
  4. Build Session{Token, UserID, TenantID, Roles, ExpiresAt, IssuedAt, ...}
  5. SET session:{token} {json} EX {ttl_seconds}   ← Redis
  6. INSERT INTO iam_sessions (token_hash, user_id, tenant_id, issued_at, expires_at, device_id, ip_address)
  7. INSERT INTO iam_login_audit (event: "login", ...)
  8. Return token to client (Authorization: Bearer {token})

REQUEST
  1. Extract token from "Authorization: Bearer {token}"
  2. GET session:{token} from Redis
  3. Unmarshal Session
  4. Session.IsExpired(now) → 401 if true
  5. Validate tenant status = ACTIVE
  6. session.RequestID = c.Locals("request_id")
  7. ctx = auth.WithViewer(ctx, session.ToViewer())
  8. Proceed to route handler

LOGOUT
  1. DEL session:{token}   ← Redis
  2. UPDATE iam_sessions SET revoked_at = now() WHERE token_hash = sha256(token)
  3. INSERT INTO iam_login_audit (event: "logout", ...)

EXPIRY (natural)
  1. Redis TTL expires automatically — GET returns nil
  2. Background job: UPDATE iam_sessions SET expired_at = now() WHERE expires_at < now() AND expired_at IS NULL
```

---

## 6. Session Invalidation

Sessions are invalidated in three ways:

| Mechanism | Trigger | Latency |
|---|---|---|
| Natural expiry | Redis TTL exhausted | Immediate (Redis) |
| Explicit revocation | Logout, forced logout by admin, role change | Immediate (Redis DEL) |
| Tenant suspension | Tenant status check on each request | Immediate (check on request) |

After a role is assigned or revoked for a user, the IAM module MUST revoke all active sessions for that user. The next login will load the updated role set.

---

## 7. Concurrent Sessions

A single user may have multiple active sessions (different devices, API clients). Each session has an independent token, TTL, and `DeviceID`. `iam_sessions` tracks all active sessions.

Forced logout by device: the IAM admin revokes the session matching a specific DeviceID. This DELs that specific Redis key; other sessions are unaffected.

Forced logout of all sessions: the IAM admin revokes all sessions for a user. This DELs all Redis keys for that user's active sessions (fetched from `iam_sessions`).

---

## 8. Service Account Sessions (API Tokens)

Service accounts authenticate via long-lived API tokens (`iam_api_tokens`). Their sessions differ from human sessions:

| Property | Human session | Service account session |
|---|---|---|
| Stored in Redis | Yes | No (stateless) |
| Expiry | `ExpiresAt` field | API token expiry date |
| Token format | 256-bit random | 256-bit random (same) |
| Validation | Redis GET | DB lookup (cached 60s) |
| `UserID` | Set | `uuid.Nil` |
| `ServiceAccountID` | `uuid.Nil` | Set |

On each request with an API token:
1. Hash the token (SHA-256)
2. `GET iam:api_token:{hash}` from Redis (60-second cache)
3. On miss: query `iam_api_tokens` by hash; populate cache
4. Build a synthetic `Session` (no Redis storage)
5. Proceed identically to a human session from step 3 of REQUEST above

API token sessions are never stored in `iam_sessions`. API token usage is recorded in `iam_login_audit`.

---

## 9. Security Properties

1. **Tokens are opaque**: No self-verification. Validity requires a Redis round-trip. Server-side revocation is instant.
2. **Tokens are never logged**: `Token` is excluded from all structured logs, error responses, and audit records.
3. **Tokens are not JWTs**: No algorithm confusion attacks, no signature bypass, no claim injection.
4. **Session data is server-authoritative**: Roles are loaded at login from the IAM store, not carried in the token.
5. **Redis failure is hard-fail**: If Redis is unreachable, session validation fails with 503. Silent pass is not permitted — security is structural, not additive.
6. **Tenant isolation is enforced per request**: Tenant status is checked on every request, not cached in the session.

---

## 10. Normative Requirements

- The `Session` struct MUST NOT gain or lose fields without a new ADR (ADR-004 freeze).
- `Token` MUST be generated with `GenerateToken()` — never from weak sources.
- `Token` MUST NOT appear in logs, audit records, or API error responses.
- `RequestID` MUST carry `json:"-"` — it MUST NOT be stored in Redis.
- Session middleware MUST fail with 401 on `IsExpired(now) == true`.
- Session middleware MUST fail with 503 if Redis is unreachable.
- Session middleware MUST fail with 402/503/410 based on tenant status (see [`04-multitenancy/TENANT_LIFECYCLE.md`](../04-multitenancy/TENANT_LIFECYCLE.md)).
- Role changes MUST trigger session revocation for affected users.
- `ToActor()` MUST return a defensive copy of `Roles` — not a shared slice.

---

## References

- `awo/auth/session.go` — Session struct and methods
- [`03-auth/VIEWER_CONTEXT.md`](VIEWER_CONTEXT.md) — ToViewer() output
- [`03-auth/ACTOR_SPEC.md`](ACTOR_SPEC.md) — ToActor() output
- [`11-cache/CACHE_KEY_REFERENCE.md`](../11-cache/CACHE_KEY_REFERENCE.md) — `session:{token}` key pattern
- [`04-multitenancy/TENANT_LIFECYCLE.md`](../04-multitenancy/TENANT_LIFECYCLE.md) — Tenant status enforcement
- ADR-004 in [`00-overview/DECISION_REGISTER.md`](../00-overview/DECISION_REGISTER.md)
