package middleware

import (
	"context"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"awo.so/awo/api/response"
	"awo.so/awo/audit"
	"awo.so/awo/auth"
	"awo.so/awo/runtime"
)

// SessionValidator is the interface RequireAuth depends on. It is satisfied by
// *iam.AuthService. Exposing it as an interface here keeps the middleware
// package free of a direct import on the iam package, enabling testing with
// a stub and avoiding an import cycle if iam ever imports middleware.
//
// Implementations must distinguish:
//   - missing/expired session (401, code "iam.session.not_found")
//   - infrastructure failure (503, code "iam.service_unavailable")
type SessionValidator interface {
	// ValidateToken looks up the Bearer token in the session store and returns
	// the associated Session. Returns a *runtime.BusinessError on failure.
	ValidateToken(ctx context.Context, token string) (*auth.Session, error)

	// ValidateAPIToken validates a service account API key against tenantID.
	// Returns a synthetic Session carrying the service account identity.
	// Returns a *runtime.BusinessError on failure or if not found.
	ValidateAPIToken(ctx context.Context, rawToken string, tenantID uuid.UUID) (*auth.Session, error)
}

// RequireAuth validates the Bearer token, loads the session from the store,
// builds a ViewerContext, and stores both in Fiber locals for downstream handlers.
//
// auditSigningSecret is the HMAC-SHA256 key used to derive AuditRecord.SessionID.
// Pass the AUDIT_SIGNING_SECRET environment variable value. An empty string
// disables session ID correlation in audit records (acceptable in dev/test).
//
// Authentication strategy (tried in order):
//  1. Human session token — validated via [SessionValidator.ValidateToken].
//  2. Service account API key — tried only when (1) returns 401, using
//     [SessionValidator.ValidateAPIToken] with the X-Tenant-ID header.
//
// Locals set on success:
//   - "session"  (*auth.Session)      — raw session (used by logout/me handlers)
//   - "viewer"   (auth.ViewerContext) — authorization surface for hooks and policies
//
// Redis failure on (1) causes 503 — cannot authenticate without session store.
// Redis failure on (2) also causes 503 — API token cache is mandatory for safety.
func RequireAuth(svc SessionValidator, auditSigningSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := extractBearerToken(c)
		if token == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(response.Wrap(&runtime.BusinessError{
				Code:    "iam.missing_token",
				Message: "Authentication token is required.",
				Status:  401,
			}))
		}

		session, err := svc.ValidateToken(c.UserContext(), token)
		if err != nil {
			// If the session lookup returned 401 (not_found / expired),
			// attempt service account API key validation.
			if be, ok := asBusiness(err); ok && be.Status == 401 {
				session, err = tryAPIToken(c, svc, token)
				if err != nil {
					return c.Status(response.HTTPStatus(err)).JSON(response.Wrap(err))
				}
			} else {
				return c.Status(response.HTTPStatus(err)).JSON(response.Wrap(err))
			}
		}

		// Populate the ephemeral RequestID for log correlation. This field
		// is explicitly excluded from Redis storage (json:"-") — it is set
		// fresh on every request from the X-Request-ID header.
		reqID, _ := c.Locals("request_id").(string)
		session.RequestID = reqID

		viewer := session.ToViewer()

		// Inject audit.RequestContext so pipeline.RunAuditRecord can populate
		// AuditRecord.RequestID, AuditRecord.IPAddress, AuditRecord.SessionID
		// without reaching back into the Fiber context.
		var sessionID string
		if auditSigningSecret != "" {
			sessionID = audit.HashSessionToken(token, auditSigningSecret)
		}
		rc := audit.RequestContext{
			RequestID: reqID,
			IPAddress: c.IP(),
			SessionID: sessionID,
		}

		// Embed viewer and audit context into the Go context propagation chain.
		ctx := auth.WithViewer(c.UserContext(), viewer)
		ctx = audit.WithRequestContext(ctx, rc)
		c.SetUserContext(ctx)

		// Store session and viewer in Fiber locals for handlers that need them directly.
		c.Locals("session", session)
		c.Locals("viewer", viewer)

		return c.Next()
	}
}

// tryAPIToken attempts service account authentication using the raw token
// and the X-Tenant-ID request header.
func tryAPIToken(c *fiber.Ctx, svc SessionValidator, rawToken string) (*auth.Session, error) {
	tenantHeader := c.Get("X-Tenant-ID")
	tenantID, err := uuid.Parse(tenantHeader)
	if err != nil || tenantID == uuid.Nil {
		// No tenant header → cannot validate API token without tenant scope.
		return nil, &runtime.BusinessError{
			Code:    "iam.session.not_found",
			Message: "Invalid or expired authentication token.",
			Status:  401,
		}
	}
	return svc.ValidateAPIToken(c.UserContext(), rawToken, tenantID)
}

func asBusiness(err error) (*runtime.BusinessError, bool) {
	var be *runtime.BusinessError
	if errors.As(err, &be) {
		return be, true
	}
	return nil, false
}

func extractBearerToken(c *fiber.Ctx) string {
	h := c.Get("Authorization")
	if after, ok := strings.CutPrefix(h, "Bearer "); ok {
		return strings.TrimSpace(after)
	}
	return ""
}
