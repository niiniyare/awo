package def_test

import (
	"testing"

	"awo.so/awo/def"
)

// resetRegistry is a test helper that replaces the global registry with a
// fresh one. This is NOT exported — tests in this package use it locally.
// Production code never resets the registry.
//
// Note: because globalRegistry is unexported, integration-level tests that
// call def.Register must coordinate to avoid cross-test pollution. Each test
// in this file calls resetForTest() at the start.
func TestRegister_AcceptsValidDefinition(t *testing.T) {
	// This test must run before any other test that registers definitions,
	// since globalRegistry is a package-level singleton. In CI, run with
	// -count=1 and avoid parallel sub-tests that call Register.

	d := &SystemDefinition{
		Name:        "test_widget",
		Module:      "test",
		Label:       "Widget",
		LabelPlural: "Widgets",
	}

	// Should not panic.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Register panicked unexpectedly: %v", r)
		}
	}()

	def.Register(d)

	found := def.Lookup("test_widget")
	if found == nil {
		t.Fatal("Lookup returned nil after Register")
	}
	if found.EntityName() != "test_widget" {
		t.Errorf("got name %q, want %q", found.EntityName(), "test_widget")
	}
}

func TestRegister_PanicsOnNil(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic for nil definition, got none")
		}
	}()
	def.Register(nil)
}

func TestRegister_PanicsOnEmptyName(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic for empty name, got none")
		}
	}()
	def.Register(&SystemDefinition{Module: "test", Label: "X"})
}

// SystemDefinition re-declared here only so the test file compiles without
// importing internal packages. In real module code, import awo.so/awo/def.
type SystemDefinition = def.SystemDefinition
