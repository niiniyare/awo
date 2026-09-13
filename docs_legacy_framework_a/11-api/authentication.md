> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "API Authentication"
id: api-008
status: accepted
category: SPEC
stability: FROZEN
audience: [module-authors, api-consumers, operators]
since: "1.0"
normative-level: normative
related:
  - "[Sessions](../07-iam/sessions.md)"
  - "[Authentication](../07-iam/authentication.md)"
  - "[API Clients](../07-iam/api-clients.md)"
  - "[Entity API Reference](entity-api-reference.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# API Authentication

**API-008 | Status: Accepted | Stability: Frozen**

How clients authenticate to the Awo API: session tokens, API client credentials, tenant identification, and error responses.

---

## 1. Authentication Methods

| Method | Use Case | Token Header |
|---|---|---|
| Session token | Human users (browser, mobile) | `Authorization: Bearer {token}` |
| API client credentials | Machine-to-machine | `Authorization: Bearer {token}` (same header, different token type) |

There is no cookie-based authentication. All tokens are in the `Authorization` header.

---

## 2. Session Token Flow (Human Users)

### Login

```
POST /api/v1/auth/login
Content-Type: application/json
X-Tenant-ID: {tenant-uuid}

{
  "email": "user@company.com",
  "password": "correct-horse-battery-staple"
}
```

**Success (no MFA)**:

```json
HTTP 200
{
  "data": {
    "token": "3f2a9b8c...",
    "expires_at": "2024-12-16T09:00:00+03:00",
    "user": {
      "id": "usr_01j...",
      "email": "user@company.com",
      "display_name": "Alice Kamau",
      "roles": ["role:finance.viewer"]
    }
  }
}
```

**MFA Required**:

```json
HTTP 200
{
  "data": {
    "mfa_required": true,
    "mfa_session_token": "mfa_3f2a...",   // short-lived, MFA-only token
    "mfa_methods": ["totp"]
  }
}
```

### MFA Verification

```
POST /api/v1/auth/mfa/verify
Authorization: Bearer {mfa_session_token}
X-Tenant-ID: {tenant-uuid}

{
  "code": "123456"
}
```

Returns same `{ data: { token, expires_at, user } }` on success.

### Logout

```
POST /api/v1/auth/logout
Authorization: Bearer {token}
X-Tenant-ID: {tenant-uuid}
```

Immediately revokes the session in Redis. Token is invalid for all subsequent requests.

---

## 3. API Client Flow (Machine-to-Machine)

### Obtain Token

```
POST /api/v1/auth/token
Content-Type: application/json
X-Tenant-ID: {tenant-uuid}

{
  "client_id": "client_01j...",
  "client_secret": "secret_01j..."
}
```

**Success**:

```json
HTTP 200
{
  "data": {
    "token": "apiclient_3f2a...",
    "expires_at": "2024-12-16T22:00:00+03:00",
    "scopes": ["invoice:read", "invoice:create"]
  }
}
```

API client tokens are Redis-backed server-side sessions — same infrastructure as user sessions. Expiry is configurable per client (default: 8 hours).

---

## 4. Tenant Identification

Every request requires tenant identification. Resolved in priority order:

| Method | Header / Source | Use Case |
|---|---|---|
| 1st | `X-Tenant-ID: {uuid}` | API clients, mobile apps |
| 2nd | `?tenant_id={uuid}` | Webhooks, legacy (disable in prod) |
| 3rd | Subdomain | Browser: `bo.{tenant}.awo.so` |

Missing or invalid tenant → `401 Unauthorized` before authentication is attempted.

---

## 5. Token Usage

Every authenticated request:

```
GET /api/v1/entities/finance_invoice
Authorization: Bearer 3f2a9b8c...
X-Tenant-ID: 018e1234-...
```

The middleware pipeline (see [Architecture Laws](../02-architecture/laws.md)) validates the token and sets the actor context before any handler runs.

---

## 6. Token Validation Steps

Framework middleware validates every token:

1. Extract `Authorization: Bearer {token}`
2. `HGETALL session:{token}` in Redis
3. If miss → `401 Unauthorized`
4. Check `expires_at` > now → if expired, delete key, return `401`
5. Verify `tenant_id` in session matches `X-Tenant-ID` header
6. Load user roles from session → set `Actor` in request context
7. Slide TTL: `EXPIRE session:{token} {ttl}` (if sliding window)

Step 2 is synchronous. Redis unavailability → `503 Service Unavailable` (correct: cannot authenticate without session store).

---

## 7. Error Responses

| Scenario | HTTP Status | Error Code |
|---|---|---|
| Missing Authorization header | 401 | `auth.missing_token` |
| Invalid / expired token | 401 | `auth.invalid_token` |
| Missing X-Tenant-ID | 401 | `auth.missing_tenant` |
| Invalid tenant ID format | 400 | `auth.invalid_tenant_id` |
| Tenant not found or not ACTIVE | 401 | `auth.tenant_unavailable` |
| Valid token, wrong tenant | 403 | `auth.tenant_mismatch` |
| Insufficient permissions | 403 | `auth.forbidden` |
| MFA required but not completed | 403 | `auth.mfa_required` |
| Account locked (too many attempts) | 429 | `auth.account_locked` |

All error responses use the standard envelope:

```json
{
  "error": {
    "code": "auth.invalid_token",
    "message": "Authentication required"
  }
}
```

Stack traces are never included. Internal error details are logged server-side with `request_id`.

---

## 8. Rate Limiting on Auth Endpoints

Auth endpoints have tighter rate limits than data endpoints:

| Endpoint | Limit | Window |
|---|---|---|
| `POST /auth/login` | 10 attempts | 1 minute per IP + tenant |
| `POST /auth/token` | 20 attempts | 1 minute per client ID |
| `POST /auth/mfa/verify` | 5 attempts | 5 minutes per MFA session |

Exceeded limits → `429 Too Many Requests` with `Retry-After` header. Lock persists in Redis for the window duration.

---

## 9. Session Security Headers

All API responses include:

```
Strict-Transport-Security: max-age=63072000; includeSubDomains; preload
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Cache-Control: no-store
```

Auth responses additionally include:

```
Pragma: no-cache
```

---

## Related Documents

- [Sessions](../07-iam/sessions.md) — Redis session storage internals
- [Session Management](../07-iam/session-management.md) — sliding TTL, revocation
- [Authentication](../07-iam/authentication.md) — login flow, bcrypt, constant-time
- [API Clients](../07-iam/api-clients.md) — machine-to-machine credentials
- [Entity API Reference](entity-api-reference.md) — authenticated data endpoints
