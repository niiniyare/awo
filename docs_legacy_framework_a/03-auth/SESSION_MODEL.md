> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Session Model

**Classification:** Specification — Tier 1
**Owner:** `03-auth/SESSION_MODEL.md`
**Status:** Frozen at v1.0 (ADR-004)
**Package:** `awo.so/awo/auth`

---

## Purpose

This document specifies the `Session` struct, its storage model in Redis, its validation protocol, and its conversion to `ViewerContext` and `Actor`.

## Dependencies

- [`03-auth/VIEWER_CONTEXT.md`](VIEWER_CONTEXT.md) — ViewerContext constructed from Session
- [`03-auth/ACTOR_MODEL.md`](ACTOR_MODEL.md) — Actor constructed from Session
- [`11-cache/CACHE_KEY_REFERENCE.md`](../11-cache/CACHE_KEY_REFERENCE.md) — Redis key pattern

---

## 1. Session Struct

```go
// Package: awo.so/awo/auth

// Session is the complete authentication state for one authenticated request.
// Sessions are stored in Redis as JSON-serialized values under session:{token}.
//
// Session is constructed at login and stored in Redis. The middleware reads it
// on every authenticated request. It is NOT stored in PostgreSQL.
type Session struct {
    // Token is the opaque session token. This is the Redis key suffix.
    // Generated as a cryptographically random 256-bit value, base64url-encoded.
    Token string

    // UserID is the UUID of the authenticated human user.
    // uuid.Nil for service account sessions.
    UserID uuid.UUID

    // ServiceAccountID is the UUID of the authenticated service account.
    // uuid.Nil for human user sessions.
    ServiceAccountID uuid.UUID

    // TenantID is the UUID of the tenant this session operates within.
    TenantID uuid.UUID

    // Roles is the complete set of role names at the time of login.
    // Role changes take effect at next login.
    Roles []string

    // ExpiresAt is the wall-clock time when this session expires.
    // The middleware MUST reject sessions where now > ExpiresAt.
    ExpiresAt time.Time

    // IssuedAt is the wall-clock time when this session was created.
    IssuedAt time.Time

    // DeviceID is an optional client device identifier for session tracking.
    DeviceID string

    // IPAddress is the client IP at login time. Used for security alerting.
    IPAddress string

    // RequestID is set on every request using this session, for log correlation.
    // Not stored in Redis — set by the middleware for the duration of one request.
    RequestID string
}
```

---

## 2. Methods

```go
// IsExpired returns true when the session has passed its expiry time.
func (s *Session) IsExpired(now time.Time) bool {
    return now.After(s.ExpiresAt)
}

// ToActor converts the session to a def.Actor for injection into hooks
// and action handlers.
func (s *Session) ToActor() *def.Actor {
    return &def.Actor{
        UserID:           s.UserID,
        ServiceAccountID: s.ServiceAccountID,
        TenantID:         s.TenantID,
        Roles:            append([]string(nil), s.Roles...),
    }
}

// ToViewer converts the session to a ViewerContext for embedding in context.Context.
func (s *Session) ToViewer() ViewerContext {
    return NewViewer(s)
}
```

---

## 3. Redis Storage

**Key pattern:** `session:{token}`

**Value format:** JSON-serialized `Session` struct.

**TTL:** Set to `ExpiresAt - now` at session creation. Redis automatically expires the key.

**Example key:** `session:eyJhbGciOiJub25lIn0.eyJ1c2VyX2lkIjoiL...`

**Storage operations:**
- `SET session:{token} {json} EX {ttl_seconds}` — at login
- `GET session:{token}` — at every authenticated request
- `DEL session:{token}` — at logout or forced revocation

---

## 4. Session Validation Protocol

The middleware executes this sequence on every authenticated request:

```
1. Extract token from Authorization header: "Bearer {token}"
   → Missing or malformed → HTTP 401

2. Redis GET session:{token}
   → Key absent (expired or revoked) → HTTP 401

3. Deserialize JSON → Session struct
   → Deserialization failure → HTTP 500 (infrastructure error)

4. session.IsExpired(time.Now())
   → Expired → HTTP 401
   → DEL session:{token} (clean up expired session proactively)

5. Validate tenant status
   → Fetch tenant record from tenants table
   → status != 'ACTIVE' → appropriate HTTP status (402 SUSPENDED, 503 PENDING, 410 ARCHIVED)

6. session.ToViewer() → DefaultViewer
   → auth.WithViewer(ctx, viewer) → embed in context

7. set_tenant_context(tenantID) → sets PostgreSQL GUC for RLS

8. Proceed to route handler
```

**Redis unavailability:** Step 2 fails → HTTP 503. Session validation MUST NOT proceed without Redis confirmation. This is correct behavior — operating without session validation would be a security regression.

---

## 5. Session Lifecycle

```
Login request:
  → Authenticate user (password, OAuth, API key)
  → Fetch user's roles from IAM module
  → Generate cryptographically random token
  → Construct Session struct
  → Redis SET session:{token} {json} EX {ttl}
  → Return token to client

Authenticated request:
  → Middleware: GET session:{token}
  → Validate → embed ViewerContext
  → Request proceeds

Logout request:
  → Redis DEL session:{token}
  → Return HTTP 200

Session expiry:
  → Redis TTL reaches 0 → key deleted automatically
  → Next request with this token → HTTP 401

Forced revocation (e.g., role change, suspicious activity):
  → Redis DEL session:{token}
  → User must re-authenticate
```

---

## 6. Token Generation

Session tokens MUST be:
- Cryptographically random (use `crypto/rand`)
- At minimum 256 bits of entropy
- Base64url-encoded (no padding)

```go
func generateToken() (string, error) {
    b := make([]byte, 32)  // 256 bits
    if _, err := rand.Read(b); err != nil {
        return "", fmt.Errorf("generate session token: %w", err)
    }
    return base64.RawURLEncoding.EncodeToString(b), nil
}
```

---

## 7. Session Duration

Default session TTL: 24 hours for human users, configurable per tenant via the Settings module.

Service account session TTL: Configurable, typically longer (7 days to 90 days) or non-expiring (API key semantics with explicit revocation).

---

## 8. Normative Requirements

- Sessions MUST be stored in Redis, not PostgreSQL.
- Session tokens MUST have at minimum 256 bits of entropy.
- The middleware MUST reject expired sessions. It MUST NOT re-validate the expiry clock against the stored `ExpiresAt` if Redis TTL is relied upon — always call `IsExpired(time.Now())` as defense in depth.
- Session validation MUST fail closed (reject) when Redis is unavailable.
- `Roles` MUST be a snapshot at login time. Role changes MUST require re-authentication to take effect.
- Sessions MUST be revocable immediately via `DEL session:{token}`.
- The `RequestID` field MUST NOT be stored in Redis — it is set by middleware per request.

---

## 9. Security Considerations

- Store the raw `Token` only in Redis, never in application logs or error messages.
- The `session:` key prefix is predictable. Protect Redis with authentication and network isolation.
- Implement rate limiting on login attempts to prevent token brute-forcing.
- Session fixation: always generate a new token on successful login; never reuse pre-authentication tokens.

---

## References

- `awo/auth/session.go` — Session struct and methods
- [`03-auth/VIEWER_CONTEXT.md`](VIEWER_CONTEXT.md) — ViewerContext constructed from Session
- [`03-auth/ACTOR_MODEL.md`](ACTOR_MODEL.md) — Actor constructed from Session
- [`11-cache/CACHE_KEY_REFERENCE.md`](../11-cache/CACHE_KEY_REFERENCE.md) — All key patterns
- ADR-004 in [`00-overview/DECISION_REGISTER.md`](../00-overview/DECISION_REGISTER.md)
