package iam

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"awo.so/awo/auth"
	"awo.so/awo/runtime"
)

func registerRoutes(app *fiber.App, svc *AuthService, loginLimiter fiber.Handler) {
	h := &authHandler{svc: svc}

	ag := app.Group("/api/v1/auth")
	ag.Post("/login", loginLimiter, h.login)
	ag.Post("/logout", h.logout)
	ag.Get("/me", h.me)
}

type authHandler struct {
	svc *AuthService
}

// ── POST /api/v1/auth/login ───────────────────────────────────────────────────

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	TenantID string `json:"tenant_id"`
	DeviceID string `json:"device_id,omitempty"`
}

type loginResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
	UserID    string `json:"user_id"`
	TenantID  string `json:"tenant_id"`
}

func (h *authHandler) login(c *fiber.Ctx) error {
	var req loginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{"code": "invalid_body", "message": "Request body must be valid JSON."},
		})
	}

	if req.Email == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{"code": "validation_error", "message": "email and password are required."},
		})
	}

	tenantID, err := uuid.Parse(req.TenantID)
	if err != nil || tenantID == uuid.Nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{"code": "validation_error", "message": "tenant_id must be a valid UUID."},
		})
	}

	result, err := h.svc.Login(c.UserContext(), LoginInput{
		Email:     req.Email,
		Password:  req.Password,
		TenantID:  tenantID,
		DeviceID:  req.DeviceID,
		IPAddress: c.IP(),
	})
	if err != nil {
		return handleAuthError(c, err)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": loginResponse{
			Token:     result.Token,
			ExpiresAt: result.Session.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
			UserID:    result.Session.UserID.String(),
			TenantID:  result.Session.TenantID.String(),
		},
	})
}

// ── POST /api/v1/auth/logout ──────────────────────────────────────────────────

func (h *authHandler) logout(c *fiber.Ctx) error {
	// Session must exist in context (set by session middleware).
	session, ok := c.Locals("session").(*auth.Session)
	if !ok || session == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": fiber.Map{"code": "unauthenticated", "message": "No active session."},
		})
	}

	if err := h.svc.Logout(c.UserContext(), session); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{"code": "logout_failed", "message": "Failed to revoke session."},
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{"message": "Logged out successfully."},
	})
}

// ── GET /api/v1/auth/me ───────────────────────────────────────────────────────

type meResponse struct {
	UserID    string   `json:"user_id"`
	TenantID  string   `json:"tenant_id"`
	Roles     []string `json:"roles"`
	IssuedAt  string   `json:"issued_at"`
	ExpiresAt string   `json:"expires_at"`
}

func (h *authHandler) me(c *fiber.Ctx) error {
	session, ok := c.Locals("session").(*auth.Session)
	if !ok || session == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": fiber.Map{"code": "unauthenticated", "message": "No active session."},
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": meResponse{
			UserID:    session.UserID.String(),
			TenantID:  session.TenantID.String(),
			Roles:     session.Roles,
			IssuedAt:  session.IssuedAt.Format("2006-01-02T15:04:05Z07:00"),
			ExpiresAt: session.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		},
	})
}

// ── Error mapping ─────────────────────────────────────────────────────────────

func handleAuthError(c *fiber.Ctx, err error) error {
	status := runtime.HTTPStatus(err)
	return c.Status(status).JSON(fiber.Map{
		"error": fiber.Map{
			"code":    errorCode(err),
			"message": err.Error(),
		},
	})
}

func errorCode(err error) string {
	var be *runtime.BusinessError
	if errors.As(err, &be) {
		return be.Code
	}
	return "internal_error"
}
