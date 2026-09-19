package iam_test

// Real HTTP-route-level tests for POST /auth/logout and GET /auth/me.
//
// AuthService-level tests (service_integration_test.go, service_test.go)
// exercise Logout/ValidateToken directly and cannot catch a wiring bug where
// the Fiber route itself never runs the middleware that populates
// c.Locals("session") — both handlers silently return 401 for every caller
// regardless of credentials in that case. Only a test that goes through the
// actual fiber.App and iam.Module.RegisterRoutes wiring, the same way a real
// request does, can catch that class of bug.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/api/middleware"
	"awo.so/awo/audit"
	"awo.so/awo/cache"
	contribpgx "awo.so/awo/contrib/pgx"
	contribRedis "awo.so/awo/contrib/redis"
	"awo.so/awo/platform/iam"
	testdb "awo.so/awo/testutil/db"
)

// setupIAMApp builds a real fiber.App with the IAM module's actual routes
// registered, exactly as cmd/server/main.go / cmd/awo/serve_impl.go wire it
// (WithAuthMiddleware(middleware.RequireAuth(...)) before RegisterRoutes) —
// not a hand-built stand-in for that wiring.
func setupIAMApp(t *testing.T) (*fiber.App, *iam.AuthService, uuid.UUID) {
	t.Helper()

	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, iamUsersDDL)
	testdb.ApplySQL(t, pool, iamUserRolesDDL)
	testdb.ApplySQL(t, pool, iamSessionsDDL)

	tenantID := testdb.RawTenantID()
	testdb.ActivateTenant(t, pool, tenantID)

	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	store := contribRedis.NewSessionStore(rdb)

	repos := iam.IAMRepositories{
		Sessions:  contribpgx.NewRepository(pool, iamSessionsSchema()),
		Users:     contribpgx.NewRepository(pool, iamUsersSchema()),
		UserRoles: contribpgx.NewRepository(pool, iamUserRolesSchema()),
	}

	m := iam.New(pool, store, cache.NoopCache{}, repos).WithAuditWriter(audit.NoopAuditWriter{})
	m = m.WithAuthMiddleware(middleware.RequireAuth(m.Auth, ""))

	app := fiber.New()
	m.RegisterRoutes(app)

	return app, m.Auth, tenantID
}

func createIAMTestUser(t *testing.T, svc *iam.AuthService, tenantID uuid.UUID, email, password string) uuid.UUID {
	t.Helper()
	userID := uuid.New()
	pwHash := hashTestPassword(t, password)
	testdb.ApplySQL(t, svc.DB, fmt.Sprintf(
		`INSERT INTO iam_users (id, tenant_id, email, password_hash, status)
		 VALUES ('%s', '%s', '%s', '%s', 'active')`,
		userID, tenantID, email, pwHash,
	))
	return userID
}

func doJSON(t *testing.T, app *fiber.App, method, path, body, bearer string) (*http.Response, map[string]any) {
	t.Helper()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()
	var parsed map[string]any
	if len(raw) > 0 {
		require.NoError(t, json.Unmarshal(raw, &parsed))
	}
	return resp, parsed
}

// TestLogoutRoute_ValidToken_Returns200_AndActuallyRevokesSession is the
// regression test for the bug where /auth/logout and /auth/me always
// returned 401: registerRoutes previously never applied any session-
// validating middleware ahead of these two routes, so their handlers'
// c.Locals("session") lookup always missed regardless of the caller's
// credentials.
func TestLogoutRoute_ValidToken_Returns200_AndActuallyRevokesSession(t *testing.T) {
	app, svc, tenantID := setupIAMApp(t)
	createIAMTestUser(t, svc, tenantID, "route-logout@example.com", "correct horse battery staple")

	loginBody := fmt.Sprintf(`{"email":"route-logout@example.com","password":"correct horse battery staple","tenant_id":%q}`, tenantID.String())
	resp, parsed := doJSON(t, app, http.MethodPost, "/api/v1/auth/login", loginBody, "")
	require.Equal(t, http.StatusOK, resp.StatusCode, "login must succeed to obtain a token: %+v", parsed)
	data := parsed["data"].(map[string]any)
	token := data["token"].(string)
	require.NotEmpty(t, token)

	resp, parsed = doJSON(t, app, http.MethodPost, "/api/v1/auth/logout", "", token)
	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"POST /auth/logout with a valid bearer token must return 200, not 401 — got body: %+v", parsed)

	// The session must actually be gone afterward, not just report success.
	_, err := svc.ValidateToken(context.Background(), token)
	assert.Error(t, err, "the token must be rejected after logout — logout must not be a no-op that still returns 200")
}

// TestMeRoute_ValidToken_ReturnsIdentity is the companion regression test for
// GET /auth/me — same missing-middleware bug, opposite (read) route.
func TestMeRoute_ValidToken_ReturnsIdentity(t *testing.T) {
	app, svc, tenantID := setupIAMApp(t)
	userID := createIAMTestUser(t, svc, tenantID, "route-me@example.com", "correct horse battery staple")

	loginBody := fmt.Sprintf(`{"email":"route-me@example.com","password":"correct horse battery staple","tenant_id":%q}`, tenantID.String())
	resp, parsed := doJSON(t, app, http.MethodPost, "/api/v1/auth/login", loginBody, "")
	require.Equal(t, http.StatusOK, resp.StatusCode, "login must succeed to obtain a token: %+v", parsed)
	token := parsed["data"].(map[string]any)["token"].(string)

	resp, parsed = doJSON(t, app, http.MethodGet, "/api/v1/auth/me", "", token)
	require.Equal(t, http.StatusOK, resp.StatusCode,
		"GET /auth/me with a valid bearer token must return 200, not 401 — got body: %+v", parsed)
	meData := parsed["data"].(map[string]any)
	assert.Equal(t, userID.String(), meData["user_id"])
	assert.Equal(t, tenantID.String(), meData["tenant_id"])
}

// TestMeRoute_NoToken_Returns401 proves the route is not accidentally
// wide-open now that a real middleware is mounted ahead of it.
func TestMeRoute_NoToken_Returns401(t *testing.T) {
	app, _, _ := setupIAMApp(t)
	resp, _ := doJSON(t, app, http.MethodGet, "/api/v1/auth/me", "", "")
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
