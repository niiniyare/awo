// Package authz provides the Fiber middleware bridge between the HTTP layer
// and the Awo authorization subsystem (ADR-001, ADR-002).
//
// Enforcement architecture:
//
//  1. Session middleware populates a [auth.ViewerContext] into the request
//     context via [auth.WithViewer].
//  2. [RequirePermission] reads the viewer with [auth.ViewerFromContext].
//  3. Platform admins (viewer.IsPlatformAdmin() == true) bypass Casbin entirely.
//  4. All other requests are evaluated by [auth.PolicyEvaluator.CanPerform]:
//     (true, nil) → next handler; (false, nil) → 403; (_, err) → 500.
//
// This package contains no policy data and no Casbin state.
// Policy loading lives in [awo/auth.NewCasbinEvaluator].
// Permission identifiers live in EntityDefinition.Permissions (PermissionSet).
package authz

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"

	"awo.so/awo/api/response"
	"awo.so/awo/auth"
	"awo.so/awo/runtime"
)

// RequirePermission returns a Fiber middleware that enforces RBAC for the
// named entity and action.
//
// entityName is the qualified entity name (e.g. "finance_invoice").
// action is one of "create", "read", "update", "delete", or a custom action
// name declared in ActionDef.Name (e.g. "submit", "approve").
//
// The middleware expects [auth.ViewerContext] to be present in the request
// context — set by the session validation middleware via [auth.WithViewer].
// If the viewer is absent the request panics (fail-fast; session middleware
// must always run before this middleware).
//
// Platform admins bypass Casbin entirely and are always allowed through.
func RequirePermission(eval auth.PolicyEvaluator, entityName, action string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		viewer := auth.ViewerFromContext(c.UserContext())

		// Platform admins bypass all Casbin checks.
		// Log every bypass for audit trail — platform admin access is
		// privileged and must be detectable in log analysis.
		if viewer.IsPlatformAdmin() {
			slog.InfoContext(c.UserContext(), "authz: platform-admin bypass",
				"entity", entityName,
				"action", action,
				"user_id", viewer.UserID().String(),
				"tenant_id", viewer.TenantID().String(),
				"path", c.Path(),
				"method", c.Method(),
			)
			return c.Next()
		}

		ok, err := eval.CanPerform(c.UserContext(), viewer, entityName, action)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(response.Wrap(err))
		}
		if !ok {
			return c.Status(fiber.StatusForbidden).JSON(response.Wrap(&runtime.PermissionError{
				EntityName: entityName,
				Action:     action,
			}))
		}
		return c.Next()
	}
}
