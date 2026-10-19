package main

import (
	"fmt"
	"io/fs"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"

	awoweb "awo.so/awo/web"
)

// mountWebAssets wires the showcase frontend — embedded at compile time in
// awo.so/awo/web (see that package's doc comment and
// docs/00-overview/TECHNICAL_FINDINGS_AND_FUTURE_IMPROVEMENTS.md §B.10/R.10)
// — onto app:
//
//   - "/"         serves whatever asset path was requested out of web/dist
//     (the SPA's own JS/CSS/schema files), falling through to...
//   - "/ui/*"     ...the embedded dist/index.html for any SPA route.
//   - "/showcase*" serves the embedded showcase/index.html developer portal.
//
// Both roots come from the compiled-in embed.FS, never a runtime path — this
// is identical regardless of the process's current working directory, for
// both `go run ./cmd/awo serve` and a built executable. Call this AFTER
// registering the real API routes (router.Register) so a request matching a
// real API path is resolved by its own route first; this middleware then
// only ever sees requests that didn't match anything else, exactly like the
// literal-path app.Static call it replaces.
func mountWebAssets(app *fiber.App) error {
	distRoot, err := fs.Sub(awoweb.DistFS, "dist")
	if err != nil {
		return fmt.Errorf("mountWebAssets: sub dist: %w", err)
	}
	showcaseRoot, err := fs.Sub(awoweb.ShowcaseFS, "showcase")
	if err != nil {
		return fmt.Errorf("mountWebAssets: sub showcase: %w", err)
	}

	distHTTPFS := http.FS(distRoot)
	showcaseHTTPFS := http.FS(showcaseRoot)

	// Asset files (schemas/*, mock-tenants.json, vite.svg, ...). A path that
	// doesn't exist in the embedded tree falls through (filesystem.New sets
	// 404 and calls c.Next()) to the SPA catch-all below, never to a silent
	// 200 — a genuinely missing dist/index.html would then surface as a real
	// fiber.ErrNotFound from filesystem.SendFile, not a misleading success.
	app.Use("/", filesystem.New(filesystem.Config{
		Root:   distHTTPFS,
		MaxAge: 86400,
	}))
	// Web UI SPA — all /ui/* paths serve the embedded index.html.
	app.Get("/ui/*", func(c *fiber.Ctx) error {
		return filesystem.SendFile(c, distHTTPFS, "/index.html")
	})
	// Showcase developer portal.
	app.Get("/showcase*", func(c *fiber.Ctx) error {
		return filesystem.SendFile(c, showcaseHTTPFS, "/index.html")
	})
	return nil
}
