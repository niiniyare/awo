package widget_test

import (
	"testing"

	"awo.so/awo/sdui/widget"
)

// TestNodeKind_Constants verifies that all NodeKind constants have non-empty
// values and are distinct from each other. This guards against accidental
// duplication or zero-value constants that would render silently.
func TestNodeKind_Constants(t *testing.T) {
	t.Parallel()

	kinds := []widget.NodeKind{
		widget.NodePage, widget.NodeForm, widget.NodeList, widget.NodeSection, widget.NodeTabs,
		widget.NodeTable, widget.NodeDialog,
		widget.NodeField, widget.NodeText, widget.NodeTextArea, widget.NodeNumber, widget.NodeSelect,
		widget.NodeDate, widget.NodeDateTime, widget.NodeSwitch, widget.NodeEditor,
		widget.NodeButton,
	}

	seen := make(map[widget.NodeKind]bool, len(kinds))
	for _, k := range kinds {
		if k == "" {
			t.Errorf("NodeKind constant must not be empty string")
		}
		if seen[k] {
			t.Errorf("duplicate NodeKind constant: %q", k)
		}
		seen[k] = true
	}
}

// TestNode_ZeroValue verifies that a zero-value Node is constructable without
// panics — the generator relies on composing nodes without mandatory fields.
func TestNode_ZeroValue(t *testing.T) {
	t.Parallel()
	var n widget.Node
	if n.Kind != "" {
		t.Error("zero Node.Kind must be empty string")
	}
	if n.Hidden {
		t.Error("zero Node.Hidden must be false")
	}
	if n.Children != nil {
		t.Error("zero Node.Children must be nil")
	}
}

// TestNode_HiddenSemantics documents that Hidden == true means "omit from
// output", not "render with hidden property". This is enforced by the renderer,
// not the widget package — the test exists to document the invariant.
func TestNode_HiddenSemantics(t *testing.T) {
	t.Parallel()
	n := &widget.Node{Kind: widget.NodeText, Name: "password_hash", Hidden: true}
	if !n.Hidden {
		t.Error("Hidden flag not set")
	}
	// Renderer must check n.Hidden and omit the node — not render hidden:true.
	// This test is a documentation anchor; renderer_test.go validates behaviour.
}

// TestDataSource_Defaults verifies DataSource zero values make sense.
func TestDataSource_Defaults(t *testing.T) {
	t.Parallel()
	ds := &widget.DataSource{URL: "/api/v1/finance/invoices"}
	if ds.Method != "" {
		t.Errorf("zero DataSource.Method = %q, want empty (renderer defaults to GET)", ds.Method)
	}
	if ds.ValueField != "" {
		t.Errorf("zero DataSource.ValueField = %q, want empty (renderer defaults to 'id')", ds.ValueField)
	}
}

// TestActionNode_Fields verifies ActionNode fields are addressable and correct
// zero values don't cause nil panics in renderers.
func TestActionNode_Fields(t *testing.T) {
	t.Parallel()
	a := &widget.ActionNode{
		Label:      "Submit",
		ActionType: "ajax",
		Level:      "primary",
		API:        "POST:/api/v1/finance/invoices/${id}/submit",
	}
	if a.Label == "" || a.ActionType == "" || a.Level == "" || a.API == "" {
		t.Error("ActionNode fields not set correctly")
	}
	if a.ConfirmText != "" {
		t.Error("zero ConfirmText must be empty")
	}
}
