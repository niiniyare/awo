// Package health provides liveness and readiness check handlers.
//
//	GET /health/live  → 200 if process is running (no dependency checks)
//	GET /health/ready → 200 only if PostgreSQL + Redis reachable and
//	                    EntityRegistry populated
//
// Liveness failure: restart the process.
// Readiness failure: remove from load balancer rotation.
package health

import (
	"context"
	"time"

	goredis "github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"awo.so/awo/def"
)

// Checker holds the dependencies needed for readiness checks.
type Checker struct {
	pool  *pgxpool.Pool
	redis *goredis.Client
}

// New creates a Checker.
func New(pool *pgxpool.Pool, redis *goredis.Client) *Checker {
	return &Checker{pool: pool, redis: redis}
}

// Live returns 200 unconditionally — the process is running.
func (ch *Checker) Live(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}

// Ready returns 200 if all dependencies are reachable.
// Returns 503 with a JSON body describing which checks failed.
func (ch *Checker) Ready(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.UserContext(), 3*time.Second)
	defer cancel()

	checks := map[string]string{}
	ok := true

	// PostgreSQL ping.
	if err := ch.pool.Ping(ctx); err != nil {
		checks["postgres"] = err.Error()
		ok = false
	} else {
		checks["postgres"] = "ok"
	}

	// Redis ping.
	if err := ch.redis.Ping(ctx).Err(); err != nil {
		checks["redis"] = err.Error()
		ok = false
	} else {
		checks["redis"] = "ok"
	}

	// Entity registry populated.
	if def.Count() == 0 {
		checks["registry"] = "empty — no entities registered"
		ok = false
	} else {
		checks["registry"] = "ok"
	}

	if !ok {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status": "degraded",
			"checks": checks,
		})
	}
	return c.JSON(fiber.Map{"status": "ok", "checks": checks})
}
