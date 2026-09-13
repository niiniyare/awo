package compiler

import (
	"fmt"
	"sort"

	"awo.so/awo/def"
)

// DependencyGraph is an immutable, directed graph of entity dependencies derived
// from FieldTypeLink and EdgeDef declarations. It is built once during compilation
// and attached to CompiledSchema.
//
// Edges represent: "entity A depends on entity B" — either via a FieldTypeLink
// field whose target is B, or an EdgeDef whose target entity is B.
//
// The graph is used by the compiler to:
//  1. Detect circular FK references (build-time error).
//  2. Detect orphaned link targets (build-time error).
//  3. Produce a topological migration order (for schema generation).
type DependencyGraph struct {
	// nodes is the set of all qualified entity names in the graph.
	nodes map[string]bool

	// edges maps source → set of targets.
	// edges["finance_invoice"]["finance_currency"] = true means
	// finance_invoice has a FieldTypeLink or EdgeDef pointing to finance_currency.
	edges map[string]map[string]bool

	// SelfRefs is the set of qualified names that reference themselves.
	// Self-referential FKs (e.g. parent_id on platform_organization) are valid
	// and must NOT be reported as cycles.
	SelfRefs map[string]bool

	// TopologicalOrder is the migration-safe ordering of entities: dependencies
	// come before dependents. Populated only after a successful cycle check.
	// nil when a cycle is present.
	TopologicalOrder []string
}

// buildDependencyGraph constructs a DependencyGraph from the compiled entity
// schemas. Link targets are resolved using the qualifiedNames set.
//
// It is called after Phase 1 stub building (all EntitySchema objects exist)
// but before Phase 2.5 so that link resolution errors surface early.
func buildDependencyGraph(schemas []*EntitySchema, qualifiedNames map[string]bool) (*DependencyGraph, Diagnostics) {
	var ds Diagnostics

	g := &DependencyGraph{
		nodes:    make(map[string]bool, len(schemas)),
		edges:    make(map[string]map[string]bool, len(schemas)),
		SelfRefs: make(map[string]bool),
	}

	for _, es := range schemas {
		src := es.QualifiedName
		g.nodes[src] = true
		if g.edges[src] == nil {
			g.edges[src] = make(map[string]bool)
		}

		// FieldTypeLink and FieldTypeLinkList fields.
		for _, f := range es.Fields {
			if f.Type != def.FieldTypeLink && f.Type != def.FieldTypeLinkList {
				continue
			}
			target := f.LinkTarget
			if target == "" {
				continue
			}
			if !qualifiedNames[target] {
				ds = append(ds, Diagnostic{Severity: SeverityError,
					EntityName: src,
					Message:    fmt.Sprintf("field %q: link target %q not found in registry", f.Name, target)})
				continue
			}
			if target == src {
				g.SelfRefs[src] = true
				continue // self-refs are valid; exclude from cycle detection
			}
			g.edges[src][target] = true
		}

		// EdgeDef references.
		for _, e := range es.Edges {
			target := e.Target
			if target == "" {
				continue
			}
			if !qualifiedNames[target] {
				ds = append(ds, Diagnostic{Severity: SeverityError,
					EntityName: src,
					Message:    fmt.Sprintf("edge %q: target entity %q not found in registry", e.Name, target)})
				continue
			}
			if target == src {
				g.SelfRefs[src] = true
				continue
			}
			g.edges[src][target] = true
		}
	}

	if ds.HasErrors() {
		return g, ds
	}

	// Cycle detection via DFS.
	cycles := g.detectCycles()
	for _, cycle := range cycles {
		ds = append(ds, Diagnostic{Severity: SeverityError,
			Message: fmt.Sprintf("circular FK dependency: %s", formatCycle(cycle))})
	}

	if !ds.HasErrors() {
		order, err := g.topologicalSort()
		if err != nil {
			// Should not happen — cycle detection above would have caught it.
			ds = append(ds, Diagnostic{Severity: SeverityError,
				Message: fmt.Sprintf("topological sort: %v", err)})
		} else {
			g.TopologicalOrder = order
		}
	}

	return g, ds
}

// Deps returns the direct dependency set of the given qualified entity name.
// Returns nil if the entity is not in the graph.
func (g *DependencyGraph) Deps(qualifiedName string) []string {
	m, ok := g.edges[qualifiedName]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(m))
	for t := range m {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// Dependents returns all entities that directly depend on the given entity.
func (g *DependencyGraph) Dependents(qualifiedName string) []string {
	var out []string
	for src, targets := range g.edges {
		if targets[qualifiedName] {
			out = append(out, src)
		}
	}
	sort.Strings(out)
	return out
}

// IsSelfRef returns true if the entity has a self-referential FK (e.g. parent_id).
func (g *DependencyGraph) IsSelfRef(qualifiedName string) bool {
	return g.SelfRefs[qualifiedName]
}

// detectCycles returns all cycles in the graph as node sequences.
// Each cycle is represented as the sequence of nodes forming the loop
// (the first and last element are the same node).
func (g *DependencyGraph) detectCycles() [][]string {
	// Kahn's algorithm: nodes with remaining in-degree after topological
	// removal are part of a cycle. We collect them and then trace the cycle
	// path with a DFS.
	inDegree := make(map[string]int, len(g.nodes))
	for n := range g.nodes {
		inDegree[n] = 0
	}
	for src, targets := range g.edges {
		_ = src
		for t := range targets {
			inDegree[t]++
		}
	}

	queue := []string{}
	for n, d := range inDegree {
		if d == 0 {
			queue = append(queue, n)
		}
	}
	sort.Strings(queue) // deterministic

	visited := make(map[string]bool, len(g.nodes))
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		visited[n] = true
		for t := range g.edges[n] {
			inDegree[t]--
			if inDegree[t] == 0 {
				queue = append(queue, t)
				sort.Strings(queue)
			}
		}
	}

	// Any node not visited is part of a cycle.
	cycleNodes := make(map[string]bool)
	for n := range g.nodes {
		if !visited[n] {
			cycleNodes[n] = true
		}
	}
	if len(cycleNodes) == 0 {
		return nil
	}

	// DFS to trace one cycle path per strongly-connected component.
	var cycles [][]string
	traced := make(map[string]bool)
	for start := range cycleNodes {
		if traced[start] {
			continue
		}
		path := []string{}
		pathSet := map[string]bool{}
		var dfs func(n string) bool
		dfs = func(n string) bool {
			if !cycleNodes[n] {
				return false
			}
			if pathSet[n] {
				// Found cycle — slice from n to end of path.
				for i, p := range path {
					if p == n {
						cycle := append([]string{}, path[i:]...)
						cycle = append(cycle, n)
						cycles = append(cycles, cycle)
						return true
					}
				}
				return true
			}
			path = append(path, n)
			pathSet[n] = true
			targets := g.Deps(n) // sorted for determinism
			for _, t := range targets {
				if cycleNodes[t] && dfs(t) {
					traced[n] = true
					return true
				}
			}
			path = path[:len(path)-1]
			delete(pathSet, n)
			return false
		}
		dfs(start)
		traced[start] = true
	}
	return cycles
}

// topologicalSort returns entities in dependency order (dependencies first).
// Returns an error if a cycle exists (call detectCycles first).
func (g *DependencyGraph) topologicalSort() ([]string, error) {
	inDegree := make(map[string]int, len(g.nodes))
	for n := range g.nodes {
		inDegree[n] = 0
	}
	for _, targets := range g.edges {
		for t := range targets {
			inDegree[t]++
		}
	}

	queue := []string{}
	for n, d := range inDegree {
		if d == 0 {
			queue = append(queue, n)
		}
	}
	sort.Strings(queue)

	var order []string
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		order = append(order, n)
		next := g.Deps(n)
		for _, t := range next {
			inDegree[t]--
			if inDegree[t] == 0 {
				queue = append(queue, t)
				sort.Strings(queue)
			}
		}
	}

	if len(order) != len(g.nodes) {
		return nil, fmt.Errorf("cycle detected: topological sort incomplete (%d/%d nodes visited)", len(order), len(g.nodes))
	}
	// Kahn's yields dependents first (nodes with no incoming edges = nothing
	// depends on them). For migration order we need dependencies first, so reverse.
	for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
		order[i], order[j] = order[j], order[i]
	}
	return order, nil
}

// GraphReport is a structured summary of the dependency graph, suitable for
// printing by `awo schema graph` or embedding in compiler diagnostics output.
type GraphReport struct {
	// TopologicalOrder is the migration-safe entity ordering (dependencies first).
	// Nil when the graph contains a cycle (TopologicalOrder cannot be computed).
	TopologicalOrder []string `json:"topological_order,omitempty"`

	// SelfRefs lists entities with self-referential FK references (valid; excluded
	// from cycle detection).
	SelfRefs []string `json:"self_refs,omitempty"`

	// Edges lists all FK dependencies as source→target pairs.
	Edges []GraphEdge `json:"edges,omitempty"`

	// NodeCount is the total number of entity nodes in the graph.
	NodeCount int `json:"node_count"`

	// EdgeCount is the total number of FK dependency edges.
	EdgeCount int `json:"edge_count"`
}

// GraphEdge represents a single directed dependency edge in the graph.
type GraphEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// Report produces a structured summary of the dependency graph.
func (g *DependencyGraph) Report() GraphReport {
	r := GraphReport{
		TopologicalOrder: g.TopologicalOrder,
		NodeCount:        len(g.nodes),
	}

	// Self-refs (sorted for determinism).
	for n := range g.SelfRefs {
		r.SelfRefs = append(r.SelfRefs, n)
	}
	sort.Strings(r.SelfRefs)

	// Edges (sorted for determinism).
	for src, targets := range g.edges {
		for t := range targets {
			r.Edges = append(r.Edges, GraphEdge{Source: src, Target: t})
		}
	}
	sort.Slice(r.Edges, func(i, j int) bool {
		if r.Edges[i].Source != r.Edges[j].Source {
			return r.Edges[i].Source < r.Edges[j].Source
		}
		return r.Edges[i].Target < r.Edges[j].Target
	})
	r.EdgeCount = len(r.Edges)
	return r
}

func formatCycle(cycle []string) string {
	if len(cycle) == 0 {
		return "(empty)"
	}
	s := cycle[0]
	for _, n := range cycle[1:] {
		s += " → " + n
	}
	return s
}
