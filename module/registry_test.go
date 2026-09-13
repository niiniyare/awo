package module

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mkManifest(name string, deps ...string) Manifest {
	d := make([]Dependency, 0, len(deps))
	for _, dep := range deps {
		d = append(d, Dependency{Module: dep, Minimum: Version{1, 0, 0}})
	}
	return Manifest{Name: name, Version: Version{1, 0, 0}, Label: name, DependsOn: d}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := New()
	require.NoError(t, r.Register(mkManifest("finance")))
	m, ok := r.Get("finance")
	require.True(t, ok)
	assert.Equal(t, "finance", m.Name)
}

func TestRegistry_DuplicateRegister(t *testing.T) {
	r := New()
	require.NoError(t, r.Register(mkManifest("finance")))
	assert.Error(t, r.Register(mkManifest("finance")))
}

func TestRegistry_EmptyName(t *testing.T) {
	r := New()
	assert.Error(t, r.Register(Manifest{}))
}

func TestRegistry_Resolve_NoDeps(t *testing.T) {
	r := New()
	require.NoError(t, r.Register(mkManifest("a")))
	require.NoError(t, r.Register(mkManifest("b")))
	ms, err := r.Resolve()
	require.NoError(t, err)
	assert.Len(t, ms, 2)
}

func TestRegistry_Resolve_DepsFirst(t *testing.T) {
	r := New()
	require.NoError(t, r.Register(mkManifest("platform")))
	require.NoError(t, r.Register(mkManifest("finance", "platform")))
	require.NoError(t, r.Register(mkManifest("hr", "platform")))
	require.NoError(t, r.Register(mkManifest("payroll", "hr", "finance")))

	ms, err := r.Resolve()
	require.NoError(t, err)

	pos := make(map[string]int, len(ms))
	for i, m := range ms {
		pos[m.Name] = i
	}
	assert.Less(t, pos["platform"], pos["finance"], "platform must precede finance")
	assert.Less(t, pos["platform"], pos["hr"], "platform must precede hr")
	assert.Less(t, pos["finance"], pos["payroll"], "finance must precede payroll")
	assert.Less(t, pos["hr"], pos["payroll"], "hr must precede payroll")
}

func TestRegistry_Resolve_MissingDep(t *testing.T) {
	r := New()
	require.NoError(t, r.Register(mkManifest("finance", "platform")))
	_, err := r.Resolve()
	assert.Error(t, err, "unregistered dependency should cause error")
}

func TestRegistry_Resolve_VersionTooLow(t *testing.T) {
	r := New()
	platform := Manifest{Name: "platform", Version: Version{1, 0, 0}, Label: "Platform"}
	finance := Manifest{
		Name: "finance", Version: Version{1, 0, 0}, Label: "Finance",
		DependsOn: []Dependency{{Module: "platform", Minimum: Version{2, 0, 0}}},
	}
	require.NoError(t, r.Register(platform))
	require.NoError(t, r.Register(finance))
	_, err := r.Resolve()
	assert.Error(t, err, "installed version below minimum should cause error")
}

func TestRegistry_Register_CycleDetected(t *testing.T) {
	r := New()
	a := Manifest{Name: "a", Version: Version{1, 0, 0}, Label: "A",
		DependsOn: []Dependency{{Module: "b", Minimum: Version{1, 0, 0}}}}
	b := Manifest{Name: "b", Version: Version{1, 0, 0}, Label: "B",
		DependsOn: []Dependency{{Module: "a", Minimum: Version{1, 0, 0}}}}
	require.NoError(t, r.Register(a))
	assert.Error(t, r.Register(b), "cycle a→b→a should be detected on registration")
}

func TestRegistry_All(t *testing.T) {
	r := New()
	require.NoError(t, r.Register(mkManifest("c")))
	require.NoError(t, r.Register(mkManifest("a")))
	require.NoError(t, r.Register(mkManifest("b")))
	all := r.All()
	require.Len(t, all, 3)
	assert.Equal(t, "a", all[0].Name)
	assert.Equal(t, "b", all[1].Name)
	assert.Equal(t, "c", all[2].Name)
}
