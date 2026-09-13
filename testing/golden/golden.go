// Package golden provides golden file testing utilities.
//
// Golden files are stored in testdata/golden/ relative to the calling test's
// directory. Set UPDATE_GOLDEN=1 to regenerate golden files instead of
// comparing.
package golden

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const updateEnv = "UPDATE_GOLDEN"

// goldenPath returns the path to the golden file for the given name,
// relative to the test's source directory.
func goldenPath(name string) string {
	return filepath.Join("testdata", "golden", name+".golden")
}

// AssertGolden compares actual (JSON-marshalled) against a golden file.
// If UPDATE_GOLDEN=1 env var is set, updates the file instead.
func AssertGolden(t testing.TB, name string, actual any) {
	t.Helper()
	data, err := json.MarshalIndent(actual, "", "  ")
	if err != nil {
		t.Fatalf("golden: marshal actual: %v", err)
	}
	AssertGoldenText(t, name, string(data))
}

// AssertGoldenText compares actual string against a golden file.
// If UPDATE_GOLDEN=1 env var is set, updates the file instead.
func AssertGoldenText(t testing.TB, name string, actual string) {
	t.Helper()
	path := goldenPath(name)

	if os.Getenv(updateEnv) == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("golden: create dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(actual), 0o644); err != nil {
			t.Fatalf("golden: write file %s: %v", path, err)
		}
		t.Logf("golden: updated %s", path)
		return
	}

	expected, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			t.Fatalf("golden: file %s does not exist — run with %s=1 to create it", path, updateEnv)
		}
		t.Fatalf("golden: read file %s: %v", path, err)
	}

	if string(expected) != actual {
		t.Errorf("golden: %s mismatch\n--- want ---\n%s\n--- got ---\n%s", name, string(expected), actual)
	}
}
