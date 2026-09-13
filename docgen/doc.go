// Package docgen generates human-readable documentation from compiled
// EntityDefinitions — Markdown API references, entity field tables, and
// RBAC permission matrices — with no manual authoring required.
//
// # Design
//
// docgen is intentionally split into three concerns that a rewrite or a
// new output format should not re-merge:
//
//  1. Rendering (this file's siblings render.go, entity.go) walks a
//     *compiler.CompiledSchema and decides *what* content belongs in the
//     docs — which fields to show, which sections apply to which entity.
//  2. Markdown mechanics (markdown.go) is a small, format-specific writer
//     that knows *how* to emit valid, safe Markdown: escaping table cells,
//     accumulating write errors, building headings. It has no knowledge of
//     EntityDefinition or CompiledSchema at all.
//  3. Anchors (slug.go) is a single shared algorithm used by both the
//     table of contents and section headings, so links never go stale
//     relative to headings. Anchors are also disambiguated (entity/module
//     names that produce the same slug get a "-2" suffix), matching how
//     GitHub itself resolves duplicate headings.
//
// If Awo ever needs a second output format (HTML for an in-app docs
// viewer, or a JSON export for tooling), add a new file implementing the
// same walk over CompiledSchema and swap in a different mechanics layer —
// don't grow Markdown-specific string formatting into the entity walk.
//
// # Sensitive and hidden fields
//
// Because Awo tenants can define their own custom fields and entities
// (see the platform_organization / platform_org_type pattern for
// metadata-driven extensibility), generated docs default to omitting any
// field marked Sensitive or Hidden. Callers that need a complete internal
// reference — e.g. platform engineering — must opt in explicitly via
// Options; public/tenant-facing doc generation should never do so.
package docgen
