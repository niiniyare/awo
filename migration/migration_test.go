package migration_test

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/migration"
)

func makeFS(files map[string]string) fstest.MapFS {
	mfs := make(fstest.MapFS)
	for name, content := range files {
		mfs[name] = &fstest.MapFile{Data: []byte(content)}
	}
	return mfs
}

func TestBuildFS_SingleModule(t *testing.T) {
	// Reset registry for test isolation.
	// NOTE: this test must run in isolation from other tests that call Register.
	// In practice, use a fresh process or the exported BuildFSFrom helper.
	src := migration.Source{
		Module:   "test",
		Priority: 0,
		FS: makeFS(map[string]string{
			"001_create_foo.up.sql":   "CREATE TABLE foo (id uuid);",
			"001_create_foo.down.sql": "DROP TABLE foo;",
			"002_add_bar.up.sql":      "ALTER TABLE foo ADD COLUMN bar text;",
		}),
	}

	vfs := migration.BuildFSFrom([]migration.Source{src})

	err := fstest.TestFS(
		vfs,
		"000001_test_create_foo.up.sql",
		"000001_test_create_foo.down.sql",
		"000002_test_add_bar.up.sql",
	)
	require.NoError(t, err)
	// _ = entries
}

func TestBuildFS_DependencyOrder(t *testing.T) {
	bootstrap := migration.Source{
		Module:   "bootstrap",
		Priority: 0,
		FS:       makeFS(map[string]string{"001_ext.up.sql": "-- ext"}),
	}
	iam := migration.Source{
		Module:    "iam",
		Priority:  20,
		DependsOn: []string{"bootstrap"},
		FS:        makeFS(map[string]string{"001_user.up.sql": "-- user"}),
	}

	sources := migration.TopoSortSources([]migration.Source{iam, bootstrap})
	assert.Equal(t, "bootstrap", sources[0].Module)
	assert.Equal(t, "iam", sources[1].Module)
}

func TestBuildFS_VersionCollisionPanics(t *testing.T) {
	// Two modules with same Priority produce collision.
	a := migration.Source{
		Module:   "mod_a",
		Priority: 50,
		FS:       makeFS(map[string]string{"001_a.up.sql": "-- a"}),
	}
	b := migration.Source{
		Module:   "mod_b",
		Priority: 50,
		FS:       makeFS(map[string]string{"001_b.up.sql": "-- b"}),
	}
	assert.Panics(t, func() {
		migration.BuildFSFrom([]migration.Source{a, b})
	})
}
