// Package bootstrap implements the Awo framework startup sequence.
//
// Startup order (hard dependency chain):
//
//  1. Config load & validate
//  2. PostgreSQL pool init (ping)
//  3. Redis client init (ping)
//  4. EntityRegistry init + system entity registration (all modules via init())
//  5. Fiber app init + route registration (derived from EntityRegistry)
//  6. Fiber server start
//  7. Temporal worker start (concurrent with server, degraded-ok)
//
// EntityRegistry failure is fatal — the process exits because it cannot serve
// requests without registered routes. Temporal failure is degraded — CRUD
// works but workflow starts fail gracefully.
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	goredis "github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"

	"awo.so/awo/audit"
	"awo.so/awo/compiler"
	"awo.so/awo/registry"
)

// Config holds all required bootstrap configuration. All fields are required;
// Bootstrap returns an error if any are zero-valued.
type Config struct {
	// DatabaseURL is the pgx-compatible PostgreSQL DSN.
	// Required. Must include the database name.
	DatabaseURL string

	// RedisURL is the go-redis compatible URL (redis://host:port/db).
	// Required. Redis failure during startup is fatal.
	RedisURL string

	// AppName is used in observability labels and log context.
	AppName string

	// ConnectTimeout is how long to wait for initial DB and Redis connections.
	// Default: 10 seconds.
	ConnectTimeout time.Duration
}

func (c *Config) applyDefaults() {
	if c.ConnectTimeout == 0 {
		c.ConnectTimeout = 10 * time.Second
	}
	if c.AppName == "" {
		c.AppName = "awo"
	}
}

func (c *Config) validate() error {
	if c.DatabaseURL == "" {
		return errors.New("bootstrap: DatabaseURL is required")
	}
	if c.RedisURL == "" {
		return errors.New("bootstrap: RedisURL is required")
	}
	return nil
}

// Result holds the initialized infrastructure after a successful Bootstrap call.
type Result struct {
	Pool   *pgxpool.Pool
	Redis  *goredis.Client
	Schema *compiler.CompiledSchema
}

// Run initialises all framework infrastructure in dependency order.
// It returns a Result on success. Any failure is fatal — callers should log
// the error and exit the process.
//
// Run does NOT start the HTTP server or Temporal worker. Those are the
// caller's responsibility after receiving the Result.
func Run(ctx context.Context, cfg Config) (*Result, error) {
	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	// Step 1: PostgreSQL.
	pool, err := initPostgres(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: postgres: %w", err)
	}
	slog.Info("postgres connected", "app", cfg.AppName)

	// Step 2: Redis.
	rdb, err := initRedis(ctx, cfg)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("bootstrap: redis: %w", err)
	}
	slog.Info("redis connected", "app", cfg.AppName)

	// Step 3: Build and compile entity registry.
	// All platform and business modules must have registered their definitions
	// via init() before this point. Import side effects drive registration.
	reg := registry.Build()
	slog.Info("entity registry built", "entities", len(reg.All()))

	// Seal the audit config registry — no further Register calls are permitted.
	// This must happen after all init() functions have run (guaranteed by the
	// time Run is called) and before any request is served.
	audit.Seal()

	schema, err := compiler.Compile(reg)
	if err != nil {
		pool.Close()
		_ = rdb.Close()
		return nil, fmt.Errorf("bootstrap: compile schema: %w", err)
	}
	slog.Info("schema compiled",
		"entities", len(schema.Entities),
		"routes", len(schema.Routes),
		"capability_grants", len(schema.CapabilityGrants),
	)

	return &Result{Pool: pool, Redis: rdb, Schema: schema}, nil
}

// Shutdown gracefully closes all infrastructure connections.
// Call with a deadline context to bound the shutdown time.
func Shutdown(_ context.Context, r *Result) {
	if r.Pool != nil {
		r.Pool.Close()
	}
	if r.Redis != nil {
		_ = r.Redis.Close()
	}
}

func initPostgres(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	pctx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()

	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(pctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(pctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}

func initRedis(ctx context.Context, cfg Config) (*goredis.Client, error) {
	opts, err := goredis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	rdb := goredis.NewClient(opts)

	pctx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()

	if err := rdb.Ping(pctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return rdb, nil
}
