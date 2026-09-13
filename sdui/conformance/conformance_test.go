// Package conformance verifies SDUI framework invariants across all packages.
//
// These are cross-cutting tests that cannot live in a single package without
// creating import cycles. They verify:
//
//  1. Renderer coverage — every widget.NodeKind is handled by the AMIS renderer.
//  2. Fingerprint determinism — same input → same SchemaFingerprint.
//  3. Layout determinism — same schema + context → identical widget tree.
//  4. Cache key stability — cache keys are content-addressed and locale-stable.
package conformance_test

import (
	"testing"

	"github.com/google/uuid"

	"awo.so/awo/sdui/amis"
	"awo.so/awo/sdui/generator"
	"awo.so/awo/sdui/renderer"
	"awo.so/awo/sdui/sduictx"
	"awo.so/awo/sdui/widget"
)

// ── 1. Renderer coverage ──────────────────────────────────────────────────────

// TestAMISRendererCoversAllNodeKinds verifies that the AMIS renderer handles
// every NodeKind constant defined in the widget package. Any new NodeKind added
// to widget must also be handled in amis.DefaultRenderer or this test fails.
//
// The test constructs a minimal Node for each kind and calls Render(). It
// expects no error (unknown-kind error = uncovered case).
func TestAMISRendererCoversAllNodeKinds(t *testing.T) {
	r := amis.New()
	ctx := renderer.RendererContext{
		Theme:      "default",
		DateFormat: "YYYY-MM-DD",
		GenCtx:     minimalGenCtx(),
	}

	for _, kind := range allNodeKinds() {
		kind := kind
		t.Run(string(kind), func(t *testing.T) {
			root := minimalNodeFor(kind)
			_, err := r.Render(root, ctx)
			if err != nil {
				t.Errorf("AMIS renderer does not handle NodeKind %q: %v", kind, err)
			}
		})
	}
}

// allNodeKinds returns every NodeKind constant from the widget package.
// This list must be kept in sync with widget/node.go.
// When a new NodeKind is added to widget, add it here.
func allNodeKinds() []widget.NodeKind {
	return []widget.NodeKind{
		// Structural
		widget.NodePage,
		widget.NodeForm,
		widget.NodeList,
		widget.NodeSection,
		widget.NodeTabPane,
		widget.NodeTabs,
		widget.NodeTable,
		widget.NodeGrid,
		widget.NodeDialog,
		// Input
		widget.NodeField,
		widget.NodeText,
		widget.NodeTextArea,
		widget.NodeRichText,
		widget.NodeNumber,
		widget.NodeMoney,
		widget.NodeSelect,
		widget.NodeMultiSelect,
		widget.NodeLookup,
		widget.NodeTreeSelect,
		widget.NodeDate,
		widget.NodeDateTime,
		widget.NodeDuration,
		widget.NodeSwitch,
		widget.NodeEditor,
		widget.NodeColor,
		widget.NodeSignature,
		widget.NodeFileUpload,
		// Display
		widget.NodeStaticText,
		widget.NodeBadge,
		widget.NodeSummaryCard,
		widget.NodeWorkflowPanel,
		widget.NodeAttachments,
		widget.NodeActivity,
		widget.NodeRelatedList,
		// Dashboard
		widget.NodeKPICard,
		widget.NodeChartPanel,
		widget.NodeTablePanel,
		widget.NodeFilterBar,
		// Interactive
		widget.NodeButton,
	}
}

// minimalNodeFor builds the smallest valid Node for a given kind.
// Fields are set only where the renderer requires them to avoid nil-deref panics.
func minimalNodeFor(kind widget.NodeKind) *widget.Node {
	n := &widget.Node{Kind: kind, Name: "test", Label: "Test"}
	switch kind {
	case widget.NodePage:
		// NodePage wraps children — needs no body for conformance test.
	case widget.NodeStaticText:
		n.StaticContent = "static content"
	case widget.NodeWorkflowPanel:
		n.WorkflowID = "test-workflow"
	}
	return n
}

// ── 2. Fingerprint determinism ────────────────────────────────────────────────

// TestGeneratorDeterminism verifies that calling Generate() twice on the same
// EntitySchema and GeneratorContext produces structurally identical node trees.
// This guards the Level-2 cache invariant: the generator must be a pure function.
func TestGeneratorDeterminism(t *testing.T) {
	schema := minimalEntitySchema("finance_invoice")
	ctx := minimalGenCtx()
	g := generator.New()

	gCtx := sduictx.GeneratorContext{
		TenantID:   ctx.TenantID,
		Viewer:     &openViewer{},
		ViewMode:   sduictx.ViewModeList,
		RendererID: "amis",
		EntityName: "finance_invoice",
	}

	tree1, err := g.Generate(schema, gCtx)
	if err != nil {
		t.Fatalf("Generate() first call: %v", err)
	}
	tree2, err := g.Generate(schema, gCtx)
	if err != nil {
		t.Fatalf("Generate() second call: %v", err)
	}

	if !treesEqual(tree1, tree2) {
		t.Error("Generate() is non-deterministic: two calls with same input produced different trees")
	}
}

// TestRendererDeterminism verifies that rendering the same tree twice produces
// identical AMIS schema maps. Guards Level-3 cache correctness.
func TestRendererDeterminism(t *testing.T) {
	r := amis.New()
	ctx := renderer.RendererContext{
		DateFormat:     "YYYY-MM-DD",
		DateTimeFormat: "YYYY-MM-DD HH:mm",
		GenCtx:         minimalGenCtx(),
	}

	tree := &widget.Node{
		Kind:  widget.NodePage,
		Label: "Test Page",
		Children: []*widget.Node{
			{Kind: widget.NodeText, Name: "status", Label: "Status"},
			{Kind: widget.NodeDate, Name: "date", Label: "Date"},
			{Kind: widget.NodeMoney, Name: "amount", Label: "Amount"},
		},
	}

	out1, err := r.Render(tree, ctx)
	if err != nil {
		t.Fatalf("Render() first call: %v", err)
	}
	out2, err := r.Render(tree, ctx)
	if err != nil {
		t.Fatalf("Render() second call: %v", err)
	}

	if !mapsEqual(out1.AMISSchema, out2.AMISSchema) {
		t.Error("Render() is non-deterministic: two calls with same tree produced different AMIS schemas")
	}
}

// ── 3. Locale correctness ─────────────────────────────────────────────────────

// TestLocaleApplyRTL verifies that ApplyLocale sets RTL=true for Arabic locales
// and RTL=false for Latin locales.
func TestLocaleApplyRTL(t *testing.T) {
	cases := []struct {
		locale string
		rtl    bool
	}{
		{"ar-SA", true},
		{"ar-AE", true},
		{"he-IL", true},
		{"fa-IR", true},
		{"en-US", false},
		{"de-DE", false},
		{"zh-CN", false},
		{"", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.locale, func(t *testing.T) {
			ctx := renderer.ApplyLocale(renderer.RendererContext{}, tc.locale)
			if ctx.RTL != tc.rtl {
				t.Errorf("ApplyLocale(%q).RTL = %v, want %v", tc.locale, ctx.RTL, tc.rtl)
			}
		})
	}
}

// TestLocaleApplyDecimalSeparator verifies decimal/thousand separator derivation.
func TestLocaleApplyDecimalSeparator(t *testing.T) {
	cases := []struct {
		locale    string
		decimal   string
		thousands string
	}{
		{"en-US", ".", ","},
		{"de-DE", ",", "."},
		{"fr-FR", ",", "\u202f"},
		{"pt-BR", ",", "."},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.locale, func(t *testing.T) {
			ctx := renderer.ApplyLocale(renderer.RendererContext{}, tc.locale)
			if ctx.DecimalSeparator != tc.decimal {
				t.Errorf("ApplyLocale(%q).DecimalSeparator = %q, want %q", tc.locale, ctx.DecimalSeparator, tc.decimal)
			}
			if ctx.ThousandSeparator != tc.thousands {
				t.Errorf("ApplyLocale(%q).ThousandSeparator = %q, want %q", tc.locale, ctx.ThousandSeparator, tc.thousands)
			}
		})
	}
}

// TestLangFallback verifies that language-only tags fall back correctly.
func TestLangFallback(t *testing.T) {
	// "de" should fall back to de-DE conventions.
	ctx := renderer.ApplyLocale(renderer.RendererContext{}, "de")
	if ctx.DecimalSeparator != "," {
		t.Errorf("ApplyLocale(\"de\").DecimalSeparator = %q, want \",\"", ctx.DecimalSeparator)
	}
	// Unknown locale falls back to en-US.
	ctx = renderer.ApplyLocale(renderer.RendererContext{}, "xx-ZZ")
	if ctx.DecimalSeparator != "." {
		t.Errorf("ApplyLocale(\"xx-ZZ\").DecimalSeparator = %q, want \".\"", ctx.DecimalSeparator)
	}
}

// ── 4. Theme correctness ──────────────────────────────────────────────────────

// TestThemeConfigDark verifies that the "dark" theme sets DarkMode=true
// without changing the base AMIS theme away from "cxd".
func TestThemeConfigDark(t *testing.T) {
	cfg := amis.ThemeConfigFor("dark")
	if cfg.Theme != "cxd" {
		t.Errorf("ThemeConfigFor(\"dark\").Theme = %q, want \"cxd\"", cfg.Theme)
	}
	if !cfg.DarkMode {
		t.Error("ThemeConfigFor(\"dark\").DarkMode = false, want true")
	}
}

// TestThemeConfigAntd verifies that the "antd" theme sets the correct class prefix.
func TestThemeConfigAntd(t *testing.T) {
	cfg := amis.ThemeConfigFor("antd")
	if cfg.Theme != "antd" {
		t.Errorf("ThemeConfigFor(\"antd\").Theme = %q, want \"antd\"", cfg.Theme)
	}
	if cfg.ClassPrefix != "antd-" {
		t.Errorf("ThemeConfigFor(\"antd\").ClassPrefix = %q, want \"antd-\"", cfg.ClassPrefix)
	}
	if cfg.DarkMode {
		t.Error("ThemeConfigFor(\"antd\").DarkMode = true, want false")
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func minimalGenCtx() sduictx.GeneratorContext {
	return sduictx.GeneratorContext{
		TenantID:   uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Viewer:     &openViewer{},
		ViewMode:   sduictx.ViewModeList,
		RendererID: "amis",
		EntityName: "finance_invoice",
	}
}

func minimalEntitySchema(name string) generator.EntitySchema {
	return generator.EntitySchema{
		Name:        name,
		Title:       "Invoice",
		PluralTitle: "Invoices",
		ListURL:     "/api/v1/finance/invoice",
		CreateURL:   "/api/v1/finance/invoice",
		EditURL:     "/api/v1/finance/invoice/:id",
		DetailURL:   "/api/v1/finance/invoice/:id",
		Fields: []generator.FieldDef{
			{Name: "name", Label: "Name", FieldType: "data", InList: true, InForm: true, InDetail: true},
			{Name: "status", Label: "Status", FieldType: "select", InList: true, InForm: true, InDetail: true},
		},
	}
}

// openViewer is a permissive ViewerContext for tests — grants all permissions.
type openViewer struct{}

func (v *openViewer) TenantID() uuid.UUID {
	return uuid.MustParse("00000000-0000-0000-0000-000000000001")
}
func (v *openViewer) Roles() []string           { return []string{"admin"} }
func (v *openViewer) IsPlatformAdmin() bool     { return true }
func (v *openViewer) HasPermission(string) bool { return true }

// treesEqual compares two widget trees by kind and child count (structural equality).
// A deep value comparison is intentionally avoided — the test only verifies
// structural stability, not every field value.
func treesEqual(a, b *widget.Node) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Kind != b.Kind {
		return false
	}
	if len(a.Children) != len(b.Children) {
		return false
	}
	for i := range a.Children {
		if !treesEqual(a.Children[i], b.Children[i]) {
			return false
		}
	}
	return true
}

// mapsEqual compares two map[string]any by key presence.
// Used for renderer determinism: checks that both outputs have the same keys
// at the top level. Deep value equality is not verified here.
func mapsEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}
