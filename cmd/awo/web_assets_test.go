package main

import (
	"io"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// R.10 — cmd/awo/serve_impl.go previously served the showcase frontend from
// literal, process-cwd-relative paths ("./awo/web/dist", "./awo/web/showcase")
// that only resolved by coincidence when launched from the parent of a
// directory literally named "awo". mountWebAssets (web_assets.go) replaces
// that with compile-time-embedded assets (awo.so/awo/web). These tests prove
// asset resolution no longer depends on the process's working directory, and
// that a missing asset produces a clear 404, not a misleading 200. See
// docs/00-overview/TECHNICAL_FINDINGS_AND_FUTURE_IMPROVEMENTS.md §B.10.

func newWebAssetsTestApp(t *testing.T) *fiber.App {
	t.Helper()
	app := fiber.New()
	if err := mountWebAssets(app); err != nil {
		t.Fatalf("mountWebAssets: %v", err)
	}
	return app
}

// chdirElsewhere changes the process's working directory to a freshly
// created, unrelated temp directory for the duration of the test, and
// restores the original on cleanup. This directly exercises acceptance
// criterion 2 ("asset resolution does not silently depend on an undocumented
// current working directory") — if mountWebAssets ever regresses to reading
// a runtime-relative path, this test fails regardless of what the original
// cwd happened to be named.
func chdirElsewhere(t *testing.T) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir(%s): %v", tmp, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatalf("restore Chdir(%s): %v", orig, err)
		}
	})
}

func TestMountWebAssets_RootIndex_ServedRegardlessOfWorkingDirectory(t *testing.T) {
	chdirElsewhere(t)
	app := newWebAssetsTestApp(t)

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("GET / = %d, want 200 (asset resolution must not depend on cwd)", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if len(body) == 0 {
		t.Fatal("GET / returned 200 with an empty body")
	}
}

func TestMountWebAssets_UIRoute_ServedRegardlessOfWorkingDirectory(t *testing.T) {
	chdirElsewhere(t)
	app := newWebAssetsTestApp(t)

	resp, err := app.Test(httptest.NewRequest("GET", "/ui/business/expenses", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("GET /ui/business/expenses = %d, want 200", resp.StatusCode)
	}
}

func TestMountWebAssets_ShowcaseRoute_ServedRegardlessOfWorkingDirectory(t *testing.T) {
	chdirElsewhere(t)
	app := newWebAssetsTestApp(t)

	resp, err := app.Test(httptest.NewRequest("GET", "/showcase", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("GET /showcase = %d, want 200", resp.StatusCode)
	}
}

func TestMountWebAssets_UnknownStaticAsset_ReturnsClearNotFound_NotMisleadingSuccess(t *testing.T) {
	chdirElsewhere(t)
	app := newWebAssetsTestApp(t)

	resp, err := app.Test(httptest.NewRequest("GET", "/this-asset-does-not-exist.js", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	// filesystem.New falls through to Fiber's unmatched-route fallback when
	// the asset isn't found (no NotFoundFile configured for raw asset
	// paths) — the real response must be a 404, never a silent 200 that
	// would hide a genuinely missing file.
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("GET /this-asset-does-not-exist.js = %d, want 404 (missing assets must fail clearly)", resp.StatusCode)
	}
}
