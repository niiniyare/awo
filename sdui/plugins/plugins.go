// Package plugins implements the SDUI extension pipeline.
//
// There are nine extension points. Plugins register at a specific extension
// point with an integer priority. Execution order is ascending by priority
// (lower number = higher priority = runs first). Duplicate priorities within
// the same extension point are a bootstrap error — detected at Seal().
//
// Plugins are registered from init() functions, before the registry is sealed.
// After sealing, the plugin list is frozen and concurrent lookups are lock-free.
//
// Plugin errors:
//   - Recoverable (default): the pipeline logs the error and continues with the
//     unmodified input (the plugin's output is discarded on error).
//   - Fatal: the plugin returns PluginFatalError — the pipeline aborts and
//     returns an error to the caller. Use only for conditions that indicate
//     data corruption or security violations.
//
// Forbidden operations for any plugin:
//   - Modifying GeneratorContext (received as value — physically impossible)
//   - Bypassing permission gates (plugin sees the already-gated widget tree)
//   - Injecting renderer-specific constructs into the widget tree
//   - Modifying the schema fingerprint or permission fingerprint
package plugins

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"

	"awo.so/awo/sdui/sduictx"
	"awo.so/awo/sdui/widget"
)

// ExtensionPoint identifies a named hook in the SDUI pipeline.
type ExtensionPoint string

const (
	// ExtPreGeneration runs before generation. Receives the compiled entity
	// schema (as map[string]any) and may return a modified copy.
	// Use to add synthetic fields or reorder sections before tree building.
	ExtPreGeneration ExtensionPoint = "pre_generation"

	// ExtFieldNodeOverride runs for each field node after the generator emits it.
	// A plugin may replace the node for a specific field by returning a non-nil node.
	// Return nil to use the default generator output.
	ExtFieldNodeOverride ExtensionPoint = "field_node_override"

	// ExtPostGeneration runs after the complete widget tree is built.
	// Receives the tree root and may return a modified root.
	ExtPostGeneration ExtensionPoint = "post_generation"

	// ExtPreRender runs immediately before rendering. The tree has passed
	// validation. Plugins may make final renderer-agnostic modifications.
	ExtPreRender ExtensionPoint = "pre_render"

	// ExtCustomNodeRenderer provides a renderer for a specific NodeKind.
	// The plugin returns renderer-specific output for that NodeKind.
	// Only one plugin per (NodeKind, RendererID) pair is allowed.
	ExtCustomNodeRenderer ExtensionPoint = "custom_node_renderer"

	// ExtPostRender runs after rendering. Plugins may modify the renderer output.
	ExtPostRender ExtensionPoint = "post_render"

	// ExtDashboardPanel registers a custom dashboard panel type.
	ExtDashboardPanel ExtensionPoint = "dashboard_panel"

	// ExtValidation adds custom validation rules to the validation pass.
	ExtValidation ExtensionPoint = "validation"

	// ExtLayoutTransform modifies the layout computation for a section.
	ExtLayoutTransform ExtensionPoint = "layout_transform"
)

// PluginFatalError signals that a plugin encountered an unrecoverable condition.
// Wrapping an error in PluginFatalError causes the pipeline to abort.
type PluginFatalError struct {
	PluginName string
	Point      ExtensionPoint
	Cause      error
}

func (e *PluginFatalError) Error() string {
	return fmt.Sprintf("plugin %q at %q: fatal: %v", e.PluginName, e.Point, e.Cause)
}

func (e *PluginFatalError) Unwrap() error { return e.Cause }

// IsFatal reports whether err is (or wraps) a PluginFatalError.
func IsFatal(err error) bool {
	var fe *PluginFatalError
	return errors.As(err, &fe)
}

// TreeTransformFunc is a plugin function for extension points that transform the
// widget tree (ExtPostGeneration, ExtPreRender).
// Receives a copy of the GeneratorContext (immutable — modifications have no effect).
// Returns the (possibly modified) root node.
// Return a PluginFatalError to abort the pipeline.
type TreeTransformFunc func(root *widget.Node, ctx sduictx.GeneratorContext) (*widget.Node, error)

// FieldNodeOverrideFunc is a plugin function for ExtFieldNodeOverride.
// fieldName is the entity field name. Returns nil to use the default node.
type FieldNodeOverrideFunc func(fieldName string, defaultNode *widget.Node, ctx sduictx.GeneratorContext) (*widget.Node, error)

// ValidationFunc is a plugin function for ExtValidation.
// Receives the complete widget tree and returns a list of validation errors.
// Returning a non-nil error from the function itself (not ValidationIssue) is fatal.
type ValidationFunc func(root *widget.Node, ctx sduictx.GeneratorContext) ([]ValidationIssue, error)

// ValidationIssue is a single validation finding from a plugin.
type ValidationIssue struct {
	NodeID   string
	NodeKind widget.NodeKind
	Field    string
	Message  string
	Fatal    bool
}

// registration is an internal record of a registered plugin.
type registration struct {
	name     string
	priority int
	point    ExtensionPoint
	fn       any // concrete type depends on ExtensionPoint
}

// Pipeline holds the ordered plugin registrations for all extension points.
type Pipeline struct {
	mu     sync.RWMutex
	regs   map[ExtensionPoint][]registration
	sealed atomic.Bool
}

// global is the production pipeline singleton.
var global = &Pipeline{
	regs: make(map[ExtensionPoint][]registration),
}

// NewIsolated returns a new, empty pipeline for use in tests.
func NewIsolated() *Pipeline {
	return &Pipeline{regs: make(map[ExtensionPoint][]registration)}
}

// RegisterTreeTransform registers a tree-transform plugin at the given extension
// point (must be ExtPostGeneration or ExtPreRender).
// Priority must be unique within the extension point — duplicate = bootstrap error.
// Call from init() only.
func RegisterTreeTransform(point ExtensionPoint, name string, priority int, fn TreeTransformFunc) {
	if err := global.RegisterTreeTransform(point, name, priority, fn); err != nil {
		panic(fmt.Sprintf("plugins.RegisterTreeTransform: %v", err))
	}
}

// RegisterFieldNodeOverride registers an ExtFieldNodeOverride plugin.
func RegisterFieldNodeOverride(name string, priority int, fn FieldNodeOverrideFunc) {
	if err := global.RegisterFieldNodeOverride(name, priority, fn); err != nil {
		panic(fmt.Sprintf("plugins.RegisterFieldNodeOverride: %v", err))
	}
}

// RegisterValidation registers an ExtValidation plugin.
func RegisterValidation(name string, priority int, fn ValidationFunc) {
	if err := global.RegisterValidation(name, priority, fn); err != nil {
		panic(fmt.Sprintf("plugins.RegisterValidation: %v", err))
	}
}

// Seal freezes the global pipeline. Called at bootstrap after all init() runs.
func Seal() {
	if err := global.seal(); err != nil {
		panic(fmt.Sprintf("plugins.Seal: %v", err))
	}
}

// GlobalPipeline returns the production pipeline singleton.
// Callers must not register into it after bootstrap.
func GlobalPipeline() *Pipeline { return global }

// ── Pipeline instance methods ─────────────────────────────────────────────────

func (p *Pipeline) RegisterTreeTransform(point ExtensionPoint, name string, priority int, fn TreeTransformFunc) error {
	if point != ExtPostGeneration && point != ExtPreRender {
		return fmt.Errorf("extension point %q does not accept TreeTransformFunc (use ExtPostGeneration or ExtPreRender)", point)
	}
	return p.register(registration{name: name, priority: priority, point: point, fn: fn})
}

func (p *Pipeline) RegisterFieldNodeOverride(name string, priority int, fn FieldNodeOverrideFunc) error {
	return p.register(registration{name: name, priority: priority, point: ExtFieldNodeOverride, fn: fn})
}

func (p *Pipeline) RegisterValidation(name string, priority int, fn ValidationFunc) error {
	return p.register(registration{name: name, priority: priority, point: ExtValidation, fn: fn})
}

func (p *Pipeline) register(r registration) error {
	if p.sealed.Load() {
		return fmt.Errorf("pipeline is sealed: cannot register plugin %q after bootstrap", r.name)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	existing := p.regs[r.point]
	for _, e := range existing {
		if e.priority == r.priority {
			return fmt.Errorf("duplicate priority %d at extension point %q: plugins %q and %q (duplicate priority is a bootstrap error — assign unique priorities)",
				r.priority, r.point, e.name, r.name)
		}
	}
	p.regs[r.point] = append(p.regs[r.point], r)
	return nil
}

// seal validates and freezes the pipeline. Sorts registrations by priority.
func (p *Pipeline) seal() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Sort all extension points by ascending priority for deterministic execution.
	for point, regs := range p.regs {
		sorted := make([]registration, len(regs))
		copy(sorted, regs)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].priority < sorted[j].priority
		})
		p.regs[point] = sorted
	}
	p.sealed.Store(true)
	return nil
}

// Seal freezes the pipeline instance.
func (p *Pipeline) Seal() error { return p.seal() }

// RunTreeTransforms executes all registered plugins at the given extension
// point in priority order. Returns the (possibly modified) root.
// On recoverable plugin error: logs error, continues with previous root.
// On fatal plugin error: returns error immediately.
func (p *Pipeline) RunTreeTransforms(point ExtensionPoint, root *widget.Node, ctx sduictx.GeneratorContext) (*widget.Node, error) {
	regs := p.lookupRegs(point)
	for _, r := range regs {
		fn, ok := r.fn.(TreeTransformFunc)
		if !ok {
			continue
		}
		result, err := fn(root, ctx)
		if err != nil {
			if IsFatal(err) {
				return nil, fmt.Errorf("plugin %q at %q: %w", r.name, point, err)
			}
			// Recoverable: discard result, log (caller may log), continue.
			continue
		}
		if result != nil {
			root = result
		}
	}
	return root, nil
}

// RunFieldNodeOverrides executes all ExtFieldNodeOverride plugins for a single field.
// Returns the final node (default if no plugin overrides it).
func (p *Pipeline) RunFieldNodeOverrides(fieldName string, defaultNode *widget.Node, ctx sduictx.GeneratorContext) (*widget.Node, error) {
	regs := p.lookupRegs(ExtFieldNodeOverride)
	current := defaultNode
	for _, r := range regs {
		fn, ok := r.fn.(FieldNodeOverrideFunc)
		if !ok {
			continue
		}
		result, err := fn(fieldName, current, ctx)
		if err != nil {
			if IsFatal(err) {
				return nil, fmt.Errorf("plugin %q (field_node_override): %w", r.name, err)
			}
			continue
		}
		if result != nil {
			current = result
		}
	}
	return current, nil
}

// RunValidation executes all ExtValidation plugins. Aggregates issues.
// Returns an error only if a plugin function itself fails fatally.
func (p *Pipeline) RunValidation(root *widget.Node, ctx sduictx.GeneratorContext) ([]ValidationIssue, error) {
	regs := p.lookupRegs(ExtValidation)
	var all []ValidationIssue
	for _, r := range regs {
		fn, ok := r.fn.(ValidationFunc)
		if !ok {
			continue
		}
		issues, err := fn(root, ctx)
		if err != nil {
			if IsFatal(err) {
				return nil, fmt.Errorf("plugin %q (validation): %w", r.name, err)
			}
			continue
		}
		all = append(all, issues...)
	}
	return all, nil
}

func (p *Pipeline) lookupRegs(point ExtensionPoint) []registration {
	if p.sealed.Load() {
		return p.regs[point] // lock-free after seal
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.regs[point]
}
