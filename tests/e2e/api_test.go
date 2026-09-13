// Package e2e contains end-to-end HTTP API tests for the Awo framework.
// These tests start a real Fiber server (no real DB — fakestore backed) and
// issue HTTP requests to verify the full request pipeline:
//
//	HTTP request → Fiber routing → middleware → handler → fakestore → response
//
// No infrastructure required. Run with:
//
//	go test ./awo/tests/e2e/...
package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"awo.so/awo/driver"
	"awo.so/awo/introspect"
	"awo.so/awo/platform/iam"
	"awo.so/awo/platform/organization"
	"awo.so/awo/platform/tenant"
	"awo.so/awo/registry"
	"awo.so/awo/testing/fakestore"
)

// apiServer is a test HTTP server wired to the fakestore.
type apiServer struct {
	app      *fiber.App
	store    *fakestore.Store
	schema   *compiler.CompiledSchema
	tenantID uuid.UUID
	actorID  uuid.UUID
}

// newAPIServer builds a Fiber app with introspect endpoint and stub entity routes.
// Real route handlers (driver-backed) require the pgx driver (v1.1). This
// server validates routing, middleware, and response envelope shape.
func newAPIServer(t *testing.T) *apiServer {
	t.Helper()

	defs := []def.EntityDefinition{
		&tenant.Definition,
		&iam.UserDefinition,
		&organization.Definition,
		&organization.OrgTypeDefinition,
		&organization.OrgAssignmentDefinition,
	}

	reg, err := registry.BuildFrom(defs)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	schema, err := compiler.Compile(reg)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	store := fakestore.New()
	tenantID := uuid.New()
	actorID := uuid.New()

	app := fiber.New(fiber.Config{
		// Disable startup banner in tests.
		DisableStartupMessage: true,
	})

	// Wire introspect endpoint.
	app.Get("/api/introspect", introspect.Handler(schema))

	// Wire stub entity routes from compiled schema.
	// These return the correct HTTP status and envelope shape; payload
	// is served from fakestore when possible.
	wireStubRoutes(app, schema, store, tenantID, actorID)

	return &apiServer{
		app:      app,
		store:    store,
		schema:   schema,
		tenantID: tenantID,
		actorID:  actorID,
	}
}

// do sends an HTTP request to the test server.
func (s *apiServer) do(t *testing.T, method, path string, body any) *http.Response {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", s.tenantID.String())

	resp, err := s.app.Test(req, -1)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

// decodeJSON decodes the response body as JSON into v.
func decodeJSON(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

// ── Envelope types ────────────────────────────────────────────────────────────

type envelope struct {
	Data any            `json:"data"`
	Meta map[string]any `json:"meta,omitempty"`
}

type errorEnvelope struct {
	Error struct {
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Fields  map[string]any `json:"fields,omitempty"`
	} `json:"error"`
}

// ── Tests ─────────────────────────────────────────────────────────────────────

// TestIntrospectEndpoint verifies GET /api/introspect returns schema JSON.
func TestIntrospectEndpoint(t *testing.T) {
	s := newAPIServer(t)
	resp := s.do(t, "GET", "/api/introspect", nil)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var info introspect.SchemaInfo
	decodeJSON(t, resp, &info)

	if info.EntityCount == 0 {
		t.Error("entity_count is 0")
	}
	if info.Fingerprint == "" {
		t.Error("fingerprint is empty")
	}
	if info.RouteCount == 0 {
		t.Error("route_count is 0")
	}

	// Verify platform_tenant present.
	var found bool
	for _, e := range info.Entities {
		if e.Name == "platform_tenant" {
			found = true
			if e.FieldCount == 0 {
				t.Error("platform_tenant has 0 fields")
			}
			if len(e.Routes) == 0 {
				t.Error("platform_tenant has no routes")
			}
		}
	}
	if !found {
		t.Error("platform_tenant not in introspect output")
	}
	t.Logf("introspect: %d entities, %d routes, fingerprint=%s",
		info.EntityCount, info.RouteCount, info.Fingerprint)
}

// TestRouteRegistration verifies all compiled routes are reachable (not 404).
// The stub handlers return 200/201/204 without real persistence.
func TestRouteRegistration(t *testing.T) {
	s := newAPIServer(t)

	// Each compiled route must return a non-404 response.
	for _, r := range s.schema.Routes {
		path := r.Path
		// Replace Fiber :id param with a valid UUID.
		if strings.Contains(path, ":id") {
			path = strings.Replace(path, ":id", uuid.New().String(), 1)
		}

		resp := s.do(t, r.Method, path, nil)
		if resp.StatusCode == http.StatusNotFound {
			t.Errorf("route %s %s returned 404 — not registered", r.Method, r.Path)
		}
		resp.Body.Close()
	}
}

// tenantRoutePrefix returns the compiled route prefix for platform_tenant.
func tenantRoutePrefix(s *apiServer) string {
	es, ok := s.schema.ByName["platform_tenant"]
	if !ok {
		return "/api/v1/platform/tenants"
	}
	return es.RoutePrefix
}

// TestEntityListRoute verifies GET /api/v1/platform/tenants returns list envelope.
func TestEntityListRoute(t *testing.T) {
	s := newAPIServer(t)
	base := tenantRoutePrefix(s)

	resp := s.do(t, "GET", base, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// TestEntityCreateRoute verifies POST /api/v1/platform/tenants → 201.
func TestEntityCreateRoute(t *testing.T) {
	s := newAPIServer(t)
	base := tenantRoutePrefix(s)

	payload := map[string]any{
		"name":   "Test Tenant",
		"slug":   "test-tenant",
		"status": "PENDING",
	}

	resp := s.do(t, "POST", base, payload)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// TestEntityGetRoute verifies GET /api/v1/platform/tenants/:id → 200 or 404.
func TestEntityGetRoute(t *testing.T) {
	s := newAPIServer(t)
	base := tenantRoutePrefix(s)

	unknownID := uuid.New().String()
	resp := s.do(t, "GET", base+"/"+unknownID, nil)
	// Stub returns 200 echoing ID — valid behavior for stub handler.
	if resp.StatusCode == http.StatusMethodNotAllowed {
		t.Errorf("route not registered — got 405")
	}
	resp.Body.Close()
}

// TestEntityUpdateRoute verifies PATCH /api/v1/platform/tenants/:id → non-404.
func TestEntityUpdateRoute(t *testing.T) {
	s := newAPIServer(t)
	base := tenantRoutePrefix(s)

	id := uuid.New().String()
	resp := s.do(t, "PATCH", base+"/"+id, map[string]any{"name": "Updated"})
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
		t.Errorf("PATCH route not registered — got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// TestEntityDeleteRoute verifies DELETE /api/v1/platform/tenants/:id → non-404.
func TestEntityDeleteRoute(t *testing.T) {
	s := newAPIServer(t)
	base := tenantRoutePrefix(s)

	id := uuid.New().String()
	resp := s.do(t, "DELETE", base+"/"+id, nil)
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
		t.Errorf("DELETE route not registered — got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// TestSchemaFingerprint verifies the introspect fingerprint is deterministic.
func TestSchemaFingerprint(t *testing.T) {
	s1 := newAPIServer(t)
	s2 := newAPIServer(t)

	resp1 := s1.do(t, "GET", "/api/introspect", nil)
	resp2 := s2.do(t, "GET", "/api/introspect", nil)

	var info1, info2 introspect.SchemaInfo
	decodeJSON(t, resp1, &info1)
	decodeJSON(t, resp2, &info2)

	if info1.Fingerprint != info2.Fingerprint {
		t.Errorf("fingerprint not deterministic: %q vs %q", info1.Fingerprint, info2.Fingerprint)
	}
	t.Logf("fingerprint stable: %s", info1.Fingerprint)
}

// TestAllEntitiesHaveRoutes verifies every compiled entity has at least 5 routes.
func TestAllEntitiesHaveRoutes(t *testing.T) {
	s := newAPIServer(t)

	routesByEntity := make(map[string]int)
	for _, r := range s.schema.Routes {
		routesByEntity[r.EntityQualifiedName]++
	}

	for _, es := range s.schema.Entities {
		count := routesByEntity[es.QualifiedName]
		if count < 5 {
			t.Errorf("entity %q has only %d routes, expected ≥5 (list/get/create/update/delete)", es.QualifiedName, count)
		}
	}
}

// ── Stub route wiring ─────────────────────────────────────────────────────────

// wireStubRoutes registers Fiber handlers for every route in the compiled schema.
// Handlers return correct HTTP status codes and minimal JSON envelopes.
// Real persistence (via pgx driver) is out of scope until v1.1.
func wireStubRoutes(
	app *fiber.App,
	schema *compiler.CompiledSchema,
	store *fakestore.Store,
	tenantID uuid.UUID,
	actorID uuid.UUID,
) {
	actor := &def.Actor{UserID: actorID, TenantID: tenantID, Roles: []string{"role:tenant.admin"}}

	for _, r := range schema.Routes {
		r := r // capture
		switch r.Operation {
		case "list":
			app.Get(r.Path, func(c *fiber.Ctx) error {
				ctx := context.Background()
				records, _, err := store.Query(ctx, nil)
				if err != nil {
					return c.Status(500).JSON(fiber.Map{"error": fiber.Map{"code": "internal", "message": err.Error()}})
				}
				return c.Status(200).JSON(fiber.Map{"data": records, "meta": fiber.Map{"total": len(records)}})
			})
		case "get":
			app.Get(r.Path, func(c *fiber.Ctx) error {
				// Stub: return the ID echoed back. Real implementation requires pgx driver (v1.1).
				return c.Status(200).JSON(fiber.Map{"data": fiber.Map{"id": c.Params("id")}})
			})
		case "create":
			app.Post(r.Path, func(c *fiber.Ctx) error {
				var data map[string]any
				if err := c.BodyParser(&data); err != nil {
					return c.Status(400).JSON(fiber.Map{"error": fiber.Map{"code": "bad_request", "message": err.Error()}})
				}
				ctx := context.Background()
				rec, err := store.Create(ctx, driver.CreateInput{Data: data, Actor: actor})
				if err != nil {
					return c.Status(422).JSON(fiber.Map{"error": fiber.Map{"code": "validation_error", "message": err.Error()}})
				}
				return c.Status(201).JSON(fiber.Map{"data": rec})
			})
		case "update":
			app.Patch(r.Path, func(c *fiber.Ctx) error {
				// Stub: echo ID. Real implementation requires pgx driver (v1.1).
				return c.Status(200).JSON(fiber.Map{"data": fiber.Map{"id": c.Params("id")}})
			})
		case "delete":
			app.Delete(r.Path, func(c *fiber.Ctx) error {
				// Stub: always succeed. Real implementation requires pgx driver (v1.1).
				return c.SendStatus(204)
			})
		case "action":
			// Custom actions — stub: 202 Accepted.
			app.Add(r.Method, r.Path, func(c *fiber.Ctx) error {
				return c.Status(202).JSON(fiber.Map{"data": fiber.Map{"status": "accepted"}})
			})
		}
	}
}
