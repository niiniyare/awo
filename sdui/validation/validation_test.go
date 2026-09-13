package validation_test

import (
	"testing"

	"github.com/google/uuid"

	"awo.so/awo/sdui/registry"
	"awo.so/awo/sdui/sduictx"
	"awo.so/awo/sdui/validation"
	"awo.so/awo/sdui/widget"
)

type stubViewer struct{ tenantID uuid.UUID }

func (s stubViewer) TenantID() uuid.UUID       { return s.tenantID }
func (s stubViewer) Roles() []string           { return nil }
func (s stubViewer) IsPlatformAdmin() bool     { return false }
func (s stubViewer) HasPermission(string) bool { return true }

func makeCtx() sduictx.GeneratorContext {
	ctx, _ := sduictx.NewGeneratorContext(
		uuid.New(), stubViewer{tenantID: uuid.New()}, "test_entity",
		sduictx.ViewModeList, "amis",
	).Build()
	return ctx
}

func makeReg() *registry.Registry {
	r := registry.NewIsolated()
	defs := []registry.WidgetDef{
		{Kind: widget.NodePage, IsContainer: true},
		{Kind: widget.NodeForm, IsContainer: true},
		{Kind: widget.NodeTabs, IsContainer: true},
		{Kind: widget.NodeSection, IsContainer: true},
		{Kind: widget.NodeTabPane, IsContainer: true},
		{Kind: widget.NodeText, IsInputField: true},
		{Kind: widget.NodeList, IsContainer: true, RequiresDataSource: true},
		{Kind: widget.NodeLookup, IsInputField: true, RequiresDataSource: true},
	}
	for _, d := range defs {
		r.Register(d) //nolint
	}
	r.Seal()
	return r
}

func TestValidate_ValidTree(t *testing.T) {
	r := makeReg()
	v := validation.NewWithRegistry(r)

	root := &widget.Node{
		Kind: widget.NodePage,
		Children: []*widget.Node{
			{Kind: widget.NodeForm, Children: []*widget.Node{
				{Kind: widget.NodeText, Name: "title"},
			}},
		},
	}

	result := v.Validate(root, makeCtx())
	if result.HasFatal() {
		t.Errorf("expected no fatals, got: %v", result.Fatals())
	}
}

func TestValidate_NilRoot(t *testing.T) {
	v := validation.New()
	result := v.Validate(nil, makeCtx())
	if !result.HasFatal() {
		t.Error("expected fatal for nil root")
	}
}

func TestValidate_UnknownNodeKind(t *testing.T) {
	r := registry.NewIsolated()
	r.Register(registry.WidgetDef{Kind: widget.NodePage, IsContainer: true}) //nolint
	r.Seal()
	v := validation.NewWithRegistry(r)

	root := &widget.Node{
		Kind:     widget.NodePage,
		Children: []*widget.Node{{Kind: widget.NodeKind("totally_unknown")}},
	}
	result := v.Validate(root, makeCtx())
	if !result.HasFatal() {
		t.Error("expected fatal for unknown NodeKind")
	}
}

func TestValidate_InputFieldMissingName(t *testing.T) {
	r := makeReg()
	v := validation.NewWithRegistry(r)
	root := &widget.Node{
		Kind:     widget.NodePage,
		Children: []*widget.Node{{Kind: widget.NodeText}}, // missing Name
	}
	result := v.Validate(root, makeCtx())
	if !result.HasFatal() {
		t.Error("expected fatal for input field missing Name")
	}
}

func TestValidate_TabPaneOutsideTabs(t *testing.T) {
	r := makeReg()
	v := validation.NewWithRegistry(r)
	root := &widget.Node{
		Kind:     widget.NodePage,
		Children: []*widget.Node{{Kind: widget.NodeTabPane}}, // not inside NodeTabs
	}
	result := v.Validate(root, makeCtx())
	if !result.HasFatal() {
		t.Error("expected fatal for NodeTabPane outside NodeTabs")
	}
}

func TestValidate_SectionInsideTabs(t *testing.T) {
	r := makeReg()
	v := validation.NewWithRegistry(r)
	root := &widget.Node{
		Kind: widget.NodePage,
		Children: []*widget.Node{
			{Kind: widget.NodeTabs, Children: []*widget.Node{
				{Kind: widget.NodeSection}, // should be NodeTabPane
			}},
		},
	}
	result := v.Validate(root, makeCtx())
	if !result.HasFatal() {
		t.Error("expected fatal for NodeSection inside NodeTabs")
	}
}

func TestValidate_MissingDataSource(t *testing.T) {
	r := makeReg()
	v := validation.NewWithRegistry(r)
	root := &widget.Node{
		Kind:     widget.NodePage,
		Children: []*widget.Node{{Kind: widget.NodeList}}, // no DataSource
	}
	result := v.Validate(root, makeCtx())
	if !result.HasFatal() {
		t.Error("expected fatal for NodeList missing DataSource")
	}
}

func TestValidate_NilExprRef(t *testing.T) {
	r := makeReg()
	v := validation.NewWithRegistry(r)
	root := &widget.Node{
		Kind:      widget.NodePage,
		VisibleOn: &widget.ExpressionRef{Expr: nil}, // invalid: Expr is nil
	}
	result := v.Validate(root, makeCtx())
	if !result.HasFatal() {
		t.Error("expected fatal for ExpressionRef with nil Expr")
	}
}
