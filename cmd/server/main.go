// Command server starts the Awo API server and Temporal worker.
//
// The server starts in two phases:
//  1. Bootstrap: PostgreSQL + Redis + entity registry + schema compilation.
//  2. Serve: Fiber HTTP server starts; Temporal worker starts concurrently.
//
// Configuration is loaded from environment variables. All required variables
// must be set; the process exits with a clear error message if any are missing.
//
// Environment variables:
//
//	DATABASE_URL   — PostgreSQL DSN (required)
//	REDIS_URL      — Redis URL (required)
//	PORT           — HTTP listen port (default: 8080)
//	TEMPORAL_HOST  — Temporal server host:port (default: localhost:7233)
//	LOG_LEVEL      — debug|info|warn|error (default: info)
//	APP_NAME       — service name for observability labels (default: awo)
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	goredis "github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	temporalclient "go.temporal.io/sdk/client"

	"awo.so/awo/api/middleware"
	"awo.so/awo/api/openapi"
	"awo.so/awo/api/router"
	showcasepkg "awo.so/awo/api/showcase"
	"awo.so/awo/audit"
	"awo.so/awo/auth"
	"awo.so/awo/bootstrap"
	contrib "awo.so/awo/contrib/pgx"
	contribredis "awo.so/awo/contrib/redis"
	"awo.so/awo/events/outbox"
	"awo.so/awo/observability/health"
	"awo.so/awo/observability/metrics"
	"awo.so/awo/platform/iam"
	sdui_amis "awo.so/awo/sdui/amis"
	sdui_cache "awo.so/awo/sdui/cache"
	sdui_engine "awo.so/awo/sdui/engine"
	sdui_generator "awo.so/awo/sdui/generator"
	sdui_layout "awo.so/awo/sdui/layout"
	sdui_renderer "awo.so/awo/sdui/renderer"
	sdui_validation "awo.so/awo/sdui/validation"

	// Platform module init() calls — imports drive entity registration.
	// platform/iam is already imported above for iam.New; the init()
	// side effect (entity registration) is included via that import.
	_ "awo.so/awo/platform/attachment"
	_ "awo.so/awo/platform/audit"
	_ "awo.so/awo/platform/flags"
	_ "awo.so/awo/platform/mail"
	_ "awo.so/awo/platform/metadata"

	// Finance module — registers all finance_* entities via init().
	_ "awo.so/modules/finance"
	_ "awo.so/awo/platform/notification"
	_ "awo.so/awo/platform/organization"
	_ "awo.so/awo/platform/registry"
	_ "awo.so/awo/platform/settings"
	_ "awo.so/awo/platform/tenant"
)

func main() {
	setupLogging()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Bootstrap infrastructure.
	result, err := bootstrap.Run(ctx, bootstrap.Config{
		DatabaseURL: mustEnv("DATABASE_URL"),
		RedisURL:    mustEnv("REDIS_URL"),
		AppName:     getEnv("APP_NAME", "awo"),
	})
	if err != nil {
		slog.Error("bootstrap failed", "err", err)
		os.Exit(1)
	}
	defer bootstrap.Shutdown(context.Background(), result)

	// Audit writer — writes AuditRecord to platform_audit_log inside the entity
	// transaction. contrib.NewPoolQuerier provides the fallback connection for
	// standalone auth writes that occur outside an entity transaction.
	//
	// Sanitizer strips sensitive fields before any INSERT; RiskScorer computes
	// the 0-100 risk score and derives the Severity for each record.
	auditSanitizer := audit.NewSanitizer()
	// SensitiveEntityLoader queries audit_sensitive_entities (Phase 2 migration).
	// When the table does not yet exist, the query fails and the loader returns
	// nil, nil — the scorer falls back to default rules (no risk premium).
	pool := result.Pool
	auditScorer := audit.NewRiskScorer().WithLoader(func(ctx context.Context) ([]string, error) {
		rows, err := pool.Query(ctx, "SELECT entity_name FROM audit_sensitive_entities")
		if err != nil {
			// Table may not exist pre-Phase-2 migration; treat as empty.
			slog.WarnContext(ctx, "audit: sensitive entity table not available; using defaults", "err", err)
			return nil, nil
		}
		defer rows.Close()
		var entities []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return nil, err
			}
			entities = append(entities, name)
		}
		return entities, rows.Err()
	})
	if err := auditScorer.Warm(ctx); err != nil {
		slog.Warn("audit scorer warm failed; using default scoring rules", "err", err)
	}
	// FlagLoader reads the feature flag from platform_audit_config.
	// When the table does not exist (pre-migration), the query fails and the
	// loader returns false — legacy audit system remains active (ADR-019 Phase 2).
	auditFlagLoader := audit.FlagLoader(func(ctx context.Context) bool {
		var value string
		err := pool.QueryRow(
			ctx,
			"SELECT value FROM platform_audit_config WHERE key = 'feature.unified_audit.enabled'",
		).Scan(&value)
		if err != nil {
			slog.Warn("audit: feature flag read failed; unified audit disabled", "err", err)
			return false
		}
		return value == "true"
	})

	auditWriter := audit.NewTransactionalWriter(contrib.NewPoolQuerier(result.Pool), auditSanitizer, auditScorer).
		WithFlagLoader(auditFlagLoader)

	// IAM module — authenticates users and manages sessions.
	// Construct the SessionStore and token cache from the shared Redis client,
	// then inject them into iam.New as abstract interfaces.
	sessions := contribredis.NewSessionStore(result.Redis)
	tokenCache := contribredis.New(result.Redis)

	// IAM EntityRepository instances — used for session persistence, last_login_at
	// updates, and user-role loading. Each repo is scoped to its entity schema.
	// Operations on tenant-scoped tables must run inside repo.WithTx so the
	// pgx contrib layer calls set_tenant_context before executing DML.
	iamSessionSchema, ok := result.Schema.ByName["iam_session"]
	if !ok {
		slog.Error("iam_session entity not found in schema — platform/iam not registered")
		os.Exit(1)
	}
	iamUserSchema, ok := result.Schema.ByName["iam_user"]
	if !ok {
		slog.Error("iam_user entity not found in schema — platform/iam not registered")
		os.Exit(1)
	}
	iamUserRoleSchema, ok := result.Schema.ByName["iam_user_role"]
	if !ok {
		slog.Error("iam_user_role entity not found in schema — platform/iam not registered")
		os.Exit(1)
	}
	iamRepos := iam.IAMRepositories{
		Sessions:  contrib.NewRepository(result.Pool, iamSessionSchema),
		Users:     contrib.NewRepository(result.Pool, iamUserSchema),
		UserRoles: contrib.NewRepository(result.Pool, iamUserRoleSchema),
	}
	iamModule := iam.New(result.Pool, sessions, tokenCache, iamRepos).WithAuditWriter(auditWriter)

	// Tenant entity repository for TenantResolver middleware.
	tenantSchema, ok := result.Schema.ByName["platform_tenant"]
	if !ok {
		slog.Error("platform_tenant entity not found in schema — platform/tenant not registered")
		os.Exit(1)
	}
	tenantRepo := contrib.NewRepository(result.Pool, tenantSchema)

	// Outbox relay — delivers domain events from the transactional outbox.
	relay := outbox.New(result.Pool)
	go func() {
		if err := relay.Start(ctx); err != nil && ctx.Err() == nil {
			slog.Error("outbox relay stopped unexpectedly", "err", err)
		}
	}()

	// Build Fiber app.
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ReadTimeout:           30 * time.Second,
		WriteTimeout:          30 * time.Second,
	})

	// Global middleware pipeline — request ID, logger, recovery, CORS.
	router.RegisterMiddleware(app, middleware.CORSConfig{
		BaseDomain:     getEnv("BASE_DOMAIN", ""),
		AllowedOrigins: splitComma(getEnv("CORS_ALLOWED_ORIGINS", "")),
	})

	// Metrics middleware.
	app.Use(metrics.Middleware())

	// Health checks (no auth required).
	checker := health.New(result.Pool, result.Redis)
	app.Get("/health/live", checker.Live)
	app.Get("/health/ready", checker.Ready)
	app.Get("/metrics", metrics.Handler())

	// Auth routes (login/logout/me — no upstream RequireAuth middleware).
	iamModule.RegisterRoutes(app)

	// Metadata API — entity schema introspection, no auth required.
	router.RegisterMeta(app, result.Schema)

	// OpenAPI schema endpoint (no auth required).
	app.Get("/api/openapi.json", func(c *fiber.Ctx) error {
		doc := openapi.Generate(result.Schema, "")
		return c.JSON(doc)
	})

	// Build RBAC evaluator from compiled capability grants + IAM role-permission bindings.
	rolePerms, err := iamModule.Auth.LoadRolePermissions(ctx)
	if err != nil {
		slog.Error("load role permissions failed", "err", err)
		os.Exit(1)
	}
	evaluator, err := auth.NewCasbinEvaluator(result.Schema.CapabilityGrants, rolePerms)
	if err != nil {
		slog.Error("casbin evaluator init failed", "err", err)
		os.Exit(1)
	}

	// SDUI engine — produces rendered UI schemas from EntitySchema.
	// Cache backed by Redis when available; nil disables caching (dev mode).
	sduiEng := sdui_engine.New(sdui_engine.Options{
		Generator: sdui_generator.New(),
		Validator: sdui_validation.New(),
		Layout:    sdui_layout.New(),
		Renderers: map[string]sdui_renderer.Renderer{
			sdui_amis.RendererID: sdui_amis.New(),
		},
		DefaultRendererID: sdui_amis.RendererID,
		Cache:             sduiCacheFor(result.Redis),
	})

	// Temporal client — wired when TEMPORAL_HOST is set; nil = degraded mode.
	// NoopExecutor in the router/handler layer handles nil gracefully.
	var temporalClient temporalclient.Client
	if temporalHost := getEnv("TEMPORAL_HOST", ""); temporalHost != "" {
		tc, tcErr := temporalclient.Dial(temporalclient.Options{
			HostPort: temporalHost,
		})
		if tcErr != nil {
			slog.Warn("temporal client dial failed; workflow starts disabled", "host", temporalHost, "err", tcErr)
		} else {
			temporalClient = tc
			defer temporalClient.Close()
			slog.Info("temporal client connected", "host", temporalHost)
		}
	} else {
		slog.Info("TEMPORAL_HOST not set; running in degraded mode (no workflow starts)")
	}

	// CRUD routes for all registered entities — full middleware pipeline applied inside.
	router.Register(app, result.Schema, router.RegisterOptions{
		Pool:               result.Pool,
		Redis:              result.Redis,
		IAM:                iamModule.Auth,
		Tenants:            tenantRepo,
		Authz:              evaluator,
		Temporal:           temporalClient,
		AuditWriter:        auditWriter,
		AuditSigningSecret: getEnv("AUDIT_SIGNING_SECRET", ""),
		SDUIEngine:         sduiEng,
	})

	// Static file serving — amis SDK assets.
	// Serve /static/* from awo/web/static/ relative to the binary working dir.
	app.Static("/static", "./awo/web/static", fiber.Static{
		Compress: true,
		MaxAge:   86400, // 1 day — SDK files are pinned and content-stable
	})

	// Web UI fallback — serve index.html for all /ui/* paths (SPA routing).
	app.Get("/ui/*", func(c *fiber.Ctx) error {
		return c.SendFile("./awo/web/pages/index.html")
	})

	// Showcase — developer experience portal.
	app.Get("/showcase*", func(c *fiber.Ctx) error {
		return c.SendFile("./awo/web/showcase/index.html")
	})

	// Showcase diagnostic API — unauthenticated; expose only in non-production.
	showcaseHandler := showcasepkg.New(result.Schema, sduiEng)
	showcaseHandler.Register(app)

	// Start server.
	port := getEnv("PORT", "8080")
	slog.Info("server starting", "port", port, "app", getEnv("APP_NAME", "awo"))

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- app.Listen(":" + port)
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := app.ShutdownWithContext(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", "err", err)
		}
	case err := <-serverErr:
		if err != nil {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}
}

func setupLogging() {
	level := slog.LevelInfo
	if l := os.Getenv("LOG_LEVEL"); l != "" {
		_ = level.UnmarshalText([]byte(l))
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required environment variable is not set", "var", key)
		os.Exit(1)
	}
	return v
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// sduiCacheFor constructs an SDUI cache backed by Redis, or a no-op cache when rdb is nil.
func sduiCacheFor(rdb *goredis.Client) *sdui_cache.Cache {
	if rdb == nil {
		return sdui_cache.New(nil)
	}
	return sdui_cache.New(&goRedisSDUIAdapter{rdb: rdb})
}

// goRedisSDUIAdapter adapts *goredis.Client to sdui_cache.RedisClient.
// The sdui cache uses plain string Get/Set; go-redis uses Cmd result types.
type goRedisSDUIAdapter struct {
	rdb *goredis.Client
}

func (a *goRedisSDUIAdapter) Get(ctx context.Context, key string) (string, error) {
	val, err := a.rdb.Get(ctx, key).Result()
	if err == goredis.Nil {
		return "", sdui_cache.ErrCacheMiss
	}
	return val, err
}

func (a *goRedisSDUIAdapter) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return a.rdb.Set(ctx, key, value, ttl).Err()
}

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
