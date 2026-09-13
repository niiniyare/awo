package meta_test

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"awo.so/awo/api/meta"
	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"awo.so/awo/registry"
)

// ── test helpers ──────────────────────────────────────────────────────────────

func buildTestSchema(t *testing.T, entities ...def.EntityDefinition) *compiler.CompiledSchema {
	t.Helper()
	reg, err := registry.BuildFrom(entities)
	if err != nil {
		t.Fatalf("BuildFrom: %v", err)
	}
	schema, err := compiler.Compile(reg)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	return schema
}

func newTestApp(t *testing.T, entities ...def.EntityDefinition) *fiber.App {
	t.Helper()
	schema := buildTestSchema(t, entities...)
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	h := meta.New(schema)
	h.Register(app.Group("/api/v1/meta"))
	return app
}

func get(t *testing.T, app *fiber.App, path string) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body
}

func testEntity() def.EntityDefinition {
	return &def.SystemDefinition{
		Name:        "invoice",
		Module:      "finance",
		Label:       "Invoice",
		LabelPlural: "Invoices",
		Description: "Customer invoice.",
		Fields: []def.FieldDef{
			{Name: "number", Type: def.FieldTypeData, Label: "Number", Required: true},
			{Name: "status", Type: def.FieldTypeSelect, Label: "Status",
				Options: []string{"draft", "submitted"}, Required: true},
			{Name: "secret_key", Type: def.FieldTypeData, Label: "Secret", Sensitive: true},
		},
		Permissions: def.PermissionSet{
			Create: []string{"finance.invoice.create"},
			Read:   []string{"finance.invoice.read"},
			Write:  []string{"finance.invoice.update"},
			Delete: []string{"finance.invoice.delete"},
		},
		Actions: []def.ActionDef{
			{
				Name:        "submit",
				Label:       "Submit",
				HandlerFunc: func(_ *def.ActionContext) (*def.ActionResult, error) { return nil, nil },
			},
		},
	}
}

// ── GET /api/v1/meta/entities ─────────────────────────────────────────────────

func TestListEntities_Status200(t *testing.T) {
	app := newTestApp(t, testEntity())
	status, _ := get(t, app, "/api/v1/meta/entities")
	if status != 200 {
		t.Errorf("status: got %d, want 200", status)
	}
}

func TestListEntities_ReturnsArray(t *testing.T) {
	app := newTestApp(t, testEntity())
	_, body := get(t, app, "/api/v1/meta/entities")
	var items []map[string]any
	if err := json.Unmarshal(body, &items); err != nil {
		t.Fatalf("unmarshal: %v\nbody: %s", err, body)
	}
	if len(items) == 0 {
		t.Error("expected at least one entity in list")
	}
}

func TestListEntities_ContainsEntityName(t *testing.T) {
	app := newTestApp(t, testEntity())
	_, body := get(t, app, "/api/v1/meta/entities")
	var items []map[string]any
	_ = json.Unmarshal(body, &items)
	found := false
	for _, item := range items {
		if item["name"] == "finance_invoice" {
			found = true
			break
		}
	}
	if !found {
		t.Error("finance_invoice not found in entity list")
	}
}

func TestListEntities_SummaryHasExpectedFields(t *testing.T) {
	app := newTestApp(t, testEntity())
	_, body := get(t, app, "/api/v1/meta/entities")
	var items []map[string]any
	_ = json.Unmarshal(body, &items)
	if len(items) == 0 {
		t.Fatal("empty list")
	}
	item := items[0]
	for _, key := range []string{"name", "module", "label", "is_system", "field_count", "route_prefix"} {
		if _, ok := item[key]; !ok {
			t.Errorf("summary missing field %q", key)
		}
	}
}

// ── GET /api/v1/meta/entities/:name ──────────────────────────────────────────

func TestGetEntity_Status200(t *testing.T) {
	app := newTestApp(t, testEntity())
	status, _ := get(t, app, "/api/v1/meta/entities/finance_invoice")
	if status != 200 {
		t.Errorf("status: got %d, want 200", status)
	}
}

func TestGetEntity_NotFound_Returns404(t *testing.T) {
	app := newTestApp(t, testEntity())
	status, _ := get(t, app, "/api/v1/meta/entities/no_such_entity")
	if status != 404 {
		t.Errorf("status: got %d, want 404", status)
	}
}

func TestGetEntity_ReturnsSchema(t *testing.T) {
	app := newTestApp(t, testEntity())
	_, body := get(t, app, "/api/v1/meta/entities/finance_invoice")
	var detail map[string]any
	if err := json.Unmarshal(body, &detail); err != nil {
		t.Fatalf("unmarshal: %v\nbody: %s", err, body)
	}
	if detail["name"] != "finance_invoice" {
		t.Errorf("name: got %q, want finance_invoice", detail["name"])
	}
	if detail["label"] != "Invoice" {
		t.Errorf("label: got %q", detail["label"])
	}
}

func TestGetEntity_FieldsPresent(t *testing.T) {
	app := newTestApp(t, testEntity())
	_, body := get(t, app, "/api/v1/meta/entities/finance_invoice")
	var detail map[string]any
	_ = json.Unmarshal(body, &detail)
	fields, ok := detail["fields"].([]any)
	if !ok || len(fields) == 0 {
		t.Error("fields array missing or empty")
	}
}

func TestGetEntity_ActionsPresent(t *testing.T) {
	app := newTestApp(t, testEntity())
	_, body := get(t, app, "/api/v1/meta/entities/finance_invoice")
	var detail map[string]any
	_ = json.Unmarshal(body, &detail)
	actions, ok := detail["actions"].([]any)
	if !ok || len(actions) == 0 {
		t.Error("actions array missing or empty")
	}
	first := actions[0].(map[string]any)
	if first["name"] != "submit" {
		t.Errorf("action name: got %q, want submit", first["name"])
	}
}

func TestGetEntity_PermissionsPresent(t *testing.T) {
	app := newTestApp(t, testEntity())
	_, body := get(t, app, "/api/v1/meta/entities/finance_invoice")
	var detail map[string]any
	_ = json.Unmarshal(body, &detail)
	perms, ok := detail["permissions"].(map[string]any)
	if !ok {
		t.Fatal("permissions field missing")
	}
	if _, ok := perms["read"]; !ok {
		t.Error("permissions.read missing")
	}
}

// ── GET /api/v1/meta/permissions ─────────────────────────────────────────────

func TestListPermissions_Status200(t *testing.T) {
	app := newTestApp(t, testEntity())
	status, _ := get(t, app, "/api/v1/meta/permissions")
	if status != 200 {
		t.Errorf("status: got %d, want 200", status)
	}
}

func TestListPermissions_ReturnsArray(t *testing.T) {
	app := newTestApp(t, testEntity())
	_, body := get(t, app, "/api/v1/meta/permissions")
	var items []map[string]any
	if err := json.Unmarshal(body, &items); err != nil {
		t.Fatalf("unmarshal: %v\nbody: %s", err, body)
	}
	if len(items) == 0 {
		t.Error("expected at least one permission entry")
	}
}

func TestListPermissions_EntryHasPermissionAndEntityFields(t *testing.T) {
	app := newTestApp(t, testEntity())
	_, body := get(t, app, "/api/v1/meta/permissions")
	var items []map[string]any
	_ = json.Unmarshal(body, &items)
	if len(items) == 0 {
		t.Fatal("empty permissions list")
	}
	entry := items[0]
	for _, key := range []string{"permission", "entity", "action"} {
		if _, ok := entry[key]; !ok {
			t.Errorf("permission entry missing field %q", key)
		}
	}
}

func TestListPermissions_ContainsInvoiceReadPermission(t *testing.T) {
	app := newTestApp(t, testEntity())
	_, body := get(t, app, "/api/v1/meta/permissions")
	var items []map[string]any
	_ = json.Unmarshal(body, &items)
	found := false
	for _, item := range items {
		if item["permission"] == "finance.invoice.read" {
			found = true
			break
		}
	}
	if !found {
		t.Error("finance.invoice.read permission not found in list")
	}
}
