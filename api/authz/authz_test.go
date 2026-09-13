package authz_test

import (
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/api/authz"
	"awo.so/awo/auth"
	"awo.so/awo/def"
)

// ── stubs ─────────────────────────────────────────────────────────────────────

type stubViewer struct {
	isPlatformAdmin bool
	roles           []string
}

func (v *stubViewer) TenantID() uuid.UUID         { return uuid.New() }
func (v *stubViewer) UserID() uuid.UUID           { return uuid.New() }
func (v *stubViewer) ServiceAccountID() uuid.UUID { return uuid.Nil }
func (v *stubViewer) Roles() []string             { return v.roles }
func (v *stubViewer) HasRole(r string) bool {
	for _, role := range v.roles {
		if role == r {
			return true
		}
	}
	return false
}
func (v *stubViewer) IsPlatformAdmin() bool { return v.isPlatformAdmin }
func (v *stubViewer) Actor() *def.Actor {
	return &def.Actor{Roles: append([]string(nil), v.roles...)}
}

type stubEvaluator struct {
	allow bool
	err   error
}

func (e *stubEvaluator) CanPerform(_ context.Context, _ auth.ViewerContext, _, _ string) (bool, error) {
	return e.allow, e.err
}

// ── helpers ───────────────────────────────────────────────────────────────────

func appWithMiddleware(eval auth.PolicyEvaluator, viewer auth.ViewerContext) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: func(c *fiber.Ctx, err error) error {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}})
	app.Get("/", func(c *fiber.Ctx) error {
		ctx := auth.WithViewer(c.UserContext(), viewer)
		c.SetUserContext(ctx)
		return c.Next()
	}, authz.RequirePermission(eval, "finance_invoice", "read"), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	return app
}

func doGet(t *testing.T, app *fiber.App) int {
	t.Helper()
	req := httptest.NewRequest("GET", "/", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resp.StatusCode
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestRequirePermission_PlatformAdminBypass(t *testing.T) {
	viewer := &stubViewer{isPlatformAdmin: true}
	eval := &stubEvaluator{allow: false} // evaluator would deny — bypass takes precedence
	app := appWithMiddleware(eval, viewer)
	assert.Equal(t, 200, doGet(t, app))
}

func TestRequirePermission_Allowed(t *testing.T) {
	viewer := &stubViewer{roles: []string{"role:finance.viewer"}}
	eval := &stubEvaluator{allow: true}
	app := appWithMiddleware(eval, viewer)
	assert.Equal(t, 200, doGet(t, app))
}

func TestRequirePermission_Denied(t *testing.T) {
	viewer := &stubViewer{roles: []string{"role:finance.viewer"}}
	eval := &stubEvaluator{allow: false}
	app := appWithMiddleware(eval, viewer)
	assert.Equal(t, 403, doGet(t, app))
}

func TestRequirePermission_EvaluatorError(t *testing.T) {
	viewer := &stubViewer{roles: []string{"role:finance.viewer"}}
	eval := &stubEvaluator{err: errors.New("casbin internal error")}
	app := appWithMiddleware(eval, viewer)
	assert.Equal(t, 500, doGet(t, app))
}
