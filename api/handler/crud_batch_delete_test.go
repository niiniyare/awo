package handler_test

// Request-contract tests for EntityHandler.DeleteBatch — the explicit
// {"ids": [...]} JSON body shape, and its validation before anything
// touches EntityService (so these need no database: a nil service never
// gets called for any of the invalid-input cases below, which is itself
// part of what's being verified — an invalid request must fail before
// reaching the delete path at all).

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/api/handler"
	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"awo.so/awo/registry"
)

func batchDeleteTestSchema(t *testing.T) *compiler.EntitySchema {
	t.Helper()
	d := &def.SystemDefinition{
		Name:   "batchdel_probe",
		Module: "test",
		Label:  "Batch Delete Probe",
		Fields: []def.FieldDef{{Name: "status", Type: def.FieldTypeData}},
	}
	reg, err := registry.BuildFrom([]def.EntityDefinition{d})
	require.NoError(t, err)
	schema, err := compiler.Compile(reg)
	require.NoError(t, err)
	es := schema.ByName["test_batchdel_probe"]
	require.NotNil(t, es)
	return es
}

func newBatchDeleteTestApp(t *testing.T) *fiber.App {
	t.Helper()
	// svc/actions are nil: every case tested here must fail validation
	// before the handler ever touches them.
	h := handler.NewEntityHandler(batchDeleteTestSchema(t), nil, nil)
	app := fiber.New()
	// svc is nil above — recover() converts the resulting nil-pointer panic
	// (once request parsing/validation succeeds and the handler tries to
	// actually call svc.WithTx) into a clean 500 instead of crashing the
	// test, so "did validation reject this body" stays distinguishable
	// from "validation passed, something downstream failed".
	app.Use(recover.New())
	app.Delete("/items", h.DeleteBatch)
	return app
}

func TestDeleteBatch_MissingBody_Rejected(t *testing.T) {
	app := newBatchDeleteTestApp(t)
	req := httptest.NewRequest("DELETE", "/items", nil)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
}

func TestDeleteBatch_EmptyIDs_Rejected(t *testing.T) {
	app := newBatchDeleteTestApp(t)
	req := httptest.NewRequest("DELETE", "/items", strings.NewReader(`{"ids":[]}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode, "an empty ids array must be rejected, not treated as a no-op success")
}

func TestDeleteBatch_InvalidUUID_Rejected(t *testing.T) {
	app := newBatchDeleteTestApp(t)
	req := httptest.NewRequest("DELETE", "/items", strings.NewReader(`{"ids":["not-a-uuid"]}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode, "a malformed id must be rejected before any delete is attempted")
}

func TestDeleteBatch_CommaSeparatedString_AcceptedAsIDs(t *testing.T) {
	// AMIS 6.7.0's bulk-action data mapping can send ids as a comma-joined
	// string ("${ids}" template interpolation) rather than a real JSON
	// array ("$ids" bare variable reference) — which one a real browser
	// actually sends could not be confirmed without a browser tool in this
	// environment, so both are accepted. This proves the string form
	// parses into two ids and reaches real UUID validation (not rejected
	// as a malformed request) — svc is nil, so a 500 here means parsing
	// succeeded and the handler got as far as calling Delete.
	app := newBatchDeleteTestApp(t)
	body := `{"ids":"11111111-1111-1111-1111-111111111111,22222222-2222-2222-2222-222222222222"}`
	req := httptest.NewRequest("DELETE", "/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.NotEqual(t, 400, resp.StatusCode, "a comma-separated ids string must parse successfully, not be rejected as malformed")
}

func TestDeleteBatch_JSONArray_AcceptedAsIDs(t *testing.T) {
	app := newBatchDeleteTestApp(t)
	body := `{"ids":["11111111-1111-1111-1111-111111111111"]}`
	req := httptest.NewRequest("DELETE", "/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.NotEqual(t, 400, resp.StatusCode, "a real JSON array of ids must parse successfully")
}

func TestDeleteBatch_TooManyIDs_Rejected(t *testing.T) {
	app := newBatchDeleteTestApp(t)
	ids := make([]string, 201)
	for i := range ids {
		ids[i] = `"` + "00000000-0000-0000-0000-000000000001" + `"`
	}
	body := `{"ids":[` + strings.Join(ids, ",") + `]}`
	req := httptest.NewRequest("DELETE", "/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode, "an unreasonably large batch must be rejected by the sanity bound")
}
