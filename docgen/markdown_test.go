package docgen

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"
)

// MarkdownSuite covers mdWriter: the format-specific mechanics layer that
// knows how to emit safe Markdown. These tests intentionally never touch
// compiler.EntitySchema — that separation is the point of the rewrite (see
// doc.go), so this suite proves the mechanics layer is correct entirely on
// its own terms.
type MarkdownSuite struct {
	suite.Suite

	buf *bytes.Buffer
	m   *mdWriter
}

func TestMarkdownSuite(t *testing.T) {
	suite.Run(t, new(MarkdownSuite))
}

// SetupTest runs before every test method, giving each test a fresh buffer
// and writer so tests can't leak state into one another.
func (s *MarkdownSuite) SetupTest() {
	s.buf = &bytes.Buffer{}
	s.m = newMDWriter(s.buf)
}

// --- escapeCell ---

// TestEscapeCell_EmptyStringBecomesPlaceholder verifies an empty field
// (e.g. a FieldDef with no Description) renders as the em-dash placeholder
// rather than an empty, visually broken table cell.
func (s *MarkdownSuite) TestEscapeCell_EmptyStringBecomesPlaceholder() {
	s.Require().Equal(emptyCellPlaceholder, escapeCell(""))
}

// TestEscapeCell_PipeIsEscaped is the core regression test for the bug
// found in review: a tenant-authored description containing "|" must not
// be allowed to terminate the table cell early and corrupt the row.
func (s *MarkdownSuite) TestEscapeCell_PipeIsEscaped() {
	got := escapeCell("Weird | pipe here")
	s.Require().Equal(`Weird \| pipe here`, got)
	s.Require().NotContains(got, "| pipe here |", "an unescaped pipe would visually split the cell in two")
}

// TestEscapeCell_NewlinesBecomeBreaks verifies embedded newlines (which
// would otherwise break a Markdown table row across multiple lines) are
// converted to an inline <br>, preserving both the row structure and the
// line-break intent of the original text.
func (s *MarkdownSuite) TestEscapeCell_NewlinesBecomeBreaks() {
	s.Require().Equal("line one<br>line two", escapeCell("line one\nline two"))
	s.Require().Equal("line one<br>line two", escapeCell("line one\r\nline two"),
		"CRLF must be normalized the same as LF, not left as a stray \\r")
}

// TestEscapeCell_BackslashIsEscapedFirst verifies escaping order: if a
// cell already contains a literal backslash, it must be escaped before the
// pipe-escaping step runs, or a description like "a\|b" (backslash then
// pipe) could be misread as an already-escaped pipe by a downstream
// renderer.
func (s *MarkdownSuite) TestEscapeCell_BackslashIsEscapedFirst() {
	got := escapeCell(`a\|b`)
	s.Require().Equal(`a\\\|b`, got)
}

// TestEscapeCell_CombinedHostileInput exercises pipes, backslashes, and
// newlines together, matching the kind of adversarial-but-plausible input
// a tenant admin might type into a custom field's Description.
func (s *MarkdownSuite) TestEscapeCell_CombinedHostileInput() {
	input := "Contains | a pipe\nand a newline\\ and a backslash"
	got := escapeCell(input)
	s.Require().NotContains(got, "\n", "no raw newline may survive into a table cell")
	s.Require().Contains(got, `\|`, "the pipe must be escaped")
	s.Require().Contains(got, "<br>", "the newline must become a <br>")
}

// --- escapeCells ---

// TestEscapeCells_PreservesOrderAndLength verifies escapeCells maps over
// a header/row slice without reordering or dropping any cell.
func (s *MarkdownSuite) TestEscapeCells_PreservesOrderAndLength() {
	in := []string{"a", "b | c", ""}
	got := escapeCells(in)
	s.Require().Len(got, 3)
	s.Require().Equal("a", got[0])
	s.Require().Equal(`b \| c`, got[1])
	s.Require().Equal(emptyCellPlaceholder, got[2])
}

// --- escapeInline ---

// TestEscapeInline_StripsNewlinesFromProse verifies heading/TOC text (not
// a table cell) still has the one structural hazard relevant to it —
// embedded newlines splitting a single-line heading — neutralized.
func (s *MarkdownSuite) TestEscapeInline_StripsNewlinesFromProse() {
	s.Require().Equal("a b", escapeInline("a\nb"))
	s.Require().Equal("a b", escapeInline("a\r\nb"))
}

// TestEscapeInline_DoesNotTouchPipes verifies inline text is deliberately
// less aggressive than cell escaping: a heading is prose, not a table row,
// so a literal "|" in a heading is not a structural hazard and must be
// left alone.
func (s *MarkdownSuite) TestEscapeInline_DoesNotTouchPipes() {
	s.Require().Equal("A | B", escapeInline("A | B"))
}

// --- mdWriter.table ---

// TestTable_EmptyRowsProducesNoOutput verifies a section with zero rows
// (e.g. an entity with no Edges) renders nothing at all, rather than an
// empty header-only table — this is what lets writeEdgesTable etc. skip
// emitting a heading for sections with no content.
func (s *MarkdownSuite) TestTable_EmptyRowsProducesNoOutput() {
	s.m.table([]string{"Name", "Type"}, nil)
	s.Require().NoError(s.m.Err())
	s.Require().Empty(s.buf.String())
}

// TestTable_HeaderAndRowsAreEscapedAndFormatted verifies the full render
// path: header cells and every row cell pass through escaping, and the
// separator row has one "------" per column.
func (s *MarkdownSuite) TestTable_HeaderAndRowsAreEscapedAndFormatted() {
	s.m.table(
		[]string{"Name", "Description"},
		[][]string{
			{"code", "contains a | pipe"},
			{"name", ""},
		},
	)
	s.Require().NoError(s.m.Err())

	out := s.buf.String()
	s.Require().Contains(out, "| Name | Description |")
	s.Require().Contains(out, "| ------ | ------ |")
	s.Require().Contains(out, `| code | contains a \| pipe |`)
	s.Require().Contains(out, "| name | "+emptyCellPlaceholder+" |")
}

// --- mdWriter error accumulation ---

// failingWriter always returns an error, simulating a broken io.Writer
// (e.g. a client that disconnected mid-response).
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("boom: simulated write failure")
}

// TestMDWriter_ErrIsCapturedAndStopsFurtherWrites is the core regression
// test for the "errors silently swallowed" bug found in review: once the
// underlying writer fails, every subsequent method call must become a
// no-op, and Err() must return the original failure rather than nil.
func (s *MarkdownSuite) TestMDWriter_ErrIsCapturedAndStopsFurtherWrites() {
	m := newMDWriter(failingWriter{})

	m.heading(1, "Title")
	s.Require().Error(m.Err(), "the first write should have failed and been captured")

	// None of these subsequent calls should panic or attempt to write
	// again — they must all silently no-op once err is set.
	m.raw("more text")
	m.table([]string{"a"}, [][]string{{"b"}})
	m.blankLine()
	m.rule()
	m.tocEntry(0, "x", "x")

	s.Require().Error(m.Err(), "err must remain set, not be cleared by later calls")
}

// TestMDWriter_ErrIsNilOnHappyPath is the converse of the above: a writer
// that never fails must leave Err() nil, so WriteMarkdown's wrapWriteErr
// call correctly returns nil for the common case.
func (s *MarkdownSuite) TestMDWriter_ErrIsNilOnHappyPath() {
	s.m.heading(1, "Title")
	s.m.raw("some text\n")
	s.m.blankLine()
	s.m.rule()

	s.Require().NoError(s.m.Err())
	s.Require().NotEmpty(s.buf.String())
}

// --- mdWriter.heading / headingWithAnchor ---

// TestHeading_LevelIsClampedToValidRange verifies clampHeadingLevel keeps
// heading(0, ...) and heading(9, ...) from producing invalid Markdown
// (zero or seven-plus "#" characters).
func (s *MarkdownSuite) TestHeading_LevelIsClampedToValidRange() {
	s.Require().Equal(1, clampHeadingLevel(0))
	s.Require().Equal(1, clampHeadingLevel(-5))
	s.Require().Equal(6, clampHeadingLevel(7))
	s.Require().Equal(6, clampHeadingLevel(100))
	s.Require().Equal(3, clampHeadingLevel(3), "a valid level must pass through unchanged")
}

// TestHeadingWithAnchor_PanicsOnMismatch is the core regression test for
// the TOC/heading divergence bug found in review: if a caller passes an
// anchor that does not match the slug of the heading text it's attached
// to, headingWithAnchor must fail loudly (panic) in development rather
// than silently shipping a broken link.
func (s *MarkdownSuite) TestHeadingWithAnchor_PanicsOnMismatch() {
	s.Require().Panics(func() {
		s.m.headingWithAnchor(3, "Organization", "totally-wrong-anchor")
	})
}

// TestHeadingWithAnchor_SucceedsWhenAnchorMatchesSlug verifies the
// non-panic path: an anchor correctly derived via slugify() from the same
// text is accepted and the heading is written normally.
func (s *MarkdownSuite) TestHeadingWithAnchor_SucceedsWhenAnchorMatchesSlug() {
	s.Require().NotPanics(func() {
		s.m.headingWithAnchor(3, "Organization Type", slugify("Organization Type"))
	})
	s.Require().NoError(s.m.Err())
	s.Require().Contains(s.buf.String(), "### Organization Type")
}

// TestHeadingWithAnchor_EmptyAnchorSkipsAssertion verifies passing an
// empty anchor (used when no anchor tracking applies) does not trigger
// the mismatch panic, since there is nothing to validate against.
func (s *MarkdownSuite) TestHeadingWithAnchor_EmptyAnchorSkipsAssertion() {
	s.Require().NotPanics(func() {
		s.m.headingWithAnchor(2, "Untracked Heading", "")
	})
}
