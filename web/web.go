// Package web embeds the showcase's static frontend assets directly into the
// awo binary at compile time.
//
// R.10 (docs/00-overview/TECHNICAL_FINDINGS_AND_FUTURE_IMPROVEMENTS.md §B.10):
// cmd/awo/serve_impl.go previously referenced these assets via the literal,
// process-working-directory-relative paths "./awo/web/dist" and
// "./awo/web/showcase" — paths that only happened to resolve if the server
// was launched from the parent of a directory literally named "awo" (true by
// coincidence for this checkout, false for the documented
// `go run ./cmd/awo serve` invocation run from the repository root itself).
// Embedding removes the runtime path dependency entirely: these files become
// part of the compiled binary, so serving them is identical regardless of
// the process's current working directory, for both `go run` and a built
// executable — see cmd/awo/web_assets.go.
package web

import "embed"

// DistFS holds the showcase SPA shell served at "/" and "/ui/*". Its root is
// the web/dist directory (not the repository root) — callers typically wrap
// it with fs.Sub(DistFS, "dist") to get a filesystem rooted at dist's own
// contents.
//
//go:embed dist
var DistFS embed.FS

// ShowcaseFS holds the developer-diagnostic showcase portal served at
// "/showcase*". Its root is the web/showcase directory — callers typically
// wrap it with fs.Sub(ShowcaseFS, "showcase").
//
//go:embed showcase
var ShowcaseFS embed.FS
