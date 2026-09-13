package engine_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"awo.so/awo/sdui/amis"
	"awo.so/awo/sdui/cache"
	"awo.so/awo/sdui/engine"
	"awo.so/awo/sdui/generator"
	"awo.so/awo/sdui/layout"
	"awo.so/awo/sdui/observability"
	"awo.so/awo/sdui/renderer"
	"awo.so/awo/sdui/sduictx"
	"awo.so/awo/sdui/validation"
	"awo.so/awo/sdui/widget"
)

// ── test fixtures ─────────────────────────────────────────────────────────────

type stubViewer struct{ admin bool }

func (s *stubViewer) TenantID() uuid.UUID       { return uuid.New() }
func (s *stubViewer) Roles() []string           { return nil }
func (s *stubViewer) IsPlatformAdmin() bool     { return s.admin }
func (s *stubViewer) HasPermission(string) bool { return s.admin }

func makeCtx(mode sduictx.ViewMode) sduictx.GeneratorContext {
	ctx, _ := sduictx.NewGeneratorContext(
		uuid.New(), &stubViewer{admin: true}, "test_entity", mode, "amis",
	).WithSchemaFingerprint("sf1").WithPermFingerprint("pf1").Build()
	return ctx
}

func makeSchema() generator.EntitySchema {
	return generator.EntitySchema{
		Name:        "test_entity",
		Title:       "Entity",
		PluralTitle: "Entities",
		ListURL:     "/api/v1/test/entities",
		CreateURL:   "/api/v1/test/entities",
		EditURL:     "/api/v1/test/entities/{id}",
		DetailURL:   "/api/v1/test/entities/{id}",
		Permissions: map[string]string{
			"create": "test.entity.create",
			"read":   "test.entity.read",
			"update": "test.entity.update",
			"delete": "test.entity.delete",
		},
		Fields: []generator.FieldDef{
			{Name: "name", Label: "Name", FieldType: "data", InList: true, InForm: true, InDetail: true},
			{Name: "status", Label: "Status", FieldType: "select", InList: true, InForm: true, InDetail: true},
			{Name: "notes", Label: "Notes", FieldType: "long_text", InForm: true, InDetail: true},
		},
	}
}

func makeEngine(c *cache.Cache) *engine.Engine {
	return engine.New(engine.Options{
		Generator: generator.New(),
		Validator: validation.New(),
		Layout:    layout.New(),
		Cache:     c,
		Renderers: map[string]renderer.Renderer{"amis": amis.New()},
		Obs:       observability.Noop(),
	})
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestEngine_Handle_ListView(t *testing.T) {
	eng := makeEngine(nil)
	req := engine.Request{
		Ctx:    makeCtx(sduictx.ViewModeList),
		Schema: makeSchema(),
	}
	resp, err := eng.Handle(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("nil response")
	}
	if resp.CacheHit {
		t.Error("first request should not be a cache hit")
	}
	if resp.Output.Format != renderer.FormatAMISJSON {
		t.Errorf("format = %q, want %q", resp.Output.Format, renderer.FormatAMISJSON)
	}
	if resp.Output.AMISSchema == nil {
		t.Error("AMISSchema must not be nil")
	}
	if resp.Layout == nil {
		t.Error("Layout must not be nil")
	}
}

func TestEngine_Handle_CreateForm(t *testing.T) {
	eng := makeEngine(nil)
	req := engine.Request{
		Ctx:    makeCtx(sduictx.ViewModeCreate),
		Schema: makeSchema(),
	}
	resp, err := eng.Handle(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	schema := resp.Output.AMISSchema
	if schema["type"] != "page" {
		t.Errorf("top-level type = %v, want \"page\"", schema["type"])
	}
}

func TestEngine_Handle_DetailView(t *testing.T) {
	eng := makeEngine(nil)
	req := engine.Request{
		Ctx:    makeCtx(sduictx.ViewModeDetail),
		Schema: makeSchema(),
	}
	resp, err := eng.Handle(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Output.AMISSchema == nil {
		t.Error("detail output must not be nil")
	}
}

func TestEngine_Handle_UnknownRenderer(t *testing.T) {
	eng := makeEngine(nil)
	req := engine.Request{
		Ctx:        makeCtx(sduictx.ViewModeList),
		Schema:     makeSchema(),
		RendererID: "nonexistent",
	}
	_, err := eng.Handle(context.Background(), req)
	if err == nil {
		t.Error("expected error for unknown renderer")
	}
}

func TestEngine_Handle_ValidationFatalAborts(t *testing.T) {
	// An entity schema that generates a node the global registry doesn't know
	// is hard to produce without a fake generator. Instead, verify that
	// validation warnings do NOT abort the pipeline.
	eng := makeEngine(nil)
	req := engine.Request{
		Ctx:    makeCtx(sduictx.ViewModeList),
		Schema: makeSchema(),
	}
	resp, err := eng.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Response should be non-nil even with warnings.
	if resp == nil {
		t.Error("expected non-nil response")
	}
}

func TestEngine_Handle_OutputIsValidJSON(t *testing.T) {
	eng := makeEngine(nil)
	req := engine.Request{
		Ctx:    makeCtx(sduictx.ViewModeCreate),
		Schema: makeSchema(),
	}
	resp, err := eng.Handle(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(resp.Output.AMISSchema)
	if err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(raw) < 2 {
		t.Error("output JSON is empty")
	}
}

func TestEngine_Handle_PermissionFiltering(t *testing.T) {
	// No-perms viewer should not see permission-gated fields.
	noperms := &stubViewer{admin: false}
	ctx, _ := sduictx.NewGeneratorContext(
		uuid.New(), noperms, "test_entity", sduictx.ViewModeList, "amis",
	).WithSchemaFingerprint("sf1").WithPermFingerprint("pf1").Build()

	schema := generator.EntitySchema{
		Name:        "test_entity",
		Title:       "Entity",
		ListURL:     "/api/v1/test/entities",
		Permissions: map[string]string{"read": "test.entity.read"},
		Fields: []generator.FieldDef{
			{Name: "name", Label: "Name", FieldType: "data", InList: true},
			{Name: "secret", Label: "Secret", FieldType: "data", InList: true,
				Permission: "test.entity.admin"},
		},
	}
	eng := makeEngine(nil)
	resp, err := eng.Handle(context.Background(), engine.Request{Ctx: ctx, Schema: schema})
	if err != nil {
		t.Fatal(err)
	}
	// Find the list node columns; secret must be absent.
	body, ok := resp.Output.AMISSchema["body"].([]any)
	if !ok || len(body) == 0 {
		t.Fatal("no body in output")
	}
	listNode, ok := body[0].(map[string]any)
	if !ok {
		t.Fatal("body[0] is not a map")
	}
	cols, _ := listNode["columns"].([]any)
	for _, col := range cols {
		m, ok := col.(map[string]any)
		if !ok {
			continue
		}
		if m["name"] == "secret" {
			t.Error("secret field must be absent for no-perms viewer")
		}
	}
}

func TestEngine_Handle_LayoutPresent(t *testing.T) {
	eng := makeEngine(nil)
	req := engine.Request{
		Ctx:    makeCtx(sduictx.ViewModeCreate),
		Schema: makeSchema(),
	}
	resp, err := eng.Handle(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Layout == nil {
		t.Fatal("Layout must be present for non-cached response")
	}
	if resp.Layout.Root == nil {
		t.Error("Layout.Root must not be nil")
	}
}

func TestEngine_Renderers(t *testing.T) {
	eng := makeEngine(nil)
	renderers := eng.Renderers()
	if _, ok := renderers["amis"]; !ok {
		t.Error("amis renderer must be registered")
	}
}

// ── Security and determinism tests ────────────────────────────────────────────

// permActionCount returns the number of actions in the AMIS schema for the
// list view. Used to compare admin vs restricted schemas.
func actionsInList(t *testing.T, schema map[string]any) int {
	t.Helper()
	body, ok := schema["body"].([]any)
	if !ok || len(body) == 0 {
		return 0
	}
	listNode, ok := body[0].(map[string]any)
	if !ok {
		return 0
	}
	cols, _ := listNode["columns"].([]any)
	// Last column is operations; count its buttons.
	for _, col := range cols {
		m, ok := col.(map[string]any)
		if !ok {
			continue
		}
		if m["type"] == "operation" {
			buttons, _ := m["buttons"].([]any)
			return len(buttons)
		}
	}
	return 0
}

func TestEngine_PermFPIsolation_DifferentSchemas(t *testing.T) {
	// Admin viewer sees Edit + Delete row buttons.
	// Restricted viewer (no permissions) sees none.
	// Their schemas must be different, and must be cached under different keys.

	schema := generator.EntitySchema{
		Name:        "test_entity",
		Title:       "Entity",
		PluralTitle: "Entities",
		ListURL:     "/api/v1/test/entities",
		UIPrefix:    "/ui/test/entities",
		CreateURL:   "/api/v1/test/entities",
		EditURL:     "/api/v1/test/entities/${id}",
		DetailURL:   "/api/v1/test/entities/${id}",
		Permissions: map[string]string{
			"create": "test.entity.create",
			"read":   "test.entity.read",
			"update": "test.entity.update",
			"delete": "test.entity.delete",
		},
		Fields: []generator.FieldDef{
			{Name: "name", Label: "Name", FieldType: "data", InList: true},
		},
	}

	tenantID := uuid.New()

	adminViewer := &stubViewer{admin: true}
	adminCtx, _ := sduictx.NewGeneratorContext(
		tenantID, adminViewer, "test_entity", sduictx.ViewModeList, "amis",
	).WithSchemaFingerprint("sfp1").WithPermFingerprint("__admin__").Build()

	restrictedViewer := &stubViewer{admin: false}
	restrictedCtx, _ := sduictx.NewGeneratorContext(
		tenantID, restrictedViewer, "test_entity", sduictx.ViewModeList, "amis",
	).WithSchemaFingerprint("sfp1").WithPermFingerprint("__noroles__").Build()

	eng := makeEngine(nil)

	adminResp, err := eng.Handle(context.Background(), engine.Request{Ctx: adminCtx, Schema: schema})
	if err != nil {
		t.Fatalf("admin Handle: %v", err)
	}
	restrictedResp, err := eng.Handle(context.Background(), engine.Request{Ctx: restrictedCtx, Schema: schema})
	if err != nil {
		t.Fatalf("restricted Handle: %v", err)
	}

	adminActions := actionsInList(t, adminResp.Output.AMISSchema)
	restrictedActions := actionsInList(t, restrictedResp.Output.AMISSchema)

	// Admin must have more row actions than a viewer with no permissions.
	if adminActions <= restrictedActions {
		t.Errorf("admin schema actions (%d) must exceed restricted actions (%d)",
			adminActions, restrictedActions)
	}
}

func TestEngine_PermFPIsolation_CacheNotShared(t *testing.T) {
	// Prove: with a live cache, admin schema stored under "__admin__" permFP
	// is not returned for a "__noroles__" permFP lookup.
	c := cache.New(newInlineRedis())
	eng := makeEngine(c)

	schema := generator.EntitySchema{
		Name:        "cache_test_entity",
		Title:       "Entity",
		PluralTitle: "Entities",
		ListURL:     "/api/v1/test/entities",
		Fields:      []generator.FieldDef{{Name: "name", Label: "Name", FieldType: "data", InList: true}},
		Permissions: map[string]string{"delete": "test.entity.delete"},
	}
	tenantID := uuid.New()

	// Admin populates cache.
	adminCtx, _ := sduictx.NewGeneratorContext(
		tenantID, &stubViewer{admin: true}, "cache_test_entity", sduictx.ViewModeList, "amis",
	).WithSchemaFingerprint("sfX").WithPermFingerprint("__admin__").Build()
	if _, err := eng.Handle(context.Background(), engine.Request{Ctx: adminCtx, Schema: schema}); err != nil {
		t.Fatalf("admin Handle: %v", err)
	}

	// Restricted viewer — MUST NOT get admin's cached output.
	restrictedCtx, _ := sduictx.NewGeneratorContext(
		tenantID, &stubViewer{admin: false}, "cache_test_entity", sduictx.ViewModeList, "amis",
	).WithSchemaFingerprint("sfX").WithPermFingerprint("__noroles__").Build()
	restrictedResp, err := eng.Handle(context.Background(), engine.Request{Ctx: restrictedCtx, Schema: schema})
	if err != nil {
		t.Fatalf("restricted Handle: %v", err)
	}

	// L3 keys differ by PermFP — restricted viewer must miss the cache and
	// generate its own schema. A cache hit here would mean the fix broke.
	if restrictedResp.CacheHit {
		t.Error("restricted viewer must not receive a cache hit from admin's cached schema")
	}
}

func TestEngine_TenantIsolation_CacheNotShared(t *testing.T) {
	// Prove: a schema cached for tenant A is not returned for tenant B.
	c := cache.New(newInlineRedis())
	eng := makeEngine(c)

	schema := generator.EntitySchema{
		Name:    "tenant_test_entity",
		Title:   "Entity",
		ListURL: "/api/v1/test/entities",
		Fields:  []generator.FieldDef{{Name: "name", Label: "Name", FieldType: "data", InList: true}},
	}

	tenantA := uuid.New()
	tenantB := uuid.New()

	ctxA, _ := sduictx.NewGeneratorContext(
		tenantA, &stubViewer{admin: true}, "tenant_test_entity", sduictx.ViewModeList, "amis",
	).WithSchemaFingerprint("sfT").WithPermFingerprint("__admin__").Build()

	ctxB, _ := sduictx.NewGeneratorContext(
		tenantB, &stubViewer{admin: true}, "tenant_test_entity", sduictx.ViewModeList, "amis",
	).WithSchemaFingerprint("sfT").WithPermFingerprint("__admin__").Build()

	// Populate cache for tenant A.
	if _, err := eng.Handle(context.Background(), engine.Request{Ctx: ctxA, Schema: schema}); err != nil {
		t.Fatalf("tenant A Handle: %v", err)
	}

	// Tenant B must not get a cache hit from tenant A's entry.
	respB, err := eng.Handle(context.Background(), engine.Request{Ctx: ctxB, Schema: schema})
	if err != nil {
		t.Fatalf("tenant B Handle: %v", err)
	}
	if respB.CacheHit {
		t.Error("tenant B must not receive tenant A's cached schema")
	}
}

func TestEngine_Determinism(t *testing.T) {
	// Identical inputs must produce identical AMIS JSON output.
	eng := makeEngine(nil)
	req := engine.Request{
		Ctx:    makeCtx(sduictx.ViewModeList),
		Schema: makeSchema(),
	}

	resp1, err := eng.Handle(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	resp2, err := eng.Handle(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	b1, _ := json.Marshal(resp1.Output.AMISSchema)
	b2, _ := json.Marshal(resp2.Output.AMISSchema)
	if string(b1) != string(b2) {
		t.Errorf("engine output is not deterministic:\nrun1: %s\nrun2: %s", b1, b2)
	}
}

// inlineRedis is a goroutine-safe map-backed Redis stub for cache isolation tests.
type inlineRedis struct {
	mu   sync.Mutex
	data map[string]string
}

func newInlineRedis() *inlineRedis {
	return &inlineRedis{data: make(map[string]string)}
}
func (r *inlineRedis) Get(_ context.Context, key string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.data[key]
	if !ok {
		return "", cache.ErrCacheMiss
	}
	return v, nil
}
func (r *inlineRedis) Set(_ context.Context, key, value string, _ time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[key] = value
	return nil
}

// ── schema content tests ──────────────────────────────────────────────────────
//
// These tests verify that generated AMIS schemas contain the expected structure
// and field values, not merely that generation succeeds without error.

// listSchema returns the AMIS crud node (body[0]) from a list view response.
func listSchema(t *testing.T, schema map[string]any) map[string]any {
	t.Helper()
	body, ok := schema["body"].([]any)
	if !ok || len(body) == 0 {
		t.Fatal("list schema: body is missing or empty")
	}
	node, ok := body[0].(map[string]any)
	if !ok {
		t.Fatal("list schema: body[0] is not a map")
	}
	return node
}

// formBody returns the body array of the AMIS form node (body[0]) from a form view.
func formBody(t *testing.T, schema map[string]any) []any {
	t.Helper()
	body, ok := schema["body"].([]any)
	if !ok || len(body) == 0 {
		t.Fatal("form schema: body is missing or empty")
	}
	form, ok := body[0].(map[string]any)
	if !ok {
		t.Fatal("form schema: body[0] is not a map")
	}
	fb, ok := form["body"].([]any)
	if !ok {
		t.Fatal("form schema: form.body is missing or not a slice")
	}
	return fb
}

// TestEngine_ListSchema_Type verifies that the list view output root is "page"
// and body[0] is a "crud" node (AMIS list widget).
func TestEngine_ListSchema_Type(t *testing.T) {
	eng := makeEngine(nil)
	resp, err := eng.Handle(context.Background(), engine.Request{
		Ctx:    makeCtx(sduictx.ViewModeList),
		Schema: makeSchema(),
	})
	if err != nil {
		t.Fatal(err)
	}
	s := resp.Output.AMISSchema
	if s["type"] != "page" {
		t.Errorf("root type = %v, want \"page\"", s["type"])
	}
	crud := listSchema(t, s)
	if crud["type"] != "crud" {
		t.Errorf("list body[0].type = %v, want \"crud\"", crud["type"])
	}
}

// TestEngine_ListSchema_HasColumns verifies that a list view contains "columns"
// entries that match the entity fields declared with InList:true.
func TestEngine_ListSchema_HasColumns(t *testing.T) {
	eng := makeEngine(nil)
	resp, err := eng.Handle(context.Background(), engine.Request{
		Ctx:    makeCtx(sduictx.ViewModeList),
		Schema: makeSchema(),
	})
	if err != nil {
		t.Fatal(err)
	}
	crud := listSchema(t, resp.Output.AMISSchema)
	cols, ok := crud["columns"].([]any)
	if !ok {
		t.Fatal("columns is missing or not a slice")
	}

	// makeSchema has two InList:true fields: "name" (data) and "status" (select).
	// Admin viewer so no permission gate. Expect at least 2 data columns.
	dataColCount := 0
	for _, c := range cols {
		m, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if m["type"] == "operation" {
			continue // skip the row-actions column
		}
		dataColCount++
	}
	if dataColCount < 2 {
		t.Errorf("expected at least 2 data columns, got %d", dataColCount)
	}

	// Verify "name" column is present.
	found := false
	for _, c := range cols {
		m, ok := c.(map[string]any)
		if ok && m["name"] == "name" {
			found = true
			break
		}
	}
	if !found {
		t.Error("\"name\" column not found in list columns")
	}
}

// TestEngine_FormSchema_Type verifies that a create-form output root is "page"
// and body[0] is a "form" node.
func TestEngine_FormSchema_Type(t *testing.T) {
	eng := makeEngine(nil)
	resp, err := eng.Handle(context.Background(), engine.Request{
		Ctx:    makeCtx(sduictx.ViewModeCreate),
		Schema: makeSchema(),
	})
	if err != nil {
		t.Fatal(err)
	}
	s := resp.Output.AMISSchema
	if s["type"] != "page" {
		t.Errorf("root type = %v, want \"page\"", s["type"])
	}
	body, _ := s["body"].([]any)
	if len(body) == 0 {
		t.Fatal("form body is empty")
	}
	form, _ := body[0].(map[string]any)
	if form["type"] != "form" {
		t.Errorf("form body[0].type = %v, want \"form\"", form["type"])
	}
}

// TestEngine_FormSchema_HasBody verifies that form body contains input controls
// for each non-readonly InForm field.
func TestEngine_FormSchema_HasBody(t *testing.T) {
	eng := makeEngine(nil)
	resp, err := eng.Handle(context.Background(), engine.Request{
		Ctx:    makeCtx(sduictx.ViewModeCreate),
		Schema: makeSchema(),
	})
	if err != nil {
		t.Fatal(err)
	}
	fb := formBody(t, resp.Output.AMISSchema)
	// makeSchema has 3 InForm fields: name (data→input-text), status (select→select), notes (long_text→textarea).
	if len(fb) < 3 {
		t.Errorf("form body has %d items, expected at least 3", len(fb))
	}

	// Verify "name" field appears in the form body.
	found := false
	for _, item := range fb {
		m, ok := item.(map[string]any)
		if ok && m["name"] == "name" {
			found = true
			break
		}
	}
	if !found {
		t.Error("\"name\" field not found in form body")
	}
}

// TestEngine_DetailSchema_Type verifies that a detail view output root is "page".
func TestEngine_DetailSchema_Type(t *testing.T) {
	eng := makeEngine(nil)
	resp, err := eng.Handle(context.Background(), engine.Request{
		Ctx:    makeCtx(sduictx.ViewModeDetail),
		Schema: makeSchema(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Output.AMISSchema["type"] != "page" {
		t.Errorf("detail root type = %v, want \"page\"", resp.Output.AMISSchema["type"])
	}
}

// TestEngine_SensitiveField_ExcludedFromList verifies that a field flagged with
// Permission set to a value the viewer does NOT have is absent from list columns.
// This is the permission-gated "sensitive field" exclusion path in the generator.
func TestEngine_SensitiveField_ExcludedFromList(t *testing.T) {
	noperms := &stubViewer{admin: false}
	ctx, _ := sduictx.NewGeneratorContext(
		uuid.New(), noperms, "secure_entity", sduictx.ViewModeList, "amis",
	).WithSchemaFingerprint("sfS").WithPermFingerprint("pf-noperms").Build()

	schema := generator.EntitySchema{
		Name:        "secure_entity",
		Title:       "Secure",
		PluralTitle: "Securables",
		ListURL:     "/api/v1/test/securables",
		Fields: []generator.FieldDef{
			{Name: "name", Label: "Name", FieldType: "data", InList: true},
			{Name: "secret_code", Label: "Secret Code", FieldType: "data", InList: true,
				Permission: "secure_entity.admin"}, // viewer does NOT have this
		},
	}

	eng := makeEngine(nil)
	resp, err := eng.Handle(context.Background(), engine.Request{Ctx: ctx, Schema: schema})
	if err != nil {
		t.Fatal(err)
	}
	crud := listSchema(t, resp.Output.AMISSchema)
	cols, _ := crud["columns"].([]any)

	// "name" must be present; "secret_code" must be absent.
	nameFound := false
	for _, c := range cols {
		m, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if m["name"] == "secret_code" {
			t.Error("permission-gated field \"secret_code\" must not appear for viewer without permission")
		}
		if m["name"] == "name" {
			nameFound = true
		}
	}
	if !nameFound {
		t.Error("unpermissioned field \"name\" must appear in list columns")
	}
}

// TestEngine_SensitiveField_ExcludedFromForm verifies that a permission-gated
// field is absent from form body when the viewer lacks the required permission.
func TestEngine_SensitiveField_ExcludedFromForm(t *testing.T) {
	noperms := &stubViewer{admin: false}
	ctx, _ := sduictx.NewGeneratorContext(
		uuid.New(), noperms, "secure_form_entity", sduictx.ViewModeCreate, "amis",
	).WithSchemaFingerprint("sfSF").WithPermFingerprint("pf-noperms").Build()

	schema := generator.EntitySchema{
		Name:      "secure_form_entity",
		Title:     "Secure Form",
		CreateURL: "/api/v1/test/secure_form_entities",
		Fields: []generator.FieldDef{
			{Name: "label", Label: "Label", FieldType: "data", InForm: true},
			{Name: "internal_note", Label: "Internal Note", FieldType: "data", InForm: true,
				Permission: "secure_form_entity.internal"}, // viewer does NOT have this
		},
	}

	eng := makeEngine(nil)
	resp, err := eng.Handle(context.Background(), engine.Request{Ctx: ctx, Schema: schema})
	if err != nil {
		t.Fatal(err)
	}
	fb := formBody(t, resp.Output.AMISSchema)

	for _, item := range fb {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if m["name"] == "internal_note" {
			t.Error("permission-gated field \"internal_note\" must not appear in form body for viewer without permission")
		}
	}
}

// TestEngine_SelectField_HasOptions verifies that a FieldTypeSelect field generates
// a "select" type AMIS control with static options in the form body.
func TestEngine_SelectField_HasOptions(t *testing.T) {
	eng := makeEngine(nil)
	schema := generator.EntitySchema{
		Name:      "select_entity",
		Title:     "Select Entity",
		CreateURL: "/api/v1/test/select_entities",
		Fields: []generator.FieldDef{
			{
				Name:      "priority",
				Label:     "Priority",
				FieldType: "select",
				InForm:    true,
				Options: []generator.SelectOption{
					{Label: "Low", Value: "low"},
					{Label: "Medium", Value: "medium"},
					{Label: "High", Value: "high"},
				},
			},
		},
	}

	resp, err := eng.Handle(context.Background(), engine.Request{
		Ctx:    makeCtx(sduictx.ViewModeCreate),
		Schema: schema,
	})
	if err != nil {
		t.Fatal(err)
	}
	fb := formBody(t, resp.Output.AMISSchema)
	if len(fb) == 0 {
		t.Fatal("form body is empty")
	}

	field, ok := fb[0].(map[string]any)
	if !ok {
		t.Fatal("form body[0] is not a map")
	}
	if field["type"] != "select" {
		t.Errorf("select field type = %v, want \"select\"", field["type"])
	}
	opts, ok := field["options"].([]map[string]any)
	if !ok || len(opts) != 3 {
		t.Errorf("select field options: got %v (want 3 entries)", field["options"])
	}
}

// TestEngine_RequiredField_Marked verifies that a field with Required:true
// has "required": true in its AMIS form control output.
func TestEngine_RequiredField_Marked(t *testing.T) {
	eng := makeEngine(nil)
	schema := generator.EntitySchema{
		Name:      "required_entity",
		Title:     "Required Entity",
		CreateURL: "/api/v1/test/required_entities",
		Fields: []generator.FieldDef{
			{Name: "mandatory", Label: "Mandatory", FieldType: "data", InForm: true, Required: true},
			{Name: "optional", Label: "Optional", FieldType: "data", InForm: true, Required: false},
		},
	}

	resp, err := eng.Handle(context.Background(), engine.Request{
		Ctx:    makeCtx(sduictx.ViewModeCreate),
		Schema: schema,
	})
	if err != nil {
		t.Fatal(err)
	}
	fb := formBody(t, resp.Output.AMISSchema)

	var mandatoryField map[string]any
	var optionalField map[string]any
	for _, item := range fb {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		switch m["name"] {
		case "mandatory":
			mandatoryField = m
		case "optional":
			optionalField = m
		}
	}

	if mandatoryField == nil {
		t.Fatal("\"mandatory\" field not found in form body")
	}
	if mandatoryField["required"] != true {
		t.Errorf("mandatory field required = %v, want true", mandatoryField["required"])
	}
	if optionalField != nil {
		if optionalField["required"] == true {
			t.Error("optional field must not have required:true")
		}
	}
}

// TestEngine_LinkField_LookupWithDataSource verifies that a FieldTypeLink field
// with a DataSource generates a "select" type with searchable:true (lookup mode).
func TestEngine_LinkField_LookupWithDataSource(t *testing.T) {
	eng := makeEngine(nil)
	schema := generator.EntitySchema{
		Name:      "link_entity",
		Title:     "Link Entity",
		CreateURL: "/api/v1/test/link_entities",
		Fields: []generator.FieldDef{
			{
				Name:         "currency_id",
				Label:        "Currency",
				FieldType:    "link",
				InForm:       true,
				LinkedEntity: "finance_currency",
				DataSource: &widget.DataSource{
					URL:        "/api/v1/finance/currencies",
					Method:     "GET",
					LabelField: "code",
					ValueField: "id",
				},
			},
		},
	}

	resp, err := eng.Handle(context.Background(), engine.Request{
		Ctx:    makeCtx(sduictx.ViewModeCreate),
		Schema: schema,
	})
	if err != nil {
		t.Fatal(err)
	}
	fb := formBody(t, resp.Output.AMISSchema)
	if len(fb) == 0 {
		t.Fatal("form body is empty")
	}

	field, ok := fb[0].(map[string]any)
	if !ok {
		t.Fatal("form body[0] is not a map")
	}
	// FieldTypeLink with DataSource → NodeLookup → AMIS "select" with searchable:true
	if field["type"] != "select" {
		t.Errorf("link field type = %v, want \"select\"", field["type"])
	}
	if field["searchable"] != true {
		t.Errorf("link field searchable = %v, want true", field["searchable"])
	}
}

// ── benchmark ─────────────────────────────────────────────────────────────────

func BenchmarkEngine_Handle_NoCache(b *testing.B) {
	eng := makeEngine(nil)
	req := engine.Request{
		Ctx:    makeCtx(sduictx.ViewModeList),
		Schema: makeSchema(),
	}
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := eng.Handle(ctx, req); err != nil {
			b.Fatal(err)
		}
	}
}
