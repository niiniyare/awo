package middleware

import (
	"errors"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Logger emits a structured slog entry for every completed request.
// Fields: request_id, method, path, status, duration_ms, ip.
//
// Status derivation (R.12): a handler that returns a non-nil error instead of
// calling c.Status() itself has NOT yet had its real response status written
// anywhere by the time c.Next() returns here. Fiber only writes that status
// later, in app.handler's call to the configured fiber.Config.ErrorHandler
// (DefaultErrorHandler unless overridden - Awo does not override it) - which
// runs at the top of the request after every app.Use middleware, including
// this one, has already returned up the call stack. Two real, confirmed
// cases take exactly this path: Fiber's own router.go "no route matched at
// all" fallback (router.go's app.next, which returns a *fiber.Error{Code:
// 404} without ever touching c.Response()), and any Awo handler that itself
// returns a *fiber.Error directly rather than calling c.Status() first
// (e.g. api/sdui/handler.go, api/meta/handler.go). Reading
// c.Response().StatusCode() in either case sees fasthttp's un-set default
// (200), not the 404/other status that will actually be sent - verified live
// by comparing the real wire response against this log line for the same
// request_id. DefaultErrorHandler's own status derivation is mirrored here
// so the logged status matches what the client actually receives: a
// *fiber.Error's Code, or 500 for any other error type (DefaultErrorHandler's
// own fallback). A handler that already called c.Status() itself (Awo's own
// CRUD/API handlers all do, e.g. api/handler/crud.go) returns a nil error in
// the common case, so this fallback path is not taken and behavior for those
// responses is unchanged.
func Logger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		duration := time.Since(start)

		status := c.Response().StatusCode()
		if err != nil {
			status = fiber.StatusInternalServerError
			var fiberErr *fiber.Error
			if errors.As(err, &fiberErr) {
				status = fiberErr.Code
			}
		}

		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		} else if status >= 400 {
			level = slog.LevelWarn
		}

		slog.Log(c.Context(), level, "request",
			"request_id", c.Locals("request_id"),
			"tenant_id", c.Locals("tenant_id"),
			"user_id", c.Locals("user_id"),
			"method", c.Method(),
			"path", c.Path(),
			"status", status,
			"duration_ms", duration.Milliseconds(),
			"ip", c.IP(),
		)
		return err
	}
}
