package compiler

import (
	"fmt"
	"sort"

	"awo.so/awo/def"
)

// DepGraph is the entity dependency graph derived from Link/LinkList fields.
// An edge A→B means entity A references entity B (A depends on B).
type DepGraph struct {
	// deps maps entity name → set of entity names it directly depends on.
	deps map[string]map[string]bool
	// all is the set of all entity names in the graph.
	all map[string]bool
}

// Build constructs the dependency graph from a compiled schema.
func Build(s *CompiledSchema) *DepGraph {
	g := &DepGraph{
		deps: make(map[string]map[string]bool, len(s.Entities)),
		all:  make(map[string]bool, len(s.Entities)),
	}
	for _, es := range s.Entities {
		name := es.QualifiedName
		g.all[name] = true
		if g.deps[name] == nil {
			g.deps[name] = make(map[string]bool)
		}
		for _, f := range es.Fields {
			if (f.Type == def.FieldTypeLink || f.Type == def.FieldTypeLinkList) && f.LinkTarget != "" {
				g.deps[name][f.LinkTarget] = true
			}
		}
	}
	return g
}

// DependsOn returns true if entity a directly or transitively depends on entity b.
func (g *DepGraph) DependsOn(a, b string) bool {
	visited := make(map[string]bool)
	return g.dfsReach(a, b, visited)
}

func (g *DepGraph) dfsReach(current, target string, visited map[string]bool) bool {
	if visited[current] {
		return false
	}
	visited[current] = true
	for dep := range g.deps[current] {
		if dep == target || g.dfsReach(dep, target, visited) {
			return true
		}
	}
	return false
}

// TopologicalOrder returns entity names where dependencies come before dependents.
// Uses Kahn's algorithm. Returns error on cycle detection.
//
// Edge direction: A→B means A depends on B; B must appear before A in the output.
func (g *DepGraph) TopologicalOrder() ([]string, error) {
	// depCount[A] = number of A's direct dependencies not yet placed.
	depCount := make(map[string]int, len(g.all))
	reverseDeps := make(map[string][]string) // B → list of A that depend on B
	for name := range g.all {
		depCount[name] = len(g.deps[name])
	}
	for name, deps := range g.deps {
		for dep := range deps {
			reverseDeps[dep] = append(reverseDeps[dep], name)
		}
	}

	// Collect all names with zero deps; sort for determinism.
	var queue []string
	for name := range g.all {
		if depCount[name] == 0 {
			queue = append(queue, name)
		}
	}
	sort.Strings(queue)

	var result []string
	for len(queue) > 0 {
		// Pop first.
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		// Reduce dep count for everything that depends on node.
		dependants := reverseDeps[node]
		sort.Strings(dependants)
		for _, dep := range dependants {
			depCount[dep]--
			if depCount[dep] == 0 {
				queue = append(queue, dep)
				sort.Strings(queue)
			}
		}
	}

	if len(result) != len(g.all) {
		return nil, fmt.Errorf("circular dependency detected in entity graph")
	}
	return result, nil
}

// Dependents returns all entities that directly or transitively depend on the given entity.
func (g *DepGraph) Dependents(name string) []string {
	// Build reverse map on demand.
	reverse := make(map[string]map[string]bool)
	for a, deps := range g.deps {
		for b := range deps {
			if reverse[b] == nil {
				reverse[b] = make(map[string]bool)
			}
			reverse[b][a] = true
		}
	}

	visited := make(map[string]bool)
	var result []string
	var dfs func(n string)
	dfs = func(n string) {
		for dep := range reverse[n] {
			if !visited[dep] {
				visited[dep] = true
				result = append(result, dep)
				dfs(dep)
			}
		}
	}
	dfs(name)
	sort.Strings(result)
	return result
}
