package module

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// ModuleRegistry is a concurrent-safe collection of module Manifests. It
// validates dependency declarations and resolves a topological load order via
// Kahn's algorithm.
type ModuleRegistry struct {
	mu        sync.RWMutex
	manifests map[string]Manifest // keyed by Manifest.Name
}

// New returns an empty ModuleRegistry.
func New() *ModuleRegistry {
	return &ModuleRegistry{
		manifests: make(map[string]Manifest),
	}
}

// Register adds m to the registry. It returns an error if:
//   - m.Name is empty
//   - a manifest with the same Name is already registered
//   - a dependency cycle is introduced by this registration
//
// Register is safe for concurrent use but should not be called from request
// handlers — register all modules during process initialisation.
func (r *ModuleRegistry) Register(m Manifest) error {
	if m.Name == "" {
		return fmt.Errorf("module.Registry.Register: manifest has empty Name")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.manifests[m.Name]; exists {
		return fmt.Errorf("module.Registry.Register: module %q is already registered", m.Name)
	}

	r.manifests[m.Name] = m

	// Detect cycles after insertion. If a cycle exists we roll back.
	if err := r.detectCycle(); err != nil {
		delete(r.manifests, m.Name)
		return fmt.Errorf("module.Registry.Register: registering %q introduces a dependency cycle: %w", m.Name, err)
	}

	return nil
}

// Resolve returns all registered manifests sorted in dependency order
// (topological sort via Kahn's algorithm). A module always appears after all
// modules it depends on.
//
// Resolve returns an error if:
//   - a dependency references a module that is not registered
//   - the installed version of a dependency does not satisfy the declared minimum
//   - a dependency cycle exists (cycle detection is also done eagerly in Register)
func (r *ModuleRegistry) Resolve() ([]Manifest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.topoSort()
}

// Get returns the Manifest for the named module. The second return value is
// false if no module with that name is registered.
func (r *ModuleRegistry) Get(name string) (Manifest, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.manifests[name]
	return m, ok
}

// All returns all registered manifests in an unspecified (but stable) order.
func (r *ModuleRegistry) All() []Manifest {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Manifest, 0, len(r.manifests))
	for _, m := range r.manifests {
		out = append(out, m)
	}
	// Sort by name for stability.
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// detectCycle runs a topological sort (ignoring missing deps) purely to find
// cycles among currently registered modules. It is called under the write lock.
func (r *ModuleRegistry) detectCycle() error {
	// Build in-degree map and adjacency list using only registered modules.
	inDegree := make(map[string]int, len(r.manifests))
	adj := make(map[string][]string, len(r.manifests))

	for name := range r.manifests {
		if _, ok := inDegree[name]; !ok {
			inDegree[name] = 0
		}
		for _, dep := range r.manifests[name].DependsOn {
			if _, registered := r.manifests[dep.Module]; !registered {
				// Ignore unregistered deps for cycle detection — Resolve will
				// report them as missing.
				continue
			}
			adj[dep.Module] = append(adj[dep.Module], name)
			inDegree[name]++
		}
	}

	processed := kahn(inDegree, adj)
	if processed < len(r.manifests) {
		return fmt.Errorf("cycle detected among registered modules")
	}
	return nil
}

// topoSort performs a full Kahn's algorithm topological sort with version
// validation. Called under the read lock.
func (r *ModuleRegistry) topoSort() ([]Manifest, error) {
	// Validate all declared dependencies exist and meet version requirements.
	for _, m := range r.manifests {
		for _, dep := range m.DependsOn {
			installed, ok := r.manifests[dep.Module]
			if !ok {
				return nil, fmt.Errorf(
					"module.Registry.Resolve: module %q requires %q which is not registered",
					m.Name, dep.Module,
				)
			}
			if !dep.Minimum.Compatible(installed.Version) {
				return nil, fmt.Errorf(
					"module.Registry.Resolve: module %q requires %q >= %s (same major), installed %s",
					m.Name, dep.Module, dep.Minimum, installed.Version,
				)
			}
		}
	}

	// Build adjacency list and in-degree map.
	// Edge: dep.Module → m.Name  (dep must come before m)
	inDegree := make(map[string]int, len(r.manifests))
	adj := make(map[string][]string, len(r.manifests))

	for name := range r.manifests {
		if _, ok := inDegree[name]; !ok {
			inDegree[name] = 0
		}
	}
	for _, m := range r.manifests {
		for _, dep := range m.DependsOn {
			adj[dep.Module] = append(adj[dep.Module], m.Name)
			inDegree[m.Name]++
		}
	}

	// Kahn's algorithm with deterministic ordering: process the queue sorted
	// alphabetically so output is stable across runs.
	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}
	sort.Strings(queue)

	result := make([]Manifest, 0, len(r.manifests))
	for len(queue) > 0 {
		// Pop front.
		name := queue[0]
		queue = queue[1:]
		result = append(result, r.manifests[name])

		// Reduce in-degree of successors.
		var ready []string
		for _, successor := range adj[name] {
			inDegree[successor]--
			if inDegree[successor] == 0 {
				ready = append(ready, successor)
			}
		}
		sort.Strings(ready)
		queue = append(queue, ready...)
	}

	if len(result) != len(r.manifests) {
		// Identify cycle participants for a useful error message.
		var cyclic []string
		for name, deg := range inDegree {
			if deg > 0 {
				cyclic = append(cyclic, name)
			}
		}
		sort.Strings(cyclic)
		return nil, fmt.Errorf(
			"module.Registry.Resolve: dependency cycle detected among modules: %s",
			strings.Join(cyclic, ", "),
		)
	}

	return result, nil
}

// kahn runs Kahn's algorithm and returns the number of nodes processed.
// Used for cycle detection only — does not validate version constraints.
func kahn(inDegree map[string]int, adj map[string][]string) int {
	deg := make(map[string]int, len(inDegree))
	for k, v := range inDegree {
		deg[k] = v
	}

	var queue []string
	for name, d := range deg {
		if d == 0 {
			queue = append(queue, name)
		}
	}
	sort.Strings(queue)

	processed := 0
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		processed++
		var ready []string
		for _, s := range adj[name] {
			deg[s]--
			if deg[s] == 0 {
				ready = append(ready, s)
			}
		}
		sort.Strings(ready)
		queue = append(queue, ready...)
	}
	return processed
}
