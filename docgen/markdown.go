package docgen

import (
	"fmt"
	"io"
	"strings"
)

// mdWriter is a minimal accumulating Markdown writer.
//
// It exists so that every Fprintf error is checked in exactly one place
// (err) instead of being checked — or silently ignored — at each call site.
// Once err is non-nil, every subsequent method becomes a no-op, so callers
// can chain writes freely and check err once at the end.
type mdWriter struct {
	w   io.Writer
	err error
}

func newMDWriter(w io.Writer) *mdWriter {
	return &mdWriter{w: w}
}

// raw writes a pre-formatted line as-is. Internal use only — all
// user-controlled content must go through row/heading, which escape it.
func (m *mdWriter) raw(format string, args ...any) {
	if m.err != nil {
		return
	}
	_, m.err = fmt.Fprintf(m.w, format, args...)
}

// heading writes a Markdown heading at the given level (1-6), optionally
// followed by a slug-based anchor comment usable by renderers that support
// custom heading IDs. text is escaped for safety; it is still just prose,
// not a table cell, so only newlines are stripped.
func (m *mdWriter) heading(level int, text string) {
	if m.err != nil {
		return
	}
	hashes := strings.Repeat("#", clampHeadingLevel(level))
	m.raw("%s %s\n\n", hashes, escapeInline(text))
}

// headingWithAnchor writes a heading whose text is expected to slug to the
// given anchor under GitHub's auto-slug rules. It does not emit a
// {#custom-id} attribute — GitHub's Markdown renderer does not support
// that syntax, so relying on it produces literal "{#...}" text in the
// heading with the TOC link pointing at GitHub's own (different) auto-slug
// instead. Passing the anchor here is a caller-side assertion, checked in
// non-production builds via panic, that the heading text and the anchor
// used to link to it were derived from the same slugify() call — see
// slug.go. If a renderer that *does* support explicit heading IDs is
// introduced later, this is the one place to add {#anchor} back in.
func (m *mdWriter) headingWithAnchor(level int, text, anchor string) {
	if m.err != nil {
		return
	}
	if got := slugify(text); anchor != "" && !strings.HasPrefix(anchor, got) {
		// Anchor and heading text have diverged — almost certainly because
		// the caller slugified different input than what ends up in the
		// heading. Fail loudly in development rather than shipping a
		// silently broken TOC link.
		panic(fmt.Sprintf("docgen: heading anchor %q does not match slug of heading text %q (slug: %q)", anchor, text, got))
	}
	m.heading(level, text)
}

// blankLine writes a single blank line, useful for separating sections.
func (m *mdWriter) blankLine() {
	m.raw("\n")
}

// rule writes a horizontal rule.
func (m *mdWriter) rule() {
	m.raw("---\n\n")
}

// tocEntry writes one line of the table of contents at the given indent
// depth (0 = top-level module, 1 = entity under a module).
func (m *mdWriter) tocEntry(depth int, text, anchor string) {
	if m.err != nil {
		return
	}
	indent := strings.Repeat("  ", depth)
	m.raw("%s- [%s](#%s)\n", indent, escapeInline(text), anchor)
}

// table writes a Markdown table given a header row and data rows. Every
// cell is escaped so embedded "|" or newlines from tenant-authored content
// (labels, descriptions, custom field names) cannot corrupt the table
// structure or bleed into adjacent rows.
func (m *mdWriter) table(header []string, rows [][]string) {
	if m.err != nil {
		return
	}
	if len(rows) == 0 {
		return
	}
	m.raw("| %s |\n", strings.Join(escapeCells(header), " | "))
	seps := make([]string, len(header))
	for i := range seps {
		seps[i] = "------"
	}
	m.raw("| %s |\n", strings.Join(seps, " | "))
	for _, row := range rows {
		m.raw("| %s |\n", strings.Join(escapeCells(row), " | "))
	}
	m.blankLine()
}

// Err returns the first write error encountered, if any.
func (m *mdWriter) Err() error {
	return m.err
}

func clampHeadingLevel(level int) int {
	if level < 1 {
		return 1
	}
	if level > 6 {
		return 6
	}
	return level
}

// escapeCells escapes a slice of table cell values.
func escapeCells(cells []string) []string {
	out := make([]string, len(cells))
	for i, c := range cells {
		out[i] = escapeCell(c)
	}
	return out
}

// escapeCell makes a string safe to embed as a single Markdown table cell:
// pipes would otherwise terminate the cell early, and raw newlines would
// otherwise break the row onto multiple lines.
func escapeCell(s string) string {
	if s == "" {
		return emptyCellPlaceholder
	}
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\r\n", "<br>")
	s = strings.ReplaceAll(s, "\n", "<br>")
	return s
}

// escapeInline escapes text destined for prose (headings, TOC labels)
// rather than a table cell — only newlines are a structural hazard there.
func escapeInline(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

const emptyCellPlaceholder = "—"
