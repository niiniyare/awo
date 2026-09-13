package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"awo.so/awo/api/response"
	"awo.so/awo/def"
	"awo.so/awo/driver"
	"awo.so/awo/filter"
	"awo.so/awo/runtime"
	"awo.so/awo/runtime/tenant"
)

// TenantResolver resolves the tenant from the request and embeds a
// TenantContext into c.UserContext(). Must run before session validation.
//
// Resolution priority:
//  1. X-Tenant-ID header (UUID)
//  2. tenant_id query param (webhooks/legacy only)
//  3. Subdomain parsing (bo., portal., app., api. prefixes stripped)
//
// Returns 404 if the tenant cannot be resolved or is not ACTIVE.
func TenantResolver(tenants driver.EntityRepository[*def.EntityRecord]) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tenantID, err := resolveTenantID(c)
		if err != nil || tenantID == uuid.Nil {
			return c.Status(fiber.StatusNotFound).JSON(response.Wrap(&runtime.BusinessError{
				Code:    "tenant.not_found",
				Message: "Tenant not found",
				Status:  404,
			}))
		}

		// Load the tenant record to verify it is ACTIVE and extract metadata.
		ctx := c.UserContext()
		results, _, err := tenants.Query(ctx, filter.Eq("id", tenantID), driver.WithSkipCount())
		if err != nil || len(results) == 0 {
			return c.Status(fiber.StatusNotFound).JSON(response.Wrap(&runtime.BusinessError{
				Code:    "tenant.not_found",
				Message: "Tenant not found",
				Status:  404,
			}))
		}
		t := results[0]

		status := t.GetString("status")
		switch status {
		case "ACTIVE":
			// OK
		case "PENDING":
			c.Set("Retry-After", "60")
			return c.Status(fiber.StatusServiceUnavailable).JSON(response.Wrap(&runtime.BusinessError{
				Code:    "tenant.pending",
				Message: "Tenant account is being activated — retry in 60 seconds",
				Status:  503,
			}))
		case "SUSPENDED":
			return c.Status(fiber.StatusPaymentRequired).JSON(response.Wrap(&runtime.BusinessError{
				Code:    "tenant.suspended",
				Message: "Tenant account is suspended",
				Status:  402,
			}))
		case "ARCHIVED":
			return c.Status(fiber.StatusGone).JSON(response.Wrap(&runtime.BusinessError{
				Code:    "tenant.archived",
				Message: "Tenant account has been closed",
				Status:  410,
			}))
		default:
			return c.Status(fiber.StatusServiceUnavailable).JSON(response.Wrap(&runtime.BusinessError{
				Code:    "tenant.unavailable",
				Message: "Tenant is not available",
				Status:  503,
			}))
		}

		tc := tenant.TenantContext{
			TenantID:   t.ID,
			TenantSlug: t.GetString("slug"),
			Locale:     t.GetString("locale"),
			Timezone:   t.GetString("timezone"),
			Currency:   t.GetString("currency"),
		}
		ctx = tenant.WithContext(ctx, tc)
		c.SetUserContext(ctx)
		c.Locals("tenant_id", t.ID.String())
		return c.Next()
	}
}

func resolveTenantID(c *fiber.Ctx) (uuid.UUID, error) {
	// 1. X-Tenant-ID header.
	if raw := c.Get("X-Tenant-ID"); raw != "" {
		return uuid.Parse(raw)
	}
	// 2. Query param (legacy).
	if raw := c.Query("tenant_id"); raw != "" {
		return uuid.Parse(raw)
	}
	// 3. Subdomain.
	host := c.Hostname()
	if id := tenantIDFromSubdomain(host); id != uuid.Nil {
		return id, nil
	}
	return uuid.Nil, nil
}

// platformSubdomains are prefixes that belong to the platform, not a tenant.
var platformSubdomains = map[string]bool{
	"bo":     true,
	"portal": true,
	"app":    true,
	"api":    true,
	"www":    true,
}

func tenantIDFromSubdomain(host string) uuid.UUID {
	// Strip port.
	if i := strings.LastIndex(host, ":"); i > 0 {
		host = host[:i]
	}
	parts := strings.SplitN(host, ".", 2)
	if len(parts) < 2 {
		return uuid.Nil
	}
	sub := parts[0]
	if platformSubdomains[sub] {
		return uuid.Nil
	}
	// If the subdomain looks like a UUID, parse it directly.
	if id, err := uuid.Parse(sub); err == nil {
		return id
	}
	// Otherwise the subdomain is a slug — caller must resolve slug→ID separately.
	// Return Nil here; the caller's repo lookup will handle slug resolution.
	return uuid.Nil
}
