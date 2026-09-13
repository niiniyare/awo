// Package renderer defines the renderer contract for the SDUI framework.
//
// A Renderer translates a validated widget tree into target-specific output.
// Each renderer registers itself with a unique RendererID (e.g., "amis",
// "flutter", "pdf") and is selected at request time by the SDUI handler.
//
// The Renderer interface is the only contract between the SDUI pipeline and
// a renderer implementation. The AMIS renderer, Flutter renderer, and PDF
// renderer all implement this interface independently with no shared code
// beyond this package.
//
// RenderedOutput is a typed union — not map[string]any — to support renderers
// that produce binary output (PDF) or typed structs (Flutter) alongside JSON.
package renderer

import (
	"fmt"

	"awo.so/awo/sdui/sduictx"
	"awo.so/awo/sdui/widget"
)

// Renderer translates a validated widget tree into target-specific output.
// Implementations must be safe for concurrent use — Render may be called
// from multiple goroutines simultaneously.
type Renderer interface {
	// ID returns the unique renderer identifier (e.g., "amis", "flutter", "pdf").
	// Matches GeneratorContext.RendererID for request routing.
	ID() string

	// Version returns the renderer version token used in cache key construction.
	// Change this whenever the renderer's output format changes, to invalidate
	// stale Level-3 cache entries automatically.
	Version() string

	// Render translates root and all its descendants into target-specific output.
	// root must not be nil and must have passed validation.
	// Returns an error if any node cannot be rendered.
	Render(root *widget.Node, ctx RendererContext) (RenderedOutput, error)
}

// RendererContext carries rendering hints that are renderer-specific.
// Unlike GeneratorContext (semantic, renderer-independent), RendererContext
// may contain renderer-specific values such as CSS framework hints, theme
// tokens, or locale formatting preferences.
//
// RendererContext is constructed by the SDUI handler alongside GeneratorContext
// and passed only to the Renderer — never to the Generator.
type RendererContext struct {
	// GenCtx is the originating generator context. Provides tenant, viewer,
	// entity, and cache dimension values that renderers may need for API URLs.
	GenCtx sduictx.GeneratorContext

	// Theme is the active theme identifier (e.g., "default", "compact", "dark").
	// Renderers interpret theme names in their own way.
	Theme string

	// GridSystem is the column grid system name (e.g., "bootstrap", "tailwind").
	// AMIS renderer maps LayoutHint.ColSpan to the appropriate CSS class.
	GridSystem string

	// DateFormat is the display date format for this locale (e.g., "DD/MM/YYYY").
	// Derived from GenCtx.Locale by the handler. Renderers use this for
	// date column display and date picker hint strings.
	DateFormat string

	// TimeFormat is the display time format (e.g., "HH:mm", "hh:mm A").
	DateTimeFormat string

	// DecimalSeparator is the locale decimal separator ("." or ",").
	DecimalSeparator string

	// ThousandSeparator is the locale thousands separator ("," or ".").
	ThousandSeparator string

	// RTL is true when the locale requires right-to-left layout.
	RTL bool
}

// RenderedOutput is a typed union of possible renderer outputs.
// Exactly one of AMISSchema, Raw, or Typed is non-nil depending on Format.
type RenderedOutput struct {
	// Format identifies the output format (e.g., "amis-json", "pdf-bytes", "flutter-tree").
	// Callers switch on Format to access the correct field.
	Format string

	// AMISSchema is the AMIS JSON schema as a Go map. Non-nil when Format == "amis-json".
	// Safe for encoding/json marshalling.
	AMISSchema map[string]any

	// Raw is the binary output. Non-nil when Format == "pdf-bytes" or other binary formats.
	Raw []byte

	// Typed is a renderer-specific typed value. Non-nil for renderers that produce
	// a typed Go struct (e.g., a Flutter widget tree model).
	// Callers must type-assert to the renderer's documented output type.
	Typed any
}

// OutputFormat constants for well-known renderer formats.
const (
	FormatAMISJSON    = "amis-json"
	FormatPDFBytes    = "pdf-bytes"
	FormatFlutterTree = "flutter-tree"
)

// NewAMISOutput constructs a RenderedOutput for the AMIS renderer.
func NewAMISOutput(schema map[string]any) RenderedOutput {
	return RenderedOutput{Format: FormatAMISJSON, AMISSchema: schema}
}

// NewRawOutput constructs a RenderedOutput for binary-format renderers.
func NewRawOutput(format string, data []byte) RenderedOutput {
	return RenderedOutput{Format: format, Raw: data}
}

// NewTypedOutput constructs a RenderedOutput for typed-struct renderers.
func NewTypedOutput(format string, v any) RenderedOutput {
	return RenderedOutput{Format: format, Typed: v}
}

// Registry is a collection of named renderers. The SDUI handler looks up a
// renderer by RendererID at request time.
type Registry struct {
	renderers map[string]Renderer
}

// NewRegistry returns an empty renderer registry.
func NewRegistry() *Registry {
	return &Registry{renderers: make(map[string]Renderer)}
}

// Register adds a renderer. Returns an error if a renderer with the same ID
// is already registered.
func (r *Registry) Register(renderer Renderer) error {
	id := renderer.ID()
	if _, exists := r.renderers[id]; exists {
		return fmt.Errorf("renderer.Registry: renderer %q is already registered", id)
	}
	r.renderers[id] = renderer
	return nil
}

// Lookup returns the renderer for the given ID.
func (r *Registry) Lookup(id string) (Renderer, bool) {
	renderer, ok := r.renderers[id]
	return renderer, ok
}

// MustLookup returns the renderer for the given ID, panicking if not found.
func (r *Registry) MustLookup(id string) Renderer {
	renderer, ok := r.renderers[id]
	if !ok {
		panic(fmt.Sprintf("renderer.Registry: no renderer registered for ID %q", id))
	}
	return renderer
}
