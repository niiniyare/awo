package sdui

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"awo.so/awo/auth"
	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"awo.so/awo/registry"
	"awo.so/awo/sdui/sduictx"
)

// ── stub provider implementations ─────────────────────────────────────────────

// stubSettingsProvider returns a fixed map of settings.
type stubSettingsProvider struct {
	data map[string]string
}

func (s stubSettingsProvider) TenantSettings(_ context.Context, _ uuid.UUID) map[string]string {
	if s.data == nil {
		return map[string]string{}
	}
	return s.data
}

// errorSettingsProvider simulates a provider that fails.
type errorSettingsProvider struct{}

func (errorSettingsProvider) TenantSettings(_ context.Context, _ uuid.UUID) map[string]string {
	// Simulates a failed fetch — returns empty map (non-blocking contract).
	return map[string]string{}
}

// stubFlagsProvider returns a fixed flags map.
type stubFlagsProvider struct {
	flags map[string]bool
}

func (s stubFlagsProvider) EnabledFlags(_ context.Context, _ uuid.UUID, _ auth.ViewerContext) map[string]bool {
	if s.flags == nil {
		return map[string]bool{}
	}
	return s.flags
}

// errorFlagsProvider simulates a flags provider that fails.
type errorFlagsProvider struct{}

func (errorFlagsProvider) EnabledFlags(_ context.Context, _ uuid.UUID, _ auth.ViewerContext) map[string]bool {
	return map[string]bool{}
}

// stubRecordFetcher returns a fixed record state for a given ID.
type stubRecordFetcher struct {
	state map[string]any
	err   error
}

func (s *stubRecordFetcher) FetchRecord(_ context.Context, _ string, _ uuid.UUID) (map[string]any, error) {
	return s.state, s.err
}

// ── helpers ───────────────────────────────────────────────────────────────────

func buildSchema(t *testing.T, defs ...def.EntityDefinition) *compiler.CompiledSchema {
	t.Helper()
	reg, err := registry.BuildFrom(defs)
	if err != nil {
		t.Fatalf("registry.BuildFrom: %v", err)
	}
	cs, err := compiler.Compile(reg)
	if err != nil {
		t.Fatalf("compiler.Compile: %v", err)
	}
	return cs
}

func minimalDef(name, module string) *def.SystemDefinition {
	return &def.SystemDefinition{
		Name:   name,
		Module: module,
		Permissions: def.PermissionSet{
			Read: []string{module + "." + name + ".read"},
		},
	}
}

// testViewer is a minimal auth.ViewerContext for testing.
type testViewer struct{ tenantID uuid.UUID }

func (v testViewer) TenantID() uuid.UUID         { return v.tenantID }
func (v testViewer) UserID() uuid.UUID           { return uuid.Nil }
func (v testViewer) ServiceAccountID() uuid.UUID { return uuid.Nil }
func (v testViewer) Roles() []string             { return nil }
func (v testViewer) HasRole(string) bool         { return false }
func (v testViewer) IsPlatformAdmin() bool       { return true }
func (v testViewer) Actor() *def.Actor {
	return &def.Actor{TenantID: v.tenantID, Roles: nil}
}

// ── pageBuilderFor tests ──────────────────────────────────────────────────────

func TestPageBuilderFor_List(t *testing.T) {
	called := false
	pbs := def.PageBuilderSet{
		List: func(_ context.Context, _ def.PageContext) (map[string]any, error) {
			called = true
			return map[string]any{"type": "list"}, nil
		},
	}
	pb := pageBuilderFor(pbs, sduictx.ViewModeList)
	if pb == nil {
		t.Fatal("expected non-nil builder for ViewModeList")
	}
	result, err := pb(context.Background(), def.PageContext{})
	if err != nil {
		t.Fatalf("builder error: %v", err)
	}
	if !called {
		t.Error("List builder was not called")
	}
	if result["type"] != "list" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestPageBuilderFor_Create(t *testing.T) {
	pbs := def.PageBuilderSet{
		Create: func(_ context.Context, _ def.PageContext) (map[string]any, error) {
			return map[string]any{"type": "create"}, nil
		},
	}
	pb := pageBuilderFor(pbs, sduictx.ViewModeCreate)
	if pb == nil {
		t.Fatal("expected builder for ViewModeCreate")
	}
}

func TestPageBuilderFor_Edit(t *testing.T) {
	pbs := def.PageBuilderSet{
		Edit: func(_ context.Context, _ def.PageContext) (map[string]any, error) {
			return map[string]any{"type": "edit"}, nil
		},
	}
	pb := pageBuilderFor(pbs, sduictx.ViewModeEdit)
	if pb == nil {
		t.Fatal("expected builder for ViewModeEdit")
	}
}

func TestPageBuilderFor_Detail(t *testing.T) {
	pbs := def.PageBuilderSet{
		Detail: func(_ context.Context, _ def.PageContext) (map[string]any, error) {
			return map[string]any{"type": "detail"}, nil
		},
	}
	pb := pageBuilderFor(pbs, sduictx.ViewModeDetail)
	if pb == nil {
		t.Fatal("expected builder for ViewModeDetail")
	}
}

func TestPageBuilderFor_NilBuilders(t *testing.T) {
	pbs := def.PageBuilderSet{} // all nil
	for _, mode := range []sduictx.ViewMode{
		sduictx.ViewModeList,
		sduictx.ViewModeCreate,
		sduictx.ViewModeEdit,
		sduictx.ViewModeDetail,
	} {
		if pb := pageBuilderFor(pbs, mode); pb != nil {
			t.Errorf("mode %v: expected nil builder, got non-nil", mode)
		}
	}
}

func TestPageBuilderFor_UnknownMode(t *testing.T) {
	pbs := def.PageBuilderSet{
		List: func(_ context.Context, _ def.PageContext) (map[string]any, error) {
			return nil, nil
		},
	}
	pb := pageBuilderFor(pbs, sduictx.ViewMode("dashboard"))
	if pb != nil {
		t.Error("unknown mode should return nil builder")
	}
}

func TestPageBuilderFor_OnlyListSet_OtherModesNil(t *testing.T) {
	pbs := def.PageBuilderSet{
		List: func(_ context.Context, _ def.PageContext) (map[string]any, error) {
			return nil, nil
		},
	}
	if pb := pageBuilderFor(pbs, sduictx.ViewModeCreate); pb != nil {
		t.Error("Create should be nil when only List is set")
	}
	if pb := pageBuilderFor(pbs, sduictx.ViewModeEdit); pb != nil {
		t.Error("Edit should be nil when only List is set")
	}
	if pb := pageBuilderFor(pbs, sduictx.ViewModeDetail); pb != nil {
		t.Error("Detail should be nil when only List is set")
	}
}

// ── viewModeToPageKind tests ──────────────────────────────────────────────────

func TestViewModeToPageKind_AllModes(t *testing.T) {
	cases := []struct {
		mode sduictx.ViewMode
		want def.PageKind
	}{
		{sduictx.ViewModeList, def.PageKindList},
		{sduictx.ViewModeCreate, def.PageKindCreate},
		{sduictx.ViewModeEdit, def.PageKindEdit},
		{sduictx.ViewModeDetail, def.PageKindDetail},
	}
	for _, tc := range cases {
		got := viewModeToPageKind(tc.mode)
		if got != tc.want {
			t.Errorf("viewModeToPageKind(%v) = %v, want %v", tc.mode, got, tc.want)
		}
	}
}

func TestViewModeToPageKind_Unknown_Passthrough(t *testing.T) {
	got := viewModeToPageKind(sduictx.ViewMode("custom"))
	if string(got) != "custom" {
		t.Errorf("unknown mode should pass through as PageKind, got %q", got)
	}
}

// ── viewerToActor tests ───────────────────────────────────────────────────────

func TestViewerToActor_PropagatesTenantID(t *testing.T) {
	tenantID := uuid.New()
	viewer := testViewer{tenantID: tenantID}
	actor := viewerToActor(viewer)
	if actor.TenantID != tenantID {
		t.Errorf("actor TenantID mismatch: got %v, want %v", actor.TenantID, tenantID)
	}
}

// ── nav handler tests ─────────────────────────────────────────────────────────

func TestNav_GroupsByModule(t *testing.T) {
	cs := buildSchema(t,
		minimalDef("invoice", "finance"),
		minimalDef("payment", "finance"),
		minimalDef("user", "iam"),
	)

	app := fiber.New()
	h := New(cs, nil, nil)
	h.Register(app.Group("/api/v1/ui"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ui/nav", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result []NavModule
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("unmarshal nav response: %v\nbody: %s", err, body)
	}

	// Expect 2 modules: finance (2 entries) and iam (1 entry).
	if len(result) != 2 {
		t.Errorf("expected 2 modules, got %d: %+v", len(result), result)
	}

	byModule := make(map[string]NavModule)
	for _, m := range result {
		byModule[m.Module] = m
	}

	finance, ok := byModule["finance"]
	if !ok {
		t.Fatal("finance module missing from nav")
	}
	if len(finance.Entries) != 2 {
		t.Errorf("finance: expected 2 entries, got %d", len(finance.Entries))
	}

	iam, ok := byModule["iam"]
	if !ok {
		t.Fatal("iam module missing from nav")
	}
	if len(iam.Entries) != 1 {
		t.Errorf("iam: expected 1 entry, got %d", len(iam.Entries))
	}
}

func TestNav_ExcludesEntitiesWithoutReadPermission(t *testing.T) {
	withRead := minimalDef("invoice", "finance")
	withoutRead := &def.SystemDefinition{
		Name:   "internal_log",
		Module: "finance",
		// no Read permission declared
	}
	cs := buildSchema(t, withRead, withoutRead)

	app := fiber.New()
	h := New(cs, nil, nil)
	h.Register(app.Group("/api/v1/ui"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ui/nav", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result []NavModule
	json.Unmarshal(body, &result) //nolint:errcheck

	for _, mod := range result {
		for _, entry := range mod.Entries {
			if entry.Entity == "finance_internal_log" {
				t.Error("entity without Read permission should be excluded from nav")
			}
		}
	}
}

func TestNav_EntryHasCorrectListURL(t *testing.T) {
	cs := buildSchema(t, minimalDef("invoice", "finance"))

	app := fiber.New()
	h := New(cs, nil, nil)
	h.Register(app.Group("/api/v1/ui"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ui/nav", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result []NavModule
	json.Unmarshal(body, &result) //nolint:errcheck

	if len(result) == 0 || len(result[0].Entries) == 0 {
		t.Fatal("expected at least one nav entry")
	}

	entry := result[0].Entries[0]
	if entry.ListURL == "" {
		t.Error("nav entry ListURL should not be empty")
	}
	// URL should contain module and resource.
	if entry.Module != "finance" {
		t.Errorf("entry.Module = %q, want \"finance\"", entry.Module)
	}
}

func TestNav_EmptySchema(t *testing.T) {
	// Schema with no entities — nav should return empty array.
	cs := &compiler.CompiledSchema{Entities: nil}

	app := fiber.New()
	h := New(cs, nil, nil)
	h.Register(app.Group("/api/v1/ui"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ui/nav", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// ── PageBuilderSet wiring: end-to-end (BUG-012 core verification) ─────────────

func TestPageBuilderSet_ListBuilder_CalledForListRequest(t *testing.T) {
	builderCalled := false
	d := &def.SystemDefinition{
		Name:   "invoice",
		Module: "finance",
		Permissions: def.PermissionSet{
			Read: []string{"finance.invoice.read"},
		},
		PageBuilders: def.PageBuilderSet{
			List: func(_ context.Context, pctx def.PageContext) (map[string]any, error) {
				builderCalled = true
				return map[string]any{"type": "page", "title": "Custom Invoice List"}, nil
			},
		},
	}
	cs := buildSchema(t, d)

	app := fiber.New()
	h := New(cs, nil, nil)
	h.Register(app.Group("/api/v1/ui"))

	// Inject viewer into Fiber context via middleware.
	tenantID := uuid.New()
	app.Use(func(c *fiber.Ctx) error {
		ctx := auth.WithViewer(c.UserContext(), testViewer{tenantID: tenantID})
		c.SetUserContext(ctx)
		return c.Next()
	})
	// Re-register after middleware (middleware must come before routes).
	app2 := fiber.New()
	app2.Use(func(c *fiber.Ctx) error {
		ctx := auth.WithViewer(c.UserContext(), testViewer{tenantID: tenantID})
		c.SetUserContext(ctx)
		return c.Next()
	})
	h2 := New(cs, nil, nil)
	h2.Register(app2.Group("/api/v1/ui"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ui/finance/invoices", nil)
	resp, err := app2.Test(req)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}
	defer resp.Body.Close()

	if !builderCalled {
		t.Error("BUG-012: PageBuilderSet.List was not called for list request")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if result["title"] != "Custom Invoice List" {
		t.Errorf("expected custom builder result, got: %v", result)
	}
}

func TestPageBuilderSet_NilReturn_FallsThrough(t *testing.T) {
	// When builder returns nil, engine should be called instead.
	// With nil engine, we expect an internal error (proving fall-through happened).
	d := &def.SystemDefinition{
		Name:   "invoice",
		Module: "finance",
		Permissions: def.PermissionSet{
			Read: []string{"finance.invoice.read"},
		},
		PageBuilders: def.PageBuilderSet{
			List: func(_ context.Context, _ def.PageContext) (map[string]any, error) {
				return nil, nil // nil → fall through to engine
			},
		},
	}
	cs := buildSchema(t, d)
	tenantID := uuid.New()

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		ctx := auth.WithViewer(c.UserContext(), testViewer{tenantID: tenantID})
		c.SetUserContext(ctx)
		return c.Next()
	})
	h := New(cs, nil, nil) // nil engine
	h.Register(app.Group("/api/v1/ui"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ui/finance/invoices", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}
	defer resp.Body.Close()

	// nil engine → 500; this proves the builder returned nil and engine was attempted.
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 (nil engine fallthrough), got %d", resp.StatusCode)
	}
}

func TestPageBuilderSet_NoBuilder_FallsThrough(t *testing.T) {
	// No PageBuilders set → should reach engine (nil engine → 500).
	d := minimalDef("invoice", "finance")
	cs := buildSchema(t, d)
	tenantID := uuid.New()

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		ctx := auth.WithViewer(c.UserContext(), testViewer{tenantID: tenantID})
		c.SetUserContext(ctx)
		return c.Next()
	})
	h := New(cs, nil, nil) // nil engine — proves we reached engine path
	h.Register(app.Group("/api/v1/ui"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ui/finance/invoices", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 (nil engine), got %d", resp.StatusCode)
	}
}

func TestPageBuilderSet_EntityNotFound_Returns404(t *testing.T) {
	cs := buildSchema(t, minimalDef("invoice", "finance"))
	tenantID := uuid.New()

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		ctx := auth.WithViewer(c.UserContext(), testViewer{tenantID: tenantID})
		c.SetUserContext(ctx)
		return c.Next()
	})
	h := New(cs, nil, nil)
	h.Register(app.Group("/api/v1/ui"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ui/finance/nonexistent", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for unknown entity, got %d", resp.StatusCode)
	}
}

// ── Handler.New / construction tests ─────────────────────────────────────────

func TestNew_PrecomputesSchemaFingerprints(t *testing.T) {
	cs := buildSchema(t,
		minimalDef("invoice", "finance"),
		minimalDef("payment", "finance"),
	)
	h := New(cs, nil, nil)
	if len(h.schemaFPs) != len(cs.Entities) {
		t.Errorf("expected %d fingerprints, got %d", len(cs.Entities), len(h.schemaFPs))
	}
	for _, es := range cs.Entities {
		if _, ok := h.schemaFPs[es.QualifiedName]; !ok {
			t.Errorf("missing fingerprint for %s", es.QualifiedName)
		}
	}
}

func TestFindEntity_FindsByModuleAndResource(t *testing.T) {
	cs := buildSchema(t,
		minimalDef("invoice", "finance"),
		minimalDef("user", "iam"),
	)
	h := New(cs, nil, nil)

	es := h.findEntity("finance", "invoices")
	if es == nil {
		t.Fatal("expected to find finance/invoices")
	}
	if es.QualifiedName != "finance_invoice" {
		t.Errorf("wrong entity: %s", es.QualifiedName)
	}
}

func TestFindEntity_ReturnsNilForUnknown(t *testing.T) {
	cs := buildSchema(t, minimalDef("invoice", "finance"))
	h := New(cs, nil, nil)

	if h.findEntity("finance", "nonexistent") != nil {
		t.Error("expected nil for unknown entity")
	}
	if h.findEntity("unknown_module", "invoices") != nil {
		t.Error("expected nil for unknown module")
	}
}

// ── PageContext dynamic data population tests ──────────────────────────────────

// newTestApp creates a Fiber app with viewer middleware and the handler registered.
func newTestApp(t *testing.T, cs *compiler.CompiledSchema, tenantID uuid.UUID, opts ...HandlerOption) *fiber.App {
	t.Helper()
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		ctx := auth.WithViewer(c.UserContext(), testViewer{tenantID: tenantID})
		c.SetUserContext(ctx)
		return c.Next()
	})
	h := New(cs, nil, nil, opts...)
	h.Register(app.Group("/api/v1/ui"))
	return app
}

// builderCapture wraps a PageBuilder and stores the PageContext it received.
type builderCapture struct {
	captured def.PageContext
	schema   map[string]any
}

func (bc *builderCapture) builder(_ context.Context, pctx def.PageContext) (map[string]any, error) {
	bc.captured = pctx
	if bc.schema != nil {
		return bc.schema, nil
	}
	return map[string]any{"type": "page"}, nil
}

func TestPageContext_TenantSettings_Populated(t *testing.T) {
	bc := &builderCapture{}
	d := &def.SystemDefinition{
		Name:   "invoice",
		Module: "finance",
		Permissions: def.PermissionSet{
			Read: []string{"finance.invoice.read"},
		},
		PageBuilders: def.PageBuilderSet{
			List: bc.builder,
		},
	}
	cs := buildSchema(t, d)
	tenantID := uuid.New()

	want := map[string]string{"currency": "KES", "date_format": "DD/MM/YYYY"}
	app := newTestApp(t, cs, tenantID, WithSettingsProvider(stubSettingsProvider{data: want}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ui/finance/invoices", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	for k, v := range want {
		if got := bc.captured.TenantSettings[k]; got != v {
			t.Errorf("TenantSettings[%q] = %q, want %q", k, got, v)
		}
	}
}

func TestPageContext_FeatureFlags_Populated(t *testing.T) {
	bc := &builderCapture{}
	d := &def.SystemDefinition{
		Name:   "invoice",
		Module: "finance",
		Permissions: def.PermissionSet{
			Read: []string{"finance.invoice.read"},
		},
		PageBuilders: def.PageBuilderSet{
			List: bc.builder,
		},
	}
	cs := buildSchema(t, d)
	tenantID := uuid.New()

	want := map[string]bool{"new_invoice_ui": true, "bulk_import": false}
	app := newTestApp(t, cs, tenantID, WithFlagsProvider(stubFlagsProvider{flags: want}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ui/finance/invoices", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	for k, v := range want {
		if got := bc.captured.EnabledFeatureFlags[k]; got != v {
			t.Errorf("EnabledFeatureFlags[%q] = %v, want %v", k, got, v)
		}
	}
}

func TestPageContext_RecordID_PopulatedForDetail(t *testing.T) {
	bc := &builderCapture{}
	d := &def.SystemDefinition{
		Name:   "invoice",
		Module: "finance",
		Permissions: def.PermissionSet{
			Read: []string{"finance.invoice.read"},
		},
		PageBuilders: def.PageBuilderSet{
			Detail: bc.builder,
		},
	}
	cs := buildSchema(t, d)
	tenantID := uuid.New()
	recordID := uuid.New()

	app := newTestApp(t, cs, tenantID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ui/finance/invoices/"+recordID.String(), nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	if bc.captured.RecordID != recordID {
		t.Errorf("RecordID = %v, want %v", bc.captured.RecordID, recordID)
	}
}

func TestPageContext_RecordState_PopulatedWhenFetcher(t *testing.T) {
	bc := &builderCapture{}
	d := &def.SystemDefinition{
		Name:   "invoice",
		Module: "finance",
		Permissions: def.PermissionSet{
			Read: []string{"finance.invoice.read"},
		},
		PageBuilders: def.PageBuilderSet{
			Detail: bc.builder,
		},
	}
	cs := buildSchema(t, d)
	tenantID := uuid.New()
	recordID := uuid.New()

	wantState := map[string]any{"status": "draft", "amount": 1000.0}
	fetcher := &stubRecordFetcher{state: wantState}
	app := newTestApp(t, cs, tenantID, WithRecordFetcher(fetcher))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ui/finance/invoices/"+recordID.String(), nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	if bc.captured.RecordState == nil {
		t.Fatal("RecordState is nil; expected populated map")
	}
	if bc.captured.RecordState["status"] != "draft" {
		t.Errorf("RecordState[status] = %v, want %q", bc.captured.RecordState["status"], "draft")
	}
}

func TestPageContext_RecordID_NilForList(t *testing.T) {
	bc := &builderCapture{}
	d := &def.SystemDefinition{
		Name:   "invoice",
		Module: "finance",
		Permissions: def.PermissionSet{
			Read: []string{"finance.invoice.read"},
		},
		PageBuilders: def.PageBuilderSet{
			List: bc.builder,
		},
	}
	cs := buildSchema(t, d)
	tenantID := uuid.New()

	app := newTestApp(t, cs, tenantID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ui/finance/invoices", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	if bc.captured.RecordID != uuid.Nil {
		t.Errorf("RecordID should be uuid.Nil for list view, got %v", bc.captured.RecordID)
	}
	if bc.captured.RecordState != nil {
		t.Errorf("RecordState should be nil for list view, got %v", bc.captured.RecordState)
	}
}

func TestPageContext_ProviderError_DoesNotBlock(t *testing.T) {
	// Both providers return empty maps on simulated failure — request must succeed.
	bc := &builderCapture{}
	d := &def.SystemDefinition{
		Name:   "invoice",
		Module: "finance",
		Permissions: def.PermissionSet{
			Read: []string{"finance.invoice.read"},
		},
		PageBuilders: def.PageBuilderSet{
			List: bc.builder,
		},
	}
	cs := buildSchema(t, d)
	tenantID := uuid.New()

	app := newTestApp(t, cs, tenantID,
		WithSettingsProvider(errorSettingsProvider{}),
		WithFlagsProvider(errorFlagsProvider{}),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ui/finance/invoices", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 200 even on provider error, got %d: %s", resp.StatusCode, body)
	}

	// Providers returned empty maps — PageContext fields should be non-nil but empty.
	if bc.captured.TenantSettings == nil {
		t.Error("TenantSettings should be non-nil empty map, not nil")
	}
	if len(bc.captured.TenantSettings) != 0 {
		t.Errorf("TenantSettings should be empty on error, got %v", bc.captured.TenantSettings)
	}
	if bc.captured.EnabledFeatureFlags == nil {
		t.Error("EnabledFeatureFlags should be non-nil empty map, not nil")
	}
}

// ── computeFlagFingerprint tests ──────────────────────────────────────────────

func TestFeatureFlagFP_InCacheKey_DifferentFlagSetsDifferentFP(t *testing.T) {
	flags1 := map[string]bool{"feature_a": true}
	flags2 := map[string]bool{"feature_a": true, "feature_b": false}

	fp1 := computeFlagFingerprint(flags1)
	fp2 := computeFlagFingerprint(flags2)

	if fp1 == fp2 {
		t.Errorf("different flag sets should produce different fingerprints, both got %q", fp1)
	}
	if fp1 == "" {
		t.Error("non-empty flags should produce non-empty fingerprint")
	}
}

func TestFeatureFlagFP_EmptyFlags_NoDimension(t *testing.T) {
	// Empty flags → empty fingerprint preserving legacy cache key format.
	fp := computeFlagFingerprint(map[string]bool{})
	if fp != "" {
		t.Errorf("empty flags should produce empty fingerprint, got %q", fp)
	}
}

func TestFeatureFlagFP_Deterministic(t *testing.T) {
	flags := map[string]bool{"alpha": true, "beta": false, "gamma": true}

	fp1 := computeFlagFingerprint(flags)
	fp2 := computeFlagFingerprint(flags)

	if fp1 != fp2 {
		t.Errorf("computeFlagFingerprint must be deterministic; got %q and %q", fp1, fp2)
	}
}

func TestFeatureFlagFP_OrderIndependent(t *testing.T) {
	// Two maps with same content in different insertion order should produce same FP.
	flags1 := map[string]bool{"alpha": true, "beta": true}
	flags2 := map[string]bool{"beta": true, "alpha": true}

	fp1 := computeFlagFingerprint(flags1)
	fp2 := computeFlagFingerprint(flags2)

	if fp1 != fp2 {
		t.Errorf("flag fingerprint must be order-independent; got %q and %q", fp1, fp2)
	}
}
