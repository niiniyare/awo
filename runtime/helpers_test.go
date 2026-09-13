package runtime_test

import (
	"fmt"
	"testing"

	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"awo.so/awo/registry"
)

// buildTestSchema compiles a minimal CompiledSchema from the given definition.
// Calls t.Fatal on compile failure. Used by runtime_test package tests.
func buildTestSchema(t *testing.T, defs ...def.EntityDefinition) *compiler.CompiledSchema {
	t.Helper()
	reg, err := registry.BuildFrom(defs)
	if err != nil {
		t.Fatalf("buildTestSchema: BuildFrom: %v", err)
	}
	schema, err := compiler.Compile(reg)
	if err != nil {
		t.Fatalf("buildTestSchema: Compile: %v", err)
	}
	return schema
}

// mustBuildTestSchema is like buildTestSchema but panics instead of calling t.Fatal.
// Use in init() or package-level vars where *testing.T is unavailable.
func mustBuildTestSchema(defs ...def.EntityDefinition) *compiler.CompiledSchema {
	reg, err := registry.BuildFrom(defs)
	if err != nil {
		panic(fmt.Sprintf("mustBuildTestSchema: BuildFrom: %v", err))
	}
	schema, err := compiler.Compile(reg)
	if err != nil {
		panic(fmt.Sprintf("mustBuildTestSchema: Compile: %v", err))
	}
	return schema
}
