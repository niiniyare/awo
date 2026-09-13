> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "ADR-016: Server-Side Sessions over JWT"
id: adr-016
status: accepted
category: ADR
stability: STABLE
audience: [framework-authors, module-authors]
since: "1.0"
normative-level: normative
related:
  - "[IAM Sessions](../07-iam/sessions.md)"
  - "[MFA](../07-iam/mfa.md)"
  - "[RBAC](../07-iam/rbac.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# ADR-016: Server-Side Sessions over JWT

**Status: Accepted**

---

## Context

Awo needs to authenticate API requests and associate them with a user identity and tenant. The two primary options are stateless JSON Web Tokens (JWT) and stateful server-side sessions backed by Redis.

ERP systems have specific requirements that differ from typical web applications:
- **Immediate revocation**: when a user is terminated or a session is compromised, access must be cut off within seconds, not hours
- **Single active session policy**: some tenants require that a user can only have one active session at a time
- **MFA state**: multi-step authentication requires storing intermediate state between requests
- **Session audit**: compliance requires knowing when each session was created, from which IP, and when it expired or was revoked

---

## Decision

Use **opaque server-side sessions** stored in Redis. Session tokens are 256-bit cryptographically random values. The session record in Redis stores: user ID, tenant ID, roles, IP, user agent, expiry, MFA status.

---

## Consequences

### Positive

**Immediate revocation**: deleting or expiring the Redis key instantly invalidates the session. No waiting for JWT expiry. Termination → access cut-off in milliseconds.

**Single active session**: enforced by tracking all sessions per user; revoke others on new login.

**MFA intermediate state**: the `mfa_challenge_token` flows naturally — it is a short-lived Redis key pointing to a partially-authenticated session.

**No secret distribution**: no JWT signing keys to rotate and distribute across instances. The Redis store is the single authority.

**Auditability**: every session has an exact creation timestamp, last-seen timestamp, and IP. Queries like "list all active sessions for user X" are trivial.

### Negative

**Redis is a hard dependency for authentication**: if Redis is unavailable, authentication fails. This is intentional and security-correct — cannot authenticate without the session store. Operators must ensure Redis high availability.

**Session lookup latency**: every request makes a Redis `GET`. Typically sub-millisecond in the same datacenter. Acceptable for ERP usage patterns.

**Horizontal scaling caveat**: all instances must connect to the same Redis cluster. Partitioned Redis deployments must use consistent hashing or a single primary.

---

## Alternatives Considered

### JWT (stateless)

JWTs can be verified without a Redis lookup — useful for very high read volumes. However:

- **No immediate revocation**: a JWT signed with a 1-hour expiry cannot be revoked before that hour is up without introducing a blocklist — which reintroduces statefulness.
- **MFA complexity**: storing MFA state in a JWT claim requires signing a new token after MFA completion, which complicates the auth flow.
- **Refresh token management**: short-lived access tokens + refresh tokens add complexity; refresh tokens require server-side storage anyway.

JWT is appropriate for microservice-to-microservice auth where services cannot share a Redis cluster. Awo is a monolith — this constraint does not apply.

### Database-backed sessions

Using PostgreSQL for session storage avoids the Redis hard dependency for auth. Rejected because:
- Session lookups on every request add DB load
- Cannot use `FORCE ROW LEVEL SECURITY` on the session table (session validation happens before tenant context is established)
- Redis `GET` is ~0.1ms; PostgreSQL session lookup is ~1–5ms

---

## Implementation Notes

- Token format: `hex(32 random bytes)` = 64 hex characters
- Redis key: `session:{token}`
- TTL: configurable per tenant; default 8 hours; activity sliding window resets on each request
- Session record: stored as Redis hash (`HSET`)
- Never log the session token — only log a truncated hint (first 8 chars) for debugging

---

## Related Documents

- [IAM Sessions](../07-iam/sessions.md) — full session lifecycle implementation
- [MFA](../07-iam/mfa.md) — MFA challenge token pattern built on sessions
- [Architecture Laws](../02-architecture/laws.md) — LAW-003 (Redis hard dependency for auth)
