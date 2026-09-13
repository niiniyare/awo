package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// CORSConfig controls which origins are allowed.
type CORSConfig struct {
	// BaseDomain is the root domain, e.g. "awo.so".
	// Tenant subdomains are matched as *.BaseDomain.
	BaseDomain string

	// AllowedOrigins is an explicit allowlist of origins (in addition to
	// subdomain matching). Use for developer portals, localhost in dev, etc.
	AllowedOrigins []string
}

// CORS returns per-tenant subdomain-aware CORS middleware.
//
// Allowed origins:
//  1. Any explicit origin in cfg.AllowedOrigins.
//  2. Any *.{BaseDomain} subdomain.
//  3. {BaseDomain} itself (apex).
//
// Non-matching origins receive no CORS headers — the browser enforces the
// same-origin policy and blocks the request.
func CORS(cfg CORSConfig) fiber.Handler {
	explicitSet := make(map[string]bool, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		explicitSet[strings.ToLower(o)] = true
	}

	return func(c *fiber.Ctx) error {
		origin := c.Get("Origin")
		if origin == "" {
			// Non-browser request — skip CORS headers.
			return c.Next()
		}

		if isAllowed(origin, cfg.BaseDomain, explicitSet) {
			c.Set("Access-Control-Allow-Origin", origin)
			c.Set("Access-Control-Allow-Credentials", "true")
			c.Set("Access-Control-Allow-Methods", "GET,POST,PATCH,PUT,DELETE,OPTIONS")
			c.Set("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Tenant-ID,X-Request-ID")
			c.Set("Access-Control-Max-Age", "86400")
			c.Set("Vary", "Origin")
		}

		// Handle preflight.
		if c.Method() == fiber.MethodOptions {
			return c.SendStatus(fiber.StatusNoContent)
		}
		return c.Next()
	}
}

// isAllowed reports whether origin is permitted under the given baseDomain.
func isAllowed(origin, baseDomain string, explicit map[string]bool) bool {
	lower := strings.ToLower(origin)

	if explicit[lower] {
		return true
	}
	if baseDomain == "" {
		return false
	}

	// Strip scheme.
	host := lower
	if i := strings.Index(host, "://"); i >= 0 {
		host = host[i+3:]
	}
	// Strip port.
	if i := strings.LastIndex(host, ":"); i >= 0 {
		host = host[:i]
	}

	base := strings.ToLower(baseDomain)
	return host == base || strings.HasSuffix(host, "."+base)
}
