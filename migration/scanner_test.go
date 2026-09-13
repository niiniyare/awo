package migration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
	require.NoError(t, err)
}

func TestScan_EmptyDir(t *testing.T) {
	steps, err := Scan(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, steps)
}

func TestScan_NonexistentDir(t *testing.T) {
	_, err := Scan("/does/not/exist/ever")
	assert.Error(t, err)
}

func TestScan_ParsesUpAndDown(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "20240101120000_create_users.up.sql", "CREATE TABLE users ();")
	writeFile(t, dir, "20240101120000_create_users.down.sql", "DROP TABLE users;")

	steps, err := Scan(dir)
	require.NoError(t, err)
	assert.Len(t, steps, 2)
}

func TestScan_SortedByVersion(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "20240201000000_second.up.sql", "SELECT 2;")
	writeFile(t, dir, "20240101000000_first.up.sql", "SELECT 1;")
	writeFile(t, dir, "20240301000000_third.up.sql", "SELECT 3;")

	steps, err := Scan(dir)
	require.NoError(t, err)
	require.Len(t, steps, 3)
	assert.Less(t, steps[0].Version, steps[1].Version)
	assert.Less(t, steps[1].Version, steps[2].Version)
}

func TestScan_IgnoresNonMatchingFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "docs")
	writeFile(t, dir, "20240101120000_init.up.sql", "SELECT 1;")
	writeFile(t, dir, "not_a_migration.sql", "SELECT 2;")

	steps, err := Scan(dir)
	require.NoError(t, err)
	assert.Len(t, steps, 1, "non-matching files should be silently ignored")
}

func TestScan_ChecksumPopulated(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "20240101120000_init.up.sql", "CREATE TABLE x ();")

	steps, err := Scan(dir)
	require.NoError(t, err)
	require.Len(t, steps, 1)
	assert.NotEmpty(t, steps[0].Checksum)
	assert.Len(t, steps[0].Checksum, 64, "SHA-256 hex must be 64 chars")
}

func TestScan_DirectionParsed(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "20240101120000_init.up.sql", "UP;")
	writeFile(t, dir, "20240101120000_init.down.sql", "DOWN;")

	steps, err := Scan(dir)
	require.NoError(t, err)

	dirs := make(map[Direction]bool)
	for _, s := range steps {
		dirs[s.Direction] = true
	}
	assert.True(t, dirs[DirectionUp], "should have an up step")
	assert.True(t, dirs[DirectionDown], "should have a down step")
}
