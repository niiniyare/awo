// Package iam is the Awo Framework Identity and Access Management subsystem.
//
// IAM is a first-class framework package. It has zero dependency on any
// ERP-specific module, business entity, workflow, or application code. It is
// designed to be portable to any application built on the Awo Framework.
//
// # Entity model
//
// IAM manages the following tenant-scoped entities:
//
//   - iam_user: authenticated human principal (one per tenant account)
//   - iam_user_role: user-to-role assignment within a tenant
//   - iam_service_account: machine principal for API-to-API authentication
//   - iam_api_token: long-lived SHA-256-hashed API key for a service account
//   - iam_session: SQL audit trail (live state is in Redis, not here)
//   - iam_login_audit: tamper-evident append-only authentication event log
//
// Three global (non-tenant-scoped) tables are managed via migrations only
// and are NOT registered as EntityDefinitions:
//
//   - iam_roles: role catalogue seeded at bootstrap
//   - iam_permissions: permission identifier registry (compiler-populated)
//   - iam_role_permissions: role-to-permission bindings for CasbinEvaluator
//
// # Usage
//
//	rdb := goredis.NewClient(...)
//	sessions := redis.NewSessionStore(rdb)
//	tokenCache := redis.New(rdb)
//	m := iam.New(db, sessions, tokenCache)
//	m.RegisterRoutes(app)
//	rolePerms, _ := m.Auth.LoadRolePermissions(ctx)
//	evaluator, _ := auth.NewCasbinEvaluator(schema.CapabilityGrants, rolePerms)
//
// # Framework boundaries
//
// This package is application-agnostic. It MUST NOT import:
//   - ERP modules (finance, inventory, CRM, etc.)
//   - ERP entity types or services
//   - ERP configuration
//   - ERP workflows or business logic
//
// Dependencies allowed: awo/def, awo/auth, awo/cache, awo/filter, awo/runtime,
// and third-party infrastructure libraries (Fiber, pgx, bcrypt). Direct
// dependency on go-redis is eliminated — the caller injects [auth.SessionStore]
// and [cache.Cache] abstractions instead.
//
// # Migration location
//
// SQL migrations are in ./migrations/ relative to this package.
// They follow the golang-migrate naming convention:
// YYYYMMDDNNNNNN_description.{up,down}.sql
package iam

import (
	"awo.so/awo/audit"
	"awo.so/awo/auth"
	"awo.so/awo/cache"
	"awo.so/awo/def"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

// loginLimiterKey is a sentinel no-op handler used when no rate limiter is set.
// Using a real handler avoids a nil-check in registerRoutes.
var passthroughHandler fiber.Handler = func(c *fiber.Ctx) error { return c.Next() }

// Module bundles all IAM subsystem components into a single unit for
// framework-level registration. Construct with [New], then call
// [Module.RegisterRoutes] to attach HTTP endpoints to a Fiber application.
//
// Module is safe for concurrent use after [New] returns. Its fields are
// immutable after construction.
type Module struct {
	// Auth is the primary IAM service. Exposes Login, Logout,
	// RevokeUserSessions, and LoadRolePermissions for startup wiring.
	Auth *AuthService

	// loginLimiter is applied to POST /auth/login before the handler runs.
	// Defaults to passthroughHandler (no rate limiting) if not set via
	// [Module.WithLoginRateLimiter].
	loginLimiter fiber.Handler
}

// New constructs the IAM Module with its required runtime dependencies and
// wires them into the framework entity hook instances.
//
// New MUST be called before the Fiber application starts accepting requests.
// Calling New after the first HTTP request is a data race.
//
// deps:
//   - db:         PostgreSQL connection pool. Retained for the credential lookup
//     (sqlLookupCredentials) and API-token JOIN query that cannot be expressed
//     through EntityRepository (N+6D candidates).
//   - sessions:   Session store. Used for session persistence, retrieval, and
//     revocation. Production: [contrib/redis.RedisSessionStore].
//   - tokenCache: Cache for API token validation results. Avoids a database
//     round-trip on every service account request. Production:
//     [contrib/redis.Client] (implements [cache.Cache]).
//   - repos:      EntityRepository instances for tenant-scoped IAM entities.
//     See [IAMRepositories]. Build with [contrib/pgx.NewRepository] using
//     the compiled schema from [bootstrap.Result].
func New(db *pgxpool.Pool, sessions auth.SessionStore, tokenCache cache.Cache, repos IAMRepositories) *Module {
	// Wire the UserRoleChangeHook singleton with the session store.
	// The singleton is referenced by UserRoleDefinition.Hooks at init() time;
	// its Sessions field must be set before the first request is handled.
	userRoleChangeHook.Sessions = sessions

	return &Module{
		Auth: &AuthService{
			DB:       db,
			Sessions: sessions,
			Cache:    tokenCache,
			Repos:    repos,
		},
		loginLimiter: passthroughHandler,
	}
}

// WithAuditWriter sets the AuditWriter used to emit login, logout, and
// session-revocation events to the unified platform_audit_log table.
//
// Call before [Module.RegisterRoutes]. Nil disables audit writes (dev/test).
func (m *Module) WithAuditWriter(aw audit.AuditWriter) *Module {
	m.Auth.AuditWriter = aw
	return m
}

// WithLoginRateLimiter sets the Fiber handler applied to POST /auth/login
// before the login handler runs. Intended for use with
// [middleware.LoginRateLimit].
//
// Call before [Module.RegisterRoutes]. Calling after RegisterRoutes has no
// effect on already-registered routes.
//
//	m := iam.New(db, redis).WithLoginRateLimiter(
//	    middleware.LoginRateLimit(counter, middleware.DefaultLoginRateLimit),
//	)
func (m *Module) WithLoginRateLimiter(h fiber.Handler) *Module {
	m.loginLimiter = h
	return m
}

// RegisterRoutes attaches the IAM HTTP endpoints to the Fiber application.
//
// Routes registered (no authentication middleware on login):
//
//	POST  /api/v1/auth/login   — issue a session token
//	POST  /api/v1/auth/logout  — revoke the current session (requires session)
//	GET   /api/v1/auth/me      — return the current viewer's identity (requires session)
func (m *Module) RegisterRoutes(app *fiber.App) {
	registerRoutes(app, m.Auth, m.loginLimiter)
}

func init() {
	def.Register(&UserDefinition)
	def.Register(&UserRoleDefinition)
	def.Register(&ServiceAccountDefinition)
	def.Register(&APITokenDefinition)
	def.Register(&SessionDefinition)
	def.Register(&LoginAuditDefinition)
}
