package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	goredis "github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
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
	"awo.so/awo/def"
	"awo.so/awo/driver"

	// FIX: "awo.so/awo/events/outbox" — re-enable when events_outbox migration applied
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

	// Platform module init() calls.
	_ "awo.so/awo/platform/audit"
	_ "awo.so/awo/platform/flags"
	_ "awo.so/awo/platform/metadata"
	_ "awo.so/awo/platform/registry"
	_ "awo.so/awo/platform/settings"
	_ "awo.so/awo/platform/tenant"
)

// ServeConfig holds all configuration options for the HTTP server.
type ServeConfig struct {
	Port        string
	DB          string
	Redis       string
	LogLevel    string
	AppName     string
	OpenBrowser bool
}

// parseServeFlags parses args in the form [--flag value ...] and merges with
// environment variable fallbacks.
func parseServeFlags(args []string) ServeConfig {
	cfg := ServeConfig{
		Port:     getEnvDefault("PORT", "8080"),
		DB:       os.Getenv("DATABASE_URL"),
		Redis:    os.Getenv("REDIS_URL"),
		LogLevel: getEnvDefault("LOG_LEVEL", "info"),
		AppName:  getEnvDefault("APP_NAME", "awo"),
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--port", "-p":
			if i+1 < len(args) {
				i++
				cfg.Port = args[i]
			}
		case "--db":
			if i+1 < len(args) {
				i++
				cfg.DB = args[i]
			}
		case "--redis":
			if i+1 < len(args) {
				i++
				cfg.Redis = args[i]
			}
		case "--log-level":
			if i+1 < len(args) {
				i++
				cfg.LogLevel = args[i]
			}
		case "--app-name":
			if i+1 < len(args) {
				i++
				cfg.AppName = args[i]
			}
		case "--open":
			cfg.OpenBrowser = true
		}
	}
	return cfg
}

// startServer bootstraps the framework and starts the Fiber HTTP server.
// It blocks until the server is stopped by a signal or error.
func startServer(cfg ServeConfig) error {
	setupServeLogging(cfg.LogLevel)
	printServeBanner()

	if cfg.DB == "" {
		return fmt.Errorf("--db or DATABASE_URL is required")
	}
	if cfg.Redis == "" {
		return fmt.Errorf("--redis or REDIS_URL is required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	result, err := bootstrap.Run(ctx, bootstrap.Config{
		DatabaseURL: cfg.DB,
		RedisURL:    cfg.Redis,
		AppName:     cfg.AppName,
	})
	if err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}
	defer bootstrap.Shutdown(context.Background(), result)

	pool := result.Pool

	// Audit writer.
	auditSanitizer := audit.NewSanitizer()
	auditScorer := audit.NewRiskScorer().WithLoader(func(loaderCtx context.Context) ([]string, error) {
		rows, qErr := pool.Query(loaderCtx, "SELECT entity_name FROM audit_sensitive_entities")
		if qErr != nil {
			return nil, nil
		}
		defer rows.Close()
		var entities []string
		for rows.Next() {
			var name string
			if sErr := rows.Scan(&name); sErr != nil {
				return nil, sErr
			}
			entities = append(entities, name)
		}
		return entities, rows.Err()
	})
	if wErr := auditScorer.Warm(ctx); wErr != nil {
		slog.Warn("audit scorer warm failed; using defaults", "err", wErr)
	}
	auditFlagLoader := audit.FlagLoader(func(flagCtx context.Context) bool {
		var value string
		fErr := pool.QueryRow(
			flagCtx,
			"SELECT value FROM platform_audit_config WHERE key = 'feature.unified_audit.enabled'",
		).Scan(&value)
		if fErr != nil {
			return false
		}
		return value == "true"
	})
	auditWriter := audit.NewTransactionalWriter(contrib.NewPoolQuerier(pool), auditSanitizer, auditScorer).
		WithFlagLoader(auditFlagLoader)

	// IAM.
	sessions := contribredis.NewSessionStore(result.Redis)
	tokenCache := contribredis.New(result.Redis)

	iamSessionSchema, iamSessionOk := result.Schema.ByName["iam_session"]
	iamUserSchema, iamUserOk := result.Schema.ByName["iam_user"]
	iamUserRoleSchema, iamUserRoleOk := result.Schema.ByName["iam_user_role"]
	if !iamSessionOk || !iamUserOk || !iamUserRoleOk {
		return fmt.Errorf("required IAM entity schemas not found — platform/iam not registered")
	}
	iamRepos := iam.IAMRepositories{
		Sessions:  contrib.NewRepository(pool, iamSessionSchema),
		Users:     contrib.NewRepository(pool, iamUserSchema),
		UserRoles: contrib.NewRepository(pool, iamUserRoleSchema),
	}
	iamModule := iam.New(pool, sessions, tokenCache, iamRepos).WithAuditWriter(auditWriter)

	tenantSchema, ok := result.Schema.ByName["platform_tenant"]
	if !ok {
		return fmt.Errorf("platform_tenant entity not found in schema")
	}
	tenantRepo := contrib.NewRepository(pool, tenantSchema)

	// FIX: outbox relay disabled until events_outbox migration is applied.
	// relay := outbox.New(pool)
	// go func() {
	// 	if rErr := relay.Start(ctx); rErr != nil && ctx.Err() == nil {
	// 		slog.Error("outbox relay stopped", "err", rErr)
	// 	}
	// }()

	// Build Fiber app.
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ReadTimeout:           30 * time.Second,
		WriteTimeout:          30 * time.Second,
	})

	router.RegisterMiddleware(app, middleware.CORSConfig{
		BaseDomain:     getEnvDefault("BASE_DOMAIN", ""),
		AllowedOrigins: splitCommaStr(getEnvDefault("CORS_ALLOWED_ORIGINS", "")),
	})
	app.Use(metrics.Middleware())

	checker := health.New(pool, result.Redis)
	app.Get("/health/live", checker.Live)
	app.Get("/health/ready", checker.Ready)
	app.Get("/metrics", metrics.Handler())

	iamModule.RegisterRoutes(app)

	app.Get("/api/openapi.json", func(c *fiber.Ctx) error {
		return c.JSON(openapi.Generate(result.Schema, ""))
	})

	// Load RBAC policy. Degrade gracefully when IAM tables don't exist yet
	// (pre-migration dev environment). When tables are missing, disable both
	// RBAC and auth middleware so the UI is immediately accessible.
	var evaluator auth.PolicyEvaluator
	var iamAuth middleware.SessionValidator
	rolePerms, err := iamModule.Auth.LoadRolePermissions(ctx)
	if err != nil {
		slog.Warn("auth+RBAC disabled — IAM tables not found (run migrations to enable)",
			"err", err)
		// iamAuth stays nil → RequireAuth middleware skipped → UI accessible without login
	} else {
		iamAuth = iamModule.Auth
		evaluator, err = auth.NewCasbinEvaluator(result.Schema.CapabilityGrants, rolePerms)
		if err != nil {
			return fmt.Errorf("casbin evaluator: %w", err)
		}
	}

	// Temporal client — wired when TEMPORAL_HOST is set; nil = degraded mode.
	// When nil, the NoopExecutor in the router/handler layer handles workflow
	// starts gracefully so CRUD still works.
	var temporalClient temporalclient.Client
	if temporalHost := getEnvDefault("TEMPORAL_HOST", ""); temporalHost != "" {
		tc, tcErr := temporalclient.Dial(temporalclient.Options{
			HostPort: temporalHost,
		})
		if tcErr != nil {
			slog.Warn("temporal client dial failed; workflow starts disabled",
				"host", temporalHost, "err", tcErr)
		} else {
			temporalClient = tc
			defer temporalClient.Close()
			slog.Info("temporal client connected", "host", temporalHost)
		}
	}

	sduiEng := sdui_engine.New(sdui_engine.Options{
		Generator: sdui_generator.New(),
		Validator: sdui_validation.New(),
		Layout:    sdui_layout.New(),
		Renderers: map[string]sdui_renderer.Renderer{
			sdui_amis.RendererID: sdui_amis.New(),
		},
		DefaultRendererID: sdui_amis.RendererID,
		Cache:             serveSDUICache(result.Redis),
	})

	// In dev mode (iamAuth == nil), inject a platform-admin dev viewer into
	// every request so ViewerFromContext() doesn't panic in handlers.
	// Also skip TenantResolver — no tenant table yet.
	var tenants driver.EntityRepository[*def.EntityRecord]
	if iamAuth != nil {
		tenants = tenantRepo
	} else {
		// uuid.Nil fails GeneratorContext.Validate(). Use a stable non-nil dev UUID.
		devTenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
		devViewer := auth.NewSystemViewer(devTenantID)
		app.Use(func(c *fiber.Ctx) error {
			c.SetUserContext(auth.WithViewer(c.UserContext(), devViewer))
			return c.Next()
		})
		slog.Warn("dev mode: all requests run as platform-admin (no auth)")
	}

	router.Register(app, result.Schema, router.RegisterOptions{
		Pool:        pool,
		Redis:       result.Redis,
		IAM:         iamAuth,
		Tenants:     tenants,
		Authz:       evaluator,
		Temporal:    temporalClient,
		AuditWriter: auditWriter,
		SDUIEngine:  sduiEng,
	})

	// Vite build output. Serves /assets/*, /sdk/*, /locales/*, etc.
	// Files not found here fall through to the SPA catch-all below.
	app.Static("/", "./awo/web/dist", fiber.Static{Compress: true, MaxAge: 86400})
	// Web UI SPA — all /ui/* paths serve the built index.html.
	// In development, Vite dev server (port 3000) handles this instead.
	app.Get("/ui/*", func(c *fiber.Ctx) error {
		return c.SendFile("./awo/web/dist/index.html")
	})
	// Showcase developer portal.
	app.Get("/showcase*", func(c *fiber.Ctx) error {
		return c.SendFile("./awo/web/showcase/index.html")
	})

	// Showcase diagnostic API (unauthenticated dev endpoints).
	showcaseHandler := showcasepkg.New(result.Schema, sduiEng)
	showcaseHandler.Register(app)

	port := cfg.Port
	printServeStartupInfo(port, len(result.Schema.Entities), len(result.Schema.Routes))

	if cfg.OpenBrowser {
		go func() {
			time.Sleep(500 * time.Millisecond)
			serveOpenBrowser("http://localhost:" + port + "/showcase")
		}()
	}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- app.Listen(":" + port)
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return app.ShutdownWithContext(shutdownCtx)
	case sErr := <-serverErr:
		return sErr
	}
}

func printServeBanner() {
	fmt.Println()
	fmt.Println("  +------------------------------------------+")
	fmt.Println("  |  Awo Framework  --  metadata-driven ERP  |")
	fmt.Println("  |  SDUI . Multi-tenant . Workflow-native    |")
	fmt.Println("  +------------------------------------------+")
	fmt.Println()
}

func printServeStartupInfo(port string, entities, routes int) {
	fmt.Printf("  Server ready  (%d entities, %d routes)\n", entities, routes)
	fmt.Printf("     API      http://localhost:%s/api/v1\n", port)
	fmt.Printf("     UI       http://localhost:%s/ui\n", port)
	fmt.Printf("     Showcase http://localhost:%s/showcase\n", port)
	fmt.Printf("     Health   http://localhost:%s/health/ready\n", port)
	fmt.Println()
}

func setupServeLogging(level string) {
	l := slog.LevelInfo
	_ = l.UnmarshalText([]byte(level))
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: l})))
}

func serveOpenBrowser(url string) {
	var cmd string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
		return
	default:
		cmd = "xdg-open"
	}
	_ = exec.Command(cmd, url).Start()
}

func serveSDUICache(rdb *goredis.Client) *sdui_cache.Cache {
	if rdb == nil {
		return sdui_cache.New(nil)
	}
	return sdui_cache.New(&serveRedisAdapter{rdb: rdb})
}

type serveRedisAdapter struct {
	rdb *goredis.Client
}

func (a *serveRedisAdapter) Get(ctx context.Context, key string) (string, error) {
	val, err := a.rdb.Get(ctx, key).Result()
	if err == goredis.Nil {
		return "", sdui_cache.ErrCacheMiss
	}
	return val, err
}

func (a *serveRedisAdapter) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return a.rdb.Set(ctx, key, value, ttl).Err()
}

func getEnvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func splitCommaStr(s string) []string {
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
