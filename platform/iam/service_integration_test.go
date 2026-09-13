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
