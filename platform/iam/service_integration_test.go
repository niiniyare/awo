package iam_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"awo.so/awo/audit"
	"awo.so/awo/auth"
	"awo.so/awo/cache"
	"awo.so/awo/compiler"
	contribpgx "awo.so/awo/contrib/pgx"
	contribRedis "awo.so/awo/contrib/redis"
	"awo.so/awo/def"
	"awo.so/awo/platform/iam"
	testdb "awo.so/awo/testutil/db"
)

// ── DDL ────────────────────────────────────────────────────────────────────────

const iamUsersDDL = `
CREATE TABLE iam_users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     uuid NOT NULL,
    email         text NOT NULL,
    password_hash text NOT NULL,
    status        text NOT NULL DEFAULT 'active',
    last_login_at timestamptz,
    custom_fields jsonb,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE iam_users ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam_users FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON iam_users
    USING (tenant_id = current_tenant_id());
GRANT SELECT, INSERT, UPDATE, DELETE ON iam_users TO awo_app;
`

const iamUserRolesDDL = `
CREATE TABLE iam_user_roles (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     uuid NOT NULL,
    user_id       uuid NOT NULL,
    role_name     text NOT NULL,
    custom_fields jsonb,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE iam_user_roles ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam_user_roles FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON iam_user_roles
    USING (tenant_id = current_tenant_id());
GRANT SELECT, INSERT, UPDATE, DELETE ON iam_user_roles TO awo_app;
`

const iamSessionsDDL = `
CREATE TABLE iam_sessions (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id          uuid NOT NULL,
    token_hash         text NOT NULL,
    user_id            uuid,
    service_account_id uuid,
    issued_at          timestamptz NOT NULL,
    expires_at         timestamptz NOT NULL,
    device_id          text,
    ip_address         text,
    revoked_at         timestamptz,
    custom_fields      jsonb,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE iam_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam_sessions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON iam_sessions
    USING (tenant_id = current_tenant_id());
GRANT SELECT, INSERT, UPDATE, DELETE ON iam_sessions TO awo_app;
`

// ── EntitySchema helpers ──────────────────────────────────────────────────────

func iamUsersSchema() *compiler.EntitySchema {
	return &compiler.EntitySchema{
		QualifiedName: "iam_users",
		TableName:     "iam_users",
		IsSystem:      true,
		FieldsByName: map[string]def.FieldDef{
			"email":         {Name: "email", Type: def.FieldTypeData},
			"password_hash": {Name: "password_hash", Type: def.FieldTypeData, Sensitive: true},
			"status":        {Name: "status", Type: def.FieldTypeData},
			"last_login_at": {Name: "last_login_at", Type: def.FieldTypeDateTime},
		},
	}
}

func iamUserRolesSchema() *compiler.EntitySchema {
	return &compiler.EntitySchema{
		QualifiedName: "iam_user_roles",
		TableName:     "iam_user_roles",
		IsSystem:      true,
		FieldsByName: map[string]def.FieldDef{
			"user_id":   {Name: "user_id", Type: def.FieldTypeLink},
			"role_name": {Name: "role_name", Type: def.FieldTypeData},
		},
	}
}

func iamSessionsSchema() *compiler.EntitySchema {
	return &compiler.EntitySchema{
		QualifiedName: "iam_sessions",
		TableName:     "iam_sessions",
		IsSystem:      true,
		FieldsByName: map[string]def.FieldDef{
			"token_hash":         {Name: "token_hash", Type: def.FieldTypeData},
			"user_id":            {Name: "user_id", Type: def.FieldTypeLink},
			"service_account_id": {Name: "service_account_id", Type: def.FieldTypeLink},
			"issued_at":          {Name: "issued_at", Type: def.FieldTypeDateTime},
			"expires_at":         {Name: "expires_at", Type: def.FieldTypeDateTime},
			"device_id":          {Name: "device_id", Type: def.FieldTypeData},
			"ip_address":         {Name: "ip_address", Type: def.FieldTypeData},
			"revoked_at":         {Name: "revoked_at", Type: def.FieldTypeDateTime},
		},
	}
}

// ── Setup helpers ─────────────────────────────────────────────────────────────

// setupIAM creates an isolated PG schema, applies all IAM DDL, starts miniredis,
// and returns a fully wired AuthService plus a tenant-activated context.
func setupIAM(t *testing.T) (*iam.AuthService, context.Context, uuid.UUID) {
	t.Helper()

	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, iamUsersDDL)
	testdb.ApplySQL(t, pool, iamUserRolesDDL)
	testdb.ApplySQL(t, pool, iamSessionsDDL)

	tenantID := testdb.RawTenantID()
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := testdb.WithTenant(context.Background(), tenantID)

	// Miniredis for session store.
	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	store := contribRedis.NewSessionStore(rdb)

	svc := &iam.AuthService{
		DB:       pool,
		Sessions: store,
		Cache:    cache.NoopCache{},
		Repos: iam.IAMRepositories{
			Sessions:  contribpgx.NewRepository(pool, iamSessionsSchema()),
			Users:     contribpgx.NewRepository(pool, iamUsersSchema()),
			UserRoles: contribpgx.NewRepository(pool, iamUserRolesSchema()),
		},
		AuditWriter: audit.NoopAuditWriter{},
	}

	return svc, ctx, tenantID
}

// setupIAMWithTenantLifecycle is setupIAM, but installs the real
// set_tenant_context (see testutil/db.InstallTenantLifecycle) instead of the
// simple pass-through, and creates a real platform_tenant row with the given
// status instead of an arbitrary unbacked UUID. Use this for tests that
// assert on tenant-lifecycle enforcement; use setupIAM for tests that only
// care about tenant-ID row isolation.
func setupIAMWithTenantLifecycle(t *testing.T, tenantStatus string) (*iam.AuthService, context.Context, uuid.UUID) {
	t.Helper()

	pool := testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, pool)
	testdb.ApplySQL(t, pool, iamUsersDDL)
	testdb.ApplySQL(t, pool, iamUserRolesDDL)
	testdb.ApplySQL(t, pool, iamSessionsDDL)

	tenantID := testdb.CreateTenant(t, pool, tenantStatus)

	var ctx context.Context
	if tenantStatus == "ACTIVE" {
		testdb.ActivateTenant(t, pool, tenantID)
		ctx = testdb.WithTenant(context.Background(), tenantID)
	} else {
		// Non-ACTIVE tenants can't establish RLS context at all (that's the
		// property under test) — ctx carries the tenant ID for the caller's
		// own bookkeeping only; AuthService.Login re-derives context itself
		// via its own set_tenant_context call, so this ctx is never used to
		// bypass that check.
		ctx = context.Background()
	}

	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	store := contribRedis.NewSessionStore(rdb)

	svc := &iam.AuthService{
		DB:       pool,
		Sessions: store,
		Cache:    cache.NoopCache{},
		Repos: iam.IAMRepositories{
			Sessions:  contribpgx.NewRepository(pool, iamSessionsSchema()),
			Users:     contribpgx.NewRepository(pool, iamUsersSchema()),
			UserRoles: contribpgx.NewRepository(pool, iamUserRolesSchema()),
		},
		AuditWriter: audit.NoopAuditWriter{},
	}

	return svc, ctx, tenantID
}

// hashTestPassword returns a bcrypt hash at MinCost for use in tests.
// Using MinCost keeps tests fast (~1ms vs ~200ms at cost 12).
func hashTestPassword(t *testing.T, plain string) string {
	t.Helper()
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.MinCost)
	require.NoError(t, err)
	return string(b)
}

// sha256hex mirrors the tokenHash helper in the iam package.
func sha256hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum)
}

// ── Login tests ───────────────────────────────────────────────────────────────

func TestAuthService_Login_Success_StoresSessionInRedis(t *testing.T) {
	svc, ctx, tenantID := setupIAM(t)

	// Insert a user directly via SQL (superuser path avoids hook).
	userID := uuid.New()
	pwHash := hashTestPassword(t, "hunter2!")
	testdb.ApplySQL(t, svc.DB, fmt.Sprintf(
		`INSERT INTO iam_users (id, tenant_id, email, password_hash, status)
		 VALUES ('%s', '%s', 'alice@example.com', '%s', 'active')`,
		userID, tenantID, pwHash,
	))

	result, err := svc.Login(ctx, iam.LoginInput{
		Email:    "alice@example.com",
		Password: "hunter2!",
		TenantID: tenantID,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Token, "token must be non-empty")

	// Session must be recoverable from the store (Redis).
	session, err := svc.ValidateToken(ctx, result.Token)
	require.NoError(t, err)
	assert.Equal(t, tenantID, session.TenantID)
	assert.Equal(t, userID, session.UserID)
	assert.False(t, session.IsExpired(time.Now()), "session must not be expired")
}

func TestAuthService_Login_WrongPassword_Returns401(t *testing.T) {
	svc, ctx, tenantID := setupIAM(t)

	userID := uuid.New()
	pwHash := hashTestPassword(t, "correctpassword")
	testdb.ApplySQL(t, svc.DB, fmt.Sprintf(
		`INSERT INTO iam_users (id, tenant_id, email, password_hash, status)
		 VALUES ('%s', '%s', 'bob@example.com', '%s', 'active')`,
		userID, tenantID, pwHash,
	))

	_, err := svc.Login(ctx, iam.LoginInput{
		Email:    "bob@example.com",
		Password: "wrongpassword",
		TenantID: tenantID,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "iam.login.invalid_credentials")
}

func TestAuthService_Login_UnknownUser_Returns401(t *testing.T) {
	svc, ctx, tenantID := setupIAM(t)

	_, err := svc.Login(ctx, iam.LoginInput{
		Email:    "nobody@example.com",
		Password: "doesntmatter",
		TenantID: tenantID,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "iam.login.invalid_credentials")
}

func TestAuthService_Login_InactiveUser_Returns403(t *testing.T) {
	svc, ctx, tenantID := setupIAM(t)

	userID := uuid.New()
	pwHash := hashTestPassword(t, "password123")
	testdb.ApplySQL(t, svc.DB, fmt.Sprintf(
		`INSERT INTO iam_users (id, tenant_id, email, password_hash, status)
		 VALUES ('%s', '%s', 'carol@example.com', '%s', 'suspended')`,
		userID, tenantID, pwHash,
	))

	_, err := svc.Login(ctx, iam.LoginInput{
		Email:    "carol@example.com",
		Password: "password123",
		TenantID: tenantID,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "suspended")
}

// ── Tenant lifecycle tests ────────────────────────────────────────────────────
//
// POST /api/v1/auth/login is the one authenticated route NOT covered by
// TenantResolver middleware (see api/router/router.go — login is registered
// directly on the bare app, outside the /api/v1 group). Without a check at
// some other layer, this would let a suspended/archived tenant's users still
// authenticate and receive a live session. These tests prove Login itself
// rejects every non-ACTIVE status, because set_tenant_context() enforces it
// at the database layer regardless of which application code path calls it.

func TestAuthService_Login_SuspendedTenant_Rejected(t *testing.T) {
	svc, _, tenantID := setupIAMWithTenantLifecycle(t, "SUSPENDED")

	_, err := svc.Login(context.Background(), iam.LoginInput{
		Email:    "anyone@example.com",
		Password: "irrelevant",
		TenantID: tenantID,
	})
	require.Error(t, err, "login for a SUSPENDED tenant must be rejected — "+
		"the login route sits outside the tenant-status HTTP middleware, so this "+
		"guarantee has to come from the database layer or it doesn't exist at all")
	assert.Contains(t, err.Error(), "tenant")
}

func TestAuthService_Login_ArchivedTenant_Rejected(t *testing.T) {
	svc, _, tenantID := setupIAMWithTenantLifecycle(t, "ARCHIVED")

	_, err := svc.Login(context.Background(), iam.LoginInput{
		Email:    "anyone@example.com",
		Password: "irrelevant",
		TenantID: tenantID,
	})
	require.Error(t, err, "login for an ARCHIVED tenant must be rejected")
	assert.Contains(t, err.Error(), "tenant")
}

func TestAuthService_Login_PendingTenant_Rejected(t *testing.T) {
	svc, _, tenantID := setupIAMWithTenantLifecycle(t, "PENDING")

	_, err := svc.Login(context.Background(), iam.LoginInput{
		Email:    "anyone@example.com",
		Password: "irrelevant",
		TenantID: tenantID,
	})
	require.Error(t, err, "login for a PENDING tenant must be rejected")
	assert.Contains(t, err.Error(), "tenant")
}

func TestAuthService_Login_NonexistentTenant_Rejected(t *testing.T) {
	svc, _, _ := setupIAMWithTenantLifecycle(t, "ACTIVE") // schema/pool setup only

	_, err := svc.Login(context.Background(), iam.LoginInput{
		Email:    "anyone@example.com",
		Password: "irrelevant",
		TenantID: testdb.RawTenantID(), // no backing platform_tenant row
	})
	require.Error(t, err, "login against a nonexistent tenant ID must be rejected")
	assert.Contains(t, err.Error(), "tenant")
}

// TestAuthService_Login_ActiveTenant_ValidCredentials_Succeeds is also the
// regression test for a second bug found while writing it: Login is called
// with a plain context.Background() by the real HTTP handler (its route is
// registered outside the TenantResolver-gated group, which is the only
// thing that normally embeds a tenant.TenantContext into the request
// context). Every downstream call in this test — loadUserRoles, storeSession,
// auditLogin — must work correctly from that same bare context, exactly as
// production does, not from a context a test helper pre-seeded with tenant
// info. Before the fix, loadUserRoles silently ran with RLS inactive
// (returning zero roles instead of erroring), and auditLogin's session-record
// write panicked outright.
func TestAuthService_Login_ActiveTenant_ValidCredentials_Succeeds(t *testing.T) {
	svc, _, tenantID := setupIAMWithTenantLifecycle(t, "ACTIVE")

	userID := uuid.New()
	pwHash := hashTestPassword(t, "correct horse battery staple")
	testdb.ApplySQL(t, svc.DB, fmt.Sprintf(
		`INSERT INTO iam_users (id, tenant_id, email, password_hash, status)
		 VALUES ('%s', '%s', 'dana@example.com', '%s', 'active')`,
		userID, tenantID, pwHash,
	))
	testdb.ApplySQL(t, svc.DB, fmt.Sprintf(
		`INSERT INTO iam_user_roles (id, tenant_id, user_id, role_name)
		 VALUES (gen_random_uuid(), '%s', '%s', 'role:finance.viewer')`,
		tenantID, userID,
	))

	result, err := svc.Login(context.Background(), iam.LoginInput{
		Email:    "dana@example.com",
		Password: "correct horse battery staple",
		TenantID: tenantID,
	})
	require.NoError(t, err, "login for an ACTIVE tenant with valid credentials must succeed — "+
		"the tenant-lifecycle fix must not break the legitimate path")
	assert.Equal(t, tenantID, result.Session.TenantID)
	assert.Equal(t, userID, result.Session.UserID)
	assert.Equal(t, []string{"role:finance.viewer"}, result.Session.Roles,
		"the role assigned to this user must actually be loaded — an RLS-inactive "+
			"role query would silently return zero roles instead of failing")

	// auditLogin's session-record write must have actually landed (it
	// previously panicked before reaching this point).
	var sessionCount int
	require.NoError(t, testdb.QueryRowSQL(t, svc.DB,
		`SELECT count(*) FROM iam_sessions WHERE token_hash = $1`, sha256hex(result.Token),
	).Scan(&sessionCount))
	assert.Equal(t, 1, sessionCount, "the best-effort SQL session record must be written, not skipped")
}

// ── Session fallback tests ────────────────────────────────────────────────────

func TestAuthService_ValidateToken_RedisMiss_RecoverFromDB(t *testing.T) {
	svc, ctx, tenantID := setupIAM(t)

	// Generate a raw token and compute its hash (mirrors tokenHash in iam package).
	rawToken, err := auth.GenerateToken()
	require.NoError(t, err)
	hash := sha256hex(rawToken)

	userID := uuid.New()
	now := time.Now().UTC()
	expiresAt := now.Add(24 * time.Hour)

	// Insert the session directly into iam_sessions as superuser (bypasses RLS).
	testdb.ApplySQL(t, svc.DB, fmt.Sprintf(
		`INSERT INTO iam_sessions
		    (id, tenant_id, token_hash, user_id, service_account_id,
		     issued_at, expires_at, device_id, ip_address)
		 VALUES (gen_random_uuid(), '%s', '%s', '%s', NULL,
		         '%s', '%s', 'test-device', '127.0.0.1')`,
		tenantID, hash, userID,
		now.Format(time.RFC3339Nano),
		expiresAt.Format(time.RFC3339Nano),
	))

	// Redis has no session (miniredis is empty) → Load returns ErrSessionNotFound.
	// ValidateToken must recover the session from PostgreSQL.
	session, err := svc.ValidateToken(ctx, rawToken)
	require.NoError(t, err, "session must be recovered from PostgreSQL when Redis is empty")

	assert.Equal(t, tenantID, session.TenantID)
	assert.Equal(t, userID, session.UserID)
	assert.Equal(t, rawToken, session.Token)
	assert.False(t, session.IsExpired(time.Now()), "recovered session must not be expired")
}

func TestAuthService_ValidateToken_ExpiredSession_Returns401(t *testing.T) {
	svc, ctx, tenantID := setupIAM(t)

	rawToken, err := auth.GenerateToken()
	require.NoError(t, err)
	hash := sha256hex(rawToken)

	userID := uuid.New()
	past := time.Now().UTC().Add(-48 * time.Hour)
	expiredAt := time.Now().UTC().Add(-1 * time.Hour)

	// Insert an already-expired session — sqlLoadSessionByHash has expires_at > NOW()
	// so this should NOT be returned by the DB query.
	testdb.ApplySQL(t, svc.DB, fmt.Sprintf(
		`INSERT INTO iam_sessions
		    (id, tenant_id, token_hash, user_id, service_account_id,
		     issued_at, expires_at, device_id, ip_address)
		 VALUES (gen_random_uuid(), '%s', '%s', '%s', NULL,
		         '%s', '%s', 'device', '127.0.0.1')`,
		tenantID, hash, userID,
		past.Format(time.RFC3339Nano),
		expiredAt.Format(time.RFC3339Nano),
	))

	// Redis miss + DB returns no rows (expired) → 401.
	_, err = svc.ValidateToken(ctx, rawToken)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "iam.session.not_found")
}

func TestAuthService_ValidateToken_RevokedSession_NotReturned(t *testing.T) {
	svc, ctx, tenantID := setupIAM(t)

	rawToken, err := auth.GenerateToken()
	require.NoError(t, err)
	hash := sha256hex(rawToken)

	userID := uuid.New()
	now := time.Now().UTC()

	// Insert a revoked session (revoked_at IS NOT NULL → excluded by query).
	testdb.ApplySQL(t, svc.DB, fmt.Sprintf(
		`INSERT INTO iam_sessions
		    (id, tenant_id, token_hash, user_id, service_account_id,
		     issued_at, expires_at, device_id, ip_address, revoked_at)
		 VALUES (gen_random_uuid(), '%s', '%s', '%s', NULL,
		         '%s', '%s', 'device', '127.0.0.1', '%s')`,
		tenantID, hash, userID,
		now.Format(time.RFC3339Nano),
		now.Add(24*time.Hour).Format(time.RFC3339Nano),
		now.Format(time.RFC3339Nano),
	))

	_, err = svc.ValidateToken(ctx, rawToken)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "iam.session.not_found")
}

// TestAuthService_ValidateToken_RevocationTombstone_BlocksResurrection is a
// deterministic regression test for a session-revocation race.
//
// Logout/RevokeUserSessions delete the live Redis key synchronously (the
// critical, error-propagating path) but mark iam_sessions.revoked_at only
// best-effort, in a separate write that can fail independently — a network
// blip, a connection pool exhaustion, anything. Before this fix,
// recoverSessionFromDB's query only checked "revoked_at IS NULL AND
// expires_at > NOW()", so if that best-effort write had failed (exactly the
// state this test constructs directly, without going through Logout, to
// isolate the race from the mechanism that triggers it), a Redis-miss
// lookup would silently resurrect the revoked session as valid.
//
// This test proves the fix holds even in that exact state: PostgreSQL still
// says "not revoked" (revoked_at IS NULL), but Sessions.Delete's tombstone
// write is what actually blocks it, independent of PostgreSQL's state. If
// the IsRevoked check in ValidateToken (or the tombstone write in
// RedisSessionStore.Delete) is ever removed or bypassed, this test fails.
func TestAuthService_ValidateToken_RevocationTombstone_BlocksResurrection(t *testing.T) {
	svc, ctx, tenantID := setupIAM(t)

	rawToken, err := auth.GenerateToken()
	require.NoError(t, err)
	hash := sha256hex(rawToken)

	userID := uuid.New()
	now := time.Now().UTC()
	expiresAt := now.Add(24 * time.Hour)

	// Simulate the state after a real login: a live session exists in
	// PostgreSQL with revoked_at IS NULL (the "still valid" state that
	// recoverSessionFromDB's query would match).
	testdb.ApplySQL(t, svc.DB, fmt.Sprintf(
		`INSERT INTO iam_sessions
		    (id, tenant_id, token_hash, user_id, service_account_id,
		     issued_at, expires_at, device_id, ip_address)
		 VALUES (gen_random_uuid(), '%s', '%s', '%s', NULL,
		         '%s', '%s', 'device', '127.0.0.1')`,
		tenantID, hash, userID,
		now.Format(time.RFC3339Nano),
		expiresAt.Format(time.RFC3339Nano),
	))

	// Simulate exactly the failure mode this test targets: the session was
	// logged out (Sessions.Delete ran and succeeded — the live Redis key is
	// gone and the tombstone is written), but the best-effort PostgreSQL
	// revocation write never landed (revoked_at is still NULL, as inserted
	// above — we deliberately do NOT update it, standing in for a failed
	// auditLogout BulkUpdate). Call Delete directly (not Logout) so this
	// test isolates the tombstone mechanism from Logout's own orchestration,
	// which is covered separately by the logout/me route-level test.
	session := &auth.Session{
		Token:            rawToken,
		UserID:           userID,
		ServiceAccountID: uuid.Nil,
		TenantID:         tenantID,
		ExpiresAt:        expiresAt,
		IssuedAt:         now,
	}
	// Store it in Redis first so Delete has something to remove — mirrors a
	// real session's lifecycle (Store at login, Delete at logout).
	require.NoError(t, svc.Sessions.Store(ctx, session))
	require.NoError(t, svc.Sessions.Delete(ctx, session))

	// PostgreSQL still says this session is valid (revoked_at IS NULL,
	// expires_at in the future) — confirm that precondition directly.
	var revokedAt *time.Time
	require.NoError(t, testdb.QueryRowSQL(t, svc.DB,
		`SELECT revoked_at FROM iam_sessions WHERE token_hash = $1`, hash).Scan(&revokedAt))
	require.Nil(t, revokedAt, "precondition: PostgreSQL must still show this session as not-revoked")

	// Redis has no live key (Delete removed it) → ValidateToken falls
	// through to the Redis-miss branch, which must consult the tombstone
	// BEFORE trusting the still-valid-looking PostgreSQL row.
	_, err = svc.ValidateToken(ctx, rawToken)
	require.Error(t, err, "a token revoked via Delete must never validate again, "+
		"even when PostgreSQL's own revocation record is missing")
	assert.Contains(t, err.Error(), "iam.session.not_found")
}
