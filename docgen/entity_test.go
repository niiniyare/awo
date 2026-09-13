package docgen

import (
	"bytes"
	"context"
	"testing"

	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"github.com/stretchr/testify/suite"
)

// EntitySuite covers writeEntitySection and its sub-renderers, using
// entity shapes modeled directly on the two real EntityDefinitions this
// package was reviewed against: platform_organization (system entity with
// hooks, a tenant-defined type field, sensitive/hidden fields) and
// finance_invoice (edges, actions, workflow triggers, a dynamic Policy
// func). Keeping fixtures close to real definitions catches shape
// mismatches that a purely synthetic fixture might miss.
type EntitySuite struct {
	suite.Suite

	buf   *bytes.Buffer
	m     *mdWriter
	slugs *entitySlugs
}

func TestEntitySuite(t *testing.T) {
	suite.Run(t, new(EntitySuite))
}

// SetupTest gives every test a fresh writer and a fresh slug tracker, since
// entitySlugs accumulates state across calls (that's how disambiguation
// works) and tests must not leak counts into one another.
func (s *EntitySuite) SetupTest() {
	s.buf = &bytes.Buffer{}
	s.m = newMDWriter(s.buf)
	s.slugs = newEntitySlugs()
}

// organizationFixture returns an EntitySchema modeled on the real
// platform_organization SystemDefinition: a system entity with a required
// unique immutable "code" field, a sensitive-adjacent tenant-defined
// "type" field, and read/write/create/delete permissions but no dynamic
// Policy.
func organizationFixture() *compiler.EntitySchema {
	return &compiler.EntitySchema{
		QualifiedName: "organization",
		Module:        "platform",
		Label:         "Organization",
		LocalName:     "organization",
		TableName:     "platform_organization",
		IsSystem:      true,
		Fields: []def.FieldDef{
			{Name: "name", Type: def.FieldTypeData, Required: true, Searchable: true, MaxLen: 255},
			{
				Name: "code", Type: def.FieldTypeData, Required: true, Unique: true, Immutable: true, MaxLen: 50,
				Description: "Used in workflow IDs, Redis keys, audit records.",
			},
			{Name: "tax_id", Type: def.FieldTypeData, Sensitive: true, Description: "national tax identifier"},
			{Name: "depth", Type: def.FieldTypeInt, Hidden: true},
		},
		Permissions: def.PermissionSet{
			Create: []string{"platform.organization.create"},
			Read:   []string{"platform.organization.read"},
			Write:  []string{"platform.organization.update"},
			Delete: []string{"platform.organization.delete"},
		},
	}
}

// invoiceFixture returns an EntitySchema modeled on the real
// finance_invoice SystemDefinition: edges, actions with a confirm message,
// a workflow trigger, and a dynamic row-level Policy func.
func invoiceFixture() *compiler.EntitySchema {
	return &compiler.EntitySchema{
		QualifiedName: "finance_invoice",
		Module:        "finance",
		Label:         "Invoice",
		LocalName:     "finance_invoice",
		TableName:     "finance_invoice",
		IsSystem:      true,
		Fields: []def.FieldDef{
			{Name: "number", Type: def.FieldTypeNamingSeries, TenantOverridable: true, ReadOnly: true},
			{Name: "customer", Type: def.FieldTypeLink, LinkTarget: "crm_customer", Required: true},
			{Name: "total_kes", Type: def.FieldTypeCurrency, Required: true},
		},
		Edges: []def.EdgeDef{
			{Name: "lines", Target: "finance_invoice_line", Type: def.EdgeOneToMany, CascadeDelete: true, OrderBy: "created_at ASC"},
		},
		Actions: []def.ActionDef{
			{
				Name: "submit", Method: def.ActionMethodPost, Permission: "role:finance.accounts_payable",
				ConfirmMessage: "Submit this invoice for approval?",
			},
			{Name: "cancel", Method: def.ActionMethodPost, Permission: "role:tenant.admin"},
		},
		WorkflowTriggers: []def.WorkflowTrigger{
			{On: def.EventOnSubmit, WorkflowFn: "InvoiceApprovalWorkflow", TaskQueue: "finance.invoice.submit"},
		},
		Permissions: def.PermissionSet{
			Create: []string{"role:finance.accounts_payable", "role:tenant.admin"},
			Read:   []string{"role:finance.viewer", "role:finance.accounts_payable", "role:tenant.admin"},
			Write:  []string{"role:finance.accounts_payable", "role:tenant.admin"},
			Delete: []string{"role:tenant.admin"},
			Policy: def.PolicyFunc(func(_ context.Context) def.Filter { return nil }),
		},
	}
}

// --- writeFieldsTable ---

// TestFieldsTable_SensitiveFieldExcludedByDefault is the core regression
// test for Options.IncludeSensitiveFields: a field marked Sensitive must
// never appear when the option is left at its zero value (false), since
// DefaultOptions() is what public/tenant-facing doc generation uses.
func (s *EntitySuite) TestFieldsTable_SensitiveFieldExcludedByDefault() {
	writeFieldsTable(s.m, organizationFixture(), DefaultOptions())
	s.Require().NoError(s.m.Err())

	out := s.buf.String()
	s.Require().NotContains(out, "tax_id", "a Sensitive field must not appear when IncludeSensitiveFields is false")
	s.Require().NotContains(out, "national tax identifier")
}

// TestFieldsTable_HiddenFieldExcludedByDefault mirrors the sensitive-field
// test for Options.IncludeHiddenFields.
func (s *EntitySuite) TestFieldsTable_HiddenFieldExcludedByDefault() {
	writeFieldsTable(s.m, organizationFixture(), DefaultOptions())
	s.Require().NoError(s.m.Err())
	s.Require().NotContains(s.buf.String(), "depth", "a Hidden field must not appear by default")
}

// TestFieldsTable_SensitiveFieldIncludedWhenOptedIn verifies platform
// engineering can still get the complete picture by explicitly opting in,
// per Options' documented contract.
func (s *EntitySuite) TestFieldsTable_SensitiveFieldIncludedWhenOptedIn() {
	opts := DefaultOptions()
	opts.IncludeSensitiveFields = true

	writeFieldsTable(s.m, organizationFixture(), opts)
	s.Require().NoError(s.m.Err())

	out := s.buf.String()
	s.Require().Contains(out, "tax_id")
	s.Require().Contains(out, "sensitive", "the sensitive constraint tag should appear in the Constraints column")
}

// TestFieldsTable_HiddenFieldIncludedWhenOptedIn mirrors the above for
// IncludeHiddenFields.
func (s *EntitySuite) TestFieldsTable_HiddenFieldIncludedWhenOptedIn() {
	opts := DefaultOptions()
	opts.IncludeHiddenFields = true

	writeFieldsTable(s.m, organizationFixture(), opts)
	s.Require().NoError(s.m.Err())
	s.Require().Contains(s.buf.String(), "depth")
}

// TestFieldsTable_NoVisibleFieldsProducesNoOutput verifies an entity whose
// only fields are all Sensitive/Hidden (and thus filtered out) renders no
// "#### Fields" heading at all, rather than an empty table.
func (s *EntitySuite) TestFieldsTable_NoVisibleFieldsProducesNoOutput() {
	onlySensitive := &compiler.EntitySchema{
		Fields: []def.FieldDef{{Name: "secret", Sensitive: true}},
	}
	writeFieldsTable(s.m, onlySensitive, DefaultOptions())
	s.Require().NoError(s.m.Err())
	s.Require().Empty(s.buf.String())
}

// TestFieldsTable_ConstraintsColumnListsAllApplicableTags verifies the
// Constraints column aggregates every applicable tag (unique, immutable,
// tenant-overridable) for a single field, matching finance_invoice's
// "number" field (TenantOverridable) and organization's "code" field
// (unique + immutable).
func (s *EntitySuite) TestFieldsTable_ConstraintsColumnListsAllApplicableTags() {
	writeFieldsTable(s.m, organizationFixture(), DefaultOptions())
	s.Require().NoError(s.m.Err())
	s.Require().Contains(s.buf.String(), "unique, immutable")
}

// TestFieldsTable_TenantOverridableTagRendered verifies the
// TenantOverridable flag — used by finance_invoice's naming-series
// "number" field to let tenants change the invoice prefix without a
// redeploy — is surfaced in the Constraints column, since it's operator-
// relevant metadata that the original implementation didn't render at all.
func (s *EntitySuite) TestFieldsTable_TenantOverridableTagRendered() {
	writeFieldsTable(s.m, invoiceFixture(), DefaultOptions())
	s.Require().NoError(s.m.Err())
	s.Require().Contains(s.buf.String(), "tenant-overridable")
}

// --- writeEdgesTable ---

// TestEdgesTable_RendersCascadeDeleteMark verifies finance_invoice's
// "lines" edge (CascadeDelete: true) shows the checkmark, since cascade
// delete is safety-critical information for anyone reading the schema.
func (s *EntitySuite) TestEdgesTable_RendersCascadeDeleteMark() {
	writeEdgesTable(s.m, invoiceFixture())
	s.Require().NoError(s.m.Err())

	out := s.buf.String()
	s.Require().Contains(out, "`lines`")
	s.Require().Contains(out, "`finance_invoice_line`")
	s.Require().Contains(out, "✓", "CascadeDelete: true must render the checkmark")
}

// TestEdgesTable_NoEdgesProducesNoOutput verifies an entity with zero
// Edges (e.g. organization) renders nothing for this section.
func (s *EntitySuite) TestEdgesTable_NoEdgesProducesNoOutput() {
	writeEdgesTable(s.m, organizationFixture())
	s.Require().NoError(s.m.Err())
	s.Require().Empty(s.buf.String())
}

// --- writeActionsTable ---

// TestActionsTable_RendersConfirmMessage verifies the "submit" action's
// ConfirmMessage ("Submit this invoice for approval?") is surfaced — this
// matters operationally since it tells a reader the action is not a silent
// one-click mutation.
func (s *EntitySuite) TestActionsTable_RendersConfirmMessage() {
	writeActionsTable(s.m, invoiceFixture())
	s.Require().NoError(s.m.Err())
	s.Require().Contains(s.buf.String(), "Submit this invoice for approval?")
}

// TestActionsTable_MissingConfirmMessageRendersPlaceholder verifies an
// action with no ConfirmMessage (like "cancel") gets the empty-cell
// placeholder rather than an empty, misaligned cell.
func (s *EntitySuite) TestActionsTable_MissingConfirmMessageRendersPlaceholder() {
	writeActionsTable(s.m, invoiceFixture())
	s.Require().NoError(s.m.Err())

	out := s.buf.String()
	s.Require().Contains(out, "`cancel`")
	s.Require().Contains(out, "| "+emptyCellPlaceholder+" |", "cancel's empty ConfirmMessage should render as the placeholder")
}

// TestActionsTable_DoesNotReapplyMethodDefault is the core regression test
// for the "duplicated defaulting" issue found in review: docgen must
// render whatever Method value is already on the ActionDef, not silently
// substitute "POST" for an empty value itself — that resolution belongs to
// the compiler, not the doc renderer.
func (s *EntitySuite) TestActionsTable_DoesNotReapplyMethodDefault() {
	withBlankMethod := &compiler.EntitySchema{
		Actions: []def.ActionDef{{Name: "weird", Method: "", Permission: "role:tenant.admin"}},
	}
	writeActionsTable(s.m, withBlankMethod)
	s.Require().NoError(s.m.Err())
	s.Require().Contains(s.buf.String(), "``", "an empty Method must render as empty backticks, not be silently defaulted to POST by docgen")
}

// TestActionsTable_NoActionsProducesNoOutput verifies organization (no
// Actions) renders nothing for this section.
func (s *EntitySuite) TestActionsTable_NoActionsProducesNoOutput() {
	writeActionsTable(s.m, organizationFixture())
	s.Require().NoError(s.m.Err())
	s.Require().Empty(s.buf.String())
}

// --- writeWorkflowTriggersTable ---

// TestWorkflowTriggersTable_RendersEventWorkflowAndQueue verifies
// finance_invoice's EventOnSubmit -> InvoiceApprovalWorkflow trigger is
// fully rendered. The original implementation had no support for workflow
// triggers at all, so this whole section is new coverage.
func (s *EntitySuite) TestWorkflowTriggersTable_RendersEventWorkflowAndQueue() {
	writeWorkflowTriggersTable(s.m, invoiceFixture())
	s.Require().NoError(s.m.Err())

	out := s.buf.String()
	s.Require().Contains(out, "`on_submit`")
	s.Require().Contains(out, "`InvoiceApprovalWorkflow`")
	s.Require().Contains(out, "`finance.invoice.submit`")
}

// TestWorkflowTriggersTable_NoTriggersProducesNoOutput verifies
// organization (no WorkflowTriggers) renders nothing for this section.
func (s *EntitySuite) TestWorkflowTriggersTable_NoTriggersProducesNoOutput() {
	writeWorkflowTriggersTable(s.m, organizationFixture())
	s.Require().NoError(s.m.Err())
	s.Require().Empty(s.buf.String())
}

// --- writePermissionsTable ---

// TestPermissionsTable_RendersAllFourOperations verifies organization's
// full create/read/write/delete role lists all appear.
func (s *EntitySuite) TestPermissionsTable_RendersAllFourOperations() {
	writePermissionsTable(s.m, organizationFixture())
	s.Require().NoError(s.m.Err())

	out := s.buf.String()
	s.Require().Contains(out, "platform.organization.create")
	s.Require().Contains(out, "platform.organization.read")
	s.Require().Contains(out, "platform.organization.update")
	s.Require().Contains(out, "platform.organization.delete")
}

// TestPermissionsTable_OmitsRowForOperationWithNoRoles verifies that if an
// operation has an empty role list, no row is emitted for it (rather than
// an empty, uninformative "| write | |" row).
func (s *EntitySuite) TestPermissionsTable_OmitsRowForOperationWithNoRoles() {
	readOnly := &compiler.EntitySchema{
		Permissions: def.PermissionSet{Read: []string{"platform.org_type.read"}},
	}
	writePermissionsTable(s.m, readOnly)
	s.Require().NoError(s.m.Err())

	out := s.buf.String()
	s.Require().Contains(out, "| read |")
	s.Require().NotContains(out, "| create |")
	s.Require().NotContains(out, "| write |")
	s.Require().NotContains(out, "| delete |")
}

// TestPermissionsTable_DynamicPolicyProducesExplicitNote is the core
// regression test for the "invisible Policy func" gap found in review:
// finance_invoice's row-level Policy (scoping viewers to Draft invoices)
// is not representable as a static table row, so it must be surfaced as an
// explicit note rather than silently vanishing and letting a reader
// mistake the static role table for the complete access story.
func (s *EntitySuite) TestPermissionsTable_DynamicPolicyProducesExplicitNote() {
	writePermissionsTable(s.m, invoiceFixture())
	s.Require().NoError(s.m.Err())
	s.Require().Contains(s.buf.String(), "row-level access policy",
		"an entity with a non-nil Policy func must get an explicit note that static roles are not the whole story")
}

// TestPermissionsTable_NoPolicyProducesNoNote verifies the converse: an
// entity with no dynamic Policy (organization) does not get the note,
// since it would be misleading noise for an entity that has none.
func (s *EntitySuite) TestPermissionsTable_NoPolicyProducesNoNote() {
	writePermissionsTable(s.m, organizationFixture())
	s.Require().NoError(s.m.Err())
	s.Require().NotContains(s.buf.String(), "row-level access policy")
}

// TestPermissionsTable_NoPermissionsAtAllProducesNoOutput verifies an
// entity with a totally empty PermissionSet renders nothing.
func (s *EntitySuite) TestPermissionsTable_NoPermissionsAtAllProducesNoOutput() {
	writePermissionsTable(s.m, &compiler.EntitySchema{})
	s.Require().NoError(s.m.Err())
	s.Require().Empty(s.buf.String())
}

// --- writeRoutesTable ---

// TestRoutesTable_OnlyRendersRoutesForMatchingEntity verifies the
// precomputed routesByEntity index correctly isolates routes by
// QualifiedName — a route belonging to a different entity must never leak
// into this entity's table.
func (s *EntitySuite) TestRoutesTable_OnlyRendersRoutesForMatchingEntity() {
	index := map[string][]compiler.RouteDescriptor{
		"platform.organization": {
			{EntityQualifiedName: "platform.organization", Method: "GET", Path: "/api/platform/organizations", RequiredPermission: "platform.organization.read"},
		},
		"finance.finance_invoice": {
			{EntityQualifiedName: "finance.finance_invoice", Method: "GET", Path: "/api/finance/invoices", RequiredPermission: "finance.finance_invoice.read"},
		},
	}

	writeRoutesTable(s.m, organizationFixture(), index)
	s.Require().NoError(s.m.Err())

	out := s.buf.String()
	s.Require().Contains(out, "/api/platform/organizations")
	s.Require().NotContains(out, "/api/finance/invoices", "a route for a different entity must not appear")
}

// TestRoutesTable_UnknownEntityProducesNoOutput verifies an entity with no
// entry in the index (no routes registered) renders nothing, rather than
// panicking on the missing map key.
func (s *EntitySuite) TestRoutesTable_UnknownEntityProducesNoOutput() {
	writeRoutesTable(s.m, organizationFixture(), map[string][]compiler.RouteDescriptor{})
	s.Require().NoError(s.m.Err())
	s.Require().Empty(s.buf.String())
}

// --- writeEntitySection (integration across sub-renderers) ---

// TestWriteEntitySection_HeadingAnchorMatchesSlug verifies the anchor
// registered via slugs.anchorFor is actually consistent with the heading
// text written — i.e. headingWithAnchor's internal assertion does not
// panic for a normal entity label. This is an end-to-end guard against the
// TOC/heading divergence bug reintroducing itself.
func (s *EntitySuite) TestWriteEntitySection_HeadingAnchorMatchesSlug() {
	s.Require().NotPanics(func() {
		writeEntitySection(s.m, organizationFixture(), nil, DefaultOptions(), s.slugs)
	})
	s.Require().NoError(s.m.Err())
	s.Require().Contains(s.buf.String(), "### Organization")
}

// TestWriteEntitySection_EndsWithHorizontalRule verifies every entity
// section is terminated with a "---" rule, which is what visually
// separates consecutive entities in the rendered document.
func (s *EntitySuite) TestWriteEntitySection_EndsWithHorizontalRule() {
	writeEntitySection(s.m, organizationFixture(), nil, DefaultOptions(), s.slugs)
	s.Require().NoError(s.m.Err())
	s.Require().True(bytes.HasSuffix(bytes.TrimRight(s.buf.Bytes(), "\n"), []byte("---")))
}

// TestWriteEntitySection_SkipsSectionsWithNoContent verifies a minimal
// entity (org_type-like: a handful of fields and only read permission, no
// edges/actions/triggers) doesn't render empty headings for the sections
// it has nothing to say about.
func (s *EntitySuite) TestWriteEntitySection_SkipsSectionsWithNoContent() {
	minimal := &compiler.EntitySchema{
		Label:     "Organization Type",
		LocalName: "org_type",
		Fields: []def.FieldDef{
			{Name: "name", Type: def.FieldTypeData, Required: true, Unique: true, Immutable: true},
		},
		Permissions: def.PermissionSet{Read: []string{"platform.org_type.read"}},
	}
	writeEntitySection(s.m, minimal, nil, DefaultOptions(), s.slugs)
	s.Require().NoError(s.m.Err())

	out := s.buf.String()
	s.Require().NotContains(out, "#### Edges")
	s.Require().NotContains(out, "#### Custom Actions")
	s.Require().NotContains(out, "#### Workflow Triggers")
	s.Require().NotContains(out, "#### API Routes")
	s.Require().Contains(out, "#### Fields")
	s.Require().Contains(out, "#### Permissions")
}

// --- joinBackticked / fieldConstraints / boolMark (small pure helpers) ---

// TestJoinBackticked_JoinsWithBacktickCommaBacktick verifies the helper
// used to render a role list as a single backtick-wrapped, comma-separated
// cell (e.g. "`role:a`, `role:b`" once the caller wraps the whole result
// in backticks).
func (s *EntitySuite) TestJoinBackticked_JoinsWithBacktickCommaBacktick() {
	got := joinBackticked([]string{"role:a", "role:b", "role:c"})
	s.Require().Equal("role:a`, `role:b`, `role:c", got)
}

// TestJoinBackticked_SingleElementReturnsUnchanged verifies the trivial
// one-element case doesn't add spurious separators.
func (s *EntitySuite) TestJoinBackticked_SingleElementReturnsUnchanged() {
	s.Require().Equal("role:a", joinBackticked([]string{"role:a"}))
}

// TestFieldConstraints_EmptyWhenNoFlagsSet verifies a plain field with no
// Unique/Immutable/Sensitive/Searchable/TenantOverridable flags produces
// an empty string (which escapeCell then turns into the placeholder),
// rather than a stray comma or leading separator.
func (s *EntitySuite) TestFieldConstraints_EmptyWhenNoFlagsSet() {
	s.Require().Equal("", fieldConstraints(def.FieldDef{}))
}

// TestFieldConstraints_JoinsMultipleFlagsWithCommaSpace verifies the
// exact separator formatting between multiple applicable tags.
func (s *EntitySuite) TestFieldConstraints_JoinsMultipleFlagsWithCommaSpace() {
	got := fieldConstraints(def.FieldDef{Unique: true, Immutable: true, Searchable: true})
	s.Require().Equal("unique, immutable, searchable", got)
}

// TestBoolMark_TrueAndFalse verifies the checkmark helper's two branches.
func (s *EntitySuite) TestBoolMark_TrueAndFalse() {
	s.Require().Equal("✓", boolMark(true))
	s.Require().Equal("", boolMark(false))
}
