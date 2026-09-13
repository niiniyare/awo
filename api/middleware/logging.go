package middleware

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Logger emits a structured slog entry for every completed request.
// Fields: request_id, method, path, status, duration_ms, ip.
func Logger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		duration := time.Since(start)

		level := slog.LevelInfo
		if c.Response().StatusCode() >= 500 {
			level = slog.LevelError
		} else if c.Response().StatusCode() >= 400 {
			level = slog.LevelWarn
		}

		slog.Log(c.Context(), level, "request",
			"request_id", c.Locals("request_id"),
			"tenant_id", c.Locals("tenant_id"),
			"user_id", c.Locals("user_id"),
			"method", c.Method(),
			"path", c.Path(),
			"status", c.Response().StatusCode(),
			"duration_ms", duration.Milliseconds(),
			"ip", c.IP(),
		)
		return err
	}
}
