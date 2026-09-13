// Package sduictx defines GeneratorContext — the immutable value that drives
// all SDUI generation decisions.
//
// GeneratorContext is passed by VALUE throughout the generator pipeline and to
// all plugin extension points. Plugins receive a copy and are physically unable
// to mutate the context seen by other pipeline stages. There are no setters and
// no pointer receivers on GeneratorContext.
//
// Construction is via GeneratorContextBuilder, which validates required fields
// before returning a sealed GeneratorContext value.
package sduictx

import (
	"fmt"

	"github.com/google/uuid"
)

// ViewMode is the intended rendering purpose for a widget tree.
// The generator selects which fields, columns, and actions appear based on
// the view mode.
type ViewMode string

const (
	// ViewModeList renders a paginated list with column nodes.
	ViewModeList ViewMode = "list"

	// ViewModeCreate renders a create form (no pre-populated data).
	ViewModeCreate ViewMode = "create"

	// ViewModeEdit renders an edit form (pre-populated with record data).
	ViewModeEdit ViewMode = "edit"

	// ViewModeDetail renders a read-only detail page.
	ViewModeDetail ViewMode = "detail"

	// ViewModeDashboard renders a dashboard panel layout.
	ViewModeDashboard ViewMode = "dashboard"
)

// ViewerContext is the minimal authorization interface required by the generator.
// It is a subset of auth.ViewerContext — defined here to avoid importing the
// full auth package into the sdui package tree.
//
// The concrete implementation is always auth.ViewerContext, passed through by
// the SDUI HTTP handler.
type ViewerContext interface {
	// TenantID returns the tenant UUID for this request.
	TenantID() uuid.UUID

	// Roles returns the viewer's role list for permission gating.
	Roles() []string

	// IsPlatformAdmin returns true if the viewer bypasses all permission gates.
	IsPlatformAdmin() bool

	// HasPermission reports whether the viewer holds the named permission identifier.
	// The generator calls this to decide which nodes are included in the tree.
	// Permission-gated nodes must be absent (not hidden) from the output.
	HasPermission(permissionID string) bool
}

// GeneratorContext is the immutable context for a single SDUI generation invocation.
// All generator stages and plugins receive this as a value copy.
//
// All fields are set at construction time and never mutated. The zero value is
// invalid — always construct via NewGeneratorContext.
type GeneratorContext struct {
	// ── Identity ──────────────────────────────────────────────────────────────

	// TenantID is the tenant for this request. Required.
	TenantID uuid.UUID

	// Viewer is the authorization subject. Required.
	// Generator uses Viewer.HasPermission to gate which nodes appear.
	Viewer ViewerContext

	// ── Generation parameters ─────────────────────────────────────────────────

	// ViewMode is the intended rendering purpose. Required.
	ViewMode ViewMode

	// ReadOnly forces all field nodes to be non-editable, regardless of their
	// individual ReadOnly settings. Used for ViewModeDetail.
	ReadOnly bool

	// Locale is the BCP 47 locale tag (e.g., "en-US", "ar-SA").
	// The generator uses locale for label resolution and date format hints.
	// Defaults to "en-US" when empty.
	Locale string

	// ── Cache dimensions ──────────────────────────────────────────────────────

	// SchemaFingerprint is the SHA-256 fingerprint of the CompiledSchema for
	// this entity. Set by the SDUI handler from the compiled schema.
	// Included in cache keys to invalidate caches after schema changes.
	SchemaFingerprint string

	// PermFingerprint is the opaque permission fingerprint produced by
	// PolicyEvaluator.ComputeFingerprint(viewer). SDUI never computes this —
	// it is always provided by the authorization layer.
	// Included in cache keys to isolate widget trees by permission set.
	PermFingerprint string

	// ── Renderer targeting ────────────────────────────────────────────────────

	// RendererID identifies the target renderer (e.g., "amis", "flutter", "pdf").
	// Required. Included in cache keys.
	RendererID string

	// RendererVersion is a semver token for the renderer implementation.
	// When the renderer is upgraded, this token changes and invalidates all
	// rendered output caches (Level 3) automatically.
	RendererVersion string

	// ── Entity targeting ──────────────────────────────────────────────────────

	// EntityName is the entity being rendered (e.g., "finance_invoice").
	// Required. Included in cache keys.
	EntityName string
}

// Locale returns the effective locale, defaulting to "en-US" when not set.
func (g GeneratorContext) EffectiveLocale() string {
	if g.Locale == "" {
		return "en-US"
	}
	return g.Locale
}

// WithReadOnly returns a copy of the context with ReadOnly set to true.
// Used when deriving a detail-view context from an edit-view context.
func (g GeneratorContext) WithReadOnly() GeneratorContext {
	g.ReadOnly = true
	return g
}

// WithViewMode returns a copy of the context with a different view mode.
func (g GeneratorContext) WithViewMode(mode ViewMode) GeneratorContext {
	g.ViewMode = mode
	return g
}

// Validate checks that all required fields are present.
// Returns an error describing the first missing field.
func (g GeneratorContext) Validate() error {
	if g.TenantID == uuid.Nil {
		return fmt.Errorf("sduictx.GeneratorContext: TenantID is required")
	}
	if g.Viewer == nil {
		return fmt.Errorf("sduictx.GeneratorContext: Viewer is required")
	}
	if g.ViewMode == "" {
		return fmt.Errorf("sduictx.GeneratorContext: ViewMode is required")
	}
	if g.RendererID == "" {
		return fmt.Errorf("sduictx.GeneratorContext: RendererID is required")
	}
	if g.EntityName == "" {
		return fmt.Errorf("sduictx.GeneratorContext: EntityName is required")
	}
	return nil
}

// GeneratorContextBuilder constructs a GeneratorContext with validation.
// Use this instead of struct literal construction to catch missing fields early.
type GeneratorContextBuilder struct {
	ctx GeneratorContext
}

// NewGeneratorContext returns a builder seeded with required fields.
func NewGeneratorContext(tenantID uuid.UUID, viewer ViewerContext, entityName string, viewMode ViewMode, rendererID string) *GeneratorContextBuilder {
	return &GeneratorContextBuilder{
		ctx: GeneratorContext{
			TenantID:   tenantID,
			Viewer:     viewer,
			EntityName: entityName,
			ViewMode:   viewMode,
			RendererID: rendererID,
		},
	}
}

// WithLocale sets the BCP 47 locale tag.
func (b *GeneratorContextBuilder) WithLocale(locale string) *GeneratorContextBuilder {
	b.ctx.Locale = locale
	return b
}

// WithSchemaFingerprint sets the compiled schema fingerprint.
func (b *GeneratorContextBuilder) WithSchemaFingerprint(fp string) *GeneratorContextBuilder {
	b.ctx.SchemaFingerprint = fp
	return b
}

// WithPermFingerprint sets the permission fingerprint from PolicyEvaluator.
func (b *GeneratorContextBuilder) WithPermFingerprint(fp string) *GeneratorContextBuilder {
	b.ctx.PermFingerprint = fp
	return b
}

// WithRendererVersion sets the renderer version token.
func (b *GeneratorContextBuilder) WithRendererVersion(v string) *GeneratorContextBuilder {
	b.ctx.RendererVersion = v
	return b
}

// WithReadOnly forces all generated fields to be read-only.
func (b *GeneratorContextBuilder) WithReadOnly() *GeneratorContextBuilder {
	b.ctx.ReadOnly = true
	return b
}

// Build validates and returns the immutable GeneratorContext.
// Returns an error if any required field is missing.
func (b *GeneratorContextBuilder) Build() (GeneratorContext, error) {
	if err := b.ctx.Validate(); err != nil {
		return GeneratorContext{}, err
	}
	return b.ctx, nil
}
