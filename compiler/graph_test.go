package compiler

import (
	"testing"
)

// buildTestGraph constructs a DependencyGraph directly from an edge map,
// bypassing field/edge resolution. For unit-testing graph algorithms only.
func buildTestGraph(edges map[string][]string) *DependencyGraph {
	g := &DependencyGraph{
		nodes:    make(map[string]bool),
		edges:    make(map[string]map[string]bool),
		SelfRefs: make(map[string]bool),
	}
	for src, targets := range edges {
		g.nodes[src] = true
		if g.edges[src] == nil {
			g.edges[src] = make(map[string]bool)
		}
		for _, t := range targets {
			g.nodes[t] = true
			if g.edges[t] == nil {
				g.edges[t] = make(map[string]bool)
			}
			g.edges[src][t] = true
		}
	}
	return g
}

func TestTopologicalSort_Linear(t *testing.T) {
	// A → B → C  (A depends on B, B depends on C; migration order: C, B, A)
	g := buildTestGraph(map[string][]string{
		"A": {"B"},
		"B": {"C"},
		"C": {},
	})
	order, err := g.topologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pos := map[string]int{}
	for i, n := range order {
		pos[n] = i
	}
	if pos["C"] >= pos["B"] {
		t.Errorf("C must precede B in topo order; got %v", order)
	}
	if pos["B"] >= pos["A"] {
		t.Errorf("B must precede A in topo order; got %v", order)
	}
}

func TestTopologicalSort_Diamond(t *testing.T) {
	// A → B, A → C, B → D, C → D
	g := buildTestGraph(map[string][]string{
		"A": {"B", "C"},
		"B": {"D"},
		"C": {"D"},
		"D": {},
	})
	order, err := g.topologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pos := map[string]int{}
	for i, n := range order {
		pos[n] = i
	}
	if pos["D"] >= pos["B"] {
		t.Errorf("D must precede B; got %v", order)
	}
	if pos["D"] >= pos["C"] {
		t.Errorf("D must precede C; got %v", order)
	}
}

func TestDetectCycles_NoCycle(t *testing.T) {
	g := buildTestGraph(map[string][]string{
		"A": {"B"},
		"B": {"C"},
		"C": {},
	})
	if cycles := g.detectCycles(); len(cycles) != 0 {
		t.Errorf("expected no cycles; got %v", cycles)
	}
}

func TestDetectCycles_SimpleCycle(t *testing.T) {
	// A → B → A
	g := buildTestGraph(map[string][]string{
		"A": {"B"},
		"B": {"A"},
	})
	if cycles := g.detectCycles(); len(cycles) == 0 {
		t.Error("expected a cycle A→B→A to be detected")
	}
}

func TestDetectCycles_ThreeNodeCycle(t *testing.T) {
	// A → B → C → A
	g := buildTestGraph(map[string][]string{
		"A": {"B"},
		"B": {"C"},
		"C": {"A"},
	})
	if cycles := g.detectCycles(); len(cycles) == 0 {
		t.Error("expected cycle A→B→C→A to be detected")
	}
}

func TestDetectCycles_SelfRefExcluded(t *testing.T) {
	// Self-refs are excluded from the edges map; SelfRefs tracks them separately.
	// An entity with only a self-reference should NOT trigger cycle detection.
	g := &DependencyGraph{
		nodes:    map[string]bool{"org": true},
		edges:    map[string]map[string]bool{"org": {}}, // no self-ref edge
		SelfRefs: map[string]bool{"org": true},
	}
	if cycles := g.detectCycles(); len(cycles) != 0 {
		t.Errorf("self-ref should not be treated as a cycle; got %v", cycles)
	}
}

func TestDetectCycles_MixedCycleAndDAG(t *testing.T) {
	// X → Y → Z (DAG), A → B → A (cycle). Both in same graph.
	g := buildTestGraph(map[string][]string{
		"X": {"Y"},
		"Y": {"Z"},
		"Z": {},
		"A": {"B"},
		"B": {"A"},
	})
	cycles := g.detectCycles()
	if len(cycles) == 0 {
		t.Error("expected cycle A→B→A to be detected in mixed graph")
	}
	// Ensure DAG nodes are not reported as part of a cycle.
	for _, cycle := range cycles {
		for _, n := range cycle {
			if n == "X" || n == "Y" || n == "Z" {
				t.Errorf("DAG node %q incorrectly included in cycle %v", n, cycle)
			}
		}
	}
}

func TestDeps_ReturnsSorted(t *testing.T) {
	g := buildTestGraph(map[string][]string{
		"A": {"Z", "B", "M"},
	})
	deps := g.Deps("A")
	for i := 1; i < len(deps); i++ {
		if deps[i-1] > deps[i] {
			t.Errorf("Deps not sorted: %v", deps)
		}
	}
}

func TestDeps_UnknownNode(t *testing.T) {
	g := buildTestGraph(map[string][]string{"A": {"B"}})
	if got := g.Deps("nonexistent"); got != nil {
		t.Errorf("expected nil for unknown node; got %v", got)
	}
}

func TestDependents(t *testing.T) {
	g := buildTestGraph(map[string][]string{
		"finance_invoice": {"finance_currency"},
		"finance_payment": {"finance_currency"},
		"finance_journal": {"finance_currency"},
	})
	deps := g.Dependents("finance_currency")
	if len(deps) != 3 {
		t.Errorf("expected 3 dependents of finance_currency; got %v", deps)
	}
	// Verify sorted.
	for i := 1; i < len(deps); i++ {
		if deps[i-1] > deps[i] {
			t.Errorf("Dependents not sorted: %v", deps)
		}
	}
}

func TestIsSelfRef(t *testing.T) {
	g := &DependencyGraph{
		nodes:    map[string]bool{"platform_organization": true},
		edges:    map[string]map[string]bool{"platform_organization": {}},
		SelfRefs: map[string]bool{"platform_organization": true},
	}
	if !g.IsSelfRef("platform_organization") {
		t.Error("expected IsSelfRef=true for platform_organization")
	}
	if g.IsSelfRef("finance_invoice") {
		t.Error("expected IsSelfRef=false for finance_invoice")
	}
}

func TestFormatCycle(t *testing.T) {
	cycle := []string{"A", "B", "C", "A"}
	got := formatCycle(cycle)
	want := "A → B → C → A"
	if got != want {
		t.Errorf("formatCycle: got %q, want %q", got, want)
	}
}

func TestFormatCycle_Empty(t *testing.T) {
	got := formatCycle(nil)
	if got != "(empty)" {
		t.Errorf("formatCycle(nil): got %q, want \"(empty)\"", got)
	}
}

func TestReport_Basic(t *testing.T) {
	g := buildTestGraph(map[string][]string{
		"finance_invoice":  {"finance_currency"},
		"finance_payment":  {"finance_currency"},
		"finance_currency": {},
	})
	// Populate TopologicalOrder manually for report test.
	order, err := g.topologicalSort()
	if err != nil {
		t.Fatalf("topologicalSort: %v", err)
	}
	g.TopologicalOrder = order

	r := g.Report()

	if r.NodeCount != 3 {
		t.Errorf("expected 3 nodes, got %d", r.NodeCount)
	}
	if r.EdgeCount != 2 {
		t.Errorf("expected 2 edges, got %d", r.EdgeCount)
	}
	if len(r.TopologicalOrder) != 3 {
		t.Errorf("expected 3 in topo order, got %d", len(r.TopologicalOrder))
	}
	// Verify sorted edges.
	for i := 1; i < len(r.Edges); i++ {
		prev := r.Edges[i-1]
		curr := r.Edges[i]
		if prev.Source > curr.Source || (prev.Source == curr.Source && prev.Target > curr.Target) {
			t.Errorf("edges not sorted: %v before %v", prev, curr)
		}
	}
}

func TestReport_SelfRefs(t *testing.T) {
	g := &DependencyGraph{
		nodes: map[string]bool{
			"platform_organization": true,
		},
		edges: map[string]map[string]bool{
			"platform_organization": {},
		},
		SelfRefs: map[string]bool{
			"platform_organization": true,
		},
		TopologicalOrder: []string{"platform_organization"},
	}
	r := g.Report()
	if len(r.SelfRefs) != 1 || r.SelfRefs[0] != "platform_organization" {
		t.Errorf("expected SelfRefs=[platform_organization], got %v", r.SelfRefs)
	}
}

func TestReport_NoEdges(t *testing.T) {
	g := &DependencyGraph{
		nodes:            map[string]bool{"A": true},
		edges:            map[string]map[string]bool{"A": {}},
		SelfRefs:         map[string]bool{},
		TopologicalOrder: []string{"A"},
	}
	r := g.Report()
	if r.EdgeCount != 0 {
		t.Errorf("expected 0 edges, got %d", r.EdgeCount)
	}
	if r.NodeCount != 1 {
		t.Errorf("expected 1 node, got %d", r.NodeCount)
	}
}
