// Package registry maintains the mapping from NodeKind to widget definitions.
//
// The global registry is populated during init() and sealed at bootstrap.
// After sealing, all reads are lock-free (concurrent-safe). Writes after
// sealing panic immediately — this catches incorrect usage at startup.
//
// Tests must use NewIsolated() to get a private registry instance with no
// shared state. Never call the global Register/Seal functions in tests.
//
// Registration lifecycle:
//  1. Each package registers its widgets via init() → registry.Register(...)
//  2. Bootstrap calls registry.Seal() once all init() functions have run.
//  3. After Seal(), Lookup() is safe from any goroutine without locks.
//  4. Any Register() call after Seal() panics with a clear message.
package registry

import (
	"fmt"
	"sync"
	"sync/atomic"

	"awo.so/awo/sdui/widget"
)

// WidgetDef describes a registered widget kind.
// Every NodeKind used by the generator or renderer must have a corresponding
// WidgetDef in the registry or rendering will fail at validation time.
type WidgetDef struct {
	// Kind is the NodeKind this definition covers.
	Kind widget.NodeKind

	// DisplayName is a human-readable name for error messages and tooling.
	DisplayName string

	// AllowedChildren lists which NodeKinds are valid children of this node.
	// nil means any children are allowed. Empty (non-nil) means no children allowed.
	AllowedChildren []widget.NodeKind

	// RequiresDataSource is true when the node is invalid without a DataSource.
	// Used by the validation pass.
	RequiresDataSource bool

	// IsContainer is true when the node may have Children.
	IsContainer bool

	// IsInputField is true when the node represents a form field that binds
	// to a data value. Input fields require Name to be non-empty.
	IsInputField bool

	// SupportedRenderers lists renderer IDs that support this NodeKind.
	// nil means all renderers support it (assume universal support).
	// Non-nil means only the listed renderers can render this node; other
	// renderers will receive an ErrUnknownWidget error.
	SupportedRenderers []string

	// FrameworkVersion is the minimum framework version that introduced this
	// NodeKind. Used for compatibility checks.
	FrameworkVersion string
}

// Registry holds widget definitions. The global singleton is this package's
// unexported var; tests use NewIsolated().
type Registry struct {
	mu     sync.RWMutex
	defs   map[widget.NodeKind]WidgetDef
	sealed atomic.Bool
}

// global is the production-use singleton registry.
var global = &Registry{
	defs: make(map[widget.NodeKind]WidgetDef),
}

// NewIsolated returns a new, empty, unsealed Registry for use in tests.
// It has no connection to the global registry and no pre-registered widgets.
// Call Register on the returned instance to populate it for the test.
func NewIsolated() *Registry {
	return &Registry{
		defs: make(map[widget.NodeKind]WidgetDef),
	}
}

// Register adds a WidgetDef to the global registry.
// Must be called from init() only — never from handlers, tests, or constructors.
// Panics if called after Seal() or if the NodeKind is already registered.
func Register(def WidgetDef) {
	if err := global.Register(def); err != nil {
		panic(fmt.Sprintf("registry.Register: %v", err))
	}
}

// Seal marks the global registry as read-only. Must be called once at
// bootstrap after all init() functions have run. Subsequent Register calls
// will panic. Seal is idempotent — calling it multiple times is safe.
func Seal() { global.Seal() }

// Lookup returns the WidgetDef for the given NodeKind from the global registry.
// Returns false if the kind is not registered.
func Lookup(kind widget.NodeKind) (WidgetDef, bool) { return global.Lookup(kind) }

// IsSealed reports whether the global registry has been sealed.
func IsSealed() bool { return global.IsSealed() }

// AllKinds returns all registered NodeKinds from the global registry.
// Returns a new slice on each call — safe to modify.
func AllKinds() []widget.NodeKind { return global.AllKinds() }

// ── Instance methods (used by both global and isolated registries) ─────────────

// Register adds a WidgetDef to the registry.
// Returns an error if the registry is sealed or if the NodeKind is already registered.
func (r *Registry) Register(def WidgetDef) error {
	if r.sealed.Load() {
		return fmt.Errorf("registry is sealed: cannot register NodeKind %q after bootstrap", def.Kind)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.defs[def.Kind]; exists {
		return fmt.Errorf("NodeKind %q is already registered (duplicate registration is a bootstrap error)", def.Kind)
	}
	r.defs[def.Kind] = def
	return nil
}

// Seal marks the registry as read-only.
func (r *Registry) Seal() { r.sealed.Store(true) }

// IsSealed reports whether the registry has been sealed.
func (r *Registry) IsSealed() bool { return r.sealed.Load() }

// Lookup returns the WidgetDef for the given NodeKind.
// After Seal(), this is lock-free for concurrent callers.
func (r *Registry) Lookup(kind widget.NodeKind) (WidgetDef, bool) {
	if r.sealed.Load() {
		// After sealing, the map is never written — no lock needed.
		def, ok := r.defs[kind]
		return def, ok
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.defs[kind]
	return def, ok
}

// AllKinds returns all registered NodeKinds. Returns a new slice on each call.
func (r *Registry) AllKinds() []widget.NodeKind {
	if r.sealed.Load() {
		kinds := make([]widget.NodeKind, 0, len(r.defs))
		for k := range r.defs {
			kinds = append(kinds, k)
		}
		return kinds
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	kinds := make([]widget.NodeKind, 0, len(r.defs))
	for k := range r.defs {
		kinds = append(kinds, k)
	}
	return kinds
}

// MustLookup returns the WidgetDef for the given NodeKind.
// Panics if the kind is not registered. Use in renderer internals where an
// unknown kind represents a programming error, not a user error.
func (r *Registry) MustLookup(kind widget.NodeKind) WidgetDef {
	def, ok := r.Lookup(kind)
	if !ok {
		panic(fmt.Sprintf("registry.MustLookup: NodeKind %q is not registered — register it in an init() function", kind))
	}
	return def
}

// Validate checks that all NodeKinds listed in an AllowedChildren slice are
// themselves registered. Returns an error for the first unknown child kind.
// Call this at bootstrap after Seal() to catch missing registrations.
func (r *Registry) Validate() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for parentKind, def := range r.defs {
		for _, childKind := range def.AllowedChildren {
			if _, ok := r.defs[childKind]; !ok {
				return fmt.Errorf("registry.Validate: NodeKind %q lists allowed child %q which is not registered",
					parentKind, childKind)
			}
		}
	}
	return nil
}
