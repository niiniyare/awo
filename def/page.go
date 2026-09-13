package def

import (
	"context"

	"github.com/google/uuid"
)

// PageBuilderSet allows module authors to override the auto-generated amis
// page schemas for specific views. Only declare builders for views that need
// customisation — the framework generates defaults for all omitted kinds.
//
// Generated schemas are cached in Redis (key: "page:{entity}:{version}:{tenant}",
// TTL 5 minutes) and invalidated on permission change or feature flag change.
type PageBuilderSet struct {
	// List overrides the auto-generated list/table page.
	List PageBuilder

	// Create overrides the auto-generated create form page.
	Create PageBuilder

	// Edit overrides the auto-generated edit form page.
	Edit PageBuilder

	// Detail overrides the auto-generated detail/read view page.
	Detail PageBuilder
}

// PageBuilder is a function that produces an amis JSON schema for a specific
// view. It receives the rendering context (actor roles, feature flags, tenant
// settings) and returns the raw schema as a map.
//
// The schema must conform to the pinned amis SDK version in web/sdk/.
// Permission-gated elements must be ABSENT from the schema (not just
// disabled) — the framework strips elements based on the actor's roles before
// caching.
//
// Returning nil instructs the framework to fall back to the auto-generated
// schema for this page kind.
type PageBuilder func(ctx context.Context, pctx PageContext) (map[string]any, error)

// PageContext carries the context available to a PageBuilder when generating
// a schema.
type PageContext struct {
	// Actor is the authenticated principal requesting the page.
	Actor *Actor

	// EntityName is the stable name of the entity being rendered.
	EntityName string

	// Kind identifies which view is being rendered.
	Kind PageKind

	// TenantSettings is a flat key-value map of tenant-specific settings
	// relevant to this entity. Read-only.
	TenantSettings map[string]string

	// EnabledFeatureFlags is the set of feature flag names that are active
	// for the current actor and tenant.
	EnabledFeatureFlags map[string]bool

	// RecordID is the UUID of the record being viewed or edited.
	// Populated for detail and edit views; uuid.Nil for list and create views.
	RecordID uuid.UUID

	// RecordState holds the current field values of the record being viewed.
	// Populated only when RecordID is non-nil and a RecordFetcher is wired.
	// Nil for list and create views, and when no RecordFetcher is configured.
	// PageBuilder implementations must treat nil as "no record data available".
	RecordState map[string]any
}
