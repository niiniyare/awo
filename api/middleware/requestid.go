// Package middleware provides the fixed-order Awo middleware pipeline.
//
// Pipeline order (not configurable at runtime — changing requires code change + redeploy):
//  1. Request ID
//  2. Structured logging
//  3. Panic recovery
//  4. CORS
//  5. Tenant resolution → set_tenant_context()
//  6. Session validation (Redis lookup, expiry check)
//  7. Rate limiting (Redis sliding window, per-tenant + per-user)
package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const headerRequestID = "X-Request-ID"

// RequestID injects a request ID into every request. If the client provides
// X-Request-ID, that value is used; otherwise a UUIDv4 is generated.
// The request ID is stored in c.Locals("request_id") and echoed back in
// the X-Request-ID response header.
func RequestID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Get(headerRequestID)
		if id == "" {
			id = uuid.New().String()
		}
		c.Locals("request_id", id)
		c.Set(headerRequestID, id)
		return c.Next()
	}
}
