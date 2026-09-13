package middleware

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"

	"awo.so/awo/api/response"
	"awo.so/awo/cache"
	"awo.so/awo/runtime"
)

// RateLimitConfig controls the sliding-window rate limiter.
type RateLimitConfig struct {
	// Requests is the maximum number of requests allowed per window.
	Requests int64
	// Window is the sliding window duration.
	Window time.Duration
}

// DefaultRateLimit is the global default: 1000 req/min per tenant+user.
var DefaultRateLimit = RateLimitConfig{
	Requests: 1000,
	Window:   time.Minute,
}

// RateLimit enforces per-tenant per-user sliding window rate limits using
// the Counter abstraction (Redis INCR in production).
//
// Key pattern: rl:{tenant_id}:{user_id}:{window_start_epoch}
func RateLimit(counter cache.Counter, cfg RateLimitConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tenantID, _ := c.Locals("tenant_id").(string)
		userID, _ := c.Locals("user_id").(string)
		if tenantID == "" {
			tenantID = "anon"
		}
		if userID == "" {
			userID = c.IP()
		}
		key := fmt.Sprintf("rl:%s:%s", tenantID, userID)

		count, err := counter.IncrementWithReset(c.UserContext(), key, 1, cfg.Window)
		if err != nil {
			// Counter failure is non-fatal — allow request through (fail open).
			// Log only.
			return c.Next()
		}

		c.Set("X-RateLimit-Limit", fmt.Sprintf("%d", cfg.Requests))
		c.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", max64(0, cfg.Requests-count)))

		if count > cfg.Requests {
			return c.Status(fiber.StatusTooManyRequests).JSON(response.Wrap(&runtime.BusinessError{
				Code:    "rate_limit_exceeded",
				Message: "Too many requests — please slow down",
				Status:  429,
			}))
		}
		return c.Next()
	}
}

// LoginRateLimit returns a Fiber middleware that enforces an IP-based
// sliding-window rate limit specifically for the login endpoint.
//
// It is intentionally separate from [RateLimit]:
//   - Login is unauthenticated — no session or tenant context exists yet.
//   - The key is derived from the client IP, not tenant+user.
//   - The default limit is much stricter (10 attempts/min vs 1000/min).
//
// On Redis failure the middleware fails-open (allows the request through)
// to prevent a Redis outage from locking all users out. Log the error.
//
// Default configuration: 10 attempts per IP per minute.
// Override by passing a custom [RateLimitConfig].
func LoginRateLimit(counter cache.Counter, cfg RateLimitConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		key := fmt.Sprintf("rl:login:%s", c.IP())

		count, err := counter.IncrementWithReset(c.UserContext(), key, 1, cfg.Window)
		if err != nil {
			// Fail-open: Redis outage must not prevent legitimate logins.
			// TODO: emit metric rl_login_redis_failure_total
			return c.Next()
		}

		if count > cfg.Requests {
			return c.Status(fiber.StatusTooManyRequests).JSON(response.Wrap(&runtime.BusinessError{
				Code:    "iam.login_rate_limited",
				Message: "Too many login attempts. Please wait before trying again.",
				Status:  429,
			}))
		}

		return c.Next()
	}
}

// DefaultLoginRateLimit is a conservative IP-based limit for POST /auth/login.
// 10 attempts per minute per IP. Adjust at registration time via [LoginRateLimit].
var DefaultLoginRateLimit = RateLimitConfig{
	Requests: 10,
	Window:   time.Minute,
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
