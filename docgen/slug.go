package docgen

import (
	"fmt"
	"strings"
)

// slugify converts arbitrary text into a GitHub-compatible heading anchor:
// lowercased, spaces and underscores collapsed to hyphens, and any
// character outside [a-z0-9-] dropped. This mirrors the slug algorithm
// GitHub's Markdown renderer applies automatically to "## Heading Text"
// headings, so links generated here always resolve on GitHub without
// relying on {#custom-id} attribute syntax, which GitHub does not support.
//
// slug is the single source of truth for anchor generation: both the
// table-of-contents links and the section headings must derive their
// anchors by calling this same function on the same input text, or the
// two will silently diverge (this was the root cause of broken TOC links
// in the previous implementation, which slugged module names and entity
// names using two different, inconsistent rules).
func slugify(text string) string {
	var b strings.Builder
	lastWasHyphen := true // suppress leading hyphens
	for _, r := range strings.ToLower(text) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastWasHyphen = false
		case r == ' ' || r == '_' || r == '-':
			if !lastWasHyphen {
				b.WriteByte('-')
				lastWasHyphen = true
			}
		default:
			// Drop punctuation entirely, matching GitHub's slugger.
		}
	}
	return strings.TrimRight(b.String(), "-")
}

// disambiguate appends "-2", "-3", ... to a slug if it has already been
// used, matching GitHub's behavior for duplicate headings (e.g. two
// modules or entities that slug to the same text).
func disambiguate(slug string, seen map[string]int) string {
	seen[slug]++
	if n := seen[slug]; n > 1 {
		return fmt.Sprintf("%s-%d", slug, n-1)
	}
	return slug
}
