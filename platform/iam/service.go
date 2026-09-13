package iam

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"awo.so/awo/audit"
	"awo.so/awo/auth"
	"awo.so/awo/cache"
	"awo.so/awo/def"
	"awo.so/awo/driver"
	"awo.so/awo/filter"
	"awo.so/awo/runtime"
	"awo.so/awo/runtime/tenant"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const (
	// humanSessionTTL is the default session lifetime for human users.
	humanSessionTTL = 24 * time.Hour

	// apiTokenCacheTTL is how long a validated API token is cached in Redis.
	// Avoids a database round-trip on every service account request.
	// A short TTL (60s) ensures revocation takes effect promptly.
	apiTokenCacheTTL = 60 * time.Second

	// serviceAccountRoles is the default role set for all service accounts.
	// Service account role management is a future enhancement; for now all
	// service accounts carry role:api-client.
	serviceAccountDefaultRole = "role:api-client"
)

// IAMRepositories groups the EntityRepository instances used by AuthService.
// Grouping avoids constructor explosion when adding repository operations.
//
// All three repositories target tenant-scoped tables. Repository operations
// that write or update data must be called inside [driver.EntityRepository.WithTx]
// so that the pgx contrib driver can call set_tenant_context before executing
// any DML — activating PostgreSQL Row Level Security for the transaction.
//
//   - Sessions: iam_session (SQL audit trail; authoritative store is Redis)
//   - Users:    iam_user   (last_login_at updates on successful login)
//   - UserRoles: iam_user_role (role loading at login)
type IAMRepositories struct {
	Sessions  driver.EntityRepository[*def.EntityRecord]
	Users     driver.EntityRepository[*def.EntityRecord]
	UserRoles driver.EntityRepository[*def.EntityRecord]
}

// AuthService provides the authentication and session management operations
// that the API middleware and IAM handlers depend on.
//
// It is the sole writer to:
//   - Session store (human sessions, via [Sessions])
//   - API token cache (service account token cache, via [Cache])
//   - iam_sessions table (SQL audit trail for human sessions, via [Repos.Sessions])
//   - iam_users.last_login_at (via [Repos.Users])
//   - platform_audit_log (unified audit trail, via [AuditWriter])
//
// AuthService is constructed once at startup by [New] and shared across all
// concurrent requests. All methods are goroutine-safe.
type AuthService struct {
	DB          *pgxpool.Pool
	Sessions    auth.SessionStore
	Cache       cache.Cache
	Repos       IAMRepositories
	AuditWriter audit.AuditWriter // nil = audit disabled (dev/test)
}

// ── Login ──────────────────────────────────────────────────────────────────────

// LoginInput carries the credentials and metadata for a login attempt.
type LoginInput struct {
	Email     string
	Password  string
	TenantID  uuid.UUID
	DeviceID  string
	IPAddress string
}

// LoginResult carries the session token and session record returned after a
// successful login. The caller returns Token to the client as a Bearer credential.
type LoginResult struct {
	Token   string
	Session *auth.Session
}

// Login authenticates a human user and issues a new session.
//
// The credential verification sequence runs inside a single database transaction
// so that the tenant RLS context (set by set_tenant_context) is guaranteed to
// be active for the credential lookup on the same connection. Without a
// transaction, pgxpool may dispatch each DB call to a different connection where
// the tenant context has not been set, making RLS invisible.
//
// Role loading runs outside the credential transaction, in its own
// EntityRepository.WithTx call. This separates the sensitive credential path
// from the role-loading path and allows each to fail independently. The session
// model (ADR-004) bakes roles into sessions; stale roles are acceptable until
// the next session refresh.
//
// Flow:
//  1. Open a read transaction and establish tenant RLS context.
//  2. Load the iam_user record by email (RLS scopes to tenant).
//  3. Verify the bcrypt password hash (constant-time comparison).
//  4. Commit the credential transaction.
//  5. Load the user's active roles via EntityRepository.
//  6. Generate a cryptographically random 256-bit session token.
//  7. Store the session in Redis with TTL (critical path — failure aborts login).
//  8. Write SQL audit records and update last_login_at (best-effort).
//
// On credential failure, Login returns a generic error to prevent user
// enumeration. The specific failure reason is written to platform_audit_log.
//
// Redis failure on step 7 causes Login to fail — sessions cannot be issued when
// the session store is unavailable. This is correct security behaviour.
func (s *AuthService) Login(ctx context.Context, input LoginInput) (*LoginResult, error) {
	// Phase 1: Verify credentials in a transaction.
	// The transaction keeps the tenant RLS context active for all DB calls
	// inside it (set_tenant_context is called once at the start).
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("iam: login: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after Commit; ensures cleanup on early return

	// Establish tenant RLS context for all queries in this transaction.
	// set_tenant_context() calls set_config('app.current_tenant_id', $1, TRUE).
	// The TRUE (is_local) flag makes the setting transaction-local — it reverts
	// automatically on COMMIT or ROLLBACK, so the connection returned to the pool
	// is clean and will not leak the tenant context to the next request.
	if _, err := tx.Exec(ctx, sqlSetTenantContext, input.TenantID); err != nil {
		return nil, fmt.Errorf("iam: login: set tenant context: %w", err)
	}

	var (
		userID       uuid.UUID
		passwordHash string
		status       string
	)
	err = tx.QueryRow(ctx, sqlLookupCredentials, input.Email).Scan(&userID, &passwordHash, &status)
	if err != nil {
		// Rollback the read transaction before writing the failure audit.
		tx.Rollback(ctx)
		s.writeFailedLoginAudit(ctx, input, uuid.Nil, "user_not_found")
		return nil, &runtime.BusinessError{
			Code:    "iam.login.invalid_credentials",
			Message: "Invalid email or password.",
			Status:  401,
		}
	}

	if status != "active" {
		tx.Rollback(ctx)
		s.writeFailedLoginAudit(ctx, input, userID, "account_"+status)
		return nil, &runtime.BusinessError{
			Code:    "iam.login.account_" + status,
			Message: "Your account is " + status + ". Contact your administrator.",
			Status:  403,
		}
	}

	// bcrypt.CompareHashAndPassword performs a constant-time comparison, making
	// it resistant to timing attacks that could reveal whether a user exists.
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(input.Password)); err != nil {
		tx.Rollback(ctx)
		s.writeFailedLoginAudit(ctx, input, userID, "invalid_password")
		return nil, &runtime.BusinessError{
			Code:    "iam.login.invalid_credentials",
			Message: "Invalid email or password.",
			Status:  401,
		}
	}

	// Commit the credential transaction. Credentials are verified.
	// Role loading runs in its own EntityRepository.WithTx below — this keeps
	// the sensitive credential path short and avoids holding the tx open during
	// the additional role query.
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("iam: login: commit: %w", err)
	}

	// Phase 2: Load user roles via EntityRepository.
	// WithTx establishes tenant RLS context from TenantContext already in ctx
	// (injected by TenantResolver middleware before Login was called).
	// Session model ADR-004: roles are baked into sessions; role changes only
	// take effect after the user re-authenticates. Stale roles in the window
	// between credential commit and role load are acceptable.
	roles, err := s.loadUserRoles(ctx, input.TenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("iam: login: load roles: %w", err)
	}

	// Phase 3: Generate and store the session.
	// Token is 256 bits of cryptographically random entropy, base64url-encoded.
	token, err := auth.GenerateToken()
	if err != nil {
		return nil, fmt.Errorf("iam: login: generate token: %w", err)
	}

	now := time.Now()
	session := &auth.Session{
		Token:            token,
		UserID:           userID,
		ServiceAccountID: uuid.Nil,
		TenantID:         input.TenantID,
		Roles:            roles,
		ExpiresAt:        now.Add(humanSessionTTL),
		IssuedAt:         now,
		DeviceID:         input.DeviceID,
		IPAddress:        input.IPAddress,
	}

	// Store in Redis — this is the critical path. If Redis is unavailable, login
	// fails. A session that exists only in Redis but not in SQL is live and valid;
	// the SQL record is an audit trail, not the authoritative session store.
	if err := s.storeSession(ctx, session); err != nil {
		return nil, fmt.Errorf("iam: login: store session: %w", err)
	}

	// Phase 4: Best-effort audit writes — session record + last_login_at.
	// These are non-fatal; the session is already live in Redis.
	s.auditLogin(ctx, session, input)

	return &LoginResult{Token: token, Session: session}, nil
}

// auditLogin persists the session SQL record and updates last_login_at in a
// single best-effort EntityRepository transaction. Failures are logged and
// silently ignored — the session is already live in Redis and the user is
// authenticated.
//
// Both operations share one WithTx call so they commit atomically. WithTx
// establishes tenant RLS context automatically from TenantContext in ctx.
func (s *AuthService) auditLogin(ctx context.Context, session *auth.Session, input LoginInput) {
	if err := s.Repos.Sessions.WithTx(ctx, func(txCtx context.Context) error {
		if _, err := s.Repos.Sessions.Create(txCtx, driver.CreateInput{
			Data: map[string]any{
				"token_hash":         tokenHash(session.Token),
				"user_id":            nullUUID(session.UserID),
				"service_account_id": nullUUID(session.ServiceAccountID),
				"issued_at":          session.IssuedAt,
				"expires_at":         session.ExpiresAt,
				"device_id":          session.DeviceID,
				"ip_address":         session.IPAddress,
			},
		}); err != nil {
			return fmt.Errorf("create session record: %w", err)
		}
		// last_login_at is only meaningful for human user sessions.
		// UserID is always set in Login; this guard is defensive.
		if session.UserID != uuid.Nil {
			if _, err := s.Repos.Users.Update(txCtx, session.UserID, driver.UpdateInput{
				Data: map[string]any{"last_login_at": session.IssuedAt},
			}); err != nil {
				return fmt.Errorf("update last_login_at: %w", err)
			}
		}
		return nil
	}); err != nil {
		slog.WarnContext(ctx, "iam: audit login: persistence failed (best-effort)",
			"user_id", session.UserID,
			"tenant_id", session.TenantID,
			"err", err,
		)
	}

	// Write the unified audit record after persistence.
	// platform_audit_log has no RLS; no tenant context required.
	s.writeAuthAudit(ctx, session.TenantID,
		&def.Actor{UserID: session.UserID, TenantID: session.TenantID},
		audit.OperationLogin,
		input.IPAddress,
		map[string]any{"device_id": input.DeviceID},
	)
}

// ── Logout ────────────────────────────────────────────────────────────────────

// Logout invalidates an active session identified by the session record.
//
// Flow:
//  1. DEL session:{token} from Redis (authoritative revocation).
//  2. ZREM token from the user_sessions sorted set index.
//  3. BulkUpdate iam_sessions SET revoked_at (best-effort, via EntityRepository).
//  4. Write logout event to platform_audit_log.
func (s *AuthService) Logout(ctx context.Context, session *auth.Session) error {
	// SessionStore.Delete is the authoritative revocation. The primary session
	// key removal is critical (error returned on failure); the user index entry
	// is removed best-effort inside Delete.
	if err := s.Sessions.Delete(ctx, session); err != nil {
		return fmt.Errorf("iam: logout: %w", err)
	}

	// Best-effort: mark the SQL audit record revoked and write a logout event.
	s.auditLogout(ctx, session)
	return nil
}

// auditLogout marks the session revoked in iam_sessions and writes the logout
// event to platform_audit_log. Best-effort — Redis revocation has already occurred.
//
// BulkUpdate runs inside WithTx so that set_tenant_context is called before
// the UPDATE, activating PostgreSQL RLS for the iam_sessions table.
func (s *AuthService) auditLogout(ctx context.Context, session *auth.Session) {
	hash := tokenHash(session.Token)

	if err := s.Repos.Sessions.WithTx(ctx, func(txCtx context.Context) error {
		_, err := s.Repos.Sessions.BulkUpdate(
			txCtx,
			filter.And(
				filter.Eq("token_hash", hash),
				filter.IsNull("revoked_at"),
			),
			driver.Patch{Set: map[string]any{
				"revoked_at": time.Now().UTC(),
			}},
		)
		return err
	}); err != nil {
		slog.WarnContext(ctx, "iam: audit logout: session revocation failed (best-effort)",
			"tenant_id", session.TenantID,
			"err", err,
		)
	}

	// Write the unified audit record after the BulkUpdate.
	s.writeAuthAudit(ctx, session.TenantID,
		&def.Actor{UserID: session.UserID, TenantID: session.TenantID},
		audit.OperationLogout,
		session.IPAddress,
		map[string]any{"device_id": session.DeviceID},
	)
}

// ── RevokeUserSessions ────────────────────────────────────────────────────────

// RevokeUserSessions revokes ALL active sessions for the given user within the
// tenant. Called by the UserRoleChangeHook after role assignments change, and by
// IAM administrators via the management API.
//
// Session tokens are retrieved from the user_sessions:{tenantID}:{userID} sorted
// set. All session keys are deleted from Redis atomically. The SQL audit table is
// updated and an audit event written in a best-effort BulkUpdate.
//
// Note: if storeSession failed to add a token to the sorted set (it is
// best-effort), RevokeUserSessions will not find that token and cannot revoke it.
// This edge case cannot be avoided without making session creation synchronous
// on the sorted set write.
func (s *AuthService) RevokeUserSessions(ctx context.Context, tenantID, userID uuid.UUID) error {
	tokens, err := s.Sessions.ListUserTokens(ctx, tenantID, userID)
	if err != nil {
		return fmt.Errorf("iam: revoke user sessions: list tokens: %w", err)
	}
	if len(tokens) == 0 {
		return nil
	}

	hashes := make([]string, len(tokens))
	for i, tok := range tokens {
		hashes[i] = tokenHash(tok)
	}

	if err := s.Sessions.DeleteAll(ctx, tenantID, userID, tokens); err != nil {
		return fmt.Errorf("iam: revoke user sessions: delete: %w", err)
	}

	// Best-effort: mark sessions revoked in SQL and write audit event.
	s.auditRevoke(ctx, tenantID, userID, hashes)
	return nil
}

// auditRevoke marks sessions revoked in iam_sessions via BulkUpdate and writes
// a session_revoked audit event. Best-effort — Redis revocation has already occurred.
//
// BulkUpdate runs inside WithTx for RLS. filter.InStrings generates
// "token_hash = ANY(ARRAY[$1,$2,...])" which is set-based (single UPDATE).
// No per-record processing or hook execution occurs.
func (s *AuthService) auditRevoke(ctx context.Context, tenantID, userID uuid.UUID, hashes []string) {
	var affected int64
	if err := s.Repos.Sessions.WithTx(ctx, func(txCtx context.Context) error {
		n, err := s.Repos.Sessions.BulkUpdate(
			txCtx,
			filter.And(
				filter.InStrings("token_hash", hashes),
				filter.IsNull("revoked_at"),
			),
			driver.Patch{Set: map[string]any{
				"revoked_at": time.Now().UTC(),
			}},
		)
		affected = n
		return err
	}); err != nil {
		slog.WarnContext(ctx, "iam: audit revoke: session BulkUpdate failed (best-effort)",
			"tenant_id", tenantID,
			"user_id", userID,
			"hashes", len(hashes),
			"err", err,
		)
	}

	// Write the unified audit record.
	s.writeAuthAudit(ctx, tenantID,
		&def.Actor{UserID: userID, TenantID: tenantID},
		audit.OperationSystem,
		"",
		map[string]any{
			"reason":        "admin_revoke_all",
			"session_count": len(hashes),
			"revoked":       affected,
		},
	)
}

// ── ValidateToken ─────────────────────────────────────────────────────────────

// ValidateToken looks up the session token in Redis, deserializes the session,
// and validates the expiry timestamp. Called on every authenticated request by
// the RequireAuth middleware.
//
// Redis failure semantics:
//   - goredis.Nil (key not found or expired) → 401 Unauthorized. The session
//     does not exist; the client must re-authenticate.
//   - Any other Redis error → 503 Service Unavailable. This is an infrastructure
//     failure. Masking it as 401 would hide outages behind spurious auth errors.
//
// The Redis TTL provides primary expiry enforcement. The IsExpired wall-clock
// check is a defense-in-depth measure in case of clock skew or TTL misconfiguration.
func (s *AuthService) ValidateToken(ctx context.Context, token string) (*auth.Session, error) {
	session, err := s.Sessions.Load(ctx, token)
	if err != nil {
		if errors.Is(err, auth.ErrSessionNotFound) {
			// Redis miss: key absent, TTL-expired, or revoked.
			// Attempt PostgreSQL recovery — handles Redis eviction/restart without
			// forcing users to re-authenticate.
			recovered, recErr := s.recoverSessionFromDB(ctx, token)
			if recErr != nil {
				// Not in DB either — session genuinely gone.
				return nil, &runtime.BusinessError{
					Code:    "iam.session.not_found",
					Message: "Session not found or expired.",
					Status:  401,
				}
			}
			session = recovered
		} else {
			// Infrastructure failure — store unavailable. Return 503 so the client
			// knows to retry rather than re-authenticate unnecessarily.
			return nil, &runtime.BusinessError{
				Code:    "iam.service_unavailable",
				Message: "Authentication service temporarily unavailable.",
				Status:  503,
			}
		}
	}

	// Defense-in-depth: check wall-clock expiry even though store TTL should
	// have already evicted the key. Protects against clock skew or TTL bugs.
	if session.IsExpired(time.Now()) {
		return nil, &runtime.BusinessError{
			Code:    "iam.session.expired",
			Message: "Session has expired.",
			Status:  401,
		}
	}

	return session, nil
}

// ── ValidateAPIToken ──────────────────────────────────────────────────────────

// ValidateAPIToken authenticates a service account API token and returns a
// synthetic session for the service account principal.
//
// API tokens are long-lived opaque strings. They are validated by SHA-256
// hashing the raw token and looking up the hash in iam_api_tokens. To avoid a
// database round-trip on every request, the validation result is cached in Redis
// for apiTokenCacheTTL (60 seconds). Cache invalidation on token revocation is
// the caller's responsibility (revoke the iam_api_token record and DEL the cache key).
//
// The returned Session carries:
//   - ServiceAccountID (not UserID) as the authenticated principal.
//   - Roles: ["role:api-client"] (the default service account role).
//   - ExpiresAt: from the iam_api_tokens.expires_at column.
//
// Redis failure semantics follow the same rules as ValidateToken:
//   - goredis.Nil on cache miss → fall through to DB lookup (normal path).
//   - Any other Redis error → 503.
//
// tenantID must be pre-resolved by the TenantResolver middleware before this
// method is called. It is used to establish RLS context for the DB lookup.
func (s *AuthService) ValidateAPIToken(ctx context.Context, rawToken string, tenantID uuid.UUID) (*auth.Session, error) {
	hash := tokenHash(rawToken)
	cacheKey := "iam:api_token:" + hash

	// Fast path: check the token cache (60-second TTL).
	var cached auth.Session
	err := s.Cache.Get(ctx, cacheKey, &cached)
	if err == nil {
		// Cache hit — validate expiry.
		if cached.IsExpired(time.Now()) {
			// Token expired since it was cached. Evict the stale entry.
			_ = s.Cache.Delete(ctx, cacheKey)
			return nil, &runtime.BusinessError{
				Code:    "iam.api_token.expired",
				Message: "API token has expired.",
				Status:  401,
			}
		}
		return &cached, nil
	}
	if !errors.Is(err, cache.ErrMiss) {
		// Infrastructure failure — cache unavailable.
		return nil, &runtime.BusinessError{
			Code:    "iam.service_unavailable",
			Message: "Authentication service temporarily unavailable.",
			Status:  503,
		}
	}

	// Cache miss — look up the token in the database.
	// A transaction is required to establish tenant RLS context.
	session, err := s.lookupAPIToken(ctx, hash, tenantID)
	if err != nil {
		return nil, err
	}

	// Populate cache for subsequent requests. TTL is the minimum of
	// apiTokenCacheTTL and the remaining token lifetime.
	ttl := apiTokenCacheTTL
	if remaining := session.TTL(time.Now()); remaining < ttl {
		ttl = remaining
	}
	if ttl > 0 {
		_ = s.Cache.Set(ctx, cacheKey, session, ttl)
	}

	return session, nil
}

// lookupAPIToken queries iam_api_tokens and iam_service_accounts for the given
// token hash within the tenant. Returns a synthetic auth.Session on success.
// Runs inside a transaction to ensure the tenant RLS context is active.
//
// This join query is intentionally raw SQL (not EntityRepository) because it
// spans two entity types. EntityRepository is scoped to a single entity;
// cross-entity joins are reserved for N+6D DatabaseQuerier.
func (s *AuthService) lookupAPIToken(ctx context.Context, hash string, tenantID uuid.UUID) (*auth.Session, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("iam: lookup api token: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, sqlSetTenantContext, tenantID); err != nil {
		return nil, fmt.Errorf("iam: lookup api token: set tenant context: %w", err)
	}

	var (
		serviceAccountID uuid.UUID
		expiresAt        *time.Time // nullable
		isRevoked        bool
		saStatus         string
	)
	err = tx.QueryRow(ctx, sqlLookupAPIToken, hash).Scan(&serviceAccountID, &expiresAt, &isRevoked, &saStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &runtime.BusinessError{
				Code:    "iam.api_token.not_found",
				Message: "Invalid API token.",
				Status:  401,
			}
		}
		return nil, fmt.Errorf("iam: lookup api token: query: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("iam: lookup api token: commit: %w", err)
	}

	if isRevoked {
		return nil, &runtime.BusinessError{
			Code:    "iam.api_token.revoked",
			Message: "API token has been revoked.",
			Status:  401,
		}
	}

	now := time.Now()
	if expiresAt != nil && expiresAt.Before(now) {
		return nil, &runtime.BusinessError{
			Code:    "iam.api_token.expired",
			Message: "API token has expired.",
			Status:  401,
		}
	}

	if saStatus != "active" {
		return nil, &runtime.BusinessError{
			Code:    "iam.service_account." + saStatus,
			Message: "Service account is " + saStatus + ".",
			Status:  403,
		}
	}

	// Build a synthetic session for the service account.
	// Service accounts use ServiceAccountID; UserID is uuid.Nil.
	// All service accounts carry role:api-client as the default role.
	// Role management for service accounts is a future enhancement.
	var tokenExpiry time.Time
	if expiresAt != nil {
		tokenExpiry = *expiresAt
	} else {
		// Non-expiring token — set a far-future expiry for the session struct.
		tokenExpiry = now.Add(365 * 24 * time.Hour)
	}

	session := &auth.Session{
		Token:            "", // not stored in Redis under session:{token}
		UserID:           uuid.Nil,
		ServiceAccountID: serviceAccountID,
		TenantID:         tenantID,
		Roles:            []string{serviceAccountDefaultRole},
		ExpiresAt:        tokenExpiry,
		IssuedAt:         now,
	}

	return session, nil
}

// ── LoadRolePermissions ───────────────────────────────────────────────────────

// LoadRolePermissions returns all role-to-permission bindings from the global
// iam_role_permissions table. Called at startup to initialize
// [auth.CasbinEvaluator] Phase 2, and after any role-permission change to
// trigger a policy reload.
//
// iam_role_permissions is a global table (no tenant_id, no RLS). This method
// queries it without setting tenant context.
//
// This query remains raw SQL (not EntityRepository) because iam_role_permissions
// is a global bootstrap table with no tenant scope and is not registered as an
// EntityDefinition. Migrating it would require registering a non-tenant entity,
// which is a separate framework capability reserved for N+6D.
func (s *AuthService) LoadRolePermissions(ctx context.Context) ([]auth.RolePermission, error) {
	rows, err := s.DB.Query(ctx, sqlLoadRolePermissions)
	if err != nil {
		return nil, fmt.Errorf("iam: load role permissions: %w", err)
	}
	defer rows.Close()

	var result []auth.RolePermission
	for rows.Next() {
		var rp auth.RolePermission
		if err := rows.Scan(&rp.Role, &rp.Permission); err != nil {
			return nil, fmt.Errorf("iam: load role permissions: scan: %w", err)
		}
		result = append(result, rp)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iam: load role permissions: rows: %w", err)
	}
	return result, nil
}

// ── Audit helpers ──────────────────────────────────────────────────────────────

// writeAuthAudit emits an AUTH-category audit record to the unified pipeline.
// Best-effort — failures are silently ignored; session state is already committed.
//
// op should be OperationLogin, OperationLogout, or OperationSystem.
// actor carries the authenticated principal; context carries event-specific metadata.
func (s *AuthService) writeAuthAudit(ctx context.Context, tenantID uuid.UUID, actor *def.Actor, op audit.OperationType, ipAddress string, extra map[string]any) {
	if s.AuditWriter == nil {
		return
	}
	rec := audit.AuditRecord{
		TenantID:      tenantID,
		EntityName:    "iam_session",
		Operation:     op,
		EventCategory: audit.CategoryAuth,
		Actor:         actor,
		IPAddress:     ipAddress,
		Context:       extra,
	}
	_ = s.AuditWriter.Write(ctx, rec)
}

// ── Session recovery ──────────────────────────────────────────────────────────

// recoverSessionFromDB queries iam_sessions by token_hash to reconstruct a
// session that has been evicted from Redis. This handles Redis restarts or
// eviction without forcing users to re-authenticate.
//
// Recovery steps:
//  1. Extract TenantContext from ctx (injected by TenantResolver before
//     RequireAuth runs). Returns error immediately if absent.
//  2. Open a transaction and call set_tenant_context so RLS scopes the lookup
//     to the correct tenant.
//  3. Query iam_sessions for a non-revoked, non-expired session.
//  4. Reload roles for human users via loadUserRoles.
//  5. Best-effort: restore the session to Redis for subsequent requests.
//
// recoverSessionFromDB returns a non-nil error when the session is not present
// in the database. ValidateToken maps this to a 401.
func (s *AuthService) recoverSessionFromDB(ctx context.Context, token string) (*auth.Session, error) {
	tc, ok := tenant.TryFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("iam: session recovery: no tenant context")
	}

	hash := tokenHash(token)

	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("iam: session recovery: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, sqlSetTenantContext, tc.TenantID); err != nil {
		return nil, fmt.Errorf("iam: session recovery: set tenant context: %w", err)
	}

	var (
		userID           uuid.UUID
		serviceAccountID uuid.UUID
		issuedAt         time.Time
		expiresAt        time.Time
		deviceID         string
		ipAddress        string
		tenantID         uuid.UUID
	)
	err = tx.QueryRow(ctx, sqlLoadSessionByHash, hash).Scan(
		&userID, &serviceAccountID, &issuedAt, &expiresAt, &deviceID, &ipAddress, &tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("iam: session recovery: not found in db: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("iam: session recovery: commit: %w", err)
	}

	// Reload roles for human user sessions.
	var roles []string
	if userID != uuid.Nil {
		roles, err = s.loadUserRoles(ctx, tenantID, userID)
		if err != nil {
			// Role reload failure is non-fatal: issue session with empty roles.
			// The user will be re-authenticated on next Casbin check failure.
			slog.WarnContext(ctx, "iam: session recovery: role reload failed; using empty roles",
				"user_id", userID, "tenant_id", tenantID, "err", err)
			roles = nil
		}
	} else {
		roles = []string{serviceAccountDefaultRole}
	}

	session := &auth.Session{
		Token:            token,
		UserID:           userID,
		ServiceAccountID: serviceAccountID,
		TenantID:         tenantID,
		Roles:            roles,
		ExpiresAt:        expiresAt,
		IssuedAt:         issuedAt,
		DeviceID:         deviceID,
		IPAddress:        ipAddress,
	}

	// Best-effort: restore session to Redis for subsequent requests.
	// Failure here is non-fatal — the recovered session is returned regardless.
	if storeErr := s.Sessions.Store(ctx, session); storeErr != nil {
		slog.WarnContext(ctx, "iam: session recovery: redis restore failed (best-effort)",
			"user_id", userID, "tenant_id", tenantID, "err", storeErr)
	}

	return session, nil
}

// ── Internal helpers ───────────────────────────────────────────────────────────

// loadUserRoles queries iam_user_roles for all role names assigned to the user
// within the tenant. Uses EntityRepository.WithTx to establish tenant RLS context
// automatically from TenantContext already in ctx (set by TenantResolver middleware).
//
// Called after the credential transaction commits (not inside it). See Login for
// the rationale on keeping role loading separate from credential verification.
func (s *AuthService) loadUserRoles(ctx context.Context, tenantID, userID uuid.UUID) ([]string, error) {
	var roles []string
	err := s.Repos.UserRoles.WithTx(ctx, func(txCtx context.Context) error {
		records, _, err := s.Repos.UserRoles.Query(
			txCtx,
			filter.And(
				filter.Eq("tenant_id", tenantID),
				filter.Eq("user_id", userID),
			),
			driver.WithSkipCount(),
		)
		if err != nil {
			return fmt.Errorf("query: %w", err)
		}
		roles = make([]string, 0, len(records))
		for _, rec := range records {
			if name := roleNameFromRecord(rec); name != "" {
				roles = append(roles, name)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("iam: load user roles: %w", err)
	}
	return roles, nil
}

// roleNameFromRecord extracts the role_name string from an iam_user_role
// EntityRecord. Centralises field extraction to avoid scattering raw field
// name strings throughout the role-loading logic.
func roleNameFromRecord(r *def.EntityRecord) string {
	return r.GetString("role_name")
}

// storeSession persists the session via [SessionStore.Store].
// Called on the critical login path — failure aborts login.
func (s *AuthService) storeSession(ctx context.Context, session *auth.Session) error {
	if err := s.Sessions.Store(ctx, session); err != nil {
		return fmt.Errorf("iam: store session: %w", err)
	}
	return nil
}

// writeFailedLoginAudit writes a failed_login audit record to the unified pipeline.
// Best-effort — failures are silently ignored; the login error is already returned to
// the caller. platform_audit_log has no RLS so no TX or tenant context is required.
func (s *AuthService) writeFailedLoginAudit(ctx context.Context, input LoginInput, userID uuid.UUID, reason string) {
	s.writeAuthAudit(ctx, input.TenantID,
		&def.Actor{UserID: userID, TenantID: input.TenantID},
		audit.OperationLogin,
		input.IPAddress,
		map[string]any{"device_id": input.DeviceID, "failure_reason": reason, "failed": true},
	)
}

// ── Helpers ────────────────────────────────────────────────────────────────────

// tokenHash is declared in hooks.go (shared across the package).

// nullUUID returns nil for uuid.Nil, otherwise returns the UUID value.
// Used to store nullable UUID columns correctly in pgx (uuid.Nil → NULL).
func nullUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}
