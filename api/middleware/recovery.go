package middleware

import (
	"log/slog"
	"runtime/debug"

	"github.com/gofiber/fiber/v2"

	"awo.so/awo/api/response"
)

// Recovery catches panics and returns a structured 500 response.
// Stack traces are logged but never sent to the client.
func Recovery() fiber.Handler {
	return func(c *fiber.Ctx) (err error) {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				slog.Error("panic recovered",
					"request_id", c.Locals("request_id"),
					"panic", r,
					"stack", string(stack),
				)
				err = c.Status(fiber.StatusInternalServerError).JSON(response.Error{
					Code:    "internal_error",
					Message: "An unexpected error occurred",
				})
			}
		}()
		return c.Next()
	}
}
