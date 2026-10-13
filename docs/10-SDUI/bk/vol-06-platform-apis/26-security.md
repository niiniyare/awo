# Chapter 26 — Security Architecture

> **Volume:** VI — Platform APIs & Configuration
> **Phase:** 3
> **Audience:** Backend Engineers, Platform Engineers, Security Engineers
> **Prerequisites:** Chapters 03, 25

---

## Table of Contents

- [26.1 Security Principles](#261-security-principles)
- [26.2 Authentication](#262-authentication)
- [26.3 Transport Security](#263-transport-security)
- [26.4 CORS Policy](#264-cors-policy)
- [26.5 CSRF Protection](#265-csrf-protection)
- [26.6 Rate Limiting](#266-rate-limiting)
- [26.7 Input Validation and Injection Prevention](#267-input-validation-and-injection-prevention)
- [26.8 Secret Management](#268-secret-management)
- [26.9 Audit Logging](#269-audit-logging)
- [26.10 Security Headers](#2610-security-headers)

---

## 26.1 Security Principles

**Defense in depth.** No single control is trusted to be the only barrier. Authentication, authorization, transport encryption, input validation, rate limiting, and audit logging each operate independently. A failure in one layer does not compromise the entire system.

**Tenant isolation at every layer.** The `X-Tenant-ID` header is not self-trusted. The backend resolves the tenant from the authenticated user's token claims and validates that the declared `X-Tenant-ID` matches. Data queries use PostgreSQL Row Level Security (RLS) as the final enforcement boundary, independent of application-layer tenant checks.

**Least privilege by default.** Users have no permissions unless explicitly granted. The authorization system (Chapter 27) operates on a deny-by-default model. Service accounts for background jobs hold only the permissions required for their specific operations.

**Secrets never in code.** All credentials, signing keys, API keys, and connection strings are injected via environment variables from the platform's secret manager. The repository contains no secrets — only references.

---

## 26.2 Authentication

### 26.2.1 OAuth 2.0 with PKCE

AwoERP uses the OAuth 2.0 Authorization Code flow with PKCE (Proof Key for Code Exchange) for all client authentication. This flow is suitable for both public clients (SPAs, mobile apps) and confidential clients (backend services).

**Authorization flow:**

```
1. Client generates code_verifier (random 43–128 char string)
2. Client computes code_challenge = BASE64URL(SHA256(code_verifier))
3. Client redirects to:
   GET /auth/authorize
     ?response_type=code
     &client_id={client_id}
     &redirect_uri={redirect_uri}
     &scope=openid profile email offline_access
     &state={random_state}
     &code_challenge={code_challenge}
     &code_challenge_method=S256

4. Auth server authenticates user (SSO, MFA, etc.)
5. Auth server redirects back to redirect_uri with code and state
6. Client verifies state, then exchanges code:
   POST /auth/token
     grant_type=authorization_code
     &code={code}
     &code_verifier={code_verifier}
     &redirect_uri={redirect_uri}
     &client_id={client_id}

7. Auth server returns access_token + refresh_token
```

### 26.2.2 Token Structure

Access tokens are signed JWTs with the following payload:

```json
{
  "iss": "https://auth.awoerp.com",
  "sub": "usr-123",
  "aud": "awoerp-api",
  "exp": 1710500000,
  "iat": 1710496400,
  "jti": "tok-abc123",
  "tenant_id": "ten-456",
  "roles": ["finance_manager", "invoice_approver"],
  "permissions_hash": "sha256:def789"
}
```

Access tokens have a 1-hour lifetime. Refresh tokens have a 7-day lifetime and are rotated on each use. Refresh token rotation means a stolen refresh token can only be used once before the legitimate user invalidates it.

### 26.2.3 Token Validation Middleware

Every API handler (except `/auth/*` endpoints) passes through token validation middleware:

```go
func AuthMiddleware(jwtConfig JWTConfig) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            authHeader := r.Header.Get("Authorization")
            if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
                writeError(w, 401, "unauthenticated", "Missing or invalid Authorization header")
                return
            }

            tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
            claims, err := jwtConfig.Validate(tokenStr)
            if err != nil {
                writeError(w, 401, "unauthenticated", "Invalid or expired token")
                return
            }

            // Validate tenant claim matches X-Tenant-ID header
            declaredTenantID := r.Header.Get("X-Tenant-ID")
            if declaredTenantID != "" && declaredTenantID != claims.TenantID {
                writeError(w, 403, "forbidden", "Tenant mismatch")
                return
            }

            // Check token against revocation list (Redis)
            if jwtConfig.IsRevoked(r.Context(), claims.JWTID) {
                writeError(w, 401, "unauthenticated", "Token has been revoked")
                return
            }

            ctx := context.WithValue(r.Context(), contextKeyUserID, claims.Subject)
            ctx = context.WithValue(ctx, contextKeyTenantID, claims.TenantID)
            ctx = context.WithValue(ctx, contextKeyRoles, claims.Roles)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

### 26.2.4 Token Revocation

When a user logs out or an admin invalidates a session, the token's `jti` (JWT ID) is written to a Redis revocation set with a TTL matching the token's remaining lifetime. The middleware checks this set on every request.

```go
// On logout
func (s *AuthService) Logout(ctx context.Context, tokenID string, expiresAt time.Time) error {
    ttl := time.Until(expiresAt)
    if ttl <= 0 {
        return nil // Already expired
    }
    key := fmt.Sprintf("revoked_token:%s", tokenID)
    return s.redis.Set(ctx, key, "1", ttl).Err()
}

// On token validation
func (j *JWTConfig) IsRevoked(ctx context.Context, tokenID string) bool {
    key := fmt.Sprintf("revoked_token:%s", tokenID)
    exists, _ := j.redis.Exists(ctx, key).Result()
    return exists > 0
}
```

### 26.2.5 Multi-Factor Authentication

MFA is required for roles with elevated permissions (finance roles, admin roles). The auth service enforces this at the authorization endpoint before issuing an access token. The token's `amr` (Authentication Methods References) claim records the authentication methods used; the API middleware can reject tokens that do not include `mfa` in `amr` for protected resources.

---

## 26.3 Transport Security

### 26.3.1 HTTPS Everywhere

All client-to-backend communication is over HTTPS (TLS 1.2 minimum, TLS 1.3 preferred). HTTP requests are redirected to HTTPS with a 301. Strict Transport Security is set for 1 year with `includeSubDomains` and `preload`.

### 26.3.2 Certificate Pinning (Mobile)

Mobile clients pin the backend's TLS certificate public key hash. This prevents MITM attacks even in environments where the device trust store has been compromised:

**iOS (Swift):**
```swift
class PinnedSessionDelegate: NSObject, URLSessionDelegate {
    static let pinnedPublicKeyHash = "sha256/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

    func urlSession(_ session: URLSession,
                    didReceive challenge: URLAuthenticationChallenge,
                    completionHandler: @escaping (URLSession.AuthChallengeDisposition, URLCredential?) -> Void) {
        guard challenge.protectionSpace.authenticationMethod == NSURLAuthenticationMethodServerTrust,
              let serverTrust = challenge.protectionSpace.serverTrust,
              let certificate = SecTrustCopyCertificateChain(serverTrust) as? [SecCertificate],
              let firstCert = certificate.first else {
            completionHandler(.cancelAuthenticationChallenge, nil)
            return
        }

        let publicKeyHash = publicKeyHashForCertificate(firstCert)
        if publicKeyHash == Self.pinnedPublicKeyHash {
            completionHandler(.useCredential, URLCredential(trust: serverTrust))
        } else {
            completionHandler(.cancelAuthenticationChallenge, nil)
        }
    }
}
```

Certificate pins are updated via app updates. A 30-day overlap between old and new pins is maintained during rotation.

### 26.3.3 mTLS for gRPC (Mobile)

gRPC connections from mobile clients use mutual TLS. The backend issues client certificates during device registration. This provides an additional layer of transport-level authentication that complements the bearer token.

---

## 26.4 CORS Policy

The CORS policy allows requests only from registered origins (the web application domains). It is not a wildcard.

```go
func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
    originSet := make(map[string]struct{}, len(allowedOrigins))
    for _, o := range allowedOrigins {
        originSet[o] = struct{}{}
    }

    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            origin := r.Header.Get("Origin")
            if _, allowed := originSet[origin]; allowed {
                w.Header().Set("Access-Control-Allow-Origin", origin)
                w.Header().Set("Vary", "Origin")
                w.Header().Set("Access-Control-Allow-Credentials", "true")
                w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
                w.Header().Set("Access-Control-Allow-Headers",
                    "Authorization, Content-Type, X-Tenant-ID, X-Idempotency-Key, X-CSRF-Token")
                w.Header().Set("Access-Control-Max-Age", "86400")
            }

            if r.Method == http.MethodOptions {
                w.WriteHeader(http.StatusNoContent)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}
```

**Allowed origins** are configured per deployment environment and include all tenant custom domain names. A tenant that configures a custom domain (`app.acme.co.ke`) gets their domain added to the allowed origins list.

---

## 26.5 CSRF Protection

### 26.5.1 Double Submit Cookie Pattern

The web client uses the double submit cookie pattern for CSRF protection:

1. On login, the server sets a `csrf_token` cookie (HttpOnly: false, SameSite: Strict)
2. For every state-mutating request, the React client reads the cookie and sends it as the `X-CSRF-Token` request header
3. The middleware verifies the header value matches the cookie value

```go
func CSRFMiddleware() func(http.Handler) http.Handler {
    safeMethods := map[string]bool{"GET": true, "HEAD": true, "OPTIONS": true}

    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if safeMethods[r.Method] {
                next.ServeHTTP(w, r)
                return
            }

            cookieToken, err := r.Cookie("csrf_token")
            if err != nil {
                writeError(w, 403, "csrf_failed", "Missing CSRF token cookie")
                return
            }

            headerToken := r.Header.Get("X-CSRF-Token")
            if headerToken == "" || !hmac.Equal([]byte(cookieToken.Value), []byte(headerToken)) {
                writeError(w, 403, "csrf_failed", "CSRF token mismatch")
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}
```

### 26.5.2 SameSite Cookie Attribute

All session cookies use `SameSite=Strict`, which provides an additional layer of CSRF protection for modern browsers. The CSRF double-submit check remains as defense-in-depth for older browsers and non-browser clients.

### 26.5.3 Mobile Clients

Mobile clients do not use cookies and are not subject to CSRF. The bearer token in the `Authorization` header is the sole credential. The CSRF middleware skips requests that do not have an `Origin` header set to a browser origin.

---

## 26.6 Rate Limiting

### 26.6.1 Rate Limit Tiers

Rate limits are applied per user, per tenant, and globally. The Redis-based rate limiter uses a sliding window algorithm.

| Endpoint Category | Per-User Limit | Per-Tenant Limit |
|------------------|---------------|-----------------|
| Surface definition fetch | 120/min | 10,000/min |
| Data source query | 300/min | 50,000/min |
| Mutations (POST/PUT/PATCH/DELETE) | 60/min | 5,000/min |
| Bulk mutations | 10/min | 500/min |
| Auth endpoints | 10/min | 100/min |
| SSE stream connections | 50 concurrent | 5,000 concurrent |

### 26.6.2 Rate Limit Response

When a rate limit is exceeded, the server returns:

```
HTTP/1.1 429 Too Many Requests
Retry-After: 15
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1710497000

{
  "error": "rate_limited",
  "message": "Too many requests. Please retry after 15 seconds.",
  "retry_after_seconds": 15
}
```

### 26.6.3 Go Rate Limiter

```go
type RateLimiter struct {
    redis  *redis.Client
    limits map[string]RateLimit
}

type RateLimit struct {
    Requests int
    Window   time.Duration
}

func (rl *RateLimiter) Allow(ctx context.Context, key string, limit RateLimit) (bool, int, error) {
    now := time.Now().UnixMilli()
    windowStart := now - limit.Window.Milliseconds()
    pipe := rl.redis.Pipeline()

    // Sliding window log
    pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStart, 10))
    pipe.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: now})
    count := pipe.ZCard(ctx, key)
    pipe.Expire(ctx, key, limit.Window)

    _, err := pipe.Exec(ctx)
    if err != nil {
        return true, limit.Requests, err // Fail open on Redis errors
    }

    current := int(count.Val())
    remaining := limit.Requests - current
    if remaining < 0 {
        remaining = 0
    }

    return current <= limit.Requests, remaining, nil
}
```

---

## 26.7 Input Validation and Injection Prevention

### 26.7.1 SQL Injection Prevention

All database queries use parameterized queries via `sqlc`-generated code. No string concatenation is used to construct SQL. The `store` layer is the only code that touches the database, and it uses typed query functions:

```go
// Generated by sqlc — no string concatenation
func (q *Queries) GetInvoice(ctx context.Context, arg GetInvoiceParams) (Invoice, error) {
    row := q.db.QueryRowContext(ctx, getInvoice, arg.TenantID, arg.ID)
    // ...
}
```

PostgreSQL RLS provides a final safety net — even if a query were somehow manipulated, RLS policies enforce that only the current tenant's rows are visible.

### 26.7.2 Expression Injection Prevention

The SDUI `$expr{}` evaluator runs in a sandboxed scope with no access to the Go runtime, file system, network, or any external state beyond the explicitly provided binding scope. The evaluator whitelist is checked at compile time (Chapter 8), and any expression referencing out-of-scope identifiers is rejected.

### 26.7.3 Request Body Size Limits

All endpoints enforce a maximum request body size to prevent memory exhaustion:

```go
func BodySizeLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
            next.ServeHTTP(w, r)
        })
    }
}
```

Default limits: 1MB for JSON mutations, 10MB for file uploads, 100MB for bulk import.

### 26.7.4 Filename and Path Sanitization

File upload endpoints sanitize filenames to prevent path traversal:

```go
func sanitizeFilename(name string) string {
    // Remove directory components
    name = filepath.Base(name)
    // Replace non-alphanumeric chars (except . - _) with _
    safe := regexp.MustCompile(`[^a-zA-Z0-9.\-_]`).ReplaceAllString(name, "_")
    // Truncate to 255 chars
    if len(safe) > 255 {
        safe = safe[:255]
    }
    return safe
}
```

Uploaded files are stored in isolated blob storage (not the filesystem) with a UUID-based path, never the original filename as the storage key.

---

## 26.8 Secret Management

### 26.8.1 Environment Variable Injection

Secrets are never committed to the repository. All credentials are injected at runtime via environment variables from the platform's secret manager. The Go application reads these at startup:

```go
type Config struct {
    Database   DatabaseConfig
    Redis      RedisConfig
    JWT        JWTConfig
    SMTP       SMTPConfig
    Storage    StorageConfig
}

func LoadConfig() (*Config, error) {
    return &Config{
        Database: DatabaseConfig{
            DSN: requireEnv("DATABASE_URL"),
        },
        Redis: RedisConfig{
            URL:      requireEnv("REDIS_URL"),
            Password: os.Getenv("REDIS_PASSWORD"), // Optional for dev
        },
        JWT: JWTConfig{
            SigningKey: requireEnv("JWT_SIGNING_KEY"),
            PublicKey:  requireEnv("JWT_PUBLIC_KEY"),
        },
    }, nil
}

func requireEnv(key string) string {
    val := os.Getenv(key)
    if val == "" {
        panic(fmt.Sprintf("required environment variable %s is not set", key))
    }
    return val
}
```

### 26.8.2 Key Rotation

JWT signing keys are rotated on a 90-day schedule. The key rotation process:
1. New key pair generated and stored in the secret manager
2. New key added to the JWKS endpoint alongside the old key (with `kid` discriminator)
3. New tokens are issued with the new key's `kid`
4. Old tokens remain valid until expiry (max 1 hour)
5. Old key removed from JWKS after all old tokens have expired

---

## 26.9 Audit Logging

### 26.9.1 What is Logged

Every mutation (POST, PUT, PATCH, DELETE) generates an audit log entry. Surface definition fetches are not audit-logged (they are too high volume; telemetry covers them). The following are always logged:

- User authentication events (login, logout, token refresh, MFA)
- Permission changes (role assignments, permission grants)
- Tenant configuration changes
- Resource mutations (create, update, delete, bulk operations)
- Workflow signals (approve, reject, escalate)
- Admin actions

### 26.9.2 Audit Log Structure

```go
type AuditEvent struct {
    ID         string         `json:"id"`
    Timestamp  time.Time      `json:"timestamp"`
    TenantID   string         `json:"tenant_id"`
    UserID     string         `json:"user_id"`
    UserAgent  string         `json:"user_agent"`
    IPAddress  string         `json:"ip_address"`
    RequestID  string         `json:"request_id"`
    Action     string         `json:"action"`      // e.g. "invoice.approve"
    Resource   string         `json:"resource"`    // e.g. "invoice"
    ResourceID string         `json:"resource_id"` // e.g. "inv-123"
    Changes    map[string]any `json:"changes"`     // before/after for updates
    Result     string         `json:"result"`      // "success" | "failure"
    ErrorCode  string         `json:"error_code,omitempty"`
}
```

### 26.9.3 Audit Middleware

```go
func AuditMiddleware(auditStore AuditStore) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if isSafeMethod(r.Method) {
                next.ServeHTTP(w, r)
                return
            }

            rec := &statusRecorder{ResponseWriter: w, status: 200}
            next.ServeHTTP(rec, r)

            event := AuditEvent{
                ID:        newID(),
                Timestamp: time.Now(),
                TenantID:  middleware.TenantID(r.Context()),
                UserID:    middleware.UserID(r.Context()),
                UserAgent: r.UserAgent(),
                IPAddress: realIP(r),
                RequestID: r.Header.Get("X-Request-ID"),
                Action:    routeToAction(r),
                Result:    statusToResult(rec.status),
            }

            // Non-blocking write to audit queue
            go auditStore.Write(context.Background(), event)
        })
    }
}
```

Audit logs are written to an append-only store (separate PostgreSQL schema with no UPDATE/DELETE grants to the application user). Audit logs are retained for 7 years per financial record-keeping requirements.

---

## 26.10 Security Headers

All responses include the following security headers:

```go
func SecurityHeadersMiddleware() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            h := w.Header()
            // Prevent MIME sniffing
            h.Set("X-Content-Type-Options", "nosniff")
            // Prevent clickjacking
            h.Set("X-Frame-Options", "DENY")
            // XSS Protection (belt-and-suspenders for older browsers)
            h.Set("X-XSS-Protection", "1; mode=block")
            // HSTS — 1 year, include subdomains, preload
            h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
            // Content Security Policy
            h.Set("Content-Security-Policy",
                "default-src 'self'; "+
                "script-src 'self' 'nonce-{nonce}'; "+
                "style-src 'self' 'unsafe-inline' fonts.googleapis.com; "+
                "font-src fonts.gstatic.com; "+
                "img-src 'self' data: cdn.awoerp.com; "+
                "connect-src 'self' wss://api.awoerp.com; "+
                "frame-ancestors 'none'")
            // Referrer Policy
            h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
            // Permissions Policy
            h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")

            next.ServeHTTP(w, r)
        })
    }
}
```

The CSP `nonce` is generated per request and injected into the HTML shell template for inline scripts (React hydration scripts).

---

*End of Chapter 26*

**Previous:** [Chapter 25 — API Contracts](./25-api-contracts.md)
**Next:** [Chapter 27 — Authorization](./27-authorization.md)
