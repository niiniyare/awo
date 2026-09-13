package compiler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/def"
	"awo.so/awo/registry"
)

// helpers ----------------------------------------------------------------

func mustCompile(defs []def.EntityDefinition) *CompiledSchema {
	reg, err := registry.BuildFrom(defs)
	if err != nil {
		panic(err)
	}
	s, err := Compile(reg)
	if err != nil {
		panic(err)
	}
	return s
}

func simpleDef(name, module string) *def.SystemDefinition {
	return &def.SystemDefinition{
		Name:        name,
		Module:      module,
		Label:       name,
		LabelPlural: name + "s",
	}
}

func linkedDef(name, module, linkField, target string) *def.SystemDefinition {
	return &def.SystemDefinition{
		Name:        name,
		Module:      module,
		Label:       name,
		LabelPlural: name + "s",
		Fields: []def.FieldDef{
			{Name: linkField, Type: def.FieldTypeLink, LinkTarget: target},
		},
	}
}

// tests ------------------------------------------------------------------

func TestDepGraph_NoDeps(t *testing.T) {
	a := simpleDef("mod_a", "mod")
	b := simpleDef("mod_b", "mod")
	s := mustCompile([]def.EntityDefinition{a, b})
	g := Build(s)

	assert.False(t, g.DependsOn("mod_a", "mod_b"))
	assert.False(t, g.DependsOn("mod_b", "mod_a"))
}

func TestDepGraph_DirectDep(t *testing.T) {
	base := simpleDef("mod_base", "mod")
	child := linkedDef("mod_child", "mod", "parent", "mod_base")
	s := mustCompile([]def.EntityDefinition{base, child})
	g := Build(s)

	assert.True(t, g.DependsOn("mod_child", "mod_base"))
	assert.False(t, g.DependsOn("mod_base", "mod_child"))
}

func TestDepGraph_TransitiveDep(t *testing.T) {
	a := simpleDef("mod_a", "mod")
	b := linkedDef("mod_b", "mod", "link_a", "mod_a")
	c := linkedDef("mod_c", "mod", "link_b", "mod_b")
	s := mustCompile([]def.EntityDefinition{a, b, c})
	g := Build(s)

	assert.True(t, g.DependsOn("mod_c", "mod_a"), "mod_c should transitively depend on mod_a")
}

func TestDepGraph_TopologicalOrder(t *testing.T) {
	a := simpleDef("mod_a", "mod")
	b := linkedDef("mod_b", "mod", "link_a", "mod_a")
	c := linkedDef("mod_c", "mod", "link_b", "mod_b")
	s := mustCompile([]def.EntityDefinition{a, b, c})
	g := Build(s)

	order, err := g.TopologicalOrder()
	require.NoError(t, err)

	pos := make(map[string]int, len(order))
	for i, n := range order {
		pos[n] = i
	}
	assert.Less(t, pos["mod_a"], pos["mod_b"], "mod_a must precede mod_b")
	assert.Less(t, pos["mod_b"], pos["mod_c"], "mod_b must precede mod_c")
}

func TestDepGraph_Dependents(t *testing.T) {
	a := simpleDef("mod_a", "mod")
	b := linkedDef("mod_b", "mod", "link_a", "mod_a")
	c := linkedDef("mod_c", "mod", "link_a2", "mod_a")
	s := mustCompile([]def.EntityDefinition{a, b, c})
	g := Build(s)

	deps := g.Dependents("mod_a")
	assert.Len(t, deps, 2, "mod_a should have 2 dependents: mod_b and mod_c")
}
