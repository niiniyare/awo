// Package sdui provides the HTTP handler layer for the SDUI engine.
//
// # Responsibilities
//
// This package owns the HTTP surface of the SDUI subsystem. It:
//   - Extracts request parameters (entity name, view mode, locale, renderer ID).
//   - Bridges auth.ViewerContext → sduictx.ViewerContext via adapt.ViewerAdapter.
//   - Builds the sduictx.GeneratorContext for each request.
//   - Delegates all generation and rendering to engine.Engine.
//   - Serializes the rendered output as JSON.
//   - Sets ETag and Cache-Control headers.
//
// # Endpoints
//
// All endpoints are mounted at /api/v1/ui by the router:
//
//	GET /api/v1/ui/{module}/{entity}          → list view
//	GET /api/v1/ui/{module}/{entity}/create   → create form
//	GET /api/v1/ui/{module}/{entity}/{id}     → detail view
//	GET /api/v1/ui/{module}/{entity}/{id}/edit → edit form
//	GET /api/v1/ui/nav                        → sidebar navigation schema
//
// The renderer is selected via the Accept-SDUI-Renderer header (default: amis).
// Locale is selected via the Accept-Language header (default: en-US).
//
// # No business logic
//
// This package contains no SDUI logic. All generation, caching, and rendering
// decisions are delegated to engine.Engine. HTTP handlers are thin.
package sdui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"awo.so/awo/auth"
	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"awo.so/awo/sdui/adapt"
	"awo.so/awo/sdui/cache"
	"awo.so/awo/sdui/engine"
	"awo.so/awo/sdui/renderer"
	"awo.so/awo/sdui/sduictx"
)

// ── Provider interfaces ────────────────────────────────────────────────────────

// SettingsProvider fetches a flat map of tenant-level settings for use in
// PageContext. Implementations should be fast (cache-backed). The handler
// calls this once per page builder invocation; errors produce an empty map
// (non-blocking) so a settings service failure never prevents page delivery.
type SettingsProvider interface {
	// TenantSettings returns a flat key-value map of settings for tenantID.
	// Implementations must not return nil — return an empty map on error.
	TenantSettings(ctx context.Context, tenantID uuid.UUID) map[string]string
}

// FlagsProvider evaluates which feature flags are active for a given viewer
// and tenant. Implementations should be cache-backed. Errors produce an empty
// map (non-blocking).
type FlagsProvider interface {
	// EnabledFlags returns the set of active feature flag names for the given
	// viewer+tenant combination. Keys are flag identifiers; values are always
	// true (absent keys are disabled). Implementations must not return nil.
	EnabledFlags(ctx context.Context, tenantID uuid.UUID, viewer auth.ViewerContext) map[string]bool
}

// RecordFetcher retrieves a record's field values by entity name and record ID.
// Used to populate PageContext.RecordState for detail and edit PageBuilder
// overrides. Implementations must respect the viewer's tenant context (RLS).
type RecordFetcher interface {
	// FetchRecord returns the field values for entityName/id.
	// Returns (nil, nil) when the record does not exist.
	// Returns (nil, err) on a storage error.
	FetchRecord(ctx context.Context, entityName string, id uuid.UUID) (map[string]any, error)
}

// ── Noop provider implementations ────────────────────────────────────────────

// noopSettingsProvider returns an empty map. Used when no SettingsProvider is wired.
type noopSettingsProvider struct{}

func (noopSettingsProvider) TenantSettings(_ context.Context, _ uuid.UUID) map[string]string {
	return map[string]string{}
}

// noopFlagsProvider returns an empty map. Used when no FlagsProvider is wired.
type noopFlagsProvider struct{}

func (noopFlagsProvider) EnabledFlags(_ context.Context, _ uuid.UUID, _ auth.ViewerContext) map[string]bool {
	return map[string]bool{}
}

// noopRecordFetcher returns nil,nil. Used when no RecordFetcher is wired.
type noopRecordFetcher struct{}

func (noopRecordFetcher) FetchRecord(_ context.Context, _ string, _ uuid.UUID) (map[string]any, error) {
	return nil, nil
}

// NavEntry is a single navigation item linking to an entity list view.
type NavEntry struct {
	Module  string `json:"module"`
	Label   string `json:"label"`
	Entity  string `json:"entity"`
	ListURL string `json:"listUrl"`
	Icon    string `json:"icon,omitempty"`
}

// NavModule groups NavEntry values by module for sidebar rendering.
type NavModule struct {
	Module  string     `json:"module"`
	Label   string     `json:"label"`
	Entries []NavEntry `json:"entries"`
}

const (
	// defaultRendererID is used when no renderer is specified in the request.
	defaultRendererID = "amis"

	// headerRenderer is the HTTP request header for renderer selection.
	headerRenderer = "Accept-SDUI-Renderer"

	// headerLocale is the HTTP request header for locale selection.
	headerLocale = "Accept-Language"

	// headerETag is the HTTP response header for conditional requests.
	headerETag = "ETag"

	// headerCacheControl is the HTTP response header for cache hints.
	headerCacheControl = "Cache-Control"

	// sduiPublicMaxAge is the max-age for SDUI responses (5 minutes).
	sduiPublicMaxAge = "private, max-age=300"
)

// Handler handles SDUI page requests for a compiled schema.
// Safe for concurrent use; holds no mutable state.
type Handler struct {
	schema    *compiler.CompiledSchema
	engine    *engine.Engine
	evaluator auth.PolicyEvaluator // may be nil
	grants    adapt.GrantIndex     // pre-built permission index
	schemaFPs map[string]string    // entityName → fingerprint (precomputed)

	// Optional dynamic data providers for PageContext enrichment.
	settings SettingsProvider
	flags    FlagsProvider
	fetcher  RecordFetcher
}

// HandlerOption is a functional option for configuring a Handler.
type HandlerOption func(*Handler)

// WithSettingsProvider wires a SettingsProvider that populates
// PageContext.TenantSettings on every PageBuilder invocation.
// When not set, PageContext.TenantSettings is always an empty map.
func WithSettingsProvider(sp SettingsProvider) HandlerOption {
	return func(h *Handler) { h.settings = sp }
}

// WithFlagsProvider wires a FlagsProvider that populates
// PageContext.EnabledFeatureFlags on every PageBuilder invocation.
// When not set, PageContext.EnabledFeatureFlags is always an empty map.
func WithFlagsProvider(fp FlagsProvider) HandlerOption {
	return func(h *Handler) { h.flags = fp }
}

// WithRecordFetcher wires a RecordFetcher that populates
// PageContext.RecordState for detail and edit views.
// When not set, PageContext.RecordState is always nil.
func WithRecordFetcher(rf RecordFetcher) HandlerOption {
	return func(h *Handler) { h.fetcher = rf }
}

// New constructs a Handler.
//
//   - schema is the compiled schema produced by compiler.Compile. Required.
//   - eng is the configured SDUI engine. Required.
//   - evaluator is the policy evaluator for field permission gating. May be nil.
//   - opts are optional functional options for dynamic data providers.
//
// New precomputes schema fingerprints and the grant index for O(1) access
// during request handling. When no provider options are supplied all three
// providers default to noop implementations that return empty maps/nil.
func New(schema *compiler.CompiledSchema, eng *engine.Engine, evaluator auth.PolicyEvaluator, opts ...HandlerOption) *Handler {
	fps := make(map[string]string, len(schema.Entities))
	for _, es := range schema.Entities {
		fps[es.QualifiedName] = adapt.SchemaFingerprint(es)
	}
	h := &Handler{
		schema:    schema,
		engine:    eng,
		evaluator: evaluator,
		grants:    adapt.BuildGrantIndex(schema.CapabilityGrants),
		schemaFPs: fps,
		// Noop defaults — never nil; simplifies nil-guard-free provider calls.
		settings: noopSettingsProvider{},
		flags:    noopFlagsProvider{},
		fetcher:  noopRecordFetcher{},
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// Register mounts SDUI endpoints on group g.
// The caller is responsible for applying auth middleware before the group.
//
//	ui := app.Group("/api/v1/ui")
//	ui.Use(middleware.RequireAuth(...))
//	h.Register(ui)
func (h *Handler) Register(g fiber.Router) {
	// Navigation. Must be registered before /:module/:resource to avoid
	// "nav" being matched as a :module parameter.
	g.Get("/nav", h.nav)

	// List view.
	g.Get("/:module/:resource", h.list)

	// Create form.
	g.Get("/:module/:resource/create", h.create)

	// Edit form.
	g.Get("/:module/:resource/:id/edit", h.edit)

	// Detail view. Register after /create and /:id/edit to avoid ambiguity.
	g.Get("/:module/:resource/:id", h.detail)
}

// ── view handlers ─────────────────────────────────────────────────────────────

// nav returns the sidebar navigation schema.
// Groups entities by module; only entities with a declared Read permission
// are included. Order matches CompiledSchema.Entities declaration order.
//
// GET /api/v1/ui/nav
func (h *Handler) nav(c *fiber.Ctx) error {
	seen := make(map[string]int) // module → index in result
	var result []NavModule

	for _, es := range h.schema.Entities {
		if len(es.Permissions.Read) == 0 {
			continue // no read permission declared — omit from nav
		}
		idx, ok := seen[es.Module]
		if !ok {
			idx = len(result)
			seen[es.Module] = idx
			moduleLabel := es.Module
			if len(moduleLabel) > 0 {
				moduleLabel = strings.ToUpper(moduleLabel[:1]) + moduleLabel[1:]
			}
			result = append(result, NavModule{
				Module: es.Module,
				Label:  moduleLabel,
			})
		}
		result[idx].Entries = append(result[idx].Entries, NavEntry{
			Module:  es.Module,
			Label:   es.LabelPlural,
			Entity:  es.QualifiedName,
			ListURL: "/ui/" + es.Module + "/" + es.APIResource,
			Icon:    es.Icon,
		})
	}
	return c.JSON(result)
}

func (h *Handler) list(c *fiber.Ctx) error {
	return h.handle(c, sduictx.ViewModeList, false)
}

func (h *Handler) create(c *fiber.Ctx) error {
	return h.handle(c, sduictx.ViewModeCreate, false)
}

func (h *Handler) detail(c *fiber.Ctx) error {
	return h.handle(c, sduictx.ViewModeDetail, true)
}

func (h *Handler) edit(c *fiber.Ctx) error {
	return h.handle(c, sduictx.ViewModeEdit, false)
}

// handle is the shared implementation for all view handlers.
func (h *Handler) handle(c *fiber.Ctx, mode sduictx.ViewMode, readOnly bool) error {
	module := c.Params("module")
	resource := c.Params("resource")

	slog.Info("sdui: request", "method", c.Method(), "path", c.Path(),
		"module", module, "resource", resource, "mode", mode)

	// Resolve entity by module + resource path segment.
	es := h.findEntity(module, resource)
	if es == nil {
		slog.Warn("sdui: entity not found", "module", module, "resource", resource,
			"available_entities", h.entityList())
		return fiber.NewError(fiber.StatusNotFound,
			fmt.Sprintf("sdui: entity not found: %s/%s", module, resource))
	}

	slog.Info("sdui: entity resolved", "entity", es.QualifiedName, "fields", len(es.Fields))

	viewer := auth.ViewerFromContext(c.UserContext())
	rendererID := h.rendererID(c)
	locale := h.locale(c)
	schemaFP := h.schemaFPs[es.QualifiedName]

	// Extract record ID for detail and edit views.
	// For list and create views, :id is absent — uuid.Nil is the zero value.
	recordID := uuid.Nil
	if rawID := c.Params("id"); rawID != "" {
		if parsed, parseErr := uuid.Parse(rawID); parseErr == nil {
			recordID = parsed
		}
	}

	// Fetch dynamic PageContext data — all three fetches are non-blocking:
	// errors produce empty maps / nil rather than failing the request.
	tenantID := viewer.TenantID()
	reqCtx := c.UserContext()

	tenantSettings := h.fetchTenantSettings(reqCtx, tenantID)
	featureFlags := h.fetchFeatureFlags(reqCtx, tenantID, viewer)

	var recordState map[string]any
	if recordID != uuid.Nil {
		recordState = h.fetchRecordState(reqCtx, es.QualifiedName, recordID)
	}

	// Compute feature flag fingerprint for ETag cache partitioning.
	// Empty string when no flags are active (preserves pre-flag cache behaviour).
	flagFP := computeFlagFingerprint(featureFlags)

	// Build viewer adapter (bridges auth.ViewerContext → sduictx.ViewerContext).
	sduiViewer := adapt.NewViewerAdapter(c.UserContext(), viewer, h.evaluator, h.grants)

	// Build GeneratorContext.
	// PermFingerprint partitions the cache by viewer permission set so that
	// schemas generated for high-privilege viewers are not served to restricted
	// viewers from cache. The backend enforces permissions independently;
	// the fingerprint prevents stale action buttons in the UI.
	permFP := adapt.RolesFingerprint(viewer)
	builder := sduictx.NewGeneratorContext(
		tenantID,
		sduiViewer,
		es.QualifiedName,
		mode,
		rendererID,
	).
		WithLocale(locale).
		WithSchemaFingerprint(schemaFP).
		WithPermFingerprint(permFP)

	if readOnly {
		builder = builder.WithReadOnly()
	}

	ctx, err := builder.Build()
	if err != nil {
		slog.Error("sdui: context build failed", "err", err)
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	// Check for PageBuilder override before running the full generation pipeline.
	// When an entity declares a builder for this view mode and it returns a
	// non-nil schema, that schema is used directly (no generator, no cache).
	if pb := pageBuilderFor(es.PageBuilders, mode); pb != nil {
		actor := viewerToActor(viewer)
		pctx := def.PageContext{
			Actor:               &actor,
			EntityName:          es.QualifiedName,
			Kind:                viewModeToPageKind(mode),
			TenantSettings:      tenantSettings,
			EnabledFeatureFlags: featureFlags,
			RecordID:            recordID,
			RecordState:         recordState,
		}
		custom, pbErr := pb(c.UserContext(), pctx)
		if pbErr != nil {
			slog.Error("sdui: page builder error", "entity", es.QualifiedName, "err", pbErr)
			return fiber.NewError(fiber.StatusInternalServerError, pbErr.Error())
		}
		if custom != nil {
			// Builder returned an explicit schema — serve it directly.
			return c.JSON(custom)
		}
		// Builder returned nil — fall through to auto-generation below.
	}

	// Convert compiled schema → generator schema.
	gSchema := adapt.FromCompiled(es)

	// Delegate to engine.
	if h.engine == nil {
		return fiber.NewError(fiber.StatusInternalServerError, "sdui: no engine configured")
	}
	rctx := renderer.ApplyLocale(renderer.RendererContext{GenCtx: ctx}, locale)
	slog.Info("sdui: calling engine", "entity", es.QualifiedName, "renderer", rendererID, "schemaFP", schemaFP)
	resp, err := h.engine.Handle(c.UserContext(), engine.Request{
		Ctx:         ctx,
		Schema:      gSchema,
		RendererID:  rendererID,
		RendererCtx: rctx,
	})
	if err != nil {
		slog.Error("sdui: engine error", "entity", es.QualifiedName, "err", err)
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	slog.Info("sdui: engine success", "entity", es.QualifiedName,
		"cache_hit", resp.CacheHit, "schema_keys", len(resp.Output.AMISSchema))

	// Set ETag from schema fingerprint + renderer + locale + permission fingerprint
	// + tenant hash + feature flag fingerprint. All six dimensions must participate
	// so that:
	//
	//   - A permission change (new permFP) produces a new ETag.
	//   - A tenant switch (new tenantHash) produces a new ETag.
	//   - A feature flag change (new flagFP) produces a new ETag.
	//
	// Cache-Control is "private" — only the requesting browser may cache this
	// response. No shared proxy/CDN caches this resource.
	tenantHash := cache.HashTenantID(tenantID.String())
	etag := fmt.Sprintf(`"%s-%s-%s-%s-%s-%s"`, schemaFP, rendererID, locale, permFP, tenantHash, flagFP)
	c.Set(headerETag, etag)
	c.Set(headerCacheControl, sduiPublicMaxAge)

	// Check conditional request (If-None-Match).
	if c.Get("If-None-Match") == etag {
		return c.SendStatus(fiber.StatusNotModified)
	}

	return c.JSON(resp.Output.AMISSchema)
}

// ── Dynamic PageContext helpers ────────────────────────────────────────────────

// fetchTenantSettings calls the SettingsProvider for the given tenant.
// On error, logs a warning and returns an empty map so the request proceeds.
func (h *Handler) fetchTenantSettings(ctx context.Context, tenantID uuid.UUID) map[string]string {
	result := h.settings.TenantSettings(ctx, tenantID)
	if result == nil {
		return map[string]string{}
	}
	return result
}

// fetchFeatureFlags calls the FlagsProvider for the given tenant+viewer.
// On error, logs a warning and returns an empty map so the request proceeds.
func (h *Handler) fetchFeatureFlags(ctx context.Context, tenantID uuid.UUID, viewer auth.ViewerContext) map[string]bool {
	result := h.flags.EnabledFlags(ctx, tenantID, viewer)
	if result == nil {
		return map[string]bool{}
	}
	return result
}

// fetchRecordState calls the RecordFetcher for the given entity+id.
// On error, logs a warning and returns nil — PageBuilder must handle nil gracefully.
func (h *Handler) fetchRecordState(ctx context.Context, entityName string, id uuid.UUID) map[string]any {
	state, err := h.fetcher.FetchRecord(ctx, entityName, id)
	if err != nil {
		slog.Warn("sdui: record fetch failed; RecordState will be nil",
			"entity", entityName, "id", id, "err", err)
		return nil
	}
	return state
}

// computeFlagFingerprint produces a stable SHA-256 hex fingerprint of the
// active feature flags map. Returns an empty string when flags is empty,
// preserving pre-flag cache behaviour (no extra cache dimension).
func computeFlagFingerprint(flags map[string]bool) string {
	if len(flags) == 0 {
		return ""
	}
	// Sort keys for deterministic ordering.
	keys := make([]string, 0, len(flags))
	for k := range flags {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		if flags[k] {
			h.Write([]byte("1"))
		} else {
			h.Write([]byte("0"))
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ── helpers ───────────────────────────────────────────────────────────────────

// findEntity resolves an entity schema by module and resource (plural local name).
// Returns nil when not found.
func (h *Handler) findEntity(module, resource string) *compiler.EntitySchema {
	for _, es := range h.schema.Entities {
		if es.Module == module && es.APIResource == resource {
			return es
		}
	}
	return nil
}

// entityList returns a short summary of registered entities for diagnostic logging.
func (h *Handler) entityList() []string {
	list := make([]string, 0, len(h.schema.Entities))
	for _, es := range h.schema.Entities {
		list = append(list, es.Module+"/"+es.APIResource)
	}
	return list
}

// rendererID extracts the renderer ID from Accept-SDUI-Renderer header.
// Falls back to defaultRendererID.
func (h *Handler) rendererID(c *fiber.Ctx) string {
	r := strings.TrimSpace(c.Get(headerRenderer))
	if r == "" {
		return defaultRendererID
	}
	return r
}

// locale extracts the locale from Accept-Language header.
// Returns only the first language tag (e.g. "en-US" from "en-US,en;q=0.9").
// Falls back to "en-US".
func (h *Handler) locale(c *fiber.Ctx) string {
	raw := strings.TrimSpace(c.Get(headerLocale))
	if raw == "" {
		return "en-US"
	}
	// Take the first locale tag before any comma or semicolon.
	for _, sep := range []string{",", ";"} {
		if idx := strings.IndexByte(raw, sep[0]); idx >= 0 {
			raw = raw[:idx]
		}
	}
	return strings.TrimSpace(raw)
}

// ── PageBuilder helpers ───────────────────────────────────────────────────────

// pageBuilderFor returns the PageBuilder registered for the given view mode,
// or nil when no override is declared. The zero-value PageBuilderSet (all nil
// builders) is safe and represents "use auto-generation for all views".
func pageBuilderFor(pbs def.PageBuilderSet, mode sduictx.ViewMode) def.PageBuilder {
	switch mode {
	case sduictx.ViewModeList:
		return pbs.List
	case sduictx.ViewModeCreate:
		return pbs.Create
	case sduictx.ViewModeEdit:
		return pbs.Edit
	case sduictx.ViewModeDetail:
		return pbs.Detail
	default:
		return nil
	}
}

// viewModeToPageKind maps the SDUI view mode to the def.PageKind used in
// PageContext so page builders know which view they are producing.
func viewModeToPageKind(mode sduictx.ViewMode) def.PageKind {
	switch mode {
	case sduictx.ViewModeList:
		return def.PageKindList
	case sduictx.ViewModeCreate:
		return def.PageKindCreate
	case sduictx.ViewModeEdit:
		return def.PageKindEdit
	case sduictx.ViewModeDetail:
		return def.PageKindDetail
	default:
		return def.PageKind(string(mode))
	}
}

// viewerToActor constructs a def.Actor from an auth.ViewerContext using the
// ViewerContext.Actor() method which is the canonical conversion path.
func viewerToActor(v auth.ViewerContext) def.Actor {
	if a := v.Actor(); a != nil {
		return *a
	}
	return def.Actor{TenantID: v.TenantID(), Roles: v.Roles()}
}
