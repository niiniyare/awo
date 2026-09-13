package engine_test

// Fuzz tests for the SDUI engine.
//
// Malformed or adversarial inputs must never panic the framework.
// Errors are expected and acceptable; panics are not.
//
// Run: go test -fuzz=FuzzEngine_Handle ./awo/sdui/engine/...

import (
	"context"
	"testing"
	"unicode/utf8"

	"awo.so/awo/sdui/engine"
	"awo.so/awo/sdui/generator"
	"awo.so/awo/sdui/sduictx"
)

// FuzzEngine_FieldTypes fuzzes the FieldType field of FieldDef.
// The engine must not panic for any field type string.
func FuzzEngine_FieldTypes(f *testing.F) {
	// Seed corpus: known field types.
	knownTypes := []string{
		"data", "int", "float", "currency", "select", "multi_select",
		"bool", "date", "datetime", "long_text", "rich_text", "json",
		"file", "link", "tree_link", "duration", "color", "signature",
		"", "UNKNOWN", "'; DROP TABLE entities; --", "<script>alert(1)</script>",
		"data\x00null", string([]byte{0xFF, 0xFE}),
	}
	for _, t := range knownTypes {
		f.Add(t)
	}

	eng := makeEngine(nil)

	f.Fuzz(func(t *testing.T, fieldType string) {
		if !utf8.ValidString(fieldType) {
			return // skip invalid UTF-8 — JSON encoding would reject it anyway
		}
		schema := generator.EntitySchema{
			Name:      "fuzz_entity",
			Title:     "Fuzz",
			CreateURL: "/api/v1/fuzz",
			Fields: []generator.FieldDef{
				{Name: "f1", Label: "F1", FieldType: fieldType, InForm: true},
			},
		}
		req := engine.Request{
			Ctx:    makeCtx(sduictx.ViewModeCreate),
			Schema: schema,
		}
		// Must not panic. Errors are acceptable.
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic with fieldType=%q: %v", fieldType, r)
				}
			}()
			_, _ = eng.Handle(context.Background(), req)
		}()
	})
}

// FuzzEngine_FieldNames fuzzes the Name field of FieldDef.
// Empty names should produce a validation error, not a panic.
func FuzzEngine_FieldNames(f *testing.F) {
	f.Add("normal_field")
	f.Add("")
	f.Add("a b c")
	f.Add("field\nwith\nnewlines")
	f.Add("SELECT * FROM users")
	f.Add(string(make([]byte, 1024))) // large name

	eng := makeEngine(nil)

	f.Fuzz(func(t *testing.T, name string) {
		if !utf8.ValidString(name) {
			return
		}
		schema := generator.EntitySchema{
			Name:      "fuzz_entity",
			Title:     "Fuzz",
			CreateURL: "/api/v1/fuzz",
			Fields: []generator.FieldDef{
				{Name: name, Label: "Field", FieldType: "data", InForm: true},
			},
		}
		req := engine.Request{
			Ctx:    makeCtx(sduictx.ViewModeCreate),
			Schema: schema,
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic with name=%q: %v", name, r)
				}
			}()
			_, _ = eng.Handle(context.Background(), req)
		}()
	})
}

// FuzzEngine_SchemaName fuzzes the entity schema name.
// The pipeline must not panic for any schema name.
func FuzzEngine_SchemaName(f *testing.F) {
	f.Add("finance_invoice")
	f.Add("")
	f.Add("a")
	f.Add(string(make([]byte, 256)))
	f.Add("entity with spaces")
	f.Add("entity/with/slashes")

	eng := makeEngine(nil)

	f.Fuzz(func(t *testing.T, name string) {
		if !utf8.ValidString(name) {
			return
		}
		schema := generator.EntitySchema{
			Name:      name,
			Title:     "Fuzz",
			CreateURL: "/api/v1/fuzz",
			Fields: []generator.FieldDef{
				{Name: "f1", Label: "F", FieldType: "data", InForm: true},
			},
		}
		req := engine.Request{
			Ctx:    makeCtx(sduictx.ViewModeCreate),
			Schema: schema,
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic with schema name=%q: %v", name, r)
				}
			}()
			_, _ = eng.Handle(context.Background(), req)
		}()
	})
}
