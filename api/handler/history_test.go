package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/api/handler"
	"awo.so/awo/audit"
	"awo.so/awo/compiler"
)

// --- stub queryer ---

// stubQueryer is a test double for audit.Queryer.
type stubQueryer struct {
	entries []audit.HistoryEntry
	err     error
	// captured args for assertion
	capturedLimit int
}

func (s *stubQueryer) History(_ context.Context, _ string, _ uuid.UUID, limit int) ([]audit.HistoryEntry, error) {
	s.capturedLimit = limit
	if s.err != nil {
		return nil, s.err
	}
	return s.entries, nil
}

// --- helpers ---

// auditSchema builds a minimal EntitySchema for test use.
func auditSchema(allowAudit bool) *compiler.EntitySchema {
	return &compiler.EntitySchema{
		QualifiedName: "finance_invoice",
		Module:        "finance",
		APIResource:   "invoices",
		LocalName:     "invoice",
		AllowAudit:    allowAudit,
	}
}

// buildApp mounts handler.Handle at GET /:id/history on a Fiber app.
func buildApp(h *handler.HistoryHandler) *fiber.App {
	app := fiber.New(fiber.Config{
		// Disable Fiber's default error handler to let us assert raw JSON.
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
		},
	})
	app.Get("/:id/history", h.Handle)
	return app
}

func doRequest(app *fiber.App, path string) (*http.Response, []byte) {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		panic(fmt.Sprintf("test request failed: %v", err))
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, body
}

// --- tests ---

// TestHistoryHandler_ReturnsEntries verifies that a stub queryer returning two
// entries produces a 200 with a JSON array of those entries.
func TestHistoryHandler_ReturnsEntries(t *testing.T) {
	id1 := uuid.New()
	actor := uuid.New()
	entries := []audit.HistoryEntry{
		{
			ID:         id1,
			ActorID:    &actor,
			Action:     "create",
			OccurredAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			ID:         uuid.New(),
			Action:     "update",
			OccurredAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		},
	}
	q := &stubQueryer{entries: entries}
	h := handler.NewHistoryHandler(auditSchema(true), q)
	app := buildApp(h)

	recordID := uuid.New()
	resp, body := doRequest(app, "/"+recordID.String()+"/history")

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	assert.Len(t, envelope.Data, 2)
	assert.Equal(t, "create", envelope.Data[0]["action"])
	assert.Equal(t, "update", envelope.Data[1]["action"])
}

// TestHistoryHandler_AllowAuditFalse_Returns404 verifies that an entity with
// AllowAudit:false returns 404 — history is not available for that entity.
func TestHistoryHandler_AllowAuditFalse_Returns404(t *testing.T) {
	q := &stubQueryer{}
	h := handler.NewHistoryHandler(auditSchema(false), q)
	app := buildApp(h)

	resp, body := doRequest(app, "/"+uuid.New().String()+"/history")

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	assert.Equal(t, "history.not_available", envelope.Error.Code)
}

// TestHistoryHandler_InvalidID_Returns400 verifies that a malformed UUID in
// the :id path parameter produces a 400 response.
func TestHistoryHandler_InvalidID_Returns400(t *testing.T) {
	q := &stubQueryer{}
	h := handler.NewHistoryHandler(auditSchema(true), q)
	app := buildApp(h)

	resp, body := doRequest(app, "/not-a-uuid/history")

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var envelope struct {
		Error struct {
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	assert.Contains(t, envelope.Error.Fields, "id")
}

// TestHistoryHandler_LimitParam verifies that ?limit=5 is forwarded to the
// queryer as limit=5.
func TestHistoryHandler_LimitParam(t *testing.T) {
	q := &stubQueryer{entries: []audit.HistoryEntry{}}
	h := handler.NewHistoryHandler(auditSchema(true), q)
	app := buildApp(h)

	doRequest(app, "/"+uuid.New().String()+"/history?limit=5")

	assert.Equal(t, 5, q.capturedLimit)
}

// TestHistoryHandler_DefaultLimit verifies that when no ?limit param is
// provided, the default limit (100) is forwarded to the queryer.
func TestHistoryHandler_DefaultLimit(t *testing.T) {
	q := &stubQueryer{entries: nil}
	h := handler.NewHistoryHandler(auditSchema(true), q)
	app := buildApp(h)

	doRequest(app, "/"+uuid.New().String()+"/history")

	assert.Equal(t, 100, q.capturedLimit)
}

// TestHistoryHandler_MaxLimitCapped verifies that ?limit=9999 is capped at
// 500 — the server-enforced maximum.
func TestHistoryHandler_MaxLimitCapped(t *testing.T) {
	q := &stubQueryer{entries: nil}
	h := handler.NewHistoryHandler(auditSchema(true), q)
	app := buildApp(h)

	doRequest(app, "/"+uuid.New().String()+"/history?limit=9999")

	assert.Equal(t, 500, q.capturedLimit)
}

// TestHistoryHandler_EmptyResult_ReturnsArray verifies that nil entries from
// the queryer are converted to an empty JSON array (not null).
func TestHistoryHandler_EmptyResult_ReturnsArray(t *testing.T) {
	q := &stubQueryer{entries: nil} // queryer returns nil
	h := handler.NewHistoryHandler(auditSchema(true), q)
	app := buildApp(h)

	resp, body := doRequest(app, "/"+uuid.New().String()+"/history")

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	assert.NotNil(t, envelope.Data, "data must be [] not null")
	assert.Len(t, envelope.Data, 0)
}

// TestHistoryHandler_InvalidLimitParam_Returns400 verifies that a non-numeric
// ?limit value produces a 400.
func TestHistoryHandler_InvalidLimitParam_Returns400(t *testing.T) {
	q := &stubQueryer{}
	h := handler.NewHistoryHandler(auditSchema(true), q)
	app := buildApp(h)

	resp, _ := doRequest(app, "/"+uuid.New().String()+"/history?limit=abc")

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestHistoryHandler_NilQueryer_UsesNoop verifies that passing nil as the
// queryer falls back to NoopQueryer (no panic, returns empty array).
func TestHistoryHandler_NilQueryer_UsesNoop(t *testing.T) {
	h := handler.NewHistoryHandler(auditSchema(true), nil)
	app := buildApp(h)

	resp, _ := doRequest(app, "/"+uuid.New().String()+"/history")

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
