package docgen

import (
	"io"
	"sort"
	"strings"

	"awo.so/awo/compiler"
)

// WriteMarkdown writes a full Markdown API reference for the compiled
// schema to w. Entities are grouped by module and sorted alphabetically
// within each group. Any write failure is returned wrapped as a
// docgen.CodeWriteFailed error.
func WriteMarkdown(w io.Writer, schema *compiler.CompiledSchema, opts Options) error {
	groups, moduleOrder := groupByModule(schema.Entities)
	routesByEntity := indexRoutesByEntity(schema.Routes)
	slugs := newEntitySlugs()

	m := newMDWriter(w)
	m.heading(1, "Awo Entity Reference")
	m.raw("Generated from `%d` entities across `%d` modules.\n\n", len(schema.Entities), len(moduleOrder))

	writeTableOfContents(m, groups, moduleOrder, slugs)
	m.rule()

	// A fresh slug tracker for headings: TOC and headings must agree on
	// the anchor for a given label, so they share the exact same
	// disambiguation sequence. Reusing `slugs` (not creating a second
	// tracker) is what keeps the two in sync — see slugify's doc comment.
	for _, mod := range moduleOrder {
		moduleTitle := toTitle(mod)
		m.headingWithAnchor(2, moduleTitle, slugs.anchorFor(moduleTitle))
		for _, es := range groups[mod] {
			writeEntitySection(m, es, routesByEntity, opts, slugs)
		}
	}

	return wrapWriteErr(m.Err())
}

// groupByModule buckets entities by Module and returns modules in a stable,
// alphabetically sorted order, with entities within each module also
// sorted alphabetically by LocalName. Sorting happens exactly once, here,
// so the table-of-contents pass and the section-rendering pass can never
// disagree about ordering — the previous implementation relied on the TOC
// loop's in-place sort incidentally mutating the same backing array the
// later render loop read, which only worked because of loop ordering.
func groupByModule(entities []*compiler.EntitySchema) (map[string][]*compiler.EntitySchema, []string) {
	groups := make(map[string][]*compiler.EntitySchema)
	var moduleOrder []string
	for _, es := range entities {
		if _, seen := groups[es.Module]; !seen {
			moduleOrder = append(moduleOrder, es.Module)
		}
		groups[es.Module] = append(groups[es.Module], es)
	}
	sort.Strings(moduleOrder)
	for _, mod := range moduleOrder {
		entities := groups[mod]
		sort.Slice(entities, func(i, j int) bool {
			return entities[i].LocalName < entities[j].LocalName
		})
	}
	return groups, moduleOrder
}

// writeTableOfContents renders the two-level TOC (module, then entities
// within it), using the exact same slug tracker that heading rendering
// will use later, so every link resolves.
func writeTableOfContents(m *mdWriter, groups map[string][]*compiler.EntitySchema, moduleOrder []string, slugs *entitySlugs) {
	m.heading(2, "Table of Contents")
	for _, mod := range moduleOrder {
		moduleTitle := toTitle(mod)
		m.tocEntry(0, moduleTitle, slugs.anchorFor(moduleTitle))
		for _, es := range groups[mod] {
			m.tocEntry(1, es.Label, slugs.anchorFor(es.Label))
		}
	}
}

// indexRoutesByEntity builds an entity→routes lookup in a single pass over
// schema.Routes, so writeRoutesTable doesn't rescan the full route list
// once per entity.
func indexRoutesByEntity(routes []compiler.RouteDescriptor) map[string][]compiler.RouteDescriptor {
	idx := make(map[string][]compiler.RouteDescriptor)
	for _, r := range routes {
		idx[r.EntityQualifiedName] = append(idx[r.EntityQualifiedName], r)
	}
	return idx
}

func toTitle(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}
