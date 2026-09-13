> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "ADR-005: Server-Side Sessions over JWT"
id: adr-005
status: accepted
category: ADR
stability: FROZEN
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: informative
related:
  - "[Sessions](../07-iam/sessions.md)"
  - "[Authentication](../07-iam/authentication.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
  - "[Invariants](../02-architecture/invariants.md)"
---

# ADR-005: Server-Side Sessions over JWT

**Status:** Accepted
**Date:** 2024-01-25
**Authors:** Framework Team

---

## Context

Awo's IAM layer needs to validate user identity on every request. The two dominant approaches are JSON Web Tokens (JWT) and server-side sessions. The choice has security implications for session revocation, which matters for an ERP system where immediate access revocation (suspended user, compromised account) is a compliance requirement.

---

## Options Considered

### Option A: JWT (Stateless Tokens)

The server issues a signed JWT. Validation requires only the signing key — no database or cache lookup.

Pros:
- Stateless: scales horizontally without shared session store
- No Redis dependency for validation

Cons:
- **No immediate revocation**: a valid JWT remains valid until expiry, even if the user is suspended or the account is compromised. The only mitigation is short expiry (minutes) + refresh tokens, which adds complexity.
- Refresh token revocation requires a blacklist — effectively a session store (defeats statelessness benefit)
- Role changes don't take effect until token expiry (user retains old permissions)
- Short-lived tokens degrade UX (frequent re-authentication)

### Option B: Server-Side Sessions with Redis

The server issues an opaque random token. Session data (actor, roles, expiry) stored in Redis. Validation requires a Redis lookup.

Pros:
- Immediate revocation: `DEL session:{token}` instantly invalidates all requests
- Role changes take effect immediately on next request (session data contains current roles)
- Simple security model: token is random, not signed — no key rotation complexity

Cons:
- Redis is now a hard dependency for authentication
- Redis unavailability means all authentication fails (cannot validate any session)
- Session store must be highly available

---

## Decision

**Use server-side sessions with Redis.**

For ERP systems, immediate revocation is non-negotiable. A suspended user or compromised account must be locked out in seconds, not after token expiry. The Redis hard dependency is acceptable — Redis is already required for caching; making it a hard authentication dependency is correct behavior (better to fail closed than to accept stale sessions).

---

## Consequences

**Positive:**
- Immediate revocation: `DEL session:{token}` = instant lockout
- Role changes reflected immediately
- Simple security model: no key rotation, no token signing

**Negative:**
- Redis unavailability = complete authentication failure (all requests return 503)
- This is the correct behavior, but must be operationally planned for

**Architecture Laws generated:**
- INV-007: Session validation always requires live Redis

---

## Revisit Trigger

If Redis availability becomes a concern at scale, consider a multi-region Redis setup or a hybrid approach: short-lived JWT (5 minutes) with a server-side revocation list (checked only on JWT validation, not on every request). This reduces Redis load while preserving near-immediate revocation.
