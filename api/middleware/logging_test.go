package middleware_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"awo.so/awo/api/middleware"
)

// captureLoggedStatus installs a slog handler that writes JSON lines into buf
// for the duration of fn, then restores the previous default logger. Returns
// the "status" field of the single logged "request" record fn's request
// produced.
func captureLoggedStatus(t *testing.T, fn func()) int {
	t.Helper()

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	fn()

	var loggedStatus int
	found := false
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &rec))
		if rec["msg"] != "request" {
			continue
		}
		s, ok := rec["status"].(float64)
		require.True(t, ok, "request log record missing numeric status field: %v", rec)
		loggedStatus = int(s)
		found = true
	}
	require.True(t, found, "no \"request\" log record captured; buf=%s", buf.String())
	return loggedStatus
}

// R.12 — contrib/pgx's generic scanner defect (R.6) is unrelated; this covers
// a separate, confirmed defect in api/middleware/logging.go's Logger(): the
// logged "status" field did not match the real HTTP response status whenever
// a handler returned an error instead of calling c.Status() itself first.
// Two real, live-confirmed cases take this path: Fiber's own "no route
// matched at all" fallback, and any Awo handler (api/sdui/handler.go,
// api/meta/handler.go) that returns a *fiber.Error directly. See
// docs/00-overview/TECHNICAL_FINDINGS_AND_FUTURE_IMPROVEMENTS.md §B.12.

func TestLogger_UnmatchedRoute_LoggedStatusMatchesRealResponse(t *testing.T) {
	app := fiber.New()
	app.Use(middleware.Logger())
	// No routes registered at all - every request is genuinely unmatched.

	var realStatus int
	loggedStatus := captureLoggedStatus(t, func() {
		req := httptest.NewRequest("GET", "/totally-unmatched-path", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		realStatus = resp.StatusCode
	})

	require.Equal(t, 404, realStatus, "sanity check: Fiber's own fallback must be a 404")
	require.Equal(t, realStatus, loggedStatus,
		"logged status must match the real response status for an unmatched route")
}

func TestLogger_MatchedRouteHandlerReturnsErrorDirectly_LoggedStatusMatchesRealResponse(t *testing.T) {
	app := fiber.New()
	app.Use(middleware.Logger())
	// Mirrors api/sdui/handler.go's and api/meta/handler.go's real pattern:
	// a matched route whose handler returns a *fiber.Error directly, never
	// calling c.Status() itself.
	app.Get("/business/does-not-exist", func(c *fiber.Ctx) error {
		return fiber.NewError(fiber.StatusNotFound, "entity not found: does-not-exist")
	})

	var realStatus int
	loggedStatus := captureLoggedStatus(t, func() {
		req := httptest.NewRequest("GET", "/business/does-not-exist", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		realStatus = resp.StatusCode
	})

	require.Equal(t, 404, realStatus, "sanity check: the handler's *fiber.Error carries Code 404")
	require.Equal(t, realStatus, loggedStatus,
		"logged status must match the real response status for a matched route whose handler returns an error")
}

func TestLogger_MatchedRouteSucceeds_LoggedStatusMatchesRealResponse(t *testing.T) {
	app := fiber.New()
	app.Use(middleware.Logger())
	// Mirrors Awo's own CRUD handler pattern: explicit c.Status() + nil error.
	app.Get("/ok", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(map[string]string{"status": "ok"})
	})

	var realStatus int
	loggedStatus := captureLoggedStatus(t, func() {
		req := httptest.NewRequest("GET", "/ok", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		realStatus = resp.StatusCode
	})

	require.Equal(t, 200, realStatus)
	require.Equal(t, realStatus, loggedStatus,
		"logged status must match the real response status for an ordinary successful route (control case)")
}
