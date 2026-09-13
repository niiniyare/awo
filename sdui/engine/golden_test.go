package engine_test

// Golden tests protect generated SDUI schemas from unintended changes.
//
// When a schema intentionally changes, regenerate golden files by running:
//
//	go test ./awo/sdui/engine/... -run TestGolden -update
//
// The -update flag rewrites all .golden.json files in testdata/.
// Commit the updated files with the change that caused them.
//
// Golden files are versioned in git. A failing golden test means the rendered
// schema changed without a deliberate update — investigate before committing.

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"awo.so/awo/sdui/amis"
	"awo.so/awo/sdui/engine"
	"awo.so/awo/sdui/generator"
	"awo.so/awo/sdui/layout"
	"awo.so/awo/sdui/observability"
	"awo.so/awo/sdui/renderer"
	"awo.so/awo/sdui/sduictx"
	"awo.so/awo/sdui/validation"
)

var update = flag.Bool("update", false, "overwrite golden files with current output")

// goldenEngine returns a deterministic engine for golden test use.
// Uses a fixed TenantID so cache keys are stable across runs.
func goldenEngine() *engine.Engine {
	return engine.New(engine.Options{
		Generator: generator.New(),
		Validator: validation.New(),
		Layout:    layout.New(),
		Renderers: map[string]renderer.Renderer{"amis": amis.New()},
		Obs:       observability.Noop(),
	})
}

// goldenCtx returns a GeneratorContext with stable fingerprints for golden tests.
func goldenCtx(entityName string, mode sduictx.ViewMode) sduictx.GeneratorContext {
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	viewer := &stubViewer{admin: true}
	ctx, _ := sduictx.NewGeneratorContext(tenantID, viewer, entityName, mode, "amis").
		WithSchemaFingerprint("golden-sf").
		WithPermFingerprint("golden-pf").
		Build()
	return ctx
}

// invoiceSchema returns a representative invoice entity schema for golden tests.
// URLs use ${id} (AMIS template variable) as adapt.FromCompiled produces in production.
// UIPrefix is set so the golden files capture realistic navigation links.
func invoiceSchema() generator.EntitySchema {
	return generator.EntitySchema{
		Name:        "finance_invoice",
		Title:       "Invoice",
		PluralTitle: "Invoices",
		ListURL:     "/api/v1/finance/invoices",
		CreateURL:   "/api/v1/finance/invoices",
		EditURL:     "/api/v1/finance/invoices/${id}",
		DetailURL:   "/api/v1/finance/invoices/${id}",
		UIPrefix:    "/ui/finance/invoices",
		Permissions: map[string]string{
			"create": "finance.invoice.create",
			"read":   "finance.invoice.read",
			"update": "finance.invoice.update",
			"delete": "finance.invoice.delete",
		},
		Fields: []generator.FieldDef{
			{Name: "number", Label: "Invoice #", FieldType: "data", InList: true, InForm: false, InDetail: true, ReadOnly: true},
			{Name: "status", Label: "Status", FieldType: "select", InList: true, InForm: true, InDetail: true},
			{Name: "amount", Label: "Amount", FieldType: "currency", InList: true, InForm: true, InDetail: true},
			{Name: "due_date", Label: "Due Date", FieldType: "date", InList: true, InForm: true, InDetail: true},
			{Name: "customer", Label: "Customer", FieldType: "data", InList: true, InForm: true, InDetail: true},
			{Name: "notes", Label: "Notes", FieldType: "long_text", InForm: true, InDetail: true},
		},
	}
}

// ── golden test cases ─────────────────────────────────────────────────────────

var goldenCases = []struct {
	name       string
	goldenFile string
	mode       sduictx.ViewMode
	schema     func() generator.EntitySchema
}{
	{
		name:       "invoice_list",
		goldenFile: "testdata/invoice_list.golden.json",
		mode:       sduictx.ViewModeList,
		schema:     invoiceSchema,
	},
	{
		name:       "invoice_create",
		goldenFile: "testdata/invoice_create.golden.json",
		mode:       sduictx.ViewModeCreate,
		schema:     invoiceSchema,
	},
	{
		name:       "invoice_edit",
		goldenFile: "testdata/invoice_edit.golden.json",
		mode:       sduictx.ViewModeEdit,
		schema:     invoiceSchema,
	},
	{
		name:       "invoice_detail",
		goldenFile: "testdata/invoice_detail.golden.json",
		mode:       sduictx.ViewModeDetail,
		schema:     invoiceSchema,
	},
}

func TestGolden(t *testing.T) {
	eng := goldenEngine()

	// Ensure testdata directory exists.
	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatalf("mkdir testdata: %v", err)
	}

	for _, tc := range goldenCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := engine.Request{
				Ctx:    goldenCtx(tc.schema().Name, tc.mode),
				Schema: tc.schema(),
			}
			resp, err := eng.Handle(context.Background(), req)
			if err != nil {
				t.Fatalf("Handle: %v", err)
			}

			got, err := json.MarshalIndent(resp.Output.AMISSchema, "", "  ")
			if err != nil {
				t.Fatalf("marshal output: %v", err)
			}
			got = append(got, '\n')

			if *update {
				if err := os.WriteFile(tc.goldenFile, got, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				t.Logf("updated %s", tc.goldenFile)
				return
			}

			want, err := os.ReadFile(tc.goldenFile)
			if err != nil {
				if os.IsNotExist(err) {
					t.Fatalf("golden file %q missing — run with -update to create it", tc.goldenFile)
				}
				t.Fatalf("read golden: %v", err)
			}

			if string(got) != string(want) {
				t.Errorf("output differs from golden %s\n"+
					"run: go test ./awo/sdui/engine/... -run TestGolden -update\n"+
					"--- got (first 500 chars) ---\n%s",
					filepath.Base(tc.goldenFile),
					truncate(string(got), 500))
			}
		})
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
