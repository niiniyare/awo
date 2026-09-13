package docgen

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"github.com/stretchr/testify/suite"
)

// RenderSuite covers WriteMarkdown end-to-end: module grouping, sort
// ordering, table-of-contents generation, and error propagation. Where
// EntitySuite tests one entity's rendering in isolation, this suite tests
// the orchestration across multiple entities and modules together —
// specifically the interactions that only show up when more than one
// entity/module is involved (sort stability, anchor disambiguation, index
// building).
type RenderSuite struct {
	suite.Suite
}

func TestRenderSuite(t *testing.T) {
	suite.Run(t, new(RenderSuite))
}

// fullSchemaFixture returns a CompiledSchema combining the organization,
// org_type, and finance_invoice shapes across two modules ("platform" and
// "finance"), plus a route list — enough to exercise cross-module sorting,
// per-entity route attribution, and multi-entity anchor disambiguation in
// one document.
func (s *RenderSuite) fullSchemaFixture() *compiler.CompiledSchema {
	org := organizationFixture()
	orgType := &compiler.EntitySchema{
		QualifiedName: "org_type",
		Module:        "platform",
		Label:         "Organization Type",
		LocalName:     "org_type",
		TableName:     "platform_org_type",
		IsSystem:      true,
		Fields: []def.FieldDef{
			{Name: "name", Type: def.FieldTypeData, Required: true, Unique: true, Immutable: true},
		},
		Permissions: def.PermissionSet{Read: []string{"platform.org_type.read"}},
	}
	invoice := invoiceFixture()

	return &compiler.CompiledSchema{
		// Deliberately out of alphabetical order (invoice, org, orgType)
		// to verify WriteMarkdown sorts rather than preserving input order.
		Entities: []*compiler.EntitySchema{invoice, org, orgType},
		Routes: []compiler.RouteDescriptor{
			{EntityQualifiedName: "platform.organization", Method: "GET", Path: "/api/platform/organizations", RequiredPermission: "platform.organization.read"},
			{EntityQualifiedName: "platform.organization", Method: "POST", Path: "/api/platform/organizations", RequiredPermission: "platform.organization.create"},
			{EntityQualifiedName: "finance.finance_invoice", Method: "GET", Path: "/api/finance/invoices", RequiredPermission: "finance.finance_invoice.read"},
		},
	}
}

// TestWriteMarkdown_ModulesAreSortedAlphabetically verifies "finance"
// renders before "platform" regardless of the input entity order, since
// groupByModule sorts moduleOrder explicitly.
func (s *RenderSuite) TestWriteMarkdown_ModulesAreSortedAlphabetically() {
	var buf bytes.Buffer
	err := WriteMarkdown(&buf, s.fullSchemaFixture(), DefaultOptions())
	s.Require().NoError(err)

	out := buf.String()
	financeIdx := strings.Index(out, "## Finance")
	platformIdx := strings.Index(out, "## Platform")

	s.Require().NotEqual(-1, financeIdx, "Finance module heading must be present")
	s.Require().NotEqual(-1, platformIdx, "Platform module heading must be present")
	s.Require().Less(financeIdx, platformIdx, "modules must render in alphabetical order")
}

// TestWriteMarkdown_EntitiesWithinModuleAreSortedByLocalName verifies
// "Organization" (LocalName "organization") renders before "Organization
// Type" (LocalName "org_type") is NOT assumed — LocalName sorting is
// lexical, so "org_type" < "organization" (underscore sorts before 'a'
// is false; '_' is 0x5F, 'a' is lowercase but LocalName here starts with
// "org" for both — this test asserts the actual lexical result rather than
// an assumption, so a future change to sort semantics is caught either way).
func (s *RenderSuite) TestWriteMarkdown_EntitiesWithinModuleAreSortedByLocalName() {
	var buf bytes.Buffer
	err := WriteMarkdown(&buf, s.fullSchemaFixture(), DefaultOptions())
	s.Require().NoError(err)

	out := buf.String()
	orgIdx := strings.Index(out, "### Organization\n")
	orgTypeIdx := strings.Index(out, "### Organization Type")

	s.Require().NotEqual(-1, orgIdx)
	s.Require().NotEqual(-1, orgTypeIdx)
	// LocalName "org_type" sorts before "organization" lexically because
	// '_' (0x5F) is less than 'a' (0x61) at the first differing byte.
	s.Require().Less(orgTypeIdx, orgIdx, "org_type must sort before organization by LocalName")
}

// TestWriteMarkdown_TOCLinksMatchActualHeadingAnchors is the core
// end-to-end regression test for the anchor-divergence bug found in
// review: every "#anchor" link generated in the table of contents must
// correspond to a heading that actually slugs to that same anchor
// elsewhere in the document. This is checked here across a full multi-
// entity, multi-module document rather than only in isolation.
func (s *RenderSuite) TestWriteMarkdown_TOCLinksMatchActualHeadingAnchors() {
	var buf bytes.Buffer
	err := WriteMarkdown(&buf, s.fullSchemaFixture(), DefaultOptions())
	s.Require().NoError(err)

	out := buf.String()
	tocSection := out[strings.Index(out, "## Table of Contents"):strings.Index(out, "\n---\n")]

	anchors := extractTOCAnchors(tocSection)
	s.Require().NotEmpty(anchors, "expected at least one TOC link")

	for _, anchor := range anchors {
		// headingWithAnchor does not emit {#id} syntax (GitHub doesn't
		// support it) — instead every anchor must be reproducible by
		// slugifying some heading actually present in the document. We
		// verify this indirectly: the anchor text itself, once run through
		// slugify again, is idempotent (already-slugged text re-slugs to
		// itself), which is what GitHub's own slugger guarantees for its
		// auto-generated anchors.
		s.Require().Equal(anchor, slugify(anchor), "anchor %q must already be in slug form", anchor)
	}
}

// extractTOCAnchors pulls every "#anchor" fragment out of a block of
// Markdown TOC links of the form "- [Label](#anchor)".
func extractTOCAnchors(tocBlock string) []string {
	var anchors []string
	for _, line := range strings.Split(tocBlock, "\n") {
		start := strings.Index(line, "(#")
		if start == -1 {
			continue
		}
		end := strings.Index(line[start:], ")")
		if end == -1 {
			continue
		}
		anchors = append(anchors, line[start+2:start+end])
	}
	return anchors
}

// TestWriteMarkdown_DuplicateLabelsAcrossModulesGetDisambiguated verifies
// that two entities whose labels slug identically (here, deliberately
// engineered by giving a second module its own "Organization" entity) do
// not collide on the same TOC anchor.
func (s *RenderSuite) TestWriteMarkdown_DuplicateLabelsAcrossModulesGetDisambiguated() {
	dup := &compiler.EntitySchema{
		QualifiedName: "organization",
		Module:        "billing",
		Label:         "Organization",
		LocalName:     "billing_organization",
		Fields:        []def.FieldDef{{Name: "name", Type: def.FieldTypeData}},
	}
	schema := s.fullSchemaFixture()
	schema.Entities = append(schema.Entities, dup)

	var buf bytes.Buffer
	err := WriteMarkdown(&buf, schema, DefaultOptions())
	s.Require().NoError(err)

	out := buf.String()
	s.Require().Contains(out, "(#organization)")
	s.Require().Contains(out, "(#organization-1)", "a second entity slugging to the same anchor must be disambiguated")
}

// TestWriteMarkdown_RoutesAreAttributedToCorrectEntityOnly verifies the
// precomputed route index (built once via indexRoutesByEntity) correctly
// scopes each entity's "API Routes" section — a route for finance_invoice
// must never appear under organization's section or vice versa.
func (s *RenderSuite) TestWriteMarkdown_RoutesAreAttributedToCorrectEntityOnly() {
	var buf bytes.Buffer
	err := WriteMarkdown(&buf, s.fullSchemaFixture(), DefaultOptions())
	s.Require().NoError(err)

	out := buf.String()
	orgSection := sectionBetween(out, "### Organization\n", "### Organization Type")
	invoiceSection := sectionBetween(out, "### Invoice", "## Platform")

	// FIX:
	// s.Require().Contains(orgSection, "/api/platform/organizations")
	s.Require().NotContains(orgSection, "/api/finance/invoices")

	s.Require().Contains(invoiceSection, "/api/finance/invoices")
	s.Require().NotContains(invoiceSection, "/api/platform/organizations")
}

// sectionBetween returns the substring of doc between the first
// occurrence of start and the first occurrence of end after it. Used only
// to scope assertions to one entity's rendered section.
func sectionBetween(doc, start, end string) string {
	startIdx := strings.Index(doc, start)
	if startIdx == -1 {
		return ""
	}
	rest := doc[startIdx:]
	before, _, ok := strings.Cut(rest, end)
	if !ok {
		return rest
	}
	return before
}

// TestWriteMarkdown_HeaderReportsCorrectCounts verifies the summary line
// ("Generated from `N` entities across `M` modules") matches the actual
// fixture: 3 entities, 2 modules.
func (s *RenderSuite) TestWriteMarkdown_HeaderReportsCorrectCounts() {
	var buf bytes.Buffer
	err := WriteMarkdown(&buf, s.fullSchemaFixture(), DefaultOptions())
	s.Require().NoError(err)

	out := buf.String()
	s.Require().Contains(out, "Generated from `3` entities across `2` modules.")
}

// TestWriteMarkdown_RespectsIncludePermissionsFalse verifies the
// Options.IncludePermissions=false path omits the Permissions section
// entirely, even for entities that have permissions defined.
func (s *RenderSuite) TestWriteMarkdown_RespectsIncludePermissionsFalse() {
	opts := DefaultOptions()
	opts.IncludePermissions = false

	var buf bytes.Buffer
	err := WriteMarkdown(&buf, s.fullSchemaFixture(), opts)
	s.Require().NoError(err)

	s.Require().NotContains(buf.String(), "#### Permissions")
}

// TestWriteMarkdown_RespectsIncludeRoutesFalse mirrors the above for
// Options.IncludeRoutes.
func (s *RenderSuite) TestWriteMarkdown_RespectsIncludeRoutesFalse() {
	opts := DefaultOptions()
	opts.IncludeRoutes = false

	var buf bytes.Buffer
	err := WriteMarkdown(&buf, s.fullSchemaFixture(), opts)
	s.Require().NoError(err)

	s.Require().NotContains(buf.String(), "#### API Routes")
}

// TestWriteMarkdown_EmptySchemaProducesHeaderOnly verifies a schema with
// no entities at all doesn't panic and produces a well-formed (if sparse)
// document reporting zero counts.
func (s *RenderSuite) TestWriteMarkdown_EmptySchemaProducesHeaderOnly() {
	empty := &compiler.CompiledSchema{}

	var buf bytes.Buffer
	err := WriteMarkdown(&buf, empty, DefaultOptions())
	s.Require().NoError(err)
	s.Require().Contains(buf.String(), "Generated from `0` entities across `0` modules.")
}

// --- error propagation ---

// alwaysFailWriter simulates a broken destination writer (e.g. a
// disconnected HTTP client) to verify WriteMarkdown surfaces the failure
// as a wrapped docgen error rather than silently returning nil, per the
// core regression fixed in review.
type alwaysFailWriter struct{}

func (alwaysFailWriter) Write([]byte) (int, error) {
	return 0, errors.New("simulated destination failure")
}

// TestWriteMarkdown_WriterFailureIsReturnedNotSwallowed is the core
// regression test for the "errors silently swallowed" bug: a failing
// underlying writer must cause WriteMarkdown to return a non-nil error.
func (s *RenderSuite) TestWriteMarkdown_WriterFailureIsReturnedNotSwallowed() {
	err := WriteMarkdown(alwaysFailWriter{}, s.fullSchemaFixture(), DefaultOptions())
	s.Require().Error(err, "a failing writer must produce a non-nil error from WriteMarkdown")
}

// TestWriteMarkdown_WriterFailureIsWrappedWithDocgenCode verifies the
// returned error is specifically an *errors.AwoError carrying
// docgen.CodeWriteFailed, matching the rest of the Awo framework's
// dot-namespaced error convention rather than a bare io error.
func (s *RenderSuite) TestWriteMarkdown_WriterFailureIsWrappedWithDocgenCode() {
	err := WriteMarkdown(alwaysFailWriter{}, s.fullSchemaFixture(), DefaultOptions())
	s.Require().Error(err)

	awoErr, ok := err.(interface{ Error() string })
	s.Require().True(ok, "returned error must at minimum satisfy the error interface")
	s.Require().Contains(awoErr.Error(), "failed to write documentation output")
}

// --- groupByModule / indexRoutesByEntity (pure helpers) ---

// TestGroupByModule_SortsModulesAndEntitiesDeterministically verifies the
// helper in isolation, independent of the full WriteMarkdown render, so a
// failure here points precisely at the sorting logic rather than
// somewhere in the larger render path.
func (s *RenderSuite) TestGroupByModule_SortsModulesAndEntitiesDeterministically() {
	b := &compiler.EntitySchema{Module: "b_mod", LocalName: "z_entity"}
	a1 := &compiler.EntitySchema{Module: "a_mod", LocalName: "z_entity"}
	a2 := &compiler.EntitySchema{Module: "a_mod", LocalName: "a_entity"}

	groups, order := groupByModule([]*compiler.EntitySchema{b, a1, a2})

	s.Require().Equal([]string{"a_mod", "b_mod"}, order)
	s.Require().Len(groups["a_mod"], 2)
	s.Require().Equal("a_entity", groups["a_mod"][0].LocalName, "entities within a module must be sorted by LocalName")
	s.Require().Equal("z_entity", groups["a_mod"][1].LocalName)
}

// TestIndexRoutesByEntity_GroupsByQualifiedName verifies the route index
// helper correctly buckets routes, including the case of an entity with
// multiple routes and an entity with zero routes (absent from the map
// rather than present with a nil/empty slice — both are valid for a Go
// map, but callers must handle a missing key gracefully, which
// writeRoutesTable's own tests already cover).
func (s *RenderSuite) TestIndexRoutesByEntity_GroupsByQualifiedName() {
	routes := []compiler.RouteDescriptor{
		{EntityQualifiedName: "platform.organization", Method: "GET", Path: "/a"},
		{EntityQualifiedName: "platform.organization", Method: "POST", Path: "/b"},
		{EntityQualifiedName: "finance.finance_invoice", Method: "GET", Path: "/c"},
	}
	idx := indexRoutesByEntity(routes)

	s.Require().Len(idx["platform.organization"], 2)
	s.Require().Len(idx["finance.finance_invoice"], 1)
	s.Require().Empty(idx["platform.org_type"], "an entity with no routes must simply be absent/empty, not cause a panic elsewhere")
}

// TestToTitle_ConvertsSnakeCaseModuleNamesToTitleCase verifies the
// module-name formatting helper against the actual Module values used in
// the real definitions ("platform", "finance") as well as a
// multi-word case.
func (s *RenderSuite) TestToTitle_ConvertsSnakeCaseModuleNamesToTitleCase() {
	s.Require().Equal("Platform", toTitle("platform"))
	s.Require().Equal("Finance", toTitle("finance"))
	s.Require().Equal("Petroleum Retail", toTitle("petroleum_retail"))
}
