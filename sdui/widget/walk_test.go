package widget_test

import (
	"testing"

	"awo.so/awo/sdui/widget"
)

func buildTestTree() *widget.Node {
	return &widget.Node{
		Kind:  widget.NodePage,
		ID:    "root",
		Label: "Root",
		Children: []*widget.Node{
			{
				Kind:  widget.NodeForm,
				ID:    "form1",
				Label: "Form",
				Children: []*widget.Node{
					{Kind: widget.NodeText, ID: "field1", Name: "name"},
					{Kind: widget.NodeNumber, ID: "field2", Name: "amount"},
				},
			},
			{
				Kind:  widget.NodeSection,
				ID:    "sec1",
				Label: "Section",
			},
		},
	}
}

func TestWalk_VisitsAllNodes(t *testing.T) {
	root := buildTestTree()
	var visited []string
	widget.WalkAll(root, func(n *widget.Node) {
		visited = append(visited, n.ID)
	})
	if len(visited) != 5 {
		t.Fatalf("expected 5 nodes, got %d: %v", len(visited), visited)
	}
	if visited[0] != "root" {
		t.Errorf("expected root first, got %q", visited[0])
	}
}

func TestWalk_PrunesSubtree(t *testing.T) {
	root := buildTestTree()
	var visited []string
	widget.Walk(root, func(n *widget.Node) bool {
		visited = append(visited, n.ID)
		// Stop descending into form1
		return n.ID != "form1"
	})
	// Should visit: root, form1, sec1 (form1's children pruned)
	if len(visited) != 3 {
		t.Fatalf("expected 3 nodes after pruning, got %d: %v", len(visited), visited)
	}
}

func TestWalk_NilRoot(t *testing.T) {
	// Must not panic
	widget.Walk(nil, func(n *widget.Node) bool {
		t.Fatal("fn should not be called for nil root")
		return true
	})
}

func TestFindByID_Found(t *testing.T) {
	root := buildTestTree()
	n := widget.FindByID(root, "field2")
	if n == nil {
		t.Fatal("expected to find field2")
	}
	if n.Name != "amount" {
		t.Errorf("expected name=amount, got %q", n.Name)
	}
}

func TestFindByID_NotFound(t *testing.T) {
	root := buildTestTree()
	n := widget.FindByID(root, "nonexistent")
	if n != nil {
		t.Errorf("expected nil, got %+v", n)
	}
}

func TestCountNodes(t *testing.T) {
	root := buildTestTree()
	if got := widget.CountNodes(root); got != 5 {
		t.Errorf("expected 5, got %d", got)
	}
}

func TestCollect(t *testing.T) {
	root := buildTestTree()
	inputs := widget.Collect(root, func(n *widget.Node) bool {
		return n.Kind == widget.NodeText || n.Kind == widget.NodeNumber
	})
	if len(inputs) != 2 {
		t.Errorf("expected 2 input nodes, got %d", len(inputs))
	}
}
