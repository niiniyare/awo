package docgen

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// SlugSuite covers slugify and disambiguate: the single shared anchor
// algorithm that both the table of contents and section headings depend
// on. A regression here silently breaks every "jump to entity" link in
// generated docs, so this is tested in isolation from rendering.
type SlugSuite struct {
	suite.Suite
}

func TestSlugSuite(t *testing.T) {
	suite.Run(t, new(SlugSuite))
}

// TestSlugify_BasicLowercasing checks the common case: a normal title-case
// heading collapses to a lowercase, hyphenated anchor.
func (s *SlugSuite) TestSlugify_BasicLowercasing() {
	got := slugify("Organization Type")
	// Require, not Assert: every other assertion in this suite assumes
	// slugify behaves correctly, so a failure here should stop the test
	// immediately rather than cascade into confusing follow-on failures.
	s.Require().Equal("organization-type", got)
}

// TestSlugify_UnderscoresBecomeHyphens verifies that module names written
// with underscores (as EntityDefinition.Module values are, e.g.
// "platform", "finance") slug the same way spaces do, so a TOC entry
// derived from toTitle(mod) and one derived from the raw mod string never
// diverge.
func (s *SlugSuite) TestSlugify_UnderscoresBecomeHyphens() {
	s.Require().Equal("org-assignment", slugify("org_assignment"))
	s.Require().Equal("org-assignment", slugify("Org Assignment"))
}

// TestSlugify_PunctuationIsDropped verifies punctuation is stripped
// entirely rather than turned into hyphens, matching GitHub's own slugger
// (e.g. "API Routes (v2)" -> "api-routes-v2", not "api-routes--v2-").
func (s *SlugSuite) TestSlugify_PunctuationIsDropped() {
	s.Require().Equal("api-routes-v2", slugify("API Routes (v2)"))
	s.Require().Equal("invoice-lines", slugify("Invoice: Lines"))
}

// TestSlugify_CollapsesRepeatedSeparators ensures that multiple spaces,
// underscores, or hyphens in a row collapse to a single hyphen instead of
// producing "double-hyphen" anchors that GitHub's renderer would not
// reproduce.
func (s *SlugSuite) TestSlugify_CollapsesRepeatedSeparators() {
	s.Require().Equal("a-b", slugify("A   B"))
	s.Require().Equal("a-b", slugify("A___B"))
	s.Require().Equal("a-b", slugify("A - - B"))
}

// TestSlugify_TrimsLeadingAndTrailingHyphens ensures text that starts or
// ends with punctuation/whitespace doesn't leak a stray leading or
// trailing hyphen into the anchor.
func (s *SlugSuite) TestSlugify_TrimsLeadingAndTrailingHyphens() {
	s.Require().Equal("hello", slugify("  Hello!!"))
	s.Require().Equal("hello", slugify("Hello  "))
	s.Require().Equal("hello", slugify("_Hello_"))
}

// TestSlugify_EmptyAndPunctuationOnlyInput checks the degenerate case of
// a heading with no sluggable characters at all, which must not panic and
// must return an empty string rather than a stray hyphen.
func (s *SlugSuite) TestSlugify_EmptyAndPunctuationOnlyInput() {
	s.Require().Equal("", slugify(""))
	s.Require().Equal("", slugify("!!!"))
}

// TestDisambiguate_FirstOccurrenceIsUnsuffixed verifies the first time a
// slug is seen it is returned unchanged — matching GitHub, where the first
// "## Overview" heading gets #overview, not #overview-1.
func (s *SlugSuite) TestDisambiguate_FirstOccurrenceIsUnsuffixed() {
	seen := make(map[string]int)
	got := disambiguate("overview", seen)
	s.Require().Equal("overview", got)
}

// TestDisambiguate_RepeatedSlugsGetIncrementingSuffixes verifies that two
// entities/modules whose labels happen to slug identically (e.g. an
// "Org Type" entity in one module and an "org-type" heading elsewhere)
// don't silently collide on the same anchor — the second and third
// occurrences must get distinct, incrementing suffixes.
func (s *SlugSuite) TestDisambiguate_RepeatedSlugsGetIncrementingSuffixes() {
	seen := make(map[string]int)
	first := disambiguate("overview", seen)
	second := disambiguate("overview", seen)
	third := disambiguate("overview", seen)

	s.Require().Equal("overview", first)
	s.Require().Equal("overview-1", second)
	s.Require().Equal("overview-2", third)
}

// TestDisambiguate_IndependentSlugsDoNotInterfere ensures the seen map is
// keyed per-slug, so disambiguating one label's repeats doesn't affect the
// counter for an unrelated label.
func (s *SlugSuite) TestDisambiguate_IndependentSlugsDoNotInterfere() {
	seen := make(map[string]int)
	disambiguate("invoice", seen)
	disambiguate("invoice", seen)
	firstOrg := disambiguate("organization", seen)

	s.Require().Equal("organization", firstOrg, "an unrelated slug's first occurrence must not inherit another slug's suffix count")
}
